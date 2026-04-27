package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type DocsDocumentKeyRepository struct {
	db *gorm.DB
}

func NewDocsDocumentKeyRepository(db *gorm.DB) *DocsDocumentKeyRepository {
	return &DocsDocumentKeyRepository{db: db}
}

func (r *DocsDocumentKeyRepository) GetByKey(ctx context.Context, workspaceID, keyType, key string) (*model.DocsDocumentKey, error) {
	var record model.DocsDocumentKey
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND key_type = ? AND key = ?", workspaceID, keyType, key).
		First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs document key: %w", err)
	}
	return &record, nil
}

func (r *DocsDocumentKeyRepository) Upsert(ctx context.Context, record *model.DocsDocumentKey) error {
	if record == nil {
		return fmt.Errorf("docs document key is required")
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}, {Name: "key_type"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"document_id",
			"updated_at",
		}),
	}).Create(record).Error; err != nil {
		return fmt.Errorf("upsert docs document key: %w", err)
	}
	return nil
}
