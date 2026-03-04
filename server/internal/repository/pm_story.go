package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// PMStoryRepository handles DB operations for stories and relations.
type PMStoryRepository struct {
	db *gorm.DB
}

// NewPMStoryRepository creates a new PMStoryRepository.
func NewPMStoryRepository(db *gorm.DB) *PMStoryRepository {
	return &PMStoryRepository{db: db}
}

// List returns stories with filters and pagination.
func (r *PMStoryRepository) List(ctx context.Context, workspaceID string, filters model.PMStoryFilters, pagination model.PMPagination) ([]model.PMStory, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.PMStory{}).Where("workspace_id = ?", workspaceID)

	if filters.TeamID != nil && *filters.TeamID != "" {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.EpicID != nil && *filters.EpicID != "" {
		query = query.Where("epic_id = ?", *filters.EpicID)
	}
	if filters.IterationID != nil && *filters.IterationID != "" {
		query = query.Where("iteration_id = ?", *filters.IterationID)
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
	if filters.Priority != nil && *filters.Priority != "" {
		query = query.Where("priority = ?", *filters.Priority)
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
	return stories, total, nil
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

// ListByWorkflowState returns board columns grouped by workflow state, optionally filtered by team.
func (r *PMStoryRepository) ListByWorkflowState(ctx context.Context, workflowID string, teamID string) ([]model.StoryStateColumn, error) {
	var states []model.PMWorkflowState
	if err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("position ASC").
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list board states: %w", err)
	}

	// Fetch all stories for the workflow in a single query.
	stateIDs := make([]string, len(states))
	for i, s := range states {
		stateIDs[i] = s.ID
	}

	var allStories []model.PMStory
	storyQuery := r.db.WithContext(ctx).
		Where("workflow_state_id IN ? AND archived = false", stateIDs)
	if teamID != "" {
		storyQuery = storyQuery.Where("team_id = ?", teamID)
	}
	if err := storyQuery.
		Order("position ASC, updated_at DESC").
		Find(&allStories).Error; err != nil {
		return nil, fmt.Errorf("list board stories: %w", err)
	}

	// Group stories by state and collect unique IDs for batch lookups.
	storiesByState := map[string][]model.PMStory{}
	epicIDs := map[string]struct{}{}
	ownerIDs := map[string]struct{}{}
	for _, s := range allStories {
		storiesByState[s.WorkflowStateID] = append(storiesByState[s.WorkflowStateID], s)
		if s.EpicID != nil {
			epicIDs[*s.EpicID] = struct{}{}
		}
		if s.OwnerID != nil {
			ownerIDs[*s.OwnerID] = struct{}{}
		}
	}

	// Batch-lookup epic names.
	epicNameMap := map[string]string{}
	if len(epicIDs) > 0 {
		ids := make([]string, 0, len(epicIDs))
		for id := range epicIDs {
			ids = append(ids, id)
		}
		var rows []struct {
			ID   string
			Name string
		}
		if err := r.db.WithContext(ctx).Table("pm_epics").Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("batch lookup epic names: %w", err)
		}
		for _, row := range rows {
			epicNameMap[row.ID] = row.Name
		}
	}

	// Batch-lookup owner names.
	ownerNameMap := map[string]string{}
	if len(ownerIDs) > 0 {
		ids := make([]string, 0, len(ownerIDs))
		for id := range ownerIDs {
			ids = append(ids, id)
		}
		var rows []struct {
			ID       string
			FullName string `gorm:"column:full_name"`
		}
		if err := r.db.WithContext(ctx).Table("users").Select("id, full_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("batch lookup owner names: %w", err)
		}
		for _, row := range rows {
			ownerNameMap[row.ID] = row.FullName
		}
	}

	// Build enriched board stories.
	columns := make([]model.StoryStateColumn, 0, len(states))
	for _, state := range states {
		stateStories := storiesByState[state.ID]
		boardStories := make([]model.BoardStory, 0, len(stateStories))
		pointTotal := 0
		for _, story := range stateStories {
			bs := model.BoardStory{PMStory: story}
			if story.EpicID != nil {
				if name, ok := epicNameMap[*story.EpicID]; ok {
					bs.EpicName = &name
				}
			}
			if story.OwnerID != nil {
				if name, ok := ownerNameMap[*story.OwnerID]; ok {
					bs.OwnerName = &name
				}
			}
			if story.Estimate != nil {
				pointTotal += *story.Estimate
			}
			boardStories = append(boardStories, bs)
		}
		columns = append(columns, model.StoryStateColumn{
			State:      state,
			Stories:    boardStories,
			StoryCount: len(boardStories),
			PointTotal: pointTotal,
		})
	}
	return columns, nil
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
		Order("ws.position ASC").
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

func (r *PMStoryRepository) buildStoryDetail(ctx context.Context, story model.PMStory) (*model.StoryDetail, error) {
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

	var iterationName *string
	if story.IterationID != nil {
		var value string
		if err := r.db.WithContext(ctx).Table("pm_iterations").Select("name").Where("id = ?", *story.IterationID).Scan(&value).Error; err == nil && value != "" {
			iterationName = &value
		}
	}

	return &model.StoryDetail{
		Story:         story,
		Owners:        owners,
		Followers:     followers,
		Labels:        labels,
		EpicName:      epicName,
		IterationName: iterationName,
		State:         &state,
	}, nil
}
