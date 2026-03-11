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
	gmailClient      *sync.GmailSyncClient
	emailRepo        *repository.CRMEmailRepository
	contactRepo      *repository.CRMContactRepository
	calendarRepo     *repository.CRMCalendarRepository
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository
}

// NewEmailSyncActivities creates email sync activities.
func NewEmailSyncActivities(
	gmailClient *sync.GmailSyncClient,
	emailRepo *repository.CRMEmailRepository,
	contactRepo *repository.CRMContactRepository,
	calendarRepo *repository.CRMCalendarRepository,
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository,
) *EmailSyncActivities {
	return &EmailSyncActivities{
		gmailClient:      gmailClient,
		emailRepo:        emailRepo,
		contactRepo:      contactRepo,
		calendarRepo:     calendarRepo,
		syncSettingsRepo: syncSettingsRepo,
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

	// Load sync settings for filtering.
	settings, err := a.loadSyncSettings(ctx, account.WorkspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load sync settings, using defaults", "error", err)
		defaults := model.DefaultEmailSyncSettings()
		settings = &defaults
	}

	// Fetch messages using configured historical sync days.
	syncDays := settings.HistoricalSyncDays
	if syncDays <= 0 {
		syncDays = 90
	}
	query := fmt.Sprintf("after:%d", time.Now().AddDate(0, 0, -syncDays).Unix())
	processed := 0
	pageToken := ""

	for {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("processed %d messages", processed))

		messages, nextPage, err := a.gmailClient.ListMessages(ctx, accessToken, query, 100, pageToken)
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}

		for i := range messages {
			if err := a.storeMessage(ctx, account, &messages[i], settings); err != nil {
				slog.ErrorContext(ctx, "failed to store email", "error", err, "message_id", messages[i].ID)
				continue
			}
			processed++
			if processed%10 == 0 {
				activity.RecordHeartbeat(ctx, fmt.Sprintf("processed %d messages", processed))
			}
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

	// Load sync settings once for the entire sync cycle.
	settings, err := a.loadSyncSettings(ctx, account.WorkspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load sync settings, using defaults", "error", err)
		defaults := model.DefaultEmailSyncSettings()
		settings = &defaults
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
		if err := a.storeMessage(ctx, account, &messages[i], settings); err != nil {
			slog.ErrorContext(ctx, "failed to store email", "error", err, "message_id", messages[i].ID)
			continue
		}
		processed++
		if processed%10 == 0 {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("incremental: processed %d messages", processed))
		}
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

// loadSyncSettings loads sync settings for a workspace, returning defaults if not found.
func (a *EmailSyncActivities) loadSyncSettings(ctx context.Context, workspaceID string) (*model.CRMEmailSyncSettings, error) {
	if a.syncSettingsRepo == nil {
		defaults := model.DefaultEmailSyncSettings()
		defaults.WorkspaceID = workspaceID
		return &defaults, nil
	}
	settings, err := a.syncSettingsRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		defaults := model.DefaultEmailSyncSettings()
		defaults.WorkspaceID = workspaceID
		return &defaults, nil
	}
	return settings, nil
}

func (a *EmailSyncActivities) storeMessage(ctx context.Context, account *model.CRMEmailAccount, msg *sync.GmailMessage, settings *model.CRMEmailSyncSettings) error {
	// Check if already stored.
	existing, _ := a.emailRepo.GetMessageByExternalID(ctx, account.ID, msg.ID)
	if existing != nil {
		return nil // Already stored
	}

	// Apply email filtering — check sender address against filter patterns.
	if settings != nil && model.ShouldFilterEmail(settings, msg.From) {
		return nil // Filtered out
	}

	// Check internal exclusion — skip if all participants share the same domain.
	if settings != nil && model.IsInternalEmail(settings, msg.From, msg.To, account.EmailAddress) {
		return nil // Internal email excluded
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
	contactName := msg.FromName
	if direction == model.CRMEmailDirectionOutbound && len(msg.To) > 0 {
		contactEmail = msg.To[0]
		// Use recipient's display name from the To header, not the sender's name.
		contactName = ""
		if msg.ToNames != nil {
			contactName = msg.ToNames[strings.ToLower(msg.To[0])]
		}
	}

	// Never create a contact for the connected account's own email.
	isSelf := strings.EqualFold(contactEmail, account.EmailAddress)

	contact := matchContactByEmail(ctx, a.contactRepo, account.WorkspaceID, contactEmail)
	if contact != nil {
		message.ContactID = &contact.ID
	}

	// Auto-create contact if record creation is enabled and no existing contact matched.
	if contact == nil && settings != nil && !isSelf {
		contact = a.maybeCreateContactFromEmail(ctx, account.WorkspaceID, contactEmail, contactName, direction, settings)
		if contact != nil {
			message.ContactID = &contact.ID
		}
	}

	return a.emailRepo.CreateMessage(ctx, message)
}

// maybeCreateContactFromEmail creates a CRM contact from an email based on record creation settings.
func (a *EmailSyncActivities) maybeCreateContactFromEmail(ctx context.Context, workspaceID, emailAddr, name, direction string, settings *model.CRMEmailSyncSettings) *model.CRMContact {
	if settings.RecordCreationMode == "disabled" {
		return nil
	}

	// Selective mode: only create for outbound emails.
	if settings.RecordCreationMode == "selective" && direction != model.CRMEmailDirectionOutbound {
		return nil
	}

	// Check if the email prefix is blocked from record creation.
	if model.IsBlockedRecordPrefix(settings, emailAddr) {
		return nil
	}

	emailAddr = strings.TrimSpace(emailAddr)
	if emailAddr == "" {
		return nil
	}

	// Parse name into first/last.
	firstName := emailAddr // fallback to email as name
	var lastName *string
	if name != "" {
		parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = &parts[1]
		}
	}

	source := "email_sync"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		Email:          &emailAddr,
		FirstName:      firstName,
		LastName:       lastName,
		LifecycleStage: "subscriber",
		Source:         &source,
	}

	if err := a.contactRepo.Create(ctx, contact); err != nil {
		slog.ErrorContext(ctx, "failed to auto-create contact from email", "error", err, "email", emailAddr)
		return nil
	}

	slog.InfoContext(ctx, "auto-created contact from email sync", "contact_id", contact.ID, "email", emailAddr)
	return contact
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
