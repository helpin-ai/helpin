package model

import "testing"

func TestExtractLatestApprovalRequestParsesStructuredBlock(t *testing.T) {
	text := `before
<approval_request phase="stories">
  <title>Story plan approval</title>
  <summary>Three stories are ready.</summary>
</approval_request>
after`

	request := ExtractLatestApprovalRequest(text)
	if request == nil {
		t.Fatal("expected approval request")
	}
	if request.Phase != "stories" {
		t.Fatalf("expected phase %q, got %q", "stories", request.Phase)
	}
	if request.Title != "Story plan approval" {
		t.Fatalf("expected title to be parsed, got %q", request.Title)
	}
	if request.Summary != "Three stories are ready." {
		t.Fatalf("expected summary to be parsed, got %q", request.Summary)
	}
}

func TestExtractLatestApprovalRequestAllowsNonPlannerPhases(t *testing.T) {
	request := ExtractLatestApprovalRequest(`<approval_request phase="review"><title>Review</title></approval_request>`)
	if request == nil {
		t.Fatal("expected approval request")
	}
	if request.Phase != "review" {
		t.Fatalf("expected generic phase to be preserved, got %q", request.Phase)
	}
}

func TestExtractLatestApprovalRequestParsesSelfClosingAttributeForm(t *testing.T) {
	request := ExtractLatestApprovalRequest(`<approval_request phase="prd" title="PRD approval" summary="Review the latest draft" />`)
	if request == nil {
		t.Fatal("expected approval request")
	}
	if request.Phase != "prd" {
		t.Fatalf("expected phase %q, got %q", "prd", request.Phase)
	}
	if request.Title != "PRD approval" {
		t.Fatalf("expected title to be parsed, got %q", request.Title)
	}
	if request.Summary != "Review the latest draft" {
		t.Fatalf("expected summary to be parsed, got %q", request.Summary)
	}
}
