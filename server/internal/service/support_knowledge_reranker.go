package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

type SupportRerankCandidate struct {
	ID   string
	Text string
}

type SupportRerankScore struct {
	Index int
	Score float64
}

// SupportKnowledgeReranker is implemented by a fixed-cost cross-encoder.
type SupportKnowledgeReranker interface {
	Rerank(ctx context.Context, query string, candidates []SupportRerankCandidate) ([]SupportRerankScore, error)
	Name() string
}

// HTTPSupportKnowledgeReranker speaks the common text-embeddings-inference
// rerank contract: POST {query,texts,raw_scores,return_text,truncate}.
type HTTPSupportKnowledgeReranker struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

func NewHTTPSupportKnowledgeReranker(endpoint, model, apiKey string) *HTTPSupportKnowledgeReranker {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return nil
	}
	return &HTTPSupportKnowledgeReranker{
		endpoint: endpoint,
		model:    strings.TrimSpace(model),
		apiKey:   strings.TrimSpace(apiKey),
		client:   &http.Client{Timeout: 250 * time.Millisecond},
	}
}

func (r *HTTPSupportKnowledgeReranker) Name() string {
	if r == nil {
		return ""
	}
	if r.model != "" {
		return r.model
	}
	return "cross_encoder"
}

func (r *HTTPSupportKnowledgeReranker) Rerank(ctx context.Context, query string, candidates []SupportRerankCandidate) ([]SupportRerankScore, error) {
	if r == nil || r.endpoint == "" || len(candidates) == 0 {
		return nil, nil
	}
	texts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		texts = append(texts, candidate.Text)
	}
	payload := map[string]any{
		"query":       query,
		"texts":       texts,
		"raw_scores":  false,
		"return_text": false,
		"truncate":    true,
	}
	if r.model != "" {
		payload["model"] = r.model
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal rerank request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create rerank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read rerank response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rerank returned status %d", resp.StatusCode)
	}
	var direct []struct {
		Index          int     `json:"index"`
		Score          float64 `json:"score"`
		RelevanceScore float64 `json:"relevance_score"`
	}
	if err := json.Unmarshal(responseBody, &direct); err != nil {
		var wrapped struct {
			Results []struct {
				Index          int     `json:"index"`
				Score          float64 `json:"score"`
				RelevanceScore float64 `json:"relevance_score"`
			} `json:"results"`
		}
		if wrappedErr := json.Unmarshal(responseBody, &wrapped); wrappedErr != nil {
			return nil, fmt.Errorf("parse rerank response: %w", err)
		}
		scores := make([]SupportRerankScore, 0, len(wrapped.Results))
		for _, result := range wrapped.Results {
			score := result.Score
			if score == 0 {
				score = result.RelevanceScore
			}
			scores = append(scores, SupportRerankScore{Index: result.Index, Score: score})
		}
		return scores, nil
	}
	scores := make([]SupportRerankScore, 0, len(direct))
	for _, result := range direct {
		score := result.Score
		if score == 0 {
			score = result.RelevanceScore
		}
		scores = append(scores, SupportRerankScore{Index: result.Index, Score: score})
	}
	return scores, nil
}

func (s *SupportAIService) semanticRerankKnowledgeResults(ctx context.Context, query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if s == nil || s.knowledgeReranker == nil || len(results) < 2 || strings.TrimSpace(query) == "" {
		return results
	}
	if len(results) > 40 {
		results = results[:40]
	}
	candidates := make([]SupportRerankCandidate, 0, len(results))
	for _, result := range results {
		candidates = append(candidates, SupportRerankCandidate{
			ID: result.ID,
			Text: strings.Join(nonEmptyStrings([]string{
				result.Title,
				result.HeadingPath,
				excerptText(result.Content, 900),
			}), "\n"),
		})
	}
	startedAt := time.Now()
	rerankCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	scores, err := s.knowledgeReranker.Rerank(rerankCtx, query, candidates)
	if err != nil {
		slog.WarnContext(ctx, "support semantic reranker failed; retaining fused order",
			"reranker", s.knowledgeReranker.Name(),
			"latency_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return results
	}
	slog.InfoContext(ctx, "support semantic reranker completed",
		"reranker", s.knowledgeReranker.Name(),
		"candidate_count", len(candidates),
		"score_count", len(scores),
		"latency_ms", time.Since(startedAt).Milliseconds(),
	)
	for idx := range results {
		results[idx].CombinedScore = -1
	}
	for _, score := range scores {
		if score.Index < 0 || score.Index >= len(results) {
			continue
		}
		results[score.Index].CombinedScore = score.Score
	}
	for idx := range results {
		if results[idx].SourceType == knowledgeSourceTypeGuidance {
			results[idx].CombinedScore += 100
		}
	}
	sort.SliceStable(results, func(left, right int) bool {
		return results[left].CombinedScore > results[right].CombinedScore
	})
	return results
}
