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
			company_id TEXT,
			signal_type TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'manual',
			source_id TEXT,
			source_thread_id TEXT,
			summary TEXT NOT NULL,
			evidence_excerpt TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			confidence REAL NOT NULL DEFAULT 0,
			detected_at DATETIME NOT NULL,
			detector_kind TEXT NOT NULL DEFAULT 'direct',
			signal_domain TEXT NOT NULL DEFAULT 'conversation',
			polarity TEXT NOT NULL DEFAULT 'neutral',
			rule_key TEXT,
			rule_version TEXT,
			window_started_at DATETIME,
			window_ended_at DATETIME,
			evidence_identity_method TEXT NOT NULL DEFAULT 'unknown',
			evidence_identity_trust TEXT NOT NULL DEFAULT 'untrusted',
			evidence_fingerprint TEXT NOT NULL DEFAULT '',
			dismissed_at DATETIME,
			dismissed_by_member_id TEXT,
			dismissal_reason TEXT,
			reviewed_at DATETIME,
			acted_at DATETIME,
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

func TestSignalDetectionService_RequiresVerifiedConfidentEvidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		evidence   string
		want       int
	}{
		{name: "confidence below threshold", confidence: 0.59, evidence: "enterprise pricing", want: 0},
		{name: "evidence absent from source", confidence: 0.91, evidence: "approved procurement", want: 0},
		{name: "verified evidence", confidence: 0.91, evidence: "enterprise pricing", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupCRMSignalDetectionTestDB(t)
			svc := NewSignalDetectionService(&fakeLLMProvider{content: fmt.Sprintf(
				`[{"signal_type":"buying_intent","summary":"Pricing interest","confidence":%v,"raw_evidence":%q}]`,
				tt.confidence, tt.evidence,
			)}, repository.NewCRMSignalRepository(db), &fakeSummaryRequester{})
			rows, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{{
				WorkspaceID: "ws-1", SourceType: model.CRMSignalSourceEmail, SourceID: "message-1",
				Body: "Please send enterprise pricing and implementation details.",
			}})
			if err != nil {
				t.Fatalf("DetectSignals: %v", err)
			}
			if len(rows) != tt.want {
				t.Fatalf("signals = %d, want %d", len(rows), tt.want)
			}
		})
	}
}

func TestSignalDetectionServiceVerifiesEvidenceAcrossInlineHTMLTags(t *testing.T) {
	db := setupCRMSignalDetectionTestDB(t)
	svc := NewSignalDetectionService(&fakeLLMProvider{content: `[
		{"signal_type":"buying_intent","summary":"The buyer requested pricing.","confidence":0.92,"raw_evidence":"Please send pricing details."}
	]`}, repository.NewCRMSignalRepository(db), &fakeSummaryRequester{})

	rows, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{{
		WorkspaceID: "ws-1", SourceType: model.CRMSignalSourceEmail, SourceID: "message-1",
		Body: `<p>Please send pri<strong>cing</strong> details.</p>`,
	}})
	if err != nil {
		t.Fatalf("DetectSignals: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("signals = %d, want 1", len(rows))
	}
}

func TestSignalDetectionService_BatchBindsSignalToDeclaredSource(t *testing.T) {
	db := setupCRMSignalDetectionTestDB(t)
	repo := repository.NewCRMSignalRepository(db)
	svc := NewSignalDetectionService(&fakeLLMProvider{content: `[
		{"source_type":"email","source_id":"message-2","signal_type":"timeline_signal","summary":"The buyer needs a decision by September 15.","confidence":0.94,"raw_evidence":"We need to decide by September 15, 2026.","timeline_date":"2026-09-15"}
	]`}, repo, &fakeSummaryRequester{})
	contact1, contact2 := "contact-1", "contact-2"
	payloads := []model.SignalSourcePayload{
		{WorkspaceID: "ws-1", SourceType: model.CRMSignalSourceEmail, SourceID: "message-1", ContactID: &contact1, Body: "Please send the overview."},
		{WorkspaceID: "ws-1", SourceType: model.CRMSignalSourceEmail, SourceID: "message-2", ContactID: &contact2, Body: "We need to decide by September 15, 2026."},
	}
	signals, err := svc.DetectSignals(context.Background(), payloads)
	if err != nil {
		t.Fatalf("DetectSignals: %v", err)
	}
	if len(signals) != 1 || signals[0].SourceID == nil || *signals[0].SourceID != "message-2" {
		t.Fatalf("signals = %+v, want one signal sourced from message-2", signals)
	}
	if signals[0].ContactID == nil || *signals[0].ContactID != contact2 {
		t.Fatalf("contact_id = %v, want %s", signals[0].ContactID, contact2)
	}
	if got := signals[0].Metadata["timeline_date"]; got != "2026-09-15" {
		t.Fatalf("metadata.timeline_date = %v, want 2026-09-15", got)
	}
}
