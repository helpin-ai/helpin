package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SaveVisitorFeedback records one immutable vote per answer. Retries return the saved
// vote, including when another tab won the race. Row locking also protects enrichment.
func (r *SupportMessageRepository) SaveVisitorFeedback(ctx context.Context, workspaceID, messageID string, helpful bool) (*model.SupportMessage, bool, error) {
	var message model.SupportMessage
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", messageID, workspaceID).First(&message).Error; err != nil {
			return err
		}
		if !message.CanReceiveVisitorFeedback() {
			return fmt.Errorf("answer not available for feedback")
		}
		if message.VisitorFeedback() != nil {
			return nil
		}
		metadata := map[string]json.RawMessage{}
		if message.Metadata != "" {
			if err := json.Unmarshal([]byte(message.Metadata), &metadata); err != nil {
				return fmt.Errorf("decode answer metadata: %w", err)
			}
		}
		if metadata == nil {
			metadata = map[string]json.RawMessage{}
		}
		now := time.Now().UTC()
		feedback, err := json.Marshal(model.SupportAnswerFeedback{Helpful: helpful, SubmittedAt: now})
		if err != nil {
			return err
		}
		metadata["visitor_feedback"] = feedback
		raw, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		message.Metadata = string(raw)
		if err := tx.Model(&message).Update("metadata", message.Metadata).Error; err != nil {
			return err
		}
		// Existing daily coverage candidate selection uses conversation.updated_at.
		// A late vote must be analyzed even when the conversation was already resolved.
		if err := tx.Model(&model.SupportConversation{}).Where("id = ? AND workspace_id = ?", message.ConversationID, workspaceID).Update("updated_at", now).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return &message, created, err
}

func (r *SupportMessageRepository) updateEnrichedMetadata(ctx context.Context, id, metadata string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var message model.SupportMessage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&message, "id = ?", id).Error; err != nil {
			return err
		}
		var next map[string]json.RawMessage
		if err := json.Unmarshal([]byte(metadata), &next); err != nil {
			return err
		}
		if next == nil {
			next = map[string]json.RawMessage{}
		}
		// Enrichment cannot overwrite or manufacture visitor feedback.
		delete(next, "visitor_feedback")
		if feedback := message.VisitorFeedback(); feedback != nil {
			raw, err := json.Marshal(feedback)
			if err != nil {
				return err
			}
			next["visitor_feedback"] = raw
		}
		raw, err := json.Marshal(next)
		if err != nil {
			return err
		}
		return tx.Model(&message).Update("metadata", string(raw)).Error
	})
}
