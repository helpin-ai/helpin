package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCoverageConversationAnalysisInputExcludesInternalAndParsesAIMetadata(t *testing.T) {
	flowState := model.SupportConversationFlowStateAssignedToHuman
	conversation := model.SupportConversation{
		ID:          "conversation-1",
		WorkspaceID: "ws-1",
		Subject:     "Refund question",
		Status:      model.SupportConversationStatusResolved,
		FlowState:   &flowState,
		AITurnCount: 1,
	}
	base := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{
			ID:             "internal-note",
			WorkspaceID:    "ws-1",
			ConversationID: "conversation-1",
			SenderType:     "user",
			MessageType:    "reply",
			Content:        "Internal note",
			IsInternal:     true,
			CreatedAt:      base.Add(time.Minute),
		},
		{
			ID:             "customer-1",
			WorkspaceID:    "ws-1",
			ConversationID: "conversation-1",
			SenderType:     "customer",
			MessageType:    "reply",
			Content:        "Can I get a refund?",
			CreatedAt:      base,
		},
		{
			ID:             "ai-1",
			WorkspaceID:    "ws-1",
			ConversationID: "conversation-1",
			SenderType:     "ai",
			MessageType:    "reply",
			Content:        "I am not sure.",
			Metadata:       `{"ai_confidence":0.41,"ai_reply_kind":"answer","ai_issue_key":"refunds","ai_issue_summary":"Refund request","ai_progress_state":"stalled"}`,
			CreatedAt:      base.Add(2 * time.Minute),
		},
		{
			ID:             "human-1",
			WorkspaceID:    "ws-1",
			ConversationID: "conversation-1",
			SenderType:     "user",
			MessageType:    "reply",
			Content:        "I refunded the order from Stripe and explained the exception.",
			CreatedAt:      base.Add(3 * time.Minute),
		},
	}
	trace := model.SupportAIRetrievalTrace{
		MessageID:      "ai-1",
		SearchQueries:  json.RawMessage(`["refund exception"]`),
		Results:        json.RawMessage(`[{"source_type":"docs","document_id":"doc-1","title":"Refunds","snippet":"Current refund docs","combined_score":0.74}]`),
		CitedSourceIDs: json.RawMessage(`["doc-1"]`),
		AIConfidence:   0.41,
		FailureMode:    "weak_retrieval",
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, []model.SupportAIRetrievalTrace{trace})
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if input.WorkspaceID != "ws-1" || input.ConversationID != "conversation-1" || input.FlowState != flowState {
		t.Fatalf("conversation fields were not copied: %+v", input)
	}
	if len(input.Messages) != 3 {
		t.Fatalf("expected internal message to be excluded, got %d messages", len(input.Messages))
	}
	if input.Messages[1].ID != "ai-1" || input.Messages[1].AIConfidence != 0.41 || input.Messages[1].AIIssueKey != "refunds" || input.Messages[1].AIProgressState != "stalled" {
		t.Fatalf("AI metadata was not parsed: %+v", input.Messages[1])
	}
	if input.Messages[2].SenderType != "user" || input.Messages[2].Content == "" {
		t.Fatalf("expected human resolution reply to be included: %+v", input.Messages[2])
	}
	if len(input.RetrievalTraces) != 1 || input.RetrievalTraces[0].MessageID != "ai-1" || len(input.RetrievalTraces[0].Results) != 1 {
		t.Fatalf("retrieval trace was not parsed: %+v", input.RetrievalTraces)
	}
	if input.TranscriptHash == "" {
		t.Fatal("expected transcript hash")
	}
}

func TestCoverageConversationAnalysisInputTruncatesLongTranscriptsDeterministically(t *testing.T) {
	conversation := model.SupportConversation{ID: "conversation-1", WorkspaceID: "ws-1"}
	base := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{}
	for i := 0; i < 90; i++ {
		messages = append(messages, model.SupportMessage{
			ID:             string(rune('a' + (i % 26))),
			WorkspaceID:    "ws-1",
			ConversationID: "conversation-1",
			SenderType:     "customer",
			MessageType:    "reply",
			Content:        "long " + string(make([]byte, 2500)),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if len(input.Messages) != coverageAnalysisMaxMessages {
		t.Fatalf("expected %d messages, got %d", coverageAnalysisMaxMessages, len(input.Messages))
	}
	for _, msg := range input.Messages {
		if len(msg.Content) > coverageAnalysisMaxMessageChars+3 {
			t.Fatalf("message content was not capped: %d chars", len(msg.Content))
		}
	}

	again, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput second: %v", err)
	}
	if input.TranscriptHash != again.TranscriptHash {
		t.Fatal("expected deterministic transcript hash")
	}
}

func TestCoverageConversationAnalysisInputTranscriptHashStableByMessageOrder(t *testing.T) {
	base := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	first := []model.SupportMessage{
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Answer", Metadata: `{"b":2,"a":1}`, CreatedAt: base.Add(time.Minute)},
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Question\r\n", CreatedAt: base},
	}
	second := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Question\n", CreatedAt: base},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Answer", Metadata: `{"a":1,"b":2}`, CreatedAt: base.Add(time.Minute)},
	}
	changed := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Question changed", CreatedAt: base},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Answer", Metadata: `{"a":1,"b":2}`, CreatedAt: base.Add(time.Minute)},
	}

	if CoverageTranscriptHash(first) != CoverageTranscriptHash(second) {
		t.Fatal("expected hash to be stable for equivalent ordered content and normalized metadata")
	}
	if CoverageTranscriptHash(first) == CoverageTranscriptHash(changed) {
		t.Fatal("expected content change to alter transcript hash")
	}
}

func TestSupportCoverageDailyAnalyzer_AnalyzeConversationParsesGapTypes(t *testing.T) {
	cases := []struct {
		name           string
		response       string
		wantHasGap     bool
		wantKind       string
		wantCategory   string
		wantPrimaryFix string
	}{
		{
			name:           "knowledge gap",
			response:       `{"has_gap":true,"gap_kind":"content","gap_category":"knowledge","canonical_title":"Refund policy gap","customer_need":"Customer needed refund exception terms","ai_failure":"AI found generic refund docs only","human_resolution":"Agent explained an exception","decision_reason":"Human answer shows docs are incomplete","search_query":"refund exception policy","should_run_retrieval":true,"recommended_fixes":[{"type":"update_article","target_type":"docs","target_id":"doc-1","target_title":"Refunds","priority":"primary","rationale":"Existing docs are close","suggested_change":"Add exception criteria","implementation_notes":"Mention Stripe refund path"}],"confidence":0.86}`,
			wantHasGap:     true,
			wantKind:       "content",
			wantCategory:   model.SupportCoverageGapCategoryKnowledge,
			wantPrimaryFix: model.SupportCoverageFixUpdateArticle,
		},
		{
			name:           "data gap",
			response:       `{"has_gap":true,"gap_kind":"data","gap_category":"context","canonical_title":"Subscription status unavailable","customer_need":"Customer asked why renewal failed","ai_failure":"AI lacked subscription status","human_resolution":"Agent checked billing data","decision_reason":"Resolution depended on account data","search_query":"","should_run_retrieval":false,"recommended_fixes":[{"type":"add_data","target_type":"data_source","priority":"primary","rationale":"AI needs subscription status","suggested_change":"Expose subscription status to inbox AI","implementation_notes":"Connect billing source"}],"confidence":0.8}`,
			wantHasGap:     true,
			wantKind:       "data",
			wantCategory:   model.SupportCoverageGapCategoryContext,
			wantPrimaryFix: model.SupportCoverageFixAddData,
		},
		{
			name:           "action gap",
			response:       `{"has_gap":true,"gap_kind":"action","gap_category":"action","canonical_title":"Cancel subscription action missing","customer_need":"Customer wanted cancellation","ai_failure":"AI could explain only","human_resolution":"Agent cancelled subscription","decision_reason":"Resolution required an operation","search_query":"","should_run_retrieval":false,"recommended_fixes":[{"type":"add_action","target_type":"tool_action","priority":"primary","rationale":"AI needs a cancellation action","suggested_change":"Add guarded cancel action","implementation_notes":"Require confirmation"}],"confidence":0.81}`,
			wantHasGap:     true,
			wantKind:       "action",
			wantCategory:   model.SupportCoverageGapCategoryAction,
			wantPrimaryFix: model.SupportCoverageFixAddAction,
		},
		{
			name:       "no gap",
			response:   `{"has_gap":false,"gap_kind":"","gap_category":"","canonical_title":"","customer_need":"Customer asked setup question","ai_failure":"","human_resolution":"","decision_reason":"AI correctly resolved the issue","search_query":"","should_run_retrieval":false,"recommended_fixes":[],"confidence":0.91}`,
			wantHasGap: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &scriptedSupportPlannerLLM{
				responses: []llm.ChatResponse{{Content: tc.response}},
			}
			analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")

			result, raw, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{
				WorkspaceID:    "ws-1",
				ConversationID: "conversation-1",
				Subject:        "Support question",
				Messages: []CoverageConversationMessage{
					{ID: "m-1", SenderType: "customer", Content: "Can you help?"},
				},
			})
			if err != nil {
				t.Fatalf("AnalyzeConversation: %v", err)
			}
			if len(raw) == 0 {
				t.Fatal("expected raw output")
			}
			if result.HasGap != tc.wantHasGap {
				t.Fatalf("HasGap = %v, want %v", result.HasGap, tc.wantHasGap)
			}
			if tc.wantHasGap {
				if result.GapKind != tc.wantKind || result.GapCategory != tc.wantCategory {
					t.Fatalf("unexpected gap route: %+v", result)
				}
				if len(result.RecommendedFixes) == 0 || result.RecommendedFixes[0].Type != tc.wantPrimaryFix {
					t.Fatalf("unexpected recommended fixes: %+v", result.RecommendedFixes)
				}
			}
			if len(provider.requests) != 1 {
				t.Fatalf("expected one request, got %d", len(provider.requests))
			}
			req := provider.requests[0]
			if !req.JSONMode || req.JSONSchema == nil {
				t.Fatalf("expected JSON mode with schema: %+v", req)
			}
		})
	}
}

func TestSupportCoverageDailyAnalyzer_AnalyzeConversationRejectsMalformedJSON(t *testing.T) {
	provider := &scriptedSupportPlannerLLM{
		responses: []llm.ChatResponse{{Content: `not-json`}},
	}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")

	_, _, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		Messages:       []CoverageConversationMessage{{ID: "m-1", SenderType: "customer", Content: "Question"}},
	})
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

func TestSupportCoverageDailyAnalyzer_RefineFixBundleWithKnowledge(t *testing.T) {
	cases := []struct {
		name        string
		result      CoverageConversationAnalysisResult
		candidates  []CoverageKnowledgeCandidate
		response    string
		wantFixType string
		wantTarget  string
	}{
		{
			name: "existing docs article incomplete",
			result: CoverageConversationAnalysisResult{
				HasGap:       true,
				GapKind:      "content",
				GapCategory:  model.SupportCoverageGapCategoryKnowledge,
				CustomerNeed: "Customer needs refund exception criteria",
			},
			candidates: []CoverageKnowledgeCandidate{{
				SourceType: "docs",
				TargetType: "docs",
				DocumentID: "doc-1",
				Title:      "Refunds",
				Excerpt:    "Basic refund policy",
			}},
			response:    `{"recommended_fixes":[{"type":"update_article","target_type":"docs","target_id":"doc-1","target_title":"Refunds","target_url":"","priority":"primary","rationale":"The article is close but incomplete","suggested_change":"Add exception criteria","implementation_notes":"Include examples"}],"decision_reason":"Existing article covers the topic but needs detail.","confidence":0.87}`,
			wantFixType: model.SupportCoverageFixUpdateArticle,
			wantTarget:  "doc-1",
		},
		{
			name: "existing website page incomplete",
			result: CoverageConversationAnalysisResult{
				HasGap:       true,
				GapKind:      "content",
				GapCategory:  model.SupportCoverageGapCategoryKnowledge,
				CustomerNeed: "Prospect asks about security certifications",
			},
			candidates: []CoverageKnowledgeCandidate{{
				SourceType: "website",
				TargetType: "website_page",
				PageID:     "page-1",
				Title:      "Security",
				URL:        "https://example.com/security",
			}},
			response:    `{"recommended_fixes":[{"type":"update_website_page","target_type":"website_page","target_id":"page-1","target_title":"Security","target_url":"https://example.com/security","priority":"primary","rationale":"Prospects need this before buying","suggested_change":"Add certification details","implementation_notes":"Coordinate with marketing"}],"decision_reason":"Website page is the correct surface.","confidence":0.84}`,
			wantFixType: model.SupportCoverageFixUpdateWebsitePage,
			wantTarget:  "page-1",
		},
		{
			name: "no relevant candidate creates new content",
			result: CoverageConversationAnalysisResult{
				HasGap:       true,
				GapKind:      "content",
				GapCategory:  model.SupportCoverageGapCategoryKnowledge,
				CustomerNeed: "Customer needs migration steps",
			},
			response:    `{"recommended_fixes":[{"type":"create_article","target_type":"docs","target_id":"","target_title":"Migration steps","target_url":"","priority":"primary","rationale":"No existing candidate covers it","suggested_change":"Create a migration guide","implementation_notes":"Use support transcript as outline"}],"decision_reason":"No matching content exists.","confidence":0.79}`,
			wantFixType: model.SupportCoverageFixCreateArticle,
		},
		{
			name: "non knowledge fix preserved",
			result: CoverageConversationAnalysisResult{
				HasGap:      true,
				GapKind:     "action",
				GapCategory: model.SupportCoverageGapCategoryAction,
				RecommendedFixes: []CoverageRecommendedFix{{
					Type:       model.SupportCoverageFixAddAction,
					TargetType: "tool_action",
					Priority:   model.SupportCoverageRecommendationPriorityPrimary,
				}},
			},
			response:    `{"recommended_fixes":[{"type":"add_action","target_type":"tool_action","target_id":"","target_title":"Cancel subscription","target_url":"","priority":"primary","rationale":"The human performed this operation","suggested_change":"Add a guarded cancellation action","implementation_notes":"Require confirmation"}],"decision_reason":"Action recommendation remains primary.","confidence":0.82}`,
			wantFixType: model.SupportCoverageFixAddAction,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &scriptedSupportPlannerLLM{
				responses: []llm.ChatResponse{{Content: tc.response}},
			}
			analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")

			decision, err := analyzer.RefineFixBundleWithKnowledge(context.Background(), tc.result, tc.candidates)
			if err != nil {
				t.Fatalf("RefineFixBundleWithKnowledge: %v", err)
			}
			if decision.DecisionReason == "" || decision.Confidence == 0 {
				t.Fatalf("expected decision reason and confidence: %+v", decision)
			}
			if len(decision.RecommendedFixes) == 0 {
				t.Fatal("expected recommended fixes")
			}
			if decision.RecommendedFixes[0].Type != tc.wantFixType {
				t.Fatalf("fix type = %q, want %q", decision.RecommendedFixes[0].Type, tc.wantFixType)
			}
			if tc.wantTarget != "" && decision.RecommendedFixes[0].TargetID != tc.wantTarget {
				t.Fatalf("target = %q, want %q", decision.RecommendedFixes[0].TargetID, tc.wantTarget)
			}
			if len(provider.requests) != 1 || !provider.requests[0].JSONMode {
				t.Fatalf("expected one JSON-mode request, got %+v", provider.requests)
			}
		})
	}
}
