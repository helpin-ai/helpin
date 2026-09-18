package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportMessagePromptTextIncludesAttachmentSummary(t *testing.T) {
	msg := model.SupportMessage{
		Content: "What is wrong here?",
		Attachments: []model.SupportAttachmentPayload{
			{FileName: "screenshot.png", FileType: "image/png", URL: "https://assets.example.com/screenshot.png"},
			{FileName: "error.log", FileType: "text/plain", URL: "https://assets.example.com/error.log"},
		},
	}

	got := supportMessagePromptText(msg)
	if got == "" {
		t.Fatal("expected prompt text")
	}
	if !containsAll(got, []string{"What is wrong here?", "screenshot.png", "error.log"}) {
		t.Fatalf("expected prompt text to mention content and attachments, got %q", got)
	}
}

func TestSanitizeConversationHistoryKeepsAttachmentOnlyMessages(t *testing.T) {
	history := []model.SupportMessage{
		{
			ID:      "m1",
			Content: "",
			Attachments: []model.SupportAttachmentPayload{
				{FileName: "photo.jpg", FileType: "image/jpeg", URL: "https://assets.example.com/photo.jpg"},
			},
		},
		{
			ID:      "m2",
			Content: "",
		},
	}

	got := sanitizeConversationHistory(history, "")
	if len(got) != 1 || got[0].ID != "m1" {
		t.Fatalf("expected attachment-only message to be retained, got %#v", got)
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
