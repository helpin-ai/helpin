package repository

import (
	"context"
	"errors"
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
	var existing model.CRMAssociation
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND from_object_type = ? AND from_object_id = ? AND to_object_type = ? AND to_object_id = ?",
			assoc.WorkspaceID, assoc.FromObjectType, assoc.FromObjectID, assoc.ToObjectType, assoc.ToObjectID).
		First(&existing).Error
	switch {
	case err == nil:
		assoc.ID = existing.ID
		assoc.CreatedAt = existing.CreatedAt
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("check association: %w", err)
	}

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

// ListByObjectEnriched returns associations with linked object names for a given object.
func (r *CRMAssociationRepository) ListByObjectEnriched(ctx context.Context, workspaceID, objectType, objectID string) ([]model.CRMAssociationEnriched, error) {
	assocs, err := r.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return nil, err
	}
	if len(assocs) == 0 {
		return []model.CRMAssociationEnriched{}, nil
	}

	// Collect IDs by type (the "other" side of each association)
	idsByType := map[string][]string{}
	for _, a := range assocs {
		ot, oid := a.ToObjectType, a.ToObjectID
		if a.ToObjectType == objectType && a.ToObjectID == objectID {
			ot, oid = a.FromObjectType, a.FromObjectID
		}
		idsByType[ot] = append(idsByType[ot], oid)
	}

	type nameRow struct {
		ID        string
		Name      string
		DisplayID string
	}
	nameMap := map[string]nameRow{} // keyed by id

	for objType, ids := range idsByType {
		var rows []nameRow
		switch objType {
		case model.CRMObjectContact:
			r.db.WithContext(ctx).Raw("SELECT id, first_name AS name, display_id FROM crm_contacts WHERE id IN (?)", ids).Scan(&rows)
		case model.CRMObjectCompany:
			r.db.WithContext(ctx).Raw("SELECT id, name, display_id FROM crm_companies WHERE id IN (?)", ids).Scan(&rows)
		case model.CRMObjectDeal:
			r.db.WithContext(ctx).Raw("SELECT id, name, display_id FROM crm_deals WHERE id IN (?)", ids).Scan(&rows)
		case model.CRMObjectEpic:
			r.db.WithContext(ctx).Raw("SELECT id, name, '' AS display_id FROM pm_epics WHERE id IN (?)", ids).Scan(&rows)
		case model.CRMObjectStory:
			r.db.WithContext(ctx).Raw("SELECT id, name, CAST(display_id AS TEXT) AS display_id FROM pm_stories WHERE id IN (?)", ids).Scan(&rows)
		case model.CRMObjectSupportTicket:
			r.db.WithContext(ctx).Raw("SELECT id, subject AS name, CAST(display_id AS TEXT) AS display_id FROM support_tickets WHERE id IN (?)", ids).Scan(&rows)
		}
		for _, row := range rows {
			nameMap[row.ID] = row
		}
	}

	enriched := make([]model.CRMAssociationEnriched, len(assocs))
	for i, a := range assocs {
		linkedID := a.ToObjectID
		if a.ToObjectType == objectType && a.ToObjectID == objectID {
			linkedID = a.FromObjectID
		}
		row := nameMap[linkedID]
		enriched[i] = model.CRMAssociationEnriched{
			CRMAssociation:        a,
			LinkedObjectName:      row.Name,
			LinkedObjectDisplayID: row.DisplayID,
		}
	}
	return enriched, nil
}

// GetByID returns an association by ID.
func (r *CRMAssociationRepository) GetByID(ctx context.Context, id string) (*model.CRMAssociation, error) {
	var assoc model.CRMAssociation
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&assoc).Error; err != nil {
		return nil, fmt.Errorf("get association: %w", err)
	}
	return &assoc, nil
}
