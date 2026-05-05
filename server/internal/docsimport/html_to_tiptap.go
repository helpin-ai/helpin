package docsimport

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

var facebookBackgroundIDPattern = regexp.MustCompile(`\b\d{12,20}\b`)

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
	addHeadingAnchorAliases(nodes)

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
	var (
		result       []Node
		inlineBuffer []Node
	)

	flushInline := func() {
		inlineBuffer = trimInlineNodes(inlineBuffer)
		if len(inlineBuffer) == 0 {
			inlineBuffer = nil
			return
		}
		result = append(result, Paragraph(inlineBuffer...))
		inlineBuffer = nil
	}

	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Dt {
			if dd := nextElementSibling(child, atom.Dd); dd != nil {
				nodes := c.convertDefinitionPair(child, dd)
				for _, node := range nodes {
					if node.Type == "" {
						continue
					}
					if isInlineNode(node) {
						inlineBuffer = append(inlineBuffer, node)
						continue
					}
					flushInline()
					result = append(result, node)
				}
				child = dd
				continue
			}
		}

		nodes := c.convertNode(child)
		for _, node := range nodes {
			if node.Type == "" {
				continue
			}
			if isInlineNode(node) {
				inlineBuffer = append(inlineBuffer, node)
				continue
			}
			flushInline()
			result = append(result, node)
		}
	}
	flushInline()
	return compactBlockNodes(result)
}

// convertNode converts a single HTML node into Tiptap nodes.
func (c *converter) convertNode(n *html.Node) []Node {
	switch n.Type {
	case html.TextNode:
		text := normalizeInlineText(n.Data)
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
	if video, ok := parseWistiaEmbedContainer(n); ok {
		return []Node{video}
	}
	if grid, ok := c.convertHelpScoutFacebookBackgroundGrid(n); ok {
		return []Node{grid}
	}
	if n.Type == html.ElementNode && n.DataAtom == atom.Div && hasAttr(n, "data-html-block") {
		if containsElement(n, atom.Details) {
			return c.convertChildren(n)
		}
		raw := renderChildren(n)
		if requiresSandboxedHTMLBlock(n) {
			if strings.TrimSpace(raw) != "" {
				return []Node{SandboxedHTMLBlock(raw)}
			}
			return nil
		}
		sanitized := tiptap.SanitizeHTMLBlock(raw)
		if strings.TrimSpace(sanitized) != "" {
			return []Node{HTMLBlock(sanitized)}
		}
		return nil
	}

	// Check for Help Scout callout first
	if variant, ok := isHelpScoutCallout(n); ok {
		content := c.convertChildren(n)
		content = ensureBlockContent(content)
		c.warn(warnCalloutGuess(getAttr(n, "class"), variant))
		return []Node{Callout(variant, content...)}
	}
	if isStyledCustomHTMLContainer(n) {
		raw := renderNode(n)
		if requiresSandboxedHTMLBlock(n) {
			if strings.TrimSpace(raw) != "" {
				c.warn(warnHTMLBlockFallback())
				return []Node{SandboxedHTMLBlock(raw)}
			}
			return nil
		}
		sanitized := tiptap.SanitizeHTMLBlock(raw)
		if strings.TrimSpace(sanitized) != "" {
			c.warn(warnHTMLBlockFallback())
			return []Node{HTMLBlock(sanitized)}
		}
		return nil
	}

	switch n.DataAtom {
	// Block elements
	case atom.P:
		return c.convertParagraph(n)

	case atom.H1:
		return c.convertHeading(1, n)
	case atom.H2:
		return c.convertHeading(2, n)
	case atom.H3:
		return c.convertHeading(3, n)
	case atom.H4:
		return c.convertHeading(4, n)
	case atom.H5:
		return c.convertHeading(5, n)
	case atom.H6:
		return c.convertHeading(6, n)

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
	case atom.Dl:
		return c.convertDefinitionList(n)
	case atom.Dt, atom.Dd:
		content := c.convertChildren(n)
		if len(content) > 0 {
			return content
		}
		inline := trimInlineNodes(c.convertInline(n))
		if len(inline) > 0 {
			return []Node{Paragraph(inline...)}
		}
		return nil

	case atom.Table:
		return []Node{c.convertTable(n)}

	case atom.Img:
		src := getAttr(n, "src")
		if src == "" {
			src = getAttr(n, "data-src")
		}
		alt := getAttr(n, "alt")
		if src != "" {
			return []Node{Image(src, alt, "")}
		}
		return nil

	case atom.Iframe:
		return c.convertIframe(n)
	case atom.Details:
		return c.convertDetails(n)

	// Inline elements stay inline; convertChildren is responsible for wrapping
	// contiguous inline content into paragraphs when needed.
	case atom.A, atom.Strong, atom.B, atom.Em, atom.I, atom.U, atom.S, atom.Del,
		atom.Code, atom.Sub, atom.Sup, atom.Span, atom.Small, atom.Mark:
		return c.convertInlineElement(n, nil)

	case atom.Figure:
		return c.convertFigure(n)

	case atom.Div, atom.Section, atom.Article, atom.Aside, atom.Header, atom.Footer, atom.Nav, atom.Main:
		// Always try to extract native content from containers first.
		// Only fall back to htmlBlock if the container has classes AND
		// contains no native-convertible children.
		children := c.convertChildren(n)
		if len(children) > 0 {
			return children
		}
		// Empty container with classes — preserve as htmlBlock
		classes := getAttr(n, "class")
		if classes != "" {
			rendered := renderNode(n)
			sanitized := tiptap.SanitizeHTMLBlock(rendered)
			if strings.TrimSpace(sanitized) != "" {
				c.warn(warnHTMLBlockFallback())
				return []Node{HTMLBlock(sanitized)}
			}
		}
		return nil

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

// convertParagraph handles <p> elements. If the paragraph contains block-level
// elements like <img> (which is atom/block in Tiptap), they are extracted as
// separate block nodes. Text content is grouped into paragraphs.
func (c *converter) convertParagraph(n *html.Node) []Node {
	var result []Node
	var inlineBuffer []Node

	flushInline := func() {
		inlineBuffer = trimInlineNodes(inlineBuffer)
		if len(inlineBuffer) > 0 {
			result = append(result, Paragraph(inlineBuffer...))
			inlineBuffer = nil
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Img {
			flushInline()
			src := getAttr(child, "src")
			if src == "" {
				src = getAttr(child, "data-src")
			}
			alt := getAttr(child, "alt")
			if src != "" {
				result = append(result, Image(src, alt, ""))
			}
		} else if child.Type == html.ElementNode && child.DataAtom == atom.Br {
			inlineBuffer = append(inlineBuffer, HardBreak())
		} else if child.Type == html.TextNode {
			text := normalizeInlineText(child.Data)
			if text != "" {
				inlineBuffer = append(inlineBuffer, Text(text))
			}
		} else if child.Type == html.ElementNode {
			inlineBuffer = append(inlineBuffer, c.convertInlineElement(child, nil)...)
		}
	}
	flushInline()

	if len(result) == 0 {
		return nil
	}
	return compactBlockNodes(result)
}

func (c *converter) convertHeading(level int, n *html.Node) []Node {
	content := trimInlineNodes(c.convertInline(n))
	if len(content) == 0 {
		return nil
	}
	attrs := map[string]any{}
	if id := strings.TrimSpace(getAttr(n, "id")); id != "" {
		attrs["id"] = id
	}
	return []Node{HeadingWithAttrs(level, attrs, content...)}
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
			text := normalizeInlineText(child.Data)
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
		href := normalizeImportHref(getAttr(n, "href"))
		// If the only child is an <img>, create a linked image node.
		if img := onlyChildImg(n); img != nil {
			src := getAttr(img, "src")
			if src == "" {
				src = getAttr(img, "data-src")
			}
			alt := getAttr(img, "alt")
			linkUrl := ""
			if href != "" && isSafeURL(href) {
				linkUrl = href
			}
			if src != "" {
				return []Node{Image(src, alt, linkUrl)}
			}
			return nil
		}
		if href != "" && isSafeURL(href) {
			m := LinkMark(href)
			mark = &m
		}
	case atom.Br:
		return []Node{HardBreak()}
	case atom.Img:
		src := getAttr(n, "src")
		if src == "" {
			src = getAttr(n, "data-src")
		}
		alt := getAttr(n, "alt")
		if src != "" {
			return []Node{Image(src, alt, "")}
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

func (c *converter) convertDefinitionList(dl *html.Node) []Node {
	var pairs []definitionPair
	for child := dl.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.DataAtom != atom.Dt {
			continue
		}
		dd := nextElementSibling(child, atom.Dd)
		if dd == nil {
			continue
		}
		pairs = append(pairs, definitionPair{term: child, definition: dd})
		child = dd
	}
	if len(pairs) == 0 {
		return c.convertChildren(dl)
	}

	if start, ok := numericDefinitionStart(pairs); ok {
		items := make([]Node, 0, len(pairs))
		for _, pair := range pairs {
			items = append(items, ListItem(ensureBlockContent(c.convertChildren(pair.definition))...))
		}
		return []Node{OrderedList(start, items...)}
	}

	items := make([]Node, 0, len(pairs))
	for _, pair := range pairs {
		itemContent := []Node{Paragraph(Text(trimImportSpace(extractText(pair.term)), BoldMark()))}
		itemContent = append(itemContent, ensureBlockContent(c.convertChildren(pair.definition))...)
		items = append(items, ListItem(compactBlockNodes(itemContent)...))
	}
	return []Node{BulletList(items...)}
}

func (c *converter) convertDefinitionPair(dt, dd *html.Node) []Node {
	pair := definitionPair{term: dt, definition: dd}
	if start, ok := numericDefinitionStart([]definitionPair{pair}); ok {
		return []Node{OrderedList(start, ListItem(ensureBlockContent(c.convertChildren(dd))...))}
	}
	itemContent := []Node{Paragraph(Text(trimImportSpace(extractText(dt)), BoldMark()))}
	itemContent = append(itemContent, ensureBlockContent(c.convertChildren(dd))...)
	return []Node{BulletList(ListItem(compactBlockNodes(itemContent)...))}
}

type definitionPair struct {
	term       *html.Node
	definition *html.Node
}

func numericDefinitionStart(pairs []definitionPair) (int, bool) {
	if len(pairs) == 0 {
		return 1, false
	}
	start := 0
	for i, pair := range pairs {
		n, err := strconv.Atoi(trimImportSpace(extractText(pair.term)))
		if err != nil || n <= 0 {
			return 1, false
		}
		if i == 0 {
			start = n
			continue
		}
		if n != start+i {
			return 1, false
		}
	}
	return start, true
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

func (c *converter) convertDetails(n *html.Node) []Node {
	attrs := map[string]any{"title": "Details"}
	if summary := findFirstChild(n, atom.Summary); summary != nil {
		title, icon, badge := detailsSummaryParts(summary)
		if title != "" {
			attrs["title"] = title
		}
		if icon != "" {
			attrs["icon"] = icon
		}
		if badge != "" {
			attrs["badgeText"] = badge
		}
		if icon != "" || badge != "" {
			attrs["sourceStyle"] = "helpScoutCard"
		}
	}
	if hasAttr(n, "open") {
		attrs["open"] = true
	}

	container := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: atom.Div.String()}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Summary {
			continue
		}
		container.AppendChild(cloneNodeTree(child))
	}

	content := c.convertChildren(container)
	content = ensureBlockContent(content)
	return []Node{ToggleSectionWithAttrs(attrs, content...)}
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
				result = append(result, Image(src, alt, ""))
			}
		} else if child.DataAtom == atom.Figcaption {
			// Caption as a paragraph
			result = append(result, Paragraph(c.convertInline(child)...))
		}
	}
	return result
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parseWistiaEmbedContainer(n *html.Node) (Node, bool) {
	if n.Type != html.ElementNode {
		return Node{}, false
	}
	switch n.DataAtom {
	case atom.Div, atom.Section:
	default:
		return Node{}, false
	}

	classes := strings.Fields(getAttr(n, "class"))
	hasWistiaEmbed := false
	videoID := ""
	for _, cls := range classes {
		if cls == "wistia_embed" {
			hasWistiaEmbed = true
			continue
		}
		if strings.HasPrefix(cls, "wistia_async_") {
			videoID = strings.TrimPrefix(cls, "wistia_async_")
		}
	}
	if !hasWistiaEmbed || !isSafeWistiaID(videoID) {
		return Node{}, false
	}

	embedURL := "https://fast.wistia.net/embed/iframe/" + videoID
	return VideoEmbed("wistia", embedURL, embedURL), true
}

func isSafeWistiaID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func (c *converter) convertHelpScoutFacebookBackgroundGrid(n *html.Node) (Node, bool) {
	if n.Type != html.ElementNode || n.DataAtom != atom.Div || !hasAttr(n, "data-html-block") {
		return Node{}, false
	}
	swatches := extractFacebookBackgroundSwatches(n)
	if len(swatches) < 2 {
		return Node{}, false
	}

	var b strings.Builder
	b.WriteString(`<div class="docs-fb-background-grid" data-fb-background-grid="">`)
	for _, swatch := range swatches {
		b.WriteString(`<div class="docs-fb-background-card"><img src="`)
		b.WriteString(swatch.src)
		b.WriteString(`" alt="" width="36" height="36" loading="lazy"><code>`)
		b.WriteString(swatch.id)
		b.WriteString(`</code></div>`)
	}
	b.WriteString(`</div>`)
	return HTMLBlock(tiptap.SanitizeHTMLBlock(b.String())), true
}

type facebookBackgroundSwatch struct {
	src string
	id  string
}

func extractFacebookBackgroundSwatches(n *html.Node) []facebookBackgroundSwatch {
	var swatches []facebookBackgroundSwatch
	seen := map[string]bool{}
	walkElements(n, func(candidate *html.Node) {
		if candidate.DataAtom != atom.Table || countDescendantImages(candidate) != 1 {
			return
		}
		img := firstDescendantImage(candidate)
		if img == nil {
			return
		}
		src := getAttr(img, "src")
		if !isSafeDataImageSrc(src) {
			return
		}
		id := facebookBackgroundIDPattern.FindString(extractText(candidate))
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		swatches = append(swatches, facebookBackgroundSwatch{src: src, id: id})
	})
	return swatches
}

func countDescendantImages(n *html.Node) int {
	count := 0
	walkElements(n, func(candidate *html.Node) {
		if candidate.DataAtom == atom.Img {
			count++
		}
	})
	return count
}

func firstDescendantImage(n *html.Node) *html.Node {
	var img *html.Node
	walkElements(n, func(candidate *html.Node) {
		if img == nil && candidate.DataAtom == atom.Img {
			img = candidate
		}
	})
	return img
}

func isSafeDataImageSrc(src string) bool {
	lower := strings.ToLower(strings.TrimSpace(src))
	switch {
	case strings.HasPrefix(lower, "data:image/png;base64,"):
		return true
	case strings.HasPrefix(lower, "data:image/jpeg;base64,"):
		return true
	case strings.HasPrefix(lower, "data:image/jpg;base64,"):
		return true
	case strings.HasPrefix(lower, "data:image/gif;base64,"):
		return true
	case strings.HasPrefix(lower, "data:image/webp;base64,"):
		return true
	default:
		return false
	}
}

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

func nextElementSibling(n *html.Node, tag atom.Atom) *html.Node {
	for sibling := n.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == html.TextNode && strings.TrimSpace(sibling.Data) == "" {
			continue
		}
		if sibling.Type != html.ElementNode {
			return nil
		}
		if sibling.DataAtom == tag {
			return sibling
		}
		return nil
	}
	return nil
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func detailsSummaryParts(summary *html.Node) (title string, icon string, badge string) {
	var titleParts []string
	for child := summary.FirstChild; child != nil; child = child.NextSibling {
		text := trimImportSpace(extractText(child))
		text = strings.Trim(text, "▼▾▴")
		text = trimImportSpace(text)
		if text == "" {
			continue
		}
		if badge == "" && looksLikeDetailsBadge(text) {
			badge = text
			continue
		}
		if icon == "" && looksLikeSummaryIcon(text) {
			icon = text
			continue
		}
		titleParts = append(titleParts, text)
	}
	if len(titleParts) == 0 {
		return trimImportSpace(strings.Trim(extractText(summary), "▼▾▴")), icon, badge
	}
	return strings.Join(titleParts, " "), icon, badge
}

func looksLikeDetailsBadge(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "topic") || strings.Contains(lower, "article") || strings.Contains(lower, "item")
}

func looksLikeSummaryIcon(text string) bool {
	runes := []rune(text)
	if len(runes) == 0 || len(runes) > 3 {
		return false
	}
	for _, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func cloneNodeTree(n *html.Node) *html.Node {
	clone := &html.Node{
		Type:      n.Type,
		DataAtom:  n.DataAtom,
		Data:      n.Data,
		Namespace: n.Namespace,
		Attr:      append([]html.Attribute(nil), n.Attr...),
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		clone.AppendChild(cloneNodeTree(child))
	}
	return clone
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

func containsElement(n *html.Node, tag atom.Atom) bool {
	found := false
	walkElements(n, func(candidate *html.Node) {
		if candidate.DataAtom == tag {
			found = true
		}
	})
	return found
}

func requiresSandboxedHTMLBlock(n *html.Node) bool {
	if n == nil {
		return false
	}
	if n.Type == html.ElementNode {
		switch n.DataAtom {
		case atom.Script, atom.Form, atom.Input, atom.Select, atom.Textarea, atom.Button, atom.Iframe:
			return true
		}
		for _, attr := range n.Attr {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(attr.Key)), "on") {
				return true
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if requiresSandboxedHTMLBlock(child) {
			return true
		}
	}
	return false
}

func isStyledCustomHTMLContainer(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.Div, atom.Section, atom.Article, atom.Aside, atom.Header, atom.Footer, atom.Nav, atom.Main:
	default:
		return false
	}
	style := strings.ToLower(getAttr(n, "style"))
	if strings.TrimSpace(style) == "" {
		return false
	}
	for _, token := range []string{
		"border", "border-radius", "background", "padding", "display:flex", "display: flex", "gap:",
		"box-shadow", "align-items", "justify-content",
	} {
		if strings.Contains(style, token) {
			return true
		}
	}
	return false
}

// renderNode renders an HTML node back to string for htmlBlock fallback.
func renderNode(n *html.Node) string {
	var sb strings.Builder
	html.Render(&sb, n)
	return sb.String()
}

func renderChildren(n *html.Node) string {
	var sb strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		html.Render(&sb, child)
	}
	return sb.String()
}

// onlyChildImg returns the sole <img> child of n if n has exactly one element
// child and it is an <img>. Returns nil otherwise.
func onlyChildImg(n *html.Node) *html.Node {
	var img *html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode && strings.TrimSpace(child.Data) == "" {
			continue // skip whitespace text nodes
		}
		if child.Type == html.ElementNode && child.DataAtom == atom.Img && img == nil {
			img = child
			continue
		}
		// More than one meaningful child or non-img element.
		return nil
	}
	return img
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

func normalizeImportHref(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "#") {
		if idx := strings.LastIndex(href, "#"); idx > 0 {
			return "#" + href[idx+1:]
		}
	}
	return href
}

func addHeadingAnchorAliases(nodes []Node) {
	existing := map[string]bool{}
	headingsByText := map[string][]*Node{}
	var collect func(nodes []Node)
	collect = func(nodes []Node) {
		for i := range nodes {
			node := &nodes[i]
			if node.Type == "heading" {
				if id := stringAttr(node.Attrs, "id"); id != "" {
					existing[id] = true
				}
				key := normalizeAnchorText(nodeText(node))
				if key != "" {
					headingsByText[key] = append(headingsByText[key], node)
				}
			}
			if len(node.Content) > 0 {
				collect(node.Content)
			}
		}
	}
	collect(nodes)
	if len(headingsByText) == 0 {
		return
	}

	var apply func(nodes []Node)
	apply = func(nodes []Node) {
		for i := range nodes {
			node := &nodes[i]
			if node.Type == "text" {
				for _, mark := range node.Marks {
					if mark.Type != "link" {
						continue
					}
					href := stringAttr(mark.Attrs, "href")
					if !strings.HasPrefix(href, "#") || len(href) <= 1 {
						continue
					}
					anchor := strings.TrimPrefix(href, "#")
					if existing[anchor] {
						continue
					}
					candidates := headingsByText[normalizeAnchorText(node.Text)]
					if len(candidates) != 1 {
						continue
					}
					addHeadingAlias(candidates[0], anchor)
					existing[anchor] = true
				}
			}
			if len(node.Content) > 0 {
				apply(node.Content)
			}
		}
	}
	apply(nodes)
}

func addHeadingAlias(node *Node, anchor string) {
	if node.Attrs == nil {
		node.Attrs = map[string]any{}
	}
	aliases, _ := node.Attrs["anchorAliases"].([]string)
	for _, alias := range aliases {
		if alias == anchor {
			return
		}
	}
	node.Attrs["anchorAliases"] = append(aliases, anchor)
}

func nodeText(node *Node) string {
	var b strings.Builder
	var walk func(*Node)
	walk = func(n *Node) {
		if n.Text != "" {
			b.WriteString(n.Text)
		}
		for i := range n.Content {
			walk(&n.Content[i])
		}
	}
	walk(node)
	return b.String()
}

func normalizeAnchorText(text string) string {
	text = strings.ToLower(trimImportSpace(text))
	text = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '&' || r == '/' {
			return ' '
		}
		return -1
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func stringAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if value, ok := attrs[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

// ensureBlockContent wraps inline-only content in a paragraph if needed.
func ensureBlockContent(nodes []Node) []Node {
	nodes = compactBlockNodes(nodes)
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

func compactBlockNodes(nodes []Node) []Node {
	result := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Type == "" {
			continue
		}
		if node.Type == "paragraph" {
			node.Content = trimInlineNodes(node.Content)
			if len(node.Content) == 0 {
				continue
			}
		}
		result = append(result, node)
	}
	return result
}

func trimInlineNodes(nodes []Node) []Node {
	trimmed := append([]Node(nil), nodes...)
	for len(trimmed) > 0 {
		if candidate, ok := trimInlineNodeLeft(trimmed[0]); ok {
			trimmed[0] = candidate
			break
		}
		trimmed = trimmed[1:]
	}
	for len(trimmed) > 0 {
		last := len(trimmed) - 1
		if candidate, ok := trimInlineNodeRight(trimmed[last]); ok {
			trimmed[last] = candidate
			break
		}
		trimmed = trimmed[:last]
	}
	return compactInlineBreaks(trimmed)
}

func compactInlineBreaks(nodes []Node) []Node {
	result := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		switch node.Type {
		case "hardBreak":
			if len(result) == 0 || result[len(result)-1].Type == "hardBreak" {
				continue
			}
			if result[len(result)-1].Type == "text" {
				result[len(result)-1].Text = strings.TrimRightFunc(result[len(result)-1].Text, unicode.IsSpace)
				if result[len(result)-1].Text == "" {
					result = result[:len(result)-1]
					if len(result) == 0 || result[len(result)-1].Type == "hardBreak" {
						continue
					}
				}
			}
			result = append(result, node)
		case "text":
			if len(result) > 0 && result[len(result)-1].Type == "hardBreak" {
				node.Text = strings.TrimLeftFunc(node.Text, unicode.IsSpace)
				if node.Text == "" {
					continue
				}
			}
			result = append(result, node)
		default:
			result = append(result, node)
		}
	}
	for len(result) > 0 && result[len(result)-1].Type == "hardBreak" {
		result = result[:len(result)-1]
	}
	return ensureInlineBoundarySpacing(result)
}

func ensureInlineBoundarySpacing(nodes []Node) []Node {
	if len(nodes) < 2 {
		return nodes
	}
	for i := 1; i < len(nodes); i++ {
		prev := &nodes[i-1]
		curr := &nodes[i]
		if prev.Type != "text" || curr.Type != "text" {
			continue
		}
		if !shouldInsertBoundarySpace(*prev, *curr) {
			continue
		}
		prev.Text = strings.TrimRightFunc(prev.Text, unicode.IsSpace) + " "
		curr.Text = strings.TrimLeftFunc(curr.Text, unicode.IsSpace)
	}
	return nodes
}

func shouldInsertBoundarySpace(prev, curr Node) bool {
	if len(prev.Marks) == 0 && len(curr.Marks) == 0 {
		return false
	}
	if prev.Text == "" || curr.Text == "" {
		return false
	}
	prevLast, okPrev := lastNonSpaceRune(prev.Text)
	currFirst, okCurr := firstNonSpaceRune(curr.Text)
	if !okPrev || !okCurr {
		return false
	}
	if unicode.IsSpace(prevLast) || unicode.IsSpace(currFirst) {
		return false
	}
	return isWordLikeBoundaryRune(prevLast) && isWordLikeBoundaryRune(currFirst)
}

func trimInlineNodeLeft(node Node) (Node, bool) {
	if node.Type != "text" {
		return node, true
	}
	text := strings.TrimLeftFunc(replaceNBSP(node.Text), unicode.IsSpace)
	if text == "" {
		return Node{}, false
	}
	node.Text = text
	return node, true
}

func normalizeInlineText(text string) string {
	text = replaceNBSP(text)
	if text == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(text))
	lastWasSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				builder.WriteByte(' ')
				lastWasSpace = true
			}
			continue
		}
		builder.WriteRune(r)
		lastWasSpace = false
	}
	return builder.String()
}

func firstNonSpaceRune(text string) (rune, bool) {
	for _, r := range text {
		if !unicode.IsSpace(r) {
			return r, true
		}
	}
	return 0, false
}

func lastNonSpaceRune(text string) (rune, bool) {
	for i := len(text); i > 0; {
		r, size := utf8.DecodeLastRuneInString(text[:i])
		i -= size
		if !unicode.IsSpace(r) {
			return r, true
		}
	}
	return 0, false
}

func isWordLikeBoundaryRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func trimInlineNodeRight(node Node) (Node, bool) {
	if node.Type != "text" {
		return node, true
	}
	text := strings.TrimRightFunc(replaceNBSP(node.Text), unicode.IsSpace)
	if text == "" {
		return Node{}, false
	}
	node.Text = text
	return node, true
}

func isInlineNode(node Node) bool {
	switch node.Type {
	case "text", "hardBreak":
		return true
	default:
		return false
	}
}
