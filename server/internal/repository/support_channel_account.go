package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportChannelAccountNotFound means no such account in the workspace.
var ErrSupportChannelAccountNotFound = errors.New("channel account not found")

// SupportChannelAccountRepository stores connected channel accounts.
type SupportChannelAccountRepository struct {
	db *gorm.DB
}

// NewSupportChannelAccountRepository creates the repository.
func NewSupportChannelAccountRepository(db *gorm.DB) *SupportChannelAccountRepository {
	return &SupportChannelAccountRepository{db: db}
}

// Create inserts an account.
func (r *SupportChannelAccountRepository) Create(ctx context.Context, account *model.SupportChannelAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

// FindLive returns the live account for a provider account id in any
// workspace, or nil.
func (r *SupportChannelAccountRepository) FindLive(ctx context.Context, provider, externalAccountID string) (*model.SupportChannelAccount, error) {
	var account model.SupportChannelAccount
	err := r.db.WithContext(ctx).Where("provider = ? AND external_account_id = ? AND disabled_at IS NULL", provider, externalAccountID).
		Limit(1).Find(&account).Error
	if err != nil || account.ID == "" {
		return nil, err
	}
	return &account, nil
}

// Get returns a workspace's account, including disconnected ones.
func (r *SupportChannelAccountRepository) Get(ctx context.Context, workspaceID, id string) (*model.SupportChannelAccount, error) {
	var account model.SupportChannelAccount
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Limit(1).Find(&account).Error
	if err != nil {
		return nil, err
	}
	if account.ID == "" {
		return nil, ErrSupportChannelAccountNotFound
	}
	return &account, nil
}

// GetForWebhook returns a live account by id and provider, or nil. Webhooks
// arrive without a workspace.
func (r *SupportChannelAccountRepository) GetForWebhook(ctx context.Context, provider, id string) (*model.SupportChannelAccount, error) {
	var account model.SupportChannelAccount
	err := r.db.WithContext(ctx).Where("id = ? AND provider = ? AND disabled_at IS NULL", id, provider).Limit(1).Find(&account).Error
	if err != nil || account.ID == "" {
		return nil, err
	}
	return &account, nil
}

// List returns the workspace's connected accounts with delivery health,
// newest first. Disconnected accounts are omitted.
func (r *SupportChannelAccountRepository) List(ctx context.Context, workspaceID string) ([]model.SupportChannelAccountView, error) {
	var accounts []model.SupportChannelAccount
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND disabled_at IS NULL", workspaceID).
		Order("created_at DESC, id").Find(&accounts).Error; err != nil {
		return nil, err
	}
	views := make([]model.SupportChannelAccountView, len(accounts))
	if len(accounts) == 0 {
		return views, nil
	}
	ids := make([]string, len(accounts))
	for i, account := range accounts {
		ids[i] = account.ID
		views[i].SupportChannelAccount = account
	}
	var counts []struct {
		ChannelAccountID string
		DispatchStatus   string
		Count            int64
	}
	if err := r.db.WithContext(ctx).Table("support_channel_outbox").
		Select("channel_account_id, dispatch_status, COUNT(*) AS count").
		Where("workspace_id = ? AND channel_account_id IN ? AND dispatch_status IN ?", workspaceID, ids, []string{"rejected", "uncertain"}).
		Group("channel_account_id, dispatch_status").Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, c := range counts {
		for i := range views {
			if views[i].ID != c.ChannelAccountID {
				continue
			}
			if c.DispatchStatus == "rejected" {
				views[i].FailedDeliveries = c.Count
			} else {
				views[i].UnconfirmedDeliveries = c.Count
			}
		}
	}
	return views, nil
}

// Update applies column changes to a live account.
func (r *SupportChannelAccountRepository) Update(ctx context.Context, workspaceID, id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.SupportChannelAccount{}).
		Where("workspace_id = ? AND id = ? AND disabled_at IS NULL", workspaceID, id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSupportChannelAccountNotFound
	}
	return nil
}

// Disconnect disables the account and cancels its undelivered replies, which
// can no longer be sent. History stays.
func (r *SupportChannelAccountRepository) Disconnect(ctx context.Context, workspaceID, id string, at time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SupportChannelAccount{}).
			Where("workspace_id = ? AND id = ? AND disabled_at IS NULL", workspaceID, id).
			Updates(map[string]any{"disabled_at": at, "status": model.SupportChannelAccountDisabled, "updated_at": at})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrSupportChannelAccountNotFound
		}
		return tx.Table("support_channel_outbox").
			Where("workspace_id = ? AND channel_account_id = ? AND dispatch_status IN ?", workspaceID, id, []string{"pending", "dispatching"}).
			Updates(map[string]any{"dispatch_status": "cancelled", "last_error_code": "disconnected", "claim_token": nil, "lease_expires_at": nil, "updated_at": at}).Error
	})
}

// TouchEvent records that a webhook event was accepted.
func (r *SupportChannelAccountRepository) TouchEvent(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.SupportChannelAccount{}).Where("id = ?", id).
		UpdateColumn("last_event_at", at).Error
}

// MailboxInWorkspace reports whether a mailbox belongs to the workspace.
func (r *SupportChannelAccountRepository) MailboxInWorkspace(ctx context.Context, workspaceID, mailboxID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SupportMailbox{}).Where("workspace_id = ? AND id = ?", workspaceID, mailboxID).Count(&count).Error
	return count > 0, err
}

// ReplyDestinations returns the live provider threads the outbox can deliver to.
// Other accounts in the workspace are never destinations for this conversation.
func (r *SupportConversationRepository) ReplyDestinations(ctx context.Context, c *model.SupportConversation) ([]model.SupportProviderDestination, error) {
	if c.ChannelIdentityID == nil && !c.MergeHasChannel && len(c.ProviderDestinations) == 0 {
		return nil, nil
	}
	db := r.db.WithContext(ctx)
	q := db.Table("support_conversations c").
		Select(`c.id, a.provider, c.display_id, CASE WHEN a.provider = 'slack' THEN '#' || COALESCE(NULLIF(r.channel_name,''), NULLIF(c.provider_channel_name,''),split_part(c.provider_thread_key, ':', 1)) ELSE COALESCE(c.customer_name,'') END AS name`).
		Joins("JOIN support_channel_accounts a ON a.id = c.channel_account_id AND a.workspace_id = c.workspace_id AND a.provider = c.channel").
		Joins("LEFT JOIN slack_channel_routes r ON a.provider = 'slack' AND r.account_id = a.id AND r.channel_id = split_part(c.provider_thread_key, ':', 1)").
		Where("c.workspace_id = ? AND c.channel_identity_id IS NOT NULL AND c.anonymized_at IS NULL AND a.status = ? AND a.disabled_at IS NULL", c.WorkspaceID, model.SupportChannelAccountActive).
		Where("a.provider = 'telegram' OR (a.provider = 'slack' AND COALESCE(r.purpose,'mixed') IN ('support','mixed') AND split_part(c.provider_thread_key, ':', 2) <> '')")
	switch {
	case len(c.ProviderDestinations) > 0:
		ids := make([]string, 0, len(c.ProviderDestinations))
		for _, origin := range c.ProviderDestinations {
			ids = append(ids, origin.ID)
		}
		q = q.Where("c.id IN ?", ids)
	case c.ChannelIdentityID != nil:
		q = q.Where("c.id = ?", c.ID)
	default:
		// Telegram-only merges follow the latest inbound provider message.
		// Resolve that origin before checking availability so removing a bot
		// cannot redirect a reply to an earlier sender.
		origin := db.Table("support_messages m").Select("o.id").
			Joins("JOIN support_conversations o ON o.id = COALESCE(m.origin_conversation_id, m.conversation_id) AND o.workspace_id = m.workspace_id").
			Where("m.workspace_id = ? AND m.conversation_id = ? AND m.sender_type = 'customer' AND m.channel_account_id IS NOT NULL AND m.deleted_at IS NULL AND o.channel_identity_id IS NOT NULL", c.WorkspaceID, c.ID).
			Order("m.created_at DESC").Limit(1)
		q = q.Where("c.id = (?)", origin)
	}
	var destinations []model.SupportProviderDestination
	err := q.Order("c.display_id, c.id").Scan(&destinations).Error
	return destinations, err
}
