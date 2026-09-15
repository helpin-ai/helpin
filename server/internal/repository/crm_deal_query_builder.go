package repository

import "github.com/helpin-ai/helpin/server/internal/querybuilder"

var crmDealFilterDefinitions = querybuilder.Definitions{
	"name":            {Column: "crm_deals.name", Type: querybuilder.FieldTypeText},
	"stage_id":        {Expression: "CAST(crm_deals.stage_id AS TEXT)", Type: querybuilder.FieldTypeID},
	"stage_type":      {Expression: "(SELECT stage_type FROM crm_pipeline_stages WHERE id = crm_deals.stage_id)", Type: querybuilder.FieldTypeEnum},
	"owner_member_id": {Expression: "CAST(crm_deals.owner_member_id AS TEXT)", Type: querybuilder.FieldTypeID},
	"amount":          {Column: "crm_deals.amount", Type: querybuilder.FieldTypeNumber},
	"currency":        {Column: "crm_deals.currency", Type: querybuilder.FieldTypeEnum},
	"revenue_type":    {Column: "crm_deals.revenue_type", Type: querybuilder.FieldTypeEnum},
	"probability":     {Expression: "COALESCE(crm_deals.probability, (SELECT probability FROM crm_pipeline_stages WHERE id = crm_deals.stage_id))", Type: querybuilder.FieldTypeNumber},
	"close_date":      {Column: "crm_deals.close_date", Type: querybuilder.FieldTypeDate},
	"created_at":      {Column: "crm_deals.created_at", Type: querybuilder.FieldTypeDate},
	"updated_at":      {Column: "crm_deals.updated_at", Type: querybuilder.FieldTypeDate},
}
