package docsimport

import (
	"testing"
)

func TestNextraMeta_JSONObjectOrder(t *testing.T) {
	data := []byte(`{
		"index": "Introduction",
		"getting-started": "Getting Started",
		"guides": "Guides",
		"api": "API Reference"
	}`)
	entries, warnings := ParseNextraMeta("_meta.json", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	expected := []string{"index", "getting-started", "guides", "api"}
	for i, key := range expected {
		if entries[i].Key != key {
			t.Errorf("entry %d: expected key %q, got %q", i, key, entries[i].Key)
		}
	}
}

func TestNextraMeta_JSExportDefault(t *testing.T) {
	data := []byte(`export default {
		"index": "Introduction",
		"guide": "Guide"
	}`)
	entries, warnings := ParseNextraMeta("_meta.js", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "index" || entries[0].Title != "Introduction" {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
}

func TestNextraMeta_TSExportDefaultAsConst(t *testing.T) {
	data := []byte(`export default {
		"index": "Introduction",
		"guide": { "title": "Guide", "display": "hidden" }
	} as const`)
	entries, warnings := ParseNextraMeta("_meta.ts", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].Key != "guide" || entries[1].Title != "Guide" || entries[1].Display != "hidden" {
		t.Errorf("unexpected guide entry: %+v", entries[1])
	}
}

func TestNextraMeta_StringValues(t *testing.T) {
	data := []byte(`{"index": "Introduction", "about": "About Us"}`)
	entries, _ := ParseNextraMeta("_meta.json", data)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "Introduction" {
		t.Errorf("expected title 'Introduction', got %q", entries[0].Title)
	}
	if entries[1].Title != "About Us" {
		t.Errorf("expected title 'About Us', got %q", entries[1].Title)
	}
}

func TestNextraMeta_ObjectValues(t *testing.T) {
	data := []byte(`{
		"guide": { "title": "Guide", "display": "hidden" },
		"external": { "title": "External", "href": "https://example.com", "type": "page" },
		"themed": { "title": "Themed", "theme": { "sidebar": false } }
	}`)
	entries, warnings := ParseNextraMeta("_meta.json", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	guide := entries[0]
	if guide.Title != "Guide" || guide.Display != "hidden" {
		t.Errorf("guide: %+v", guide)
	}

	ext := entries[1]
	if ext.Title != "External" || ext.Href != "https://example.com" || ext.Type != "page" {
		t.Errorf("external: %+v", ext)
	}

	themed := entries[2]
	if themed.Title != "Themed" {
		t.Errorf("themed title: %q", themed.Title)
	}
	if themed.Theme == nil {
		t.Error("themed.Theme should not be nil")
	}
}

func TestNextraMeta_TypePage(t *testing.T) {
	data := []byte(`{
		"index": "Home",
		"docs": { "title": "Docs", "type": "page" },
		"blog": { "title": "Blog", "type": "page" }
	}`)
	entries, _ := ParseNextraMeta("_meta.json", data)
	if entries[1].Type != "page" {
		t.Errorf("expected type 'page', got %q", entries[1].Type)
	}
}

func TestNextraMeta_DynamicValueProducesWarning(t *testing.T) {
	data := []byte(`export default {
		index: "Home",
		guide: () => ({ title: "Dynamic" })
	}`)
	entries, warnings := ParseNextraMeta("_meta.js", data)
	// Should still parse what it can and warn about the rest.
	_ = entries
	foundWarning := false
	for _, w := range warnings {
		if w.Type == "meta_parse_fallback" || w.Type == "meta_parse_error" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning for dynamic meta value")
	}
}

func TestNextraMeta_SatisfiesMeta(t *testing.T) {
	data := []byte(`import { Meta } from 'nextra'

export default {
	"index": "Introduction",
	"setup": "Setup"
} satisfies Meta`)
	entries, warnings := ParseNextraMeta("_meta.ts", data)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d; warnings: %v", len(entries), warnings)
	}
	if entries[0].Key != "index" || entries[0].Title != "Introduction" {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
}

func TestNextraMeta_UnquotedKeys(t *testing.T) {
	data := []byte(`export default {
		index: "Home",
		"getting-started": "Getting Started"
	}`)
	entries, warnings := ParseNextraMeta("_meta.js", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "index" {
		t.Errorf("expected key 'index', got %q", entries[0].Key)
	}
}

func TestNextraMeta_TrailingCommas(t *testing.T) {
	data := []byte(`{
		"index": "Home",
		"guide": "Guide",
	}`)
	entries, warnings := ParseNextraMeta("_meta.json", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestNextraMeta_ConstVarExportDefault(t *testing.T) {
	data := []byte(`const meta = {
  "installing-usermaven": "Installing Usermaven",
  "cookieless-tracking": "Cookieless Tracking",
};
export default meta;`)
	entries, warnings := ParseNextraMeta("_meta.js", data)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "installing-usermaven" {
		t.Errorf("expected key 'installing-usermaven', got %q", entries[0].Key)
	}
}

func TestNextraMeta_EmptyFile(t *testing.T) {
	entries, warnings := ParseNextraMeta("_meta.json", []byte(""))
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	foundWarning := false
	for _, w := range warnings {
		if w.Type == "meta_parse_error" || w.Type == "meta_parse_fallback" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for empty file")
	}
}
