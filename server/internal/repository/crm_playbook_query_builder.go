package repository

import "github.com/helpin-ai/helpin/server/internal/querybuilder"

func playbookEligibilityDefinitions() querybuilder.Definitions {
	return querybuilder.Definitions{
		"company_id":      {Column: "c.id", Type: querybuilder.FieldTypeID},
		"contact_id":      {Column: "contact.id", Type: querybuilder.FieldTypeID},
		"deal_id":         {Column: "d.id", Type: querybuilder.FieldTypeID},
		"pipeline_id":     {Column: "d.pipeline_id", Type: querybuilder.FieldTypeID},
		"stage_id":        {Column: "d.stage_id", Type: querybuilder.FieldTypeID},
		"company_domain":  {Column: "c.domain", Type: querybuilder.FieldTypeText},
		"owner_member_id": {Column: "s.owner_member_id", Type: querybuilder.FieldTypeID},
		"created_at":      {Column: "s.created_at", Type: querybuilder.FieldTypeDate},
	}
}
