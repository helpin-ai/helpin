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

// List returns top-level comments for an entity with author info, nested replies, reactions, and attachments.
func (r *PMCommentRepository) List(ctx context.Context, entityType, entityID string) ([]model.CommentWithAuthor, error) {
	var comments []model.PMComment
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	grouped, err := r.commentsWithAuthorsByEntity(ctx, comments)
	if err != nil {
		return nil, err
	}
	return grouped[entityID], nil
}

// ListByEntityIDs returns comments grouped by entity ID for entities of the same type.
func (r *PMCommentRepository) ListByEntityIDs(ctx context.Context, entityType string, entityIDs []string) (map[string][]model.CommentWithAuthor, error) {
	result := make(map[string][]model.CommentWithAuthor, len(entityIDs))
	if len(entityIDs) == 0 {
		return result, nil
	}
	var comments []model.PMComment
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id IN ?", entityType, entityIDs).
		Order("entity_id ASC, created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("list comments by entity ids: %w", err)
	}
	grouped, err := r.commentsWithAuthorsByEntity(ctx, comments)
	if err != nil {
		return nil, err
	}
	for _, entityID := range entityIDs {
		result[entityID] = grouped[entityID]
	}
	return result, nil
}

func (r *PMCommentRepository) commentsWithAuthorsByEntity(ctx context.Context, comments []model.PMComment) (map[string][]model.CommentWithAuthor, error) {
	result := make(map[string][]model.CommentWithAuthor)
	if len(comments) == 0 {
		return result, nil
	}
	commentIDs := make([]string, 0, len(comments))
	for _, c := range comments {
		commentIDs = append(commentIDs, c.ID)
	}

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

	reactionsMap := r.loadReactions(ctx, commentIDs)
	attachmentsMap := r.loadCommentAttachments(ctx, commentIDs)

	allByEntity := make(map[string][]model.CommentWithAuthor)
	for _, c := range comments {
		allByEntity[c.EntityID] = append(allByEntity[c.EntityID], model.CommentWithAuthor{
			Comment:     c,
			Author:      authorMap[c.AuthorID],
			Reactions:   reactionsMap[c.ID],
			Attachments: attachmentsMap[c.ID],
		})
	}

	for entityID, comments := range allByEntity {
		childrenMap := make(map[string][]model.CommentWithAuthor)
		var topLevel []model.CommentWithAuthor
		for _, cwa := range comments {
			if cwa.Comment.ParentID != nil && *cwa.Comment.ParentID != "" {
				childrenMap[*cwa.Comment.ParentID] = append(childrenMap[*cwa.Comment.ParentID], cwa)
			} else {
				topLevel = append(topLevel, cwa)
			}
		}

		entityResult := make([]model.CommentWithAuthor, 0, len(topLevel))
		for _, tl := range topLevel {
			replies := childrenMap[tl.Comment.ID]
			tl.ReplyCount = len(replies)
			tl.Replies = replies
			entityResult = append(entityResult, tl)
		}
		result[entityID] = entityResult
	}
	return result, nil
}

// loadReactions batch-loads reactions for the given comment IDs and returns them grouped by comment ID.
func (r *PMCommentRepository) loadReactions(ctx context.Context, commentIDs []string) map[string][]model.ReactionSummary {
	result := make(map[string][]model.ReactionSummary)
	if len(commentIDs) == 0 {
		return result
	}

	var reactions []model.PMCommentReaction
	if err := r.db.WithContext(ctx).
		Where("comment_id IN ?", commentIDs).
		Order("created_at ASC").
		Find(&reactions).Error; err != nil {
		return result
	}

	// Group by comment_id + emoji.
	type key struct {
		commentID string
		emoji     string
	}
	grouped := make(map[key][]string)
	order := make(map[string][]string) // comment_id → ordered emojis (first-seen order)
	seen := make(map[key]bool)

	for _, rx := range reactions {
		k := key{rx.CommentID, rx.Emoji}
		grouped[k] = append(grouped[k], rx.UserID)
		if !seen[k] {
			seen[k] = true
			order[rx.CommentID] = append(order[rx.CommentID], rx.Emoji)
		}
	}

	for commentID, emojis := range order {
		summaries := make([]model.ReactionSummary, 0, len(emojis))
		for _, emoji := range emojis {
			k := key{commentID, emoji}
			summaries = append(summaries, model.ReactionSummary{
				Emoji:   emoji,
				Count:   len(grouped[k]),
				UserIDs: grouped[k],
			})
		}
		result[commentID] = summaries
	}
	return result
}

// loadCommentAttachments batch-loads uploaded attachments for the given comment IDs.
func (r *PMCommentRepository) loadCommentAttachments(ctx context.Context, commentIDs []string) map[string][]model.AttachmentResponse {
	result := make(map[string][]model.AttachmentResponse)
	if len(commentIDs) == 0 {
		return result
	}

	var attachments []model.PMAttachment
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id IN ? AND is_uploaded = ?", "comment", commentIDs, true).
		Order("created_at ASC").
		Find(&attachments).Error; err != nil {
		return result
	}

	for _, a := range attachments {
		result[a.EntityID] = append(result[a.EntityID], model.AttachmentResponse{
			Attachment: a,
		})
	}
	return result
}

// AddReaction adds a reaction to a comment.
func (r *PMCommentRepository) AddReaction(ctx context.Context, reaction *model.PMCommentReaction) error {
	if err := r.db.WithContext(ctx).Create(reaction).Error; err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

// RemoveReaction removes a reaction from a comment.
func (r *PMCommentRepository) RemoveReaction(ctx context.Context, commentID, userID, emoji string) error {
	if err := r.db.WithContext(ctx).
		Where("comment_id = ? AND user_id = ? AND emoji = ?", commentID, userID, emoji).
		Delete(&model.PMCommentReaction{}).Error; err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	return nil
}

// HasReaction checks if a user has reacted with a specific emoji on a comment.
func (r *PMCommentRepository) HasReaction(ctx context.Context, commentID, userID, emoji string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.PMCommentReaction{}).
		Where("comment_id = ? AND user_id = ? AND emoji = ?", commentID, userID, emoji).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check reaction: %w", err)
	}
	return count > 0, nil
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
	db := r.db.WithContext(ctx)
	if comment.ResolvedAt == nil && comment.ResolvedBy == nil {
		db = db.Omit("ResolvedAt", "ResolvedBy")
	}
	if err := db.Create(comment).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

// Update updates a comment.
func (r *PMCommentRepository) Update(ctx context.Context, comment *model.PMComment) error {
	db := r.db.WithContext(ctx)
	if comment.ResolvedAt == nil && comment.ResolvedBy == nil {
		db = db.Omit("ResolvedAt", "ResolvedBy")
	}
	if err := db.Save(comment).Error; err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	return nil
}

// UpdateResolution updates only the comment resolution fields.
func (r *PMCommentRepository) UpdateResolution(ctx context.Context, id string, resolvedAt interface{}, resolvedBy interface{}) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PMComment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"resolved_at": resolvedAt,
			"resolved_by": resolvedBy,
		}).Error; err != nil {
		return fmt.Errorf("update comment resolution: %w", err)
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
