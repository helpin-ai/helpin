// Package zendesk reads Zendesk help centers through the Help Center API,
// which Zendesk serves to anonymous visitors on the help center's own
// address. A help center that refuses anonymous reads can be read with an
// API token instead.
package zendesk

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// Key names this reader and the platform of the sites it reads.
const Key = "zendesk"

const (
	pageSize = 100
	// maxPages bounds one listing, so a help center that never stops paging
	// cannot stall an import. It allows 50,000 articles.
	maxPages = 500
	// categoryPrefix keeps category IDs apart from section IDs, which come
	// from a different sequence.
	categoryPrefix = "category-"
)

var (
	_localePath = regexp.MustCompile(`^/hc/([A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*)(?:/|$)`)
	// An article address is "/hc/<locale>/articles/<number>-<slug>"; the
	// locale may be missing and the number may stand alone.
	_contentPath = regexp.MustCompile(`^/hc/(?:([A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*)/)?(articles|sections|categories)/(\d+)`)
)

// Reader reads Zendesk help centers.
type Reader struct {
	fetch        *publichelp.Fetcher
	email, token string
}

var _ publichelp.Reader = (*Reader)(nil)

// New returns a reader that asks as an anonymous visitor.
func New(fetch *publichelp.Fetcher) *Reader { return &Reader{fetch: fetch} }

// WithToken returns a reader that signs its requests with a Zendesk API token
// of the given agent, for help centers that refuse anonymous reads.
func (r *Reader) WithToken(email, token string) *Reader {
	return &Reader{fetch: r.fetch, email: strings.TrimSpace(email), token: strings.TrimSpace(token)}
}

// Key implements publichelp.Reader.
func (r *Reader) Key() string { return Key }

// Probe implements publichelp.Reader. The page of a Zendesk help center may
// be behind a bot challenge while its API answers, so a page that could not
// be read is still asked through the API.
func (r *Reader) Probe(ctx context.Context, home *publichelp.Page) (*publichelp.Site, error) {
	looksLikeZendesk := strings.HasSuffix(strings.ToLower(home.URL.Hostname()), ".zendesk.com") ||
		home.Header.Get("X-Zendesk-Origin-Server") != "" ||
		bytes.Contains(home.Body, []byte("zdassets.com")) || bytes.Contains(home.Body, []byte("HelpCenter.user"))
	if home.Status == http.StatusOK && !looksLikeZendesk {
		return nil, publichelp.ErrNotHelpCenter
	}
	// A token is shown only to a host known to be Zendesk; a page that could
	// not be read is asked without it.
	asker := r
	if !looksLikeZendesk {
		asker = New(r.fetch)
	}
	locales, err := asker.locales(ctx, home.Origin())
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if looksLikeZendesk && (errors.Is(err, publichelp.ErrChallenged) || errors.Is(err, publichelp.ErrRestricted)) {
			return nil, &publichelp.PlatformError{Platform: Key, Err: err}
		}
		return nil, publichelp.ErrNotHelpCenter
	}
	locale := locales.Default
	if match := _localePath.FindStringSubmatch(home.URL.Path); match != nil {
		if i := slices.IndexFunc(locales.Locales, func(known string) bool { return strings.EqualFold(known, match[1]) }); i >= 0 {
			locale = locales.Locales[i]
		}
	}
	site := newSite(home.Origin(), locale)
	if home.Status == http.StatusOK {
		site.Name = pageName(home.Body)
	}
	return site, nil
}

// Load implements publichelp.Reader.
func (r *Reader) Load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	parsed, err := url.Parse(homeURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: %q", publichelp.ErrInvalidURL, homeURL)
	}
	origin := parsed.Scheme + "://" + parsed.Host
	locale := ""
	if match := _localePath.FindStringSubmatch(parsed.Path); match != nil {
		locale = match[1]
	} else {
		locales, err := r.locales(ctx, origin)
		if err != nil {
			return nil, err
		}
		locale = locales.Default
	}
	site := newSite(origin, locale)
	api := origin + "/api/v2/help_center/" + locale
	var categories []category
	var sections []section
	var articles []article
	for _, list := range []struct {
		name string
		keep func(listPage)
	}{
		{name: "categories", keep: func(p listPage) { categories = append(categories, p.Categories...) }},
		{name: "sections", keep: func(p listPage) { sections = append(sections, p.Sections...) }},
		{name: "articles", keep: func(p listPage) { articles = append(articles, p.Articles...) }},
	} {
		if err := r.list(ctx, api+"/"+list.name+".json?page%5Bsize%5D="+strconv.Itoa(pageSize), list.keep, progress); err != nil {
			return nil, fmt.Errorf("list %s: %w", list.name, err)
		}
	}
	site.Collections = tree(categories, sections, articles)
	return site, nil
}

// Article implements publichelp.Reader.
func (r *Reader) Article(ctx context.Context, articleURL string) (*publichelp.Article, error) {
	parsed, err := url.Parse(strings.TrimSpace(articleURL))
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: article address %q", publichelp.ErrInvalidURL, articleURL)
	}
	match := _contentPath.FindStringSubmatch(parsed.Path)
	if match == nil || match[2] != "articles" {
		return nil, fmt.Errorf("%w: %q is not a Zendesk article address", publichelp.ErrInvalidURL, articleURL)
	}
	api := parsed.Scheme + "://" + parsed.Host + "/api/v2/help_center/"
	if match[1] != "" {
		api += match[1] + "/"
	}
	var out struct {
		Article *article `json:"article"`
	}
	if err := r.getJSON(ctx, api+"articles/"+match[3]+".json", &out); err != nil {
		return nil, err
	}
	if out.Article == nil {
		return nil, fmt.Errorf("%w: article %s has no readable content", publichelp.ErrNotHelpCenter, match[3])
	}
	found := out.Article
	base := parsed
	if published, err := url.Parse(found.HTMLURL); err == nil && published.Hostname() != "" {
		base = published
	}
	return &publichelp.Article{
		ID: found.id(), Title: found.title(), URL: base.String(), HTML: publichelp.AbsoluteHTML(found.Body, base),
	}, nil
}

type localeList struct {
	Locales []string `json:"locales"`
	Default string   `json:"default_locale"`
}

func (r *Reader) locales(ctx context.Context, origin string) (*localeList, error) {
	var out localeList
	if err := r.getJSON(ctx, origin+"/api/v2/help_center/locales.json", &out); err != nil {
		return nil, err
	}
	if out.Default == "" {
		return nil, publichelp.ErrNotHelpCenter
	}
	return &out, nil
}

func (r *Reader) getJSON(ctx context.Context, target string, out any) error {
	page, err := r.fetch.Get(ctx, target, func(req *http.Request) {
		req.Header.Set("Accept", "application/json")
		if r.token != "" {
			req.SetBasicAuth(r.email+"/token", r.token)
		}
	})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(page.Body, out); err != nil {
		return fmt.Errorf("%w: the help center API did not answer with JSON", publichelp.ErrNotHelpCenter)
	}
	return nil
}

type listPage struct {
	Meta struct {
		HasMore bool `json:"has_more"`
	} `json:"meta"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
	Categories []category `json:"categories"`
	Sections   []section  `json:"sections"`
	Articles   []article  `json:"articles"`
}

// list reads every page of a listing, following the cursor the API returns.
func (r *Reader) list(ctx context.Context, first string, keep func(listPage), progress func()) error {
	start, err := url.Parse(first)
	if err != nil {
		return fmt.Errorf("%w: %q", publichelp.ErrInvalidURL, first)
	}
	next := first
	for pages := 0; next != "" && pages < maxPages; pages++ {
		var page listPage
		if err := r.getJSON(ctx, next, &page); err != nil {
			return err
		}
		keep(page)
		if progress != nil {
			progress()
		}
		next = ""
		// The next page must stay on the help center's host: the address comes
		// from the response, and the token must not follow it elsewhere.
		if following, err := url.Parse(page.Links.Next); page.Meta.HasMore && err == nil && following.Host == start.Host {
			next = following.String()
		}
	}
	return nil
}

type category struct {
	ID          json.Number `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	HTMLURL     string      `json:"html_url"`
	Position    int         `json:"position"`
}

type section struct {
	ID              json.Number `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	HTMLURL         string      `json:"html_url"`
	Position        int         `json:"position"`
	CategoryID      json.Number `json:"category_id"`
	ParentSectionID json.Number `json:"parent_section_id"`
}

type article struct {
	ID        json.Number `json:"id"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	HTMLURL   string      `json:"html_url"`
	Position  int         `json:"position"`
	SectionID json.Number `json:"section_id"`
	Draft     bool        `json:"draft"`
}

func (a article) id() string { return a.ID.String() }

func (a article) title() string { return cmp.Or(strings.TrimSpace(a.Title), "Untitled") }

// tree arranges categories, their sections and the sections inside those, in
// the order the help center shows them, with each section's articles. Drafts,
// which only a signed request sees, are left out.
func tree(categories []category, sections []section, articles []article) []publichelp.Collection {
	slices.SortStableFunc(categories, func(a, b category) int { return a.Position - b.Position })
	slices.SortStableFunc(sections, func(a, b section) int { return a.Position - b.Position })
	slices.SortStableFunc(articles, func(a, b article) int { return a.Position - b.Position })
	articlesBySection := map[string][]publichelp.ArticleRef{}
	for _, found := range articles {
		if found.Draft || found.HTMLURL == "" {
			continue
		}
		articlesBySection[found.SectionID.String()] = append(articlesBySection[found.SectionID.String()],
			publichelp.ArticleRef{ID: found.id(), Title: found.title(), URL: found.HTMLURL})
	}
	// A section sits under its parent section when it has one, otherwise under
	// its category.
	sectionsByParent := map[string][]section{}
	for _, found := range sections {
		parent := categoryPrefix + found.CategoryID.String()
		if found.ParentSectionID.String() != "" {
			parent = found.ParentSectionID.String()
		}
		sectionsByParent[parent] = append(sectionsByParent[parent], found)
	}
	var below func(parent string, depth int) []publichelp.Collection
	below = func(parent string, depth int) []publichelp.Collection {
		// Zendesk nests sections five deep; anything deeper is a loop.
		if depth > 8 {
			return nil
		}
		var out []publichelp.Collection
		for _, found := range sectionsByParent[parent] {
			id := found.ID.String()
			out = append(out, publichelp.Collection{
				ID: id, Name: cmp.Or(strings.TrimSpace(found.Name), "Untitled"), Description: strings.TrimSpace(found.Description),
				URL: found.HTMLURL, Articles: articlesBySection[id], Children: below(id, depth+1),
			})
		}
		return out
	}
	out := make([]publichelp.Collection, 0, len(categories))
	for _, found := range categories {
		id := categoryPrefix + found.ID.String()
		collection := publichelp.Collection{
			ID: id, Name: cmp.Or(strings.TrimSpace(found.Name), "Untitled"), Description: strings.TrimSpace(found.Description),
			URL: found.HTMLURL, Children: below(id, 0),
		}
		if hasArticles(collection) {
			out = append(out, collection)
		}
	}
	return out
}

// hasArticles reports whether a collection or anything below it lists an
// article. Empty categories are common and not worth copying.
func hasArticles(collection publichelp.Collection) bool {
	return len(collection.Articles) > 0 || slices.ContainsFunc(collection.Children, hasArticles)
}

func newSite(origin, locale string) *publichelp.Site {
	return &publichelp.Site{
		Platform: Key, BaseURL: origin + "/hc/" + locale + "/", Locale: locale,
		MatchLink: func(link *url.URL) (publichelp.LinkKind, string, bool) {
			match := _contentPath.FindStringSubmatch(link.Path)
			switch {
			case match == nil:
				return "", "", false
			case match[2] == "articles":
				return publichelp.LinkArticle, match[3], true
			case match[2] == "categories":
				return publichelp.LinkCollection, categoryPrefix + match[3], true
			}
			return publichelp.LinkCollection, match[3], true
		},
	}
}

// pageName takes the help center's name from its home page.
func pageName(body []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	if name, ok := doc.Find(`meta[property="og:site_name"]`).Attr("content"); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}
