// Package tiptap renders TipTap/ProseMirror JSON documents to HTML.
package tiptap

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
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
		if len(n.Content) == 0 {
			b.WriteString("<br>")
		} else {
			renderChildren(b, n)
		}
		b.WriteString("</p>\n")

	case "heading":
		level := intAttr(n.Attrs, "level", 2)
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		id := strAttr(n.Attrs, "id")
		if id == "" {
			id = headingID(n)
		}
		for _, alias := range stringSliceAttr(n.Attrs, "anchorAliases") {
			if alias == "" || alias == id {
				continue
			}
			b.WriteString(`<span id="`)
			b.WriteString(html.EscapeString(alias))
			b.WriteString(`" class="docs-heading-anchor-alias" aria-hidden="true"></span>`)
		}
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
		assignee := strAttr(n.Attrs, "assigneeName")
		dueDate := strAttr(n.Attrs, "dueDate")
		pmTaskID := strAttr(n.Attrs, "pmTaskId")
		taskKey := strAttr(n.Attrs, "taskKey")
		b.WriteString(`<li class="task-item"`)
		if checked {
			b.WriteString(` data-checked="true"`)
		}
		if assignee != "" {
			b.WriteString(` data-assignee-name="`)
			b.WriteString(html.EscapeString(assignee))
			b.WriteString(`"`)
		}
		if dueDate != "" {
			b.WriteString(` data-due-date="`)
			b.WriteString(html.EscapeString(dueDate))
			b.WriteString(`"`)
		}
		if pmTaskID != "" {
			b.WriteString(` data-pm-task-id="`)
			b.WriteString(html.EscapeString(pmTaskID))
			b.WriteString(`"`)
		}
		if taskKey != "" {
			b.WriteString(` data-task-key="`)
			b.WriteString(html.EscapeString(taskKey))
			b.WriteString(`"`)
		}
		b.WriteString(`>`)
		if checked {
			b.WriteString(`<input type="checkbox" checked disabled> `)
		} else {
			b.WriteString(`<input type="checkbox" disabled> `)
		}
		renderChildren(b, n)
		if assignee != "" || dueDate != "" || taskKey != "" {
			b.WriteString(`<span class="task-item-meta">`)
			b.WriteString(html.EscapeString(strings.TrimSpace(assignee + " " + dueDate + " " + taskKey)))
			b.WriteString(`</span>`)
		}
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

	case "callout":
		variant := normalizeCalloutVariant(strAttr(n.Attrs, "variant"))
		fmt.Fprintf(b, "<aside class=\"docs-callout docs-callout--%s\" data-callout-variant=\"%s\">\n", variant, variant)
		renderChildren(b, n)
		b.WriteString("</aside>\n")

	case "toggleSection":
		title := strAttr(n.Attrs, "title")
		if title == "" {
			title = "Details"
		}
		icon := strAttr(n.Attrs, "icon")
		badge := strAttr(n.Attrs, "badgeText")
		sourceStyle := strAttr(n.Attrs, "sourceStyle")
		b.WriteString(`<details class="docs-toggle-section" data-toggle-section`)
		if boolAttr(n.Attrs, "open") {
			b.WriteString(` open`)
		}
		b.WriteString(` data-toggle-title="`)
		b.WriteString(html.EscapeString(title))
		b.WriteByte('"')
		if icon != "" {
			b.WriteString(` data-toggle-icon="`)
			b.WriteString(html.EscapeString(icon))
			b.WriteByte('"')
		}
		if badge != "" {
			b.WriteString(` data-toggle-badge="`)
			b.WriteString(html.EscapeString(badge))
			b.WriteByte('"')
		}
		if sourceStyle != "" {
			b.WriteString(` data-toggle-style="`)
			b.WriteString(html.EscapeString(sourceStyle))
			b.WriteByte('"')
		}
		b.WriteString(`>`)
		if icon != "" || badge != "" || sourceStyle != "" {
			b.WriteString(`<summary>`)
			if icon != "" {
				b.WriteString(`<span class="docs-toggle-icon">`)
				b.WriteString(html.EscapeString(icon))
				b.WriteString(`</span>`)
			}
			b.WriteString(`<span class="docs-toggle-title">`)
			b.WriteString(html.EscapeString(title))
			b.WriteString(`</span>`)
			if badge != "" {
				b.WriteString(`<span class="docs-toggle-badge">`)
				b.WriteString(html.EscapeString(badge))
				b.WriteString(`</span>`)
			}
			b.WriteString(`</summary>`)
		} else {
			b.WriteString(`<summary>`)
			b.WriteString(html.EscapeString(title))
			b.WriteString(`</summary>`)
		}
		b.WriteString("\n")
		renderChildren(b, n)
		b.WriteString("</details>\n")

	case "aiSection":
		renderChildren(b, n)

	case "citationBlock":
		renderCitationBlock(b, n)

	case "entityEmbed":
		entityType := strAttr(n.Attrs, "entityType")
		entityID := strAttr(n.Attrs, "entityId")
		switch entityType {
		case "task", "story", "epic", "support_conversation", "deal", "contact", "company", "reference":
		default:
			entityType = "task"
		}
		title := strAttr(n.Attrs, "title")
		if title == "" {
			title = "Linked entity"
		}
		displayID := strAttr(n.Attrs, "displayId")
		status := strAttr(n.Attrs, "status")
		b.WriteString("<div class=\"docs-entity-embed\" data-entity-embed data-entity-type=\"")
		b.WriteString(html.EscapeString(entityType))
		b.WriteString("\" data-entity-id=\"")
		b.WriteString(html.EscapeString(entityID))
		b.WriteString("\" data-entity-title=\"")
		b.WriteString(html.EscapeString(title))
		if displayID != "" {
			b.WriteString("\" data-entity-display-id=\"")
			b.WriteString(html.EscapeString(displayID))
		}
		if status != "" {
			b.WriteString("\" data-entity-status=\"")
			b.WriteString(html.EscapeString(status))
		}
		b.WriteString("\">")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</div>\n")

	case "savedViewEmbed":
		module := strAttr(n.Attrs, "module")
		if module == "" {
			module = "pm"
		}
		viewID := strAttr(n.Attrs, "viewId")
		viewName := strAttr(n.Attrs, "viewName")
		if viewName == "" {
			viewName = "Saved view"
		}
		b.WriteString("<section class=\"docs-saved-view-embed\" data-saved-view-embed data-saved-view-module=\"")
		b.WriteString(html.EscapeString(module))
		if viewID != "" {
			b.WriteString("\" data-saved-view-id=\"")
			b.WriteString(html.EscapeString(viewID))
		}
		b.WriteString("\" data-saved-view-name=\"")
		b.WriteString(html.EscapeString(viewName))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(viewName))
		b.WriteString("</section>\n")

	case "videoEmbed":
		embedUrl := strAttr(n.Attrs, "embedUrl")
		provider := strAttr(n.Attrs, "provider")
		if embedUrl != "" {
			// Parse URL and validate scheme + host to prevent injection
			allowed := isAllowedVideoEmbed(embedUrl)
			if allowed {
				b.WriteString("<div class=\"docs-video-embed\" data-video-provider=\"")
				b.WriteString(html.EscapeString(provider))
				b.WriteString("\">\n")
				b.WriteString("<iframe src=\"")
				b.WriteString(html.EscapeString(embedUrl))
				b.WriteString("\" frameborder=\"0\" allowfullscreen sandbox=\"allow-scripts allow-same-origin allow-popups allow-presentation\" referrerpolicy=\"strict-origin-when-cross-origin\" loading=\"lazy\"></iframe>\n")
				b.WriteString("</div>\n")
			}
		}

	case "artifactVideo":
		fileName := strAttr(n.Attrs, "fileName")
		if fileName == "" {
			fileName = "Private video recording"
		}
		description := strAttr(n.Attrs, "description")
		caption := strAttr(n.Attrs, "caption")
		b.WriteString(`<figure class="docs-artifact-video" data-private-artifact-video>`)
		b.WriteString(`<div class="docs-artifact-video-placeholder">`)
		b.WriteString(html.EscapeString(fileName))
		if description != "" {
			b.WriteString(`<span class="sr-only"> — `)
			b.WriteString(html.EscapeString(description))
			b.WriteString(`</span>`)
		}
		b.WriteString(`</div>`)
		if caption != "" {
			b.WriteString(`<figcaption>`)
			b.WriteString(html.EscapeString(caption))
			b.WriteString(`</figcaption>`)
		}
		b.WriteString("</figure>\n")

	case "htmlBlock":
		rawHTML := strAttr(n.Attrs, "html")
		if rawHTML != "" {
			if shouldRenderHTMLBlockIsolated(rawHTML, strAttr(n.Attrs, "renderMode")) {
				b.WriteString(renderIsolatedHTMLBlock(rawHTML))
				break
			}
			sanitized := SanitizeHTMLBlock(rawHTML)
			if sanitized != "" {
				b.WriteString("<div class=\"docs-html-block\">\n")
				b.WriteString(sanitized)
				b.WriteString("\n</div>\n")
			}
		}

	case "excalidraw":
		title := strAttr(n.Attrs, "title")
		if title == "" {
			title = "Excalidraw drawing"
		}
		b.WriteString("<figure class=\"docs-excalidraw-block\" data-excalidraw>\n")
		b.WriteString("<div class=\"docs-excalidraw-placeholder\">")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</div>\n")
		b.WriteString("<figcaption>")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</figcaption>\n")
		b.WriteString("</figure>\n")

	case "horizontalRule":
		b.WriteString("<hr>\n")

	case "hardBreak":
		b.WriteString("<br>")

	case "image", "resizableImage":
		alignment := strAttr(n.Attrs, "alignment")
		linkUrl := strAttr(n.Attrs, "linkUrl")
		caption := strAttr(n.Attrs, "caption")
		linkNewTab := true
		if v, ok := n.Attrs["linkNewTab"]; ok {
			if bv, ok := v.(bool); ok {
				linkNewTab = bv
			}
		}

		if caption != "" {
			b.WriteString("<figure class=\"docs-image-figure\">\n")
		}
		if alignment != "" && alignment != "center" {
			var alignStyle string
			switch alignment {
			case "left":
				alignStyle = "text-align:left"
			case "right":
				alignStyle = "text-align:right"
			}
			fmt.Fprintf(b, "<div class=\"docs-image-block\" style=\"%s\">\n", alignStyle)
		} else if alignment == "" || alignment == "center" {
			b.WriteString("<div class=\"docs-image-block\" style=\"text-align:center\">\n")
		}

		if linkUrl != "" {
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(linkUrl))
			b.WriteByte('"')
			if linkNewTab {
				b.WriteString(` target="_blank" rel="noopener noreferrer"`)
			}
			b.WriteByte('>')
		}

		renderImage(b, n)

		if linkUrl != "" {
			b.WriteString("</a>")
		}
		if caption != "" {
			b.WriteString("\n<figcaption>")
			b.WriteString(html.EscapeString(caption))
			b.WriteString("</figcaption>")
		}
		b.WriteString("\n</div>\n")
		if caption != "" {
			b.WriteString("</figure>\n")
		}

	case "fileAttachment":
		fileName := strAttr(n.Attrs, "fileName")
		if fileName == "" {
			fileName = "Attachment"
		}
		url := strAttr(n.Attrs, "url")
		contentType := strAttr(n.Attrs, "contentType")
		b.WriteString(`<div class="docs-file-attachment" data-file-attachment`)
		if contentType != "" {
			b.WriteString(` data-content-type="`)
			b.WriteString(html.EscapeString(contentType))
			b.WriteString(`"`)
		}
		b.WriteString(`>`)
		if url != "" {
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(url))
			b.WriteString(`" target="_blank" rel="noopener noreferrer">`)
			b.WriteString(html.EscapeString(fileName))
			b.WriteString(`</a>`)
		} else {
			b.WriteString(html.EscapeString(fileName))
		}
		b.WriteString("</div>\n")

	case "tableOfContents":
		b.WriteString(`<nav data-docs-toc>Table of contents</nav>`)
		b.WriteString("\n")

	case "richEmbed":
		url := strAttr(n.Attrs, "url")
		title := strAttr(n.Attrs, "title")
		if title == "" {
			title = url
		}
		provider := strAttr(n.Attrs, "provider")
		description := strAttr(n.Attrs, "description")
		imageURL := strAttr(n.Attrs, "image_url")
		b.WriteString(`<a class="docs-rich-embed" data-rich-embed href="`)
		b.WriteString(html.EscapeString(url))
		b.WriteString(`" data-embed-url="`)
		b.WriteString(html.EscapeString(url))
		if provider != "" {
			b.WriteString(`" data-embed-provider="`)
			b.WriteString(html.EscapeString(provider))
		}
		if title != "" {
			b.WriteString(`" data-embed-title="`)
			b.WriteString(html.EscapeString(title))
		}
		if description != "" {
			b.WriteString(`" data-embed-description="`)
			b.WriteString(html.EscapeString(description))
		}
		if imageURL != "" {
			b.WriteString(`" data-embed-image-url="`)
			b.WriteString(html.EscapeString(imageURL))
		}
		b.WriteString(`" target="_blank" rel="noopener noreferrer">`)
		if provider != "" {
			b.WriteString(html.EscapeString(provider))
			b.WriteString(": ")
		}
		b.WriteString(html.EscapeString(title))
		b.WriteString("</a>\n")

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
	darkSrc := strAttr(n.Attrs, "darkSrc")
	alt := strAttr(n.Attrs, "alt")
	width := strAttr(n.Attrs, "width")
	height := strAttr(n.Attrs, "height")

	var style []string
	if width != "" && width != "auto" {
		style = append(style, "width:"+html.EscapeString(width))
	}
	if height != "" && height != "auto" {
		style = append(style, "height:"+html.EscapeString(height))
	}

	if darkSrc != "" {
		b.WriteString(`<span class="docs-theme-image-set">`)
		renderImageTag(b, src, alt, style, "docs-theme-image docs-theme-image-light")
		renderImageTag(b, darkSrc, alt, style, "docs-theme-image docs-theme-image-dark")
		b.WriteString(`</span>`)
		return
	}

	renderImageTag(b, src, alt, style, "")
}

func renderImageTag(b *strings.Builder, src string, alt string, style []string, className string) {
	b.WriteString(`<img`)
	if className != "" {
		b.WriteString(` class="`)
		b.WriteString(html.EscapeString(className))
		b.WriteByte('"')
	}
	b.WriteString(` src="`)
	b.WriteString(html.EscapeString(src))
	b.WriteString(`" alt="`)
	b.WriteString(html.EscapeString(alt))
	b.WriteByte('"')
	if len(style) > 0 {
		fmt.Fprintf(b, ` style="%s"`, strings.Join(style, ";"))
	}
	b.WriteString(" loading=\"lazy\">")
}

func renderCitationBlock(b *strings.Builder, n *Node) {
	title := strAttr(n.Attrs, "title")
	if title == "" {
		title = "Sources"
	}
	sources := sourceListAttr(n.Attrs, "sources")

	b.WriteString("<section class=\"docs-citation-block\" data-citation-block data-citation-title=\"")
	b.WriteString(html.EscapeString(title))
	b.WriteString("\">\n")
	b.WriteString("<h3>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h3>\n")
	if len(sources) == 0 {
		b.WriteString("<p>No sources attached</p>\n")
		b.WriteString("</section>\n")
		return
	}

	b.WriteString("<ol>\n")
	for _, source := range sources {
		sourceType := citationSourceType(strMapAttr(source, "sourceType"))
		sourceID := strMapAttr(source, "sourceId")
		access := strMapAttr(source, "access")
		if access == "" {
			access = "unknown"
		}
		redacted := access == "redacted"
		title := strMapAttr(source, "title")
		if title == "" {
			title = citationSourceLabel(sourceType)
		}
		if redacted {
			title = "Restricted source"
		}

		b.WriteString("<li data-source-type=\"")
		b.WriteString(html.EscapeString(sourceType))
		b.WriteString("\" data-source-id=\"")
		b.WriteString(html.EscapeString(sourceID))
		b.WriteString("\" data-source-access=\"")
		b.WriteString(html.EscapeString(access))
		b.WriteString("\">")

		url := strMapAttr(source, "url")
		if url != "" && !redacted {
			b.WriteString("<a href=\"")
			b.WriteString(html.EscapeString(url))
			b.WriteString("\" target=\"_blank\" rel=\"noopener noreferrer\">")
			b.WriteString(html.EscapeString(title))
			b.WriteString("</a>")
		} else {
			b.WriteString("<span>")
			b.WriteString(html.EscapeString(title))
			b.WriteString("</span>")
		}
		b.WriteString(" <small>")
		b.WriteString(html.EscapeString(citationSourceLabel(sourceType)))
		if confidence, ok := numericMapAttr(source, "confidence"); ok && !redacted {
			if confidence <= 1 {
				confidence *= 100
			}
			fmt.Fprintf(b, " %.0f%%", confidence)
		}
		b.WriteString("</small>")

		if redacted {
			b.WriteString("<p>Hidden because this viewer cannot access the underlying source.</p>")
		} else if excerpt := strMapAttr(source, "excerpt"); excerpt != "" {
			b.WriteString("<p>")
			b.WriteString(html.EscapeString(excerpt))
			b.WriteString("</p>")
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ol>\n</section>\n")
}

// allowedVideoHosts maps hostnames to required path prefixes for video embeds.
var allowedVideoHosts = map[string]string{
	"www.youtube.com":  "/embed/",
	"player.vimeo.com": "/video/",
	"www.loom.com":     "/embed/",
	"fast.wistia.net":  "/embed/iframe/",
}

// isAllowedVideoEmbed validates that the embed URL uses https and matches
// a whitelisted host + path prefix. This prevents injection via crafted URLs
// like https://attacker.example/?next=youtube.com/embed/abc.
func isAllowedVideoEmbed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	prefix, ok := allowedVideoHosts[u.Host]
	if !ok {
		return false
	}
	return strings.HasPrefix(u.Path, prefix)
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

func stringSliceAttr(attrs map[string]any, key string) []string {
	if attrs == nil {
		return nil
	}
	v, ok := attrs[key]
	if !ok {
		return nil
	}
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeCalloutVariant(value string) string {
	switch value {
	case "blue", "info":
		return "info"
	case "yellow", "warning":
		return "warning"
	case "green", "tip":
		return "tip"
	case "red", "danger":
		return "danger"
	case "success":
		return "success"
	case "grey":
		return "info"
	default:
		return "info"
	}
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

func sourceListAttr(attrs map[string]any, key string) []map[string]any {
	if attrs == nil {
		return nil
	}
	raw, ok := attrs[key]
	if !ok || raw == nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func strMapAttr(attrs map[string]any, key string) string {
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

func numericMapAttr(attrs map[string]any, key string) (float64, bool) {
	if attrs == nil {
		return 0, false
	}
	v, ok := attrs[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func citationSourceType(value string) string {
	switch value {
	case "docs_chunk", "support_conversation":
		return value
	default:
		return "docs_chunk"
	}
}

func citationSourceLabel(sourceType string) string {
	switch sourceType {
	case "support_conversation":
		return "Support conversation"
	case "docs_chunk":
		return "Docs chunk"
	default:
		return "Source"
	}
}
