package repository

import (
	"context"
	"fmt"

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
