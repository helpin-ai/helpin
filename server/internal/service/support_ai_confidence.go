package service

// evaluateConfidence computes a grounded confidence score for support-chat replies.
//
// When retrieval produced chunks, we require strong retrieval quality plus explicit citation coverage.
// When retrieval produced no chunks, only conversational turns like greetings or
// safe limitation/redirect responses for out-of-scope questions should pass.
//
// isGreeting marks replies to messages like "hi"/"hello" that the system prompt
// instructs the model to answer without citing sources. Vector search has no
// similarity floor, so it can still return spurious low-relevance chunks for a
// bare greeting; grounding the confidence score against those chunks would
// penalize a correctly-uncited greeting reply. Skip the grounded formula for
// this case the same way we do when retrieval found nothing.
func evaluateConfidence(searchResults []KnowledgeSearchResult, response *AIResponseContract, isGreeting bool) float64 {
	llmConfidence := clamp01(response.Confidence)

	canAnswerScore := 0.0
	if response.CanAnswer {
		canAnswerScore = 1.0
	}

	if len(searchResults) == 0 || isGreeting {
		return (llmConfidence * 0.65) + (canAnswerScore * 0.35)
	}

	retrievalQuality := 0.0
	retrievedPublicDocs := map[string]struct{}{}
	hasInternalGrounding := false
	citedEvidence := map[string]struct{}{}
	for _, docID := range response.SourceDocIDs {
		citedEvidence[docID] = struct{}{}
	}
	for _, claim := range response.Claims {
		for _, evidenceID := range claim.EvidenceIDs {
			citedEvidence[evidenceID] = struct{}{}
		}
	}
	for _, result := range searchResults {
		_, citesRuntimeID := citedEvidence[result.ID]
		_, citesReferenceID := citedEvidence[result.ReferenceID]
		if len(citedEvidence) > 0 && !citesRuntimeID && !citesReferenceID {
			continue
		}
		if quality := supportEvidenceRetrievalQuality(result); quality > retrievalQuality {
			retrievalQuality = quality
		}
		if result.IsInternal {
			hasInternalGrounding = true
		} else {
			// Runtime knowledge tools expose the per-run evidence ID to the
			// agent, while the older in-process pipeline cites ReferenceID.
			// Both identify this retrieved public chunk.
			retrievedPublicDocs[result.ID] = struct{}{}
			retrievedPublicDocs[result.ReferenceID] = struct{}{}
		}
	}

	citedDocs := map[string]struct{}{}
	for _, docID := range response.SourceDocIDs {
		if _, ok := retrievedPublicDocs[docID]; ok {
			citedDocs[docID] = struct{}{}
		}
	}

	// Candidate retrieval deliberately includes diverse alternatives. Do not
	// penalize a grounded answer for declining to cite irrelevant candidates;
	// claim validation separately verifies that the sources actually support
	// the rendered answer.
	sourceCoverage := 0.0
	if len(citedDocs) > 0 || hasInternalGrounding {
		sourceCoverage = 1
	}

	return (retrievalQuality * 0.4) +
		(sourceCoverage * 0.25) +
		(llmConfidence * 0.2) +
		(canAnswerScore * 0.15)
}

// supportEvidenceRetrievalQuality normalizes the relevance signals used by
// the reply gate. Canonical pricing chunks that contain an exact currency
// value receive an authority floor: these chunks may be deliberately promoted
// from the same pricing page after semantic retrieval, so their own vector
// score can be absent even though they are the authoritative exact evidence.
func supportEvidenceRetrievalQuality(result KnowledgeSearchResult) float64 {
	quality := maxFloat(result.VectorScore, clamp01(result.LexicalScore/0.35))
	switch {
	case result.SourceType == knowledgeSourceTypeGuidance:
		quality = maxFloat(quality, 0.9)
	case result.SourceType == supportChildSourceOfficialWeb,
		result.SourceType == supportChildSourceRepository,
		result.SourceType == supportChildSourceWorkspaceResearch:
		quality = maxFloat(quality, 0.85)
	case isCanonicalPricingURL(result.URL) && currencyValuePattern.MatchString(result.Content):
		quality = maxFloat(quality, 0.85)
	}
	return clamp01(quality)
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
