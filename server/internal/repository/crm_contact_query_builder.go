package repository

import (
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

var crmContactFilterDefinitions = querybuilder.Definitions{
	"name": {
		Expression: "TRIM(COALESCE(first_name, '') || ' ' || COALESCE(last_name, ''))",
		Type:       querybuilder.FieldTypeText,
	},
	"email": {
		Column: "email",
		Type:   querybuilder.FieldTypeText,
	},
	"phone": {
		Column: "phone",
		Type:   querybuilder.FieldTypeText,
	},
	"job_title": {
		Column: "job_title",
		Type:   querybuilder.FieldTypeText,
	},
	"lifecycle_stage": {
		Column: "lifecycle_stage",
		Type:   querybuilder.FieldTypeEnum,
	},
	"lead_status": {
		Column: "lead_status",
		Type:   querybuilder.FieldTypeEnum,
	},
	"owner_member_id": {
		Column: "owner_member_id",
		Type:   querybuilder.FieldTypeID,
	},
	"portal_access": {
		Column: "portal_access",
		Type:   querybuilder.FieldTypeEnum,
	},
	"source": {
		Column: "source",
		Type:   querybuilder.FieldTypeText,
	},
	"created_at": {
		Column: "created_at",
		Type:   querybuilder.FieldTypeDate,
	},
	"updated_at": {
		Column: "updated_at",
		Type:   querybuilder.FieldTypeDate,
	},
}
