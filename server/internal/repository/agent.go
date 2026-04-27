package repository

import (
	"context"
	"fmt"
	"strings"
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

// DB returns the underlying *gorm.DB for transaction support.
func (r *AgentRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a repository bound to the provided transaction.
func (r *AgentRepository) WithTx(tx *gorm.DB) *AgentRepository {
	if tx == nil {
		return r
	}
	return &AgentRepository{db: tx}
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
	db                   *gorm.DB
	notifier             AgentRunNotifier // optional, set via SetNotifier
	triggerExecutionRepo *AgentTriggerExecutionRepository
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

// SetTriggerExecutionRepository sets the execution history repository used to
// keep trigger execution rows in sync with run state changes.
func (r *AgentRunRepository) SetTriggerExecutionRepository(repo *AgentTriggerExecutionRepository) {
	if r == nil {
		return
	}
	r.triggerExecutionRepo = repo
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

// ListByTask returns runs for a task.
func (r *AgentRunRepository) ListByTask(ctx context.Context, workspaceID, storyID string) ([]model.AgentRun, error) {
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
	sanitizeAgentRunMessageForPostgres(message)
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
	if r.triggerExecutionRepo != nil {
		_ = r.triggerExecutionRepo.SyncRunStatus(ctx, run)
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

// AgentTriggerExecutionRepository handles DB operations for durable trigger execution history.
type AgentTriggerExecutionRepository struct {
	db *gorm.DB
}

// NewAgentTriggerExecutionRepository creates a new AgentTriggerExecutionRepository.
func NewAgentTriggerExecutionRepository(db *gorm.DB) *AgentTriggerExecutionRepository {
	return &AgentTriggerExecutionRepository{db: db}
}

// Create persists a trigger execution row.
func (r *AgentTriggerExecutionRepository) Create(ctx context.Context, execution *model.AgentTriggerExecution) error {
	if err := r.db.WithContext(ctx).Create(execution).Error; err != nil {
		return fmt.Errorf("create trigger execution: %w", err)
	}
	return nil
}

// ListByAgent returns recent trigger execution rows for an agent.
func (r *AgentTriggerExecutionRepository) ListByAgent(ctx context.Context, workspaceID, agentID string, limit int) ([]model.AgentTriggerExecution, error) {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ?", workspaceID, agentID).
		Order("fired_at DESC, created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var executions []model.AgentTriggerExecution
	if err := query.Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("list trigger executions: %w", err)
	}
	return executions, nil
}

// ListLatestAutomationRuleExecutions returns the most recent trigger execution
// row for each automation rule reference in the workspace.
func (r *AgentTriggerExecutionRepository) ListLatestAutomationRuleExecutions(ctx context.Context, workspaceID string, ruleIDs []string) ([]model.AgentTriggerExecution, error) {
	if len(ruleIDs) == 0 {
		return []model.AgentTriggerExecution{}, nil
	}

	var executions []model.AgentTriggerExecution
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND binding_kind = ? AND reference_type = ? AND reference_id IN ?", workspaceID, "automation_rule", "automation_rule", ruleIDs).
		Order("reference_id ASC, COALESCE(completed_at, started_at, fired_at) DESC, fired_at DESC, created_at DESC").
		Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("list latest automation rule executions: %w", err)
	}

	latest := make(map[string]model.AgentTriggerExecution, len(ruleIDs))
	for _, execution := range executions {
		if execution.ReferenceID == nil || strings.TrimSpace(*execution.ReferenceID) == "" {
			continue
		}
		refID := strings.TrimSpace(*execution.ReferenceID)
		if _, exists := latest[refID]; exists {
			continue
		}
		latest[refID] = execution
	}

	result := make([]model.AgentTriggerExecution, 0, len(latest))
	for _, ruleID := range ruleIDs {
		if execution, ok := latest[strings.TrimSpace(ruleID)]; ok {
			result = append(result, execution)
		}
	}
	return result, nil
}

// ListByWorkspace returns recent trigger executions in a workspace with
// optional filters and pagination.
func (r *AgentTriggerExecutionRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID string,
	filters model.TriggerExecutionListFilters,
	pagination model.PMPagination,
) ([]model.AgentTriggerExecution, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AgentTriggerExecution{}).Where("workspace_id = ?", workspaceID)

	if filters.AgentID != nil && strings.TrimSpace(*filters.AgentID) != "" {
		query = query.Where("agent_id = ?", strings.TrimSpace(*filters.AgentID))
	}
	if filters.BindingID != nil && strings.TrimSpace(*filters.BindingID) != "" {
		query = query.Where("binding_id = ?", strings.TrimSpace(*filters.BindingID))
	}
	if filters.TriggerType != nil && strings.TrimSpace(*filters.TriggerType) != "" {
		query = query.Where("trigger_type = ?", strings.TrimSpace(*filters.TriggerType))
	}
	if filters.BindingKind != nil && strings.TrimSpace(*filters.BindingKind) != "" {
		query = query.Where("binding_kind = ?", strings.TrimSpace(*filters.BindingKind))
	}
	if filters.Status != nil && strings.TrimSpace(*filters.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*filters.Status))
	}
	if filters.ReferenceID != nil && strings.TrimSpace(*filters.ReferenceID) != "" {
		query = query.Where("reference_id = ?", strings.TrimSpace(*filters.ReferenceID))
	}
	if filters.FiredAfter != nil {
		query = query.Where("fired_at >= ?", *filters.FiredAfter)
	}
	if filters.FiredBefore != nil {
		query = query.Where("fired_at <= ?", *filters.FiredBefore)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count trigger executions: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	var executions []model.AgentTriggerExecution
	if err := query.
		Order("fired_at DESC, created_at DESC").
		Offset(offset).
		Limit(perPage).
		Find(&executions).Error; err != nil {
		return nil, 0, fmt.Errorf("list workspace trigger executions: %w", err)
	}
	return executions, total, nil
}

// SyncRunStatus updates any linked trigger execution rows to match the latest run status.
func (r *AgentTriggerExecutionRepository) SyncRunStatus(ctx context.Context, run *model.AgentRun) error {
	if r == nil || run == nil || strings.TrimSpace(run.ID) == "" {
		return nil
	}

	updates := map[string]any{
		"status":        strings.TrimSpace(run.Status),
		"error_message": run.ErrorMessage,
		"started_at":    run.StartedAt,
		"completed_at":  run.CompletedAt,
		"target_type":   nilIfBlank(run.TargetType),
		"target_id":     nilIfBlank(run.TargetID),
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentTriggerExecution{}).
		Where("workspace_id = ? AND run_id = ?", run.WorkspaceID, run.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("sync trigger execution run status: %w", err)
	}
	return nil
}

func nilIfBlank(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
	sanitizeAgentRunArtifactForPostgres(artifact)
	if err := r.db.WithContext(ctx).Create(artifact).Error; err != nil {
		return fmt.Errorf("create run artifact: %w", err)
	}
	return nil
}
