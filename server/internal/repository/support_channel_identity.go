package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UpsertSupportChannelIdentityTx records a customer of a connected account,
// refreshing the profile snapshot they arrive with, and returns the stored row.
func UpsertSupportChannelIdentityTx(ctx context.Context, tx *gorm.DB, identity *model.SupportChannelIdentity, at time.Time) (*model.SupportChannelIdentity, error) {
	if identity.ID == "" {
		identity.ID = uuid.NewString()
	}
	identity.LastInboundAt = &at
	identity.CreatedAt, identity.UpdatedAt = at, at
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_account_id"}, {Name: "external_user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"display_name", "username", "last_inbound_at", "updated_at"}),
	}).Create(identity).Error; err != nil {
		return nil, err
	}
	var stored model.SupportChannelIdentity
	if err := tx.WithContext(ctx).Where("channel_account_id = ? AND external_user_id = ?", identity.ChannelAccountID, identity.ExternalUserID).
		First(&stored).Error; err != nil {
		return nil, err
	}
	return &stored, nil
}
