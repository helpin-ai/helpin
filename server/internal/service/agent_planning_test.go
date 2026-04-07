package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestValidateSpecClarificationsRequiresAnswersAndReasons(t *testing.T) {
	current := []model.SpecClarificationItem{
		{ID: "open_question_1", Kind: model.SpecClarificationKindOpenQuestion, Prompt: "Who gets access?"},
		{ID: "assumption_1", Kind: model.SpecClarificationKindAssumption, Prompt: "Launch for paid plans only"},
	}

	_, _, err := validateSpecClarifications([]model.SpecClarificationItem{
		{ID: "open_question_1", Disposition: model.SpecClarificationDispositionAnswered, Response: ""},
		{ID: "assumption_1", Disposition: model.SpecClarificationDispositionRejected, Response: ""},
	}, current)
	if err == nil {
		t.Fatal("expected validation error for missing answer/rejection reason")
	}
}

func TestValidateSpecClarificationsCountsPendingItems(t *testing.T) {
	current := []model.SpecClarificationItem{
		{ID: "open_question_1", Kind: model.SpecClarificationKindOpenQuestion, Prompt: "Who gets access?"},
		{ID: "assumption_1", Kind: model.SpecClarificationKindAssumption, Prompt: "Launch for paid plans only"},
	}

	updated, pendingCount, err := validateSpecClarifications([]model.SpecClarificationItem{
		{ID: "open_question_1", Disposition: model.SpecClarificationDispositionAnswered, Response: "Workspace admins only"},
		{ID: "assumption_1", Disposition: model.SpecClarificationDispositionPending},
	}, current)
	if err != nil {
		t.Fatalf("validateSpecClarifications returned error: %v", err)
	}
	if pendingCount != 1 {
		t.Fatalf("expected 1 pending clarification, got %d", pendingCount)
	}
	if len(updated) != 2 || updated[0].Response != "Workspace admins only" {
		t.Fatalf("unexpected updated clarifications: %#v", updated)
	}
}

func TestUpsertSpecClarificationsSectionReplacesExistingSection(t *testing.T) {
	markdown := strings.Join([]string{
		"# Problem",
		"",
		"Base spec body",
		"",
		"## Clarifications",
		"",
		"### Open Questions Resolved",
		"",
		"- Old question -> old answer",
	}, "\n")

	updated := upsertSpecClarificationsSection(markdown, []model.SpecClarificationItem{
		{
			ID:          "open_question_1",
			Kind:        model.SpecClarificationKindOpenQuestion,
			Prompt:      "Who gets access?",
			Disposition: model.SpecClarificationDispositionAnswered,
			Response:    "Workspace admins only",
		},
	})

	if strings.Count(updated, "## Clarifications") != 1 {
		t.Fatalf("expected one clarifications section, got %q", updated)
	}
	if !strings.Contains(updated, "Workspace admins only") {
		t.Fatalf("expected updated clarification answer in markdown, got %q", updated)
	}
	if strings.Contains(updated, "old answer") {
		t.Fatalf("expected old clarifications section to be replaced, got %q", updated)
	}
}

func TestValidatePlanningStoriesNormalizesPlannerEnums(t *testing.T) {
	priority := "critical"
	stories := []model.ProposedTask{
		{
			Name:               "Stabilize ingest",
			TaskType:          "task",
			Priority:           &priority,
			AcceptanceCriteria: []string{"Ingest completes successfully"},
		},
	}

	if err := validatePlanningTasks(stories); err != nil {
		t.Fatalf("validatePlanningTasks returned error: %v", err)
	}
	if stories[0].TaskType != model.PMTaskTypeFeature {
		t.Fatalf("expected task type to normalize to feature, got %q", stories[0].TaskType)
	}
	if stories[0].Priority == nil || *stories[0].Priority != model.PMTaskPriorityUrgent {
		t.Fatalf("expected priority to normalize to urgent, got %#v", stories[0].Priority)
	}
}

func TestValidatePlanningStoriesSanitizesImplementationBrief(t *testing.T) {
	priority := "normal"
	stories := []model.ProposedTask{
		{
			Name:               "Render structured planner questions",
			TaskType:          "feature",
			Priority:           &priority,
			AcceptanceCriteria: []string{"Question blocks render inline"},
			ImplementationBrief: &model.TaskImplementationBrief{
				Approach:       " follow the existing transcript renderer ",
				TestStrategy:   " add parser coverage ",
				VerticalLayers: []string{" frontend_component ", "", "frontend_hook"},
				DependsOnFiles: []string{" frontend/src/components/pm/agentRunInteractions.ts ", ""},
				FilesToModify: []model.FileChange{
					{Path: " ", Action: "modify", Description: "skip blank"},
					{Path: " frontend/src/components/pm/AgentRunDrawer.tsx ", Action: "", Description: " render approval card "},
				},
			},
		},
	}

	if err := validatePlanningTasks(stories); err != nil {
		t.Fatalf("validatePlanningTasks returned error: %v", err)
	}

	brief := stories[0].ImplementationBrief
	if brief == nil {
		t.Fatal("expected implementation brief")
	}
	if brief.Approach != "follow the existing transcript renderer" {
		t.Fatalf("expected approach to trim whitespace, got %#v", brief)
	}
	if brief.TestStrategy != "add parser coverage" {
		t.Fatalf("expected test strategy to trim whitespace, got %#v", brief)
	}
	if len(brief.FilesToModify) != 1 {
		t.Fatalf("expected blank file paths to be dropped, got %#v", brief.FilesToModify)
	}
	if brief.FilesToModify[0].Path != "frontend/src/components/pm/AgentRunDrawer.tsx" {
		t.Fatalf("expected file path to trim whitespace, got %#v", brief.FilesToModify[0])
	}
	if brief.FilesToModify[0].Description != "render approval card" {
		t.Fatalf("expected file description to trim whitespace, got %#v", brief.FilesToModify[0])
	}
	if len(brief.VerticalLayers) != 2 || brief.VerticalLayers[0] != "frontend_component" || brief.VerticalLayers[1] != "frontend_hook" {
		t.Fatalf("expected vertical layers to filter blanks, got %#v", brief.VerticalLayers)
	}
	if len(brief.DependsOnFiles) != 1 || brief.DependsOnFiles[0] != "frontend/src/components/pm/agentRunInteractions.ts" {
		t.Fatalf("expected depends_on_files to filter blanks, got %#v", brief.DependsOnFiles)
	}
}

func TestValidatePlanningStoriesReturnsRepairOrientedErrorForMissingName(t *testing.T) {
	stories := []model.ProposedTask{
		{
			Description:        "Missing title field",
			TaskType:          "feature",
			AcceptanceCriteria: []string{"works"},
		},
	}

	err := validatePlanningTasks(stories)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), `use field "name"`) {
		t.Fatalf("expected repair-oriented error, got %v", err)
	}
}

func TestCreateStoriesFromProposalInheritsEpicTeam(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-planner-team"
	userID := "user-planner-team"
	memberID := "member-planner-team"
	defaultWorkflowID := "wf-planner-default"
	defaultStateID := "state-planner-default"
	teamWorkflowID := "wf-planner-team"
	teamStateID := "state-planner-team"
	epicID := "epic-planner-team"
	teamID := "team-planner-team"

	seedUser(t, db, userID, "planner@test.com", "Planner User", "hash")
	seedWorkspace(t, db, workspaceID, "Planner Workspace", "planner-workspace", userID)
	seedWorkspaceMember(t, db, memberID, workspaceID, userID, "planner@test.com", "Planner User", model.RoleAdmin)
	seedWorkflow(t, db, defaultWorkflowID, workspaceID, defaultStateID)
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		teamWorkflowID, workspaceID, "Engineering Workflow", teamID, teamStateID)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		teamStateID, teamWorkflowID, "To Do", "unstarted", 0, true)

	epicRepo := repository.NewPMEpicRepository(db)
	if err := epicRepo.Create(ctx, &model.PMEpic{
		ID:                 epicID,
		WorkspaceID:        workspaceID,
		Name:               "NATS Migration",
		TeamID:             &teamID,
		Health:             model.PMEpicHealthNone,
		PlanningState:      model.EpicPlanningStateReadyForStoryPlanning,
		SpecClarifications: json.RawMessage("[]"),
		CreatedBy:          &userID,
	}); err != nil {
		t.Fatalf("create epic: %v", err)
	}

	taskRepo := repository.NewPMTaskRepository(db)
	taskService := NewPMTaskService(
		taskRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
		repository.NewPMEpicRepository(db),
		repository.NewPMSprintRepository(db),
		repository.NewPMLabelRepository(db),
		nil,
		nil,
		repository.NewPMAttachmentRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		nil,
		nil,
	)
	svc := &AgentService{
		taskRepo:    taskRepo,
		epicRepo:     epicRepo,
		taskService: taskService,
	}

	stories, err := svc.createStoriesFromProposal(ctx, workspaceID, epicID, userID, []model.ProposedTask{
		{
			Ref:                "NATS-1",
			Name:               "Add NATS configuration module",
			Description:        "Create the initial configuration slice for NATS support.",
			TaskType:          model.PMTaskTypeFeature,
			AcceptanceCriteria: []string{"NATS configuration can be loaded for the service"},
		},
	})
	if err != nil {
		t.Fatalf("createStoriesFromProposal returned error: %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("expected 1 created story, got %#v", stories)
	}
	if stories[0].TeamID == nil || *stories[0].TeamID != teamID {
		t.Fatalf("expected story team_id to inherit epic team %q, got %#v", teamID, stories[0].TeamID)
	}
	if stories[0].WorkflowID != teamWorkflowID {
		t.Fatalf("expected story workflow_id %q, got %q", teamWorkflowID, stories[0].WorkflowID)
	}
	if stories[0].WorkflowStateID != teamStateID {
		t.Fatalf("expected story workflow_state_id %q, got %q", teamStateID, stories[0].WorkflowStateID)
	}
}

func TestPlannerStoryTeamIDRequiresEpicTeam(t *testing.T) {
	_, err := plannerTaskTeamID(&model.PMEpic{Name: "Teamless epic"})
	if err == nil {
		t.Fatal("expected missing epic team to be rejected")
	}
	if !strings.Contains(err.Error(), "must have a team before tasks can be created") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderPlannedStoryDescriptionRendersHTML(t *testing.T) {
	html := renderPlannedTaskDescription(model.ProposedTask{
		Description: "Document all new metrics and validation checks.",
		AcceptanceCriteria: []string{
			"GIVEN metrics docs WHEN opened THEN names and labels are documented",
			"GIVEN Prometheus scrapes /metrics WHEN queried THEN new metrics are exposed",
		},
		DependencyRefs: []string{"METRICS-1"},
	})

	if html == "" {
		t.Fatal("expected non-empty html description")
	}
	if !strings.Contains(html, "<h2") || !strings.Contains(html, "Summary") {
		t.Fatalf("expected summary heading to render as html, got %q", html)
	}
	if !strings.Contains(html, "<li>") {
		t.Fatalf("expected acceptance criteria/dependencies to render as html list, got %q", html)
	}
	if strings.Contains(html, "## Summary") {
		t.Fatalf("expected markdown headings to be converted, got %q", html)
	}
}
