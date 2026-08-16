package repository

import (
	"context"
	"encoding/json"
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

// GetAccountByIDForWorkspace returns an email account only when it belongs to
// the requested workspace. HTTP-facing code should use this method so an
// entity UUID can never bypass workspace authorization.
func (r *CRMEmailRepository) GetAccountByIDForWorkspace(ctx context.Context, workspaceID, id string) (*model.CRMEmailAccount, error) {
	var account model.CRMEmailAccount
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace email account: %w", err)
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

// GetAccountByNormalizedEmail returns a non-pending account by normalized mailbox
// identity within a workspace and provider.
func (r *CRMEmailRepository) GetAccountByNormalizedEmail(ctx context.Context, workspaceID, provider, normalizedEmail string) (*model.CRMEmailAccount, error) {
	var account model.CRMEmailAccount
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND provider = ? AND normalized_email_address = ?", workspaceID, provider, normalizedEmail).
		Where("status <> ?", model.CRMEmailAccountStatusPendingOAuth).
		First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get email account by normalized email: %w", err)
	}
	return &account, nil
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

// DeletePendingOAuthAccounts removes abandoned OAuth attempts for one member.
// Starting a new flow intentionally invalidates any older flow for that member.
func (r *CRMEmailRepository) DeletePendingOAuthAccounts(ctx context.Context, workspaceID, memberID string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND member_id = ? AND status = ?", workspaceID, memberID, model.CRMEmailAccountStatusPendingOAuth).
		Delete(&model.CRMEmailAccount{}).Error; err != nil {
		return fmt.Errorf("delete pending oauth accounts: %w", err)
	}
	return nil
}

// UpdateSyncState updates the sync state for an email account.
func (r *CRMEmailRepository) UpdateSyncState(ctx context.Context, accountID string, syncState model.JSONB) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailAccount{}).Where("id = ?", accountID).Update("sync_state", syncState).Error; err != nil {
		return fmt.Errorf("update sync state: %w", err)
	}
	return nil
}

// ListAccountsWithSyncedData returns the subset of account IDs that currently
// have synced email, thread, or calendar records.
func (r *CRMEmailRepository) ListAccountsWithSyncedData(ctx context.Context, accountIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}

	type source struct {
		table string
	}
	sources := []source{
		{table: "crm_email_threads"},
		{table: "crm_email_messages"},
		{table: "crm_calendar_events"},
	}
	for _, src := range sources {
		var ids []string
		if err := r.db.WithContext(ctx).
			Table(src.table).
			Distinct("email_account_id").
			Where("email_account_id IN ?", accountIDs).
			Pluck("email_account_id", &ids).Error; err != nil {
			return nil, fmt.Errorf("list synced account ids from %s: %w", src.table, err)
		}
		for _, id := range ids {
			result[id] = true
		}
	}

	return result, nil
}

// GetAccountRecordCounts returns synced record counts for a mailbox.
func (r *CRMEmailRepository) GetAccountRecordCounts(ctx context.Context, accountID string) (model.CRMEmailSyncedDataCounts, error) {
	var counts model.CRMEmailSyncedDataCounts

	if err := r.db.WithContext(ctx).
		Table("crm_email_threads").
		Where("email_account_id = ?", accountID).
		Count(&counts.Threads).Error; err != nil {
		return counts, fmt.Errorf("count email threads: %w", err)
	}
	if err := r.db.WithContext(ctx).
		Table("crm_email_messages").
		Where("email_account_id = ?", accountID).
		Count(&counts.Messages).Error; err != nil {
		return counts, fmt.Errorf("count email messages: %w", err)
	}
	if err := r.db.WithContext(ctx).
		Table("crm_calendar_events").
		Where("email_account_id = ?", accountID).
		Count(&counts.CalendarEvents).Error; err != nil {
		return counts, fmt.Errorf("count calendar events: %w", err)
	}

	return counts, nil
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

// GetMessageByID returns a message by ID with participant contact IDs populated.
func (r *CRMEmailRepository) GetMessageByID(ctx context.Context, id string) (*model.CRMEmailMessage, error) {
	var message model.CRMEmailMessage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get email message: %w", err)
	}
	type row struct {
		ContactID string
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("crm_email_message_contacts").
		Select("contact_id").
		Where("message_id = ?", message.ID).
		Order("participant_role ASC, contact_id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list message contact ids: %w", err)
	}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seen[row.ContactID]; exists {
			continue
		}
		seen[row.ContactID] = struct{}{}
		message.ContactIDs = append(message.ContactIDs, row.ContactID)
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

// GetThreadByID returns a thread by ID.
func (r *CRMEmailRepository) GetThreadByID(ctx context.Context, id string) (*model.CRMEmailThread, error) {
	var thread model.CRMEmailThread
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&thread).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get email thread: %w", err)
	}
	return &thread, nil
}

// CRMEntityBelongsToWorkspace verifies an optional CRM entity reference before
// it is accepted on an email record. The table name is selected by trusted
// server code; user input is never interpolated into the query.
func (r *CRMEmailRepository) CRMEntityBelongsToWorkspace(ctx context.Context, entityType, workspaceID, id string) (bool, error) {
	if id == "" {
		return true, nil
	}
	var table string
	switch entityType {
	case "contact":
		table = "crm_contacts"
	case "deal":
		table = "crm_deals"
	default:
		return false, fmt.Errorf("unsupported crm entity type %q", entityType)
	}
	var count int64
	if err := r.db.WithContext(ctx).Table(table).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("verify %s workspace: %w", entityType, err)
	}
	return count > 0, nil
}

// IncrementThreadMessageCount increments the message count and updates last_message_at.
func (r *CRMEmailRepository) IncrementThreadMessageCount(ctx context.Context, threadID string, lastMessageAt time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailThread{}).Where("id = ?", threadID).
		Updates(map[string]interface{}{
			"message_count":   gorm.Expr("message_count + 1"),
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
	if filters.ContactID != nil && *filters.ContactID != "" {
		contactSubquery := r.db.WithContext(ctx).
			Model(&model.CRMEmailMessage{}).
			Select("1").
			Joins("JOIN crm_email_message_contacts ON crm_email_message_contacts.message_id = crm_email_messages.id").
			Where("crm_email_messages.thread_id = crm_email_threads.id").
			Where("crm_email_message_contacts.contact_id = ?", *filters.ContactID)

		if r.db.Dialector.Name() == "postgres" {
			query = query.Where(
				"crm_email_threads.contact_ids @> ?::jsonb OR EXISTS (?)",
				fmt.Sprintf(`["%s"]`, *filters.ContactID),
				contactSubquery,
			)
		} else {
			query = query.Where("EXISTS (?)", contactSubquery)
		}
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

// ReplaceMessageContacts replaces all participant-contact associations for a message
// and updates the legacy primary contact_id field in the same transaction.
func (r *CRMEmailRepository) ReplaceMessageContacts(ctx context.Context, messageID string, primaryContactID *string, associations []model.CRMEmailMessageContact) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("message_id = ?", messageID).Delete(&model.CRMEmailMessageContact{}).Error; err != nil {
			return fmt.Errorf("clear message contacts: %w", err)
		}
		if len(associations) > 0 {
			if err := tx.Create(&associations).Error; err != nil {
				return fmt.Errorf("create message contacts: %w", err)
			}
		}
		if err := tx.Model(&model.CRMEmailMessage{}).
			Where("id = ?", messageID).
			Update("contact_id", primaryContactID).Error; err != nil {
			return fmt.Errorf("update message contact_id: %w", err)
		}
		return nil
	})
}

// ListMessagesMissingAssociations returns a batch of messages that do not yet
// have participant-contact associations.
func (r *CRMEmailRepository) ListMessagesMissingAssociations(ctx context.Context, limit int) ([]model.CRMEmailMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	subquery := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessageContact{}).
		Select("1").
		Where("crm_email_message_contacts.message_id = crm_email_messages.id")

	var messages []model.CRMEmailMessage
	if err := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessage{}).
		Where("NOT EXISTS (?)", subquery).
		Order("sent_at ASC, id ASC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages missing associations: %w", err)
	}
	return messages, nil
}

// ListMessagesMissingAssociationsByAccount returns a batch of messages for one mailbox
// that do not yet have participant-contact associations.
func (r *CRMEmailRepository) ListMessagesMissingAssociationsByAccount(ctx context.Context, accountID string, limit int) ([]model.CRMEmailMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	subquery := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessageContact{}).
		Select("1").
		Where("crm_email_message_contacts.message_id = crm_email_messages.id")

	var messages []model.CRMEmailMessage
	if err := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessage{}).
		Where("email_account_id = ?", accountID).
		Where("NOT EXISTS (?)", subquery).
		Order("sent_at ASC, id ASC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list account messages missing associations: %w", err)
	}
	return messages, nil
}

// CountMessagesMissingAssociationsByAccount returns the number of mailbox messages without participant associations.
func (r *CRMEmailRepository) CountMessagesMissingAssociationsByAccount(ctx context.Context, accountID string) (int64, error) {
	subquery := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessageContact{}).
		Select("1").
		Where("crm_email_message_contacts.message_id = crm_email_messages.id")

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessage{}).
		Where("email_account_id = ?", accountID).
		Where("NOT EXISTS (?)", subquery).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count messages missing associations: %w", err)
	}
	return count, nil
}

// CountThreadsWithEmptyContactIDsByAccount returns the number of threads whose cache is empty
// even though message-contact associations exist.
func (r *CRMEmailRepository) CountThreadsWithEmptyContactIDsByAccount(ctx context.Context, accountID string) (int64, error) {
	associationSubquery := r.db.WithContext(ctx).
		Table("crm_email_message_contacts").
		Select("1").
		Joins("JOIN crm_email_messages ON crm_email_messages.id = crm_email_message_contacts.message_id").
		Where("crm_email_messages.thread_id = crm_email_threads.id")

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.CRMEmailThread{}).
		Where("email_account_id = ?", accountID).
		Where("EXISTS (?)", associationSubquery).
		Where("COALESCE(CAST(contact_ids AS TEXT), '') IN ('', '[]')").
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count threads with empty contact cache: %w", err)
	}
	return count, nil
}

// RefreshThreadContactIDs rebuilds the thread-level contact cache from message
// participant associations.
func (r *CRMEmailRepository) RefreshThreadContactIDs(ctx context.Context, threadID string) error {
	var contactIDs []string
	if err := r.db.WithContext(ctx).
		Table("crm_email_message_contacts").
		Distinct("crm_email_message_contacts.contact_id").
		Joins("JOIN crm_email_messages ON crm_email_messages.id = crm_email_message_contacts.message_id").
		Where("crm_email_messages.thread_id = ?", threadID).
		Order("crm_email_message_contacts.contact_id ASC").
		Pluck("crm_email_message_contacts.contact_id", &contactIDs).Error; err != nil {
		return fmt.Errorf("list thread contact ids: %w", err)
	}

	payload, err := json.Marshal(contactIDs)
	if err != nil {
		return fmt.Errorf("marshal thread contact ids: %w", err)
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CRMEmailThread{}).
		Where("id = ?", threadID).
		Update("contact_ids", json.RawMessage(payload)).Error; err != nil {
		return fmt.Errorf("update thread contact ids: %w", err)
	}
	return nil
}

// RefreshAllThreadContactIDs rebuilds the contact cache for all email threads.
func (r *CRMEmailRepository) RefreshAllThreadContactIDs(ctx context.Context) error {
	var threadIDs []string
	if err := r.db.WithContext(ctx).
		Model(&model.CRMEmailThread{}).
		Order("id ASC").
		Pluck("id", &threadIDs).Error; err != nil {
		return fmt.Errorf("list thread ids: %w", err)
	}
	for _, threadID := range threadIDs {
		if err := r.RefreshThreadContactIDs(ctx, threadID); err != nil {
			return err
		}
	}
	return nil
}

// ListRecentMessagesForThread returns the most recent earlier messages in a
// thread before the provided message. Results are ordered oldest-first.
func (r *CRMEmailRepository) ListRecentMessagesForThread(ctx context.Context, threadID, beforeMessageID string, beforeSentAt time.Time, limit int) ([]model.CRMEmailMessage, error) {
	if threadID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 3
	}

	query := r.db.WithContext(ctx).
		Model(&model.CRMEmailMessage{}).
		Where("thread_id = ?", threadID).
		Where("(sent_at < ?) OR (sent_at = ? AND id <> ?)", beforeSentAt, beforeSentAt, beforeMessageID).
		Order("sent_at DESC, id DESC").
		Limit(limit)

	var messages []model.CRMEmailMessage
	if err := query.Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list recent thread messages: %w", err)
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
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
		contactSubquery := r.db.WithContext(ctx).
			Model(&model.CRMEmailMessageContact{}).
			Select("1").
			Where("crm_email_message_contacts.message_id = crm_email_messages.id").
			Where("crm_email_message_contacts.contact_id = ?", *filters.ContactID)
		query = query.Where(
			"crm_email_messages.contact_id = ? OR EXISTS (?)",
			*filters.ContactID,
			contactSubquery,
		)
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
	if err := r.populateMessageContactIDs(ctx, messages); err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

func (r *CRMEmailRepository) populateMessageContactIDs(ctx context.Context, messages []model.CRMEmailMessage) error {
	if len(messages) == 0 {
		return nil
	}

	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		messageIDs = append(messageIDs, message.ID)
	}

	type row struct {
		MessageID string
		ContactID string
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("crm_email_message_contacts").
		Select("message_id, contact_id").
		Where("message_id IN ?", messageIDs).
		Order("message_id ASC, participant_role ASC, contact_id ASC").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("list message contact ids: %w", err)
	}

	grouped := make(map[string][]string, len(messages))
	seen := make(map[string]map[string]struct{}, len(messages))
	for _, row := range rows {
		if _, ok := seen[row.MessageID]; !ok {
			seen[row.MessageID] = map[string]struct{}{}
		}
		if _, ok := seen[row.MessageID][row.ContactID]; ok {
			continue
		}
		seen[row.MessageID][row.ContactID] = struct{}{}
		grouped[row.MessageID] = append(grouped[row.MessageID], row.ContactID)
	}

	for i := range messages {
		if ids := grouped[messages[i].ID]; len(ids) > 0 {
			messages[i].ContactIDs = ids
			continue
		}
		if messages[i].ContactID != nil {
			messages[i].ContactIDs = []string{*messages[i].ContactID}
		} else {
			messages[i].ContactIDs = []string{}
		}
	}
	return nil
}
