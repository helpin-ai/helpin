package service

import (
	"strings"
	"testing"
)

func TestValidateCompleteSupportCoverageGapRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     completeSupportCoverageGapRequest
		wantErr string
	}{
		{
			name: "verified document resolves gap",
			req: completeSupportCoverageGapRequest{
				Outcome: SupportCoverageAgentOutcomeResolved, Action: SupportCoverageAgentActionDocumentUpdated,
				SourceStatus: SupportCoverageAgentSourceVerified, DocumentID: "doc-1",
				DocumentationEvidence: "Reviewed the current command bar help docs.",
				SourceEvidence:        "Verified command registration and keyboard handling in source and tests.",
				Summary:               "Updated the command bar article with verified behavior.",
			},
		},
		{
			name: "unverified draft is review ready",
			req: completeSupportCoverageGapRequest{
				Outcome: SupportCoverageAgentOutcomeReviewReady, Action: SupportCoverageAgentActionDocumentCreated,
				SourceStatus: SupportCoverageAgentSourceNotFound, DocumentID: "doc-2",
				DocumentationEvidence: "No existing article matched the customer question.",
				SourceEvidence:        "Searched the linked repository but found no matching implementation.",
				Summary:               "Created a draft and left it open for product verification.",
			},
		},
		{
			name: "missing feature routes to product",
			req: completeSupportCoverageGapRequest{
				Outcome: SupportCoverageAgentOutcomeRouted, Action: SupportCoverageAgentActionFeatureNotFound,
				SourceStatus: SupportCoverageAgentSourceNotFound, HandoffOwner: "Product engineering",
				DocumentationEvidence: "No existing public or internal article described the feature.",
				SourceEvidence:        "Searched the linked repositories and found no command bar implementation.",
				Summary:               "Route the apparent product capability gap to engineering.",
			},
		},
		{
			name: "policy gap does not require repository",
			req: completeSupportCoverageGapRequest{
				Outcome: SupportCoverageAgentOutcomeRouted, Action: SupportCoverageAgentActionNonDocGap,
				SourceStatus: SupportCoverageAgentSourceNotApplicable, HandoffOwner: "Support operations",
				DocumentationEvidence: "Reviewed the support policy collection.",
				Summary:               "The missing policy decision belongs to support operations.",
			},
		},
		{
			name: "resolved cannot use missing source",
			req: completeSupportCoverageGapRequest{
				Outcome: SupportCoverageAgentOutcomeResolved, Action: SupportCoverageAgentActionDocumentCreated,
				SourceStatus: SupportCoverageAgentSourceNotFound, DocumentID: "doc-3",
				DocumentationEvidence: "No article found.", SourceEvidence: "No implementation found.",
				Summary: "Created an unverified article.",
			},
			wantErr: "resolved requires verified or not_applicable",
		},
		{
			name: "feature not found requires matching source status",
			req: completeSupportCoverageGapRequest{
				Outcome: SupportCoverageAgentOutcomeRouted, Action: SupportCoverageAgentActionFeatureNotFound,
				SourceStatus: SupportCoverageAgentSourceVerified, HandoffOwner: "Engineering",
				DocumentationEvidence: "No article found.", SourceEvidence: "Feature was verified.",
				Summary: "Contradictory disposition.",
			},
			wantErr: "feature_not_found requires source_status not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCompleteSupportCoverageGapRequest(tt.req)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNormalizeCompleteSupportCoverageGapRequestAcceptsLegacyProposal(t *testing.T) {
	req := completeSupportCoverageGapRequest{
		Outcome:    SupportCoverageAgentOutcomeProposalSubmitted,
		DocumentID: "doc-1", ProposalID: "proposal-1", Summary: "Submitted proposal.",
	}
	normalizeCompleteSupportCoverageGapRequest(&req)
	if req.Outcome != SupportCoverageAgentOutcomeReviewReady || req.Action != SupportCoverageAgentActionProposalSubmitted {
		t.Fatalf("legacy proposal was not normalized: %#v", req)
	}
	if err := validateCompleteSupportCoverageGapRequest(req); err != nil {
		t.Fatalf("normalized legacy proposal should validate: %v", err)
	}
}
