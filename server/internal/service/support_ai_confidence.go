package service

import "github.com/helpin-ai/helpin/server/internal/repository"

// evaluateConfidence computes a grounded confidence score for support-chat replies.
//
// When retrieval produced chunks, we require strong retrieval quality plus explicit citation coverage.
// When retrieval produced no chunks, only conversational turns like greetings or
// safe limitation/redirect responses for out-of-scope questions should pass.
func evaluateConfidence(searchResults []repository.DocsChunkSearchResult, response *AIResponseContract) float64 {
	llmConfidence := clamp01(response.Confidence)

	canAnswerScore := 0.0
	if response.CanAnswer {
		canAnswerScore = 1.0
	}

	if len(searchResults) == 0 {
		return (llmConfidence * 0.65) + (canAnswerScore * 0.35)
	}

	bestVector := 0.0
	bestLexical := 0.0
	retrievedDocs := map[string]struct{}{}
	for _, result := range searchResults {
		if result.VectorScore > bestVector {
			bestVector = result.VectorScore
		}
		if result.LexicalScore > bestLexical {
			bestLexical = result.LexicalScore
		}
		retrievedDocs[result.DocumentID] = struct{}{}
	}

	// ts_rank scores are typically small; normalize them into a 0-1 band.
	normalizedLexical := clamp01(bestLexical / 0.35)
	retrievalQuality := maxFloat(bestVector, normalizedLexical)

	citedDocs := map[string]struct{}{}
	for _, docID := range response.SourceDocIDs {
		if _, ok := retrievedDocs[docID]; ok {
			citedDocs[docID] = struct{}{}
		}
	}

	sourceCoverage := 0.0
	if len(retrievedDocs) > 0 {
		sourceCoverage = clamp01(float64(len(citedDocs)) / float64(minInt(len(retrievedDocs), 3)))
	}

	return (retrievalQuality * 0.4) +
		(sourceCoverage * 0.25) +
		(llmConfidence * 0.2) +
		(canAnswerScore * 0.15)
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
