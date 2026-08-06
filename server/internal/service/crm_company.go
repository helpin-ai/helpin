package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMCompanyService contains CRM company business logic.
type CRMCompanyService struct {
	companyRepo *repository.CRMCompanyRepository
}

// NewCRMCompanyService creates a new CRMCompanyService.
func NewCRMCompanyService(companyRepo *repository.CRMCompanyRepository) *CRMCompanyService {
	return &CRMCompanyService{companyRepo: companyRepo}
}

// List returns companies with filters and pagination.
func (s *CRMCompanyService) List(ctx context.Context, workspaceID string, filters model.CRMCompanyListFilters, pagination model.PMPagination) ([]model.CRMCompany, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.companyRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a company by ID.
func (s *CRMCompanyService) GetByID(ctx context.Context, id string) (*model.CRMCompany, error) {
	company, err := s.companyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if company == nil {
		return nil, fmt.Errorf("company not found")
	}
	return company, nil
}

// Create creates a company.
func (s *CRMCompanyService) Create(ctx context.Context, req model.CreateCRMCompanyRequest) (*model.CRMCompany, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}

	displayID, err := s.companyRepo.GetNextDisplayID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}

	company := &model.CRMCompany{
		WorkspaceID:      req.WorkspaceID,
		DisplayID:        displayID,
		ExternalID:       req.ExternalID,
		Name:             strings.TrimSpace(req.Name),
		Domain:           req.Domain,
		Industry:         req.Industry,
		EmployeeCount:    req.EmployeeCount,
		AnnualRevenue:    req.AnnualRevenue,
		Description:      req.Description,
		LogoURL:          req.LogoURL,
		OwnerMemberID:    req.OwnerMemberID,
		CustomProperties: model.JSONB(req.CustomProperties),
	}

	if err := s.companyRepo.Create(ctx, company); err != nil {
		return nil, err
	}
	return company, nil
}

// Update updates a company.
func (s *CRMCompanyService) Update(ctx context.Context, id string, req model.UpdateCRMCompanyRequest) (*model.CRMCompany, error) {
	company, err := s.companyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if company == nil {
		return nil, fmt.Errorf("company not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		company.Name = name
	}
	if req.ExternalID != nil {
		company.ExternalID = req.ExternalID
	}
	if req.Domain != nil {
		company.Domain = req.Domain
	}
	if req.Industry != nil {
		company.Industry = req.Industry
	}
	if req.EmployeeCount != nil {
		company.EmployeeCount = req.EmployeeCount
	}
	if req.AnnualRevenue != nil {
		company.AnnualRevenue = req.AnnualRevenue
	}
	if req.Description != nil {
		company.Description = req.Description
	}
	if req.LogoURL != nil {
		company.LogoURL = req.LogoURL
	}
	if req.ClearOwner {
		company.OwnerMemberID = nil
	} else if req.OwnerMemberID != nil {
		company.OwnerMemberID = req.OwnerMemberID
	}
	if req.CustomProperties != nil {
		company.CustomProperties = model.JSONB(req.CustomProperties)
	}

	if err := s.companyRepo.Update(ctx, company); err != nil {
		return nil, err
	}
	return company, nil
}

// Delete removes a company.
func (s *CRMCompanyService) Delete(ctx context.Context, id string) error {
	company, err := s.companyRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if company == nil {
		return fmt.Errorf("company not found")
	}
	return s.companyRepo.Delete(ctx, id)
}
