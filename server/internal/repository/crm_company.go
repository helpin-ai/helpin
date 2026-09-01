package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// DB returns the underlying database for coordinated transactions.
func (r *CRMCompanyRepository) DB() *gorm.DB { return r.db }

// WithTx returns a new CRMCompanyRepository using the given transaction.
func (r *CRMCompanyRepository) WithTx(tx *gorm.DB) *CRMCompanyRepository {
	return &CRMCompanyRepository{db: tx}
}

// LockExternalID serializes workspace-scoped external company identity upserts.
// Callers must invoke it inside the transaction that performs the lookup/create.
func (r *CRMCompanyRepository) LockExternalID(ctx context.Context, workspaceID, externalID string) error {
	if r.db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := lockCRMCompanyExternalID(r.db, ctx, workspaceID, externalID).Error; err != nil {
		return fmt.Errorf("lock company external id: %w", err)
	}
	return nil
}

func lockCRMCompanyExternalID(db *gorm.DB, ctx context.Context, workspaceID, externalID string) *gorm.DB {
	normalizedExternalID := strings.ToLower(strings.TrimSpace(externalID))
	// Bind both lock dimensions separately because PostgreSQL text parameters
	// reject NUL-delimited keys.
	return db.WithContext(ctx).Exec(
		"SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))",
		workspaceID,
		normalizedExternalID,
	)
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

// ListContacts returns contacts directly associated with a company.
func (r *CRMCompanyRepository) ListContacts(ctx context.Context, workspaceID, companyID, search string, pagination model.PMPagination) ([]model.CRMContact, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMContact{}).
		Where("crm_contacts.workspace_id = ?", workspaceID).
		Where(`EXISTS (
			SELECT 1 FROM crm_associations ca
			WHERE ca.workspace_id = crm_contacts.workspace_id
			  AND ((ca.from_object_type = 'contact' AND ca.from_object_id = crm_contacts.id AND ca.to_object_type = 'company' AND ca.to_object_id = ?)
			    OR (ca.to_object_type = 'contact' AND ca.to_object_id = crm_contacts.id AND ca.from_object_type = 'company' AND ca.from_object_id = ?))
		)`, companyID, companyID)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		like := "%" + strings.ToLower(trimmed) + "%"
		query = query.Where("LOWER(CONCAT_WS(' ', first_name, last_name, email, job_title)) LIKE ?", like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count company contacts: %w", err)
	}
	page, perPage := normalizeCompanyPagination(pagination)
	var contacts []model.CRMContact
	if err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&contacts).Error; err != nil {
		return nil, 0, fmt.Errorf("list company contacts: %w", err)
	}
	return contacts, total, nil
}

// ListDeals returns deals for which this company is the canonical customer.
func (r *CRMCompanyRepository) ListDeals(ctx context.Context, workspaceID, companyID, search string, pagination model.PMPagination) ([]model.CRMDeal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMDeal{}).
		Where("crm_deals.workspace_id = ?", workspaceID).
		Where(`EXISTS (
			SELECT 1 FROM crm_associations ca
			WHERE ca.workspace_id = crm_deals.workspace_id
			  AND ca.from_object_type = 'deal'
			  AND ca.from_object_id = crm_deals.id
			  AND ca.to_object_type = 'company'
			  AND ca.to_object_id = ?
			  AND ca.association_label = 'deal_customer'
		)`, companyID)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		query = query.Where("LOWER(crm_deals.name) LIKE ?", "%"+strings.ToLower(trimmed)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count company deals: %w", err)
	}
	page, perPage := normalizeCompanyPagination(pagination)
	var deals []model.CRMDeal
	if err := query.Preload("Pipeline").Preload("Stage").Order("crm_deals.updated_at DESC, crm_deals.id DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&deals).Error; err != nil {
		return nil, 0, fmt.Errorf("list company deals: %w", err)
	}
	return deals, total, nil
}

func normalizeCompanyPagination(pagination model.PMPagination) (int, int) {
	page, perPage := pagination.Page, pagination.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 25
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
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
	if r.db.Migrator().HasTable(&model.CRMCompanyCommercialStateHealth{}) {
		var health model.CRMCompanyCommercialStateHealth
		err := r.db.WithContext(ctx).Where("company_id = ?", id).First(&health).Error
		if err == nil {
			company.CommercialStateHealth = &health
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get company commercial state health: %w", err)
		}
	}
	return &company, nil
}

// GetByExternalID returns the first company in a workspace with the given external account ID.
func (r *CRMCompanyRepository) GetByExternalID(ctx context.Context, workspaceID, externalID string) (*model.CRMCompany, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}

	var company model.CRMCompany
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND external_id = ?", workspaceID, externalID).
		Order("created_at ASC, id ASC").
		First(&company).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get company by external id: %w", err)
	}
	return &company, nil
}

// GetByDomain returns the first company in a workspace with the given domain.
func (r *CRMCompanyRepository) GetByDomain(ctx context.Context, workspaceID, domain string) (*model.CRMCompany, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil, nil
	}

	var company model.CRMCompany
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND domain IS NOT NULL AND LOWER(domain) = ?", workspaceID, domain).
		Order("created_at ASC, id ASC").
		First(&company).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get company by domain: %w", err)
	}
	return &company, nil
}

// GetByName returns the first company in a workspace with the given name.
func (r *CRMCompanyRepository) GetByName(ctx context.Context, workspaceID, name string) (*model.CRMCompany, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return nil, nil
	}

	var company model.CRMCompany
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(name) = ?", workspaceID, name).
		Order("created_at ASC, id ASC").
		First(&company).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get company by name: %w", err)
	}
	return &company, nil
}

// Create inserts a company.
func (r *CRMCompanyRepository) Create(ctx context.Context, company *model.CRMCompany) error {
	query := r.db.WithContext(ctx)
	if !r.db.Migrator().HasColumn(&model.CRMCompany{}, "customer_success_owner_member_id") {
		query = query.Omit("customer_success_owner_member_id")
	}
	if err := query.Create(company).Error; err != nil {
		return fmt.Errorf("create company: %w", err)
	}
	return nil
}

// Update updates a company.
func (r *CRMCompanyRepository) Update(ctx context.Context, company *model.CRMCompany) error {
	query := r.db.WithContext(ctx)
	if !r.db.Migrator().HasColumn(&model.CRMCompany{}, "customer_success_owner_member_id") {
		query = query.Omit("customer_success_owner_member_id")
	}
	if err := query.Save(company).Error; err != nil {
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
