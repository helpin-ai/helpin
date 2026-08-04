package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func helpcenterTestChunks() []repository.DocsChunkSearchResult {
	return []repository.DocsChunkSearchResult{
		{ID: "chunk-1", DocumentID: "doc-1", Title: "Pricing", Content: "The Growth plan costs $84 per month.", CombinedScore: 0.9},
		{ID: "chunk-2", DocumentID: "doc-1", Title: "Pricing", Content: "Annual billing saves 20%.", CombinedScore: 0.7},
		{ID: "chunk-3", DocumentID: "doc-2", Title: "Setup", Content: "Install the snippet in your site head.", CombinedScore: 0.8},
	}
}

func helpcenterTestRefs() map[string]model.PublicSearchResultResponse {
	return map[string]model.PublicSearchResultResponse{
		"doc-1": {ID: "doc-1", Title: "Pricing", Slug: "pricing", PublicID: "pub-1", SpaceSlug: "help"},
		"doc-2": {ID: "doc-2", Title: "Setup", Slug: "setup", PublicID: "pub-2", SpaceSlug: "help"},
	}
}

func TestValidateHelpcenterAnswer(t *testing.T) {
	chunks := helpcenterTestChunks()
	refs := helpcenterTestRefs()

	tests := []struct {
		name          string
		contract      helpcenterAnswerContract
		wantValid     bool
		wantCitations int
	}{
		{
			name: "grounded answer passes with deduped citations",
			contract: helpcenterAnswerContract{
				CanAnswer:     true,
				Answer:        "The Growth plan costs $84 per month; annual billing saves 20%.",
				CitedChunkIDs: []string{"chunk-1", "chunk-2"},
				Confidence:    0.9,
			},
			wantValid:     true,
			wantCitations: 1,
		},
		{
			name: "citations across documents",
			contract: helpcenterAnswerContract{
				CanAnswer:     true,
				Answer:        "Install the snippet; the Growth plan costs $84.",
				CitedChunkIDs: []string{"chunk-3", "chunk-1"},
				Confidence:    0.8,
			},
			wantValid:     true,
			wantCitations: 2,
		},
		{
			name: "unknown chunk id rejected",
			contract: helpcenterAnswerContract{
				CanAnswer:     true,
				Answer:        "Something",
				CitedChunkIDs: []string{"chunk-999"},
				Confidence:    0.9,
			},
		},
		{
			name: "cannot answer rejected",
			contract: helpcenterAnswerContract{
				CanAnswer:     false,
				CitedChunkIDs: []string{"chunk-1"},
				Confidence:    0.9,
			},
		},
		{
			name: "low confidence rejected",
			contract: helpcenterAnswerContract{
				CanAnswer:     true,
				Answer:        "The Growth plan costs $84.",
				CitedChunkIDs: []string{"chunk-1"},
				Confidence:    0.3,
			},
		},
		{
			name: "no citations rejected",
			contract: helpcenterAnswerContract{
				CanAnswer:  true,
				Answer:     "The Growth plan costs $84.",
				Confidence: 0.9,
			},
		},
		{
			name: "raw link smuggled into answer rejected",
			contract: helpcenterAnswerContract{
				CanAnswer:     true,
				Answer:        "See https://evil.example.com/pricing for details.",
				CitedChunkIDs: []string{"chunk-1"},
				Confidence:    0.9,
			},
		},
		{
			name: "markdown link smuggled into answer rejected",
			contract: helpcenterAnswerContract{
				CanAnswer:     true,
				Answer:        "See [pricing](javascript:alert(1)) for details.",
				CitedChunkIDs: []string{"chunk-1"},
				Confidence:    0.9,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			citations, valid := validateHelpcenterAnswer(&tt.contract, chunks, refs)
			if valid != tt.wantValid {
				t.Fatalf("validateHelpcenterAnswer() valid = %v, want %v", valid, tt.wantValid)
			}
			if valid && len(citations) != tt.wantCitations {
				t.Errorf("citations = %d, want %d", len(citations), tt.wantCitations)
			}
		})
	}
}

func TestValidateHelpcenterAnswerRejectsChunkWithoutPublishedArticle(t *testing.T) {
	chunks := helpcenterTestChunks()
	refs := helpcenterTestRefs()
	delete(refs, "doc-2")
	contract := helpcenterAnswerContract{
		CanAnswer:     true,
		Answer:        "Install the snippet in your site head.",
		CitedChunkIDs: []string{"chunk-3"},
		Confidence:    0.9,
	}
	if _, valid := validateHelpcenterAnswer(&contract, chunks, refs); valid {
		t.Fatal("expected chunk without a published article ref to be rejected")
	}
}

func TestHelpcenterAnswerCacheKeyNormalizesQueries(t *testing.T) {
	a := helpcenterAnswerCacheKey("ws-1", "en", "", "How  Much does IT cost?", "10:x")
	b := helpcenterAnswerCacheKey("ws-1", "en", "", "how much does it cost?", "10:x")
	if a != b {
		t.Error("expected whitespace/case-insensitive cache key match")
	}
	c := helpcenterAnswerCacheKey("ws-1", "en", "", "how much does it cost?", "11:y")
	if a == c {
		t.Error("expected content fingerprint change to change the cache key")
	}
	d := helpcenterAnswerCacheKey("ws-1", "de", "", "how much does it cost?", "10:x")
	if a == d {
		t.Error("expected locale to partition the cache")
	}
}

func TestGroupChunksByDocumentOrdersByBestScore(t *testing.T) {
	groups := groupChunksByDocument(helpcenterTestChunks())
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	if groups[0].documentID != "doc-1" || groups[1].documentID != "doc-2" {
		t.Fatalf("unexpected order: %s, %s", groups[0].documentID, groups[1].documentID)
	}
	if len(groups[0].chunks) != 2 {
		t.Fatalf("doc-1 chunks = %d, want 2", len(groups[0].chunks))
	}
}

func TestConsumeHelpcenterAnswerBudgetFailsOpenWithoutRedis(t *testing.T) {
	svc := &HelpcenterAISearchService{}
	if !svc.consumeAnswerBudget(t.Context(), "ws-1") {
		t.Fatal("expected nil-redis budget check to fail open")
	}
}

func TestResolveHelpcenterAnswerRouting(t *testing.T) {
	tests := []struct {
		name             string
		explicitProvider string
		explicitModel    string
		openrouter       bool
		openai           bool
		anthropic        bool
		wantProvider     string
		wantModel        string
	}{
		{
			name:             "explicit provider and model win",
			explicitProvider: "openai",
			explicitModel:    "gpt-5.6-terra",
			wantProvider:     "openai",
			wantModel:        "gpt-5.6-terra",
		},
		{
			name:             "explicit provider gets its cheap default model",
			explicitProvider: "anthropic",
			anthropic:        true,
			wantProvider:     "anthropic",
			wantModel:        "claude-haiku-4-5",
		},
		{
			name:         "openrouter key preferred",
			openrouter:   true,
			openai:       true,
			anthropic:    true,
			wantProvider: "openrouter",
			wantModel:    "deepseek/deepseek-v4-flash-0731",
		},
		{
			name:         "openai key next",
			openai:       true,
			anthropic:    true,
			wantProvider: "openai",
			wantModel:    "gpt-5-mini",
		},
		{
			name:         "anthropic key last",
			anthropic:    true,
			wantProvider: "anthropic",
			wantModel:    "claude-haiku-4-5",
		},
		{
			name: "no keys disables generation",
		},
		{
			name:          "explicit model without provider follows key chain",
			explicitModel: "custom/model",
			openai:        true,
			wantProvider:  "openai",
			wantModel:     "custom/model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, modelName := ResolveHelpcenterAnswerRouting(tt.explicitProvider, tt.explicitModel, tt.openrouter, tt.openai, tt.anthropic)
			if provider != tt.wantProvider || modelName != tt.wantModel {
				t.Errorf("ResolveHelpcenterAnswerRouting() = %q/%q, want %q/%q", provider, modelName, tt.wantProvider, tt.wantModel)
			}
		})
	}
}
