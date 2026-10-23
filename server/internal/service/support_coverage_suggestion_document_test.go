package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func TestCoverageSuggestionDocument(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantText []string
	}{
		{
			name:     "valid document is kept",
			raw:      `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Keep me"}]}]}`,
			wantText: []string{"Keep me"},
		},
		{
			name:     "model sections become headings and paragraphs",
			raw:      `{"title":"Audio formats","summary":"Which files Talking Avatar accepts.","sections":[{"heading":"Supported formats","body":"- **MP3**\n- WAV"}]}`,
			wantText: []string{"Which files Talking Avatar accepts.", "Supported formats", "MP3", "WAV"},
		},
		{
			name:     "markdown string is converted",
			raw:      `"## Refunds\n\nRefunds take 3-5 business days."`,
			wantText: []string{"Refunds", "3-5 business days"},
		},
		{
			name: "unreadable shape becomes an empty document",
			raw:  `["not", "a", "draft"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coverageSuggestionDocument(json.RawMessage(tt.raw))
			if err := tiptap.ValidateDocument(got); err != nil {
				t.Fatalf("result is not a TipTap document: %v (%s)", err, got)
			}
			text := tiptap.RichTextToMarkdown(string(got))
			for _, want := range tt.wantText {
				if !strings.Contains(text, want) {
					t.Errorf("document %s lacks %q", got, want)
				}
			}
		})
	}
}

func TestNormalizeCoverageConversationAnalysisResult_PercentConfidence(t *testing.T) {
	result := CoverageConversationAnalysisResult{ConversationType: "support_query", HasGap: true, Confidence: 85}
	normalizeCoverageConversationAnalysisResult(&result)
	if result.Confidence != 0.85 {
		t.Fatalf("confidence = %v, want 0.85", result.Confidence)
	}
}
