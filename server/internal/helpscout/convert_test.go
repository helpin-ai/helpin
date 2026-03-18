package helpscout

import (
	"strings"
	"testing"
)

func TestConvertHTML(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains string // substring that must appear in output
	}{
		{"empty", "", ""},
		{"paragraph", "<p>Hello world</p>", "Hello world"},
		{"heading", "<h2>Title</h2>", "## Title"},
		{"bold", "<p><strong>bold</strong></p>", "**bold**"},
		{"italic", "<p><em>italic</em></p>", "*italic*"},
		{"link", `<p><a href="https://example.com">click</a></p>`, "[click](https://example.com)"},
		{"unordered list", "<ul><li>one</li><li>two</li></ul>", "- one"},
		{"ordered list", "<ol><li>first</li><li>second</li></ol>", "1. first"},
		{"image", `<img src="https://img.com/pic.png" alt="photo">`, "![photo](https://img.com/pic.png)"},
		{"callout info", `<div class="callout callout-info"><p>Note text</p></div>`, "> **Note:**"},
		{"callout warn", `<div class="callout callout-warn"><p>Warning text</p></div>`, "> **Warning:**"},
		{"iframe", `<iframe src="https://www.youtube.com/embed/abc123"></iframe>`, "https://www.youtube.com/embed/abc123"},
		{"strips inline style", `<p style="color:red;font-size:14px">styled</p>`, "styled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertHTML(tt.html)
			if err != nil {
				t.Fatalf("ConvertHTML() error = %v", err)
			}
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("ConvertHTML() = %q, want substring %q", result, tt.contains)
			}
		})
	}
}
