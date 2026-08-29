package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMCommercialStateSynchronizer materializes accepted server-origin group()
// assertions from the canonical event store. Event IDs make the overlap safe.
type CRMCommercialStateSynchronizer struct {
	service  *CRMSignalService
	repo     *repository.CRMSignalRepository
	projects repository.EventWorkspaceResolver
	eventsDB *sql.DB
	owner    string
}

func NewCRMCommercialStateSynchronizer(
	service *CRMSignalService,
	repo *repository.CRMSignalRepository,
	projects repository.EventWorkspaceResolver,
	eventsDB *sql.DB,
) *CRMCommercialStateSynchronizer {
	return &CRMCommercialStateSynchronizer{service: service, repo: repo, projects: projects, eventsDB: eventsDB, owner: uuid.NewString()}
}

func (s *CRMCommercialStateSynchronizer) Run(ctx context.Context) {
	if s == nil || s.eventsDB == nil || s.projects == nil {
		return
	}
	s.runAndLog(ctx)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runAndLog(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *CRMCommercialStateSynchronizer) runAndLog(ctx context.Context) {
	workspaceIDs, err := s.projects.ListAllActiveEventWorkspaceIDs(ctx)
	if err != nil {
		slog.WarnContext(ctx, "list workspaces for CRM commercial-state synchronization failed", "error", err)
		return
	}
	for _, workspaceID := range workspaceIDs {
		s.runWorkspaceAndLog(ctx, workspaceID, time.Now().UTC())
	}
}

func (s *CRMCommercialStateSynchronizer) runWorkspaceAndLog(ctx context.Context, workspaceID string, endedAt time.Time) {
	leaseKey := commercialStateLeaseKey(workspaceID)
	initial := endedAt.Add(-366 * 24 * time.Hour)
	watermark, acquired, err := s.repo.TryAcquireSignalEvaluatorLease(
		ctx, leaseKey, s.owner, endedAt, 20*time.Minute, initial,
	)
	if err != nil || !acquired {
		if err != nil {
			slog.WarnContext(ctx, "CRM commercial-state workspace lease failed", "error", err, "workspace_id", workspaceID)
		}
		return
	}
	start := watermark.Add(-24 * time.Hour)
	if start.Before(initial) {
		start = initial
	}
	accepted, err := s.runWorkspaceSweep(ctx, workspaceID, start, endedAt)
	if err != nil {
		_ = s.repo.AbandonSignalEvaluatorLease(ctx, leaseKey, s.owner)
		slog.WarnContext(ctx, "CRM commercial-state workspace synchronization failed", "error", err, "workspace_id", workspaceID)
		return
	}
	if err := s.repo.ReleaseSignalEvaluatorLease(ctx, leaseKey, s.owner, endedAt); err != nil {
		slog.WarnContext(ctx, "CRM commercial-state workspace lease release failed", "error", err, "workspace_id", workspaceID)
		return
	}
	if accepted > 0 {
		slog.InfoContext(ctx, "CRM commercial-state workspace synchronization complete",
			"workspace_id", workspaceID, "accepted", accepted)
	}
}

func commercialStateLeaseKey(workspaceID string) string {
	return "commercial_state_sync:" + workspaceID
}

func (s *CRMCommercialStateSynchronizer) RunSweep(ctx context.Context, start, end time.Time) (int, error) {
	if s == nil || s.service == nil || s.repo == nil || s.eventsDB == nil || s.projects == nil {
		return 0, fmt.Errorf("commercial-state synchronizer is not configured")
	}
	workspaceIDs, err := s.projects.ListAllActiveEventWorkspaceIDs(ctx)
	if err != nil {
		return 0, err
	}
	accepted := 0
	var sweepErrors []error
	for _, workspaceID := range workspaceIDs {
		workspaceAccepted, err := s.runWorkspaceSweep(ctx, workspaceID, start, end)
		accepted += workspaceAccepted
		if err != nil {
			sweepErrors = append(sweepErrors, fmt.Errorf("workspace %s: %w", workspaceID, err))
		}
	}
	return accepted, errors.Join(sweepErrors...)
}

func (s *CRMCommercialStateSynchronizer) runWorkspaceSweep(
	ctx context.Context,
	workspaceID string,
	start, end time.Time,
) (int, error) {
	reader, err := repository.NewClickHouseEventRepository(s.eventsDB, s.projects, workspaceID)
	if err != nil {
		return 0, err
	}
	events, err := reader.ListCommercialStateEvents(ctx, start, end)
	if err != nil {
		return 0, err
	}
	externalIDs := make([]string, 0, len(events))
	for _, event := range events {
		externalIDs = append(externalIDs, event.CompanyExternalID)
	}
	companyIDs, err := s.repo.ResolveCompanyIDsByExternalID(ctx, workspaceID, externalIDs)
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, event := range events {
		companyID, ok := companyIDs[event.CompanyExternalID]
		if !ok {
			continue
		}
		changed, err := s.service.MaterializeCompanyCommercialState(
			ctx, workspaceID, companyID, event.EventID, event.IdentityMethod,
			model.JSONB(event.Patch), event.StateUpdatedAt, end,
		)
		if err != nil {
			slog.WarnContext(ctx, "commercial-state event rejected", "error", err,
				"workspace_id", workspaceID, "company_external_id", event.CompanyExternalID, "event_id", event.EventID)
			continue
		}
		if changed {
			accepted++
		}
	}
	return accepted, nil
}
