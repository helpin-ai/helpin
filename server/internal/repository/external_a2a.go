package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ExternalA2ARepository persists external A2A agent connections, their task
// contexts, per-run upload tokens and projection idempotency claims.
type ExternalA2ARepository struct {
	db *gorm.DB
}

func NewExternalA2ARepository(db *gorm.DB) *ExternalA2ARepository {
	return &ExternalA2ARepository{db: db}
}

func (r *ExternalA2ARepository) Create(ctx context.Context, agent *model.ExternalA2AAgent) error {
	if err := r.db.WithContext(ctx).Create(agent).Error; err != nil {
		return fmt.Errorf("create external A2A agent: %w", err)
	}
	return nil
}

func (r *ExternalA2ARepository) List(ctx context.Context, workspaceID string) ([]model.ExternalA2AAgent, error) {
	var agents []model.ExternalA2AAgent
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("list external A2A agents: %w", err)
	}
	return agents, nil
}

func (r *ExternalA2ARepository) Get(ctx context.Context, workspaceID, id string) (*model.ExternalA2AAgent, error) {
	return r.first(ctx, "workspace_id = ? AND id = ?", workspaceID, id)
}

// GetByAgentID resolves the connection linked to a Helpin agent.
func (r *ExternalA2ARepository) GetByAgentID(ctx context.Context, workspaceID, agentID string) (*model.ExternalA2AAgent, error) {
	return r.first(ctx, "workspace_id = ? AND agent_id = ?", workspaceID, agentID)
}

func (r *ExternalA2ARepository) first(ctx context.Context, query string, args ...any) (*model.ExternalA2AAgent, error) {
	var agent model.ExternalA2AAgent
	err := r.db.WithContext(ctx).Where(query, args...).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get external A2A agent: %w", err)
	}
	return &agent, nil
}

func (r *ExternalA2ARepository) Update(ctx context.Context, workspaceID, id string, updates map[string]any) error {
	updates["updated_at"] = time.Now()
	result := r.db.WithContext(ctx).Model(&model.ExternalA2AAgent{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update external A2A agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SyncNameForAgent copies a renamed linked agent's name onto its connection.
func (r *ExternalA2ARepository) SyncNameForAgent(ctx context.Context, workspaceID, agentID, name string) error {
	if err := r.db.WithContext(ctx).Model(&model.ExternalA2AAgent{}).
		Where("workspace_id = ? AND agent_id = ? AND name <> ?", workspaceID, agentID, name).
		Updates(map[string]any{"name": name, "updated_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("sync external A2A agent name: %w", err)
	}
	return nil
}

func (r *ExternalA2ARepository) Delete(ctx context.Context, workspaceID, id string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).
		Delete(&model.ExternalA2AAgent{})
	if result.Error != nil {
		return fmt.Errorf("delete external A2A agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListLinkedAgents returns the id and name of the workspace's agents with
// runtime_kind a2a, for matching comment mentions.
func (r *ExternalA2ARepository) ListLinkedAgents(ctx context.Context, workspaceID string) ([]model.Agent, error) {
	var agents []model.Agent
	if err := r.db.WithContext(ctx).Model(&model.Agent{}).Select("id", "name", "runtime_kind").
		Where("workspace_id = ? AND runtime_kind = ?", workspaceID, model.AgentRuntimeKindA2A).
		Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("list external A2A linked agents: %w", err)
	}
	return agents, nil
}

func (r *ExternalA2ARepository) GetTaskContext(ctx context.Context, workspaceID, taskID, externalAgentID string) (*model.A2ATaskContext, error) {
	var row model.A2ATaskContext
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ? AND external_a2a_agent_id = ?", workspaceID, taskID, externalAgentID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get A2A task context: %w", err)
	}
	return &row, nil
}

// UpsertTaskContext stores the latest remote context and task IDs for a
// (task, external agent) pair.
func (r *ExternalA2ARepository) UpsertTaskContext(ctx context.Context, row *model.A2ATaskContext) error {
	now := time.Now()
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "external_a2a_agent_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"context_id", "last_remote_task_id", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return fmt.Errorf("upsert A2A task context: %w", err)
	}
	return nil
}

func (r *ExternalA2ARepository) CreateUploadToken(ctx context.Context, row *model.A2ARunUploadToken) error {
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create A2A upload token: %w", err)
	}
	return nil
}

// LatestUsableUploadToken returns the newest unrevoked, unexpired token row for a run.
func (r *ExternalA2ARepository) LatestUsableUploadToken(ctx context.Context, runID string, now time.Time) (*model.A2ARunUploadToken, error) {
	var row model.A2ARunUploadToken
	err := r.db.WithContext(ctx).
		Where("agent_run_id = ? AND revoked_at IS NULL AND expires_at > ?", runID, now).
		Order("created_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get A2A upload token: %w", err)
	}
	return &row, nil
}

func (r *ExternalA2ARepository) GetUploadTokenByHash(ctx context.Context, tokenSHA256 string) (*model.A2ARunUploadToken, error) {
	var row model.A2ARunUploadToken
	err := r.db.WithContext(ctx).Where("token_sha256 = ?", tokenSHA256).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get A2A upload token: %w", err)
	}
	return &row, nil
}

// ReserveUploadBytes atomically adds size to a token's usage when the total
// stays within limit. It reports false when the budget would be exceeded.
func (r *ExternalA2ARepository) ReserveUploadBytes(ctx context.Context, tokenID string, size, limit int64) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.A2ARunUploadToken{}).
		Where("id = ? AND revoked_at IS NULL AND bytes_used + ? <= ?", tokenID, size, limit).
		UpdateColumn("bytes_used", gorm.Expr("bytes_used + ?", size))
	if result.Error != nil {
		return false, fmt.Errorf("reserve A2A upload bytes: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *ExternalA2ARepository) ReleaseUploadBytes(ctx context.Context, tokenID string, size int64) error {
	return r.db.WithContext(ctx).Model(&model.A2ARunUploadToken{}).
		Where("id = ? AND bytes_used >= ?", tokenID, size).
		UpdateColumn("bytes_used", gorm.Expr("bytes_used - ?", size)).Error
}

// RevokeRunUploadTokens revokes every active upload token for a run.
func (r *ExternalA2ARepository) RevokeRunUploadTokens(ctx context.Context, runID string, now time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.A2ARunUploadToken{}).
		Where("agent_run_id = ? AND revoked_at IS NULL", runID).
		Update("revoked_at", now).Error; err != nil {
		return fmt.Errorf("revoke A2A upload tokens: %w", err)
	}
	return nil
}

// ClaimProjectedItem records that a projected side effect is being applied.
// It returns false when the item was already claimed.
func (r *ExternalA2ARepository) ClaimProjectedItem(ctx context.Context, workspaceID, runID, key string) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.A2AProjectedItem{AgentRunID: runID, ItemKey: key, WorkspaceID: workspaceID})
	if result.Error != nil {
		return false, fmt.Errorf("claim A2A projected item: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// ReleaseProjectedItem drops a claim whose side effect failed so a replayed
// event can retry it.
func (r *ExternalA2ARepository) ReleaseProjectedItem(ctx context.Context, runID, key string) error {
	return r.db.WithContext(ctx).Where("agent_run_id = ? AND item_key = ?", runID, key).
		Delete(&model.A2AProjectedItem{}).Error
}
