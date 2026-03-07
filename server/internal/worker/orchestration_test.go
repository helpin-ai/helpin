package worker

import "testing"

func TestExtractOrchestrationProposalParsesJSONAndFencedJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "plain json",
			content: `{"summary":"Breakdown","proposed_stories":[{"name":"Story A","description":"Do A","story_type":"feature","estimate":3}]}`,
		},
		{
			name:    "fenced json",
			content: "```json\n{\"summary\":\"Breakdown\",\"proposed_stories\":[{\"name\":\"Story A\",\"description\":\"Do A\",\"story_type\":\"feature\",\"estimate\":3}]}\n```",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proposal, err := extractOrchestrationProposal([]Message{
				{
					Role: "assistant",
					Content: []ContentBlock{
						{Type: "text", Text: tc.content},
					},
				},
			}, "epic-1", 123)
			if err != nil {
				t.Fatalf("extractOrchestrationProposal returned error: %v", err)
			}
			if proposal.EpicID != "epic-1" {
				t.Fatalf("expected epic ID epic-1, got %q", proposal.EpicID)
			}
			if proposal.TokensUsed != 123 {
				t.Fatalf("expected tokens 123, got %d", proposal.TokensUsed)
			}
			if len(proposal.ProposedStories) != 1 || proposal.ProposedStories[0].Name != "Story A" {
				t.Fatalf("unexpected proposal stories: %+v", proposal.ProposedStories)
			}
		})
	}
}
