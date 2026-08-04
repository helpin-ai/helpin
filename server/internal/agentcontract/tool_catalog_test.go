package agentcontract

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
)

func TestPMToolCatalogContracts(t *testing.T) {
	expected := map[string]string{
		"list_workspace_members":          "Workspace",
		"list_team_workflows_with_stages": "PM / Tasks",
		"list_pm_labels":                  "PM / Tasks",
		"get_task":                        "PM / Tasks",
		"update_task":                     "PM / Tasks",
		"create_task_checklist_item":      "PM / Tasks",
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
	for _, tool := range catalog.Tools {
		toolsByName[tool.Name] = tool.InputSchema
		categoryByName[tool.Name] = tool.Category
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

	createTask := requireCatalogSchema(t, toolsByName, "create_task")
	assertSchemaFields(t, "create_task", createTask, []string{"sprint_id", "severity", "blocked", "blocker", "checklist_items"})
	assertClosedObjectSchemas(t, createTask, "create_task")
	assertBoundedArraySchemas(t, createTask, "create_task")
	listTasks := requireCatalogSchema(t, toolsByName, "list_tasks")
	assertSchemaFields(t, "list_tasks", listTasks, []string{
		"epic_id", "sprint_id", "workflow_id", "state_id", "task_type", "priority", "severity",
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
			assertPropertyDescriptionContains(t, alias, schema, field, "complete set")
			assertPropertyDescriptionContains(t, alias, schema, field, "empty array clears")
		}
	}

	if _, ok := toolsByName["assign_task_agent"]; !ok {
		t.Fatal("deprecated assign_task_agent compatibility entry was removed")
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
