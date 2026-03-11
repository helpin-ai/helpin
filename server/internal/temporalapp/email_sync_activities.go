package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
)

// EmailSyncActivities contains activities for email synchronization.
type EmailSyncActivities struct {
	gmailClient  *sync.GmailSyncClient
	emailRepo    *repository.CRMEmailRepository
	contactRepo  *repository.CRMContactRepository
	calendarRepo *repository.CRMCalendarRepository
}

// NewEmailSyncActivities creates email sync activities.
func NewEmailSyncActivities(
	gmailClient *sync.GmailSyncClient,
	emailRepo *repository.CRMEmailRepository,
	contactRepo *repository.CRMContactRepository,
	calendarRepo *repository.CRMCalendarRepository,
) *EmailSyncActivities {
	return &EmailSyncActivities{
		gmailClient:  gmailClient,
		emailRepo:    emailRepo,
		contactRepo:  contactRepo,
		calendarRepo: calendarRepo,
	}
}

// BackfillEmailsActivity fetches last 90 days of emails.
func (a *EmailSyncActivities) BackfillEmailsActivity(ctx context.Context, accountID string) (*EmailSyncResult, error) {
	account, err := a.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}

	accessToken, err := a.gmailClient.GetValidToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get valid token: %w", err)
	}

	// Fetch messages from last 90 days.
	query := fmt.Sprintf("after:%d", time.Now().AddDate(0, 0, -90).Unix())
	processed := 0
	pageToken := ""

	for {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("processed %d messages", processed))

		messages, nextPage, err := a.gmailClient.ListMessages(ctx, accessToken, query, 100, pageToken)
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}

		for i := range messages {
			if err := a.storeMessage(ctx, account, &messages[i]); err != nil {
				slog.ErrorContext(ctx, "failed to store email", "error", err, "message_id", messages[i].ID)
				continue
			}
			processed++
		}

		if nextPage == "" {
			break
		}
		pageToken = nextPage
	}

	// Update sync state.
	syncState := model.JSONB{"status": "syncing", "last_history_id": ""}
	if err := a.emailRepo.UpdateSyncState(ctx, accountID, syncState); err != nil {
		slog.ErrorContext(ctx, "failed to update sync state", "error", err, "account_id", accountID)
	}

	slog.Info("backfill email sync complete", "account_id", accountID, "messages_processed", processed)

	return &EmailSyncResult{MessagesProcessed: processed}, nil
}

// IncrementalSyncActivity fetches new emails since last sync.
func (a *EmailSyncActivities) IncrementalSyncActivity(ctx context.Context, accountID string) (*EmailSyncResult, error) {
	account, err := a.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}

	accessToken, err := a.gmailClient.GetValidToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get valid token: %w", err)
	}

	// Use timestamp-based sync.
	query := ""
	if account.LastSyncedAt != nil {
		query = fmt.Sprintf("after:%d", account.LastSyncedAt.Unix())
	} else {
		query = fmt.Sprintf("after:%d", time.Now().Add(-24*time.Hour).Unix())
	}

	messages, _, err := a.gmailClient.ListMessages(ctx, accessToken, query, 500, "")
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	processed := 0
	for i := range messages {
		if err := a.storeMessage(ctx, account, &messages[i]); err != nil {
			slog.ErrorContext(ctx, "failed to store email", "error", err, "message_id", messages[i].ID)
			continue
		}
		processed++
	}

	// Update last synced at.
	now := time.Now()
	account.LastSyncedAt = &now
	if err := a.emailRepo.UpdateAccount(ctx, account); err != nil {
		slog.ErrorContext(ctx, "failed to update account last_synced_at", "error", err, "account_id", accountID)
	}

	slog.Info("incremental email sync complete", "account_id", accountID, "messages_processed", processed)

	return &EmailSyncResult{MessagesProcessed: processed}, nil
}

func (a *EmailSyncActivities) storeMessage(ctx context.Context, account *model.CRMEmailAccount, msg *sync.GmailMessage) error {
	// Check if already stored.
	existing, _ := a.emailRepo.GetMessageByExternalID(ctx, account.ID, msg.ID)
	if existing != nil {
		return nil // Already stored
	}

	// Determine direction.
	direction := model.CRMEmailDirectionInbound
	if strings.EqualFold(msg.From, account.EmailAddress) {
		direction = model.CRMEmailDirectionOutbound
	}

	// Find or create thread.
	var threadID *string
	if msg.ThreadID != "" {
		thread, _ := a.emailRepo.GetThreadByExternalID(ctx, account.ID, msg.ThreadID)
		if thread == nil {
			thread = &model.CRMEmailThread{
				WorkspaceID:      account.WorkspaceID,
				EmailAccountID:   account.ID,
				ThreadExternalID: msg.ThreadID,
				Subject:          msg.Subject,
				LastMessageAt:    msg.Date,
				MessageCount:     0,
			}
			if err := a.emailRepo.CreateThread(ctx, thread); err != nil {
				return fmt.Errorf("create thread: %w", err)
			}
		}
		threadID = &thread.ID
		// Update thread stats.
		if err := a.emailRepo.IncrementThreadMessageCount(ctx, thread.ID, msg.Date); err != nil {
			slog.ErrorContext(ctx, "failed to increment thread count", "error", err, "thread_id", thread.ID)
		}
	}

	// Build to/cc as JSONB.
	toJSON, _ := json.Marshal(msg.To)
	ccJSON, _ := json.Marshal(msg.CC)

	var fromNamePtr *string
	if msg.FromName != "" {
		fromNamePtr = &msg.FromName
	}

	var bodyText, bodyHTML *string
	if msg.BodyText != "" {
		bodyText = &msg.BodyText
	}
	if msg.BodyHTML != "" {
		bodyHTML = &msg.BodyHTML
	}

	message := &model.CRMEmailMessage{
		WorkspaceID:       account.WorkspaceID,
		EmailAccountID:    account.ID,
		ThreadID:          threadID,
		MessageExternalID: msg.ID,
		FromAddress:       msg.From,
		FromName:          fromNamePtr,
		ToAddresses:       json.RawMessage(toJSON),
		CCAddresses:       json.RawMessage(ccJSON),
		Subject:           msg.Subject,
		BodyText:          bodyText,
		BodyHTML:          bodyHTML,
		Direction:         direction,
		SentAt:            msg.Date,
	}

	// Auto-match contact by email.
	contactEmail := msg.From
	if direction == model.CRMEmailDirectionOutbound && len(msg.To) > 0 {
		contactEmail = msg.To[0]
	}
	contact := matchContactByEmail(ctx, a.contactRepo, account.WorkspaceID, contactEmail)
	if contact != nil {
		message.ContactID = &contact.ID
	}

	return a.emailRepo.CreateMessage(ctx, message)
}


// matchContactByEmail finds a contact matching the given email address.
func matchContactByEmail(ctx context.Context, contactRepo *repository.CRMContactRepository, workspaceID, emailAddr string) *model.CRMContact {
	search := strings.TrimSpace(emailAddr)
	if search == "" {
		return nil
	}
	contacts, _, err := contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{
		Search: &search,
	}, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil || len(contacts) == 0 {
		return nil
	}
	// Only match if the email matches exactly.
	if contacts[0].Email != nil && strings.EqualFold(*contacts[0].Email, search) {
		return &contacts[0]
	}
	return nil
}
