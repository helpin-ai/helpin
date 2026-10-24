package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm/clause"
)

// ListPins lists a user's pins in one workspace, newest pins first.
func (r *DockChatRepository) ListPins(ctx context.Context, workspaceID, userID string) ([]model.DockPin, error) {
	var pins []model.DockPin
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Order("created_at DESC, resource_id").Find(&pins).Error
	return pins, err
}

// SetPin idempotently saves or removes a personal pin.
func (r *DockChatRepository) SetPin(ctx context.Context, pin model.DockPin, pinned bool) error {
	if !pinned {
		return r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ? AND kind = ? AND resource_id = ?", pin.WorkspaceID, pin.UserID, pin.Kind, pin.ResourceID).Delete(&model.DockPin{}).Error
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&pin).Error
}
