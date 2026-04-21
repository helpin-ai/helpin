package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestRequestUserInputResumeContentUsesGenericQuestionAnswerFormatting(t *testing.T) {
	interaction := &model.AgentRunInteraction{
		RequestPayload: json.RawMessage(`{
			"questions": [
				{
					"id": "next_step",
					"question": "What should I do next with this review?"
				}
			]
		}`),
	}

	responsePayload := json.RawMessage(`{
		"answers": {
			"next_step": {
				"answers": ["Implement changes"]
			}
		}
	}`)

	got := requestUserInputResumeContent(interaction, responsePayload)
	if !strings.Contains(got, "What should I do next with this review?") {
		t.Fatalf("expected generic question text in resume content, got %q", got)
	}
	if !strings.Contains(got, "Implement changes") {
		t.Fatalf("expected selected answer in resume content, got %q", got)
	}
}

func TestReviewCheckpointResumeContentUsesSelectedFindingsForApprove(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "review",
		Title: "Lens review findings",
		Findings: []model.ReviewFinding{
			{ID: "finding_1", Title: "Regression A", Body: "Breaks filter state.", Priority: "P1"},
			{ID: "finding_2", Title: "Regression B", Body: "Drops sort order.", Priority: "P2"},
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	responsePayload := json.RawMessage(`{
		"decision":"approve",
		"selection_mode":"selected",
		"selected_finding_ids":["finding_2"],
		"message":"Only ship the smaller fix first."
	}`)

	got := reviewCheckpointResumeContent(requestPayload, responsePayload, model.AgentRunResumeIntentApprove)
	if !strings.Contains(got, "Approved review findings for implementation:") {
		t.Fatalf("expected structured approve resume content, got %q", got)
	}
	if strings.Contains(got, "Regression A") {
		t.Fatalf("expected unselected finding to be omitted, got %q", got)
	}
	if !strings.Contains(got, "Regression B") || !strings.Contains(got, "Only ship the smaller fix first.") {
		t.Fatalf("expected selected finding and note to be preserved, got %q", got)
	}
}

func TestReviewCheckpointResumeContentFallsBackToAllFindings(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "review",
		Title: "Lens review findings",
		Findings: []model.ReviewFinding{
			{ID: "finding_1", Title: "Regression A", Body: "Breaks filter state.", Priority: "P1"},
			{ID: "finding_2", Title: "Regression B", Body: "Drops sort order.", Priority: "P2"},
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	responsePayload := json.RawMessage(`{"decision":"approve"}`)
	got := reviewCheckpointResumeContent(requestPayload, responsePayload, model.AgentRunResumeIntentApprove)
	if !strings.Contains(got, "Regression A") || !strings.Contains(got, "Regression B") {
		t.Fatalf("expected all findings to be included by default, got %q", got)
	}
}

func TestReviewCheckpointResumeContentDoesNotFallBackToAllWhenSelectedIDsAreMissing(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "review",
		Title: "Lens review findings",
		Findings: []model.ReviewFinding{
			{ID: "finding_1", Title: "Regression A", Body: "Breaks filter state.", Priority: "P1"},
			{ID: "finding_2", Title: "Regression B", Body: "Drops sort order.", Priority: "P2"},
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	responsePayload := json.RawMessage(`{
		"decision":"approve",
		"selection_mode":"selected",
		"selected_finding_ids":[]
	}`)

	got := reviewCheckpointResumeContent(requestPayload, responsePayload, model.AgentRunResumeIntentApprove)
	if got != "" {
		t.Fatalf("expected empty resume content when selected scope is empty, got %q", got)
	}
}
