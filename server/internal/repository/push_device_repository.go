package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PushDeviceRepository persists mobile push notification device registrations.
type PushDeviceRepository struct {
	db *gorm.DB
}

// NewPushDeviceRepository creates a new repository.
func NewPushDeviceRepository(db *gorm.DB) *PushDeviceRepository {
	return &PushDeviceRepository{db: db}
}

// UpsertByToken creates or updates a push device registration keyed by token.
// Re-registering an existing token reassigns the row to device.UserID, since
// phones change owners/accounts.
func (r *PushDeviceRepository) UpsertByToken(ctx context.Context, device *model.PushDevice) error {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token"}},
			UpdateAll: true,
		}).
		Create(device).Error; err != nil {
		return fmt.Errorf("upsert push device: %w", err)
	}
	return nil
}

// DeleteByToken removes a device registration. Scoped to userID so a user may
// only unregister their own device.
func (r *PushDeviceRepository) DeleteByToken(ctx context.Context, userID, token string) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND token = ?", userID, token).
		Delete(&model.PushDevice{}).Error; err != nil {
		return fmt.Errorf("delete push device: %w", err)
	}
	return nil
}

// ListByUserIDs returns all registered push devices for the given users.
func (r *PushDeviceRepository) ListByUserIDs(ctx context.Context, userIDs []string) ([]model.PushDevice, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var devices []model.PushDevice
	if err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("list push devices: %w", err)
	}
	return devices, nil
}
