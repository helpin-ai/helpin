package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const epicActivityDateLayout = "2006-01-02"

type epicActivityChange struct {
	action    string
	fieldName string
	oldValue  *string
	newValue  *string
	metadata  map[string]interface{}
}

func epicActivityString(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func epicActivityDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(epicActivityDateLayout)
	return &formatted
}

func epicActivityBool(value bool) *string {
	formatted := strconv.FormatBool(value)
	return &formatted
}

func epicActivityValuesEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func buildEpicActivityChanges(before, after model.PMEpic) []epicActivityChange {
	changes := make([]epicActivityChange, 0, 12)
	appendChange := func(action, fieldName string, oldValue, newValue *string) {
		if epicActivityValuesEqual(oldValue, newValue) {
			return
		}
		changes = append(changes, epicActivityChange{
			action:    action,
			fieldName: fieldName,
			oldValue:  oldValue,
			newValue:  newValue,
		})
	}

	appendChange("updated", "name", epicActivityString(&before.Name), epicActivityString(&after.Name))
	if normalizedTaskText(before.Description) != normalizedTaskText(after.Description) {
		changes = append(changes, epicActivityChange{action: "updated", fieldName: "description"})
	}
	appendChange("updated", "epic_state_id", epicActivityString(before.EpicStateID), epicActivityString(after.EpicStateID))
	appendChange("updated", "owner_member_id", epicActivityString(before.OwnerMemberID), epicActivityString(after.OwnerMemberID))
	appendChange("updated", "team_id", epicActivityString(before.TeamID), epicActivityString(after.TeamID))
	appendChange("updated", "planned_start_date", epicActivityDate(before.PlannedStartDate), epicActivityDate(after.PlannedStartDate))
	appendChange("updated", "deadline", epicActivityDate(before.Deadline), epicActivityDate(after.Deadline))
	appendChange("health_updated", "health", epicActivityString(&before.Health), epicActivityString(&after.Health))
	if normalizedTaskText(before.HealthComment) != normalizedTaskText(after.HealthComment) {
		changes = append(changes, epicActivityChange{action: "updated", fieldName: "health_comment"})
	}
	appendChange("updated", "planning_repository_id", epicActivityString(before.PlanningRepositoryID), epicActivityString(after.PlanningRepositoryID))
	appendChange("updated", "assigned_agent_id", epicActivityString(before.AssignedAgentID), epicActivityString(after.AssignedAgentID))
	if before.Archived != after.Archived {
		action := "restored"
		if after.Archived {
			action = "archived"
		}
		appendChange(action, "archived", epicActivityBool(before.Archived), epicActivityBool(after.Archived))
	}
	if !nullableStringsEqual(before.Color, after.Color) {
		changes = append(changes, epicActivityChange{action: "updated", fieldName: "color"})
	}

	return changes
}

func epicLabelActivityChanges(before, after []model.PMLabel) []epicActivityChange {
	beforeIDs := make(map[string]struct{}, len(before))
	afterIDs := make(map[string]struct{}, len(after))
	for _, label := range before {
		beforeIDs[label.ID] = struct{}{}
	}
	for _, label := range after {
		afterIDs[label.ID] = struct{}{}
	}

	changes := make([]epicActivityChange, 0)
	for _, label := range after {
		if _, exists := beforeIDs[label.ID]; exists {
			continue
		}
		value := label.ID
		changes = append(changes, epicActivityChange{
			action: "label_added", fieldName: "label", newValue: &value,
			metadata: map[string]interface{}{"new_label": label.Name},
		})
	}
	for _, label := range before {
		if _, exists := afterIDs[label.ID]; exists {
			continue
		}
		value := label.ID
		changes = append(changes, epicActivityChange{
			action: "label_removed", fieldName: "label", oldValue: &value,
			metadata: map[string]interface{}{"old_label": label.Name},
		})
	}
	return changes
}

func (s *PMEpicService) epicActivityValueLabel(ctx context.Context, workspaceID, fieldName string, value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return ""
	}
	id := strings.TrimSpace(*value)
	switch fieldName {
	case "epic_state_id":
		if s.workflowRepo == nil {
			return ""
		}
		states, err := s.workflowRepo.ListEpicStates(ctx, workspaceID)
		if err != nil {
			return ""
		}
		for _, state := range states {
			if state.ID == id {
				return state.Name
			}
		}
	case "owner_member_id":
		if s.workspaceRepo == nil {
			return ""
		}
		member, err := s.workspaceRepo.GetAssignableMemberByID(ctx, workspaceID, id)
		if err == nil && member != nil {
			return taskMemberName(member)
		}
	case "team_id":
		if s.workspaceRepo == nil {
			return ""
		}
		team, err := s.workspaceRepo.GetTeamByID(ctx, workspaceID, id)
		if err == nil && team != nil {
			return team.Name
		}
	case "planning_repository_id":
		if s.gitRepo == nil {
			return ""
		}
		repo, err := s.gitRepo.GetByID(ctx, workspaceID, id)
		if err == nil && repo != nil {
			return repo.FullName
		}
	case "assigned_agent_id":
		if s.agentService == nil || s.agentService.agentRepo == nil {
			return ""
		}
		agent, err := s.agentService.agentRepo.GetByID(ctx, workspaceID, id)
		if err == nil && agent != nil {
			return agent.Name
		}
	}
	return ""
}

func (s *PMEpicService) enrichEpicActivityChange(ctx context.Context, workspaceID string, change epicActivityChange) epicActivityChange {
	oldLabel := s.epicActivityValueLabel(ctx, workspaceID, change.fieldName, change.oldValue)
	newLabel := s.epicActivityValueLabel(ctx, workspaceID, change.fieldName, change.newValue)
	if oldLabel == "" && newLabel == "" {
		return change
	}
	if change.metadata == nil {
		change.metadata = make(map[string]interface{})
	}
	if oldLabel != "" {
		change.metadata["old_label"] = oldLabel
	}
	if newLabel != "" {
		change.metadata["new_label"] = newLabel
	}
	return change
}

func (s *PMEpicService) logEpicUpdateActivity(ctx context.Context, before, after *model.EpicWithStats, actorID string) {
	if s == nil || s.activityService == nil || before == nil || after == nil {
		return
	}
	changes := buildEpicActivityChanges(before.Epic, after.Epic)
	changes = append(changes, epicLabelActivityChanges(before.Labels, after.Labels)...)
	for _, rawChange := range changes {
		change := s.enrichEpicActivityChange(ctx, after.Epic.WorkspaceID, rawChange)
		fieldName := change.fieldName
		if err := s.activityService.Log(
			ctx,
			after.Epic.WorkspaceID,
			"epic",
			after.Epic.ID,
			optionalActor(actorID),
			change.action,
			&fieldName,
			change.oldValue,
			change.newValue,
			change.metadata,
		); err != nil {
			s.logger.ErrorContext(ctx, "failed to log epic field activity", "error", err, "epic_id", after.Epic.ID, "field_name", fieldName)
		}
	}
}
