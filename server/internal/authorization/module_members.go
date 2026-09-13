package authorization

import (
	"context"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// FilterMembersByModuleAccess keeps active identities with effective module access.
// Use the same authorization rules as module entry, including inherited team grants.
func (s *AuthzService) FilterMembersByModuleAccess(ctx context.Context, workspaceID string, members []model.AssignableMember, module model.ModuleID) ([]model.AssignableMember, error) {
	if !model.IsValidWorkspaceModule(module) {
		return nil, fmt.Errorf("unknown module %q", module)
	}
	allowed := make([]model.AssignableMember, 0, len(members))
	for _, member := range members {
		if member.Status != model.WorkspaceMemberStatusActive {
			continue
		}
		actor := &Actor{WorkspaceID: workspaceID, WorkspaceMemberID: member.ID, Role: member.Role, Status: member.Status}
		if member.Role != model.RoleOwner && member.Role != model.RoleAdmin {
			teams, err := s.memberRepo.GetTeamMemberships(ctx, member.ID)
			if err != nil {
				return nil, fmt.Errorf("resolve member teams: %w", err)
			}
			actor.TeamMemberships = teams
		}
		access, err := s.CanAccessModule(ctx, actor, module)
		if err != nil {
			return nil, err
		}
		if access {
			allowed = append(allowed, member)
		}
	}
	return allowed, nil
}
