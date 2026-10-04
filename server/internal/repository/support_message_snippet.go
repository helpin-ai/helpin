package repository

import (
	"bytes"
	stdhtml "html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	markdownhtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/net/html"
)

// Raw HTML is only parsed for text here, never served or rendered in a browser.
var snippetMarkdown = goldmark.New(goldmark.WithRendererOptions(markdownhtml.WithUnsafe()))
var snippetUnfinishedTag = regexp.MustCompile(`(?is)<(?:img|a|div|span|table|tr|td|p|br|style|script|html|body|head|meta|link)\b[^>]*$`)
var snippetUnfinishedLink = regexp.MustCompile(`!?\[([^\]\n]*)\]\([^)]*$`)

func snippetPlainText(raw string) string {
	raw = stdhtml.UnescapeString(raw)
	raw = mdAutolinkPattern.ReplaceAllString(raw, "$1")
	raw = snippetUnfinishedTag.ReplaceAllString(raw, "")
	raw = mdTableSepPattern.ReplaceAllString(raw, " ")
	var rendered bytes.Buffer
	if err := snippetMarkdown.Convert([]byte(raw), &rendered); err != nil {
		return ""
	}
	root, err := html.Parse(&rendered)
	if err != nil {
		return ""
	}
	var text strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
			return
		}
		if node.Type == html.ElementNode {
			switch node.Data {
			case "head", "style", "script", "template", "noscript", "svg", "iframe", "object":
				return
			}
			for _, attr := range node.Attr {
				style := strings.ToLower(strings.Join(strings.Fields(attr.Val), ""))
				if attr.Key == "hidden" || (attr.Key == "aria-hidden" && style == "true") ||
					(attr.Key == "style" && (strings.Contains(style, "display:none") || strings.Contains(style, "visibility:hidden"))) {
					return
				}
			}
			text.WriteByte(' ')
			if node.Data == "img" {
				for _, attr := range node.Attr {
					if attr.Key == "alt" && !strings.Contains(strings.ToLower(attr.Val), "://") {
						text.WriteString(attr.Val)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
		if node.Type == html.ElementNode {
			text.WriteByte(' ')
		}
	}
	visit(root)
	// A cached preview may stop before a destination's closing parenthesis.
	// Keep its label, rather than exposing the unfinished URL as literal text.
	cleaned := snippetUnfinishedLink.ReplaceAllString(strings.TrimSpace(text.String()), "$1")
	return strings.ReplaceAll(cleaned, "|", " ")
}
