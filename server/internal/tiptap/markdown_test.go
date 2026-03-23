package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarkdownToJSON_EmptyInput(t *testing.T) {
	for _, input := range []string{"", "   ", "\n\n"} {
		raw := MarkdownToJSON(input)
		var doc Node
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if doc.Type != "doc" {
			t.Fatalf("expected doc, got %q", doc.Type)
		}
		if len(doc.Content) != 1 || doc.Content[0].Type != "paragraph" {
			t.Fatalf("expected single empty paragraph, got %+v", doc.Content)
		}
	}
}

func TestMarkdownToJSON_PlainParagraph(t *testing.T) {
	raw := MarkdownToJSON("Hello world")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	p := doc.Content[0]
	if p.Type != "paragraph" {
		t.Fatalf("expected paragraph, got %q", p.Type)
	}
	// Goldmark may split text across nodes; concatenate to verify.
	var text string
	for _, n := range p.Content {
		text += n.Text
	}
	if strings.TrimSpace(text) != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", text)
	}
}

func TestMarkdownToJSON_MultipleParagraphs(t *testing.T) {
	raw := MarkdownToJSON("First paragraph\n\nSecond paragraph")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(doc.Content))
	}
	for i, p := range doc.Content {
		if p.Type != "paragraph" {
			t.Fatalf("node %d: expected paragraph, got %q", i, p.Type)
		}
	}
}

func TestMarkdownToJSON_Headings(t *testing.T) {
	md := "# H1\n\n## H2\n\n### H3\n\n#### H4\n\n##### H5\n\n###### H6"
	raw := MarkdownToJSON(md)
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 6 {
		t.Fatalf("expected 6 headings, got %d", len(doc.Content))
	}
	for i, h := range doc.Content {
		if h.Type != "heading" {
			t.Fatalf("node %d: expected heading, got %q", i, h.Type)
		}
		level, _ := h.Attrs["level"].(float64)
		if int(level) != i+1 {
			t.Fatalf("node %d: expected level %d, got %v", i, i+1, h.Attrs["level"])
		}
	}
}

func TestMarkdownToJSON_Bold(t *testing.T) {
	raw := MarkdownToJSON("This is **bold** text")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := doc.Content[0]
	if len(p.Content) != 3 {
		t.Fatalf("expected 3 text nodes, got %d: %+v", len(p.Content), p.Content)
	}
	bold := p.Content[1]
	if bold.Text != "bold" {
		t.Fatalf("expected 'bold', got %q", bold.Text)
	}
	if len(bold.Marks) != 1 || bold.Marks[0].Type != "bold" {
		t.Fatalf("expected bold mark, got %+v", bold.Marks)
	}
}

func TestMarkdownToJSON_Italic(t *testing.T) {
	raw := MarkdownToJSON("This is *italic* text")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := doc.Content[0]
	found := false
	for _, n := range p.Content {
		if n.Text == "italic" && len(n.Marks) == 1 && n.Marks[0].Type == "italic" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected italic text node, got %+v", p.Content)
	}
}

func TestMarkdownToJSON_BoldItalic(t *testing.T) {
	raw := MarkdownToJSON("This is ***bold and italic*** text")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := doc.Content[0]
	found := false
	for _, n := range p.Content {
		if n.Text == "bold and italic" {
			hasBold := false
			hasItalic := false
			for _, m := range n.Marks {
				if m.Type == "bold" {
					hasBold = true
				}
				if m.Type == "italic" {
					hasItalic = true
				}
			}
			if hasBold && hasItalic {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected bold+italic text node, got %+v", p.Content)
	}
}

func TestMarkdownToJSON_InlineCode(t *testing.T) {
	raw := MarkdownToJSON("Use `fmt.Println` here")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := doc.Content[0]
	found := false
	for _, n := range p.Content {
		if n.Text == "fmt.Println" && len(n.Marks) == 1 && n.Marks[0].Type == "code" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected code-marked text node, got %+v", p.Content)
	}
}

func TestMarkdownToJSON_Strikethrough(t *testing.T) {
	raw := MarkdownToJSON("This is ~~deleted~~ text")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := doc.Content[0]
	found := false
	for _, n := range p.Content {
		if n.Text == "deleted" && len(n.Marks) == 1 && n.Marks[0].Type == "strike" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected strike-marked text node, got %+v", p.Content)
	}
}

func TestMarkdownToJSON_Link(t *testing.T) {
	raw := MarkdownToJSON("Visit [Google](https://google.com) now")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := doc.Content[0]
	found := false
	for _, n := range p.Content {
		if n.Text == "Google" && len(n.Marks) == 1 && n.Marks[0].Type == "link" {
			href, _ := n.Marks[0].Attrs["href"].(string)
			if href == "https://google.com" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected link text node, got %+v", p.Content)
	}
}

func TestMarkdownToJSON_BulletList(t *testing.T) {
	raw := MarkdownToJSON("- first\n- second\n- third")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	list := doc.Content[0]
	if list.Type != "bulletList" {
		t.Fatalf("expected bulletList, got %q", list.Type)
	}
	if len(list.Content) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list.Content))
	}
	expected := []string{"first", "second", "third"}
	for i, item := range list.Content {
		if item.Type != "listItem" {
			t.Fatalf("item %d: expected listItem, got %q", i, item.Type)
		}
		text := extractNodeText(item)
		if strings.TrimSpace(text) != expected[i] {
			t.Fatalf("item %d: expected %q, got %q", i, expected[i], text)
		}
	}
}

func TestMarkdownToJSON_OrderedList(t *testing.T) {
	raw := MarkdownToJSON("1. first\n2. second\n3. third")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	list := doc.Content[0]
	if list.Type != "orderedList" {
		t.Fatalf("expected orderedList, got %q", list.Type)
	}
	if len(list.Content) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list.Content))
	}
	expected := []string{"first", "second", "third"}
	for i, item := range list.Content {
		text := extractNodeText(item)
		if strings.TrimSpace(text) != expected[i] {
			t.Fatalf("item %d: expected %q, got %q", i, expected[i], text)
		}
	}
}

func TestMarkdownToJSON_FencedCodeBlock(t *testing.T) {
	md := "```go\nfmt.Println(\"hello\")\n```"
	raw := MarkdownToJSON(md)
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	cb := doc.Content[0]
	if cb.Type != "codeBlock" {
		t.Fatalf("expected codeBlock, got %q", cb.Type)
	}
	lang, _ := cb.Attrs["language"].(string)
	if lang != "go" {
		t.Fatalf("expected language 'go', got %q", lang)
	}
	if len(cb.Content) != 1 || !strings.Contains(cb.Content[0].Text, "fmt.Println") {
		t.Fatalf("unexpected code content: %+v", cb.Content)
	}
}

func TestMarkdownToJSON_Blockquote(t *testing.T) {
	raw := MarkdownToJSON("> This is a quote")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	bq := doc.Content[0]
	if bq.Type != "blockquote" {
		t.Fatalf("expected blockquote, got %q", bq.Type)
	}
	if len(bq.Content) == 0 || bq.Content[0].Type != "paragraph" {
		t.Fatalf("expected paragraph inside blockquote, got %+v", bq.Content)
	}
}

func TestMarkdownToJSON_HorizontalRule(t *testing.T) {
	raw := MarkdownToJSON("Before\n\n---\n\nAfter")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// paragraph, horizontalRule, paragraph
	if len(doc.Content) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(doc.Content))
	}
	if doc.Content[1].Type != "horizontalRule" {
		t.Fatalf("expected horizontalRule, got %q", doc.Content[1].Type)
	}
}

func TestMarkdownToJSON_Table(t *testing.T) {
	md := "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n| Bob | 25 |"
	raw := MarkdownToJSON(md)
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	tbl := doc.Content[0]
	if tbl.Type != "table" {
		t.Fatalf("expected table, got %q", tbl.Type)
	}
	if len(tbl.Content) != 3 {
		t.Fatalf("expected 3 rows (1 header + 2 body), got %d", len(tbl.Content))
	}
	// First row should have tableHeader cells.
	headerRow := tbl.Content[0]
	if headerRow.Type != "tableRow" {
		t.Fatalf("expected tableRow, got %q", headerRow.Type)
	}
	if len(headerRow.Content) != 2 {
		t.Fatalf("expected 2 cells in header, got %d", len(headerRow.Content))
	}
	if headerRow.Content[0].Type != "tableHeader" {
		t.Fatalf("expected tableHeader cell, got %q", headerRow.Content[0].Type)
	}
	// Body rows should have tableCell.
	bodyRow := tbl.Content[1]
	if bodyRow.Content[0].Type != "tableCell" {
		t.Fatalf("expected tableCell, got %q", bodyRow.Content[0].Type)
	}
}

func TestMarkdownToJSON_MixedContent(t *testing.T) {
	md := "# Title\n\nA paragraph with **bold** text.\n\n- item one\n- item two\n\n```\ncode here\n```"
	raw := MarkdownToJSON(md)
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	types := make([]string, len(doc.Content))
	for i, n := range doc.Content {
		types[i] = n.Type
	}
	expected := []string{"heading", "paragraph", "bulletList", "codeBlock"}
	if len(types) != len(expected) {
		t.Fatalf("expected types %v, got %v", expected, types)
	}
	for i := range expected {
		if types[i] != expected[i] {
			t.Fatalf("node %d: expected %q, got %q", i, expected[i], types[i])
		}
	}
}

func TestMarkdownToJSON_PreservesHeadingsAndBullets(t *testing.T) {
	// This mirrors the existing test from activities_test.go.
	raw := MarkdownToJSON("# Problem\n\n- first item\n- second item\n\nPlain paragraph")
	var doc struct {
		Type    string                   `json:"type"`
		Content []map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Type != "doc" {
		t.Fatalf("expected root type doc, got %q", doc.Type)
	}
	if len(doc.Content) < 3 {
		t.Fatalf("expected heading, list, and paragraph nodes, got %d nodes", len(doc.Content))
	}
	if doc.Content[0]["type"] != "heading" {
		t.Fatalf("expected first node to be heading, got %v", doc.Content[0]["type"])
	}
	if doc.Content[1]["type"] != "bulletList" {
		t.Fatalf("expected second node to be bulletList, got %v", doc.Content[1]["type"])
	}
	if doc.Content[2]["type"] != "paragraph" {
		t.Fatalf("expected third node to be paragraph, got %v", doc.Content[2]["type"])
	}
}

func TestMarkdownToJSON_RoundTrip(t *testing.T) {
	md := "# Hello\n\nA **bold** paragraph with *italic* and `code`.\n\n- item 1\n- item 2\n\n> blockquote\n\n---\n\n| Col A | Col B |\n| --- | --- |\n| 1 | 2 |"
	raw := MarkdownToJSON(md)

	// Verify the JSON can be rendered to HTML by the existing renderer.
	html, err := RenderHTML(raw)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if html == "" {
		t.Fatal("RenderHTML returned empty string")
	}
	// Basic sanity checks on the HTML output.
	for _, fragment := range []string{"<h1", "<strong>bold</strong>", "<em>italic</em>", "<code>code</code>", "<ul>", "<li>", "<blockquote>", "<hr>", "<table>"} {
		if !strings.Contains(html, fragment) {
			t.Errorf("expected HTML to contain %q, got:\n%s", fragment, html)
		}
	}
}

func TestMarkdownToJSON_Image(t *testing.T) {
	raw := MarkdownToJSON("![alt text](https://example.com/image.png)")
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Content))
	}
	p := doc.Content[0]
	found := false
	for _, n := range p.Content {
		if n.Type == "image" {
			src, _ := n.Attrs["src"].(string)
			alt, _ := n.Attrs["alt"].(string)
			if src == "https://example.com/image.png" && alt == "alt text" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected image node with correct src/alt, got %+v", p.Content)
	}
}

func TestMarkdownToJSON_TightListPreservesText(t *testing.T) {
	// Tight lists (no blank lines between items) use TextBlock in goldmark.
	md := "## Goals\n\n- Migrate all Kafka workloads to NATS\n- Achieve lower latency (<10ms p99)\n- Maintain system stability during migration"
	raw := MarkdownToJSON(md)
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) < 2 {
		t.Fatalf("expected heading + list, got %d nodes", len(doc.Content))
	}
	list := doc.Content[1]
	if list.Type != "bulletList" {
		t.Fatalf("expected bulletList, got %q", list.Type)
	}
	if len(list.Content) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list.Content))
	}
	for i, item := range list.Content {
		text := extractNodeText(item)
		if strings.TrimSpace(text) == "" {
			t.Fatalf("item %d: expected non-empty text, got empty", i)
		}
	}
	// Verify specific content.
	first := extractNodeText(list.Content[0])
	if !strings.Contains(first, "Migrate") {
		t.Fatalf("first item should contain 'Migrate', got %q", first)
	}
}

// extractNodeText recursively extracts all text from a tiptap Node tree.
func extractNodeText(n Node) string {
	var buf strings.Builder
	buf.WriteString(n.Text)
	for _, child := range n.Content {
		buf.WriteString(extractNodeText(child))
	}
	return buf.String()
}
