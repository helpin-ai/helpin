// Package freshdesk reads the knowledge base of a Freshdesk support portal
// from its public pages. Portals are themed, so the pages are read by the
// shape of their addresses rather than by their markup: the knowledge base
// home links the categories, a category page its folders, and a folder page
// its articles and the folders inside it.
package freshdesk

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// Key names this reader and the platform of the sites it reads.
const Key = "freshdesk"

const (
	// concurrency stays below the other readers': a portal counts requests.
	concurrency = 4
	// maxPages bounds one folder, so a folder that never stops paging cannot
	// stall an import.
	maxPages = 400
	// maxDepth bounds folders inside folders; anything deeper is a loop.
	maxDepth = 6
	// lowBudget is the number of requests a portal still allows below which
	// the reader pauses after each one.
	lowBudget = 50
	// categoryPrefix keeps category IDs apart from folder IDs, which come
	// from a different sequence.
	categoryPrefix = "category-"
	otherID        = "other"
	otherName      = "Other articles"
	homePath       = "/support/solutions"
	sitemapPath    = "/support/sitemap.xml"
	loginPath      = "/support/login"
)

const (
	kindCategory = ""
	kindFolder   = "folders"
	kindArticle  = "articles"
)

var (
	// A knowledge base address is "/support/solutions/<category>",
	// ".../folders/<folder>" with an optional "/page/<n>", or
	// ".../articles/<article>-<slug>". Portals in several languages put the
	// language first.
	_contentPath = regexp.MustCompile(
		`^((?:/[A-Za-z]{2}(?:-[A-Za-z]{2,4})?)?)/support/solutions/(?:(articles|folders)/)?(\d+)(?:-([^/]*))?(?:/page/(\d+))?/?$`)
	_portalPath = regexp.MustCompile(`^((?:/[A-Za-z]{2}(?:-[A-Za-z]{2,4})?)?)/support(?:/|$)`)
	// _bodySelectors find the article text: the current theme, the older
	// theme, then what any theme is likely to have.
	_bodySelectors = []string{
		".fw-content--single-article", "#article-body", ".article-body", `[itemprop="articleBody"]`, "article", "main",
	}
)

// Reader reads Freshdesk knowledge bases.
type Reader struct {
	fetch *publichelp.Fetcher
	// pause is how long a read waits when the portal's request budget runs
	// low; tests shorten it.
	pause time.Duration
}

var _ publichelp.Reader = (*Reader)(nil)

// New returns a reader that asks as an anonymous visitor.
func New(fetch *publichelp.Fetcher) *Reader { return &Reader{fetch: fetch, pause: 5 * time.Second} }

// Key implements publichelp.Reader.
func (r *Reader) Key() string { return Key }

// Probe implements publichelp.Reader. A Freshdesk portal is known by its
// session cookie, its freshdesk.com address, or the Freshworks request-budget
// header on a "/support/" page.
func (r *Reader) Probe(_ context.Context, home *publichelp.Page) (*publichelp.Site, error) {
	portal := strings.HasSuffix(strings.ToLower(home.URL.Hostname()), ".freshdesk.com")
	for _, cookie := range home.Header.Values("Set-Cookie") {
		portal = portal || strings.HasPrefix(cookie, "_helpkit_session=")
	}
	// Not every portal sets the cookie.
	if home.Header.Get("X-Fw-Ratelimiting-Managed") != "" {
		portal = portal || _portalPath.MatchString(home.URL.Path) || bytes.Contains(home.Body, []byte(`"/support/solutions`))
	}
	if !portal {
		return nil, publichelp.ErrNotHelpCenter
	}
	if isLogin(home.URL) {
		return nil, &publichelp.PlatformError{Platform: Key, Err: publichelp.ErrRestricted}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(home.Body))
	if home.Status != http.StatusOK || err != nil {
		return nil, publichelp.ErrNotHelpCenter
	}
	site := newSite(home.Origin(), portalPrefix(home.URL))
	site.Name, site.Locale = siteName(doc), strings.TrimSpace(doc.Find("html").AttrOr("lang", ""))
	return site, nil
}

// Article implements publichelp.Reader.
func (r *Reader) Article(ctx context.Context, articleURL string) (*publichelp.Article, error) {
	page, err := r.page(ctx, strings.TrimSpace(articleURL), nil)
	if err != nil {
		return nil, err
	}
	match := _contentPath.FindStringSubmatch(page.url.Path)
	if match == nil || match[2] != kindArticle {
		return nil, fmt.Errorf("%w: %q is not a Freshdesk article address", publichelp.ErrInvalidURL, articleURL)
	}
	title := firstText(page.doc.Find("h1.fw-page-title, .page-title, .main h2.heading").First())
	var body *goquery.Selection
	for i, selector := range _bodySelectors {
		if body = page.doc.Find(selector).First(); body.Length() == 0 {
			continue
		}
		if i >= len(_bodySelectors)-2 {
			// A whole <article> or <main> brings the page's furniture along.
			body.Find("nav, form, header, footer, aside, h1, .breadcrumb").Remove()
		}
		break
	}
	if body.Length() == 0 {
		return nil, fmt.Errorf("%w: article %s has no readable content", publichelp.ErrNotHelpCenter, match[3])
	}
	// Themes may repeat the title inside the text, and list attached files
	// under a count and a file-type badge; the links to the files stay.
	body.Find(".fw-page-title, .attachments > p, .fw-attachment-type").Remove()
	body.Children().First().Filter("hr").Remove()
	content, err := body.Html()
	if err != nil {
		return nil, fmt.Errorf("read article %s: %w", match[3], err)
	}
	description, _ := page.doc.Find(`meta[name="description"]`).Attr("content")
	return &publichelp.Article{
		ID: match[3], Title: cmp.Or(title, titleBeforeSite(page.doc), publichelp.TitleFromSlug(match[4])),
		Description: strings.TrimSpace(description), URL: page.url.String(), HTML: publichelp.AbsoluteHTML(content, page.url),
	}, nil
}

type fetched struct {
	url *url.URL
	doc *goquery.Document
}

// page fetches a portal page. Freshdesk tells how many requests it still
// allows; when few are left the read waits before going on, and a portal
// that has stopped answering for now is an error rather than a sign-in wall.
func (r *Reader) page(ctx context.Context, target string, progress func()) (*fetched, error) {
	page, err := r.fetch.Open(ctx, target)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress()
	}
	limited := page.Status == http.StatusForbidden || page.Status == http.StatusTooManyRequests
	if limited && page.Header.Get("Retry-After") != "" {
		return nil, fmt.Errorf("%w: the help center limits how fast it can be read", publichelp.ErrUnreachable)
	}
	if err := page.Err(); err != nil {
		return nil, err
	}
	if isLogin(page.URL) {
		return nil, publichelp.ErrRestricted
	}
	if requestsLeft(page.Header) < lowBudget {
		timer := time.NewTimer(r.pause)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page.Body))
	if err != nil {
		return nil, fmt.Errorf("%w: %s could not be read", publichelp.ErrNotHelpCenter, target)
	}
	return &fetched{url: page.URL, doc: doc}, nil
}

// requestsLeft is the smallest request budget a response reports.
func requestsLeft(header http.Header) float64 {
	left := math.Inf(1)
	for _, value := range header.Values("X-Ratelimit-Remaining") {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			left = min(left, parsed)
		}
	}
	return left
}

// link is a category, folder or article as a page links to it.
type link struct {
	id, slug, title, url string
}

// links returns the knowledge base links of one kind inside scope, each
// once, in page order.
func links(scope *goquery.Selection, base *url.URL, prefix, kind string) []link {
	var out []link
	seen := map[string]bool{}
	scope.Find("a[href]").Each(func(_ int, anchor *goquery.Selection) {
		target, err := base.Parse(strings.TrimSpace(anchor.AttrOr("href", "")))
		if err != nil || !strings.EqualFold(target.Hostname(), base.Hostname()) {
			return
		}
		match := _contentPath.FindStringSubmatch(target.Path)
		if match == nil || !strings.EqualFold(match[1], prefix) || match[2] != kind || seen[match[3]] {
			return
		}
		seen[match[3]] = true
		// A link to a later page of a folder is a link to the folder.
		path := strings.TrimSuffix(strings.TrimSuffix(target.Path, "/"), "/page/"+match[5])
		out = append(out, link{id: match[3], slug: match[4], title: firstText(anchor), url: target.Scheme + "://" + target.Host + path})
	})
	return out
}

// firstText is the first run of text inside a link or heading. Themes put a
// count, a date or a print button after the name, never before it.
func firstText(selection *goquery.Selection) string {
	var walk func(node *html.Node) string
	walk = func(node *html.Node) string {
		if node.Type == html.TextNode {
			return strings.Join(strings.Fields(node.Data), " ")
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if found := walk(child); found != "" {
				return found
			}
		}
		return ""
	}
	for _, node := range selection.Nodes {
		if found := walk(node); found != "" {
			return found
		}
	}
	return ""
}

// titleBeforeSite is the page's own name: a portal titles every page
// "<name> : <portal>", and themes that change the separator keep the order.
func titleBeforeSite(doc *goquery.Document) string {
	title := strings.Join(strings.Fields(doc.Find("title").First().Text()), " ")
	site, _ := doc.Find(`meta[property="og:site_name"]`).Attr("content")
	if site = strings.TrimSpace(site); site != "" && strings.HasSuffix(title, site) {
		return strings.TrimRight(strings.TrimSuffix(title, site), " :|-")
	}
	if cut := strings.LastIndex(title, " : "); cut >= 0 {
		title = title[:cut]
	}
	return title
}

// siteName takes the portal's name from a page.
func siteName(doc *goquery.Document) string {
	if name, ok := doc.Find(`meta[property="og:site_name"]`).Attr("content"); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	title := strings.Join(strings.Fields(doc.Find("title").First().Text()), " ")
	if cut := strings.LastIndex(title, " : "); cut >= 0 {
		title = title[cut+len(" : "):]
	}
	return title
}

func isLogin(address *url.URL) bool {
	return strings.HasSuffix(strings.TrimSuffix(address.Path, "/"), loginPath)
}

// portalPrefix is the language part of a portal address, such as "/fr", or
// "" for a portal in one language.
func portalPrefix(address *url.URL) string {
	if match := _portalPath.FindStringSubmatch(address.Path); match != nil {
		return match[1]
	}
	return ""
}

func newSite(origin, prefix string) *publichelp.Site {
	return &publichelp.Site{
		Platform: Key, BaseURL: origin + prefix + homePath,
		MatchLink: func(link *url.URL) (publichelp.LinkKind, string, bool) {
			match := _contentPath.FindStringSubmatch(link.Path)
			switch {
			case match == nil:
				return "", "", false
			case match[2] == kindArticle:
				return publichelp.LinkArticle, match[3], true
			case match[2] == kindFolder:
				return publichelp.LinkCollection, match[3], true
			}
			return publichelp.LinkCollection, categoryPrefix + match[3], true
		},
	}
}
