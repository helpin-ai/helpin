package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func supportGateEvidence() []KnowledgeSearchResult {
	return []KnowledgeSearchResult{
		{
			ID:            "chunk-1",
			ReferenceID:   "docs:chunk-1",
			SourceType:    "docs",
			Title:         "Pricing",
			Content:       "The Pro plan costs $49 per month and includes 10 seats.",
			VectorScore:   0.9,
			LexicalScore:  0.5,
			CombinedScore: 0.9,
		},
	}
}

func aiHistoryMessage(confidence float64) model.SupportMessage {
	metadata, _ := json.Marshal(map[string]interface{}{"ai_auto_reply": true, "ai_confidence": confidence})
	return model.SupportMessage{SenderType: "ai", Content: "earlier reply", Metadata: string(metadata)}
}

func TestSupportReplyInternalProcessDisclosures(t *testing.T) {
	bad := "The knowledge base search results did not meet the confidence threshold, so I launched a child agent."
	if got := supportReplyInternalProcessDisclosures(bad); len(got) < 4 {
		t.Fatalf("expected internal process disclosures, got %v", got)
	}
	if got := supportReplyInternalProcessDisclosures("I launched a sub-agent to inspect the repository."); len(got) == 0 {
		t.Fatalf("expected sub-agent process disclosure to be rejected, got %v", got)
	}
	good := "Usermaven supports Google Ads conversion tracking. I can confirm the exact offline-sync workflow with our product team."
	if got := supportReplyInternalProcessDisclosures(good); len(got) != 0 {
		t.Fatalf("customer-facing limitation was rejected: %v", got)
	}
}

func TestEvaluateSupportReplyGate(t *testing.T) {
	tests := []struct {
		name       string
		input      supportReplyGateInput
		wantOK     bool
		wantReason string
	}{
		{
			name: "grounded answer passes",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content:      "The Pro plan costs $49 per month.",
					CanAnswer:    true,
					Confidence:   0.9,
					SourceDocIDs: []string{"chunk-1"},
					Claims:       []AIResponseClaim{{Text: "The Pro plan costs $49 per month.", EvidenceIDs: []string{"chunk-1"}}},
				},
				Evidence:  supportGateEvidence(),
				Threshold: 0.7,
			},
			wantOK: true,
		},
		{
			name: "claim citing unknown evidence escalates",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content:    "You get unlimited seats.",
					CanAnswer:  true,
					Confidence: 0.9,
					Claims:     []AIResponseClaim{{Text: "You get unlimited seats.", EvidenceIDs: []string{"missing"}}},
				},
				Evidence:  supportGateEvidence(),
				Threshold: 0.7,
			},
			wantOK:     false,
			wantReason: "answer_validation_" + supportValidationUngrounded,
		},
		{
			name: "runtime evidence id contributes citation coverage",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content:      "The Pro plan costs $49 per month.",
					CanAnswer:    true,
					Confidence:   0.92,
					SourceDocIDs: []string{"chunk-1"},
					Claims:       []AIResponseClaim{{Text: "The Pro plan costs $49 per month.", EvidenceIDs: []string{"chunk-1"}}},
				},
				Evidence: []KnowledgeSearchResult{{
					ID: "chunk-1", ReferenceID: "content:page-1", Content: "The Pro plan costs $49 per month.", VectorScore: 0.4,
				}},
				Threshold: 0.7,
			},
			wantOK: true,
		},
		{
			name: "numeric mismatch escalates",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content:      "The Pro plan costs $99 per month.",
					CanAnswer:    true,
					Confidence:   0.95,
					SourceDocIDs: []string{"chunk-1"},
					Claims:       []AIResponseClaim{{Text: "The Pro plan costs $99 per month.", EvidenceIDs: []string{"chunk-1"}}},
				},
				Evidence:  supportGateEvidence(),
				Threshold: 0.7,
			},
			wantOK:     false,
			wantReason: "answer_validation_" + supportValidationNumeric,
		},
		{
			name: "free trial cannot be generalized into free tier",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content:      "We offer a free tier plus paid plans.",
					CanAnswer:    true,
					Confidence:   0.95,
					SourceDocIDs: []string{"trial"},
					Claims:       []AIResponseClaim{{Text: "We offer a free tier.", EvidenceIDs: []string{"trial"}}},
				},
				Evidence: []KnowledgeSearchResult{{
					ID: "trial", ReferenceID: "content:pricing", Content: "Start a 14-day free trial. Sign up free.", VectorScore: 0.9,
				}},
				Threshold: 0.7,
			},
			wantOK:     false,
			wantReason: "answer_validation_" + supportValidationUngrounded,
		},
		{
			name: "low confidence answer escalates",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content:      "The Pro plan costs $49 per month.",
					CanAnswer:    false,
					Confidence:   0.1,
					SourceDocIDs: []string{"chunk-1"},
					Claims:       []AIResponseClaim{{Text: "The Pro plan costs $49 per month.", EvidenceIDs: []string{"chunk-1"}}},
				},
				Evidence:  supportGateEvidence(),
				Threshold: 0.9,
			},
			wantOK:     false,
			wantReason: "low_confidence",
		},
		{
			name: "conversational reply passes without evidence",
			input: supportReplyGateInput{
				Kind:      "conversational",
				Contract:  &AIResponseContract{Content: "Hi there! How can I help?", CanAnswer: true, Confidence: 0.95},
				Threshold: 0.7,
			},
			wantOK: true,
		},
		{
			name: "declining satisfaction escalates",
			input: supportReplyGateInput{
				Kind:     "conversational",
				Contract: &AIResponseContract{Content: "Let me try again...", CanAnswer: true, Confidence: 0.45},
				History: []model.SupportMessage{
					aiHistoryMessage(0.95),
					aiHistoryMessage(0.8),
				},
				Threshold: 0.7,
			},
			wantOK: false,
			// The exact reason constant comes from the escalation module.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateSupportReplyGate(tt.input)
			if got.OK != tt.wantOK {
				t.Fatalf("gate OK = %v (reason=%q outcome=%q conf=%.2f), want %v", got.OK, got.EscalationReason, got.ValidationOutcome, got.Confidence, tt.wantOK)
			}
			if tt.wantReason != "" && got.EscalationReason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.EscalationReason, tt.wantReason)
			}
			if !got.OK && got.EscalationReason == "" {
				t.Error("failed gate must carry an escalation reason")
			}
		})
	}
}

func TestNormalizeSupportReplyKind(t *testing.T) {
	if normalizeSupportReplyKind(" Clarify ") != supportReplyKindClarify {
		t.Error("clarify not normalized")
	}
	if normalizeSupportReplyKind("unknown") != supportReplyKindAnswer {
		t.Error("unknown kind should default to answer")
	}
}
