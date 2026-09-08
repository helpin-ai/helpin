package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListRuntimeReconciliationCandidates returns mapped runs needing lifecycle or usage recovery.
func (r *AgentRunRepository) ListRuntimeReconciliationCandidates(ctx context.Context, externalRuntime string, olderThan time.Time, limit int, after *model.AgentRun) ([]model.AgentRun, error) {
	externalRuntime = strings.TrimSpace(externalRuntime)
	if externalRuntime == "" {
		return []model.AgentRun{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	pendingUsage := "output_summary->'ai_usage_metering' IS NOT NULL AND (COALESCE(output_summary->>'agent_runtime_usage_consumed', 'false') <> 'true' OR input_tokens > COALESCE((output_summary->'ai_usage_checkpoint'->>'input_tokens')::bigint, 0) OR output_tokens > COALESCE((output_summary->'ai_usage_checkpoint'->>'output_tokens')::bigint, 0))"
	if r.db.Dialector.Name() == "sqlite" {
		pendingUsage = "json_extract(output_summary, '$.ai_usage_metering') IS NOT NULL AND (COALESCE(json_extract(output_summary, '$.agent_runtime_usage_consumed'), 0) <> 1 OR input_tokens > COALESCE(json_extract(output_summary, '$.ai_usage_checkpoint.input_tokens'), 0) OR output_tokens > COALESCE(json_extract(output_summary, '$.ai_usage_checkpoint.output_tokens'), 0))"
	}

	query := r.db.WithContext(ctx)
	if after != nil {
		query = query.Where("updated_at > ? OR (updated_at = ? AND id > ?)", after.UpdatedAt, after.UpdatedAt, after.ID)
	}

	var runs []model.AgentRun
	if err := query.
		Where("external_runtime = ? AND external_runtime_id IS NOT NULL", externalRuntime).
		Where("status IN ? OR (status IN ? AND ("+pendingUsage+"))", []string{
			model.AgentRunStatusQueued, model.AgentRunStatusRunning, model.AgentRunStatusPaused,
		}, []string{model.AgentRunStatusCompleted, model.AgentRunStatusFailed, model.AgentRunStatusCancelled}).
		Where("updated_at < ?", olderThan).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list active agent runs by external runtime: %w", err)
	}
	return runs, nil
}
