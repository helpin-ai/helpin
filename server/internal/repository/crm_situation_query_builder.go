package repository

import "github.com/helpin-ai/helpin/server/internal/querybuilder"

func situationQueryDefinitions() querybuilder.Definitions {
	return querybuilder.Definitions{
		"title":                       {Column: "s.title", Type: querybuilder.FieldTypeText},
		"objective":                   {Column: "s.objective", Type: querybuilder.FieldTypeText},
		"commercial_motion":           {Column: "s.commercial_motion", Type: querybuilder.FieldTypeEnum},
		"lifecycle":                   {Column: "s.lifecycle", Type: querybuilder.FieldTypeEnum},
		"attention":                   {Expression: "(" + situationAttentionSQL + ")", Type: querybuilder.FieldTypeEnum},
		"owner_member_id":             {Column: "s.owner_member_id", Type: querybuilder.FieldTypeID},
		"next_action_owner_member_id": {Column: "s.next_action_owner_member_id", Type: querybuilder.FieldTypeID},
		"company_id":                  {Column: "s.company_id", Type: querybuilder.FieldTypeID},
		"contact_id":                  {Column: "s.contact_id", Type: querybuilder.FieldTypeID},
		"deal_id":                     {Column: "s.deal_id", Type: querybuilder.FieldTypeID},
		"created_at":                  {Column: "s.created_at", Type: querybuilder.FieldTypeDate},
		"next_checkpoint_at":          {Column: "s.next_checkpoint_at", Type: querybuilder.FieldTypeDate},
	}
}
