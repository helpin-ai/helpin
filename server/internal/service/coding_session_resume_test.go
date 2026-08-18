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

func TestRequestUserInputResumeContentFallsBackToGenericAnswersForHelpinSchema(t *testing.T) {
	interaction := &model.AgentRunInteraction{
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload: json.RawMessage(`{
			"questions": [
				{
					"id": "metric_scope",
					"header": "Metric scope",
					"question": "Which 3xx responses should this epic measure?"
				}
			]
		}`),
	}

	responsePayload := json.RawMessage(`{
		"answers": {
			"metric_scope": {
				"answers": ["All HTTP 3xx"]
			}
		}
	}`)

	got := requestUserInputResumeContent(interaction, responsePayload)
	if !strings.Contains(got, "Which 3xx responses should this epic measure?") {
		t.Fatalf("expected generic question text in resume content, got %q", got)
	}
	if !strings.Contains(got, "All HTTP 3xx") {
		t.Fatalf("expected selected answer in resume content, got %q", got)
	}
}

func TestResumeRequestForResolvedUserInputDoesNotFailOnEmptyPayload(t *testing.T) {
	interaction := &model.AgentRunInteraction{
		InteractionKind: model.AgentRunInteractionKindRequestUserInput,
		RequestPayload: json.RawMessage(`{
			"questions": [
				{
					"id": "metric_scope",
					"question": "Which 3xx responses should this epic measure?"
				}
			]
		}`),
	}

	req, err := resumeRequestForResolvedInteraction(interaction, nil, "")
	if err != nil {
		t.Fatalf("resumeRequestForResolvedInteraction returned error: %v", err)
	}
	if !strings.Contains(req.Content, "Continue with the selected answers") {
		t.Fatalf("expected deterministic fallback content, got %q", req.Content)
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
	for _, expected := range []string{
		"Approved PRD.",
		"Call ensure_epic_spec_doc with {}",
		"Then call write_document_content with the returned document_id and the full approved markdown.",
		"Then call approve_epic_spec with {}",
		"do not continue to task planning, until all three tool calls succeed.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected PRD continuation to contain %q, got %q", expected, got)
		}
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
	for _, expected := range []string{
		"Approved task plan.",
		"Call create_task_batch with the full approved proposed_tasks array",
		"Preserve every approved task field and dependency_refs.",
		"Do not claim the task plan was applied and do not finish until create_task_batch succeeds.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected task-plan continuation to contain %q, got %q", expected, got)
		}
	}
}

func TestApprovalRequestResumeContentPreservesTaskPlanApproveMessageAsNote(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "tasks",
		Title: "Approve Task Plan",
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	got := approvalRequestResumeContent(requestPayload, json.RawMessage(`{"decision":"approve","message":"Keep the five-task ordering."}`), model.AgentRunResumeIntentApprove)
	if !strings.Contains(got, "Call create_task_batch") || !strings.Contains(got, "Human note: Keep the five-task ordering.") {
		t.Fatalf("expected task creation instructions and human note, got %q", got)
	}
}

func TestApprovalRequestResumeContentSynthesizesTaskDocumentPersistenceTools(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "task_doc",
		Title: "Approve Task Planning Document",
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	got := approvalRequestResumeContent(requestPayload, json.RawMessage(`{"decision":"approve"}`), model.AgentRunResumeIntentApprove)
	for _, expected := range []string{
		"Approved task planning document.",
		"Call ensure_task_plan_doc with {}",
		"Then call write_document_content with the returned document_id and the full approved markdown.",
		"Do not claim the document was persisted and do not finish until both tool calls succeed.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected task-document continuation to contain %q, got %q", expected, got)
		}
	}
}

func TestApprovalRequestResumeContentPreservesTaskDocumentApproveMessageAsNote(t *testing.T) {
	requestPayload, err := json.Marshal(model.ApprovalRequest{
		Phase: "task_doc",
		Title: "Approve Task Planning Document",
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	got := approvalRequestResumeContent(requestPayload, json.RawMessage(`{"decision":"approve","message":"Use the final wording."}`), model.AgentRunResumeIntentApprove)
	if !strings.Contains(got, "Call ensure_task_plan_doc with {}") || !strings.Contains(got, "Human note: Use the final wording.") {
		t.Fatalf("expected persistence instructions and human note, got %q", got)
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
	if !strings.Contains(got, "Call ensure_epic_spec_doc with {}") || !strings.Contains(got, "Human note: Looks good. Proceed.") {
		t.Fatalf("expected persistence instructions and human note, got %q", got)
	}
}
