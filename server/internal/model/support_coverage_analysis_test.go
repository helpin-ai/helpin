package model

import "testing"

func TestSupportCoverageAnalysisModelContracts(t *testing.T) {
	if got := (SupportCoverageAnalysisRun{}).TableName(); got != "support_coverage_analysis_runs" {
		t.Fatalf("analysis run table = %q", got)
	}
	if got := (SupportCoverageConversationAnalysis{}).TableName(); got != "support_coverage_conversation_analyses" {
		t.Fatalf("conversation analysis table = %q", got)
	}
	if got := (SupportAIRetrievalTrace{}).TableName(); got != "support_ai_retrieval_traces" {
		t.Fatalf("retrieval trace table = %q", got)
	}
	if got := (SupportCoverageRecommendation{}).TableName(); got != "support_coverage_recommendations" {
		t.Fatalf("recommendation table = %q", got)
	}

	if SupportCoverageFixUpdateWebsitePage != "update_website_page" {
		t.Fatalf("website recommendation constant drifted")
	}
	if SupportCoverageRecommendationPriorityPrimary != "primary" {
		t.Fatalf("recommendation priority constant drifted")
	}
}
