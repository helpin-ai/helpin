package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMAssociationRepository handles DB operations for CRM associations.
type CRMAssociationRepository struct {
	db *gorm.DB
}

// NewCRMAssociationRepository creates a new CRMAssociationRepository.
func NewCRMAssociationRepository(db *gorm.DB) *CRMAssociationRepository {
	return &CRMAssociationRepository{db: db}
}

// Create inserts an association.
func (r *CRMAssociationRepository) Create(ctx context.Context, assoc *model.CRMAssociation) error {
	if err := r.db.WithContext(ctx).Create(assoc).Error; err != nil {
		return fmt.Errorf("create association: %w", err)
	}
	return nil
}

// Delete removes an association by ID.
func (r *CRMAssociationRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMAssociation{}).Error; err != nil {
		return fmt.Errorf("delete association: %w", err)
	}
	return nil
}

// ListByObject returns all associations for a given object.
func (r *CRMAssociationRepository) ListByObject(ctx context.Context, workspaceID, objectType, objectID string) ([]model.CRMAssociation, error) {
	var assocs []model.CRMAssociation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND ((from_object_type = ? AND from_object_id = ?) OR (to_object_type = ? AND to_object_id = ?))",
			workspaceID, objectType, objectID, objectType, objectID).
		Order("created_at DESC").
		Find(&assocs).Error; err != nil {
		return nil, fmt.Errorf("list associations: %w", err)
	}
	return assocs, nil
}

// GetByID returns an association by ID.
func (r *CRMAssociationRepository) GetByID(ctx context.Context, id string) (*model.CRMAssociation, error) {
	var assoc model.CRMAssociation
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&assoc).Error; err != nil {
		return nil, fmt.Errorf("get association: %w", err)
	}
	return &assoc, nil
}
