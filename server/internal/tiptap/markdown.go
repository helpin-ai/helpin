// Package tiptap provides conversion between TipTap/ProseMirror JSON and other formats.

package tiptap

import (
	"bufio"
	"encoding/json"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	gmhtml "github.com/yuin/goldmark/renderer/html"
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
	normalizeDocumentTree(root)

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
	children := convertInlineChildren(n, source, nil)

	// TipTap treats image as a block-level node. If this paragraph
	// contains only an image (the common markdown pattern
	// `![alt](src)` on its own line), return the image directly
	// instead of wrapping it in a paragraph.
	if len(children) == 1 && children[0].Type == "resizableImage" {
		img := children[0]
		return &img
	}

	p := &Node{Type: "paragraph"}
	p.Content = children
	return p
}

// normalizeDocumentTree enforces TipTap structural constraints that Markdown
// does not: block images cannot remain inside inline text containers, lists
// cannot be empty, and list items must start with a paragraph.
func normalizeDocumentTree(node *Node) {
	for i := range node.Content {
		normalizeDocumentTree(&node.Content[i])
	}

	content := make([]Node, 0, len(node.Content))
	for _, child := range node.Content {
		if (child.Type == "bulletList" || child.Type == "orderedList") && len(child.Content) == 0 {
			continue
		}
		if child.Type != "paragraph" && child.Type != "heading" {
			content = append(content, child)
			continue
		}

		start := 0
		for i, inline := range child.Content {
			if inline.Type != "resizableImage" {
				continue
			}
			if i > start {
				textBlock := child
				textBlock.Content = append([]Node(nil), child.Content[start:i]...)
				content = append(content, textBlock)
			}
			content = append(content, inline)
			start = i + 1
		}
		if start == 0 {
			content = append(content, child)
		} else if start < len(child.Content) {
			textBlock := child
			textBlock.Content = append([]Node(nil), child.Content[start:]...)
			content = append(content, textBlock)
		}
	}

	if node.Type == "listItem" && (len(content) == 0 || content[0].Type != "paragraph") {
		content = append([]Node{{Type: "paragraph"}}, content...)
	}
	node.Content = content
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
	code := codeBlockText(n, source)
	// Skip empty code blocks entirely — some imported sources (e.g., Nextra
	// that used a React component to inject code at runtime) emit fences
	// with no content, which would render as empty boxes in the editor.
	if code == "" {
		return nil
	}
	node := &Node{Type: "codeBlock"}
	lang := string(fcb.Language(source))
	if lang != "" {
		node.Attrs = map[string]any{"language": lang}
	}
	node.Content = []Node{{Type: "text", Text: code}}
	return node
}

func convertCodeBlock(n ast.Node, source []byte) *Node {
	code := codeBlockText(n, source)
	if code == "" {
		return nil
	}
	node := &Node{Type: "codeBlock"}
	node.Content = []Node{{Type: "text", Text: code}}
	return node
}

func convertBlockquote(n ast.Node, source []byte) *Node {
	// Convert children first so we can inspect the first paragraph for a
	// GFM alert marker (e.g., [!NOTE], [!WARNING]).
	var children []Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if child := convertNode(c, source); child != nil {
			children = append(children, *child)
		}
	}

	// Detect GFM alert: first child must be a paragraph whose first text
	// node starts with "[!TYPE]" on its own line. Strip the marker line
	// and wrap the remaining content as a callout node.
	if variant, rest, ok := extractGFMAlert(children); ok {
		return &Node{
			Type:    "callout",
			Attrs:   map[string]any{"variant": variant},
			Content: rest,
		}
	}

	return &Node{Type: "blockquote", Content: children}
}

// extractGFMAlert inspects the first paragraph of a blockquote's children
// for a GitHub-style alert marker like "[!NOTE]" on its own line. Returns
// the mapped callout variant, the remaining children (with the marker
// stripped), and ok=true on a match. Recognizes: NOTE, TIP, IMPORTANT,
// WARNING, CAUTION.
//
// Goldmark's tokenizer splits the marker across several text nodes (e.g.
// `[`, `!WARNING`, `] `) because `[` and `]` are link delimiters, and
// soft line breaks between the marker and the body get converted to
// spaces rather than '\n'. We therefore scan leading plain-text nodes
// character-by-character looking for a closing `]` that completes a
// `[!MARKER]` prefix, then discard the consumed bytes and rebuild the
// paragraph with whatever remains.
func extractGFMAlert(children []Node) (variant string, rest []Node, ok bool) {
	if len(children) == 0 || children[0].Type != "paragraph" || len(children[0].Content) == 0 {
		return "", nil, false
	}
	content := children[0].Content

	// Accumulate leading plain-text nodes into a single string. Track
	// where in this accumulated string each node starts so we can map a
	// byte offset back onto a (node index, offset-within-node) pair.
	type nodeSpan struct {
		idx   int
		start int
		end   int
	}
	var buf strings.Builder
	var spans []nodeSpan
	for i, node := range content {
		if node.Type != "text" || len(node.Marks) > 0 {
			break
		}
		start := buf.Len()
		buf.WriteString(node.Text)
		spans = append(spans, nodeSpan{idx: i, start: start, end: buf.Len()})
	}
	accum := buf.String()
	trimmed := strings.TrimLeft(accum, " \t\n")
	if !strings.HasPrefix(trimmed, "[!") {
		return "", nil, false
	}
	close := strings.IndexByte(trimmed, ']')
	if close < 0 {
		return "", nil, false
	}
	marker := trimmed[:close+1]
	mapped := mapGFMAlertMarker(strings.TrimSpace(marker))
	if mapped == "" {
		return "", nil, false
	}

	// Offset (in bytes) within `accum` of the first byte *after* the
	// marker. Leading whitespace was trimmed; account for it.
	consumedInAccum := (len(accum) - len(trimmed)) + close + 1
	// Skip any separator whitespace (spaces from soft-line-break, or
	// leading newline characters) immediately following the marker.
	for consumedInAccum < len(accum) {
		c := accum[consumedInAccum]
		if c == ' ' || c == '\t' || c == '\n' {
			consumedInAccum++
			continue
		}
		break
	}

	// Map the consumed byte offset back to (node index, offset-in-node).
	consumedNodes := 0
	residualText := ""
	for _, s := range spans {
		if consumedInAccum >= s.end {
			consumedNodes = s.idx + 1
			continue
		}
		// Marker ends inside this node. Keep the remainder of this node
		// as a fresh text node, and drop everything up to consumedNodes.
		consumedNodes = s.idx
		off := consumedInAccum - s.start
		residualText = content[s.idx].Text[off:]
		break
	}
	// If we consumed past every scanned node, there's no residual text
	// from a partial node; consumedNodes already points past them.

	newFirstContent := make([]Node, 0, len(content))
	if residualText != "" {
		newFirstContent = append(newFirstContent, Node{Type: "text", Text: residualText})
		consumedNodes++ // skip the partially-consumed original node
	}
	if consumedNodes < len(content) {
		newFirstContent = append(newFirstContent, content[consumedNodes:]...)
	}

	rest = make([]Node, 0, len(children))
	if len(newFirstContent) > 0 {
		rest = append(rest, Node{Type: "paragraph", Content: newFirstContent})
	}
	rest = append(rest, children[1:]...)
	// Callout must contain at least one block — insert an empty paragraph
	// if the body was only the marker.
	if len(rest) == 0 {
		rest = []Node{{Type: "paragraph"}}
	}
	return mapped, rest, true
}

// mapGFMAlertMarker returns the callout variant for a GFM alert marker
// like "[!NOTE]" or empty string if the line isn't a marker.
func mapGFMAlertMarker(line string) string {
	if !strings.HasPrefix(line, "[!") || !strings.HasSuffix(line, "]") {
		return ""
	}
	switch strings.ToUpper(line) {
	case "[!NOTE]", "[!INFO]", "[!IMPORTANT]":
		return "blue"
	case "[!TIP]":
		return "green"
	case "[!WARNING]":
		return "yellow"
	case "[!CAUTION]", "[!ERROR]", "[!DANGER]":
		return "red"
	default:
		return ""
	}
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
		if !s.IsRaw() && !s.IsCode() {
			txt = markdownTextValue(s.Value)
		}
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

// markdownTextValue follows Goldmark's entity and backslash rules for prose.
// Its writer emits escaped HTML text; remove that output-encoding layer to
// store plain TipTap text. Raw/code nodes must bypass this conversion.
func markdownTextValue(raw []byte) string {
	if !strings.ContainsAny(string(raw), "&\\\x00") {
		return string(raw)
	}
	var escaped strings.Builder
	writer := bufio.NewWriterSize(&escaped, len(raw)+16)
	gmhtml.DefaultWriter.Write(writer, raw)
	_ = writer.Flush() // strings.Builder writes cannot fail.
	return html.UnescapeString(escaped.String())
}

func convertText(n ast.Node, source []byte, marks []Mark) []Node {
	t := n.(*ast.Text)
	raw := t.Segment.Value(source)
	txt := string(raw)
	if !t.IsRaw() {
		txt = markdownTextValue(raw)
	}
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
	// TipTap's code mark excludes every other mark. Markdown can nest code
	// inside emphasis or links, so discard the inherited marks here.
	return convertInlineChildren(n, source, []Mark{{Type: "code"}})
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
		Type: "resizableImage",
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

// appendMark returns a new mark slice with the given mark appended. TipTap
// permits at most one mark of each type on a text node.
func appendMark(marks []Mark, m Mark) []Mark {
	for _, existing := range marks {
		if existing.Type == m.Type {
			return marks
		}
	}
	newMarks := make([]Mark, len(marks)+1)
	copy(newMarks, marks)
	newMarks[len(marks)] = m
	return newMarks
}
