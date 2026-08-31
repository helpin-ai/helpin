package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
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

// ListLatestRuleConfigs returns the effective newest version of every rule,
// including disabled and context-only rules for the admin calibration UI.
func (r *CRMSignalRepository) ListLatestRuleConfigs(ctx context.Context, workspaceID string) (map[string]model.CRMSignalRuleConfig, error) {
	var configs []model.CRMSignalRuleConfig
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? OR workspace_id IS NULL", workspaceID).
		Order("rule_key, CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END, version DESC").
		Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("list latest rule configs: %w", err)
	}
	effective := make(map[string]model.CRMSignalRuleConfig, len(configs))
	for _, config := range configs {
		if _, exists := effective[config.RuleKey]; !exists {
			effective[config.RuleKey] = config
		}
	}
	return effective, nil
}

// ListWorkspaceSignalCandidates returns signals for composition. A zero limit
// requests the complete workspace set; score and severity filters are applied
// after scoring by the service.
func (r *CRMSignalRepository) ListWorkspaceSignalCandidates(ctx context.Context, workspaceID string, filters model.CRMSignalListFilters, now time.Time, limit int) ([]model.CRMSignal, error) {
	if limit < 0 || limit > 5000 {
		limit = 500
	}
	query := r.db.WithContext(ctx).Model(&model.CRMSignal{}).Where("crm_signals.workspace_id = ?", workspaceID)
	hasSupersession := r.db.Migrator().HasColumn(&model.CRMSignal{}, "superseded_at")
	if filters.Query != nil {
		var err error
		query, err = querybuilder.ApplyGORM(query, filters.Query, crmSignalFilterDefinitions)
		if err != nil {
			return nil, err
		}
	}
	if filters.Status != nil && *filters.Status == "dismissed" {
		query = query.Where("dismissed_at IS NOT NULL")
	} else if hasSupersession && filters.Status != nil && *filters.Status == "superseded" {
		query = query.Where("superseded_at IS NOT NULL")
	} else if filters.Status == nil || *filters.Status == "active" {
		query = query.Where("dismissed_at IS NULL")
		if hasSupersession {
			query = query.Where("superseded_at IS NULL")
		}
	}
	if filters.CompanyID != nil && *filters.CompanyID != "" {
		query = query.Where("company_id = ?", *filters.CompanyID)
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}
	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_id = ?", *filters.ContactID)
	}
	if filters.SignalType != nil && *filters.SignalType != "" {
		query = query.Where("signal_type = ?", *filters.SignalType)
	}
	if filters.SourceType != nil && *filters.SourceType != "" {
		query = query.Where("source_type = ?", *filters.SourceType)
	}
	if filters.SignalDomain != nil && *filters.SignalDomain != "" {
		query = query.Where("signal_domain = ?", *filters.SignalDomain)
	}
	if filters.Polarity != nil && *filters.Polarity != "" {
		query = query.Where("polarity = ?", *filters.Polarity)
	}
	if r.db.Migrator().HasColumn(&model.CRMSignal{}, "commercial_motion") && filters.CommercialMotion != nil && *filters.CommercialMotion != "" {
		query = query.Where("commercial_motion = ?", *filters.CommercialMotion)
	}
	if filters.EvidenceIdentityTrust != nil && *filters.EvidenceIdentityTrust != "" {
		query = query.Where("evidence_identity_trust = ?", *filters.EvidenceIdentityTrust)
	}
	if filters.MaxAgeDays != nil && *filters.MaxAgeDays > 0 {
		query = query.Where("detected_at >= ?", now.Add(-time.Duration(*filters.MaxAgeDays)*24*time.Hour))
	}
	var signals []model.CRMSignal
	query = query.Order("detected_at DESC, id DESC")
	ownerFiltered := filters.OwnerMemberID != nil && strings.TrimSpace(*filters.OwnerMemberID) != ""
	if limit > 0 && !ownerFiltered {
		query = query.Limit(limit)
	}
	if err := query.Find(&signals).Error; err != nil {
		return nil, fmt.Errorf("list workspace signal candidates: %w", err)
	}
	if err := r.hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, err
	}
	if ownerFiltered {
		signals = filterSignalsByResolvedOwner(signals, strings.TrimSpace(*filters.OwnerMemberID))
		if limit > 0 && len(signals) > limit {
			signals = signals[:limit]
		}
	}
	return signals, nil
}
