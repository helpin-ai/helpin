package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMStoryRepository handles DB operations for stories and relations.
type PMStoryRepository struct {
	db *gorm.DB
}

const boardDoneGroupThisWeekLabel = "This Week"

// NewPMStoryRepository creates a new PMStoryRepository.
func NewPMStoryRepository(db *gorm.DB) *PMStoryRepository {
	return &PMStoryRepository{db: db}
}

func boardStoryOrderClause(stateType string) string {
	if stateType == "done" {
		return "COALESCE(completed_at, moved_at, updated_at) DESC, updated_at DESC, position ASC"
	}
	return "position ASC, updated_at DESC"
}

func boardStoryGroupDate(story model.PMStory) time.Time {
	if story.CompletedAt != nil {
		return story.CompletedAt.UTC()
	}
	if story.MovedAt != nil {
		return story.MovedAt.UTC()
	}
	return story.UpdatedAt.UTC()
}

func startOfBoardWeek(t time.Time) time.Time {
	utc := t.UTC()
	offset := (int(utc.Weekday()) + 6) % 7
	return time.Date(utc.Year(), utc.Month(), utc.Day()-offset, 0, 0, 0, 0, time.UTC)
}

func buildDoneStoryGroups(stories []model.BoardStory, now time.Time) []model.StoryGroup {
	if len(stories) == 0 {
		return nil
	}

	currentWeekStart := startOfBoardWeek(now)
	groups := make([]model.StoryGroup, 0, len(stories))
	groupIndexByKey := make(map[string]int, len(stories))

	for _, story := range stories {
		weekStart := startOfBoardWeek(boardStoryGroupDate(story.PMStory))
		key := weekStart.Format("2006-01-02")
		label := "Week of " + weekStart.Format("Jan 2, 2006")
		if weekStart.Equal(currentWeekStart) {
			label = boardDoneGroupThisWeekLabel
		}

		index, ok := groupIndexByKey[key]
		if !ok {
			index = len(groups)
			groupIndexByKey[key] = index
			groups = append(groups, model.StoryGroup{
				Key:     key,
				Label:   label,
				Stories: []model.BoardStory{},
			})
		}

		groups[index].Stories = append(groups[index].Stories, story)
	}

	return groups
}

// List returns stories with filters and pagination.
func (r *PMStoryRepository) List(ctx context.Context, workspaceID string, filters model.PMStoryFilters, pagination model.PMPagination) ([]model.BoardStory, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.PMStory{}).Where("workspace_id = ?", workspaceID)

	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.EpicID != nil && *filters.EpicID != "" {
		query = query.Where("epic_id = ?", *filters.EpicID)
	}
	if filters.SprintID != nil && *filters.SprintID != "" {
		query = query.Where("sprint_id = ?", *filters.SprintID)
	}
	if filters.WorkflowID != nil && *filters.WorkflowID != "" {
		query = query.Where("workflow_id = ?", *filters.WorkflowID)
	}
	if filters.WorkflowStateID != nil && *filters.WorkflowStateID != "" {
		query = query.Where("workflow_state_id = ?", *filters.WorkflowStateID)
	}
	if filters.StoryType != nil && *filters.StoryType != "" {
		query = query.Where("story_type = ?", *filters.StoryType)
	}
	if filters.OwnerID != nil && *filters.OwnerID != "" {
		query = query.Where("owner_id = ?", *filters.OwnerID)
	}
	if filters.OwnerMemberID != nil && *filters.OwnerMemberID != "" {
		query = query.Where("owner_member_id = ?", *filters.OwnerMemberID)
	}
	if filters.Priority != nil && *filters.Priority != "" {
		query = query.Where("priority = ?", *filters.Priority)
	}
	if filters.RequesterID != nil && *filters.RequesterID != "" {
		query = query.Where("requester_id = ?", *filters.RequesterID)
	}
	if filters.RequesterMemberID != nil && *filters.RequesterMemberID != "" {
		query = query.Where("requester_member_id = ?", *filters.RequesterMemberID)
	}
	if filters.Severity != nil && *filters.Severity != "" {
		query = query.Where("severity = ?", *filters.Severity)
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
	if filters.LabelID != nil && *filters.LabelID != "" {
		query = query.Joins("JOIN pm_story_labels psl ON psl.story_id = pm_stories.id").Where("psl.label_id = ?", *filters.LabelID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count stories: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}

	var stories []model.PMStory
	if err := query.Order("updated_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&stories).Error; err != nil {
		return nil, 0, fmt.Errorf("list stories: %w", err)
	}

	return r.collectAndEnrich(ctx, stories), total, nil
}

// GetByID returns a story detail payload.
func (r *PMStoryRepository) GetByID(ctx context.Context, id string) (*model.StoryDetail, error) {
	var story model.PMStory
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&story).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story: %w", err)
	}
	return r.buildStoryDetail(ctx, story)
}

// GetByDisplayID returns a story detail by display ID.
func (r *PMStoryRepository) GetByDisplayID(ctx context.Context, workspaceID string, displayID int) (*model.StoryDetail, error) {
	var story model.PMStory
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND display_id = ?", workspaceID, displayID).
		First(&story).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story by display id: %w", err)
	}
	return r.buildStoryDetail(ctx, story)
}

// GetRawByID returns a raw story model by ID.
func (r *PMStoryRepository) GetRawByID(ctx context.Context, id string) (*model.PMStory, error) {
	var story model.PMStory
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&story).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get story raw: %w", err)
	}
	return &story, nil
}

// ListByIDs returns raw stories by ID for a workspace.
func (r *PMStoryRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.PMStory, error) {
	if len(ids) == 0 {
		return []model.PMStory{}, nil
	}

	var stories []model.PMStory
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&stories).Error; err != nil {
		return nil, fmt.Errorf("list stories by ids: %w", err)
	}
	return stories, nil
}

// Create inserts a story and auto-populates display_id per workspace.
func (r *PMStoryRepository) Create(ctx context.Context, story *model.PMStory) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if story.DisplayID == 0 {
			var maxDisplayID int
			if err := tx.Model(&model.PMStory{}).
				Where("workspace_id = ?", story.WorkspaceID).
				Select("COALESCE(MAX(display_id), 0)").
				Scan(&maxDisplayID).Error; err != nil {
				return fmt.Errorf("allocate display id: %w", err)
			}
			story.DisplayID = maxDisplayID + 1
		}

		if err := tx.Create(story).Error; err != nil {
			return fmt.Errorf("create story: %w", err)
		}
		return nil
	})
}

// Update updates a story model.
func (r *PMStoryRepository) Update(ctx context.Context, story *model.PMStory) error {
	if err := r.db.WithContext(ctx).Save(story).Error; err != nil {
		return fmt.Errorf("update story: %w", err)
	}
	return nil
}

// Delete archives a story.
func (r *PMStoryRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Where("id = ?", id).
		Update("archived", true).Error; err != nil {
		return fmt.Errorf("archive story: %w", err)
	}
	return nil
}

// MoveToState moves a story to a new state and position.
func (r *PMStoryRepository) MoveToState(ctx context.Context, storyID, stateID string, position int) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"workflow_state_id": stateID,
		"position":          position,
		"moved_at":          now,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Where("id = ?", storyID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("move story: %w", err)
	}
	return nil
}

// Reorder updates story position in current column.
func (r *PMStoryRepository) Reorder(ctx context.Context, storyID string, position int) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Where("id = ?", storyID).
		Update("position", position).Error; err != nil {
		return fmt.Errorf("reorder story: %w", err)
	}
	return nil
}

// AddOwner links an owner to a story.
func (r *PMStoryRepository) AddOwner(ctx context.Context, storyID, userID string) error {
	owner := model.PMStoryOwner{StoryID: storyID, UserID: userID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&owner).Error; err != nil {
		return fmt.Errorf("add story owner: %w", err)
	}
	return nil
}

// RemoveOwner unlinks an owner from a story.
func (r *PMStoryRepository) RemoveOwner(ctx context.Context, storyID, userID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMStoryOwner{}, "story_id = ? AND user_id = ?", storyID, userID).Error; err != nil {
		return fmt.Errorf("remove story owner: %w", err)
	}
	return nil
}

// AddFollower links a follower to a story.
func (r *PMStoryRepository) AddFollower(ctx context.Context, storyID, userID string) error {
	follower := model.PMStoryFollower{StoryID: storyID, UserID: userID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&follower).Error; err != nil {
		return fmt.Errorf("add story follower: %w", err)
	}
	return nil
}

// RemoveFollower unlinks a follower from a story.
func (r *PMStoryRepository) RemoveFollower(ctx context.Context, storyID, userID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMStoryFollower{}, "story_id = ? AND user_id = ?", storyID, userID).Error; err != nil {
		return fmt.Errorf("remove story follower: %w", err)
	}
	return nil
}

// AddLabel links a label to a story.
func (r *PMStoryRepository) AddLabel(ctx context.Context, storyID, labelID string) error {
	link := model.PMStoryLabel{StoryID: storyID, LabelID: labelID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&link).Error; err != nil {
		return fmt.Errorf("add story label: %w", err)
	}
	return nil
}

// RemoveLabel unlinks a label from a story.
func (r *PMStoryRepository) RemoveLabel(ctx context.Context, storyID, labelID string) error {
	if err := r.db.WithContext(ctx).
		Delete(&model.PMStoryLabel{}, "story_id = ? AND label_id = ?", storyID, labelID).Error; err != nil {
		return fmt.Errorf("remove story label: %w", err)
	}
	return nil
}

// ReplaceOwners replaces all story owners.
func (r *PMStoryRepository) ReplaceOwners(ctx context.Context, storyID string, userIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMStoryOwner{}, "story_id = ?", storyID).Error; err != nil {
			return fmt.Errorf("clear story owners: %w", err)
		}
		for _, userID := range userIDs {
			if err := tx.Create(&model.PMStoryOwner{StoryID: storyID, UserID: userID}).Error; err != nil {
				return fmt.Errorf("replace story owners: %w", err)
			}
		}
		return nil
	})
}

// ReplaceFollowers replaces all story followers.
func (r *PMStoryRepository) ReplaceFollowers(ctx context.Context, storyID string, userIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMStoryFollower{}, "story_id = ?", storyID).Error; err != nil {
			return fmt.Errorf("clear story followers: %w", err)
		}
		for _, userID := range userIDs {
			if err := tx.Create(&model.PMStoryFollower{StoryID: storyID, UserID: userID}).Error; err != nil {
				return fmt.Errorf("replace story followers: %w", err)
			}
		}
		return nil
	})
}

// ReplaceLabels replaces all story labels.
func (r *PMStoryRepository) ReplaceLabels(ctx context.Context, storyID string, labelIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.PMStoryLabel{}, "story_id = ?", storyID).Error; err != nil {
			return fmt.Errorf("clear story labels: %w", err)
		}
		for _, labelID := range labelIDs {
			if err := tx.Create(&model.PMStoryLabel{StoryID: storyID, LabelID: labelID}).Error; err != nil {
				return fmt.Errorf("replace story labels: %w", err)
			}
		}
		return nil
	})
}

// ListByWorkflowState returns board columns grouped by workflow state with optional filters.
// perStateLimit controls how many stories are returned per column (0 = unlimited).
func (r *PMStoryRepository) ListByWorkflowState(ctx context.Context, workflowID string, filters model.PMStoryFilters, perStateLimit int) ([]model.StoryStateColumn, error) {
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
		Model(&model.PMStory{}).
		Where("workflow_state_id IN ? AND archived = false", stateIDs)
	baseQuery = r.applyBoardFilters(baseQuery, filters)

	var aggregateRows []struct {
		StateID    string `gorm:"column:state_id"`
		StoryCount int    `gorm:"column:story_count"`
		PointTotal int    `gorm:"column:point_total"`
	}
	if err := baseQuery.
		Select("pm_stories.workflow_state_id AS state_id, COUNT(pm_stories.id) AS story_count, COALESCE(SUM(pm_stories.estimate), 0) AS point_total").
		Group("pm_stories.workflow_state_id").
		Scan(&aggregateRows).Error; err != nil {
		return nil, fmt.Errorf("list board aggregates: %w", err)
	}

	type aggregateMeta struct {
		storyCount int
		pointTotal int
	}
	aggregates := map[string]aggregateMeta{}
	for _, row := range aggregateRows {
		aggregates[row.StateID] = aggregateMeta{storyCount: row.StoryCount, pointTotal: row.PointTotal}
	}

	type columnMeta struct {
		totalCount int
		pointTotal int
		hasMore    bool
		visible    []model.PMStory
	}
	metas := make([]columnMeta, len(states))
	var allStories []model.PMStory
	for i, state := range states {
		query := r.db.WithContext(ctx).
			Where("workflow_state_id = ? AND archived = false", state.ID)
		query = r.applyBoardFilters(query, filters)
		query = query.Order(boardStoryOrderClause(state.StateType))
		if perStateLimit > 0 {
			query = query.Limit(perStateLimit)
		}

		var visibleStories []model.PMStory
		if err := query.Find(&visibleStories).Error; err != nil {
			return nil, fmt.Errorf("list board column stories: %w", err)
		}

		allStories = append(allStories, visibleStories...)

		aggregate := aggregates[state.ID]
		metas[i] = columnMeta{
			totalCount: aggregate.storyCount,
			pointTotal: aggregate.pointTotal,
			hasMore:    aggregate.storyCount > len(visibleStories),
			visible:    visibleStories,
		}
	}

	enriched := r.collectAndEnrich(ctx, allStories)
	// Build a map from story ID → BoardStory for column assembly
	enrichedMap := make(map[string]model.BoardStory, len(enriched))
	for _, bs := range enriched {
		enrichedMap[bs.ID] = bs
	}

	columns := make([]model.StoryStateColumn, 0, len(states))
	for i, state := range states {
		m := metas[i]
		colStories := make([]model.BoardStory, 0, len(m.visible))
		for _, s := range m.visible {
			colStories = append(colStories, enrichedMap[s.ID])
		}
		var storyGroups []model.StoryGroup
		if state.StateType == "done" {
			storyGroups = buildDoneStoryGroups(colStories, time.Now().UTC())
		}
		columns = append(columns, model.StoryStateColumn{
			State:       state,
			Stories:     colStories,
			StoryGroups: storyGroups,
			StoryCount:  m.totalCount,
			PointTotal:  m.pointTotal,
			HasMore:     m.hasMore,
		})
	}
	return columns, nil
}

// ListColumnStories returns a page of stories for a single workflow state, enriched for board display.
func (r *PMStoryRepository) ListColumnStories(ctx context.Context, stateID string, filters model.PMStoryFilters, offset, limit int) ([]model.BoardStory, []model.StoryGroup, int, error) {
	var state model.PMWorkflowState
	if err := r.db.WithContext(ctx).Where("id = ?", stateID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, fmt.Errorf("workflow state not found")
		}
		return nil, nil, 0, fmt.Errorf("get workflow state: %w", err)
	}

	storyQuery := r.db.WithContext(ctx).
		Where("workflow_state_id = ? AND archived = false", stateID)
	storyQuery = r.applyBoardFilters(storyQuery, filters)

	var total int64
	if err := storyQuery.Model(&model.PMStory{}).Count(&total).Error; err != nil {
		return nil, nil, 0, fmt.Errorf("count column stories: %w", err)
	}

	var stories []model.PMStory
	if err := storyQuery.
		Order(boardStoryOrderClause(state.StateType)).
		Offset(offset).Limit(limit).
		Find(&stories).Error; err != nil {
		return nil, nil, 0, fmt.Errorf("list column stories: %w", err)
	}

	enriched := r.collectAndEnrich(ctx, stories)
	var storyGroups []model.StoryGroup
	if state.StateType == "done" {
		storyGroups = buildDoneStoryGroups(enriched, time.Now().UTC())
	}
	return enriched, storyGroups, int(total), nil
}

// collectAndEnrich collects related IDs from stories, batch-loads names/labels, and returns enriched BoardStory slices.
func (r *PMStoryRepository) collectAndEnrich(ctx context.Context, stories []model.PMStory) []model.BoardStory {
	stories = r.applyDependencySummaries(ctx, stories)
	epicIDs := map[string]struct{}{}
	ownerMemberIDs := map[string]struct{}{}
	ownerIDs := map[string]struct{}{}
	storyIDs := make([]string, len(stories))
	for i, s := range stories {
		storyIDs[i] = s.ID
		if s.EpicID != nil {
			epicIDs[*s.EpicID] = struct{}{}
		}
		if s.OwnerMemberID != nil {
			ownerMemberIDs[*s.OwnerMemberID] = struct{}{}
		}
		if s.OwnerID != nil {
			ownerIDs[*s.OwnerID] = struct{}{}
		}
	}
	epicNameMap := r.batchEpicNames(ctx, epicIDs)
	ownerNameMap := r.batchMemberNames(ctx, ownerMemberIDs)
	legacyOwnerNameMap := r.batchOwnerNames(ctx, ownerIDs)
	labelMap := r.batchStoryLabels(ctx, storyIDs)
	return r.enrichBoardStories(stories, epicNameMap, ownerNameMap, legacyOwnerNameMap, labelMap)
}

// enrichBoardStories maps epic/owner names and labels onto raw stories for board display.
func (r *PMStoryRepository) enrichBoardStories(stories []model.PMStory, epicNameMap, ownerNameMap, legacyOwnerNameMap map[string]string, labelMap map[string][]model.PMLabel) []model.BoardStory {
	result := make([]model.BoardStory, 0, len(stories))
	for _, story := range stories {
		bs := model.BoardStory{PMStory: story, Labels: []model.PMLabel{}}
		if story.EpicID != nil {
			if name, ok := epicNameMap[*story.EpicID]; ok {
				bs.EpicName = &name
			}
		}
		if story.OwnerMemberID != nil {
			if name, ok := ownerNameMap[*story.OwnerMemberID]; ok {
				bs.OwnerName = &name
			}
		} else if story.OwnerID != nil {
			if name, ok := legacyOwnerNameMap[*story.OwnerID]; ok {
				bs.OwnerName = &name
			}
		}
		if labels, ok := labelMap[story.ID]; ok {
			bs.Labels = labels
		}
		result = append(result, bs)
	}
	return result
}

// applyBoardFilters adds board-specific WHERE clauses to a query (shared between ListByWorkflowState and ListColumnStories).
func (r *PMStoryRepository) applyBoardFilters(q *gorm.DB, filters model.PMStoryFilters) *gorm.DB {
	if filters.TeamID != nil && *filters.TeamID != "" {
		q = q.Where("team_id = ?", *filters.TeamID)
	}
	if filters.Priority != nil && *filters.Priority != "" {
		vals := strings.Split(*filters.Priority, ",")
		q = q.Where("priority IN ?", vals)
	}
	if filters.StoryType != nil && *filters.StoryType != "" {
		vals := strings.Split(*filters.StoryType, ",")
		q = q.Where("story_type IN ?", vals)
	}
	if filters.EpicID != nil && *filters.EpicID != "" {
		vals := strings.Split(*filters.EpicID, ",")
		q = q.Where("epic_id IN ?", vals)
	}
	if filters.SprintID != nil && *filters.SprintID != "" {
		vals := strings.Split(*filters.SprintID, ",")
		q = q.Where("sprint_id IN ?", vals)
	}
	if filters.OwnerID != nil && *filters.OwnerID != "" {
		vals := strings.Split(*filters.OwnerID, ",")
		q = q.Where("owner_id IN ?", vals)
	}
	if filters.OwnerMemberID != nil && *filters.OwnerMemberID != "" {
		vals := strings.Split(*filters.OwnerMemberID, ",")
		q = q.Where("owner_member_id IN ?", vals)
	}
	if filters.RequesterID != nil && *filters.RequesterID != "" {
		vals := strings.Split(*filters.RequesterID, ",")
		q = q.Where("requester_id IN ?", vals)
	}
	if filters.RequesterMemberID != nil && *filters.RequesterMemberID != "" {
		vals := strings.Split(*filters.RequesterMemberID, ",")
		q = q.Where("requester_member_id IN ?", vals)
	}
	if filters.Severity != nil && *filters.Severity != "" {
		vals := strings.Split(*filters.Severity, ",")
		q = q.Where("severity IN ?", vals)
	}
	if filters.Blocked != nil && *filters.Blocked != "" {
		q = r.applyDerivedBlockedFilter(q, *filters.Blocked == "true")
	}
	if filters.Blocking != nil && *filters.Blocking != "" {
		q = r.applyBlockingFilter(q, *filters.Blocking == "true")
	}
	if filters.UpdatedAfter != nil && *filters.UpdatedAfter != "" {
		t, err := time.Parse(time.RFC3339, *filters.UpdatedAfter)
		if err == nil {
			q = q.Where("pm_stories.updated_at >= ?", t)
		}
	}
	if filters.LabelID != nil && *filters.LabelID != "" {
		vals := strings.Split(*filters.LabelID, ",")
		q = q.Joins("JOIN pm_story_labels psl ON psl.story_id = pm_stories.id").
			Where("psl.label_id IN ?", vals)
	}
	return q
}

// batchEpicNames looks up epic names by IDs.
func (r *PMStoryRepository) batchEpicNames(ctx context.Context, epicIDs map[string]struct{}) map[string]string {
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

// batchOwnerNames looks up user full names by IDs.
func (r *PMStoryRepository) batchOwnerNames(ctx context.Context, ownerIDs map[string]struct{}) map[string]string {
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
func (r *PMStoryRepository) batchMemberNames(ctx context.Context, memberIDs map[string]struct{}) map[string]string {
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

// batchStoryLabels loads labels for a set of story IDs, keyed by story ID.
func (r *PMStoryRepository) batchStoryLabels(ctx context.Context, storyIDs []string) map[string][]model.PMLabel {
	result := map[string][]model.PMLabel{}
	if len(storyIDs) == 0 {
		return result
	}
	var rows []struct {
		model.PMLabel
		StoryID string `gorm:"column:story_id"`
	}
	if err := r.db.WithContext(ctx).
		Table("pm_labels").
		Select("pm_labels.*, pm_story_labels.story_id").
		Joins("JOIN pm_story_labels ON pm_story_labels.label_id = pm_labels.id").
		Where("pm_story_labels.story_id IN ?", storyIDs).
		Order("pm_labels.name ASC").
		Scan(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		label := row.PMLabel
		result[row.StoryID] = append(result[row.StoryID], label)
	}
	return result
}

// CountByState returns story counts grouped by state for a workflow.
func (r *PMStoryRepository) CountByState(ctx context.Context, workflowID string) ([]model.StoryStateCount, error) {
	var counts []model.StoryStateCount
	if err := r.db.WithContext(ctx).
		Table("pm_workflow_states ws").
		Select("ws.id AS state_id, ws.name AS state_name, ws.state_type AS state_type, COUNT(s.id) AS story_count").
		Joins("LEFT JOIN pm_stories s ON s.workflow_state_id = ws.id AND s.archived = false").
		Where("ws.workflow_id = ?", workflowID).
		Group("ws.id, ws.name, ws.state_type, ws.position").
		Order("CASE ws.state_type WHEN 'backlog' THEN 0 WHEN 'unstarted' THEN 1 WHEN 'started' THEN 2 WHEN 'done' THEN 3 ELSE 4 END, ws.position ASC").
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("count stories by state: %w", err)
	}
	return counts, nil
}

// CountByWorkflowState returns number of active stories in a state.
func (r *PMStoryRepository) CountByWorkflowState(ctx context.Context, stateID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Where("workflow_state_id = ? AND archived = false", stateID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count stories in state: %w", err)
	}
	return count, nil
}

// UpdateStartedCompleted computes and updates started/completed fields from state type.
func (r *PMStoryRepository) UpdateStartedCompleted(ctx context.Context, storyID string) error {
	var row struct {
		StateType string
	}
	if err := r.db.WithContext(ctx).
		Table("pm_stories s").
		Select("ws.state_type").
		Joins("JOIN pm_workflow_states ws ON ws.id = s.workflow_state_id").
		Where("s.id = ?", storyID).
		Scan(&row).Error; err != nil {
		return fmt.Errorf("load story state type: %w", err)
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{}
	switch row.StateType {
	case model.PMStateTypeDone:
		updates["started"] = true
		updates["completed"] = true
		updates["moved_at"] = now
		updates["completed_at"] = now
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
		Model(&model.PMStory{}).
		Where("id = ?", storyID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update story started/completed: %w", err)
	}
	return nil
}

// UpdateSprintID updates only the sprint_id field of a story.
func (r *PMStoryRepository) UpdateSprintID(ctx context.Context, storyID string, sprintID *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Where("id = ?", storyID).
		Update("sprint_id", sprintID).Error; err != nil {
		return fmt.Errorf("update story sprint_id: %w", err)
	}
	return nil
}

func (r *PMStoryRepository) buildStoryDetail(ctx context.Context, story model.PMStory) (*model.StoryDetail, error) {
	story = r.applyDependencySummaries(ctx, []model.PMStory{story})[0]

	var owners []model.User
	if err := r.db.WithContext(ctx).
		Table("users u").
		Select("u.*").
		Joins("JOIN pm_story_owners so ON so.user_id = u.id").
		Where("so.story_id = ?", story.ID).
		Order("u.full_name ASC").
		Find(&owners).Error; err != nil {
		return nil, fmt.Errorf("list story owners: %w", err)
	}

	var followers []model.User
	if err := r.db.WithContext(ctx).
		Table("users u").
		Select("u.*").
		Joins("JOIN pm_story_followers sf ON sf.user_id = u.id").
		Where("sf.story_id = ?", story.ID).
		Order("u.full_name ASC").
		Find(&followers).Error; err != nil {
		return nil, fmt.Errorf("list story followers: %w", err)
	}

	ownerMember, err := r.loadAssignableMember(ctx, story.WorkspaceID, story.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	requesterMember, err := r.loadAssignableMember(ctx, story.WorkspaceID, story.RequesterMemberID)
	if err != nil {
		return nil, err
	}

	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Table("pm_labels l").
		Select("l.*").
		Joins("JOIN pm_story_labels sl ON sl.label_id = l.id").
		Where("sl.story_id = ?", story.ID).
		Order("l.name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list story labels: %w", err)
	}

	var state model.PMWorkflowState
	_ = r.db.WithContext(ctx).Where("id = ?", story.WorkflowStateID).First(&state).Error

	var epicName *string
	if story.EpicID != nil {
		var value string
		if err := r.db.WithContext(ctx).Table("pm_epics").Select("name").Where("id = ?", *story.EpicID).Scan(&value).Error; err == nil && value != "" {
			epicName = &value
		}
	}

	var sprintName *string
	if story.SprintID != nil {
		var value string
		if err := r.db.WithContext(ctx).Table("pm_sprints").Select("name").Where("id = ?", *story.SprintID).Scan(&value).Error; err == nil && value != "" {
			sprintName = &value
		}
	}

	var objectiveName *string
	var objectiveID *string
	if story.EpicID != nil {
		var obj struct {
			ID   string
			Name string
		}
		if err := r.db.WithContext(ctx).
			Table("pm_objectives o").
			Select("o.id, o.name").
			Joins("JOIN pm_epic_objectives eo ON eo.objective_id = o.id").
			Where("eo.epic_id = ?", *story.EpicID).
			Limit(1).
			Scan(&obj).Error; err == nil && obj.ID != "" {
			objectiveName = &obj.Name
			objectiveID = &obj.ID
		}
	}

	return &model.StoryDetail{
		Story:           story,
		Owners:          owners,
		Followers:       followers,
		OwnerMember:     ownerMember,
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
	blockedByStories []model.StoryDependencyStory
	blockingStories  []model.StoryDependencyStory
	blockedByCount   int
	blockingCount    int
	hasInboundBlocks bool
	isBlockedByStory bool
	isBlockingOther  bool
}

func (r *PMStoryRepository) applyDependencySummaries(ctx context.Context, stories []model.PMStory) []model.PMStory {
	if len(stories) == 0 {
		return stories
	}
	summaries := r.loadDependencySummaries(ctx, stories[0].WorkspaceID, stories)
	result := make([]model.PMStory, 0, len(stories))
	for _, story := range stories {
		summary := summaries[story.ID]
		story.BlockedByStories = summary.blockedByStories
		story.BlockingStories = summary.blockingStories
		story.BlockedByCount = summary.blockedByCount
		story.BlockingCount = summary.blockingCount
		story.IsBlockedByStory = summary.isBlockedByStory
		story.IsBlockingOther = summary.isBlockingOther
		story.Blocked = isStoryBlocked(story, summary)
		result = append(result, story)
	}
	return result
}

func (r *PMStoryRepository) loadDependencySummaries(ctx context.Context, workspaceID string, stories []model.PMStory) map[string]dependencySummary {
	result := make(map[string]dependencySummary, len(stories))
	if workspaceID == "" || len(stories) == 0 {
		return result
	}

	storyIDs := make([]string, 0, len(stories))
	for _, story := range stories {
		storyIDs = append(storyIDs, story.ID)
		result[story.ID] = dependencySummary{}
	}

	var links []model.PMStoryLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND link_type = ? AND (source_story_id IN ? OR target_story_id IN ?)",
			workspaceID, model.PMStoryLinkTypeBlocks, storyIDs, storyIDs).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return result
	}
	if len(links) == 0 {
		return result
	}

	relatedIDs := make(map[string]struct{}, len(links)*2)
	for _, link := range links {
		relatedIDs[link.SourceStoryID] = struct{}{}
		relatedIDs[link.TargetStoryID] = struct{}{}
	}
	ids := make([]string, 0, len(relatedIDs))
	for id := range relatedIDs {
		ids = append(ids, id)
	}

	var relatedStories []model.PMStory
	if err := r.db.WithContext(ctx).
		Select("id, display_id, name, workflow_state_id, completed").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&relatedStories).Error; err != nil {
		return result
	}

	relatedMap := make(map[string]model.StoryDependencyStory, len(relatedStories))
	for _, story := range relatedStories {
		relatedMap[story.ID] = model.StoryDependencyStory{
			ID:              story.ID,
			DisplayID:       story.DisplayID,
			Name:            story.Name,
			WorkflowStateID: story.WorkflowStateID,
			Completed:       story.Completed,
		}
	}

	for _, link := range links {
		if _, ok := result[link.TargetStoryID]; ok {
			summary := result[link.TargetStoryID]
			source, exists := relatedMap[link.SourceStoryID]
			if exists {
				summary.blockedByStories = append(summary.blockedByStories, source)
				summary.hasInboundBlocks = true
				if !source.Completed {
					summary.blockedByCount++
					summary.isBlockedByStory = true
				}
			}
			result[link.TargetStoryID] = summary
		}
		if _, ok := result[link.SourceStoryID]; ok {
			summary := result[link.SourceStoryID]
			target, exists := relatedMap[link.TargetStoryID]
			if exists {
				summary.blockingStories = append(summary.blockingStories, target)
				summary.blockingCount++
				summary.isBlockingOther = true
			}
			result[link.SourceStoryID] = summary
		}
	}

	for storyID, summary := range result {
		sort.SliceStable(summary.blockedByStories, func(i, j int) bool {
			if summary.blockedByStories[i].Completed != summary.blockedByStories[j].Completed {
				return !summary.blockedByStories[i].Completed
			}
			return summary.blockedByStories[i].DisplayID < summary.blockedByStories[j].DisplayID
		})
		sort.SliceStable(summary.blockingStories, func(i, j int) bool {
			if summary.blockingStories[i].Completed != summary.blockingStories[j].Completed {
				return !summary.blockingStories[i].Completed
			}
			return summary.blockingStories[i].DisplayID < summary.blockingStories[j].DisplayID
		})
		result[storyID] = summary
	}

	return result
}

func isStoryBlocked(story model.PMStory, summary dependencySummary) bool {
	if strings.TrimSpace(stringPtrValue(story.Blocker)) != "" {
		return true
	}
	if summary.blockedByCount > 0 {
		return true
	}
	return story.Blocked && !summary.hasInboundBlocks
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *PMStoryRepository) applyDerivedBlockedFilter(q *gorm.DB, blocked bool) *gorm.DB {
	expr := `
(
	COALESCE(NULLIF(TRIM(pm_stories.blocker), ''), '') <> ''
	OR EXISTS (
		SELECT 1
		FROM pm_story_links sl
		JOIN pm_stories blockers ON blockers.id = sl.source_story_id
		WHERE sl.workspace_id = pm_stories.workspace_id
		  AND sl.target_story_id = pm_stories.id
		  AND sl.link_type = ?
		  AND blockers.completed = FALSE
	)
	OR (
		pm_stories.blocked = TRUE
		AND NOT EXISTS (
			SELECT 1
			FROM pm_story_links legacy_sl
			WHERE legacy_sl.workspace_id = pm_stories.workspace_id
			  AND legacy_sl.target_story_id = pm_stories.id
			  AND legacy_sl.link_type = ?
		)
	)
)`
	if blocked {
		return q.Where(expr, model.PMStoryLinkTypeBlocks, model.PMStoryLinkTypeBlocks)
	}
	return q.Where("NOT "+expr, model.PMStoryLinkTypeBlocks, model.PMStoryLinkTypeBlocks)
}

func (r *PMStoryRepository) applyBlockingFilter(q *gorm.DB, blocking bool) *gorm.DB {
	expr := `
EXISTS (
	SELECT 1
	FROM pm_story_links sl
	WHERE sl.workspace_id = pm_stories.workspace_id
	  AND sl.source_story_id = pm_stories.id
	  AND sl.link_type = ?
)`
	if blocking {
		return q.Where(expr, model.PMStoryLinkTypeBlocks)
	}
	return q.Where("NOT "+expr, model.PMStoryLinkTypeBlocks)
}

func (r *PMStoryRepository) loadAssignableMember(ctx context.Context, workspaceID string, memberID *string) (*model.AssignableMember, error) {
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
			wm.status,
			wm.invited_by,
			wm.invited_at,
			wm.accepted_at
		`).
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND wm.id = ?", workspaceID, *memberID).
		Scan(&member).Error; err != nil {
		return nil, fmt.Errorf("load story member: %w", err)
	}
	if member.ID == "" {
		return nil, nil
	}
	return &member, nil
}
