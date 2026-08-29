package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ValidateSignalEntityScope prevents external evidence from creating cross-workspace references.
func (r *CRMSignalRepository) ValidateSignalEntityScope(ctx context.Context, workspaceID string, contactID, dealID, companyID *string) error {
	checks := []struct {
		name  string
		id    *string
		model interface{}
	}{
		{name: "contact", id: contactID, model: &model.CRMContact{}},
		{name: "deal", id: dealID, model: &model.CRMDeal{}},
		{name: "company", id: companyID, model: &model.CRMCompany{}},
	}
	for _, check := range checks {
		if check.id == nil || *check.id == "" {
			continue
		}
		var count int64
		if err := r.db.WithContext(ctx).Model(check.model).Where("workspace_id = ? AND id = ?", workspaceID, *check.id).Count(&count).Error; err != nil {
			return fmt.Errorf("validate external evidence %s: %w", check.name, err)
		}
		if count == 0 {
			return fmt.Errorf("%s not found in this workspace", check.name)
		}
	}
	return nil
}

func (r *CRMSignalRepository) GetSignalRuleConfigVersion(ctx context.Context, workspaceID, ruleKey string, version int) (*model.CRMSignalRuleConfig, error) {
	var config model.CRMSignalRuleConfig
	err := r.db.WithContext(ctx).
		Where("rule_key = ? AND version = ? AND (workspace_id = ? OR workspace_id IS NULL)", ruleKey, version, workspaceID).
		Order("CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END").First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get signal rule version: %w", err)
	}
	return &config, nil
}

// IngestExternalEvidence atomically stores normalized evidence and its signal.
func (r *CRMSignalRepository) IngestExternalEvidence(ctx context.Context, evidence *model.CRMSignalExternalEvidence, signal *model.CRMSignal) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "provider"}, {Name: "provider_evidence_id"}, {Name: "rule_key"}, {Name: "rule_version"}},
			DoNothing: true,
		}).Create(evidence)
		if result.Error != nil {
			return fmt.Errorf("create external signal evidence: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("workspace_id = ? AND provider = ? AND provider_evidence_id = ? AND rule_key = ? AND rule_version = ?",
				evidence.WorkspaceID, evidence.Provider, evidence.ProviderEvidenceID, evidence.RuleKey, evidence.RuleVersion).
				First(evidence).Error; err != nil {
				return fmt.Errorf("load existing external signal evidence: %w", err)
			}
			return nil
		}
		created = true
		signalRepo := NewCRMSignalRepository(tx)
		signalCreated, err := signalRepo.CreateRuleSignalIfAbsent(ctx, signal)
		if err != nil {
			return err
		}
		if !signalCreated {
			return fmt.Errorf("external evidence signal fingerprint conflict")
		}
		evidence.SignalID = &signal.ID
		if err := tx.Model(evidence).Update("signal_id", signal.ID).Error; err != nil {
			return fmt.Errorf("link external evidence signal: %w", err)
		}
		return nil
	})
	return created, err
}
