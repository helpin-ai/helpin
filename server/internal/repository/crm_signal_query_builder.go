package repository

import "github.com/helpin-ai/helpin/server/internal/querybuilder"

var crmSignalFilterDefinitions = querybuilder.Definitions{
	"account_id": {
		Column: "crm_buyer_signals.company_id",
		Type:   querybuilder.FieldTypeID,
	},
	"owner_member_id": {
		Expression: `COALESCE(
			(SELECT d.owner_member_id FROM crm_deals d
			 WHERE d.workspace_id = crm_buyer_signals.workspace_id AND d.id = crm_buyer_signals.deal_id LIMIT 1),
			(SELECT c.owner_member_id FROM crm_companies c
			 WHERE c.workspace_id = crm_buyer_signals.workspace_id AND c.id = crm_buyer_signals.company_id LIMIT 1)
		)`,
		Type: querybuilder.FieldTypeID,
	},
	"signal_type": {
		Column: "crm_buyer_signals.signal_type",
		Type:   querybuilder.FieldTypeEnum,
	},
	"source_type": {
		Column: "crm_buyer_signals.source_type",
		Type:   querybuilder.FieldTypeEnum,
	},
	"domain": {
		Column: "crm_buyer_signals.signal_domain",
		Type:   querybuilder.FieldTypeEnum,
	},
	"polarity": {
		Column: "crm_buyer_signals.polarity",
		Type:   querybuilder.FieldTypeEnum,
	},
	"trust": {
		Column: "crm_buyer_signals.evidence_identity_trust",
		Type:   querybuilder.FieldTypeEnum,
	},
	"status": {
		Expression: "CASE WHEN crm_buyer_signals.dismissed_at IS NULL THEN 'active' ELSE 'dismissed' END",
		Type:       querybuilder.FieldTypeEnum,
	},
	"detected_at": {
		Column: "crm_buyer_signals.detected_at",
		Type:   querybuilder.FieldTypeDate,
	},
}
