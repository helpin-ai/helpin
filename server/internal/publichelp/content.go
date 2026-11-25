package publichelp

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

const (
	// sitemapMaxFiles and sitemapMaxURLs bound one sitemap read, so a site
	// with endless or huge sitemaps cannot stall an import.
	sitemapMaxFiles = 60
	sitemapMaxURLs  = 50000
)

// _urlAttributes are the attributes of an article body that carry an address.
var _urlAttributes = map[string][]string{
	"a": {"href"}, "img": {"src"}, "source": {"src"}, "video": {"src", "poster"}, "iframe": {"src"},
}

// AbsoluteHTML returns an HTML fragment with every link, image and embed
// address made absolute against base, lazily loaded images given their real
// address, and scripts and styles removed.
func AbsoluteHTML(fragment string, base *url.URL) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return fragment
	}
	doc.Find("script, style, noscript").Remove()
	// An image that loads on scroll carries its real address beside a placeholder.
	doc.Find("img").Each(func(_ int, image *goquery.Selection) {
		if src := image.AttrOr("src", ""); src != "" && !strings.HasPrefix(src, "data:") {
			return
		}
		for _, attribute := range []string{"data-src", "data-lazy-src", "data-original"} {
			if lazy := image.AttrOr(attribute, ""); lazy != "" {
				image.SetAttr("src", lazy)
				return
			}
		}
	})
	for tag, attributes := range _urlAttributes {
		doc.Find(tag).Each(func(_ int, node *goquery.Selection) {
			for _, attribute := range attributes {
				value, ok := node.Attr(attribute)
				if !ok || strings.HasPrefix(value, "#") || strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "mailto:") {
					continue
				}
				if resolved, err := base.Parse(strings.TrimSpace(value)); err == nil {
					node.SetAttr(attribute, resolved.String())
				}
			}
		})
	}
	html, err := doc.Find("body").Html()
	if err != nil {
		return fragment
	}
	return html
}

var _markdown = goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(gmhtml.WithUnsafe()))

// MarkdownToHTML renders Markdown, with tables and other GitHub additions, as
// HTML. HTML written inside the Markdown is kept.
func MarkdownToHTML(markdown string) (string, error) {
	var out bytes.Buffer
	if err := _markdown.Convert([]byte(markdown), &out); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return out.String(), nil
}

// SitemapEntry is one address a sitemap lists.
type SitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// Sitemap reads a sitemap and the sitemaps it points to, and returns every
// page address in the order listed.
func (f *Fetcher) Sitemap(ctx context.Context, sitemapURL string, progress func()) ([]SitemapEntry, error) {
	var entries []SitemapEntry
	queue := []string{sitemapURL}
	seen := map[string]bool{}
	for files := 0; len(queue) > 0 && files < sitemapMaxFiles && len(entries) < sitemapMaxURLs; files++ {
		next := queue[0]
		queue = queue[1:]
		if seen[next] {
			continue
		}
		seen[next] = true
		page, err := f.Get(ctx, next)
		if err != nil {
			// A broken part of a sitemap index costs its pages, not the rest.
			if files == 0 {
				return nil, err
			}
			continue
		}
		if progress != nil {
			progress()
		}
		var parsed struct {
			Sitemaps []SitemapEntry `xml:"sitemap"`
			URLs     []SitemapEntry `xml:"url"`
		}
		if err := xml.Unmarshal(page.Body, &parsed); err != nil {
			if files == 0 {
				return nil, fmt.Errorf("%w: the sitemap could not be read", ErrNotHelpCenter)
			}
			continue
		}
		for _, sitemap := range parsed.Sitemaps {
			queue = append(queue, strings.TrimSpace(sitemap.Loc))
		}
		for _, entry := range parsed.URLs {
			entry.Loc = strings.TrimSpace(entry.Loc)
			if entry.Loc != "" {
				entries = append(entries, entry)
			}
		}
	}
	return entries, nil
}

// TitleFromSlug turns "reset-your-password" into "Reset your password", for
// pages a listing names only by address.
func TitleFromSlug(slug string) string {
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(slug))
	if len(words) == 0 {
		return "Untitled"
	}
	title := strings.Join(words, " ")
	return strings.ToUpper(title[:1]) + title[1:]
}
