package crmtext

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	nethtml "golang.org/x/net/html"
)

var htmlMarkupPattern = regexp.MustCompile(`(?i)<(?:!doctype|/?[a-z][^>]*)>`)

var blockElements = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"div": true, "dl": true, "fieldset": true, "figcaption": true,
	"figure": true, "footer": true, "form": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "header": true,
	"hr": true, "li": true, "main": true, "nav": true, "ol": true,
	"p": true, "pre": true, "section": true, "table": true, "tr": true,
	"ul": true,
}

// PlainText converts HTML into stable text for CRM AI prompting and evidence
// verification. Inline tags do not introduce whitespace, so text such as
// "pri<strong>cing</strong>" remains "pricing".
func PlainText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !htmlMarkupPattern.MatchString(value) {
		return collapseWhitespace(html.UnescapeString(value))
	}

	document, err := nethtml.Parse(strings.NewReader(value))
	if err != nil {
		return collapseWhitespace(html.UnescapeString(value))
	}
	var builder strings.Builder
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && (node.Data == "script" || node.Data == "style" || node.Data == "head") {
			return
		}
		if node.Type == nethtml.TextNode {
			builder.WriteString(node.Data)
			return
		}
		if node.Type == nethtml.ElementNode && (node.Data == "br" || blockElements[node.Data]) {
			builder.WriteByte('\n')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
		if node.Type == nethtml.ElementNode && blockElements[node.Data] {
			builder.WriteByte('\n')
		}
	}
	visit(document)
	return collapseWhitespace(builder.String())
}

// PreferredBody uses plain text when present and otherwise sanitizes HTML.
func PreferredBody(bodyText, bodyHTML string, maxRunes int) string {
	body := collapseWhitespace(bodyText)
	if body == "" {
		body = PlainText(bodyHTML)
	}
	return Truncate(body, maxRunes)
}

// Truncate limits text without splitting a UTF-8 code point.
func Truncate(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}

// Normalize returns comparable evidence text.
func Normalize(value string) string {
	return strings.ToLower(PlainText(value))
}

func collapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
