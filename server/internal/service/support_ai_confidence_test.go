package service

import "testing"

func TestEvaluateConfidence(t *testing.T) {
	tests := []struct {
		name          string
		searchResults []KnowledgeSearchResult
		response      *AIResponseContract
		isGreeting    bool
		wantAbove     bool // want confidence >= 0.7 (the default AIConfidenceThreshold)
	}{
		{
			name:          "greeting with no retrieval hits answers confidently",
			searchResults: nil,
			response:      &AIResponseContract{CanAnswer: true, SourceDocIDs: nil, Confidence: 0.95},
			isGreeting:    true,
			wantAbove:     true,
		},
		{
			name: "greeting with spurious low-relevance retrieval hits still answers confidently",
			searchResults: []KnowledgeSearchResult{
				{ReferenceID: "doc-1", VectorScore: 0.1, LexicalScore: 0},
				{ReferenceID: "doc-2", VectorScore: 0.05, LexicalScore: 0},
			},
			response:   &AIResponseContract{CanAnswer: true, SourceDocIDs: nil, Confidence: 0.95},
			isGreeting: true,
			wantAbove:  true,
		},
		{
			name: "non-greeting answer with uncited low-relevance retrieval stays low confidence",
			searchResults: []KnowledgeSearchResult{
				{ReferenceID: "doc-1", VectorScore: 0.1, LexicalScore: 0},
				{ReferenceID: "doc-2", VectorScore: 0.05, LexicalScore: 0},
			},
			response:   &AIResponseContract{CanAnswer: true, SourceDocIDs: nil, Confidence: 0.95},
			isGreeting: false,
			wantAbove:  false,
		},
		{
			name: "non-greeting answer grounded in cited high-relevance retrieval is confident",
			searchResults: []KnowledgeSearchResult{
				{ReferenceID: "doc-1", VectorScore: 0.95, LexicalScore: 0},
			},
			response:   &AIResponseContract{CanAnswer: true, SourceDocIDs: []string{"doc-1"}, Confidence: 0.9},
			isGreeting: false,
			wantAbove:  true,
		},
		{
			name: "relevant cited sources are not diluted by unrelated retrieval candidates",
			searchResults: []KnowledgeSearchResult{
				{ReferenceID: "pricing", VectorScore: 0.458},
				{ReferenceID: "candidate-2", VectorScore: 0.43},
				{ReferenceID: "candidate-3", VectorScore: 0.42},
				{ReferenceID: "candidate-4", VectorScore: 0.41},
			},
			response:   &AIResponseContract{CanAnswer: true, SourceDocIDs: []string{"pricing"}, Confidence: 0.98},
			isGreeting: false,
			wantAbove:  true,
		},
		{
			name: "uncited high scoring candidate cannot inflate weak cited evidence",
			searchResults: []KnowledgeSearchResult{
				{ID: "canonical-unrelated", ReferenceID: "pricing", VectorScore: 0.99},
				{ID: "cited-weak", ReferenceID: "comparison", VectorScore: 0.1},
			},
			response: &AIResponseContract{
				CanAnswer: true, SourceDocIDs: []string{"cited-weak"}, Confidence: 0.98,
				Claims: []AIResponseClaim{{Text: "A claim", EvidenceIDs: []string{"cited-weak"}}},
			},
			isGreeting: false,
			wantAbove:  false,
		},
		{
			name: "non-greeting answer grounded in high-relevance internal retrieval does not require a citation",
			searchResults: []KnowledgeSearchResult{
				{ReferenceID: "internal-doc", IsInternal: true, VectorScore: 0.95},
			},
			response:   &AIResponseContract{CanAnswer: true, SourceDocIDs: nil, Confidence: 0.9},
			isGreeting: false,
			wantAbove:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateConfidence(tt.searchResults, tt.response, tt.isGreeting)
			gotAbove := got >= 0.7
			if gotAbove != tt.wantAbove {
				t.Errorf("evaluateConfidence() = %v, above threshold = %v, want above threshold = %v", got, gotAbove, tt.wantAbove)
			}
		})
	}
}
