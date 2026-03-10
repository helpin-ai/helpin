package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
)

// CRMEmailService contains CRM email business logic.
type CRMEmailService struct {
	emailRepo     *repository.CRMEmailRepository
	contactRepo   *repository.CRMContactRepository
	oauthClient   *oauth.GmailOAuthClient
	encryptionKey []byte
	gmailSync     *sync.GmailSyncClient
}

// NewCRMEmailService creates a new CRMEmailService.
func NewCRMEmailService(emailRepo *repository.CRMEmailRepository, contactRepo *repository.CRMContactRepository, oauthClient *oauth.GmailOAuthClient, encryptionKey []byte, gmailSync *sync.GmailSyncClient) *CRMEmailService {
	return &CRMEmailService{
		emailRepo:     emailRepo,
		contactRepo:   contactRepo,
		oauthClient:   oauthClient,
		encryptionKey: encryptionKey,
		gmailSync:     gmailSync,
	}
}

// ── Email Accounts ──

// ListAccounts returns email accounts for a workspace.
func (s *CRMEmailService) ListAccounts(ctx context.Context, workspaceID string, filters model.CRMEmailAccountListFilters) ([]model.CRMEmailAccount, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.emailRepo.ListAccounts(ctx, workspaceID, filters)
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
		WorkspaceID:  req.WorkspaceID,
		MemberID:     req.MemberID,
		Provider:     req.Provider,
		EmailAddress: strings.TrimSpace(req.EmailAddress),
		IsActive:     true,
		SyncState:    model.JSONB{"status": "pending_oauth"},
	}

	if err := s.emailRepo.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

// DeleteAccount removes an email account.
func (s *CRMEmailService) DeleteAccount(ctx context.Context, id string) error {
	account, err := s.emailRepo.GetAccountByID(ctx, id)
	if err != nil {
		return err
	}
	if account == nil {
		return fmt.Errorf("email account not found")
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
		OAuthState:   &state,
		SyncState:    model.JSONB{"status": "pending_oauth"},
	}

	if err := s.emailRepo.CreateAccount(ctx, account); err != nil {
		return "", fmt.Errorf("create pending account: %w", err)
	}

	slog.InfoContext(ctx, "initiated Gmail OAuth flow", "workspace_id", workspaceID, "member_id", memberID)

	return s.oauthClient.GenerateAuthURL(state), nil
}

// CompleteOAuth handles the OAuth callback, exchanging code for tokens.
func (s *CRMEmailService) CompleteOAuth(ctx context.Context, state, code string) error {
	if s.oauthClient == nil {
		return fmt.Errorf("Gmail OAuth not configured")
	}
	if len(s.encryptionKey) == 0 {
		return fmt.Errorf("encryption key not configured")
	}

	// Find the account by state.
	account, err := s.emailRepo.GetAccountByOAuthState(ctx, state)
	if err != nil {
		return fmt.Errorf("lookup oauth state: %w", err)
	}
	if account == nil {
		return fmt.Errorf("invalid or expired OAuth state")
	}

	// Exchange code for tokens.
	tokenPair, err := s.oauthClient.ExchangeCode(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	// Encrypt tokens.
	encAccessToken, err := crypto.EncryptString(tokenPair.AccessToken, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	encRefreshToken, err := crypto.EncryptString(tokenPair.RefreshToken, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	// Update account.
	account.AccessTokenEncrypted = &encAccessToken
	account.RefreshTokenEncrypted = &encRefreshToken
	account.TokenExpiresAt = &tokenPair.ExpiresAt
	account.OAuthState = nil // Clear state
	account.IsActive = true
	account.SyncState = model.JSONB{"status": "connected"}

	// TODO: Fetch the actual email address from the Gmail API userinfo.
	// For now, the email will be updated on first sync.

	if err := s.emailRepo.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("update account: %w", err)
	}

	slog.InfoContext(ctx, "completed Gmail OAuth flow", "account_id", account.ID, "workspace_id", account.WorkspaceID)

	return nil
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

	messageID, err := s.gmailSync.SendMessage(ctx, accessToken, account.EmailAddress, to, cc, subject, bodyHTML)
	if err != nil {
		return nil, fmt.Errorf("send email: %w", err)
	}

	// Build to/cc as JSONB.
	toJSON, _ := json.Marshal(to)
	ccJSON, _ := json.Marshal(cc)

	now := time.Now()
	message := &model.CRMEmailMessage{
		WorkspaceID:       account.WorkspaceID,
		EmailAccountID:    account.ID,
		MessageExternalID: messageID,
		FromAddress:       account.EmailAddress,
		ToAddresses:       mustUnmarshalJSONB(toJSON),
		CCAddresses:       mustUnmarshalJSONB(ccJSON),
		Subject:           subject,
		BodyHTML:          &bodyHTML,
		Direction:         model.CRMEmailDirectionOutbound,
		SentAt:            now,
	}

	// Auto-match contact by first "to" address.
	if len(to) > 0 {
		matchedContact := s.matchContactByEmail(ctx, account.WorkspaceID, to[0])
		if matchedContact != nil {
			message.ContactID = &matchedContact.ID
		}
	}

	if err := s.emailRepo.CreateMessage(ctx, message); err != nil {
		slog.ErrorContext(ctx, "failed to store sent email", "error", err, "account_id", accountID)
		// Don't fail the send — the email was already sent.
	}

	slog.InfoContext(ctx, "email sent", "account_id", accountID, "message_id", messageID, "to", to)

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

	message := &model.CRMEmailMessage{
		WorkspaceID:    req.WorkspaceID,
		EmailAccountID: req.EmailAccountID,
		ThreadID:       req.ThreadID,
		FromAddress:    strings.TrimSpace(req.FromAddress),
		FromName:       req.FromName,
		ToAddresses:    model.JSONB(req.ToAddresses),
		CCAddresses:    model.JSONB(req.CCAddresses),
		Subject:        req.Subject,
		BodyText:       req.BodyText,
		BodyHTML:       req.BodyHTML,
		Direction:      direction,
		SentAt:         sentAt,
		ContactID:      req.ContactID,
		DealID:         req.DealID,
	}

	// Auto-match contact by email address if not provided.
	if message.ContactID == nil {
		matchedContact := s.matchContactByEmail(ctx, req.WorkspaceID, req.FromAddress)
		if matchedContact != nil {
			message.ContactID = &matchedContact.ID
		}
	}

	if err := s.emailRepo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	return message, nil
}

// matchContactByEmail finds a contact matching the given email address.
func (s *CRMEmailService) matchContactByEmail(ctx context.Context, workspaceID, emailAddr string) *model.CRMContact {
	search := strings.TrimSpace(emailAddr)
	if search == "" {
		return nil
	}
	contacts, _, err := s.contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{
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

// mustUnmarshalJSONB converts JSON bytes to a JSONB map.
func mustUnmarshalJSONB(data []byte) model.JSONB {
	var result model.JSONB
	if err := json.Unmarshal(data, &result); err != nil {
		// The data might be a JSON array — wrap it.
		var arr []interface{}
		if err2 := json.Unmarshal(data, &arr); err2 == nil {
			return model.JSONB{"items": arr}
		}
		return model.JSONB{}
	}
	return result
}
