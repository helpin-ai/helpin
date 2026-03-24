package crawler

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// stripElements are removed entirely (content + children).
var stripElements = map[atom.Atom]struct{}{
	atom.Script:   {},
	atom.Style:    {},
	atom.Noscript: {},
	atom.Svg:      {},
	atom.Iframe:   {},
	atom.Link:     {},
	atom.Meta:     {},
	atom.Head:     {},
}

// noiseSelectors are elements stripped when extracting main content.
// Mirrors Firecrawl's removeUnwantedElements list.
var noiseClasses = map[string]struct{}{
	"navbar": {}, "nav": {}, "sidebar": {}, "side": {},
	"breadcrumb": {}, "breadcrumbs": {}, "menu": {},
	"ad": {}, "ads": {}, "advert": {}, "advertisement": {},
	"modal": {}, "popup": {}, "overlay": {}, "widget": {},
	"social": {}, "social-media": {}, "share": {},
	"cookie": {}, "cookie-banner": {}, "cookie-consent": {},
	"lang-selector": {}, "language-selector": {},
	"skip-to-content": {}, "skip-nav": {},
}

var noiseIDs = map[string]struct{}{
	"nav": {}, "navbar": {}, "sidebar": {}, "footer": {},
	"header": {}, "menu": {}, "social": {}, "widget": {},
	"cookie-banner": {}, "cookie-consent": {},
}

// HTMLToMarkdown converts raw HTML into clean markdown suitable for RAG
// embeddings. It strips noise elements (nav, ads, scripts, etc.) and
// converts headings, lists, links, tables, and text blocks to markdown.
// Inspired by Firecrawl's preprocessing + conversion pipeline.
func HTMLToMarkdown(rawHTML string, baseURL *url.URL) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}

	// Step 1: Find <body> (or use the whole document).
	body := findBody(doc)
	if body == nil {
		body = doc
	}

	// Step 2: Strip noise elements.
	stripNoiseNodes(body)

	// Step 3: Convert to markdown.
	var sb strings.Builder
	renderNode(&sb, body, baseURL, 0)

	// Step 4: Normalize whitespace.
	return cleanMarkdown(sb.String())
}

// ExtractTitle returns the content of the <title> tag.
func ExtractTitle(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	return findTitle(doc)
}

// --- DOM helpers ---

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findBody(c); found != nil {
			return found
		}
	}
	return nil
}

func findTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.DataAtom == atom.Title {
		return strings.TrimSpace(textContent(n))
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findTitle(c); t != "" {
			return t
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

// stripNoiseNodes removes script/style/nav/ads/etc. from the DOM tree.
func stripNoiseNodes(n *html.Node) {
	var toRemove []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if shouldStrip(c) {
			toRemove = append(toRemove, c)
		} else {
			stripNoiseNodes(c)
		}
	}
	for _, c := range toRemove {
		n.RemoveChild(c)
	}
}

func shouldStrip(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	// Strip by tag name.
	if _, ok := stripElements[n.DataAtom]; ok {
		return true
	}
	// Strip nav/header/footer tags.
	switch n.DataAtom {
	case atom.Nav, atom.Footer:
		return true
	}
	// Strip by class or ID.
	for _, a := range n.Attr {
		switch a.Key {
		case "class":
			for _, cls := range strings.Fields(a.Val) {
				cls = strings.ToLower(cls)
				if _, ok := noiseClasses[cls]; ok {
					return true
				}
			}
		case "id":
			id := strings.ToLower(a.Val)
			if _, ok := noiseIDs[id]; ok {
				return true
			}
		case "aria-hidden":
			if strings.ToLower(a.Val) == "true" {
				return true
			}
		case "hidden":
			return true
		}
	}
	return false
}

// --- Markdown rendering ---

func renderNode(sb *strings.Builder, n *html.Node, baseURL *url.URL, depth int) {
	switch n.Type {
	case html.TextNode:
		text := strings.ReplaceAll(n.Data, "\t", " ")
		sb.WriteString(text)
		return

	case html.ElementNode:
		// skip
	default:
		// Recurse for document/fragment nodes.
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderNode(sb, c, baseURL, depth)
		}
		return
	}

	switch n.DataAtom {
	// Headings.
	case atom.H1:
		sb.WriteString("\n\n# ")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")
	case atom.H2:
		sb.WriteString("\n\n## ")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")
	case atom.H3:
		sb.WriteString("\n\n### ")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")
	case atom.H4:
		sb.WriteString("\n\n#### ")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")
	case atom.H5:
		sb.WriteString("\n\n##### ")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")
	case atom.H6:
		sb.WriteString("\n\n###### ")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")

	// Block elements.
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Main, atom.Aside:
		sb.WriteString("\n\n")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n\n")
	case atom.Br:
		sb.WriteString("\n")
	case atom.Hr:
		sb.WriteString("\n\n---\n\n")

	// Lists.
	case atom.Ul, atom.Ol:
		sb.WriteString("\n")
		renderListItems(sb, n, baseURL, depth, n.DataAtom == atom.Ol)
		sb.WriteString("\n")
	case atom.Li:
		// Handled by renderListItems.
		renderChildren(sb, n, baseURL, depth)

	// Links.
	case atom.A:
		href := getAttr(n, "href")
		linkText := strings.TrimSpace(childrenText(n, baseURL, depth))
		if linkText == "" {
			return
		}
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			sb.WriteString(linkText)
			return
		}
		href = resolveURL(href, baseURL)
		sb.WriteString("[")
		sb.WriteString(linkText)
		sb.WriteString("](")
		sb.WriteString(href)
		sb.WriteString(")")

	// Images — keep alt text only, drop the URL (noise for embeddings).
	case atom.Img, atom.Picture:
		alt := getAttr(n, "alt")
		if alt != "" {
			sb.WriteString(alt)
		}

	// Tables.
	case atom.Table:
		sb.WriteString("\n\n")
		renderTable(sb, n, baseURL, depth)
		sb.WriteString("\n\n")

	// Inline formatting.
	case atom.Strong, atom.B:
		sb.WriteString("**")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("**")
	case atom.Em, atom.I:
		sb.WriteString("*")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("*")
	case atom.Code:
		sb.WriteString("`")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("`")
	case atom.Pre:
		sb.WriteString("\n\n```\n")
		renderChildren(sb, n, baseURL, depth)
		sb.WriteString("\n```\n\n")
	case atom.Blockquote:
		sb.WriteString("\n\n> ")
		text := strings.TrimSpace(childrenText(n, baseURL, depth))
		sb.WriteString(strings.ReplaceAll(text, "\n", "\n> "))
		sb.WriteString("\n\n")

	// Skip entirely (already stripped, but be safe).
	case atom.Script, atom.Style, atom.Noscript, atom.Svg:
		return

	default:
		renderChildren(sb, n, baseURL, depth)
	}
}

func renderChildren(sb *strings.Builder, n *html.Node, baseURL *url.URL, depth int) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNode(sb, c, baseURL, depth)
	}
}

func childrenText(n *html.Node, baseURL *url.URL, depth int) string {
	var sb strings.Builder
	renderChildren(&sb, n, baseURL, depth)
	return sb.String()
}

func renderListItems(sb *strings.Builder, list *html.Node, baseURL *url.URL, depth int, ordered bool) {
	idx := 1
	for c := list.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.DataAtom != atom.Li {
			continue
		}
		indent := strings.Repeat("  ", depth)
		if ordered {
			sb.WriteString(fmt.Sprintf("%s%d. ", indent, idx))
			idx++
		} else {
			sb.WriteString(indent + "- ")
		}
		text := strings.TrimSpace(childrenText(c, baseURL, depth+1))
		sb.WriteString(text)
		sb.WriteString("\n")
	}
}

// --- Table rendering ---

func renderTable(sb *strings.Builder, table *html.Node, baseURL *url.URL, depth int) {
	rows := collectTableRows(table)
	if len(rows) == 0 {
		return
	}

	// Determine column count.
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return
	}

	// Render header row.
	sb.WriteString("| ")
	for i := 0; i < maxCols; i++ {
		if i < len(rows[0]) {
			sb.WriteString(rows[0][i])
		}
		sb.WriteString(" | ")
	}
	sb.WriteString("\n|")
	for i := 0; i < maxCols; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Render body rows.
	for _, row := range rows[1:] {
		sb.WriteString("| ")
		for i := 0; i < maxCols; i++ {
			if i < len(row) {
				sb.WriteString(row[i])
			}
			sb.WriteString(" | ")
		}
		sb.WriteString("\n")
	}
}

func collectTableRows(table *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					text := strings.TrimSpace(textContent(c))
					text = strings.Join(strings.Fields(text), " ")
					cells = append(cells, text)
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

// --- Utilities ---

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func resolveURL(href string, baseURL *url.URL) string {
	if baseURL == nil {
		return href
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	return baseURL.ResolveReference(parsed).String()
}

// cleanMarkdown collapses excessive blank lines and trims whitespace.
func cleanMarkdown(s string) string {
	// Collapse 3+ newlines into 2.
	prev := ""
	for s != prev {
		prev = s
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}
