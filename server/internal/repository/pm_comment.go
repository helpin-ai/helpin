package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMCommentRepository handles DB operations for comments.
type PMCommentRepository struct {
	db *gorm.DB
}

// NewPMCommentRepository creates a new PMCommentRepository.
func NewPMCommentRepository(db *gorm.DB) *PMCommentRepository {
	return &PMCommentRepository{db: db}
}

// List returns comments for an entity with author info.
func (r *PMCommentRepository) List(ctx context.Context, entityType, entityID string) ([]model.CommentWithAuthor, error) {
	var comments []model.PMComment
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	result := make([]model.CommentWithAuthor, 0, len(comments))
	for _, comment := range comments {
		var author model.User
		if err := r.db.WithContext(ctx).Where("id = ?", comment.AuthorID).First(&author).Error; err != nil {
			return nil, fmt.Errorf("load comment author: %w", err)
		}
		result = append(result, model.CommentWithAuthor{Comment: comment, Author: author})
	}
	return result, nil
}

// GetByID returns a comment.
func (r *PMCommentRepository) GetByID(ctx context.Context, id string) (*model.PMComment, error) {
	var comment model.PMComment
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&comment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get comment: %w", err)
	}
	return &comment, nil
}

// Create inserts a comment.
func (r *PMCommentRepository) Create(ctx context.Context, comment *model.PMComment) error {
	if err := r.db.WithContext(ctx).Create(comment).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

// Update updates a comment.
func (r *PMCommentRepository) Update(ctx context.Context, comment *model.PMComment) error {
	if err := r.db.WithContext(ctx).Save(comment).Error; err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	return nil
}

// Delete removes a comment.
func (r *PMCommentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMComment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}
