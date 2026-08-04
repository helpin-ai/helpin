package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// HelpcenterAnswerRepository stores cached public help center AI answers.
type HelpcenterAnswerRepository struct {
	db *gorm.DB
}

// NewHelpcenterAnswerRepository creates a HelpcenterAnswerRepository.
func NewHelpcenterAnswerRepository(db *gorm.DB) *HelpcenterAnswerRepository {
	return &HelpcenterAnswerRepository{db: db}
}

// Upsert stores an answer keyed by its cache key. Concurrent asks for the
// same query keep the first stored row.
func (r *HelpcenterAnswerRepository) Upsert(ctx context.Context, answer *model.HelpcenterAnswer) error {
	if answer == nil {
		return fmt.Errorf("answer is required")
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "cache_key"}},
			DoNothing: true,
		}).
		Create(answer).Error; err != nil {
		return fmt.Errorf("upsert helpcenter answer: %w", err)
	}
	if strings.TrimSpace(answer.ID) == "" {
		// Conflict path: load the winning row so callers return its ID.
		existing, err := r.GetByCacheKey(ctx, answer.WorkspaceID, answer.CacheKey)
		if err != nil {
			return err
		}
		if existing != nil {
			*answer = *existing
		}
	}
	return nil
}

// GetByCacheKey returns the cached answer for a cache key, or nil.
func (r *HelpcenterAnswerRepository) GetByCacheKey(ctx context.Context, workspaceID, cacheKey string) (*model.HelpcenterAnswer, error) {
	var answer model.HelpcenterAnswer
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND cache_key = ?", workspaceID, cacheKey).
		First(&answer).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get helpcenter answer: %w", err)
	}
	return &answer, nil
}

// GetByID returns one answer row, or nil.
func (r *HelpcenterAnswerRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.HelpcenterAnswer, error) {
	var answer model.HelpcenterAnswer
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&answer).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get helpcenter answer by id: %w", err)
	}
	return &answer, nil
}

// IncrementFeedback records one thumbs vote on an answer.
func (r *HelpcenterAnswerRepository) IncrementFeedback(ctx context.Context, workspaceID, id string, isHelpful bool) error {
	column := "not_helpful_count"
	if isHelpful {
		column = "helpful_count"
	}
	if err := r.db.WithContext(ctx).
		Model(&model.HelpcenterAnswer{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Update(column, gorm.Expr(column+" + 1")).Error; err != nil {
		return fmt.Errorf("record helpcenter answer feedback: %w", err)
	}
	return nil
}
