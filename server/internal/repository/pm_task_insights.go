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

// PMTaskInsightsRepository stores read cursors and durable task standing briefs.
type PMTaskInsightsRepository struct {
	db *gorm.DB
}

func NewPMTaskInsightsRepository(db *gorm.DB) *PMTaskInsightsRepository {
	return &PMTaskInsightsRepository{db: db}
}

func (r *PMTaskInsightsRepository) GetReadState(ctx context.Context, workspaceID, taskID, userID string) (*model.PMTaskUpdateRead, error) {
	var state model.PMTaskUpdateRead
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ? AND user_id = ?", workspaceID, taskID, userID).
		First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task update read state: %w", err)
	}
	return &state, nil
}

func (r *PMTaskInsightsRepository) AdvanceReadState(ctx context.Context, workspaceID, taskID, userID string, seenThrough time.Time, initializeOnly bool) (*model.PMTaskUpdateRead, error) {
	state := model.PMTaskUpdateRead{
		WorkspaceID: workspaceID,
		TaskID:      taskID,
		UserID:      userID,
		SeenThrough: seenThrough.UTC(),
	}
	assignments := map[string]interface{}{
		"seen_through": gorm.Expr("CASE WHEN pm_task_update_reads.seen_through < EXCLUDED.seen_through THEN EXCLUDED.seen_through ELSE pm_task_update_reads.seen_through END"),
		"updated_at":   time.Now().UTC(),
	}
	if initializeOnly {
		assignments = map[string]interface{}{}
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "task_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(assignments),
		DoNothing: initializeOnly,
	}).Create(&state).Error; err != nil {
		return nil, fmt.Errorf("advance task update read state: %w", err)
	}
	return r.GetReadState(ctx, workspaceID, taskID, userID)
}

func (r *PMTaskInsightsRepository) GetBrief(ctx context.Context, workspaceID, taskID string) (*model.PMTaskStandingBrief, error) {
	var brief model.PMTaskStandingBrief
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ?", workspaceID, taskID).
		First(&brief).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task standing brief: %w", err)
	}
	return &brief, nil
}

func (r *PMTaskInsightsRepository) SaveBrief(ctx context.Context, brief *model.PMTaskStandingBrief) error {
	if brief == nil {
		return fmt.Errorf("task standing brief is required")
	}
	updates := map[string]interface{}{
		"narrative":         brief.Narrative,
		"suggestions":       brief.Suggestions,
		"evidence":          brief.Evidence,
		"status":            brief.Status,
		"source_updated_at": brief.SourceUpdatedAt,
		"computed_at":       brief.ComputedAt,
		"last_triggered_at": brief.LastTriggeredAt,
		"last_error":        brief.LastError,
		"version":           brief.Version,
		"updated_at":        time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "task_id"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(brief).Error; err != nil {
		return fmt.Errorf("save task standing brief: %w", err)
	}
	return nil
}

func (r *PMTaskInsightsRepository) DismissSuggestion(ctx context.Context, dismissal *model.PMTaskBriefSuggestionDismissal) error {
	if dismissal == nil {
		return fmt.Errorf("task brief suggestion dismissal is required")
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(dismissal).Error; err != nil {
		return fmt.Errorf("dismiss task brief suggestion: %w", err)
	}
	return nil
}

func (r *PMTaskInsightsRepository) ListDismissedSuggestionKeys(ctx context.Context, workspaceID, taskID string) (map[string]struct{}, error) {
	var rows []model.PMTaskBriefSuggestionDismissal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ?", workspaceID, taskID).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list dismissed task brief suggestions: %w", err)
	}
	keys := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		keys[row.SuggestionKey] = struct{}{}
	}
	return keys, nil
}

func (r *PMTaskInsightsRepository) LatestSourceUpdatedAt(ctx context.Context, workspaceID, taskID string, taskUpdatedAt time.Time) (time.Time, error) {
	latest := taskUpdatedAt.UTC()
	sources := []struct {
		value interface{}
		where string
		args  []interface{}
		field string
	}{
		{&model.PMComment{}, "entity_type = ? AND entity_id = ?", []interface{}{"task", taskID}, "updated_at"},
		{&model.PMActivityLog{}, "workspace_id = ? AND entity_type = ? AND entity_id = ?", []interface{}{workspaceID, "task", taskID}, "created_at"},
		{&model.AgentRun{}, "workspace_id = ? AND target_type = ? AND target_id = ?", []interface{}{workspaceID, "task", taskID}, "updated_at"},
		{&model.TaskGitLink{}, "workspace_id = ? AND task_id = ?", []interface{}{workspaceID, taskID}, "updated_at"},
		{&model.PMChecklistItem{}, "task_id = ?", []interface{}{taskID}, "updated_at"},
		{&model.PMExternalLink{}, "(task_id = ? OR (entity_type = ? AND entity_id = ?))", []interface{}{taskID, "task", taskID}, "updated_at"},
	}
	for _, source := range sources {
		var value *time.Time
		if err := r.db.WithContext(ctx).Model(source.value).
			Select("MAX("+source.field+")").
			Where(source.where, source.args...).
			Scan(&value).Error; err != nil {
			return time.Time{}, fmt.Errorf("load latest task source timestamp: %w", err)
		}
		if value != nil && value.After(latest) {
			latest = value.UTC()
		}
	}
	return latest, nil
}
