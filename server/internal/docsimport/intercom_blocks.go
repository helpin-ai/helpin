package docsimport

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/intercom"
)

// Warning types an Intercom block conversion can report.
const (
	WarningIntercomUnknownBlock  = "intercom_unknown_block"
	WarningIntercomCalloutColor  = "intercom_callout_color"
	WarningIntercomVideoDropped  = "intercom_video_dropped"
	WarningIntercomUnknownInline = "intercom_unknown_inline_tag"
)

// zeroWidthSpace pads otherwise empty Intercom paragraphs.
const zeroWidthSpace = "\u200b"

// IntercomImage records one image of a converted article.
type IntercomImage struct {
	// Source is the address the article published.
	Source string
	// Stored is where the copy lives, or "" when the image could not be
	// copied and the article still points at Source.
	Stored string
}

// IntercomBlockOptions tunes a block conversion.
type IntercomBlockOptions struct {
	// CopyImage stores an image and returns its new address. It returns ""
	// when the image cannot be copied; the article then keeps the source
	// address. Nil keeps every source address.
	CopyImage func(source string) string
}

// IntercomBlockResult is a converted article body.
type IntercomBlockResult struct {
	Doc      Node
	Warnings []Warning
	Images   []IntercomImage
}

// ConvertIntercomBlocks converts the block list a public Intercom article
// publishes into a Tiptap document. Content is carried over as written;
// nothing is summarized or reworded.
func ConvertIntercomBlocks(blocks []intercom.Block, opts IntercomBlockOptions) IntercomBlockResult {
	c := &intercomBlockConverter{opts: opts, seenWarnings: map[string]bool{}}
	content := c.blocks(blocks, 0)
	if len(content) == 0 {
		content = []Node{Paragraph()}
	}
	return IntercomBlockResult{Doc: Doc(content...), Warnings: c.warnings, Images: c.images}
}

// intercomBlockMaxDepth bounds nesting so a malformed article cannot recurse
// without end.
const intercomBlockMaxDepth = 12

var _intercomHeadingLevels = map[string]int{
	"heading": 2, "subheading": 3, "subheading3": 4, "subheading4": 5,
}

// _intercomCalloutVariants maps the first six hex digits of a callout's
// background color to the Helpin callout it matches.
var _intercomCalloutVariants = map[string]string{
	"d7efdc": "tip", "e8e8e8": "grey", "fdf2e6": "warning", "feedaf": "warning",
	"fed9db": "danger", "e3e7fa": "info", "d9e5fb": "info",
}

type intercomVideoProvider struct{ embed, source string }

var _intercomVideoProviders = map[string]intercomVideoProvider{
	"youtube": {embed: "https://www.youtube.com/embed/%s", source: "https://www.youtube.com/watch?v=%s"},
	"vimeo":   {embed: "https://player.vimeo.com/video/%s", source: "https://vimeo.com/%s"},
	"loom":    {embed: "https://www.loom.com/embed/%s", source: "https://www.loom.com/share/%s"},
	"wistia":  {embed: "https://fast.wistia.net/embed/iframe/%s", source: "https://fast.wistia.net/embed/iframe/%s"},
}

var _intercomInlineMarks = map[string]string{
	"b": "bold", "strong": "bold", "i": "italic", "em": "italic", "u": "underline",
	"s": "strike", "strike": "strike", "del": "strike", "code": "code",
	"sub": "subscript", "sup": "superscript",
}

// _intercomTransparentInline are wrappers that carry no formatting.
var _intercomTransparentInline = map[string]bool{"span": true, "p": true, "div": true, "font": true}

// intercomOpenTag is an inline tag that is still open, with the mark it adds.
type intercomOpenTag struct {
	tag  string
	mark *Mark
}

// _htmlVoidElements have no end tag.
var _htmlVoidElements = map[string]bool{
	"area": true, "base": true, "col": true, "embed": true, "hr": true, "img": true, "input": true,
	"link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true,
}

type intercomBlockConverter struct {
	opts         IntercomBlockOptions
	warnings     []Warning
	seenWarnings map[string]bool
	images       []IntercomImage
}

// warn records a warning once per distinct message, so an article with forty
// identical callouts reports one line.
func (c *intercomBlockConverter) warn(kind, message string) {
	key := kind + "\x00" + message
	if c.seenWarnings[key] {
		return
	}
	c.seenWarnings[key] = true
	c.warnings = append(c.warnings, Warning{Type: kind, Message: message})
}

func (c *intercomBlockConverter) blocks(blocks []intercom.Block, depth int) []Node {
	if depth > intercomBlockMaxDepth {
		return nil
	}
	var out []Node
	for _, block := range blocks {
		out = append(out, c.block(block, depth)...)
	}
	return out
}

// container converts nested blocks for a node that must not be empty.
func (c *intercomBlockConverter) container(blocks []intercom.Block, depth int) []Node {
	if content := c.blocks(blocks, depth+1); len(content) > 0 {
		return content
	}
	return []Node{Paragraph()}
}

func (c *intercomBlockConverter) block(block intercom.Block, depth int) []Node {
	if level, ok := _intercomHeadingLevels[block.Type]; ok {
		return c.heading(block, level)
	}
	switch block.Type {
	case "paragraph":
		return c.paragraph(block.Text)
	case "image":
		return c.image(block)
	case "code":
		return []Node{CodeBlock(strings.TrimSpace(block.Language), intercomCodeText(block.Text))}
	case "horizontalRule":
		return []Node{HorizontalRule()}
	case "orderedNestedList", "unorderedNestedList":
		return c.list(block, depth)
	case "callout":
		return []Node{Callout(c.calloutVariant(block.Style.BackgroundColor), c.container(block.Content, depth)...)}
	case "table":
		return c.table(block, depth)
	case "collapsibleSection":
		title := strings.TrimSpace(intercomPlainText(block.Summary.Text))
		if title == "" {
			title = "Details"
		}
		return []Node{ToggleSection(title, false, c.container(block.Content, depth)...)}
	case "video":
		return c.video(block)
	}
	c.warn(WarningIntercomUnknownBlock, fmt.Sprintf("block type %q is not supported; its text was kept as a paragraph", block.Type))
	nodes := c.paragraph(block.Text)
	return append(nodes, c.blocks(block.Content, depth+1)...)
}

func (c *intercomBlockConverter) paragraph(text string) []Node {
	inline := c.inline(text)
	if !hasVisibleInline(inline) {
		// Intercom pads spacing with paragraphs that hold only a
		// zero-width space; they carry no content.
		return nil
	}
	return []Node{Paragraph(inline...)}
}

func (c *intercomBlockConverter) heading(block intercom.Block, level int) []Node {
	inline := c.inline(block.Text)
	if !hasVisibleInline(inline) {
		return nil
	}
	var attrs map[string]any
	if id := strings.TrimSpace(block.IDAttribute); id != "" {
		// Keeps "#section" links into this article working.
		attrs = map[string]any{"id": id}
	}
	return []Node{HeadingWithAttrs(level, attrs, inline...)}
}

func (c *intercomBlockConverter) list(block intercom.Block, depth int) []Node {
	var items []Node
	for _, item := range block.Items {
		content := c.blocks(item.Content, depth+1)
		if len(content) == 0 || content[0].Type != "paragraph" {
			// A list item must open with a paragraph.
			content = append([]Node{Paragraph()}, content...)
		}
		items = append(items, ListItem(content...))
	}
	if len(items) == 0 {
		return nil
	}
	if block.Type == "orderedNestedList" {
		return []Node{OrderedList(1, items...)}
	}
	return []Node{BulletList(items...)}
}

func (c *intercomBlockConverter) table(block intercom.Block, depth int) []Node {
	var rows []Node
	for _, row := range block.Rows {
		var cells []Node
		for _, cell := range row.Cells {
			cells = append(cells, TableCell(c.container(cell.Content, depth)...))
		}
		if len(cells) > 0 {
			rows = append(rows, TableRow(cells...))
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return []Node{Table(rows...)}
}

func (c *intercomBlockConverter) calloutVariant(background string) string {
	color := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(background), "#"))
	if len(color) > 6 {
		color = color[:6]
	}
	if variant, ok := _intercomCalloutVariants[color]; ok {
		return variant
	}
	if color != "" {
		c.warn(WarningIntercomCalloutColor, fmt.Sprintf("callout color #%s has no matching style; shown as an info callout", color))
	}
	return "info"
}

func (c *intercomBlockConverter) video(block intercom.Block) []Node {
	provider := strings.ToLower(strings.TrimSpace(block.Provider))
	id := strings.TrimSpace(string(block.ID))
	known, ok := _intercomVideoProviders[provider]
	if !ok || id == "" {
		c.warn(WarningIntercomVideoDropped, fmt.Sprintf("video from %q is not supported and was left out", block.Provider))
		return nil
	}
	return []Node{VideoEmbed(provider, fmt.Sprintf(known.source, id), fmt.Sprintf(known.embed, id))}
}

func (c *intercomBlockConverter) image(block intercom.Block) []Node {
	source := strings.TrimSpace(block.URL)
	if !isHTTPImageURL(source) {
		return nil
	}
	stored := ""
	if c.opts.CopyImage != nil {
		stored = c.opts.CopyImage(source)
	}
	c.images = append(c.images, IntercomImage{Source: source, Stored: stored})
	src := stored
	if src == "" {
		src = source
	}
	linkURL := strings.TrimSpace(block.LinkURL)
	if !isSafeURL(linkURL) {
		linkURL = ""
	}
	node := Image(src, strings.TrimSpace(intercomPlainText(block.Text)), linkURL)
	switch block.Align {
	case "left", "center", "right":
		node.Attrs["alignment"] = block.Align
	}
	if width := block.DisplayWidth(); width > 0 {
		node.Attrs["width"] = fmt.Sprintf("%dpx", width)
	}
	return []Node{node}
}

// isHTTPImageURL reports whether an image address can be shown and copied:
// a web address, never inline data or a script.
func isHTTPImageURL(source string) bool {
	lower := strings.ToLower(source)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

// inline converts the HTML fragment of a text block into text nodes with
// marks, dropping leading and trailing line breaks.
func (c *intercomBlockConverter) inline(fragment string) []Node {
	if fragment == "" {
		return nil
	}
	var (
		nodes []Node
		stack []intercomOpenTag
	)
	tokenizer := html.NewTokenizer(strings.NewReader(fragment))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return trimHardBreaks(nodes)
		case html.TextToken:
			text := strings.ReplaceAll(string(tokenizer.Text()), zeroWidthSpace, "")
			if text == "" {
				continue
			}
			nodes = append(nodes, Text(text, activeMarks(stack)...))
		case html.SelfClosingTagToken:
			if name, _ := tokenizer.TagName(); string(name) == "br" {
				nodes = append(nodes, HardBreak())
			}
		case html.StartTagToken:
			name, hasAttr := tokenizer.TagName()
			tag := string(name)
			if tag == "br" {
				nodes = append(nodes, HardBreak())
				continue
			}
			if _htmlVoidElements[tag] {
				// An element that never closes cannot wrap text.
				continue
			}
			stack = append(stack, intercomOpenTag{tag: tag, mark: c.inlineMark(tag, tokenizer, hasAttr)})
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			// Close the tag this end tag names, with anything left open inside
			// it. A stray end tag closes nothing.
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == string(name) {
					stack = stack[:i]
					break
				}
			}
		}
	}
}

// inlineMark returns the mark an opening tag adds, or nil for a tag that
// only wraps text.
func (c *intercomBlockConverter) inlineMark(tag string, tokenizer *html.Tokenizer, hasAttr bool) *Mark {
	if tag == "a" {
		href := ""
		for hasAttr {
			var key, value []byte
			key, value, hasAttr = tokenizer.TagAttr()
			if string(key) == "href" {
				href = strings.TrimSpace(string(value))
			}
		}
		// The text of a link with no address, or with one that could run
		// script, is kept as plain text.
		if href == "" || !isSafeURL(href) {
			return nil
		}
		mark := LinkMark(href)
		return &mark
	}
	if markType, ok := _intercomInlineMarks[tag]; ok {
		return &Mark{Type: markType}
	}
	if !_intercomTransparentInline[tag] {
		c.warn(WarningIntercomUnknownInline, fmt.Sprintf("inline tag <%s> is not supported; its text was kept without it", tag))
	}
	return nil
}

// activeMarks flattens the open tags into the marks of one text node: each
// mark type once, and inline code alone because it cannot combine.
func activeMarks(stack []intercomOpenTag) []Mark {
	var marks []Mark
	seen := map[string]bool{}
	for _, open := range stack {
		mark := open.mark
		if mark == nil || seen[mark.Type] {
			continue
		}
		seen[mark.Type] = true
		marks = append(marks, *mark)
	}
	if !seen["code"] || len(marks) == 1 {
		return marks
	}
	return []Mark{CodeMark()}
}

func trimHardBreaks(nodes []Node) []Node {
	for len(nodes) > 0 && nodes[0].Type == "hardBreak" {
		nodes = nodes[1:]
	}
	for len(nodes) > 0 && nodes[len(nodes)-1].Type == "hardBreak" {
		nodes = nodes[:len(nodes)-1]
	}
	return nodes
}

// hasVisibleInline reports whether a run of inline nodes shows anything.
func hasVisibleInline(nodes []Node) bool {
	for _, node := range nodes {
		if node.Type == "text" && strings.TrimSpace(strings.ReplaceAll(node.Text, "\u00a0", " ")) != "" {
			return true
		}
	}
	return false
}

// intercomPlainText strips the markup from an HTML fragment.
func intercomPlainText(fragment string) string {
	return intercomText(fragment, " ")
}

// intercomCodeText returns the text of a code block with its line breaks.
func intercomCodeText(fragment string) string {
	return strings.TrimRight(intercomText(fragment, "\n"), "\n")
}

func intercomText(fragment, lineBreak string) string {
	var out strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(fragment))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.ReplaceAll(out.String(), zeroWidthSpace, "")
		case html.TextToken:
			out.Write(tokenizer.Text())
		case html.StartTagToken, html.SelfClosingTagToken:
			if name, _ := tokenizer.TagName(); string(name) == "br" {
				out.WriteString(lineBreak)
			}
		}
	}
}
