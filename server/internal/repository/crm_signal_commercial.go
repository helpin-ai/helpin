package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SignalCommercialContext loads seller context and workspace-scoped customer facts.
func (r *CRMSignalRepository) SignalCommercialContext(ctx context.Context, payload model.SignalSourcePayload) (model.SignalCommercialContext, error) {
	var workspace model.Workspace
	result := r.db.WithContext(ctx).Select("id", "name", "company_product_context", "description", "website_url").Where("id = ?", payload.WorkspaceID).First(&workspace)
	if result.Error != nil {
		return model.SignalCommercialContext{}, fmt.Errorf("load signal workspace context: %w", result.Error)
	}
	c := model.SignalCommercialContext{WorkspaceName: workspace.Name, CompanyID: payload.CompanyID}
	if workspace.CompanyProductContext != nil {
		c.ProductContext = strings.TrimSpace(*workspace.CompanyProductContext)
	}
	if c.ProductContext == "" && workspace.Description != nil {
		c.ProductContext = strings.TrimSpace(*workspace.Description)
	}
	if workspace.WebsiteURL != nil {
		c.WebsiteURL = *workspace.WebsiteURL
	}
	if payload.ContactID != nil && *payload.ContactID != "" {
		var contact model.CRMContact
		if err := r.db.WithContext(ctx).Select("id", "lifecycle_stage").Where("workspace_id = ? AND id = ?", payload.WorkspaceID, *payload.ContactID).First(&contact).Error; err != nil {
			return c, fmt.Errorf("load signal contact context: %w", err)
		}
		c.LifecycleStage = contact.LifecycleStage
		if c.CompanyID == nil {
			var ids []string
			err := r.db.WithContext(ctx).Table("crm_companies c").Select("c.id").Where("c.workspace_id = ?", payload.WorkspaceID).
				Where(`EXISTS (SELECT 1 FROM crm_associations a WHERE a.workspace_id = c.workspace_id AND
     ((a.from_object_type = 'contact' AND a.from_object_id = ? AND a.to_object_type = 'company' AND a.to_object_id = c.id)
     OR (a.to_object_type = 'contact' AND a.to_object_id = ? AND a.from_object_type = 'company' AND a.from_object_id = c.id)))`, *payload.ContactID, *payload.ContactID).
				Limit(2).Scan(&ids).Error
			if err != nil {
				return c, fmt.Errorf("resolve signal company context: %w", err)
			}
			if len(ids) == 1 {
				c.CompanyID = &ids[0]
			}
		}
	}
	if c.CompanyID != nil && *c.CompanyID != "" {
		if err := r.ValidateSignalReferences(ctx, payload.WorkspaceID, nil, nil, c.CompanyID); err != nil {
			return c, err
		}
		var state model.CRMCompanyCommercialState
		err := r.db.WithContext(ctx).Where("workspace_id = ? AND company_id = ?", payload.WorkspaceID, *c.CompanyID).First(&state).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return c, fmt.Errorf("load signal subscription context: %w", err)
		}
		if err == nil {
			c.SubscriptionStatus, _ = state.State["subscription_status"].(string)
		}
	}
	if payload.DealID != nil && *payload.DealID != "" {
		var deal struct {
			CommercialMotion        *string
			DefaultCommercialMotion string
			StageType               string
		}
		result := r.db.WithContext(ctx).Table("crm_deals d").Select("d.commercial_motion, p.default_commercial_motion, s.stage_type").
			Joins("JOIN crm_pipelines p ON p.id = d.pipeline_id AND p.workspace_id = d.workspace_id").
			Joins("JOIN crm_pipeline_stages s ON s.id = d.stage_id AND s.pipeline_id = d.pipeline_id").
			Where("d.workspace_id = ? AND d.id = ?", payload.WorkspaceID, *payload.DealID).Scan(&deal)
		if result.Error != nil {
			return c, fmt.Errorf("load signal deal context: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return c, fmt.Errorf("signal deal not found in workspace")
		}
		c.DealMotion, c.DealStageType = deal.DefaultCommercialMotion, deal.StageType
		if deal.CommercialMotion != nil && *deal.CommercialMotion != "" {
			c.DealMotion = *deal.CommercialMotion
		}
	}
	return c, nil
}

// commercialSignalsOnly excludes unqualified conversation history and context-only
// operational rules before pagination. Those rows remain available in record history.
func commercialSignalsOnly(query *gorm.DB) *gorm.DB {
	return query.Where(`(
  crm_signals.source_type = ? OR
  (crm_signals.detector_kind = 'llm_extracted' AND crm_signals.rule_key = ? AND crm_signals.rule_version >= ? AND crm_signals.metadata->>'commercial_relevance' = 'relevant') OR
  (crm_signals.detector_kind = ? AND crm_signals.rule_key IN ? AND crm_signals.polarity <> ?)
 )`, model.CRMSignalSourceManual, model.CRMSignalRuleConversationExtraction, model.CRMSignalCommercialDetectorVersion,
		model.CRMSignalDetectorRuleDerived, []string{
			model.CRMSignalRuleUrgentIssueOpenDeal,
			model.CRMSignalRuleDealStageStalled, model.CRMSignalRuleDealGoneDark,
			model.CRMSignalRuleChampionQuiet, model.CRMSignalRuleTimelineFollowupLapsed,
			model.CRMSignalRuleDealSingleThreaded, model.CRMSignalRuleRenewalApproaching,
			model.CRMSignalRuleBuyingCommitteeExpanded, model.CRMSignalRuleBuyingCommitteeShrank,
			model.CRMSignalRuleRepeatedPricingActivity, model.CRMSignalRuleProcurementPageActivity,
			model.CRMSignalRuleHighIntentProductEvent, model.CRMSignalRuleConfiguredForm,
			model.CRMSignalRulePaymentFailed, model.CRMSignalRuleDowngradeRequested,
		}, model.CRMSignalPolarityNeutral)
}

func isCommerciallyQualifiedConversation(signal model.CRMSignal) bool {
	return signal.DetectorKind == model.CRMSignalDetectorLLMExtracted &&
		signal.RuleKey != nil && *signal.RuleKey == model.CRMSignalRuleConversationExtraction &&
		signal.RuleVersion != nil && *signal.RuleVersion >= model.CRMSignalCommercialDetectorVersion &&
		signal.Metadata["commercial_relevance"] == "relevant" && signal.CommercialMotion != ""
}

// Event-specific meaning (for example a customer upgrade) is independent of the
// account's baseline lifecycle motion. New evidence and freshness govern its exit.
func excludeQualifiedCommercialEvents(query *gorm.DB) *gorm.DB {
	return query.Where(`NOT (COALESCE(detector_kind, '') = ? AND COALESCE(rule_key, '') = ? AND COALESCE(rule_version, 0) >= ? AND COALESCE(metadata->>'commercial_relevance', '') = 'relevant' AND commercial_motion NOT IN ('prospecting', 'conversion'))`,
		model.CRMSignalDetectorLLMExtracted, model.CRMSignalRuleConversationExtraction, model.CRMSignalCommercialDetectorVersion)
}
