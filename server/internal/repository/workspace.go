package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// WorkspaceRepository handles database operations for workspaces and workspace members.
type WorkspaceRepository struct {
	db *gorm.DB
}

// NewWorkspaceRepository creates a new WorkspaceRepository.
func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// Create inserts a new workspace.
func (r *WorkspaceRepository) Create(ctx context.Context, name, slug, ownerID string, organizationID *string, description, websiteURL *string, timezone string) (*model.Workspace, error) {
	ws := &model.Workspace{
		Name:           name,
		Slug:           slug,
		OwnerID:        ownerID,
		OrganizationID: organizationID,
		Description:    description,
		WebsiteURL:     websiteURL,
		Timezone:       timezone,
	}
	if err := r.db.WithContext(ctx).Create(ws).Error; err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return ws, nil
}

// List returns all workspaces a user is an active member of, along with their role.
func (r *WorkspaceRepository) List(ctx context.Context, userID string, organizationID string) ([]model.WorkspaceWithRole, error) {
	var results []model.WorkspaceWithRole
	q := r.db.WithContext(ctx).
		Table("workspaces w").
		Select("w.id, w.name, w.slug, w.owner_id, w.organization_id, w.description, w.website_url, w.logo_url, w.timezone, w.created_at, w.updated_at, wm.role").
		Joins("JOIN workspace_members wm ON w.id = wm.workspace_id").
		Where("wm.user_id = ? AND wm.status = ?", userID, model.WorkspaceMemberStatusActive)
	if organizationID != "" {
		q = q.Where("w.organization_id = ?", organizationID)
	}
	if err := q.Order("w.created_at ASC").Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	return results, nil
}

// ListIDs returns all workspace IDs in creation order.
func (r *WorkspaceRepository) ListIDs(ctx context.Context) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.Workspace{}).
		Order("created_at ASC").
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list workspace ids: %w", err)
	}
	return ids, nil
}

// GetByID returns a workspace by its ID.
func (r *WorkspaceRepository) GetByID(ctx context.Context, id string) (*model.Workspace, error) {
	ws := &model.Workspace{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(ws).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace by id: %w", err)
	}
	return ws, nil
}

// GetBySlug returns a workspace by its slug.
func (r *WorkspaceRepository) GetBySlug(ctx context.Context, slug string) (*model.Workspace, error) {
	ws := &model.Workspace{}
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(ws).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace by slug: %w", err)
	}
	return ws, nil
}

// GetTeamByID returns a workspace team by its ID.
func (r *WorkspaceRepository) GetTeamByID(ctx context.Context, workspaceID, teamID string) (*model.WorkspaceTeam, error) {
	team := &model.WorkspaceTeam{}
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, teamID).
		First(team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace team: %w", err)
	}
	return team, nil
}

// Update modifies workspace fields.
func (r *WorkspaceRepository) Update(ctx context.Context, id string, name, description, websiteURL, logoURL, timezone *string) (*model.Workspace, error) {
	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if websiteURL != nil {
		if *websiteURL == "" {
			updates["website_url"] = nil
		} else {
			updates["website_url"] = *websiteURL
		}
	}
	if logoURL != nil {
		updates["logo_url"] = *logoURL
	}
	if timezone != nil {
		updates["timezone"] = *timezone
	}

	if err := r.db.WithContext(ctx).Model(&model.Workspace{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}

	ws := &model.Workspace{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(ws).Error; err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}
	return ws, nil
}

// Delete removes a workspace and all associated data via cascade deletion.
func (r *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Helper subqueries for indirect children.
		storyQ := "SELECT id FROM pm_tasks WHERE workspace_id = ?"
		epicQ := "SELECT id FROM pm_epics WHERE workspace_id = ?"
		sprintQ := "SELECT id FROM pm_sprints WHERE workspace_id = ?"
		objectiveQ := "SELECT id FROM pm_objectives WHERE workspace_id = ?"
		workflowQ := "SELECT id FROM pm_workflows WHERE workspace_id = ?"
		invitationQ := "SELECT id FROM workspace_invitations WHERE workspace_id = ?"
		teamQ := "SELECT id FROM workspace_teams WHERE workspace_id = ?"
		memberQ := "SELECT id FROM workspace_members WHERE workspace_id = ?"
		ticketQ := "SELECT id FROM support_conversations WHERE workspace_id = ?"

		queries := []string{
			// ── Phase 1: Indirect children (via subqueries) ──

			// Story children
			"DELETE FROM pm_task_owners WHERE task_id IN (" + storyQ + ")",
			"DELETE FROM pm_task_followers WHERE task_id IN (" + storyQ + ")",
			"DELETE FROM pm_task_labels WHERE task_id IN (" + storyQ + ")",
			"DELETE FROM pm_checklist_items WHERE task_id IN (" + storyQ + ")",
			"DELETE FROM pm_external_links WHERE task_id IN (" + storyQ + ")",

			// Comments (polymorphic via entity_id on stories and epics)
			"DELETE FROM pm_comments WHERE entity_id IN (" + storyQ + ") OR entity_id IN (" + epicQ + ")",

			// Epic children
			"DELETE FROM pm_epic_labels WHERE epic_id IN (" + epicQ + ")",
			"DELETE FROM pm_epic_objectives WHERE epic_id IN (" + epicQ + ")",

			// Sprint label join table
			"DELETE FROM pm_sprint_labels WHERE sprint_id IN (" + sprintQ + ")",

			// Objective children
			"DELETE FROM pm_key_results WHERE objective_id IN (" + objectiveQ + ")",
			"DELETE FROM pm_objective_teams WHERE objective_id IN (" + objectiveQ + ")",
			"DELETE FROM pm_objective_owners WHERE objective_id IN (" + objectiveQ + ")",
			"DELETE FROM pm_objective_labels WHERE objective_id IN (" + objectiveQ + ")",

			// Workflow states
			"DELETE FROM pm_workflow_states WHERE workflow_id IN (" + workflowQ + ")",

			// Invitation pre-assignments
			"DELETE FROM invitation_team_preassignments WHERE invitation_id IN (" + invitationQ + ")",

			// Team-scoped settings
			"DELETE FROM pm_team_estimate_settings WHERE team_id IN (" + teamQ + ")",
			"DELETE FROM pm_team_field_visibility WHERE team_id IN (" + teamQ + ")",
			"DELETE FROM pm_team_repo_defaults WHERE team_id IN (" + teamQ + ")",

			// Reward profiles (via workspace members)
			"DELETE FROM reward_profiles WHERE workspace_member_id IN (" + memberQ + ")",

			// Support messages (via conversations for FK ordering)
			"DELETE FROM support_messages WHERE conversation_id IN (" + ticketQ + ")",

			// ── Phase 2: Direct workspace_id tables ──

			// PM module
			"DELETE FROM pm_attachments WHERE workspace_id = ?",
			"DELETE FROM pm_activity_log WHERE workspace_id = ?",
			"DELETE FROM pm_tasks WHERE workspace_id = ?",
			"DELETE FROM pm_epics WHERE workspace_id = ?",
			"DELETE FROM pm_sprints WHERE workspace_id = ?",
			"DELETE FROM pm_labels WHERE workspace_id = ?",
			"DELETE FROM pm_objectives WHERE workspace_id = ?",
			"DELETE FROM pm_workflows WHERE workspace_id = ?",
			"DELETE FROM pm_epic_workflow_states WHERE workspace_id = ?",
			"DELETE FROM pm_views WHERE workspace_id = ?",
			"DELETE FROM pm_automations WHERE workspace_id = ?",
			"DELETE FROM pm_import_jobs WHERE workspace_id = ?",

			// Git module
			"DELETE FROM story_delivery_targets WHERE workspace_id = ?",
			"DELETE FROM story_git_links WHERE workspace_id = ?",
			"DELETE FROM git_repositories WHERE workspace_id = ?",
			"DELETE FROM git_integrations WHERE workspace_id = ?",

			// Agent module
			"DELETE FROM agent_run_artifacts WHERE workspace_id = ?",
			"DELETE FROM agent_runs WHERE workspace_id = ?",
			"DELETE FROM agents WHERE workspace_id = ?",
			"DELETE FROM agent_handoffs WHERE workspace_id = ?",

			// Support module
			"DELETE FROM support_conversations WHERE workspace_id = ?",
			"DELETE FROM support_widget_sessions WHERE workspace_id = ?",
			"DELETE FROM support_widget_installations WHERE workspace_id = ?",

			// Workspace structure
			"DELETE FROM team_workspace_memberships WHERE team_id IN (" + teamQ + ") OR workspace_member_id IN (" + memberQ + ")",
			"DELETE FROM workspace_managers WHERE workspace_id = ?",
			"DELETE FROM job_role_criteria WHERE workspace_id = ?",
			"DELETE FROM workspace_teams WHERE workspace_id = ?",
			"DELETE FROM workspace_settings WHERE workspace_id = ?",
			"DELETE FROM workspace_invitations WHERE workspace_id = ?",
			"DELETE FROM workspace_members WHERE workspace_id = ?",

			// Clear user default workspace references
			"UPDATE users SET default_workspace_id = NULL WHERE default_workspace_id = ?",

			// Finally, delete the workspace itself
			"DELETE FROM workspaces WHERE id = ?",
		}

		for _, q := range queries {
			argCount := strings.Count(q, "?")
			args := make([]interface{}, argCount)
			for i := range args {
				args[i] = id
			}
			if err := tx.Exec(q, args...).Error; err != nil {
				return fmt.Errorf("delete workspace %s: %w", id, err)
			}
		}

		return nil
	})
}

// AddMember adds or activates a user as a workspace member.
func (r *WorkspaceRepository) AddMember(ctx context.Context, workspaceID, userID, role string) (*model.WorkspaceMember, error) {
	userRecord, err := r.getUserIdentity(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var member *model.WorkspaceMember
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := r.findMemberTx(tx, workspaceID, userID, userRecord.Email)
		if err != nil {
			return err
		}

		if existing == nil {
			userIDCopy := userID
			member = &model.WorkspaceMember{
				WorkspaceID: workspaceID,
				UserID:      &userIDCopy,
				Email:       userRecord.Email,
				DisplayName: userRecord.FullName,
				Role:        role,
				Status:      model.WorkspaceMemberStatusActive,
				InvitedAt:   &now,
				AcceptedAt:  &now,
			}
			if err := tx.Create(member).Error; err != nil {
				return fmt.Errorf("add workspace member: %w", err)
			}
			return nil
		}

		existing.UserID = &userID
		existing.Email = userRecord.Email
		existing.DisplayName = userRecord.FullName
		existing.Role = role
		existing.Status = model.WorkspaceMemberStatusActive
		existing.AcceptedAt = &now
		if existing.InvitedAt == nil {
			existing.InvitedAt = &now
		}
		if err := tx.Save(existing).Error; err != nil {
			return fmt.Errorf("activate workspace member: %w", err)
		}
		member = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return member, nil
}

// UpsertPendingMember creates or updates a pending member identity for an invitation.
func (r *WorkspaceRepository) UpsertPendingMember(ctx context.Context, workspaceID, email, role, invitedBy string) (*model.WorkspaceMember, error) {
	email = normalizeEmail(email)
	now := time.Now().UTC()
	var member *model.WorkspaceMember
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := r.getMembershipByEmailTx(tx, workspaceID, email)
		if err != nil {
			return err
		}
		if existing == nil {
			member = &model.WorkspaceMember{
				WorkspaceID: workspaceID,
				Email:       email,
				DisplayName: email,
				Role:        role,
				Status:      model.WorkspaceMemberStatusPending,
				InvitedBy:   stringPtr(invitedBy),
				InvitedAt:   &now,
			}
			if err := tx.Create(member).Error; err != nil {
				return fmt.Errorf("create pending workspace member: %w", err)
			}
			return nil
		}

		if existing.Status != model.WorkspaceMemberStatusActive {
			existing.Status = model.WorkspaceMemberStatusPending
		}
		existing.Role = role
		existing.Email = email
		if strings.TrimSpace(existing.DisplayName) == "" {
			existing.DisplayName = email
		}
		existing.InvitedBy = stringPtr(invitedBy)
		if existing.InvitedAt == nil {
			existing.InvitedAt = &now
		}
		if err := tx.Save(existing).Error; err != nil {
			return fmt.Errorf("update pending workspace member: %w", err)
		}
		member = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return member, nil
}

// ActivatePendingMember links a workspace member identity to a real user on invite acceptance.
func (r *WorkspaceRepository) ActivatePendingMember(ctx context.Context, workspaceID string, memberID *string, userID, email, displayName, role string) (*model.WorkspaceMember, error) {
	email = normalizeEmail(email)
	now := time.Now().UTC()
	var member *model.WorkspaceMember
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing *model.WorkspaceMember
		var err error
		if memberID != nil && *memberID != "" {
			existing, err = r.getMembershipByIDTx(tx, workspaceID, *memberID)
			if err != nil {
				return err
			}
		}
		if existing == nil {
			existing, err = r.findMemberTx(tx, workspaceID, userID, email)
			if err != nil {
				return err
			}
		}

		if existing == nil {
			userIDCopy := userID
			member = &model.WorkspaceMember{
				WorkspaceID: workspaceID,
				UserID:      &userIDCopy,
				Email:       email,
				DisplayName: displayName,
				Role:        role,
				Status:      model.WorkspaceMemberStatusActive,
				InvitedAt:   &now,
				AcceptedAt:  &now,
			}
			if err := tx.Create(member).Error; err != nil {
				return fmt.Errorf("create accepted workspace member: %w", err)
			}
			return nil
		}

		existing.UserID = &userID
		existing.Email = email
		existing.DisplayName = displayName
		existing.Role = role
		existing.Status = model.WorkspaceMemberStatusActive
		if existing.InvitedAt == nil {
			existing.InvitedAt = &now
		}
		existing.AcceptedAt = &now
		if err := tx.Save(existing).Error; err != nil {
			return fmt.Errorf("activate pending workspace member: %w", err)
		}
		member = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return member, nil
}

// UpdateMemberStatus updates the lifecycle status for a workspace member.
func (r *WorkspaceRepository) UpdateMemberStatus(ctx context.Context, memberID, status string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceMember{}).
		Where("id = ?", memberID).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("update workspace member status: %w", err)
	}
	return nil
}

// GetMemberRole returns the role a user has in a workspace, or empty string if not an active member.
func (r *WorkspaceRepository) GetMemberRole(ctx context.Context, workspaceID, userID string) (string, error) {
	var m model.WorkspaceMember
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND status = ?", workspaceID, userID, model.WorkspaceMemberStatusActive).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return m.Role, nil
}

// GetMembershipByID returns a workspace membership by membership ID.
func (r *WorkspaceRepository) GetMembershipByID(ctx context.Context, workspaceID, memberID string) (*model.WorkspaceMember, error) {
	return r.getMembershipByIDTx(r.db.WithContext(ctx), workspaceID, memberID)
}

// UpdateMemberRole updates the role for a workspace member.
func (r *WorkspaceRepository) UpdateMemberRole(ctx context.Context, workspaceID, memberID, role string) error {
	result := r.db.WithContext(ctx).
		Model(&model.WorkspaceMember{}).
		Where("workspace_id = ? AND id = ?", workspaceID, memberID).
		Update("role", role)
	if result.Error != nil {
		return fmt.Errorf("update workspace member role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}

// CountMembersByRole returns the number of workspace members with a given role.
func (r *WorkspaceRepository) CountMembersByRole(ctx context.Context, workspaceID, role string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceMember{}).
		Where("workspace_id = ? AND role = ? AND status = ?", workspaceID, role, model.WorkspaceMemberStatusActive).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count workspace members by role: %w", err)
	}
	return count, nil
}

// ListMembers returns all active joined members of a workspace with user details.
func (r *WorkspaceRepository) ListMembers(ctx context.Context, workspaceID string) ([]model.MemberWithUser, error) {
	var results []model.MemberWithUser
	err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select("wm.id, wm.user_id, wm.role, wm.email, COALESCE(NULLIF(wm.display_name, ''), u.full_name) AS full_name, u.avatar_url").
		Joins("JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND wm.status = ?", workspaceID, model.WorkspaceMemberStatusActive).
		Order("COALESCE(NULLIF(wm.display_name, ''), u.full_name) ASC").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	return results, nil
}

// ListAssignableMembers returns both joined and pending identities for PM assignment pickers.
func (r *WorkspaceRepository) ListAssignableMembers(ctx context.Context, workspaceID string) ([]model.AssignableMember, error) {
	var members []model.AssignableMember
	err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select(`
			wm.id,
			wm.user_id,
			wm.role,
			wm.email,
			COALESCE(NULLIF(wm.display_name, ''), u.full_name, wm.email) AS display_name,
			u.avatar_url,
			wm.status,
			wm.invited_by,
			wm.invited_at,
			wm.accepted_at
		`).
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND wm.status <> ?", workspaceID, model.WorkspaceMemberStatusRevoked).
		Order("CASE wm.status WHEN 'active' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END, LOWER(COALESCE(NULLIF(wm.display_name, ''), u.full_name, wm.email)) ASC, LOWER(wm.email) ASC").
		Scan(&members).Error
	if err != nil {
		return nil, fmt.Errorf("list assignable workspace members: %w", err)
	}
	return members, nil
}

// GetMembership returns the active membership record for a user in a workspace.
func (r *WorkspaceRepository) GetMembership(ctx context.Context, workspaceID, userID string) (*model.WorkspaceMember, error) {
	m := &model.WorkspaceMember{}
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND status = ?", workspaceID, userID, model.WorkspaceMemberStatusActive).
		First(m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return m, nil
}

// GetAssignableMemberByID loads a workspace member identity by ID.
func (r *WorkspaceRepository) GetAssignableMemberByID(ctx context.Context, workspaceID, memberID string) (*model.AssignableMember, error) {
	var member model.AssignableMember
	err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select(`
			wm.id,
			wm.user_id,
			wm.role,
			wm.email,
			wm.display_name,
			u.avatar_url,
			wm.status,
			wm.invited_by,
			wm.invited_at,
			wm.accepted_at
		`).
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND wm.id = ?", workspaceID, memberID).
		Scan(&member).Error
	if err != nil {
		return nil, fmt.Errorf("get assignable workspace member: %w", err)
	}
	if member.ID == "" {
		return nil, nil
	}
	return &member, nil
}

// ResolveMemberReference accepts either a workspace_member.id or a legacy user id and returns the member record.
func (r *WorkspaceRepository) ResolveMemberReference(ctx context.Context, workspaceID, reference string) (*model.WorkspaceMember, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, nil
	}

	m, err := r.getMembershipByIDTx(r.db.WithContext(ctx), workspaceID, reference)
	if err != nil {
		return nil, err
	}
	if m != nil {
		return m, nil
	}

	m, err = r.getMembershipByUserIDTx(r.db.WithContext(ctx), workspaceID, reference)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *WorkspaceRepository) getUserIdentity(ctx context.Context, userID string) (*model.User, error) {
	user := &model.User{}
	err := r.db.WithContext(ctx).
		Select("id, email, full_name").
		Where("id = ?", userID).
		First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user identity: %w", err)
	}
	return user, nil
}

func (r *WorkspaceRepository) findMemberTx(tx *gorm.DB, workspaceID, userID, email string) (*model.WorkspaceMember, error) {
	if strings.TrimSpace(userID) != "" {
		member, err := r.getMembershipByUserIDTx(tx, workspaceID, userID)
		if err != nil {
			return nil, err
		}
		if member != nil {
			return member, nil
		}
	}
	if strings.TrimSpace(email) != "" {
		return r.getMembershipByEmailTx(tx, workspaceID, email)
	}
	return nil, nil
}

func (r *WorkspaceRepository) getMembershipByIDTx(tx *gorm.DB, workspaceID, memberID string) (*model.WorkspaceMember, error) {
	member := &model.WorkspaceMember{}
	err := tx.Where("workspace_id = ? AND id = ?", workspaceID, memberID).First(member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership by id: %w", err)
	}
	return member, nil
}

func (r *WorkspaceRepository) getMembershipByUserIDTx(tx *gorm.DB, workspaceID, userID string) (*model.WorkspaceMember, error) {
	member := &model.WorkspaceMember{}
	err := tx.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership by user id: %w", err)
	}
	return member, nil
}

func (r *WorkspaceRepository) getMembershipByEmailTx(tx *gorm.DB, workspaceID, email string) (*model.WorkspaceMember, error) {
	member := &model.WorkspaceMember{}
	err := tx.Where("workspace_id = ? AND LOWER(email) = LOWER(?)", workspaceID, normalizeEmail(email)).First(member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership by email: %w", err)
	}
	return member, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GetUserIDByHandle resolves a @mention handle to a user ID within a workspace.
// It tries matching against user full_name (case-insensitive, dot-separated).
func (r *WorkspaceRepository) GetUserIDByHandle(ctx context.Context, workspaceID, handle string) (string, error) {
	// Convert handle like "john.doe" to "john doe" for name matching.
	namePattern := strings.ReplaceAll(handle, ".", " ")
	var userID string
	err := r.db.WithContext(ctx).
		Table("workspace_members").
		Joins("JOIN users ON users.id = workspace_members.user_id").
		Where("workspace_members.workspace_id = ? AND workspace_members.status = 'active'", workspaceID).
		Where("LOWER(users.full_name) = LOWER(?) OR LOWER(REPLACE(users.full_name, ' ', '.')) = LOWER(?)", namePattern, handle).
		Select("workspace_members.user_id").
		Limit(1).
		Scan(&userID).Error
	if err != nil {
		return "", err
	}
	return userID, nil
}

// GetTeamByHandle resolves a persisted team handle within a workspace.
func (r *WorkspaceRepository) GetTeamByHandle(ctx context.Context, workspaceID, handle string) (*model.WorkspaceTeam, error) {
	team := &model.WorkspaceTeam{}
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(handle) = LOWER(?)", workspaceID, handle).
		First(team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team by handle: %w", err)
	}
	return team, nil
}

// ListActiveTeamUserIDs returns active linked user accounts for a team in a workspace.
func (r *WorkspaceRepository) ListActiveTeamUserIDs(ctx context.Context, workspaceID, teamID string) ([]string, error) {
	var userIDs []string
	err := r.db.WithContext(ctx).
		Table("team_workspace_memberships twm").
		Joins("JOIN workspace_members wm ON wm.id = twm.workspace_member_id").
		Where("twm.team_id = ? AND wm.workspace_id = ? AND wm.status = ? AND wm.user_id IS NOT NULL",
			teamID, workspaceID, model.WorkspaceMemberStatusActive).
		Distinct().
		Order("wm.user_id ASC").
		Pluck("wm.user_id", &userIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list active team user ids: %w", err)
	}
	return userIDs, nil
}

// ListActiveUserIDsByRoles returns active linked workspace user IDs for the given roles.
func (r *WorkspaceRepository) ListActiveUserIDsByRoles(ctx context.Context, workspaceID string, roles []string) ([]string, error) {
	if len(roles) == 0 {
		return []string{}, nil
	}

	var userIDs []string
	err := r.db.WithContext(ctx).
		Table("workspace_members").
		Where("workspace_id = ? AND status = ? AND user_id IS NOT NULL AND role IN ?", workspaceID, model.WorkspaceMemberStatusActive, roles).
		Distinct().
		Order("user_id ASC").
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list active workspace user ids by roles: %w", err)
	}
	return userIDs, nil
}

// CanUserAccessPMTeams reports whether the user can read a PM entity scoped to the given teams.
// Admins and owners can read all entities. Members and viewers can only read entities in their teams.
// An empty team scope represents an admin-only entity.
func (r *WorkspaceRepository) CanUserAccessPMTeams(ctx context.Context, workspaceID, userID string, teamIDs []string) (bool, error) {
	var member struct {
		ID   string
		Role string
	}
	err := r.db.WithContext(ctx).
		Table("workspace_members").
		Select("id, role").
		Where("workspace_id = ? AND user_id = ? AND status = ?", workspaceID, userID, model.WorkspaceMemberStatusActive).
		Limit(1).
		Scan(&member).Error
	if err != nil {
		return false, fmt.Errorf("check workspace member access: %w", err)
	}
	if member.ID == "" {
		return false, nil
	}
	if member.Role == model.RoleOwner || member.Role == model.RoleAdmin {
		return true, nil
	}
	if len(teamIDs) == 0 {
		return false, nil
	}

	var count int64
	err = r.db.WithContext(ctx).
		Table("team_workspace_memberships").
		Where("workspace_member_id = ? AND team_id IN ?", member.ID, teamIDs).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check team membership access: %w", err)
	}
	return count > 0, nil
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := value
	return &v
}
