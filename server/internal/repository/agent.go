package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentRepository handles DB operations for agents.
type AgentRepository struct {
	db *gorm.DB
}

// NewAgentRepository creates a new AgentRepository.
func NewAgentRepository(db *gorm.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// List returns all agents in a workspace.
func (r *AgentRepository) List(ctx context.Context, workspaceID string) ([]model.Agent, error) {
	var agents []model.Agent
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("is_system DESC, created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return agents, nil
}

// GetByID returns a single agent by ID within a workspace.
func (r *AgentRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.Agent, error) {
	var agent model.Agent
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return &agent, nil
}

// GetSystemByPreset returns the first system agent for a preset within a workspace.
func (r *AgentRepository) GetSystemByPreset(ctx context.Context, workspaceID, presetKey string) (*model.Agent, error) {
	var agent model.Agent
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND is_system = ? AND preset_key = ?", workspaceID, true, presetKey).
		Order("created_at ASC").
		First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get system agent by preset: %w", err)
	}
	return &agent, nil
}

// Create creates a new agent.
func (r *AgentRepository) Create(ctx context.Context, agent *model.Agent) error {
	if err := r.db.WithContext(ctx).Create(agent).Error; err != nil {
		return fmt.Errorf("create agent: %w", err)
	}
	return nil
}

// Update saves an agent.
func (r *AgentRepository) Update(ctx context.Context, agent *model.Agent) error {
	if err := r.db.WithContext(ctx).Save(agent).Error; err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	return nil
}

// Delete removes an agent.
func (r *AgentRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.Agent{}).Error; err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

// AgentRunNotifier publishes agent run events to WebSocket clients.
// This abstracts the cross-process notification mechanism so the repository
// does not depend on a specific transport (pg_notify, Redis, etc.).
type AgentRunNotifier interface {
	PublishRunEvent(ctx context.Context, run *model.AgentRun)
}

// AgentRunRepository handles DB operations for agent runs.
type AgentRunRepository struct {
	db       *gorm.DB
	notifier AgentRunNotifier // optional, set via SetNotifier
}

// NewAgentRunRepository creates a new AgentRunRepository.
func NewAgentRunRepository(db *gorm.DB) *AgentRunRepository {
	return &AgentRunRepository{db: db}
}

// SetNotifier sets the event notifier used by Notify(). This breaks a
// circular dependency: the repository is created before the publisher exists.
func (r *AgentRunRepository) SetNotifier(n AgentRunNotifier) {
	if r == nil {
		return
	}
	r.notifier = n
}

// ListByAgent returns runs for an agent with pagination.
func (r *AgentRunRepository) ListByAgent(ctx context.Context, workspaceID, agentID string, pagination model.PMPagination) ([]model.AgentRun, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AgentRun{}).Where("workspace_id = ? AND agent_id = ?", workspaceID, agentID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count agent runs: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	var runs []model.AgentRun
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list agent runs: %w", err)
	}
	return runs, total, nil
}

// ListByWorkspace returns runs in a workspace with pagination.
func (r *AgentRunRepository) ListByWorkspace(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.AgentRun, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AgentRun{}).Where("workspace_id = ?", workspaceID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count workspace agent runs: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	var runs []model.AgentRun
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list workspace agent runs: %w", err)
	}
	return runs, total, nil
}

// ListByStory returns runs for a task.
func (r *AgentRunRepository) ListByStory(ctx context.Context, workspaceID, storyID string) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND task_id = ?", workspaceID, storyID).Order("created_at DESC").Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list story agent runs: %w", err)
	}
	return runs, nil
}

// ListByTarget returns runs for a target object.
func (r *AgentRunRepository) ListByTarget(ctx context.Context, workspaceID, targetType, targetID string) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND target_type = ? AND target_id = ?", workspaceID, targetType, targetID).
		Order("created_at DESC").
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list target agent runs: %w", err)
	}
	return runs, nil
}

// FindActiveByTarget returns the currently queued or running run for a target.
func (r *AgentRunRepository) FindActiveByTarget(ctx context.Context, workspaceID, targetType, targetID string) (*model.AgentRun, error) {
	var run model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND target_type = ? AND target_id = ? AND status IN ?", workspaceID, targetType, targetID, []string{"queued", "running", "paused"}).
		Order("created_at DESC").
		First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find active target run: %w", err)
	}
	return &run, nil
}

// ListActive returns queued, running, or approval-pending runs in a workspace.
func (r *AgentRunRepository) ListActive(ctx context.Context, workspaceID string, limit int) ([]model.AgentRun, error) {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status IN ?", workspaceID, []string{"queued", "running", "paused"}).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var runs []model.AgentRun
	if err := query.Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list active agent runs: %w", err)
	}
	return runs, nil
}

// AgentRunMessageRepository handles persisted run conversation history.
type AgentRunMessageRepository struct {
	db *gorm.DB
}

func NewAgentRunMessageRepository(db *gorm.DB) *AgentRunMessageRepository {
	return &AgentRunMessageRepository{db: db}
}

func (r *AgentRunMessageRepository) ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunMessage, error) {
	var messages []model.AgentRunMessage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Order("sequence_no ASC, created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list agent run messages: %w", err)
	}
	return messages, nil
}

func (r *AgentRunMessageRepository) ListByRunAfterSequence(ctx context.Context, workspaceID, runID string, afterSequenceNo int) ([]model.AgentRunMessage, error) {
	var messages []model.AgentRunMessage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ? AND sequence_no > ?", workspaceID, runID, afterSequenceNo).
		Order("sequence_no ASC, created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list agent run messages after sequence: %w", err)
	}
	return messages, nil
}

func (r *AgentRunMessageRepository) NextSequence(ctx context.Context, workspaceID, runID string) (int, error) {
	type result struct {
		Max int
	}
	var row result
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRunMessage{}).
		Select("COALESCE(MAX(sequence_no), 0) AS max").
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Scan(&row).Error; err != nil {
		return 0, fmt.Errorf("next agent run message sequence: %w", err)
	}
	return row.Max + 1, nil
}

func (r *AgentRunMessageRepository) Create(ctx context.Context, message *model.AgentRunMessage) error {
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		return fmt.Errorf("create agent run message: %w", err)
	}
	return nil
}

// GetByID returns a single run.
func (r *AgentRunRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.AgentRun, error) {
	var run model.AgentRun
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent run: %w", err)
	}
	return &run, nil
}

// GetByIDAny returns a run by ID without requiring the caller to know its workspace.
func (r *AgentRunRepository) GetByIDAny(ctx context.Context, id string) (*model.AgentRun, error) {
	var run model.AgentRun
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent run: %w", err)
	}
	return &run, nil
}

// Create creates a new run.
func (r *AgentRunRepository) Create(ctx context.Context, run *model.AgentRun) error {
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}
	return nil
}

// Update saves a run.
func (r *AgentRunRepository) Update(ctx context.Context, run *model.AgentRun) error {
	if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
		return fmt.Errorf("update agent run: %w", err)
	}
	return nil
}

// GetByWorkflowID returns a run by temporal workflow ID.
func (r *AgentRunRepository) GetByWorkflowID(ctx context.Context, workflowID string) (*model.AgentRun, error) {
	var run model.AgentRun
	if err := r.db.WithContext(ctx).Where("workflow_id = ?", workflowID).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent run by workflow id: %w", err)
	}
	return &run, nil
}

// UpdateStage updates workflow execution stage metadata for a run.
func (r *AgentRunRepository) UpdateStage(ctx context.Context, workspaceID, runID, stage string, heartbeatAt *time.Time) error {
	updates := map[string]any{
		"execution_stage": stage,
	}
	if heartbeatAt != nil {
		updates["last_heartbeat_at"] = heartbeatAt
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("workspace_id = ? AND id = ?", workspaceID, runID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update agent run stage: %w", err)
	}
	return nil
}

// AddTokens increments the token count for a run.
func (r *AgentRunRepository) AddTokens(ctx context.Context, workspaceID, runID string, tokens int) error {
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("workspace_id = ? AND id = ?", workspaceID, runID).
		UpdateColumn("tokens_used", gorm.Expr("tokens_used + ?", tokens)).Error; err != nil {
		return fmt.Errorf("add agent run tokens: %w", err)
	}
	return nil
}

// Notify publishes an agent_run updated event so WebSocket clients see
// the run status change in real time. When a notifier is configured
// (via SetNotifier), the event is published through it (Redis Pub/Sub
// in production). Safe to call when no notifier is set — it is a no-op.
func (r *AgentRunRepository) Notify(ctx context.Context, run *model.AgentRun) {
	if r == nil || run == nil || r.notifier == nil {
		return
	}
	r.notifier.PublishRunEvent(ctx, run)
}

// AgentRunArtifactRepository handles DB operations for run artifacts.
type AgentRunArtifactRepository struct {
	db *gorm.DB
}

// NewAgentRunArtifactRepository creates a new AgentRunArtifactRepository.
func NewAgentRunArtifactRepository(db *gorm.DB) *AgentRunArtifactRepository {
	return &AgentRunArtifactRepository{db: db}
}

// ListByRun returns artifacts for a run.
func (r *AgentRunArtifactRepository) ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunArtifact, error) {
	var artifacts []model.AgentRunArtifact
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND run_id = ?", workspaceID, runID).Order("sequence_no ASC, created_at ASC").Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("list run artifacts: %w", err)
	}
	return artifacts, nil
}

// NextSequence returns the next sequence number for a run artifact.
func (r *AgentRunArtifactRepository) NextSequence(ctx context.Context, workspaceID, runID string) (int, error) {
	type result struct {
		Max int
	}
	var row result
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRunArtifact{}).
		Select("COALESCE(MAX(sequence_no), 0) AS max").
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Scan(&row).Error; err != nil {
		return 0, fmt.Errorf("next agent run artifact sequence: %w", err)
	}
	return row.Max + 1, nil
}

// Create creates a new artifact.
func (r *AgentRunArtifactRepository) Create(ctx context.Context, artifact *model.AgentRunArtifact) error {
	if err := r.db.WithContext(ctx).Create(artifact).Error; err != nil {
		return fmt.Errorf("create run artifact: %w", err)
	}
	return nil
}
