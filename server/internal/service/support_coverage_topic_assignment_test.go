package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDecideCoverageTopicAssignment(t *testing.T) {
	finding := model.CoverageFinding{CustomerNeed: "Customer cannot reset their password", FixType: "update_article", Confidence: .9}
	tests := []struct {
		name       string
		candidates []model.CoverageTopicV2
		want       string
	}{
		{"attach compatible high confidence", []model.CoverageTopicV2{{ID: "topic-1", CustomerNeed: "Customers cannot reset a password", AssignmentPolicy: "v1"}}, model.CoverageAssignmentAttach},
		{"create when no similar topic", []model.CoverageTopicV2{{ID: "topic-2", CustomerNeed: "Export invoices", AssignmentPolicy: "v1"}}, model.CoverageAssignmentCreate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := DecideCoverageTopicAssignment(finding, test.candidates)
			if decision.Outcome != test.want {
				t.Fatalf("decision = %+v", decision)
			}
		})
	}
	finding.Confidence = .45
	if decision := DecideCoverageTopicAssignment(finding, nil); decision.Outcome != model.CoverageAssignmentReview {
		t.Fatalf("low-confidence decision = %+v", decision)
	}
}

func TestCoverageTopicAssignmentIsReversibleNotAMerge(t *testing.T) {
	decision := DecideCoverageTopicAssignment(model.CoverageFinding{CustomerNeed: "Reset password", FixType: "update_article", Confidence: .95}, []model.CoverageTopicV2{
		{ID: "topic-1", CustomerNeed: "Password reset", AssignmentPolicy: "v1"},
		{ID: "topic-2", CustomerNeed: "Recover account password", AssignmentPolicy: "v1"},
	})
	if decision.TopicID == "" || decision.Outcome != model.CoverageAssignmentAttach {
		t.Fatalf("expected one reversible membership target, got %+v", decision)
	}
}
