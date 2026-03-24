package tiptap

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// htmlBlockPolicy defines the allowed HTML subset for htmlBlock nodes.
// This is the single source of truth for what HTML is safe to render
// in the public Help Center.
var htmlBlockPolicy *bluemonday.Policy

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
	p.AddTargetBlankToFullyQualifiedLinks(false)

	// Images — strict attribute validation
	p.AllowAttrs("src", "alt", "title", "width", "height", "loading").OnElements("img")

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
