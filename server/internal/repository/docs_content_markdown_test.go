package repository

import (
	"encoding/json"
	"testing"
)

func TestExtractPlainTextIndexesMarkdownBeforeEditorVisit(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`{"_markdown_source":"# Setup\n\nInstall the widget."}`, "# Setup\n\nInstall the widget."},
		{`{"_markdown_source":"  "}`, ""},
		{`{"_markdown_source":42}`, ""},
		{`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Existing rich text"}]}]}`, "Existing rich text "},
	} {
		if got := extractPlainText(json.RawMessage(tc.input)); got != tc.want {
			t.Errorf("extractPlainText(%s) = %q; want %q", tc.input, got, tc.want)
		}
	}
}
