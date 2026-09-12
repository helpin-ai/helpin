package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMDealRepository handles DB operations for CRM deals and pipelines.
type CRMDealRepository struct {
	db *gorm.DB
}

// NewCRMDealRepository creates a new CRMDealRepository.
func NewCRMDealRepository(db *gorm.DB) *CRMDealRepository {
	return &CRMDealRepository{db: db}
}

// GetStage returns a pipeline stage by ID.
func (r *CRMDealRepository) GetStage(ctx context.Context, id string) (*model.CRMPipelineStage, error) {
	var stage model.CRMPipelineStage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&stage).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get stage: %w", err)
	}
	return &stage, nil
}

// CountDealsByPipeline returns the number of deals in a pipeline.
func (r *CRMDealRepository) CountDealsByPipeline(ctx context.Context, pipelineID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("pipeline_id = ?", pipelineID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count deals by pipeline: %w", err)
	}
	return count, nil
}

// ── Deal operations ──

// GetNextDisplayID generates the next sequential display ID for deals in a workspace.
func (r *CRMDealRepository) GetNextDisplayID(ctx context.Context, workspaceID string) (string, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return "", fmt.Errorf("count deals: %w", err)
	}
	return fmt.Sprintf("DEAL-%d", count+1), nil
}

// List returns deals in a workspace with optional filters.
func (r *CRMDealRepository) List(ctx context.Context, workspaceID string, filters model.CRMDealListFilters, pagination model.PMPagination) ([]model.CRMDeal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("crm_deals.workspace_id = ?", workspaceID)

	if filters.PipelineID != nil && *filters.PipelineID != "" {
		query = query.Where("pipeline_id = ?", *filters.PipelineID)
	}
	if filters.StageID != nil && *filters.StageID != "" {
		query = query.Where("stage_id = ?", *filters.StageID)
	}
	if filters.OwnerMemberID != nil && *filters.OwnerMemberID != "" {
		query = query.Where("owner_member_id = ?", *filters.OwnerMemberID)
	}
	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where(`EXISTS (
			SELECT 1 FROM crm_associations ca
			WHERE ca.workspace_id = crm_deals.workspace_id
			  AND ((ca.from_object_type = 'deal' AND ca.from_object_id = crm_deals.id AND ca.to_object_type = 'contact' AND ca.to_object_id = ?)
			    OR (ca.to_object_type = 'deal' AND ca.to_object_id = crm_deals.id AND ca.from_object_type = 'contact' AND ca.from_object_id = ?))
		)`, *filters.ContactID, *filters.ContactID)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(*filters.Search)) + "%"
		query = query.Where("LOWER(name) LIKE ?", search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count deals: %w", err)
	}

	var deals []model.CRMDeal
	offset := (pagination.Page - 1) * pagination.PerPage
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}
	if err := query.
		Preload("Pipeline").
		Preload("Stage").
		Order("created_at DESC").
		Offset(offset).Limit(pagination.PerPage).
		Find(&deals).Error; err != nil {
		return nil, 0, fmt.Errorf("list deals: %w", err)
	}
	return deals, total, nil
}

// ListByIDs returns deals by ID for a workspace.
func (r *CRMDealRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.CRMDeal, error) {
	if len(ids) == 0 {
		return []model.CRMDeal{}, nil
	}
	var deals []model.CRMDeal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&deals).Error; err != nil {
		return nil, fmt.Errorf("list deals by ids: %w", err)
	}
	return deals, nil
}

// GetByID returns a deal by ID with pipeline and stage.
func (r *CRMDealRepository) GetByID(ctx context.Context, id string) (*model.CRMDeal, error) {
	var deal model.CRMDeal
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Preload("Pipeline").
		Preload("Stage").
		First(&deal).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get deal: %w", err)
	}
	return &deal, nil
}

// Create inserts a deal.
func (r *CRMDealRepository) Create(ctx context.Context, deal *model.CRMDeal) error {
	if err := r.db.WithContext(ctx).Create(deal).Error; err != nil {
		return fmt.Errorf("create deal: %w", err)
	}
	return nil
}

// ObjectExists reports whether a supported CRM customer object belongs to the workspace.
func (r *CRMDealRepository) ObjectExists(ctx context.Context, workspaceID, objectType, objectID string) (bool, error) {
	var table string
	switch objectType {
	case model.CRMObjectContact:
		table = "crm_contacts"
	case model.CRMObjectCompany:
		table = "crm_companies"
	default:
		return false, fmt.Errorf("unsupported customer object type")
	}
	var count int64
	if err := r.db.WithContext(ctx).Table(table).
		Where("workspace_id = ? AND id = ?", workspaceID, objectID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check %s: %w", objectType, err)
	}
	return count > 0, nil
}

// GetPrimaryCompanyIDForContact returns the contact's explicitly primary company.
func (r *CRMDealRepository) GetPrimaryCompanyIDForContact(ctx context.Context, workspaceID, contactID string) (string, error) {
	var row struct{ CompanyID string }
	err := r.db.WithContext(ctx).Raw(`
		SELECT CASE
			WHEN from_object_type = 'company' THEN from_object_id
			ELSE to_object_id
		END AS company_id
		FROM crm_associations
		WHERE workspace_id = ? AND association_label = 'primary'
		  AND ((from_object_type = 'contact' AND from_object_id = ? AND to_object_type = 'company')
		    OR (to_object_type = 'contact' AND to_object_id = ? AND from_object_type = 'company'))
		ORDER BY created_at ASC, id ASC
		LIMIT 1`, workspaceID, contactID, contactID).Scan(&row).Error
	if err != nil {
		return "", fmt.Errorf("get contact primary company: %w", err)
	}
	return row.CompanyID, nil
}

// CreateWithCustomer creates a deal and its canonical customer relationships atomically.
func (r *CRMDealRepository) CreateWithCustomer(ctx context.Context, deal *model.CRMDeal, customer model.CRMDealCustomer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(deal).Error; err != nil {
			return fmt.Errorf("create deal: %w", err)
		}
		if err := ensureDealAssociation(tx, deal.WorkspaceID, deal.ID, customer.CustomerType, customer.CustomerID, model.CRMAssociationLabelDealCustomer); err != nil {
			return err
		}
		if customer.CustomerType == model.CRMObjectCompany && customer.PrimaryContactID != "" {
			if err := ensureDealAssociation(tx, deal.WorkspaceID, deal.ID, model.CRMObjectContact, customer.PrimaryContactID, model.CRMAssociationLabelDealPrimaryContact); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceCustomer atomically assigns the one canonical customer and optional primary contact.
func (r *CRMDealRepository) ReplaceCustomer(ctx context.Context, workspaceID, dealID string, customer model.CRMDealCustomer) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.CRMDeal{}).Where("workspace_id = ? AND id = ?", workspaceID, dealID).Count(&count).Error; err != nil {
			return fmt.Errorf("check deal: %w", err)
		}
		if count == 0 {
			return fmt.Errorf("deal not found")
		}
		if err := tx.Model(&model.CRMAssociation{}).
			Where("workspace_id = ? AND from_object_type = ? AND from_object_id = ? AND association_label IN ?", workspaceID, model.CRMObjectDeal, dealID, []string{model.CRMAssociationLabelDealCustomer, model.CRMAssociationLabelDealPrimaryContact}).
			Update("association_label", nil).Error; err != nil {
			return fmt.Errorf("clear deal customer labels: %w", err)
		}
		if err := ensureDealAssociation(tx, workspaceID, dealID, customer.CustomerType, customer.CustomerID, model.CRMAssociationLabelDealCustomer); err != nil {
			return err
		}
		if customer.CustomerType == model.CRMObjectCompany && customer.PrimaryContactID != "" {
			if err := ensureDealAssociation(tx, workspaceID, dealID, model.CRMObjectContact, customer.PrimaryContactID, model.CRMAssociationLabelDealPrimaryContact); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetCustomer returns the canonical customer relationships, if the deal has been resolved.
func (r *CRMDealRepository) GetCustomer(ctx context.Context, workspaceID, dealID string) (*model.CRMDealCustomer, error) {
	var associations []model.CRMAssociation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND from_object_type = ? AND from_object_id = ? AND association_label IN ?", workspaceID, model.CRMObjectDeal, dealID, []string{model.CRMAssociationLabelDealCustomer, model.CRMAssociationLabelDealPrimaryContact}).
		Order("created_at ASC, id ASC").Find(&associations).Error; err != nil {
		return nil, fmt.Errorf("get deal customer: %w", err)
	}
	var customer model.CRMDealCustomer
	for _, assoc := range associations {
		if assoc.AssociationLabel == nil {
			continue
		}
		switch *assoc.AssociationLabel {
		case model.CRMAssociationLabelDealCustomer:
			customer.CustomerType, customer.CustomerID = assoc.ToObjectType, assoc.ToObjectID
		case model.CRMAssociationLabelDealPrimaryContact:
			customer.PrimaryContactID = assoc.ToObjectID
		}
	}
	if customer.CustomerID == "" {
		return nil, nil
	}
	if customer.CustomerType == model.CRMObjectContact {
		customer.PrimaryContactID = customer.CustomerID
	}
	return &customer, nil
}

func ensureDealAssociation(tx *gorm.DB, workspaceID, dealID, targetType, targetID, label string) error {
	var assoc model.CRMAssociation
	err := tx.Where("workspace_id = ? AND from_object_type = ? AND from_object_id = ? AND to_object_type = ? AND to_object_id = ?", workspaceID, model.CRMObjectDeal, dealID, targetType, targetID).First(&assoc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		assoc = model.CRMAssociation{WorkspaceID: workspaceID, FromObjectType: model.CRMObjectDeal, FromObjectID: dealID, ToObjectType: targetType, ToObjectID: targetID, AssociationLabel: &label}
		if err := tx.Create(&assoc).Error; err != nil {
			return fmt.Errorf("create deal association: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("find deal association: %w", err)
	}
	if err := tx.Model(&assoc).Update("association_label", label).Error; err != nil {
		return fmt.Errorf("label deal association: %w", err)
	}
	return nil
}

// Update updates a deal.
func (r *CRMDealRepository) Update(ctx context.Context, deal *model.CRMDeal) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, deal.WorkspaceID); err != nil {
			return err
		}
		var previous model.CRMDeal
		if err := tx.Where("workspace_id = ? AND id = ?", deal.WorkspaceID, deal.ID).Take(&previous).Error; err != nil {
			return err
		}
		if err := tx.Save(deal).Error; err != nil {
			return fmt.Errorf("update deal: %w", err)
		}
		return enterWonDeal(tx, *deal, previous.StageID, deal.UpdatedAt)
	})
}

// Delete removes a deal.
func (r *CRMDealRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("(from_object_type = ? AND from_object_id = ?) OR (to_object_type = ? AND to_object_id = ?)", model.CRMObjectDeal, id, model.CRMObjectDeal, id).Delete(&model.CRMAssociation{}).Error; err != nil {
			return fmt.Errorf("delete deal associations: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&model.CRMDeal{}).Error; err != nil {
			return fmt.Errorf("delete deal: %w", err)
		}
		return nil
	})
}

// SeedDefaultPipeline creates a default "Sales Pipeline" with simple sales stages
// if the workspace has no pipelines yet.
func (r *CRMDealRepository) SeedDefaultPipeline(ctx context.Context, workspaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPipelineWorkspace(tx, workspaceID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.CRMPipeline{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
			return fmt.Errorf("count pipelines: %w", err)
		}
		if count > 0 {
			return nil
		}

		pipeline := &model.CRMPipeline{
			WorkspaceID:             workspaceID,
			Name:                    "Sales Pipeline",
			IsDefault:               true,
			DefaultCommercialMotion: model.CRMDealMotionNewBusiness,
			Stages: []model.CRMPipelineStage{
				{Name: "Lead", StageType: "open", Position: 0, Probability: 20},
				{Name: "In Discussion", StageType: "open", Position: 1, Probability: 50},
				{Name: "Proposal Sent", StageType: "open", Position: 2, Probability: 80},
				{Name: "Won", StageType: "won", Position: 3, Probability: 100},
				{Name: "Lost", StageType: "lost", Position: 4, Probability: 0},
			},
		}

		if err := tx.Create(pipeline).Error; err != nil {
			return fmt.Errorf("seed default pipeline: %w", err)
		}
		return nil
	})
}
