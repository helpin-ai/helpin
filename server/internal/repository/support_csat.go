package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportCSATRepository stores customer satisfaction ratings.
type SupportCSATRepository struct {
	db *gorm.DB
}

func NewSupportCSATRepository(db *gorm.DB) *SupportCSATRepository {
	return &SupportCSATRepository{db: db}
}

// Upsert saves the conversation's rating. It returns the stored row, the
// previous row when one existed, and whether anything changed. Concurrent
// submissions for one conversation never fail on the unique index: the
// insert skips conflicts and updates are optimistic on the previous values.
func (r *SupportCSATRepository) Upsert(ctx context.Context, response model.SupportCSATResponse) (*model.SupportCSATResponse, *model.SupportCSATResponse, bool, error) {
	if response.ID == "" {
		response.ID = uuid.NewString()
	}
	db := r.db.WithContext(ctx)
	inserted := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "conversation_id"}}, DoNothing: true}).Create(&response)
	if inserted.Error != nil {
		return nil, nil, false, fmt.Errorf("create csat response: %w", inserted.Error)
	}
	if inserted.RowsAffected == 1 {
		return &response, nil, true, nil
	}
	for attempt := 0; attempt < 5; attempt++ {
		var existing model.SupportCSATResponse
		if err := db.Where("workspace_id = ? AND conversation_id = ?", response.WorkspaceID, response.ConversationID).
			Take(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil, false, fmt.Errorf("csat response for conversation %s belongs to another workspace", response.ConversationID)
			}
			return nil, nil, false, fmt.Errorf("load csat response: %w", err)
		}
		if existing.Rating == response.Rating && equalOptionalString(existing.Comment, response.Comment) {
			return &existing, &existing, false, nil
		}
		updates := map[string]any{
			"rating": response.Rating, "comment": response.Comment, "channel": response.Channel,
			"resolved_by": response.ResolvedBy, "assigned_user_id": response.AssignedUserID,
			"submitted_at": response.SubmittedAt, "updated_at": time.Now().UTC(),
		}
		updated := db.Model(&model.SupportCSATResponse{}).
			Where("id = ? AND rating = ? AND comment IS NOT DISTINCT FROM ?", existing.ID, existing.Rating, existing.Comment).
			Updates(updates)
		if updated.Error != nil {
			return nil, nil, false, fmt.Errorf("update csat response: %w", updated.Error)
		}
		if updated.RowsAffected == 0 {
			continue // Another submission changed the rating first; compare again.
		}
		var saved model.SupportCSATResponse
		if err := db.Where("id = ?", existing.ID).Take(&saved).Error; err != nil {
			return nil, nil, false, fmt.Errorf("reload csat response: %w", err)
		}
		return &saved, &existing, true, nil
	}
	return nil, nil, false, fmt.Errorf("csat response for conversation %s kept changing", response.ConversationID)
}

// SummariesByConversation returns the ratings for the given conversations.
func (r *SupportCSATRepository) SummariesByConversation(ctx context.Context, workspaceID string, conversationIDs []string) (map[string]model.SupportCSATSummary, error) {
	result := make(map[string]model.SupportCSATSummary, len(conversationIDs))
	if len(conversationIDs) == 0 {
		return result, nil
	}
	var rows []model.SupportCSATResponse
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id IN ?", workspaceID, conversationIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list csat responses: %w", err)
	}
	for _, row := range rows {
		result[row.ConversationID] = model.SupportCSATSummary{Rating: row.Rating, Comment: row.Comment, SubmittedAt: row.SubmittedAt}
	}
	return result, nil
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
