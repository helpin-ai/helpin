package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const coverageTestGapJSON = `{"is_support_query":true,"conversation_type":"support_query","classification_reason":"customer asks a billing support question","has_gap":true,"gap_kind":"content","gap_category":"knowledge","canonical_title":"Billing update steps","customer_need":"Customer needed billing update steps.","ai_failure":"AI did not know the billing update flow.","human_resolution":"A human explained the billing flow.","decision_reason":"The missing answer should be documented.","search_query":"","should_run_retrieval":false,"recommended_fixes":[{"type":"create_article","target_type":"docs","target_id":"","target_title":"Billing update steps","target_url":"","priority":"primary","rationale":"Customers need documented billing steps.","suggested_change":"Create a billing update article.","implementation_notes":"Use the product billing settings flow."}],"confidence":0.82}`

func TestAnalyzeConversationFallsBackWhenPrimaryReplyIsUnusable(t *testing.T) {
	tests := []struct {
		name    string
		primary llm.ChatResponse
	}{
		{name: "empty reply", primary: llm.ChatResponse{Content: ""}},
		{name: "reply cut off by the token limit", primary: llm.ChatResponse{Content: `{"is_support_query":true,"has_gap":`, FinishReason: "length"}},
		{name: "unreadable reply", primary: llm.ChatResponse{Content: "not json"}},
		{name: "empty JSON object is not a no-gap decision", primary: llm.ChatResponse{Content: `{}`}},
		{name: "missing gap decision", primary: llm.ChatResponse{Content: strings.Replace(coverageTestGapJSON, `"has_gap":true,`, "", 1)}},
		{name: "null gap decision", primary: llm.ChatResponse{Content: strings.Replace(coverageTestGapJSON, `"has_gap":true`, `"has_gap":null`, 1)}},
		{name: "unknown classification", primary: llm.ChatResponse{Content: strings.Replace(coverageTestGapJSON, `"conversation_type":"support_query"`, `"conversation_type":"unknown"`, 1)}},
		{name: "missing recommended change", primary: llm.ChatResponse{Content: strings.Replace(coverageTestGapJSON, `"suggested_change":"Create a billing update article."`, `"suggested_change":""`, 1)}},
		{name: "whitespace rationale", primary: llm.ChatResponse{Content: strings.Replace(coverageTestGapJSON, `"rationale":"Customers need documented billing steps."`, `"rationale":"  "`, 1)}},
		{name: "missing customer need", primary: llm.ChatResponse{Content: strings.Replace(coverageTestGapJSON, `"customer_need":"Customer needed billing update steps."`, `"customer_need":""`, 1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{tt.primary, {Content: coverageTestGapJSON}}}
			analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")

			result, _, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{
				WorkspaceID: "ws-1", ConversationID: "conversation-1",
				Messages: []CoverageConversationMessage{{ID: "m-1", SenderType: "customer", Content: "How do I update billing?"}},
			})

			if err != nil {
				t.Fatalf("AnalyzeConversation: %v", err)
			}
			if result == nil || !result.HasGap || result.CustomerNeed == "" {
				t.Fatalf("fallback result = %+v", result)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("model calls = %d, want primary then fallback", len(provider.requests))
			}
			if provider.requests[0].MaxTokens < 4000 {
				t.Fatalf("max tokens = %d, want room for reasoning models", provider.requests[0].MaxTokens)
			}
		})
	}
}

// The completion service rejects a request above the route or action ceiling,
// so raising the analyzer's limit must raise both.
func TestCoverageOutputLimitsFitRouteAndActionCeilings(t *testing.T) {
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureCoverageGapAnalysis, "")
	if !ok {
		t.Fatal("missing coverage route policy")
	}
	for name, limit := range map[string]int{"analyze": coverageAnalyzeMaxTokens, "refine": coverageRefineMaxTokens} {
		if limit > policy.MaximumOutputTokens {
			t.Errorf("%s limit %d exceeds route ceiling %d", name, limit, policy.MaximumOutputTokens)
		}
	}
	registry := aipolicy.DefaultRegistry()
	for key, limit := range map[string]int{aipolicy.ActionSupportCoverageAnalyze: coverageAnalyzeMaxTokens, aipolicy.ActionSupportCoverageRefine: coverageRefineMaxTokens} {
		action, ok := registry.Lookup(key)
		if !ok {
			t.Fatalf("missing action %s", key)
		}
		if limit > action.MaxOutputTokens {
			t.Errorf("%s limit %d exceeds action ceiling %d", key, limit, action.MaxOutputTokens)
		}
	}
}

type sequentialCoverageEmbeddings struct {
	calls   int
	vectors [][]float32
}

func (f *sequentialCoverageEmbeddings) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	vectors := make([][]float32, 0, len(req.Inputs))
	for range req.Inputs {
		vectors = append(vectors, f.vectors[f.calls%len(f.vectors)])
		f.calls++
	}
	return &llm.EmbeddingResponse{Vectors: vectors}, nil
}

// needMatchLLM answers the need judge: it picks the first candidate when
// match is true, and "none" otherwise.
type needMatchLLM struct {
	match bool
	calls int
}

func (f *needMatchLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	var input struct {
		Candidates []struct {
			ID string `json:"id"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(req.Messages[0].Content), &input); err != nil || len(input.Candidates) == 0 {
		return &llm.ChatResponse{Content: `{"match_id":"none","confidence":0.9}`}, nil
	}
	if !f.match {
		return &llm.ChatResponse{Content: `{"match_id":"none","confidence":0.9}`}, nil
	}
	return &llm.ChatResponse{Content: fmt.Sprintf(`{"match_id":%q,"confidence":0.9}`, input.Candidates[0].ID)}, nil
}

func TestCoverageMaterializerAsksJudgeForCloseMatches(t *testing.T) {
	tests := []struct {
		name     string
		match    bool
		wantGaps int64
	}{
		{name: "judged the same need attaches to the existing gap", match: true, wantGaps: 1},
		{name: "judged a different need becomes its own gap", match: false, wantGaps: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupCoverageFindingUpsertTestDB(t)
			analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
			coverageRepo := repository.NewSupportCoverageRepository(db)
			judge := &needMatchLLM{match: tt.match}
			// cosine({1,0,0}, {0.8,0.6,0}) = 0.8: close, but below automatic attach.
			analyzer := NewSupportCoverageDailyAnalyzer(judge, "", "").
				SetCoverageRepositories(coverageRepo, analysisRepo).
				SetEmbeddingProvider(&sequentialCoverageEmbeddings{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "")
			ctx := context.Background()
			now := time.Now()

			seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "run-1", "conversation-1", "Customers need to reconnect Instagram after a token expires.", "Reconnect Instagram", now)
			if _, err := analyzer.materializePendingFindings(ctx, "ws-1"); err != nil {
				t.Fatalf("first materialization: %v", err)
			}
			seedMaterializerAnalysis(t, analysisRepo, "analysis-2", "run-2", "conversation-2", "Customer's Instagram account keeps disconnecting and needs reconnecting.", "Instagram disconnects", now.Add(time.Minute))
			if _, err := analyzer.materializePendingFindings(ctx, "ws-1"); err != nil {
				t.Fatalf("second materialization: %v", err)
			}

			var gaps int64
			if err := db.Model(&model.SupportCoverageGap{}).Count(&gaps).Error; err != nil {
				t.Fatal(err)
			}
			if gaps != tt.wantGaps {
				t.Fatalf("gaps = %d, want %d", gaps, tt.wantGaps)
			}
			if judge.calls != 1 {
				t.Fatalf("judge calls = %d, want 1", judge.calls)
			}
		})
	}
}

func TestCoverageMaterializerPicksUpFindingsFromEarlierRuns(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	analyzer := NewSupportCoverageDailyAnalyzer(nil, "", "").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetEmbeddingProvider(&fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}}}, "")
	// A retried attempt records its analysis under the batch id, not the run id.
	seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "batch-from-retry", "conversation-1", "Customers need refund exception rules.", "Refund exception criteria", time.Now())

	result, err := analyzer.materializePendingFindings(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("materializePendingFindings: %v", err)
	}
	if result.NewGapsCreated != 1 {
		t.Fatalf("NewGapsCreated = %d, want 1", result.NewGapsCreated)
	}
}

func TestCoverageDuplicateCheckSettlesSuggestionsWithJudge(t *testing.T) {
	tests := []struct {
		name             string
		match            bool
		wantMerged       int
		wantKeptSeparate int
		wantSourceOpen   bool
	}{
		{name: "same need is merged", match: true, wantMerged: 1, wantSourceOpen: false},
		{name: "different need is kept separate", match: false, wantKeptSeparate: 1, wantSourceOpen: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, db := setupCoverageTestEnv(t)
			repo := repository.NewSupportCoverageRepository(db)
			judge := &needMatchLLM{match: tt.match}
			svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "").
				SetNeedJudge(judge, nil)
			ctx := context.Background()
			now := time.Now()
			for _, gap := range []model.SupportCoverageGap{
				{ID: "gap-reset", DedupeKey: "reset", Title: "Users cannot reset passwords", EvidenceCount: 3},
				{ID: "gap-password", DedupeKey: "password", Title: "Password reset instructions are missing", EvidenceCount: 2},
			} {
				gap.WorkspaceID, gap.GapKind, gap.GapCategory = "ws-1", "content", model.SupportCoverageGapCategoryKnowledge
				gap.Status, gap.Confidence, gap.FirstSeenAt, gap.LastSeenAt = model.SupportCoverageGapStatusOpen, 0.8, now.Add(-time.Hour), now
				seedCoverageGapForRebuild(t, db, gap)
			}

			result, err := svc.RebuildWorkspace(ctx, "ws-1")
			if err != nil {
				t.Fatalf("RebuildWorkspace: %v", err)
			}

			if result.AutoMerged != tt.wantMerged || result.KeptSeparate != tt.wantKeptSeparate {
				t.Fatalf("merged=%d kept separate=%d, want %d and %d", result.AutoMerged, result.KeptSeparate, tt.wantMerged, tt.wantKeptSeparate)
			}
			if result.SuggestionsCreated != 0 {
				t.Fatalf("suggestions left for review = %d, want 0", result.SuggestionsCreated)
			}
			var source model.SupportCoverageGap
			if err := db.First(&source, "id = ?", "gap-password").Error; err != nil {
				t.Fatal(err)
			}
			if (source.Status == model.SupportCoverageGapStatusOpen) != tt.wantSourceOpen {
				t.Fatalf("source status = %q, want open=%v", source.Status, tt.wantSourceOpen)
			}

			again, err := svc.RebuildWorkspace(ctx, "ws-1")
			if err != nil {
				t.Fatalf("second RebuildWorkspace: %v", err)
			}
			if again.SuggestionsCreated != 0 || judge.calls != 1 {
				t.Fatalf("second check: pending=%d judge calls=%d, want 0 and 1", again.SuggestionsCreated, judge.calls)
			}
		})
	}
}

func TestCoverageDuplicateCheckLeavesUnsureSuggestionsForReview(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	unsure := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{{Content: `{"match_id":"none","confidence":0.4}`}}}
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "").
		SetNeedJudge(unsure, nil)
	now := time.Now()
	for _, gap := range []model.SupportCoverageGap{
		{ID: "gap-reset", DedupeKey: "reset", Title: "Users cannot reset passwords", EvidenceCount: 3},
		{ID: "gap-password", DedupeKey: "password", Title: "Password reset instructions are missing", EvidenceCount: 2},
	} {
		gap.WorkspaceID, gap.GapKind, gap.GapCategory = "ws-1", "content", model.SupportCoverageGapCategoryKnowledge
		gap.Status, gap.Confidence, gap.FirstSeenAt, gap.LastSeenAt = model.SupportCoverageGapStatusOpen, 0.8, now.Add(-time.Hour), now
		seedCoverageGapForRebuild(t, db, gap)
	}

	result, err := svc.RebuildWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.SuggestionsCreated != 1 || result.AutoMerged != 0 || result.KeptSeparate != 0 {
		t.Fatalf("result = %+v, want one suggestion left for review", result)
	}
}

func TestCoverageRefinementFallsBackBeforeReplacingValidRecommendations(t *testing.T) {
	var original CoverageConversationAnalysisResult
	if err := json.Unmarshal([]byte(coverageTestGapJSON), &original); err != nil {
		t.Fatal(err)
	}
	good, err := json.Marshal(CoverageFixBundleDecision{RecommendedFixes: original.RecommendedFixes, Confidence: 0.85})
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{
		{Content: `{"recommended_fixes":[{"type":"update_article","target_type":"docs"}],"confidence":0.9}`},
		{Content: string(good)},
	}}
	analyzer := NewSupportCoverageDailyAnalyzer(provider, "openai", "gpt-5.5")
	got, err := analyzer.RefineFixBundleWithKnowledge(context.Background(), original, []CoverageKnowledgeCandidate{{TargetType: "docs", DocumentID: "doc-1", Title: "Billing"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 2 || len(got.RecommendedFixes) != 1 || got.RecommendedFixes[0].SuggestedChange == "" {
		t.Fatalf("invalid refinement accepted: calls=%d result=%+v", len(provider.requests), got)
	}
}
