// Package tiptap provides conversion between TipTap/ProseMirror JSON and other formats.

package tiptap

import (
	"encoding/json"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/yuin/goldmark/extension"
)

// mdParser is a package-level goldmark parser configured with GFM extensions.
var mdParser = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
).Parser()

// MarkdownToJSON converts a markdown string into TipTap/ProseMirror JSON.
// It uses goldmark with GFM extensions to parse the markdown, supporting
// headings, paragraphs, bold, italic, strikethrough, inline code, fenced code
// blocks, bullet lists, ordered lists, blockquotes, horizontal rules, tables,
// links, images, and hard breaks.
func MarkdownToJSON(markdown string) json.RawMessage {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return emptyDoc()
	}

	source := []byte(markdown)
	reader := text.NewReader(source)
	doc := mdParser.Parse(reader)

	root := convertNode(doc, source)
	if root == nil || len(root.Content) == 0 {
		return emptyDoc()
	}

	payload, _ := json.Marshal(root)
	return payload
}

func emptyDoc() json.RawMessage {
	return json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)
}

// convertNode recursively converts a goldmark AST node into a tiptap Node.
func convertNode(n ast.Node, source []byte) *Node {
	switch n.Kind() {
	case ast.KindDocument:
		return convertDocument(n, source)
	case ast.KindParagraph, ast.KindTextBlock:
		return convertParagraph(n, source)
	case ast.KindHeading:
		return convertHeading(n, source)
	case ast.KindList:
		return convertList(n, source)
	case ast.KindListItem:
		return convertListItem(n, source)
	case ast.KindFencedCodeBlock:
		return convertFencedCodeBlock(n, source)
	case ast.KindCodeBlock:
		return convertCodeBlock(n, source)
	case ast.KindBlockquote:
		return convertBlockquote(n, source)
	case ast.KindThematicBreak:
		return &Node{Type: "horizontalRule"}
	case extast.KindTable:
		return convertTable(n, source)
	case extast.KindTableHeader:
		return convertTableSection(n, source, true)
	case extast.KindTableRow:
		return convertTableSection(n, source, false)
	default:
		// For unknown block types, try to convert children.
		return convertBlockChildren(n, source)
	}
}

func convertDocument(n ast.Node, source []byte) *Node {
	doc := &Node{Type: "doc"}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			doc.Content = append(doc.Content, *child)
		}
	}
	return doc
}

func convertParagraph(n ast.Node, source []byte) *Node {
	p := &Node{Type: "paragraph"}
	p.Content = convertInlineChildren(n, source, nil)
	return p
}

func convertHeading(n ast.Node, source []byte) *Node {
	h := n.(*ast.Heading)
	node := &Node{
		Type:  "heading",
		Attrs: map[string]any{"level": h.Level},
	}
	node.Content = convertInlineChildren(n, source, nil)
	return node
}

func convertList(n ast.Node, source []byte) *Node {
	l := n.(*ast.List)
	var nodeType string
	if l.IsOrdered() {
		nodeType = "orderedList"
	} else {
		nodeType = "bulletList"
	}
	node := &Node{Type: nodeType}
	if l.IsOrdered() && l.Start != 1 {
		node.Attrs = map[string]any{"start": l.Start}
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			node.Content = append(node.Content, *child)
		}
	}
	return node
}

func convertListItem(n ast.Node, source []byte) *Node {
	li := &Node{Type: "listItem"}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			li.Content = append(li.Content, *child)
		}
	}
	return li
}

func convertFencedCodeBlock(n ast.Node, source []byte) *Node {
	fcb := n.(*ast.FencedCodeBlock)
	node := &Node{Type: "codeBlock"}
	lang := string(fcb.Language(source))
	if lang != "" {
		node.Attrs = map[string]any{"language": lang}
	}
	code := codeBlockText(n, source)
	if code != "" {
		node.Content = []Node{{Type: "text", Text: code}}
	}
	return node
}

func convertCodeBlock(n ast.Node, source []byte) *Node {
	node := &Node{Type: "codeBlock"}
	code := codeBlockText(n, source)
	if code != "" {
		node.Content = []Node{{Type: "text", Text: code}}
	}
	return node
}

func convertBlockquote(n ast.Node, source []byte) *Node {
	bq := &Node{Type: "blockquote"}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			bq.Content = append(bq.Content, *child)
		}
	}
	return bq
}

func convertTable(n ast.Node, source []byte) *Node {
	tbl := &Node{Type: "table"}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			// TableHeader and TableRow both become tableRow nodes.
			tbl.Content = append(tbl.Content, *child)
		}
	}
	return tbl
}

func convertTableSection(n ast.Node, source []byte, isHeader bool) *Node {
	row := &Node{Type: "tableRow"}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		cellType := "tableCell"
		if isHeader {
			cellType = "tableHeader"
		}
		cell := &Node{Type: cellType}
		// Table cells contain inline content directly; wrap in a paragraph.
		inline := convertInlineChildren(c, source, nil)
		p := Node{Type: "paragraph"}
		p.Content = inline
		cell.Content = []Node{p}
		row.Content = append(row.Content, *cell)
	}
	return row
}

func convertBlockChildren(n ast.Node, source []byte) *Node {
	// Generic wrapper — collect block children.
	var children []Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			children = append(children, *child)
		}
	}
	if len(children) == 0 {
		return nil
	}
	if len(children) == 1 {
		return &children[0]
	}
	// Wrap in a doc-like container; shouldn't normally happen.
	return &Node{Type: "doc", Content: children}
}

// convertInlineChildren converts all inline children of a goldmark node
// into a slice of tiptap Nodes (text nodes with marks).
func convertInlineChildren(n ast.Node, source []byte, marks []Mark) []Node {
	var nodes []Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		nodes = append(nodes, convertInline(c, source, marks)...)
	}
	return nodes
}

// convertInline converts a single inline goldmark node into tiptap nodes.
func convertInline(n ast.Node, source []byte, marks []Mark) []Node {
	switch n.Kind() {
	case ast.KindText:
		return convertText(n, source, marks)
	case ast.KindString:
		s := n.(*ast.String)
		txt := string(s.Value)
		if txt == "" {
			return nil
		}
		return []Node{makeTextNode(txt, marks)}
	case ast.KindEmphasis:
		return convertEmphasis(n, source, marks)
	case ast.KindCodeSpan:
		return convertCodeSpan(n, source, marks)
	case ast.KindLink:
		return convertLink(n, source, marks)
	case ast.KindAutoLink:
		return convertAutoLink(n, source, marks)
	case ast.KindImage:
		return convertImage(n, source)
	case extast.KindStrikethrough:
		newMarks := appendMark(marks, Mark{Type: "strike"})
		return convertInlineChildren(n, source, newMarks)
	default:
		// Unknown inline — recurse into children.
		return convertInlineChildren(n, source, marks)
	}
}

func convertText(n ast.Node, source []byte, marks []Mark) []Node {
	t := n.(*ast.Text)
	raw := t.Segment.Value(source)
	txt := string(raw)
	// Soft line breaks become a space.
	if t.SoftLineBreak() {
		txt = strings.TrimRight(txt, "\n")
		if txt != "" {
			txt += " "
		}
	}
	var nodes []Node
	if txt != "" {
		nodes = append(nodes, makeTextNode(txt, marks))
	}
	if t.HardLineBreak() {
		nodes = append(nodes, Node{Type: "hardBreak"})
	}
	return nodes
}

func convertEmphasis(n ast.Node, source []byte, marks []Mark) []Node {
	e := n.(*ast.Emphasis)
	var markType string
	if e.Level == 2 {
		markType = "bold"
	} else {
		markType = "italic"
	}
	newMarks := appendMark(marks, Mark{Type: markType})
	return convertInlineChildren(n, source, newMarks)
}

func convertCodeSpan(n ast.Node, source []byte, marks []Mark) []Node {
	newMarks := appendMark(marks, Mark{Type: "code"})
	return convertInlineChildren(n, source, newMarks)
}

func convertLink(n ast.Node, source []byte, marks []Mark) []Node {
	l := n.(*ast.Link)
	linkMark := Mark{
		Type: "link",
		Attrs: map[string]any{
			"href":   string(l.Destination),
			"target": "_blank",
			"rel":    "noopener noreferrer",
		},
	}
	newMarks := appendMark(marks, linkMark)
	return convertInlineChildren(n, source, newMarks)
}

func convertAutoLink(n ast.Node, source []byte, marks []Mark) []Node {
	al := n.(*ast.AutoLink)
	url := string(al.URL(source))
	label := string(al.Label(source))
	linkMark := Mark{
		Type: "link",
		Attrs: map[string]any{
			"href":   url,
			"target": "_blank",
			"rel":    "noopener noreferrer",
		},
	}
	newMarks := appendMark(marks, linkMark)
	return []Node{makeTextNode(label, newMarks)}
}

func convertImage(n ast.Node, source []byte) []Node {
	img := n.(*ast.Image)
	// Extract alt text from children.
	var altBuf strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			altBuf.Write(t.Segment.Value(source))
		}
	}
	node := Node{
		Type: "image",
		Attrs: map[string]any{
			"src": string(img.Destination),
			"alt": altBuf.String(),
		},
	}
	return []Node{node}
}

// codeBlockText reads all lines from a code block node.
func codeBlockText(n ast.Node, source []byte) string {
	var buf strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	// Trim trailing newline to match TipTap convention.
	return strings.TrimRight(buf.String(), "\n")
}

// makeTextNode creates a tiptap text node with optional marks.
func makeTextNode(txt string, marks []Mark) Node {
	n := Node{Type: "text", Text: txt}
	if len(marks) > 0 {
		cp := make([]Mark, len(marks))
		copy(cp, marks)
		n.Marks = cp
	}
	return n
}

// appendMark returns a new mark slice with the given mark appended.
func appendMark(marks []Mark, m Mark) []Mark {
	newMarks := make([]Mark, len(marks)+1)
	copy(newMarks, marks)
	newMarks[len(marks)] = m
	return newMarks
}
