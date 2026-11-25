// Package hubspot reads HubSpot knowledge bases from their public pages: the
// home page and the menu beside every category name the categories, and a
// category page lists its subcategories with their articles.
package hubspot

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// Key names this reader and the platform of the sites it reads.
const Key = "hubspot"

const (
	concurrency = 6
	// maxCategories bounds one read, so menus that keep naming new categories
	// cannot stall an import.
	maxCategories = 500
	// categoryPrefix keeps collection IDs apart from article IDs: both are the
	// address path.
	categoryPrefix = "category-"
	otherID        = "other"
	otherName      = "Other articles"
	// loginPath is where HubSpot sends a visitor who must sign in.
	loginPath = "/_hcms/mem/login"
	// systemPrefix starts the search, "not found" and ticket pages every
	// knowledge base has beside its articles.
	systemPrefix = "kb-"
)

// Reader reads HubSpot knowledge bases.
type Reader struct {
	fetch *publichelp.Fetcher
}

var _ publichelp.Reader = (*Reader)(nil)

// New returns a reader that asks as an anonymous visitor.
func New(fetch *publichelp.Fetcher) *Reader { return &Reader{fetch: fetch} }

// Key implements publichelp.Reader.
func (r *Reader) Key() string { return Key }

// Probe implements publichelp.Reader. A knowledge base is known by the
// HubSpot response header together with the "hs-kb-" classes of its pages.
func (r *Reader) Probe(_ context.Context, home *publichelp.Page) (*publichelp.Site, error) {
	if home.Header.Get("X-Hs-Hub-Id") == "" || home.Status != http.StatusOK {
		return nil, publichelp.ErrNotHelpCenter
	}
	if strings.HasPrefix(home.URL.Path, loginPath) {
		return nil, &publichelp.PlatformError{Platform: Key, Err: publichelp.ErrRestricted}
	}
	if !bytes.Contains(home.Body, []byte(`class="hs-kb-`)) {
		return nil, publichelp.ErrNotHelpCenter
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(home.Body))
	if err != nil {
		return nil, publichelp.ErrNotHelpCenter
	}
	// An address inside the knowledge base names its home in the breadcrumbs.
	base := home.URL
	if crumb, ok := doc.Find("ol.hs-kb-breadcrumbs a[href]").First().Attr("href"); ok {
		if parsed, err := home.URL.Parse(strings.TrimSpace(crumb)); err == nil && strings.EqualFold(parsed.Hostname(), home.URL.Hostname()) {
			base = parsed
		}
	}
	site := newSite(base)
	if base == home.URL {
		site.Name = text(doc.Find("title").First())
	}
	site.Locale = pageLocale(doc)
	return site, nil
}

// Load implements publichelp.Reader.
func (r *Reader) Load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	progress = publichelp.Serial(progress)
	home, err := r.page(ctx, homeURL, progress)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(home.url.Path, loginPath) {
		return nil, &publichelp.PlatformError{Platform: Key, Err: publichelp.ErrRestricted}
	}
	site := newSite(home.url)
	site.Name = text(home.doc.Find("title").First())
	site.Locale = pageLocale(home.doc)
	base, _ := url.Parse(site.BaseURL)

	var sitemap []publichelp.SitemapEntry
	sitemapDone := make(chan struct{})
	go func() {
		defer close(sitemapDone)
		// A knowledge base without a sitemap is read from its pages alone.
		sitemap, _ = r.fetch.Sitemap(ctx, base.Scheme+"://"+base.Host+"/sitemap.xml", progress)
	}()

	// The home page may show only some categories; the menu beside a category
	// names them all, so the pages are read in rounds until no new one shows.
	var listed []publichelp.Collection
	known := map[string]bool{}
	queue := newLinks(homeCategories(home), known)
	for len(queue) > 0 && len(listed) < maxCategories {
		queue = queue[:min(len(queue), maxCategories-len(listed))]
		found := make([]publichelp.Collection, len(queue))
		menus := make([][]link, len(queue))
		group, groupCtx := errgroup.WithContext(ctx)
		group.SetLimit(concurrency)
		for i, category := range queue {
			group.Go(func() error {
				page, err := r.page(groupCtx, category.url, progress)
				if err != nil {
					return fmt.Errorf("read category %s: %w", category.id, err)
				}
				found[i] = categoryOf(category, page)
				menus[i] = menuCategories(page)
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			<-sitemapDone
			return nil, err
		}
		listed = append(listed, found...)
		queue = nil
		for _, menu := range menus {
			queue = append(queue, newLinks(menu, known)...)
		}
	}
	<-sitemapDone
	if err := ctx.Err(); err != nil {
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
	other, err := r.unlisted(ctx, base, site.Locale, sitemap, known, seen, progress)
	if err != nil {
		return nil, err
	}
	if len(other) > 0 {
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
	if strings.HasPrefix(page.url.Path, loginPath) {
		return nil, publichelp.ErrRestricted
	}
	body := page.doc.Find("article.knowledgebase-post").First()
	if body.Length() == 0 {
		return nil, fmt.Errorf("%w: %q is not an article", publichelp.ErrNotFound, articleURL)
	}
	// The title and the line under it are fields of their own, above the text.
	heading := body.Find("h1").First()
	subtitle := body.Find(`h2[data-hs-cos-type="inline_text_field"]`).First()
	title, summary := text(heading), text(subtitle)
	heading.Remove()
	subtitle.Remove()
	// The text sits in rich text fields inside layout rows; a theme without
	// them keeps the whole article.
	var html strings.Builder
	parts := body.Find(`[data-hs-cos-type="inline_richtext_field"]`)
	if parts.Length() == 0 {
		parts = body
	}
	for _, part := range parts.EachIter() {
		inner, err := part.Html()
		if err != nil {
			return nil, fmt.Errorf("read article %s: %w", pathID(page.url), err)
		}
		html.WriteString(inner)
	}
	description, _ := page.doc.Find(`meta[name="description"]`).Attr("content")
	page.url.RawQuery, page.url.Fragment = "", ""
	return &publichelp.Article{
		ID: pathID(page.url), Title: cmp.Or(title, text(page.doc.Find("title").First()), publichelp.TitleFromSlug(lastSegment(page.url))),
		Description: cmp.Or(summary, strings.TrimSpace(description)),
		URL:         page.url.String(), HTML: publichelp.AbsoluteHTML(html.String(), page.url),
	}, nil
}

// unlisted returns the articles the sitemap names that no category page
// lists. The sitemap does not say what a page is, so each one is opened.
func (r *Reader) unlisted(
	ctx context.Context, base *url.URL, locale string, sitemap []publichelp.SitemapEntry,
	categories, seen map[string]bool, progress func(),
) ([]publichelp.ArticleRef, error) {
	root := strings.TrimRight(base.Path, "/")
	var candidates []*url.URL
	for _, entry := range sitemap {
		parsed, err := url.Parse(entry.Loc)
		if err != nil || !strings.EqualFold(parsed.Hostname(), base.Hostname()) || !strings.HasPrefix(parsed.Path, root+"/") {
			continue
		}
		id := pathID(parsed)
		rest := strings.TrimPrefix(parsed.Path, root+"/")
		if rest == "" || strings.HasPrefix(rest, systemPrefix) || seen[id] || categories[id] {
			continue
		}
		seen[id] = true
		candidates = append(candidates, parsed)
	}
	found := make([]*publichelp.ArticleRef, len(candidates))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrency)
	for i, candidate := range candidates {
		group.Go(func() error {
			page, err := r.page(groupCtx, candidate.String(), progress)
			if err != nil {
				// A page that cannot be read is not an article to import.
				return groupCtx.Err()
			}
			article := page.doc.Find("article.knowledgebase-post").First()
			// Without a language in its address, another language's page is
			// told apart by what it declares.
			if article.Length() == 0 || (locale != "" && pageLocale(page.doc) != locale) {
				return nil
			}
			found[i] = &publichelp.ArticleRef{
				ID: pathID(candidate), URL: candidate.String(),
				Title: cmp.Or(text(article.Find("h1").First()), publichelp.TitleFromSlug(lastSegment(candidate))),
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	var out []publichelp.ArticleRef
	for _, article := range found {
		if article != nil {
			out = append(out, *article)
		}
	}
	return out, nil
}

// homeCategories returns the categories the home page shows.
func homeCategories(home *fetched) []link {
	var out []link
	for _, anchor := range home.doc.Find("a.hs-kb-home-list__category-link[href]").EachIter() {
		if category, ok := linkTo(anchor, home.url); ok {
			category.title = text(anchor.Find(".hs-kb-home-list__text__category-title").First())
			out = append(out, category)
		}
	}
	return out
}

// menuCategories returns the categories the menu beside a category names.
// The page's own entry and the subcategories point inside a page.
func menuCategories(page *fetched) []link {
	var out []link
	for _, anchor := range page.doc.Find("a.hs-kb-category-accordion_link[href]").EachIter() {
		target, err := page.url.Parse(strings.TrimSpace(anchor.AttrOr("href", "")))
		if err != nil || (target.Fragment != "" && target.Fragment != "main-content") || target.Path == page.url.Path {
			continue
		}
		if category, ok := linkTo(anchor, page.url); ok {
			category.title = text(anchor)
			out = append(out, category)
		}
	}
	return out
}

// categoryOf reads a category page: its subcategories become child
// collections, and the articles outside one stay on the category.
func categoryOf(category link, page *fetched) publichelp.Collection {
	head := page.doc.Find(".hs-kb-category-listing").First()
	out := publichelp.Collection{
		ID: categoryPrefix + category.id, Slug: lastSegment(page.url), URL: category.url,
		Name:        cmp.Or(text(head.Find("h1").First()), category.title, publichelp.TitleFromSlug(lastSegment(page.url))),
		Description: text(head.Find("p").First()),
	}
	children := map[string]int{}
	for _, anchor := range page.doc.Find("a.hs-kb-category-article-list__link[href]").EachIter() {
		article, ok := linkTo(anchor, page.url)
		if !ok {
			continue
		}
		ref := publichelp.ArticleRef{ID: article.id, Title: cmp.Or(text(anchor), publichelp.TitleFromSlug(article.id)), URL: article.url}
		section := anchor.Closest(".hs-kb-subcategory-listing")
		name := text(section.Find("h2").First())
		if name == "" {
			out.Articles = append(out.Articles, ref)
			continue
		}
		// A subcategory is a part of its category's page, named by its anchor.
		id := out.ID + "#" + cmp.Or(section.AttrOr("id", ""), name)
		at, ok := children[id]
		if !ok {
			at = len(out.Children)
			children[id] = at
			out.Children = append(out.Children, publichelp.Collection{ID: id, Name: name})
		}
		out.Children[at].Articles = append(out.Children[at].Articles, ref)
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
	id, title, url string
}

// linkTo reads where an anchor on the knowledge base's own host points,
// without the language marker HubSpot adds to every link.
func linkTo(anchor *goquery.Selection, base *url.URL) (link, bool) {
	target, err := base.Parse(strings.TrimSpace(anchor.AttrOr("href", "")))
	if err != nil || !strings.EqualFold(target.Hostname(), base.Hostname()) || pathID(target) == "" {
		return link{}, false
	}
	target.RawQuery, target.Fragment = "", ""
	return link{id: pathID(target), url: target.String()}, true
}

// newLinks keeps the links not yet in known, each once, and marks them.
func newLinks(links []link, known map[string]bool) []link {
	var out []link
	for _, found := range links {
		if !known[found.id] {
			known[found.id] = true
			out = append(out, found)
		}
	}
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

// pathID identifies a page by its address path: a knowledge base page has no
// number of its own in its listing.
// shortcut: an article whose address is changed is imported again as a new
// article; read the article-id of the feedback form if that becomes a problem.
func pathID(address *url.URL) string { return strings.Trim(address.Path, "/") }

func lastSegment(address *url.URL) string {
	path := strings.Trim(address.Path, "/")
	return path[strings.LastIndex(path, "/")+1:]
}

func text(node *goquery.Selection) string { return strings.Join(strings.Fields(node.Text()), " ") }

func pageLocale(doc *goquery.Document) string {
	return strings.ToLower(strings.TrimSpace(doc.Find("html").AttrOr("lang", "")))
}

// newSite returns the site whose home is at the given address. The home may
// sit below the host's root, as in "/en" or "/knowledge".
func newSite(home *url.URL) *publichelp.Site {
	base := *home
	base.RawQuery, base.Fragment = "", ""
	return &publichelp.Site{Platform: Key, BaseURL: base.String()}
}
