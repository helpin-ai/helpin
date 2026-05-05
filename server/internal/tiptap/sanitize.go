package tiptap

import (
	"encoding/base64"
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
)

// htmlBlockPolicy defines the allowed HTML subset for htmlBlock nodes.
// This is the single source of truth for what HTML is safe to render
// in the public Help Center.
var htmlBlockPolicy *bluemonday.Policy
var safeDataImagePrefix = regexp.MustCompile(`^image/(gif|jpe?g|png|webp);base64,`)

func init() {
	p := bluemonday.NewPolicy()

	// Structural/content tags
	p.AllowElements(
		"div", "section", "article", "aside", "header", "footer", "nav", "main",
		"figure", "figcaption",
		"dl", "dt", "dd",
		"details", "summary",
		"blockquote",
		"br", "hr", "wbr",
	)

	// Text formatting
	p.AllowElements(
		"p", "span",
		"strong", "b", "em", "i", "u", "s", "del", "ins",
		"small", "mark", "abbr", "cite", "q",
		"sub", "sup",
		"code", "pre", "kbd", "samp", "var",
	)

	// Lists
	p.AllowElements("ul", "ol", "li")

	// Headings
	p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")

	// Tables
	p.AllowElements("table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col")
	p.AllowAttrs("colspan", "rowspan").OnElements("th", "td")

	// Links — strict attribute validation
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`^(noopener noreferrer|noopener|noreferrer|nofollow)$`)).OnElements("a")
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("https", "http", "mailto", "tel")
	p.AllowURLSchemeWithCustomPolicy("data", func(u *url.URL) bool {
		if u.RawQuery != "" || u.Fragment != "" {
			return false
		}
		matched := safeDataImagePrefix.FindString(u.Opaque)
		if matched == "" {
			return false
		}
		_, err := base64.StdEncoding.DecodeString(u.Opaque[len(matched):])
		return err == nil
	})
	p.AddTargetBlankToFullyQualifiedLinks(false)

	// Images — strict attribute validation
	p.AllowAttrs("src", "alt", "title", "width", "height", "loading").OnElements("img")

	// Presentation-only inline styles. This preserves Help Scout custom HTML
	// blocks without allowing URL-bearing or layout-breaking CSS.
	p.AllowAttrs("style").Globally()
	p.AllowStyles(
		"background", "background-color", "border", "border-color", "border-radius", "border-style", "border-width",
		"align-content", "align-items", "align-self", "box-sizing", "color", "column-gap", "display", "flex-basis",
		"flex-direction", "flex-grow", "flex-shrink", "flex-wrap",
		"font-size", "font-style", "font-weight", "gap", "grid-template-columns", "grid-template-rows", "height",
		"justify-content", "justify-items", "justify-self", "letter-spacing", "line-height",
		"list-style", "list-style-position", "list-style-type",
		"margin", "margin-bottom", "margin-left", "margin-right", "margin-top",
		"max-height", "max-width", "min-height", "min-width", "object-fit", "overflow", "overflow-x", "overflow-y",
		"padding", "padding-bottom", "padding-left", "padding-right", "padding-top",
		"row-gap", "text-align", "text-decoration", "text-transform", "vertical-align", "white-space", "width",
	).MatchingHandler(isSafeInlineStyleValue).Globally()

	// Safe global attrs on all elements
	p.AllowAttrs("class", "id").Globally()
	p.AllowDataAttributes()

	// Do NOT allow: script, style, iframe, object, embed, form, input, button,
	// link, meta, base, event handlers (on*), javascript: URLs

	htmlBlockPolicy = p
}

// SanitizeHTMLBlock sanitizes an HTML fragment for safe rendering.
func SanitizeHTMLBlock(rawHTML string) string {
	return htmlBlockPolicy.Sanitize(rawHTML)
}

func isSafeInlineStyleValue(value string) bool {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" || strings.Contains(v, "url(") || strings.Contains(v, "expression") || strings.Contains(v, "@import") {
		return false
	}
	for _, r := range v {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case ' ', '#', '.', ',', '%', '(', ')', '-', '_', '/':
			continue
		default:
			return false
		}
	}
	return true
}

// StripHTML removes all HTML tags and returns plain text.
// Useful for generating notification bodies from rich-text content.
// Unescapes HTML entities (e.g. &#39; → ') after sanitizing so the
// output reads as natural text.
func StripHTML(raw string) string {
	sanitized := bluemonday.StrictPolicy().Sanitize(raw)
	return strings.TrimSpace(html.UnescapeString(sanitized))
}
