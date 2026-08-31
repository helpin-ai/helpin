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

// CRMUsageBaselineSynchronizer builds workspace-local weekday expectations at
// most once per day. A shared lease prevents replicas from duplicating the
// retained-event scans.
type CRMUsageBaselineSynchronizer struct {
	repo     *repository.CRMSignalRepository
	projects repository.EventWorkspaceResolver
	eventsDB *sql.DB
	owner    string
}

func NewCRMUsageBaselineSynchronizer(
	repo *repository.CRMSignalRepository,
	projects repository.EventWorkspaceResolver,
	eventsDB *sql.DB,
) *CRMUsageBaselineSynchronizer {
	return &CRMUsageBaselineSynchronizer{repo: repo, projects: projects, eventsDB: eventsDB, owner: uuid.NewString()}
}

func (s *CRMUsageBaselineSynchronizer) Run(ctx context.Context) {
	if s == nil || s.repo == nil || s.projects == nil || s.eventsDB == nil {
		return
	}
	s.runAndLog(ctx)
	ticker := time.NewTicker(24 * time.Hour)
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

func (s *CRMUsageBaselineSynchronizer) runAndLog(ctx context.Context) {
	now := time.Now().UTC()
	workspaceIDs, err := s.projects.ListAllActiveEventWorkspaceIDs(ctx)
	if err != nil {
		slog.WarnContext(ctx, "list workspaces for CRM usage baselines failed", "error", err)
		return
	}
	for _, workspaceID := range workspaceIDs {
		s.runWorkspaceAndLog(ctx, workspaceID, now)
	}
}

func (s *CRMUsageBaselineSynchronizer) runWorkspaceAndLog(ctx context.Context, workspaceID string, now time.Time) {
	leaseKey := usageBaselineLeaseKey(workspaceID)
	initial := now.Add(-24 * time.Hour)
	_, acquired, err := s.repo.TryAcquireSignalEvaluatorLease(
		ctx, leaseKey, s.owner, now, 30*time.Minute, initial,
	)
	if err != nil || !acquired {
		if err != nil {
			slog.WarnContext(ctx, "CRM usage-baseline workspace lease failed", "error", err, "workspace_id", workspaceID)
		}
		return
	}
	written, err := s.runWorkspaceSweep(ctx, workspaceID, now)
	if err != nil {
		_ = s.repo.AbandonSignalEvaluatorLease(ctx, leaseKey, s.owner)
		slog.WarnContext(ctx, "CRM usage-baseline workspace synchronization failed", "error", err, "workspace_id", workspaceID)
		return
	}
	if err := s.repo.ReleaseSignalEvaluatorLease(ctx, leaseKey, s.owner, now); err != nil {
		slog.WarnContext(ctx, "CRM usage-baseline workspace lease release failed", "error", err, "workspace_id", workspaceID)
		return
	}
	if written > 0 {
		slog.InfoContext(ctx, "CRM usage-baseline workspace synchronization complete",
			"workspace_id", workspaceID, "rows", written)
	}
}

func usageBaselineLeaseKey(workspaceID string) string {
	return "crm_usage_baseline_sync:" + workspaceID
}

func (s *CRMUsageBaselineSynchronizer) RunSweep(ctx context.Context, now time.Time) (int, error) {
	workspaceIDs, err := s.projects.ListAllActiveEventWorkspaceIDs(ctx)
	if err != nil {
		return 0, err
	}
	written := 0
	var sweepErrors []error
	for _, workspaceID := range workspaceIDs {
		workspaceWritten, err := s.runWorkspaceSweep(ctx, workspaceID, now)
		written += workspaceWritten
		if err != nil {
			sweepErrors = append(sweepErrors, fmt.Errorf("workspace %s: %w", workspaceID, err))
		}
	}
	return written, errors.Join(sweepErrors...)
}

func (s *CRMUsageBaselineSynchronizer) runWorkspaceSweep(ctx context.Context, workspaceID string, now time.Time) (int, error) {
	timezone, err := s.repo.GetWorkspaceTimezone(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	windowEnd, err := workspaceLocalMidnightUTC(now, timezone)
	if err != nil {
		return 0, fmt.Errorf("invalid timezone %q", timezone)
	}
	reader, err := repository.NewClickHouseEventRepository(s.eventsDB, s.projects, workspaceID)
	if err != nil {
		return 0, err
	}
	workspaceBaselines := make([]model.CRMUsageWeekdayBaseline, 0)
	var metricErrors []error
	for _, metricKey := range []string{"feature_used", "workflow_failed"} {
		rows, err := reader.BuildUsageWeekdayBaselines(ctx, metricKey, windowEnd, timezone)
		if err != nil {
			metricErrors = append(metricErrors, fmt.Errorf("metric %s: %w", metricKey, err))
			continue
		}
		externalIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			externalIDs = append(externalIDs, row.CompanyExternalID)
		}
		companyIDs, err := s.repo.ResolveCompanyIDsByExternalID(ctx, workspaceID, externalIDs)
		if err != nil {
			metricErrors = append(metricErrors, fmt.Errorf("metric %s: %w", metricKey, err))
			continue
		}
		for _, row := range rows {
			companyID, ok := companyIDs[row.CompanyExternalID]
			if !ok {
				continue
			}
			workspaceBaselines = append(workspaceBaselines, model.CRMUsageWeekdayBaseline{
				WorkspaceID: workspaceID, CompanyID: companyID, MetricKey: row.MetricKey,
				Weekday: row.Weekday, MedianValue: row.MedianValue,
				ObservationCount: row.ObservationCount, CompleteWeeks: row.CompleteWeeks,
				IdentityMethod:  row.IdentityMethod,
				WindowStartedAt: row.WindowStartedAt, WindowEndedAt: row.WindowEndedAt,
				CalculatedAt: now,
			})
		}
	}
	if err := s.repo.UpsertUsageWeekdayBaselines(ctx, workspaceBaselines); err != nil {
		return 0, err
	}
	return len(workspaceBaselines), errors.Join(metricErrors...)
}

func workspaceLocalMidnightUTC(now time.Time, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	localNow := now.In(location)
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).UTC(), nil
}
