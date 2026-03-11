package temporalapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
)

type gmailHistorySyncClient interface {
	GetValidToken(ctx context.Context, account *model.CRMEmailAccount) (string, error)
	ListMessages(ctx context.Context, accessToken, query string, maxResults int, pageToken string) ([]sync.GmailMessage, string, error)
	GetMessageDetail(ctx context.Context, accessToken, messageID string) (*sync.GmailMessage, error)
	GetMailboxProfile(ctx context.Context, accessToken string) (*sync.GmailProfile, error)
	ListHistory(ctx context.Context, accessToken, startHistoryID string) (*sync.GmailHistoryResult, error)
}

// EmailSyncActivities contains activities for email synchronization.
type EmailSyncActivities struct {
	gmailClient      gmailHistorySyncClient
	emailRepo        *repository.CRMEmailRepository
	calendarRepo     *repository.CRMCalendarRepository
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository
	resolver         *crmemail.Resolver
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
		calendarRepo:     calendarRepo,
		syncSettingsRepo: syncSettingsRepo,
		resolver:         crmemail.NewResolver(contactRepo),
	}
}

// BackfillEmailsActivity fetches last 90 days of emails.
func (a *EmailSyncActivities) BackfillEmailsActivity(ctx context.Context, accountID string) (*EmailSyncResult, error) {
	account, err := a.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}
	if !account.IsActive {
		return &EmailSyncResult{}, nil
	}
	if a.gmailClient == nil {
		return nil, fmt.Errorf("gmail sync client not configured")
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

	if account.LastHistoryID != nil && strings.TrimSpace(*account.LastHistoryID) != "" {
		if err := a.persistAccountCheckpoint(ctx, account, nil, strings.TrimSpace(*account.LastHistoryID)); err != nil {
			slog.ErrorContext(ctx, "failed to persist existing account checkpoint", "error", err, "account_id", accountID)
		}
		return &EmailSyncResult{NewHistoryID: strings.TrimSpace(*account.LastHistoryID)}, nil
	}

	startTime := historicalSyncStart(settings)
	if account.LastSyncedAt != nil && account.LastSyncedAt.After(startTime) {
		startTime = *account.LastSyncedAt
	}

	processed, err := a.backfillMessagesSince(ctx, account, accessToken, settings, startTime)
	if err != nil {
		return nil, err
	}

	profile, err := a.gmailClient.GetMailboxProfile(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("get mailbox profile: %w", err)
	}
	if err := a.persistAccountCheckpoint(ctx, account, profile, profile.HistoryID); err != nil {
		slog.ErrorContext(ctx, "failed to update account checkpoint after backfill", "error", err, "account_id", accountID)
	}

	slog.Info("backfill email sync complete", "account_id", accountID, "messages_processed", processed)

	return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: strings.TrimSpace(profile.HistoryID)}, nil
}

// IncrementalSyncActivity fetches new emails since last sync.
func (a *EmailSyncActivities) IncrementalSyncActivity(ctx context.Context, accountID string) (*EmailSyncResult, error) {
	account, err := a.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}
	if !account.IsActive {
		return &EmailSyncResult{}, nil
	}
	if a.gmailClient == nil {
		return nil, fmt.Errorf("gmail sync client not configured")
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

	lastHistoryID := strings.TrimSpace(stringValue(account.LastHistoryID))
	if lastHistoryID == "" {
		processed, newHistoryID, recoverErr := a.recoverHistoryCheckpoint(ctx, account, accessToken, settings)
		return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: newHistoryID}, recoverErr
	}

	history, err := a.gmailClient.ListHistory(ctx, accessToken, lastHistoryID)
	if err != nil {
		if historyCursorExpired(err) {
			processed, newHistoryID, recoverErr := a.recoverHistoryCheckpoint(ctx, account, accessToken, settings)
			return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: newHistoryID}, recoverErr
		}
		return nil, fmt.Errorf("list gmail history: %w", err)
	}

	processed := 0
	for _, messageID := range history.MessageIDs {
		msg, err := a.gmailClient.GetMessageDetail(ctx, accessToken, messageID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get changed gmail message detail", "error", err, "message_id", messageID)
			continue
		}
		if err := a.storeMessage(ctx, account, msg, settings); err != nil {
			slog.ErrorContext(ctx, "failed to store incremental email", "error", err, "message_id", messageID)
			continue
		}
		processed++
		if processed%10 == 0 {
			recordHeartbeat(ctx, fmt.Sprintf("incremental: processed %d messages", processed))
		}
	}

	newHistoryID := strings.TrimSpace(history.LatestHistoryID)
	if newHistoryID == "" {
		newHistoryID = lastHistoryID
	}
	if err := a.persistAccountCheckpoint(ctx, account, nil, newHistoryID); err != nil {
		slog.ErrorContext(ctx, "failed to update account incremental checkpoint", "error", err, "account_id", accountID)
	}

	slog.Info("incremental email sync complete", "account_id", accountID, "messages_processed", processed, "history_id", newHistoryID)

	return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: newHistoryID}, nil
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

func (a *EmailSyncActivities) backfillMessagesSince(ctx context.Context, account *model.CRMEmailAccount, accessToken string, settings *model.CRMEmailSyncSettings, start time.Time) (int, error) {
	query := fmt.Sprintf("after:%d", start.Unix())
	processed := 0
	pageToken := ""

	for {
		recordHeartbeat(ctx, fmt.Sprintf("processed %d messages", processed))

		messages, nextPage, err := a.gmailClient.ListMessages(ctx, accessToken, query, 100, pageToken)
		if err != nil {
			return processed, fmt.Errorf("list messages: %w", err)
		}

		for i := range messages {
			if err := a.storeMessage(ctx, account, &messages[i], settings); err != nil {
				slog.ErrorContext(ctx, "failed to store email during backfill", "error", err, "message_id", messages[i].ID)
				continue
			}
			processed++
			if processed%10 == 0 {
				recordHeartbeat(ctx, fmt.Sprintf("processed %d messages", processed))
			}
		}

		if nextPage == "" {
			break
		}
		pageToken = nextPage
	}

	return processed, nil
}

func (a *EmailSyncActivities) recoverHistoryCheckpoint(ctx context.Context, account *model.CRMEmailAccount, accessToken string, settings *model.CRMEmailSyncSettings) (int, string, error) {
	start := historicalSyncStart(settings)
	if account.LastSyncedAt != nil && account.LastSyncedAt.After(start) {
		start = *account.LastSyncedAt
	}

	processed, err := a.backfillMessagesSince(ctx, account, accessToken, settings, start)
	if err != nil {
		return processed, "", err
	}

	profile, err := a.gmailClient.GetMailboxProfile(ctx, accessToken)
	if err != nil {
		return processed, "", fmt.Errorf("get mailbox profile after history recovery: %w", err)
	}
	newHistoryID := strings.TrimSpace(profile.HistoryID)
	if err := a.persistAccountCheckpoint(ctx, account, profile, newHistoryID); err != nil {
		slog.ErrorContext(ctx, "failed to persist recovered account checkpoint", "error", err, "account_id", account.ID)
	}
	return processed, newHistoryID, nil
}

func (a *EmailSyncActivities) persistAccountCheckpoint(ctx context.Context, account *model.CRMEmailAccount, profile *sync.GmailProfile, historyID string) error {
	now := time.Now().UTC()
	account.LastSyncedAt = &now
	account.IsActive = true
	account.Status = model.CRMEmailAccountStatusConnected
	account.DisconnectedAt = nil
	if profile != nil {
		if normalized := crmemail.NormalizeEmailAddress(profile.EmailAddress); normalized != "" {
			account.EmailAddress = profile.EmailAddress
			account.NormalizedEmailAddress = stringPtr(normalized)
		}
	}
	historyID = strings.TrimSpace(historyID)
	if historyID != "" {
		account.LastHistoryID = stringPtr(historyID)
	}
	account.SyncState = withSyncStatus(account.SyncState, model.CRMEmailAccountStatusConnected)
	if account.LastHistoryID != nil && *account.LastHistoryID != "" {
		account.SyncState["last_history_id"] = *account.LastHistoryID
	}
	return a.emailRepo.UpdateAccount(ctx, account)
}

func historicalSyncStart(settings *model.CRMEmailSyncSettings) time.Time {
	syncDays := 90
	if settings != nil && settings.HistoricalSyncDays > 0 {
		syncDays = settings.HistoricalSyncDays
	}
	return time.Now().AddDate(0, 0, -syncDays)
}

func historyCursorExpired(err error) bool {
	var apiErr *sync.GmailAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode == 404 {
		return true
	}
	if apiErr.StatusCode != 400 {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(body, "history") || strings.Contains(body, "stale") || strings.Contains(body, "too old")
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func withSyncStatus(syncState model.JSONB, status string) model.JSONB {
	if syncState == nil {
		syncState = model.JSONB{}
	}
	syncState["status"] = status
	return syncState
}

func recordHeartbeat(ctx context.Context, details string) {
	defer func() {
		_ = recover()
	}()
	activity.RecordHeartbeat(ctx, details)
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
	if settings != nil && model.IsInternalEmail(settings, msg.From, msg.To, msg.CC, account.EmailAddress) {
		return nil // Internal email excluded
	}

	// Determine direction.
	direction := model.CRMEmailDirectionInbound
	if hasLabel(msg.LabelIDs, "SENT") {
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

	resolution, err := a.resolver.Resolve(ctx, crmemail.ResolveInput{
		WorkspaceID: account.WorkspaceID,
		Direction:   direction,
		Settings:    settings,
		SelfEmails:  []string{account.EmailAddress},
		From: crmemail.Participant{
			Email: msg.From,
			Name:  msg.FromName,
			Role:  model.CRMEmailParticipantRoleFrom,
		},
		To: participantsFromNamedAddresses(msg.To, msg.ToNames, model.CRMEmailParticipantRoleTo),
		CC: participantsFromNamedAddresses(msg.CC, msg.CCNames, model.CRMEmailParticipantRoleCC),
	})
	if err != nil {
		return fmt.Errorf("resolve message participants: %w", err)
	}

	toJSON, _ := json.Marshal(participantEmails(resolution.To))
	ccJSON, _ := json.Marshal(participantEmails(resolution.CC))

	var fromNamePtr *string
	if resolution.From.Name != "" {
		fromNamePtr = &resolution.From.Name
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
		FromAddress:       resolution.From.Email,
		FromName:          fromNamePtr,
		ToAddresses:       json.RawMessage(toJSON),
		CCAddresses:       json.RawMessage(ccJSON),
		Subject:           msg.Subject,
		BodyText:          bodyText,
		BodyHTML:          bodyHTML,
		Direction:         direction,
		SentAt:            msg.Date,
		ContactID:         resolution.PrimaryContactID,
		ContactIDs:        resolution.ContactIDs,
	}

	if err := a.emailRepo.CreateMessage(ctx, message); err != nil {
		return err
	}

	associations := cloneAssociationsForMessage(message.ID, resolution.Associations)
	if err := a.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, associations); err != nil {
		return fmt.Errorf("replace message contacts: %w", err)
	}
	if threadID != nil {
		if err := a.emailRepo.RefreshThreadContactIDs(ctx, *threadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh thread contacts", "error", err, "thread_id", *threadID)
		}
	}
	return nil
}

func participantsFromNamedAddresses(addresses []string, names map[string]string, role string) []crmemail.Participant {
	result := make([]crmemail.Participant, 0, len(addresses))
	for _, address := range addresses {
		normalized := crmemail.NormalizeEmailAddress(address)
		if normalized == "" {
			continue
		}
		result = append(result, crmemail.Participant{
			Email: normalized,
			Name:  strings.TrimSpace(names[normalized]),
			Role:  role,
		})
	}
	return result
}

func participantEmails(participants []crmemail.Participant) []string {
	result := make([]string, 0, len(participants))
	for _, participant := range participants {
		if participant.Email == "" {
			continue
		}
		result = append(result, participant.Email)
	}
	return result
}

func cloneAssociationsForMessage(messageID string, associations []model.CRMEmailMessageContact) []model.CRMEmailMessageContact {
	if len(associations) == 0 {
		return nil
	}
	result := make([]model.CRMEmailMessageContact, 0, len(associations))
	for _, association := range associations {
		association.MessageID = messageID
		result = append(result, association)
	}
	return result
}

func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if strings.EqualFold(label, target) {
			return true
		}
	}
	return false
}
