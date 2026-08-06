package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSearchResult represents a unified search result across CRM objects.
type CRMSearchResult struct {
	Type   string      `json:"type"` // contact, company, deal
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Detail string      `json:"detail"` // email for contacts, domain for companies, amount for deals
	Object interface{} `json:"object"`
}

// CRMSearchService provides unified search across CRM entities.
type CRMSearchService struct {
	contactRepo *repository.CRMContactRepository
	companyRepo *repository.CRMCompanyRepository
	dealRepo    *repository.CRMDealRepository
}

// NewCRMSearchService creates a new CRMSearchService.
func NewCRMSearchService(contactRepo *repository.CRMContactRepository, companyRepo *repository.CRMCompanyRepository, dealRepo *repository.CRMDealRepository) *CRMSearchService {
	return &CRMSearchService{
		contactRepo: contactRepo,
		companyRepo: companyRepo,
		dealRepo:    dealRepo,
	}
}

// Search performs a unified search across contacts, companies, and deals.
func (s *CRMSearchService) Search(ctx context.Context, workspaceID, query string) ([]CRMSearchResult, error) {
	return s.SearchLimit(ctx, workspaceID, query, 10)
}

// SearchLimit searches each CRM entity group with a bounded per-group limit.
func (s *CRMSearchService) SearchLimit(ctx context.Context, workspaceID, query string, limit int) ([]CRMSearchResult, error) {
	if workspaceID == "" || query == "" {
		return nil, fmt.Errorf("workspace_id and query are required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 600 {
		limit = 600
	}

	var results []CRMSearchResult
	pagination := model.PMPagination{Page: 1, PerPage: limit}

	// Search contacts.
	contacts, _, err := s.contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{
		Search: &query,
	}, pagination)
	if err == nil {
		for _, c := range contacts {
			detail := ""
			if c.Email != nil {
				detail = *c.Email
			}
			name := c.FirstName
			if c.LastName != nil {
				name += " " + *c.LastName
			}
			results = append(results, CRMSearchResult{
				Type:   "contact",
				ID:     c.ID,
				Name:   name,
				Detail: detail,
				Object: c,
			})
		}
	}

	// Search companies.
	companies, _, err := s.companyRepo.List(ctx, workspaceID, model.CRMCompanyListFilters{
		Search: &query,
	}, pagination)
	if err == nil {
		for _, co := range companies {
			detail := ""
			if co.Domain != nil {
				detail = *co.Domain
			}
			results = append(results, CRMSearchResult{
				Type:   "company",
				ID:     co.ID,
				Name:   co.Name,
				Detail: detail,
				Object: co,
			})
		}
	}

	// Search deals.
	deals, _, err := s.dealRepo.List(ctx, workspaceID, model.CRMDealListFilters{
		Search: &query,
	}, pagination)
	if err == nil {
		for _, d := range deals {
			detail := ""
			if d.Amount != nil {
				detail = fmt.Sprintf("%.2f %s", *d.Amount, d.Currency)
			}
			results = append(results, CRMSearchResult{
				Type:   "deal",
				ID:     d.ID,
				Name:   d.Name,
				Detail: detail,
				Object: d,
			})
		}
	}

	return results, nil
}
