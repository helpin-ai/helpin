package repository

import "github.com/helpin-ai/helpin/server/internal/querybuilder"

func inboxQueryDefinitions() querybuilder.Definitions {
	fields := querybuilder.Definitions{
		"priority":        {Column: "inbox.priority_band", Type: querybuilder.FieldTypeEnum},
		"evidence_review": {Column: "inbox.evidence_review", Type: querybuilder.FieldTypeEnum},
		"attention":       {Column: "inbox.attention", Type: querybuilder.FieldTypeEnum},
	}
	// Fixed taxonomy, never interpolated request values. EXISTS keeps one row per objective.
	for _, kind := range []string{"deal_create", "deal_advance", "follow_up", "risk_alert", "enrichment"} {
		fields["has_"+kind] = querybuilder.FieldDefinition{Expression: `CASE WHEN EXISTS (SELECT 1 FROM crm_suggestions typed
			WHERE CAST(typed.workspace_id AS TEXT) = inbox.workspace_id AND typed.status = 'pending'
			AND (inbox.kind = 'recommendation' AND CAST(typed.id AS TEXT) = inbox.id OR inbox.kind = 'situation' AND EXISTS (SELECT 1 FROM crm_situation_references ref WHERE ref.workspace_id = typed.workspace_id AND ref.kind = 'suggestion' AND ref.source_id = typed.id AND CAST(ref.situation_id AS TEXT) = inbox.id))
			AND typed.suggestion_type = '` + kind + `') THEN 'yes' ELSE 'no' END`, Type: querybuilder.FieldTypeEnum}
	}
	return fields
}
