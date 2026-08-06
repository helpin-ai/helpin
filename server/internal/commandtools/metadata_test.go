package commandtools

import "testing"

func TestSafeOperationalToolMetadata(t *testing.T) {
	expected := map[string]string{
		"update_task_delivery_target":         "PM / Delivery",
		"update_epic_delivery_target":         "PM / Delivery",
		"update_document_metadata":            "Docs",
		"get_crm_contact":                     "CRM / Discovery",
		"get_crm_company":                     "CRM / Discovery",
		"get_crm_deal":                        "CRM / Discovery",
		"list_crm_companies":                  "CRM / Discovery",
		"list_crm_pipelines":                  "CRM / Discovery",
		"list_crm_associations":               "CRM / Discovery",
		"update_crm_contact":                  "CRM / Operations",
		"update_crm_company":                  "CRM / Operations",
		"update_crm_deal":                     "CRM / Operations",
		"add_crm_activity":                    "CRM / Operations",
		"link_crm_objects":                    "CRM / Operations",
		"unlink_crm_association":              "CRM / Operations",
		"set_primary_contact_company":         "CRM / Operations",
		"list_support_conversations":          "Support / Discovery",
		"get_support_conversation":            "Support / Discovery",
		"list_support_tags":                   "Support / Discovery",
		"list_support_inboxes":                "Support / Discovery",
		"list_support_assignees":              "Support / Discovery",
		"assign_support_conversation":         "Support / Triage",
		"move_support_conversation":           "Support / Triage",
		"add_support_conversation_tag":        "Support / Triage",
		"remove_support_conversation_tag":     "Support / Triage",
		"link_support_conversation_task":      "Support / Triage",
		"link_support_conversation_contact":   "Support / Triage",
		"update_support_conversation_subject": "Support / Triage",
	}

	for alias, category := range expected {
		t.Run(alias, func(t *testing.T) {
			metadata, ok := ToolMetadataForAlias(alias)
			if !ok {
				t.Fatalf("metadata missing for %q", alias)
			}
			if metadata.Category != category {
				t.Fatalf("category = %q, want %q", metadata.Category, category)
			}
			if metadata.InputSchema["type"] != "object" || metadata.InputSchema["additionalProperties"] != false {
				t.Fatalf("schema must be a closed object: %#v", metadata.InputSchema)
			}
		})
	}
}

func TestSafeCRMUpdateSchemasExcludeGuardedFields(t *testing.T) {
	tests := []struct {
		alias     string
		allowed   []string
		forbidden []string
	}{
		{
			alias:     "update_crm_contact",
			allowed:   []string{"contact_id", "lifecycle_stage", "lead_status", "owner_member_id", "clear_owner", "labels"},
			forbidden: []string{"first_name", "last_name", "email", "phone", "job_title", "description", "source", "custom_properties", "linkedin_url"},
		},
		{
			alias:     "update_crm_company",
			allowed:   []string{"company_id", "owner_member_id", "clear_owner"},
			forbidden: []string{"name", "domain", "industry", "employee_count", "annual_revenue", "description", "logo_url", "custom_properties"},
		},
		{
			alias:     "update_crm_deal",
			allowed:   []string{"deal_id", "name", "pipeline_id", "stage_id", "amount", "clear_amount", "currency", "close_date", "clear_close_date", "owner_member_id", "clear_owner", "probability", "clear_probability"},
			forbidden: []string{"custom_properties"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			metadata, ok := ToolMetadataForAlias(tt.alias)
			if !ok {
				t.Fatalf("metadata missing for %q", tt.alias)
			}
			properties, ok := metadata.InputSchema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("properties missing: %#v", metadata.InputSchema)
			}
			for _, field := range tt.allowed {
				if _, ok := properties[field]; !ok {
					t.Errorf("expected safe field %q", field)
				}
			}
			for _, field := range tt.forbidden {
				if _, ok := properties[field]; ok {
					t.Errorf("guarded field %q must not be exposed", field)
				}
			}
		})
	}
}

func TestSafeOperationalListSchemasAreBounded(t *testing.T) {
	for _, alias := range []string{"list_crm_companies", "list_crm_pipelines", "list_crm_associations", "list_support_conversations", "list_support_tags", "list_support_inboxes", "list_support_assignees"} {
		t.Run(alias, func(t *testing.T) {
			metadata, ok := ToolMetadataForAlias(alias)
			if !ok {
				t.Fatalf("metadata missing for %q", alias)
			}
			properties := metadata.InputSchema["properties"].(map[string]any)
			limit, ok := properties["limit"].(map[string]any)
			if !ok || limit["maximum"] != 100 {
				t.Fatalf("limit must be bounded to 100: %#v", properties["limit"])
			}
		})
	}
}
