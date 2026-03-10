package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMEmailRepository handles DB operations for CRM email entities.
type CRMEmailRepository struct {
	db *gorm.DB
}

// NewCRMEmailRepository creates a new CRMEmailRepository.
func NewCRMEmailRepository(db *gorm.DB) *CRMEmailRepository {
	return &CRMEmailRepository{db: db}
}

// ── Email Accounts ──

// CreateAccount inserts an email account.
func (r *CRMEmailRepository) CreateAccount(ctx context.Context, account *model.CRMEmailAccount) error {
	if err := r.db.WithContext(ctx).Create(account).Error; err != nil {
		return fmt.Errorf("create email account: %w", err)
	}
	return nil
}

// GetAccountByID returns an email account by ID.
func (r *CRMEmailRepository) GetAccountByID(ctx context.Context, id string) (*model.CRMEmailAccount, error) {
	var account model.CRMEmailAccount
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get email account: %w", err)
	}
	return &account, nil
}

// ListAccounts returns email accounts in a workspace with optional filters.
func (r *CRMEmailRepository) ListAccounts(ctx context.Context, workspaceID string, filters model.CRMEmailAccountListFilters) ([]model.CRMEmailAccount, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMEmailAccount{}).Where("workspace_id = ?", workspaceID)

	if filters.MemberID != nil && *filters.MemberID != "" {
		query = query.Where("member_id = ?", *filters.MemberID)
	}
	if filters.Provider != nil && *filters.Provider != "" {
		query = query.Where("provider = ?", *filters.Provider)
	}
	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}

	var accounts []model.CRMEmailAccount
	if err := query.Order("created_at DESC").Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("list email accounts: %w", err)
	}
	return accounts, nil
}

// UpdateAccount updates an email account.
func (r *CRMEmailRepository) UpdateAccount(ctx context.Context, account *model.CRMEmailAccount) error {
	if err := r.db.WithContext(ctx).Save(account).Error; err != nil {
		return fmt.Errorf("update email account: %w", err)
	}
	return nil
}

// DeleteAccount removes an email account.
func (r *CRMEmailRepository) DeleteAccount(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMEmailAccount{}).Error; err != nil {
		return fmt.Errorf("delete email account: %w", err)
	}
	return nil
}

// GetAccountByOAuthState returns an email account by its OAuth state token.
func (r *CRMEmailRepository) GetAccountByOAuthState(ctx context.Context, state string) (*model.CRMEmailAccount, error) {
	var account model.CRMEmailAccount
	if err := r.db.WithContext(ctx).Where("oauth_state = ?", state).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get email account by oauth state: %w", err)
	}
	return &account, nil
}

// UpdateSyncState updates the sync state for an email account.
func (r *CRMEmailRepository) UpdateSyncState(ctx context.Context, accountID string, syncState model.JSONB) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailAccount{}).Where("id = ?", accountID).Update("sync_state", syncState).Error; err != nil {
		return fmt.Errorf("update sync state: %w", err)
	}
	return nil
}

// GetMessageByExternalID returns a message by its external Gmail ID within an account.
func (r *CRMEmailRepository) GetMessageByExternalID(ctx context.Context, accountID, externalID string) (*model.CRMEmailMessage, error) {
	var message model.CRMEmailMessage
	if err := r.db.WithContext(ctx).Where("email_account_id = ? AND message_external_id = ?", accountID, externalID).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get message by external id: %w", err)
	}
	return &message, nil
}

// GetThreadByExternalID returns a thread by its external Gmail thread ID within an account.
func (r *CRMEmailRepository) GetThreadByExternalID(ctx context.Context, accountID, externalID string) (*model.CRMEmailThread, error) {
	var thread model.CRMEmailThread
	if err := r.db.WithContext(ctx).Where("email_account_id = ? AND thread_external_id = ?", accountID, externalID).First(&thread).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get thread by external id: %w", err)
	}
	return &thread, nil
}

// IncrementThreadMessageCount increments the message count and updates last_message_at.
func (r *CRMEmailRepository) IncrementThreadMessageCount(ctx context.Context, threadID string, lastMessageAt time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailThread{}).Where("id = ?", threadID).
		Updates(map[string]interface{}{
			"message_count":  gorm.Expr("message_count + 1"),
			"last_message_at": lastMessageAt,
		}).Error; err != nil {
		return fmt.Errorf("increment thread message count: %w", err)
	}
	return nil
}

// ── Email Threads ──

// CreateThread inserts an email thread.
func (r *CRMEmailRepository) CreateThread(ctx context.Context, thread *model.CRMEmailThread) error {
	if err := r.db.WithContext(ctx).Create(thread).Error; err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	return nil
}

// ListThreads returns email threads with optional filters.
func (r *CRMEmailRepository) ListThreads(ctx context.Context, workspaceID string, filters model.CRMEmailThreadListFilters, pagination model.PMPagination) ([]model.CRMEmailThread, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMEmailThread{}).Where("workspace_id = ?", workspaceID)

	if filters.EmailAccountID != nil && *filters.EmailAccountID != "" {
		query = query.Where("email_account_id = ?", *filters.EmailAccountID)
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + *filters.Search + "%"
		query = query.Where("subject ILIKE ?", search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count email threads: %w", err)
	}

	var threads []model.CRMEmailThread
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("last_message_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&threads).Error; err != nil {
		return nil, 0, fmt.Errorf("list email threads: %w", err)
	}
	return threads, total, nil
}

// ── Email Messages ──

// CreateMessage inserts an email message.
func (r *CRMEmailRepository) CreateMessage(ctx context.Context, message *model.CRMEmailMessage) error {
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		return fmt.Errorf("create email message: %w", err)
	}
	return nil
}

// ListMessages returns email messages with optional filters and pagination.
func (r *CRMEmailRepository) ListMessages(ctx context.Context, workspaceID string, filters model.CRMEmailMessageListFilters, pagination model.PMPagination) ([]model.CRMEmailMessage, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMEmailMessage{}).Where("workspace_id = ?", workspaceID)

	if filters.ThreadID != nil && *filters.ThreadID != "" {
		query = query.Where("thread_id = ?", *filters.ThreadID)
	}
	if filters.EmailAccountID != nil && *filters.EmailAccountID != "" {
		query = query.Where("email_account_id = ?", *filters.EmailAccountID)
	}
	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_id = ?", *filters.ContactID)
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}
	if filters.Direction != nil && *filters.Direction != "" {
		query = query.Where("direction = ?", *filters.Direction)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count email messages: %w", err)
	}

	var messages []model.CRMEmailMessage
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("sent_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&messages).Error; err != nil {
		return nil, 0, fmt.Errorf("list email messages: %w", err)
	}
	return messages, total, nil
}
