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
	stories := []model.ProposedStory{
		{
			Name:               "Stabilize ingest",
			StoryType:          "task",
			Priority:           &priority,
			AcceptanceCriteria: []string{"Ingest completes successfully"},
		},
	}

	if err := validatePlanningStories(stories); err != nil {
		t.Fatalf("validatePlanningStories returned error: %v", err)
	}
	if stories[0].StoryType != model.PMStoryTypeChore {
		t.Fatalf("expected story type to normalize to chore, got %q", stories[0].StoryType)
	}
	if stories[0].Priority == nil || *stories[0].Priority != model.PMStoryPriorityUrgent {
		t.Fatalf("expected priority to normalize to urgent, got %#v", stories[0].Priority)
	}
}

func TestValidatePlanningStoriesSanitizesImplementationBrief(t *testing.T) {
	priority := "normal"
	stories := []model.ProposedStory{
		{
			Name:               "Render structured planner questions",
			StoryType:          "feature",
			Priority:           &priority,
			AcceptanceCriteria: []string{"Question blocks render inline"},
			ImplementationBrief: &model.StoryImplementationBrief{
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

	if err := validatePlanningStories(stories); err != nil {
		t.Fatalf("validatePlanningStories returned error: %v", err)
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

func TestCreateStoriesFromProposalInheritsEpicTeam(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-planner-team"
	userID := "user-planner-team"
	memberID := "member-planner-team"
	workflowID := "wf-planner-team"
	stateID := "state-planner-team"
	epicID := "epic-planner-team"
	teamID := "team-planner-team"

	seedUser(t, db, userID, "planner@test.com", "Planner User", "hash")
	seedWorkspace(t, db, workspaceID, "Planner Workspace", "planner-workspace", userID)
	seedWorkspaceMember(t, db, memberID, workspaceID, userID, "planner@test.com", "Planner User", model.RoleAdmin)
	seedWorkflow(t, db, workflowID, workspaceID, stateID)

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

	storyRepo := repository.NewPMStoryRepository(db)
	storyService := NewPMStoryService(
		storyRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
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
		storyRepo:    storyRepo,
		epicRepo:     epicRepo,
		storyService: storyService,
	}

	stories, err := svc.createStoriesFromProposal(ctx, workspaceID, epicID, userID, []model.ProposedStory{
		{
			Ref:                "NATS-1",
			Name:               "Add NATS configuration module",
			Description:        "Create the initial configuration slice for NATS support.",
			StoryType:          model.PMStoryTypeFeature,
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
}

func TestPlannerStoryTeamIDRequiresEpicTeam(t *testing.T) {
	_, err := plannerStoryTeamID(&model.PMEpic{Name: "Teamless epic"})
	if err == nil {
		t.Fatal("expected missing epic team to be rejected")
	}
	if !strings.Contains(err.Error(), "must have a team before creating stories") {
		t.Fatalf("unexpected error: %v", err)
	}
}
