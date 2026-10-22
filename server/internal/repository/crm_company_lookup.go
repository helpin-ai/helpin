package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListByIDs returns companies by ID within one workspace.
func (r *CRMCompanyRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.CRMCompany, error) {
	if len(ids) == 0 {
		return []model.CRMCompany{}, nil
	}
	var companies []model.CRMCompany
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("list companies by ids: %w", err)
	}
	return companies, nil
}

// GetByDomainVariants returns the oldest workspace company whose stored domain
// matches any of the supplied spellings, compared case-insensitively.
func (r *CRMCompanyRepository) GetByDomainVariants(ctx context.Context, workspaceID string, variants []string) (*model.CRMCompany, error) {
	normalized := make([]string, 0, len(variants))
	for _, variant := range variants {
		if value := strings.ToLower(strings.TrimSpace(variant)); value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	var company model.CRMCompany
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND domain IS NOT NULL AND LOWER(domain) IN ?", workspaceID, normalized).
		Order("created_at ASC, id ASC").
		First(&company).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get company by domain variants: %w", err)
	}
	return &company, nil
}

// ListFilterSources returns the industry and custom properties of up to limit
// of the workspace's most recently updated companies.
func (r *CRMCompanyRepository) ListFilterSources(ctx context.Context, workspaceID string, limit int) ([]model.CRMCompanyFilterSource, error) {
	var sources []model.CRMCompanyFilterSource
	if err := r.db.WithContext(ctx).
		Model(&model.CRMCompany{}).
		Select("industry", "custom_properties").
		Where("workspace_id = ?", workspaceID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&sources).Error; err != nil {
		return nil, fmt.Errorf("list company filter sources: %w", err)
	}
	return sources, nil
}
