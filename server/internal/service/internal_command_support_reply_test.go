package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestRuntimeSupportReplyDoesNotCreateSecondUsageCharge(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	service := &InternalCommandService{
		supportUsageMeter: NewTokenPricedAIUsageMeter(consumer),
	}

	service.consumeSupportReplyBilling(context.Background(), "ws-1", "conversation-1", "message-1")

	if consumer.input.Context.WorkspaceID != "" {
		t.Fatalf("runtime reply created usage charge %#v, want agent-run usage only", consumer.input)
	}
}

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
			name: "public answer without configured knowledge escalates",
			input: supportReplyGateInput{
				Kind: "answer",
				Contract: &AIResponseContract{
					Content: "Open the legacy tab.", CanAnswer: true, Confidence: 1,
					Claims: []AIResponseClaim{{Text: "Open the legacy tab.", EvidenceIDs: []string{"missing"}}},
				},
				Threshold: 0.7,
			},
			wantReason: "answer_validation_" + supportValidationUngrounded,
		},
		{
			name: "clarification does not require missing product evidence",
			input: supportReplyGateInput{
				Kind: "clarify",
				Contract: &AIResponseContract{
					Content: "Which account are you trying to reconnect?", CanAnswer: true, Confidence: 0.8,
				},
				Threshold: 0.7,
			},
			wantOK: true,
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

func TestSupportReplyGateRejectsLegacyWebEvidence(t *testing.T) {
	for _, kind := range []string{"answer", "conversational"} {
		for _, citeIn := range []string{"claims", "sources"} {
			t.Run(kind+"/"+citeIn, func(t *testing.T) {
				evidence := supportGateEvidence()
				evidence = append(evidence, KnowledgeSearchResult{ID: "child-result:old-web", ReferenceID: "child-result:old-web", SourceType: supportChildSourceOfficialWeb, Content: "The Pro plan costs $49 per month.", VectorScore: 0.9})
				contract := &AIResponseContract{Content: "The Pro plan costs $49 per month.", CanAnswer: true, Confidence: 0.95, SourceDocIDs: []string{"chunk-1"}, Claims: []AIResponseClaim{{Text: "The Pro plan costs $49 per month.", EvidenceIDs: []string{"chunk-1"}}}}
				if citeIn == "claims" {
					contract.Claims[0].EvidenceIDs = []string{"child-result:old-web"}
				} else {
					contract.SourceDocIDs = []string{"child-result:old-web"}
				}
				gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: kind, Contract: contract, Evidence: evidence, Threshold: 0.7})
				if gate.OK || gate.EscalationReason != "answer_validation_unapproved_source" {
					t.Fatalf("legacy web reply verdict = %+v, want unapproved source", gate)
				}
			})
		}
	}
}

func TestSupportReplyGateIgnoresUncitedLegacyWebEvidence(t *testing.T) {
	evidence := append(supportGateEvidence(), KnowledgeSearchResult{ID: "old-web", SourceType: supportChildSourceOfficialWeb})
	contract := &AIResponseContract{Content: "The Pro plan costs $49 per month.", CanAnswer: true, Confidence: 0.9, SourceDocIDs: []string{"chunk-1"}, Claims: []AIResponseClaim{{Text: "The Pro plan costs $49 per month.", EvidenceIDs: []string{"chunk-1"}}}}
	if gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: "answer", Contract: contract, Evidence: evidence, Threshold: 0.7}); !gate.OK {
		t.Fatalf("configured knowledge was rejected: %+v", gate)
	}
}

// Reproduce the live greeting -> small talk -> pricing sequence through the
// runtime reply tool. The former confidence-trend gate escalated 1.0 -> .935 ->
// .74 even though the visitor had not expressed dissatisfaction.
func TestSupportSendReplyAfterGreetingDoesNotInferDissatisfaction(t *testing.T) {
	ctx := context.Background()
	chat, db, conv, source, processing, settings, _ := setupSupportGreetingTest(t)
	if handled, err := chat.replyToInitialGreeting(ctx, conv, source, &model.Agent{ID: "agent"}, processing, settings); err != nil || !handled {
		t.Fatalf("opening greeting: handled=%v err=%v", handled, err)
	}
	ai := chat.supportAIService
	ai.conversationRepo = chat.conversationRepo
	ai.messageRepo = chat.messageRepo
	ai.processingRepo = chat.processingRepo
	ai.installationRepo = repository.NewSupportInboxInstallationRepository(db)
	commands := &InternalCommandService{supportAIService: ai, supportProcessingRepo: chat.processingRepo}
	meta := model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID, AgentID: "agent"}
	for _, turn := range []struct {
		id, question, reply        string
		confidence, wantConfidence float64
	}{
		{"small-talk", "What's going on?", "What can I help you with today?", .9, .935},
		{"pricing", "What is your pricing?", "Are you interested in monthly or annual pricing?", .6, .74},
	} {
		t.Run(turn.id, func(t *testing.T) {
			message := &model.SupportMessage{ID: turn.id, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "customer", MessageType: "reply", Content: turn.question}
			if err := chat.messageRepo.Create(ctx, message); err != nil {
				t.Fatal(err)
			}
			if err := chat.conversationRepo.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"last_public_message_id": message.ID}); err != nil {
				t.Fatal(err)
			}
			claim, ok := chat.processingRepo.BeginAttempt(ctx, conv.WorkspaceID, message.ID, conv.ID)
			if !ok {
				t.Fatal("could not claim customer turn")
			}
			input, err := json.Marshal(map[string]any{"content": turn.reply, "reply_kind": "clarify", "confidence": turn.confidence})
			if err != nil {
				t.Fatal(err)
			}
			output, err := commands.executeSupportSendReply(ctx, meta, input)
			if err != nil {
				t.Fatalf("send reply: %v", err)
			}
			var result struct {
				Status     string  `json:"status"`
				Confidence float64 `json:"confidence"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "sent" || math.Abs(result.Confidence-turn.wantConfidence) > .000001 {
				t.Fatalf("reply outcome = %s", output)
			}
			var settled model.AIMessageProcessing
			if err := db.First(&settled, "id = ?", claim.ID).Error; err != nil {
				t.Fatal(err)
			}
			if settled.Status != "completed" || settled.ReplyMessageID == nil {
				t.Fatalf("reply did not settle its source turn: %+v", settled)
			}
			var reply model.SupportMessage
			if err := db.First(&reply, "id = ?", *settled.ReplyMessageID).Error; err != nil {
				t.Fatal(err)
			}
			if reply.Content != turn.reply || reply.IsInternal {
				t.Fatalf("unexpected published reply: %+v", reply)
			}
			current, err := chat.conversationRepo.GetByID(ctx, conv.WorkspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if derefString(current.AIState) == "escalated" || current.HumanTakeover != nil && *current.HumanTakeover {
				t.Fatal("ordinary customer question was handed to a human")
			}
		})
	}
}

func TestSupportReplyGateRejectsUnreviewedMCPResult(t *testing.T) {
	evidence := []KnowledgeSearchResult{{ID: "mcp__logs__lookup", SourceType: "external_mcp", IsInternal: true, Content: "Import failed", VectorScore: 1}}
	contract := &AIResponseContract{Content: "Your import succeeded.", CanAnswer: true, Confidence: 1, Claims: []AIResponseClaim{{Text: "Your import succeeded.", EvidenceIDs: []string{"mcp__logs__lookup"}}}}
	gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: "answer", Contract: contract, Evidence: evidence, Threshold: .7})
	if gate.OK {
		t.Fatal("an MCP tool's successful execution must not prove an unsupported answer")
	}
}
