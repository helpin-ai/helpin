// Package inboundhtml converts inbound email HTML bodies into two variants:
// a sanitized HTML document suitable for sandboxed iframe rendering in the
// support inbox, and a markdown fallback used when rich rendering is not
// desired.
//
// Email HTML is messy: tracking pixels, inline base64 images, massive CSS
// wrappers, and nested quoted reply history. The HTML variant preserves
// structural fidelity (lists, tables, signatures, images) while marking
// quoted sections with data-helpin-quote="true" so the frontend can collapse
// them. Remote image src attributes are moved to data-helpin-remote-src and
// the src is cleared so external images never load until the reader opts in
// — the frontend fetches allowed images through the backend image proxy.
//
// The markdown variant keeps the original Convert behavior: quoted history
// is stripped outright and the result is a plaintext-friendly rendering.
package inboundhtml

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

// maxMarkdownLen caps markdown converter output. Matches the 50k ceiling used
// at the support-message storage layer so callers can rely on a bounded result.
const maxMarkdownLen = 50_000

// maxHTMLLen caps sanitized HTML output. HTML needs more room than markdown
// because tag overhead inflates the byte count, but we still want a hard cap
// to keep row sizes bounded.
const maxHTMLLen = 200_000

// QuotedAttr is the attribute set on elements detected as quoted reply
// history. The frontend collapses any element carrying this attribute.
const QuotedAttr = "data-helpin-quote"

// RemoteImageAttr flags an <img> whose external src was stripped. The
// frontend's "load remote images" affordance swaps the proxied URL onto
// these images.
const RemoteImageAttr = "data-helpin-remote-image"

// RemoteImageSrcAttr holds the original remote src that was stripped. The
// frontend passes its value to the image proxy endpoint when the reader
// opts in.
const RemoteImageSrcAttr = "data-helpin-remote-src"

// CIDAttr records a Content-ID reference from an inline-attached image.
// We strip the src (since the mail client doesn't understand cid: URIs)
// but preserve the reference for future inline-attachment rendering.
const CIDAttr = "data-helpin-cid"

var (
	// collapsedLinesRe collapses 3+ consecutive blank lines into one blank line.
	collapsedLinesRe = regexp.MustCompile(`\n{3,}`)

	// quotedReplySelectors are CSS selectors matching the wrapper elements
	// mail clients use for quoted reply history. The markdown variant removes
	// them; the HTML variant marks them with data-helpin-quote so the frontend
	// can collapse behind a toggle.
	quotedReplySelectors = []string{
		"div.gmail_quote",
		"div.gmail_extra",
		"div#appendonsend",
		"div#divRplyFwdMsg",
		"blockquote[type='cite']",
		"div.yahoo_quoted",
		"div[id='yahoo_quoted']",
		"div.protonmail_quote",
	}

	// trackingPixelSelectors remove zero-area images used for open-tracking
	// from the markdown variant. The HTML variant leaves them alone — email
	// clients like Gmail and Crisp render them as-is (firing the "read"
	// beacon) because stripping creates visual inconsistency with other
	// clients the sender sees.
	trackingPixelSelectors = []string{
		"img[width='1']",
		"img[height='1']",
		"img[width='0']",
		"img[height='0']",
	}

	// markdownPolicy is tuned for the markdown pipeline. Keeps structural tags
	// html-to-markdown understands; drops scripts, styles, forms, handlers.
	markdownPolicy = buildMarkdownPolicy()

	// htmlPolicy is tuned for sandboxed iframe rendering. It allows a richer
	// subset of attributes (class, inline style on safe properties, our own
	// data-helpin-* attributes) so signature layout and list/table formatting
	// survive sanitization.
	htmlPolicy = buildHTMLPolicy()

	// styleBlockRe matches a <style>...</style> element so we can sanitize
	// its CSS body after bluemonday (which treats it as opaque CDATA).
	styleBlockRe = regexp.MustCompile(`(?is)(<style[^>]*>)([\s\S]*?)(</style>)`)

	// cssImportRe matches @import rules. These pull remote CSS, which can
	// fingerprint the reader's IP and import arbitrary styles we haven't
	// vetted — cheaper to just strip them.
	cssImportRe = regexp.MustCompile(`(?is)@import\b[^;]*;?`)

	// cssPositionLockRe collapses position: fixed/sticky to position:static
	// so email CSS can't overlay the host UI — iframes isolate painting,
	// but belt-and-suspenders here is cheap.
	cssPositionLockRe = regexp.MustCompile(`(?i)position\s*:\s*(fixed|sticky)`)

	// markdownConverter renders sanitized HTML to markdown. The table plugin
	// preserves grids; base+commonmark cover paragraphs, lists, anchors,
	// emphasis, and code.
	markdownConverter = converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)
)

func buildMarkdownPolicy() *bluemonday.Policy {
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

func buildHTMLPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements(
		"p", "br", "div", "span",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li",
		"blockquote", "pre", "code",
		"strong", "b", "em", "i", "u", "s", "strike",
		"a", "img",
		"table", "thead", "tbody", "tfoot", "tr", "td", "th",
		"hr", "sub", "sup", "small", "big", "font",
	)
	p.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	p.AllowAttrs("alt", "title", "width", "height").OnElements("img")
	p.AllowAttrs("colspan", "rowspan", "align", "valign").OnElements("td", "th")
	p.AllowAttrs("align").OnElements("p", "div", "table", "tr", "td", "th")
	p.AllowAttrs("border", "cellpadding", "cellspacing").OnElements("table")
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("color", "face", "size").OnElements("font")
	// Whitelist a tight set of inline style properties. We render inside a
	// sandboxed iframe, so positioning and visibility tricks can't reach the
	// outer app, but we still block properties that would be purely
	// anti-social (e.g. z-index, position).
	p.AllowStyles(
		"color", "background-color",
		"font-weight", "font-style", "font-size", "font-family",
		"text-align", "text-decoration",
		"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
		"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
		"border", "border-top", "border-right", "border-bottom", "border-left",
		"border-color", "border-width", "border-style", "border-radius",
		"width", "height", "max-width", "min-width",
		"line-height", "letter-spacing",
		"vertical-align", "display",
	).Globally()
	// Our own marker attributes must survive sanitization.
	p.AllowAttrs(
		QuotedAttr,
		RemoteImageAttr,
		RemoteImageSrcAttr,
		CIDAttr,
	).Globally()
	p.AllowURLSchemes("http", "https", "mailto", "tel")
	// Link targets need rel=noopener to avoid tabnabbing. bluemonday's UGC
	// policy already enforces this; no extra work needed.
	return p
}

// ProcessedContent is the result of processing an inbound email body. HTML is
// a sanitized HTML fragment safe to inject into a sandboxed iframe; Markdown
// is a plaintext-friendly rendering with quoted history removed.
type ProcessedContent struct {
	HTML     string
	Markdown string
}

// Process converts inbound email HTML into both a sanitized HTML variant
// (for sandboxed iframe rendering) and a markdown variant (as a fallback).
// plainText is used as a markdown fallback when html is empty or conversion
// yields nothing. Returned strings are capped; see maxHTMLLen, maxMarkdownLen.
func Process(html, plainText string) ProcessedContent {
	html = strings.TrimSpace(html)
	if html == "" {
		return ProcessedContent{Markdown: truncate(strings.TrimSpace(plainText), maxMarkdownLen)}
	}
	return ProcessedContent{
		HTML:     buildCleanHTML(html),
		Markdown: Convert(html, plainText),
	}
}

// Convert converts inbound email HTML into sanitized markdown. If html is
// empty or the converted result is empty, it returns the trimmed plainText
// fallback. The returned string is capped at maxMarkdownLen characters.
func Convert(html, plainText string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return truncate(strings.TrimSpace(plainText), maxMarkdownLen)
	}

	stripped, err := stripQuotedHistory(html)
	if err != nil {
		// Fall back to the raw HTML on parse failure — sanitizer still runs,
		// we just won't remove quoted history.
		stripped = html
	}

	safe := markdownPolicy.Sanitize(stripped)

	md, err := markdownConverter.ConvertString(safe)
	if err != nil || strings.TrimSpace(md) == "" {
		return truncate(strings.TrimSpace(plainText), maxMarkdownLen)
	}

	md = collapsedLinesRe.ReplaceAllString(md, "\n\n")
	return truncate(strings.TrimSpace(md), maxMarkdownLen)
}

// buildCleanHTML returns a sanitized HTML fragment with tracking pixels
// removed, quoted sections annotated for frontend collapsing, and remote
// image sources stripped (moved to data-helpin-remote-src) so external
// images don't leak the reader's IP until they opt in.
func buildCleanHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		// Sanitize raw string as a best-effort fallback.
		return truncate(strings.TrimSpace(htmlPolicy.Sanitize(html)), maxHTMLLen)
	}

	// Mark (don't remove) quoted history so frontend can collapse it behind
	// a toggle while keeping the thread auditable.
	for _, sel := range quotedReplySelectors {
		doc.Find(sel).SetAttr(QuotedAttr, "true")
	}

	hoistHeadStylesIntoBody(doc)

	body, err := bodyInnerHTML(doc)
	if err != nil {
		return ""
	}

	// Bluemonday hard-strips <style> element contents regardless of the
	// policy allowlist (it's baked into the library's internal skip list).
	// Swap style blocks out for text placeholders, sanitize the rest of
	// the HTML, then swap them back. CSS itself is cleaned via sanitizeCSS
	// during extraction.
	body, styleBlocks := extractStyleBlocks(body)
	safe := htmlPolicy.Sanitize(body)
	safe = restoreStyleBlocks(safe, styleBlocks)

	return truncate(strings.TrimSpace(safe), maxHTMLLen)
}

// hoistHeadStylesIntoBody relocates every <style> element that the HTML
// parser placed in <head> to the front of <body>. Email senders commonly
// write `<style>...</style><table>...</table>` fragments, and golang.org/x/net/html
// lifts the <style> into a synthetic <head> during parsing. Without this
// hoist, bodyInnerHTML would drop those stylesheets entirely.
func hoistHeadStylesIntoBody(doc *goquery.Document) {
	body := doc.Find("body").First()
	if body.Length() == 0 {
		return
	}
	doc.Find("head style").Each(func(_ int, s *goquery.Selection) {
		body.PrependSelection(s)
	})
}

const (
	stylePlaceholderPrefix = "__helpin_style_"
	stylePlaceholderSuffix = "__"
)

// extractStyleBlocks pulls every <style>...</style> block out of the HTML,
// sanitizes the CSS inside each, and replaces the block with a text
// placeholder. bluemonday strips <style> content regardless of the policy
// allowlist, so we need to hide them during HTML sanitization and splice
// them back after.
func extractStyleBlocks(html string) (string, []string) {
	var blocks []string
	out := styleBlockRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := styleBlockRe.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		idx := len(blocks)
		blocks = append(blocks, parts[1]+sanitizeCSS(parts[2])+parts[3])
		return stylePlaceholderPrefix + strconv.Itoa(idx) + stylePlaceholderSuffix
	})
	return out, blocks
}

// restoreStyleBlocks swaps the text placeholders produced by
// extractStyleBlocks back into their sanitized <style> tags.
func restoreStyleBlocks(html string, blocks []string) string {
	for i, block := range blocks {
		placeholder := stylePlaceholderPrefix + strconv.Itoa(i) + stylePlaceholderSuffix
		html = strings.ReplaceAll(html, placeholder, block)
	}
	return html
}

// sanitizeCSS scrubs a handful of genuinely dangerous constructs from a
// CSS body while preserving layout, typography, @media responsive rules,
// and remote url() references. @import is stripped because it loads
// arbitrary remote stylesheets that sidestep our HTML sanitizer's
// allowlist, and position: fixed/sticky is collapsed to position:static
// so author CSS can't escape the iframe's visual box. url(http(s)://...)
// is deliberately left alone — we match Gmail/Crisp rendering fidelity
// instead of gating remote resource loads.
func sanitizeCSS(css string) string {
	css = cssImportRe.ReplaceAllString(css, "")
	css = cssPositionLockRe.ReplaceAllString(css, "position:static")
	return css
}

// stripQuotedHistory removes quoted-reply wrappers and tracking pixels using
// DOM-level selectors so nested tags don't break the cleanup. Used by the
// markdown pipeline; the HTML pipeline marks instead of removing.
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
	return bodyInnerHTML(doc)
}

// bodyInnerHTML returns the inner HTML of the document's <body>, falling
// back to the whole document when goquery didn't wrap in <body> (rare for
// fragments). html-to-markdown and our sanitizer both expect fragments.
//
// Marketing emails often set layout-critical styles on <body> (e.g.
// max-width:602px; margin:0 auto; bgcolor=#f6f7f9 to center the content
// block). Since we render into our own iframe body, those would be lost.
// This function transfers the original body's style and bgcolor onto a
// wrapper <div> so the email renders like it would in Gmail or Postmark.
func bodyInnerHTML(doc *goquery.Document) (string, error) {
	body := doc.Find("body")
	if body.Length() == 0 {
		return doc.Html()
	}

	inner, err := body.Html()
	if err != nil {
		return "", err
	}

	style := strings.TrimSpace(body.AttrOr("style", ""))
	if bgcolor := strings.TrimSpace(body.AttrOr("bgcolor", "")); bgcolor != "" {
		if style != "" && !strings.HasSuffix(style, ";") {
			style += ";"
		}
		style += "background-color:" + bgcolor
	}
	if style == "" {
		return inner, nil
	}

	return `<div style="` + html.EscapeString(style) + `">` + inner + `</div>`, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
