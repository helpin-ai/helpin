package service

// Evidence ranking for support retrieval: cross-encoder rerank, chunk/
// document dedupe, official-domain authority boosts, and canonical-source
// filtering for pricing queries (third-party pages must not outrank the
// vendor's own pricing page as grounding evidence).

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func rerankKnowledgeResults(query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if len(results) == 0 {
		return nil
	}

	queryTerms := normalizedTerms(query)
	reranked := make([]KnowledgeSearchResult, len(results))
	copy(reranked, results)

	for idx := range reranked {
		lexical := clamp01(reranked[idx].LexicalScore / 0.35)
		bodyOverlap := termOverlapScore(queryTerms, reranked[idx].Content)
		titleOverlap := termOverlapScore(queryTerms, reranked[idx].Title)
		reranked[idx].CombinedScore = (reranked[idx].VectorScore * 0.35) +
			(lexical * 0.2) +
			(bodyOverlap * 0.25) +
			(titleOverlap * 0.15) +
			(reranked[idx].CombinedScore * 4)
		if reranked[idx].SourceType == knowledgeSourceTypeGuidance {
			reranked[idx].CombinedScore += 100
		}
	}

	sort.SliceStable(reranked, func(i, j int) bool {
		return reranked[i].CombinedScore > reranked[j].CombinedScore
	})

	return reranked
}

func limitKnowledgeResultsPerDocument(results []KnowledgeSearchResult, limit int) []KnowledgeSearchResult {
	if limit <= 0 || len(results) == 0 {
		return results
	}
	byDocCount := map[string]int{}
	limited := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		documentKey := knowledgeResultDocumentKey(result)
		if byDocCount[documentKey] >= limit {
			continue
		}
		byDocCount[documentKey]++
		limited = append(limited, result)
	}
	return limited
}

func dedupeKnowledgeResults(results []KnowledgeSearchResult) []KnowledgeSearchResult {
	seen := make(map[string]int, len(results))
	deduped := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		key := knowledgeResultChunkKey(result)
		if idx, ok := seen[key]; ok {
			if result.CombinedScore > deduped[idx].CombinedScore {
				deduped[idx] = result
			}
			continue
		}
		seen[key] = len(deduped)
		deduped = append(deduped, result)
	}
	return deduped
}

func knowledgeResultChunkKey(result KnowledgeSearchResult) string {
	if normalizedURL := normalizeKnowledgeResultURL(result.URL); normalizedURL != "" {
		return fmt.Sprintf("url:%s|chunk:%d|section:%s", normalizedURL, result.ChunkIndex, strings.TrimSpace(result.SectionKey))
	}
	if strings.TrimSpace(result.ReferenceID) != "" {
		return fmt.Sprintf("ref:%s|chunk:%d|section:%s", strings.TrimSpace(result.ReferenceID), result.ChunkIndex, strings.TrimSpace(result.SectionKey))
	}
	return "id:" + result.ID
}

func knowledgeResultDocumentKey(result KnowledgeSearchResult) string {
	if normalizedURL := normalizeKnowledgeResultURL(result.URL); normalizedURL != "" {
		return "url:" + normalizedURL
	}
	if strings.TrimSpace(result.ReferenceID) != "" {
		return "ref:" + strings.TrimSpace(result.ReferenceID)
	}
	return "id:" + result.ID
}

func normalizeKnowledgeResultURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Path != "/" {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	return parsed.String()
}

func applyKnowledgeAuthorityRanking(query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if len(results) < 2 || !isPricingKnowledgeQuery(query) {
		return results
	}
	reranked := append([]KnowledgeSearchResult(nil), results...)
	for idx := range reranked {
		switch {
		case isCanonicalPricingURL(reranked[idx].URL):
			reranked[idx].CombinedScore += 5
		case isArticleKnowledgeURL(reranked[idx].URL):
			reranked[idx].CombinedScore -= 1
		}
	}
	sort.SliceStable(reranked, func(i, j int) bool {
		return reranked[i].CombinedScore > reranked[j].CombinedScore
	})
	return reranked
}

// filterConflictingPricingEvidence prevents a stale high-overlap marketing
// page from supplying a different price when the same retrieval already found
// the canonical pricing page. Comparative questions keep both sides because
// the visitor explicitly asked for a comparison.
func filterConflictingPricingEvidence(query string, results []KnowledgeSearchResult) []KnowledgeSearchResult {
	if len(results) < 2 || !isPricingKnowledgeQuery(query) || isComparativeKnowledgeQuery(query) {
		return results
	}
	hasCanonicalPricing := false
	for _, result := range results {
		if isCanonicalPricingURL(result.URL) {
			hasCanonicalPricing = true
			break
		}
	}
	if !hasCanonicalPricing {
		return results
	}
	filtered := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		if result.IsInternal || isCanonicalPricingURL(result.URL) || !currencyValuePattern.MatchString(result.Content) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func isPricingKnowledgeQuery(query string) bool {
	for _, term := range normalizedTerms(query) {
		switch term {
		case "price", "prices", "pricing", "cost", "costs", "plan", "plans", "subscription", "subscriptions", "billing":
			return true
		}
	}
	return false
}

func isComparativeKnowledgeQuery(query string) bool {
	for _, term := range normalizedTerms(query) {
		switch term {
		case "compare", "compared", "comparison", "comparisons", "versus", "vs", "alternative", "alternatives":
			return true
		}
	}
	return false
}

func isCanonicalPricingURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || isArticleKnowledgeURL(rawURL) {
		return false
	}
	segments := strings.Split(strings.Trim(strings.ToLower(parsed.Path), "/"), "/")
	if len(segments) == 0 {
		return false
	}
	switch segments[len(segments)-1] {
	case "pricing", "plans", "pricing-plans", "plans-pricing", "subscriptions":
		return true
	default:
		return false
	}
}

func isArticleKnowledgeURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	path := "/" + strings.Trim(strings.ToLower(parsed.Path), "/") + "/"
	for _, segment := range []string{"/blog/", "/blogs/", "/article/", "/articles/", "/news/", "/guides/", "/compare/", "/comparison/", "/comparisons/"} {
		if strings.Contains(path, segment) {
			return true
		}
	}
	return false
}
