package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIExecutionUsageRepository persists core usage and the corresponding run watermark.
type AIExecutionUsageRepository struct{ db *gorm.DB }

// NewAIExecutionUsageRepository creates a nonfinancial usage repository.
func NewAIExecutionUsageRepository(db *gorm.DB) *AIExecutionUsageRepository {
	return &AIExecutionUsageRepository{db: db}
}

// RecordExecutionUsage records an idempotent delta and its run summary atomically.
func (r *AIExecutionUsageRepository) RecordExecutionUsage(ctx context.Context, entry model.AIExecutionUsage, summary model.JSONBlob) error {
	if r == nil || r.db == nil || entry.IdempotencyKey == "" {
		return fmt.Errorf("usage repository and execution identity are required")
	}
	entry.ID = uuid.NewString()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current, requested RunUsageWatermark
		if entry.RunID != "" {
			if len(summary) == 0 {
				return fmt.Errorf("run usage requires its output summary")
			}
			var err error
			current, err = LoadRunUsageWatermark(tx, entry.WorkspaceID, entry.RunID)
			if err != nil {
				return err
			}
			requested, err = ParseRunUsageWatermark(json.RawMessage(summary))
			if err != nil {
				return err
			}
		}
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}, {Name: "idempotency_key"}}, DoNothing: true,
		}).Create(&entry)
		if result.Error != nil {
			return fmt.Errorf("record execution usage: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			var existing model.AIExecutionUsage
			if err := tx.Where("workspace_id = ? AND idempotency_key = ?", entry.WorkspaceID, entry.IdempotencyKey).First(&existing).Error; err != nil {
				return err
			}
			if !sameExecutionUsage(existing, entry) {
				return ErrAIUsageWatermarkChanged
			}
			if entry.RunID != "" && current != requested {
				return ErrAIUsageWatermarkChanged
			}
			// The original transaction already committed its watermark. Never
			// overwrite a newer run summary when an old event is retried.
			return nil
		}
		if entry.RunID != "" {
			if requested.Turn <= current.Turn || requested.InputTokens < current.InputTokens ||
				requested.OutputTokens < current.OutputTokens || requested.CachedInputTokens < current.CachedInputTokens ||
				requested.ReasoningOutputTokens < current.ReasoningOutputTokens {
				return ErrAIUsageWatermarkChanged
			}
			result := tx.Model(&model.AgentRun{}).Where("id = ? AND workspace_id = ?", entry.RunID, entry.WorkspaceID).Update("output_summary", summary)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("usage run not found in workspace")
			}
		}
		return nil
	})
}

func sameExecutionUsage(a, b model.AIExecutionUsage) bool {
	return a.RunID == b.RunID && a.FeatureKey == b.FeatureKey && a.Provider == b.Provider && a.Model == b.Model &&
		a.InputTokens == b.InputTokens && a.OutputTokens == b.OutputTokens && a.ReasoningTokens == b.ReasoningTokens &&
		a.CacheReadTokens == b.CacheReadTokens && a.CacheWriteTokens == b.CacheWriteTokens &&
		a.MeasurementStatus == b.MeasurementStatus && bytes.Equal(a.PaidTools, b.PaidTools)
}
