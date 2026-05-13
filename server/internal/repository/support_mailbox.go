package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportMailboxRepository struct {
	db *gorm.DB
}

func NewSupportMailboxRepository(db *gorm.DB) *SupportMailboxRepository {
	return &SupportMailboxRepository{db: db}
}

func supportMailboxAccessCondition(mailboxAlias string) string {
	return fmt.Sprintf(`(
		EXISTS (
			SELECT 1
			FROM support_mailbox_memberships smm
			JOIN workspace_members wm_explicit ON wm_explicit.id = smm.workspace_member_id
			WHERE smm.mailbox_id = %s.id
			  AND smm.workspace_member_id = ?
			  AND wm_explicit.status = 'active'
		)
		OR EXISTS (
			SELECT 1
			FROM team_workspace_memberships twm
			JOIN workspace_members wm_team ON wm_team.id = twm.workspace_member_id
			WHERE twm.team_id = %s.linked_team_id
			  AND twm.workspace_member_id = ?
			  AND wm_team.status = 'active'
		)
	)`, mailboxAlias, mailboxAlias)
}

func supportMailboxEffectiveMemberCountExpr(mailboxAlias string) string {
	return fmt.Sprintf(`(
		SELECT COUNT(DISTINCT wm.id)
		FROM workspace_members wm
		WHERE wm.workspace_id = %s.workspace_id
		  AND wm.status = 'active'
		  AND (
			EXISTS (
				SELECT 1
				FROM support_mailbox_memberships smm
				WHERE smm.mailbox_id = %s.id
				  AND smm.workspace_member_id = wm.id
			)
			OR EXISTS (
				SELECT 1
				FROM team_workspace_memberships twm
				WHERE twm.team_id = %s.linked_team_id
				  AND twm.workspace_member_id = wm.id
			)
		  )
	)`, mailboxAlias, mailboxAlias, mailboxAlias)
}

func (r *SupportMailboxRepository) Create(ctx context.Context, mailbox *model.SupportMailbox) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxPosition int
		if err := tx.Model(&model.SupportMailbox{}).
			Where("workspace_id = ?", mailbox.WorkspaceID).
			Select("COALESCE(MAX(position), -1)").
			Scan(&maxPosition).Error; err != nil {
			return fmt.Errorf("load support mailbox position: %w", err)
		}
		mailbox.Position = maxPosition + 1
		if err := tx.Create(mailbox).Error; err != nil {
			return fmt.Errorf("create support mailbox: %w", err)
		}
		return nil
	})
}

func (r *SupportMailboxRepository) Update(ctx context.Context, mailbox *model.SupportMailbox) error {
	if err := r.db.WithContext(ctx).Save(mailbox).Error; err != nil {
		return fmt.Errorf("update support mailbox: %w", err)
	}
	return nil
}

func (r *SupportMailboxRepository) GetByID(ctx context.Context, workspaceID, mailboxID string) (*model.SupportMailbox, error) {
	var mailbox model.SupportMailbox
	if err := r.baseMailboxQuery(ctx).
		Where("sm.workspace_id = ? AND sm.id = ?", workspaceID, mailboxID).
		First(&mailbox).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support mailbox: %w", err)
	}
	return &mailbox, nil
}

func (r *SupportMailboxRepository) GetByHandle(ctx context.Context, workspaceID, handle string) (*model.SupportMailbox, error) {
	var mailbox model.SupportMailbox
	if err := r.baseMailboxQuery(ctx).
		Where("sm.workspace_id = ? AND LOWER(sm.handle) = LOWER(?)", workspaceID, handle).
		First(&mailbox).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support mailbox by handle: %w", err)
	}
	return &mailbox, nil
}

func (r *SupportMailboxRepository) ListByWorkspace(ctx context.Context, workspaceID string, includeArchived bool) ([]model.SupportMailbox, error) {
	query := r.baseMailboxQuery(ctx).Where("sm.workspace_id = ?", workspaceID)
	if !includeArchived {
		query = query.Where("sm.active = ?", true)
	}
	var mailboxes []model.SupportMailbox
	if err := query.Order("sm.position ASC, sm.created_at ASC").Find(&mailboxes).Error; err != nil {
		return nil, fmt.Errorf("list support mailboxes: %w", err)
	}
	return mailboxes, nil
}

func (r *SupportMailboxRepository) ListAccessible(ctx context.Context, workspaceID, workspaceMemberID, role string, includeArchived bool) ([]model.SupportMailbox, error) {
	query := r.baseMailboxQuery(ctx).Where("sm.workspace_id = ?", workspaceID)
	if !includeArchived {
		query = query.Where("sm.active = ?", true)
	}

	isElevated := role == model.RoleOwner || role == model.RoleAdmin
	if !isElevated {
		query = query.Where(supportMailboxAccessCondition("sm"), workspaceMemberID, workspaceMemberID)
	}

	var mailboxes []model.SupportMailbox
	if err := query.Order("sm.position ASC, sm.created_at ASC").Find(&mailboxes).Error; err != nil {
		return nil, fmt.Errorf("list accessible support mailboxes: %w", err)
	}
	return mailboxes, nil
}

func (r *SupportMailboxRepository) Reorder(ctx context.Context, workspaceID string, mailboxIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, mailboxID := range mailboxIDs {
			if err := tx.Model(&model.SupportMailbox{}).
				Where("workspace_id = ? AND id = ?", workspaceID, mailboxID).
				Update("position", index).Error; err != nil {
				return fmt.Errorf("reorder support mailbox: %w", err)
			}
		}
		return nil
	})
}

func (r *SupportMailboxRepository) ReplaceMembers(ctx context.Context, mailboxID string, workspaceMemberIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("mailbox_id = ?", mailboxID).Delete(&model.SupportMailboxMembership{}).Error; err != nil {
			return fmt.Errorf("clear support mailbox members: %w", err)
		}
		for _, workspaceMemberID := range workspaceMemberIDs {
			memberID := strings.TrimSpace(workspaceMemberID)
			if memberID == "" {
				continue
			}
			if err := tx.Create(&model.SupportMailboxMembership{
				MailboxID:         mailboxID,
				WorkspaceMemberID: memberID,
			}).Error; err != nil {
				return fmt.Errorf("create support mailbox membership: %w", err)
			}
		}
		return nil
	})
}

func (r *SupportMailboxRepository) AddMembers(ctx context.Context, mailboxID string, workspaceMemberIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, workspaceMemberID := range workspaceMemberIDs {
			memberID := strings.TrimSpace(workspaceMemberID)
			if memberID == "" {
				continue
			}
			record := &model.SupportMailboxMembership{
				MailboxID:         mailboxID,
				WorkspaceMemberID: memberID,
			}
			if err := tx.Where("mailbox_id = ? AND workspace_member_id = ?", mailboxID, memberID).
				FirstOrCreate(record).Error; err != nil {
				return fmt.Errorf("add support mailbox member: %w", err)
			}
		}
		return nil
	})
}

func (r *SupportMailboxRepository) ListMembers(ctx context.Context, mailboxID string) ([]model.SupportMailboxMember, error) {
	var members []model.SupportMailboxMember
	if err := r.db.WithContext(ctx).
		Table("support_mailbox_memberships smm").
		Joins("JOIN workspace_members wm ON wm.id = smm.workspace_member_id").
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("smm.mailbox_id = ? AND wm.status = ?", mailboxID, model.WorkspaceMemberStatusActive).
		Order("COALESCE(NULLIF(wm.display_name, ''), u.full_name, wm.email) ASC").
		Select(`
			smm.workspace_member_id,
			wm.user_id,
			wm.email,
			COALESCE(NULLIF(wm.display_name, ''), u.full_name, wm.email) AS display_name,
			u.avatar_url,
			u.avatar_style,
			u.avatar_seed,
			u.avatar_background_mode,
			u.avatar_background_color,
			wm.role
		`).
		Scan(&members).Error; err != nil {
		return nil, fmt.Errorf("list support mailbox members: %w", err)
	}
	return members, nil
}

func (r *SupportMailboxRepository) ListActiveMemberUserIDs(ctx context.Context, workspaceID, mailboxID string) ([]string, error) {
	var userIDs []string
	if err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Where("wm.workspace_id = ? AND wm.status = ? AND wm.user_id IS NOT NULL", workspaceID, model.WorkspaceMemberStatusActive).
		Where(`(
			EXISTS (
				SELECT 1
				FROM support_mailbox_memberships smm
				WHERE smm.mailbox_id = ?
				  AND smm.workspace_member_id = wm.id
			)
			OR EXISTS (
				SELECT 1
				FROM support_mailboxes sm
				JOIN team_workspace_memberships twm ON twm.team_id = sm.linked_team_id
				WHERE sm.id = ?
				  AND twm.workspace_member_id = wm.id
			)
		)`, mailboxID, mailboxID).
		Distinct().
		Order("wm.user_id ASC").
		Pluck("wm.user_id", &userIDs).Error; err != nil {
		return nil, fmt.Errorf("list support mailbox member user ids: %w", err)
	}
	return userIDs, nil
}

func (r *SupportMailboxRepository) ImportLinkedTeamMembers(ctx context.Context, workspaceID, mailboxID, teamID string) ([]string, error) {
	var memberIDs []string
	if err := r.db.WithContext(ctx).
		Table("team_workspace_memberships twm").
		Joins("JOIN workspace_members wm ON wm.id = twm.workspace_member_id").
		Where("twm.team_id = ? AND wm.workspace_id = ? AND wm.status = ?", teamID, workspaceID, model.WorkspaceMemberStatusActive).
		Order("twm.workspace_member_id ASC").
		Pluck("twm.workspace_member_id", &memberIDs).Error; err != nil {
		return nil, fmt.Errorf("load linked team members for support mailbox: %w", err)
	}
	if len(memberIDs) == 0 {
		return []string{}, nil
	}
	if err := r.AddMembers(ctx, mailboxID, memberIDs); err != nil {
		return nil, err
	}
	return memberIDs, nil
}

func (r *SupportMailboxRepository) IsMember(ctx context.Context, mailboxID, workspaceMemberID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("support_mailboxes sm").
		Where("sm.id = ? AND sm.active = ?", mailboxID, true).
		Where(supportMailboxAccessCondition("sm"), workspaceMemberID, workspaceMemberID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check support mailbox membership: %w", err)
	}
	return count > 0, nil
}

// WorkspaceUnreadCount pairs a workspace ID with the count of open conversations
// containing unread customer replies in that workspace.
type WorkspaceUnreadCount struct {
	WorkspaceID string `gorm:"column:workspace_id"`
	UnreadCount int    `gorm:"column:unread_count"`
}

// CountUnreadByWorkspacesForUser returns one row per workspace where the user is
// an active member and there is at least one open, non-AI-resolved conversation
// with unread customer replies. Workspaces with zero unread are omitted.
func (r *SupportMailboxRepository) CountUnreadByWorkspacesForUser(ctx context.Context, userID string) ([]WorkspaceUnreadCount, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, nil
	}

	query := r.db.WithContext(ctx).
		Table("support_conversations sc").
		Select("sc.workspace_id AS workspace_id, COUNT(*) AS unread_count").
		Joins(`INNER JOIN workspace_members wm
			ON wm.workspace_id = sc.workspace_id
			AND wm.user_id = ?
			AND wm.status = 'active'`, userID).
		Where("sc.status NOT IN ?", []string{model.SupportConversationStatusResolved, model.SupportConversationStatusSpam}).
		Where("NOT ("+conversationResolvedByAICondition("sc")+")").
		Where(`(
			SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = sc.id
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(sc.team_last_seen_at, ?)
		) > 0`, "1970-01-01 00:00:00").
		Group("sc.workspace_id")

	var rows []WorkspaceUnreadCount
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count support unread by workspace: %w", err)
	}
	return rows, nil
}

func (r *SupportMailboxRepository) CountUnread(ctx context.Context, workspaceID string, mailboxID *string) (int, error) {
	query := r.db.WithContext(ctx).Table("support_conversations sc").Where("sc.workspace_id = ? AND sc.status NOT IN ?", workspaceID, []string{model.SupportConversationStatusResolved, model.SupportConversationStatusSpam})
	if mailboxID == nil {
		query = query.Where("sc.mailbox_id IS NULL")
	} else {
		query = query.Where("sc.mailbox_id = ?", *mailboxID)
	}
	query = query.Where(conversationHumanInboxCondition("sc"))

	var count int64
	if err := query.Where(`
		(
			SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = sc.id
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(sc.team_last_seen_at, ?)
		) > 0
	`, "1970-01-01 00:00:00").Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count support mailbox unread: %w", err)
	}
	return int(count), nil
}

func (r *SupportMailboxRepository) CountWorkload(ctx context.Context, workspaceID string, mailboxID *string) (int, error) {
	query := r.db.WithContext(ctx).
		Table("support_conversations sc").
		Where("sc.workspace_id = ?", workspaceID).
		Where(conversationHumanInboxCondition("sc"))
	if mailboxID == nil {
		query = query.Where("sc.mailbox_id IS NULL")
	} else {
		query = query.Where("sc.mailbox_id = ?", *mailboxID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count support mailbox workload: %w", err)
	}
	return int(count), nil
}

func (r *SupportMailboxRepository) SelectRoundRobinOwnerUserID(ctx context.Context, workspaceID, mailboxID string) (*string, error) {
	type row struct {
		UserID string
	}

	var result row
	if err := r.db.WithContext(ctx).Raw(`
		SELECT wm.user_id
		FROM workspace_members wm
		LEFT JOIN support_conversations sc
		  ON sc.workspace_id = ?
		 AND sc.mailbox_id = ?
		 AND sc.assigned_user_id = wm.user_id
		WHERE wm.workspace_id = ?
		  AND wm.status = ?
		  AND wm.user_id IS NOT NULL
		  AND (
			EXISTS (
				SELECT 1
				FROM support_mailbox_memberships smm
				WHERE smm.mailbox_id = ?
				  AND smm.workspace_member_id = wm.id
			)
			OR EXISTS (
				SELECT 1
				FROM support_mailboxes sm
				JOIN team_workspace_memberships twm ON twm.team_id = sm.linked_team_id
				WHERE sm.id = ?
				  AND twm.workspace_member_id = wm.id
			)
		  )
		GROUP BY wm.user_id
		ORDER BY COALESCE(MAX(sc.created_at), '1970-01-01 00:00:00') ASC, wm.user_id ASC
		LIMIT 1
	`, workspaceID, mailboxID, workspaceID, model.WorkspaceMemberStatusActive, mailboxID, mailboxID).Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("select support mailbox round robin owner: %w", err)
	}
	if strings.TrimSpace(result.UserID) == "" {
		return nil, nil
	}
	return &result.UserID, nil
}

func (r *SupportMailboxRepository) baseMailboxQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("support_mailboxes sm").
		Joins("LEFT JOIN workspace_teams wt ON wt.id = sm.linked_team_id").
		Select(fmt.Sprintf(`
			sm.*,
			wt.name AS linked_team_name,
			%s AS member_count
		`, supportMailboxEffectiveMemberCountExpr("sm")))
}
