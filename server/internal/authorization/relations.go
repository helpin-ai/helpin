package authorization

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AuthorizationRelation represents a row in the authorization_relations table.
// It stores principal → resource → relation tuples for object-level access control.
type AuthorizationRelation struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PrincipalType string    `gorm:"type:varchar(50);not null;index:idx_authz_rel_principal"`
	PrincipalID   string    `gorm:"type:varchar(100);not null;index:idx_authz_rel_principal"`
	ResourceType  string    `gorm:"type:varchar(50);not null;index:idx_authz_rel_resource"`
	ResourceID    string    `gorm:"type:varchar(100);not null;index:idx_authz_rel_resource"`
	Relation      string    `gorm:"type:varchar(50);not null"`
	WorkspaceID   string    `gorm:"type:uuid;not null;index:idx_authz_rel_workspace"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

func (AuthorizationRelation) TableName() string {
	return "authorization_relations"
}

// RelationEngine handles object-level access control using relation tuples.
type RelationEngine struct {
	db *gorm.DB
}

// NewRelationEngine creates a new RelationEngine.
func NewRelationEngine(db *gorm.DB) *RelationEngine {
	return &RelationEngine{db: db}
}

// GrantRelation creates a relation tuple. Idempotent: does nothing if the
// tuple already exists.
func (e *RelationEngine) GrantRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID string) error {
	var count int64
	err := e.db.Model(&AuthorizationRelation{}).
		Where("principal_type = ? AND principal_id = ? AND resource_type = ? AND resource_id = ? AND relation = ? AND workspace_id = ?",
			principalType, principalID, resourceType, resourceID, relation, workspaceID).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("check existing relation: %w", err)
	}
	if count > 0 {
		return nil
	}

	rel := &AuthorizationRelation{
		PrincipalType: principalType,
		PrincipalID:   principalID,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Relation:      relation,
		WorkspaceID:   workspaceID,
	}
	if err := e.db.Create(rel).Error; err != nil {
		return fmt.Errorf("grant relation: %w", err)
	}
	return nil
}

// RevokeRelation removes a relation tuple.
func (e *RelationEngine) RevokeRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID string) error {
	err := e.db.Where(
		"principal_type = ? AND principal_id = ? AND resource_type = ? AND resource_id = ? AND relation = ? AND workspace_id = ?",
		principalType, principalID, resourceType, resourceID, relation, workspaceID,
	).Delete(&AuthorizationRelation{}).Error
	if err != nil {
		return fmt.Errorf("revoke relation: %w", err)
	}
	return nil
}

// CanAccess checks whether the actor can access a resource with the given relation.
// Resolution order: direct user → team → workspace → public.
func (e *RelationEngine) CanAccess(actor *Actor, resourceType, resourceID, relation string) bool {
	// 1. Direct user relation
	if e.hasRelation("user", actor.WorkspaceMemberID, resourceType, resourceID, relation, actor.WorkspaceID) {
		return true
	}
	// 2. Team relation (check all teams the actor belongs to)
	for _, tm := range actor.TeamMemberships {
		if e.hasRelation("team", tm.TeamID, resourceType, resourceID, relation, actor.WorkspaceID) {
			return true
		}
	}
	// 3. Workspace-wide relation
	if e.hasRelation("workspace", actor.WorkspaceID, resourceType, resourceID, relation, actor.WorkspaceID) {
		return true
	}
	// 4. Public relation (only for viewer)
	if relation == "viewer" && e.hasRelation("public", "*", resourceType, resourceID, relation, actor.WorkspaceID) {
		return true
	}
	return false
}

// ListAccessible returns resource IDs of a given type that the actor can access
// with a given relation.
func (e *RelationEngine) ListAccessible(actor *Actor, resourceType, relation string) ([]string, error) {
	// Build the set of principal conditions the actor satisfies.
	type principalCond struct {
		PType string
		PID   string
	}
	conds := []principalCond{
		{PType: "user", PID: actor.WorkspaceMemberID},
		{PType: "workspace", PID: actor.WorkspaceID},
	}
	if relation == "viewer" {
		conds = append(conds, principalCond{PType: "public", PID: "*"})
	}
	for _, tm := range actor.TeamMemberships {
		conds = append(conds, principalCond{PType: "team", PID: tm.TeamID})
	}

	// Build OR query
	q := e.db.Model(&AuthorizationRelation{}).
		Select("DISTINCT resource_id").
		Where("resource_type = ? AND relation = ? AND workspace_id = ?", resourceType, relation, actor.WorkspaceID)

	// Build principal filter
	q = q.Where(e.db.Where("1=0")) // start false, OR conditions below
	subQ := e.db.Where("1=0")
	for _, c := range conds {
		subQ = subQ.Or("principal_type = ? AND principal_id = ?", c.PType, c.PID)
	}
	q = e.db.Model(&AuthorizationRelation{}).
		Select("DISTINCT resource_id").
		Where("resource_type = ? AND relation = ? AND workspace_id = ?", resourceType, relation, actor.WorkspaceID).
		Where(subQ)

	var ids []string
	if err := q.Pluck("resource_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list accessible: %w", err)
	}
	return ids, nil
}

func (e *RelationEngine) hasRelation(principalType, principalID, resourceType, resourceID, relation, workspaceID string) bool {
	var count int64
	e.db.Model(&AuthorizationRelation{}).
		Where("principal_type = ? AND principal_id = ? AND resource_type = ? AND resource_id = ? AND relation = ? AND workspace_id = ?",
			principalType, principalID, resourceType, resourceID, relation, workspaceID).
		Count(&count)
	return count > 0
}
