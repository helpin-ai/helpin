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

func (r *SupportEmailSenderRepository) UpdateDisplayName(ctx context.Context, workspaceID, senderID, displayName string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailSender{}).
		Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(senderID)).
		Updates(map[string]any{
			"display_name": strings.TrimSpace(displayName),
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		return fmt.Errorf("update support email sender display name: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderRepository) UpdateVerificationByDomain(ctx context.Context, workspaceID, domainName string, senderDomain *model.SupportEmailSenderDomain) error {
	if senderDomain == nil {
		return nil
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"postmark_domain_id":             senderDomain.PostmarkDomainID,
		"return_path_domain":             senderDomain.ReturnPathDomain,
		"return_path_domain_cname_value": senderDomain.ReturnPathDomainCNAMEValue,
		"return_path_domain_verified":    senderDomain.ReturnPathDomainVerified,
		"dkim_host":                      senderDomain.DKIMHost,
		"dkim_text_value":                senderDomain.DKIMTextValue,
		"dkim_pending_host":              senderDomain.DKIMPendingHost,
		"dkim_pending_text_value":        senderDomain.DKIMPendingTextValue,
		"dkim_verified":                  senderDomain.DKIMVerified,
		"dkim_update_status":             senderDomain.DKIMUpdateStatus,
		"domain_status":                  senderDomain.Status,
		"verification_status":            senderDomain.Status,
		"last_checked_at":                senderDomain.LastCheckedAt,
		"last_error":                     senderDomain.LastError,
		"updated_at":                     now,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailSender{}).
		Where("workspace_id = ? AND LOWER(domain) = LOWER(?)", workspaceID, strings.TrimSpace(domainName)).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update support email sender verification by domain: %w", err)
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
	if err := r.attachMailboxIDs(ctx, workspaceID, senders); err != nil {
		return nil, err
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
	senders := []model.SupportEmailSender{sender}
	if err := r.attachMailboxIDs(ctx, workspaceID, senders); err != nil {
		return nil, err
	}
	return &senders[0], nil
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
	senders := []model.SupportEmailSender{sender}
	if err := r.attachMailboxIDs(ctx, workspaceID, senders); err != nil {
		return nil, err
	}
	return &senders[0], nil
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
	senders := []model.SupportEmailSender{sender}
	if err := r.attachMailboxIDs(ctx, workspaceID, senders); err != nil {
		return nil, err
	}
	return &senders[0], nil
}

func (r *SupportEmailSenderRepository) GetByForwardingVerificationToken(ctx context.Context, token string) (*model.SupportEmailSender, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Where("ses.forwarding_verification_token = ?", token).
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email sender by forwarding token: %w", err)
	}
	senders := []model.SupportEmailSender{sender}
	if err := r.attachMailboxIDs(ctx, sender.WorkspaceID, senders); err != nil {
		return nil, err
	}
	return &senders[0], nil
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
	senders := []model.SupportEmailSender{sender}
	if err := r.attachMailboxIDs(ctx, workspaceID, senders); err != nil {
		return nil, err
	}
	return &senders[0], nil
}

func (r *SupportEmailSenderRepository) GetMailboxDefaultVerified(ctx context.Context, workspaceID string, mailboxID *string) (*model.SupportEmailSender, error) {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return nil, nil
	}
	normalizedMailboxID := strings.TrimSpace(*mailboxID)
	var sender model.SupportEmailSender
	if err := r.baseQuery(ctx).
		Joins("LEFT JOIN support_email_sender_mailboxes sesm ON sesm.sender_id = ses.id AND sesm.mailbox_id = ?", normalizedMailboxID).
		Joins("JOIN support_mailboxes target_sm ON target_sm.id = ?", normalizedMailboxID).
		Where("ses.workspace_id = ? AND target_sm.workspace_id = ? AND target_sm.active = ? AND ses.active = ? AND ses.default_scope = ? AND ses.dkim_verified = ? AND ses.return_path_domain_verified = ? AND (sesm.mailbox_id = ? OR ses.mailbox_id = ?)", workspaceID, workspaceID, true, true, "mailbox", true, true, normalizedMailboxID, normalizedMailboxID).
		First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get mailbox default support email sender: %w", err)
	}
	senders := []model.SupportEmailSender{sender}
	if err := r.attachMailboxIDs(ctx, workspaceID, senders); err != nil {
		return nil, err
	}
	return &senders[0], nil
}

func (r *SupportEmailSenderRepository) SetDefault(ctx context.Context, workspaceID, senderID, scope string, mailboxIDs []string) error {
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
			if err := tx.Where("workspace_id = ? AND sender_id = ?", workspaceID, senderID).Delete(&model.SupportEmailSenderMailbox{}).Error; err != nil {
				return fmt.Errorf("clear sender inbox mappings: %w", err)
			}
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND id = ?", workspaceID, senderID).
				Updates(map[string]any{"default_scope": "workspace", "mailbox_id": nil, "active": true, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("set workspace default support email sender: %w", err)
			}
		case "mailbox":
			normalizedMailboxIDs := normalizeUniqueStrings(mailboxIDs)
			if len(normalizedMailboxIDs) == 0 {
				return fmt.Errorf("mailbox_ids are required for inbox sender default")
			}
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND mailbox_id IN ? AND default_scope = ?", workspaceID, normalizedMailboxIDs, "mailbox").
				Updates(map[string]any{"default_scope": "none", "active": false, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("clear mailbox default support email sender: %w", err)
			}
			if err := tx.Where("workspace_id = ? AND mailbox_id IN ?", workspaceID, normalizedMailboxIDs).Delete(&model.SupportEmailSenderMailbox{}).Error; err != nil {
				return fmt.Errorf("clear selected inbox sender mappings: %w", err)
			}
			if err := tx.Where("workspace_id = ? AND sender_id = ?", workspaceID, senderID).Delete(&model.SupportEmailSenderMailbox{}).Error; err != nil {
				return fmt.Errorf("clear sender inbox mappings: %w", err)
			}
			if err := deactivateMailboxSendersWithoutMappings(tx, workspaceID, senderID, now); err != nil {
				return err
			}
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND id = ?", workspaceID, senderID).
				Updates(map[string]any{"default_scope": "mailbox", "mailbox_id": normalizedMailboxIDs[0], "active": true, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("set mailbox default support email sender: %w", err)
			}
			for _, mailboxID := range normalizedMailboxIDs {
				mapping := model.SupportEmailSenderMailbox{
					WorkspaceID: workspaceID,
					SenderID:    senderID,
					MailboxID:   mailboxID,
				}
				if err := tx.Create(&mapping).Error; err != nil {
					return fmt.Errorf("create sender inbox mapping: %w", err)
				}
			}
		case "none":
			if err := tx.Where("workspace_id = ? AND sender_id = ?", workspaceID, senderID).Delete(&model.SupportEmailSenderMailbox{}).Error; err != nil {
				return fmt.Errorf("clear sender inbox mappings: %w", err)
			}
			if err := tx.Model(&model.SupportEmailSender{}).
				Where("workspace_id = ? AND id = ?", workspaceID, senderID).
				Updates(map[string]any{"default_scope": "none", "mailbox_id": nil, "active": false, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("clear support email sender default: %w", err)
			}
		default:
			return fmt.Errorf("unsupported sender default scope")
		}
		return nil
	})
}

func deactivateMailboxSendersWithoutMappings(tx *gorm.DB, workspaceID, exceptSenderID string, now time.Time) error {
	if err := tx.Model(&model.SupportEmailSender{}).
		Where("workspace_id = ? AND default_scope = ? AND id <> ? AND NOT EXISTS (SELECT 1 FROM support_email_sender_mailboxes sesm WHERE sesm.sender_id = support_email_senders.id)", workspaceID, "mailbox", exceptSenderID).
		Updates(map[string]any{"default_scope": "none", "mailbox_id": nil, "active": false, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("clear orphaned inbox sender defaults: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderRepository) Disable(ctx context.Context, workspaceID, senderID string) error {
	senderID = strings.TrimSpace(senderID)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND sender_id = ?", workspaceID, senderID).Delete(&model.SupportEmailSenderMailbox{}).Error; err != nil {
			return fmt.Errorf("clear sender inbox mappings: %w", err)
		}
		if err := tx.
			Model(&model.SupportEmailSender{}).
			Where("workspace_id = ? AND id = ?", workspaceID, senderID).
			Updates(map[string]any{
				"active":        false,
				"default_scope": "none",
				"mailbox_id":    nil,
				"updated_at":    time.Now().UTC(),
			}).Error; err != nil {
			return fmt.Errorf("disable support email sender: %w", err)
		}
		return nil
	})
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

func (r *SupportEmailSenderRepository) attachMailboxIDs(ctx context.Context, workspaceID string, senders []model.SupportEmailSender) error {
	if len(senders) == 0 {
		return nil
	}
	senderIDs := make([]string, 0, len(senders))
	senderByID := make(map[string]*model.SupportEmailSender, len(senders))
	for i := range senders {
		senderIDs = append(senderIDs, senders[i].ID)
		senderByID[senders[i].ID] = &senders[i]
		if senders[i].MailboxID != nil && strings.TrimSpace(*senders[i].MailboxID) != "" {
			senders[i].MailboxIDs = []string{strings.TrimSpace(*senders[i].MailboxID)}
		}
	}

	var mappings []model.SupportEmailSenderMailbox
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND sender_id IN ?", workspaceID, senderIDs).
		Order("created_at ASC").
		Find(&mappings).Error; err != nil {
		return fmt.Errorf("list sender inbox mappings: %w", err)
	}
	for _, mapping := range mappings {
		if sender := senderByID[mapping.SenderID]; sender != nil {
			sender.MailboxIDs = appendIfMissing(sender.MailboxIDs, mapping.MailboxID)
		}
	}
	return nil
}

func normalizeUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func appendIfMissing(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
