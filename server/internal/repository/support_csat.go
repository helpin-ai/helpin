package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

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
// previous row when one existed, and whether anything changed.
func (r *SupportCSATRepository) Upsert(ctx context.Context, response model.SupportCSATResponse) (*model.SupportCSATResponse, *model.SupportCSATResponse, bool, error) {
	var saved model.SupportCSATResponse
	var previous *model.SupportCSATResponse
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.SupportCSATResponse
		err := tx.Where("workspace_id = ? AND conversation_id = ?", response.WorkspaceID, response.ConversationID).
			Take(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if response.ID == "" {
				response.ID = uuid.NewString()
			}
			if err := tx.Create(&response).Error; err != nil {
				return fmt.Errorf("create csat response: %w", err)
			}
			saved, changed = response, true
			return nil
		case err != nil:
			return fmt.Errorf("load csat response: %w", err)
		}
		before := existing
		previous = &before
		if existing.Rating == response.Rating && equalOptionalString(existing.Comment, response.Comment) {
			saved = existing
			return nil
		}
		updates := map[string]any{
			"rating": response.Rating, "comment": response.Comment, "channel": response.Channel,
			"resolved_by": response.ResolvedBy, "assigned_user_id": response.AssignedUserID,
			"submitted_at": response.SubmittedAt, "updated_at": time.Now().UTC(),
		}
		if err := tx.Model(&model.SupportCSATResponse{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update csat response: %w", err)
		}
		if err := tx.Where("id = ?", existing.ID).Take(&saved).Error; err != nil {
			return fmt.Errorf("reload csat response: %w", err)
		}
		changed = true
		return nil
	})
	if err != nil {
		return nil, nil, false, err
	}
	return &saved, previous, changed, nil
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
