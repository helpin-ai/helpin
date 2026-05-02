package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportEmailSenderRepository struct {
	db *gorm.DB
}

func NewSupportEmailSenderRepository(db *gorm.DB) *SupportEmailSenderRepository {
	return &SupportEmailSenderRepository{db: db}
}

func (r *SupportEmailSenderRepository) Create(ctx context.Context, sender *model.SupportEmailSender) error {
	if err := r.db.WithContext(ctx).Create(sender).Error; err != nil {
		return fmt.Errorf("create support email sender: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderRepository) Update(ctx context.Context, sender *model.SupportEmailSender) error {
	if err := r.db.WithContext(ctx).Save(sender).Error; err != nil {
		return fmt.Errorf("update support email sender: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportEmailSender, error) {
	var senders []model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.workspace_id = ?", workspaceID).
		Order("ses.active DESC, CASE ses.default_scope WHEN 'workspace' THEN 0 WHEN 'mailbox' THEN 1 ELSE 2 END ASC, LOWER(ses.email) ASC").
		Find(&senders).Error; err != nil {
		return nil, fmt.Errorf("list support email senders: %w", err)
	}
	return senders, nil
}

func (r *SupportEmailSenderRepository) GetByID(ctx context.Context, workspaceID, senderID string) (*model.SupportEmailSender, error) {
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.workspace_id = ? AND ses.id = ?", workspaceID, strings.TrimSpace(senderID)).
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email sender: %w", err)
	}
	return &sender, nil
}

func (r *SupportEmailSenderRepository) GetByEmail(ctx context.Context, workspaceID, senderEmail string) (*model.SupportEmailSender, error) {
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.workspace_id = ? AND LOWER(ses.email) = LOWER(?)", workspaceID, strings.TrimSpace(senderEmail)).
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email sender by email: %w", err)
	}
	return &sender, nil
}

func (r *SupportEmailSenderRepository) GetByDomain(ctx context.Context, workspaceID, domainName string) (*model.SupportEmailSender, error) {
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.workspace_id = ? AND LOWER(ses.domain) = LOWER(?)", workspaceID, strings.TrimSpace(domainName)).
		Order("ses.active DESC, ses.created_at ASC").
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email sender by domain: %w", err)
	}
	return &sender, nil
}

func (r *SupportEmailSenderRepository) GetWorkspaceDefaultVerified(ctx context.Context, workspaceID string) (*model.SupportEmailSender, error) {
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.workspace_id = ? AND ses.active = ? AND ses.default_scope = ? AND ses.dkim_verified = ? AND ses.return_path_domain_verified = ?", workspaceID, true, "workspace", true, true).
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace default support email sender: %w", err)
	}
	return &sender, nil
}

func (r *SupportEmailSenderRepository) GetMailboxDefaultVerified(ctx context.Context, workspaceID string, mailboxID *string) (*model.SupportEmailSender, error) {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return nil, nil
	}
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.workspace_id = ? AND ses.mailbox_id = ? AND ses.active = ? AND ses.default_scope = ? AND ses.dkim_verified = ? AND ses.return_path_domain_verified = ?", workspaceID, strings.TrimSpace(*mailboxID), true, "mailbox", true, true).
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get mailbox default support email sender: %w", err)
	}
	return &sender, nil
}

func (r *SupportEmailSenderRepository) SetDefault(ctx context.Context, workspaceID, senderID, scope string, mailboxID *string) error {
	scope = strings.TrimSpace(scope)
	senderID = strings.TrimSpace(senderID)
	now := time.Now().UTC()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch scope {
		case "workspace":
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND default_scope = ?", workspaceID, "workspace").
				Updates(map[string]any{"default_scope": "none", "active": false, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("clear workspace default support email sender: %w", err)
			}
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND id = ?", workspaceID, senderID).
				Updates(map[string]any{"default_scope": "workspace", "mailbox_id": nil, "active": true, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("set workspace default support email sender: %w", err)
			}
		case "mailbox":
			if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
				return fmt.Errorf("mailbox_id is required for mailbox sender default")
			}
			normalizedMailboxID := strings.TrimSpace(*mailboxID)
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND mailbox_id = ? AND default_scope = ?", workspaceID, normalizedMailboxID, "mailbox").
				Updates(map[string]any{"default_scope": "none", "active": false, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("clear mailbox default support email sender: %w", err)
			}
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND id = ?", workspaceID, senderID).
				Updates(map[string]any{"default_scope": "mailbox", "mailbox_id": normalizedMailboxID, "active": true, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("set mailbox default support email sender: %w", err)
			}
		case "none":
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND id = ?", workspaceID, senderID).
				Updates(map[string]any{"default_scope": "none", "active": false, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("clear support email sender default: %w", err)
			}
		default:
			return fmt.Errorf("unsupported sender default scope")
		}
		return nil
	})
}

func (r *SupportEmailSenderRepository) Disable(ctx context.Context, workspaceID, senderID string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailSender{}).
		Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(senderID)).
		Updates(map[string]any{
			"active":        false,
			"default_scope": "none",
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
		return fmt.Errorf("disable support email sender: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderRepository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("support_email_senders ses").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = ses.mailbox_id").
		Select(`
			ses.*,
			sm.name AS mailbox_name,
			sm.handle AS mailbox_handle,
			sm.icon AS mailbox_icon
		`)
}
