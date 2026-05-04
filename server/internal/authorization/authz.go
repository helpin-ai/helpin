package authorization

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MemberRepository is the minimal interface the AuthzService needs to resolve actors.
type MemberRepository interface {
	GetMembership(ctx context.Context, workspaceID, userID string) (*MemberInfo, error)
	GetTeamMemberships(ctx context.Context, workspaceMemberID string) ([]TeamRole, error)
}

type ModuleAccessRepository interface {
	ListAccessibleModules(ctx context.Context, workspaceID, workspaceMemberID string, teamIDs []string) ([]model.ModuleID, error)
}

type WorkspaceMFARepository interface {
	GetWorkspaceMFAPolicy(ctx context.Context, workspaceID, userID string) (model.WorkspaceMFAPolicy, error)
}

// MemberInfo holds the membership data needed for actor resolution.
type MemberInfo struct {
	ID     string
	Role   string
	Status string
}

// AuthzService is the single authorization boundary for the application.
type AuthzService struct {
	rbac       *RBACEngine
	relations  *RelationEngine
	memberRepo MemberRepository
	moduleRepo ModuleAccessRepository
	mfaRepo    WorkspaceMFARepository
}

// NewAuthzService creates a new AuthzService.
func NewAuthzService(db *gorm.DB, memberRepo MemberRepository, moduleRepo ModuleAccessRepository) *AuthzService {
	return &AuthzService{
		rbac:       NewRBACEngine(),
		relations:  NewRelationEngine(db),
		memberRepo: memberRepo,
		moduleRepo: moduleRepo,
	}
}

func (s *AuthzService) SetWorkspaceMFARepository(repo WorkspaceMFARepository) {
	s.mfaRepo = repo
}

func (s *AuthzService) WorkspaceMFAPolicy(ctx context.Context, workspaceID, userID string) (model.WorkspaceMFAPolicy, error) {
	if s.mfaRepo == nil {
		return model.WorkspaceMFAPolicy{}, nil
	}
	return s.mfaRepo.GetWorkspaceMFAPolicy(ctx, workspaceID, userID)
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

func (s *AuthzService) AccessibleModules(ctx context.Context, actor *Actor) ([]model.ModuleID, error) {
	allowed := map[model.ModuleID]struct{}{
		model.ModulePM:   {},
		model.ModuleDocs: {},
	}
	if actor == nil {
		return nil, fmt.Errorf("actor is required")
	}

	if actor.Role == model.RoleOwner || actor.Role == model.RoleAdmin {
		allowed[model.ModuleCRM] = struct{}{}
		allowed[model.ModuleSupport] = struct{}{}
		allowed[model.ModuleAutomation] = struct{}{}
		return orderedModules(allowed), nil
	}

	if s.moduleRepo == nil {
		return orderedModules(allowed), nil
	}

	modules, err := s.moduleRepo.ListAccessibleModules(ctx, actor.WorkspaceID, actor.WorkspaceMemberID, actor.TeamIDs())
	if err != nil {
		return nil, fmt.Errorf("list accessible modules: %w", err)
	}
	for _, module := range modules {
		if model.IsManagedWorkspaceModule(module) {
			allowed[module] = struct{}{}
		}
	}
	return orderedModules(allowed), nil
}

func (s *AuthzService) CanAccessModule(ctx context.Context, actor *Actor, module model.ModuleID) (bool, error) {
	if !model.IsValidWorkspaceModule(module) {
		return false, fmt.Errorf("unknown module %q", module)
	}
	modules, err := s.AccessibleModules(ctx, actor)
	if err != nil {
		return false, err
	}
	for _, candidate := range modules {
		if candidate == module {
			return true, nil
		}
	}
	return false, nil
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
	ErrNotAMember         = fmt.Errorf("not a member of this workspace")
	ErrMembershipPending  = fmt.Errorf("membership is pending")
	ErrMembershipRevoked  = fmt.Errorf("membership has been revoked")
	ErrMembershipInactive = fmt.Errorf("membership is inactive")
)

func orderedModules(allowed map[model.ModuleID]struct{}) []model.ModuleID {
	modules := make([]model.ModuleID, 0, len(allowed))
	for _, module := range model.AllWorkspaceModules() {
		if _, ok := allowed[module]; ok {
			modules = append(modules, module)
		}
	}
	return modules
}
