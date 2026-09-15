package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UpdateRuntimeProjection prevents a stale event handler from overwriting usage
// committed by another worker between settlement and the final projection save.
func (r *AgentRunRepository) UpdateRuntimeProjection(ctx context.Context, run *model.AgentRun) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := LoadRunUsageWatermark(tx, run.WorkspaceID, run.ID)
		if err != nil {
			return err
		}
		next, err := ParseRunUsageWatermark(run.OutputSummary)
		if err != nil {
			return err
		}
		if current.Turn > next.Turn || current.InputTokens > next.InputTokens ||
			current.OutputTokens > next.OutputTokens || current.CachedInputTokens > next.CachedInputTokens ||
			current.ReasoningOutputTokens > next.ReasoningOutputTokens {
			return ErrAIUsageWatermarkChanged
		}
		// Keep the existing legacy-schema fallback. A PostgreSQL statement error
		// aborts its transaction, so isolate the first attempt in a savepoint.
		err = tx.Transaction(func(attempt *gorm.DB) error {
			return attempt.Save(run).Error
		})
		if isMissingAgentRunVersionColumn(err) {
			err = tx.Omit("AgentVersionID", "agent_version_id").Save(run).Error
		}
		if err != nil {
			return fmt.Errorf("save runtime projection: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if r.triggerExecutionRepo != nil {
		if err := r.triggerExecutionRepo.SyncRunStatus(ctx, run); err != nil {
			slog.ErrorContext(ctx, "sync projected trigger execution status", "run_id", run.ID, "error", err)
		}
	}
	return nil
}

// RunUsageWatermark identifies the cumulative usage accepted with a run projection.
type RunUsageWatermark struct {
	Turn                  int `json:"turn"`
	InputTokens           int `json:"input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

// ParseRunUsageWatermark decodes the shared checkpoint carried in output summaries.
func ParseRunUsageWatermark(summary json.RawMessage) (RunUsageWatermark, error) {
	var body struct {
		Checkpoint RunUsageWatermark `json:"ai_usage_checkpoint"`
	}
	if len(summary) > 0 {
		if err := json.Unmarshal(summary, &body); err != nil {
			return RunUsageWatermark{}, fmt.Errorf("decode run usage watermark: %w", err)
		}
	}
	return body.Checkpoint, nil
}

// LoadRunUsageWatermark locks and reads a run checkpoint within the caller transaction.
func LoadRunUsageWatermark(tx *gorm.DB, workspaceID, runID string) (RunUsageWatermark, error) {
	var row struct {
		OutputSummary model.JSONBlob
	}
	query := tx.Model(&model.AgentRun{}).Select("output_summary").Where("id = ? AND workspace_id = ?", runID, workspaceID)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Take(&row).Error; err != nil {
		return RunUsageWatermark{}, fmt.Errorf("load run usage watermark: %w", err)
	}
	return ParseRunUsageWatermark(json.RawMessage(row.OutputSummary))
}

// ErrAIUsageWatermarkChanged requires reloading the accepted run before retrying usage.
var ErrAIUsageWatermarkChanged = errors.New("terminal usage watermark changed")
