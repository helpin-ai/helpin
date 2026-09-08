package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func crmActionMessageID(intentID string) string { return "<crm-action-" + intentID + "@helpin.ai>" }

// ValidateActionSender uses the same personal mailbox ownership rule as CRM's composer.
func (s *CRMEmailService) ValidateActionSender(ctx context.Context, ws, accountID, userID string) error {
	if s == nil || s.emailRepo == nil {
		return fmt.Errorf("email is not available")
	}
	if _, ok := s.gmailSync.(gmailIntentClient); !ok {
		return fmt.Errorf("email action reconciliation is not available")
	}
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, ws, accountID)
	if err != nil {
		return err
	}
	if account == nil || account.MemberID != userID || !account.IsActive || account.Status != model.CRMEmailAccountStatusConnected || account.Provider != model.CRMEmailProviderGmail {
		return fmt.Errorf("select your connected sending mailbox")
	}
	return nil
}

// SendActionEmail is used only after the CRM action's durable approval claim.
func (s *CRMEmailService) SendActionEmail(ctx context.Context, ws, userID, intentID string, email model.CRMPlaybookEmailAction) (*model.CRMEmailMessage, error) {
	if err := s.ValidateActionSender(ctx, ws, email.AccountID, userID); err != nil {
		return nil, err
	}
	return s.sendEmail(ctx, ws, email.AccountID, userID, []string{email.To}, nil, email.Subject, email.BodyHTML, "", nil, intentID)
}

// ReconcileActionEmail inspects the provider; it never sends or treats absence as failure.
func (s *CRMEmailService) ReconcileActionEmail(ctx context.Context, ws, userID, intentID string, email model.CRMPlaybookEmailAction) (bool, *string, error) {
	if !validSituationID(intentID) {
		return false, nil, ErrCRMPlaybookInput
	}
	if err := s.ValidateActionSender(ctx, ws, email.AccountID, userID); err != nil {
		return false, nil, err
	}
	account, err := s.emailRepo.GetAccountByIDForWorkspace(ctx, ws, email.AccountID)
	if err != nil {
		return false, nil, err
	}
	token, err := s.gmailSync.GetValidToken(ctx, account)
	if err != nil {
		return false, nil, err
	}
	message, err := s.gmailSync.(gmailIntentClient).FindSentMessageByMessageID(ctx, token, crmActionMessageID(intentID))
	if err != nil || message == nil {
		return false, nil, err
	}
	if message.Subject != email.Subject || len(message.To) != 1 || !strings.EqualFold(message.To[0], email.To) || len(message.CC) != 0 || !strings.EqualFold(message.From, account.EmailAddress) || message.BodyHTML != email.BodyHTML {
		return false, nil, fmt.Errorf("sent message does not match the approved action; inspection required")
	}
	local, err := s.emailRepo.GetMessageByExternalID(ctx, account.ID, message.ID)
	if err != nil {
		return false, nil, err
	}
	if local != nil {
		return true, &local.ID, nil
	}
	// The confirmed provider result is honest even while the normal sync catches up.
	return true, nil, nil
}
