package service

import (
	"context"
	"errors"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// MaintainScheduledWork repairs interrupted execution and checks for new facts.
// It shares the existing scheduled-events worker; it is not a second scheduler.
func (s *CRMPlaybookExecutionService) MaintainScheduledWork(ctx context.Context) error {
	store, ok := s.store.(interface {
		MaintenanceRuns(context.Context, time.Time) ([]model.AgentRun, error)
		ClaimRunMaintenance(context.Context, model.AgentRun, time.Time) (bool, error)
		MaintenanceBindings(context.Context, time.Time) ([]model.CRMPlaybookAutomationBinding, error)
		MaintainBinding(context.Context, string, string, time.Time, func(model.CRMPlaybookExecutionSource, model.CRMPlaybookActionFacts) (string, error)) error
	})
	if !ok {
		return nil
	}
	now := s.now().UTC()
	bindings, err := store.MaintenanceBindings(ctx, now)
	if err != nil {
		return err
	}
	var failures []error
	for _, binding := range bindings {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := store.MaintainBinding(ctx, binding.WorkspaceID, binding.SituationID, now, crmPlaybookActionContextFingerprint); err != nil {
			failures = append(failures, err)
		}
	}
	runs, err := store.MaintenanceRuns(ctx, now)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		claimed, err := store.ClaimRunMaintenance(ctx, run, now)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if !claimed {
			continue
		}
		if !model.IsAgentRunActiveStatus(run.Status) {
			if err := s.ObserveTerminalRun(ctx, run); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		// Cancellation is allowed after authority has gone; no new run is launched.
		if _, _, err := s.AuthorizeRun(ctx, run.WorkspaceID, run.ID); err != nil {
			var entitlement *EntitlementError
			if !errors.Is(err, ErrCRMPlaybookForbidden) && !errors.Is(err, repository.ErrCRMPlaybookExecutionBlocked) && !errors.Is(err, repository.ErrCRMPlaybookUnavailable) && !errors.As(err, &entitlement) {
				// A temporary database/billing read failure denies callbacks but
				// does not prove that the user revoked an already-running check.
				failures = append(failures, err)
				continue
			}
			if s.launcher != nil {
				check, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := s.launcher.CancelRun(check, run.WorkspaceID, run.ID, derefString(run.TriggeredByUserID))
				cancel()
				if err != nil {
					failures = append(failures, err)
				}
			}
			continue
		}
		if recoverer, ok := s.launcher.(interface {
			RecoverPlaybookRun(context.Context, *model.AgentRun) error
		}); ok {
			check, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := recoverer.RecoverPlaybookRun(check, &run)
			cancel()
			if err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
