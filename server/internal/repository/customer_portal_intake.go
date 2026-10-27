package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RecordIntakeEvent stores one step of writing a portal request.
func (r *CustomerPortalRepository) RecordIntakeEvent(ctx context.Context, workspaceID, intakeID, eventType string, articleIDs []string, conversationID *string) error {
	return recordPortalIntakeEvent(r.db.WithContext(ctx), workspaceID, intakeID, eventType, articleIDs, conversationID)
}

func recordPortalIntakeEvent(db *gorm.DB, workspaceID, intakeID, eventType string, articleIDs []string, conversationID *string) error {
	if articleIDs == nil {
		articleIDs = []string{}
	}
	encoded, err := json.Marshal(articleIDs)
	if err != nil {
		return err
	}
	return db.Create(&model.SupportPortalIntakeEvent{
		ID: uuid.NewString(), WorkspaceID: workspaceID, IntakeID: intakeID, EventType: eventType,
		ArticleIDs: string(encoded), ArticleCount: len(articleIDs), ConversationID: conversationID,
		OccurredAt: time.Now().UTC(),
	}).Error
}

// ConversationPortalIntake returns what the customer saw while writing the
// request, from its first customer message, or nil.
func (r *CustomerPortalRepository) ConversationPortalIntake(ctx context.Context, workspaceID, conversationID string) (*model.PortalIntake, error) {
	var row struct{ Metadata string }
	err := r.db.WithContext(ctx).Table("support_messages").Select("metadata").
		Where("workspace_id = ? AND conversation_id = ? AND sender_type = 'customer' AND deleted_at IS NULL", workspaceID, conversationID).
		Order("created_at ASC, id ASC").Limit(1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var metadata struct {
		Intake *model.PortalIntake `json:"portal_intake"`
	}
	if row.Metadata == "" || json.Unmarshal([]byte(row.Metadata), &metadata) != nil {
		return nil, nil
	}
	return metadata.Intake, nil
}
