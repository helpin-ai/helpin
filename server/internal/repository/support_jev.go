package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportJevRepository records bounded, deduplicated external decision attempts.
type SupportJevRepository struct{ db *gorm.DB }

// NewSupportJevRepository uses the existing triage event table; no content is stored.
func NewSupportJevRepository(db *gorm.DB) *SupportJevRepository { return &SupportJevRepository{db: db} }

// Reserve serializes admission per workspace, including failed calls in the daily cap.
func (r *SupportJevRepository) Reserve(ctx context.Context, workspace, conversation, hash string, limit int) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ws model.Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", workspace).First(&ws).Error; err != nil {
			return err
		}
		var n int64
		base := tx.Model(&model.SupportConversationTriageEvent{}).Where("workspace_id = ? AND event_type = ?", workspace, "jev_decision")
		if err := base.Session(&gorm.Session{}).Where("conversation_id = ? AND input_hash = ?", conversation, hash).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		if err := base.Session(&gorm.Session{}).Where("created_at >= ?", time.Now().UTC().Truncate(24*time.Hour)).Count(&n).Error; err != nil {
			return err
		}
		if n >= int64(limit) {
			return nil
		}
		id = uuid.NewString()
		return tx.Create(&model.SupportConversationTriageEvent{ID: id, WorkspaceID: workspace, ConversationID: conversation, EventType: "jev_decision", InputHash: hash, Payload: model.JSONB{"status": "pending"}}).Error
	})
	return id, err
}

// Finish stores only decision metadata, never the conversation or API credential.
func (r *SupportJevRepository) Finish(ctx context.Context, workspace, id string, payload json.RawMessage) error {
	var value model.JSONB
	if err := json.Unmarshal(payload, &value); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.SupportConversationTriageEvent{}).Where("workspace_id = ? AND id = ? AND event_type = ?", workspace, id, "jev_decision").Update("payload", value).Error
}
