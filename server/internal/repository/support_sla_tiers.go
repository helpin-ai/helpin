package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportSLATierNotFound means the tier does not exist in the workspace.
var ErrSupportSLATierNotFound = errors.New("SLA tier not found")

// SupportSLACompany is the company context SLA policies match on.
type SupportSLACompany struct {
	ID     string
	TierID *string
	// TierChangedAt is when the company's tier last changed.
	TierChangedAt *time.Time
}

// ListTiers returns the workspace's tiers in display order, with how many
// companies carry each.
func (r *SupportSLARepository) ListTiers(ctx context.Context, workspaceID string) ([]model.SupportSLATier, error) {
	var tiers []model.SupportSLATier
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("position, created_at, id").Find(&tiers).Error; err != nil {
		return nil, fmt.Errorf("list SLA tiers: %w", err)
	}
	var counts []struct {
		TierID string
		Count  int64
	}
	if err := r.db.WithContext(ctx).Table("crm_companies").
		Select("support_sla_tier_id AS tier_id, COUNT(*) AS count").
		Where("workspace_id = ? AND support_sla_tier_id IS NOT NULL", workspaceID).
		Group("support_sla_tier_id").Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("count companies per SLA tier: %w", err)
	}
	for i := range tiers {
		for _, count := range counts {
			if count.TierID == tiers[i].ID {
				tiers[i].CompanyCount = count.Count
			}
		}
	}
	return tiers, nil
}

// GetTier returns one tier.
func (r *SupportSLARepository) GetTier(ctx context.Context, workspaceID, tierID string) (*model.SupportSLATier, error) {
	var tier model.SupportSLATier
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, tierID).Take(&tier).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupportSLATierNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get SLA tier: %w", err)
	}
	return &tier, nil
}

// CreateTier appends a tier after the workspace's existing ones.
func (r *SupportSLARepository) CreateTier(ctx context.Context, tier *model.SupportSLATier) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var last struct{ Position *int }
		if err := tx.Model(&model.SupportSLATier{}).Select("MAX(position) AS position").
			Where("workspace_id = ?", tier.WorkspaceID).Scan(&last).Error; err != nil {
			return fmt.Errorf("find last SLA tier position: %w", err)
		}
		tier.Position = 0
		if last.Position != nil {
			tier.Position = *last.Position + 1
		}
		if tier.ID == "" {
			tier.ID = uuid.NewString()
		}
		if err := tx.Create(tier).Error; err != nil {
			return fmt.Errorf("create SLA tier: %w", err)
		}
		return nil
	})
}

// UpdateTier renames a tier and changes its description.
func (r *SupportSLARepository) UpdateTier(ctx context.Context, tier *model.SupportSLATier) error {
	result := r.db.WithContext(ctx).Model(&model.SupportSLATier{}).
		Where("workspace_id = ? AND id = ?", tier.WorkspaceID, tier.ID).
		Updates(map[string]any{"name": tier.Name, "description": tier.Description, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("update SLA tier: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSupportSLATierNotFound
	}
	return nil
}

// DeleteTier removes a tier, clears it from companies and from policy
// conditions, and returns the companies that carried it.
func (r *SupportSLARepository) DeleteTier(ctx context.Context, workspaceID, tierID string, at time.Time) ([]string, error) {
	var companyIDs []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("crm_companies").Where("workspace_id = ? AND support_sla_tier_id = ?", workspaceID, tierID).
			Pluck("id", &companyIDs).Error; err != nil {
			return fmt.Errorf("list companies of SLA tier: %w", err)
		}
		if len(companyIDs) > 0 {
			if err := tx.Table("crm_companies").Where("workspace_id = ? AND support_sla_tier_id = ?", workspaceID, tierID).
				Updates(map[string]any{"support_sla_tier_id": nil, "support_sla_tier_changed_at": at}).Error; err != nil {
				return fmt.Errorf("clear SLA tier from companies: %w", err)
			}
		}
		var policies []model.SupportSLAPolicy
		if err := tx.Where("workspace_id = ?", workspaceID).Find(&policies).Error; err != nil {
			return fmt.Errorf("list SLA policies for tier delete: %w", err)
		}
		for _, policy := range policies {
			if !slices.Contains([]string(policy.CompanyTierIDs), tierID) {
				continue
			}
			remaining := model.DocsStringArray(slices.DeleteFunc(slices.Clone([]string(policy.CompanyTierIDs)), func(id string) bool { return id == tierID }))
			if remaining == nil {
				remaining = model.DocsStringArray{}
			}
			if err := tx.Model(&model.SupportSLAPolicy{}).Where("workspace_id = ? AND id = ?", workspaceID, policy.ID).
				Updates(map[string]any{"company_tier_ids": remaining, "conditions_changed_at": at, "updated_at": at}).Error; err != nil {
				return fmt.Errorf("remove SLA tier from policy: %w", err)
			}
		}
		result := tx.Where("workspace_id = ? AND id = ?", workspaceID, tierID).Delete(&model.SupportSLATier{})
		if result.Error != nil {
			return fmt.Errorf("delete SLA tier: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrSupportSLATierNotFound
		}
		return nil
	})
	return companyIDs, err
}

// ReorderTiers sets positions to the order given. The list must contain
// every tier of the workspace exactly once.
func (r *SupportSLARepository) ReorderTiers(ctx context.Context, workspaceID string, tierIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []string
		if err := tx.Model(&model.SupportSLATier{}).Where("workspace_id = ?", workspaceID).Pluck("id", &existing).Error; err != nil {
			return fmt.Errorf("list SLA tiers to reorder: %w", err)
		}
		if len(existing) != len(tierIDs) {
			return ErrSupportSLATierNotFound
		}
		seen := make(map[string]bool, len(tierIDs))
		for _, id := range tierIDs {
			if !slices.Contains(existing, id) || seen[id] {
				return ErrSupportSLATierNotFound
			}
			seen[id] = true
		}
		for position, id := range tierIDs {
			if err := tx.Model(&model.SupportSLATier{}).Where("workspace_id = ? AND id = ?", workspaceID, id).
				Updates(map[string]any{"position": position, "updated_at": time.Now().UTC()}).Error; err != nil {
				return fmt.Errorf("reorder SLA tiers: %w", err)
			}
		}
		return nil
	})
}

// ExistingCompanyIDs returns which of companyIDs exist in the workspace.
func (r *SupportSLARepository) ExistingCompanyIDs(ctx context.Context, workspaceID string, companyIDs []string) ([]string, error) {
	if len(companyIDs) == 0 {
		return nil, nil
	}
	var ids []string
	if err := r.db.WithContext(ctx).Table("crm_companies").Where("workspace_id = ? AND id IN ?", workspaceID, companyIDs).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("check companies: %w", err)
	}
	return ids, nil
}

// CompanyNames returns the names of the given companies in the workspace.
func (r *SupportSLARepository) CompanyNames(ctx context.Context, workspaceID string, companyIDs []string) (map[string]string, error) {
	names := map[string]string{}
	if len(companyIDs) == 0 {
		return names, nil
	}
	var rows []model.SupportSLAPolicyCompany
	if err := r.db.WithContext(ctx).Table("crm_companies").Select("id, name").
		Where("workspace_id = ? AND id IN ?", workspaceID, companyIDs).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load company names: %w", err)
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

// GetCompany returns a company's SLA context, or nil when the company is not
// in the workspace.
func (r *SupportSLARepository) GetCompany(ctx context.Context, workspaceID, companyID string) (*SupportSLACompany, error) {
	var rows []struct {
		ID                      string
		SupportSLATierID        *string
		SupportSLATierChangedAt *time.Time
	}
	if err := r.db.WithContext(ctx).Table("crm_companies").
		Select("id, support_sla_tier_id, support_sla_tier_changed_at").
		Where("workspace_id = ? AND id = ?", workspaceID, companyID).Limit(1).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load company SLA tier: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &SupportSLACompany{ID: rows[0].ID, TierID: rows[0].SupportSLATierID, TierChangedAt: rows[0].SupportSLATierChangedAt}, nil
}

// SetCompanyTier gives companies a tier, or none when tierID is nil, and
// returns the companies whose tier changed.
func (r *SupportSLARepository) SetCompanyTier(ctx context.Context, workspaceID string, companyIDs []string, tierID *string, at time.Time) ([]string, error) {
	if len(companyIDs) == 0 {
		return nil, nil
	}
	var changed []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table("crm_companies").Where("workspace_id = ? AND id IN ?", workspaceID, companyIDs)
		if tierID == nil {
			query = query.Where("support_sla_tier_id IS NOT NULL")
		} else {
			query = query.Where("support_sla_tier_id IS NULL OR support_sla_tier_id <> ?", *tierID)
		}
		if err := query.Pluck("id", &changed).Error; err != nil {
			return fmt.Errorf("find companies to change SLA tier: %w", err)
		}
		if len(changed) == 0 {
			return nil
		}
		if err := tx.Table("crm_companies").Where("workspace_id = ? AND id IN ?", workspaceID, changed).
			Updates(map[string]any{"support_sla_tier_id": tierID, "support_sla_tier_changed_at": at, "updated_at": at}).Error; err != nil {
			return fmt.Errorf("set company SLA tier: %w", err)
		}
		return nil
	})
	return changed, err
}

// EnqueueCompanyConversations queues the open conversations of companies for
// evaluation, including those without SLA state yet.
func (r *SupportSLARepository) EnqueueCompanyConversations(ctx context.Context, workspaceID string, companyIDs []string, at time.Time) error {
	if len(companyIDs) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO support_sla_work (conversation_id, workspace_id, enqueued_at)
		SELECT c.id, c.workspace_id, ?
		FROM support_conversations AS c
		WHERE c.workspace_id = ? AND c.crm_company_id IN ? AND c.status IN ? AND c.merged_into_id IS NULL
		ON CONFLICT (conversation_id) DO UPDATE SET enqueued_at = excluded.enqueued_at`,
		at, workspaceID, companyIDs,
		[]string{model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer}).Error
	if err != nil {
		return fmt.Errorf("enqueue company conversations for SLA: %w", err)
	}
	return nil
}
