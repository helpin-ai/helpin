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

// List returns top-level comments for an entity with author info and nested replies.
func (r *PMCommentRepository) List(ctx context.Context, entityType, entityID string) ([]model.CommentWithAuthor, error) {
	var comments []model.PMComment
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	// Build author cache to avoid N+1 queries.
	authorIDs := make(map[string]struct{}, len(comments))
	for _, c := range comments {
		authorIDs[c.AuthorID] = struct{}{}
	}
	uniqueIDs := make([]string, 0, len(authorIDs))
	for id := range authorIDs {
		uniqueIDs = append(uniqueIDs, id)
	}
	var authors []model.User
	if len(uniqueIDs) > 0 {
		if err := r.db.WithContext(ctx).Where("id IN ?", uniqueIDs).Find(&authors).Error; err != nil {
			return nil, fmt.Errorf("load comment authors: %w", err)
		}
	}
	authorMap := make(map[string]model.User, len(authors))
	for _, a := range authors {
		authorMap[a.ID] = a
	}

	// Separate top-level and replies, then nest replies under parents.
	allWithAuthor := make([]model.CommentWithAuthor, 0, len(comments))
	for _, c := range comments {
		allWithAuthor = append(allWithAuthor, model.CommentWithAuthor{
			Comment: c,
			Author:  authorMap[c.AuthorID],
		})
	}

	childrenMap := make(map[string][]model.CommentWithAuthor)
	var topLevel []model.CommentWithAuthor
	for _, cwa := range allWithAuthor {
		if cwa.Comment.ParentID != nil && *cwa.Comment.ParentID != "" {
			childrenMap[*cwa.Comment.ParentID] = append(childrenMap[*cwa.Comment.ParentID], cwa)
		} else {
			topLevel = append(topLevel, cwa)
		}
	}

	result := make([]model.CommentWithAuthor, 0, len(topLevel))
	for _, tl := range topLevel {
		replies := childrenMap[tl.Comment.ID]
		tl.ReplyCount = len(replies)
		tl.Replies = replies
		result = append(result, tl)
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
