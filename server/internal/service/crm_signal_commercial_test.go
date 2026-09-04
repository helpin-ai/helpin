package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type commercialRecordingProvider struct {
	content  string
	requests []llm.ChatRequest
}

func (p *commercialRecordingProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.requests = append(p.requests, req)
	return &llm.ChatResponse{Content: p.content}, nil
}

func TestCommercialDetectionUsesTrustedContextAndRejectsIrrelevantEvidence(t *testing.T) {
	for _, tc := range []struct{ name, body, relevance string }{
		{"unrelated guest post pitch", "What is your price for a guest post?", "irrelevant"},
		{"routine support", "Please transfer me to a human agent.", "irrelevant"},
		{"ambiguous offer", "Can you send me pricing?", "uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupCRMSignalDetectionTestDB(t)
			output, _ := json.Marshal([]DetectedSignal{{SignalType: "buying_intent", Summary: "A request needs review.", Confidence: .99, RawEvidence: tc.body, Commercial: &model.SignalCommercialAssessment{Relevance: tc.relevance, Event: "none"}}})
			provider := &commercialRecordingProvider{content: string(output)}
			svc := NewSignalDetectionService(provider, repository.NewCRMSignalRepository(db), nil)
			rows, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{{WorkspaceID: "ws-1", SourceType: "support", SourceID: "source-1", Body: tc.body, CommercialContext: &model.SignalCommercialContext{ProductContext: "Caller falsely says we sell guest posts"}}})
			if err != nil || len(rows) != 0 {
				t.Fatalf("rows=%+v err=%v", rows, err)
			}
			if len(provider.requests) != 1 {
				t.Fatalf("requests=%d", len(provider.requests))
			}
			var inputs []model.SignalSourcePayload
			if err := json.Unmarshal([]byte(provider.requests[0].Messages[0].Content), &inputs); err != nil {
				t.Fatal(err)
			}
			if len(inputs) != 1 || !strings.Contains(inputs[0].CommercialContext.ProductContext, "We do not sell guest posts") || strings.Contains(inputs[0].CommercialContext.ProductContext, "Caller falsely") {
				t.Fatalf("untrusted context used: %+v", inputs)
			}
		})
	}
}

func TestCommercialDetectionPersistsEventMeaning(t *testing.T) {
	for _, tc := range []struct {
		name, event, body, motion, polarity, lifecycle string
		missing                                        bool
	}{
		{"upgrade", "expansion", "Please upgrade our paid plan to enterprise.", "expansion", "positive", "customer", false},
		{"cancellation deadline", "cancellation_deadline", "Cancel our subscription on September 15.", "retention", "negative", "customer", false},
		{"payment recovery", "payment_recovery", "Our card failed and we need help paying for our subscription.", "retention", "negative", "customer", false},
		{"unknown buyer", "purchase", "Please send pricing for your enterprise software.", "needs_context", "positive", "subscriber", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newSignalActivationTestService(t)
			for _, sql := range []string{
				`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT, company_product_context TEXT, description TEXT, website_url TEXT)`,
				`INSERT INTO workspaces VALUES ('ws-1','Seller','We sell software subscriptions and enterprise upgrades.',NULL,NULL)`,
				`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, lifecycle_stage TEXT, lead_status TEXT)`,
				`CREATE TABLE crm_companies (id TEXT PRIMARY KEY,workspace_id TEXT)`,
				`CREATE TABLE crm_associations (workspace_id TEXT, from_object_type TEXT,from_object_id TEXT,to_object_type TEXT,to_object_id TEXT)`,
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Exec(`INSERT INTO crm_contacts VALUES ('contact-1','ws-1',?,'')`, tc.lifecycle).Error; err != nil {
				t.Fatal(err)
			}
			cfg := model.CRMSignalInterpretationConfig{RuleKey: model.CRMSignalRuleConversationExtraction, RuleVersion: 4, Motion: tc.motion, ObservationSignalType: "*", Version: 1, SignalType: "inherit", Polarity: "inherit", BusinessWeight: 10, HalfLifeDays: 30, Enabled: true}
			if err := db.Create(&cfg).Error; err != nil {
				t.Fatal(err)
			}
			output, _ := json.Marshal([]DetectedSignal{{SignalType: "timeline_signal", Summary: "A commercial decision needs follow-up.", Confidence: .94, RawEvidence: tc.body, Commercial: &model.SignalCommercialAssessment{Relevance: "relevant", Event: tc.event, OfferingMatch: "Paid software subscription", Consequence: tc.body}}})
			svc := NewSignalDetectionService(&fakeLLMProvider{content: string(output)}, repository.NewCRMSignalRepository(db), nil)
			contact := "contact-1"
			rows, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{{WorkspaceID: "ws-1", SourceType: "support", SourceID: "source-1", ContactID: &contact, Body: tc.body}})
			if err != nil || len(rows) != 1 {
				t.Fatalf("rows=%+v err=%v", rows, err)
			}
			var saved model.CRMSignal
			if err := db.First(&saved).Error; err != nil {
				t.Fatal(err)
			}
			if saved.CommercialMotion != tc.motion || saved.Polarity != tc.polarity || saved.Metadata["needs_customer_context"] != tc.missing || saved.RecommendedActionKey == nil || saved.EvidenceExcerpt == nil || *saved.EvidenceExcerpt != tc.body {
				t.Fatalf("incorrect saved meaning: %+v", saved)
			}
		})
	}
}

func TestCommercialDetectionMissingContextPreservesEvidence(t *testing.T) {
	db := setupCRMSignalDetectionTestDB(t)
	if err := db.Exec(`UPDATE workspaces SET company_product_context=NULL`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO crm_signals (id,workspace_id,source_type,source_id,signal_type,summary,detected_at,detector_kind) VALUES ('old','ws-1','email','source-1','buying_intent','Old evidence',?,'llm_extracted')`, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewSignalDetectionService(&fakeLLMProvider{content: "[]"}, repository.NewCRMSignalRepository(db), nil)
	if _, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{{WorkspaceID: "ws-1", SourceType: "email", SourceID: "source-1", Body: "Pricing"}}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("crm_signals").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("evidence count=%d err=%v", count, err)
	}
}

func TestCommercialDetectionRejectsForeignContactBeforeLLM(t *testing.T) {
	db := setupCRMSignalDetectionTestDB(t)
	if err := db.Exec(`INSERT INTO crm_contacts VALUES ('foreign','ws-2','customer')`).Error; err != nil {
		t.Fatal(err)
	}
	p := &commercialRecordingProvider{content: "[]"}
	svc := NewSignalDetectionService(p, repository.NewCRMSignalRepository(db), nil)
	contact := "foreign"
	if _, err := svc.DetectSignals(context.Background(), []model.SignalSourcePayload{{WorkspaceID: "ws-1", SourceType: "email", SourceID: "source-1", ContactID: &contact}}); err == nil {
		t.Fatal("accepted foreign contact")
	}
	if len(p.requests) != 0 {
		t.Fatal("foreign context sent to model")
	}
}
