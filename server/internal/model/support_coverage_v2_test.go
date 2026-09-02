package model

import "testing"

func TestCoverageV2TableNames(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{(CoverageBatch{}).TableName(), "coverage_batches"},
		{(CoverageAnalysisAttempt{}).TableName(), "coverage_analysis_attempts"},
		{(CoverageFinding{}).TableName(), "coverage_findings"},
		{(CoverageTopicV2{}).TableName(), "coverage_topics"},
		{(CoverageTopicMembership{}).TableName(), "coverage_topic_memberships"},
		{(CoverageAssignmentAttempt{}).TableName(), "coverage_assignment_attempts"},
		{(CoverageUnreviewedSignal{}).TableName(), "coverage_unreviewed_signals"},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("TableName() = %q, want %q", test.got, test.want)
		}
	}
}

func TestCoverageV2EnumsRejectUnsupportedValues(t *testing.T) {
	valid := []struct {
		name string
		ok   bool
	}{
		{"batch", IsCoverageBatchStatus(CoverageBatchQueued)},
		{"attempt", IsCoverageAttemptStatus(CoverageAttemptRetryable)},
		{"failure", IsCoverageFailureClass(CoverageFailureLLMProvider)},
		{"membership", IsCoverageMembershipSource(CoverageMembershipAutomatic)},
		{"signal", IsCoverageSignalStatus(CoverageSignalUnreviewed)},
		{"bad batch", IsCoverageBatchStatus("made_up")},
		{"bad attempt", IsCoverageAttemptStatus("made_up")},
		{"bad failure", IsCoverageFailureClass("made_up")},
		{"bad membership", IsCoverageMembershipSource("made_up")},
		{"bad signal", IsCoverageSignalStatus("made_up")},
	}
	for _, test := range valid {
		want := test.name[:3] != "bad"
		if test.ok != want {
			t.Errorf("%s validity = %v, want %v", test.name, test.ok, want)
		}
	}
}

func TestCoverageLegacyArchiveStatusIsExplicit(t *testing.T) {
	if SupportCoverageGapStatusArchivedV1 != "archived_v1" {
		t.Fatalf("archive status = %q", SupportCoverageGapStatusArchivedV1)
	}
}
