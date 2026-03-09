package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMCompanyRepository handles DB operations for CRM companies.
type CRMCompanyRepository struct {
	db *gorm.DB
}

// NewCRMCompanyRepository creates a new CRMCompanyRepository.
func NewCRMCompanyRepository(db *gorm.DB) *CRMCompanyRepository {
	return &CRMCompanyRepository{db: db}
}

// GetNextDisplayID generates the next sequential display ID for companies in a workspace.
func (r *CRMCompanyRepository) GetNextDisplayID(ctx context.Context, workspaceID string) (string, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMCompany{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return "", fmt.Errorf("count companies: %w", err)
	}
	return fmt.Sprintf("COM-%d", count+1), nil
}

// List returns companies in a workspace with optional filters.
func (r *CRMCompanyRepository) List(ctx context.Context, workspaceID string, filters model.CRMCompanyListFilters, pagination model.PMPagination) ([]model.CRMCompany, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMCompany{}).Where("workspace_id = ?", workspaceID)

	if filters.Industry != nil && *filters.Industry != "" {
		query = query.Where("industry = ?", *filters.Industry)
	}
	if filters.OwnerMemberID != nil && *filters.OwnerMemberID != "" {
		query = query.Where("owner_member_id = ?", *filters.OwnerMemberID)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + *filters.Search + "%"
		query = query.Where("(name ILIKE ? OR domain ILIKE ?)", search, search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count companies: %w", err)
	}

	var companies []model.CRMCompany
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&companies).Error; err != nil {
		return nil, 0, fmt.Errorf("list companies: %w", err)
	}
	return companies, total, nil
}

// GetByID returns a company by ID.
func (r *CRMCompanyRepository) GetByID(ctx context.Context, id string) (*model.CRMCompany, error) {
	var company model.CRMCompany
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&company).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get company: %w", err)
	}
	return &company, nil
}

// Create inserts a company.
func (r *CRMCompanyRepository) Create(ctx context.Context, company *model.CRMCompany) error {
	if err := r.db.WithContext(ctx).Create(company).Error; err != nil {
		return fmt.Errorf("create company: %w", err)
	}
	return nil
}

// Update updates a company.
func (r *CRMCompanyRepository) Update(ctx context.Context, company *model.CRMCompany) error {
	if err := r.db.WithContext(ctx).Save(company).Error; err != nil {
		return fmt.Errorf("update company: %w", err)
	}
	return nil
}

// Delete removes a company.
func (r *CRMCompanyRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMCompany{}).Error; err != nil {
		return fmt.Errorf("delete company: %w", err)
	}
	return nil
}
