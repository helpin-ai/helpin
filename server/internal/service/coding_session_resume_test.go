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
	requestPayload, err := json.Marshal(model.ReviewCheckpointRequest{
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

func TestReviewCheckpointResolveRequestPreservesApprovedFindingsWhenNoteIsFilled(t *testing.T) {
	requestPayload, err := json.Marshal(model.ReviewCheckpointRequest{
		Phase: "review",
		Title: "Lens review findings",
		Findings: []model.ReviewFinding{
			{ID: "finding_1", Title: "Regression A", Body: "Breaks filter state.", Priority: "P1"},
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}
	responsePayload := json.RawMessage(`{
		"decision":"approve",
		"selection_mode":"selected",
		"selected_finding_ids":["finding_1"],
		"message":"Use the smaller patch."
	}`)

	req, err := resumeRequestForResolvedInteraction(&model.AgentRunInteraction{
		InteractionKind: model.AgentRunInteractionKindReviewCheckpoint,
		RequestPayload:  requestPayload,
	}, responsePayload, "Use the smaller patch.")
	if err != nil {
		t.Fatalf("resume request: %v", err)
	}
	if !req.SendMessage {
		t.Fatalf("expected approved review note to be sent as a message")
	}
	if !strings.Contains(req.Content, "Approved review findings for implementation:") {
		t.Fatalf("expected structured approved findings in content, got %q", req.Content)
	}
	if !strings.Contains(req.Content, "Regression A") {
		t.Fatalf("expected selected finding in content, got %q", req.Content)
	}
	if !strings.Contains(req.Content, "Human note: Use the smaller patch.") {
		t.Fatalf("expected note in content, got %q", req.Content)
	}
}

func TestReviewCheckpointResumeContentFallsBackToAllFindings(t *testing.T) {
	requestPayload, err := json.Marshal(model.ReviewCheckpointRequest{
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
	requestPayload, err := json.Marshal(model.ReviewCheckpointRequest{
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

func TestReviewCheckpointResumeContentSkipsFindingsWithNote(t *testing.T) {
	requestPayload, err := json.Marshal(model.ReviewCheckpointRequest{
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
		"decision":"skip",
		"selection_mode":"none",
		"selected_finding_ids":[],
		"message":"Not worth changing for this run."
	}`)

	got := reviewCheckpointResumeContent(requestPayload, responsePayload, model.AgentRunResumeIntentReply)
	if !strings.Contains(got, "Skipped review findings.") {
		t.Fatalf("expected skip resume content, got %q", got)
	}
	if !strings.Contains(got, "Do not implement these review findings.") {
		t.Fatalf("expected explicit no-implementation instruction, got %q", got)
	}
	if !strings.Contains(got, "Human note: Not worth changing for this run.") {
		t.Fatalf("expected note to be preserved, got %q", got)
	}
	if strings.Contains(got, "Regression A") || strings.Contains(got, "Regression B") {
		t.Fatalf("expected skipped finding details to be omitted, got %q", got)
	}
}

func TestResolveIntentForSkippedReviewCheckpointIsReply(t *testing.T) {
	interaction := &model.AgentRunInteraction{
		InteractionKind: model.AgentRunInteractionKindReviewCheckpoint,
	}
	intent := resolveIntentForInteraction(interaction, json.RawMessage(`{"decision":"skip","selection_mode":"none"}`))
	if intent != model.AgentRunResumeIntentReply {
		t.Fatalf("expected skip to resume as neutral reply, got %q", intent)
	}
}

func TestApprovalRequestResumeContentSynthesizesPRDContinuation(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "prd",
		Title: "Approve PRD",
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	got := approvalRequestResumeContent(requestPayload, json.RawMessage(`{"decision":"approve"}`), model.AgentRunResumeIntentApprove)
	if got != "Approved PRD. Continue to task planning." {
		t.Fatalf("unexpected synthesized PRD resume content: %q", got)
	}
}

func TestApprovalRequestResumeContentSynthesizesTaskPlanContinuation(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "tasks",
		Title: "Approve Task Plan",
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	got := approvalRequestResumeContent(requestPayload, json.RawMessage(`{"decision":"approve"}`), model.AgentRunResumeIntentApprove)
	if got != "Approved task plan. Apply it and create tasks." {
		t.Fatalf("unexpected synthesized task-plan resume content: %q", got)
	}
}

func TestApprovalRequestResumeContentPreservesExplicitApproveMessage(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "prd",
		Title: "Approve PRD",
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	got := approvalRequestResumeContent(requestPayload, json.RawMessage(`{"decision":"approve","message":"Looks good. Proceed."}`), model.AgentRunResumeIntentApprove)
	if got != "Looks good. Proceed." {
		t.Fatalf("expected explicit approval message to win, got %q", got)
	}
}
