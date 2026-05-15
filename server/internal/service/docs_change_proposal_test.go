package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestProposalApplyLabel(t *testing.T) {
	t.Parallel()

	longSummary := strings.Repeat("a", 120)

	tests := []struct {
		name     string
		proposal *model.DocsChangeProposal
		want     string
	}{
		{
			name:     "nil proposal returns generic label",
			proposal: nil,
			want:     "Applied proposal",
		},
		{
			name:     "empty summary returns generic label",
			proposal: &model.DocsChangeProposal{Summary: ""},
			want:     "Applied proposal",
		},
		{
			name:     "whitespace-only summary returns generic label",
			proposal: &model.DocsChangeProposal{Summary: "   \n\t  "},
			want:     "Applied proposal",
		},
		{
			name:     "summary is included after Applied prefix",
			proposal: &model.DocsChangeProposal{Summary: "Update billing FAQ"},
			want:     "Applied: Update billing FAQ",
		},
		{
			name:     "summary is trimmed before use",
			proposal: &model.DocsChangeProposal{Summary: "  Update billing FAQ\n"},
			want:     "Applied: Update billing FAQ",
		},
		{
			name:     "long summary is truncated with ellipsis",
			proposal: &model.DocsChangeProposal{Summary: longSummary},
			want:     "Applied: " + strings.Repeat("a", 80) + "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proposalApplyLabel(tt.proposal)
			if got != tt.want {
				t.Errorf("proposalApplyLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
