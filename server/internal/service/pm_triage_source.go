package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/pmtriage"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// ErrPMTriageSourceUnavailable identifies missing, archived or erased source work.
var ErrPMTriageSourceUnavailable = errors.New("PM triage source unavailable")

type pmTriageSource struct {
	input  pmtriage.Input
	hash   string
	taskID string
	teamID *string
}

func (s *PMTriageService) loadSource(ctx context.Context, workspaceID, kind, id string) (*pmTriageSource, error) {
	source := &pmTriageSource{input: pmtriage.Input{SourceKind: kind, SourceID: id, Teams: []pmtriage.Option{}, Labels: []pmtriage.Option{}, Candidates: []pmtriage.Candidate{}}}
	switch kind {
	case "task":
		task, err := s.tasks.GetRawByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if task == nil || task.WorkspaceID != workspaceID || task.Archived {
			return nil, ErrPMTriageSourceUnavailable
		}
		if err := requireTeamAccess(ctx, task.TeamID); err != nil {
			return nil, err
		}
		source.taskID = task.ID
		source.teamID = task.TeamID
		source.input.Text = task.Name + "\n" + pmTriageText(task.Description)
		source.hash = pmTriageHash(source.input.Text + "\n" + task.UpdatedAt.String())
	case "support_conversation":
		actor := authorization.GetActor(ctx)
		if actor == nil || !authorization.NewRBACEngine().Can(actor.Role, authorization.PermSupportEdit) {
			return nil, &model.ErrForbidden{Message: "permission to edit support conversations is required"}
		}
		conversation, err := s.support.loadConversationAccessible(ctx, workspaceID, id)
		if err != nil {
			return nil, err
		}
		if conversation == nil || conversation.AnonymizedAt != nil {
			return nil, ErrPMTriageSourceUnavailable
		}
		messages, err := s.support.ListConversationMessages(ctx, workspaceID, id, false)
		if err != nil {
			return nil, err
		}
		source.input.Text, source.hash = supportPMTriageEvidence(conversation, messages)
	default:
		return nil, fmt.Errorf("unsupported triage source")
	}
	return source, nil
}

func (s *PMTriageService) addOptions(ctx context.Context, workspaceID string, source *pmTriageSource) error {
	teams, err := s.workspaces.ListTeams(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, team := range teams {
		if !canAccessTeam(ctx, &team.ID) {
			continue
		}
		description := ""
		if team.Description != nil {
			description = *team.Description
		}
		source.input.Teams = append(source.input.Teams, pmtriage.Option{ID: team.ID, Name: team.Name, Description: description})
	}
	archived := false
	// Workspace-wide labels and labels in the task's current team only. Team
	// reassignment never implicitly applies labels belonging to another team.
	labels, err := s.labels.ListByWorkspace(ctx, workspaceID, repository.PMLabelListOptions{TeamID: source.teamID, IncludeShared: source.teamID != nil, Archived: &archived})
	if err != nil {
		return err
	}
	for _, label := range labels {
		description := ""
		if label.Description != nil {
			description = *label.Description
		}
		source.input.Labels = append(source.input.Labels, pmtriage.Option{ID: label.ID, Name: label.Name, Description: description})
	}
	sort.Slice(source.input.Teams, func(i, j int) bool { return source.input.Teams[i].ID < source.input.Teams[j].ID })
	sort.Slice(source.input.Labels, func(i, j int) bool { return source.input.Labels[i].ID < source.input.Labels[j].ID })
	return nil
}

func pmTriageText(value *string) string {
	if value == nil {
		return ""
	}
	return tiptap.RichTextToMarkdown(*value)
}
