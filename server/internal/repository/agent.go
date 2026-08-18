package repository

import (
	"context"
	"encoding/json"
	"errors"
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
	if err := r.LoadTeamAccess(ctx, agents); err != nil {
		return nil, err
	}
	if err := r.LoadTokenTotals(ctx, agents); err != nil {
		return nil, err
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
	agents := []model.Agent{agent}
	if err := r.LoadTeamAccess(ctx, agents); err != nil {
		return nil, err
	}
	if err := r.LoadTokenTotals(ctx, agents); err != nil {
		return nil, err
	}
	agent = agents[0]
	return &agent, nil
}

// ListByIDs returns lightweight agent records for the requested workspace IDs.
func (r *AgentRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.Agent, error) {
	if len(ids) == 0 {
		return []model.Agent{}, nil
	}
	var agents []model.Agent
	if err := r.db.WithContext(ctx).
		Select("id", "workspace_id", "name", "icon_key", "preset_key").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("list agents by ids: %w", err)
	}
	return agents, nil
}

// GetSystemByPreset returns the first system agent for a preset within a workspace.
func (r *AgentRepository) GetSystemByPreset(ctx context.Context, workspaceID, presetKey string) (*model.Agent, error) {
	presetKeys := []string{presetKey}
	if presetKey == model.AgentPresetCommandAgent {
		presetKeys = append(presetKeys, model.AgentPresetResearcher)
	}
	var agent model.Agent
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND is_system = ? AND preset_key IN ?", workspaceID, true, presetKeys).
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
		if isMissingAgentActiveVersionColumn(err) {
			if err := r.db.WithContext(ctx).Omit("ActiveVersionID", "active_version_id").Create(agent).Error; err != nil {
				return fmt.Errorf("create agent: %w", err)
			}
		} else {
			return fmt.Errorf("create agent: %w", err)
		}
	}
	if len(normalizeAgentTeamIDs(agent.TeamIDs)) > 0 {
		if err := r.ReplaceTeamAccess(ctx, agent.ID, agent.TeamIDs); err != nil {
			return err
		}
	}
	return nil
}

// Update saves an agent.
func (r *AgentRepository) Update(ctx context.Context, agent *model.Agent) error {
	if err := r.db.WithContext(ctx).Save(agent).Error; err != nil {
		if isMissingAgentActiveVersionColumn(err) {
			if err := r.db.WithContext(ctx).Omit("ActiveVersionID", "active_version_id").Save(agent).Error; err != nil {
				return fmt.Errorf("update agent: %w", err)
			}
		} else {
			return fmt.Errorf("update agent: %w", err)
		}
	}
	if err := r.ReplaceTeamAccess(ctx, agent.ID, agent.TeamIDs); err != nil {
		return err
	}
	agents := []model.Agent{*agent}
	if err := r.LoadTokenTotals(ctx, agents); err != nil {
		return err
	}
	agent.TokensUsedTotal = agents[0].TokensUsedTotal
	return nil
}

// Delete removes an agent.
func (r *AgentRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.Agent{}).Error; err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

func (r *AgentRepository) LoadTeamAccess(ctx context.Context, agents []model.Agent) error {
	if len(agents) == 0 {
		return nil
	}
	ids := make([]string, 0, len(agents))
	indexByID := make(map[string]int, len(agents))
	for idx := range agents {
		ids = append(ids, agents[idx].ID)
		indexByID[agents[idx].ID] = idx
		agents[idx].TeamIDs = nil
	}
	var access []model.AgentTeamAccess
	if err := r.db.WithContext(ctx).Where("agent_id IN ?", ids).Order("agent_id ASC, created_at ASC, team_id ASC").Find(&access).Error; err != nil {
		if isMissingAgentTeamAccessTable(err) {
			for idx := range agents {
				if agents[idx].TeamID != nil && strings.TrimSpace(*agents[idx].TeamID) != "" {
					agents[idx].TeamIDs = []string{strings.TrimSpace(*agents[idx].TeamID)}
				}
			}
			return nil
		}
		return fmt.Errorf("load agent team access: %w", err)
	}
	for _, row := range access {
		idx, ok := indexByID[row.AgentID]
		if !ok {
			continue
		}
		agents[idx].TeamIDs = append(agents[idx].TeamIDs, row.TeamID)
	}
	for idx := range agents {
		if len(agents[idx].TeamIDs) == 0 && agents[idx].TeamID != nil && strings.TrimSpace(*agents[idx].TeamID) != "" {
			agents[idx].TeamIDs = []string{strings.TrimSpace(*agents[idx].TeamID)}
		}
	}
	return nil
}

func (r *AgentRepository) ReplaceTeamAccess(ctx context.Context, agentID string, teamIDs []string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if err := r.db.WithContext(ctx).Where("agent_id = ?", agentID).Delete(&model.AgentTeamAccess{}).Error; err != nil {
		if isMissingAgentTeamAccessTable(err) {
			return nil
		}
		return fmt.Errorf("delete agent team access: %w", err)
	}
	normalized := normalizeAgentTeamIDs(teamIDs)
	if len(normalized) == 0 {
		return nil
	}
	rows := make([]model.AgentTeamAccess, 0, len(normalized))
	for _, teamID := range normalized {
		rows = append(rows, model.AgentTeamAccess{AgentID: agentID, TeamID: teamID})
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
		if isMissingAgentTeamAccessTable(err) {
			return nil
		}
		return fmt.Errorf("replace agent team access: %w", err)
	}
	return nil
}

func isMissingAgentTeamAccessTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table: agent_team_access") ||
		strings.Contains(msg, `relation "agent_team_access" does not exist`)
}

func (r *AgentRepository) LoadTokenTotals(ctx context.Context, agents []model.Agent) error {
	if len(agents) == 0 {
		return nil
	}
	ids := make([]string, 0, len(agents))
	indexByID := make(map[string]int, len(agents))
	for idx := range agents {
		ids = append(ids, agents[idx].ID)
		indexByID[agents[idx].ID] = idx
		agents[idx].TokensUsedTotal = 0
	}
	type tokenTotalRow struct {
		AgentID         string
		TokensUsedTotal int
	}
	var rows []tokenTotalRow
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Select("agent_id, COALESCE(SUM(tokens_used), 0) AS tokens_used_total").
		Where("agent_id IN ?", ids).
		Group("agent_id").
		Scan(&rows).Error; err != nil {
		if isMissingAgentRunsTable(err) {
			return nil
		}
		return fmt.Errorf("load agent token totals: %w", err)
	}
	for _, row := range rows {
		idx, ok := indexByID[row.AgentID]
		if !ok {
			continue
		}
		agents[idx].TokensUsedTotal = row.TokensUsedTotal
	}
	return nil
}

func isMissingAgentRunsTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table: agent_runs") ||
		strings.Contains(msg, `relation "agent_runs" does not exist`)
}

func isMissingAgentActiveVersionColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no column named active_version_id") ||
		strings.Contains(msg, "no such column: active_version_id") ||
		strings.Contains(msg, `column "active_version_id" of relation "agents" does not exist`)
}

func isMissingAgentRunVersionColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no column named agent_version_id") ||
		strings.Contains(msg, "no such column: agent_version_id") ||
		strings.Contains(msg, `column "agent_version_id" of relation "agent_runs" does not exist`)
}

func normalizeAgentTeamIDs(teamIDs []string) []string {
	seen := make(map[string]struct{}, len(teamIDs))
	normalized := make([]string, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		teamID = strings.TrimSpace(teamID)
		if teamID == "" {
			continue
		}
		if _, ok := seen[teamID]; ok {
			continue
		}
		seen[teamID] = struct{}{}
		normalized = append(normalized, teamID)
	}
	return normalized
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

const (
	_agentRunListDefaultPageSize = 50
	_agentRunListMaxPageSize     = 500
	_agentRunListColumns         = `
		id,
		workspace_id,
		agent_id,
		task_id,
		conversation_id,
		target_type,
		target_id,
		runtime_kind,
		invocation_mode,
		parent_run_id,
		dock_chat_id,
		handoff_state,
		approval_state,
		pause_reason,
		triggered_by_user_id,
		status,
		workflow_id,
		workflow_run_id,
		external_runtime,
		external_runtime_id,
		task_queue,
		runner_pool,
		agent_version_id,
		repository_id,
		repo_full_name,
		base_branch,
		working_branch,
		delivery_target_id,
		execution_stage,
		last_heartbeat_at,
		input,
		cached_input_tokens,
		input_tokens,
		output_tokens,
		tokens_used,
		error_message,
		started_at,
		completed_at,
		created_at,
		updated_at`
)

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

	page, perPage := agentRunListPagination(pagination)
	offset := (page - 1) * perPage

	var runs []model.AgentRun
	if err := query.Select(_agentRunListColumns).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(perPage).
		Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list agent runs: %w", err)
	}
	return runs, total, nil
}

// ListByAgentSince returns all runs for an agent created at or after the given time.
func (r *AgentRunRepository) ListByAgentSince(ctx context.Context, workspaceID, agentID string, since time.Time) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ? AND created_at >= ?", workspaceID, agentID, since).
		Order("created_at ASC").
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list agent runs since: %w", err)
	}
	return runs, nil
}

// ListByWorkspace returns runs in a workspace with pagination.
func (r *AgentRunRepository) ListByWorkspace(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.AgentRun, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AgentRun{}).Where("workspace_id = ?", workspaceID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count workspace agent runs: %w", err)
	}

	page, perPage := agentRunListPagination(pagination)
	offset := (page - 1) * perPage

	var runs []model.AgentRun
	if err := query.Select(_agentRunListColumns).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(perPage).
		Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list workspace agent runs: %w", err)
	}
	return runs, total, nil
}

// CountWorkspaceRunsRequiringAttention returns the number of paused runs that
// need a user action. Runs paused while waiting for a chat reply are excluded.
func (r *AgentRunRepository) CountWorkspaceRunsRequiringAttention(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("workspace_id = ?", workspaceID).
		Where("status = ?", model.AgentRunStatusPaused).
		Where("COALESCE(pause_reason, '') <> ?", model.AgentRunPauseReasonUserMessage).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count workspace agent runs requiring attention: %w", err)
	}
	return count, nil
}

func agentRunListPagination(pagination model.PMPagination) (int, int) {
	page := pagination.Page
	if page <= 0 {
		page = 1
	}

	perPage := pagination.PerPage
	if perPage <= 0 {
		perPage = _agentRunListDefaultPageSize
	}
	if perPage > _agentRunListMaxPageSize {
		perPage = _agentRunListMaxPageSize
	}
	return page, perPage
}

// ListWorkspaceRunsWithoutTriggerExecutions returns agent runs that do not
// already have a durable trigger execution row. Automation activity uses these
// as synthetic timeline items for command-bar and child-run launches.
func (r *AgentRunRepository) ListWorkspaceRunsWithoutTriggerExecutions(ctx context.Context, workspaceID string, filters model.TriggerExecutionListFilters, limit int) ([]model.AgentRun, int64, error) {
	if r == nil || r.db == nil {
		return []model.AgentRun{}, 0, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return []model.AgentRun{}, 0, nil
	}
	bindingKinds := splitCSVFilter(filters.BindingKind)
	if filters.BindingKind != nil {
		if len(bindingKinds) > 0 && !csvFilterIncludes(bindingKinds, "agent_run") && !csvFilterIncludes(bindingKinds, model.AgentRunTriggerSourceCommandBar) && !csvFilterIncludes(bindingKinds, model.AgentRunTriggerSourceManual) {
			return []model.AgentRun{}, 0, nil
		}
	}
	bindingID := ""
	bindingIDs := splitCSVFilter(filters.BindingID)
	if filters.BindingID != nil && strings.TrimSpace(*filters.BindingID) != "" {
		bindingID = strings.TrimSpace(*filters.BindingID)
		if len(bindingIDs) == 1 {
			bindingID = bindingIDs[0]
		}
		hasRunnableBindingID := false
		for _, value := range bindingIDs {
			if value == "agent_run.run" || value == "agent_run.child_run" || value == "command_bar.run" || manualRunTargetTypeForBindingID(value) != "" {
				hasRunnableBindingID = true
				break
			}
		}
		if len(bindingIDs) > 0 && !hasRunnableBindingID {
			return []model.AgentRun{}, 0, nil
		}
	}
	triggerTypes := splitCSVFilter(filters.TriggerType)
	if len(triggerTypes) > 0 && !csvFilterIncludes(triggerTypes, model.AgentRunTriggerTypeCommandBar) && !csvFilterIncludes(triggerTypes, model.AgentRunTriggerTypeManual) {
		return []model.AgentRun{}, 0, nil
	}
	if filters.ReferenceID != nil && strings.TrimSpace(*filters.ReferenceID) != "" {
		return []model.AgentRun{}, 0, nil
	}

	query := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Joins("LEFT JOIN agent_trigger_executions ON agent_trigger_executions.workspace_id = agent_runs.workspace_id AND agent_trigger_executions.run_id = agent_runs.id").
		Where("agent_runs.workspace_id = ? AND agent_trigger_executions.id IS NULL", workspaceID)
	if values := splitCSVFilter(filters.AgentID); len(values) > 0 {
		query = query.Where("agent_runs.agent_id IN ?", values)
	}
	if values := splitCSVFilter(filters.Status); len(values) > 0 {
		query = query.Where("agent_runs.status IN ?", values)
	}
	if csvFilterIncludes(bindingKinds, model.AgentRunTriggerSourceCommandBar) || csvFilterIncludes(bindingIDs, "command_bar.run") || csvFilterIncludes(triggerTypes, model.AgentRunTriggerTypeCommandBar) {
		query = query.Where(agentRunInputTriggerStringPredicate(r.db, "source"), model.AgentRunTriggerSourceCommandBar)
	}
	manualTargetType := manualRunTargetTypeForBindingID(bindingID)
	if csvFilterIncludes(bindingKinds, model.AgentRunTriggerSourceManual) || csvFilterIncludes(triggerTypes, model.AgentRunTriggerTypeManual) || manualTargetType != "" {
		query = query.Where(agentRunInputManualTriggerPredicate(r.db), model.AgentRunTriggerSourceManual)
	}
	if manualTargetType != "" {
		query = query.Where("agent_runs.target_type = ?", manualTargetType)
	}
	if bindingID == "agent_run.child_run" {
		query = query.Where("agent_runs.parent_run_id IS NOT NULL AND agent_runs.parent_run_id <> ''")
	}
	if bindingID == "agent_run.run" {
		query = query.Where("(agent_runs.parent_run_id IS NULL OR agent_runs.parent_run_id = '')")
	}
	if filters.RunID != nil && strings.TrimSpace(*filters.RunID) != "" {
		query = query.Where("agent_runs.id = ?", strings.TrimSpace(*filters.RunID))
	}
	if filters.FiredAfter != nil {
		query = query.Where("agent_runs.created_at >= ?", *filters.FiredAfter)
	}
	if filters.FiredBefore != nil {
		query = query.Where("agent_runs.created_at <= ?", *filters.FiredBefore)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count workspace agent runs without trigger executions: %w", err)
	}
	if limit <= 0 {
		limit = 100
	}
	var runs []model.AgentRun
	if err := query.Order("agent_runs.created_at DESC").Limit(limit).Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list workspace agent runs without trigger executions: %w", err)
	}
	return runs, total, nil
}

func agentRunInputTriggerStringPredicate(db *gorm.DB, field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		field = "source"
	}
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		return fmt.Sprintf("agent_runs.input -> 'trigger' ->> '%s' = ?", field)
	}
	return fmt.Sprintf("json_extract(agent_runs.input, '$.trigger.%s') = ?", field)
}

func agentRunInputManualTriggerPredicate(db *gorm.DB) string {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		return "COALESCE(agent_runs.input -> 'trigger' ->> 'source', '') IN (?, '')"
	}
	return "COALESCE(json_extract(agent_runs.input, '$.trigger.source'), '') IN (?, '')"
}

func manualRunTargetTypeForBindingID(bindingID string) string {
	switch strings.TrimSpace(bindingID) {
	case "manual.task_run":
		return "task"
	case "manual.epic_run":
		return "epic"
	case "manual.support_run":
		return "support_conversation"
	case "manual.repository_run":
		return "repository"
	case "manual.workspace_run":
		return "workspace"
	default:
		return ""
	}
}

func splitCSVFilter(value *string) []string {
	if value == nil {
		return nil
	}
	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, part := range strings.Split(*value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}

func csvFilterIncludes(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ListRecentForActor returns the most recent runs triggered by the given user
// in the workspace. Used by the command runs rail to rehydrate standalone
// runs (runs not tied to a command-bar plan) on mount.
func (r *AgentRunRepository) ListRecentForActor(ctx context.Context, workspaceID, actorID string, limit int) ([]model.AgentRun, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("agent run repository is not configured")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND triggered_by_user_id = ?", workspaceID, actorID).
		Order("created_at DESC").
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list recent agent runs for actor: %w", err)
	}
	return runs, nil
}

// ListDockRunsForActor returns active user-owned runs plus terminal runs
// updated since the supplied cutoff. Chat-backing runs stay on the Chats tab.
func (r *AgentRunRepository) ListDockRunsForActor(
	ctx context.Context,
	workspaceID string,
	actorID string,
	recentSince time.Time,
	limit int,
) ([]model.AgentRun, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("agent run repository is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	activeStatuses := []string{
		model.AgentRunStatusQueued,
		model.AgentRunStatusRunning,
		model.AgentRunStatusPaused,
	}
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Select(_agentRunListColumns).
		Where("workspace_id = ? AND triggered_by_user_id = ?", workspaceID, actorID).
		Where("dock_chat_id IS NULL").
		Where("(status IN ? OR updated_at >= ?)", activeStatuses, recentSince).
		Order("updated_at DESC, created_at DESC").
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list dock agent runs: %w", err)
	}
	return runs, nil
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
	if message != nil && message.DockChatID != nil && strings.TrimSpace(*message.DockChatID) != "" && message.DockChatSequence == nil {
		var sequence int64
		if err := r.db.WithContext(ctx).Raw(`
			UPDATE dock_chats AS chat
			SET next_message_sequence = CASE
					WHEN COALESCE(chat.next_message_sequence, 0) >= COALESCE((
						SELECT MAX(existing.dock_chat_sequence)
						FROM agent_run_messages AS existing
						WHERE existing.dock_chat_id = chat.id
					), 0)
					THEN COALESCE(chat.next_message_sequence, 0)
					ELSE COALESCE((
						SELECT MAX(existing.dock_chat_sequence)
						FROM agent_run_messages AS existing
						WHERE existing.dock_chat_id = chat.id
					), 0)
				END + 1,
				updated_at = updated_at
			WHERE chat.workspace_id = ? AND chat.id = ?
			RETURNING next_message_sequence
		`, message.WorkspaceID, strings.TrimSpace(*message.DockChatID)).Scan(&sequence).Error; err != nil {
			return fmt.Errorf("allocate dock chat message sequence: %w", err)
		}
		if sequence <= 0 {
			return fmt.Errorf("allocate dock chat message sequence: dock chat not found")
		}
		message.DockChatSequence = &sequence
	}
	sanitizeAgentRunMessageForPostgres(message)
	query := r.db.WithContext(ctx)
	// Legacy/non-Dock run-message tables do not participate in the chat
	// timeline. Omitting Dock-only columns also keeps focused embedders and
	// migrations that intentionally expose the older table contract working.
	if message != nil && message.DockChatID == nil && message.ClientMessageID == nil {
		query = query.Omit("dock_chat_id", "dock_chat_sequence", "client_message_id", "delivery_status")
	}
	if err := query.Create(message).Error; err != nil {
		return fmt.Errorf("create agent run message: %w", err)
	}
	return nil
}

// GetByClientMessageID returns an idempotently submitted human message.
func (r *AgentRunMessageRepository) GetByClientMessageID(ctx context.Context, workspaceID, clientMessageID string) (*model.AgentRunMessage, error) {
	clientMessageID = strings.TrimSpace(clientMessageID)
	if clientMessageID == "" {
		return nil, nil
	}
	var message model.AgentRunMessage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND client_message_id = ?", workspaceID, clientMessageID).
		First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent run message by client id: %w", err)
	}
	return &message, nil
}

// ListByDockChat returns the newest persisted chat messages as an ascending
// page. before is an exclusive stable cursor; nil starts at the current tail.
func (r *AgentRunMessageRepository) ListByDockChat(ctx context.Context, workspaceID, dockChatID string, before *int64, limit int) ([]model.AgentRunMessage, *int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_id = ? AND dock_chat_sequence IS NOT NULL AND delivery_status <> ?", workspaceID, dockChatID, "failed")
	if before != nil && *before > 0 {
		query = query.Where("dock_chat_sequence < ?", *before)
	}
	var messages []model.AgentRunMessage
	if err := query.Order("dock_chat_sequence DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, nil, fmt.Errorf("list dock chat messages: %w", err)
	}
	var nextBefore *int64
	if len(messages) > limit {
		messages = messages[:limit]
		if sequence := messages[len(messages)-1].DockChatSequence; sequence != nil {
			value := *sequence
			nextBefore = &value
		}
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nextBefore, nil
}

// ListDockChatTurnThroughMessage returns the persisted conversation interval
// after the preceding user message through targetMessageID. The target is
// scoped to the same workspace and Dock chat before any interval is read.
func (r *AgentRunMessageRepository) ListDockChatTurnThroughMessage(
	ctx context.Context,
	workspaceID, dockChatID, targetMessageID string,
) ([]model.AgentRunMessage, error) {
	var target model.AgentRunMessage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_id = ? AND id = ? AND dock_chat_sequence IS NOT NULL AND delivery_status <> ?", workspaceID, dockChatID, targetMessageID, "failed").
		First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []model.AgentRunMessage{}, nil
		}
		return nil, fmt.Errorf("get dock chat work target: %w", err)
	}
	if target.DockChatSequence == nil {
		return []model.AgentRunMessage{}, nil
	}

	type sequenceResult struct {
		Sequence int64
	}
	var previous sequenceResult
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRunMessage{}).
		Select("COALESCE(MAX(dock_chat_sequence), 0) AS sequence").
		Where("workspace_id = ? AND dock_chat_id = ? AND role = ? AND dock_chat_sequence < ? AND delivery_status <> ?", workspaceID, dockChatID, "user", *target.DockChatSequence, "failed").
		Scan(&previous).Error; err != nil {
		return nil, fmt.Errorf("find dock chat work boundary: %w", err)
	}

	var messages []model.AgentRunMessage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_id = ? AND dock_chat_sequence > ? AND dock_chat_sequence <= ? AND delivery_status <> ?", workspaceID, dockChatID, previous.Sequence, *target.DockChatSequence, "failed").
		Order("dock_chat_sequence ASC").
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list dock chat work detail: %w", err)
	}
	return messages, nil
}

// UpdateDeliveryStatus records whether a persisted human message reached the
// runtime. Failed attempts remain available for audit and idempotent retry.
func (r *AgentRunMessageRepository) UpdateDeliveryStatus(ctx context.Context, workspaceID, messageID, status string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRunMessage{}).
		Where("workspace_id = ? AND id = ?", workspaceID, messageID).
		Update("delivery_status", status).Error; err != nil {
		return fmt.Errorf("update agent run message delivery status: %w", err)
	}
	return nil
}

func (r *AgentRunMessageRepository) UpdatePendingDeliveryByRun(ctx context.Context, workspaceID, runID, status string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRunMessage{}).
		Where("workspace_id = ? AND run_id = ? AND delivery_status = ?", workspaceID, runID, "pending").
		Update("delivery_status", status).Error; err != nil {
		return fmt.Errorf("update pending run message delivery status: %w", err)
	}
	return nil
}

// Update replaces the mutable transcript fields of an existing run message.
func (r *AgentRunMessageRepository) Update(ctx context.Context, message *model.AgentRunMessage) error {
	if message == nil {
		return nil
	}
	sanitizeAgentRunMessageForPostgres(message)
	updates := map[string]any{
		"runtime_message_id": message.RuntimeMessageID,
		"role":               message.Role,
		"content":            message.Content,
		"message_type":       message.MessageType,
		"content_blocks":     message.ContentBlocks,
		"turn_segments":      message.TurnSegments,
		"tool_invocations":   message.ToolInvocations,
		"token_usage":        message.TokenUsage,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRunMessage{}).
		Where("workspace_id = ? AND run_id = ? AND id = ?", message.WorkspaceID, message.RunID, message.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update agent run message: %w", err)
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

// GetByExternalRuntimeID returns a run linked to an external runtime run ID.
func (r *AgentRunRepository) GetByExternalRuntimeID(ctx context.Context, externalRuntime, externalRuntimeID string) (*model.AgentRun, error) {
	externalRuntime = strings.TrimSpace(externalRuntime)
	externalRuntimeID = strings.TrimSpace(externalRuntimeID)
	if externalRuntime == "" || externalRuntimeID == "" {
		return nil, nil
	}
	var run model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("external_runtime = ? AND external_runtime_id = ?", externalRuntime, externalRuntimeID).
		First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent run by external runtime id: %w", err)
	}
	return &run, nil
}

// ListActiveByExternalRuntime returns non-terminal mapped runs old enough to reconcile.
func (r *AgentRunRepository) ListActiveByExternalRuntime(ctx context.Context, externalRuntime string, olderThan time.Time, limit int) ([]model.AgentRun, error) {
	externalRuntime = strings.TrimSpace(externalRuntime)
	if externalRuntime == "" {
		return []model.AgentRun{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("external_runtime = ? AND external_runtime_id IS NOT NULL AND status IN ?", externalRuntime, []string{
			model.AgentRunStatusQueued,
			model.AgentRunStatusRunning,
			model.AgentRunStatusPaused,
		}).
		Where("updated_at < ?", olderThan).
		Order("updated_at ASC").
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list active agent runs by external runtime: %w", err)
	}
	return runs, nil
}

// ListByIDs returns runs in a workspace for a set of IDs.
func (r *AgentRunRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.AgentRun, error) {
	if len(ids) == 0 {
		return []model.AgentRun{}, nil
	}
	var runs []model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Order("created_at ASC").
		Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("list agent runs by ids: %w", err)
	}
	return runs, nil
}

// FindByParentRunID returns the first run linked to the given parent run.
func (r *AgentRunRepository) FindByParentRunID(ctx context.Context, workspaceID, parentRunID string) (*model.AgentRun, error) {
	var run model.AgentRun
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND parent_run_id = ?", workspaceID, parentRunID).
		Order("created_at ASC").
		First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find run by parent run id: %w", err)
	}
	return &run, nil
}

// FindCompletedByTargetStage returns the latest completed run for a target and execution stage.
func (r *AgentRunRepository) FindCompletedByTargetStage(ctx context.Context, workspaceID, targetType, targetID, stage string) (*model.AgentRun, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("agent run repository is not configured")
	}
	var run model.AgentRun
	if err := r.db.WithContext(ctx).
		Where(
			"workspace_id = ? AND target_type = ? AND target_id = ? AND execution_stage = ? AND status = ?",
			workspaceID,
			targetType,
			targetID,
			stage,
			model.AgentRunStatusCompleted,
		).
		Order("completed_at DESC, created_at DESC").
		First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find completed target stage run: %w", err)
	}
	return &run, nil
}

// Create creates a new run.
func (r *AgentRunRepository) Create(ctx context.Context, run *model.AgentRun) error {
	if err := r.db.WithContext(ctx).Create(run).Error; err != nil {
		if isMissingAgentRunVersionColumn(err) {
			if err := r.db.WithContext(ctx).Omit("AgentVersionID", "agent_version_id").Create(run).Error; err != nil {
				return fmt.Errorf("create agent run: %w", err)
			}
			return nil
		}
		return fmt.Errorf("create agent run: %w", err)
	}
	return nil
}

// Update saves a run.
func (r *AgentRunRepository) Update(ctx context.Context, run *model.AgentRun) error {
	if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
		if isMissingAgentRunVersionColumn(err) {
			if err := r.db.WithContext(ctx).Omit("AgentVersionID", "agent_version_id").Save(run).Error; err != nil {
				return fmt.Errorf("update agent run: %w", err)
			}
		} else {
			return fmt.Errorf("update agent run: %w", err)
		}
	}
	if r.triggerExecutionRepo != nil {
		_ = r.triggerExecutionRepo.SyncRunStatus(ctx, run)
	}
	return nil
}

// UpdateOutputSummary updates only the run output summary.
func (r *AgentRunRepository) UpdateOutputSummary(ctx context.Context, runID string, outputSummary json.RawMessage) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("agent run repository is not configured")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("id = ?", runID).
		Update("output_summary", outputSummary).
		Error; err != nil {
		return fmt.Errorf("update agent run output summary: %w", err)
	}
	return nil
}

// UpdateInput replaces a run's immutable-at-launch input metadata only when a
// dock chat receives a new user turn with a different attached context.
func (r *AgentRunRepository) UpdateInput(ctx context.Context, workspaceID, runID string, input json.RawMessage) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("agent run repository is not configured")
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("workspace_id = ? AND id = ?", workspaceID, runID).
		Update("input", input).Error; err != nil {
		return fmt.Errorf("update agent run input: %w", err)
	}
	return nil
}

// UpdateReconciledFailure persists only the fields changed when a stale run is
// failed during read-time reconciliation. The targeted update keeps projected
// list rows from overwriting large fields that were intentionally not loaded.
func (r *AgentRunRepository) UpdateReconciledFailure(ctx context.Context, run *model.AgentRun) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("agent run repository is not configured")
	}
	if run == nil {
		return fmt.Errorf("agent run is required")
	}
	updates := map[string]any{
		"status":            run.Status,
		"pause_reason":      run.PauseReason,
		"completed_at":      run.CompletedAt,
		"error_message":     run.ErrorMessage,
		"execution_stage":   run.ExecutionStage,
		"last_heartbeat_at": run.LastHeartbeatAt,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("workspace_id = ? AND id = ?", run.WorkspaceID, run.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update reconciled agent run failure: %w", err)
	}
	if r.triggerExecutionRepo != nil {
		if err := r.triggerExecutionRepo.SyncRunStatus(ctx, run); err != nil {
			return fmt.Errorf("sync reconciled agent run status: %w", err)
		}
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

// UpdateRuntimeResumeState updates local bookkeeping fields after a delegated
// runtime resume. Runtime event projection owns status and pause fields.
func (r *AgentRunRepository) UpdateRuntimeResumeState(ctx context.Context, workspaceID, runID, approvalState, stage string, heartbeatAt *time.Time) error {
	updates := map[string]any{
		"approval_state":  approvalState,
		"execution_stage": stage,
	}
	if heartbeatAt != nil {
		updates["last_heartbeat_at"] = heartbeatAt
	}
	if err := r.db.WithContext(ctx).
		Model(&model.AgentRun{}).
		Where("workspace_id = ? AND id = ?", workspaceID, runID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update agent run runtime resume state: %w", err)
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

// AutomationRuleExecutionCounts contains lifetime execution totals used by the
// Flows inventory read model.
type AutomationRuleExecutionCounts struct {
	Total  int64
	Failed int64
}

// CountAutomationRuleExecutions returns lifetime total and failed trigger
// execution counts for each automation rule reference in the workspace.
func (r *AgentTriggerExecutionRepository) CountAutomationRuleExecutions(ctx context.Context, workspaceID string, ruleIDs []string) (map[string]AutomationRuleExecutionCounts, error) {
	if len(ruleIDs) == 0 {
		return map[string]AutomationRuleExecutionCounts{}, nil
	}

	type countRow struct {
		ReferenceID string
		Total       int64
		Failed      int64
	}
	var rows []countRow
	if err := r.db.WithContext(ctx).
		Model(&model.AgentTriggerExecution{}).
		Select("reference_id, COUNT(*) AS total, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS failed", model.AgentTriggerExecutionStatusFailed).
		Where("workspace_id = ? AND binding_kind = ? AND reference_type = ? AND reference_id IN ?", workspaceID, "automation_rule", "automation_rule", ruleIDs).
		Group("reference_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count automation rule executions: %w", err)
	}

	counts := make(map[string]AutomationRuleExecutionCounts, len(rows))
	for _, row := range rows {
		refID := strings.TrimSpace(row.ReferenceID)
		if refID == "" {
			continue
		}
		counts[refID] = AutomationRuleExecutionCounts{Total: row.Total, Failed: row.Failed}
	}
	return counts, nil
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

	if filters.ExecutionID != nil && strings.TrimSpace(*filters.ExecutionID) != "" {
		query = query.Where("id = ?", strings.TrimSpace(*filters.ExecutionID))
	}
	if values := splitCSVFilter(filters.AgentID); len(values) > 0 {
		query = query.Where("agent_id IN ?", values)
	}
	if values := splitCSVFilter(filters.BindingID); len(values) > 0 {
		query = query.Where("binding_id IN ?", values)
	}
	if values := splitCSVFilter(filters.TriggerType); len(values) > 0 {
		query = query.Where("trigger_type IN ?", values)
	}
	if values := splitCSVFilter(filters.BindingKind); len(values) > 0 {
		query = query.Where("binding_kind IN ?", values)
	}
	if values := splitCSVFilter(filters.Status); len(values) > 0 {
		query = query.Where("status IN ?", values)
	}
	if values := splitCSVFilter(filters.ReferenceID); len(values) > 0 {
		query = query.Where("reference_id IN ?", values)
	}
	if filters.RunID != nil && strings.TrimSpace(*filters.RunID) != "" {
		query = query.Where("run_id = ?", strings.TrimSpace(*filters.RunID))
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

// GetByIDAndWorkspace returns one artifact without allowing cross-workspace lookup.
func (r *AgentRunArtifactRepository) GetByIDAndWorkspace(ctx context.Context, workspaceID, artifactID string) (*model.AgentRunArtifact, error) {
	var artifact model.AgentRunArtifact
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, artifactID).First(&artifact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get run artifact: %w", err)
	}
	return &artifact, nil
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
