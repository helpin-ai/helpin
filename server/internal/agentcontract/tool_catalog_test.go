package agentcontract

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMToolCatalogContracts(t *testing.T) {
	expected := map[string]string{
		"list_workspace_members":          "Workspace",
		"list_team_workflows_with_stages": "PM / Tasks",
		"list_pm_labels":                  "PM / Tasks",
		"create_task":                     "PM / Tasks",
		"list_tasks":                      "PM / Tasks",
		"get_task":                        "PM / Tasks",
		"update_task":                     "PM / Tasks",
		"create_task_checklist_item":      "PM / Tasks",
		"list_task_checklist":             "PM / Tasks",
		"update_task_checklist_item":      "PM / Tasks",
		"add_pm_comment":                  "PM / Tasks",
		"list_epics":                      "PM / Epics",
		"get_epic":                        "PM / Epics",
		"create_epic":                     "PM / Epics",
		"update_epic":                     "PM / Epics",
		"list_sprints":                    "PM / Sprints",
		"get_sprint":                      "PM / Sprints",
		"list_sprint_tasks":               "PM / Sprints",
		"create_sprint":                   "PM / Sprints",
		"update_sprint":                   "PM / Sprints",
		"list_objectives":                 "PM / Objectives",
		"get_objective":                   "PM / Objectives",
		"create_objective":                "PM / Objectives",
		"update_objective":                "PM / Objectives",
		"create_key_result":               "PM / Objectives",
		"update_key_result":               "PM / Objectives",
	}

	catalog := ListToolCatalog()
	toolsByName := make(map[string]any, len(catalog.Tools))
	categoryByName := make(map[string]string, len(catalog.Tools))
	descriptionByName := make(map[string]string, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		toolsByName[tool.Name] = tool.InputSchema
		categoryByName[tool.Name] = tool.Category
		descriptionByName[tool.Name] = tool.Description
	}

	for alias, category := range expected {
		t.Run(alias, func(t *testing.T) {
			metadata, ok := commandtools.ToolMetadataForAlias(alias)
			if !ok {
				t.Fatalf("canonical metadata missing for %q", alias)
			}
			if metadata.Category != category {
				t.Fatalf("metadata category = %q, want %q", metadata.Category, category)
			}
			catalogSchema, ok := toolsByName[alias]
			if !ok {
				t.Fatalf("catalog entry missing for %q", alias)
			}
			if categoryByName[alias] != category {
				t.Fatalf("catalog category = %q, want %q", categoryByName[alias], category)
			}
			if descriptionByName[alias] != metadata.Description {
				t.Fatalf("catalog description for %q drifted from canonical metadata\nmetadata: %q\ncatalog: %q", alias, metadata.Description, descriptionByName[alias])
			}
			if !reflect.DeepEqual(normalizeJSONValue(metadata.InputSchema), catalogSchema) {
				t.Fatalf("catalog schema for %q drifted from canonical metadata\nmetadata: %#v\ncatalog: %#v", alias, metadata.InputSchema, catalogSchema)
			}
			assertClosedObjectSchemas(t, catalogSchema, alias)
			assertBoundedArraySchemas(t, catalogSchema, alias)
		})
	}

	wantCategories := []string{"PM / Tasks", "PM / Epics", "PM / Sprints", "PM / Objectives"}
	for i, category := range wantCategories {
		if got := indexOf(catalog.Categories, category); got < 0 {
			t.Fatalf("category %q missing from %#v", category, catalog.Categories)
		} else if i > 0 {
			previous := indexOf(catalog.Categories, wantCategories[i-1])
			if got != previous+1 {
				t.Fatalf("category %q must immediately follow %q: %#v", category, wantCategories[i-1], catalog.Categories)
			}
		}
	}

	for _, alias := range []string{"list_workspace_members", "list_team_workflows_with_stages", "list_pm_labels", "list_tasks", "list_epics", "list_sprints", "list_sprint_tasks", "list_objectives"} {
		schema := requireCatalogSchema(t, toolsByName, alias)
		assertBoundedListSchema(t, alias, schema)
	}
	for _, alias := range []string{"list_tasks", "list_epics", "list_sprints", "list_sprint_tasks", "list_objectives", "list_contacts", "list_deals"} {
		assertSchemaFields(t, alias, requireCatalogSchema(t, toolsByName, alias), []string{"query"})
	}

	createTask := requireCatalogSchema(t, toolsByName, "create_task")
	assertSchemaFields(t, "create_task", createTask, []string{"sprint_id", "severity", "blocked", "blocker", "checklist_items"})
	assertClosedObjectSchemas(t, createTask, "create_task")
	assertBoundedArraySchemas(t, createTask, "create_task")
	listTasks := requireCatalogSchema(t, toolsByName, "list_tasks")
	assertSchemaFields(t, "list_tasks", listTasks, []string{
		"query", "epic_id", "sprint_id", "workflow_id", "state_id", "task_type", "priority", "severity",
		"completed", "archived", "updated_after", "page", "per_page",
	})
	assertClosedObjectSchemas(t, listTasks, "list_tasks")
	assertBoundedArraySchemas(t, listTasks, "list_tasks")

	updateTask := requireCatalogSchema(t, toolsByName, "update_task")
	properties := schemaProperties(t, "update_task", updateTask)
	for _, forbidden := range []string{"team_id", "workflow_id", "state_id", "workflow_state_id", "archived"} {
		if _, ok := properties[forbidden]; ok {
			t.Errorf("update_task must not expose %q", forbidden)
		}
	}

	assertRequiredFields(t, "create_task", createTask, []string{"name", "team_id"})
	assertRequiredFields(t, "create_task_checklist_item", requireCatalogSchema(t, toolsByName, "create_task_checklist_item"), []string{"text"})
	listChecklist := requireCatalogSchema(t, toolsByName, "list_task_checklist")
	assertSchemaFields(t, "list_task_checklist", listChecklist, []string{"task_id", "limit"})
	updateChecklist := requireCatalogSchema(t, toolsByName, "update_task_checklist_item")
	assertRequiredFields(t, "update_task_checklist_item", updateChecklist, []string{"checklist_item_id"})
	assertSchemaFields(t, "update_task_checklist_item", updateChecklist, []string{"task_id", "due_date"})
	assertRequiredFields(t, "add_pm_comment", requireCatalogSchema(t, toolsByName, "add_pm_comment"), []string{"content"})
	assertRequiredFields(t, "create_epic", requireCatalogSchema(t, toolsByName, "create_epic"), []string{"name"})
	assertRequiredFields(t, "create_sprint", requireCatalogSchema(t, toolsByName, "create_sprint"), []string{"name", "start_date", "end_date", "team_id"})
	assertRequiredFields(t, "create_objective", requireCatalogSchema(t, toolsByName, "create_objective"), []string{"name", "objective_type"})
	assertRequiredFields(t, "create_key_result", requireCatalogSchema(t, toolsByName, "create_key_result"), []string{"name"})

	assertSchemaEnum(t, "create_task.task_type", createTask, "task_type", []string{"feature", "bug", "chore"})
	assertSchemaEnum(t, "create_task.priority", createTask, "priority", []string{"none", "low", "medium", "high", "urgent"})
	assertSchemaEnum(t, "create_task.severity", createTask, "severity", []string{"none", "minor", "major", "critical"})
	assertSchemaEnum(t, "add_pm_comment.entity_type", requireCatalogSchema(t, toolsByName, "add_pm_comment"), "entity_type", []string{"task", "epic", "sprint", "objective"})
	assertSchemaEnum(t, "create_epic.health", requireCatalogSchema(t, toolsByName, "create_epic"), "health", []string{"no_health", "on_track", "at_risk", "off_track"})
	assertSchemaEnum(t, "list_sprints.status", requireCatalogSchema(t, toolsByName, "list_sprints"), "status", []string{"unstarted", "started", "done"})
	assertSchemaEnum(t, "create_objective.objective_type", requireCatalogSchema(t, toolsByName, "create_objective"), "objective_type", []string{"tactical", "strategic"})
	assertSchemaEnum(t, "create_objective.state", requireCatalogSchema(t, toolsByName, "create_objective"), "state", []string{"not_started", "active", "closed"})
	assertSchemaEnum(t, "create_objective.health", requireCatalogSchema(t, toolsByName, "create_objective"), "health", []string{"on_track", "at_risk", "off_track"})
	assertSchemaEnum(t, "create_key_result.result_type", requireCatalogSchema(t, toolsByName, "create_key_result"), "result_type", []string{"boolean", "percent", "numeric"})

	for alias, fields := range map[string][]string{
		"create_task":      {"deadline"},
		"update_task":      {"deadline"},
		"create_epic":      {"planned_start_date", "deadline"},
		"update_epic":      {"planned_start_date", "deadline"},
		"create_sprint":    {"start_date", "end_date"},
		"update_sprint":    {"start_date", "end_date"},
		"create_objective": {"planned_start_date", "deadline"},
		"update_objective": {"planned_start_date", "deadline"},
	} {
		schema := requireCatalogSchema(t, toolsByName, alias)
		for _, field := range fields {
			assertPropertyDescriptionContains(t, alias, schema, field, "YYYY-MM-DD")
		}
	}

	for alias, fields := range map[string][]string{
		"create_task": {"epic_id", "sprint_id", "workflow_id", "state_id"},
		"create_epic": {"epic_state_id", "owner_id", "owner_member_id", "team_id", "planning_repository_id"},
	} {
		schema := requireCatalogSchema(t, toolsByName, alias)
		for _, field := range fields {
			assertPropertyDescriptionContains(t, alias, schema, field, "Omit")
			assertPropertyDescriptionNotContains(t, alias, schema, field, "empty string to clear")
			assertPropertyMinimumLength(t, alias, schema, field, 1)
		}
	}

	for alias, fields := range map[string][]string{
		"create_epic":      {"planned_start_date", "deadline"},
		"create_objective": {"planned_start_date", "deadline"},
	} {
		schema := requireCatalogSchema(t, toolsByName, alias)
		for _, field := range fields {
			assertPropertyDescriptionContains(t, alias, schema, field, "Omit")
			assertPropertyDescriptionContains(t, alias, schema, field, "YYYY-MM-DD")
			assertPropertyDescriptionNotContains(t, alias, schema, field, "empty string to clear")
		}
	}

	for _, field := range []string{"epic_state_id", "owner_id", "owner_member_id", "team_id", "planning_repository_id", "planned_start_date", "deadline"} {
		assertPropertyDescriptionContains(t, "update_epic", requireCatalogSchema(t, toolsByName, "update_epic"), field, "empty string to clear")
	}
	for _, field := range []string{"planned_start_date", "deadline"} {
		assertPropertyDescriptionContains(t, "update_objective", requireCatalogSchema(t, toolsByName, "update_objective"), field, "empty string to clear")
	}
	assertPropertyDescriptionContains(t, "update_task_checklist_item", updateChecklist, "due_date", "YYYY-MM-DD")
	assertPropertyDescriptionContains(t, "update_task_checklist_item", updateChecklist, "due_date", "empty string to clear")

	for alias, fields := range map[string][]string{
		"update_task":      {"owner_member_ids", "label_ids"},
		"update_epic":      {"label_ids"},
		"update_sprint":    {"label_ids"},
		"update_objective": {"team_ids", "owner_ids", "owner_member_ids", "label_ids", "epic_ids"},
	} {
		schema := requireCatalogSchema(t, toolsByName, alias)
		for _, field := range fields {
			assertPropertyDescriptionContains(t, alias, schema, field, "preserve")
			assertPropertyDescriptionContains(t, alias, schema, field, "complete set")
			assertPropertyDescriptionContains(t, alias, schema, field, "empty array clears")
		}
	}

	createChecklist := requireCatalogSchema(t, toolsByName, "create_task_checklist_item")
	assertPropertyDescriptionContains(t, "create_task_checklist_item", createChecklist, "assignee_id", "workspace user ID")
	assertPropertyDescriptionContains(t, "update_task_checklist_item", updateChecklist, "assignee_id", "workspace user ID")
	checklistItems := schemaProperties(t, "create_task", createTask)["checklist_items"].(map[string]any)
	checklistItem := checklistItems["items"].(map[string]any)
	assertPropertyDescriptionContains(t, "create_task.checklist_items.items", checklistItem, "assignee_id", "workspace user ID")

	for alias, fields := range map[string][]string{
		"create_epic":      {"label_ids"},
		"create_sprint":    {"label_ids"},
		"create_objective": {"team_ids", "owner_ids", "owner_member_ids", "label_ids", "epic_ids"},
	} {
		schema := requireCatalogSchema(t, toolsByName, alias)
		for _, field := range fields {
			assertPropertyDescriptionNotContains(t, alias, schema, field, "on update")
			assertPropertyDescriptionNotContains(t, alias, schema, field, "preserve")
		}
	}

	if _, ok := toolsByName["assign_task_agent"]; !ok {
		t.Fatal("deprecated assign_task_agent compatibility entry was removed")
	}
}

func TestInsertDocumentArtifactCatalogContract(t *testing.T) {
	catalog := ListToolCatalog()
	toolsByName := make(map[string]any, len(catalog.Tools))
	descriptionsByName := make(map[string]string, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		toolsByName[tool.Name] = tool.InputSchema
		descriptionsByName[tool.Name] = tool.Description
	}
	schema := requireCatalogSchema(t, toolsByName, "insert_document_artifact")
	assertSchemaFields(t, "insert_document_artifact", schema, []string{"document_id", "artifact_id", "description", "after_block_id", "caption"})
	assertRequiredFields(t, "insert_document_artifact", schema, []string{"document_id", "artifact_id", "description"})
	assertClosedObjectSchemas(t, schema, "insert_document_artifact")
	if _, ok := toolsByName["insert_document_image"]; !ok {
		t.Fatal("compatibility insert_document_image alias is missing")
	}
	for toolName, required := range map[string][]string{
		"write_document_content":   {"does not embed private run artifacts", "call insert_document_artifact"},
		"insert_document_artifact": {"authenticated image or video block", "artifact_id", "text only"},
		"browser_record":           {"removes idle agent-reasoning gaps", "call insert_document_artifact", "does not embed the video"},
	} {
		for _, snippet := range required {
			if !strings.Contains(descriptionsByName[toolName], snippet) {
				t.Fatalf("%s description missing %q: %s", toolName, snippet, descriptionsByName[toolName])
			}
		}
	}
}

func TestSafeOperationalToolCatalogContracts(t *testing.T) {
	expected := map[string]string{
		"update_task_delivery_target": "PM / Delivery", "update_epic_delivery_target": "PM / Delivery",
		"update_document_metadata": "Docs",
		"get_crm_contact":          "CRM / Discovery", "get_crm_company": "CRM / Discovery", "get_crm_deal": "CRM / Discovery",
		"list_crm_companies": "CRM / Discovery", "list_crm_pipelines": "CRM / Discovery", "list_crm_associations": "CRM / Discovery",
		"update_crm_contact": "CRM / Operations", "update_crm_company": "CRM / Operations", "update_crm_deal": "CRM / Operations",
		"add_crm_activity": "CRM / Operations", "link_crm_objects": "CRM / Operations", "unlink_crm_association": "CRM / Operations", "set_primary_contact_company": "CRM / Operations",
		"list_support_conversations": "Support / Discovery", "get_support_conversation": "Support / Discovery", "list_support_tags": "Support / Discovery", "list_support_inboxes": "Support / Discovery", "list_support_assignees": "Support / Discovery",
		"assign_support_conversation": "Support / Triage", "move_support_conversation": "Support / Triage", "add_support_conversation_tag": "Support / Triage", "remove_support_conversation_tag": "Support / Triage",
		"link_support_conversation_task": "Support / Triage", "link_support_conversation_contact": "Support / Triage", "update_support_conversation_subject": "Support / Triage",
	}
	catalog := ListToolCatalog()
	byName := make(map[string]model.ToolCatalogEntry, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		byName[tool.Name] = tool
	}
	for alias, category := range expected {
		tool, ok := byName[alias]
		if !ok {
			t.Errorf("catalog entry missing for %q", alias)
			continue
		}
		metadata, ok := commandtools.ToolMetadataForAlias(alias)
		if !ok {
			t.Fatalf("metadata missing for %q", alias)
		}
		if tool.Category != category || tool.Description != metadata.Description || !reflect.DeepEqual(tool.InputSchema, normalizeJSONValue(metadata.InputSchema)) {
			t.Errorf("catalog entry %q drifted from canonical metadata", alias)
		}
		assertClosedObjectSchemas(t, tool.InputSchema, alias)
	}
	for _, category := range []string{"PM / Delivery", "CRM / Discovery", "CRM / Operations", "Support / Discovery", "Support / Triage"} {
		if indexOf(catalog.Categories, category) < 0 {
			t.Errorf("catalog category %q missing", category)
		}
	}
}

func requireCatalogSchema(t *testing.T, tools map[string]any, alias string) map[string]any {
	t.Helper()
	raw, ok := tools[alias]
	if !ok {
		t.Fatalf("catalog entry missing for %q", alias)
	}
	schema, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("%s schema = %#v", alias, raw)
	}
	return schema
}

func schemaProperties(t *testing.T, alias string, schema map[string]any) map[string]any {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s properties = %#v", alias, schema["properties"])
	}
	return properties
}

func assertSchemaFields(t *testing.T, alias string, schema map[string]any, fields []string) {
	t.Helper()
	properties := schemaProperties(t, alias, schema)
	for _, field := range fields {
		if _, ok := properties[field]; !ok {
			t.Errorf("%s schema missing %q", alias, field)
		}
	}
}

func assertRequiredFields(t *testing.T, alias string, schema map[string]any, want []string) {
	t.Helper()
	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("%s required = %#v", alias, schema["required"])
	}
	got := make([]string, 0, len(required))
	for _, raw := range required {
		value, ok := raw.(string)
		if !ok {
			t.Fatalf("%s required contains %#v", alias, raw)
		}
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s required = %#v, want %#v", alias, got, want)
	}
}

func assertSchemaEnum(t *testing.T, path string, schema map[string]any, field string, want []string) {
	t.Helper()
	property, ok := schemaProperties(t, path, schema)[field].(map[string]any)
	if !ok {
		t.Fatalf("%s schema missing field %q", path, field)
	}
	raw, ok := property["enum"].([]any)
	if !ok {
		t.Fatalf("%s enum = %#v", path, property["enum"])
	}
	got := make([]string, 0, len(raw))
	for _, value := range raw {
		item, ok := value.(string)
		if !ok {
			t.Fatalf("%s enum contains %#v", path, value)
		}
		got = append(got, item)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s enum = %#v, want %#v", path, got, want)
	}
}

func assertPropertyDescriptionContains(t *testing.T, alias string, schema map[string]any, field, needle string) {
	t.Helper()
	property, ok := schemaProperties(t, alias, schema)[field].(map[string]any)
	if !ok {
		t.Fatalf("%s.%s schema missing", alias, field)
	}
	description, _ := property["description"].(string)
	if !strings.Contains(description, needle) {
		t.Errorf("%s.%s description = %q, want it to contain %q", alias, field, description, needle)
	}
}

func assertPropertyDescriptionNotContains(t *testing.T, alias string, schema map[string]any, field, needle string) {
	t.Helper()
	property, ok := schemaProperties(t, alias, schema)[field].(map[string]any)
	if !ok {
		t.Fatalf("%s.%s schema missing", alias, field)
	}
	description, _ := property["description"].(string)
	if strings.Contains(description, needle) {
		t.Errorf("%s.%s description = %q, want it not to contain %q", alias, field, description, needle)
	}
}

func assertPropertyMinimumLength(t *testing.T, alias string, schema map[string]any, field string, want float64) {
	t.Helper()
	property, ok := schemaProperties(t, alias, schema)[field].(map[string]any)
	if !ok {
		t.Fatalf("%s.%s schema missing", alias, field)
	}
	if got, ok := property["minLength"].(float64); !ok || got != want {
		t.Errorf("%s.%s minLength = %#v, want %v", alias, field, property["minLength"], want)
	}
}

func assertBoundedListSchema(t *testing.T, alias string, schema map[string]any) {
	t.Helper()
	properties := schemaProperties(t, alias, schema)
	for _, field := range []string{"per_page", "limit"} {
		property, ok := properties[field].(map[string]any)
		if !ok {
			continue
		}
		if maximum, ok := property["maximum"].(float64); !ok || maximum > 100 {
			t.Fatalf("%s %s maximum = %#v, want <= 100", alias, field, property["maximum"])
		}
		return
	}
	t.Fatalf("%s must expose a bounded per_page or limit field", alias)
}

func assertClosedObjectSchemas(t *testing.T, raw any, path string) {
	t.Helper()
	value, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if value["type"] == "object" && value["additionalProperties"] != false {
		t.Errorf("%s object schema must set additionalProperties=false", path)
	}
	for key, child := range value {
		switch typed := child.(type) {
		case map[string]any:
			assertClosedObjectSchemas(t, typed, path+"."+key)
		case []any:
			for index, item := range typed {
				assertClosedObjectSchemas(t, item, path+"."+key+"["+strconv.Itoa(index)+"]")
			}
		}
	}
}

func assertBoundedArraySchemas(t *testing.T, raw any, path string) {
	t.Helper()
	value, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if value["type"] == "array" {
		maximum, ok := value["maxItems"].(float64)
		if !ok || maximum > 100 {
			t.Errorf("%s array schema maxItems = %#v, want <= 100", path, value["maxItems"])
		}
	}
	for key, child := range value {
		switch typed := child.(type) {
		case map[string]any:
			assertBoundedArraySchemas(t, typed, path+"."+key)
		case []any:
			for index, item := range typed {
				assertBoundedArraySchemas(t, item, path+"."+key+"["+strconv.Itoa(index)+"]")
			}
		}
	}
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = normalizeJSONValue(child)
		}
		return result
	case []string:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = item
		}
		return result
	case []map[string]any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = normalizeJSONValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = normalizeJSONValue(item)
		}
		return result
	case int:
		return float64(typed)
	default:
		return value
	}
}

func indexOf(values []string, needle string) int {
	for index, value := range values {
		if value == needle {
			return index
		}
	}
	return -1
}

func TestCreateTaskCatalogRejectsEmptyOptionalIDs(t *testing.T) {
	catalog := ListToolCatalog()
	for _, tool := range catalog.Tools {
		if tool.Name != "create_task" {
			continue
		}
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Fatalf("create_task schema = %#v", tool.InputSchema)
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("create_task properties = %#v", schema["properties"])
		}
		for _, field := range []string{"epic_id", "sprint_id", "workflow_id", "state_id"} {
			property, ok := properties[field].(map[string]any)
			if !ok {
				t.Fatalf("create_task %s schema = %#v", field, properties[field])
			}
			if property["minLength"] != float64(1) {
				t.Fatalf("create_task %s minLength = %#v, want 1", field, property["minLength"])
			}
			description, _ := property["description"].(string)
			if !strings.Contains(description, "do not send an empty string") {
				t.Fatalf("create_task %s description = %q", field, description)
			}
		}
		return
	}
	t.Fatal("create_task missing from tool catalog")
}

func TestGetTaskContextCatalogAllowsTargetDefaultAndCapsIDs(t *testing.T) {
	catalog := ListToolCatalog()
	for _, tool := range catalog.Tools {
		if tool.Name != "get_task_context" {
			continue
		}
		schema := tool.InputSchema.(map[string]any)
		properties := schema["properties"].(map[string]any)
		taskIDs := properties["task_ids"].(map[string]any)
		if taskIDs["maxItems"] != float64(50) {
			t.Fatalf("get_task_context task_ids maxItems = %#v, want 50", taskIDs["maxItems"])
		}
		taskKeys, ok := properties["task_keys"].(map[string]any)
		if !ok || taskKeys["maxItems"] != float64(50) {
			t.Fatalf("get_task_context task_keys = %#v, want maxItems 50", properties["task_keys"])
		}
		if required, ok := schema["required"].([]any); ok {
			for _, field := range required {
				if field == "task_ids" {
					t.Fatal("get_task_context task_ids must be optional")
				}
			}
		}
		return
	}
	t.Fatal("get_task_context missing from tool catalog")
}

func TestWorkspaceDiscoveryCatalogUsesCanonicalSearchAndPagination(t *testing.T) {
	catalog := ListToolCatalog()
	tools := make(map[string]any, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		tools[tool.Name] = tool.InputSchema
	}
	for _, alias := range []string{
		"search_workspace", "list_workspace_teams", "list_workspace_members", "list_tasks", "list_epics", "list_sprints",
		"list_sprint_tasks", "list_objectives", "list_documents", "search_documents",
		"list_deals", "list_contacts", "list_buyer_signals", "list_conversation_messages",
	} {
		schema := requireCatalogSchema(t, tools, alias)
		properties := schemaProperties(t, alias, schema)
		if _, ok := properties["limit"]; !ok {
			t.Errorf("%s is missing limit", alias)
		}
		if _, ok := properties["offset"]; !ok {
			t.Errorf("%s is missing offset", alias)
		}
	}
	search := requireCatalogSchema(t, tools, "search_workspace")
	assertRequiredFields(t, "search_workspace", search, []string{"query"})
	assertSchemaFields(t, "search_workspace", search, []string{"entity_types", "limit", "offset"})
	for _, tool := range catalog.Tools {
		if tool.Name == "search_workspace" && !strings.Contains(tool.Description, "names, and descriptions") {
			t.Fatalf("search_workspace must advertise task-description matching: %q", tool.Description)
		}
	}
}

func TestBrowserToolCatalogContracts(t *testing.T) {
	catalog := ListToolCatalog()
	want := map[string]bool{
		"browser_open": false, "browser_snapshot": false,
		"browser_act": false, "browser_screenshot": false, "browser_record": false,
	}
	for _, tool := range catalog.Tools {
		if _, ok := want[tool.Name]; !ok {
			continue
		}
		want[tool.Name] = true
		if tool.Category != "Browser" {
			t.Fatalf("%s category = %q", tool.Name, tool.Category)
		}
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("%s schema is not a strict object: %#v", tool.Name, tool.InputSchema)
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("%s missing from tool catalog", name)
		}
	}
	profile := GetRuntimeProfile("documentation")
	for name := range want {
		if indexOf(profile.AllowedTools, name) < 0 {
			t.Fatalf("documentation profile missing %s", name)
		}
	}
}

func TestWorkspaceReadToolCatalogContracts(t *testing.T) {
	catalog := ListToolCatalog()
	want := map[string]bool{"read_files": false}
	for _, tool := range catalog.Tools {
		if _, ok := want[tool.Name]; !ok {
			continue
		}
		want[tool.Name] = true
		if tool.Category != "Filesystem" {
			t.Fatalf("%s category = %q", tool.Name, tool.Category)
		}
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("%s schema is not a strict object: %#v", tool.Name, tool.InputSchema)
		}
		properties, _ := schema["properties"].(map[string]any)
		switch tool.Name {
		case "read_files":
			if !strings.Contains(tool.Description, "2,100-character content budget") ||
				!strings.Contains(tool.Description, "next_start_line") {
				t.Fatalf("read_files description omits bounded continuation contract: %q", tool.Description)
			}
			files := properties["files"].(map[string]any)
			if files["minItems"] != float64(1) || files["maxItems"] != float64(4) {
				t.Fatalf("read_files array bounds drifted: %#v", files)
			}
			items := files["items"].(map[string]any)
			if items["additionalProperties"] != false {
				t.Fatalf("read_files item schema is not strict: %#v", items)
			}
			itemProperties := items["properties"].(map[string]any)
			limit := itemProperties["limit_lines"].(map[string]any)
			if limit["minimum"] != float64(1) || limit["maximum"] != float64(240) || itemProperties["start_line"] == nil || itemProperties["repository"] == nil {
				t.Fatalf("read_files item contract drifted: %#v", itemProperties)
			}
			if description, _ := limit["description"].(string); !strings.Contains(description, "next_start_line") {
				t.Fatalf("read_files limit description omits continuation field: %#v", limit)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("%s missing from tool catalog", name)
		}
	}
}

func TestWebSearchToolCatalogDocumentsProviderPrecedence(t *testing.T) {
	catalog := ListToolCatalog()
	for _, tool := range catalog.Tools {
		if tool.Name != "web_search" {
			continue
		}
		if !strings.Contains(tool.Description, "Exa is preferred") ||
			!strings.Contains(tool.Description, "fallback for compatible fast searches") {
			t.Fatalf("web_search description omits provider selection contract: %q", tool.Description)
		}
		return
	}
	t.Fatal("web_search missing from tool catalog")
}

// The symbol tools are the catalog's view of agent-runtime's tree-sitter
// navigation tools. Their value is the exact line range they return, so the
// catalog must keep advertising that and must stay strict about inputs.
func TestSymbolNavigationToolCatalogContracts(t *testing.T) {
	catalog := ListToolCatalog()
	want := map[string]bool{
		"list_symbols": false, "read_symbol": false, "trace_symbol": false,
	}
	for _, tool := range catalog.Tools {
		if _, ok := want[tool.Name]; !ok {
			continue
		}
		want[tool.Name] = true
		if tool.Category != "Code Analysis" {
			t.Fatalf("%s category = %q, want Code Analysis", tool.Name, tool.Category)
		}
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok || schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("%s schema is not a strict object: %#v", tool.Name, tool.InputSchema)
		}
		properties, _ := schema["properties"].(map[string]any)
		if properties["repository"] == nil {
			t.Fatalf("%s schema missing repository; multi-repo selection would be impossible: %#v", tool.Name, schema)
		}
		switch tool.Name {
		case "read_symbol":
			if properties["symbol"] == nil || properties["kind"] == nil {
				t.Fatalf("read_symbol must take a symbol and optional kind: %#v", properties)
			}
		case "trace_symbol":
			if properties["symbol"] == nil || properties["direction"] == nil {
				t.Fatalf("trace_symbol must take symbol and direction: %#v", properties)
			}
		case "list_symbols":
			if !strings.Contains(tool.Description, "line range") {
				t.Fatalf("list_symbols description lost its line-range contract: %s", tool.Description)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("%s missing from tool catalog", name)
		}
	}
}
