package service

import (
	"encoding/json"
	"testing"
)

func TestRestoreImportedToggleAttrsFromSourceRestoresMissingHelpScoutMetadata(t *testing.T) {
	sourceHTML := `<details><summary><span>🔑</span><span>Authentication &amp; Setup</span><span>3 topics</span><span>▼</span></summary><div><a href="#auth">Authentication</a></div></details>`
	input := json.RawMessage(`{"type":"doc","content":[{"type":"toggleSection","attrs":{"title":"Authentication & Setup","icon":null,"badgeText":null,"sourceStyle":null,"open":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"Authentication"}]}]}]}`)

	got, changed, err := restoreImportedToggleAttrsFromSource(input, sourceHTML)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected missing imported toggle metadata to be restored")
	}

	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	content := doc["content"].([]any)
	attrs := content[0].(map[string]any)["attrs"].(map[string]any)
	if attrs["icon"] != "🔑" {
		t.Fatalf("expected icon restored, got %#v", attrs["icon"])
	}
	if attrs["badgeText"] != "3 topics" {
		t.Fatalf("expected badge restored, got %#v", attrs["badgeText"])
	}
	if attrs["sourceStyle"] != "helpScoutCard" {
		t.Fatalf("expected sourceStyle restored, got %#v", attrs["sourceStyle"])
	}
	if attrs["open"] != true {
		t.Fatalf("expected existing open attr preserved, got %#v", attrs["open"])
	}
}

func TestRestoreImportedToggleAttrsFromSourceDoesNotOverrideExistingMetadata(t *testing.T) {
	sourceHTML := `<details><summary><span>🔑</span><span>Authentication &amp; Setup</span><span>3 topics</span></summary><p>Body</p></details>`
	input := json.RawMessage(`{"type":"doc","content":[{"type":"toggleSection","attrs":{"title":"Authentication & Setup","icon":"A","badgeText":"Custom","sourceStyle":"custom"},"content":[{"type":"paragraph"}]}]}`)

	got, changed, err := restoreImportedToggleAttrsFromSource(input, sourceHTML)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected existing metadata to be preserved without changes")
	}

	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	attrs := doc["content"].([]any)[0].(map[string]any)["attrs"].(map[string]any)
	if attrs["icon"] != "A" || attrs["badgeText"] != "Custom" || attrs["sourceStyle"] != "custom" {
		t.Fatalf("expected existing attrs preserved, got %#v", attrs)
	}
}
