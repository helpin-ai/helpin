// Package helpscoutdocs reads Help Scout Docs sites from their public pages:
// the home page names the collections, a collection page its categories, and
// the category pages their articles.
package helpscoutdocs

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// Key names this reader and the platform of the sites it reads.
const Key = "helpscout"

const (
	concurrency = 6
	// maxPages bounds one category, so a category that never stops paging
	// cannot stall an import. A page lists 20 articles.
	maxPages = 250
	// Collections, categories and articles are numbered separately, so the
	// collection kinds carry a prefix.
	collectionPrefix = "collection-"
	categoryPrefix   = "category-"
	otherID          = "other"
	otherName        = "Other articles"
)

var (
	// A content address is "/article/<number>-<slug>"; a category page past
	// the first adds "/<page>".
	_contentPath = regexp.MustCompile(`^/(article|category|collection)/(\d+)(?:-([^/]*))?(?:/\d*)?$`)
	// _assetHost serves the stylesheet of every Docs site.
	_assetHost = []byte("d3eto7onm69fcz.cloudfront.net")
)

// Reader reads Help Scout Docs sites.
type Reader struct {
	fetch *publichelp.Fetcher
}

var _ publichelp.Reader = (*Reader)(nil)

// New returns a reader that asks as an anonymous visitor.
func New(fetch *publichelp.Fetcher) *Reader { return &Reader{fetch: fetch} }

// Key implements publichelp.Reader.
func (r *Reader) Key() string { return Key }

// Probe implements publichelp.Reader. A Docs site is known by the Help Scout
// stylesheet its pages load, or by its helpscoutdocs.com address.
func (r *Reader) Probe(_ context.Context, home *publichelp.Page) (*publichelp.Site, error) {
	hosted := strings.HasSuffix(strings.ToLower(home.URL.Hostname()), ".helpscoutdocs.com")
	if home.Status != http.StatusOK || (!hosted && !bytes.Contains(home.Body, _assetHost)) {
		return nil, publichelp.ErrNotHelpCenter
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(home.Body))
	if err != nil || doc.Find("#mainNav, #docsSearch, #fullArticle").Length() == 0 {
		return nil, publichelp.ErrNotHelpCenter
	}
	site := newSite(home.Origin())
	site.Name = siteName(doc)
	return site, nil
}

// Load implements publichelp.Reader.
func (r *Reader) Load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	progress = publichelp.Serial(progress)
	base, err := url.Parse(homeURL)
	if err != nil || base.Hostname() == "" {
		return nil, fmt.Errorf("%w: %q", publichelp.ErrInvalidURL, homeURL)
	}
	site := newSite(base.Scheme + "://" + base.Host)
	home, err := r.page(ctx, site.BaseURL, progress)
	if err != nil {
		return nil, err
	}
	site.Name = siteName(home.doc)
	collections := links(home.doc.Selection, home.url, "collection")

	// The sitemap and the collection pages, then every category.
	var sitemap []publichelp.SitemapEntry
	categories := make([][]link, len(collections))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrency)
	group.Go(func() error {
		// A site without a sitemap is read from its pages alone.
		sitemap, _ = r.fetch.Sitemap(groupCtx, site.BaseURL+"sitemap.xml", progress)
		return groupCtx.Err()
	})
	for i, collection := range collections {
		group.Go(func() error {
			page, err := r.page(groupCtx, collection.url, progress)
			if err != nil {
				return fmt.Errorf("read collection %s: %w", collection.id, err)
			}
			categories[i] = links(page.doc.Find("#sidebar"), page.url, "category")
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	listed := make([][]publichelp.Collection, len(collections))
	group, groupCtx = errgroup.WithContext(ctx)
	group.SetLimit(concurrency)
	for i := range collections {
		listed[i] = make([]publichelp.Collection, len(categories[i]))
		for j, category := range categories[i] {
			group.Go(func() error {
				found, err := r.category(groupCtx, category, progress)
				if err != nil {
					return fmt.Errorf("read category %s: %w", category.id, err)
				}
				listed[i][j] = found
				return nil
			})
		}
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	// An article in several categories is kept where it shows first.
	seen := map[string]bool{}
	for i, collection := range collections {
		out := publichelp.Collection{
			ID: collectionPrefix + collection.id, Name: collection.title, Slug: collection.slug, URL: collection.url,
		}
		for _, category := range listed[i] {
			category.Articles = unseen(category.Articles, seen)
			if len(category.Articles) > 0 {
				out.Children = append(out.Children, category)
			}
		}
		if len(out.Children) > 0 {
			site.Collections = append(site.Collections, out)
		}
	}
	var other []publichelp.ArticleRef
	for _, entry := range sitemap {
		parsed, err := url.Parse(entry.Loc)
		if err != nil || !strings.EqualFold(parsed.Hostname(), base.Hostname()) {
			continue
		}
		if match := _contentPath.FindStringSubmatch(parsed.Path); match != nil && match[1] == "article" {
			other = append(other, publichelp.ArticleRef{ID: match[2], Title: publichelp.TitleFromSlug(match[3]), URL: entry.Loc})
		}
	}
	if other = unseen(other, seen); len(other) > 0 {
		site.Collections = append(site.Collections, publichelp.Collection{ID: otherID, Name: otherName, Articles: other})
	}
	return site, nil
}

// Article implements publichelp.Reader.
func (r *Reader) Article(ctx context.Context, articleURL string) (*publichelp.Article, error) {
	page, err := r.page(ctx, strings.TrimSpace(articleURL), nil)
	if err != nil {
		return nil, err
	}
	match := _contentPath.FindStringSubmatch(page.url.Path)
	if match == nil || match[1] != "article" {
		// Docs has articles that only forward to a page somewhere else.
		return nil, fmt.Errorf("%w: %q leads to %s, which is not an article", publichelp.ErrNotFound, articleURL, page.url.Host)
	}
	body := page.doc.Find("#fullArticle").First()
	if body.Length() == 0 {
		if page.doc.Find(`input[type="password"]`).Length() > 0 {
			return nil, publichelp.ErrRestricted
		}
		return nil, fmt.Errorf("%w: article %s has no readable content", publichelp.ErrNotHelpCenter, match[2])
	}
	title := text(body.Find("h1.title").First())
	body.Find("h1.title, .printArticle").Remove()
	html, err := body.Html()
	if err != nil {
		return nil, fmt.Errorf("read article %s: %w", match[2], err)
	}
	description, _ := page.doc.Find(`meta[name="description"]`).Attr("content")
	return &publichelp.Article{
		ID: match[2], Title: cmp.Or(title, publichelp.TitleFromSlug(match[3])), Description: strings.TrimSpace(description),
		URL: page.url.String(), HTML: publichelp.AbsoluteHTML(html, page.url),
	}, nil
}

// category reads every page of a category's article list.
func (r *Reader) category(ctx context.Context, category link, progress func()) (publichelp.Collection, error) {
	out := publichelp.Collection{ID: categoryPrefix + category.id, Name: category.title, Slug: category.slug, URL: category.url}
	seen := map[string]bool{}
	next := category.url
	for number := 1; next != "" && number <= maxPages; number++ {
		page, err := r.page(ctx, next, progress)
		if err != nil {
			return out, err
		}
		if match := _contentPath.FindStringSubmatch(page.url.Path); match != nil && match[1] == "article" {
			// A category of one article forwards to the article.
			title := text(page.doc.Find("#fullArticle h1.title").First())
			out.Articles = []publichelp.ArticleRef{{
				ID: match[2], Title: cmp.Or(title, publichelp.TitleFromSlug(match[3])), URL: page.url.String(),
			}}
			return out, nil
		}
		if number == 1 {
			out.Name = cmp.Or(text(page.doc.Find("#categoryHead h1").First()), out.Name)
			out.Description = text(page.doc.Find("#categoryHead .descrip").First())
		}
		for _, article := range links(page.doc.Find(".articleList"), page.url, "article") {
			if !seen[article.id] {
				seen[article.id] = true
				out.Articles = append(out.Articles, publichelp.ArticleRef{ID: article.id, Title: article.title, URL: article.url})
			}
		}
		// The pager links every page; the next one ends in "/<number+1>".
		next = ""
		ending := "/" + strconv.Itoa(number+1)
		page.doc.Find(".pagination a[href]").EachWithBreak(func(_ int, anchor *goquery.Selection) bool {
			target, err := page.url.Parse(anchor.AttrOr("href", ""))
			if err != nil || !strings.HasSuffix(target.Path, ending) || _contentPath.FindStringSubmatch(target.Path) == nil {
				return true
			}
			next = target.String()
			return false
		})
	}
	return out, nil
}

type fetched struct {
	url *url.URL
	doc *goquery.Document
}

func (r *Reader) page(ctx context.Context, target string, progress func()) (*fetched, error) {
	page, err := r.fetch.Get(ctx, target)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress()
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page.Body))
	if err != nil {
		return nil, fmt.Errorf("%w: %s could not be read", publichelp.ErrNotHelpCenter, target)
	}
	return &fetched{url: page.URL, doc: doc}, nil
}

// link is a collection, category or article as a page links to it.
type link struct {
	id, slug, title, url string
}

// links returns the links of one kind inside scope, each once, in page order.
func links(scope *goquery.Selection, base *url.URL, kind string) []link {
	var out []link
	seen := map[string]bool{}
	scope.Find("a[href]").Each(func(_ int, anchor *goquery.Selection) {
		target, err := base.Parse(strings.TrimSpace(anchor.AttrOr("href", "")))
		if err != nil || !strings.EqualFold(target.Hostname(), base.Hostname()) {
			return
		}
		match := _contentPath.FindStringSubmatch(target.Path)
		if match == nil || match[1] != kind || seen[match[2]] {
			return
		}
		seen[match[2]] = true
		target.RawQuery, target.Fragment = "", ""
		out = append(out, link{
			id: match[2], slug: match[3], url: target.String(),
			title: cmp.Or(text(anchor), publichelp.TitleFromSlug(match[3])),
		})
	})
	return out
}

// unseen keeps the articles not yet in seen, and marks them.
func unseen(articles []publichelp.ArticleRef, seen map[string]bool) []publichelp.ArticleRef {
	var out []publichelp.ArticleRef
	for _, article := range articles {
		if !seen[article.ID] {
			seen[article.ID] = true
			out = append(out, article)
		}
	}
	return out
}

func text(node *goquery.Selection) string { return strings.Join(strings.Fields(node.Text()), " ") }

// siteName takes the site's name from the brand in its header.
func siteName(doc *goquery.Document) string {
	brand := doc.Find("#mainNav .brand").First()
	name := cmp.Or(text(brand), strings.TrimSpace(brand.Find("img").AttrOr("alt", "")))
	if name != "" {
		return name
	}
	// Every page but the home page titles itself "<page> - <site>".
	title := text(doc.Find("title").First())
	if _, site, ok := strings.Cut(title, " - "); ok {
		return site
	}
	return title
}

func newSite(origin string) *publichelp.Site {
	return &publichelp.Site{
		Platform: Key, BaseURL: origin + "/",
		MatchLink: func(link *url.URL) (publichelp.LinkKind, string, bool) {
			match := _contentPath.FindStringSubmatch(link.Path)
			switch {
			case match == nil:
				return "", "", false
			case match[1] == "article":
				return publichelp.LinkArticle, match[2], true
			case match[1] == "category":
				return publichelp.LinkCollection, categoryPrefix + match[2], true
			}
			return publichelp.LinkCollection, collectionPrefix + match[2], true
		},
	}
}
