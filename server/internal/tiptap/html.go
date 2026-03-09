// Package tiptap renders TipTap/ProseMirror JSON documents to HTML.
package tiptap

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// Node represents a TipTap/ProseMirror document node.
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Text    string         `json:"text,omitempty"`
}

// Mark represents a text mark (bold, italic, link, etc).
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// RenderHTML converts a TipTap JSON document to HTML.
// The input should be the raw JSON content stored in DocsContent.Content.
func RenderHTML(jsonContent json.RawMessage) (string, error) {
	if len(jsonContent) == 0 {
		return "", nil
	}

	var doc Node
	if err := json.Unmarshal(jsonContent, &doc); err != nil {
		return "", fmt.Errorf("tiptap: unmarshal: %w", err)
	}

	var b strings.Builder
	b.Grow(len(jsonContent)) // rough estimate
	renderNode(&b, &doc)
	return b.String(), nil
}

func renderNode(b *strings.Builder, n *Node) {
	switch n.Type {
	case "doc":
		renderChildren(b, n)

	case "paragraph":
		b.WriteString("<p>")
		renderChildren(b, n)
		b.WriteString("</p>\n")

	case "heading":
		level := intAttr(n.Attrs, "level", 2)
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		id := headingID(n)
		fmt.Fprintf(b, `<h%d id="%s">`, level, html.EscapeString(id))
		renderChildren(b, n)
		fmt.Fprintf(b, "</h%d>\n", level)

	case "text":
		renderText(b, n)

	case "bulletList":
		b.WriteString("<ul>\n")
		renderChildren(b, n)
		b.WriteString("</ul>\n")

	case "orderedList":
		start := intAttr(n.Attrs, "start", 1)
		if start != 1 {
			fmt.Fprintf(b, `<ol start="%d">`+"\n", start)
		} else {
			b.WriteString("<ol>\n")
		}
		renderChildren(b, n)
		b.WriteString("</ol>\n")

	case "listItem":
		b.WriteString("<li>")
		renderChildren(b, n)
		b.WriteString("</li>\n")

	case "taskList":
		b.WriteString(`<ul class="task-list">` + "\n")
		renderChildren(b, n)
		b.WriteString("</ul>\n")

	case "taskItem":
		checked := boolAttr(n.Attrs, "checked")
		if checked {
			b.WriteString(`<li class="task-item" data-checked="true">`)
			b.WriteString(`<input type="checkbox" checked disabled> `)
		} else {
			b.WriteString(`<li class="task-item">`)
			b.WriteString(`<input type="checkbox" disabled> `)
		}
		renderChildren(b, n)
		b.WriteString("</li>\n")

	case "codeBlock":
		lang := strAttr(n.Attrs, "language")
		if lang != "" {
			fmt.Fprintf(b, "<pre><code class=\"language-%s\">", html.EscapeString(lang))
		} else {
			b.WriteString("<pre><code>")
		}
		renderChildren(b, n)
		b.WriteString("</code></pre>\n")

	case "blockquote":
		b.WriteString("<blockquote>\n")
		renderChildren(b, n)
		b.WriteString("</blockquote>\n")

	case "horizontalRule":
		b.WriteString("<hr>\n")

	case "hardBreak":
		b.WriteString("<br>")

	case "image", "resizableImage":
		renderImage(b, n)

	case "table":
		b.WriteString("<table>\n")
		renderChildren(b, n)
		b.WriteString("</table>\n")

	case "tableRow":
		b.WriteString("<tr>")
		renderChildren(b, n)
		b.WriteString("</tr>\n")

	case "tableHeader":
		b.WriteString("<th>")
		renderChildren(b, n)
		b.WriteString("</th>")

	case "tableCell":
		b.WriteString("<td>")
		renderChildren(b, n)
		b.WriteString("</td>")

	default:
		// Unknown node type — render children if any.
		renderChildren(b, n)
	}
}

func renderChildren(b *strings.Builder, n *Node) {
	for i := range n.Content {
		renderNode(b, &n.Content[i])
	}
}

func renderText(b *strings.Builder, n *Node) {
	escaped := html.EscapeString(n.Text)

	if len(n.Marks) == 0 {
		b.WriteString(escaped)
		return
	}

	// Open marks.
	for _, m := range n.Marks {
		writeMarkOpen(b, &m)
	}

	b.WriteString(escaped)

	// Close marks in reverse order.
	for i := len(n.Marks) - 1; i >= 0; i-- {
		writeMarkClose(b, &n.Marks[i])
	}
}

func writeMarkOpen(b *strings.Builder, m *Mark) {
	switch m.Type {
	case "bold", "strong":
		b.WriteString("<strong>")
	case "italic", "em":
		b.WriteString("<em>")
	case "strike":
		b.WriteString("<s>")
	case "code":
		b.WriteString("<code>")
	case "underline":
		b.WriteString("<u>")
	case "subscript":
		b.WriteString("<sub>")
	case "superscript":
		b.WriteString("<sup>")
	case "highlight":
		b.WriteString("<mark>")
	case "link":
		href := strAttr(m.Attrs, "href")
		target := strAttr(m.Attrs, "target")
		rel := strAttr(m.Attrs, "rel")
		if target == "" {
			target = "_blank"
		}
		if rel == "" {
			rel = "noopener noreferrer"
		}
		fmt.Fprintf(b, `<a href="%s" target="%s" rel="%s">`,
			html.EscapeString(href),
			html.EscapeString(target),
			html.EscapeString(rel))
	}
}

func writeMarkClose(b *strings.Builder, m *Mark) {
	switch m.Type {
	case "bold", "strong":
		b.WriteString("</strong>")
	case "italic", "em":
		b.WriteString("</em>")
	case "strike":
		b.WriteString("</s>")
	case "code":
		b.WriteString("</code>")
	case "underline":
		b.WriteString("</u>")
	case "subscript":
		b.WriteString("</sub>")
	case "superscript":
		b.WriteString("</sup>")
	case "highlight":
		b.WriteString("</mark>")
	case "link":
		b.WriteString("</a>")
	}
}

func renderImage(b *strings.Builder, n *Node) {
	src := strAttr(n.Attrs, "src")
	if src == "" {
		return
	}
	alt := strAttr(n.Attrs, "alt")
	width := strAttr(n.Attrs, "width")
	height := strAttr(n.Attrs, "height")

	b.WriteString(`<img src="`)
	b.WriteString(html.EscapeString(src))
	b.WriteString(`" alt="`)
	b.WriteString(html.EscapeString(alt))
	b.WriteByte('"')

	// Build inline style for dimensions.
	var style []string
	if width != "" && width != "auto" {
		style = append(style, "width:"+html.EscapeString(width))
	}
	if height != "" && height != "auto" {
		style = append(style, "height:"+html.EscapeString(height))
	}
	if len(style) > 0 {
		fmt.Fprintf(b, ` style="%s"`, strings.Join(style, ";"))
	}
	b.WriteString(" loading=\"lazy\">")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// headingID generates a URL-safe ID from heading text content for TOC anchoring.
func headingID(n *Node) string {
	var text strings.Builder
	extractText(&text, n)
	return slugifyHeading(text.String())
}

func extractText(b *strings.Builder, n *Node) {
	if n.Text != "" {
		b.WriteString(n.Text)
	}
	for i := range n.Content {
		extractText(b, &n.Content[i])
	}
}

func slugifyHeading(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' {
			return r
		}
		return -1
	}, s)
	s = strings.Join(strings.Fields(s), "-")
	if s == "" {
		s = "heading"
	}
	return s
}

func intAttr(attrs map[string]any, key string, def int) int {
	if attrs == nil {
		return def
	}
	v, ok := attrs[key]
	if !ok {
		return def
	}
	switch vv := v.(type) {
	case float64:
		return int(vv)
	case int:
		return vv
	default:
		return def
	}
}

func strAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func boolAttr(attrs map[string]any, key string) bool {
	if attrs == nil {
		return false
	}
	v, ok := attrs[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}
