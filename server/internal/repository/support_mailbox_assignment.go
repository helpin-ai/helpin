package repository

import (
	"context"
	"fmt"
	"slices"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListAssignmentEligibleMembers resolves both module access and the proposed inbox
// access, so create/update validation runs before either setting is persisted.
func (r *SupportMailboxRepository) ListAssignmentEligibleMembers(ctx context.Context, workspaceID string, linkedTeamID *string, memberIDs []string) ([]model.AssignableMember, error) {
	workspaceRepo := NewWorkspaceRepository(r.db)
	members, err := workspaceRepo.ListSupportAssignableMembers(ctx, workspaceID, nil)
	if err != nil {
		return nil, err
	}
	var teamUserIDs []string
	if linkedTeamID != nil {
		teamUserIDs, err = workspaceRepo.ListActiveTeamUserIDs(ctx, workspaceID, *linkedTeamID)
		if err != nil {
			return nil, err
		}
	}
	eligible := make([]model.AssignableMember, 0, len(members))
	for _, member := range members {
		if member.UserID == nil || *member.UserID == "" {
			continue
		}
		if member.Role == model.RoleAdmin || member.Role == model.RoleOwner || (member.Role == model.RoleMember && (slices.Contains(memberIDs, member.ID) || slices.Contains(teamUserIDs, *member.UserID))) {
			eligible = append(eligible, member)
		}
	}
	return eligible, nil
}

// ListAssignmentCandidateUserIDs filters the configured pool against current
// permissions every time, then orders candidates by the existing round-robin rule.
// An empty or stale pool never falls back to other inbox members or admins.
func (r *SupportMailboxRepository) ListAssignmentCandidateUserIDs(ctx context.Context, workspaceID string, mailbox *model.SupportMailbox) ([]string, error) {
	if mailbox == nil || !mailbox.Active || mailbox.WorkspaceID != workspaceID || mailbox.AssignmentMode == "manual" || len(mailbox.AssignmentMemberIDs) == 0 {
		return nil, nil
	}
	members, err := NewWorkspaceRepository(r.db).ListSupportAssignableMembers(ctx, workspaceID, &mailbox.ID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(mailbox.AssignmentMemberIDs))
	for _, member := range members {
		if member.UserID == nil || *member.UserID == "" {
			continue
		}
		if (member.Role == model.RoleMember || member.Role == model.RoleAdmin || member.Role == model.RoleOwner) && slices.Contains(mailbox.AssignmentMemberIDs, member.ID) {
			userIDs = append(userIDs, *member.UserID)
		}
	}
	if len(userIDs) < 2 {
		return userIDs, nil
	}
	var ordered []string
	if err := r.db.WithContext(ctx).Table("workspace_members wm").
		Select("wm.user_id").
		Joins("LEFT JOIN support_conversations sc ON sc.workspace_id = ? AND sc.mailbox_id = ? AND sc.assigned_user_id = wm.user_id", workspaceID, mailbox.ID).
		Where("wm.workspace_id = ? AND wm.user_id IN ?", workspaceID, userIDs).
		Group("wm.user_id").
		Order("COALESCE(MAX(sc.created_at), '1970-01-01 00:00:00') ASC, wm.user_id ASC").
		Pluck("wm.user_id", &ordered).Error; err != nil {
		return nil, fmt.Errorf("order inbox assignment candidates: %w", err)
	}
	return ordered, nil
}
