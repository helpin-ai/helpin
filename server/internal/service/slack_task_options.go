package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Slack preserves inputs with matching block and action IDs when a modal is updated.
// Team-scoped block IDs reset those inputs when a different team is selected.
func (s *SlackService) taskOptions(ctx context.Context, workspace, team string) ([]any, error) {
	if team == "" || !canAccessTeam(ctx, &team) {
		return nil, nil
	}
	archived := false
	scope := repository.ScopeFilterOptions{TeamID: &team, IncludeShared: true, Archived: &archived}
	var blocks []any
	add := func(key, label string, options []map[string]any, initial string, multiple bool) {
		if len(options) == 0 {
			return
		}
		block := selectBlock(key+"_"+team, label, options, initial, true)
		if multiple {
			block["element"].(map[string]any)["type"] = "multi_static_select"
		}
		blocks = append(blocks, block)
	}
	workflow, err := s.tasks.workflowRepo.GetByTeamID(ctx, workspace, team)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		workflow, err = s.tasks.workflowRepo.GetDefaultWorkflow(ctx, workspace)
		if err != nil {
			return nil, err
		}
	}
	if workflow != nil {
		add("state", "Status", slackPickOptions(workflow.States, func(v model.PMWorkflowState) (string, string) { return v.ID, v.Name }), derefString(workflow.Workflow.DefaultStateID), false)
	}
	members, err := s.tasks.workspaceRepo.ListAssignableMembers(ctx, workspace)
	if err != nil {
		return nil, err
	}
	add("owners", "Assignees", slackPickOptions(members, func(v model.AssignableMember) (string, string) { return v.ID, v.DisplayName }), "", true)
	sprints, err := s.tasks.sprintRepo.List(ctx, workspace, model.PMSprintListFilters{TeamID: &team, Archived: &archived})
	if err != nil {
		return nil, err
	}
	add("sprint", "Sprint", slackPickOptions(sprints, func(v model.PMSprint) (string, string) { return v.ID, v.Name }), "", false)
	labels, err := s.tasks.labelRepo.ListByWorkspace(ctx, workspace, scope)
	if err != nil {
		return nil, err
	}
	add("labels", "Labels", slackPickOptions(labels, func(v model.PMLabel) (string, string) { return v.ID, v.Name }), "", true)
	if s.tasks.templateRepo != nil {
		templates, err := s.tasks.templateRepo.ListByWorkspace(ctx, workspace, scope)
		if err != nil {
			return nil, err
		}
		add("template", "Template", slackPickOptions(templates, func(v model.PMTaskTemplate) (string, string) { return v.ID, v.Name }), "", false)
	}
	return blocks, nil
}

func slackPickOptions[T any](items []T, identify func(T) (string, string)) []map[string]any {
	var options []map[string]any
	for _, item := range items {
		id, name := identify(item)
		if name == "" {
			continue
		}
		options = append(options, map[string]any{"value": id, "text": slackText(truncate(name, 75))})
		if len(options) == 100 {
			break
		}
	}
	return options
}
