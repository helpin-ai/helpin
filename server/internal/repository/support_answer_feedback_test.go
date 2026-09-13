package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMetadataEnrichmentPreservesVisitorFeedback(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	msg := &model.SupportMessage{ID: "feedback-message", WorkspaceID: "w", ConversationID: "c", SenderType: "ai", MessageType: "reply", Content: "Answer", Metadata: `{"visitor_feedback":{"helpful":false,"submitted_at":"2026-09-13T12:00:00Z"}}`}
	if err := repo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateMetadata(context.Background(), msg.ID, `{"link_previews":[{"url":"https://example.com"}]}`); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(context.Background(), msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Metadata, "visitor_feedback") || !strings.Contains(got.Metadata, "link_previews") {
		t.Fatalf("enrichment lost persisted metadata: %s", got.Metadata)
	}
}
