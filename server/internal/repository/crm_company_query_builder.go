package repository

import "github.com/helpin-ai/helpin/server/internal/querybuilder"

var crmCompanyFilterDefinitions = querybuilder.Definitions{
	"name":            {Column: "crm_companies.name", Type: querybuilder.FieldTypeText},
	"domain":          {Column: "crm_companies.domain", Type: querybuilder.FieldTypeText},
	"external_id":     {Column: "crm_companies.external_id", Type: querybuilder.FieldTypeText},
	"industry":        {Column: "crm_companies.industry", Type: querybuilder.FieldTypeText},
	"headquarters":    {Column: "crm_companies.headquarters", Type: querybuilder.FieldTypeText},
	"description":     {Column: "crm_companies.description", Type: querybuilder.FieldTypeText},
	"employee_count":  {Column: "crm_companies.employee_count", Type: querybuilder.FieldTypeNumber},
	"annual_revenue":  {Column: "crm_companies.annual_revenue", Type: querybuilder.FieldTypeNumber},
	"owner_member_id": {Column: "crm_companies.owner_member_id", Type: querybuilder.FieldTypeID},
	"created_at":      {Column: "crm_companies.created_at", Type: querybuilder.FieldTypeDate},
	"updated_at":      {Column: "crm_companies.updated_at", Type: querybuilder.FieldTypeDate},
}
