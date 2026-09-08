package service

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func (s *CRMPlaybookExecutionService) handleAutomaticEntry(ctx context.Context, event model.AutomationScheduledEvent, now time.Time) error {
	store, ok := s.store.(interface {
		AutomaticCandidates(context.Context, string, string) ([]repository.AutomaticPlaybookCandidate, error)
		EnrollAutomatically(context.Context, model.AutomationScheduledEvent, *repository.AutomaticPlaybookCandidate, string, time.Time) error
	})
	if !ok {
		return ErrCRMPlaybookRuntimeUnavailable
	}
	candidates, err := store.AutomaticCandidates(ctx, event.WorkspaceID, event.TargetID)
	if err != nil {
		return err
	}
	if len(candidates) != 1 {
		return store.EnrollAutomatically(ctx, event, nil, "", now)
	}
	candidate := candidates[0]
	if err := s.connectionReady(candidate.Connection); err != nil {
		return err
	}
	if err := s.requireEntitlements(ctx, event.WorkspaceID); err != nil {
		return err
	}
	configActor, err := s.liveActor(ctx, event.WorkspaceID, candidate.Settings.AuthorizedByMemberID)
	if err != nil {
		return err
	}
	if !s.authz.Can(configActor, authorization.PermCRMAdmin) || !s.authz.Can(configActor, authorization.PermPMAdminAutomations) {
		return ErrCRMPlaybookForbidden
	}
	if err := s.requireModules(ctx, configActor); err != nil {
		return err
	}
	if candidate.OwnerMemberID == nil {
		return store.EnrollAutomatically(ctx, event, nil, "", now)
	}
	owner, err := s.liveActor(ctx, event.WorkspaceID, *candidate.OwnerMemberID)
	if err != nil {
		return err
	}
	if !s.authz.Can(owner, authorization.PermCRMEdit) {
		return ErrCRMPlaybookForbidden
	}
	return store.EnrollAutomatically(ctx, event, &candidate, owner.WorkspaceMemberID, now)
}
