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

func (r *CoverageV2Repository) CreateBatch(ctx context.Context, batch *model.CoverageBatch) (*model.CoverageBatch, error) {
	if r == nil || r.db == nil || batch == nil || batch.WorkspaceID == "" || !batch.WindowStart.Before(batch.WindowEnd) {
		return nil, fmt.Errorf("valid coverage batch is required")
	}
	if batch.ID == "" {
		batch.ID = uuid.NewString()
	}
	if batch.Status == "" {
		batch.Status = model.CoverageBatchRunning
	}
	if batch.Metadata == nil {
		batch.Metadata = []byte("{}")
	}
	now := time.Now().UTC()
	if batch.StartedAt == nil {
		batch.StartedAt = &now
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}, {Name: "window_start"}, {Name: "window_end"}, {Name: "analyzer_version"}, {Name: "policy_version"}}, DoNothing: true,
	}).Create(batch)
	if result.Error != nil {
		return nil, fmt.Errorf("create coverage batch: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return batch, nil
	}
	var existing model.CoverageBatch
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND window_start = ? AND window_end = ? AND analyzer_version = ? AND policy_version = ?", batch.WorkspaceID, batch.WindowStart, batch.WindowEnd, batch.AnalyzerVersion, batch.PolicyVersion).First(&existing).Error
	if err != nil {
		return nil, fmt.Errorf("load coverage batch: %w", err)
	}
	return &existing, nil
}

func (r *CoverageV2Repository) CompleteBatch(ctx context.Context, id, status string, candidateCount, succeededCount, retryableCount, deadLetterCount int, failureClass, failureMessage string) error {
	if !model.IsCoverageBatchStatus(status) {
		return fmt.Errorf("unsupported coverage batch status %q", status)
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&model.CoverageBatch{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": status, "candidate_count": candidateCount, "succeeded_count": succeededCount,
		"retryable_count": retryableCount, "dead_letter_count": deadLetterCount,
		"failure_class": failureClass, "failure_message": failureMessage, "completed_at": now, "updated_at": now,
	})
	if result.Error != nil {
		return fmt.Errorf("complete coverage batch: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("coverage batch %q not found", id)
	}
	return nil
}

// StartAnalysisAttempt appends a new leased attempt after prior non-success.
// Duplicate delivery while a live lease exists returns that same attempt.
func (r *CoverageV2Repository) StartAnalysisAttempt(ctx context.Context, batchID, workspaceID, logicalWorkKey, sourceKind, sourceID, segmentID, contentHash, analyzerVersion, policyVersion, owner string, leaseDuration time.Duration) (*model.CoverageAnalysisAttempt, error) {
	if r == nil || r.db == nil || batchID == "" || workspaceID == "" || logicalWorkKey == "" || owner == "" || leaseDuration <= 0 {
		return nil, fmt.Errorf("batch, workspace, work key, owner, and lease duration are required")
	}
	var started model.CoverageAnalysisAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var latest model.CoverageAnalysisAttempt
		err := tx.Where("workspace_id = ? AND logical_work_key = ?", workspaceID, logicalWorkKey).Order("attempt DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now().UTC()
		if err == nil && latest.Status == model.CoverageAttemptSucceeded {
			started = latest
			return nil
		}
		if err == nil && latest.Status == model.CoverageAttemptLeased && latest.LeaseExpiresAt != nil && latest.LeaseExpiresAt.After(now) {
			started = latest
			return nil
		}
		attemptNumber := 1
		if err == nil {
			attemptNumber = latest.Attempt + 1
		}
		expiresAt := now.Add(leaseDuration)
		started = model.CoverageAnalysisAttempt{
			ID: uuid.NewString(), WorkspaceID: workspaceID, BatchID: batchID, LogicalWorkKey: logicalWorkKey,
			SourceKind: sourceKind, SourceID: sourceID, SegmentID: segmentID, ContentHash: contentHash,
			AnalyzerVersion: analyzerVersion, PolicyVersion: policyVersion, Attempt: attemptNumber,
			Status: model.CoverageAttemptLeased, Stage: model.CoverageFailureLLMPreflight,
			LeaseOwner: owner, LeaseExpiresAt: &expiresAt, Metadata: []byte("{}"), StartedAt: &now,
		}
		return tx.Create(&started).Error
	})
	if err != nil {
		return nil, fmt.Errorf("start coverage analysis attempt: %w", err)
	}
	return &started, nil
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

func (r *CoverageV2Repository) ListOpenTopics(ctx context.Context, workspaceID string, limit int) ([]model.CoverageTopicV2, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	var topics []model.CoverageTopicV2
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status = ?", workspaceID, model.CoverageTopicOpen).Order("conversation_count DESC, id ASC").Limit(limit).Find(&topics).Error
	return topics, err
}

func (r *CoverageV2Repository) CreateTopic(ctx context.Context, topic *model.CoverageTopicV2) (*model.CoverageTopicV2, error) {
	if topic == nil || topic.WorkspaceID == "" || topic.CanonicalKey == "" {
		return nil, fmt.Errorf("valid coverage topic is required")
	}
	if topic.ID == "" {
		topic.ID = uuid.NewString()
	}
	if topic.Status == "" {
		topic.Status = model.CoverageTopicOpen
	}
	if topic.Metadata == nil {
		topic.Metadata = []byte("{}")
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "canonical_key"}}, DoNothing: true}).Create(topic)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		return topic, nil
	}
	var existing model.CoverageTopicV2
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND canonical_key = ?", topic.WorkspaceID, topic.CanonicalKey).First(&existing).Error
	return &existing, err
}

func (r *CoverageV2Repository) AppendAssignmentAttempt(ctx context.Context, attempt *model.CoverageAssignmentAttempt) error {
	if attempt == nil || attempt.WorkspaceID == "" || attempt.FindingID == "" {
		return fmt.Errorf("valid assignment attempt is required")
	}
	if attempt.ID == "" {
		attempt.ID = uuid.NewString()
	}
	if attempt.Metadata == nil {
		attempt.Metadata = []byte("{}")
	}
	if attempt.Attempt <= 0 {
		var latest int
		if err := r.db.WithContext(ctx).Model(&model.CoverageAssignmentAttempt{}).Where("workspace_id = ? AND finding_id = ?", attempt.WorkspaceID, attempt.FindingID).Select("COALESCE(MAX(attempt), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		attempt.Attempt = latest + 1
	}
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *CoverageV2Repository) UpdateFindingAssignmentStatus(ctx context.Context, workspaceID, findingID, status string) error {
	return r.db.WithContext(ctx).Model(&model.CoverageFinding{}).Where("workspace_id = ? AND id = ? AND is_current = ?", workspaceID, findingID, true).Update("assignment_status", status).Error
}

func (r *CoverageV2Repository) UpsertUnreviewedSignal(ctx context.Context, signal *model.CoverageUnreviewedSignal) error {
	if signal == nil || signal.WorkspaceID == "" || signal.SignalKey == "" {
		return fmt.Errorf("valid unreviewed signal is required")
	}
	if signal.ID == "" {
		signal.ID = uuid.NewString()
	}
	if signal.Status == "" {
		signal.Status = model.CoverageSignalUnreviewed
	}
	if signal.Metadata == nil {
		signal.Metadata = []byte("{}")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if signal.SourceKind == "widget_search" && signal.SessionID != "" {
			var existing model.CoverageUnreviewedSignal
			err := tx.Where("workspace_id = ? AND source_kind = ? AND session_id = ? AND status = ? AND observed_at >= ? AND (? LIKE normalized_query || '%' OR normalized_query LIKE ? || '%')",
				signal.WorkspaceID, signal.SourceKind, signal.SessionID, model.CoverageSignalUnreviewed, signal.ObservedAt.Add(-10*time.Minute), signal.NormalizedQuery, signal.NormalizedQuery).
				Order("observed_at DESC").First(&existing).Error
			if err == nil {
				query := existing.NormalizedQuery
				if len(signal.NormalizedQuery) > len(query) {
					query = signal.NormalizedQuery
				}
				return tx.Model(&model.CoverageUnreviewedSignal{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
					"normalized_query": query, "meaningful_tokens": signal.MeaningfulTokens,
					"source_id": signal.SourceID, "observed_at": signal.ObservedAt, "updated_at": time.Now().UTC(),
				}).Error
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "signal_key"}}, DoNothing: true}).Create(signal).Error
	})
}

func (r *CoverageV2Repository) RefreshTopicCounts(ctx context.Context, workspaceID, topicID string) error {
	return r.db.WithContext(ctx).Exec(`UPDATE coverage_topics SET
		finding_count = (SELECT COUNT(DISTINCT m.finding_id) FROM coverage_topic_memberships m WHERE m.workspace_id = ? AND m.topic_id = ? AND m.valid_to IS NULL),
		conversation_count = (SELECT COUNT(DISTINCT f.conversation_id) FROM coverage_topic_memberships m JOIN coverage_findings f ON f.workspace_id = m.workspace_id AND f.id = m.finding_id WHERE m.workspace_id = ? AND m.topic_id = ? AND m.valid_to IS NULL AND f.is_current AND f.conversation_id IS NOT NULL),
		customer_count = (SELECT COUNT(DISTINCT f.customer_id) FROM coverage_topic_memberships m JOIN coverage_findings f ON f.workspace_id = m.workspace_id AND f.id = m.finding_id WHERE m.workspace_id = ? AND m.topic_id = ? AND m.valid_to IS NULL AND f.is_current AND f.customer_id IS NOT NULL),
		updated_at = ? WHERE workspace_id = ? AND id = ?`, workspaceID, topicID, workspaceID, topicID, workspaceID, topicID, time.Now().UTC(), workspaceID, topicID).Error
}
