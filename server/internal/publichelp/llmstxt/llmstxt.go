// Package llmstxt reads documentation and help center sites that publish an
// "llms.txt" index of their pages and a Markdown twin of every page at
// "<page>.md". Mintlify, Ferndesk, GitBook, ReadMe, Fern, Featurebase,
// Document360, HelpKit, Plain, Archbee and Kustomer sites do, among others.
package llmstxt

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// Key names this reader.
const Key = "llmstxt"

const (
	// genericPlatform is the platform of a site whose software is not
	// recognized.
	genericPlatform = "docs"
	indexName       = "llms.txt"
	// minEntries is how many pages or further indexes of the site an index
	// must list to count as one.
	minEntries = 2
	// maxProbes bounds the addresses Probe tries for the index.
	maxProbes = 2
	// maxIndexFiles and maxIndexDepth bound the index files one read follows,
	// so a site whose indexes link in circles or without end cannot stall an
	// import.
	maxIndexFiles = 200
	maxIndexDepth = 5
	// parallel is how many index files are fetched at once.
	parallel = 6
	// acceptMarkdown asks for the Markdown form of a page.
	acceptMarkdown = "text/markdown, text/plain;q=0.9, */*;q=0.1"
)

// _platforms recognizes the software behind a site from its home page: a
// response header that starts with header, or a marker in the HTML.
var _platforms = []struct {
	name, header string
	markers      []string
}{
	{name: "mintlify", header: "x-mintlify-", markers: []string{"mintcdn.com", "/_mintlify/"}},
	{name: "ferndesk", header: "x-ferndesk-", markers: []string{"/_ferndesk/", "data-fd-base-path"}},
	{name: "fern", header: "x-fern-", markers: []string{"fern-mdx-link"}},
	{name: "gitbook", header: "x-gitbook-", markers: []string{`content="GitBook`}},
	{name: "readme", markers: []string{"readme-deploy", "cdn.readme.io"}},
	{name: "featurebase", markers: []string{"featurebase-attachments.com", "helpCenterUrlParts"}},
	{name: "document360", markers: []string{"cdn.document360.io"}},
	{name: "helpkit", markers: []string{"helpkit-category", "helpkit.so"}},
	{name: "plain", markers: []string{"files.plain.com", "plain.com/help-center"}},
	{name: "archbee", markers: []string{"api.archbee.com", "archbee-doc-uploads"}},
	{name: "kustomer", markers: []string{"kustomerapp.com"}},
}

var (
	// A Link header entry that names the index: `</docs/llms.txt>; rel="llms-txt"`.
	_linkHeader = regexp.MustCompile(`<([^>]+)>\s*;[^,]*rel="?llms-txt"?`)
	_pageTitle  = regexp.MustCompile(`(?is)<title[^>]*>([^<]+)</title>`)
	// The note at the top of a Fern section index or page that names the index
	// of the whole site: "For the complete documentation index, see <address>".
	_wholeIndex = regexp.MustCompile(`(?i)(?:documentation index|index of the documentation)[^\n]{0,40}?(https?://[^\s)>\]]+/llms\.txt)`)
)

// Reader reads sites through their llms.txt index.
type Reader struct {
	fetch *publichelp.Fetcher
}

var _ publichelp.Reader = (*Reader)(nil)

// New returns a reader that asks as an anonymous visitor.
func New(fetch *publichelp.Fetcher) *Reader { return &Reader{fetch: fetch} }

// Key implements publichelp.Reader.
func (r *Reader) Key() string { return Key }

// Probe implements publichelp.Reader. It looks for the index where the home
// page says it is, then next to the home page, then at the root of the host.
// Most sites take one request; none takes more than four.
func (r *Reader) Probe(ctx context.Context, home *publichelp.Page) (*publichelp.Site, error) {
	// A home page behind a challenge or a sign-in is left alone, index or not.
	if home.Status != http.StatusOK {
		return nil, publichelp.ErrNotHelpCenter
	}
	for _, candidate := range candidates(home) {
		index, err := r.index(ctx, candidate)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			continue
		}
		// Fern serves an index for every section, and a page where a section
		// has none; each names the index of the whole site.
		if whole := wholeIndex(index); whole != "" {
			if better, err := r.index(ctx, whole); err == nil {
				index = better
			}
		}
		if path.Base(index.URL.Path) != indexName {
			continue
		}
		found := parseIndex(index.Body, index.URL, &node{})
		if found.entries < minEntries {
			continue
		}
		return &publichelp.Site{
			Platform: platform(home),
			BaseURL:  strings.TrimSuffix(pageURL(index.URL).String(), "/"+indexName),
			Name:     cmp.Or(found.title, pageName(home.Body), home.URL.Hostname()),
		}, nil
	}
	return nil, publichelp.ErrNotHelpCenter
}

// Load implements publichelp.Reader. It reads the index next to homeURL and
// the further index files it links to; it never reads a page.
func (r *Reader) Load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	progress = publichelp.Serial(progress)
	base, err := url.Parse(strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(homeURL), "/"), "/"+indexName))
	if err != nil || base.Hostname() == "" {
		return nil, fmt.Errorf("%w: %q", publichelp.ErrInvalidURL, homeURL)
	}
	report := func() {
		if progress != nil {
			progress()
		}
	}
	index, err := r.index(ctx, base.String()+"/"+indexName)
	if err != nil {
		return nil, err
	}
	report()
	root := &node{}
	found := parseIndex(index.Body, index.URL, root)
	if err := r.follow(ctx, found.linked, map[string]bool{index.URL.String(): true}, report); err != nil {
		return nil, err
	}
	site := &publichelp.Site{Platform: genericPlatform, BaseURL: base.String(), Name: cmp.Or(found.title, base.Hostname())}
	site.Collections = collections(root, site.Name, strings.Trim(base.Path, "/"))
	if len(site.Collections) == 0 {
		return nil, fmt.Errorf("%w: %s lists no pages", publichelp.ErrNotHelpCenter, index.URL)
	}
	// The platform shows on the home page only, and a job keeps only the address.
	if home, err := r.fetch.Open(ctx, base.String()); err == nil {
		report()
		site.Platform = platform(home)
	}
	return site, nil
}

// Article implements publichelp.Reader. It reads the Markdown twin of the
// page and renders it.
func (r *Reader) Article(ctx context.Context, articleURL string) (*publichelp.Article, error) {
	parsed, err := url.Parse(strings.TrimSpace(articleURL))
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: article address %q", publichelp.ErrInvalidURL, articleURL)
	}
	page := pageURL(parsed)
	source, err := r.markdown(ctx, page)
	if err != nil {
		return nil, err
	}
	title, description, body := clean(string(source.Body))
	rendered, err := publichelp.MarkdownToHTML(body)
	if err != nil {
		return nil, err
	}
	return &publichelp.Article{
		ID:          pageID(page),
		Title:       cmp.Or(title, publichelp.TitleFromSlug(path.Base(page.Path))),
		Description: description,
		URL:         page.String(),
		// Relative links start from where the page was served, which a
		// redirect may have moved.
		HTML: publichelp.AbsoluteHTML(rendered, pageURL(source.URL)),
	}, nil
}

// markdown fetches the Markdown twin of a page, or the page itself asked for
// as Markdown where the site serves no twin.
func (r *Reader) markdown(ctx context.Context, page *url.URL) (*publichelp.Page, error) {
	twin, err := r.fetch.Open(ctx, page.String()+".md", asMarkdown)
	if err != nil {
		return nil, err
	}
	if twin.Status == http.StatusOK && !isHTML(twin) {
		return twin, nil
	}
	if twin.Challenged() {
		return nil, publichelp.ErrChallenged
	}
	negotiated, err := r.fetch.Get(ctx, page.String(), asMarkdown)
	if err != nil {
		return nil, err
	}
	// Some platforms answer any address of a page with its HTML.
	if isHTML(negotiated) {
		return nil, fmt.Errorf("%w: %s has no Markdown version", publichelp.ErrNotHelpCenter, page)
	}
	return negotiated, nil
}

// index fetches an index file. A site without one often answers with an HTML
// page and status 200, which is not an index.
func (r *Reader) index(ctx context.Context, target string) (*publichelp.Page, error) {
	page, err := r.fetch.Get(ctx, target, asMarkdown)
	if err != nil {
		return nil, err
	}
	if isHTML(page) {
		return nil, fmt.Errorf("%w: %s is not a text index", publichelp.ErrNotHelpCenter, target)
	}
	return page, nil
}

// follow reads the index files the nodes link to, level by level, and fills
// each node from its file. A node whose file cannot be used keeps the pages
// its parent index showed for it.
func (r *Reader) follow(ctx context.Context, linked []*node, seen map[string]bool, report func()) error {
	budget := maxIndexFiles
	for depth := 0; len(linked) > 0; depth++ {
		var batch []*node
		for _, pending := range linked {
			if seen[pending.index] || budget == 0 || depth == maxIndexDepth {
				pending.articles = pending.preview
				continue
			}
			seen[pending.index] = true
			budget--
			batch = append(batch, pending)
		}
		pages := make([]*publichelp.Page, len(batch))
		var fetching errgroup.Group
		fetching.SetLimit(parallel)
		for i, pending := range batch {
			fetching.Go(func() error {
				// An index file that cannot be read costs its pages, not the rest.
				if page, err := r.index(ctx, pending.index); err == nil {
					pages[i] = page
				}
				report()
				return ctx.Err()
			})
		}
		if err := fetching.Wait(); err != nil {
			return err
		}
		linked = nil
		for i, pending := range batch {
			if pages[i] == nil {
				pending.articles = pending.preview
				continue
			}
			found := parseIndex(pages[i].Body, pages[i].URL, pending)
			// Fern answers "<page>/llms.txt" with the page itself, whose text
			// may hold a few links; an index is mostly links.
			if found.entries == 0 || found.entries <= found.prose {
				pending.articles, pending.children = pending.preview, nil
				continue
			}
			linked = append(linked, found.linked...)
		}
	}
	return nil
}

func asMarkdown(req *http.Request) { req.Header.Set("Accept", acceptMarkdown) }

// isHTML reports whether a page that should be text is an HTML document.
func isHTML(page *publichelp.Page) bool {
	if strings.Contains(strings.ToLower(page.Header.Get("Content-Type")), "html") {
		return true
	}
	start := bytes.ToLower(page.Body[:min(len(page.Body), 15)])
	return bytes.HasPrefix(start, []byte("<!doctype")) || bytes.HasPrefix(start, []byte("<html"))
}

// wholeIndex returns the address an index gives for the index of the whole
// site, or "" when it names none or itself.
func wholeIndex(index *publichelp.Page) string {
	match := _wholeIndex.FindSubmatch(index.Body)
	if match == nil {
		return ""
	}
	whole := resolve(index.URL, string(match[1]))
	if whole == nil || whole.Path == index.URL.Path {
		return ""
	}
	return whole.String()
}

// candidates lists the addresses where the index of the site at home may be,
// most likely first.
func candidates(home *publichelp.Page) []string {
	var out []string
	add := func(ref string) {
		target, err := home.URL.Parse(ref)
		if err != nil || !sameSite(target, home.URL) || path.Base(target.Path) != indexName {
			return
		}
		if address := pageURL(target).String(); !slices.Contains(out, address) {
			out = append(out, address)
		}
	}
	advertised := home.Header.Get("X-Llms-Txt")
	if match := _linkHeader.FindStringSubmatch(strings.Join(home.Header.Values("Link"), ",")); advertised == "" && match != nil {
		advertised = match[1]
	}
	add(advertised)
	add(strings.TrimRight(home.URL.Path, "/") + "/" + indexName)
	add("/" + indexName)
	return out[:min(len(out), maxProbes)]
}

// platform names the software behind a site from its home page.
func platform(home *publichelp.Page) string {
	for _, known := range _platforms {
		for name := range home.Header {
			if known.header != "" && strings.HasPrefix(strings.ToLower(name), known.header) {
				return known.name
			}
		}
		for _, marker := range known.markers {
			if bytes.Contains(home.Body, []byte(marker)) {
				return known.name
			}
		}
	}
	return genericPlatform
}

// pageName takes a site's name from the title of its home page.
func pageName(body []byte) string {
	match := _pageTitle.FindSubmatch(body)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(string(match[1])))
}

// pageURL is the address people open for a page: without the ".md" ending,
// query, fragment or trailing slash.
func pageURL(address *url.URL) *url.URL {
	page := *address
	// The page at the root of a host keeps its slash: its twin is "/.md".
	page.Path = cmp.Or(strings.TrimSuffix(strings.TrimRight(page.Path, "/"), ".md"), "/")
	page.RawPath, page.RawQuery, page.Fragment, page.RawFragment = "", "", "", ""
	return &page
}

// pageID identifies a page by its path on the site, and the page at the root
// of the host by "/".
func pageID(page *url.URL) string { return cmp.Or(strings.Trim(page.Path, "/"), "/") }

// sameSite reports whether two addresses are on the same host, with or
// without "www.".
func sameSite(a, b *url.URL) bool {
	bare := func(address *url.URL) string {
		return strings.TrimPrefix(strings.ToLower(address.Hostname()), "www.")
	}
	return bare(a) == bare(b)
}
