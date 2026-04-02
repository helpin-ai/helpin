package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func teamActivityAction(oldName, newName string) string {
	switch {
	case oldName == "" && newName == "":
		return ""
	case oldName == "" && newName != "":
		return "assigned this story to team " + newName
	case oldName != "" && newName == "":
		return "removed this story from team " + oldName
	case oldName != newName:
		return "moved this story from team " + oldName + " to " + newName
	default:
		return ""
	}
}

func memberActivityAction(fieldLabel, oldName, newName string) string {
	switch {
	case oldName == "" && newName == "":
		return ""
	case fieldLabel == "owner" && oldName == "" && newName != "":
		return "assigned owner " + newName
	case fieldLabel == "requester" && oldName == "" && newName != "":
		return "set requester to " + newName
	case oldName != "" && newName == "":
		return "removed " + fieldLabel + " " + oldName
	case oldName != newName:
		return "changed " + fieldLabel + " from " + oldName + " to " + newName
	default:
		return ""
	}
}

func planningLinkActivityAction(kind, oldName, newName string) string {
	switch {
	case oldName == "" && newName == "":
		return ""
	case oldName == "" && newName != "":
		return "added this story to " + kind + " " + newName
	case oldName != "" && newName == "":
		return "removed this story from " + kind + " " + oldName
	case oldName != newName:
		return "moved this story from " + kind + " " + oldName + " to " + newName
	default:
		return ""
	}
}

func estimateActivityAction(oldValue, newValue *int) string {
	switch {
	case oldValue == nil && newValue == nil:
		return ""
	case oldValue == nil && newValue != nil:
		return fmt.Sprintf("set estimate to %d", *newValue)
	case oldValue != nil && newValue == nil:
		return "cleared estimate"
	case *oldValue != *newValue:
		return fmt.Sprintf("changed estimate from %d to %d", *oldValue, *newValue)
	default:
		return ""
	}
}

func deadlineActivityAction(oldValue, newValue *time.Time) string {
	oldDate := formatTaskActivityDate(oldValue)
	newDate := formatTaskActivityDate(newValue)

	switch {
	case oldDate == "" && newDate == "":
		return ""
	case oldDate == "" && newDate != "":
		return "set due date to " + newDate
	case oldDate != "" && newDate == "":
		return "cleared due date"
	case oldDate != newDate:
		return "changed due date from " + oldDate + " to " + newDate
	default:
		return ""
	}
}

func blockerReasonActivityAction(oldValue, newValue *string, oldBlocked, newBlocked bool) string {
	if !oldBlocked || !newBlocked {
		return ""
	}
	if normalizedTaskText(oldValue) == normalizedTaskText(newValue) {
		return ""
	}
	return "updated blocker reason"
}

func resolveTaskTeamName(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, workspaceID string, teamID *string) (string, error) {
	if workspaceRepo == nil || teamID == nil || strings.TrimSpace(*teamID) == "" {
		return "", nil
	}
	team, err := workspaceRepo.GetTeamByID(ctx, workspaceID, *teamID)
	if err != nil {
		return "", err
	}
	if team == nil {
		return "", nil
	}
	return team.Name, nil
}

func taskMemberName(member *model.AssignableMember) string {
	if member == nil {
		return ""
	}
	if strings.TrimSpace(member.DisplayName) != "" {
		return member.DisplayName
	}
	return member.Email
}

func formatTaskActivityDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}

func normalizedTaskText(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
