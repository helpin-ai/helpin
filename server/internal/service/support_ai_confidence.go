package service

import (
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// evaluateConfidence computes a multi-signal confidence score.
//
// When search results exist (RAG mode):
//
//	retrieval_quality=0.4, llm_confidence=0.3, source_coverage=0.1, can_answer=0.2
//
// When no search results (conversational mode — greetings, follow-ups, etc.):
//
//	llm_confidence=0.6, can_answer=0.4
//
// This prevents greetings and conversational replies from being penalized
// by zero retrieval scores when no knowledge base lookup was needed.
func evaluateConfidence(searchResults []repository.DocsSearchResult, response *AIResponseContract) float64 {
	llmConfidence := response.Confidence
	if llmConfidence < 0 {
		llmConfidence = 0
	}
	if llmConfidence > 1 {
		llmConfidence = 1
	}

	var canAnswerScore float64
	if response.CanAnswer {
		canAnswerScore = 1.0
	}

	// Conversational mode — no search results, rely on LLM self-assessment.
	if len(searchResults) == 0 {
		return (llmConfidence * 0.6) + (canAnswerScore * 0.4)
	}

	// RAG mode — multi-signal with retrieval quality.
	var retrievalQuality float64
	bestRank := searchResults[0].Rank
	for _, r := range searchResults[1:] {
		if r.Rank > bestRank {
			bestRank = r.Rank
		}
	}
	if bestRank < 0.1 {
		retrievalQuality = 0.0
	} else if bestRank > 1.0 {
		retrievalQuality = 1.0
	} else {
		retrievalQuality = bestRank
	}

	var sourceCoverage float64
	sourceCoverage = float64(len(response.SourceDocIDs)) / float64(len(searchResults))
	if sourceCoverage > 1 {
		sourceCoverage = 1
	}

	return (retrievalQuality * 0.4) +
		(llmConfidence * 0.3) +
		(sourceCoverage * 0.1) +
		(canAnswerScore * 0.2)
}
