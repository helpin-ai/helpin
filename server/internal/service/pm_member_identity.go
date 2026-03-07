package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func resolveWorkspaceMember(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, workspaceID string, memberRef, legacyUserRef *string) (*model.WorkspaceMember, error) {
	var reference string
	if memberRef != nil {
		reference = strings.TrimSpace(*memberRef)
	} else if legacyUserRef != nil {
		reference = strings.TrimSpace(*legacyUserRef)
	}
	return resolveWorkspaceMemberReference(ctx, workspaceRepo, workspaceID, reference)
}

func resolveWorkspaceMemberReferences(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, workspaceID string, groups ...[]string) ([]*model.WorkspaceMember, error) {
	if workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository is required for member resolution")
	}

	seen := make(map[string]struct{})
	members := make([]*model.WorkspaceMember, 0)
	for _, group := range groups {
		for _, ref := range group {
			member, err := resolveWorkspaceMemberReference(ctx, workspaceRepo, workspaceID, ref)
			if err != nil {
				return nil, err
			}
			if member == nil {
				continue
			}
			if _, exists := seen[member.ID]; exists {
				continue
			}
			seen[member.ID] = struct{}{}
			members = append(members, member)
		}
	}
	return members, nil
}

func resolveWorkspaceMemberReference(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, workspaceID, reference string) (*model.WorkspaceMember, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, nil
	}
	if workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository is required for member resolution")
	}
	member, err := workspaceRepo.ResolveMemberReference(ctx, workspaceID, reference)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, fmt.Errorf("workspace member not found")
	}
	if member.Status == model.WorkspaceMemberStatusRevoked {
		return nil, fmt.Errorf("workspace member is revoked")
	}
	return member, nil
}

func memberIDs(members []*model.WorkspaceMember) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		if member == nil || strings.TrimSpace(member.ID) == "" {
			continue
		}
		ids = append(ids, member.ID)
	}
	return ids
}

func memberIDPtr(member *model.WorkspaceMember) *string {
	if member == nil {
		return nil
	}
	id := member.ID
	return &id
}

func memberUserIDPtr(member *model.WorkspaceMember) *string {
	if member == nil || member.UserID == nil || *member.UserID == "" {
		return nil
	}
	id := *member.UserID
	return &id
}
