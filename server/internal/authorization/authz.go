package authorization

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// MemberRepository is the minimal interface the AuthzService needs to resolve actors.
type MemberRepository interface {
	GetMembership(ctx context.Context, workspaceID, userID string) (*MemberInfo, error)
	GetTeamMemberships(ctx context.Context, workspaceMemberID string) ([]TeamRole, error)
}

// MemberInfo holds the membership data needed for actor resolution.
type MemberInfo struct {
	ID     string
	Role   string
	Status string
}

// AuthzService is the single authorization boundary for the application.
type AuthzService struct {
	rbac      *RBACEngine
	relations *RelationEngine
	memberRepo MemberRepository
}

// NewAuthzService creates a new AuthzService.
func NewAuthzService(db *gorm.DB, memberRepo MemberRepository) *AuthzService {
	return &AuthzService{
		rbac:       NewRBACEngine(),
		relations:  NewRelationEngine(db),
		memberRepo: memberRepo,
	}
}

// ResolveActor builds an Actor from a user ID and workspace ID.
// Returns an error if the user is not a member or not active.
func (s *AuthzService) ResolveActor(ctx context.Context, workspaceID, userID string) (*Actor, error) {
	info, err := s.memberRepo.GetMembership(ctx, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve actor: %w", err)
	}
	if info == nil {
		return nil, ErrNotAMember
	}
	if info.Status != "active" {
		switch info.Status {
		case "pending":
			return nil, ErrMembershipPending
		case "revoked":
			return nil, ErrMembershipRevoked
		case "inactive":
			return nil, ErrMembershipInactive
		default:
			return nil, ErrMembershipInactive
		}
	}

	// Load team memberships
	teams, err := s.memberRepo.GetTeamMemberships(ctx, info.ID)
	if err != nil {
		return nil, fmt.Errorf("resolve actor team memberships: %w", err)
	}

	return &Actor{
		UserID:            userID,
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: info.ID,
		Role:              info.Role,
		Status:            info.Status,
		TeamMemberships:   teams,
	}, nil
}

// --- RBAC methods ---

// Can checks whether the actor has a specific permission in their workspace.
func (s *AuthzService) Can(actor *Actor, perm Permission) bool {
	return s.rbac.Can(actor.Role, perm)
}

// CanAny checks whether the actor has at least one of the listed permissions.
func (s *AuthzService) CanAny(actor *Actor, perms ...Permission) bool {
	return s.rbac.CanAny(actor.Role, perms...)
}

// CanManageTeam checks team-level management permission.
// Returns true if the actor is a workspace owner/admin OR a team owner for the given team.
func (s *AuthzService) CanManageTeam(actor *Actor, teamID string) bool {
	if s.rbac.Can(actor.Role, PermTeamManage) {
		return true
	}
	return actor.IsTeamOwner(teamID)
}

// IsOwnerOnly checks whether the actor has the workspace owner role.
func (s *AuthzService) IsOwnerOnly(actor *Actor) bool {
	return s.rbac.IsOwnerOnly(actor.Role)
}

// PermissionsForActor returns all effective permissions for an actor.
func (s *AuthzService) PermissionsForActor(actor *Actor) []Permission {
	return s.rbac.PermissionsForRole(actor.Role)
}

// --- Relation methods ---

// CanAccessResource checks whether the actor can perform a relation-level action on a resource.
func (s *AuthzService) CanAccessResource(actor *Actor, resourceType, resourceID, relation string) bool {
	return s.relations.CanAccess(actor, resourceType, resourceID, relation)
}

// GrantRelation creates a relation tuple.
func (s *AuthzService) GrantRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID string) error {
	return s.relations.GrantRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID)
}

// RevokeRelation removes a relation tuple.
func (s *AuthzService) RevokeRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID string) error {
	return s.relations.RevokeRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID)
}

// ListAccessible returns resource IDs of a given type that the actor can access with a given relation.
func (s *AuthzService) ListAccessible(actor *Actor, resourceType, relation string) ([]string, error) {
	return s.relations.ListAccessible(actor, resourceType, relation)
}

// --- Errors ---

// Sentinel errors for membership resolution.
var (
	ErrNotAMember        = fmt.Errorf("not a member of this workspace")
	ErrMembershipPending = fmt.Errorf("membership is pending")
	ErrMembershipRevoked = fmt.Errorf("membership has been revoked")
	ErrMembershipInactive = fmt.Errorf("membership is inactive")
)
