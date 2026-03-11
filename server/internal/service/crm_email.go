package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
)

type gmailOAuthProvider interface {
	GenerateAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*oauth.TokenPair, error)
}

type gmailMailboxClient interface {
	GetMailboxProfile(ctx context.Context, accessToken string) (*sync.GmailProfile, error)
	GetValidToken(ctx context.Context, account *model.CRMEmailAccount) (string, error)
	SendMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML string) (*sync.GmailSendResult, error)
}

type emailSyncWorkflowRunner interface {
	StartAccountSync(ctx context.Context, accountID string) error
	CancelAccountSync(ctx context.Context, accountID string) error
}

type temporalEmailSyncWorkflowRunner struct {
	client tclient.Client
}

func (r *temporalEmailSyncWorkflowRunner) StartAccountSync(ctx context.Context, accountID string) error {
	if r == nil || r.client == nil || accountID == "" {
		return nil
	}
	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:        emailSyncWorkflowID(accountID),
		TaskQueue: temporalapp.QueueAutomation,
	}, temporalapp.EmailSyncWorkflow, temporalapp.EmailSyncWorkflowInput{
		AccountID: accountID,
	})
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start email sync workflow: %w", err)
	}
	return nil
}

func (r *temporalEmailSyncWorkflowRunner) CancelAccountSync(ctx context.Context, accountID string) error {
	if r == nil || r.client == nil || accountID == "" {
		return nil
	}
	err := r.client.CancelWorkflow(ctx, emailSyncWorkflowID(accountID), "")
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("cancel email sync workflow: %w", err)
	}
	return nil
}

// CRMEmailService contains CRM email business logic.
type CRMEmailService struct {
	emailRepo        *repository.CRMEmailRepository
	contactRepo      *repository.CRMContactRepository
	workspaceRepo    *repository.WorkspaceRepository
	syncSettingsRepo *repository.CRMEmailSyncSettingsRepository
	oauthClient      gmailOAuthProvider
	encryptionKey    []byte
	gmailSync        gmailMailboxClient
	syncRunner       emailSyncWorkflowRunner
	resolver         *crmemail.Resolver
}

// NewCRMEmailService creates a new CRMEmailService.
func NewCRMEmailService(emailRepo *repository.CRMEmailRepository, contactRepo *repository.CRMContactRepository, workspaceRepo *repository.WorkspaceRepository, syncSettingsRepo *repository.CRMEmailSyncSettingsRepository, oauthClient *oauth.GmailOAuthClient, encryptionKey []byte, gmailSync *sync.GmailSyncClient, temporalClient tclient.Client) *CRMEmailService {
	return &CRMEmailService{
		emailRepo:        emailRepo,
		contactRepo:      contactRepo,
		workspaceRepo:    workspaceRepo,
		syncSettingsRepo: syncSettingsRepo,
		oauthClient:      oauthClient,
		encryptionKey:    encryptionKey,
		gmailSync:        gmailSync,
		syncRunner:       &temporalEmailSyncWorkflowRunner{client: temporalClient},
		resolver:         crmemail.NewResolver(contactRepo),
	}
}

// ── Email Accounts ──

// ListAccounts returns email accounts for a workspace.
func (s *CRMEmailService) ListAccounts(ctx context.Context, workspaceID string, filters model.CRMEmailAccountListFilters) ([]model.CRMEmailAccount, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	accounts, err := s.emailRepo.ListAccounts(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}
	if err := s.populateHasSyncedData(ctx, accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetAccount returns an email account by ID.
func (s *CRMEmailService) GetAccount(ctx context.Context, id string) (*model.CRMEmailAccount, error) {
	account, err := s.emailRepo.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("email account not found")
	}
	hasSyncedData, err := s.emailRepo.ListAccountsWithSyncedData(ctx, []string{account.ID})
	if err != nil {
		return nil, err
	}
	account.HasSyncedData = hasSyncedData[account.ID]
	return account, nil
}

// CreateAccount creates a new email account (OAuth stub).
func (s *CRMEmailService) CreateAccount(ctx context.Context, req model.CreateCRMEmailAccountRequest) (*model.CRMEmailAccount, error) {
	if req.WorkspaceID == "" || req.EmailAddress == "" || req.Provider == "" {
		return nil, fmt.Errorf("workspace_id, email_address, and provider are required")
	}
	if req.Provider != model.CRMEmailProviderGmail && req.Provider != model.CRMEmailProviderMicrosoft {
		return nil, fmt.Errorf("provider must be gmail or microsoft")
	}

	account := &model.CRMEmailAccount{
		WorkspaceID:            req.WorkspaceID,
		MemberID:               req.MemberID,
		Provider:               req.Provider,
		EmailAddress:           strings.TrimSpace(req.EmailAddress),
		NormalizedEmailAddress: optionalStringPtr(normalizeMailboxEmail(req.EmailAddress)),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
		SyncState:              model.JSONB{"status": model.CRMEmailAccountStatusConnected},
	}

	if err := s.emailRepo.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

// DeleteAccount disconnects an email account while preserving synced history.
// Only the owner or an admin can disconnect.
func (s *CRMEmailService) DeleteAccount(ctx context.Context, id string, userID string, isAdmin bool) error {
	account, err := s.emailRepo.GetAccountByID(ctx, id)
	if err != nil {
		return err
	}
	if account == nil {
		return fmt.Errorf("email account not found")
	}
	if !isAdmin && account.MemberID != userID {
		return fmt.Errorf("not authorized to disconnect this email account")
	}

	if err := s.cancelEmailSync(ctx, account.ID); err != nil {
		slog.WarnContext(ctx, "failed to cancel email sync during disconnect", "error", err, "account_id", account.ID)
	}

	now := time.Now().UTC()
	account.AccessTokenEncrypted = nil
	account.RefreshTokenEncrypted = nil
	account.TokenExpiresAt = nil
	account.OAuthState = nil
	account.IsActive = false
	account.Status = model.CRMEmailAccountStatusDisconnected
	account.DisconnectedAt = &now
	account.SyncState = withSyncStatus(account.SyncState, model.CRMEmailAccountStatusDisconnected)

	return s.emailRepo.UpdateAccount(ctx, account)
}

// PurgeAccountData permanently removes a synced mailbox and all of its synced email/calendar data.
// Admins only.
func (s *CRMEmailService) PurgeAccountData(ctx context.Context, id string, isAdmin bool) error {
	if !isAdmin {
		return fmt.Errorf("not authorized to purge email account data")
	}
	account, err := s.emailRepo.GetAccountByID(ctx, id)
	if err != nil {
		return err
	}
	if account == nil {
		return fmt.Errorf("email account not found")
	}

	if err := s.cancelEmailSync(ctx, account.ID); err != nil {
		slog.WarnContext(ctx, "failed to cancel email sync during purge", "error", err, "account_id", account.ID)
	}

	return s.emailRepo.DeleteAccount(ctx, id)
}

// OAuthCallback handles the OAuth callback stub (stores tokens placeholder).
func (s *CRMEmailService) OAuthCallback(ctx context.Context, accountID string, code string) error {
	account, err := s.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account == nil {
		return fmt.Errorf("email account not found")
	}
	// Stub: actual OAuth token exchange would happen here.
	return fmt.Errorf("email sync not configured: OAuth token exchange not implemented")
}

// InitiateOAuth starts the Gmail OAuth flow and returns the redirect URL.
func (s *CRMEmailService) InitiateOAuth(ctx context.Context, workspaceID, memberID, provider string) (string, error) {
	if s.oauthClient == nil {
		return "", fmt.Errorf("Gmail OAuth not configured")
	}
	if provider == "" {
		provider = model.CRMEmailProviderGmail
	}
	if provider != model.CRMEmailProviderGmail {
		return "", fmt.Errorf("only gmail provider is currently supported for OAuth")
	}

	// Generate a random state token.
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("generate state token: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Create a pending account.
	account := &model.CRMEmailAccount{
		WorkspaceID:  workspaceID,
		MemberID:     memberID,
		Provider:     provider,
		EmailAddress: "pending@oauth.local", // Will be updated after OAuth
		IsActive:     false,
		Status:       model.CRMEmailAccountStatusPendingOAuth,
		OAuthState:   &state,
		SyncState:    model.JSONB{"status": model.CRMEmailAccountStatusPendingOAuth},
	}

	if err := s.emailRepo.CreateAccount(ctx, account); err != nil {
		return "", fmt.Errorf("create pending account: %w", err)
	}

	slog.InfoContext(ctx, "initiated Gmail OAuth flow", "workspace_id", workspaceID, "member_id", memberID)

	return s.oauthClient.GenerateAuthURL(state), nil
}

// CompleteOAuth handles the OAuth callback, exchanging code for tokens.
// Returns the workspace slug for redirect purposes.
func (s *CRMEmailService) CompleteOAuth(ctx context.Context, state, code string) (string, error) {
	if s.oauthClient == nil {
		return "", fmt.Errorf("Gmail OAuth not configured")
	}
	if len(s.encryptionKey) == 0 {
		return "", fmt.Errorf("encryption key not configured")
	}

	// Find the account by state.
	account, err := s.emailRepo.GetAccountByOAuthState(ctx, state)
	if err != nil {
		return "", fmt.Errorf("lookup oauth state: %w", err)
	}
	if account == nil {
		return "", fmt.Errorf("invalid or expired OAuth state")
	}

	// Exchange code for tokens.
	tokenPair, err := s.oauthClient.ExchangeCode(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}

	// Encrypt tokens.
	encAccessToken, err := crypto.EncryptString(tokenPair.AccessToken, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt access token: %w", err)
	}
	encRefreshToken, err := crypto.EncryptString(tokenPair.RefreshToken, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt refresh token: %w", err)
	}

	if s.gmailSync == nil {
		return "", fmt.Errorf("Gmail sync not configured")
	}

	profile, err := s.gmailSync.GetMailboxProfile(ctx, tokenPair.AccessToken)
	if err != nil {
		return "", fmt.Errorf("fetch gmail mailbox profile: %w", err)
	}
	normalizedEmail := normalizeMailboxEmail(profile.EmailAddress)
	if normalizedEmail == "" {
		return "", fmt.Errorf("gmail mailbox did not return a valid email address")
	}

	target := account
	existing, err := s.emailRepo.GetAccountByNormalizedEmail(ctx, account.WorkspaceID, account.Provider, normalizedEmail)
	if err != nil {
		return "", fmt.Errorf("lookup existing mailbox: %w", err)
	}
	if existing != nil && existing.ID != account.ID {
		target = existing
	}

	target.MemberID = account.MemberID
	target.EmailAddress = profile.EmailAddress
	target.NormalizedEmailAddress = optionalStringPtr(normalizedEmail)
	target.AccessTokenEncrypted = &encAccessToken
	target.RefreshTokenEncrypted = &encRefreshToken
	target.TokenExpiresAt = &tokenPair.ExpiresAt
	target.OAuthState = nil
	target.IsActive = true
	target.Status = model.CRMEmailAccountStatusConnected
	target.DisconnectedAt = nil
	if (target.LastHistoryID == nil || *target.LastHistoryID == "") && strings.TrimSpace(profile.HistoryID) != "" {
		target.LastHistoryID = optionalStringPtr(strings.TrimSpace(profile.HistoryID))
	}
	target.SyncState = withSyncStatus(target.SyncState, model.CRMEmailAccountStatusConnected)
	if target.LastHistoryID != nil && *target.LastHistoryID != "" {
		target.SyncState["last_history_id"] = *target.LastHistoryID
	}

	if err := s.emailRepo.UpdateAccount(ctx, target); err != nil {
		return "", fmt.Errorf("update account: %w", err)
	}
	if target.ID != account.ID {
		if err := s.emailRepo.DeleteAccount(ctx, account.ID); err != nil {
			return "", fmt.Errorf("remove pending oauth mailbox: %w", err)
		}
	}

	slog.InfoContext(ctx, "completed Gmail OAuth flow", "account_id", target.ID, "workspace_id", target.WorkspaceID, "normalized_email", normalizedEmail)

	if err := s.startEmailSync(ctx, target.ID); err != nil {
		slog.ErrorContext(ctx, "failed to start email sync workflow", "error", err, "account_id", target.ID)
	}

	// Look up workspace slug for redirect.
	ws, err := s.workspaceRepo.GetByID(ctx, target.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("lookup workspace: %w", err)
	}

	return ws.Slug, nil
}

// SendEmail sends an email via Gmail API and stores the outbound message.
func (s *CRMEmailService) SendEmail(ctx context.Context, accountID string, to, cc []string, subject, bodyHTML string) (*model.CRMEmailMessage, error) {
	if s.gmailSync == nil {
		return nil, fmt.Errorf("Gmail sync not configured")
	}

	account, err := s.emailRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("email account not found")
	}
	if !account.IsActive {
		return nil, fmt.Errorf("email account is not active")
	}

	accessToken, err := s.gmailSync.GetValidToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get valid token: %w", err)
	}

	settings, err := s.GetEmailSyncSettings(ctx, account.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load email sync settings: %w", err)
	}

	sendResult, err := s.gmailSync.SendMessage(ctx, accessToken, account.EmailAddress, to, cc, subject, bodyHTML)
	if err != nil {
		return nil, fmt.Errorf("send email: %w", err)
	}

	resolution, err := s.resolver.Resolve(ctx, crmemail.ResolveInput{
		WorkspaceID: account.WorkspaceID,
		Direction:   model.CRMEmailDirectionOutbound,
		Settings:    settings,
		SelfEmails:  []string{account.EmailAddress},
		From: crmemail.Participant{
			Email: account.EmailAddress,
			Role:  model.CRMEmailParticipantRoleFrom,
		},
		To: participantsFromAddresses(to, model.CRMEmailParticipantRoleTo),
		CC: participantsFromAddresses(cc, model.CRMEmailParticipantRoleCC),
	})
	if err != nil {
		return nil, fmt.Errorf("resolve email participants: %w", err)
	}

	toJSON, _ := json.Marshal(participantEmails(resolution.To))
	ccJSON, _ := json.Marshal(participantEmails(resolution.CC))

	now := time.Now()
	var threadID *string
	if sendResult.ThreadID != "" {
		thread, threadErr := s.ensureThread(ctx, account, sendResult.ThreadID, subject, now)
		if threadErr != nil {
			slog.ErrorContext(ctx, "failed to ensure sent email thread", "error", threadErr, "account_id", accountID, "thread_external_id", sendResult.ThreadID)
		} else {
			threadID = &thread.ID
			if err := s.emailRepo.IncrementThreadMessageCount(ctx, thread.ID, now); err != nil {
				slog.ErrorContext(ctx, "failed to increment sent email thread count", "error", err, "thread_id", thread.ID)
			}
		}
	}

	fromAddress := resolution.From.Email
	if fromAddress == "" {
		fromAddress = crmemail.NormalizeEmailAddress(account.EmailAddress)
	}
	message := &model.CRMEmailMessage{
		WorkspaceID:       account.WorkspaceID,
		EmailAccountID:    account.ID,
		ThreadID:          threadID,
		MessageExternalID: sendResult.ID,
		FromAddress:       fromAddress,
		ToAddresses:       toJSON,
		CCAddresses:       ccJSON,
		Subject:           subject,
		BodyHTML:          &bodyHTML,
		Direction:         model.CRMEmailDirectionOutbound,
		SentAt:            now,
		ContactID:         resolution.PrimaryContactID,
		ContactIDs:        resolution.ContactIDs,
	}

	if err := s.emailRepo.CreateMessage(ctx, message); err != nil {
		slog.ErrorContext(ctx, "failed to store sent email", "error", err, "account_id", accountID)
		// Don't fail the send — the email was already sent.
		return message, nil
	}

	if err := s.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, cloneAssociationsForMessage(message.ID, resolution.Associations)); err != nil {
		slog.ErrorContext(ctx, "failed to store sent email contacts", "error", err, "message_id", message.ID)
	} else if threadID != nil {
		if err := s.emailRepo.RefreshThreadContactIDs(ctx, *threadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh sent email thread contacts", "error", err, "thread_id", *threadID)
		}
	}

	slog.InfoContext(ctx, "email sent", "account_id", accountID, "message_id", sendResult.ID, "to", to)

	return message, nil
}

// ── Email Threads ──

// ListThreads returns email threads with filters.
func (s *CRMEmailService) ListThreads(ctx context.Context, workspaceID string, filters model.CRMEmailThreadListFilters, pagination model.PMPagination) ([]model.CRMEmailThread, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.emailRepo.ListThreads(ctx, workspaceID, filters, pagination)
}

// ── Email Messages ──

// ListMessages returns email messages with filters.
func (s *CRMEmailService) ListMessages(ctx context.Context, workspaceID string, filters model.CRMEmailMessageListFilters, pagination model.PMPagination) ([]model.CRMEmailMessage, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.emailRepo.ListMessages(ctx, workspaceID, filters, pagination)
}

// CreateMessage creates a new email message and optionally matches to a contact.
func (s *CRMEmailService) CreateMessage(ctx context.Context, req model.CreateCRMEmailMessageRequest) (*model.CRMEmailMessage, error) {
	if req.WorkspaceID == "" || req.EmailAccountID == "" || req.FromAddress == "" {
		return nil, fmt.Errorf("workspace_id, email_account_id, and from_address are required")
	}

	sentAt := time.Now()
	if req.SentAt != nil {
		sentAt = *req.SentAt
	}

	direction := model.CRMEmailDirectionInbound
	if req.Direction != "" {
		direction = req.Direction
	}

	account, err := s.emailRepo.GetAccountByID(ctx, req.EmailAccountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("email account not found")
	}

	settings, err := s.GetEmailSyncSettings(ctx, req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load email sync settings: %w", err)
	}

	resolution, err := s.resolver.Resolve(ctx, crmemail.ResolveInput{
		WorkspaceID: req.WorkspaceID,
		Direction:   direction,
		Settings:    settings,
		SelfEmails:  []string{account.EmailAddress},
		From: crmemail.Participant{
			Email: req.FromAddress,
			Name:  stringValue(req.FromName),
			Role:  model.CRMEmailParticipantRoleFrom,
		},
		To: participantsFromAddresses(crmemail.ParseAddressJSONArray(req.ToAddresses), model.CRMEmailParticipantRoleTo),
		CC: participantsFromAddresses(crmemail.ParseAddressJSONArray(req.CCAddresses), model.CRMEmailParticipantRoleCC),
	})
	if err != nil {
		return nil, fmt.Errorf("resolve email participants: %w", err)
	}
	mergeExplicitContact(&resolution.Associations, &resolution.ContactIDs, &resolution.PrimaryContactID, req.ContactID, req.WorkspaceID, direction)

	fromAddress := resolution.From.Email
	if fromAddress == "" {
		return nil, fmt.Errorf("from_address must be a valid email address")
	}
	toJSON, _ := json.Marshal(participantEmails(resolution.To))
	ccJSON, _ := json.Marshal(participantEmails(resolution.CC))

	message := &model.CRMEmailMessage{
		WorkspaceID:    req.WorkspaceID,
		EmailAccountID: req.EmailAccountID,
		ThreadID:       req.ThreadID,
		FromAddress:    fromAddress,
		FromName:       req.FromName,
		ToAddresses:    toJSON,
		CCAddresses:    ccJSON,
		Subject:        req.Subject,
		BodyText:       req.BodyText,
		BodyHTML:       req.BodyHTML,
		Direction:      direction,
		SentAt:         sentAt,
		ContactID:      resolution.PrimaryContactID,
		ContactIDs:     resolution.ContactIDs,
		DealID:         req.DealID,
	}

	if err := s.emailRepo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	if err := s.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, cloneAssociationsForMessage(message.ID, resolution.Associations)); err != nil {
		return nil, err
	}
	if req.ThreadID != nil {
		if err := s.emailRepo.RefreshThreadContactIDs(ctx, *req.ThreadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh manual message thread contacts", "error", err, "thread_id", *req.ThreadID)
		}
	}
	return message, nil
}

func (s *CRMEmailService) ensureThread(ctx context.Context, account *model.CRMEmailAccount, externalThreadID, subject string, sentAt time.Time) (*model.CRMEmailThread, error) {
	thread, err := s.emailRepo.GetThreadByExternalID(ctx, account.ID, externalThreadID)
	if err != nil {
		return nil, err
	}
	if thread != nil {
		return thread, nil
	}

	thread = &model.CRMEmailThread{
		WorkspaceID:      account.WorkspaceID,
		EmailAccountID:   account.ID,
		ThreadExternalID: externalThreadID,
		Subject:          subject,
		LastMessageAt:    sentAt,
		MessageCount:     0,
	}
	if err := s.emailRepo.CreateThread(ctx, thread); err != nil {
		return nil, err
	}
	return thread, nil
}

func participantsFromAddresses(addresses []string, role string) []crmemail.Participant {
	result := make([]crmemail.Participant, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, crmemail.Participant{
			Email: address,
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

func mergeExplicitContact(
	associations *[]model.CRMEmailMessageContact,
	contactIDs *[]string,
	primaryContactID **string,
	explicitContactID *string,
	workspaceID string,
	direction string,
) {
	if explicitContactID == nil || *explicitContactID == "" {
		return
	}

	for _, association := range *associations {
		if association.ContactID == *explicitContactID {
			return
		}
	}

	role := model.CRMEmailParticipantRoleManual
	if direction == model.CRMEmailDirectionInbound {
		role = model.CRMEmailParticipantRoleFrom
	} else if direction == model.CRMEmailDirectionOutbound {
		role = model.CRMEmailParticipantRoleTo
	}

	*associations = append(*associations, model.CRMEmailMessageContact{
		ContactID:       *explicitContactID,
		ParticipantRole: role,
		WorkspaceID:     workspaceID,
	})
	*contactIDs = append(*contactIDs, *explicitContactID)

	if len(*contactIDs) == 1 {
		*primaryContactID = explicitContactID
		return
	}
	*primaryContactID = nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeMailboxEmail(value string) string {
	return crmemail.NormalizeEmailAddress(value)
}

func withSyncStatus(syncState model.JSONB, status string) model.JSONB {
	if syncState == nil {
		syncState = model.JSONB{}
	}
	syncState["status"] = status
	return syncState
}

func emailSyncWorkflowID(accountID string) string {
	return "email-sync-" + accountID
}

func (s *CRMEmailService) startEmailSync(ctx context.Context, accountID string) error {
	if s.syncRunner == nil {
		return nil
	}
	return s.syncRunner.StartAccountSync(ctx, accountID)
}

func (s *CRMEmailService) cancelEmailSync(ctx context.Context, accountID string) error {
	if s.syncRunner == nil {
		return nil
	}
	return s.syncRunner.CancelAccountSync(ctx, accountID)
}

func (s *CRMEmailService) populateHasSyncedData(ctx context.Context, accounts []model.CRMEmailAccount) error {
	if len(accounts) == 0 {
		return nil
	}
	accountIDs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	hasSyncedData, err := s.emailRepo.ListAccountsWithSyncedData(ctx, accountIDs)
	if err != nil {
		return err
	}
	for i := range accounts {
		accounts[i].HasSyncedData = hasSyncedData[accounts[i].ID]
	}
	return nil
}

// ── Email Sync Settings ──

// GetEmailSyncSettings returns the email sync settings for a workspace, falling back to defaults.
func (s *CRMEmailService) GetEmailSyncSettings(ctx context.Context, workspaceID string) (*model.CRMEmailSyncSettings, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	settings, err := s.syncSettingsRepo.GetByWorkspace(ctx, workspaceID)
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

// UpdateEmailSyncSettings updates the email sync settings for a workspace.
func (s *CRMEmailService) UpdateEmailSyncSettings(ctx context.Context, workspaceID string, req model.UpdateCRMEmailSyncSettingsRequest) (*model.CRMEmailSyncSettings, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	settings, err := s.syncSettingsRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		defaults := model.DefaultEmailSyncSettings()
		defaults.WorkspaceID = workspaceID
		settings = &defaults
	}

	if req.HistoricalSyncDays != nil {
		settings.HistoricalSyncDays = *req.HistoricalSyncDays
	}
	if req.FilterMode != nil {
		settings.FilterMode = *req.FilterMode
	}
	if req.FilterPatterns != nil {
		settings.FilterPatterns = *req.FilterPatterns
	}
	if req.InternalExclusion != nil {
		settings.InternalExclusion = *req.InternalExclusion
	}
	if req.IncludePrivateMeetings != nil {
		settings.IncludePrivateMeetings = *req.IncludePrivateMeetings
	}
	if req.IncludeSoloMeetings != nil {
		settings.IncludeSoloMeetings = *req.IncludeSoloMeetings
	}
	if req.RecordCreationMode != nil {
		settings.RecordCreationMode = *req.RecordCreationMode
	}
	if req.BlockedRecordPrefixes != nil {
		settings.BlockedRecordPrefixes = *req.BlockedRecordPrefixes
	}

	if err := s.syncSettingsRepo.Upsert(ctx, settings); err != nil {
		return nil, err
	}
	return settings, nil
}
