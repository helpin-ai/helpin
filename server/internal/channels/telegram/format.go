package telegram

import (
	"bytes"
	stdhtml "html"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

// Markdown is rendered to HTML, then walked into Telegram's HTML subset. Raw
// HTML in the source is shown as typed, never interpreted.
var _markdown = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Linkify))

// markdownToTelegramHTML renders the tags Telegram's HTML parse mode accepts:
// b, i, s, u, code, pre, blockquote and a. Everything else becomes text.
func markdownToTelegramHTML(markdown string) string {
	return renderMarkdown(markdown, true)
}

// markdownToPlain renders text without markup; links keep their URL.
func markdownToPlain(markdown string) string {
	return renderMarkdown(markdown, false)
}

func renderMarkdown(markdown string, rich bool) string {
	source := []byte(markdown)
	doc := _markdown.Parser().Parse(text.NewReader(source))
	literalRawHTML(doc, source)
	var rendered bytes.Buffer
	if err := _markdown.Renderer().Render(&rendered, source, doc); err != nil {
		if rich {
			return stdhtml.EscapeString(markdown)
		}
		return markdown
	}
	root, err := html.Parse(&rendered)
	if err != nil {
		return markdown
	}
	w := &telegramWriter{rich: rich}
	w.walk(root)
	return strings.TrimSpace(collapseBlankLines(w.out.String()))
}

// literalRawHTML replaces raw HTML nodes with text, so the renderer escapes
// them instead of omitting them.
func literalRawHTML(doc ast.Node, source []byte) {
	var raw []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && (n.Kind() == ast.KindRawHTML || n.Kind() == ast.KindHTMLBlock) {
			raw = append(raw, n)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	for _, n := range raw {
		var literal []byte
		switch node := n.(type) {
		case *ast.RawHTML:
			for i := 0; i < node.Segments.Len(); i++ {
				segment := node.Segments.At(i)
				literal = append(literal, segment.Value(source)...)
			}
		case *ast.HTMLBlock:
			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				literal = append(literal, line.Value(source)...)
			}
			if node.HasClosure() {
				literal = append(literal, node.ClosureLine.Value(source)...)
			}
		}
		replacement := ast.NewString(bytes.TrimRight(literal, "\n"))
		if n.Kind() == ast.KindHTMLBlock {
			paragraph := ast.NewParagraph()
			paragraph.AppendChild(paragraph, replacement)
			n.Parent().ReplaceChild(n.Parent(), n, paragraph)
			continue
		}
		n.Parent().ReplaceChild(n.Parent(), n, replacement)
	}
}

type telegramWriter struct {
	out   strings.Builder
	rich  bool
	lists []int // ordered list counters; 0 for bullet lists
}

var _inlineTags = map[string]string{"strong": "b", "b": "b", "em": "i", "i": "i", "del": "s", "s": "s", "u": "u"}

func (w *telegramWriter) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		// goldmark separates block elements with newlines; layout comes from
		// the elements themselves.
		if strings.TrimSpace(n.Data) == "" && strings.Contains(n.Data, "\n") && !insidePre(n) {
			return
		}
		w.text(n.Data)
		return
	case html.ElementNode:
	default:
		w.children(n)
		return
	}
	switch tag := n.Data; tag {
	case "head", "script", "style":
		return
	case "p":
		w.children(n)
		w.out.WriteString("\n\n")
	case "br":
		w.out.WriteString("\n")
	case "h1", "h2", "h3", "h4", "h5", "h6":
		w.wrap("b", n)
		w.out.WriteString("\n\n")
	case "strong", "b", "em", "i", "del", "s", "u":
		w.wrap(_inlineTags[tag], n)
	case "code":
		if n.Parent != nil && n.Parent.Data == "pre" {
			w.children(n)
			return
		}
		w.wrap("code", n)
	case "pre":
		w.wrapTrimmed("pre", n)
		w.out.WriteString("\n\n")
	case "blockquote":
		w.wrapTrimmed("blockquote", n)
		w.out.WriteString("\n\n")
	case "a":
		w.link(n)
	case "img":
		w.image(n)
	case "ul", "ol":
		start := 0
		if tag == "ol" {
			start = 1
		}
		w.lists = append(w.lists, start)
		w.children(n)
		w.lists = w.lists[:len(w.lists)-1]
		if len(w.lists) == 0 {
			w.out.WriteString("\n")
		}
	case "li":
		w.listItem(n)
	case "hr":
		w.out.WriteString("———\n\n")
	default:
		w.children(n)
	}
}

func (w *telegramWriter) children(n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		w.walk(child)
	}
}

func (w *telegramWriter) text(value string) {
	if w.rich {
		value = stdhtml.EscapeString(value)
	}
	w.out.WriteString(value)
}

func (w *telegramWriter) wrap(tag string, n *html.Node) {
	if w.rich {
		w.out.WriteString("<" + tag + ">")
	}
	w.children(n)
	if w.rich {
		w.out.WriteString("</" + tag + ">")
	}
}

// wrapTrimmed wraps a block, dropping the blank lines its children leave at
// either end.
func (w *telegramWriter) wrapTrimmed(tag string, n *html.Node) {
	inner := &telegramWriter{rich: w.rich, lists: w.lists}
	inner.children(n)
	content := strings.TrimSpace(collapseBlankLines(inner.out.String()))
	if w.rich {
		w.out.WriteString("<" + tag + ">" + content + "</" + tag + ">")
		return
	}
	w.out.WriteString(content)
}

func insidePre(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "pre" {
			return true
		}
	}
	return false
}

func (w *telegramWriter) link(n *html.Node) {
	href := attr(n, "href")
	label := strings.TrimSpace(nodeText(n))
	if !safeURL(href) {
		w.children(n)
		return
	}
	if w.rich {
		w.out.WriteString(`<a href="` + stdhtml.EscapeString(href) + `">`)
		w.children(n)
		w.out.WriteString("</a>")
		return
	}
	w.children(n)
	if label != href && strings.TrimPrefix(href, "mailto:") != label {
		w.out.WriteString(" (" + href + ")")
	}
}

func (w *telegramWriter) image(n *html.Node) {
	src, alt := attr(n, "src"), strings.TrimSpace(attr(n, "alt"))
	if alt == "" {
		alt = "image"
	}
	if !safeURL(src) {
		w.text(alt)
		return
	}
	if w.rich {
		w.out.WriteString(`<a href="` + stdhtml.EscapeString(src) + `">` + stdhtml.EscapeString(alt) + "</a>")
		return
	}
	w.out.WriteString(alt + " (" + src + ")")
}

func (w *telegramWriter) listItem(n *html.Node) {
	depth := len(w.lists)
	if depth == 0 {
		w.children(n)
		return
	}
	w.out.WriteString(strings.Repeat("  ", depth-1))
	if counter := w.lists[depth-1]; counter > 0 {
		w.out.WriteString(strconv.Itoa(counter) + ". ")
		w.lists[depth-1]++
	} else {
		w.out.WriteString("• ")
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "p" {
			w.children(child)
			continue
		}
		if child.Type == html.ElementNode && (child.Data == "ul" || child.Data == "ol") {
			w.out.WriteString("\n")
		}
		w.walk(child)
	}
	if !strings.HasSuffix(w.out.String(), "\n") {
		w.out.WriteString("\n")
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return b.String()
}

func safeURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tg://")
}

func collapseBlankLines(value string) string {
	for strings.Contains(value, "\n\n\n") {
		value = strings.ReplaceAll(value, "\n\n\n", "\n\n")
	}
	return value
}

// utf16Len is Telegram's character count.
func utf16Len(value string) int {
	n := 0
	for _, r := range value {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// splitMarkdown splits text at paragraph boundaries so that each part's
// rendered text fits in limit characters. A single paragraph that is still too
// long is cut at the nearest line or word break.
func splitMarkdown(markdown string, limit int) []string {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return nil
	}
	if utf16Len(markdownToPlain(markdown)) <= limit {
		return []string{markdown}
	}
	var parts []string
	var current string
	flush := func() {
		if strings.TrimSpace(current) != "" {
			parts = append(parts, strings.TrimSpace(current))
		}
		current = ""
	}
	for _, paragraph := range strings.Split(markdown, "\n\n") {
		candidate := paragraph
		if current != "" {
			candidate = current + "\n\n" + paragraph
		}
		if utf16Len(markdownToPlain(candidate)) <= limit {
			current = candidate
			continue
		}
		flush()
		if utf16Len(markdownToPlain(paragraph)) <= limit {
			current = paragraph
			continue
		}
		parts = append(parts, hardSplit(paragraph, limit)...)
	}
	flush()
	return parts
}

// hardSplit cuts an oversized paragraph, preferring line then word breaks.
// The chunks are sent as written, so markup spanning a cut may show raw.
func hardSplit(text string, limit int) []string {
	var parts []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := 0
		size := 0
		for end < len(runes) {
			width := 1
			if runes[end] >= 0x10000 {
				width = 2
			}
			if size+width > limit {
				break
			}
			size += width
			end++
		}
		if end < len(runes) {
			if cut := lastBreak(runes[:end]); cut > end/2 {
				end = cut
			}
		}
		parts = append(parts, strings.TrimSpace(string(runes[:end])))
		runes = runes[end:]
	}
	return parts
}

func lastBreak(runes []rune) int {
	for _, sep := range []rune{'\n', ' '} {
		for i := len(runes) - 1; i > 0; i-- {
			if runes[i] == sep {
				return i + 1
			}
		}
	}
	return 0
}

// entity is a Telegram formatting span; offsets count UTF-16 code units.
type entity struct {
	Type     string `json:"type"`
	Offset   int    `json:"offset"`
	Length   int    `json:"length"`
	URL      string `json:"url"`
	Language string `json:"language"`
}

var _entityMarkers = map[string][2]string{
	"bold": {"**", "**"}, "italic": {"_", "_"}, "strikethrough": {"~~", "~~"}, "code": {"`", "`"},
}

// entitiesToMarkdown converts a message and its entities to markdown. Plain
// text is kept as typed, like widget messages.
func entitiesToMarkdown(text string, entities []entity) string {
	if len(entities) == 0 {
		return strings.TrimSpace(text)
	}
	units := utf16.Encode([]rune(text))
	type marker struct {
		pos   int
		open  bool
		width int // span length, to nest longer spans outside shorter ones
		text  string
	}
	var markers []marker
	quoted := make([]bool, len(units)+1) // inside a blockquote: each line needs "> "
	for _, e := range entities {
		if e.Offset < 0 || e.Length <= 0 || e.Offset+e.Length > len(units) {
			continue
		}
		var open, close string
		switch e.Type {
		case "bold", "italic", "strikethrough", "code":
			m := _entityMarkers[e.Type]
			open, close = m[0], m[1]
		case "pre":
			open, close = "\n```"+e.Language+"\n", "\n```\n"
		case "text_link":
			if !safeURL(e.URL) {
				continue
			}
			open, close = "[", "]("+e.URL+")"
		case "blockquote", "expandable_blockquote":
			open, close = "\n> ", "\n\n"
		default:
			continue // url, mention, hashtag and others read fine as text
		}
		if e.Type == "blockquote" || e.Type == "expandable_blockquote" {
			for i := e.Offset; i < e.Offset+e.Length; i++ {
				quoted[i] = true
			}
		}
		markers = append(markers,
			marker{pos: e.Offset, open: true, width: e.Length, text: open},
			marker{pos: e.Offset + e.Length, open: false, width: e.Length, text: close})
	}
	sort.SliceStable(markers, func(i, j int) bool {
		a, b := markers[i], markers[j]
		if a.pos != b.pos {
			return a.pos < b.pos
		}
		if a.open != b.open {
			return !a.open // close spans before opening new ones
		}
		if a.open {
			return a.width > b.width // outer span opens first
		}
		return a.width < b.width // inner span closes first
	})
	var out strings.Builder
	writeSpan := func(from, to int) {
		for i := from; i < to; i++ {
			if units[i] == '\n' && quoted[i] {
				out.WriteString("\n> ")
				continue
			}
			start := i
			for i < to && !(units[i] == '\n' && quoted[i]) {
				i++
			}
			out.WriteString(string(utf16.Decode(units[start:i])))
			i--
		}
	}
	last := 0
	for _, m := range markers {
		writeSpan(last, m.pos)
		out.WriteString(m.text)
		last = m.pos
	}
	writeSpan(last, len(units))
	return strings.TrimSpace(out.String())
}
