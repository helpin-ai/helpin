// Package crisp reads Crisp help centers from their public pages: the home
// page names the categories of one language, and a category page lists its
// sections with their articles.
package crisp

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// Key names this reader and the platform of the sites it reads.
const Key = "crisp"

const (
	concurrency = 6
	// categoryPrefix keeps category IDs apart from article IDs: both are the
	// short code that ends the address.
	categoryPrefix = "category-"
	otherID        = "other"
	otherName      = "Other articles"
)

var (
	_localePath = regexp.MustCompile(`^/([a-z]{2}(?:-[a-z]{2,4})?)(?:/|$)`)
	// A content address is "/<locale>/article/<slug>-<code>/"; the code stays
	// when the title, and with it the slug, changes.
	_contentPath = regexp.MustCompile(`^/([a-z]{2}(?:-[a-z]{2,4})?)/(article|category)/(?:([^/]*)-)?([a-z0-9]+)/?$`)
	_notSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

// Reader reads Crisp help centers.
type Reader struct {
	fetch *publichelp.Fetcher
}

var _ publichelp.Reader = (*Reader)(nil)

// New returns a reader that asks as an anonymous visitor.
func New(fetch *publichelp.Fetcher) *Reader { return &Reader{fetch: fetch} }

// Key implements publichelp.Reader.
func (r *Reader) Key() string { return Key }

// Probe implements publichelp.Reader. A Crisp help center is known by the
// "csh-" classes of its pages together with the Crisp response header or page
// script.
func (r *Reader) Probe(_ context.Context, home *publichelp.Page) (*publichelp.Site, error) {
	served := home.Header.Get("X-Crisp-Ray") != "" || bytes.Contains(home.Body, []byte("CrispHelpdesk"))
	if home.Status != http.StatusOK || !served || !bytes.Contains(home.Body, []byte(`class="csh-`)) {
		return nil, publichelp.ErrNotHelpCenter
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(home.Body))
	if err != nil {
		return nil, publichelp.ErrNotHelpCenter
	}
	locale := pageLocale(home.URL, doc)
	if locale == "" {
		return nil, publichelp.ErrNotHelpCenter
	}
	site := newSite(home.Origin(), locale)
	site.Name = siteName(doc)
	return site, nil
}

// Load implements publichelp.Reader.
func (r *Reader) Load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	progress = publichelp.Serial(progress)
	home, err := r.page(ctx, homeURL, progress)
	if err != nil {
		return nil, err
	}
	locale := pageLocale(home.url, home.doc)
	if locale == "" {
		return nil, fmt.Errorf("%w: %q has no language", publichelp.ErrNotHelpCenter, homeURL)
	}
	origin := home.url.Scheme + "://" + home.url.Host
	site := newSite(origin, locale)
	site.Name = siteName(home.doc)

	categories := links(home.doc.Selection, home.url, locale, "category")
	listed := make([]publichelp.Collection, len(categories))
	var sitemap []publichelp.SitemapEntry
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrency)
	group.Go(func() error {
		// A help center without a sitemap is read from its pages alone.
		sitemap, _ = r.fetch.Sitemap(groupCtx, origin+"/sitemap.xml", progress)
		return groupCtx.Err()
	})
	for i, category := range categories {
		group.Go(func() error {
			page, err := r.page(groupCtx, category.url, progress)
			if err != nil {
				return fmt.Errorf("read category %s: %w", category.id, err)
			}
			listed[i] = categoryOf(category, page, locale)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	// An article is kept where it shows first.
	seen := map[string]bool{}
	for _, category := range listed {
		category.Articles = unseen(category.Articles, seen)
		sections := category.Children
		category.Children = nil
		for _, section := range sections {
			if section.Articles = unseen(section.Articles, seen); len(section.Articles) > 0 {
				category.Children = append(category.Children, section)
			}
		}
		if len(category.Articles) > 0 || len(category.Children) > 0 {
			site.Collections = append(site.Collections, category)
		}
	}
	var other []publichelp.ArticleRef
	for _, entry := range sitemap {
		parsed, err := url.Parse(entry.Loc)
		if err != nil || !strings.EqualFold(parsed.Hostname(), home.url.Hostname()) {
			continue
		}
		match := _contentPath.FindStringSubmatch(parsed.Path)
		if match != nil && match[1] == locale && match[2] == "article" {
			other = append(other, publichelp.ArticleRef{ID: match[4], Title: publichelp.TitleFromSlug(match[3]), URL: entry.Loc})
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
	if match == nil || match[2] != "article" {
		return nil, fmt.Errorf("%w: %q is not a Crisp article address", publichelp.ErrInvalidURL, articleURL)
	}
	body := page.doc.Find(".csh-article-content-text").First()
	if body.Length() == 0 {
		if page.doc.Find(`input[type="password"]`).Length() > 0 {
			return nil, publichelp.ErrRestricted
		}
		return nil, fmt.Errorf("%w: article %s has no readable content", publichelp.ErrNotHelpCenter, match[4])
	}
	// The "Updated on" line and the rules around it sit inside the text.
	body.Find(".csh-article-content-updated, .csh-article-content-separate").Remove()
	// Crisp writes bold as <b><strong> and italic as <i><em>; one is enough.
	body.Find("b strong, i em").Contents().Unwrap()
	html, err := body.Html()
	if err != nil {
		return nil, fmt.Errorf("read article %s: %w", match[4], err)
	}
	title := text(page.doc.Find(".csh-article-content-header h1").First())
	description, _ := page.doc.Find(`meta[name="description"]`).Attr("content")
	return &publichelp.Article{
		ID: match[4], Title: cmp.Or(title, publichelp.TitleFromSlug(match[3])), Description: strings.TrimSpace(description),
		URL: page.url.String(), HTML: publichelp.AbsoluteHTML(html, page.url),
	}, nil
}

// categoryOf reads a category page: its sections become child collections,
// and the articles outside a named section stay on the category.
func categoryOf(category link, page *fetched, locale string) publichelp.Collection {
	out := publichelp.Collection{ID: categoryPrefix + category.id, Name: category.title, Slug: category.slug, URL: category.url}
	out.Description = text(page.doc.Find(".csh-navigation-title-list-description").First())
	sections := page.doc.Find(".csh-category-section")
	if sections.Length() == 0 {
		out.Articles = articles(page.doc.Find(".csh-category"), page.url, locale)
		return out
	}
	used := map[string]bool{}
	sections.Each(func(_ int, section *goquery.Selection) {
		listed := articles(section, page.url, locale)
		name := text(section.Find(".csh-category-section-title").First())
		if name == "" {
			out.Articles = append(out.Articles, listed...)
			return
		}
		// A section has no address or number of its own; its name within the
		// category identifies it.
		id := out.ID + "-" + strings.Trim(_notSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
		for used[id] {
			id += "-2"
		}
		used[id] = true
		out.Children = append(out.Children, publichelp.Collection{ID: id, Name: name, Articles: listed})
	})
	return out
}

func articles(scope *goquery.Selection, base *url.URL, locale string) []publichelp.ArticleRef {
	var out []publichelp.ArticleRef
	for _, article := range links(scope, base, locale, "article") {
		out = append(out, publichelp.ArticleRef{ID: article.id, Title: article.title, URL: article.url})
	}
	return out
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

// link is a category or article as a page links to it.
type link struct {
	id, slug, title, url string
}

// links returns the links of one kind and language inside scope, each once,
// in page order.
func links(scope *goquery.Selection, base *url.URL, locale, kind string) []link {
	var out []link
	seen := map[string]bool{}
	scope.Find("a[href]").Each(func(_ int, anchor *goquery.Selection) {
		target, err := base.Parse(strings.TrimSpace(anchor.AttrOr("href", "")))
		if err != nil || !strings.EqualFold(target.Hostname(), base.Hostname()) {
			return
		}
		match := _contentPath.FindStringSubmatch(target.Path)
		if match == nil || match[1] != locale || match[2] != kind || seen[match[4]] {
			return
		}
		seen[match[4]] = true
		target.RawQuery, target.Fragment = "", ""
		// A card holds the name in its own element, next to a description.
		title := text(anchor.Find(".csh-category-item-title, .csh-category-badge").First())
		out = append(out, link{
			id: match[4], slug: match[3], url: target.String(),
			title: cmp.Or(title, text(anchor), publichelp.TitleFromSlug(match[3])),
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

// pageLocale is the language a page is in: the first part of its address,
// or else what the page declares.
func pageLocale(address *url.URL, doc *goquery.Document) string {
	if match := _localePath.FindStringSubmatch(address.Path); match != nil {
		return match[1]
	}
	declared := strings.ToLower(strings.TrimSpace(doc.Find("html").AttrOr("lang", "")))
	if match := _localePath.FindStringSubmatch("/" + declared); match != nil {
		return match[1]
	}
	return ""
}

// siteName takes the help center's name from a page.
func siteName(doc *goquery.Document) string {
	if name, ok := doc.Find(`meta[property="og:site_name"]`).Attr("content"); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(doc.Find(".csh-header-main-logo img").AttrOr("alt", ""))
}

func newSite(origin, locale string) *publichelp.Site {
	return &publichelp.Site{
		Platform: Key, BaseURL: origin + "/" + locale + "/", Locale: locale,
		MatchLink: func(link *url.URL) (publichelp.LinkKind, string, bool) {
			match := _contentPath.FindStringSubmatch(link.Path)
			switch {
			case match == nil:
				return "", "", false
			case match[2] == "article":
				return publichelp.LinkArticle, match[4], true
			}
			return publichelp.LinkCollection, categoryPrefix + match[4], true
		},
	}
}
