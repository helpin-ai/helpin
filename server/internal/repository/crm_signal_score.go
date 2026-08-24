package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GetLatestSignalScoringConfig returns the newest workspace override or global config.
func (r *CRMSignalRepository) GetLatestSignalScoringConfig(ctx context.Context, workspaceID string) (*model.CRMSignalScoringConfig, error) {
	var config model.CRMSignalScoringConfig
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND (workspace_id = ? OR workspace_id IS NULL)", true, workspaceID).
		Order("CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END, version DESC").
		First(&config).Error
	if err != nil {
		return nil, fmt.Errorf("load signal scoring config: %w", err)
	}
	return &config, nil
}

// ListLatestRuleScoringConfigs returns one effective config per rule.
func (r *CRMSignalRepository) ListLatestRuleScoringConfigs(ctx context.Context, workspaceID string) (map[string]model.CRMSignalRuleConfig, error) {
	var configs []model.CRMSignalRuleConfig
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND (workspace_id = ? OR workspace_id IS NULL)", true, workspaceID).
		Order("rule_key, CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END, version DESC").
		Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("list rule scoring configs: %w", err)
	}
	effective := make(map[string]model.CRMSignalRuleConfig, len(configs))
	for _, config := range configs {
		if _, exists := effective[config.RuleKey]; !exists {
			effective[config.RuleKey] = config
		}
	}
	return effective, nil
}

// ListWorkspaceSignalCandidates returns a bounded set for composition. Score
// and severity filters are applied after scoring by the service.
func (r *CRMSignalRepository) ListWorkspaceSignalCandidates(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, now time.Time, limit int) ([]model.CRMBuyerSignal, error) {
	if limit < 1 || limit > 500 {
		limit = 500
	}
	query := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).Where("crm_buyer_signals.workspace_id = ?", workspaceID)
	if filters.Status != nil && *filters.Status == "dismissed" {
		query = query.Where("dismissed_at IS NOT NULL")
	} else if filters.Status == nil || *filters.Status == "active" {
		query = query.Where("dismissed_at IS NULL")
	}
	if filters.CompanyID != nil && *filters.CompanyID != "" {
		query = query.Where("company_id = ?", *filters.CompanyID)
	}
	if filters.SignalDomain != nil && *filters.SignalDomain != "" {
		query = query.Where("signal_domain = ?", *filters.SignalDomain)
	}
	if filters.Polarity != nil && *filters.Polarity != "" {
		query = query.Where("polarity = ?", *filters.Polarity)
	}
	if filters.EvidenceIdentityTrust != nil && *filters.EvidenceIdentityTrust != "" {
		query = query.Where("evidence_identity_trust = ?", *filters.EvidenceIdentityTrust)
	}
	if filters.MaxAgeDays != nil && *filters.MaxAgeDays > 0 {
		query = query.Where("detected_at >= ?", now.Add(-time.Duration(*filters.MaxAgeDays)*24*time.Hour))
	}
	if filters.OwnerMemberID != nil && strings.TrimSpace(*filters.OwnerMemberID) != "" {
		ownerID := strings.TrimSpace(*filters.OwnerMemberID)
		query = query.Where(`(
			EXISTS (SELECT 1 FROM crm_deals d WHERE d.workspace_id = crm_buyer_signals.workspace_id AND d.id = crm_buyer_signals.deal_id AND d.owner_member_id = ?)
			OR EXISTS (SELECT 1 FROM crm_companies c WHERE c.workspace_id = crm_buyer_signals.workspace_id AND c.id = crm_buyer_signals.company_id AND c.owner_member_id = ?)
		)`, ownerID, ownerID)
	}
	var signals []model.CRMBuyerSignal
	if err := query.Order("detected_at DESC, id DESC").Limit(limit).Find(&signals).Error; err != nil {
		return nil, fmt.Errorf("list workspace signal candidates: %w", err)
	}
	if err := r.hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, err
	}
	return signals, nil
}
