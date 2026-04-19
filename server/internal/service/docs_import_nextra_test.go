package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNextraContentToTiptapJSON_ConvertsHTMLImages(t *testing.T) {
	raw := nextraContentToTiptapJSON(`<img src="https://cdn.example.com/hero.png" alt="Hero" />`)

	var doc struct {
		Type    string `json:"type"`
		Content []struct {
			Type  string `json:"type"`
			Attrs struct {
				Src string `json:"src"`
				Alt string `json:"alt"`
			} `json:"attrs"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal tiptap: %v", err)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("content count = %d, want 1", len(doc.Content))
	}
	if doc.Content[0].Type != "resizableImage" {
		t.Fatalf("node type = %q, want resizableImage", doc.Content[0].Type)
	}
	if doc.Content[0].Attrs.Src != "https://cdn.example.com/hero.png" {
		t.Fatalf("src = %q", doc.Content[0].Attrs.Src)
	}
	if doc.Content[0].Attrs.Alt != "Hero" {
		t.Fatalf("alt = %q", doc.Content[0].Attrs.Alt)
	}
}

func TestRewriteNextraImportedContent_RewritesImagesAndLinks(t *testing.T) {
	content := strings.Join([]string{
		"![Hero](guides/images/hero.png)",
		`<img src="public/logo.png" alt="Logo" />`,
		"[Install](/guides/install#step-one)",
	}, "\n\n")

	rewritten := rewriteNextraImportedContent(content, map[string]string{
		"guides/images/hero.png": "https://cdn.example.com/hero.png",
		"public/logo.png":        "https://cdn.example.com/logo.png",
	}, map[string]string{
		"/guides/install": "/articles/install-abcd1234",
	})

	if !strings.Contains(rewritten, "![Hero](https://cdn.example.com/hero.png)") {
		t.Fatalf("rewritten missing markdown image URL: %s", rewritten)
	}
	if !strings.Contains(rewritten, `<img src="https://cdn.example.com/logo.png" alt="Logo" />`) {
		t.Fatalf("rewritten missing HTML image URL: %s", rewritten)
	}
	if !strings.Contains(rewritten, "[Install](/articles/install-abcd1234#step-one)") {
		t.Fatalf("rewritten missing canonical link: %s", rewritten)
	}
}
