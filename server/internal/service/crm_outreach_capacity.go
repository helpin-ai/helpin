package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MailboxCapacities exposes only the requester's connected mailbox budgets.
func (s *CRMOutreachService) MailboxCapacities(ctx context.Context, ws, user string) ([]model.CRMMailboxCapacity, error) {
	ctx, err := s.access(ctx, ws, user)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accounts.ListAccounts(ctx, ws, model.CRMEmailAccountListFilters{MemberID: &user})
	if err != nil {
		return nil, err
	}
	rows := []model.CRMMailboxCapacity{}
	for _, a := range accounts {
		if a.Provider != "gmail" || !a.IsActive || a.Status != "connected" {
			continue
		}
		capacity, err := s.accounts.SendingCapacity(ctx, &a, s.now())
		if err != nil {
			return nil, err
		}
		capacity.Queued, err = s.repo.QueuedForMailbox(ctx, ws, a.ID)
		if err != nil {
			return nil, err
		}
		rows = append(rows, *capacity)
	}
	return rows, nil
}

// SaveMailboxCapacity authorizes ownership before changing shared physical-mailbox limits.
func (s *CRMOutreachService) SaveMailboxCapacity(ctx context.Context, ws, user, id string, policy model.CRMMailboxSendingPolicy) error {
	ctx, err := s.access(ctx, ws, user)
	if err != nil {
		return err
	}
	a, err := s.accounts.GetAccountByIDForWorkspace(ctx, ws, id)
	if err != nil {
		return err
	}
	if a == nil || a.MemberID != user {
		return fmt.Errorf("only the mailbox owner can change sending limits")
	}
	return s.accounts.UpdateSendingPolicy(ctx, a, policy)
}
