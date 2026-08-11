package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeInternalAnchorLinksRewritesImportedHelpScoutFragments(t *testing.T) {
	doc := Node{
		Type: "doc",
		Content: []Node{
			{Type: "paragraph", Content: []Node{
				linkedText("How to Set Up Your Brand Knowledge", "#How-to-Set-Up-Your-Brand-Knowledge-hBibq"),
				linkedText("Using Your Brand Knowledge in AI Studio", "#Using-Your-Brand-Knowledge-in-AI-Studio-sPXEz"),
				linkedText("FAQs", "#FAQs-VE-7D"),
			}},
			heading("How to Set Up Your Brand Knowledge"),
			heading("Using Your Brand Knowledge in AI Studio"),
			heading("FAQs"),
		},
	}

	NormalizeInternalAnchorLinks(&doc)

	want := []string{
		"#how-to-set-up-your-brand-knowledge",
		"#using-your-brand-knowledge-in-ai-studio",
		"#faqs",
	}
	for i, textNode := range doc.Content[0].Content {
		attrs := textNode.Marks[0].Attrs
		if got := attrs["href"]; got != want[i] {
			t.Fatalf("link %d href = %v, want %q", i, got, want[i])
		}
		if _, ok := attrs["target"]; ok {
			t.Fatalf("link %d retained target", i)
		}
		if _, ok := attrs["rel"]; ok {
			t.Fatalf("link %d retained rel", i)
		}
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, `target="_blank"`) {
		t.Fatalf("internal links must not open a new page: %s", html)
	}
}

func TestRenderHTMLNormalizesExistingImportedAnchorSnapshot(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"FAQs","marks":[{"type":"link","attrs":{"href":"#FAQs-VE-7D","target":"_blank","rel":"noopener noreferrer"}}]}]},{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"FAQs"}]}]}`)

	html, err := RenderHTML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `href="#faqs"`) {
		t.Fatalf("existing imported anchor was not normalized: %s", html)
	}
	if strings.Contains(html, `target="_blank"`) {
		t.Fatalf("existing internal anchor opens a new page: %s", html)
	}
}

func TestNormalizeInternalAnchorLinksLeavesAmbiguousTargetUnchanged(t *testing.T) {
	doc := Node{
		Type: "doc",
		Content: []Node{
			{Type: "paragraph", Content: []Node{linkedText("FAQs", "#FAQs-imported")}},
			heading("FAQs"),
			heading("FAQs"),
		},
	}

	NormalizeInternalAnchorLinks(&doc)

	attrs := doc.Content[0].Content[0].Marks[0].Attrs
	if got := attrs["href"]; got != "#FAQs-imported" {
		t.Fatalf("ambiguous href = %v, want original", got)
	}
	if _, ok := attrs["target"]; ok {
		t.Fatal("internal link retained target")
	}
}

func linkedText(text, href string) Node {
	return Node{Type: "text", Text: text, Marks: []Mark{{Type: "link", Attrs: map[string]any{
		"href": href, "target": "_blank", "rel": "noopener noreferrer",
	}}}}
}

func heading(text string) Node {
	return Node{Type: "heading", Attrs: map[string]any{"level": 2}, Content: []Node{{Type: "text", Text: text}}}
}
