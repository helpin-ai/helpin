// Package docsimport converts HTML into canonical Tiptap JSON for the Docs module.
package docsimport

import "encoding/json"

// Node represents a Tiptap/ProseMirror JSON node.
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

// Doc creates a top-level document node.
func Doc(content ...Node) Node {
	return Node{Type: "doc", Content: content}
}

// Paragraph creates a paragraph node.
func Paragraph(content ...Node) Node {
	return Node{Type: "paragraph", Content: content}
}

// Heading creates a heading node with the given level.
func Heading(level int, content ...Node) Node {
	return Node{Type: "heading", Attrs: map[string]any{"level": level}, Content: content}
}

// HeadingWithAttrs creates a heading node with additional attributes.
func HeadingWithAttrs(level int, attrs map[string]any, content ...Node) Node {
	if attrs == nil {
		attrs = map[string]any{}
	}
	attrs["level"] = level
	return Node{Type: "heading", Attrs: attrs, Content: content}
}

// Text creates a text node with optional marks.
func Text(text string, marks ...Mark) Node {
	if text == "" {
		return Node{}
	}
	n := Node{Type: "text", Text: text}
	if len(marks) > 0 {
		n.Marks = marks
	}
	return n
}

// BulletList creates a bullet list node.
func BulletList(items ...Node) Node {
	return Node{Type: "bulletList", Content: items}
}

// OrderedList creates an ordered list node with an optional start number.
func OrderedList(start int, items ...Node) Node {
	attrs := map[string]any{}
	if start > 1 {
		attrs["start"] = start
	}
	return Node{Type: "orderedList", Attrs: attrs, Content: items}
}

// ListItem creates a list item node.
func ListItem(content ...Node) Node {
	return Node{Type: "listItem", Content: content}
}

// Blockquote creates a blockquote node.
func Blockquote(content ...Node) Node {
	return Node{Type: "blockquote", Content: content}
}

// CodeBlock creates a code block node with an optional language.
func CodeBlock(language string, text string) Node {
	attrs := map[string]any{}
	if language != "" {
		attrs["language"] = language
	}
	n := Node{Type: "codeBlock", Attrs: attrs}
	if text != "" {
		n.Content = []Node{Text(text)}
	}
	return n
}

// HorizontalRule creates a horizontal rule node.
func HorizontalRule() Node {
	return Node{Type: "horizontalRule"}
}

// HardBreak creates a hard break node.
func HardBreak() Node {
	return Node{Type: "hardBreak"}
}

// Image creates a resizableImage node. If linkUrl is non-empty, the image
// will be wrapped in a link when rendered.
func Image(src, alt, linkUrl string) Node {
	attrs := map[string]any{
		"src": src,
		"alt": alt,
	}
	if linkUrl != "" {
		attrs["linkUrl"] = linkUrl
		attrs["linkNewTab"] = true
	}
	return Node{
		Type:  "resizableImage",
		Attrs: attrs,
	}
}

// Table creates a table node.
func Table(rows ...Node) Node {
	return Node{Type: "table", Content: rows}
}

// TableRow creates a table row node.
func TableRow(cells ...Node) Node {
	return Node{Type: "tableRow", Content: cells}
}

// TableHeader creates a table header cell node.
func TableHeader(content ...Node) Node {
	return Node{Type: "tableHeader", Content: content}
}

// TableCell creates a table cell node.
func TableCell(content ...Node) Node {
	return Node{Type: "tableCell", Content: content}
}

// Callout creates a callout node with the given variant.
func Callout(variant string, content ...Node) Node {
	return Node{
		Type:    "callout",
		Attrs:   map[string]any{"variant": variant},
		Content: content,
	}
}

// VideoEmbed creates a video embed node.
func VideoEmbed(provider, sourceUrl, embedUrl string) Node {
	return Node{
		Type: "videoEmbed",
		Attrs: map[string]any{
			"provider":  provider,
			"sourceUrl": sourceUrl,
			"embedUrl":  embedUrl,
		},
	}
}

// ToggleSection creates a collapsible details section.
func ToggleSection(title string, open bool, content ...Node) Node {
	attrs := map[string]any{"title": title}
	if open {
		attrs["open"] = true
	}
	return Node{
		Type:    "toggleSection",
		Attrs:   attrs,
		Content: content,
	}
}

// ToggleSectionWithAttrs creates a collapsible details section with source
// metadata used to preserve imported summary styling.
func ToggleSectionWithAttrs(attrs map[string]any, content ...Node) Node {
	if attrs == nil {
		attrs = map[string]any{"title": "Details"}
	}
	if _, ok := attrs["title"]; !ok {
		attrs["title"] = "Details"
	}
	return Node{
		Type:    "toggleSection",
		Attrs:   attrs,
		Content: content,
	}
}

// HTMLBlock creates an htmlBlock node with HTML content.
func HTMLBlock(html string) Node {
	return Node{
		Type:  "htmlBlock",
		Attrs: map[string]any{"html": html},
	}
}

// SandboxedHTMLBlock creates an htmlBlock node that must render in an isolated
// iframe because it contains interactive raw HTML.
func SandboxedHTMLBlock(html string) Node {
	return Node{
		Type: "htmlBlock",
		Attrs: map[string]any{
			"html":       html,
			"renderMode": "sandboxed",
		},
	}
}

// Mark constructors.

// BoldMark creates a bold mark.
func BoldMark() Mark {
	return Mark{Type: "bold"}
}

// ItalicMark creates an italic mark.
func ItalicMark() Mark {
	return Mark{Type: "italic"}
}

// StrikeMark creates a strikethrough mark.
func StrikeMark() Mark {
	return Mark{Type: "strike"}
}

// CodeMark creates an inline code mark.
func CodeMark() Mark {
	return Mark{Type: "code"}
}

// UnderlineMark creates an underline mark.
func UnderlineMark() Mark {
	return Mark{Type: "underline"}
}

// LinkMark creates a link mark.
func LinkMark(href string) Mark {
	return Mark{
		Type: "link",
		Attrs: map[string]any{
			"href":   href,
			"target": "_blank",
			"rel":    "noopener noreferrer",
		},
	}
}

// ToJSON serializes a node to JSON bytes.
func (n Node) ToJSON() (json.RawMessage, error) {
	return json.Marshal(n)
}
