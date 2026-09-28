package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportAIControlConflict means ownership changed after the caller read it.
var ErrSupportAIControlConflict = errors.New("conversation AI control changed; refresh and try again")

// ChangeAIControl serializes a service-owned transition with AI reply publication.
// The callback returns only changed fields and an internal audit/handoff note.
// No-op transitions return nil fields. The prior run is returned for cancellation
// after commit; cancellation failure cannot undo the durable ownership change.
func (r *SupportConversationRepository) ChangeAIControl(ctx context.Context, workspaceID, id string, change func(*model.SupportConversation) (map[string]any, *model.SupportMessage, error), messages ...*model.SupportMessage) (string, bool, error) {
	return r.ChangeAIControlWithHook(ctx, workspaceID, id, change, nil, messages...)
}

// ChangeAIControlWithHook commits channel-specific work with the ownership
// transition. A hook error rolls back the entire change.
func (r *SupportConversationRepository) ChangeAIControlWithHook(ctx context.Context, workspaceID, id string, change func(*model.SupportConversation) (map[string]any, *model.SupportMessage, error), hook func(*gorm.DB) error, messages ...*model.SupportMessage) (string, bool, error) {
	var previousRun string
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&conv).Error; err != nil {
			return err
		}
		fields, note, err := change(&conv)
		if err != nil || len(fields) == 0 {
			return err
		}
		if conv.AIActiveRunID != nil {
			previousRun = *conv.AIActiveRunID
		}
		fields["ai_control_version"] = conv.AIControlVersion + 1
		fields["ai_active_run_id"] = nil
		if err := tx.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ?", workspaceID, id).Updates(fields).Error; err != nil {
			return err
		}
		if note != nil {
			if !note.IsInternal || note.WorkspaceID != workspaceID || note.ConversationID != id {
				return errors.New("invalid AI control note")
			}
			if err := NewSupportMessageRepository(tx).Create(ctx, note); err != nil {
				return err
			}
		}
		for _, message := range messages {
			if message == nil {
				continue
			}
			if message.WorkspaceID != workspaceID || message.ConversationID != id {
				return errors.New("invalid handoff message scope")
			}
			if err := NewSupportMessageRepository(tx).Create(ctx, message); err != nil {
				return err
			}
		}
		if hook != nil {
			if err := hook(tx); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return previousRun, changed && err == nil, err
}

// BindAIRun prevents a launch that raced with takeover/return from owning a
// newer conversation generation. Reply publication checks this owner again.
func (r *SupportConversationRepository) BindAIRun(ctx context.Context, conv *model.SupportConversation, runID string, agentID ...string) (bool, error) {
	fields := map[string]any{"ai_active_run_id": runID}
	if conv.Channel == "portal" && len(agentID) > 0 && agentID[0] != "" {
		fields["assigned_agent_id"] = agentID[0]
	}
	result := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND id = ? AND ai_control_version = ?", conv.WorkspaceID, conv.ID, conv.AIControlVersion).
		Where("NOT coalesce(human_takeover, false) AND assigned_user_id IS NULL AND opened_by_user_id IS NULL AND customer_requested_human_at IS NULL AND anonymized_at IS NULL AND status <> 'spam' AND coalesce(ai_state, '') <> 'escalated'").
		Updates(fields)
	return result.RowsAffected == 1, result.Error
}
