package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRichTextToMarkdown_PreservesMarkdownEnvelope(t *testing.T) {
	input := `{"_markdown_source":"# Scope\n\n- keep html out of prompts"}`

	got := RichTextToMarkdown(input)

	if got != "# Scope\n\n- keep html out of prompts" {
		t.Fatalf("RichTextToMarkdown() = %q", got)
	}
}

func TestRichTextToMarkdown_ConvertsHTML(t *testing.T) {
	input := "<h2>Scope</h2><p><strong>Important</strong> rollout</p><ul><li>First</li><li>Second</li></ul>"

	got := RichTextToMarkdown(input)

	for _, snippet := range []string{"## Scope", "**Important** rollout", "- First", "- Second"} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected markdown to contain %q, got %q", snippet, got)
		}
	}
}

func TestRichTextToMarkdown_ConvertsTipTapJSON(t *testing.T) {
	input := string(MarkdownToJSON("# Scope\n\n- First item\n- Second item"))

	got := RichTextToMarkdown(input)

	for _, snippet := range []string{"# Scope", "- First item", "- Second item"} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected markdown to contain %q, got %q", snippet, got)
		}
	}
}

func TestRichTextToMarkdown_LeavesPlainTextAlone(t *testing.T) {
	input := "Plain text story summary"

	got := RichTextToMarkdown(input)

	if got != input {
		t.Fatalf("RichTextToMarkdown() = %q", got)
	}
}

func TestRichTextToMarkdown_IgnoresNonDocumentJSON(t *testing.T) {
	inputBytes, _ := json.Marshal(map[string]any{"title": "not rich text"})

	got := RichTextToMarkdown(string(inputBytes))

	if got != string(inputBytes) {
		t.Fatalf("RichTextToMarkdown() = %q", got)
	}
}
