package tiptap

import (
	"html"
	"regexp"
	"strings"
)

var isolatedHTMLBlockPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<!doctype\b`),
	regexp.MustCompile(`(?i)<html[\s>]`),
	regexp.MustCompile(`(?i)<head[\s>]`),
	regexp.MustCompile(`(?i)<body[\s>]`),
	regexp.MustCompile(`(?i)<style[\s>]`),
	regexp.MustCompile(`(?i)<script[\s>]`),
	regexp.MustCompile(`(?i)<svg[\s>]`),
	regexp.MustCompile(`(?i)<iframe[\s>]`),
	regexp.MustCompile(`(?i)<object[\s>]`),
	regexp.MustCompile(`(?i)<embed[\s>]`),
	regexp.MustCompile(`(?i)<form[\s>]`),
	regexp.MustCompile(`(?i)\son[a-z]+\s*=`),
}

func shouldRenderHTMLBlockIsolated(rawHTML, renderMode string) bool {
	if renderMode == "sandboxed" {
		return true
	}
	for _, pattern := range isolatedHTMLBlockPatterns {
		if pattern.MatchString(rawHTML) {
			return true
		}
	}
	return false
}

func renderIsolatedHTMLBlock(rawHTML string) string {
	var b strings.Builder
	b.WriteString(`<div class="docs-html-block docs-html-block--isolated">`)
	b.WriteString("\n")
	b.WriteString(`<iframe class="docs-html-block-frame" title="Rendered HTML block" sandbox="allow-scripts allow-popups allow-forms allow-presentation" referrerpolicy="no-referrer" style="width:100%;height:900px;border:0;background:#fff;border-radius:6px;" srcdoc="`)
	b.WriteString(html.EscapeString(rawHTML))
	b.WriteString(`"></iframe>`)
	b.WriteString("\n</div>\n")
	return b.String()
}
