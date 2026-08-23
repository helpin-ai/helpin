package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crmsignal"
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

type gmailThreadClient interface {
	GetMessageDetail(ctx context.Context, accessToken, messageID string) (*sync.GmailMessage, error)
	SendThreadMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML, threadID, inReplyTo, references string) (*sync.GmailSendResult, error)
}

type emailSyncWorkflowRunner interface {
	StartAccountSync(ctx context.Context, accountID string) error
	CancelAccountSync(ctx context.Context, accountID string) error
	RequestAccountSync(ctx context.Context, accountID, mode string) error
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

func (r *temporalEmailSyncWorkflowRunner) RequestAccountSync(ctx context.Context, accountID, mode string) error {
	if r == nil || r.client == nil || accountID == "" {
		return nil
	}
	if mode != model.CRMEmailSyncModeHistorical {
		mode = model.CRMEmailSyncModeIncremental
	}
	err := r.client.SignalWorkflow(ctx, emailSyncWorkflowID(accountID), "", "email-sync-now", mode)
	if err == nil {
		return nil
	}
	var notFound *serviceerror.NotFound
	if !errors.As(err, &notFound) {
		return fmt.Errorf("signal email sync workflow: %w", err)
	}
	_, err = r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:        emailSyncWorkflowID(accountID),
		TaskQueue: temporalapp.QueueAutomation,
	}, temporalapp.EmailSyncWorkflow, temporalapp.EmailSyncWorkflowInput{
		AccountID:   accountID,
		InitialMode: mode,
	})
	if err != nil {
		return fmt.Errorf("restart email sync workflow: %w", err)
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
	signalIngestion  *crmsignal.IngestionService
	summaryRefresh   interface {
		RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
		RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
	}
}

// NewCRMEmailService creates a new CRMEmailService.
func NewCRMEmailService(emailRepo *repository.CRMEmailRepository, contactRepo *repository.CRMContactRepository, workspaceRepo *repository.WorkspaceRepository, syncSettingsRepo *repository.CRMEmailSyncSettingsRepository, oauthClient *oauth.GmailOAuthClient, encryptionKey []byte, gmailSync *sync.GmailSyncClient, temporalClient tclient.Client, summaryRefresh interface {
	RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error
	RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error
}) *CRMEmailService {
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
		signalIngestion:  crmsignal.NewIngestionService(emailRepo, crmsignal.NewTemporalStarter(temporalClient, temporalapp.QueueAutomation)),
		summaryRefresh:   summaryRefresh,
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
func (s *CRMEmailService) GetAccount(ctx context.Context, workspaceID, id string) (*model.CRMEmailAccount, error) {
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, id)
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

// GetAccountDiagnostics returns the admin diagnostics read model for a mailbox.
func (s *CRMEmailService) GetAccountDiagnostics(ctx context.Context, workspaceID, id, userID string, isAdmin bool) (*model.CRMEmailAccountDiagnostics, error) {
	account, err := s.GetAccount(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if !isAdmin && account.MemberID != userID {
		return nil, fmt.Errorf("not authorized to view email account diagnostics")
	}

	counts, err := s.emailRepo.GetAccountRecordCounts(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	messagesMissing, err := s.emailRepo.CountMessagesMissingAssociationsByAccount(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	threadsWithEmptyCache, err := s.emailRepo.CountThreadsWithEmptyContactIDsByAccount(ctx, account.ID)
	if err != nil {
		return nil, err
	}

	return &model.CRMEmailAccountDiagnostics{
		AccountID:              account.ID,
		WorkspaceID:            account.WorkspaceID,
		MemberID:               account.MemberID,
		Provider:               account.Provider,
		EmailAddress:           account.EmailAddress,
		NormalizedEmailAddress: account.NormalizedEmailAddress,
		AccountStatus:          account.Status,
		IsActive:               account.IsActive,
		DisconnectedAt:         account.DisconnectedAt,
		LastSyncedAt:           account.LastSyncedAt,
		LastHistoryID:          account.LastHistoryID,
		HasSyncedData:          account.HasSyncedData,
		Sync:                   crmemail.DecodeSyncDiagnostics(account.SyncState, account.LastHistoryID),
		Counts:                 counts,
		AssociationHealth: model.CRMEmailAssociationHealth{
			MessagesMissingAssociations: messagesMissing,
			ThreadsWithEmptyContactIDs:  threadsWithEmptyCache,
		},
	}, nil
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
		SyncState:              crmemail.MarkConnectedIdle(nil, ""),
	}

	if err := s.emailRepo.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

// DeleteAccount disconnects an email account while preserving synced history.
// Only the owner or an admin can disconnect.
func (s *CRMEmailService) DeleteAccount(ctx context.Context, workspaceID, id string, userID string, isAdmin bool) error {
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, id)
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
	account.SyncState = crmemail.MarkDisconnected(account.SyncState, stringValue(account.LastHistoryID), now)

	slog.InfoContext(ctx, "disconnected crm email mailbox", "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)

	return s.emailRepo.UpdateAccount(ctx, account)
}

// PurgeAccountData permanently removes a synced mailbox and all of its synced email/calendar data.
// Admins only.
func (s *CRMEmailService) PurgeAccountData(ctx context.Context, workspaceID, id string, isAdmin bool) error {
	if !isAdmin {
		return fmt.Errorf("not authorized to purge email account data")
	}
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if account == nil {
		return fmt.Errorf("email account not found")
	}

	if err := s.cancelEmailSync(ctx, account.ID); err != nil {
		slog.WarnContext(ctx, "failed to cancel email sync during purge", "error", err, "account_id", account.ID)
	}

	slog.InfoContext(ctx, "purging crm email mailbox data", "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)

	return s.emailRepo.DeleteAccount(ctx, id)
}

// SyncAccount requests an immediate incremental sync or a historical reimport.
// Mailbox owners and workspace CRM admins may trigger it.
func (s *CRMEmailService) SyncAccount(ctx context.Context, workspaceID, id, userID, mode string, isAdmin bool) (*model.CRMEmailAccount, error) {
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("email account not found")
	}
	if !isAdmin && account.MemberID != userID {
		return nil, fmt.Errorf("not authorized to sync this email account")
	}
	if !account.IsActive || account.Status == model.CRMEmailAccountStatusDisconnected {
		return nil, fmt.Errorf("email account is not connected")
	}
	if mode == "" {
		mode = model.CRMEmailSyncModeIncremental
	}
	if mode != model.CRMEmailSyncModeIncremental && mode != model.CRMEmailSyncModeHistorical {
		return nil, fmt.Errorf("sync mode must be incremental or historical")
	}
	account.SyncState = crmemail.MarkConnectedIdle(account.SyncState, stringValue(account.LastHistoryID))
	account.SyncState["phase"] = "queued"
	account.SyncState["requested_mode"] = mode
	account.SyncState["last_attempt_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	account.Status = model.CRMEmailAccountStatusConnected
	if err := s.emailRepo.UpdateAccount(ctx, account); err != nil {
		return nil, err
	}
	if err := s.requestEmailSync(ctx, account.ID, mode); err != nil {
		account.Status = model.CRMEmailAccountStatusError
		account.SyncState = crmemail.FailSyncCycle(account.SyncState, stringValue(account.LastHistoryID), time.Now().UTC(), "request_sync", "workflow_error", err.Error(), nil)
		if updateErr := s.emailRepo.UpdateAccount(ctx, account); updateErr != nil {
			slog.ErrorContext(ctx, "persist requested email sync failure", "error", updateErr, "workspace_id", workspaceID, "account_id", account.ID)
		}
		return nil, err
	}
	return account, nil
}

// OAuthCallback handles the OAuth callback stub (stores tokens placeholder).
func (s *CRMEmailService) OAuthCallback(ctx context.Context, workspaceID, accountID string, code string) error {
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, accountID)
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
	if err := s.emailRepo.DeletePendingOAuthAccounts(ctx, workspaceID, memberID); err != nil {
		return "", err
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
		SyncState:    model.JSONB{"status": model.CRMEmailAccountStatusPendingOAuth, "phase": crmemail.SyncPhaseIdle},
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
	isNewMailbox := target.ID == account.ID

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
	target.SyncState = crmemail.MarkConnectedIdle(target.SyncState, stringValue(target.LastHistoryID))
	if isNewMailbox && strings.TrimSpace(profile.HistoryID) != "" {
		// Preserve the connection-time cursor separately. The initial activity
		// must still import historical mail, then incremental sync resumes from
		// this cursor so messages arriving during the backfill are not missed.
		target.SyncState["initial_history_id"] = strings.TrimSpace(profile.HistoryID)
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

// CancelOAuth removes a pending OAuth attempt and returns its workspace slug
// so the browser can be redirected back to settings after denial or failure.
func (s *CRMEmailService) CancelOAuth(ctx context.Context, state string) (string, error) {
	account, err := s.emailRepo.GetAccountByOAuthState(ctx, state)
	if err != nil {
		return "", err
	}
	if account == nil {
		return "", fmt.Errorf("invalid or expired OAuth state")
	}
	ws, err := s.workspaceRepo.GetByID(ctx, account.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("lookup workspace: %w", err)
	}
	if account.Status == model.CRMEmailAccountStatusPendingOAuth {
		if err := s.emailRepo.DeleteAccount(ctx, account.ID); err != nil {
			return "", err
		}
	}
	return ws.Slug, nil
}

// RebuildAccountAssociations repairs missing email-contact associations for a mailbox.
func (s *CRMEmailService) RebuildAccountAssociations(ctx context.Context, workspaceID, id string, isAdmin bool) (*model.CRMEmailRebuildAssociationsResult, error) {
	if !isAdmin {
		return nil, fmt.Errorf("not authorized to rebuild email associations")
	}

	account, err := s.GetAccount(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}

	settings, err := s.GetEmailSyncSettings(ctx, account.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load email sync settings: %w", err)
	}

	slog.InfoContext(ctx, "rebuilding crm email associations", "workspace_id", account.WorkspaceID, "account_id", account.ID, "provider", account.Provider)

	const batchSize = 200
	result := &model.CRMEmailRebuildAssociationsResult{}
	refreshedThreads := make(map[string]struct{})

	for {
		messages, err := s.emailRepo.ListMessagesMissingAssociationsByAccount(ctx, account.ID, batchSize)
		if err != nil {
			slog.ErrorContext(ctx, "failed to list crm email messages for association rebuild", "error", err, "account_id", account.ID)
			return nil, err
		}
		if len(messages) == 0 {
			break
		}

		for i := range messages {
			message := messages[i]
			result.MessagesScanned++

			resolution, err := s.resolver.Resolve(ctx, crmemail.ResolveInput{
				WorkspaceID: account.WorkspaceID,
				Direction:   message.Direction,
				Settings:    settings,
				SelfEmails:  []string{account.EmailAddress},
				From: crmemail.Participant{
					Email: message.FromAddress,
					Name:  stringValue(message.FromName),
					Role:  model.CRMEmailParticipantRoleFrom,
				},
				To: participantsFromAddresses(crmemail.ParseAddressJSONArray(message.ToAddresses), model.CRMEmailParticipantRoleTo),
				CC: participantsFromAddresses(crmemail.ParseAddressJSONArray(message.CCAddresses), model.CRMEmailParticipantRoleCC),
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed to resolve crm email participants during rebuild", "error", err, "account_id", account.ID, "message_id", message.ID)
				return nil, fmt.Errorf("resolve message participants: %w", err)
			}

			associations := cloneAssociationsForMessage(message.ID, resolution.Associations)
			if err := s.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, associations); err != nil {
				slog.ErrorContext(ctx, "failed to rewrite crm email associations", "error", err, "account_id", account.ID, "message_id", message.ID)
				return nil, fmt.Errorf("replace message contacts: %w", err)
			}

			result.MessagesRepaired++
			result.AssociationsWritten += len(associations)
			result.ContactsCreated += resolution.ContactsCreated
			if message.ThreadID != nil && *message.ThreadID != "" {
				refreshedThreads[*message.ThreadID] = struct{}{}
			}
		}
	}

	for threadID := range refreshedThreads {
		if err := s.emailRepo.RefreshThreadContactIDs(ctx, threadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh crm email thread cache during rebuild", "error", err, "account_id", account.ID, "thread_id", threadID)
			return nil, fmt.Errorf("refresh thread contact ids: %w", err)
		}
		result.ThreadsRefreshed++
	}

	slog.InfoContext(ctx, "completed crm email association rebuild", "workspace_id", account.WorkspaceID, "account_id", account.ID, "messages_scanned", result.MessagesScanned, "messages_repaired", result.MessagesRepaired, "associations_written", result.AssociationsWritten, "threads_refreshed", result.ThreadsRefreshed, "contacts_created", result.ContactsCreated)

	return result, nil
}

// SendEmail sends an email via Gmail API and stores the outbound message.
func (s *CRMEmailService) SendEmail(ctx context.Context, workspaceID, accountID, userID string, _ bool, to, cc []string, subject, bodyHTML string) (*model.CRMEmailMessage, error) {
	if s.gmailSync == nil {
		return nil, fmt.Errorf("Gmail sync not configured")
	}

	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("email account not found")
	}
	if account.MemberID != userID {
		return nil, fmt.Errorf("not authorized to send from this email account")
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
	if threadID != nil {
		if err := s.emailRepo.IncrementThreadMessageCount(ctx, *threadID, now); err != nil {
			slog.ErrorContext(ctx, "failed to increment sent email thread count", "error", err, "thread_id", *threadID)
		}
	}

	if err := s.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, cloneAssociationsForMessage(message.ID, resolution.Associations)); err != nil {
		slog.ErrorContext(ctx, "failed to store sent email contacts", "error", err, "message_id", message.ID)
	} else if threadID != nil {
		if err := s.emailRepo.RefreshThreadContactIDs(ctx, *threadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh sent email thread contacts", "error", err, "thread_id", *threadID)
		}
	}
	s.enqueueBuyerSignalDetection(ctx, message.ID)
	s.requestSummaryRefreshForMessage(ctx, message, "manual_send")

	slog.InfoContext(ctx, "email sent", "account_id", accountID, "message_id", sendResult.ID, "to", to)

	return message, nil
}

// ── Email Threads ──

// ListThreads returns email threads with filters.
func (s *CRMEmailService) ListThreads(ctx context.Context, workspaceID, userID string, filters model.CRMEmailThreadListFilters, pagination model.PMPagination) ([]model.CRMEmailThread, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	filters.UserID = &userID
	threads, total, err := s.emailRepo.ListThreads(ctx, workspaceID, filters, pagination)
	if err != nil {
		return nil, 0, err
	}
	for i := range threads {
		account, accountErr := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, threads[i].EmailAccountID)
		if accountErr != nil {
			return nil, 0, accountErr
		}
		latest, latestErr := s.emailRepo.GetLatestMessageForThread(ctx, workspaceID, threads[i].ID)
		if latestErr != nil {
			return nil, 0, latestErr
		}
		threads[i].LatestMessage = latest
		if account != nil {
			threads[i].MailboxEmail = account.EmailAddress
			threads[i].MailboxProvider = account.Provider
			threads[i].MailboxStatus = account.Status
			threads[i].MailboxLastSync = account.LastSyncedAt
			threads[i].CanReply = account.MemberID == userID && account.IsActive && account.Status == model.CRMEmailAccountStatusConnected && account.Provider == model.CRMEmailProviderGmail
		}
		if latest != nil && latest.Direction == model.CRMEmailDirectionInbound {
			dismissal, dismissalErr := s.emailRepo.GetThreadDismissal(ctx, workspaceID, threads[i].ID, userID)
			if dismissalErr != nil {
				return nil, 0, dismissalErr
			}
			threads[i].NeedsReplyDismissed = dismissal != nil && !dismissal.DismissedAt.Before(latest.SentAt)
			threads[i].NeedsReply = !threads[i].NeedsReplyDismissed
		}
	}
	return threads, total, nil
}

func (s *CRMEmailService) GetThreadDetail(ctx context.Context, workspaceID, threadID, userID string) (*model.CRMEmailThreadDetail, error) {
	thread, err := s.emailRepo.GetThreadByID(ctx, threadID)
	if err != nil || thread == nil || thread.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("email thread not found")
	}
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, thread.EmailAccountID)
	if err != nil {
		return nil, err
	}
	latest, err := s.emailRepo.GetLatestMessageForThread(ctx, workspaceID, threadID)
	if err != nil {
		return nil, err
	}
	thread.LatestMessage = latest
	if account != nil {
		thread.MailboxEmail = account.EmailAddress
		thread.MailboxProvider = account.Provider
		thread.MailboxStatus = account.Status
		thread.MailboxLastSync = account.LastSyncedAt
		thread.CanReply = account.MemberID == userID && account.IsActive && account.Status == model.CRMEmailAccountStatusConnected && account.Provider == model.CRMEmailProviderGmail
	}
	if latest != nil && latest.Direction == model.CRMEmailDirectionInbound {
		dismissal, dismissalErr := s.emailRepo.GetThreadDismissal(ctx, workspaceID, threadID, userID)
		if dismissalErr != nil {
			return nil, dismissalErr
		}
		thread.NeedsReplyDismissed = dismissal != nil && !dismissal.DismissedAt.Before(latest.SentAt)
		thread.NeedsReply = !thread.NeedsReplyDismissed
	}
	messages, _, err := s.emailRepo.ListMessages(ctx, workspaceID, model.CRMEmailMessageListFilters{ThreadID: &threadID}, model.PMPagination{Page: 1, PerPage: 250})
	if err != nil {
		return nil, err
	}
	participants, err := s.threadParticipants(ctx, workspaceID, messages)
	if err != nil {
		return nil, err
	}
	return &model.CRMEmailThreadDetail{Thread: *thread, Messages: messages, Participants: participants}, nil
}

func (s *CRMEmailService) SetThreadDismissed(ctx context.Context, workspaceID, threadID, userID string, dismissed bool) error {
	thread, err := s.emailRepo.GetThreadByID(ctx, threadID)
	if err != nil || thread == nil || thread.WorkspaceID != workspaceID {
		return fmt.Errorf("email thread not found")
	}
	if !dismissed {
		return s.emailRepo.DeleteThreadDismissal(ctx, workspaceID, threadID, userID)
	}
	return s.emailRepo.UpsertThreadDismissal(ctx, &model.CRMEmailThreadDismissal{WorkspaceID: workspaceID, ThreadID: threadID, UserID: userID, DismissedAt: time.Now().UTC()})
}

func (s *CRMEmailService) LinkThreadDeal(ctx context.Context, workspaceID, threadID string, dealID *string) error {
	if dealID != nil && strings.TrimSpace(*dealID) != "" {
		valid, err := s.emailRepo.CRMEntityBelongsToWorkspace(ctx, "deal", workspaceID, strings.TrimSpace(*dealID))
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("deal not found")
		}
	}
	return s.emailRepo.UpdateThreadDeal(ctx, workspaceID, threadID, dealID)
}

func (s *CRMEmailService) ReplyToThread(ctx context.Context, workspaceID, threadID, userID, mode, bodyHTML string) (*model.CRMEmailMessage, error) {
	thread, err := s.emailRepo.GetThreadByID(ctx, threadID)
	if err != nil || thread == nil || thread.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("email thread not found")
	}
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, workspaceID, thread.EmailAccountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("email account not found")
	}
	if account.MemberID != userID {
		return nil, fmt.Errorf("only the connected mailbox owner can reply to this thread")
	}
	if !account.IsActive || account.Status != model.CRMEmailAccountStatusConnected {
		return nil, fmt.Errorf("email account is not connected")
	}
	threadClient, ok := s.gmailSync.(gmailThreadClient)
	if !ok || threadClient == nil {
		return nil, fmt.Errorf("thread replies are not configured")
	}
	latest, err := s.emailRepo.GetLatestMessageForThread(ctx, workspaceID, threadID)
	if err != nil || latest == nil {
		return nil, fmt.Errorf("email thread has no messages")
	}
	accessToken, err := s.gmailSync.GetValidToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get valid token: %w", err)
	}
	if latest.RFCMessageID == nil || strings.TrimSpace(*latest.RFCMessageID) == "" {
		detail, detailErr := threadClient.GetMessageDetail(ctx, accessToken, latest.MessageExternalID)
		if detailErr != nil {
			return nil, fmt.Errorf("load reply headers: %w", detailErr)
		}
		latest.RFCMessageID = optionalStringPtr(detail.RFCMessageID)
		latest.InReplyTo = optionalStringPtr(detail.InReplyTo)
		latest.ReferencesHeader = optionalStringPtr(detail.ReferencesHeader)
		if err := s.emailRepo.UpdateMessageReplyHeaders(ctx, workspaceID, latest.ID, detail.RFCMessageID, detail.InReplyTo, detail.ReferencesHeader); err != nil {
			return nil, err
		}
	}
	to, cc := threadReplyRecipients(latest, account.EmailAddress, mode)
	if len(to) == 0 {
		return nil, fmt.Errorf("thread has no reply recipient")
	}
	references := strings.TrimSpace(stringValue(latest.ReferencesHeader) + " " + stringValue(latest.RFCMessageID))
	sendResult, err := threadClient.SendThreadMessage(ctx, accessToken, account.EmailAddress, to, cc, thread.Subject, bodyHTML, thread.ThreadExternalID, stringValue(latest.RFCMessageID), references)
	if err != nil {
		return nil, fmt.Errorf("send thread reply: %w", err)
	}
	now := time.Now().UTC()
	toJSON, _ := json.Marshal(to)
	ccJSON, _ := json.Marshal(cc)
	settings, settingsErr := s.GetEmailSyncSettings(ctx, workspaceID)
	if settingsErr != nil {
		return nil, fmt.Errorf("load email sync settings: %w", settingsErr)
	}
	resolution, resolutionErr := s.resolver.Resolve(ctx, crmemail.ResolveInput{
		WorkspaceID: workspaceID,
		Direction:   model.CRMEmailDirectionOutbound,
		Settings:    settings,
		SelfEmails:  []string{account.EmailAddress},
		From:        crmemail.Participant{Email: account.EmailAddress, Role: model.CRMEmailParticipantRoleFrom},
		To:          participantsFromAddresses(to, model.CRMEmailParticipantRoleTo),
		CC:          participantsFromAddresses(cc, model.CRMEmailParticipantRoleCC),
	})
	if resolutionErr != nil {
		return nil, fmt.Errorf("resolve reply participants: %w", resolutionErr)
	}
	message := &model.CRMEmailMessage{
		WorkspaceID: workspaceID, EmailAccountID: account.ID, ThreadID: &thread.ID,
		MessageExternalID: sendResult.ID, FromAddress: account.EmailAddress,
		ToAddresses: toJSON, CCAddresses: ccJSON, Subject: thread.Subject,
		BodyHTML: &bodyHTML, Direction: model.CRMEmailDirectionOutbound, SentAt: now, DealID: thread.DealID,
		ContactID: resolution.PrimaryContactID, ContactIDs: resolution.ContactIDs,
	}
	if err := s.emailRepo.CreateMessage(ctx, message); err != nil {
		return message, nil
	}
	if err := s.emailRepo.IncrementThreadMessageCount(ctx, thread.ID, now); err != nil {
		slog.ErrorContext(ctx, "failed to increment replied thread", "error", err, "thread_id", thread.ID)
	}
	if err := s.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, cloneAssociationsForMessage(message.ID, resolution.Associations)); err != nil {
		slog.ErrorContext(ctx, "failed to persist reply contacts", "error", err, "message_id", message.ID)
	} else if err := s.emailRepo.RefreshThreadContactIDs(ctx, thread.ID); err != nil {
		slog.ErrorContext(ctx, "failed to refresh reply thread contacts", "error", err, "thread_id", thread.ID)
	}
	s.enqueueBuyerSignalDetection(ctx, message.ID)
	s.requestSummaryRefreshForMessage(ctx, message, "thread_reply")
	return message, nil
}

func threadReplyRecipients(message *model.CRMEmailMessage, selfEmail, mode string) ([]string, []string) {
	selfEmail = strings.ToLower(strings.TrimSpace(selfEmail))
	toAddresses := crmemail.ParseAddressJSONArray(message.ToAddresses)
	ccAddresses := crmemail.ParseAddressJSONArray(message.CCAddresses)
	seen := map[string]struct{}{selfEmail: {}}
	add := func(target *[]string, value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		*target = append(*target, value)
	}
	to := []string{}
	cc := []string{}
	if mode == "reply_all" {
		add(&to, message.FromAddress)
		for _, address := range toAddresses {
			add(&to, address)
		}
		for _, address := range ccAddresses {
			add(&cc, address)
		}
		return to, cc
	}
	if message.Direction == model.CRMEmailDirectionInbound {
		add(&to, message.FromAddress)
	} else {
		for _, address := range toAddresses {
			add(&to, address)
			if len(to) == 1 {
				break
			}
		}
	}
	return to, cc
}

func (s *CRMEmailService) threadParticipants(ctx context.Context, workspaceID string, messages []model.CRMEmailMessage) ([]model.CRMEmailParticipant, error) {
	type participantSeed struct {
		email string
		name  string
		role  string
	}
	byEmail := map[string]participantSeed{}
	roles := map[string]int{model.CRMEmailParticipantRoleFrom: 0, model.CRMEmailParticipantRoleTo: 1, model.CRMEmailParticipantRoleCC: 2}
	put := func(email, name, role string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			return
		}
		current, exists := byEmail[email]
		if !exists || roles[role] < roles[current.role] {
			byEmail[email] = participantSeed{email: email, name: strings.TrimSpace(name), role: role}
		} else if current.name == "" && strings.TrimSpace(name) != "" {
			current.name = strings.TrimSpace(name)
			byEmail[email] = current
		}
	}
	for _, message := range messages {
		put(message.FromAddress, stringValue(message.FromName), model.CRMEmailParticipantRoleFrom)
		for _, address := range crmemail.ParseAddressJSONArray(message.ToAddresses) {
			put(address, "", model.CRMEmailParticipantRoleTo)
		}
		for _, address := range crmemail.ParseAddressJSONArray(message.CCAddresses) {
			put(address, "", model.CRMEmailParticipantRoleCC)
		}
	}
	emails := make([]string, 0, len(byEmail))
	for email := range byEmail {
		emails = append(emails, email)
	}
	sort.Strings(emails)
	contacts, err := s.contactRepo.ListByEmails(ctx, workspaceID, emails)
	if err != nil {
		return nil, err
	}
	participants := make([]model.CRMEmailParticipant, 0, len(emails))
	for _, email := range emails {
		seed := byEmail[email]
		participant := model.CRMEmailParticipant{Email: email, Name: seed.name, Role: seed.role}
		if contact, ok := contacts[email]; ok {
			participant.ContactID = &contact.ID
			participant.ContactName = strings.TrimSpace(contact.FirstName + " " + stringValue(contact.LastName))
			if participant.Name == "" {
				participant.Name = participant.ContactName
			}
		}
		participants = append(participants, participant)
	}
	return participants, nil
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
	if account.WorkspaceID != req.WorkspaceID {
		return nil, fmt.Errorf("email account not found")
	}
	if req.ThreadID != nil && *req.ThreadID != "" {
		thread, threadErr := s.emailRepo.GetThreadByID(ctx, *req.ThreadID)
		if threadErr != nil {
			return nil, threadErr
		}
		if thread == nil || thread.WorkspaceID != req.WorkspaceID || thread.EmailAccountID != account.ID {
			return nil, fmt.Errorf("email thread not found")
		}
	}
	if req.ContactID != nil && *req.ContactID != "" {
		valid, validationErr := s.emailRepo.CRMEntityBelongsToWorkspace(ctx, "contact", req.WorkspaceID, *req.ContactID)
		if validationErr != nil {
			return nil, validationErr
		}
		if !valid {
			return nil, fmt.Errorf("contact not found")
		}
	}
	if req.DealID != nil && *req.DealID != "" {
		valid, validationErr := s.emailRepo.CRMEntityBelongsToWorkspace(ctx, "deal", req.WorkspaceID, *req.DealID)
		if validationErr != nil {
			return nil, validationErr
		}
		if !valid {
			return nil, fmt.Errorf("deal not found")
		}
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
	if req.ThreadID != nil && *req.ThreadID != "" {
		if err := s.emailRepo.IncrementThreadMessageCount(ctx, *req.ThreadID, sentAt); err != nil {
			return nil, err
		}
	}
	if err := s.emailRepo.ReplaceMessageContacts(ctx, message.ID, resolution.PrimaryContactID, cloneAssociationsForMessage(message.ID, resolution.Associations)); err != nil {
		return nil, err
	}
	if req.ThreadID != nil {
		if err := s.emailRepo.RefreshThreadContactIDs(ctx, *req.ThreadID); err != nil {
			slog.ErrorContext(ctx, "failed to refresh manual message thread contacts", "error", err, "thread_id", *req.ThreadID)
		}
	}
	s.enqueueBuyerSignalDetection(ctx, message.ID)
	s.requestSummaryRefreshForMessage(ctx, message, "manual_create")
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
		reloaded, lookupErr := s.emailRepo.GetThreadByExternalID(ctx, account.ID, externalThreadID)
		if lookupErr == nil && reloaded != nil {
			return reloaded, nil
		}
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

func (s *CRMEmailService) requestEmailSync(ctx context.Context, accountID, mode string) error {
	if s.syncRunner == nil {
		return fmt.Errorf("email sync worker is not configured")
	}
	return s.syncRunner.RequestAccountSync(ctx, accountID, mode)
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

func (s *CRMEmailService) enqueueBuyerSignalDetection(ctx context.Context, messageID string) {
	if s == nil || s.signalIngestion == nil || messageID == "" {
		return
	}
	if _, err := s.signalIngestion.EnqueueEmailMessage(ctx, messageID); err != nil {
		slog.ErrorContext(ctx, "failed to enqueue crm buyer signal detection", "error", err, "message_id", messageID)
	}
}

func (s *CRMEmailService) requestSummaryRefreshForMessage(ctx context.Context, message *model.CRMEmailMessage, source string) {
	if s == nil || s.summaryRefresh == nil || message == nil {
		return
	}

	companyRefresh, canRefreshCompanies := s.summaryRefresh.(CompanySummaryRefreshRequester)
	if message.DealID != nil && *message.DealID != "" {
		if err := s.summaryRefresh.RequestDealRefresh(ctx, message.WorkspaceID, *message.DealID); err != nil {
			slog.ErrorContext(ctx, "failed to request deal summary refresh from crm email message", "error", err, "workspace_id", message.WorkspaceID, "deal_id", *message.DealID, "message_id", message.ID, "source", source)
		}
		if canRefreshCompanies {
			if err := companyRefresh.RequestCompanyRefreshForObject(ctx, message.WorkspaceID, model.CRMObjectDeal, *message.DealID); err != nil {
				slog.ErrorContext(ctx, "failed to request company summary refresh from crm email deal", "error", err, "workspace_id", message.WorkspaceID, "deal_id", *message.DealID, "message_id", message.ID, "source", source)
			}
		}
	}

	if canRefreshCompanies {
		for _, contactID := range message.ContactIDs {
			if err := companyRefresh.RequestCompanyRefreshForObject(ctx, message.WorkspaceID, model.CRMObjectContact, contactID); err != nil {
				slog.ErrorContext(ctx, "failed to request company summary refresh from crm email contact", "error", err, "workspace_id", message.WorkspaceID, "contact_id", contactID, "message_id", message.ID, "source", source)
			}
		}
	}

	if len(message.ContactIDs) == 1 {
		if err := s.summaryRefresh.RequestContactRefresh(ctx, message.WorkspaceID, message.ContactIDs[0]); err != nil {
			slog.ErrorContext(ctx, "failed to request contact summary refresh from crm email message", "error", err, "workspace_id", message.WorkspaceID, "contact_id", message.ContactIDs[0], "message_id", message.ID, "source", source)
		}
		return
	}

	if message.DealID == nil && len(message.ContactIDs) > 1 {
		slog.InfoContext(ctx, "skipping contact summary refresh for multi-contact crm email without deal", "workspace_id", message.WorkspaceID, "message_id", message.ID, "contact_count", len(message.ContactIDs), "source", source)
	}
}

// ── Email Sync Settings ──

// GetEmailSyncSettings returns the email sync settings for a workspace, falling back to defaults.
func (s *CRMEmailService) GetEmailSyncSettings(ctx context.Context, workspaceID string) (*model.CRMEmailSyncSettings, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if s.syncSettingsRepo == nil {
		defaults := model.DefaultEmailSyncSettings()
		defaults.WorkspaceID = workspaceID
		return &defaults, nil
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
	if s.syncSettingsRepo == nil {
		return nil, fmt.Errorf("email sync settings repository not configured")
	}
	if req.HistoricalSyncDays != nil && (*req.HistoricalSyncDays < 1 || *req.HistoricalSyncDays > 3650) {
		return nil, fmt.Errorf("historical_sync_days must be between 1 and 3650")
	}
	if req.FilterMode != nil && *req.FilterMode != "blocklist" && *req.FilterMode != "allowlist" {
		return nil, fmt.Errorf("filter_mode must be blocklist or allowlist")
	}
	if req.InternalExclusion != nil && *req.InternalExclusion != "none" && *req.InternalExclusion != "exclude" {
		return nil, fmt.Errorf("internal_exclusion must be none or exclude")
	}
	if req.RecordCreationMode != nil && *req.RecordCreationMode != "disabled" && *req.RecordCreationMode != "selective" && *req.RecordCreationMode != "always" {
		return nil, fmt.Errorf("record_creation_mode must be disabled, selective, or always")
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
