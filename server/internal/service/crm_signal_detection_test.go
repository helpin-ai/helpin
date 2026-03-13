package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeLLMProvider struct {
	content string
}

func (f *fakeLLMProvider) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: f.content}, nil
}

func setupCRMSignalDetectionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:crm_signal_detection_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE crm_buyer_signals (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			signal_type TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'manual',
			source_id TEXT,
			source_thread_id TEXT,
			summary TEXT NOT NULL,
			evidence_excerpt TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			confidence REAL NOT NULL DEFAULT 0,
			detected_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX idx_crm_signals_source_thread ON crm_buyer_signals(source_thread_id)`,
		`CREATE UNIQUE INDEX idx_crm_signals_workspace_source_type_source_id_signal_type_unique
			ON crm_buyer_signals(workspace_id, source_type, source_id, signal_type)
			WHERE source_id IS NOT NULL`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema statement %q: %v", stmt, err)
		}
	}

	return db
}

func TestSignalDetectionService_PersistsProvenanceAndDedupes(t *testing.T) {
	db := setupCRMSignalDetectionTestDB(t)
	repo := repository.NewCRMSignalRepository(db)
	summary := &fakeSummaryRequester{}
	svc := NewSignalDetectionService(&fakeLLMProvider{
		content: `[{"signal_type":"buying_intent","summary":"Prospect asked for pricing","confidence":0.91,"raw_evidence":"Can you send pricing for enterprise?"}]`,
	}, repo, summary)

	threadID := "thread-1"
	threadExternalID := "ext-thread-1"
	contactID := "contact-1"
	payload := model.SignalSourcePayload{
		SourceType:             model.CRMSignalSourceEmail,
		SourceID:               "message-1",
		SourceThreadID:         &threadID,
		SourceThreadExternalID: &threadExternalID,
		WorkspaceID:            "ws-1",
		ContactID:              &contactID,
		Subject:                "Pricing",
		Body:                   "Can you send pricing for enterprise?",
		Participants: []model.SignalParticipant{
			{Email: "buyer@example.com", Role: model.CRMEmailParticipantRoleFrom},
			{Email: "owner@example.com", Role: model.CRMEmailParticipantRoleTo},
		},
		Direction:  model.CRMEmailDirectionInbound,
		OccurredAt: time.Now(),
	}

	signals, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{payload})
	if err != nil {
		t.Fatalf("DetectSignals first run: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("signals = %d, want 1", len(signals))
	}

	signals, err = svc.DetectSignals(context.Background(), []model.SignalSourcePayload{payload})
	if err != nil {
		t.Fatalf("DetectSignals second run: %v", err)
	}
	if len(signals) != 0 {
		t.Fatalf("signals on rerun = %d, want 0", len(signals))
	}

	var stored model.CRMBuyerSignal
	if err := db.Table("crm_buyer_signals").First(&stored).Error; err != nil {
		t.Fatalf("load stored signal: %v", err)
	}
	if stored.SourceThreadID == nil || *stored.SourceThreadID != "thread-1" {
		t.Fatalf("source_thread_id = %v, want thread-1", stored.SourceThreadID)
	}
	if stored.EvidenceExcerpt == nil || *stored.EvidenceExcerpt == "" {
		t.Fatal("expected evidence_excerpt to be stored")
	}
	if got := stored.Metadata["thread_external_id"]; got != "ext-thread-1" {
		t.Fatalf("metadata.thread_external_id = %v, want ext-thread-1", got)
	}
	if got := stored.Metadata["message_direction"]; got != model.CRMEmailDirectionInbound {
		t.Fatalf("metadata.message_direction = %v, want inbound", got)
	}
	var count int64
	if err := db.Table("crm_buyer_signals").Count(&count).Error; err != nil {
		t.Fatalf("count signals: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored signal count = %d, want 1", count)
	}
	if len(summary.contactRefreshes) != 1 || summary.contactRefreshes[0] != "ws-1:contact-1" {
		t.Fatalf("contact summary refreshes = %v, want ws-1:contact-1", summary.contactRefreshes)
	}
}
