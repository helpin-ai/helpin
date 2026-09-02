package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCoverageLeaseLost = errors.New("coverage work lease is no longer owned")

type CoverageV2Repository struct{ db *gorm.DB }

func NewCoverageV2Repository(db *gorm.DB) *CoverageV2Repository {
	return &CoverageV2Repository{db: db}
}

// EnqueueAttempt inserts immutable attempt history and is idempotent for the
// logical-work-key/attempt pair.
func (r *CoverageV2Repository) EnqueueAttempt(ctx context.Context, attempt *model.CoverageAnalysisAttempt) error {
	if r == nil || r.db == nil || attempt == nil {
		return fmt.Errorf("coverage repository and attempt are required")
	}
	if strings.TrimSpace(attempt.WorkspaceID) == "" || strings.TrimSpace(attempt.LogicalWorkKey) == "" || attempt.Attempt <= 0 {
		return fmt.Errorf("workspace, logical work key, and positive attempt are required")
	}
	if attempt.ID == "" {
		attempt.ID = uuid.NewString()
	}
	if attempt.Status == "" {
		attempt.Status = model.CoverageAttemptQueued
	}
	if attempt.Stage == "" {
		attempt.Stage = model.CoverageFailureCandidateQuery
	}
	if attempt.Metadata == nil {
		attempt.Metadata = []byte("{}")
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}, {Name: "logical_work_key"}, {Name: "attempt"}}, DoNothing: true,
	}).Create(attempt)
	if result.Error != nil {
		return fmt.Errorf("enqueue coverage attempt: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return r.db.WithContext(ctx).Where("workspace_id = ? AND logical_work_key = ? AND attempt = ?", attempt.WorkspaceID, attempt.LogicalWorkKey, attempt.Attempt).First(attempt).Error
	}
	return nil
}

// ClaimAttempts atomically leases bounded eligible work. PostgreSQL workers
// use SKIP LOCKED so one slow item never blocks other workers.
func (r *CoverageV2Repository) ClaimAttempts(ctx context.Context, owner string, now time.Time, leaseDuration time.Duration, limit int) ([]model.CoverageAnalysisAttempt, error) {
	if r == nil || r.db == nil || strings.TrimSpace(owner) == "" || leaseDuration <= 0 {
		return nil, fmt.Errorf("coverage repository, owner, and positive lease duration are required")
	}
	if limit <= 0 {
		return nil, nil
	}
	if limit > 100 {
		limit = 100
	}
	var claimed []model.CoverageAnalysisAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("((status IN ? AND (retry_at IS NULL OR retry_at <= ?)) OR (status = ? AND lease_expires_at <= ?))",
			[]string{model.CoverageAttemptQueued, model.CoverageAttemptRetryable}, now, model.CoverageAttemptLeased, now).
			Order("created_at ASC, id ASC").Limit(limit)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var candidates []model.CoverageAnalysisAttempt
		if err := query.Find(&candidates).Error; err != nil {
			return err
		}
		expiresAt := now.Add(leaseDuration)
		for idx := range candidates {
			candidate := &candidates[idx]
			result := tx.Model(&model.CoverageAnalysisAttempt{}).
				Where("id = ? AND ((status IN ? AND (retry_at IS NULL OR retry_at <= ?)) OR (status = ? AND lease_expires_at <= ?))",
					candidate.ID, []string{model.CoverageAttemptQueued, model.CoverageAttemptRetryable}, now, model.CoverageAttemptLeased, now).
				Updates(map[string]interface{}{"status": model.CoverageAttemptLeased, "lease_owner": owner, "lease_expires_at": expiresAt, "started_at": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				candidate.Status, candidate.LeaseOwner, candidate.LeaseExpiresAt = model.CoverageAttemptLeased, owner, &expiresAt
				claimed = append(claimed, *candidate)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim coverage attempts: %w", err)
	}
	return claimed, nil
}

func (r *CoverageV2Repository) CompleteAttempt(ctx context.Context, id, owner string, completedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", id, model.CoverageAttemptLeased, owner, completedAt).
		Updates(map[string]interface{}{"status": model.CoverageAttemptSucceeded, "lease_owner": "", "lease_expires_at": nil, "failure_class": "", "failure_message": "", "completed_at": completedAt})
	return coverageLeaseResult(result)
}

func (r *CoverageV2Repository) MarkRetryable(ctx context.Context, id, owner, failureClass, failureMessage string, retryAt time.Time, retryBudgetUsed int) error {
	now := time.Now().UTC()
	status := model.CoverageAttemptRetryable
	if failureClass == model.CoverageFailureConfiguration {
		status = model.CoverageAttemptPausedConfiguration
	}
	result := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", id, model.CoverageAttemptLeased, owner, now).
		Updates(map[string]interface{}{"status": status, "lease_owner": "", "lease_expires_at": nil, "retry_at": retryAt, "retry_budget_used": retryBudgetUsed, "failure_class": failureClass, "failure_message": failureMessage})
	return coverageLeaseResult(result)
}

func coverageLeaseResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCoverageLeaseLost
	}
	return nil
}

// ReplaceCurrentFinding keeps history and atomically advances the logical item.
func (r *CoverageV2Repository) ReplaceCurrentFinding(ctx context.Context, finding *model.CoverageFinding) error {
	if finding == nil || finding.WorkspaceID == "" || finding.LogicalWorkKey == "" {
		return fmt.Errorf("workspace and logical work key are required")
	}
	if finding.ID == "" {
		finding.ID = uuid.NewString()
	}
	if finding.Metadata == nil {
		finding.Metadata = []byte("{}")
	}
	finding.IsCurrent = true
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.CoverageFinding{}).
			Where("workspace_id = ? AND logical_work_key = ? AND is_current = ?", finding.WorkspaceID, finding.LogicalWorkKey, true).
			Updates(map[string]interface{}{"is_current": false, "superseded_at": now}).Error; err != nil {
			return err
		}
		return tx.Create(finding).Error
	})
}

// SetCurrentMembership closes the old decision before appending the new one.
func (r *CoverageV2Repository) SetCurrentMembership(ctx context.Context, membership *model.CoverageTopicMembership) error {
	if membership == nil || membership.WorkspaceID == "" || membership.FindingID == "" || membership.TopicID == "" || !model.IsCoverageMembershipSource(membership.DecisionSource) {
		return fmt.Errorf("valid workspace, finding, topic, and decision source are required")
	}
	if membership.ID == "" {
		membership.ID = uuid.NewString()
	}
	if membership.ValidFrom.IsZero() {
		membership.ValidFrom = time.Now().UTC()
	}
	if membership.Metadata == nil {
		membership.Metadata = []byte("{}")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.CoverageTopicMembership{}).
			Where("workspace_id = ? AND finding_id = ? AND valid_to IS NULL", membership.WorkspaceID, membership.FindingID).
			Update("valid_to", membership.ValidFrom).Error; err != nil {
			return err
		}
		return tx.Create(membership).Error
	})
}
