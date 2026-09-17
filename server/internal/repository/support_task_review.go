package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// LinkTaskReviewed validates current evidence while holding the conversation and
// target task locks, and commits the link and CRM association together. The
// support message projection triggers serialize public message changes through
// the parent conversation lock; ordinary reads here avoid reversing that order.
// validate must only inspect its arguments, not perform database or provider IO.
func (r *SupportConversationRepository) LinkTaskReviewed(ctx context.Context, workspaceID, conversationID, taskID string, validate func(*model.SupportConversation, []model.SupportMessage, *model.PMTask) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.PMTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", taskID, workspaceID).First(&task).Error; err != nil {
			return err
		}
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", conversationID, workspaceID).First(&conversation).Error; err != nil {
			return err
		}
		messages, err := NewSupportMessageRepository(tx).ListByConversation(ctx, workspaceID, conversationID, false)
		if err != nil {
			return err
		}
		if validate == nil {
			return errors.New("review validation is required")
		}
		if err := validate(&conversation, messages, &task); err != nil {
			return err
		}
		return linkSupportTask(ctx, tx, workspaceID, conversationID, taskID)
	})
}

func linkSupportTask(ctx context.Context, tx *gorm.DB, workspaceID, conversationID, taskID string) error {
	if err := tx.Model(&model.SupportConversation{}).Where("id = ? AND workspace_id = ?", conversationID, workspaceID).Update("linked_task_id", taskID).Error; err != nil {
		return err
	}
	// Remove task associations in either orientation while preserving CRM links.
	if err := tx.Where("workspace_id = ? AND ((from_object_type = ? AND from_object_id = ? AND to_object_type = ?) OR (to_object_type = ? AND to_object_id = ? AND from_object_type = ?))", workspaceID, model.CRMObjectSupportConversation, conversationID, model.CRMObjectTask, model.CRMObjectSupportConversation, conversationID, model.CRMObjectTask).Delete(&model.CRMAssociation{}).Error; err != nil {
		return err
	}
	return NewCRMAssociationRepository(tx).Create(ctx, &model.CRMAssociation{ID: uuid.NewString(), WorkspaceID: workspaceID, FromObjectType: model.CRMObjectSupportConversation, FromObjectID: conversationID, ToObjectType: model.CRMObjectTask, ToObjectID: taskID})
}

// RequireSupportEvidence fences a reviewed task draft inside the canonical task
// creation transaction. The caller must complete linking before committing.
func (r *PMTaskRepository) RequireSupportEvidence(ctx context.Context, workspaceID, conversationID string, validate func(*model.SupportConversation, []model.SupportMessage) error) error {
	if !r.inMutationTransaction {
		return errors.New("support evidence check requires task mutation transaction")
	}
	var conversation model.SupportConversation
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", conversationID, workspaceID).First(&conversation).Error; err != nil {
		return err
	}
	messages, err := NewSupportMessageRepository(r.db).ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		return err
	}
	return validate(&conversation, messages)
}

// LinkCreatedSupportTask commits the new task's evidence link in its transaction.
func (r *PMTaskRepository) LinkCreatedSupportTask(ctx context.Context, workspaceID, conversationID, taskID string) error {
	if !r.inMutationTransaction {
		return errors.New("support linking requires task mutation transaction")
	}
	return linkSupportTask(ctx, r.db.WithContext(ctx), workspaceID, conversationID, taskID)
}
