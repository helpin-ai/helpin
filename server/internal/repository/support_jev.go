package repository

import (
	"context"
	"encoding/json"
	"errors"
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

// SupportJevAdmission separates cached results, cooldowns and the shared daily cap.
type SupportJevAdmission struct {
	Event        *model.SupportConversationTriageEvent
	CallProvider bool
	Limited      bool
}

// Reserve serializes admission per workspace, including failed calls in the daily
// cap. Successful decisions are reusable; failed or abandoned attempts may retry
// after a minute without allowing simultaneous duplicate provider calls.
func (r *SupportJevRepository) Reserve(ctx context.Context, workspace, conversation, hash string, limit int) (*SupportJevAdmission, error) {
	if workspace == "" || conversation == "" || hash == "" || limit < 1 {
		return nil, errors.New("invalid support decision admission")
	}
	admission := &SupportJevAdmission{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ws model.Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", workspace).First(&ws).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		base := tx.Model(&model.SupportConversationTriageEvent{}).Where("workspace_id = ? AND event_type = ?", workspace, "jev_decision")
		var previous model.SupportConversationTriageEvent
		err := base.Session(&gorm.Session{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("conversation_id = ? AND input_hash = ?", conversation, hash).Order("created_at DESC, id DESC").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if previous.Payload["status"] == "ok" || now.Sub(previous.CreatedAt) < time.Minute {
				admission.Event = &previous
				return nil
			}
			if previous.Payload["status"] == "pending" {
				if err := tx.Model(&previous).Where("payload->>'status' = ?", "pending").Update("payload", model.JSONB{"status": "abandoned"}).Error; err != nil {
					return err
				}
			}
		}
		var n int64
		if err := base.Session(&gorm.Session{}).Where("created_at >= ?", now.Truncate(24*time.Hour)).Count(&n).Error; err != nil {
			return err
		}
		if n >= int64(limit) {
			admission.Limited = true
			return nil
		}
		event := &model.SupportConversationTriageEvent{ID: uuid.NewString(), WorkspaceID: workspace, ConversationID: conversation, EventType: "jev_decision", InputHash: hash, Payload: model.JSONB{"status": "pending"}, CreatedAt: now}
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		admission.Event, admission.CallProvider = event, true
		return nil
	})
	return admission, err
}

// Finish stores only decision metadata, never the conversation or API credential.
func (r *SupportJevRepository) Finish(ctx context.Context, workspace, id string, payload json.RawMessage) error {
	var value model.JSONB
	if err := json.Unmarshal(payload, &value); err != nil {
		return err
	}
	if value["status"] != "ok" && value["status"] != "provider_error" && value["status"] != "usage_error" {
		return errors.New("invalid support decision settlement")
	}
	result := r.db.WithContext(ctx).Model(&model.SupportConversationTriageEvent{}).Where("workspace_id = ? AND id = ? AND event_type = ? AND payload->>'status' = ?", workspace, id, "jev_decision", "pending").Update("payload", value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("support decision attempt no longer pending")
	}
	return nil
}
