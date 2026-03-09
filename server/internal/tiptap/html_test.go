package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderHTML_Paragraph(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<p>Hello world</p>") {
		t.Errorf("expected paragraph, got: %s", got)
	}
}

func TestRenderHTML_Heading(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Getting Started"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<h2 id="getting-started">Getting Started</h2>`) {
		t.Errorf("expected h2 with id, got: %s", got)
	}
}

func TestRenderHTML_BoldItalicStrike(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"bold"}],"text":"bold"},
		{"type":"text","text":" "},
		{"type":"text","marks":[{"type":"italic"}],"text":"italic"},
		{"type":"text","text":" "},
		{"type":"text","marks":[{"type":"strike"}],"text":"strike"}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("missing bold: %s", got)
	}
	if !strings.Contains(got, "<em>italic</em>") {
		t.Errorf("missing italic: %s", got)
	}
	if !strings.Contains(got, "<s>strike</s>") {
		t.Errorf("missing strike: %s", got)
	}
}

func TestRenderHTML_Link(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"link","attrs":{"href":"https://example.com"}}],"text":"click here"}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `href="https://example.com"`) {
		t.Errorf("missing link href: %s", got)
	}
	if !strings.Contains(got, "click here</a>") {
		t.Errorf("missing link text: %s", got)
	}
}

func TestRenderHTML_BulletList(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"bulletList","content":[
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 1"}]}]},
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 2"}]}]}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<li>") {
		t.Errorf("expected list structure, got: %s", got)
	}
	if !strings.Contains(got, "Item 1") || !strings.Contains(got, "Item 2") {
		t.Errorf("missing list items, got: %s", got)
	}
}

func TestRenderHTML_CodeBlock(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println(\"hello\")"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="language-go"`) {
		t.Errorf("missing language class: %s", got)
	}
	if !strings.Contains(got, "fmt.Println") {
		t.Errorf("missing code content: %s", got)
	}
	// Verify HTML escaping in code (Go uses &#34; for quotes)
	if !strings.Contains(got, "&#34;hello&#34;") {
		t.Errorf("code content should be escaped: %s", got)
	}
}

func TestRenderHTML_Image(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://img.example.com/photo.png","alt":"A photo","width":"50%"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="https://img.example.com/photo.png"`) {
		t.Errorf("missing image src: %s", got)
	}
	if !strings.Contains(got, `alt="A photo"`) {
		t.Errorf("missing alt: %s", got)
	}
	if !strings.Contains(got, `style="width:50%"`) {
		t.Errorf("missing width style: %s", got)
	}
}

func TestRenderHTML_Blockquote(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"A wise quote"}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<blockquote>") || !strings.Contains(got, "A wise quote") {
		t.Errorf("expected blockquote, got: %s", got)
	}
}

func TestRenderHTML_HorizontalRule(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"horizontalRule"}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<hr>") {
		t.Errorf("expected hr, got: %s", got)
	}
}

func TestRenderHTML_NestedMarks(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"bold"},{"type":"italic"}],"text":"bold italic"}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<strong><em>bold italic</em></strong>") {
		t.Errorf("expected nested marks, got: %s", got)
	}
}

func TestRenderHTML_Empty(t *testing.T) {
	got, err := RenderHTML(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty string, got: %s", got)
	}
}

func TestRenderHTML_XSSPrevention(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"<script>alert('xss')</script>"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("XSS not prevented: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag, got: %s", got)
	}
}
