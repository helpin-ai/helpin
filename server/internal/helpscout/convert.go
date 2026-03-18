package helpscout

import (
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

var (
	// calloutRe matches HelpScout callout divs: <div class="callout callout-{type}">...</div>
	calloutRe = regexp.MustCompile(`(?s)<div\s+class="callout\s+callout-(info|warn|danger)">(.*?)</div>`)

	// iframeRe matches iframe elements and captures the src URL.
	iframeRe = regexp.MustCompile(`(?s)<iframe[^>]+src="([^"]+)"[^>]*>.*?</iframe>`)

	// styleAttrRe matches inline style attributes.
	styleAttrRe = regexp.MustCompile(`\s*style="[^"]*"`)
)

// calloutLabels maps HelpScout callout types to markdown labels.
var calloutLabels = map[string]string{
	"info":   "Note:",
	"warn":   "Warning:",
	"danger": "Danger:",
}

// ConvertHTML converts an HTML string (from HelpScout) to Markdown.
// It pre-processes HelpScout-specific elements (callouts, iframes, inline styles)
// before running the html-to-markdown converter.
func ConvertHTML(html string) (string, error) {
	if strings.TrimSpace(html) == "" {
		return "", nil
	}

	html = preprocess(html)

	md, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(md), nil
}

// preprocess transforms HelpScout-specific HTML elements into standard HTML
// that the markdown converter can handle correctly.
func preprocess(html string) string {
	// Replace callout divs with blockquotes.
	html = calloutRe.ReplaceAllStringFunc(html, func(match string) string {
		groups := calloutRe.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}

		calloutType := groups[1]
		inner := groups[2]

		label, ok := calloutLabels[calloutType]
		if !ok {
			label = "Note:"
		}

		// Strip inner <p> tags to get plain text content.
		inner = strings.ReplaceAll(inner, "<p>", "")
		inner = strings.ReplaceAll(inner, "</p>", "")
		inner = strings.TrimSpace(inner)

		return `<blockquote><p><strong>` + label + `</strong> ` + inner + `</p></blockquote>`
	})

	// Replace iframes with plain links.
	html = iframeRe.ReplaceAllString(html, `<p><a href="$1">$1</a></p>`)

	// Strip inline style attributes.
	html = styleAttrRe.ReplaceAllString(html, "")

	return html
}
