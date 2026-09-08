package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UpdateRuntimeSummaryMarker atomically changes one marker without replacing
// usage or other fields from a potentially stale projection snapshot.
func (r *AgentRunRepository) UpdateRuntimeSummaryMarker(ctx context.Context, workspaceID, runID, key string, value json.RawMessage) error {
	if key == "" || !json.Valid(value) {
		return fmt.Errorf("invalid runtime summary marker")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ OutputSummary model.JSONBlob }
		query := tx.Model(&model.AgentRun{}).Select("output_summary").Where("id = ? AND workspace_id = ?", runID, workspaceID)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Take(&row).Error; err != nil {
			return fmt.Errorf("load runtime summary for marker: %w", err)
		}
		body := map[string]json.RawMessage{}
		if len(row.OutputSummary) > 0 {
			if err := json.Unmarshal(row.OutputSummary, &body); err != nil {
				return fmt.Errorf("decode runtime summary for marker: %w", err)
			}
		}
		if body == nil {
			body = map[string]json.RawMessage{}
		}
		// A delayed replay worker must not move the durable cursor backward.
		if key == "agent_runtime_v2_replay_through" {
			var current, next int64
			if err := json.Unmarshal(value, &next); err != nil {
				return err
			}
			if previous := body[key]; len(previous) > 0 {
				if err := json.Unmarshal(previous, &current); err != nil {
					return err
				}
			}
			if next <= current {
				return nil
			}
		}
		if key == "ai_usage_turn_start" {
			current, err := parseRunUsageWatermark(json.RawMessage(row.OutputSummary))
			if err != nil {
				return err
			}
			var requested runUsageWatermark
			if err := json.Unmarshal(value, &requested); err != nil {
				return err
			}
			current.BudgetTurn = 0
			if current != requested {
				return ErrAIUsageWatermarkChanged
			}
		}

		body[key] = value
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode runtime summary marker: %w", err)
		}
		return tx.Model(&model.AgentRun{}).Where("id = ? AND workspace_id = ?", runID, workspaceID).
			Update("output_summary", model.JSONBlob(encoded)).Error
	})
}
