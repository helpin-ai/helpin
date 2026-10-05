package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMergeCoverageWorkspaceIDsPreservesCandidateFairnessAndAddsQueuedWork(t *testing.T) {
	got := mergeCoverageWorkspaceIDs([]string{"ws-recent", "ws-both"}, []string{"ws-both", "ws-rebuild", ""}, 3)
	want := []string{"ws-recent", "ws-both", "ws-rebuild"}
	if !slices.Equal(got, want) {
		t.Fatalf("merged workspaces = %#v, want %#v", got, want)
	}
}

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
			response:       `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"customer asking about refund","has_gap":true,"gap_kind":"content","gap_category":"knowledge","canonical_title":"Refund policy gap","customer_need":"Customer needed refund exception terms","ai_failure":"AI found generic refund docs only","human_resolution":"Agent explained an exception","decision_reason":"Human answer shows docs are incomplete","search_query":"refund exception policy","should_run_retrieval":true,"recommended_fixes":[{"type":"update_article","target_type":"docs","target_id":"doc-1","target_title":"Refunds","priority":"primary","rationale":"Existing docs are close","suggested_change":"Add exception criteria","implementation_notes":"Mention Stripe refund path"}],"confidence":0.86}`,
			wantHasGap:     true,
			wantKind:       "content",
			wantCategory:   model.SupportCoverageGapCategoryKnowledge,
			wantPrimaryFix: model.SupportCoverageFixUpdateArticle,
		},
		{
			name:           "data gap",
			response:       `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"customer asking about renewal","has_gap":true,"gap_kind":"data","gap_category":"context","canonical_title":"Subscription status unavailable","customer_need":"Customer asked why renewal failed","ai_failure":"AI lacked subscription status","human_resolution":"Agent checked billing data","decision_reason":"Resolution depended on account data","search_query":"","should_run_retrieval":false,"recommended_fixes":[{"type":"add_data","target_type":"data_source","priority":"primary","rationale":"AI needs subscription status","suggested_change":"Expose subscription status to inbox AI","implementation_notes":"Connect billing source"}],"confidence":0.8}`,
			wantHasGap:     true,
			wantKind:       "data",
			wantCategory:   model.SupportCoverageGapCategoryContext,
			wantPrimaryFix: model.SupportCoverageFixAddData,
		},
		{
			name:           "action gap",
			response:       `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"customer requesting cancellation","has_gap":true,"gap_kind":"action","gap_category":"action","canonical_title":"Cancel subscription action missing","customer_need":"Customer wanted cancellation","ai_failure":"AI could explain only","human_resolution":"Agent cancelled subscription","decision_reason":"Resolution required an operation","search_query":"","should_run_retrieval":false,"recommended_fixes":[{"type":"add_action","target_type":"tool_action","priority":"primary","rationale":"AI needs a cancellation action","suggested_change":"Add guarded cancel action","implementation_notes":"Require confirmation"}],"confidence":0.81}`,
			wantHasGap:     true,
			wantKind:       "action",
			wantCategory:   model.SupportCoverageGapCategoryAction,
			wantPrimaryFix: model.SupportCoverageFixAddAction,
		},
		{
			name:       "no gap",
			response:   `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"customer asking setup question","has_gap":false,"gap_kind":"","gap_category":"","canonical_title":"","customer_need":"Customer asked setup question","ai_failure":"","human_resolution":"","decision_reason":"AI correctly resolved the issue","search_query":"","should_run_retrieval":false,"recommended_fixes":[],"confidence":0.91}`,
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

func TestSupportCoverageDailyAnalyzer_AnalyzeConversationRejectsPartialRecommendationContract(t *testing.T) {
	provider := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{{Content: `{"is_support_query":true,"conversation_type":"support_query","has_gap":true,"customer_need":"Reset password","ai_failure":"Missing steps","recommended_fixes":[{"type":"update_article"}],"confidence":0.9}`}}}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")
	_, _, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{WorkspaceID: "ws-1", ConversationID: "conversation-1", TranscriptHash: "hash-1"})
	if !errors.Is(err, errCoverageLLMContract) {
		t.Fatalf("AnalyzeConversation error = %v, want llm contract failure", err)
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

func setupCoverageFindingUpsertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:coverage_finding_upsert_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	tables := []string{
		`CREATE TABLE support_coverage_topics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			issue_key TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			gap_count INTEGER NOT NULL DEFAULT 0,
			cluster_key TEXT,
			canonical_title TEXT,
			last_enriched_at DATETIME,
			cooldown_until DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workspace_id, issue_key)
		)`,
		`CREATE TABLE support_coverage_gaps (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			topic_id TEXT,
			dedupe_key TEXT NOT NULL,
			gap_kind TEXT NOT NULL DEFAULT 'content',
			gap_category TEXT NOT NULL DEFAULT 'unknown',
			v1_gap_type TEXT NOT NULL DEFAULT 'needs_review',
			title TEXT NOT NULL DEFAULT '',
			issue_key TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open',
			confidence REAL NOT NULL DEFAULT 0,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			failure_mode TEXT NOT NULL DEFAULT '',
			source_signal TEXT NOT NULL DEFAULT '',
			can_answer TEXT,
			can_resolve TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			status_changed_by TEXT,
			status_changed_at DATETIME,
			issue_resolved BOOLEAN,
			closed_at DATETIME,
			closed_evidence_count INTEGER,
			result_document_id TEXT,
			rejection_reason TEXT,
			embedding TEXT,
			embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '',
			embedding_version TEXT NOT NULL DEFAULT '',
			embedding_dimensions INTEGER NOT NULL DEFAULT 0,
			embedding_text_hash TEXT NOT NULL DEFAULT '',
			embedding_updated_at DATETIME,
			nearest_content_score REAL NOT NULL DEFAULT 0,
			nearest_content_document_id TEXT,
			nearest_content_title TEXT NOT NULL DEFAULT '',
			nearest_content_checked_at DATETIME,
			impact_score REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_gap_evidence (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			evidence_type TEXT NOT NULL,
			conversation_id TEXT,
			message_id TEXT,
			widget_session_id TEXT,
			document_id TEXT,
			article_public_id TEXT,
			source_signal TEXT NOT NULL DEFAULT '',
			source_key TEXT NOT NULL DEFAULT '',
			excerpt TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_gap_suggestions (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			suggestion_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			title TEXT NOT NULL DEFAULT '',
			content TEXT,
			evidence_summary TEXT NOT NULL DEFAULT '',
			target_space_id TEXT,
			target_collection_id TEXT,
			target_document_id TEXT,
			result_document_id TEXT,
			result_article_id TEXT,
			applied_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			superseded_at DATETIME,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_gap_articles (
			id TEXT PRIMARY KEY,
			gap_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(gap_id, document_id)
		)`,
		`CREATE TABLE support_coverage_conversation_analyses (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			status TEXT NOT NULL,
			has_gap BOOLEAN NOT NULL DEFAULT 0,
			gap_id TEXT,
			gap_kind TEXT NOT NULL DEFAULT '',
			gap_category TEXT NOT NULL DEFAULT '',
			primary_recommendation_type TEXT NOT NULL DEFAULT '',
			transcript_hash TEXT NOT NULL DEFAULT '',
			analyzer_version TEXT NOT NULL DEFAULT 'v1',
			canonical_title TEXT NOT NULL DEFAULT '',
			customer_need TEXT NOT NULL DEFAULT '',
			ai_failure TEXT NOT NULL DEFAULT '',
			human_resolution TEXT NOT NULL DEFAULT '',
			decision_reason TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL DEFAULT 0,
			is_support_query BOOLEAN NOT NULL DEFAULT 1,
			conversation_type TEXT NOT NULL DEFAULT 'support_query',
			classification_reason TEXT NOT NULL DEFAULT '',
			error_message TEXT,
			raw_output TEXT NOT NULL DEFAULT '{}',
			embedding TEXT,
			embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '',
			embedding_version TEXT NOT NULL DEFAULT '',
			embedding_dimensions INTEGER NOT NULL DEFAULT 0,
			embedding_text_hash TEXT NOT NULL DEFAULT '',
			embedding_updated_at DATETIME,
			materialization_metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_coverage_recommendations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			gap_id TEXT NOT NULL,
			analysis_id TEXT,
			recommendation_type TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT '',
			target_id TEXT,
			target_title TEXT NOT NULL DEFAULT '',
			target_url TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'secondary',
			status TEXT NOT NULL DEFAULT 'open',
			rationale TEXT NOT NULL DEFAULT '',
			suggested_change TEXT NOT NULL DEFAULT '',
			implementation_notes TEXT NOT NULL DEFAULT '',
			suggestion_id TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_ai_retrieval_traces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			search_queries TEXT NOT NULL DEFAULT '[]',
			results TEXT NOT NULL DEFAULT '[]',
			cited_source_ids TEXT NOT NULL DEFAULT '[]',
			ai_confidence REAL NOT NULL DEFAULT 0,
			can_answer TEXT,
			can_resolve TEXT,
			failure_mode TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			sender_type TEXT NOT NULL,
			message_type TEXT NOT NULL DEFAULT 'reply',
			content TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			is_internal BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)`,
	}
	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestSupportCoverageDailyAnalyzer_RunOverridesHumanResolutionInStoredRawOutput(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	provider := &scriptedSupportPlannerLLM{
		responses: []llm.ChatResponse{{
			Content: `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"customer asks a billing support question","has_gap":true,"gap_kind":"content","gap_category":"knowledge","canonical_title":"Billing update steps","customer_need":"Customer needed billing update steps.","ai_failure":"AI did not know the billing update flow.","human_resolution":"A human should explain the billing flow.","decision_reason":"The missing answer should be documented.","search_query":"","should_run_retrieval":false,"recommended_fixes":[{"type":"create_article","target_type":"docs","target_id":"","target_title":"Billing update steps","target_url":"","priority":"primary","rationale":"Customers need documented billing steps.","suggested_change":"Create a billing update article.","implementation_notes":"Use the product billing settings flow."}],"confidence":0.82}`,
		}},
	}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetConversationRepositories(nil, messageRepo)
	ctx := context.Background()
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"m-1", "ws-1", "conversation-no-human", "customer", "reply", "How do I update billing details?", base).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	created, err := analyzer.runConversationCoverageAnalysis(ctx, "ws-1", "run-1", "", model.SupportConversation{
		ID:          "conversation-no-human",
		WorkspaceID: "ws-1",
		Subject:     "Billing update",
		Status:      model.SupportConversationStatusOpen,
		UpdatedAt:   base,
	}, nil)
	if err != nil {
		t.Fatalf("runConversationCoverageAnalysis: %v", err)
	}
	if !created {
		t.Fatal("expected gap finding to be staged")
	}

	var analysis model.SupportCoverageConversationAnalysis
	if err := db.First(&analysis, "conversation_id = ?", "conversation-no-human").Error; err != nil {
		t.Fatalf("load analysis: %v", err)
	}
	if analysis.GapID != nil {
		t.Fatalf("expected analysis gap_id to remain nil before materialization, got %q", *analysis.GapID)
	}
	if analysis.CanonicalTitle != "Billing update steps" {
		t.Fatalf("canonical_title = %q, want Billing update steps", analysis.CanonicalTitle)
	}
	if analysis.HumanResolution != "No human response observed" {
		t.Fatalf("stored human_resolution = %q", analysis.HumanResolution)
	}
	var materialization map[string]any
	if err := json.Unmarshal(analysis.MaterializationMetadata, &materialization); err != nil {
		t.Fatalf("unmarshal materialization metadata: %v", err)
	}
	if materialization["has_human_reply"] != false {
		t.Fatalf("has_human_reply metadata = %v, want false", materialization["has_human_reply"])
	}
	var raw map[string]any
	if err := json.Unmarshal(analysis.RawOutput, &raw); err != nil {
		t.Fatalf("unmarshal raw output: %v", err)
	}
	if raw["human_resolution"] != "No human response observed" {
		t.Fatalf("raw human_resolution = %q, want override", raw["human_resolution"])
	}
	var suggestionCount int64
	if err := db.Model(&model.SupportGapSuggestion{}).Count(&suggestionCount).Error; err != nil {
		t.Fatalf("count suggestions: %v", err)
	}
	if suggestionCount != 0 {
		t.Fatalf("expected no auto-generated suggestion without human reply, got %d", suggestionCount)
	}
	var gapCount int64
	if err := db.Model(&model.SupportCoverageGap{}).Count(&gapCount).Error; err != nil {
		t.Fatalf("count gaps: %v", err)
	}
	if gapCount != 0 {
		t.Fatalf("expected no gap before materialization, got %d", gapCount)
	}
}

func TestClassifyCoverageConversationLocally(t *testing.T) {
	cases := []struct {
		name     string
		input    CoverageConversationAnalysisInput
		wantSkip bool
		wantType string
	}{
		{
			name: "no customer messages",
			input: CoverageConversationAnalysisInput{
				Subject: "Internal note",
				Messages: []CoverageConversationMessage{
					{SenderType: "user", Content: "Forwarding this for reference"},
				},
			},
			wantSkip: true,
			wantType: "other",
		},
		{
			name: "auto-reply subject",
			input: CoverageConversationAnalysisInput{
				Subject: "Out of Office: Re: Your inquiry",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "I am currently out of the office."},
				},
			},
			wantSkip: true,
			wantType: "auto_reply",
		},
		{
			name: "delivery status notification",
			input: CoverageConversationAnalysisInput{
				Subject: "Delivery Status Notification (Failure)",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "This is an automatically generated Delivery Status Notification."},
				},
			},
			wantSkip: true,
			wantType: "auto_reply",
		},
		{
			name: "automatic reply",
			input: CoverageConversationAnalysisInput{
				Subject: "Automatic Reply: Meeting request",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "Thank you for your email. I will respond shortly."},
				},
			},
			wantSkip: true,
			wantType: "auto_reply",
		},
		{
			name: "newsletter with unsubscribe",
			input: CoverageConversationAnalysisInput{
				Subject: "Weekly Product Update",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "Check out our latest features! Click here to unsubscribe from this list."},
				},
			},
			wantSkip: true,
			wantType: "newsletter",
		},
		{
			name: "cold outreach with book a call",
			input: CoverageConversationAnalysisInput{
				Subject: "Quick question about your growth",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "Hi, we help companies like yours increase traffic. Let's book a call to discuss!"},
				},
			},
			wantSkip: true,
			wantType: "cold_outreach",
		},
		{
			name: "cold outreach with question mark — not skipped",
			input: CoverageConversationAnalysisInput{
				Subject: "Partnership opportunity",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "Would you be interested in a partnership opportunity?"},
				},
			},
			wantSkip: false,
		},
		{
			name: "newsletter pattern but has question — not skipped",
			input: CoverageConversationAnalysisInput{
				Subject: "Weekly Product Update",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "Can I unsubscribe from this list? How do I manage that?"},
				},
			},
			wantSkip: false,
		},
		{
			name: "real support query",
			input: CoverageConversationAnalysisInput{
				Subject: "Help with billing",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "I was charged twice for my subscription. Can you help?"},
				},
			},
			wantSkip: false,
		},
		{
			name: "prospect pricing question",
			input: CoverageConversationAnalysisInput{
				Subject: "Pricing inquiry",
				Messages: []CoverageConversationMessage{
					{SenderType: "customer", Content: "What are your enterprise pricing plans?"},
				},
			},
			wantSkip: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classification, skip := classifyCoverageConversationLocally(tc.input)
			if skip != tc.wantSkip {
				t.Fatalf("skip = %v, want %v (classification: %+v)", skip, tc.wantSkip, classification)
			}
			if tc.wantSkip && classification.ConversationType != tc.wantType {
				t.Fatalf("ConversationType = %q, want %q", classification.ConversationType, tc.wantType)
			}
		})
	}
}

func TestNormalizeCoverageConversationAnalysisResult_Classification(t *testing.T) {
	cases := []struct {
		name             string
		input            CoverageConversationAnalysisResult
		wantSupport      bool
		wantType         string
		wantHasGap       bool
		wantFixesCleared bool
	}{
		{
			name: "support query passes through",
			input: CoverageConversationAnalysisResult{
				IsSupportQuery:   true,
				ConversationType: "support_query",
				HasGap:           true,
				GapKind:          "content",
				RecommendedFixes: []CoverageRecommendedFix{{Type: "create_article", Priority: "primary"}},
			},
			wantSupport: true,
			wantType:    "support_query",
			wantHasGap:  true,
		},
		{
			name: "newsletter clears gap fields",
			input: CoverageConversationAnalysisResult{
				IsSupportQuery:   false,
				ConversationType: "newsletter",
				HasGap:           true,
				GapKind:          "content",
				RecommendedFixes: []CoverageRecommendedFix{{Type: "create_article"}},
			},
			wantSupport:      false,
			wantType:         "newsletter",
			wantHasGap:       false,
			wantFixesCleared: true,
		},
		{
			name: "empty conversation_type defaults to support_query",
			input: CoverageConversationAnalysisResult{
				ConversationType: "",
				HasGap:           true,
			},
			wantSupport: true,
			wantType:    "support_query",
			wantHasGap:  true,
		},
		{
			name: "unknown conversation_type becomes other",
			input: CoverageConversationAnalysisResult{
				ConversationType: "marketing_blast",
				HasGap:           true,
			},
			wantSupport:      false,
			wantType:         "other",
			wantHasGap:       false,
			wantFixesCleared: true,
		},
		{
			name: "inconsistent: is_support_query=true but type=spam — type wins",
			input: CoverageConversationAnalysisResult{
				IsSupportQuery:   true,
				ConversationType: "spam",
				HasGap:           true,
				RecommendedFixes: []CoverageRecommendedFix{{Type: "create_article"}},
			},
			wantSupport:      false,
			wantType:         "spam",
			wantHasGap:       false,
			wantFixesCleared: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.input
			normalizeCoverageConversationAnalysisResult(&result)
			if result.IsSupportQuery != tc.wantSupport {
				t.Fatalf("IsSupportQuery = %v, want %v", result.IsSupportQuery, tc.wantSupport)
			}
			if result.ConversationType != tc.wantType {
				t.Fatalf("ConversationType = %q, want %q", result.ConversationType, tc.wantType)
			}
			if result.HasGap != tc.wantHasGap {
				t.Fatalf("HasGap = %v, want %v", result.HasGap, tc.wantHasGap)
			}
			if tc.wantFixesCleared && len(result.RecommendedFixes) != 0 {
				t.Fatalf("expected fixes cleared, got %d", len(result.RecommendedFixes))
			}
		})
	}
}

func TestSupportCoverageDailyAnalyzer_AnalyzeConversationClassifiesNonSupport(t *testing.T) {
	response := `{"is_support_query":false,"conversation_type":"cold_outreach","classification_reason":"vendor pitch for SEO services","has_gap":false,"gap_kind":"","gap_category":"","canonical_title":"","customer_need":"","ai_failure":"","human_resolution":"","decision_reason":"","search_query":"","should_run_retrieval":false,"recommended_fixes":[],"confidence":0}`

	provider := &scriptedSupportPlannerLLM{
		responses: []llm.ChatResponse{{Content: response}},
	}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")

	result, _, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		Subject:        "SEO services for your company",
		Messages: []CoverageConversationMessage{
			{ID: "m-1", SenderType: "customer", Content: "Hi, I noticed your website could use some SEO improvements. Book a call with us!"},
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeConversation: %v", err)
	}
	if result.IsSupportQuery {
		t.Fatal("expected IsSupportQuery=false for cold outreach")
	}
	if result.ConversationType != "cold_outreach" {
		t.Fatalf("ConversationType = %q, want cold_outreach", result.ConversationType)
	}
	if result.HasGap {
		t.Fatal("expected HasGap=false for non-support conversation")
	}
}

// --- Segment builder tests ---

func coverageTestMessageIDs(messages []model.SupportMessage) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return ids
}

func TestBuildLatestCoverageConversationSegment_NoLifecycleEventsUsesAllPublicMessages(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "internal", SenderType: "user", MessageType: "reply", Content: "Internal", IsInternal: true, CreatedAt: base},
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "How do I reset password?", CreatedAt: base.Add(time.Minute)},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Try settings.", CreatedAt: base.Add(2 * time.Minute)},
	}

	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		t.Fatal("expected segment")
	}
	if segment.StartMessageID != "m-1" || segment.EndMessageID != "m-2" {
		t.Fatalf("unexpected segment bounds: %+v", segment)
	}
	if segment.Resolved {
		t.Fatal("segment should not be resolved")
	}
	if len(segment.PublicMessages) != 2 {
		t.Fatalf("public messages = %d, want 2", len(segment.PublicMessages))
	}
}

func TestBuildLatestCoverageConversationSegment_ReopenedThreadStartsAtFirstPublicMessageAfterResolved(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	reopened := string(model.SystemEventReopened)
	messages := []model.SupportMessage{
		{ID: "old-customer", SenderType: "customer", MessageType: "reply", Content: "Refund question", CreatedAt: base},
		{ID: "old-agent", SenderType: "user", MessageType: "reply", Content: "Refunded", CreatedAt: base.Add(time.Minute)},
		{ID: "resolved-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved conversation", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "new-customer", SenderType: "customer", MessageType: "reply", Content: "Now I need SSO help", CreatedAt: base.Add(3 * time.Minute)},
		{ID: "reopened-1", SenderType: "user", MessageType: "system", SystemEventType: &reopened, IsInternal: true, Content: "Reopened conversation", CreatedAt: base.Add(4 * time.Minute)},
		{ID: "new-ai", SenderType: "ai", MessageType: "reply", Content: "Let me check SSO docs.", CreatedAt: base.Add(5 * time.Minute)},
	}

	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		t.Fatal("expected segment")
	}
	gotIDs := coverageTestMessageIDs(segment.PublicMessages)
	wantIDs := []string{"new-customer", "new-ai"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("segment messages = %v, want %v", gotIDs, wantIDs)
	}
	if segment.StartMessageID != "new-customer" {
		t.Fatalf("start = %q, want new-customer", segment.StartMessageID)
	}
}

func TestBuildLatestCoverageConversationSegment_ResolvedConversationUsesLatestClosedSegment(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "How do I invite a user?", CreatedAt: base},
		{ID: "m-2", SenderType: "user", MessageType: "reply", Content: "Use Settings > Members.", CreatedAt: base.Add(time.Minute)},
		{ID: "resolved-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved conversation", CreatedAt: base.Add(2 * time.Minute)},
	}

	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		t.Fatal("expected segment")
	}
	if !segment.Resolved {
		t.Fatal("expected latest segment to be marked resolved")
	}
	gotIDs := coverageTestMessageIDs(segment.PublicMessages)
	wantIDs := []string{"m-1", "m-2"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("segment messages = %v, want %v", gotIDs, wantIDs)
	}
}

func TestCoverageSegmentTranscriptHashIncludesSegmentBoundary(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Question", CreatedAt: base},
	}
	first := CoverageConversationSegment{ID: "m-1:resolved-1", StartMessageID: "m-1", EndMessageID: "resolved-1", PublicMessages: messages}
	second := CoverageConversationSegment{ID: "m-1:m-1", StartMessageID: "m-1", EndMessageID: "m-1", PublicMessages: messages}

	if CoverageSegmentTranscriptHash(first) == CoverageSegmentTranscriptHash(second) {
		t.Fatal("segment boundary should affect transcript hash")
	}
}

func TestCoverageConversationAnalysisInputUsesLatestReopenedSegment(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	reopened := string(model.SystemEventReopened)
	conversation := model.SupportConversation{
		ID:          "conversation-1",
		WorkspaceID: "ws-1",
		Subject:     "Mixed thread",
		Status:      model.SupportConversationStatusOpen,
	}
	messages := []model.SupportMessage{
		{ID: "old-customer", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "customer", MessageType: "reply", Content: "Refund issue", CreatedAt: base},
		{ID: "old-agent", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "reply", Content: "Refunded", CreatedAt: base.Add(time.Minute)},
		{ID: "resolved-1", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved conversation", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "new-customer", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "customer", MessageType: "reply", Content: "SSO setup is failing", CreatedAt: base.Add(3 * time.Minute)},
		{ID: "reopened-1", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "system", SystemEventType: &reopened, IsInternal: true, Content: "Reopened conversation", CreatedAt: base.Add(4 * time.Minute)},
		{ID: "new-ai", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "ai", MessageType: "reply", Content: "Try SAML settings.", CreatedAt: base.Add(5 * time.Minute)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	gotIDs := make([]string, 0, len(input.Messages))
	for _, message := range input.Messages {
		gotIDs = append(gotIDs, message.ID)
		if strings.Contains(message.Content, "Refund") {
			t.Fatalf("old segment leaked into analyzer input: %+v", input.Messages)
		}
	}
	wantIDs := []string{"new-customer", "new-ai"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("input message ids = %v, want %v", gotIDs, wantIDs)
	}
	if input.SegmentID == "" || input.SegmentStartMessageID != "new-customer" {
		t.Fatalf("missing segment metadata: %+v", input)
	}
}

func TestCoverageConversationAnalysisInputFiltersRetrievalTracesToSentSegmentMessages(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	conversation := model.SupportConversation{ID: "conversation-1", WorkspaceID: "ws-1"}
	messages := []model.SupportMessage{
		{ID: "old-ai", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "ai", MessageType: "reply", Content: "Old answer", CreatedAt: base},
		{ID: "resolved-1", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved", CreatedAt: base.Add(time.Minute)},
		{ID: "new-customer", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "customer", MessageType: "reply", Content: "New issue", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "new-ai", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "ai", MessageType: "reply", Content: "New answer", CreatedAt: base.Add(3 * time.Minute)},
	}
	traces := []model.SupportAIRetrievalTrace{
		{MessageID: "old-ai", SearchQueries: json.RawMessage(`["refund"]`), Results: json.RawMessage(`[]`), CitedSourceIDs: json.RawMessage(`[]`)},
		{MessageID: "new-ai", SearchQueries: json.RawMessage(`["sso"]`), Results: json.RawMessage(`[]`), CitedSourceIDs: json.RawMessage(`[]`)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, traces)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if len(input.RetrievalTraces) != 1 || input.RetrievalTraces[0].MessageID != "new-ai" {
		t.Fatalf("retrieval traces were not filtered to sent segment messages: %+v", input.RetrievalTraces)
	}
}

func TestCoverageConversationAnalysisInputSetsHasHumanReplyTrue(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	conversation := model.SupportConversation{ID: "c-1", WorkspaceID: "ws-1"}
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Help", CreatedAt: base},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Let me check", CreatedAt: base.Add(time.Minute)},
		{ID: "m-3", SenderType: "user", MessageType: "reply", Content: "Here is the fix", CreatedAt: base.Add(2 * time.Minute)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if !input.HasHumanReply {
		t.Fatal("expected HasHumanReply=true when user messages exist")
	}
}

func TestCoverageConversationAnalysisInputSetsHasHumanReplyFalseWhenNoUserMessages(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	conversation := model.SupportConversation{ID: "c-1", WorkspaceID: "ws-1"}
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Help", CreatedAt: base},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Try this", CreatedAt: base.Add(time.Minute)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if input.HasHumanReply {
		t.Fatal("expected HasHumanReply=false when no user messages exist")
	}
}

func TestCoverageManualReanalysisDoesNotSkipUnchangedConversation(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	repo := repository.NewSupportCoverageAnalysisRepository(db)
	response := llm.ChatResponse{Content: `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"support question","has_gap":false,"gap_kind":"","gap_category":"","canonical_title":"","customer_need":"Billing help","ai_failure":"","human_resolution":"","decision_reason":"Current knowledge covers this","search_query":"","should_run_retrieval":false,"recommended_fixes":[],"confidence":0.9}`}
	provider := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{response, response}}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5").SetCoverageRepositories(repository.NewSupportCoverageRepository(db), repo).SetConversationRepositories(nil, repository.NewSupportMessageRepository(db))
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,message_type,content,created_at) VALUES ('m','ws-1','c','customer','reply','How do I update billing details?',?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	conversation := model.SupportConversation{ID: "c", WorkspaceID: "ws-1", Subject: "Billing help", Status: "open", UpdatedAt: now}
	for _, run := range []string{"first", "automatic-repeat"} {
		if _, err := analyzer.runConversationCoverageAnalysis(context.Background(), "ws-1", run, "", conversation, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := analyzer.runConversationCoverageAnalysis(withCoverageReanalysis(context.Background(), "request"), "ws-1", "manual-repeat", "", conversation, nil); err != nil {
		t.Fatal(err)
	}
	var rows []model.SupportCoverageConversationAnalysis
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].RunID != "manual-repeat" {
		t.Fatalf("did not reassess: %+v", rows)
	}
}
