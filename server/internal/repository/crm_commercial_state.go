package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UsageWeekdayBaselineView binds the latest persisted baseline to the external
// company identifier used by the event store.
type UsageWeekdayBaselineView struct {
	CompanyID         string
	CompanyExternalID string
	Weekday           int
	MedianValue       float64
	ObservationCount  int
	CompleteWeeks     int
	IdentityMethod    string
	WindowStartedAt   time.Time
	WindowEndedAt     time.Time
}

// GetCompanyCommercialState returns the newest accepted materialized state.
func (r *CRMSignalRepository) GetCompanyCommercialState(
	ctx context.Context,
	workspaceID, companyID string,
) (*model.CRMCompanyCommercialState, error) {
	var state model.CRMCompanyCommercialState
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND company_id = ?", workspaceID, companyID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get company commercial state: %w", err)
	}
	return &state, nil
}

// ResolveCompanyIDByExternalID binds event-pipeline company IDs to CRM companies.
func (r *CRMSignalRepository) ResolveCompanyIDByExternalID(
	ctx context.Context,
	workspaceID, externalID string,
) (*string, error) {
	var company model.CRMCompany
	err := r.db.WithContext(ctx).Select("id").
		Where("workspace_id = ? AND external_id = ?", workspaceID, externalID).
		First(&company).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve CRM company external ID: %w", err)
	}
	return &company.ID, nil
}

// ResolveCompanyIDsByExternalID batch-resolves tenant-bound event company IDs.
func (r *CRMSignalRepository) ResolveCompanyIDsByExternalID(
	ctx context.Context, workspaceID string, externalIDs []string,
) (map[string]string, error) {
	result := map[string]string{}
	if len(externalIDs) == 0 {
		return result, nil
	}
	var companies []model.CRMCompany
	if err := r.db.WithContext(ctx).Select("id", "external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("resolve CRM company external IDs: %w", err)
	}
	for _, company := range companies {
		if company.ExternalID != nil {
			result[*company.ExternalID] = company.ID
		}
	}
	return result, nil
}

// GetWorkspaceTimezone returns the IANA timezone used for local-day baselines.
func (r *CRMSignalRepository) GetWorkspaceTimezone(ctx context.Context, workspaceID string) (string, error) {
	var timezone string
	if err := r.db.WithContext(ctx).Table("workspaces").Where("id = ?", workspaceID).
		Pluck("timezone", &timezone).Error; err != nil {
		return "", fmt.Errorf("get workspace timezone: %w", err)
	}
	if timezone == "" {
		timezone = "UTC"
	}
	return timezone, nil
}

// UpsertUsageWeekdayBaseline persists the exact baseline used by customer rules.
func (r *CRMSignalRepository) UpsertUsageWeekdayBaseline(ctx context.Context, baseline *model.CRMUsageWeekdayBaseline) error {
	return r.UpsertUsageWeekdayBaselines(ctx, []model.CRMUsageWeekdayBaseline{*baseline})
}

// UpsertUsageWeekdayBaselines writes one daily baseline batch without per-row queries.
func (r *CRMSignalRepository) UpsertUsageWeekdayBaselines(ctx context.Context, baselines []model.CRMUsageWeekdayBaseline) error {
	if len(baselines) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "company_id"}, {Name: "metric_key"}, {Name: "weekday"}, {Name: "window_ended_at"}},
		DoUpdates: clause.AssignmentColumns([]string{"median_value", "observation_count", "complete_weeks", "identity_method", "calculated_at"}),
	}).CreateInBatches(&baselines, 500).Error
}

// ListLatestUsageWeekdayBaselines returns one complete current baseline per company.
func (r *CRMSignalRepository) ListLatestUsageWeekdayBaselines(
	ctx context.Context, workspaceID, metricKey string,
) ([]UsageWeekdayBaselineView, error) {
	var rows []UsageWeekdayBaselineView
	err := r.db.WithContext(ctx).Raw(`
		SELECT b.company_id, c.external_id AS company_external_id, b.weekday,
		       b.median_value, b.observation_count, b.complete_weeks,
		       b.identity_method, b.window_started_at, b.window_ended_at
		FROM crm_usage_weekday_baselines b
		JOIN crm_companies c ON c.workspace_id = b.workspace_id AND c.id = b.company_id
		JOIN (
			SELECT company_id, MAX(window_ended_at) AS window_ended_at
			FROM crm_usage_weekday_baselines
			WHERE workspace_id = ? AND metric_key = ?
			GROUP BY company_id
		) latest ON latest.company_id = b.company_id AND latest.window_ended_at = b.window_ended_at
		WHERE b.workspace_id = ? AND b.metric_key = ? AND c.external_id IS NOT NULL
		ORDER BY b.company_id, b.weekday`, workspaceID, metricKey, workspaceID, metricKey).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list latest usage weekday baselines: %w", err)
	}
	return rows, nil
}

// PruneUsageWeekdayBaselines removes superseded baseline windows. Evaluation
// reads only the newest complete window for each company, metric, and weekday.
func (r *CRMSignalRepository) PruneUsageWeekdayBaselines(ctx context.Context, before time.Time) (int64, error) {
	if !r.db.Migrator().HasTable(&model.CRMUsageWeekdayBaseline{}) {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Where("window_ended_at < ?", before).
		Delete(&model.CRMUsageWeekdayBaseline{})
	if result.Error != nil {
		return 0, fmt.Errorf("prune usage weekday baselines: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// AcceptCompanyCommercialState atomically updates current state, append-only history, and health.
func (r *CRMSignalRepository) AcceptCompanyCommercialState(
	ctx context.Context,
	workspaceID, companyID, sourceEventID string,
	patch, nextState model.JSONB,
	priorStateUpdatedAt, stateUpdatedAt, acceptedAt time.Time,
) (bool, error) {
	inserted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var companyCount int64
		if err := tx.Model(&model.CRMCompany{}).
			Where("workspace_id = ? AND id = ?", workspaceID, companyID).Count(&companyCount).Error; err != nil {
			return fmt.Errorf("validate commercial-state company: %w", err)
		}
		if companyCount == 0 {
			return fmt.Errorf("commercial-state company not found")
		}
		if rawOwner, supplied := patch["customer_success_owner_member_id"]; supplied && rawOwner != nil {
			ownerID, ok := rawOwner.(string)
			if !ok || ownerID == "" {
				return fmt.Errorf("customer_success_owner_member_id must be a member ID or null")
			}
			var ownerCount int64
			if err := tx.Model(&model.WorkspaceMember{}).
				Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, ownerID, model.WorkspaceMemberStatusActive).
				Count(&ownerCount).Error; err != nil {
				return fmt.Errorf("validate commercial-state CS owner: %w", err)
			}
			if ownerCount == 0 {
				return fmt.Errorf("customer_success_owner_member_id must be an active workspace member")
			}
		}
		var existingHistory int64
		if err := tx.Model(&model.CRMCompanyCommercialStateHistory{}).
			Where("workspace_id = ? AND source_event_id = ?", workspaceID, sourceEventID).Count(&existingHistory).Error; err != nil {
			return err
		}
		if existingHistory > 0 {
			return nil
		}
		var current model.CRMCompanyCommercialState
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND company_id = ?", workspaceID, companyID).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && !current.StateUpdatedAt.Equal(priorStateUpdatedAt) {
			return fmt.Errorf("commercial state changed concurrently")
		}
		version := 1
		if err == nil {
			version = current.Version + 1
		}
		history := model.CRMCompanyCommercialStateHistory{
			ID: uuid.NewString(), WorkspaceID: workspaceID, CompanyID: companyID, Version: version,
			Patch: patch, StateSnapshot: nextState, StateUpdatedAt: stateUpdatedAt,
			SourceEventID: sourceEventID, AppliedToCurrent: true, AcceptedAt: acceptedAt,
		}
		if err := tx.Create(&history).Error; err != nil {
			return fmt.Errorf("append commercial-state history: %w", err)
		}
		state := model.CRMCompanyCommercialState{
			CompanyID: companyID, WorkspaceID: workspaceID, Version: version, State: nextState,
			StateUpdatedAt: stateUpdatedAt, SourceEventID: sourceEventID, AcceptedAt: acceptedAt,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "company_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"version", "state", "state_updated_at", "source_event_id", "accepted_at", "updated_at"}),
		}).Create(&state).Error; err != nil {
			return fmt.Errorf("materialize commercial state: %w", err)
		}
		if rawOwner, supplied := patch["customer_success_owner_member_id"]; supplied {
			if err := tx.Model(&model.CRMCompany{}).
				Where("workspace_id = ? AND id = ?", workspaceID, companyID).
				Update("customer_success_owner_member_id", rawOwner).Error; err != nil {
				return fmt.Errorf("sync company CS owner: %w", err)
			}
		}
		health := model.CRMCompanyCommercialStateHealth{
			CompanyID: companyID, WorkspaceID: workspaceID, LastAcceptedAt: &acceptedAt,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "company_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"last_accepted_at": acceptedAt, "updated_at": acceptedAt}),
		}).Create(&health).Error; err != nil {
			return err
		}
		inserted = true
		return nil
	})
	return inserted, err
}

// RetainStaleCompanyCommercialState appends valid but out-of-order evidence
// without changing current state or integration-health rejection counters.
func (r *CRMSignalRepository) RetainStaleCompanyCommercialState(
	ctx context.Context,
	workspaceID, companyID, sourceEventID string,
	patch, currentSnapshot model.JSONB,
	stateUpdatedAt, acceptedAt time.Time,
) (bool, error) {
	var companyCount int64
	if err := r.db.WithContext(ctx).Model(&model.CRMCompany{}).
		Where("workspace_id = ? AND id = ?", workspaceID, companyID).Count(&companyCount).Error; err != nil {
		return false, fmt.Errorf("validate stale commercial-state company: %w", err)
	}
	if companyCount == 0 {
		return false, fmt.Errorf("commercial-state company not found")
	}
	history := model.CRMCompanyCommercialStateHistory{
		ID: uuid.NewString(), WorkspaceID: workspaceID, CompanyID: companyID,
		Version: 0, Patch: patch, StateSnapshot: currentSnapshot,
		StateUpdatedAt: stateUpdatedAt, SourceEventID: sourceEventID,
		AppliedToCurrent: false, AcceptedAt: acceptedAt,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "source_event_id"}},
		DoNothing: true,
	}).Create(&history)
	if result.Error != nil {
		return false, fmt.Errorf("retain stale commercial-state history: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// RecordRejectedCompanyCommercialState makes silent clock/schema failures visible.
func (r *CRMSignalRepository) RecordRejectedCompanyCommercialState(
	ctx context.Context,
	workspaceID, companyID, reason string,
	rejectedAt time.Time,
) error {
	health := model.CRMCompanyCommercialStateHealth{
		CompanyID: companyID, WorkspaceID: workspaceID, LastRejectedAt: &rejectedAt,
		LastRejectionReason: &reason, RejectedUpdateCount: 1,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_rejected_at": rejectedAt, "last_rejection_reason": reason,
			"rejected_update_count": gorm.Expr("rejected_update_count + 1"), "updated_at": rejectedAt,
		}),
	}).Create(&health).Error
}

// RecordSignalBatchSuppression makes workspace-wide anomaly guards auditable.
func (r *CRMSignalRepository) RecordSignalBatchSuppression(
	ctx context.Context, workspaceID, ruleKey string, ruleVersion, eligible, tripped int,
	start, end time.Time, reason string,
) error {
	ratio := 0.0
	if eligible > 0 {
		ratio = float64(tripped) / float64(eligible)
	}
	return r.db.WithContext(ctx).Create(&model.CRMSignalBatchSuppression{
		WorkspaceID: workspaceID, RuleKey: ruleKey, RuleVersion: ruleVersion,
		EligibleAccounts: eligible, TrippedAccounts: tripped, TrippedRatio: ratio,
		Reason: reason, EvaluationWindowStartedAt: start, EvaluationWindowEndedAt: end,
	}).Error
}
