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
	tclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crmsignal"
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
	signalIngestion  *crmsignal.IngestionService
	summaryRefresh   interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
	}
}

// NewEmailSyncActivities creates email sync activities.
func NewEmailSyncActivities(
	gmailClient *sync.GmailSyncClient,
	emailRepo *repository.CRMEmailRepository,
	contactRepo *repository.CRMContactRepository,
	calendarRepo *repository.CRMCalendarRepository,
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository,
	temporalClient tclient.Client,
	summaryRefresh interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
	},
) *EmailSyncActivities {
	return &EmailSyncActivities{
		gmailClient:      gmailClient,
		emailRepo:        emailRepo,
		calendarRepo:     calendarRepo,
		syncSettingsRepo: syncSettingsRepo,
		resolver:         crmemail.NewResolver(contactRepo),
		signalIngestion:  crmsignal.NewIngestionService(emailRepo, crmsignal.NewTemporalStarter(temporalClient, QueueAutomation)),
		summaryRefresh:   summaryRefresh,
	}
}

// BackfillEmailsActivity fetches last 90 days of emails.
func (a *EmailSyncActivities) BackfillEmailsActivity(ctx context.Context, accountID string) (*EmailSyncResult, error) {
	account, err := a.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("account not found: %s", accountID),
			"ACCOUNT_NOT_FOUND",
			fmt.Errorf("account not found: %s", accountID),
		)
	}
	if !account.IsActive {
		return &EmailSyncResult{}, nil
	}
	if a.gmailClient == nil {
		return nil, fmt.Errorf("gmail sync client not configured")
	}

	startedAt := time.Now().UTC()
	cycle := newSyncCycleAccumulator("backfill", startedAt)
	if err := a.markSyncCycleStart(ctx, account, crmemail.SyncPhaseBackfill, strings.TrimSpace(stringValue(account.LastHistoryID)), startedAt); err != nil {
		slog.WarnContext(ctx, "failed to persist crm email backfill start", "error", err, "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)
	}

	accessToken, err := a.gmailClient.GetValidToken(ctx, account)
	if err != nil {
		a.recordSyncFailure(ctx, account, "get_valid_token", cycle.stats(), err)
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
		if err := a.persistAccountCheckpoint(ctx, account, nil, strings.TrimSpace(*account.LastHistoryID), cycle.stats()); err != nil {
			a.recordSyncFailure(ctx, account, "persist_checkpoint", cycle.stats(), err)
			return nil, fmt.Errorf("persist existing account checkpoint: %w", err)
		}
		return &EmailSyncResult{NewHistoryID: strings.TrimSpace(*account.LastHistoryID)}, nil
	}

	startTime := historicalSyncStart(settings)
	if account.LastSyncedAt != nil && account.LastSyncedAt.After(startTime) {
		startTime = *account.LastSyncedAt
	}

	processed, err := a.backfillMessagesSince(ctx, account, accessToken, settings, startTime, cycle)
	if err != nil {
		a.recordSyncFailure(ctx, account, "backfill_messages", cycle.stats(), err)
		return nil, err
	}

	profile, err := a.gmailClient.GetMailboxProfile(ctx, accessToken)
	if err != nil {
		a.recordSyncFailure(ctx, account, "get_mailbox_profile", cycle.stats(), err)
		return nil, fmt.Errorf("get mailbox profile: %w", err)
	}
	if err := a.persistAccountCheckpoint(ctx, account, profile, profile.HistoryID, cycle.stats()); err != nil {
		a.recordSyncFailure(ctx, account, "persist_checkpoint", cycle.stats(), err)
		return nil, fmt.Errorf("update account checkpoint after backfill: %w", err)
	}

	stats := cycle.stats()
	slog.InfoContext(ctx, "crm email backfill sync complete", "workspace_id", account.WorkspaceID, "account_id", accountID, "provider", account.Provider, "mode", stats.Mode, "phase", crmemail.SyncPhaseBackfill, "history_id", strings.TrimSpace(profile.HistoryID), "messages_seen", stats.MessagesSeen, "messages_stored", stats.MessagesStored, "duplicates_skipped", stats.DuplicatesSkipped, "filtered_skipped", stats.FilteredSkipped, "internal_skipped", stats.InternalSkipped, "contacts_created", stats.ContactsCreated, "associations_written", stats.AssociationsWritten)

	return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: strings.TrimSpace(profile.HistoryID)}, nil
}

// IncrementalSyncActivity fetches new emails since last sync.
func (a *EmailSyncActivities) IncrementalSyncActivity(ctx context.Context, accountID string) (*EmailSyncResult, error) {
	account, err := a.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("account not found: %s", accountID),
			"ACCOUNT_NOT_FOUND",
			fmt.Errorf("account not found: %s", accountID),
		)
	}
	if !account.IsActive {
		return &EmailSyncResult{}, nil
	}
	if a.gmailClient == nil {
		return nil, fmt.Errorf("gmail sync client not configured")
	}

	startedAt := time.Now().UTC()
	cycle := newSyncCycleAccumulator("incremental", startedAt)
	if err := a.markSyncCycleStart(ctx, account, crmemail.SyncPhaseIncremental, strings.TrimSpace(stringValue(account.LastHistoryID)), startedAt); err != nil {
		slog.WarnContext(ctx, "failed to persist crm email incremental start", "error", err, "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)
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
		a.recordSyncFailure(ctx, account, "get_valid_token", cycle.stats(), err)
		return nil, fmt.Errorf("get valid token: %w", err)
	}

	lastHistoryID := strings.TrimSpace(stringValue(account.LastHistoryID))
	if lastHistoryID == "" {
		cycle.RecoveryTriggered = true
		processed, newHistoryID, recoverErr := a.recoverHistoryCheckpoint(ctx, account, accessToken, settings, cycle)
		if recoverErr != nil {
			a.recordSyncFailure(ctx, account, "recover_history_checkpoint", cycle.stats(), recoverErr)
		}
		return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: newHistoryID}, recoverErr
	}

	history, err := a.gmailClient.ListHistory(ctx, accessToken, lastHistoryID)
	if err != nil {
		if historyCursorExpired(err) {
			cycle.RecoveryTriggered = true
			processed, newHistoryID, recoverErr := a.recoverHistoryCheckpoint(ctx, account, accessToken, settings, cycle)
			if recoverErr != nil {
				a.recordSyncFailure(ctx, account, "recover_history_checkpoint", cycle.stats(), recoverErr)
			}
			return &EmailSyncResult{MessagesProcessed: processed, NewHistoryID: newHistoryID}, recoverErr
		}
		a.recordSyncFailure(ctx, account, "list_history", cycle.stats(), err)
		return nil, fmt.Errorf("list gmail history: %w", err)
	}

	processed := 0
	for _, messageID := range history.MessageIDs {
		cycle.MessagesSeen++
		msg, err := a.gmailClient.GetMessageDetail(ctx, accessToken, messageID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get changed gmail message detail", "error", err, "message_id", messageID)
			continue
		}
		result, err := a.storeMessage(ctx, account, msg, settings)
		if err != nil {
			slog.ErrorContext(ctx, "failed to store incremental email", "error", err, "message_id", messageID)
			continue
		}
		cycle.addResult(result)
		if result.Stored {
			processed++
		}
		if processed%10 == 0 {
			recordHeartbeat(ctx, fmt.Sprintf("incremental: processed %d messages", processed))
		}
	}

	newHistoryID := strings.TrimSpace(history.LatestHistoryID)
	if newHistoryID == "" {
		newHistoryID = lastHistoryID
	}
	if err := a.persistAccountCheckpoint(ctx, account, nil, newHistoryID, cycle.stats()); err != nil {
		a.recordSyncFailure(ctx, account, "persist_checkpoint", cycle.stats(), err)
		return nil, fmt.Errorf("update account incremental checkpoint: %w", err)
	}

	stats := cycle.stats()
	slog.InfoContext(ctx, "crm email incremental sync complete", "workspace_id", account.WorkspaceID, "account_id", accountID, "provider", account.Provider, "mode", stats.Mode, "phase", crmemail.SyncPhaseIncremental, "history_id", newHistoryID, "messages_seen", stats.MessagesSeen, "messages_stored", stats.MessagesStored, "duplicates_skipped", stats.DuplicatesSkipped, "filtered_skipped", stats.FilteredSkipped, "internal_skipped", stats.InternalSkipped, "contacts_created", stats.ContactsCreated, "associations_written", stats.AssociationsWritten)

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

func (a *EmailSyncActivities) backfillMessagesSince(ctx context.Context, account *model.CRMEmailAccount, accessToken string, settings *model.CRMEmailSyncSettings, start time.Time, cycle *syncCycleAccumulator) (int, error) {
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
			cycle.MessagesSeen++
			result, err := a.storeMessage(ctx, account, &messages[i], settings)
			if err != nil {
				slog.ErrorContext(ctx, "failed to store email during backfill", "error", err, "message_id", messages[i].ID)
				continue
			}
			cycle.addResult(result)
			if result.Stored {
				processed++
			}
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

func (a *EmailSyncActivities) recoverHistoryCheckpoint(ctx context.Context, account *model.CRMEmailAccount, accessToken string, settings *model.CRMEmailSyncSettings, cycle *syncCycleAccumulator) (int, string, error) {
	cycle.Mode = "recovery"
	if err := a.markSyncCycleStart(ctx, account, crmemail.SyncPhaseRecovery, strings.TrimSpace(stringValue(account.LastHistoryID)), cycle.StartedAt); err != nil {
		slog.WarnContext(ctx, "failed to persist crm email recovery start", "error", err, "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)
	}
	start := historicalSyncStart(settings)
	if account.LastSyncedAt != nil && account.LastSyncedAt.After(start) {
		start = *account.LastSyncedAt
	}

	processed, err := a.backfillMessagesSince(ctx, account, accessToken, settings, start, cycle)
	if err != nil {
		return processed, "", err
	}

	profile, err := a.gmailClient.GetMailboxProfile(ctx, accessToken)
	if err != nil {
		return processed, "", fmt.Errorf("get mailbox profile after history recovery: %w", err)
	}
	newHistoryID := strings.TrimSpace(profile.HistoryID)
	if err := a.persistAccountCheckpoint(ctx, account, profile, newHistoryID, cycle.stats()); err != nil {
		return processed, "", fmt.Errorf("persist recovered account checkpoint: %w", err)
	}
	stats := cycle.stats()
	slog.InfoContext(ctx, "crm email recovery sync complete", "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider, "mode", stats.Mode, "phase", crmemail.SyncPhaseRecovery, "history_id", newHistoryID, "messages_seen", stats.MessagesSeen, "messages_stored", stats.MessagesStored, "duplicates_skipped", stats.DuplicatesSkipped, "filtered_skipped", stats.FilteredSkipped, "internal_skipped", stats.InternalSkipped, "contacts_created", stats.ContactsCreated, "associations_written", stats.AssociationsWritten)
	return processed, newHistoryID, nil
}

func (a *EmailSyncActivities) persistAccountCheckpoint(ctx context.Context, account *model.CRMEmailAccount, profile *sync.GmailProfile, historyID string, cycle model.CRMEmailSyncCycleStats) error {
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
	account.SyncState = crmemail.CompleteSyncCycle(account.SyncState, stringValue(account.LastHistoryID), now, cycle)
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

func recordHeartbeat(ctx context.Context, details string) {
	defer func() {
		_ = recover()
	}()
	activity.RecordHeartbeat(ctx, details)
}

func (a *EmailSyncActivities) storeMessage(ctx context.Context, account *model.CRMEmailAccount, msg *sync.GmailMessage, settings *model.CRMEmailSyncSettings) (storeMessageResult, error) {
	// Check if already stored.
	existing, _ := a.emailRepo.GetMessageByExternalID(ctx, account.ID, msg.ID)
	if existing != nil {
		return storeMessageResult{Duplicate: true}, nil
	}

	// Apply email filtering — check sender address against filter patterns.
	if settings != nil && model.ShouldFilterEmail(settings, msg.From) {
		return storeMessageResult{Filtered: true}, nil
	}

	// Check internal exclusion — skip if all participants share the same domain.
	if settings != nil && model.IsInternalEmail(settings, msg.From, msg.To, msg.CC, account.EmailAddress) {
		return storeMessageResult{Internal: true}, nil
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
				return storeMessageResult{}, fmt.Errorf("create thread: %w", err)
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
		return storeMessageResult{}, fmt.Errorf("resolve message participants: %w", err)
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
		return storeMessageResult{}, err
	}

	associations := cloneAssociationsForMessage(message.ID, resolution.Associations)
	if err := a.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, associations); err != nil {
		return storeMessageResult{}, fmt.Errorf("replace message contacts: %w", err)
	}
	threadTouched := ""
	if threadID != nil {
		if err := a.emailRepo.RefreshThreadContactIDs(ctx, *threadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh thread contacts", "error", err, "thread_id", *threadID)
		}
		threadTouched = *threadID
	}
	if a.signalIngestion != nil {
		if _, err := a.signalIngestion.EnqueueEmailMessage(ctx, message.ID); err != nil {
			slog.ErrorContext(ctx, "failed to enqueue crm buyer signal detection from email sync", "error", err, "workspace_id", account.WorkspaceID, "account_id", account.ID, "message_id", message.ID)
		}
	}
	a.requestSummaryRefreshForMessage(ctx, message, "gmail_sync")
	return storeMessageResult{
		Stored:              true,
		ContactsCreated:     resolution.ContactsCreated,
		AssociationsWritten: len(associations),
		ThreadID:            threadTouched,
	}, nil
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

func (a *EmailSyncActivities) requestSummaryRefreshForMessage(ctx context.Context, message *model.CRMEmailMessage, source string) {
	if a == nil || a.summaryRefresh == nil || message == nil {
		return
	}

	if message.DealID != nil && *message.DealID != "" {
		if err := a.summaryRefresh.RequestDealRefresh(ctx, message.WorkspaceID, *message.DealID); err != nil {
			slog.ErrorContext(ctx, "failed to request deal summary refresh from email sync message", "error", err, "workspace_id", message.WorkspaceID, "deal_id", *message.DealID, "message_id", message.ID, "source", source)
		}
	}

	if len(message.ContactIDs) == 1 {
		if err := a.summaryRefresh.RequestContactRefresh(ctx, message.WorkspaceID, message.ContactIDs[0]); err != nil {
			slog.ErrorContext(ctx, "failed to request contact summary refresh from email sync message", "error", err, "workspace_id", message.WorkspaceID, "contact_id", message.ContactIDs[0], "message_id", message.ID, "source", source)
		}
		return
	}

	if message.DealID == nil && len(message.ContactIDs) > 1 {
		slog.InfoContext(ctx, "skipping contact summary refresh for multi-contact synced email without deal", "workspace_id", message.WorkspaceID, "message_id", message.ID, "contact_count", len(message.ContactIDs), "source", source)
	}
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

type storeMessageResult struct {
	Stored              bool
	Duplicate           bool
	Filtered            bool
	Internal            bool
	ContactsCreated     int
	AssociationsWritten int
	ThreadID            string
}

type syncCycleAccumulator struct {
	Mode                string
	StartedAt           time.Time
	MessagesSeen        int
	MessagesStored      int
	DuplicatesSkipped   int
	FilteredSkipped     int
	InternalSkipped     int
	ContactsCreated     int
	AssociationsWritten int
	ThreadIDs           map[string]struct{}
	RecoveryTriggered   bool
}

func newSyncCycleAccumulator(mode string, startedAt time.Time) *syncCycleAccumulator {
	return &syncCycleAccumulator{
		Mode:      mode,
		StartedAt: startedAt.UTC(),
		ThreadIDs: map[string]struct{}{},
	}
}

func (c *syncCycleAccumulator) addResult(result storeMessageResult) {
	if result.Stored {
		c.MessagesStored++
	}
	if result.Duplicate {
		c.DuplicatesSkipped++
	}
	if result.Filtered {
		c.FilteredSkipped++
	}
	if result.Internal {
		c.InternalSkipped++
	}
	c.ContactsCreated += result.ContactsCreated
	c.AssociationsWritten += result.AssociationsWritten
	if result.ThreadID != "" {
		c.ThreadIDs[result.ThreadID] = struct{}{}
	}
}

func (c *syncCycleAccumulator) stats() model.CRMEmailSyncCycleStats {
	startedAt := c.StartedAt.UTC()
	return model.CRMEmailSyncCycleStats{
		Mode:                c.Mode,
		StartedAt:           &startedAt,
		MessagesSeen:        c.MessagesSeen,
		MessagesStored:      c.MessagesStored,
		DuplicatesSkipped:   c.DuplicatesSkipped,
		FilteredSkipped:     c.FilteredSkipped,
		InternalSkipped:     c.InternalSkipped,
		ContactsCreated:     c.ContactsCreated,
		AssociationsWritten: c.AssociationsWritten,
		ThreadsTouched:      len(c.ThreadIDs),
		RecoveryTriggered:   c.RecoveryTriggered,
	}
}

func (a *EmailSyncActivities) markSyncCycleStart(ctx context.Context, account *model.CRMEmailAccount, phase, historyID string, startedAt time.Time) error {
	account.SyncState = crmemail.BeginSyncCycle(account.SyncState, phase, historyID, startedAt)
	return a.emailRepo.UpdateSyncState(ctx, account.ID, account.SyncState)
}

func (a *EmailSyncActivities) recordSyncFailure(ctx context.Context, account *model.CRMEmailAccount, operation string, cycle model.CRMEmailSyncCycleStats, syncErr error) {
	account.SyncState = crmemail.FailSyncCycle(account.SyncState, stringValue(account.LastHistoryID), time.Now().UTC(), operation, "sync_error", syncErr.Error(), &cycle)
	if err := a.emailRepo.UpdateSyncState(ctx, account.ID, account.SyncState); err != nil {
		slog.ErrorContext(ctx, "failed to persist crm email sync failure diagnostics", "error", err, "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)
	}
	slog.ErrorContext(ctx, "crm email sync failed", "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider, "mode", cycle.Mode, "phase", crmemail.SyncPhaseError, "history_id", stringValue(account.LastHistoryID), "messages_seen", cycle.MessagesSeen, "messages_stored", cycle.MessagesStored, "duplicates_skipped", cycle.DuplicatesSkipped, "filtered_skipped", cycle.FilteredSkipped, "internal_skipped", cycle.InternalSkipped, "contacts_created", cycle.ContactsCreated, "associations_written", cycle.AssociationsWritten, "error", syncErr.Error())
}
