package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMTaskRepository handles DB operations for tasks and relations.
type PMTaskRepository struct {
	db                    *gorm.DB
	inMutationTransaction bool
}

// WithMutationTransaction runs task and checklist mutations on the same database
// transaction. Repository-owned transactions keep GORM out of the service layer.
func (r *PMTaskRepository) WithMutationTransaction(ctx context.Context, fn func(*PMTaskRepository, *PMChecklistItemRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&PMTaskRepository{db: tx, inMutationTransaction: true}, &PMChecklistItemRepository{db: tx})
	})
}

func (r *PMTaskRepository) withTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	if r.inMutationTransaction {
		return fn(r.db.WithContext(ctx))
	}
	return r.db.WithContext(ctx).Transaction(fn)
}

const boardDoneGroupThisWeekLabel = "This Week"

// NewPMTaskRepository creates a new PMTaskRepository.
func NewPMTaskRepository(db *gorm.DB) *PMTaskRepository {
	return &PMTaskRepository{db: db}
}

func boardTaskOrderClause(stateType string) string {
	if stateType == "done" {
		return "COALESCE(completed_at, moved_at, updated_at) DESC, updated_at DESC, position ASC"
	}
	return "position ASC, updated_at DESC"
}

func loadWorkflowStateType(tx *gorm.DB, stateID string) (string, error) {
	var state struct {
		StateType string
	}
	if err := tx.Model(&model.PMWorkflowState{}).
		Select("state_type").
		Where("id = ?", stateID).
		Take(&state).Error; err != nil {
		return "", fmt.Errorf("load workflow state: %w", err)
	}
	return state.StateType, nil
}

func memberBoardTaskOrderClause() string {
	return "CASE ws.state_type WHEN 'backlog' THEN 0 WHEN 'unstarted' THEN 1 WHEN 'started' THEN 2 WHEN 'done' THEN 3 ELSE 4 END, ws.position ASC, pm_tasks.position ASC, pm_tasks.updated_at DESC"
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func splitFilterValues(value *string) []string {
	if value == nil || *value == "" {
		return nil
	}

	parts := strings.Split(*value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}

func applyTaskStringFilter(q *gorm.DB, column string, value *string) *gorm.DB {
	values := splitFilterValues(value)
	if len(values) == 0 {
		return q
	}
	if len(values) == 1 {
		return q.Where(column+" = ?", values[0])
	}
	return q.Where(column+" IN ?", values)
}

func applyTaskOwnerMemberIDsFilter(q *gorm.DB, memberIDs []string) *gorm.DB {
	if len(memberIDs) == 0 {
		return q
	}
	return q.Where(`EXISTS (
		SELECT 1
		FROM pm_task_owners po
		JOIN workspace_members wm ON wm.user_id = po.user_id
		WHERE po.task_id = pm_tasks.id
		  AND wm.workspace_id = pm_tasks.workspace_id
		  AND wm.id IN ?
	)`, memberIDs)
}

func applyTaskAssociationFilter(q *gorm.DB, objectType string, value *string) *gorm.DB {
	values := splitFilterValues(value)
	if len(values) == 0 {
		return q
	}

	return q.Where(
		`EXISTS (
			SELECT 1
			FROM crm_associations ca
			WHERE ca.workspace_id = pm_tasks.workspace_id
			  AND (
			    (ca.from_object_type = ? AND ca.from_object_id = pm_tasks.id AND ca.to_object_type = ? AND ca.to_object_id IN ?)
			    OR
			    (ca.to_object_type = ? AND ca.to_object_id = pm_tasks.id AND ca.from_object_type = ? AND ca.from_object_id IN ?)
			  )
		)`,
		model.CRMObjectTask, objectType, values,
		model.CRMObjectTask, objectType, values,
	)
}

func applyTaskCompanyRollupFilter(q *gorm.DB, value *string) *gorm.DB {
	if value == nil || strings.TrimSpace(*value) == "" {
		return q
	}
	companyID := strings.TrimSpace(*value)
	return q.Where(`EXISTS (
		SELECT 1 FROM crm_associations ca
		WHERE ca.workspace_id = pm_tasks.workspace_id
		  AND (
		    (ca.from_object_type = 'task' AND ca.from_object_id = pm_tasks.id AND ca.to_object_type = 'company' AND ca.to_object_id = ?)
		    OR (ca.to_object_type = 'task' AND ca.to_object_id = pm_tasks.id AND ca.from_object_type = 'company' AND ca.from_object_id = ?)
		    OR (ca.from_object_type = 'task' AND ca.from_object_id = pm_tasks.id AND ca.to_object_type = 'contact' AND ca.to_object_id IN (
		      SELECT CASE WHEN cca.from_object_type = 'contact' THEN cca.from_object_id ELSE cca.to_object_id END
		      FROM crm_associations cca WHERE cca.workspace_id = pm_tasks.workspace_id
		        AND ((cca.from_object_type = 'contact' AND cca.to_object_type = 'company' AND cca.to_object_id = ?)
		          OR (cca.to_object_type = 'contact' AND cca.from_object_type = 'company' AND cca.from_object_id = ?))
		    ))
		    OR (ca.to_object_type = 'task' AND ca.to_object_id = pm_tasks.id AND ca.from_object_type = 'contact' AND ca.from_object_id IN (
		      SELECT CASE WHEN cca.from_object_type = 'contact' THEN cca.from_object_id ELSE cca.to_object_id END
		      FROM crm_associations cca WHERE cca.workspace_id = pm_tasks.workspace_id
		        AND ((cca.from_object_type = 'contact' AND cca.to_object_type = 'company' AND cca.to_object_id = ?)
		          OR (cca.to_object_type = 'contact' AND cca.from_object_type = 'company' AND cca.from_object_id = ?))
		    ))
		  )
	)`, companyID, companyID, companyID, companyID, companyID, companyID)
}

func applyTaskSupportConversationFilter(q *gorm.DB, value *string) *gorm.DB {
	values := splitFilterValues(value)
	if len(values) == 0 {
		return q
	}

	return q.Where(
		`(
			EXISTS (
				SELECT 1
				FROM crm_associations ca
				WHERE ca.workspace_id = pm_tasks.workspace_id
				  AND (
				    (ca.from_object_type = ? AND ca.from_object_id = pm_tasks.id AND ca.to_object_type = ? AND ca.to_object_id IN ?)
				    OR
				    (ca.to_object_type = ? AND ca.to_object_id = pm_tasks.id AND ca.from_object_type = ? AND ca.from_object_id IN ?)
				  )
			)
			OR
			EXISTS (
				SELECT 1
				FROM support_conversations sc
				WHERE sc.workspace_id = pm_tasks.workspace_id
				  AND sc.id IN ?
				  AND sc.linked_task_id = pm_tasks.id
			)
		)`,
		model.CRMObjectTask, model.CRMObjectSupportConversation, values,
		model.CRMObjectTask, model.CRMObjectSupportConversation, values,
		values,
	)
}

func normalizeStateTaskPositions(tx *gorm.DB, workspaceID, stateID string) error {
	stateType, err := loadWorkflowStateType(tx, stateID)
	if err != nil {
		return err
	}

	var tasks []struct {
		ID       string
		Position int
	}
	if err := tx.Model(&model.PMTask{}).
		Select("id, position").
		Where("workspace_id = ? AND workflow_state_id = ? AND archived = false", workspaceID, stateID).
		Order(boardTaskOrderClause(stateType)).
		Find(&tasks).Error; err != nil {
		return fmt.Errorf("load state tasks for normalization: %w", err)
	}

	for index, task := range tasks {
		if task.Position == index {
			continue
		}
		if err := tx.Model(&model.PMTask{}).
			Where("id = ?", task.ID).
			UpdateColumn("position", index).Error; err != nil {
			return fmt.Errorf("normalize state task position: %w", err)
		}
	}

	return nil
}

func normalizeTaskBoardPosition(tx *gorm.DB, workspaceID, stateID, excludeTaskID string, requested *int) (int, error) {
	query := tx.Model(&model.PMTask{}).
		Where("workspace_id = ? AND workflow_state_id = ? AND archived = false", workspaceID, stateID)
	if excludeTaskID != "" {
		query = query.Where("id != ?", excludeTaskID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count board tasks: %w", err)
	}

	maxPosition := int(count)
	if requested == nil {
		return maxPosition, nil
	}
	if *requested < 0 {
		return 0, nil
	}
	if *requested > maxPosition {
		return maxPosition, nil
	}
	return *requested, nil
}

type pmDnDTaskSnapshot struct {
	ID          string
	Position    int
	UpdatedAt   time.Time
	CompletedAt *time.Time
	MovedAt     *time.Time
}

type taskEnrichOptions struct {
	includeContacts  bool
	includeCompanies bool
	includeDeals     bool
	includeSupport   bool
}

func summarizePMDnDTaskStateSnapshot(tx *gorm.DB, workspaceID, stateID string) ([]string, error) {
	stateType, err := loadWorkflowStateType(tx, stateID)
	if err != nil {
		return nil, err
	}

	var tasks []pmDnDTaskSnapshot
	if err := tx.Model(&model.PMTask{}).
		Select("id, position, updated_at, completed_at, moved_at").
		Where("workspace_id = ? AND workflow_state_id = ? AND archived = false", workspaceID, stateID).
		Order(boardTaskOrderClause(stateType)).
		Limit(10).
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("load state snapshot: %w", err)
	}

	summary := make([]string, 0, len(tasks))
	for _, task := range tasks {
		sortKey := task.UpdatedAt.UTC()
		if task.CompletedAt != nil {
			sortKey = task.CompletedAt.UTC()
		} else if task.MovedAt != nil {
			sortKey = task.MovedAt.UTC()
		}
		summary = append(summary, fmt.Sprintf("%s@%d#%s", task.ID, task.Position, sortKey.Format(time.RFC3339)))
	}
	return summary, nil
}

func boardTaskGroupDate(task model.PMTask) time.Time {
	if task.CompletedAt != nil {
		return task.CompletedAt.UTC()
	}
	if task.MovedAt != nil {
		return task.MovedAt.UTC()
	}
	return task.UpdatedAt.UTC()
}

func startOfBoardWeek(t time.Time) time.Time {
	utc := t.UTC()
	offset := (int(utc.Weekday()) + 6) % 7
	return time.Date(utc.Year(), utc.Month(), utc.Day()-offset, 0, 0, 0, 0, time.UTC)
}

func buildDoneTaskGroups(tasks []model.BoardTask, now time.Time) []model.TaskGroup {
	if len(tasks) == 0 {
		return nil
	}

	currentWeekStart := startOfBoardWeek(now)
	groups := make([]model.TaskGroup, 0, len(tasks))
	groupIndexByKey := make(map[string]int, len(tasks))

	for _, task := range tasks {
		weekStart := startOfBoardWeek(boardTaskGroupDate(task.PMTask))
		key := weekStart.Format("2006-01-02")
		label := "Week of " + weekStart.Format("Jan 2, 2006")
		if weekStart.Equal(currentWeekStart) {
			label = boardDoneGroupThisWeekLabel
		}

		index, ok := groupIndexByKey[key]
		if !ok {
			index = len(groups)
			groupIndexByKey[key] = index
			groups = append(groups, model.TaskGroup{
				Key:   key,
				Label: label,
				Tasks: []model.BoardTask{},
			})
		}

		groups[index].Tasks = append(groups[index].Tasks, task)
	}

	return groups
}

// List returns tasks with filters and pagination.
func (r *PMTaskRepository) List(ctx context.Context, workspaceID string, filters model.PMTaskFilters, pagination model.PMPagination) ([]model.BoardTask, int64, error) {
	return r.list(ctx, workspaceID, filters, pagination, false)
}

// ListSummary returns tasks with filters and pagination without rich body fields.
func (r *PMTaskRepository) ListSummary(ctx context.Context, workspaceID string, filters model.PMTaskFilters, pagination model.PMPagination) ([]model.BoardTask, int64, error) {
	return r.list(ctx, workspaceID, filters, pagination, true)
}

func (r *PMTaskRepository) list(ctx context.Context, workspaceID string, filters model.PMTaskFilters, pagination model.PMPagination, summary bool) ([]model.BoardTask, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.PMTask{}).Where("workspace_id = ?", workspaceID)

	if filters.Search != nil && strings.TrimSpace(*filters.Search) != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(*filters.Search)) + "%"
		query = query.Where("(LOWER(pm_tasks.name) LIKE ? OR LOWER(COALESCE(pm_tasks.description, '')) LIKE ? OR CAST(pm_tasks.display_id AS TEXT) LIKE ?)", search, search, search)
	}
	query = applyTaskStringFilter(query, "pm_tasks.team_id", filters.TeamID)
	query = applyTaskStringFilter(query, "pm_tasks.epic_id", filters.EpicID)
	query = applyTaskStringFilter(query, "pm_tasks.sprint_id", filters.SprintID)
	query = applyTaskAssociationFilter(query, model.CRMObjectContact, filters.ContactID)
	query = applyTaskAssociationFilter(query, model.CRMObjectCompany, filters.CompanyID)
	query = applyTaskCompanyRollupFilter(query, filters.CompanyRollupID)
	query = applyTaskAssociationFilter(query, model.CRMObjectDeal, filters.DealID)
	query = applyTaskSupportConversationFilter(query, filters.SupportConversationID)
	query = applyTaskStringFilter(query, "pm_tasks.workflow_id", filters.WorkflowID)
	query = applyTaskStringFilter(query, "pm_tasks.workflow_state_id", filters.WorkflowStateID)
	if filters.StateType != nil && strings.TrimSpace(*filters.StateType) != "" {
		query = query.Where(
			"pm_tasks.workflow_state_id IN (SELECT id FROM pm_workflow_states WHERE state_type = ?)",
			strings.TrimSpace(*filters.StateType),
		)
	}
	query = applyTaskStringFilter(query, "pm_tasks.task_type", filters.TaskType)
	query = applyTaskOwnerMemberIDsFilter(query, filters.OwnerMemberIDs)
	query = applyTaskStringFilter(query, "pm_tasks.priority", filters.Priority)
	query = applyTaskStringFilter(query, "pm_tasks.requester_id", filters.RequesterID)
	query = applyTaskStringFilter(query, "pm_tasks.requester_member_id", filters.RequesterMemberID)
	query = applyTaskStringFilter(query, "pm_tasks.severity", filters.Severity)
	if filters.Completed != nil {
		query = query.Where("pm_tasks.completed = ?", *filters.Completed)
	}
	if filters.Blocked != nil && *filters.Blocked != "" {
		query = r.applyDerivedBlockedFilter(query, *filters.Blocked == "true")
	}
	if filters.Blocking != nil && *filters.Blocking != "" {
		query = r.applyBlockingFilter(query, *filters.Blocking == "true")
	}
	if filters.Archived != nil {
		query = query.Where("archived = ?", *filters.Archived)
	}
	if filters.UpdatedAfter != nil && strings.TrimSpace(*filters.UpdatedAfter) != "" {
		query = query.Where("pm_tasks.updated_at >= ?", strings.TrimSpace(*filters.UpdatedAfter))
	}
	if labelValues := splitFilterValues(filters.LabelID); len(labelValues) > 0 {
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM pm_task_labels psl
				WHERE psl.task_id = pm_tasks.id
				  AND psl.label_id IN ?
			)`,
			labelValues,
		)
	}
	if filters.AccessibleTeamIDs != nil {
		if len(filters.AccessibleTeamIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("team_id IN ?", filters.AccessibleTeamIDs)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	if pagination.Offset == nil && page-1 > math.MaxInt/perPage {
		return []model.BoardTask{}, total, nil
	}
	offset := (page - 1) * perPage
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}

	var tasks []model.PMTask
	listQuery := query
	if summary {
		listQuery = taskSummaryQuery(listQuery)
	}
	if err := listQuery.Order("updated_at DESC").Offset(offset).Limit(perPage).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}

	return r.collectAndEnrich(ctx, tasks, taskEnrichOptions{
		includeContacts:  filters.IncludeContacts,
		includeCompanies: filters.IncludeCompanies,
		includeDeals:     filters.IncludeDeals,
		includeSupport:   filters.IncludeSupport,
	}), total, nil
}

// GetByID returns a task detail payload.
func (r *PMTaskRepository) GetByID(ctx context.Context, id string) (*model.TaskDetail, error) {
	var task model.PMTask
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return r.buildTaskDetail(ctx, task)
}

// GetByDisplayID returns a task detail by display ID.
func (r *PMTaskRepository) GetByDisplayID(ctx context.Context, workspaceID string, displayID int) (*model.TaskDetail, error) {
	var task model.PMTask
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND display_id = ?", workspaceID, displayID).
		First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get task by display id: %w", err)
	}
	return r.buildTaskDetail(ctx, task)
}

// GetRawByID returns a raw task model by ID.
func (r *PMTaskRepository) GetRawByID(ctx context.Context, id string) (*model.PMTask, error) {
	var task model.PMTask
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get task raw: %w", err)
	}
	return &task, nil
}

// ListOwnerUserIDs returns distinct task owner user IDs from the join table.
func (r *PMTaskRepository) ListOwnerUserIDs(ctx context.Context, taskID string) ([]string, error) {
	var userIDs []string
	if err := r.db.WithContext(ctx).
		Table("pm_task_owners").
		Where("task_id = ?", taskID).
		Distinct().
		Order("user_id ASC").
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, fmt.Errorf("list task owner user ids: %w", err)
	}
	return userIDs, nil
}

// ListByIDs returns raw tasks by ID for a workspace.
func (r *PMTaskRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.PMTask, error) {
	if len(ids) == 0 {
		return []model.PMTask{}, nil
	}

	var tasks []model.PMTask
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list tasks by ids: %w", err)
	}
	return tasks, nil
}

// ListByEpicID returns raw, non-archived tasks for an epic in a workspace.
func (r *PMTaskRepository) ListByEpicID(ctx context.Context, workspaceID, epicID string) ([]model.PMTask, error) {
	if workspaceID == "" || epicID == "" {
		return []model.PMTask{}, nil
	}
	var tasks []model.PMTask
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND epic_id = ? AND archived = FALSE", workspaceID, epicID).
		Order("display_id ASC, created_at ASC").
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list tasks by epic id: %w", err)
	}
	return tasks, nil
}

// ListByDisplayIDs returns raw tasks by display ID for a workspace.
func (r *PMTaskRepository) ListByDisplayIDs(ctx context.Context, workspaceID string, displayIDs []int) ([]model.PMTask, error) {
	if len(displayIDs) == 0 {
		return []model.PMTask{}, nil
	}

	var tasks []model.PMTask
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND display_id IN ?", workspaceID, displayIDs).
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list tasks by display ids: %w", err)
	}
	return tasks, nil
}

func (r *PMTaskRepository) ListStateNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	var states []model.PMWorkflowState
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list workflow states by ids: %w", err)
	}

	result := make(map[string]string, len(states))
	for _, state := range states {
		result[state.ID] = state.Name
	}
	return result, nil
}

// ListByEpicAndExternalIDs returns raw tasks for an epic keyed by external IDs.
func (r *PMTaskRepository) ListByEpicAndExternalIDs(ctx context.Context, workspaceID, epicID string, externalIDs []string) ([]model.PMTask, error) {
	if len(externalIDs) == 0 {
		return []model.PMTask{}, nil
	}

	var tasks []model.PMTask
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND epic_id = ? AND external_id IN ?", workspaceID, epicID, externalIDs).
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list tasks by epic/external ids: %w", err)
	}
	return tasks, nil
}

// Create inserts a task and auto-populates display_id per workspace.
func (r *PMTaskRepository) Create(ctx context.Context, task *model.PMTask) error {
	return r.withTransaction(ctx, func(tx *gorm.DB) error {
		if task.DisplayID == 0 {
			var maxDisplayID int
			if err := tx.Model(&model.PMTask{}).
				Where("workspace_id = ?", task.WorkspaceID).
				Select("COALESCE(MAX(display_id), 0)").
				Scan(&maxDisplayID).Error; err != nil {
				return fmt.Errorf("allocate display id: %w", err)
			}
			task.DisplayID = maxDisplayID + 1
		}

		if task.WorkflowStateID != "" {
			if err := normalizeStateTaskPositions(tx, task.WorkspaceID, task.WorkflowStateID); err != nil {
				return fmt.Errorf("normalize create state positions: %w", err)
			}
		}

		if err := tx.Create(task).Error; err != nil {
			return fmt.Errorf("create task: %w", err)
		}
		return nil
	})
}

// CreateWithPosition allocates and normalizes the new task's board position on
// the repository's current transaction. It is intended for the outer mutation
// transaction used by PMTaskService.Create.
func (r *PMTaskRepository) CreateWithPosition(ctx context.Context, task *model.PMTask, requested *int) error {
	tx := r.db.WithContext(ctx)
	if task.DisplayID == 0 {
		var maxDisplayID int
		if err := tx.Model(&model.PMTask{}).Where("workspace_id = ?", task.WorkspaceID).Select("COALESCE(MAX(display_id), 0)").Scan(&maxDisplayID).Error; err != nil {
			return fmt.Errorf("allocate display id: %w", err)
		}
		task.DisplayID = maxDisplayID + 1
	}
	if task.WorkflowStateID != "" {
		if err := normalizeStateTaskPositions(tx, task.WorkspaceID, task.WorkflowStateID); err != nil {
			return fmt.Errorf("normalize create state positions: %w", err)
		}
		position, err := normalizeTaskBoardPosition(tx, task.WorkspaceID, task.WorkflowStateID, "", requested)
		if err != nil {
			return fmt.Errorf("calculate create position: %w", err)
		}
		task.Position = position
	}
	if err := tx.Create(task).Error; err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

// GetMaxDisplayID returns the highest display ID currently assigned in a workspace.
func (r *PMTaskRepository) GetMaxDisplayID(ctx context.Context, workspaceID string) (int, error) {
	var maxDisplayID int
	if err := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("workspace_id = ?", workspaceID).
		Select("COALESCE(MAX(display_id), 0)").
		Scan(&maxDisplayID).Error; err != nil {
		return 0, fmt.Errorf("get max task display id: %w", err)
	}
	return maxDisplayID, nil
}

// CreateInBatches inserts a set of tasks in batches.
func (r *PMTaskRepository) CreateInBatches(ctx context.Context, tasks []model.PMTask, batchSize int) error {
	if len(tasks) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(tasks, batchSize).Error; err != nil {
		return fmt.Errorf("create tasks in batches: %w", err)
	}
	return nil
}

// NextPosition returns the next append position for a workflow state after
// normalizing any existing duplicate or sparse positions in that column.
func (r *PMTaskRepository) NextPosition(ctx context.Context, workspaceID, stateID string) (int, error) {
	tx := r.db.WithContext(ctx)
	if err := normalizeStateTaskPositions(tx, workspaceID, stateID); err != nil {
		return 0, fmt.Errorf("normalize next position state: %w", err)
	}
	position, err := normalizeTaskBoardPosition(tx, workspaceID, stateID, "", nil)
	if err != nil {
		return 0, fmt.Errorf("calculate next position: %w", err)
	}
	return position, nil
}

// Update updates a task model.
func (r *PMTaskRepository) Update(ctx context.Context, task *model.PMTask) error {
	if err := r.db.WithContext(ctx).Save(task).Error; err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

// UpdateFields updates specific fields on a task by ID.
func (r *PMTaskRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	if err := r.db.WithContext(ctx).Model(&model.PMTask{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("update task fields: %w", err)
	}
	return nil
}

// Delete archives a task.
func (r *PMTaskRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("id = ?", id).
		Update("archived", true).Error; err != nil {
		return fmt.Errorf("archive task: %w", err)
	}
	return nil
}

// MoveToState moves a task to a new state at the given position,
// renumbering siblings in both the source and target columns transactionally.
func (r *PMTaskRepository) MoveToState(ctx context.Context, taskID, stateID string, position *int, debugTraceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Fetch the task to get its current state and position.
		var task model.PMTask
		if err := tx.Select("id, workflow_state_id, position, workspace_id").
			Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("move task fetch: %w", err)
		}

		oldStateID := task.WorkflowStateID
		if err := normalizeStateTaskPositions(tx, task.WorkspaceID, oldStateID); err != nil {
			return fmt.Errorf("normalize source state positions: %w", err)
		}
		if stateID != oldStateID {
			if err := normalizeStateTaskPositions(tx, task.WorkspaceID, stateID); err != nil {
				return fmt.Errorf("normalize target state positions: %w", err)
			}
		}
		if err := tx.Select("id, workflow_state_id, position, workspace_id").
			Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("move task refetch: %w", err)
		}

		normalizedPosition, err := normalizeTaskBoardPosition(tx, task.WorkspaceID, stateID, taskID, position)
		if err != nil {
			return fmt.Errorf("move task normalize position: %w", err)
		}
		slog.InfoContext(ctx, "[pm-dnd] repo move normalized",
			"trace_id", debugTraceID,
			"task_id", taskID,
			"workspace_id", task.WorkspaceID,
			"from_state_id", oldStateID,
			"to_state_id", stateID,
			"old_position", task.Position,
			"requested_position", position,
			"normalized_position", normalizedPosition,
		)

		// Close the gap in the source column: shift siblings above the old position down by 1.
		if err := tx.Model(&model.PMTask{}).
			Where("workspace_id = ? AND workflow_state_id = ? AND position > ? AND id != ? AND archived = false",
				task.WorkspaceID, oldStateID, task.Position, taskID).
			UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
			return fmt.Errorf("move task close source gap: %w", err)
		}

		// Open a gap in the target column: shift siblings at or above the target position up by 1.
		if err := tx.Model(&model.PMTask{}).
			Where("workspace_id = ? AND workflow_state_id = ? AND position >= ? AND id != ? AND archived = false",
				task.WorkspaceID, stateID, normalizedPosition, taskID).
			UpdateColumn("position", gorm.Expr("position + 1")).Error; err != nil {
			return fmt.Errorf("move task open target gap: %w", err)
		}

		// Update the task itself.
		now := time.Now().UTC()
		if err := tx.Model(&model.PMTask{}).Where("id = ?", taskID).
			Updates(map[string]interface{}{
				"workflow_state_id": stateID,
				"position":          normalizedPosition,
				"moved_at":          now,
			}).Error; err != nil {
			return fmt.Errorf("move task update: %w", err)
		}

		fromSummary, fromErr := summarizePMDnDTaskStateSnapshot(tx, task.WorkspaceID, oldStateID)
		toSummary, toErr := summarizePMDnDTaskStateSnapshot(tx, task.WorkspaceID, stateID)
		slog.InfoContext(ctx, "[pm-dnd] repo move applied",
			"trace_id", debugTraceID,
			"task_id", taskID,
			"from_state_id", oldStateID,
			"to_state_id", stateID,
			"from_state_order", fromSummary,
			"to_state_order", toSummary,
			"from_state_snapshot_error", fromErr,
			"to_state_snapshot_error", toErr,
		)

		return nil
	})
}

// Reorder moves a task to a new position within its current column,
// renumbering siblings transactionally to keep positions contiguous.
func (r *PMTaskRepository) Reorder(ctx context.Context, taskID string, position int, debugTraceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.PMTask
		if err := tx.Select("id, workflow_state_id, position, workspace_id").
			Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("reorder task fetch: %w", err)
		}
		if err := normalizeStateTaskPositions(tx, task.WorkspaceID, task.WorkflowStateID); err != nil {
			return fmt.Errorf("normalize reorder state positions: %w", err)
		}
		if err := tx.Select("id, workflow_state_id, position, workspace_id").
			Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("reorder task refetch: %w", err)
		}

		normalizedPosition, err := normalizeTaskBoardPosition(tx, task.WorkspaceID, task.WorkflowStateID, taskID, &position)
		if err != nil {
			return fmt.Errorf("reorder normalize position: %w", err)
		}
		slog.InfoContext(ctx, "[pm-dnd] repo reorder normalized",
			"trace_id", debugTraceID,
			"task_id", taskID,
			"workspace_id", task.WorkspaceID,
			"state_id", task.WorkflowStateID,
			"old_position", task.Position,
			"requested_position", position,
			"normalized_position", normalizedPosition,
		)

		oldPos := task.Position
		if oldPos == normalizedPosition {
			return nil
		}

		if normalizedPosition < oldPos {
			// Moving up: shift tasks in [newPos, oldPos) down by 1
			if err := tx.Model(&model.PMTask{}).
				Where("workspace_id = ? AND workflow_state_id = ? AND position >= ? AND position < ? AND id != ? AND archived = false",
					task.WorkspaceID, task.WorkflowStateID, normalizedPosition, oldPos, taskID).
				UpdateColumn("position", gorm.Expr("position + 1")).Error; err != nil {
				return fmt.Errorf("reorder shift up: %w", err)
			}
		} else {
			// Moving down: shift tasks in (oldPos, newPos] up by 1
			if err := tx.Model(&model.PMTask{}).
				Where("workspace_id = ? AND workflow_state_id = ? AND position > ? AND position <= ? AND id != ? AND archived = false",
					task.WorkspaceID, task.WorkflowStateID, oldPos, normalizedPosition, taskID).
				UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
				return fmt.Errorf("reorder shift down: %w", err)
			}
		}

		// Set the task's new position.
		if err := tx.Model(&model.PMTask{}).Where("id = ?", taskID).
			Update("position", normalizedPosition).Error; err != nil {
			return fmt.Errorf("reorder update: %w", err)
		}

		summary, snapshotErr := summarizePMDnDTaskStateSnapshot(tx, task.WorkspaceID, task.WorkflowStateID)
		slog.InfoContext(ctx, "[pm-dnd] repo reorder applied",
			"trace_id", debugTraceID,
			"task_id", taskID,
			"state_id", task.WorkflowStateID,
			"state_order", summary,
			"state_snapshot_error", snapshotErr,
		)

		return nil
	})
}

// AddOwner links an owner to a task.
func (r *PMTaskRepository) AddOwner(ctx context.Context, taskID, userID string) error {
	owner := model.PMTaskOwner{TaskID: taskID, UserID: userID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&owner).Error; err != nil {
		return fmt.Errorf("add task owner: %w", err)
	}
	return nil
}

// RemoveOwner unlinks an owner from a task.
func (r *PMTaskRepository) RemoveOwner(ctx context.Context, taskID, userID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMTaskOwner{}, "task_id = ? AND user_id = ?", taskID, userID).Error; err != nil {
		return fmt.Errorf("remove task owner: %w", err)
	}
	return nil
}

// AddFollower links a follower to a task.
func (r *PMTaskRepository) AddFollower(ctx context.Context, taskID, userID string) error {
	follower := model.PMTaskFollower{TaskID: taskID, UserID: userID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&follower).Error; err != nil {
		return fmt.Errorf("add task follower: %w", err)
	}
	return nil
}

// RemoveFollower unlinks a follower from a task.
func (r *PMTaskRepository) RemoveFollower(ctx context.Context, taskID, userID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMTaskFollower{}, "task_id = ? AND user_id = ?", taskID, userID).Error; err != nil {
		return fmt.Errorf("remove task follower: %w", err)
	}
	return nil
}

// AddLabel links a label to a task.
func (r *PMTaskRepository) AddLabel(ctx context.Context, taskID, labelID string) error {
	link := model.PMTaskLabel{TaskID: taskID, LabelID: labelID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&link).Error; err != nil {
		return fmt.Errorf("add task label: %w", err)
	}
	return nil
}

// RemoveLabel unlinks a label from a task.
func (r *PMTaskRepository) RemoveLabel(ctx context.Context, taskID, labelID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMTaskLabel{}, "task_id = ? AND label_id = ?", taskID, labelID).Error; err != nil {
		return fmt.Errorf("remove task label: %w", err)
	}
	return nil
}

// ReplaceOwners replaces all task owners.
func (r *PMTaskRepository) ReplaceOwners(ctx context.Context, taskID string, userIDs []string) error {
	return r.withTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMTaskOwner{}, "task_id = ?", taskID).Error; err != nil {
			return fmt.Errorf("clear task owners: %w", err)
		}
		for _, userID := range userIDs {
			if err := tx.Create(&model.PMTaskOwner{TaskID: taskID, UserID: userID}).Error; err != nil {
				return fmt.Errorf("replace task owners: %w", err)
			}
		}
		return nil
	})
}

// ReplaceFollowers replaces all task followers.
func (r *PMTaskRepository) ReplaceFollowers(ctx context.Context, taskID string, userIDs []string) error {
	return r.withTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMTaskFollower{}, "task_id = ?", taskID).Error; err != nil {
			return fmt.Errorf("clear task followers: %w", err)
		}
		for _, userID := range userIDs {
			if err := tx.Create(&model.PMTaskFollower{TaskID: taskID, UserID: userID}).Error; err != nil {
				return fmt.Errorf("replace task followers: %w", err)
			}
		}
		return nil
	})
}

// ReplaceLabels replaces all task labels.
func (r *PMTaskRepository) ReplaceLabels(ctx context.Context, taskID string, labelIDs []string) error {
	return r.withTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMTaskLabel{}, "task_id = ?", taskID).Error; err != nil {
			return fmt.Errorf("clear task labels: %w", err)
		}
		for _, labelID := range labelIDs {
			if err := tx.Create(&model.PMTaskLabel{TaskID: taskID, LabelID: labelID}).Error; err != nil {
				return fmt.Errorf("replace task labels: %w", err)
			}
		}
		return nil
	})
}

// ListByWorkflowState returns board columns grouped by workflow state with optional filters.
// perStateLimit controls how many tasks are returned per column (0 = unlimited).
func (r *PMTaskRepository) ListByWorkflowState(ctx context.Context, workflowID string, filters model.PMTaskFilters, perStateLimit int) ([]model.TaskStateColumn, error) {
	var states []model.PMWorkflowState
	if err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("CASE state_type WHEN 'backlog' THEN 0 WHEN 'unstarted' THEN 1 WHEN 'started' THEN 2 WHEN 'done' THEN 3 ELSE 4 END, position ASC").
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list board states: %w", err)
	}

	stateIDs := make([]string, len(states))
	for i, s := range states {
		stateIDs[i] = s.ID
	}

	baseQuery := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("workflow_state_id IN ?", stateIDs)
	baseQuery = r.applyBoardFilters(baseQuery, filters)

	var aggregateRows []struct {
		StateID    string `gorm:"column:state_id"`
		TaskCount  int    `gorm:"column:task_count"`
		PointTotal int    `gorm:"column:point_total"`
	}
	if err := baseQuery.
		Select("pm_tasks.workflow_state_id AS state_id, COUNT(pm_tasks.id) AS task_count, COALESCE(SUM(pm_tasks.estimate), 0) AS point_total").
		Group("pm_tasks.workflow_state_id").
		Scan(&aggregateRows).Error; err != nil {
		return nil, fmt.Errorf("list board aggregates: %w", err)
	}

	type aggregateMeta struct {
		taskCount  int
		pointTotal int
	}
	aggregates := map[string]aggregateMeta{}
	for _, row := range aggregateRows {
		aggregates[row.StateID] = aggregateMeta{taskCount: row.TaskCount, pointTotal: row.PointTotal}
	}

	type columnMeta struct {
		totalCount int
		pointTotal int
		hasMore    bool
		visible    []model.PMTask
	}
	metas := make([]columnMeta, len(states))
	var allTasks []model.PMTask
	for i, state := range states {
		query := r.db.WithContext(ctx).
			Where("workflow_state_id = ?", state.ID)
		query = r.applyBoardFilters(query, filters)
		query = query.Order(boardTaskOrderClause(state.StateType))
		if perStateLimit > 0 {
			query = query.Limit(perStateLimit)
		}

		var visibleTasks []model.PMTask
		if err := taskSummaryQuery(query).Find(&visibleTasks).Error; err != nil {
			return nil, fmt.Errorf("list board column tasks: %w", err)
		}

		allTasks = append(allTasks, visibleTasks...)

		aggregate := aggregates[state.ID]
		metas[i] = columnMeta{
			totalCount: aggregate.taskCount,
			pointTotal: aggregate.pointTotal,
			hasMore:    aggregate.taskCount > len(visibleTasks),
			visible:    visibleTasks,
		}
	}

	enriched := r.collectAndEnrich(ctx, allTasks, taskEnrichOptions{
		includeContacts:  filters.IncludeContacts,
		includeCompanies: filters.IncludeCompanies,
		includeDeals:     filters.IncludeDeals,
		includeSupport:   filters.IncludeSupport,
	})
	// Build a map from task ID → BoardTask for column assembly.
	enrichedMap := make(map[string]model.BoardTask, len(enriched))
	for _, bs := range enriched {
		enrichedMap[bs.ID] = bs
	}

	columns := make([]model.TaskStateColumn, 0, len(states))
	for i, state := range states {
		m := metas[i]
		colTasks := make([]model.BoardTask, 0, len(m.visible))
		for _, s := range m.visible {
			colTasks = append(colTasks, enrichedMap[s.ID])
		}
		var taskGroups []model.TaskGroup
		if state.StateType == "done" {
			taskGroups = buildDoneTaskGroups(colTasks, time.Now().UTC())
		}
		columns = append(columns, model.TaskStateColumn{
			State:      state,
			Tasks:      colTasks,
			TaskGroups: taskGroups,
			TaskCount:  m.totalCount,
			PointTotal: m.pointTotal,
			HasMore:    m.hasMore,
		})
	}
	return columns, nil
}

// ListColumnTasks returns a page of tasks for a single workflow state, enriched for board display.
func (r *PMTaskRepository) ListColumnTasks(ctx context.Context, stateID string, filters model.PMTaskFilters, offset, limit int) ([]model.BoardTask, []model.TaskGroup, int, error) {
	var state model.PMWorkflowState
	if err := r.db.WithContext(ctx).Where("id = ?", stateID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, fmt.Errorf("workflow state not found")
		}
		return nil, nil, 0, fmt.Errorf("get workflow state: %w", err)
	}

	taskQuery := r.db.WithContext(ctx).
		Where("workflow_state_id = ?", stateID)
	taskQuery = r.applyBoardFilters(taskQuery, filters)

	var total int64
	if err := taskQuery.Model(&model.PMTask{}).Count(&total).Error; err != nil {
		return nil, nil, 0, fmt.Errorf("count column tasks: %w", err)
	}

	var tasks []model.PMTask
	if err := taskSummaryQuery(taskQuery).
		Order(boardTaskOrderClause(state.StateType)).
		Offset(offset).Limit(limit).
		Find(&tasks).Error; err != nil {
		return nil, nil, 0, fmt.Errorf("list column tasks: %w", err)
	}

	enriched := r.collectAndEnrich(ctx, tasks, taskEnrichOptions{
		includeContacts:  filters.IncludeContacts,
		includeCompanies: filters.IncludeCompanies,
		includeDeals:     filters.IncludeDeals,
		includeSupport:   filters.IncludeSupport,
	})
	var taskGroups []model.TaskGroup
	if state.StateType == "done" {
		taskGroups = buildDoneTaskGroups(enriched, time.Now().UTC())
	}
	return enriched, taskGroups, int(total), nil
}

// collectAndEnrich collects related IDs from tasks, batch-loads names/labels, and returns enriched BoardTask slices.
func (r *PMTaskRepository) collectAndEnrich(ctx context.Context, tasks []model.PMTask, options taskEnrichOptions) []model.BoardTask {
	tasks = r.applyDependencySummaries(ctx, tasks)
	tasks = r.applyLatestRunMetadata(ctx, tasks)
	epicIDs := map[string]struct{}{}
	sprintIDs := map[string]struct{}{}
	stateIDs := map[string]struct{}{}
	taskIDs := make([]string, len(tasks))
	for i, s := range tasks {
		taskIDs[i] = s.ID
		if s.EpicID != nil {
			epicIDs[*s.EpicID] = struct{}{}
		}
		if s.SprintID != nil {
			sprintIDs[*s.SprintID] = struct{}{}
		}
		stateIDs[s.WorkflowStateID] = struct{}{}
	}
	epicNameMap := r.batchEpicNames(ctx, epicIDs)
	sprintNameMap := r.batchSprintNames(ctx, sprintIDs)
	ownerMemberIDsByTaskID := r.batchTaskOwnerMemberIDs(ctx, taskIDs)
	labelMap := r.batchTaskLabels(ctx, taskIDs)
	stateInfoMap := r.batchStateInfo(ctx, stateIDs)
	contactsMap := map[string][]model.AssociationObjectSummary{}
	companiesMap := map[string][]model.AssociationObjectSummary{}
	dealsMap := map[string][]model.AssociationObjectSummary{}
	supportMap := map[string][]model.AssociationObjectSummary{}
	if options.includeContacts || options.includeCompanies || options.includeDeals || options.includeSupport {
		contactsMap, companiesMap, dealsMap, supportMap = r.batchTaskAssociations(ctx, taskIDs, options)
	}
	return r.enrichBoardTasks(tasks, epicNameMap, sprintNameMap, ownerMemberIDsByTaskID, labelMap, stateInfoMap, contactsMap, companiesMap, dealsMap, supportMap)
}

// EnrichTasksForList applies the same table-facing computed fields used by
// board/list endpoints to an already-scoped task slice.
func (r *PMTaskRepository) EnrichTasksForList(ctx context.Context, tasks []model.PMTask) []model.BoardTask {
	return r.collectAndEnrich(ctx, tasks, taskEnrichOptions{})
}

// stateInfo holds denormalized workflow state metadata for board tasks.
type stateInfo struct {
	Name      string
	StateType string
	Color     string
}

// latestRunRow is a scan row for the most recent agent_run per task.
type latestRunRow struct {
	TargetID    string     `gorm:"column:target_id"`
	ID          string     `gorm:"column:id"`
	AgentID     string     `gorm:"column:agent_id"`
	Status      string     `gorm:"column:status"`
	PauseReason string     `gorm:"column:pause_reason"`
	StartedAt   *time.Time `gorm:"column:started_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
}

// applyLatestRunMetadata populates latest task-targeted run metadata on each
// task so the UI can show the most recent run agent without task assignment
// state.
func (r *PMTaskRepository) applyLatestRunMetadata(ctx context.Context, tasks []model.PMTask) []model.PMTask {
	if len(tasks) == 0 {
		return tasks
	}
	if r == nil || r.db == nil || !r.db.Migrator().HasTable("agent_runs") {
		return tasks
	}
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	var rows []latestRunRow
	if err := r.db.WithContext(ctx).
		Table("agent_runs").
		Select("target_id, id, agent_id, status, pause_reason, started_at, created_at").
		Where("target_type = ? AND target_id IN ?", "task", ids).
		Order("target_id, COALESCE(started_at, created_at) DESC, created_at DESC").
		Find(&rows).Error; err != nil {
		slog.WarnContext(ctx, "load latest task agent runs", "error", err)
		return tasks
	}
	latest := make(map[string]latestRunRow, len(rows))
	for _, row := range rows {
		if _, ok := latest[row.TargetID]; !ok {
			latest[row.TargetID] = row
		}
	}
	for i := range tasks {
		row, ok := latest[tasks[i].ID]
		if !ok {
			continue
		}
		runAt := row.CreatedAt
		if row.StartedAt != nil {
			runAt = *row.StartedAt
		}
		runID, runAgentID, runStatus, latestRunAt := row.ID, row.AgentID, row.Status, runAt.UTC()
		tasks[i].LatestRunID = &runID
		tasks[i].LatestRunAgentID = &runAgentID
		tasks[i].LatestRunStatus = &runStatus
		if row.PauseReason != "" && row.PauseReason != "none" {
			pauseReason := row.PauseReason
			tasks[i].LatestRunPauseReason = &pauseReason
		}
		tasks[i].LatestRunAt = &latestRunAt
	}
	return tasks
}

// enrichBoardTasks maps epic/owner names, state info, and labels onto raw tasks for board display.
func (r *PMTaskRepository) enrichBoardTasks(
	tasks []model.PMTask,
	epicNameMap, sprintNameMap map[string]string,
	ownerMemberIDsByTaskID map[string][]string,
	labelMap map[string][]model.PMLabel,
	stateInfoMap map[string]stateInfo,
	contactsMap, companiesMap, dealsMap, supportMap map[string][]model.AssociationObjectSummary,
) []model.BoardTask {
	result := make([]model.BoardTask, 0, len(tasks))
	for _, task := range tasks {
		bs := model.BoardTask{
			PMTask:               task,
			OwnerMemberIDs:       ownerMemberIDsByTaskID[task.ID],
			Labels:               []model.PMLabel{},
			Contacts:             contactsMap[task.ID],
			Companies:            companiesMap[task.ID],
			Deals:                dealsMap[task.ID],
			SupportConversations: supportMap[task.ID],
		}
		bs.PMTask.OwnerMemberIDs = ownerMemberIDsByTaskID[task.ID]
		if task.EpicID != nil {
			if name, ok := epicNameMap[*task.EpicID]; ok {
				bs.EpicName = &name
			}
		}
		if task.SprintID != nil {
			if name, ok := sprintNameMap[*task.SprintID]; ok {
				bs.SprintName = &name
			}
		}
		if labels, ok := labelMap[task.ID]; ok {
			bs.Labels = labels
		}
		if si, ok := stateInfoMap[task.WorkflowStateID]; ok {
			bs.StateName = &si.Name
			bs.StateType = &si.StateType
			bs.StateColor = &si.Color
		}
		result = append(result, bs)
	}
	return result
}

// taskSummaryQuery excludes rich task-body fields from collection queries.
// Full task detail queries intentionally do not use this scope.
func taskSummaryQuery(query *gorm.DB) *gorm.DB {
	return query.Omit("description", "implementation_brief")
}

type taskAssociationLinkRow struct {
	TaskID   string `gorm:"column:task_id"`
	ObjectID string `gorm:"column:object_id"`
}

func (r *PMTaskRepository) batchTaskAssociations(ctx context.Context, taskIDs []string, options taskEnrichOptions) (
	map[string][]model.AssociationObjectSummary,
	map[string][]model.AssociationObjectSummary,
	map[string][]model.AssociationObjectSummary,
	map[string][]model.AssociationObjectSummary,
) {
	contacts := map[string][]model.AssociationObjectSummary{}
	companies := map[string][]model.AssociationObjectSummary{}
	deals := map[string][]model.AssociationObjectSummary{}
	support := map[string][]model.AssociationObjectSummary{}
	if len(taskIDs) == 0 {
		return contacts, companies, deals, support
	}

	if options.includeContacts {
		contacts = r.batchTaskCRMObjectAssociations(ctx, taskIDs, model.CRMObjectContact)
	}
	if options.includeCompanies {
		companies = r.batchTaskCRMObjectAssociations(ctx, taskIDs, model.CRMObjectCompany)
	}
	if options.includeDeals {
		deals = r.batchTaskCRMObjectAssociations(ctx, taskIDs, model.CRMObjectDeal)
	}
	if options.includeSupport {
		support = r.batchTaskSupportConversationAssociations(ctx, taskIDs)
	}
	return contacts, companies, deals, support
}

func (r *PMTaskRepository) batchTaskCRMObjectAssociations(ctx context.Context, taskIDs []string, objectType string) map[string][]model.AssociationObjectSummary {
	result := map[string][]model.AssociationObjectSummary{}
	var links []taskAssociationLinkRow
	if err := r.db.WithContext(ctx).
		Table("crm_associations ca").
		Select(`
			CASE
				WHEN ca.from_object_type = ? THEN ca.from_object_id
				ELSE ca.to_object_id
			END AS task_id,
			CASE
				WHEN ca.from_object_type = ? THEN ca.to_object_id
				ELSE ca.from_object_id
			END AS object_id
		`, model.CRMObjectTask, model.CRMObjectTask).
		Where(`
			(ca.from_object_type = ? AND ca.from_object_id IN ? AND ca.to_object_type = ?)
			OR
			(ca.to_object_type = ? AND ca.to_object_id IN ? AND ca.from_object_type = ?)
		`, model.CRMObjectTask, taskIDs, objectType, model.CRMObjectTask, taskIDs, objectType).
		Scan(&links).Error; err != nil {
		return result
	}
	if len(links) == 0 {
		return result
	}

	objectIDs := make([]string, 0, len(links))
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		if _, ok := seen[link.ObjectID]; ok {
			continue
		}
		seen[link.ObjectID] = struct{}{}
		objectIDs = append(objectIDs, link.ObjectID)
	}

	summaryByID := r.loadAssociationObjectSummaries(ctx, objectType, objectIDs)
	for _, link := range links {
		summary, ok := summaryByID[link.ObjectID]
		if !ok {
			continue
		}
		result[link.TaskID] = append(result[link.TaskID], summary)
	}
	return result
}

func (r *PMTaskRepository) batchTaskSupportConversationAssociations(ctx context.Context, taskIDs []string) map[string][]model.AssociationObjectSummary {
	result := r.batchTaskCRMObjectAssociations(ctx, taskIDs, model.CRMObjectSupportConversation)

	var rows []struct {
		ID           string `gorm:"column:id"`
		LinkedTaskID string `gorm:"column:linked_task_id"`
		DisplayID    int    `gorm:"column:display_id"`
		Subject      string `gorm:"column:subject"`
		Status       string `gorm:"column:status"`
	}
	if err := r.db.WithContext(ctx).
		Table("support_conversations").
		Select("id, linked_task_id, display_id, subject, status").
		Where("linked_task_id IN ?", taskIDs).
		Order("display_id ASC").
		Scan(&rows).Error; err != nil {
		return result
	}

	seen := map[string]map[string]struct{}{}
	for taskID, summaries := range result {
		seen[taskID] = map[string]struct{}{}
		for _, summary := range summaries {
			seen[taskID][summary.ObjectID] = struct{}{}
		}
	}

	for _, row := range rows {
		if _, ok := seen[row.LinkedTaskID]; !ok {
			seen[row.LinkedTaskID] = map[string]struct{}{}
		}
		if _, ok := seen[row.LinkedTaskID][row.ID]; ok {
			continue
		}
		displayID := fmt.Sprintf("%d", row.DisplayID)
		status := row.Status
		result[row.LinkedTaskID] = append(result[row.LinkedTaskID], model.AssociationObjectSummary{
			ObjectType: model.CRMObjectSupportConversation,
			ObjectID:   row.ID,
			DisplayID:  &displayID,
			Title:      row.Subject,
			Status:     &status,
		})
		seen[row.LinkedTaskID][row.ID] = struct{}{}
	}

	return result
}

func (r *PMTaskRepository) loadAssociationObjectSummaries(ctx context.Context, objectType string, objectIDs []string) map[string]model.AssociationObjectSummary {
	result := map[string]model.AssociationObjectSummary{}
	if len(objectIDs) == 0 {
		return result
	}

	switch objectType {
	case model.CRMObjectContact:
		var rows []struct {
			ID        string  `gorm:"column:id"`
			DisplayID string  `gorm:"column:display_id"`
			FirstName string  `gorm:"column:first_name"`
			LastName  *string `gorm:"column:last_name"`
			Email     *string `gorm:"column:email"`
		}
		if err := r.db.WithContext(ctx).
			Table("crm_contacts").
			Select("id, display_id, first_name, last_name, email").
			Where("id IN ?", objectIDs).
			Scan(&rows).Error; err != nil {
			return result
		}
		for _, row := range rows {
			title := strings.TrimSpace(row.FirstName + " " + stringValue(row.LastName))
			if title == "" {
				title = stringValue(row.Email)
			}
			displayID := row.DisplayID
			result[row.ID] = model.AssociationObjectSummary{
				ObjectType: model.CRMObjectContact,
				ObjectID:   row.ID,
				DisplayID:  &displayID,
				Title:      title,
			}
		}
	case model.CRMObjectCompany:
		var rows []struct {
			ID        string `gorm:"column:id"`
			DisplayID string `gorm:"column:display_id"`
			Name      string `gorm:"column:name"`
		}
		if err := r.db.WithContext(ctx).
			Table("crm_companies").
			Select("id, display_id, name").
			Where("id IN ?", objectIDs).
			Scan(&rows).Error; err != nil {
			return result
		}
		for _, row := range rows {
			displayID := row.DisplayID
			result[row.ID] = model.AssociationObjectSummary{
				ObjectType: model.CRMObjectCompany,
				ObjectID:   row.ID,
				DisplayID:  &displayID,
				Title:      row.Name,
			}
		}
	case model.CRMObjectDeal:
		var rows []struct {
			ID        string `gorm:"column:id"`
			DisplayID string `gorm:"column:display_id"`
			Name      string `gorm:"column:name"`
		}
		if err := r.db.WithContext(ctx).
			Table("crm_deals").
			Select("id, display_id, name").
			Where("id IN ?", objectIDs).
			Scan(&rows).Error; err != nil {
			return result
		}
		for _, row := range rows {
			displayID := row.DisplayID
			result[row.ID] = model.AssociationObjectSummary{
				ObjectType: model.CRMObjectDeal,
				ObjectID:   row.ID,
				DisplayID:  &displayID,
				Title:      row.Name,
			}
		}
	case model.CRMObjectSupportConversation:
		var rows []struct {
			ID        string `gorm:"column:id"`
			DisplayID int    `gorm:"column:display_id"`
			Subject   string `gorm:"column:subject"`
			Status    string `gorm:"column:status"`
		}
		if err := r.db.WithContext(ctx).
			Table("support_conversations").
			Select("id, display_id, subject, status").
			Where("id IN ?", objectIDs).
			Scan(&rows).Error; err != nil {
			return result
		}
		for _, row := range rows {
			displayID := fmt.Sprintf("%d", row.DisplayID)
			status := row.Status
			result[row.ID] = model.AssociationObjectSummary{
				ObjectType: model.CRMObjectSupportConversation,
				ObjectID:   row.ID,
				DisplayID:  &displayID,
				Title:      row.Subject,
				Status:     &status,
			}
		}
	}

	return result
}

// applyBoardFilters adds board-specific WHERE clauses to a query (shared between ListByWorkflowState and ListColumnTasks).
func (r *PMTaskRepository) applyBoardFilters(q *gorm.DB, filters model.PMTaskFilters) *gorm.DB {
	q = applyTaskStringFilter(q, "pm_tasks.team_id", filters.TeamID)
	q = applyTaskStringFilter(q, "pm_tasks.priority", filters.Priority)
	q = applyTaskStringFilter(q, "pm_tasks.task_type", filters.TaskType)
	q = applyTaskStringFilter(q, "pm_tasks.epic_id", filters.EpicID)
	q = applyTaskStringFilter(q, "pm_tasks.sprint_id", filters.SprintID)
	if filters.StateType != nil && strings.TrimSpace(*filters.StateType) != "" {
		q = q.Where(
			"pm_tasks.workflow_state_id IN (SELECT id FROM pm_workflow_states WHERE state_type = ?)",
			strings.TrimSpace(*filters.StateType),
		)
	}
	q = applyTaskAssociationFilter(q, model.CRMObjectContact, filters.ContactID)
	q = applyTaskAssociationFilter(q, model.CRMObjectCompany, filters.CompanyID)
	q = applyTaskCompanyRollupFilter(q, filters.CompanyRollupID)
	q = applyTaskAssociationFilter(q, model.CRMObjectDeal, filters.DealID)
	q = applyTaskSupportConversationFilter(q, filters.SupportConversationID)
	q = applyTaskOwnerMemberIDsFilter(q, filters.OwnerMemberIDs)
	q = applyTaskStringFilter(q, "pm_tasks.requester_id", filters.RequesterID)
	q = applyTaskStringFilter(q, "pm_tasks.requester_member_id", filters.RequesterMemberID)
	q = applyTaskStringFilter(q, "pm_tasks.severity", filters.Severity)
	archived := false
	if filters.Archived != nil {
		archived = *filters.Archived
	}
	q = q.Where("pm_tasks.archived = ?", archived)
	if filters.Blocked != nil && *filters.Blocked != "" {
		q = r.applyDerivedBlockedFilter(q, *filters.Blocked == "true")
	}
	if filters.Blocking != nil && *filters.Blocking != "" {
		q = r.applyBlockingFilter(q, *filters.Blocking == "true")
	}
	if filters.UpdatedAfter != nil && *filters.UpdatedAfter != "" {
		t, err := time.Parse(time.RFC3339, *filters.UpdatedAfter)
		if err == nil {
			q = q.Where("pm_tasks.updated_at >= ?", t)
		}
	}
	if labelValues := splitFilterValues(filters.LabelID); len(labelValues) > 0 {
		q = q.Where(
			`EXISTS (
				SELECT 1
				FROM pm_task_labels psl
				WHERE psl.task_id = pm_tasks.id
				  AND psl.label_id IN ?
			)`,
			labelValues,
		)
	}
	if filters.AccessibleTeamIDs != nil {
		if len(filters.AccessibleTeamIDs) == 0 {
			q = q.Where("1 = 0")
		} else {
			q = q.Where("pm_tasks.team_id IN ?", filters.AccessibleTeamIDs)
		}
	}
	return q
}

// batchEpicNames looks up epic names by IDs.
func (r *PMTaskRepository) batchSprintNames(ctx context.Context, sprintIDs map[string]struct{}) map[string]string {
	result := map[string]string{}
	if len(sprintIDs) == 0 {
		return result
	}
	ids := make([]string, 0, len(sprintIDs))
	for id := range sprintIDs {
		ids = append(ids, id)
	}
	var rows []struct {
		ID   string
		Name string
	}
	if err := r.db.WithContext(ctx).Table("pm_sprints").Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result
}

func (r *PMTaskRepository) batchEpicNames(ctx context.Context, epicIDs map[string]struct{}) map[string]string {
	epicNameMap := map[string]string{}
	if len(epicIDs) == 0 {
		return epicNameMap
	}
	ids := make([]string, 0, len(epicIDs))
	for id := range epicIDs {
		ids = append(ids, id)
	}
	var rows []struct {
		ID   string
		Name string
	}
	if err := r.db.WithContext(ctx).Table("pm_epics").Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return epicNameMap
	}
	for _, row := range rows {
		epicNameMap[row.ID] = row.Name
	}
	return epicNameMap
}

func (r *PMTaskRepository) batchTaskOwnerMemberIDs(ctx context.Context, taskIDs []string) map[string][]string {
	result := map[string][]string{}
	if len(taskIDs) == 0 {
		return result
	}
	var rows []struct {
		TaskID   string `gorm:"column:task_id"`
		MemberID string `gorm:"column:member_id"`
	}
	if err := r.db.WithContext(ctx).
		Table("pm_task_owners po").
		Select("po.task_id, wm.id AS member_id").
		Joins("JOIN pm_tasks t ON t.id = po.task_id").
		Joins("JOIN workspace_members wm ON wm.user_id = po.user_id AND wm.workspace_id = t.workspace_id").
		Where("po.task_id IN ?", taskIDs).
		Order("po.task_id, po.created_at ASC").
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.TaskID] = append(result[row.TaskID], row.MemberID)
	}
	return result
}

// batchOwnerNames looks up user full names by IDs.
func (r *PMTaskRepository) batchOwnerNames(ctx context.Context, ownerIDs map[string]struct{}) map[string]string {
	ownerNameMap := map[string]string{}
	if len(ownerIDs) == 0 {
		return ownerNameMap
	}
	ids := make([]string, 0, len(ownerIDs))
	for id := range ownerIDs {
		ids = append(ids, id)
	}
	var rows []struct {
		ID       string
		FullName string `gorm:"column:full_name"`
	}
	if err := r.db.WithContext(ctx).Table("users").Select("id, full_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return ownerNameMap
	}
	for _, row := range rows {
		ownerNameMap[row.ID] = row.FullName
	}
	return ownerNameMap
}

// batchMemberNames looks up workspace member display names by IDs.
func (r *PMTaskRepository) batchMemberNames(ctx context.Context, memberIDs map[string]struct{}) map[string]string {
	memberNameMap := map[string]string{}
	if len(memberIDs) == 0 {
		return memberNameMap
	}
	ids := make([]string, 0, len(memberIDs))
	for id := range memberIDs {
		ids = append(ids, id)
	}
	var rows []struct {
		ID          string
		DisplayName string `gorm:"column:display_name"`
	}
	if err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select("wm.id, COALESCE(NULLIF(wm.display_name, ''), u.full_name, wm.email) AS display_name").
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("wm.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return memberNameMap
	}
	for _, row := range rows {
		memberNameMap[row.ID] = row.DisplayName
	}
	return memberNameMap
}

// batchTaskLabels loads labels for a set of task IDs, keyed by task ID.
func (r *PMTaskRepository) batchTaskLabels(ctx context.Context, taskIDs []string) map[string][]model.PMLabel {
	result := map[string][]model.PMLabel{}
	if len(taskIDs) == 0 {
		return result
	}
	var rows []struct {
		model.PMLabel
		TaskID string `gorm:"column:task_id"`
	}
	if err := r.db.WithContext(ctx).
		Table("pm_labels").
		Select("pm_labels.*, pm_task_labels.task_id").
		Joins("JOIN pm_task_labels ON pm_task_labels.label_id = pm_labels.id").
		Where("pm_task_labels.task_id IN ?", taskIDs).
		Order("pm_labels.name ASC").
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		label := row.PMLabel
		result[row.TaskID] = append(result[row.TaskID], label)
	}
	return result
}

// batchStateInfo batch-loads workflow state name, type, and color for a set of state IDs.
func (r *PMTaskRepository) batchStateInfo(ctx context.Context, stateIDs map[string]struct{}) map[string]stateInfo {
	result := map[string]stateInfo{}
	if len(stateIDs) == 0 {
		return result
	}
	ids := make([]string, 0, len(stateIDs))
	for id := range stateIDs {
		ids = append(ids, id)
	}
	var rows []struct {
		ID        string `gorm:"column:id"`
		Name      string `gorm:"column:name"`
		StateType string `gorm:"column:state_type"`
		Color     string `gorm:"column:color"`
	}
	if err := r.db.WithContext(ctx).
		Table("pm_workflow_states").
		Select("id, name, state_type, color").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.ID] = stateInfo{Name: row.Name, StateType: row.StateType, Color: row.Color}
	}
	return result
}

// ListByMember returns board columns grouped by owner member.
func (r *PMTaskRepository) ListByMember(ctx context.Context, workspaceID, workflowID string, filters model.PMTaskFilters, perMemberLimit int, includeEmpty bool, memberIDs []string) ([]model.TaskMemberColumn, error) {
	// Get all workflow state IDs for the selected workflow.
	var stateIDs []string
	if err := r.db.WithContext(ctx).
		Model(&model.PMWorkflowState{}).
		Where("workflow_id = ?", workflowID).
		Pluck("id", &stateIDs).Error; err != nil {
		return nil, fmt.Errorf("list workflow states: %w", err)
	}
	if len(stateIDs) == 0 {
		return []model.TaskMemberColumn{}, nil
	}

	// Aggregate counts per owning member. Multi-owner tasks intentionally
	// contribute once to each owner's column.
	type memberAggregate struct {
		MemberID   string `gorm:"column:member_id"`
		TaskCount  int    `gorm:"column:task_count"`
		PointTotal int    `gorm:"column:point_total"`
	}
	var aggregateRows []memberAggregate
	ownedAggregateQuery := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("pm_tasks.workspace_id = ? AND pm_tasks.workflow_state_id IN ?", workspaceID, stateIDs).
		Joins("JOIN pm_task_owners po ON po.task_id = pm_tasks.id").
		Joins("JOIN workspace_members wm ON wm.user_id = po.user_id AND wm.workspace_id = pm_tasks.workspace_id")
	ownedAggregateQuery = r.applyBoardFilters(ownedAggregateQuery, filters)
	if err := ownedAggregateQuery.
		Select("wm.id AS member_id, COUNT(DISTINCT pm_tasks.id) AS task_count, COALESCE(SUM(pm_tasks.estimate), 0) AS point_total").
		Group("wm.id").
		Scan(&aggregateRows).Error; err != nil {
		return nil, fmt.Errorf("list member board aggregates: %w", err)
	}

	aggregates := map[string]memberAggregate{} // key: member_id or "" for unassigned
	for _, row := range aggregateRows {
		aggregates[row.MemberID] = row
	}

	var unassignedAggregate memberAggregate
	unassignedAggregateQuery := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("pm_tasks.workspace_id = ? AND pm_tasks.workflow_state_id IN ?", workspaceID, stateIDs).
		Where("NOT EXISTS (SELECT 1 FROM pm_task_owners po WHERE po.task_id = pm_tasks.id)")
	unassignedAggregateQuery = r.applyBoardFilters(unassignedAggregateQuery, filters)
	if err := unassignedAggregateQuery.
		Select("'' AS member_id, COUNT(DISTINCT pm_tasks.id) AS task_count, COALESCE(SUM(pm_tasks.estimate), 0) AS point_total").
		Scan(&unassignedAggregate).Error; err != nil {
		return nil, fmt.Errorf("list unassigned member board aggregate: %w", err)
	}
	if unassignedAggregate.TaskCount > 0 {
		aggregates[""] = unassignedAggregate
	}

	// Collect all member keys that have tasks.
	memberKeys := make([]string, 0, len(aggregates))
	for key := range aggregates {
		memberKeys = append(memberKeys, key)
	}

	// Fetch visible tasks per member.
	var allTasks []model.PMTask
	type columnMeta struct {
		memberKey  string
		totalCount int
		pointTotal int
		hasMore    bool
		visible    []model.PMTask
	}
	metaMap := map[string]*columnMeta{}

	for _, key := range memberKeys {
		query := r.db.WithContext(ctx).
			Model(&model.PMTask{}).
			Where("pm_tasks.workspace_id = ? AND pm_tasks.workflow_state_id IN ?", workspaceID, stateIDs)
		query = r.applyBoardFilters(query, filters)
		if key == "" {
			query = query.Where("NOT EXISTS (SELECT 1 FROM pm_task_owners po WHERE po.task_id = pm_tasks.id)")
		} else {
			query = query.
				Joins("JOIN pm_task_owners po ON po.task_id = pm_tasks.id").
				Joins("JOIN workspace_members wm ON wm.user_id = po.user_id AND wm.workspace_id = pm_tasks.workspace_id").
				Where("wm.id = ?", key)
		}
		query = query.Joins("JOIN pm_workflow_states ws ON ws.id = pm_tasks.workflow_state_id").
			Order(memberBoardTaskOrderClause())
		if perMemberLimit > 0 {
			query = query.Limit(perMemberLimit)
		}

		var memberTasks []model.PMTask
		if err := taskSummaryQuery(query).Find(&memberTasks).Error; err != nil {
			return nil, fmt.Errorf("list member column tasks: %w", err)
		}
		allTasks = append(allTasks, memberTasks...)

		agg := aggregates[key]
		metaMap[key] = &columnMeta{
			memberKey:  key,
			totalCount: agg.TaskCount,
			pointTotal: agg.PointTotal,
			hasMore:    agg.TaskCount > len(memberTasks),
			visible:    memberTasks,
		}
	}

	// Enrich all tasks at once.
	enriched := r.collectAndEnrich(ctx, allTasks, taskEnrichOptions{
		includeContacts:  filters.IncludeContacts,
		includeCompanies: filters.IncludeCompanies,
		includeDeals:     filters.IncludeDeals,
		includeSupport:   filters.IncludeSupport,
	})
	enrichedMap := make(map[string]model.BoardTask, len(enriched))
	for _, bs := range enriched {
		enrichedMap[bs.ID] = bs
	}

	// Collect all unique member IDs that need member info.
	memberInfoIDs := map[string]struct{}{}
	for _, key := range memberKeys {
		if key != "" {
			memberInfoIDs[key] = struct{}{}
		}
	}
	if includeEmpty {
		for _, id := range memberIDs {
			memberInfoIDs[id] = struct{}{}
		}
	}

	// Batch-load AssignableMember info.
	memberInfoMap := map[string]model.AssignableMember{}
	if len(memberInfoIDs) > 0 {
		ids := make([]string, 0, len(memberInfoIDs))
		for id := range memberInfoIDs {
			ids = append(ids, id)
		}
		var members []model.AssignableMember
		if err := r.db.WithContext(ctx).
			Table("workspace_members wm").
			Select("wm.id, wm.user_id, wm.role, wm.email, COALESCE(NULLIF(wm.display_name, ''), u.full_name, wm.email) AS display_name, u.avatar_url, u.avatar_style, u.avatar_seed, u.avatar_background_mode, u.avatar_background_color, wm.status").
			Joins("LEFT JOIN users u ON u.id = wm.user_id").
			Where("wm.id IN ?", ids).
			Scan(&members).Error; err != nil {
			return nil, fmt.Errorf("load member info: %w", err)
		}
		for _, m := range members {
			memberInfoMap[m.ID] = m
		}
	}

	// Build columns: unassigned first, then members sorted by display_name.
	var columns []model.TaskMemberColumn

	// Unassigned column.
	if meta, ok := metaMap[""]; ok {
		colTasks := make([]model.BoardTask, 0, len(meta.visible))
		for _, s := range meta.visible {
			colTasks = append(colTasks, enrichedMap[s.ID])
		}
		columns = append(columns, model.TaskMemberColumn{
			Member:     nil,
			Tasks:      colTasks,
			TaskCount:  meta.totalCount,
			PointTotal: meta.pointTotal,
			HasMore:    meta.hasMore,
		})
	} else if includeEmpty {
		columns = append(columns, model.TaskMemberColumn{
			Member:     nil,
			Tasks:      []model.BoardTask{},
			TaskCount:  0,
			PointTotal: 0,
			HasMore:    false,
		})
	}

	// Assigned member columns sorted by display_name.
	type memberEntry struct {
		memberID string
		name     string
	}
	var memberEntries []memberEntry
	seen := map[string]bool{}
	for _, key := range memberKeys {
		if key == "" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		name := key
		if m, ok := memberInfoMap[key]; ok {
			name = m.DisplayName
		}
		memberEntries = append(memberEntries, memberEntry{memberID: key, name: name})
	}

	// Add empty columns for members without tasks.
	if includeEmpty {
		for _, id := range memberIDs {
			if !seen[id] {
				seen[id] = true
				name := id
				if m, ok := memberInfoMap[id]; ok {
					name = m.DisplayName
				}
				memberEntries = append(memberEntries, memberEntry{memberID: id, name: name})
			}
		}
	}

	sort.Slice(memberEntries, func(i, j int) bool {
		return strings.ToLower(memberEntries[i].name) < strings.ToLower(memberEntries[j].name)
	})

	for _, entry := range memberEntries {
		meta := metaMap[entry.memberID]
		var colTasks []model.BoardTask
		totalCount := 0
		pointTotal := 0
		hasMore := false
		if meta != nil {
			colTasks = make([]model.BoardTask, 0, len(meta.visible))
			for _, s := range meta.visible {
				colTasks = append(colTasks, enrichedMap[s.ID])
			}
			totalCount = meta.totalCount
			pointTotal = meta.pointTotal
			hasMore = meta.hasMore
		} else {
			colTasks = []model.BoardTask{}
		}

		m := memberInfoMap[entry.memberID]
		columns = append(columns, model.TaskMemberColumn{
			Member:     &m,
			Tasks:      colTasks,
			TaskCount:  totalCount,
			PointTotal: pointTotal,
			HasMore:    hasMore,
		})
	}

	return columns, nil
}

// ListMemberColumnTasks returns a page of tasks for a single member column, enriched for board display.
func (r *PMTaskRepository) ListMemberColumnTasks(ctx context.Context, workspaceID, workflowID string, memberID *string, filters model.PMTaskFilters, offset, limit int) ([]model.BoardTask, int, error) {
	var stateIDs []string
	if err := r.db.WithContext(ctx).
		Model(&model.PMWorkflowState{}).
		Where("workflow_id = ?", workflowID).
		Pluck("id", &stateIDs).Error; err != nil {
		return nil, 0, fmt.Errorf("list workflow states: %w", err)
	}
	if len(stateIDs) == 0 {
		return []model.BoardTask{}, 0, nil
	}

	taskQuery := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("pm_tasks.workspace_id = ? AND pm_tasks.workflow_state_id IN ?", workspaceID, stateIDs)
	taskQuery = r.applyBoardFilters(taskQuery, filters)

	if memberID == nil {
		taskQuery = taskQuery.Where("NOT EXISTS (SELECT 1 FROM pm_task_owners po WHERE po.task_id = pm_tasks.id)")
	} else {
		taskQuery = taskQuery.
			Joins("JOIN pm_task_owners po ON po.task_id = pm_tasks.id").
			Joins("JOIN workspace_members wm ON wm.user_id = po.user_id AND wm.workspace_id = pm_tasks.workspace_id").
			Where("wm.id = ?", *memberID)
	}

	var total int64
	if err := taskQuery.Distinct("pm_tasks.id").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count member column tasks: %w", err)
	}

	var tasks []model.PMTask
	if err := taskSummaryQuery(taskQuery).
		Joins("JOIN pm_workflow_states ws ON ws.id = pm_tasks.workflow_state_id").
		Order(memberBoardTaskOrderClause()).
		Offset(offset).Limit(limit).
		Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("list member column tasks: %w", err)
	}

	enriched := r.collectAndEnrich(ctx, tasks, taskEnrichOptions{
		includeContacts:  filters.IncludeContacts,
		includeCompanies: filters.IncludeCompanies,
		includeDeals:     filters.IncludeDeals,
		includeSupport:   filters.IncludeSupport,
	})
	return enriched, int(total), nil
}

// CountByState returns task counts grouped by state for a workflow.
func (r *PMTaskRepository) CountByState(ctx context.Context, workflowID string) ([]model.TaskStateCount, error) {
	var counts []model.TaskStateCount
	if err := r.db.WithContext(ctx).
		Table("pm_workflow_states ws").
		Select("ws.id AS state_id, ws.name AS state_name, ws.state_type AS state_type, COUNT(s.id) AS task_count").
		Joins("LEFT JOIN pm_tasks s ON s.workflow_state_id = ws.id AND s.archived = false").
		Where("ws.workflow_id = ?", workflowID).
		Group("ws.id, ws.name, ws.state_type, ws.position").
		Order("CASE ws.state_type WHEN 'backlog' THEN 0 WHEN 'unstarted' THEN 1 WHEN 'started' THEN 2 WHEN 'done' THEN 3 ELSE 4 END, ws.position ASC").
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("count tasks by state: %w", err)
	}
	return counts, nil
}

// CountByWorkflowState returns number of active tasks in a state.
func (r *PMTaskRepository) CountByWorkflowState(ctx context.Context, stateID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("workflow_state_id = ? AND archived = false", stateID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count tasks in state: %w", err)
	}
	return count, nil
}

// UpdateStartedCompleted computes and updates started/completed fields from state type.
func (r *PMTaskRepository) UpdateStartedCompleted(ctx context.Context, taskID string) error {
	var row struct {
		StateType string
	}
	if err := r.db.WithContext(ctx).
		Table("pm_tasks s").
		Select("ws.state_type").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.id = ?", taskID).
		Scan(&row).Error; err != nil {
		return fmt.Errorf("load task state type: %w", err)
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{}
	switch row.StateType {
	case model.PMStateTypeDone:
		updates["started"] = true
		updates["completed"] = true
		updates["moved_at"] = now
		updates["completed_at"] = gorm.Expr("COALESCE(completed_at, ?)", now)
		updates["started_at"] = gorm.Expr("COALESCE(started_at, ?)", now)
	case model.PMStateTypeStarted:
		updates["started"] = true
		updates["completed"] = false
		updates["moved_at"] = now
		updates["completed_at"] = nil
		updates["started_at"] = gorm.Expr("COALESCE(started_at, ?)", now)
	default:
		updates["started"] = false
		updates["completed"] = false
		updates["moved_at"] = now
		updates["started_at"] = nil
		updates["completed_at"] = nil
	}

	if err := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("id = ?", taskID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update task started/completed: %w", err)
	}
	return nil
}

// UpdateSprintID updates only the sprint_id field of a task.
func (r *PMTaskRepository) UpdateSprintID(ctx context.Context, taskID string, sprintID *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMTask{}).
		Where("id = ?", taskID).
		Update("sprint_id", sprintID).Error; err != nil {
		return fmt.Errorf("update task sprint_id: %w", err)
	}
	return nil
}

func (r *PMTaskRepository) buildTaskDetail(ctx context.Context, task model.PMTask) (*model.TaskDetail, error) {
	task = r.applyDependencySummaries(ctx, []model.PMTask{task})[0]
	task = r.applyLatestRunMetadata(ctx, []model.PMTask{task})[0]

	var owners []model.User
	if err := r.db.WithContext(ctx).
		Table("users u").
		Select("u.*").
		Joins("JOIN pm_task_owners so ON so.user_id = u.id").
		Where("so.task_id = ?", task.ID).
		Order("u.full_name ASC").
		Find(&owners).Error; err != nil {
		return nil, fmt.Errorf("list task owners: %w", err)
	}

	var followers []model.User
	if err := r.db.WithContext(ctx).
		Table("users u").
		Select("u.*").
		Joins("JOIN pm_task_followers sf ON sf.user_id = u.id").
		Where("sf.task_id = ?", task.ID).
		Order("u.full_name ASC").
		Find(&followers).Error; err != nil {
		return nil, fmt.Errorf("list task followers: %w", err)
	}

	task.OwnerMemberIDs = r.batchTaskOwnerMemberIDs(ctx, []string{task.ID})[task.ID]

	requesterMember, err := r.loadAssignableMember(ctx, task.WorkspaceID, task.RequesterMemberID)
	if err != nil {
		return nil, err
	}

	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("l.*").
		Joins("JOIN pm_task_labels sl ON sl.label_id = l.id").
		Where("sl.task_id = ?", task.ID).
		Order("l.name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list task labels: %w", err)
	}

	var state model.PMWorkflowState
	_ = r.db.WithContext(ctx).Where("id = ?", task.WorkflowStateID).First(&state).Error

	var epicName *string
	if task.EpicID != nil {
		var value string
		if err := r.db.WithContext(ctx).Table("pm_epics").Select("name").Where("id = ?", *task.EpicID).Scan(&value).Error; err == nil && value != "" {
			epicName = &value
		}
	}

	var sprintName *string
	if task.SprintID != nil {
		var value string
		if err := r.db.WithContext(ctx).Table("pm_sprints").Select("name").Where("id = ?", *task.SprintID).Scan(&value).Error; err == nil && value != "" {
			sprintName = &value
		}
	}

	var objectiveName *string
	var objectiveID *string
	if task.EpicID != nil {
		var obj struct {
			ID   string
			Name string
		}
		if err := r.db.WithContext(ctx).
			Table("pm_objectives o").
			Select("o.id, o.name").
			Joins("JOIN pm_epic_objectives eo ON eo.objective_id = o.id").
			Where("eo.epic_id = ?", *task.EpicID).
			Limit(1).
			Scan(&obj).Error; err == nil && obj.ID != "" {
			objectiveName = &obj.Name
			objectiveID = &obj.ID
		}
	}

	return &model.TaskDetail{
		Task:            task,
		Owners:          owners,
		Followers:       followers,
		RequesterMember: requesterMember,
		Labels:          labels,
		EpicName:        epicName,
		SprintName:      sprintName,
		ObjectiveName:   objectiveName,
		ObjectiveID:     objectiveID,
		State:           &state,
	}, nil
}

type dependencySummary struct {
	blockedByTasks   []model.TaskDependencyTask
	blockingTasks    []model.TaskDependencyTask
	blockedByCount   int
	blockingCount    int
	hasInboundBlocks bool
	isBlockedByTask  bool
	isBlockingOther  bool
}

func (r *PMTaskRepository) applyDependencySummaries(ctx context.Context, tasks []model.PMTask) []model.PMTask {
	if len(tasks) == 0 {
		return tasks
	}
	summaries := r.loadDependencySummaries(ctx, tasks[0].WorkspaceID, tasks)
	result := make([]model.PMTask, 0, len(tasks))
	for _, task := range tasks {
		summary := summaries[task.ID]
		task.BlockedByTasks = summary.blockedByTasks
		task.BlockingTasks = summary.blockingTasks
		task.BlockedByCount = summary.blockedByCount
		task.BlockingCount = summary.blockingCount
		task.IsBlockedByTask = summary.isBlockedByTask
		task.IsBlockingOtherTask = summary.isBlockingOther
		task.Blocked = isTaskBlocked(task, summary)
		result = append(result, task)
	}
	return result
}

func (r *PMTaskRepository) loadDependencySummaries(ctx context.Context, workspaceID string, tasks []model.PMTask) map[string]dependencySummary {
	result := make(map[string]dependencySummary, len(tasks))
	if workspaceID == "" || len(tasks) == 0 {
		return result
	}

	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
		result[task.ID] = dependencySummary{}
	}

	var links []model.PMTaskLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND link_type = ? AND (source_task_id IN ? OR target_task_id IN ?)",
			workspaceID, model.PMTaskLinkTypeBlocks, taskIDs, taskIDs).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return result
	}
	if len(links) == 0 {
		return result
	}

	relatedIDs := make(map[string]struct{}, len(links)*2)
	for _, link := range links {
		relatedIDs[link.SourceTaskID] = struct{}{}
		relatedIDs[link.TargetTaskID] = struct{}{}
	}
	ids := make([]string, 0, len(relatedIDs))
	for id := range relatedIDs {
		ids = append(ids, id)
	}

	var relatedTasks []model.PMTask
	if err := r.db.WithContext(ctx).
		Select("id, display_id, name, workflow_state_id, completed").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&relatedTasks).Error; err != nil {
		return result
	}

	relatedMap := make(map[string]model.TaskDependencyTask, len(relatedTasks))
	for _, task := range relatedTasks {
		relatedMap[task.ID] = model.TaskDependencyTask{
			ID:              task.ID,
			DisplayID:       task.DisplayID,
			Name:            task.Name,
			WorkflowStateID: task.WorkflowStateID,
			Completed:       task.Completed,
		}
	}

	for _, link := range links {
		if _, ok := result[link.TargetTaskID]; ok {
			summary := result[link.TargetTaskID]
			source, exists := relatedMap[link.SourceTaskID]
			if exists {
				summary.blockedByTasks = append(summary.blockedByTasks, source)
				summary.hasInboundBlocks = true
				if !source.Completed {
					summary.blockedByCount++
					summary.isBlockedByTask = true
				}
			}
			result[link.TargetTaskID] = summary
		}
		if _, ok := result[link.SourceTaskID]; ok {
			summary := result[link.SourceTaskID]
			target, exists := relatedMap[link.TargetTaskID]
			if exists {
				summary.blockingTasks = append(summary.blockingTasks, target)
				summary.blockingCount++
				summary.isBlockingOther = true
			}
			result[link.SourceTaskID] = summary
		}
	}

	for taskID, summary := range result {
		sort.SliceStable(summary.blockedByTasks, func(i, j int) bool {
			if summary.blockedByTasks[i].Completed != summary.blockedByTasks[j].Completed {
				return !summary.blockedByTasks[i].Completed
			}
			return summary.blockedByTasks[i].DisplayID < summary.blockedByTasks[j].DisplayID
		})
		sort.SliceStable(summary.blockingTasks, func(i, j int) bool {
			if summary.blockingTasks[i].Completed != summary.blockingTasks[j].Completed {
				return !summary.blockingTasks[i].Completed
			}
			return summary.blockingTasks[i].DisplayID < summary.blockingTasks[j].DisplayID
		})
		result[taskID] = summary
	}

	return result
}

func isTaskBlocked(task model.PMTask, summary dependencySummary) bool {
	if strings.TrimSpace(stringPtrValue(task.Blocker)) != "" {
		return true
	}
	if summary.blockedByCount > 0 {
		return true
	}
	return task.Blocked && !summary.hasInboundBlocks
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *PMTaskRepository) applyDerivedBlockedFilter(q *gorm.DB, blocked bool) *gorm.DB {
	expr := `
(
	COALESCE(NULLIF(TRIM(pm_tasks.blocker), ''), '') <> ''
	OR EXISTS (
		SELECT 1
		FROM pm_task_links sl
		JOIN pm_tasks blockers ON blockers.id = sl.source_task_id
		WHERE sl.workspace_id = pm_tasks.workspace_id
		  AND sl.target_task_id = pm_tasks.id
		  AND sl.link_type = ?
		  AND blockers.completed = FALSE
	)
	OR (
		pm_tasks.blocked = TRUE
		AND NOT EXISTS (
			SELECT 1
			FROM pm_task_links legacy_sl
			WHERE legacy_sl.workspace_id = pm_tasks.workspace_id
			  AND legacy_sl.target_task_id = pm_tasks.id
			  AND legacy_sl.link_type = ?
		)
	)
)`
	if blocked {
		return q.Where(expr, model.PMTaskLinkTypeBlocks, model.PMTaskLinkTypeBlocks)
	}
	return q.Where("NOT "+expr, model.PMTaskLinkTypeBlocks, model.PMTaskLinkTypeBlocks)
}

func (r *PMTaskRepository) applyBlockingFilter(q *gorm.DB, blocking bool) *gorm.DB {
	expr := `
EXISTS (
	SELECT 1
	FROM pm_task_links sl
	WHERE sl.workspace_id = pm_tasks.workspace_id
	  AND sl.source_task_id = pm_tasks.id
	  AND sl.link_type = ?
)`
	if blocking {
		return q.Where(expr, model.PMTaskLinkTypeBlocks)
	}
	return q.Where("NOT "+expr, model.PMTaskLinkTypeBlocks)
}

func (r *PMTaskRepository) loadAssignableMember(ctx context.Context, workspaceID string, memberID *string) (*model.AssignableMember, error) {
	if memberID == nil || *memberID == "" {
		return nil, nil
	}

	var member model.AssignableMember
	if err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select(`
			wm.id,
			wm.user_id,
			wm.role,
			wm.email,
			wm.display_name,
			u.avatar_url,
			u.avatar_style,
			u.avatar_seed,
			u.avatar_background_mode,
			u.avatar_background_color,
			wm.status,
			wm.invited_by,
			wm.invited_at,
			wm.accepted_at
		`).
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND wm.id = ?", workspaceID, *memberID).
		Scan(&member).Error; err != nil {
		return nil, fmt.Errorf("load task member: %w", err)
	}
	if member.ID == "" {
		return nil, nil
	}
	return &member, nil
}

// MigrateTasksToWorkflow remaps tasks belonging to a team (or with NULL
// team_id) from old workflow states to new workflow states using the provided
// state mapping. It also sets team_id on migrated tasks.
func (r *PMTaskRepository) MigrateTasksToWorkflow(ctx context.Context, teamID, oldWorkflowID, newWorkflowID string, stateMap map[string]string) (int64, error) {
	var total int64
	for oldStateID, newStateID := range stateMap {
		result := r.db.WithContext(ctx).
			Model(&model.PMTask{}).
			Where("(team_id = ? OR team_id IS NULL) AND workflow_id = ? AND workflow_state_id = ? AND archived = false", teamID, oldWorkflowID, oldStateID).
			Updates(map[string]interface{}{
				"workflow_id":       newWorkflowID,
				"workflow_state_id": newStateID,
				"team_id":           teamID,
			})
		if result.Error != nil {
			return total, fmt.Errorf("migrate tasks from state %s to %s: %w", oldStateID, newStateID, result.Error)
		}
		total += result.RowsAffected
	}
	return total, nil
}
