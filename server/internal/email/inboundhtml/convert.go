// Package inboundhtml converts inbound email HTML bodies into safe markdown
// suitable for display in the support thread UI.
//
// Email HTML is messy: tracking pixels, inline base64 images, massive CSS
// wrappers, and nested quoted reply history. Rendering it raw in a chat
// bubble produces the wall-of-URLs problem. Pre-converting to markdown at
// ingestion keeps the frontend renderer unchanged while collapsing noisy
// anchor hrefs behind their display text.
package inboundhtml

import (
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

// maxMarkdownLen caps converter output. Matches the 50k ceiling used at the
// support-message storage layer so callers can rely on a bounded result.
const maxMarkdownLen = 50_000

var (
	// collapsedLinesRe collapses 3+ consecutive blank lines into one blank line.
	collapsedLinesRe = regexp.MustCompile(`\n{3,}`)

	// quotedReplySelectors are CSS selectors matching the wrapper elements
	// mail clients use for quoted reply history. These are removed entirely
	// before sanitization so older thread content never reaches the reader.
	quotedReplySelectors = []string{
		"div.gmail_quote",                 // Gmail web
		"div.gmail_extra",                 // Gmail attribution + quote
		"div#appendonsend",                // Outlook.com web
		"div#divRplyFwdMsg",               // Outlook desktop
		"blockquote[type='cite']",         // Apple Mail, iOS Mail, most SMTP
		"div.yahoo_quoted",                // Yahoo Mail
		"div[id='yahoo_quoted']",          // Yahoo Mail variants
		"div.protonmail_quote",            // Proton Mail
	}

	// trackingPixelSelectors remove zero-area images used for open-tracking
	// before the content is rendered. Matches both 1x1 and explicitly hidden.
	trackingPixelSelectors = []string{
		"img[width='1']",
		"img[height='1']",
		"img[width='0']",
		"img[height='0']",
	}

	// policy is a bluemonday policy tuned for inbound email content. It keeps
	// the structural tags html-to-markdown understands and drops scripts,
	// styles, forms, and inline event handlers.
	policy = buildPolicy()

	// markdownConverter renders sanitized HTML to markdown. The table plugin
	// preserves grids (rare in reply emails but common in receipts/digests),
	// and base+commonmark cover paragraphs, lists, anchors, emphasis, code.
	markdownConverter = converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)
)

func buildPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements(
		"p", "br", "div", "span",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li",
		"blockquote", "pre", "code",
		"strong", "b", "em", "i", "u", "s", "strike",
		"a", "img",
		"table", "thead", "tbody", "tr", "td", "th",
		"hr",
	)
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	p.AllowURLSchemes("http", "https", "mailto", "tel")
	return p
}

// Convert converts inbound email HTML into sanitized markdown. If html is
// empty or the converted result is empty, it returns the trimmed plainText
// fallback. The returned string is capped at 50k characters.
func Convert(html, plainText string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return truncate(strings.TrimSpace(plainText))
	}

	stripped, err := stripQuotedHistory(html)
	if err != nil {
		// Fall back to the raw HTML on parse failure — sanitizer will still
		// run, we just won't remove quoted history.
		stripped = html
	}

	safe := policy.Sanitize(stripped)

	md, err := markdownConverter.ConvertString(safe)
	if err != nil || strings.TrimSpace(md) == "" {
		return truncate(strings.TrimSpace(plainText))
	}

	md = collapsedLinesRe.ReplaceAllString(md, "\n\n")
	return truncate(strings.TrimSpace(md))
}

// stripQuotedHistory removes quoted-reply wrappers and tracking pixels using
// DOM-level selectors so nested tags don't break the cleanup.
func stripQuotedHistory(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	for _, sel := range quotedReplySelectors {
		doc.Find(sel).Remove()
	}
	for _, sel := range trackingPixelSelectors {
		doc.Find(sel).Remove()
	}
	// goquery wraps fragments in <html><head></head><body>…</body></html>.
	// Extract just the body contents so html-to-markdown sees a clean fragment.
	body := doc.Find("body")
	if body.Length() == 0 {
		out, innerErr := doc.Html()
		if innerErr != nil {
			return "", innerErr
		}
		return out, nil
	}
	out, err := body.Html()
	if err != nil {
		return "", err
	}
	return out, nil
}

func truncate(s string) string {
	if len(s) <= maxMarkdownLen {
		return s
	}
	return s[:maxMarkdownLen]
}
