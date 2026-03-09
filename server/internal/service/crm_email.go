package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMEmailService contains CRM email business logic.
type CRMEmailService struct {
	emailRepo   *repository.CRMEmailRepository
	contactRepo *repository.CRMContactRepository
}

// NewCRMEmailService creates a new CRMEmailService.
func NewCRMEmailService(emailRepo *repository.CRMEmailRepository, contactRepo *repository.CRMContactRepository) *CRMEmailService {
	return &CRMEmailService{emailRepo: emailRepo, contactRepo: contactRepo}
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
