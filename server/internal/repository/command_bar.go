package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type CommandBarUnmetIntentRepository struct {
	db *gorm.DB
}

func NewCommandBarUnmetIntentRepository(db *gorm.DB) *CommandBarUnmetIntentRepository {
	return &CommandBarUnmetIntentRepository{db: db}
}

func (r *CommandBarUnmetIntentRepository) Create(ctx context.Context, intent *model.CommandBarUnmetIntent) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("command bar unmet intent repository is not configured")
	}
	if err := r.db.WithContext(ctx).Create(intent).Error; err != nil {
		return fmt.Errorf("create command bar unmet intent: %w", err)
	}
	return nil
}
