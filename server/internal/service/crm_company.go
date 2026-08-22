package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMCompanyService contains CRM company business logic.
type CRMCompanyService struct {
	companyRepo  *repository.CRMCompanyRepository
	timelineRepo *repository.CRMCompanyTimelineRepository
	productAnalyticsEmitter
}

// NewCRMCompanyService creates a new CRMCompanyService.
func NewCRMCompanyService(companyRepo *repository.CRMCompanyRepository) *CRMCompanyService {
	return &CRMCompanyService{companyRepo: companyRepo}
}

// SetTimelineRepository enables the unified company timeline read model.
func (s *CRMCompanyService) SetTimelineRepository(repo *repository.CRMCompanyTimelineRepository) *CRMCompanyService {
	s.timelineRepo = repo
	return s
}

type crmTimelineCursor struct {
	Version int       `json:"v"`
	At      time.Time `json:"at"`
	ID      string    `json:"id"`
}

// ListTimeline returns one cursor-paginated page of the unified company timeline.
func (s *CRMCompanyService) ListTimeline(
	ctx context.Context,
	workspaceID, companyID, filter, cursor string,
	limit int,
) (*model.CRMCompanyTimelinePage, error) {
	if workspaceID == "" || companyID == "" {
		return nil, fmt.Errorf("workspace_id and company_id are required")
	}
	if s.timelineRepo == nil {
		return nil, fmt.Errorf("company timeline is not configured")
	}
	company, err := s.GetByID(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if company.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("company not found")
	}
	filter = strings.TrimSpace(filter)
	if filter == "" {
		filter = model.CRMCompanyTimelineFilterAll
	}
	if !validCRMTimelineFilter(filter) {
		return nil, fmt.Errorf("invalid timeline filter")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	query := model.CRMCompanyTimelineQuery{Filter: filter, Limit: limit + 1}
	if cursor != "" {
		decoded, err := decodeCRMTimelineCursor(cursor)
		if err != nil {
			return nil, err
		}
		query.CursorAt = &decoded.At
		query.CursorID = decoded.ID
	}
	items, err := s.timelineRepo.List(ctx, workspaceID, companyID, query)
	if err != nil {
		return nil, err
	}
	page := &model.CRMCompanyTimelinePage{Data: items}
	if len(items) > limit {
		page.Data = items[:limit]
		last := page.Data[len(page.Data)-1]
		next, err := encodeCRMTimelineCursor(crmTimelineCursor{Version: 1, At: last.OccurredAt, ID: last.ID})
		if err != nil {
			return nil, err
		}
		page.NextCursor = &next
	}
	return page, nil
}

func validCRMTimelineFilter(filter string) bool {
	switch filter {
	case model.CRMCompanyTimelineFilterAll,
		model.CRMCompanyTimelineFilterNote,
		model.CRMCompanyTimelineFilterEmail,
		model.CRMCompanyTimelineFilterCall,
		model.CRMCompanyTimelineFilterMeeting,
		model.CRMCompanyTimelineFilterTask,
		model.CRMCompanyTimelineFilterDeal,
		model.CRMCompanyTimelineFilterSupport:
		return true
	default:
		return false
	}
}

func encodeCRMTimelineCursor(cursor crmTimelineCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode CRM timeline cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCRMTimelineCursor(value string) (*crmTimelineCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid timeline cursor")
	}
	var cursor crmTimelineCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Version != 1 || cursor.At.IsZero() || cursor.ID == "" {
		return nil, fmt.Errorf("invalid timeline cursor")
	}
	return &cursor, nil
}

// List returns companies with filters and pagination.
func (s *CRMCompanyService) List(ctx context.Context, workspaceID string, filters model.CRMCompanyListFilters, pagination model.PMPagination) ([]model.CRMCompany, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.companyRepo.List(ctx, workspaceID, filters, pagination)
}

func (s *CRMCompanyService) ListContacts(ctx context.Context, workspaceID, companyID, search string, pagination model.PMPagination) ([]model.CRMContact, int64, error) {
	if err := s.requireCompanyWorkspace(ctx, workspaceID, companyID); err != nil {
		return nil, 0, err
	}
	return s.companyRepo.ListContacts(ctx, workspaceID, companyID, search, pagination)
}

func (s *CRMCompanyService) ListDeals(ctx context.Context, workspaceID, companyID, search string, pagination model.PMPagination) ([]model.CRMDeal, int64, error) {
	if err := s.requireCompanyWorkspace(ctx, workspaceID, companyID); err != nil {
		return nil, 0, err
	}
	return s.companyRepo.ListDeals(ctx, workspaceID, companyID, search, pagination)
}

func (s *CRMCompanyService) requireCompanyWorkspace(ctx context.Context, workspaceID, companyID string) error {
	if workspaceID == "" || companyID == "" {
		return fmt.Errorf("workspace_id and company_id are required")
	}
	company, err := s.companyRepo.GetByID(ctx, companyID)
	if err != nil {
		return err
	}
	if company == nil || company.WorkspaceID != workspaceID {
		return fmt.Errorf("company not found")
	}
	return nil
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
		LinkedInURL:      req.LinkedInURL,
		Headquarters:     req.Headquarters,
		OwnerMemberID:    req.OwnerMemberID,
		CustomProperties: model.JSONB(req.CustomProperties),
	}

	if err := s.companyRepo.Create(ctx, company); err != nil {
		return nil, err
	}
	s.trackProductEvent(ctx, ProductAnalyticsEvent{
		SemanticKey: "crm_company_created:" + company.ID,
		WorkspaceID: company.WorkspaceID, Name: "crm_company_created", Source: "api",
		OccurredAt: company.CreatedAt,
		Attributes: map[string]any{"entity_id": company.ID, "industry": company.Industry, "module": "crm"},
	})
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
	if req.LinkedInURL != nil {
		company.LinkedInURL = req.LinkedInURL
	}
	if req.Headquarters != nil {
		company.Headquarters = req.Headquarters
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
