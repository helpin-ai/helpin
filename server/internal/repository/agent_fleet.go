package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentRunFleetAggregate contains the bounded aggregate fields needed by the
// agent fleet page. The query count is constant regardless of agent count.
type AgentRunFleetAggregate struct {
	AgentID         string `gorm:"column:agent_id"`
	RecentRuns      int    `gorm:"column:recent_runs"`
	RecentCompleted int    `gorm:"column:recent_completed"`
	RecentFailed    int    `gorm:"column:recent_failed"`
	RecentTokens    int    `gorm:"column:recent_tokens"`
}

// SummarizeFleetSince aggregates recent run activity for the requested agents.
func (r *AgentRunRepository) SummarizeFleetSince(
	ctx context.Context,
	workspaceID string,
	agentIDs []string,
	since time.Time,
) ([]AgentRunFleetAggregate, error) {
	if len(agentIDs) == 0 {
		return []AgentRunFleetAggregate{}, nil
	}

	var rows []AgentRunFleetAggregate
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Select(`
			agent_id,
			COUNT(*) AS recent_runs,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS recent_completed,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS recent_failed,
			COALESCE(SUM(tokens_used), 0) AS recent_tokens`, model.AgentRunStatusCompleted, model.AgentRunStatusFailed).
		Where("workspace_id = ? AND agent_id IN ? AND created_at >= ?", workspaceID, agentIDs, since).
		Group("agent_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("summarize agent fleet runs: %w", err)
	}
	return rows, nil
}

// ListRecentByAgentIDs returns at most limit recent runs per agent. Windowing
// in SQL avoids loading an arbitrary workspace-wide page and discarding most
// of it in the frontend.
func (r *AgentRunRepository) ListRecentByAgentIDs(
	ctx context.Context,
	workspaceID string,
	agentIDs []string,
	limit int,
) ([]model.AgentRun, error) {
	if len(agentIDs) == 0 || limit <= 0 {
		return []model.AgentRun{}, nil
	}

	ranked := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Select(_agentRunListColumns+", ROW_NUMBER() OVER (PARTITION BY agent_id ORDER BY created_at DESC, id DESC) AS fleet_row_number").
		Where("workspace_id = ? AND agent_id IN ?", workspaceID, agentIDs)

	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Table("(?) AS ranked_agent_runs", ranked).
		Select(_agentRunListColumns).
		Where("fleet_row_number <= ?", limit).
		Order("agent_id ASC, created_at DESC, id DESC").
		Scan(&runs).Error; err != nil {
		return nil, fmt.Errorf("list recent agent fleet runs: %w", err)
	}
	return runs, nil
}

// ListPausedByAgentIDs returns paused runs that may require attention. Chat
// reply pauses are excluded because they represent an idle conversation, not
// an action for the workspace user.
func (r *AgentRunRepository) ListPausedByAgentIDs(
	ctx context.Context,
	workspaceID string,
	agentIDs []string,
) ([]model.AgentRun, error) {
	if len(agentIDs) == 0 {
		return []model.AgentRun{}, nil
	}

	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Select(_agentRunListColumns).
		Where("workspace_id = ? AND agent_id IN ?", workspaceID, agentIDs).
		Where("status = ?", model.AgentRunStatusPaused).
		Where("COALESCE(pause_reason, '') <> ?", model.AgentRunPauseReasonUserMessage).
		Order("agent_id ASC, created_at DESC, id DESC").
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list paused agent fleet runs: %w", err)
	}
	return runs, nil
}
