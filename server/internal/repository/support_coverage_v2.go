package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CoverageRebuildSearch struct {
	SourceID         string
	SessionID        string
	Query            string
	MeaningfulTokens int
	ObservedAt       time.Time
}

type CoverageRebuildPreview struct {
	LegacyGapCount  int
	EvidenceCount   int
	ConversationIDs []string
	Searches        []CoverageRebuildSearch
}

type CoverageRebuildApplyResult struct {
	AuditID         string
	QueuedWorkCount int
}

type CoverageRolloutStats struct {
	AttemptCount                      int64
	TerminalFailureCount              int64
	DuplicateIdempotencyViolations    int64
	CrossWorkspaceInvariantViolations int64
}

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

// RefreshBatchStatus derives batch health from its immutable attempt history.
func (r *CoverageV2Repository) RefreshBatchStatus(ctx context.Context, batchID string) error {
	if strings.TrimSpace(batchID) == "" {
		return fmt.Errorf("batch_id is required")
	}
	type statusCount struct {
		Status string
		Count  int
	}
	var counts []statusCount
	if err := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).
		Select("status, COUNT(*) AS count").Where("batch_id = ?", batchID).Group("status").Scan(&counts).Error; err != nil {
		return err
	}
	total, succeeded, retryable, deadLetter, queued := 0, 0, 0, 0, 0
	for _, count := range counts {
		total += count.Count
		switch count.Status {
		case model.CoverageAttemptSucceeded:
			succeeded += count.Count
		case model.CoverageAttemptDeadLetter:
			deadLetter += count.Count
		case model.CoverageAttemptQueued:
			queued += count.Count
		case model.CoverageAttemptRetryable, model.CoverageAttemptPausedConfiguration:
			retryable += count.Count
		}
	}
	status := model.CoverageBatchRunning
	if total > 0 && queued == total {
		status = model.CoverageBatchQueued
	} else if total > 0 && succeeded+deadLetter == total {
		status = model.CoverageBatchSucceeded
		if deadLetter > 0 {
			status = model.CoverageBatchPartialFailed
		}
	}
	updates := map[string]interface{}{
		"status": status, "candidate_count": total, "succeeded_count": succeeded,
		"retryable_count": retryable, "dead_letter_count": deadLetter, "updated_at": time.Now().UTC(),
	}
	if status == model.CoverageBatchSucceeded || status == model.CoverageBatchPartialFailed {
		updates["completed_at"] = time.Now().UTC()
	}
	result := r.db.WithContext(ctx).Model(&model.CoverageBatch{}).Where("id = ?", batchID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("coverage batch %q not found", batchID)
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
	return r.claimAttempts(ctx, "", owner, now, leaseDuration, limit)
}

func (r *CoverageV2Repository) ClaimAttemptsForWorkspace(ctx context.Context, workspaceID, owner string, now time.Time, leaseDuration time.Duration, limit int) ([]model.CoverageAnalysisAttempt, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return r.claimAttempts(ctx, workspaceID, owner, now, leaseDuration, limit)
}

func (r *CoverageV2Repository) claimAttempts(ctx context.Context, workspaceID, owner string, now time.Time, leaseDuration time.Duration, limit int) ([]model.CoverageAnalysisAttempt, error) {
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
		if workspaceID != "" {
			query = query.Where("workspace_id = ?", workspaceID)
		}
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

func (r *CoverageV2Repository) ListQueuedWorkspaces(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	var workspaceIDs []string
	err := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).Distinct("workspace_id").
		Where("status IN ? AND (retry_at IS NULL OR retry_at <= ?)", []string{model.CoverageAttemptQueued, model.CoverageAttemptRetryable}, time.Now().UTC()).
		Order("workspace_id").Limit(limit).Pluck("workspace_id", &workspaceIDs).Error
	return workspaceIDs, err
}

func (r *CoverageV2Repository) CompleteAttempt(ctx context.Context, id, owner string, completedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", id, model.CoverageAttemptLeased, owner, completedAt).
		Updates(map[string]interface{}{"status": model.CoverageAttemptSucceeded, "lease_owner": "", "lease_expires_at": nil, "failure_class": "", "failure_message": "", "completed_at": completedAt})
	return coverageLeaseResult(result)
}

func (r *CoverageV2Repository) MarkRetryable(ctx context.Context, id, owner, failureClass, failureMessage string, retryAt time.Time, retryBudgetUsed int) error {
	status := model.CoverageAttemptRetryable
	if failureClass == model.CoverageFailureConfiguration {
		status = model.CoverageAttemptPausedConfiguration
	}
	return r.FailAttempt(ctx, id, owner, status, failureClass, failureMessage, &retryAt, retryBudgetUsed)
}

func (r *CoverageV2Repository) FailAttempt(ctx context.Context, id, owner, status, failureClass, failureMessage string, retryAt *time.Time, retryBudgetUsed int) error {
	if status != model.CoverageAttemptRetryable && status != model.CoverageAttemptDeadLetter && status != model.CoverageAttemptPausedConfiguration {
		return fmt.Errorf("unsupported coverage failure status %q", status)
	}
	now := time.Now().UTC()
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
		var finding model.CoverageFinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", membership.WorkspaceID, membership.FindingID).First(&finding).Error; err != nil {
			return err
		}

		// The deterministic fallback and semantic path share this final fence.
		// A provider failure must not allow automatic work to replace a review.
		if membership.DecisionSource == model.CoverageMembershipAutomatic {
			if !finding.IsCurrent {
				return nil
			}
			var protected int64
			if err := tx.Model(&model.CoverageTopicMembership{}).Where("workspace_id = ? AND finding_id = ? AND valid_to IS NULL AND decision_source <> ?", membership.WorkspaceID, membership.FindingID, model.CoverageMembershipAutomatic).Count(&protected).Error; err != nil {
				return err
			}
			if protected > 0 {
				return nil
			}
			if err := tx.Model(&model.CoverageUnreviewedSignal{}).Where("workspace_id = ? AND finding_id = ? AND status <> ?", membership.WorkspaceID, membership.FindingID, model.CoverageSignalUnreviewed).Count(&protected).Error; err != nil {
				return err
			}
			if protected > 0 {
				return nil
			}
		}

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

func (r *CoverageV2Repository) ClaimFindingsForAssignment(ctx context.Context, workspaceID string, limit int) ([]model.CoverageFinding, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var claimed []model.CoverageFinding
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("workspace_id = ? AND is_current = ? AND assignment_status IN ?", workspaceID, true, []string{"pending", "retryable"}).Order("created_at, id").Limit(limit)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var candidates []model.CoverageFinding
		if err := query.Find(&candidates).Error; err != nil {
			return err
		}
		for _, candidate := range candidates {
			result := tx.Model(&model.CoverageFinding{}).
				Where("workspace_id = ? AND id = ? AND is_current = ? AND assignment_status IN ?", workspaceID, candidate.ID, true, []string{"pending", "retryable"}).
				Update("assignment_status", "assigning")
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				candidate.AssignmentStatus = "assigning"
				claimed = append(claimed, candidate)
			}
		}
		return nil
	})
	return claimed, err
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

func (r *CoverageV2Repository) ListTopics(ctx context.Context, workspaceID string, limit, offset int) ([]model.CoverageTopicV2, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var topics []model.CoverageTopicV2
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status <> ?", workspaceID, model.CoverageTopicArchived).
		Order("conversation_count DESC, customer_count DESC, updated_at DESC, id ASC").Limit(limit).Offset(max(0, offset)).Find(&topics).Error
	return topics, err
}

func (r *CoverageV2Repository) CountTopics(ctx context.Context, workspaceID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.CoverageTopicV2{}).
		Where("workspace_id = ? AND status <> ?", workspaceID, model.CoverageTopicArchived).Count(&total).Error
	return total, err
}

func (r *CoverageV2Repository) GetTopic(ctx context.Context, workspaceID, topicID string) (*model.CoverageTopicV2, []model.CoverageFinding, error) {
	var topic model.CoverageTopicV2
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, topicID).First(&topic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var findings []model.CoverageFinding
	err := r.db.WithContext(ctx).Table("coverage_findings f").Select("f.*").
		Joins("JOIN coverage_topic_memberships m ON m.workspace_id = f.workspace_id AND m.finding_id = f.id AND m.valid_to IS NULL").
		Where("f.workspace_id = ? AND m.topic_id = ? AND f.is_current = ?", workspaceID, topicID, true).
		Order("f.created_at DESC").Find(&findings).Error
	return &topic, findings, err
}

func (r *CoverageV2Repository) ListUnreviewedSignals(ctx context.Context, workspaceID string, limit, offset int) ([]model.CoverageUnreviewedSignal, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var signals []model.CoverageUnreviewedSignal
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status = ?", workspaceID, model.CoverageSignalUnreviewed).
		Order("observed_at DESC, id ASC").Limit(limit).Offset(max(0, offset)).Find(&signals).Error
	return signals, err
}

func (r *CoverageV2Repository) CountUnreviewedSignals(ctx context.Context, workspaceID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.CoverageUnreviewedSignal{}).
		Where("workspace_id = ? AND status = ?", workspaceID, model.CoverageSignalUnreviewed).Count(&total).Error
	return total, err
}

func (r *CoverageV2Repository) ReviewSignal(ctx context.Context, workspaceID, signalID, topicID, actorID string) error {
	if workspaceID == "" || signalID == "" || topicID == "" || actorID == "" {
		return fmt.Errorf("workspace, signal, topic, and actor are required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var signal model.CoverageUnreviewedSignal
		if err := tx.Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, signalID, model.CoverageSignalUnreviewed).First(&signal).Error; err != nil {
			return err
		}
		var topicCount int64
		if err := tx.Model(&model.CoverageTopicV2{}).Where("workspace_id = ? AND id = ?", workspaceID, topicID).Count(&topicCount).Error; err != nil || topicCount != 1 {
			return fmt.Errorf("coverage topic not found in workspace")
		}
		now := time.Now().UTC()
		oldTopicID := ""
		if signal.FindingID != nil {
			var finding model.CoverageFinding
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("workspace_id = ? AND id = ?", workspaceID, *signal.FindingID).First(&finding).Error; err != nil {
				return err
			}

			var current model.CoverageTopicMembership
			if err := tx.Where("workspace_id = ? AND finding_id = ? AND valid_to IS NULL", workspaceID, *signal.FindingID).First(&current).Error; err == nil {
				oldTopicID = current.TopicID
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := lockCoverageTopics(tx, workspaceID, oldTopicID, topicID); err != nil {
				return err
			}
			if err := tx.Model(&model.CoverageTopicMembership{}).Where("workspace_id = ? AND finding_id = ? AND valid_to IS NULL", workspaceID, *signal.FindingID).Update("valid_to", now).Error; err != nil {
				return err
			}
			membership := &model.CoverageTopicMembership{ID: uuid.NewString(), WorkspaceID: workspaceID, FindingID: *signal.FindingID, TopicID: topicID, DecisionSource: model.CoverageMembershipManual, Confidence: 1, PolicyVersion: "v1", ActorID: &actorID, ValidFrom: now, Metadata: []byte("{}")}
			if err := tx.Create(membership).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&model.CoverageUnreviewedSignal{}).Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, signalID, model.CoverageSignalUnreviewed).
			Updates(map[string]interface{}{"status": model.CoverageSignalAttached, "topic_id": topicID, "reviewed_by": actorID, "reviewed_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("unreviewed coverage signal not found")
		}
		txRepo := NewCoverageV2Repository(tx)
		if err := txRepo.RefreshTopicCounts(ctx, workspaceID, topicID); err != nil {
			return err
		}
		if oldTopicID != "" && oldTopicID != topicID {
			if err := txRepo.RefreshTopicCounts(ctx, workspaceID, oldTopicID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *CoverageV2Repository) DismissSignal(ctx context.Context, workspaceID, signalID, actorID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var signal model.CoverageUnreviewedSignal
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, signalID).First(&signal).Error; err != nil {
			return err
		}
		if signal.FindingID != nil {
			var finding model.CoverageFinding
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("workspace_id = ? AND id = ?", workspaceID, *signal.FindingID).First(&finding).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&model.CoverageUnreviewedSignal{}).Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, signalID, model.CoverageSignalUnreviewed).Updates(map[string]interface{}{"status": model.CoverageSignalDismissed, "reviewed_by": actorID, "reviewed_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("unreviewed coverage signal not found")
		}
		return nil
	})
}

func (r *CoverageV2Repository) LatestBatch(ctx context.Context, workspaceID string) (*model.CoverageBatch, error) {
	var batch model.CoverageBatch
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &batch, err
}

func (r *CoverageV2Repository) CoverageRolloutStats(ctx context.Context, workspaceID string, since time.Time) (CoverageRolloutStats, error) {
	var stats CoverageRolloutStats
	attempts := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).Where("workspace_id = ? AND created_at >= ?", workspaceID, since)
	if err := attempts.Count(&stats.AttemptCount).Error; err != nil {
		return stats, err
	}
	if err := attempts.Where("status = ?", model.CoverageAttemptDeadLetter).Count(&stats.TerminalFailureCount).Error; err != nil {
		return stats, err
	}
	duplicateSuccesses := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).
		Select("logical_work_key").Where("workspace_id = ? AND status = ?", workspaceID, model.CoverageAttemptSucceeded).
		Group("logical_work_key").Having("COUNT(*) > 1")
	if err := r.db.WithContext(ctx).Table("(?) AS duplicate_successes", duplicateSuccesses).Count(&stats.DuplicateIdempotencyViolations).Error; err != nil {
		return stats, err
	}
	var findingViolations, membershipViolations int64
	if err := r.db.WithContext(ctx).Table("coverage_findings f").
		Joins("JOIN coverage_analysis_attempts a ON a.id = f.analysis_attempt_id").
		Where("f.workspace_id <> a.workspace_id").Count(&findingViolations).Error; err != nil {
		return stats, err
	}
	if err := r.db.WithContext(ctx).Table("coverage_topic_memberships m").
		Joins("JOIN coverage_findings f ON f.id = m.finding_id").
		Joins("JOIN coverage_topics t ON t.id = m.topic_id").
		Where("m.workspace_id <> f.workspace_id OR m.workspace_id <> t.workspace_id").Count(&membershipViolations).Error; err != nil {
		return stats, err
	}
	stats.CrossWorkspaceInvariantViolations = findingViolations + membershipViolations
	return stats, nil
}

func (r *CoverageV2Repository) ListFailedAttempts(ctx context.Context, workspaceID string, limit int) ([]model.CoverageAnalysisAttempt, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var attempts []model.CoverageAnalysisAttempt
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status IN ?", workspaceID, []string{model.CoverageAttemptRetryable, model.CoverageAttemptDeadLetter, model.CoverageAttemptPausedConfiguration}).Order("created_at DESC").Limit(limit).Find(&attempts).Error
	return attempts, err
}

func (r *CoverageV2Repository) ReplayAttempt(ctx context.Context, workspaceID, attemptID string) error {
	result := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).
		Where("workspace_id = ? AND id = ? AND status IN ?", workspaceID, attemptID, []string{model.CoverageAttemptDeadLetter, model.CoverageAttemptPausedConfiguration, model.CoverageAttemptRetryable}).
		Updates(map[string]interface{}{"status": model.CoverageAttemptQueued, "retry_budget_used": 0, "failure_class": "", "failure_message": "", "retry_at": time.Now().UTC(), "lease_owner": "", "lease_expires_at": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("replayable coverage attempt not found")
	}
	return nil
}

func (r *CoverageV2Repository) ListArchivedV1Gaps(ctx context.Context, workspaceID string, limit, offset int) ([]model.SupportCoverageGap, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var gaps []model.SupportCoverageGap
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status = ?", workspaceID, model.SupportCoverageGapStatusArchivedV1).Order("last_seen_at DESC").Limit(limit).Offset(max(0, offset)).Find(&gaps).Error
	return gaps, err
}

func (r *CoverageV2Repository) PreviewRebuild(ctx context.Context, workspaceID string) (*CoverageRebuildPreview, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	preview := &CoverageRebuildPreview{}
	var legacyGapCount int64
	if err := r.db.WithContext(ctx).Model(&model.SupportCoverageGap{}).Where("workspace_id = ? AND status <> ?", workspaceID, model.SupportCoverageGapStatusArchivedV1).Count(&legacyGapCount).Error; err != nil {
		return nil, err
	}
	preview.LegacyGapCount = int(legacyGapCount)
	var evidenceCount int64
	if err := r.db.WithContext(ctx).Model(&model.SupportGapEvidence{}).Where("workspace_id = ?", workspaceID).Count(&evidenceCount).Error; err != nil {
		return nil, err
	}
	preview.EvidenceCount = int(evidenceCount)
	if err := r.db.WithContext(ctx).Model(&model.SupportGapEvidence{}).Distinct("conversation_id").Where("workspace_id = ? AND conversation_id IS NOT NULL", workspaceID).Order("conversation_id").Pluck("conversation_id", &preview.ConversationIDs).Error; err != nil {
		return nil, err
	}
	type searchRow struct {
		ID, WidgetSessionID, Excerpt string
		CreatedAt                    time.Time
	}
	var rows []searchRow
	if err := r.db.WithContext(ctx).Model(&model.SupportGapEvidence{}).Select("id, COALESCE(widget_session_id, '') AS widget_session_id, excerpt, created_at").Where("workspace_id = ? AND conversation_id IS NULL AND evidence_type = ?", workspaceID, model.SupportEventWidgetSearchPerformed).Order("created_at, id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		preview.Searches = append(preview.Searches, CoverageRebuildSearch{SourceID: row.ID, SessionID: row.WidgetSessionID, Query: row.Excerpt, ObservedAt: row.CreatedAt})
	}
	return preview, nil
}

func (r *CoverageV2Repository) ApplyRebuild(ctx context.Context, workspaceID, analyzerVersion, policyVersion string, preview *CoverageRebuildPreview, qualified []CoverageRebuildSearch) (*CoverageRebuildApplyResult, error) {
	if workspaceID == "" || preview == nil {
		return nil, fmt.Errorf("workspace and rebuild preview are required")
	}
	result := &CoverageRebuildApplyResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.CoverageRebuildAudit
		if err := tx.Where("workspace_id = ? AND status = ?", workspaceID, "applied").Order("created_at DESC").First(&existing).Error; err == nil {
			result.AuditID, result.QueuedWorkCount = existing.ID, existing.QueuedWorkCount
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var gaps []model.SupportCoverageGap
		if err := tx.Select("id", "status").Where("workspace_id = ? AND status <> ?", workspaceID, model.SupportCoverageGapStatusArchivedV1).Find(&gaps).Error; err != nil {
			return err
		}
		snapshot := make(map[string]string, len(gaps))
		for _, gap := range gaps {
			snapshot[gap.ID] = gap.Status
		}
		snapshotJSON, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		audit := &model.CoverageRebuildAudit{ID: uuid.NewString(), WorkspaceID: workspaceID, Status: "applied", AnalyzerVersion: analyzerVersion, PolicyVersion: policyVersion, LegacyGapCount: len(gaps), EvidenceCount: preview.EvidenceCount, ConversationCount: len(preview.ConversationIDs), SearchCount: len(preview.Searches), QueuedWorkCount: len(preview.ConversationIDs), QualifiedSearchCount: len(qualified), LegacyStatusSnapshot: snapshotJSON, CreatedAt: now}
		if err := tx.Create(audit).Error; err != nil {
			return err
		}
		if len(gaps) > 0 {
			if err := tx.Model(&model.SupportCoverageGap{}).Where("workspace_id = ? AND status <> ?", workspaceID, model.SupportCoverageGapStatusArchivedV1).Update("status", model.SupportCoverageGapStatusArchivedV1).Error; err != nil {
				return err
			}
		}
		batch := &model.CoverageBatch{ID: uuid.NewString(), WorkspaceID: workspaceID, WindowStart: now.Add(-time.Nanosecond), WindowEnd: now, AnalyzerVersion: analyzerVersion, PolicyVersion: policyVersion, Status: model.CoverageBatchQueued, CandidateCount: len(preview.ConversationIDs), Metadata: []byte(`{"source":"v1_rebuild"}`)}
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		for _, conversationID := range preview.ConversationIDs {
			workKey := coverageRebuildHash(workspaceID + ":conversation:" + conversationID + ":" + analyzerVersion + ":" + policyVersion)
			attempt := &model.CoverageAnalysisAttempt{ID: uuid.NewString(), WorkspaceID: workspaceID, BatchID: batch.ID, LogicalWorkKey: workKey, SourceKind: "conversation", SourceID: conversationID, ContentHash: "rebuild_pending", AnalyzerVersion: analyzerVersion, PolicyVersion: policyVersion, Attempt: 1, Status: model.CoverageAttemptQueued, Stage: model.CoverageFailureCandidateQuery, CorrelationID: audit.ID, Metadata: []byte(`{"source":"v1_rebuild"}`)}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "logical_work_key"}, {Name: "attempt"}}, DoNothing: true}).Create(attempt).Error; err != nil {
				return err
			}
		}
		for _, search := range qualified {
			signal := &model.CoverageUnreviewedSignal{ID: uuid.NewString(), WorkspaceID: workspaceID, SourceKind: "widget_search", SourceID: search.SourceID, SessionID: search.SessionID, NormalizedQuery: strings.ToLower(strings.Join(strings.Fields(search.Query), " ")), SignalKey: coverageRebuildHash(workspaceID + ":" + search.SessionID + ":" + strings.ToLower(strings.Join(strings.Fields(search.Query), " "))), MeaningfulTokens: search.MeaningfulTokens, Status: model.CoverageSignalUnreviewed, ObservedAt: search.ObservedAt, Metadata: []byte(`{"source":"v1_rebuild"}`)}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "signal_key"}}, DoNothing: true}).Create(signal).Error; err != nil {
				return err
			}
		}
		result.AuditID, result.QueuedWorkCount = audit.ID, len(preview.ConversationIDs)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("apply coverage rebuild: %w", err)
	}
	return result, nil
}

func (r *CoverageV2Repository) RollbackRebuild(ctx context.Context, workspaceID, auditID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var audit model.CoverageRebuildAudit
		if err := tx.Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, auditID, "applied").First(&audit).Error; err != nil {
			return err
		}
		var snapshot map[string]string
		if err := json.Unmarshal(audit.LegacyStatusSnapshot, &snapshot); err != nil {
			return err
		}
		for gapID, status := range snapshot {
			if err := tx.Model(&model.SupportCoverageGap{}).Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, gapID, model.SupportCoverageGapStatusArchivedV1).Update("status", status).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		return tx.Model(&model.CoverageRebuildAudit{}).Where("id = ?", audit.ID).Updates(map[string]interface{}{"status": "rolled_back", "rolled_back_at": now}).Error
	})
}

func (r *CoverageV2Repository) ResumeRebuild(ctx context.Context, workspaceID, auditID string) (*CoverageRebuildApplyResult, error) {
	var audit model.CoverageRebuildAudit
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, auditID, "applied").First(&audit).Error; err != nil {
		return nil, err
	}
	var queued int64
	if err := r.db.WithContext(ctx).Model(&model.CoverageAnalysisAttempt{}).Where("workspace_id = ? AND correlation_id = ? AND status IN ?", workspaceID, auditID, []string{model.CoverageAttemptQueued, model.CoverageAttemptRetryable, model.CoverageAttemptPausedConfiguration}).Count(&queued).Error; err != nil {
		return nil, err
	}
	return &CoverageRebuildApplyResult{AuditID: audit.ID, QueuedWorkCount: int(queued)}, nil
}

func coverageRebuildHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
