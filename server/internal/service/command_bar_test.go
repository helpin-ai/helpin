package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type scriptedCommandBarLLM struct {
	response  string
	responses []string
	err       error
	errors    []error
	requests  []llm.ChatRequest
	deadlines []time.Time
}

func (s *scriptedCommandBarLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.requests = append(s.requests, req)
	if deadline, ok := ctx.Deadline(); ok {
		s.deadlines = append(s.deadlines, deadline)
	} else {
		s.deadlines = append(s.deadlines, time.Time{})
	}
	if s.err != nil {
		return nil, s.err
	}
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(s.responses) > 0 {
		response := s.responses[0]
		s.responses = s.responses[1:]
		return &llm.ChatResponse{Content: response}, nil
	}
	return &llm.ChatResponse{Content: s.response}, nil
}

type commandBarSignalTemporalClient struct {
	tclient.Client
	err           error
	signalName    string
	workflowID    string
	signaledRunID string
}

func (c *commandBarSignalTemporalClient) SignalWorkflow(_ context.Context, workflowID, workflowRunID, signalName string, arg interface{}) error {
	c.workflowID = workflowID
	c.signalName = signalName
	payload, ok := arg.(temporalapp.CommandBarRunCompletedSignal)
	if !ok {
		return fmt.Errorf("unexpected command bar signal payload type %T", arg)
	}
	c.signaledRunID = payload.RunID
	return c.err
}

func TestParseExplicitNamedAgentsPreservesRequestOrder(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "task",
		EntityID:     "task-1",
		DisplayTitle: "Task 1",
	}
	candidates := []model.CommandBarAgent{
		{ID: "agent-forge", Name: "Forge", PresetKey: model.AgentPresetCodeBuilder, AllowedTargets: []string{"task"}},
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
	}

	resp := parseExplicitNamedAgents("run forge and then lens", pageContext, candidates)
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected plan response, got %#v", resp)
	}
	if resp.Plan.RunCount != 2 {
		t.Fatalf("expected run count 2, got %d", resp.Plan.RunCount)
	}
	if got := resp.Plan.Steps[0].AgentName; got != "Forge" {
		t.Fatalf("expected first step Forge, got %q", got)
	}
	if got := resp.Plan.Steps[1].AgentName; got != "Lens" {
		t.Fatalf("expected second step Lens, got %q", got)
	}
	if got := resp.Plan.Steps[0].Instructions; strings.Contains(strings.ToLower(got), "run forge and then lens") {
		t.Fatalf("expected step-scoped Forge instructions, got %q", got)
	}
	if got := resp.Plan.Steps[0].Instructions; !strings.Contains(got, "Do not invoke or run Lens") {
		t.Fatalf("expected Forge step to leave Lens to scheduler, got %q", got)
	}
}

func TestParseExplicitNamedAgentsUsesWholeWords(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
	}

	if resp := parseExplicitNamedAgents("check camera lenses", pageContext, candidates); resp != nil {
		t.Fatalf("expected no explicit agent match, got %#v", resp)
	}
}

func TestParseIntentResolvesExplicitTaskKeyTargetForNamedAgent(t *testing.T) {
	service, _, workspaceID, taskID := setupCommandBarTargetResolutionTest(t)
	ctx := context.Background()

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "run forge for USE-90",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentName != "Forge" {
		t.Fatalf("expected Forge step, got %#v", step)
	}
	if step.Target.EntityType != "task" || step.Target.EntityID != taskID {
		t.Fatalf("expected resolved task target, got %#v", step.Target)
	}
	if !strings.Contains(step.Target.DisplayTitle, "USE-90") {
		t.Fatalf("expected task key in display title, got %q", step.Target.DisplayTitle)
	}
}

func TestParseIntentResolvesExplicitTaskKeyTargetForMultiAgentPlan(t *testing.T) {
	service, _, workspaceID, taskID := setupCommandBarTargetResolutionTest(t)
	ctx := context.Background()

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "run forge then lens for USE-90",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 2 {
		t.Fatalf("expected two-step plan, got %#v", resp)
	}
	for _, step := range resp.Plan.Steps {
		if step.Target.EntityType != "task" || step.Target.EntityID != taskID {
			t.Fatalf("expected every step to use resolved task target, got %#v", resp.Plan.Steps)
		}
	}
}

func TestParseIntentAsksForTargetWhenNamedAgentHasNoConcreteTarget(t *testing.T) {
	service, _, workspaceID, _ := setupCommandBarTargetResolutionTest(t)
	ctx := context.Background()

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "run forge",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusNoMatchingAgent {
		t.Fatalf("expected no-match clarification, got %#v", resp)
	}
	if !strings.Contains(resp.Reason, "Forge needs a task target") {
		t.Fatalf("expected task target clarification, got %q", resp.Reason)
	}
}

func TestParseIntentDoesNotFallBackWhenExplicitTaskKeyIsMissing(t *testing.T) {
	service, _, workspaceID, _ := setupCommandBarTargetResolutionTest(t)
	ctx := context.Background()

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "run forge for USE-999",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusNoMatchingAgent {
		t.Fatalf("expected no-match for missing task key, got %#v", resp)
	}
	if !strings.Contains(resp.Reason, "USE-999") {
		t.Fatalf("expected missing key reason, got %q", resp.Reason)
	}
}

func TestParseIntentResolvesExplicitDocumentIDTargetForOneShot(t *testing.T) {
	service, _, workspaceID, documentID := setupCommandBarDocumentTargetResolutionTest(t)
	ctx := context.Background()
	service.llmProvider = &scriptedCommandBarLLM{responses: []string{
		`{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa","web_search_brave"],"tool_intent":"propose_change","rationale":"document update","confidence":0.98}`,
		`{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa","web_search_brave","publish_document_change_proposal"],"tool_intent":"propose_change","rationale":"document update","confidence":0.98}`,
	}}

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check if any section needs update after online research for document id " + documentID + ". use one-shot agent",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot step, got %#v", step)
	}
	if step.Target.EntityType != "document" || step.Target.EntityID != documentID {
		t.Fatalf("expected document target, got %#v", step.Target)
	}
	if !strings.Contains(step.Target.DisplayTitle, "Wire error tracking") {
		t.Fatalf("expected document title in target, got %#v", step.Target)
	}
	if !slices.Contains(step.AllowedTools, "publish_document_change_proposal") {
		t.Fatalf("expected proposal tool, got %#v", step.AllowedTools)
	}
}

func TestParseIntentDoesNotAugmentOneShotToolsFromText(t *testing.T) {
	service, _, workspaceID, documentID := setupCommandBarDocumentTargetResolutionTest(t)
	ctx := context.Background()
	service.llmProvider = &scriptedCommandBarLLM{response: `{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa","web_search_brave"],"tool_intent":"read_only","rationale":"document research","confidence":0.98}`}

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check if any section needs update after online research for document id " + documentID + ". use one-shot agent",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.Target.EntityType != "document" || step.Target.EntityID != documentID {
		t.Fatalf("expected document target, got %#v", step.Target)
	}
	if slices.Contains(step.AllowedTools, "publish_document_change_proposal") {
		t.Fatalf("did not expect backend-added proposal tool, got %#v", step.AllowedTools)
	}
	if strings.Contains(step.Instructions, "publish_document_change_proposal") {
		t.Fatalf("did not expect proposal instructions for read-only tools, got %q", step.Instructions)
	}
}

func TestParseIntentRejectsInvalidOneShotToolIntentWithoutHeuristicFallback(t *testing.T) {
	service, _, workspaceID, documentID := setupCommandBarDocumentTargetResolutionTest(t)
	ctx := context.Background()
	service.llmProvider = &scriptedCommandBarLLM{responses: []string{
		`{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa"],"tool_intent":"propose_change","rationale":"document update","confidence":0.98}`,
		`{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa"],"tool_intent":"propose_change","rationale":"document update","confidence":0.98}`,
	}}

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check if any section needs update after online research for document id " + documentID + ". use one-shot agent",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusNoMatchingAgent {
		t.Fatalf("expected invalid planner no-match, got %#v", resp)
	}
	if !strings.Contains(resp.Reason, "propose_change tool_intent requires publish_document_change_proposal") {
		t.Fatalf("expected validation reason, got %q", resp.Reason)
	}
}

func TestParseIntentRejectsDocumentMutationOneShotWithoutConcreteTarget(t *testing.T) {
	service, _, workspaceID, _ := setupCommandBarDocumentTargetResolutionTest(t)
	ctx := context.Background()
	service.llmProvider = &scriptedCommandBarLLM{response: `{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa","publish_document_change_proposal"],"rationale":"document update","confidence":0.98}`}

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check if any section needs update after online research. use one-shot agent",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusNoMatchingAgent {
		t.Fatalf("expected clarification/no-match, got %#v", resp)
	}
	if !strings.Contains(resp.Reason, "Command Agent needs a document target") {
		t.Fatalf("expected document target clarification, got %q", resp.Reason)
	}
}

func TestParseIntentPreservesLLMOneShotStepTarget(t *testing.T) {
	service, _, workspaceID, documentID := setupCommandBarDocumentTargetResolutionTest(t)
	ctx := context.Background()
	service.llmProvider = &scriptedCommandBarLLM{response: fmt.Sprintf(`{"status":"plan","route_kind":"one_shot_command","steps":[{"agent_id":"agent-command","target":{"entity_type":"document","entity_id":%q,"display_title":"Wire error tracking into metrics middleware Plan"},"instructions":"Research the document and propose updates.","allowed_tools":["read_document","web_search_exa","publish_document_change_proposal"]}],"rationale":"document update","confidence":0.98}`, documentID)}

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check if the document needs updates after web research",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.Target.EntityType != "document" || step.Target.EntityID != documentID {
		t.Fatalf("expected LLM-provided document target to be preserved, got %#v", step.Target)
	}
	if step.Target.DisplayTitle != "Wire error tracking into metrics middleware Plan" {
		t.Fatalf("expected validated document title, got %#v", step.Target)
	}
}

func TestChatTurnInfersPriorDocumentTargetForOneShot(t *testing.T) {
	service, db, workspaceID, documentID := setupCommandBarDocumentTargetResolutionTest(t)
	ctx := context.Background()
	repo := repository.NewCommandBarChatRepository(db)
	threadID := seedCommandBarThreadWithWorkingContext(t, ctx, repo, workspaceID, "actor-1", []commandBarWorkingEntityRef{
		{Type: "document", ID: documentID, Title: "Wire error tracking into metrics middleware Plan"},
	})
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"one_shot_command","reason":"document update check","confidence":0.98}`,
		`{"status":"plan","route_kind":"one_shot_command","one_shot_tools":["read_document","web_search_exa","publish_document_change_proposal"],"rationale":"document update","confidence":0.98}`,
	}}
	service.llmProvider = fakeLLM
	service.SetChatRepository(repo)

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		ThreadID:    &threadID,
		Text:        "check if any section needs update after online research",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalRunPlan || resp.Proposal.Plan == nil || len(resp.Proposal.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot run plan, got %#v", resp.Proposal)
	}
	step := resp.Proposal.Plan.Steps[0]
	if step.Target.EntityType != "document" || step.Target.EntityID != documentID {
		t.Fatalf("expected prior document target, got %#v", step.Target)
	}
}

func TestParseIntentResolvesRepositoryNameTargetForOneShot(t *testing.T) {
	service, _, workspaceID, repoID := setupCommandBarRepositoryTargetResolutionTest(t)
	ctx := context.Background()

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check the latest commits in main branch in the last month and create a changelog doc in docs. create a one shot agent. use repository target. usermaven",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot repository plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot step, got %#v", step)
	}
	if step.Target.EntityType != "repository" || step.Target.EntityID != repoID {
		t.Fatalf("expected resolved repository target, got %#v", step.Target)
	}
	if step.Target.DisplayTitle != "helpin/usermaven" {
		t.Fatalf("expected repository display title, got %#v", step.Target)
	}
	for _, required := range []string{"list_commits", "list_directory", "create_document"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
}

func TestParseIntentResolvesRepositoryNameBeforeRepositoryKeyword(t *testing.T) {
	service, _, workspaceID, _ := setupCommandBarRepositoryTargetResolutionTest(t)
	ctx := context.Background()
	repoID := "55555555-5555-5555-5555-555555555555"

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check the latest commits in main branch in the last month and create a changelog doc in docs. target scope is usermaven/usermven repository, get the repo uuid first",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot repository plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.Target.EntityType != "repository" || step.Target.EntityID != repoID {
		t.Fatalf("expected resolved repository target, got %#v", step.Target)
	}
}

func TestParseIntentCompletesUnderspecifiedLLMRepositoryChangelogTools(t *testing.T) {
	service, _, workspaceID, repoID := setupCommandBarRepositoryTargetResolutionTest(t)
	ctx := context.Background()
	service.llmProvider = &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"one_shot_command",
		"agent_id":"33333333-3333-3333-3333-333333333333",
		"instructions":"Inspect the repository and create the changelog.",
		"one_shot_tools":["list_directory"],
		"tool_intent":"read_only",
		"rationale":"The request needs repository context.",
		"confidence":0.94
	}`}

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check the latest commits in main branch in the last month and create a changelog doc in docs. create a one shot agent. use repository target. usermaven",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot repository plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.Target.EntityType != "repository" || step.Target.EntityID != repoID {
		t.Fatalf("expected resolved repository target, got %#v", step.Target)
	}
	for _, required := range []string{"list_commits", "read_files", "list_directory", "list_spaces", "list_collections", "create_document"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected completed repository changelog tool %q in %#v", required, step.AllowedTools)
		}
	}
	if !commandBarToolsIncludeMutation(step.AllowedTools) {
		t.Fatalf("expected document creation to make tool set mutating, got %#v", step.AllowedTools)
	}
	if !strings.Contains(step.Instructions, "Confirm the repository target") {
		t.Fatalf("expected repository-specific one-shot instructions, got %q", step.Instructions)
	}
}

func TestParseIntentRequiresRepositoryTargetForRepoTools(t *testing.T) {
	service, _, workspaceID, _ := setupCommandBarRepositoryTargetResolutionTest(t)
	ctx := context.Background()

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "check the latest commits in main branch in the last month and create a changelog doc in docs. create a one shot agent. use repository target",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusNoMatchingAgent {
		t.Fatalf("expected repository target clarification, got %#v", resp)
	}
	if !strings.Contains(resp.Reason, "repository target") {
		t.Fatalf("expected repository target reason, got %q", resp.Reason)
	}
}

func TestChatTurnMergesTargetClarificationWithPendingRequest(t *testing.T) {
	service, db, workspaceID, repoID := setupCommandBarRepositoryTargetResolutionTest(t)
	ctx := context.Background()
	chatRepo := repository.NewCommandBarChatRepository(db)
	service.SetChatRepository(chatRepo)
	actorID := "actor-1"
	thread := &model.CommandBarThread{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		ActorID:     &actorID,
		Title:       "Changelog request",
		Status:      model.CommandBarThreadStatusOpen,
		CreatedAt:   time.Now().Add(-3 * time.Minute),
		UpdatedAt:   time.Now().Add(-3 * time.Minute),
	}
	if err := chatRepo.CreateThread(ctx, thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	original := "check the latest commits in main branch in the last month and create a changelog doc in docs. create a one shot agent. use repository target"
	if err := chatRepo.CreateMessage(ctx, &model.CommandBarMessage{
		ID:          uuid.NewString(),
		ThreadID:    thread.ID,
		WorkspaceID: workspaceID,
		ActorID:     &actorID,
		Role:        model.CommandBarMessageRoleUser,
		Content:     original,
		CreatedAt:   time.Now().Add(-2 * time.Minute),
	}); err != nil {
		t.Fatalf("create user message: %v", err)
	}
	if err := chatRepo.CreateMessage(ctx, &model.CommandBarMessage{
		ID:           uuid.NewString(),
		ThreadID:     thread.ID,
		WorkspaceID:  workspaceID,
		ActorID:      &actorID,
		Role:         model.CommandBarMessageRoleAssistant,
		Content:      "What should I use as the target or scope for this request?",
		ProposalJSON: mustJSON(&model.CommandBarProposal{Type: model.CommandBarProposalClarification, Answer: "What should I use as the target or scope for this request?"}),
		CreatedAt:    time.Now().Add(-1 * time.Minute),
	}); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}

	resp, err := service.ChatTurn(ctx, workspaceID, actorID, model.CommandBarChatTurnRequest{
		ThreadID:    &thread.ID,
		Text:        "target scope is helpin/usermaven repository, get the repo uuid first",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalRunPlan || resp.Proposal.Plan == nil || len(resp.Proposal.Plan.Steps) != 1 {
		t.Fatalf("expected run plan proposal, got %#v", resp.Proposal)
	}
	step := resp.Proposal.Plan.Steps[0]
	if step.Target.EntityType != "repository" || step.Target.EntityID != repoID {
		t.Fatalf("expected resolved repository target, got %#v", step.Target)
	}
	if !strings.Contains(step.Instructions, "latest commits in main branch") {
		t.Fatalf("expected original request in one-shot instructions, got %q", step.Instructions)
	}
}

func TestChatTurnAsksRepositoryChoiceThenRunsSelectedRepository(t *testing.T) {
	service, db, workspaceID, _ := setupCommandBarRepositoryTargetResolutionTest(t)
	ctx := context.Background()
	chatRepo := repository.NewCommandBarChatRepository(db)
	service.SetChatRepository(chatRepo)
	actorID := "actor-1"
	original := "check the latest commits in main branch in the last month and create a changelog doc in docs. create a one shot agent. use repository target"

	first, err := service.ChatTurn(ctx, workspaceID, actorID, model.CommandBarChatTurnRequest{
		Text:        original,
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("first chat turn: %v", err)
	}
	if first.Proposal == nil || first.Proposal.Type != model.CommandBarProposalClarification {
		t.Fatalf("expected repository clarification, got %#v", first.Proposal)
	}
	if !strings.Contains(first.AssistantMessage.Content, "Choose a repository target") ||
		!strings.Contains(first.AssistantMessage.Content, "helpin/usermaven") ||
		!strings.Contains(first.AssistantMessage.Content, "usermaven/usermven") {
		t.Fatalf("expected repository choices in assistant content, got %q", first.AssistantMessage.Content)
	}

	second, err := service.ChatTurn(ctx, workspaceID, actorID, model.CommandBarChatTurnRequest{
		ThreadID:    &first.Thread.ID,
		Text:        "2",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("second chat turn: %v", err)
	}
	if second.Proposal == nil || second.Proposal.Type != model.CommandBarProposalRunPlan || second.Proposal.Plan == nil || len(second.Proposal.Plan.Steps) != 1 {
		t.Fatalf("expected selected repository run plan, got %#v", second.Proposal)
	}
	step := second.Proposal.Plan.Steps[0]
	if step.Target.EntityType != "repository" || step.Target.EntityID != "55555555-5555-5555-5555-555555555555" {
		t.Fatalf("expected second repository target, got %#v", step.Target)
	}
	if !strings.Contains(step.Instructions, "latest commits in main branch") {
		t.Fatalf("expected original request in one-shot instructions, got %q", step.Instructions)
	}
}

func TestChatTurnClassifiesExplicitTaskStatusAsInlineReadOnly(t *testing.T) {
	service, db, workspaceID, _ := setupCommandBarTargetResolutionTest(t)
	ctx := context.Background()
	createCommandBarChatTablesForTest(t, db)

	taskRepo := repository.NewPMTaskRepository(db)
	commandService := NewInternalCommandService(nil, service.agentService.taskService, nil, nil, nil, nil, taskRepo, nil)
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"read-only task status question","confidence":0.99}`,
		`{"type":"tool_call","tool":"list_tasks","input":{}}`,
		`{"type":"final","answer":"USE-90 is not completed."}`,
	}}
	service.llmProvider = fakeLLM
	service.SetChatRepository(repository.NewCommandBarChatRepository(db)).
		SetInternalCommandService(commandService)

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "USE-90 is it completed?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalInlineAnswer {
		t.Fatalf("expected inline answer proposal, got %#v", resp.Proposal)
	}
	if resp.Proposal.Plan != nil {
		t.Fatalf("task status question should not create a run plan")
	}
	if resp.AssistantMessage.Content != "USE-90 is not completed." {
		t.Fatalf("expected task status answer, got %q", resp.AssistantMessage.Content)
	}
	var toolContext commandBarReadOnlyToolContext
	if err := json.Unmarshal(resp.Proposal.Context, &toolContext); err != nil {
		t.Fatalf("decode inline context: %v", err)
	}
	if len(toolContext.ToolCalls) != 1 || toolContext.ToolCalls[0].Tool != "list_tasks" {
		t.Fatalf("expected one task lookup tool call, got %#v", toolContext.ToolCalls)
	}
	output := string(toolContext.ToolCalls[0].Output)
	if !strings.Contains(output, `"task_key":"USE-90"`) || !strings.Contains(output, `"completed":false`) {
		t.Fatalf("expected resolved USE-90 task output, got %s", output)
	}
}

func TestCommandBarAdditionalContextDoesNotIncludeRawUserRequest(t *testing.T) {
	context := commandBarAdditionalContext(
		"Execute your normal Forge role for the current target.",
		model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
		0,
		2,
	)
	if strings.Contains(strings.ToLower(context), "run forge and then lens") {
		t.Fatalf("expected raw prompt to stay out of execution context, got %q", context)
	}
	if !strings.Contains(context, "Step instruction:") {
		t.Fatalf("expected execution context to include step instruction, got %q", context)
	}
	if !strings.Contains(context, "Other command-bar plan steps are scheduled separately") {
		t.Fatalf("expected multi-step scheduler guidance, got %q", context)
	}
}

func TestParseOneShotCommandIntentForDocumentResearchIsReadOnly(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "document",
		EntityID:     "doc-1",
		DisplayTitle: "Setup guide",
	}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"document"},
			AllowedTools:   []string{"update_plan", "request_user_input", "request_approval", "web_search_exa", "web_search_brave", "fetch_url", "crawl_url", "list_documents", "list_collections", "read_document", "get_document_blocks", "search_documents", "write_document_content", "update_document_block", "link_document_to_object", "create_document"},
		},
	}

	resp := parseOneShotCommandIntent("check the web and inspect stale doc sections", pageContext, candidates)
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot plan kind, got %q", resp.Plan.PlanKind)
	}
	step := resp.Plan.Steps[0]
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot step kind, got %q", step.PlanKind)
	}
	for _, required := range []string{"web_search_exa", "read_document", "get_document_blocks"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if slices.Contains(step.AllowedTools, "publish_document_change_proposal") {
		t.Fatalf("did not expect heuristic proposal tool in %#v", step.AllowedTools)
	}
	if slices.Contains(step.AllowedTools, "search_documents") {
		t.Fatalf("did not expect search_documents for a known current document in %#v", step.AllowedTools)
	}
	for _, directWrite := range []string{"write_document_content", "update_document_block"} {
		if slices.Contains(step.AllowedTools, directWrite) {
			t.Fatalf("did not expect direct document write tool %q in %#v", directWrite, step.AllowedTools)
		}
	}
	if !strings.Contains(step.Instructions, "Do not create or save a reusable agent") {
		t.Fatalf("expected one-shot instruction guardrail, got %q", step.Instructions)
	}
	if !strings.Contains(step.Instructions, "Goal:") || !strings.Contains(step.Instructions, "Plan:") || !strings.Contains(step.Instructions, "Constraints:") {
		t.Fatalf("expected structured execution brief, got %q", step.Instructions)
	}
	if !strings.Contains(step.Instructions, "Read the current document") {
		t.Fatalf("expected document-specific execution plan, got %q", step.Instructions)
	}
	if strings.Contains(step.Instructions, "publish_document_change_proposal") || strings.Contains(step.Instructions, "Do not call request_approval for Docs proposals") {
		t.Fatalf("did not expect Docs proposal submission instructions, got %q", step.Instructions)
	}
	if !strings.Contains(step.Instructions, "Do not use search_documents to rediscover or inspect a known current document") {
		t.Fatalf("expected current-doc search guardrail, got %q", step.Instructions)
	}
}

func TestParseIntentDeterministicallyPrefersKnownAgentBeforeOneShot(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-task", Name: "Task Planner", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{ID: "agent-command", Name: "Command Agent", PresetKey: model.AgentPresetCommandAgent, AllowedTargets: []string{"workspace"}},
	}

	resp := parseIntentDeterministically("break down this initiative into tasks", pageContext, commandBarNarrowCandidates(candidates))
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected known-agent plan, got %#v", resp)
	}
	if got := resp.Plan.Steps[0].AgentID; got != "agent-task" {
		t.Fatalf("expected task planner, got %q", got)
	}
}

func TestWorkspaceTaskQuestionPrefersOneShotCommandAgent(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-task", Name: "Atlas", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "get_task_context"},
		},
	}

	resp := parsePreferredOneShotCommandIntent("how many tasks in engineering team needs attention?", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" {
		t.Fatalf("expected Command Agent, got %q", step.AgentID)
	}
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot plan kind, got %q", step.PlanKind)
	}
	for _, required := range []string{"list_workspace_teams", "list_team_workflows_with_stages", "list_tasks"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if strings.Contains(strings.ToLower(strings.Join(step.AllowedTools, ",")), "create_task") {
		t.Fatalf("did not expect create_task for read-only task question: %#v", step.AllowedTools)
	}
	if !strings.Contains(step.Instructions, "Do not create, update, or move tasks") {
		t.Fatalf("expected read-only PM analysis guardrail, got %q", step.Instructions)
	}
}

func TestEpicStoryRankingQuestionPrefersOneShotCommandAgent(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     "epic-1",
		DisplayTitle: "Add new metrics - Prometheus",
		RelatedIDs:   map[string][]string{"task_ids": {"task-1", "task-2"}},
	}
	candidates := []model.CommandBarAgent{
		{ID: "agent-epic", Name: "Epic Planner", PresetKey: model.AgentPresetEpicPlanner, AllowedTargets: []string{"epic"}},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"epic"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "get_task_context"},
		},
	}

	resp := parsePreferredOneShotCommandIntent("which is the most important story in this epic?", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" {
		t.Fatalf("expected Command Agent, got %q", step.AgentID)
	}
	for _, required := range []string{"list_workspace_teams", "list_team_workflows_with_stages", "list_tasks"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if slices.Contains(step.AllowedTools, "create_task") {
		t.Fatalf("did not expect mutation tool for ranking question, got %#v", step.AllowedTools)
	}
	if !strings.Contains(step.Instructions, "Do not create, update, or move tasks") {
		t.Fatalf("expected read-only PM analysis guardrail, got %q", step.Instructions)
	}
}

func TestParseIntentWithLLMRoutesOneShotAndNarrowsTools(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-atlas", Name: "Atlas", Description: "Task planning agent", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			Description:    "One-shot workspace operator",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "create_task"},
		},
	}
	fakeLLM := &scriptedCommandBarLLM{responses: []string{`{
		"status":"plan",
		"route_kind":"one_shot_command",
		"agent_id":"agent-command",
		"instructions":"Count engineering tasks that need attention.",
		"one_shot_tools":["list_workspace_teams","list_tasks","create_task"],
		"tool_intent":"read_only",
		"rationale":"This is an ad hoc data question, not planning.",
		"confidence":0.91
	}`, `{
		"status":"plan",
		"route_kind":"one_shot_command",
		"agent_id":"agent-command",
		"instructions":"Count engineering tasks that need attention.",
		"one_shot_tools":["list_workspace_teams","list_tasks"],
		"tool_intent":"read_only",
		"rationale":"This is an ad hoc data question, not planning.",
		"confidence":0.91
	}`}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetLLMRouterConfig("openai", "gpt-5.5", 777, time.Second)

	resp := service.parseIntentWithLLM(context.Background(), "workspace-1", "how many tasks in engineering team needs attention?", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" || step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected command one-shot step, got %#v", step)
	}
	if !slices.Contains(step.AllowedTools, "list_tasks") || !slices.Contains(step.AllowedTools, "list_workspace_teams") {
		t.Fatalf("expected read tools, got %#v", step.AllowedTools)
	}
	if slices.Contains(step.AllowedTools, "create_task") {
		t.Fatalf("did not expect mutation tool for read-only question, got %#v", step.AllowedTools)
	}
	if len(fakeLLM.requests) != 2 {
		t.Fatalf("expected LLM retry after invalid read-only mutation tools, got %d requests", len(fakeLLM.requests))
	}
	if got := fakeLLM.requests[0].Provider; got != "openai" {
		t.Fatalf("expected provider openai, got %q", got)
	}
	if got := fakeLLM.requests[0].Model; got != "gpt-5.5" {
		t.Fatalf("expected model gpt-5.5, got %q", got)
	}
	if got := fakeLLM.requests[0].MaxTokens; got != 777 {
		t.Fatalf("expected max tokens 777, got %d", got)
	}
	if fakeLLM.requests[0].JSONSchema == nil {
		t.Fatalf("expected command router JSON schema for schema-forced providers")
	}
}

func TestCommandBarRouterAttachesOpenRouterProviderOptions(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{{
		ID:             "agent-command",
		Name:           "Command Agent",
		PresetKey:      model.AgentPresetCommandAgent,
		AllowedTargets: []string{"workspace"},
		AllowedTools:   []string{"list_tasks"},
	}}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"one_shot_command",
		"agent_id":"agent-command",
		"instructions":"Count open tasks.",
		"one_shot_tools":["list_tasks"],
		"rationale":"Ad hoc data question.",
		"confidence":0.91
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetLLMRouterConfig(model.AgentModelProviderOpenRouter, "openai/gpt-5.5", 777, time.Second).
		SetCommandRouterOpenRouterProviderOptions(json.RawMessage(`{"order":["openai"],"allow_fallbacks":false}`))

	resp := service.parseIntentWithLLM(context.Background(), "workspace-1", "how many open tasks?", pageContext, candidates)
	if resp == nil || resp.Plan == nil {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	if len(fakeLLM.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(fakeLLM.requests))
	}
	if string(fakeLLM.requests[0].ProviderOptions) != `{"order":["openai"],"allow_fallbacks":false}` {
		t.Fatalf("expected openrouter provider options, got %s", string(fakeLLM.requests[0].ProviderOptions))
	}
}

func TestCommandBarRouterOmitsOpenRouterProviderOptionsForNonOpenRouter(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"no_matching_agent",
		"route_kind":"no_matching_agent",
		"reason":"No match.",
		"confidence":0.2
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetLLMRouterConfig(model.AgentModelProviderOpenAI, "gpt-5.5", 777, time.Second).
		SetCommandRouterOpenRouterProviderOptions(json.RawMessage(`{"order":["openai"]}`))

	_ = service.parseIntentWithLLM(context.Background(), "workspace-1", "hello", pageContext, nil)
	if len(fakeLLM.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(fakeLLM.requests))
	}
	if len(fakeLLM.requests[0].ProviderOptions) != 0 {
		t.Fatalf("did not expect provider options for openai, got %s", string(fakeLLM.requests[0].ProviderOptions))
	}
}

func TestCommandBarRouterUsesOpenRouterMinimumTimeout(t *testing.T) {
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"no_matching_agent",
		"route_kind":"no_matching_agent",
		"reason":"No match.",
		"confidence":0.2
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetLLMRouterConfig(model.AgentModelProviderOpenRouter, "z-ai/glm-4.7", 777, time.Second)

	_ = service.parseIntentWithLLM(context.Background(), "workspace-1", "hello", model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}, nil)
	if len(fakeLLM.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(fakeLLM.requests))
	}
	if len(fakeLLM.deadlines) != 1 || fakeLLM.deadlines[0].IsZero() {
		t.Fatal("expected command router request context to have a deadline")
	}
	if remaining := time.Until(fakeLLM.deadlines[0]); remaining < 7*time.Second {
		t.Fatalf("expected openrouter minimum timeout near 8s, got remaining %s", remaining)
	}
}

func TestCommandBarRouterPromptTellsOneShotToUseWebToolsForExternalEvidence(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "document", EntityID: "doc-1", DisplayTitle: "Document"}
	candidates := []model.CommandBarAgent{{
		ID:             "agent-command",
		Name:           "Command Agent",
		PresetKey:      model.AgentPresetCommandAgent,
		AllowedTargets: []string{"document"},
		AllowedTools:   []string{"read_document", "web_search_exa", "web_search_brave", "fetch_url"},
	}}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"one_shot_command",
		"agent_id":"agent-command",
		"one_shot_tools":["read_document","web_search_exa","fetch_url"],
		"tool_intent":"read_only",
		"rationale":"Needs current external evidence.",
		"confidence":0.91
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM)

	resp := service.parseIntentWithLLM(context.Background(), "workspace-1", "is that relevant to the current trend? web search", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot plan, got %#v", resp)
	}
	prompt := fakeLLM.requests[0].SystemPrompt + "\n" + fakeLLM.requests[0].Messages[0].Content
	if !strings.Contains(prompt, "current external evidence") || !strings.Contains(prompt, "include web search/fetch tools") {
		t.Fatalf("expected one-shot router prompt to require web tools for external evidence, got %s", prompt)
	}
}

func TestChatTurnOpenRouterProviderOptionsApplyToClassifierAndInlineChat(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"settings guidance","confidence":0.98}`,
		`{"type":"final","answer":"Open Settings."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetChatRepository(repository.NewCommandBarChatRepository(db)).
		SetLLMRouterConfig(model.AgentModelProviderOpenRouter, "openai/gpt-5.5", 777, time.Second).
		SetCommandRouterOpenRouterProviderOptions(json.RawMessage(`{"order":["openai"]}`))

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "How do I manage settings?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalInlineAnswer {
		t.Fatalf("expected inline answer proposal, got %#v", resp.Proposal)
	}
	if len(fakeLLM.requests) != 2 {
		t.Fatalf("expected classifier and inline chat requests, got %d", len(fakeLLM.requests))
	}
	for idx, req := range fakeLLM.requests {
		if string(req.ProviderOptions) != `{"order":["openai"]}` {
			t.Fatalf("request %d missing provider options: %s", idx, string(req.ProviderOptions))
		}
	}
}

func TestChatClassifierPromptRoutesExternalEvidenceOutsideInlineToolsToOneShot(t *testing.T) {
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	fakeLLM := &scriptedCommandBarLLM{response: `{"route":"one_shot_command","reason":"needs current web evidence","confidence":0.98}`}
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetInternalCommandService(commandService)

	classification, err := service.classifyCommandBarChatIntent(ctx, workspaceID, "is that relevant to the current trend? web search", model.CommandBarPageContext{EntityType: "document", EntityID: "doc-1", DisplayTitle: "Wire error tracking into metrics middleware Plan"}, fullCommandBarChatAccess(), nil)
	if err != nil {
		t.Fatalf("classify command bar chat intent: %v", err)
	}
	if classification == nil || classification.Route != "one_shot_command" {
		t.Fatalf("expected one-shot classification, got %#v", classification)
	}
	if len(fakeLLM.requests) != 1 {
		t.Fatalf("expected one classifier request, got %d", len(fakeLLM.requests))
	}
	prompt := fakeLLM.requests[0].SystemPrompt + "\n" + fakeLLM.requests[0].Messages[0].Content
	if !strings.Contains(prompt, "current external evidence") || !strings.Contains(prompt, `route "one_shot_command"`) {
		t.Fatalf("expected classifier prompt to route external evidence outside inline tools to one-shot, got %s", prompt)
	}
	if strings.Contains(prompt, `"name":"web_search_exa"`) || strings.Contains(prompt, `"name":"web_search_brave"`) || strings.Contains(prompt, `"name":"fetch_url"`) {
		t.Fatalf("test setup expected no inline web tools, got prompt %s", prompt)
	}
}

func TestParseIntentWithLLMRoutesMultiStepSavedAgents(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "task", EntityID: "task-1"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-forge", Name: "Forge", PresetKey: model.AgentPresetCodeBuilder, AllowedTargets: []string{"task"}},
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
		{ID: "agent-command", Name: "Command Agent", PresetKey: model.AgentPresetCommandAgent, AllowedTargets: []string{"task"}, AllowedTools: []string{"get_task_context"}},
	}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"multi_step",
		"steps":[
			{"agent_id":"agent-forge","instructions":"Implement the requested task."},
			{"agent_id":"agent-lens","instructions":"Review Forge's result."}
		],
		"rationale":"The user asked for implementation followed by review.",
		"confidence":0.93
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM)

	resp := service.parseIntentWithLLM(context.Background(), "workspace-1", "have Forge implement then Lens review", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 2 {
		t.Fatalf("expected two-step saved-agent plan, got %#v", resp)
	}
	if resp.Plan.Steps[0].AgentID != "agent-forge" || resp.Plan.Steps[1].AgentID != "agent-lens" {
		t.Fatalf("unexpected step order: %#v", resp.Plan.Steps)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindKnownAgent {
		t.Fatalf("expected known-agent plan kind, got %q", resp.Plan.PlanKind)
	}
}

func TestParseIntentWithLLMRoutesDAG(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1", DisplayTitle: "Workspace"}
	candidates := []model.CommandBarAgent{
		{ID: "agent-atlas", Name: "Atlas", PresetKey: model.AgentPresetTaskPlanner, AllowedTargets: []string{"workspace"}},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"list_tasks", "list_workspace_teams", "create_task"},
		},
	}
	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status":"plan",
		"route_kind":"dag",
		"steps":[
			{
				"agent_id":"agent-command",
				"target":{"entity_type":"workspace","entity_id":"workspace-1","display_title":"Workspace"},
				"instructions":"Find engineering tasks needing attention and group by team.",
				"allowed_tools":["list_tasks","list_workspace_teams"],
				"depends_on_step_indexes":[]
			},
			{
				"agent_id":"agent-atlas",
				"target":{"entity_type":"workspace","entity_id":"workspace-1","display_title":"Workspace"},
				"instructions":"Summarize the triage results and recommend next actions.",
				"depends_on_step_indexes":[0]
			}
		],
		"rationale":"The request needs discovery followed by synthesis.",
		"confidence":0.88
	}`}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM)

	resp := service.parseIntentWithLLM(context.Background(), "workspace-1", "find engineering tasks needing attention, then summarize next actions", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 2 {
		t.Fatalf("expected DAG plan, got %#v", resp)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindDAG {
		t.Fatalf("expected DAG plan kind, got %q", resp.Plan.PlanKind)
	}
	if resp.Plan.Steps[0].PlanKind != model.CommandBarPlanKindDAG || resp.Plan.Steps[1].PlanKind != model.CommandBarPlanKindDAG {
		t.Fatalf("expected DAG step kinds, got %#v", resp.Plan.Steps)
	}
	if got := resp.Plan.Steps[1].DependsOnStepIndexes; len(got) != 1 || got[0] != 0 {
		t.Fatalf("expected second step to depend on first, got %#v", got)
	}
	if slices.Contains(resp.Plan.Steps[0].AllowedTools, "create_task") {
		t.Fatalf("did not expect mutation tool in read-only DAG step, got %#v", resp.Plan.Steps[0].AllowedTools)
	}
}

func TestValidateCommandBarStepDependenciesRejectsCycle(t *testing.T) {
	steps := []model.CommandBarPlanStep{
		{PlanKind: model.CommandBarPlanKindDAG, DependsOnStepIndexes: []int{1}},
		{PlanKind: model.CommandBarPlanKindDAG, DependsOnStepIndexes: []int{0}},
	}
	err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle validation error, got %v", err)
	}
}

func TestValidateCommandBarStepDependenciesRejectsInitialFanOutCap(t *testing.T) {
	steps := make([]model.CommandBarPlanStep, 0, maxCommandBarDAGInitialFanOut+1)
	for i := 0; i < maxCommandBarDAGInitialFanOut+1; i++ {
		steps = append(steps, model.CommandBarPlanStep{PlanKind: model.CommandBarPlanKindDAG})
	}
	err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut)
	if err == nil || !strings.Contains(err.Error(), "initially runnable") {
		t.Fatalf("expected fan-out validation error, got %v", err)
	}
}

func TestCommandBarSchedulerReadinessRequiresCompletedDependencies(t *testing.T) {
	completedRunID := "run-completed"
	runningRunID := "run-running"
	runsByID := map[string]model.AgentRun{
		completedRunID: {ID: completedRunID, Status: model.AgentRunStatusCompleted},
		runningRunID:   {ID: runningRunID, Status: model.AgentRunStatusRunning},
	}

	linearSteps := []model.CommandBarPlanStep{
		{},
		{DependsOnStepIndexes: []int{0}},
	}
	if !commandBarStepDependenciesSatisfied(linearSteps[1], map[int]string{0: completedRunID}, runsByID) {
		t.Fatalf("expected completed dependency to make step ready")
	}
	parentRunID := commandBarParentRunIDForStep(linearSteps, 1, map[int]string{0: completedRunID}, runsByID)
	if parentRunID == nil || *parentRunID != completedRunID {
		t.Fatalf("expected one-to-one dependency to provide parent run %q, got %v", completedRunID, parentRunID)
	}

	if commandBarStepDependenciesSatisfied(
		model.CommandBarPlanStep{DependsOnStepIndexes: []int{1}},
		map[int]string{1: runningRunID},
		runsByID,
	) {
		t.Fatalf("expected running dependency to keep step blocked")
	}

	if commandBarStepDependenciesSatisfied(
		model.CommandBarPlanStep{DependsOnStepIndexes: []int{2}},
		map[int]string{},
		runsByID,
	) {
		t.Fatalf("expected missing dependency run to keep step blocked")
	}

	sharedDependencySteps := []model.CommandBarPlanStep{
		{},
		{DependsOnStepIndexes: []int{0}},
		{DependsOnStepIndexes: []int{0}},
	}
	if parent := commandBarParentRunIDForStep(sharedDependencySteps, 1, map[int]string{0: completedRunID}, runsByID); parent != nil {
		t.Fatalf("expected shared dependency fan-out to avoid parent_run_id, got %v", *parent)
	}
}

func TestCommandBarExistingChildRunForParent(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	service := &AgentService{runRepo: runRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	parentRunID := "22222222-2222-2222-2222-222222222222"
	childRunID := "33333333-3333-3333-3333-333333333333"
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             childRunID,
		WorkspaceID:    workspaceID,
		AgentID:        "44444444-4444-4444-4444-444444444444",
		TargetType:     "task",
		TargetID:       "55555555-5555-5555-5555-555555555555",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ParentRunID:    &parentRunID,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusQueued,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create child run: %v", err)
	}

	existing := service.commandBarExistingChildRunForParent(ctx, workspaceID, &parentRunID)
	if existing == nil || existing.ID != childRunID {
		t.Fatalf("expected existing child run %q, got %#v", childRunID, existing)
	}
	if got := service.commandBarExistingChildRunForParent(ctx, workspaceID, nil); got != nil {
		t.Fatalf("expected nil for nil parent run id, got %#v", got)
	}
}

func TestCommandBarExistingRunForStepRequiresExactCommandBarStep(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	service := &AgentService{runRepo: runRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	parentRunID := "33333333-3333-3333-3333-333333333333"
	childRunID := "44444444-4444-4444-4444-444444444444"
	taskID := "55555555-5555-5555-5555-555555555555"
	steps := []model.CommandBarPlanStep{
		{
			AgentID:   "agent-forge",
			AgentName: "Forge",
			Target:    model.CommandBarPageContext{EntityType: "task", EntityID: taskID, DisplayTitle: "Task"},
		},
		{
			AgentID:              "agent-lens",
			AgentName:            "Lens",
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: taskID, DisplayTitle: "Task"},
			DependsOnStepIndexes: []int{0},
		},
	}
	trigger, err := buildCommandBarTriggerContext("run forge then lens", model.CommandBarPageContext{EntityType: "task", EntityID: taskID}, steps, 1, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, err := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             childRunID,
		WorkspaceID:    workspaceID,
		AgentID:        "agent-lens",
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ParentRunID:    &parentRunID,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusQueued,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create child run: %v", err)
	}

	existing := service.commandBarExistingRunForStep(ctx, workspaceID, &parentRunID, planID, steps, 1)
	if existing == nil || existing.ID != childRunID {
		t.Fatalf("expected exact existing child run %q, got %#v", childRunID, existing)
	}
	if got := service.commandBarExistingRunForStep(ctx, workspaceID, &parentRunID, planID, steps, 0); got != nil {
		t.Fatalf("expected mismatched step not to reuse child run, got %#v", got)
	}
}

func TestCreateRunAllowsConflictChildWhenActiveRunIsParent(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	taskID := "22222222-2222-2222-2222-222222222222"
	parentRunID := "33333333-3333-3333-3333-333333333333"
	forgeAgent := &model.Agent{
		ID:                    "44444444-4444-4444-4444-444444444444",
		WorkspaceID:           workspaceID,
		Name:                  "Forge",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             parentRunID,
		WorkspaceID:    workspaceID,
		AgentID:        "55555555-5555-5555-5555-555555555555",
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "internal",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create parent run: %v", err)
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          forgeAgent,
		targetType:     "task",
		targetID:       taskID,
		parentRunID:    &parentRunID,
		taskID:         &taskID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected active parent to block normal child run, got %v", err)
	}

	_, err = service.createRun(ctx, createRunParams{
		workspaceID:          workspaceID,
		agent:                forgeAgent,
		targetType:           "task",
		targetID:             taskID,
		parentRunID:          &parentRunID,
		allowActiveParentRun: true,
		taskID:               &taskID,
		input:                []byte("{}"),
		invocationMode:       model.InvocationModeAutonomous,
	})
	if err == nil || strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected active parent exception to bypass duplicate-run guard, got %v", err)
	}
}

func TestCreateRunAllowsDifferentAgentsOnWorkspaceTarget(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	existingAgentID := "22222222-2222-2222-2222-222222222222"
	nextAgent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Competitive Intelligence Digest",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}

	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    workspaceID,
		AgentID:        existingAgentID,
		TargetType:     "workspace",
		TargetID:       workspaceID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanInput,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create existing workspace run: %v", err)
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          nextAgent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected different workspace agent to bypass duplicate-run guard, got %v", err)
	}

	runs, err := runRepo.ListByTarget(ctx, workspaceID, "workspace", workspaceID)
	if err != nil {
		t.Fatalf("list workspace runs: %v", err)
	}
	foundNextAgentRun := false
	for _, run := range runs {
		if run.AgentID == nextAgent.ID {
			foundNextAgentRun = true
			break
		}
	}
	if !foundNextAgentRun {
		t.Fatalf("expected new workspace run for agent %q to be created alongside existing run, got %#v", nextAgent.ID, runs)
	}
}

func TestCreateRunPreflightsAICreditsBeforeQueueingRun(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	consumer := &recordingAIUsageConsumer{preflightErr: fmt.Errorf("AI usage exhausted")}
	service := (&AgentService{runRepo: runRepo, agentRepo: agentRepo}).SetAIUsageMeter(NewAIUsageMeter(consumer))

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	taskID := "22222222-2222-2222-2222-222222222222"
	agent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Forge",
		PresetKey:             model.AgentPresetCodeBuilder,
		IsSystem:              true,
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "task",
		targetID:       taskID,
		taskID:         &taskID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "AI usage exhausted") {
		t.Fatalf("createRun() error = %v, want AI usage exhausted", err)
	}
	if consumer.preflight.WorkspaceID != workspaceID || consumer.preflight.FeatureKey != BillingFeatureForgeRun || consumer.preflight.Credits != 100 {
		t.Fatalf("preflight = %#v, want Forge run preflight", consumer.preflight)
	}
	runs, total, err := runRepo.ListByWorkspace(ctx, workspaceID, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if total != 0 || len(runs) != 0 {
		t.Fatalf("expected no queued run after failed preflight, total=%d runs=%#v", total, runs)
	}
}

func seedCreateRunAgentRow(t *testing.T, db *gorm.DB, agent *model.Agent) {
	t.Helper()
	now := time.Now().UTC()
	execConfig := strings.TrimSpace(string(agent.ExecutionConfig))
	if execConfig == "" {
		execConfig = "{}"
	}
	allowedTools := strings.TrimSpace(string(agent.AllowedTools))
	if allowedTools == "" {
		allowedTools = "[]"
	}
	allowedTargets := strings.TrimSpace(string(agent.AllowedTargets))
	if allowedTargets == "" {
		allowedTargets = "[]"
	}
	allowedCommands := strings.TrimSpace(string(agent.AllowedCommands))
	if allowedCommands == "" {
		allowedCommands = "[]"
	}
	skills, err := json.Marshal(agent.Skills.Normalize())
	if err != nil {
		t.Fatalf("marshal agent skills: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO agents (
			id, workspace_id, is_system, name, preset_key, status, runtime_kind,
			skills, trigger_mode, provider, model, execution_config, system_prompt,
			allowed_tools, allowed_commands, allowed_targets, approval_mode,
			max_concurrent_runs, default_invocation_mode, tokens_used_this_month,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.ID,
		agent.WorkspaceID,
		agent.IsSystem,
		agent.Name,
		agent.PresetKey,
		agent.Status,
		agent.RuntimeKind,
		string(skills),
		defaultString(agent.TriggerMode, "manual"),
		agent.Provider,
		agent.Model,
		execConfig,
		agent.SystemPrompt,
		allowedTools,
		allowedCommands,
		allowedTargets,
		agent.ApprovalMode,
		agent.MaxConcurrentRuns,
		agent.DefaultInvocationMode,
		agent.TokensUsedThisMonth,
		now,
		now,
	).Error; err != nil {
		t.Fatalf("seed agent row: %v", err)
	}
}

func TestCreateRunDelegatesMiraWorkspaceRunToAgentRuntime(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	service := (&AgentService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	prompt := "You are Mira."
	provider := "openai"
	modelName := "gpt-5.5"
	agent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Mira",
		PresetKey:             model.AgentPresetMarketer,
		RuntimeKind:           "native_sdk",
		Provider:              &provider,
		Model:                 &modelName,
		SystemPrompt:          &prompt,
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeInteractive,
		AllowedTargets:        json.RawMessage(`["workspace","document"]`),
		AllowedTools:          json.RawMessage(`["update_plan","request_user_input"]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{{Key: "marketing_context_setup"}},
		ExecutionConfig:       model.JSONBlob(`{"reasoning_effort":"medium"}`),
		MaxConcurrentRuns:     1,
	}
	seedCreateRunAgentRow(t, db, agent)
	additionalContext := "Prepare a short launch plan."
	payload, err := buildAgentRunInputPayload("workspace", workspaceID, manualRunTriggerContext(), nil, nil, &additionalContext, []string{"update_plan"})
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}

	run, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		actorID:        &actorID,
		input:          payload,
		invocationMode: model.InvocationModeInteractive,
	})
	if err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if len(runtimeClient.upsertAgents) != 1 {
		t.Fatalf("expected one runtime agent upsert, got %d", len(runtimeClient.upsertAgents))
	}
	upsert := runtimeClient.upsertAgents[0]
	if upsert.ID != agent.ID || upsert.AppID != "helpin" || upsert.Name != "Mira" || upsert.SystemPrompt != prompt {
		t.Fatalf("unexpected upserted agent: %#v", upsert)
	}
	if !slices.Equal(upsert.AllowedTargets, []string{"workspace", "document"}) || !slices.Equal(upsert.AllowedTools, []string{"update_plan", "request_user_input"}) {
		t.Fatalf("unexpected upserted permissions: targets=%#v tools=%#v", upsert.AllowedTargets, upsert.AllowedTools)
	}
	if len(upsert.Skills) != 1 || upsert.Skills[0].Key != "marketing_context_setup" {
		t.Fatalf("expected runtime skill refs, got %#v", upsert.Skills)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.HostRunID != run.ID || start.AgentID != agent.ID || start.Target.Type != "workspace" || start.Target.ID != workspaceID {
		t.Fatalf("unexpected runtime start request: %#v", start)
	}
	if start.ExternalActorID != actorID || start.Mode != model.InvocationModeInteractive || start.ExecutionMode != agentRuntimeExecutionModeDurable || start.Instructions != additionalContext {
		t.Fatalf("unexpected runtime start mode/actor/instructions: %#v", start)
	}
	if !slices.Equal(start.AllowedTools, []string{"update_plan"}) {
		t.Fatalf("unexpected start allowed tools: %#v", start.AllowedTools)
	}
	if start.TurnPolicy.Mode != "pause_after_assistant" {
		t.Fatalf("expected interactive turn policy, got %#v", start.TurnPolicy)
	}
	if start.Metadata["workspace_id"] != workspaceID || start.Metadata["helpin_run_id"] != run.ID || start.Target.Metadata["workspace_id"] != workspaceID {
		t.Fatalf("unexpected runtime metadata: metadata=%#v target=%#v", start.Metadata, start.Target.Metadata)
	}

	reloaded, err := runRepo.GetByID(ctx, workspaceID, run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.ExternalRuntime == nil || *reloaded.ExternalRuntime != agentRuntimeName || reloaded.ExternalRuntimeID == nil || *reloaded.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected delegated runtime mapping, got external_runtime=%v external_runtime_id=%v", reloaded.ExternalRuntime, reloaded.ExternalRuntimeID)
	}
	if reloaded.WorkflowID != nil || reloaded.WorkflowRunID != nil {
		t.Fatalf("expected no Helpin Temporal workflow IDs, got %v/%v", reloaded.WorkflowID, reloaded.WorkflowRunID)
	}
}

func TestShouldDelegateRunToAgentRuntimeUsesPresetNotName(t *testing.T) {
	if shouldDelegateRunToAgentRuntime(&model.Agent{Name: "Mira"}, "workspace") {
		t.Fatal("custom agent named Mira should not delegate without marketer preset")
	}
	if !shouldDelegateRunToAgentRuntime(&model.Agent{Name: "Launch Ops", PresetKey: model.AgentPresetMarketer}, "workspace") {
		t.Fatal("marketer preset workspace run should delegate")
	}
	if shouldDelegateRunToAgentRuntime(&model.Agent{Name: "Mira", PresetKey: model.AgentPresetMarketer}, "task") {
		t.Fatal("non-workspace target should not delegate")
	}
}

func TestCreateRunRetriesDelegatedRuntimeStartOnce(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{startRunErrs: []error{errors.New("runtime timeout")}}
	service := (&AgentService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agent := &model.Agent{
		ID:                    "22222222-2222-2222-2222-222222222222",
		WorkspaceID:           workspaceID,
		Name:                  "Mira",
		PresetKey:             model.AgentPresetMarketer,
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
		AllowedTargets:        json.RawMessage(`["workspace"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		MaxConcurrentRuns:     1,
	}
	seedCreateRunAgentRow(t, db, agent)

	run, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte(`{"target":{"target_type":"workspace","target_id":"11111111-1111-1111-1111-111111111111"}}`),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if len(runtimeClient.startRunCalls) != 2 {
		t.Fatalf("expected runtime start retry, got %d calls", len(runtimeClient.startRunCalls))
	}
	if runtimeClient.startRunCalls[0].HostRunID != run.ID || runtimeClient.startRunCalls[1].HostRunID != run.ID {
		t.Fatalf("retry should preserve host_run_id, calls=%#v", runtimeClient.startRunCalls)
	}
	reloaded, err := runRepo.GetByID(ctx, workspaceID, run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.ExternalRuntimeID == nil || *reloaded.ExternalRuntimeID != "run_runtime_1" || reloaded.Status != model.AgentRunStatusQueued {
		t.Fatalf("expected delegated mapping after retry, got %#v", reloaded)
	}
}

func TestCreateRunMarksDelegatedMiraRunFailedWhenRuntimeStartFails(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{startRunErr: errors.New("runtime unavailable")}
	service := (&AgentService{
		runRepo:            runRepo,
		agentRepo:          agentRepo,
		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agent := &model.Agent{
		ID:                    "22222222-2222-2222-2222-222222222222",
		WorkspaceID:           workspaceID,
		Name:                  "Mira",
		PresetKey:             model.AgentPresetMarketer,
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
		AllowedTargets:        json.RawMessage(`["workspace"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		MaxConcurrentRuns:     1,
	}
	seedCreateRunAgentRow(t, db, agent)
	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte(`{"target":{"target_type":"workspace","target_id":"11111111-1111-1111-1111-111111111111"}}`),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "runtime unavailable") {
		t.Fatalf("expected runtime start error, got %v", err)
	}
	runs, total, err := runRepo.ListByWorkspace(ctx, workspaceID, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if total != 1 || len(runs) != 1 {
		t.Fatalf("expected one failed run, total=%d runs=%#v", total, runs)
	}
	failed := runs[0]
	if failed.Status != model.AgentRunStatusFailed || failed.ExecutionStage == nil || *failed.ExecutionStage != "failed_to_start" || failed.ErrorMessage == nil || !strings.Contains(*failed.ErrorMessage, "runtime unavailable") {
		t.Fatalf("unexpected failed run state: %#v", failed)
	}
	if failed.ExternalRuntime != nil || failed.ExternalRuntimeID != nil {
		t.Fatalf("failed runtime start should not stamp external mapping, got %v/%v", failed.ExternalRuntime, failed.ExternalRuntimeID)
	}
	var status string
	if err := db.WithContext(ctx).Raw("SELECT status FROM agents WHERE workspace_id = ? AND id = ?", workspaceID, agent.ID).Scan(&status).Error; err != nil {
		t.Fatalf("query agent status: %v", err)
	}
	if status != "idle" {
		t.Fatalf("expected agent to be idle after failed runtime start, got %q", status)
	}
}

func TestCreateRunDedupesSameAgentOnWorkspaceTarget(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agent := &model.Agent{
		ID:                    "22222222-2222-2222-2222-222222222222",
		WorkspaceID:           workspaceID,
		Name:                  "Competitive Intelligence Digest",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	existingRunID := "33333333-3333-3333-3333-333333333333"
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             existingRunID,
		WorkspaceID:    workspaceID,
		AgentID:        agent.ID,
		TargetType:     "workspace",
		TargetID:       workspaceID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonHumanInput,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create existing workspace run: %v", err)
	}

	run, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "workspace",
		targetID:       workspaceID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err != nil {
		t.Fatalf("expected same workspace agent to reuse existing active run, got error %v", err)
	}
	if run == nil || run.ID != existingRunID {
		t.Fatalf("expected existing workspace run %q, got %#v", existingRunID, run)
	}
}

func TestCreateRunStillBlocksDifferentAgentsOnTaskTarget(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	service := &AgentService{runRepo: runRepo, agentRepo: agentRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	taskID := "22222222-2222-2222-2222-222222222222"
	nextAgent := &model.Agent{
		ID:                    "33333333-3333-3333-3333-333333333333",
		WorkspaceID:           workspaceID,
		Name:                  "Forge",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    workspaceID,
		AgentID:        "55555555-5555-5555-5555-555555555555",
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create existing task run: %v", err)
	}

	_, err := service.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          nextAgent,
		targetType:     "task",
		targetID:       taskID,
		taskID:         &taskID,
		input:          []byte("{}"),
		invocationMode: model.InvocationModeAutonomous,
	})
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected different task agent to remain blocked by duplicate-run guard, got %v", err)
	}
}

func TestParseOneShotCommandIntentRejectsUnsupportedMutation(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input"},
		},
	}

	if resp := parseOneShotCommandIntent("delete workspace", pageContext, candidates); resp != nil {
		t.Fatalf("expected unsupported mutation to stay unmatched, got %#v", resp)
	}
}

func TestParseSafeOneShotFallbackUsesReadOnlyTargetTools(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks", "create_task"},
		},
	}

	resp := parseSafeOneShotCommandFallback("what should I look at first in this workspace?", pageContext, candidates)
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected safe one-shot fallback, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand || step.AgentID != "agent-command" {
		t.Fatalf("expected Command Agent one-shot step, got %#v", step)
	}
	for _, required := range []string{"list_workspace_teams", "list_team_workflows_with_stages", "list_tasks"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected fallback tool %q in %#v", required, step.AllowedTools)
		}
	}
	if slices.Contains(step.AllowedTools, "create_task") {
		t.Fatalf("did not expect mutation tool in safe fallback: %#v", step.AllowedTools)
	}
	if !strings.Contains(step.Instructions, "Fallback routing") {
		t.Fatalf("expected fallback routing instructions, got %q", step.Instructions)
	}
}

func TestParseSafeOneShotFallbackRejectsUnsafePrompt(t *testing.T) {
	pageContext := model.CommandBarPageContext{EntityType: "workspace", EntityID: "workspace-1"}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"workspace"},
			AllowedTools:   []string{"update_plan", "request_user_input", "list_workspace_teams", "list_team_workflows_with_stages", "list_tasks"},
		},
	}

	if resp := parseSafeOneShotCommandFallback("delete all tasks in this workspace", pageContext, candidates); resp != nil {
		t.Fatalf("expected unsafe prompt to stay unmatched, got %#v", resp)
	}
}

func TestParseIntentFallsBackToOneShotAfterLLMNoMatch(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"

	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Command Agent', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"22222222-2222-2222-2222-222222222222",
		workspaceID,
		model.AgentPresetCommandAgent,
		[]byte(`["update_plan","request_user_input","list_workspace_teams","list_team_workflows_with_stages","list_tasks"]`),
		[]byte(`[]`),
		[]byte(`["workspace"]`),
	).Error; err != nil {
		t.Fatalf("seed command agent: %v", err)
	}

	fakeLLM := &scriptedCommandBarLLM{response: `{
		"status": "no_matching_agent",
		"route_kind": "no_matching_agent",
		"agent_id": "",
		"instructions": "",
		"steps": [],
		"one_shot_tools": [],
		"rationale": "No saved agent matched.",
		"reason": "No saved agent matched.",
		"clarifying_question": "",
		"confidence": 0.2
	}`}
	service := NewCommandBarService(&AgentService{agentRepo: agentRepo}, nil, nil, nil, fakeLLM)

	resp, err := service.ParseIntent(ctx, workspaceID, "actor-1", model.CommandBarParseRequest{
		Text:        "what should I look at first in this workspace?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("parse intent: %v", err)
	}
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected one-shot fallback plan after llm no-match, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentKey != model.AgentPresetCommandAgent || step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected Command Agent fallback, got %#v", step)
	}
	if !slices.Contains(step.AllowedTools, "list_tasks") {
		t.Fatalf("expected read-only task context tools, got %#v", step.AllowedTools)
	}
}

func TestChatTurnClassifierTimeoutDoesNotFallBackToOneShot(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"

	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Command Agent', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"22222222-2222-2222-2222-222222222222",
		workspaceID,
		model.AgentPresetCommandAgent,
		[]byte(`["update_plan","request_user_input","list_workspace_teams","list_team_workflows_with_stages","list_tasks"]`),
		[]byte(`[]`),
		[]byte(`["workspace"]`),
	).Error; err != nil {
		t.Fatalf("seed command agent: %v", err)
	}

	fakeLLM := &scriptedCommandBarLLM{
		errors: []error{context.DeadlineExceeded, nil},
		responses: []string{
			`{"type":"final","answer":"You have 3 open tasks assigned to you."}`,
		},
	}
	service := NewCommandBarService(&AgentService{agentRepo: agentRepo}, nil, nil, nil, fakeLLM).
		SetChatRepository(repository.NewCommandBarChatRepository(db))

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "how many open tasks i have assigned to me?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Usermaven"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalInlineAnswer {
		t.Fatalf("expected classifier timeout to stay inline, got %#v", resp.Proposal)
	}
	if resp.Proposal.Plan != nil {
		t.Fatalf("classifier timeout should not produce one-shot plan")
	}
	if resp.AssistantMessage.Content != "You have 3 open tasks assigned to you." {
		t.Fatalf("expected inline model answer, got %q", resp.AssistantMessage.Content)
	}
	if len(fakeLLM.requests) != 2 {
		t.Fatalf("expected classifier call then inline chat call, got %d", len(fakeLLM.requests))
	}
	if got := fakeLLM.requests[0].Messages[0].Content; !strings.Contains(got, "how many open tasks") {
		t.Fatalf("expected classifier to receive user question, got %s", got)
	}
}

func TestChatTurnInlineReadOnlyPersistsMessagesWithoutRunPlan(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"settings guidance","confidence":0.98}`,
		`{"type":"final","answer":"Open Settings, then choose Teams to manage team configuration."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetChatRepository(repository.NewCommandBarChatRepository(db))

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "How do I configure teams in Helpin settings?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalInlineAnswer {
		t.Fatalf("expected inline answer proposal, got %#v", resp.Proposal)
	}
	if resp.Proposal.Plan != nil {
		t.Fatalf("inline answer should not include a run plan")
	}
	if !strings.Contains(resp.AssistantMessage.Content, "Settings") {
		t.Fatalf("expected assistant answer content, got %q", resp.AssistantMessage.Content)
	}
	var messageCount int64
	if err := db.Table("command_bar_messages").Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 2 {
		t.Fatalf("expected user and assistant messages, got %d", messageCount)
	}
}

func TestChatTurnInlineReadOnlyDeniesUnavailableDomainAccess(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	fakeLLM := &scriptedCommandBarLLM{response: "Leaked CRM data"}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetChatRepository(repository.NewCommandBarChatRepository(db))

	resp, err := service.ChatTurnWithAccess(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "list CRM contacts",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID},
	}, CommandBarChatAccess{CanReadPM: true, CanReadDocs: true, CanReadCRM: false})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalInlineAnswer {
		t.Fatalf("expected inline answer proposal, got %#v", resp.Proposal)
	}
	if strings.Contains(resp.AssistantMessage.Content, "Leaked") || !strings.Contains(resp.AssistantMessage.Content, "cannot access CRM") {
		t.Fatalf("expected denied CRM answer without llm fallback, got %q", resp.AssistantMessage.Content)
	}
	if len(fakeLLM.requests) != 0 {
		t.Fatalf("expected no llm call for denied domain access, got %d", len(fakeLLM.requests))
	}
}

func TestInlineReadOnlyChatUsesModelRequestedTools(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-1"
	now := time.Now()
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", "actor-1")
	seedWorkflow(t, db, "wf-1", workspaceID, "state-1")
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"team-eng", workspaceID, "Engineering", "eng", "engineering", "feature", now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"team-growth", workspaceID, "Growth", "growth", "growth", "task", now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, team_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-open-eng", workspaceID, 1, "Build API", model.PMTaskTypeFeature, "wf-1", "state-1", "team-eng", "medium", "normal", false, false, now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, team_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-done-eng", workspaceID, 2, "Ship API", model.PMTaskTypeFeature, "wf-1", "state-1", "team-eng", "medium", "normal", true, false, now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, team_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-open-growth", workspaceID, 3, "Launch campaign", model.PMTaskTypeChore, "wf-1", "state-1", "team-growth", "medium", "normal", false, false, now, now)

	taskRepo := repository.NewPMTaskRepository(db)
	taskService := NewPMTaskService(
		taskRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		nil,
		nil,
	)
	commandService := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, nil)
	commandService.SetSettingsRepository(repository.NewSettingsRepository(db))
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"type":"tool_call","tool":"list_workspace_teams","input":{}}`,
		`{"type":"tool_call","tool":"list_tasks","input":{"team_id":"team-eng","open_only":true,"limit":10}}`,
		`{"type":"final","answer":"Engineering has 1 open task."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetInternalCommandService(commandService)

	answer, inlineContext := service.inlineReadOnlyAnswer(ctx, workspaceID, "actor-1", "how many open tasks do we have in engineering", model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID}, fullCommandBarChatAccess(), nil)
	if answer != "Engineering has 1 open task." {
		t.Fatalf("expected model final answer, got %q", answer)
	}
	if len(fakeLLM.requests) != 3 {
		t.Fatalf("expected iterative tool chat calls, got %d", len(fakeLLM.requests))
	}
	var toolContext commandBarReadOnlyToolContext
	if err := json.Unmarshal(inlineContext, &toolContext); err != nil {
		t.Fatalf("decode inline context: %v", err)
	}
	if len(toolContext.ToolCalls) != 2 || toolContext.ToolCalls[0].Tool != "list_workspace_teams" || toolContext.ToolCalls[1].Tool != "list_tasks" {
		t.Fatalf("expected workspace teams then task tools, got %#v", toolContext.ToolCalls)
	}
	if !strings.Contains(string(toolContext.ToolCalls[1].Output), `"total":1`) || strings.Contains(string(toolContext.ToolCalls[1].Output), "Launch campaign") {
		t.Fatalf("expected team-filtered task output, got %s", string(toolContext.ToolCalls[1].Output))
	}
}

func TestInlineReadOnlyChatUsesDocsToolForDocumentPublishCount(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-1"
	now := time.Now()
	mustExec(t, db, `CREATE TABLE docs_documents (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		collection_id TEXT,
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		team_id TEXT,
		owner_id TEXT,
		excerpt TEXT,
		is_pinned BOOLEAN NOT NULL DEFAULT 0,
		published_at DATETIME,
		next_review_at DATETIME,
		created_by TEXT NOT NULL,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE docs_change_proposals (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		document_id TEXT NOT NULL,
		block_id TEXT,
		agent_id TEXT,
		agent_run_id TEXT,
		scope TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		revision INTEGER NOT NULL DEFAULT 0,
		summary TEXT NOT NULL,
		content_markdown TEXT NOT NULL,
		content JSON NOT NULL,
		sources JSON NOT NULL DEFAULT '[]',
		created_by TEXT NOT NULL,
		resolved_by TEXT,
		resolved_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	for _, row := range []struct {
		id     string
		title  string
		status string
	}{
		{"doc-draft-1", "Draft API guide", model.DocStatusDraft},
		{"doc-draft-2", "Draft release notes", model.DocStatusDraft},
		{"doc-published-1", "Published help article", model.DocStatusPublished},
	} {
		mustExec(t, db, `INSERT INTO docs_documents (id, workspace_id, space_id, title, status, is_pinned, created_by, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			row.id, workspaceID, "space-1", row.title, row.status, false, "actor-1", now)
	}
	docsSvc := NewDocsDocumentService(repository.NewDocsDocumentRepository(db), repository.NewDocsSpaceRepository(db), nil, false)
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commandService.SetDocsCreateDependencies(docsSvc, nil)
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"type":"tool_call","tool":"list_documents","input":{"status":"draft","limit":10}}`,
		`{"type":"final","answer":"There are 2 documents that need to be published."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetInternalCommandService(commandService)

	answer, inlineContext := service.inlineReadOnlyAnswer(ctx, workspaceID, "actor-1", "how many documents require to be published?", model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID}, fullCommandBarChatAccess(), nil)
	if answer != "There are 2 documents that need to be published." {
		t.Fatalf("expected docs answer, got %q", answer)
	}
	var toolContext commandBarReadOnlyToolContext
	if err := json.Unmarshal(inlineContext, &toolContext); err != nil {
		t.Fatalf("decode inline context: %v", err)
	}
	if len(toolContext.ToolCalls) != 1 || toolContext.ToolCalls[0].Tool != "list_documents" {
		t.Fatalf("expected list_documents tool call, got %#v", toolContext.ToolCalls)
	}
	output := string(toolContext.ToolCalls[0].Output)
	if !strings.Contains(output, `"total":2`) || strings.Contains(output, "Published help article") {
		t.Fatalf("expected draft-only docs output, got %s", output)
	}
	if toolContext.WorkingContext == nil || len(toolContext.WorkingContext.ResultSets) != 1 || toolContext.WorkingContext.ResultSets[0].EntityType != "document" {
		t.Fatalf("expected document working context, got %#v", toolContext.WorkingContext)
	}
	prompt := fakeLLM.requests[0].Messages[0].Content
	if !strings.Contains(prompt, `"name":"list_documents"`) {
		t.Fatalf("expected list_documents in available tools, got %s", prompt)
	}
}

func TestInlineReadOnlyChatCanReadCurrentDocument(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-1"
	documentID := "doc-1"
	blockID := "block-1"
	now := time.Now()
	mustExec(t, db, `CREATE TABLE docs_documents (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		collection_id TEXT,
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		visibility TEXT NOT NULL DEFAULT 'workspace_wide',
		team_id TEXT,
		owner_id TEXT,
		excerpt TEXT,
		is_pinned BOOLEAN NOT NULL DEFAULT 0,
		published_at DATETIME,
		next_review_at DATETIME,
		created_by TEXT NOT NULL,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE docs_contents (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL UNIQUE,
		content JSON,
		content_text TEXT,
		word_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE docs_blocks (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		document_id TEXT NOT NULL,
		parent_id TEXT,
		type TEXT NOT NULL,
		content JSON NOT NULL DEFAULT '{}',
		content_text TEXT,
		sort_key TEXT NOT NULL DEFAULT '~',
		revision INTEGER NOT NULL DEFAULT 1,
		authored_by TEXT,
		last_edited_by TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)
	mustExec(t, db, `INSERT INTO docs_documents (id, workspace_id, space_id, title, status, is_pinned, created_by, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		documentID, workspaceID, "space-1", "Wire error tracking into metrics middleware Plan", model.DocStatusDraft, false, "actor-1", now)
	mustExec(t, db, `INSERT INTO docs_contents (id, document_id, content, content_text, word_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"content-1", documentID, []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"This plan wires 5xx metrics into middleware."}]}]}`), "This plan wires 5xx metrics into middleware.", 7, now, now)
	mustExec(t, db, `INSERT INTO docs_blocks (id, workspace_id, document_id, type, content, content_text, sort_key, revision, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		blockID, workspaceID, documentID, "paragraph", []byte(`{"type":"paragraph","content":[{"type":"text","text":"This plan wires 5xx metrics into middleware."}]}`), "This plan wires 5xx metrics into middleware.", "a0", 2, now, now)
	docsRepo := repository.NewDocsDocumentRepository(db)
	docsSvc := NewDocsDocumentService(docsRepo, repository.NewDocsSpaceRepository(db), nil, false)
	contentSvc := NewDocsContentService(repository.NewDocsContentRepository(db), docsRepo, nil)
	blockSvc := NewDocsBlockService(repository.NewDocsBlockRepository(db), contentSvc, docsRepo)
	commandService := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	commandService.SetDocsCreateDependencies(docsSvc, nil)
	commandService.SetDocsBlockService(blockSvc)
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"type":"tool_call","tool":"read_document","input":{}}`,
		`{"type":"final","answer":"It is a draft plan about wiring 5xx metrics into middleware."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetInternalCommandService(commandService)

	answer, inlineContext := service.inlineReadOnlyAnswer(ctx, workspaceID, "actor-1", "what in this doc?", model.CommandBarPageContext{EntityType: "document", EntityID: documentID, DisplayTitle: "Wire error tracking into metrics middleware Plan"}, fullCommandBarChatAccess(), nil)
	if answer != "It is a draft plan about wiring 5xx metrics into middleware." {
		t.Fatalf("expected document answer, got %q", answer)
	}
	var toolContext commandBarReadOnlyToolContext
	if err := json.Unmarshal(inlineContext, &toolContext); err != nil {
		t.Fatalf("decode inline context: %v", err)
	}
	if len(toolContext.ToolCalls) != 1 || toolContext.ToolCalls[0].Tool != "read_document" {
		t.Fatalf("expected read_document tool call, got %#v", toolContext.ToolCalls)
	}
	output := string(toolContext.ToolCalls[0].Output)
	if !strings.Contains(output, `"title":"Wire error tracking into metrics middleware Plan"`) || !strings.Contains(output, "5xx metrics") {
		t.Fatalf("expected document content output, got %s", output)
	}
	prompt := fakeLLM.requests[0].Messages[0].Content
	if !strings.Contains(prompt, `"name":"read_document"`) || !strings.Contains(prompt, `"entity_type":"document"`) {
		t.Fatalf("expected read_document tool and document context in prompt, got %s", prompt)
	}
}

func TestDecodeCommandBarReadOnlyToolTurnUnwrapsProviderContent(t *testing.T) {
	raw := `{"content":"{\"type\":\"tool_call\",\"tool\":\"list_tasks\",\"input\":{\"open_only\":true,\"owned_by_actor\":true,\"detail_level\":\"summary\"}}","can_answer":false,"source_doc_ids":[],"confidence":0.95}`

	turn, err := decodeCommandBarReadOnlyToolTurn(raw)
	if err != nil {
		t.Fatalf("decode tool turn: %v", err)
	}
	if turn.Type != "tool_call" || turn.Tool != "list_tasks" {
		t.Fatalf("expected nested tool call, got %#v", turn)
	}
	if !strings.Contains(string(turn.Input), `"owned_by_actor":true`) {
		t.Fatalf("expected nested tool input, got %s", string(turn.Input))
	}
}

func TestCommandBarWorkingContextExtractsGenericResultSetsFromHistory(t *testing.T) {
	priorContext := commandBarReadOnlyToolContext{
		Mode: "read_only_tool_chat",
		ToolCalls: []commandBarReadOnlyToolCall{
			{
				Tool:  "list_workspace_teams",
				Input: json.RawMessage(`{}`),
				Output: json.RawMessage(`{
					"teams":[
						{"id":"team-eng","name":"Engineering","handle":"eng","team_type":"engineering"},
						{"id":"team-ops","name":"Operations","handle":"ops","team_type":"operations"}
					]
				}`),
			},
			{
				Tool:  "list_tasks",
				Input: json.RawMessage(`{"team_id":"team-eng","open_only":true,"limit":5}`),
				Output: json.RawMessage(`{
					"tasks":[
						{"task_id":"task-90","task_key":"USE-90","name":"Add 5xx error counter metric","state_name":"To Do","priority":"urgent","severity":"normal","team_id":"team-eng"},
						{"task_id":"task-235","task_key":"USE-235","name":"Fix critical authorization bypass","state_name":"Backlog","priority":"high","severity":"critical","team_id":"team-eng"}
					],
					"total":2,
					"limit":5,
					"detail_level":"summary"
				}`),
			},
		},
	}
	rawContext, err := json.Marshal(priorContext)
	if err != nil {
		t.Fatalf("marshal prior context: %v", err)
	}
	history := []model.CommandBarMessage{{
		Role:         model.CommandBarMessageRoleAssistant,
		Content:      "Top tasks: USE-90 and USE-235.",
		ProposalJSON: mustJSON(&model.CommandBarProposal{Type: model.CommandBarProposalInlineAnswer, Context: rawContext}),
	}}

	working := commandBarWorkingContextFromHistory(history)
	if working == nil {
		t.Fatal("expected working context")
	}
	if len(working.ResultSets) != 2 {
		t.Fatalf("expected team and task result sets, got %#v", working.ResultSets)
	}
	if working.ActiveScope == nil || working.ActiveScope.SourceTool != "pm.list_tasks" || working.ActiveScope.EntityType != "task" {
		t.Fatalf("expected latest task result set to become active scope, got %#v", working.ActiveScope)
	}
	foundTask := false
	foundTeam := false
	for _, entity := range working.ReferencedEntities {
		if entity.Type == "task" && entity.ID == "task-90" && entity.Key == "USE-90" {
			foundTask = true
		}
		if entity.Type == "workspace_team" && entity.ID == "team-eng" && entity.Title == "Engineering" {
			foundTeam = true
		}
	}
	if !foundTask || !foundTeam {
		t.Fatalf("expected generic task and team refs, got %#v", working.ReferencedEntities)
	}
}

func TestChatTurnReadOnlyToolChatLoadsThreadContext(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-1"
	now := time.Now()
	createCommandBarChatTablesForTest(t, db)
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedUser(t, db, "actor-2", "other@example.com", "Other", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", "actor-1")
	seedWorkspaceMember(t, db, "member-1", workspaceID, "actor-1", "actor@example.com", "Actor", "admin")
	seedWorkspaceMember(t, db, "member-2", workspaceID, "actor-2", "other@example.com", "Other", "member")
	seedWorkflow(t, db, "wf-1", workspaceID, "state-1")
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"team-eng", workspaceID, "Engineering", "eng", "engineering", "feature", now, now)
	for _, task := range []struct {
		ID      string
		Name    string
		OwnerID string
	}{
		{ID: "task-owned", Name: "Owned engineering task", OwnerID: "actor-1"},
		{ID: "task-other", Name: "Other engineering task", OwnerID: "actor-2"},
	} {
		mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, team_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.ID, workspaceID, 1, task.Name, model.PMTaskTypeFeature, "wf-1", "state-1", "team-eng", "medium", "normal", false, false, now, now)
		mustExec(t, db, `INSERT INTO pm_task_owners (task_id, user_id, created_at) VALUES (?, ?, ?)`, task.ID, task.OwnerID, now)
	}

	taskRepo := repository.NewPMTaskRepository(db)
	taskService := NewPMTaskService(
		taskRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		nil,
		nil,
	)
	commandService := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, nil)
	commandService.SetSettingsRepository(repository.NewSettingsRepository(db))
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"read-only task count","confidence":0.98}`,
		`{"type":"tool_call","tool":"list_workspace_teams","input":{}}`,
		`{"type":"tool_call","tool":"list_tasks","input":{"team_id":"team-eng","open_only":true,"limit":10}}`,
		`{"type":"final","answer":"Engineering has 2 open tasks."}`,
		`{"route":"inline_read_only","reason":"read-only follow-up count","confidence":0.98}`,
		`{"type":"tool_call","tool":"list_tasks","input":{"team_id":"team-eng","open_only":true,"owned_by_actor":true,"limit":10}}`,
		`{"type":"final","answer":"Of those Engineering open tasks, 1 is assigned to you."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetChatRepository(repository.NewCommandBarChatRepository(db)).
		SetInternalCommandService(commandService)

	first, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "how many open tasks do we have in engineering",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID},
	})
	if err != nil {
		t.Fatalf("first chat turn: %v", err)
	}
	if first.AssistantMessage.Content != "Engineering has 2 open tasks." {
		t.Fatalf("expected first model answer, got %q", first.AssistantMessage.Content)
	}
	second, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		ThreadID:    &first.Thread.ID,
		Text:        "out of those, how many are pending on me?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID},
	})
	if err != nil {
		t.Fatalf("second chat turn: %v", err)
	}
	if second.AssistantMessage.Content != "Of those Engineering open tasks, 1 is assigned to you." {
		t.Fatalf("expected follow-up model answer, got %q", second.AssistantMessage.Content)
	}
	if len(fakeLLM.requests) != 7 {
		t.Fatalf("expected seven llm turns, got %d", len(fakeLLM.requests))
	}
	secondPrompt := fakeLLM.requests[5].Messages[0].Content
	if !strings.Contains(secondPrompt, "Engineering has 2 open tasks.") ||
		!strings.Contains(secondPrompt, `"referenced_entities"`) ||
		!strings.Contains(secondPrompt, `"task-owned"`) ||
		!strings.Contains(secondPrompt, `"team_id":"team-eng"`) {
		t.Fatalf("expected second-turn llm prompt to include recent chat and structured working context, got %s", secondPrompt)
	}
	if second.Proposal == nil || len(second.Proposal.Context) == 0 {
		t.Fatalf("expected structured inline context on follow-up proposal, got %#v", second.Proposal)
	}
	var toolContext commandBarReadOnlyToolContext
	if err := json.Unmarshal(second.Proposal.Context, &toolContext); err != nil {
		t.Fatalf("decode second context: %v", err)
	}
	if len(toolContext.ToolCalls) != 1 || !strings.Contains(string(toolContext.ToolCalls[0].Input), `"owned_by_actor":true`) || !strings.Contains(string(toolContext.ToolCalls[0].Output), `"total":1`) {
		t.Fatalf("expected actor-owned task tool result, got %#v", toolContext.ToolCalls)
	}
	if toolContext.WorkingContext == nil || len(toolContext.WorkingContext.ReferencedEntities) == 0 || len(toolContext.WorkingContext.ResultSets) == 0 {
		t.Fatalf("expected follow-up proposal to preserve working context, got %#v", toolContext.WorkingContext)
	}
}

func TestChatTurnAmbiguousPriorTargetsAllowsInlineReadOnly(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-ambiguous-inline"
	repo := repository.NewCommandBarChatRepository(db)
	threadID := seedCommandBarThreadWithWorkingContext(t, ctx, repo, workspaceID, "actor-1", []commandBarWorkingEntityRef{
		{Type: "task", ID: "task-90", Key: "USE-90", Title: "Add 5xx error counter metric"},
		{Type: "task", ID: "task-239", Key: "USE-239", Title: "Fix dependency vulnerabilities"},
	})
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"summarize prior set","confidence":0.98}`,
		`{"type":"final","answer":"Here is a summary of both tasks."}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetChatRepository(repo)

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		ThreadID:    &threadID,
		Text:        "summarize all",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalInlineAnswer {
		t.Fatalf("expected inline answer over ambiguous prior set, got %#v", resp.Proposal)
	}
	if got := resp.AssistantMessage.Content; got != "Here is a summary of both tasks." {
		t.Fatalf("expected inline answer, got %q", got)
	}
	if len(fakeLLM.requests) != 2 {
		t.Fatalf("expected classifier and inline answer calls, got %d", len(fakeLLM.requests))
	}
	classifierPrompt := fakeLLM.requests[0].Messages[0].Content
	if !strings.Contains(classifierPrompt, `"prior_target_state":"multiple"`) || !strings.Contains(classifierPrompt, "Target resolution context") {
		t.Fatalf("expected classifier prompt to include ambiguous target context, got %s", classifierPrompt)
	}
}

func TestChatTurnAmbiguousPriorTargetsUsesClassifierClarificationForRun(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-ambiguous-run"
	repo := repository.NewCommandBarChatRepository(db)
	threadID := seedCommandBarThreadWithWorkingContext(t, ctx, repo, workspaceID, "actor-1", []commandBarWorkingEntityRef{
		{Type: "task", ID: "task-90", Key: "USE-90", Title: "Add 5xx error counter metric"},
		{Type: "task", ID: "task-239", Key: "USE-239", Title: "Fix dependency vulnerabilities"},
	})
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"clarification","answer":"Which task should I run Forge on: USE-90 or USE-239?","reason":"multiple prior targets","confidence":0.98}`,
	}}
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, fakeLLM).
		SetChatRepository(repo)

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		ThreadID:    &threadID,
		Text:        "run forge",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalClarification {
		t.Fatalf("expected classifier-driven clarification, got %#v", resp.Proposal)
	}
	if !strings.Contains(resp.AssistantMessage.Content, "USE-90") || !strings.Contains(resp.AssistantMessage.Content, "USE-239") {
		t.Fatalf("expected clarification to preserve classifier answer, got %q", resp.AssistantMessage.Content)
	}
	if len(fakeLLM.requests) != 1 {
		t.Fatalf("expected only classifier call, got %d", len(fakeLLM.requests))
	}
	classifierPrompt := fakeLLM.requests[0].Messages[0].Content
	if !strings.Contains(classifierPrompt, `"prior_target_state":"multiple"`) {
		t.Fatalf("expected classifier prompt to include multiple target state, got %s", classifierPrompt)
	}
}

func TestChatTurnInfersPriorTaskTargetForTaskPlanningCodingReview(t *testing.T) {
	service, db, workspaceID, taskID := setupCommandBarTargetResolutionTest(t)
	ctx := context.Background()
	createCommandBarChatTablesForTest(t, db)

	taskRepo := repository.NewPMTaskRepository(db)
	commandService := NewInternalCommandService(nil, service.agentService.taskService, nil, nil, nil, nil, taskRepo, nil)
	fakeLLM := &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"read-only planning doc lookup","confidence":0.99}`,
		`{"type":"final","answer":"No, USE-90 does not have a planning document associated with it."}`,
		`{"route":"one_shot_command","reason":"durable multi-agent task work","confidence":0.99}`,
	}}
	service.llmProvider = fakeLLM
	service.SetChatRepository(repository.NewCommandBarChatRepository(db)).
		SetInternalCommandService(commandService)

	first, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "USE-90, does it have a planning doc associated?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("first chat turn: %v", err)
	}
	second, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		ThreadID:    &first.Thread.ID,
		Text:        "then we should run task planning, coding and review",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("second chat turn: %v", err)
	}
	if second.Proposal == nil || second.Proposal.Type != model.CommandBarProposalRunPlan || second.Proposal.Plan == nil {
		t.Fatalf("expected run plan proposal, got %#v", second.Proposal)
	}
	steps := second.Proposal.Plan.Steps
	if len(steps) != 3 {
		t.Fatalf("expected Scribe, Forge, Lens steps, got %#v", steps)
	}
	wantAgents := []string{"Scribe", "Forge", "Lens"}
	for i, step := range steps {
		if step.AgentName != wantAgents[i] {
			t.Fatalf("step %d expected %s, got %#v", i, wantAgents[i], step)
		}
		if step.Target.EntityType != "task" || step.Target.EntityID != taskID {
			t.Fatalf("step %d expected inferred task target, got %#v", i, step.Target)
		}
	}
}

func TestListChatThreadsReturnsMostRecentMessages(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, &scriptedCommandBarLLM{responses: []string{
		`{"route":"inline_read_only","reason":"settings guidance","confidence":0.98}`,
		`{"type":"final","answer":"Use Settings."}`,
		`{"route":"inline_read_only","reason":"settings guidance","confidence":0.98}`,
		`{"type":"final","answer":"Use Settings."}`,
	}}).
		SetChatRepository(repository.NewCommandBarChatRepository(db))

	first, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "How do I manage settings?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID},
	})
	if err != nil {
		t.Fatalf("first chat turn: %v", err)
	}
	second, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		ThreadID:    &first.Thread.ID,
		Text:        "How do I manage teams?",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID},
	})
	if err != nil {
		t.Fatalf("second chat turn: %v", err)
	}

	resp, err := service.ListChatThreads(ctx, workspaceID, "actor-1", 1)
	if err != nil {
		t.Fatalf("list chat threads: %v", err)
	}
	if len(resp.Threads) != 1 {
		t.Fatalf("expected one thread, got %d", len(resp.Threads))
	}
	if resp.Threads[0].Thread.ID != first.Thread.ID || second.Thread.ID != first.Thread.ID {
		t.Fatalf("expected same thread to be returned")
	}
	if got := len(resp.Threads[0].Messages); got != 4 {
		t.Fatalf("expected four persisted messages, got %d", got)
	}
	if resp.Threads[0].Messages[0].Content != "How do I manage settings?" {
		t.Fatalf("expected chronological message order, got %#v", resp.Threads[0].Messages)
	}
}

func TestConfirmChatCreateAgentRejectsDifferentActor(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	repo := repository.NewCommandBarChatRepository(db)
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, nil).SetChatRepository(repo)
	messageID := seedCommandBarCreateAgentProposal(t, ctx, repo, workspaceID, "actor-1")

	if _, err := service.ConfirmChatCreateAgent(ctx, workspaceID, "actor-2", messageID, model.ConfirmCommandBarChatProposalRequest{}); err == nil {
		t.Fatalf("expected different actor confirmation to fail")
	}
}

func TestConfirmChatCreateAgentRejectsToolTargetOverrides(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	repo := repository.NewCommandBarChatRepository(db)
	service := NewCommandBarService(&AgentService{}, nil, nil, nil, nil).SetChatRepository(repo)
	messageID := seedCommandBarCreateAgentProposal(t, ctx, repo, workspaceID, "actor-1")

	_, err := service.ConfirmChatCreateAgent(ctx, workspaceID, "actor-1", messageID, model.ConfirmCommandBarChatProposalRequest{
		AllowedTools:   []string{"create_task"},
		AllowedTargets: []string{"workspace"},
	})
	if err == nil || !strings.Contains(err.Error(), "overrides are not supported") {
		t.Fatalf("expected override rejection, got %v", err)
	}
}

func TestChatTurnRunPlanProposalUsesExistingParser(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Atlas', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"22222222-2222-2222-2222-222222222222",
		workspaceID,
		model.AgentPresetTaskPlanner,
		[]byte(`["update_plan"]`),
		[]byte(`[]`),
		[]byte(`["workspace"]`),
	).Error; err != nil {
		t.Fatalf("seed task planner: %v", err)
	}
	service := NewCommandBarService(&AgentService{agentRepo: agentRepo}, nil, nil, nil, nil).
		SetChatRepository(repository.NewCommandBarChatRepository(db))

	resp, err := service.ChatTurn(ctx, workspaceID, "actor-1", model.CommandBarChatTurnRequest{
		Text:        "break down this initiative into tasks",
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: workspaceID, DisplayTitle: "Workspace"},
	})
	if err != nil {
		t.Fatalf("chat turn: %v", err)
	}
	if resp.Proposal == nil || resp.Proposal.Type != model.CommandBarProposalRunPlan || resp.Proposal.Plan == nil {
		t.Fatalf("expected run plan proposal, got %#v", resp.Proposal)
	}
	if got := resp.Proposal.Plan.Steps[0].AgentName; got != "Atlas" {
		t.Fatalf("expected Atlas plan step, got %q", got)
	}
}

func TestCRMResearchUpdatePrefersOneShotCommandAgent(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType:   "crm_contact",
		EntityID:     "contact-1",
		DisplayTitle: "Ada Lovelace",
	}
	candidates := []model.CommandBarAgent{
		{
			ID:             "agent-crm",
			Name:           "CRM Operator",
			PresetKey:      model.AgentPresetCRMOperator,
			AllowedTargets: []string{"crm_contact"},
			AllowedTools:   []string{"list_deals", "list_contacts", "list_buyer_signals"},
		},
		{
			ID:             "agent-command",
			Name:           "Command Agent",
			PresetKey:      model.AgentPresetCommandAgent,
			AllowedTargets: []string{"crm_contact"},
			AllowedTools:   []string{"update_plan", "request_user_input", "request_approval", "web_search_exa", "web_search_brave", "fetch_url", "crawl_url", "list_deals", "list_contacts", "list_buyer_signals", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"},
		},
	}

	resp := parsePreferredOneShotCommandIntent("find info about this contact and update contact and company", pageContext, candidates)
	if resp == nil || resp.Plan == nil || len(resp.Plan.Steps) != 1 {
		t.Fatalf("expected one-shot command plan, got %#v", resp)
	}
	step := resp.Plan.Steps[0]
	if step.AgentID != "agent-command" {
		t.Fatalf("expected Command Agent, got %q", step.AgentID)
	}
	if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
		t.Fatalf("expected one-shot plan kind, got %q", step.PlanKind)
	}
	for _, required := range []string{"web_search_exa", "fetch_url", "list_contacts", "request_approval", "ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"} {
		if !slices.Contains(step.AllowedTools, required) {
			t.Fatalf("expected tool %q in %#v", required, step.AllowedTools)
		}
	}
	if !strings.Contains(step.Instructions, "Research the CRM target") || !strings.Contains(step.Instructions, "protected server-side") {
		t.Fatalf("expected CRM-specific execution brief, got %q", step.Instructions)
	}

	if deterministic := parseIntentDeterministically("find info about this contact and update contact and company", pageContext, commandBarNarrowCandidates(candidates)); deterministic == nil || deterministic.Plan.Steps[0].AgentID != "agent-crm" {
		t.Fatalf("expected deterministic fallback alone to choose CRM Operator, got %#v", deterministic)
	}
}

func TestCRMEnrichmentToolsAreCommandAgentOnlyPresetTools(t *testing.T) {
	presets := ListAgentPresets()
	var commandAgent, crmOperator *model.AgentPresetDefinition
	for idx := range presets {
		switch presets[idx].Key {
		case model.AgentPresetCommandAgent:
			commandAgent = &presets[idx]
		case model.AgentPresetCRMOperator:
			crmOperator = &presets[idx]
		}
	}
	if commandAgent == nil || crmOperator == nil {
		t.Fatalf("missing command or CRM operator preset")
	}
	for _, tool := range []string{"ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"} {
		if !slices.Contains(commandAgent.AllowedTools, tool) {
			t.Fatalf("expected Command Agent to allow %q", tool)
		}
		if slices.Contains(crmOperator.AllowedTools, tool) {
			t.Fatalf("did not expect CRM Operator to allow %q in first slice", tool)
		}
	}
	for _, tool := range []string{"web_search_exa", "fetch_url", "read_document", "get_document_blocks", "write_document_content", "update_document_block", "link_document_to_object"} {
		if !slices.Contains(commandAgent.AllowedTools, tool) {
			t.Fatalf("expected Command Agent to allow document one-shot tool %q", tool)
		}
	}
}

func TestParseFanOutIntentBuildsConcreteTargetPlan(t *testing.T) {
	pageContext := model.CommandBarPageContext{
		EntityType: "epic",
		EntityID:   "epic-1",
		RelatedIDs: map[string][]string{
			"task_ids": {"task-1", "task-2", "task-3"},
		},
	}
	candidates := []model.CommandBarAgent{
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: []string{"task"}},
	}

	resp := parseFanOutIntent("run lens across all child tasks", pageContext, candidates, candidates)
	if resp == nil || resp.Plan == nil {
		t.Fatalf("expected fan-out plan, got %#v", resp)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindFanOut {
		t.Fatalf("expected fan-out plan kind, got %q", resp.Plan.PlanKind)
	}
	if resp.Plan.RunCount != 3 {
		t.Fatalf("expected 3 fan-out runs, got %d", resp.Plan.RunCount)
	}
	for i, step := range resp.Plan.Steps {
		if step.PlanKind != model.CommandBarPlanKindFanOut {
			t.Fatalf("expected step %d fan-out kind, got %q", i, step.PlanKind)
		}
		if step.Target.EntityType != "task" {
			t.Fatalf("expected task target, got %#v", step.Target)
		}
	}
}

func TestParseEpicTaskPipelineIntentBuildsForgeLensDAG(t *testing.T) {
	ctx := context.Background()
	workspaceID := "ws-pipeline"
	epicID := "epic-pipeline"
	dbName := fmt.Sprintf("file:command_bar_pipeline_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			completed BOOLEAN NOT NULL DEFAULT 0,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_task_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source_task_id TEXT NOT NULL,
			target_task_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create pipeline table: %v", err)
		}
	}

	taskOne := model.PMTask{ID: "task-one", WorkspaceID: workspaceID, DisplayID: 1, Name: "Set up API", WorkflowID: "wf", WorkflowStateID: "state", EpicID: &epicID}
	taskTwo := model.PMTask{ID: "task-two", WorkspaceID: workspaceID, DisplayID: 2, Name: "Build UI", WorkflowID: "wf", WorkflowStateID: "state", EpicID: &epicID}
	taskThree := model.PMTask{ID: "task-three", WorkspaceID: workspaceID, DisplayID: 3, Name: "Wire integration", WorkflowID: "wf", WorkflowStateID: "state", EpicID: &epicID}
	completedTask := model.PMTask{ID: "task-done", WorkspaceID: workspaceID, DisplayID: 4, Name: "Already done", WorkflowID: "wf", WorkflowStateID: "state", EpicID: &epicID, Completed: true}
	for _, task := range []model.PMTask{taskOne, taskTwo, taskThree, completedTask} {
		if err := db.Exec(`INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id,
			epic_id, completed, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			task.ID, task.WorkspaceID, task.DisplayID, task.Name, task.WorkflowID, task.WorkflowStateID, task.EpicID, task.Completed,
		).Error; err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}
	if err := db.Exec(`INSERT INTO pm_task_links (
		id, workspace_id, source_task_id, target_task_id, link_type, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"link-one-three", workspaceID, taskOne.ID, taskThree.ID, model.PMTaskLinkTypeBlocks, "user-1",
	).Error; err != nil {
		t.Fatalf("create task link: %v", err)
	}

	service := &CommandBarService{agentService: &AgentService{
		taskRepo:     repository.NewPMTaskRepository(db),
		taskLinkRepo: repository.NewPMTaskLinkRepository(db),
	}}
	agents := []model.Agent{
		{ID: "agent-forge", Name: "Forge", PresetKey: model.AgentPresetCodeBuilder, AllowedTargets: json.RawMessage(`["task"]`)},
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: json.RawMessage(`["task"]`)},
		{ID: "agent-atlas", Name: "Atlas", PresetKey: model.AgentPresetEpicPlanner, AllowedTargets: json.RawMessage(`["epic"]`)},
		{ID: "agent-command", Name: "Command Agent", PresetKey: model.AgentPresetCommandAgent, AllowedTargets: json.RawMessage(`["epic","task"]`)},
	}
	resp := service.parseEpicTaskPipelineIntent(ctx, workspaceID, "We need to complete all tasks in these epics. do the ones that block the others first. fan out for tasks that can be run in parallel. for every task run forge and then lens.", model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     epicID,
		DisplayTitle: "Pipeline epic",
	}, agents)
	if resp == nil || resp.Status != model.CommandBarParseStatusPlan || resp.Plan == nil {
		t.Fatalf("expected pipeline plan, got %#v", resp)
	}
	if resp.Plan.PlanKind != model.CommandBarPlanKindTaskPipeline {
		t.Fatalf("expected task pipeline plan kind, got %q", resp.Plan.PlanKind)
	}
	if resp.Plan.RunCount != 11 {
		t.Fatalf("expected 11 runs for 3 active tasks, got %d", resp.Plan.RunCount)
	}
	steps := resp.Plan.Steps
	if steps[0].StepType != model.CommandBarStepTypeEnsureEpicBranch {
		t.Fatalf("expected first step to ensure epic branch, got %#v", steps[0])
	}
	for i := 1; i < 10; i += 3 {
		if steps[i].AgentName != "Forge" || steps[i+1].AgentName != "Lens" || steps[i+2].StepType != model.CommandBarStepTypeMergeTaskToEpic {
			t.Fatalf("expected Forge then Lens then merge at steps %d/%d/%d, got %s/%s/%s", i, i+1, i+2, steps[i].AgentName, steps[i+1].AgentName, steps[i+2].StepType)
		}
		if got := steps[i+1].DependsOnStepIndexes; len(got) != 1 || got[0] != i {
			t.Fatalf("expected Lens step %d to depend on Forge step %d, got %#v", i+1, i, got)
		}
		if got := steps[i+2].DependsOnStepIndexes; len(got) != 1 || got[0] != i+1 {
			t.Fatalf("expected merge step %d to depend on Lens step %d, got %#v", i+2, i+1, got)
		}
	}
	if got := steps[7].DependsOnStepIndexes; !slices.Contains(got, 3) {
		t.Fatalf("expected blocked task Forge step to depend on blocking task merge step 3, got %#v", got)
	}
	if steps[10].StepType != model.CommandBarStepTypeOpenEpicPullRequest {
		t.Fatalf("expected final step to open epic pull request, got %#v", steps[10])
	}
	if strings.Contains(strings.Join([]string{steps[1].Target.EntityID, steps[4].Target.EntityID, steps[7].Target.EntityID}, ","), completedTask.ID) {
		t.Fatalf("completed task should not be scheduled")
	}

	dependencyPromptResp := service.parseEpicTaskPipelineIntent(ctx, workspaceID, "run these tasks in this epic in parallel if they dont have any dependencies otherwise DAG. some are blocked by the others", model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     epicID,
		DisplayTitle: "Pipeline epic",
	}, agents)
	if dependencyPromptResp == nil || dependencyPromptResp.Plan == nil {
		t.Fatalf("expected dependency-aware prompt to create task pipeline plan, got %#v", dependencyPromptResp)
	}
	if dependencyPromptResp.Plan.PlanKind != model.CommandBarPlanKindTaskPipeline {
		t.Fatalf("expected dependency-aware prompt to avoid flat fan-out, got %q", dependencyPromptResp.Plan.PlanKind)
	}
	if dependencyPromptResp.Plan.RunCount != 11 {
		t.Fatalf("expected dependency-aware prompt to schedule 11 runs, got %d", dependencyPromptResp.Plan.RunCount)
	}
	if got := dependencyPromptResp.Plan.Steps[7].DependsOnStepIndexes; !slices.Contains(got, 3) {
		t.Fatalf("expected dependency-aware prompt to preserve blocks edge, got %#v", got)
	}
	if !slices.ContainsFunc(dependencyPromptResp.Plan.Guardrails, func(g model.CommandBarGuardrail) bool {
		return g.Type == "task_dependency_context" &&
			g.Message == "1 blocking link between tasks was used to order the DAG."
	}) {
		t.Fatalf("expected guardrail naming the single blocking link, got %#v", dependencyPromptResp.Plan.Guardrails)
	}

	// Without any blocking links the guardrail must say so instead of claiming
	// dependencies "were used" — that read as the planner ignoring the user's
	// ordering constraints.
	if err := db.Exec(`DELETE FROM pm_task_links`).Error; err != nil {
		t.Fatalf("delete task links: %v", err)
	}
	noLinksResp := service.parseEpicTaskPipelineIntent(ctx, workspaceID, "run these tasks in this epic in parallel if they dont have any dependencies otherwise DAG. some are blocked by the others", model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     epicID,
		DisplayTitle: "Pipeline epic",
	}, agents)
	if noLinksResp == nil || noLinksResp.Plan == nil {
		t.Fatalf("expected plan without links, got %#v", noLinksResp)
	}
	if !slices.ContainsFunc(noLinksResp.Plan.Guardrails, func(g model.CommandBarGuardrail) bool {
		return g.Type == "task_dependency_context" &&
			g.Message == "No blocking links found between these tasks — all task pipelines run in parallel."
	}) {
		t.Fatalf("expected no-links guardrail message, got %#v", noLinksResp.Plan.Guardrails)
	}
}

func TestParseEpicTaskPipelineIntentMissingAgentsDoesNotFallThrough(t *testing.T) {
	service := &CommandBarService{}
	resp := service.parseEpicTaskPipelineIntent(context.Background(), "ws-1", "run these tasks in this epic in parallel if they dont have any dependencies otherwise DAG", model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     "epic-1",
		DisplayTitle: "Epic 1",
		RelatedIDs:   map[string][]string{"task_ids": {"task-1", "task-2"}},
	}, []model.Agent{
		{ID: "agent-atlas", Name: "Atlas", PresetKey: model.AgentPresetEpicPlanner, AllowedTargets: json.RawMessage(`["epic"]`)},
	})
	if resp == nil || resp.Status != model.CommandBarParseStatusNoMatchingAgent {
		t.Fatalf("expected no-match response for missing Forge/Lens, got %#v", resp)
	}
	if !strings.Contains(resp.Reason, "Forge") || !strings.Contains(resp.Reason, "Lens") {
		t.Fatalf("expected missing agent reason, got %q", resp.Reason)
	}
}

func TestParseEpicTaskPipelineIntentSkipsPreviouslyMergedTask(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	if err := db.Exec(`CREATE TABLE pm_tasks (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		display_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		workflow_id TEXT NOT NULL,
		workflow_state_id TEXT NOT NULL,
		epic_id TEXT,
		completed BOOLEAN NOT NULL DEFAULT 0,
		archived BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create pm_tasks: %v", err)
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	epicID := "22222222-2222-2222-2222-222222222222"
	activeTaskID := "33333333-3333-3333-3333-333333333333"
	mergedTaskID := "44444444-4444-4444-4444-444444444444"
	for _, task := range []model.PMTask{
		{ID: activeTaskID, WorkspaceID: workspaceID, DisplayID: 1, Name: "Still pending", WorkflowID: "wf", WorkflowStateID: "state", EpicID: &epicID},
		{ID: mergedTaskID, WorkspaceID: workspaceID, DisplayID: 2, Name: "Already merged", WorkflowID: "wf", WorkflowStateID: "state", EpicID: &epicID},
	} {
		if err := db.Exec(`INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id,
			epic_id, completed, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			task.ID, task.WorkspaceID, task.DisplayID, task.Name, task.WorkflowID, task.WorkflowStateID, task.EpicID,
		).Error; err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}
	if err := repository.NewAgentRunRepository(db).Create(ctx, &model.AgentRun{
		ID:             "55555555-5555-5555-5555-555555555555",
		WorkspaceID:    workspaceID,
		AgentID:        "agent-command",
		TargetType:     "task",
		TargetID:       mergedTaskID,
		RuntimeKind:    "internal",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusCompleted,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{"message":"Task branch merged into epic branch."}`),
	}); err != nil {
		t.Fatalf("create prior merge run: %v", err)
	}

	service := &CommandBarService{agentService: &AgentService{
		taskRepo: repository.NewPMTaskRepository(db),
		runRepo:  repository.NewAgentRunRepository(db),
	}}
	agents := []model.Agent{
		{ID: "agent-forge", Name: "Forge", PresetKey: model.AgentPresetCodeBuilder, AllowedTargets: json.RawMessage(`["task"]`)},
		{ID: "agent-lens", Name: "Lens", PresetKey: model.AgentPresetReviewAgent, AllowedTargets: json.RawMessage(`["task"]`)},
		{ID: "agent-command", Name: "Command Agent", PresetKey: model.AgentPresetCommandAgent, AllowedTargets: json.RawMessage(`["epic","task"]`)},
	}
	resp := service.parseEpicTaskPipelineIntent(ctx, workspaceID, "run all tasks in this epic in parallel with forge then lens", model.CommandBarPageContext{
		EntityType:   "epic",
		EntityID:     epicID,
		DisplayTitle: "Pipeline epic",
	}, agents)
	if resp == nil || resp.Plan == nil {
		t.Fatalf("expected task pipeline plan, got %#v", resp)
	}
	if resp.Plan.RunCount != 5 {
		t.Fatalf("expected only active task plus epic setup/final PR, got %d runs", resp.Plan.RunCount)
	}
	for _, step := range resp.Plan.Steps {
		if step.Target.EntityID == mergedTaskID {
			t.Fatalf("previously merged task should not be scheduled, got step %#v", step)
		}
	}
	if !slices.ContainsFunc(resp.Plan.Guardrails, func(g model.CommandBarGuardrail) bool {
		return g.Type == "task_pipeline_skipped_tasks" &&
			strings.Contains(g.Message, "Already merged") &&
			strings.Contains(g.Message, "merged into the epic branch")
	}) {
		t.Fatalf("expected skipped-task guardrail for prior merge, got %#v", resp.Plan.Guardrails)
	}
}

func TestValidateDispatchStepsRequiresOneShotKindForCommandAgent(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	service := &CommandBarService{agentService: &AgentService{agentRepo: agentRepo}}
	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	agentID := "22222222-2222-2222-2222-222222222222"

	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Command Agent', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID,
		workspaceID,
		model.AgentPresetCommandAgent,
		[]byte(`["read_document","write_document_content"]`),
		[]byte(`[]`),
		[]byte(`["document"]`),
	).Error; err != nil {
		t.Fatalf("seed command agent: %v", err)
	}

	target := model.CommandBarPageContext{EntityType: "document", EntityID: "33333333-3333-3333-3333-333333333333"}
	err := service.validateDispatchSteps(ctx, workspaceID, []model.CommandBarPlanStep{{
		AgentID:      agentID,
		Target:       target,
		Instructions: "Update this doc.",
		AllowedTools: []string{"read_document"},
	}})
	if err == nil || !strings.Contains(err.Error(), "must be dispatched as a one-shot command") {
		t.Fatalf("expected one-shot kind validation error, got %v", err)
	}

	err = service.validateDispatchSteps(ctx, workspaceID, []model.CommandBarPlanStep{{
		AgentID:      agentID,
		PlanKind:     model.CommandBarPlanKindOneShotCommand,
		Target:       target,
		Instructions: "Update this doc.",
		AllowedTools: []string{"read_document"},
	}})
	if err != nil {
		t.Fatalf("expected one-shot command step to validate: %v", err)
	}
}

func TestAdvanceCommandBarPlanMarksFailedRunPlanFailed(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	runID := "33333333-3333-3333-3333-333333333333"
	agentID := "44444444-4444-4444-4444-444444444444"
	targetID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "task", EntityID: targetID, DisplayTitle: "Task 1"}
	steps := []model.CommandBarPlanStep{
		{AgentID: agentID, AgentName: "Forge", Target: pageContext, Instructions: "Build it."},
		{AgentID: agentID, AgentName: "Lens", Target: pageContext, Instructions: "Review it."},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge and lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runID})
	plan.RunIDsByStep = runIDs
	plan.CurrentStepIndex = 0
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	trigger, err := buildCommandBarTriggerContext("run forge and lens", pageContext, steps, 0, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	errMsg := "forge failed"
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "task",
		TargetID:       targetID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusFailed,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
		ErrorMessage:   &errMsg,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	nextRun, err := service.AdvanceCommandBarPlanAfterRun(ctx, runID)
	if err != nil {
		t.Fatalf("advance failed run: %v", err)
	}
	if nextRun != nil {
		t.Fatalf("expected no next run after failed step, got %#v", nextRun)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusFailed {
		t.Fatalf("expected failed plan status, got %q", updated.Status)
	}
	if updated.ErrorMessage == nil || *updated.ErrorMessage != errMsg {
		t.Fatalf("expected plan error %q, got %#v", errMsg, updated.ErrorMessage)
	}
}

func TestAdvanceFanOutCommandBarPlanWaitsForAllRuns(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	agentID := "33333333-3333-3333-3333-333333333333"
	runOneID := "44444444-4444-4444-4444-444444444444"
	runTwoID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      agentID,
			AgentName:    "Lens",
			PlanKind:     model.CommandBarPlanKindFanOut,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions: "Review task 1.",
		},
		{
			AgentID:      agentID,
			AgentName:    "Lens",
			PlanKind:     model.CommandBarPlanKindFanOut,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-2", DisplayTitle: "Task 2"},
			Instructions: "Review task 2.",
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run lens across all child tasks", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runOneID, 1: runTwoID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	trigger, err := buildCommandBarTriggerContext("run lens across all child tasks", pageContext, steps, 1, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runOneID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "task",
		TargetID:       "task-1",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create first run: %v", err)
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runTwoID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "task",
		TargetID:       "task-2",
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusCompleted,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create second run: %v", err)
	}

	nextRun, err := service.AdvanceCommandBarPlanAfterRun(ctx, runTwoID)
	if err != nil {
		t.Fatalf("advance fan-out run: %v", err)
	}
	if nextRun != nil {
		t.Fatalf("expected no sequential next run for fan-out, got %#v", nextRun)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusRunning {
		t.Fatalf("expected plan to wait for active fan-out run, got %q", updated.Status)
	}

	trigger, err = buildCommandBarTriggerContext("run lens across all child tasks", pageContext, steps, 0, planID)
	if err != nil {
		t.Fatalf("build second trigger: %v", err)
	}
	input, _ = json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err := db.Model(&model.AgentRun{}).
		Where("id = ?", runOneID).
		Updates(map[string]any{"status": model.AgentRunStatusCompleted, "input": input}).Error; err != nil {
		t.Fatalf("complete first run: %v", err)
	}
	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, runOneID); err != nil {
		t.Fatalf("advance final fan-out run: %v", err)
	}
	updated, err = planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get completed plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected completed plan after all fan-out runs complete, got %q", updated.Status)
	}
}

func TestAdvanceTaskPipelinePlanSchedulesFallbackWithoutRunEngine(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	agentID := "33333333-3333-3333-3333-333333333333"
	runOneID := "44444444-4444-4444-4444-444444444444"
	runTwoID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      agentID,
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions: "Build task 1.",
		},
		{
			AgentID:              agentID,
			AgentName:            "Lens",
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions:         "Review task 1.",
			DependsOnStepIndexes: []int{0},
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge then lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runOneID, 1: runTwoID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	for index, runID := range []string{runOneID, runTwoID} {
		trigger, err := buildCommandBarTriggerContext("run forge then lens", pageContext, steps, index, planID)
		if err != nil {
			t.Fatalf("build trigger %d: %v", index, err)
		}
		input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
		if err := runRepo.Create(ctx, &model.AgentRun{
			ID:             runID,
			WorkspaceID:    workspaceID,
			AgentID:        agentID,
			TargetType:     "task",
			TargetID:       "task-1",
			RuntimeKind:    "native_sdk",
			InvocationMode: model.InvocationModeAutonomous,
			ApprovalState:  "not_required",
			PauseReason:    model.AgentRunPauseReasonNone,
			Status:         model.AgentRunStatusCompleted,
			Input:          input,
			OutputSummary:  json.RawMessage("{}"),
		}); err != nil {
			t.Fatalf("create run %d: %v", index, err)
		}
	}

	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, runTwoID); err != nil {
		t.Fatalf("advance task pipeline run: %v", err)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected direct fallback to complete task pipeline plan, got %q", updated.Status)
	}
}

func TestAdvanceTaskPipelinePlanSignalsOnlyWithRunEngine(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	temporalClient := &commandBarSignalTemporalClient{}
	service := &AgentService{
		runRepo:            runRepo,
		commandBarPlanRepo: planRepo,
		runEngine:          temporalapp.NewRunEngine(temporalClient, "test"),
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	forgeAgentID := "33333333-3333-3333-3333-333333333333"
	lensAgentID := "44444444-4444-4444-4444-444444444444"
	forgeRunID := "55555555-5555-5555-5555-555555555555"
	taskID := "66666666-6666-6666-6666-666666666666"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      forgeAgentID,
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: taskID, DisplayTitle: "Task 1"},
			Instructions: "Build task 1.",
		},
		{
			AgentID:              lensAgentID,
			AgentName:            "Lens",
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: taskID, DisplayTitle: "Task 1"},
			Instructions:         "Review task 1.",
			DependsOnStepIndexes: []int{0},
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge then lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: forgeRunID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	trigger, err := buildCommandBarTriggerContext("run forge then lens", pageContext, steps, 0, planID)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             forgeRunID,
		WorkspaceID:    workspaceID,
		AgentID:        forgeAgentID,
		TargetType:     "task",
		TargetID:       taskID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusCompleted,
		Input:          input,
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create forge run: %v", err)
	}

	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, forgeRunID); err != nil {
		t.Fatalf("advance task pipeline run: %v", err)
	}
	if temporalClient.workflowID != temporalapp.WorkflowIDForCommandBarPlan(planID) {
		t.Fatalf("expected signal to command bar workflow, got %q", temporalClient.workflowID)
	}
	if temporalClient.signalName != temporalapp.WorkflowSignalCommandBarRun {
		t.Fatalf("expected command bar run signal, got %q", temporalClient.signalName)
	}
	if temporalClient.signaledRunID != forgeRunID {
		t.Fatalf("expected signal run id %q, got %q", forgeRunID, temporalClient.signaledRunID)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusRunning {
		t.Fatalf("expected signal-only path to leave plan running, got %q", updated.Status)
	}
	if got := decodeCommandBarPlanRunIDs(updated.RunIDsByStep); strings.TrimSpace(got[1]) != "" {
		t.Fatalf("expected signal-only path not to start Lens directly, got run ids %#v", got)
	}
}

func TestAdvanceTaskPipelinePlanFallsBackWhenSignalFails(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	temporalClient := &commandBarSignalTemporalClient{err: fmt.Errorf("workflow not found")}
	service := &AgentService{
		runRepo:            runRepo,
		commandBarPlanRepo: planRepo,
		runEngine:          temporalapp.NewRunEngine(temporalClient, "test"),
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	agentID := "33333333-3333-3333-3333-333333333333"
	runOneID := "44444444-4444-4444-4444-444444444444"
	runTwoID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic 1"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      agentID,
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions: "Build task 1.",
		},
		{
			AgentID:              agentID,
			AgentName:            "Lens",
			PlanKind:             model.CommandBarPlanKindTaskPipeline,
			Target:               model.CommandBarPageContext{EntityType: "task", EntityID: "task-1", DisplayTitle: "Task 1"},
			Instructions:         "Review task 1.",
			DependsOnStepIndexes: []int{0},
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run forge then lens", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runOneID, 1: runTwoID})
	plan.RunIDsByStep = runIDs
	plan.RunCount = 2
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	for index, runID := range []string{runOneID, runTwoID} {
		trigger, err := buildCommandBarTriggerContext("run forge then lens", pageContext, steps, index, planID)
		if err != nil {
			t.Fatalf("build trigger %d: %v", index, err)
		}
		input, _ := json.Marshal(model.AgentRunInputPayload{Trigger: trigger})
		if err := runRepo.Create(ctx, &model.AgentRun{
			ID:             runID,
			WorkspaceID:    workspaceID,
			AgentID:        agentID,
			TargetType:     "task",
			TargetID:       "task-1",
			RuntimeKind:    "native_sdk",
			InvocationMode: model.InvocationModeAutonomous,
			ApprovalState:  "not_required",
			PauseReason:    model.AgentRunPauseReasonNone,
			Status:         model.AgentRunStatusCompleted,
			Input:          input,
			OutputSummary:  json.RawMessage("{}"),
		}); err != nil {
			t.Fatalf("create run %d: %v", index, err)
		}
	}

	if _, err := service.AdvanceCommandBarPlanAfterRun(ctx, runTwoID); err != nil {
		t.Fatalf("advance task pipeline run: %v", err)
	}
	if temporalClient.signaledRunID != runTwoID {
		t.Fatalf("expected signal attempt for run %q, got %q", runTwoID, temporalClient.signaledRunID)
	}
	updated, err := planRepo.GetByID(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if updated.Status != model.CommandBarPlanStatusCompleted {
		t.Fatalf("expected fallback to complete task pipeline plan, got %q", updated.Status)
	}
}

func TestDecodeCommandBarPlanRunIDsUsesStringKeys(t *testing.T) {
	raw := json.RawMessage(`{"0":"run-0","2":"run-2","bad":"ignored"}`)
	got := decodeCommandBarPlanRunIDs(raw)
	if got[0] != "run-0" || got[2] != "run-2" {
		t.Fatalf("unexpected decoded run ids: %#v", got)
	}
	if _, ok := got[1]; ok {
		t.Fatalf("did not expect step 1 in %#v", got)
	}
}

func seedCommandBarCreateAgentProposal(t *testing.T, ctx context.Context, repo *repository.CommandBarChatRepository, workspaceID, actorID string) string {
	t.Helper()
	thread := &model.CommandBarThread{
		ID:          "33333333-3333-3333-3333-333333333333",
		WorkspaceID: workspaceID,
		ActorID:     &actorID,
		Title:       "Create agent",
		Status:      model.CommandBarThreadStatusOpen,
	}
	if err := repo.CreateThread(ctx, thread); err != nil {
		t.Fatalf("create chat thread: %v", err)
	}
	proposal := model.CommandBarProposal{
		Type: model.CommandBarProposalCreateAgent,
		Draft: &model.CustomAgentDraft{
			Name:                  "Doc Reviewer",
			Role:                  "Review docs",
			RuntimeKind:           "native_sdk",
			AllowedTools:          []string{"read_document"},
			AllowedTargets:        []string{"document"},
			ApprovalMode:          "always",
			DefaultInvocationMode: "interactive",
			MaxConcurrentRuns:     1,
			SystemPrompt:          "Review docs.",
		},
	}
	rawProposal, err := json.Marshal(proposal)
	if err != nil {
		t.Fatalf("marshal proposal: %v", err)
	}
	messageID := "44444444-4444-4444-4444-444444444444"
	message := &model.CommandBarMessage{
		ID:           messageID,
		ThreadID:     thread.ID,
		WorkspaceID:  workspaceID,
		ActorID:      &actorID,
		Role:         model.CommandBarMessageRoleAssistant,
		Content:      "Review the draft before approving.",
		ProposalJSON: rawProposal,
	}
	if err := repo.CreateMessage(ctx, message); err != nil {
		t.Fatalf("create proposal message: %v", err)
	}
	return messageID
}

func TestCommandBarPlanOwnedByActor(t *testing.T) {
	actorID := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"
	if !commandBarPlanOwnedByActor(&model.CommandBarPlanRecord{ActorID: &actorID}, actorID) {
		t.Fatal("expected owner to access plan")
	}
	if commandBarPlanOwnedByActor(&model.CommandBarPlanRecord{ActorID: &otherID}, actorID) {
		t.Fatal("expected other actor to be denied")
	}
}

func TestCommandBarUnmetIntentSummaryRedactsPromptByDefault(t *testing.T) {
	pageContext, _ := json.Marshal(model.CommandBarPageContext{EntityType: "document", EntityID: "doc-1"})
	candidates, _ := json.Marshal([]model.CommandBarAgent{{ID: "agent-1", Name: "Command Agent"}})
	intent := model.CommandBarUnmetIntent{
		ID:              "intent-1",
		WorkspaceID:     "workspace-1",
		Prompt:          "check the web and update stale doc sections with sensitive customer details",
		PageContext:     pageContext,
		CandidateAgents: candidates,
		Reason:          "No matching agent.",
		Status:          "open",
	}

	summary := commandBarUnmetIntentSummary(intent, false)
	if summary.Prompt != "" || !summary.PromptRedacted {
		t.Fatalf("expected redacted prompt, got prompt=%q redacted=%v", summary.Prompt, summary.PromptRedacted)
	}
	if summary.PromptPreview == "" || !strings.Contains(summary.PromptPreview, "check the web") {
		t.Fatalf("expected useful prompt preview, got %q", summary.PromptPreview)
	}
	if summary.PageContext.EntityType != "document" || len(summary.CandidateAgents) != 1 {
		t.Fatalf("expected decoded context and candidates, got %#v", summary)
	}
}

func TestValidatePromotedAgentTargetsRejectsOutsideSourceAllowlist(t *testing.T) {
	sourceAgent := &model.Agent{
		Name:           "Command Agent",
		AllowedTargets: json.RawMessage(`["document"]`),
	}
	if err := validatePromotedAgentTargets([]string{"document"}, sourceAgent); err != nil {
		t.Fatalf("expected document target to be accepted: %v", err)
	}
	if err := validatePromotedAgentTargets([]string{"crm_deal"}, sourceAgent); err == nil {
		t.Fatal("expected crm_deal target to be rejected")
	}
}

func setupCommandBarTargetResolutionTest(t *testing.T) (*CommandBarService, *gorm.DB, string, string) {
	t.Helper()
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-target-resolution"
	taskID := "task-use-90"
	now := time.Now()
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", "actor-1")
	mustExec(t, db, `UPDATE workspaces SET workspace_key = ? WHERE id = ?`, "USE", workspaceID)
	seedWorkspaceMember(t, db, "member-1", workspaceID, "actor-1", "actor@example.com", "Actor", "admin")
	seedWorkflow(t, db, "wf-1", workspaceID, "state-1")
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, workspaceID, 90, "Add 5xx error counter metric", model.PMTaskTypeFeature, "wf-1", "state-1", "medium", "normal", false, false, now, now)
	mustExec(t, db, `CREATE TABLE agents (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		is_system BOOLEAN NOT NULL DEFAULT 0,
		name TEXT NOT NULL,
		preset_key TEXT,
		preset_version_key TEXT,
		source_preset_key TEXT,
		source_preset_version_key TEXT,
		source_template_id TEXT,
		source_template_key TEXT,
		template_key TEXT,
		template_instance_id TEXT,
		template_version INTEGER,
		active_version_id TEXT,
		role TEXT,
		status TEXT NOT NULL DEFAULT 'idle',
		runtime_kind TEXT NOT NULL DEFAULT 'opencode',
		skills BLOB NOT NULL DEFAULT '[]',
		trigger_mode TEXT NOT NULL DEFAULT 'manual',
		provider TEXT,
		model TEXT,
		execution_config BLOB NOT NULL DEFAULT '{}',
		system_prompt TEXT,
		instruction_template_version TEXT NOT NULL DEFAULT '',
		planning_notes TEXT,
		monthly_token_budget INTEGER,
		tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
		active_task_id TEXT,
		team_id TEXT,
		allowed_tools BLOB NOT NULL DEFAULT '[]',
		allowed_commands BLOB NOT NULL DEFAULT '[]',
		allowed_targets BLOB NOT NULL DEFAULT '[]',
		approval_mode TEXT NOT NULL DEFAULT 'preset_default',
		max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
		default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE agent_team_access (
		agent_id TEXT NOT NULL,
		team_id TEXT NOT NULL,
		created_at DATETIME,
		PRIMARY KEY (agent_id, team_id)
	)`)

	agentRepo := repository.NewAgentRepository(db)
	for _, agent := range []model.Agent{
		{
			ID:                    "agent-scribe",
			WorkspaceID:           workspaceID,
			IsSystem:              true,
			Name:                  "Scribe",
			PresetKey:             model.AgentPresetTaskPlanner,
			Role:                  "Task Planner",
			Status:                "idle",
			RuntimeKind:           "native_sdk",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			ExecutionConfig:       model.JSONBlob(`{}`),
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`["task","epic","workspace"]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeInteractive,
		},
		{
			ID:                    "agent-forge",
			WorkspaceID:           workspaceID,
			IsSystem:              true,
			Name:                  "Forge",
			PresetKey:             model.AgentPresetCodeBuilder,
			Role:                  "Code Builder",
			Status:                "idle",
			RuntimeKind:           "codex",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			ExecutionConfig:       model.JSONBlob(`{}`),
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`["task","workspace"]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeAutonomous,
		},
		{
			ID:                    "agent-lens",
			WorkspaceID:           workspaceID,
			IsSystem:              true,
			Name:                  "Lens",
			PresetKey:             model.AgentPresetReviewAgent,
			Role:                  "Review Agent",
			Status:                "idle",
			RuntimeKind:           "codex",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			ExecutionConfig:       model.JSONBlob(`{}`),
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`["task","workspace"]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeAutonomous,
		},
	} {
		agent := agent
		if err := agentRepo.Create(ctx, &agent); err != nil {
			t.Fatalf("create agent %s: %v", agent.ID, err)
		}
	}

	taskRepo := repository.NewPMTaskRepository(db)
	taskService := NewPMTaskService(
		taskRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		nil,
		nil,
	)
	service := NewCommandBarService(&AgentService{
		agentRepo:   agentRepo,
		taskService: taskService,
	}, nil, nil, nil, nil)
	return service, db, workspaceID, taskID
}

func setupCommandBarDocumentTargetResolutionTest(t *testing.T) (*CommandBarService, *gorm.DB, string, string) {
	t.Helper()
	db := setupCommandBarPlanTestDB(t)
	workspaceID := "11111111-1111-1111-1111-111111111111"
	documentID := "40ab3137-86ea-4eed-b470-e6765abb52d5"
	now := time.Now()
	mustExec(t, db, `CREATE TABLE docs_documents (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		collection_id TEXT,
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		team_id TEXT,
		owner_id TEXT,
		excerpt TEXT,
		is_pinned BOOLEAN NOT NULL DEFAULT 0,
		published_at DATETIME,
		next_review_at DATETIME,
		created_by TEXT NOT NULL,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)
	mustExec(t, db, `INSERT INTO docs_documents (id, workspace_id, space_id, title, status, is_pinned, created_by, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		documentID, workspaceID, "space-1", "Wire error tracking into metrics middleware Plan", model.DocStatusDraft, false, "actor-1", now)
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Command Agent', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"33333333-3333-3333-3333-333333333333",
		workspaceID,
		model.AgentPresetCommandAgent,
		[]byte(`["update_plan","request_user_input","read_document","get_document_blocks","web_search_exa","fetch_url","publish_document_change_proposal"]`),
		[]byte(`[]`),
		[]byte(`["workspace","document"]`),
	).Error; err != nil {
		t.Fatalf("seed command agent: %v", err)
	}
	agentRepo := repository.NewAgentRepository(db)
	docRepo := repository.NewDocsDocumentRepository(db)
	docsSvc := NewDocsDocumentService(docRepo, repository.NewDocsSpaceRepository(db), nil, false)
	service := NewCommandBarService(&AgentService{agentRepo: agentRepo, docsDocumentRepo: docRepo}, nil, nil, nil, nil).
		SetReadOnlyDataServices(docsSvc, nil, nil, nil)
	return service, db, workspaceID, documentID
}

func setupCommandBarRepositoryTargetResolutionTest(t *testing.T) (*CommandBarService, *gorm.DB, string, string) {
	t.Helper()
	db := setupCommandBarPlanTestDB(t)
	workspaceID := "11111111-1111-1111-1111-111111111111"
	repoID := "44444444-4444-4444-4444-444444444444"
	now := time.Now()
	mustExec(t, db, `CREATE TABLE git_repositories (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		integration_id TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT 'github',
		base_url TEXT,
		external_id TEXT NOT NULL DEFAULT '',
		full_name TEXT NOT NULL,
		default_branch TEXT NOT NULL DEFAULT 'main',
		permissions TEXT NOT NULL DEFAULT '{}',
		private BOOLEAN NOT NULL DEFAULT 1,
		archived BOOLEAN NOT NULL DEFAULT 0,
		selected BOOLEAN NOT NULL DEFAULT 1,
		active BOOLEAN NOT NULL DEFAULT 1,
		deleted_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE agent_team_access (
		agent_id TEXT NOT NULL,
		team_id TEXT NOT NULL,
		created_at DATETIME,
		PRIMARY KEY (agent_id, team_id)
	)`)
	mustExec(t, db, `INSERT INTO git_repositories (id, workspace_id, integration_id, provider, external_id, full_name, default_branch, permissions, private, archived, selected, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		repoID, workspaceID, "integration-1", "github", "1001", "helpin/usermaven", "main", []byte(`{}`), true, false, true, true, now, now)
	mustExec(t, db, `INSERT INTO git_repositories (id, workspace_id, integration_id, provider, external_id, full_name, default_branch, permissions, private, archived, selected, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"55555555-5555-5555-5555-555555555555", workspaceID, "integration-1", "github", "1002", "usermaven/usermven", "main", []byte(`{}`), true, false, true, true, now, now)
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, status, runtime_kind,
		allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, 1, 'Command Agent', ?, 'idle', 'native_sdk', ?, ?, ?, 'never', 1, 'interactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"33333333-3333-3333-3333-333333333333",
		workspaceID,
		model.AgentPresetCommandAgent,
		[]byte(`["update_plan","request_user_input","list_repositories","list_commits","read_file","read_file_range","read_files","list_directory","search_files","ripgrep","grep","list_symbols","list_spaces","list_collections","create_document"]`),
		[]byte(`[]`),
		[]byte(`["workspace","repository"]`),
	).Error; err != nil {
		t.Fatalf("seed command agent: %v", err)
	}
	agentRepo := repository.NewAgentRepository(db)
	gitSvc := NewGitService(nil, repository.NewGitRepositoryRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", "")
	service := NewCommandBarService(&AgentService{agentRepo: agentRepo, gitService: gitSvc}, nil, nil, nil, nil)
	return service, db, workspaceID, repoID
}

// The retry handler pre-fetches the plan for per-step authorization; that
// lookup must not be owner-gated or cross-actor retries from the epic page
// die with "not found" before the team-actionable service method runs.
func TestGetWorkspacePlanBypassesOwnerGate(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &CommandBarService{
		planRepo:     planRepo,
		agentService: &AgentService{runRepo: repository.NewAgentRunRepository(db)},
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "66666666-6666-6666-6666-666666666666"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: "epic-1", DisplayTitle: "Epic"}
	steps := []model.CommandBarPlanStep{{AgentID: "agent-1", AgentName: "Forge", Target: pageContext, Instructions: "Build."}}
	plan, err := newCommandBarPlanRecord(workspaceID, "actor-owner", planID, "run all tasks", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if _, err := service.GetPlan(ctx, workspaceID, "actor-other", planID); err == nil {
		t.Fatalf("expected owner-gated GetPlan to hide another actor's plan")
	}
	detail, err := service.GetWorkspacePlan(ctx, workspaceID, planID)
	if err != nil {
		t.Fatalf("GetWorkspacePlan: %v", err)
	}
	if detail.Plan.ID != planID {
		t.Fatalf("expected plan %q, got %q", planID, detail.Plan.ID)
	}
}

// A plan can be left status "running" with all child runs failed/cancelled —
// a zombie that resume cannot revive. Retry must accept it (rejecting only
// while runs are genuinely active).
func TestRetryPlanFromStepAcceptsRunningPlanWithNoActiveRuns(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	runRepo := repository.NewAgentRunRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)
	service := &CommandBarService{
		planRepo:     planRepo,
		agentService: &AgentService{runRepo: runRepo, commandBarPlanRepo: planRepo},
	}

	ctx := context.Background()
	workspaceID := "11111111-1111-1111-1111-111111111111"
	planID := "22222222-2222-2222-2222-222222222222"
	runID := "33333333-3333-3333-3333-333333333333"
	agentID := "44444444-4444-4444-4444-444444444444"
	targetID := "55555555-5555-5555-5555-555555555555"
	pageContext := model.CommandBarPageContext{EntityType: "epic", EntityID: targetID, DisplayTitle: "Epic"}
	steps := []model.CommandBarPlanStep{
		{
			AgentID:      agentID,
			AgentName:    "Forge",
			PlanKind:     model.CommandBarPlanKindTaskPipeline,
			Target:       pageContext,
			Instructions: "Build it.",
		},
	}
	plan, err := newCommandBarPlanRecord(workspaceID, "", planID, "run all tasks", pageContext, steps)
	if err != nil {
		t.Fatalf("build plan record: %v", err)
	}
	runIDs, _ := json.Marshal(map[int]string{0: runID})
	plan.RunIDsByStep = runIDs
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.Status != model.CommandBarPlanStatusRunning {
		t.Fatalf("expected plan to start running, got %q", plan.Status)
	}
	if err := runRepo.Create(ctx, &model.AgentRun{
		ID:             runID,
		WorkspaceID:    workspaceID,
		AgentID:        agentID,
		TargetType:     "epic",
		TargetID:       targetID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeAutonomous,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusCancelled,
		Input:          json.RawMessage("{}"),
		OutputSummary:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Zombie (running plan, cancelled run): must pass the running gate and
	// only fail later on the missing temporal engine in this test harness.
	_, err = service.RetryPlanFromStep(ctx, workspaceID, "actor-2", planID, model.CommandBarRetryPlanRequest{StepIndex: 0})
	if err == nil || !strings.Contains(err.Error(), "orchestration is not configured") {
		t.Fatalf("expected zombie plan to pass the running gate, got %v", err)
	}

	// Genuinely active run: retry must be rejected.
	if err := db.Exec(`UPDATE agent_runs SET status = 'running' WHERE id = ?`, runID).Error; err != nil {
		t.Fatalf("activate run: %v", err)
	}
	_, err = service.RetryPlanFromStep(ctx, workspaceID, "actor-2", planID, model.CommandBarRetryPlanRequest{StepIndex: 0})
	if err == nil || !strings.Contains(err.Error(), "active runs") {
		t.Fatalf("expected active-run rejection, got %v", err)
	}
}

func setupCommandBarPlanTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:command_bar_plan_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			external_runtime TEXT,
			external_runtime_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			agent_version_id TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT NOT NULL DEFAULT '{}',
			output_summary TEXT NOT NULL DEFAULT '{}',
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_plans (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			status TEXT NOT NULL DEFAULT 'running',
			prompt TEXT NOT NULL,
			page_context TEXT NOT NULL DEFAULT '{}',
			steps TEXT NOT NULL DEFAULT '[]',
			run_ids_by_step TEXT NOT NULL DEFAULT '{}',
			current_step_index INTEGER NOT NULL DEFAULT 0,
			run_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			cancelled_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_threads (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_messages (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			page_context TEXT,
			proposal_json TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT,
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
		active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT '{}',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create command bar test table: %v", err)
		}
	}
	return db
}

func createCommandBarChatTablesForTest(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE command_bar_threads (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE command_bar_messages (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			page_context TEXT,
			proposal_json TEXT,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create command bar chat test table: %v", err)
		}
	}
}

func seedCommandBarThreadWithWorkingContext(t *testing.T, ctx context.Context, repo *repository.CommandBarChatRepository, workspaceID, actorID string, entities []commandBarWorkingEntityRef) string {
	t.Helper()
	thread := &model.CommandBarThread{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		ActorID:     &actorID,
		Title:       "Prior context",
		Status:      model.CommandBarThreadStatusOpen,
	}
	if err := repo.CreateThread(ctx, thread); err != nil {
		t.Fatalf("create command bar thread: %v", err)
	}
	rawContext, err := json.Marshal(commandBarReadOnlyToolContext{
		Mode: "read_only_tool_chat",
		WorkingContext: &commandBarWorkingContext{
			ReferencedEntities: entities,
		},
	})
	if err != nil {
		t.Fatalf("marshal working context: %v", err)
	}
	proposalJSON := mustJSON(&model.CommandBarProposal{
		Type:    model.CommandBarProposalInlineAnswer,
		Context: rawContext,
	})
	if err := repo.CreateMessage(ctx, &model.CommandBarMessage{
		ID:           uuid.NewString(),
		ThreadID:     thread.ID,
		WorkspaceID:  workspaceID,
		ActorID:      &actorID,
		Role:         model.CommandBarMessageRoleAssistant,
		Content:      "Prior result set.",
		ProposalJSON: proposalJSON,
	}); err != nil {
		t.Fatalf("create command bar message: %v", err)
	}
	return thread.ID
}
