package docsimport

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// ConvertHTML parses HTML and returns canonical Tiptap JSON.
func ConvertHTML(rawHTML string) (*ConversionResult, error) {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil, err
	}

	c := &converter{warnings: nil}
	nodes := c.convertChildren(findBody(doc))

	// Ensure at least one paragraph
	if len(nodes) == 0 {
		nodes = []Node{Paragraph()}
	}

	return &ConversionResult{
		Doc:      Doc(nodes...),
		Warnings: c.warnings,
	}, nil
}

type converter struct {
	warnings []Warning
}

func (c *converter) warn(w Warning) {
	c.warnings = append(c.warnings, w)
}

// convertChildren walks child nodes and returns block-level Tiptap nodes.
func (c *converter) convertChildren(parent *html.Node) []Node {
	var result []Node
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		nodes := c.convertNode(child)
		result = append(result, nodes...)
	}
	return result
}

// convertNode converts a single HTML node into Tiptap nodes.
func (c *converter) convertNode(n *html.Node) []Node {
	switch n.Type {
	case html.TextNode:
		text := n.Data
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []Node{Text(text)}

	case html.ElementNode:
		return c.convertElement(n)

	default:
		return nil
	}
}

// convertElement handles element nodes.
func (c *converter) convertElement(n *html.Node) []Node {
	// Check for Help Scout callout first
	if variant, ok := isHelpScoutCallout(n); ok {
		content := c.convertChildren(n)
		content = ensureBlockContent(content)
		c.warn(warnCalloutGuess(getAttr(n, "class"), variant))
		return []Node{Callout(variant, content...)}
	}

	switch n.DataAtom {
	// Block elements
	case atom.P:
		return []Node{Paragraph(c.convertInline(n)...)}

	case atom.H1:
		return []Node{Heading(1, c.convertInline(n)...)}
	case atom.H2:
		return []Node{Heading(2, c.convertInline(n)...)}
	case atom.H3:
		return []Node{Heading(3, c.convertInline(n)...)}
	case atom.H4:
		return []Node{Heading(4, c.convertInline(n)...)}
	case atom.H5:
		return []Node{Heading(5, c.convertInline(n)...)}
	case atom.H6:
		return []Node{Heading(6, c.convertInline(n)...)}

	case atom.Blockquote:
		content := c.convertChildren(n)
		content = ensureBlockContent(content)
		return []Node{Blockquote(content...)}

	case atom.Pre:
		lang := ""
		text := extractText(n)
		// Check for <pre><code class="language-x">
		if code := findFirstChild(n, atom.Code); code != nil {
			text = extractText(code)
			classes := getAttr(code, "class")
			if strings.HasPrefix(classes, "language-") {
				lang = strings.TrimPrefix(classes, "language-")
			}
		}
		return []Node{CodeBlock(lang, text)}

	case atom.Hr:
		return []Node{HorizontalRule()}

	case atom.Br:
		return []Node{HardBreak()}

	case atom.Ul:
		return []Node{BulletList(c.convertListItems(n)...)}
	case atom.Ol:
		start := 1
		if s := getAttr(n, "start"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 {
				start = v
			}
		}
		return []Node{OrderedList(start, c.convertListItems(n)...)}
	case atom.Li:
		content := c.convertChildren(n)
		content = ensureBlockContent(content)
		return []Node{ListItem(content...)}

	case atom.Table:
		return []Node{c.convertTable(n)}

	case atom.Img:
		src := getAttr(n, "src")
		alt := getAttr(n, "alt")
		if src != "" {
			return []Node{Image(src, alt)}
		}
		return nil

	case atom.Iframe:
		return c.convertIframe(n)

	// Inline elements — wrap in paragraph if at block level
	case atom.A, atom.Strong, atom.B, atom.Em, atom.I, atom.U, atom.S, atom.Del,
		atom.Code, atom.Sub, atom.Sup, atom.Span, atom.Small, atom.Mark:
		inline := c.convertInlineElement(n, nil)
		return []Node{Paragraph(inline...)}

	case atom.Figure:
		return c.convertFigure(n)

	case atom.Div, atom.Section, atom.Article, atom.Aside, atom.Header, atom.Footer, atom.Nav, atom.Main:
		// If the container has meaningful classes/attributes, preserve as htmlBlock
		// to avoid losing structure. Otherwise unwrap into children.
		classes := getAttr(n, "class")
		if classes != "" {
			rendered := renderNode(n)
			sanitized := tiptap.SanitizeHTMLBlock(rendered)
			if strings.TrimSpace(sanitized) != "" {
				c.warn(warnHTMLBlockFallback())
				return []Node{HTMLBlock(sanitized)}
			}
		}
		return c.convertChildren(n)

	default:
		// Unknown element — preserve as htmlBlock if it has meaningful content
		rendered := renderNode(n)
		if strings.TrimSpace(rendered) != "" {
			sanitized := tiptap.SanitizeHTMLBlock(rendered)
			if strings.TrimSpace(sanitized) != "" {
				c.warn(warnHTMLBlockFallback())
				return []Node{HTMLBlock(sanitized)}
			}
		}
		return nil
	}
}

// convertInline walks children of an inline container and returns text nodes with marks.
func (c *converter) convertInline(parent *html.Node) []Node {
	return c.convertInlineChildren(parent, nil)
}

// convertInlineChildren recursively converts inline children, accumulating marks.
func (c *converter) convertInlineChildren(parent *html.Node, marks []Mark) []Node {
	var result []Node
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			text := child.Data
			if text == "" {
				continue
			}
			result = append(result, Text(text, marks...))

		case html.ElementNode:
			result = append(result, c.convertInlineElement(child, marks)...)
		}
	}
	return result
}

// convertInlineElement converts an inline element, adding appropriate marks.
func (c *converter) convertInlineElement(n *html.Node, parentMarks []Mark) []Node {
	var mark *Mark

	switch n.DataAtom {
	case atom.Strong, atom.B:
		m := BoldMark()
		mark = &m
	case atom.Em, atom.I:
		m := ItalicMark()
		mark = &m
	case atom.U:
		m := UnderlineMark()
		mark = &m
	case atom.S, atom.Del:
		m := StrikeMark()
		mark = &m
	case atom.Code:
		m := CodeMark()
		mark = &m
	case atom.Sub:
		m := Mark{Type: "subscript"}
		mark = &m
	case atom.Sup:
		m := Mark{Type: "superscript"}
		mark = &m
	case atom.A:
		href := getAttr(n, "href")
		if href != "" && isSafeURL(href) {
			m := LinkMark(href)
			mark = &m
		}
	case atom.Br:
		return []Node{HardBreak()}
	case atom.Img:
		src := getAttr(n, "src")
		alt := getAttr(n, "alt")
		if src != "" {
			return []Node{Image(src, alt)}
		}
		return nil
	}

	marks := parentMarks
	if mark != nil {
		marks = append(append([]Mark{}, parentMarks...), *mark)
	}

	return c.convertInlineChildren(n, marks)
}

// convertListItems extracts list items from a list element.
func (c *converter) convertListItems(list *html.Node) []Node {
	var items []Node
	for child := list.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Li {
			content := c.convertChildren(child)
			content = ensureBlockContent(content)
			items = append(items, ListItem(content...))
		}
	}
	return items
}

// convertTable converts an HTML table to Tiptap table nodes.
func (c *converter) convertTable(table *html.Node) Node {
	var rows []Node
	isFirst := true

	walkElements(table, func(n *html.Node) {
		if n.DataAtom != atom.Tr {
			return
		}
		var cells []Node
		for cell := n.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type != html.ElementNode {
				continue
			}
			content := c.convertChildren(cell)
			content = ensureBlockContent(content)

			if cell.DataAtom == atom.Th || isFirst {
				cells = append(cells, TableHeader(content...))
			} else {
				cells = append(cells, TableCell(content...))
			}
		}
		if len(cells) > 0 {
			rows = append(rows, TableRow(cells...))
			isFirst = false
		}
	})

	if len(rows) == 0 {
		rows = []Node{TableRow(TableCell(Paragraph()))}
	}
	return Table(rows...)
}

// convertIframe handles iframe elements.
func (c *converter) convertIframe(n *html.Node) []Node {
	src := getAttr(n, "src")
	if src == "" {
		return nil
	}

	// Check if it's a supported video provider
	provider, sourceUrl, embedUrl, ok := parseVideoIframe(src)
	if ok {
		return []Node{VideoEmbed(provider, sourceUrl, embedUrl)}
	}

	// Unsupported iframe — convert to link if safe
	c.warn(warnUnsupportedIframe(src))
	if isSafeURL(src) {
		return []Node{Paragraph(Text(src, LinkMark(src)))}
	}
	return []Node{Paragraph(Text(src))}
}

// convertFigure handles <figure> elements (image + caption).
func (c *converter) convertFigure(n *html.Node) []Node {
	var result []Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.DataAtom == atom.Img {
			src := getAttr(child, "src")
			alt := getAttr(child, "alt")
			if src != "" {
				result = append(result, Image(src, alt))
			}
		} else if child.DataAtom == atom.Figcaption {
			// Caption as a paragraph
			result = append(result, Paragraph(c.convertInline(child)...))
		}
	}
	return result
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// findBody finds the <body> element, or returns the root if not found.
func findBody(doc *html.Node) *html.Node {
	var body *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
			if body != nil {
				return
			}
		}
	}
	f(doc)
	if body != nil {
		return body
	}
	return doc
}

// findFirstChild finds the first child element with the given tag.
func findFirstChild(parent *html.Node, tag atom.Atom) *html.Node {
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == tag {
			return c
		}
	}
	return nil
}

// extractText recursively extracts all text content from a node.
func extractText(n *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return sb.String()
}

// walkElements walks all element descendants of a node.
func walkElements(n *html.Node, fn func(*html.Node)) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			fn(c)
			walkElements(c, fn)
		}
	}
}

// renderNode renders an HTML node back to string for htmlBlock fallback.
func renderNode(n *html.Node) string {
	var sb strings.Builder
	html.Render(&sb, n)
	return sb.String()
}

// isSafeURL checks that a URL uses a safe scheme (http, https, mailto, tel, or relative).
func isSafeURL(href string) bool {
	h := strings.TrimSpace(strings.ToLower(href))
	if h == "" {
		return false
	}
	// Relative URLs and anchors are safe
	if strings.HasPrefix(h, "/") || strings.HasPrefix(h, "#") || strings.HasPrefix(h, "?") {
		return true
	}
	// Safe schemes
	if strings.HasPrefix(h, "https://") || strings.HasPrefix(h, "http://") ||
		strings.HasPrefix(h, "mailto:") || strings.HasPrefix(h, "tel:") {
		return true
	}
	// Everything else (javascript:, data:, vbscript:, etc.) is unsafe
	return false
}

// ensureBlockContent wraps inline-only content in a paragraph if needed.
func ensureBlockContent(nodes []Node) []Node {
	if len(nodes) == 0 {
		return []Node{Paragraph()}
	}
	allInline := true
	for _, n := range nodes {
		if n.Type != "text" && n.Type != "" {
			allInline = false
			break
		}
	}
	if allInline {
		return []Node{Paragraph(nodes...)}
	}
	return nodes
}
