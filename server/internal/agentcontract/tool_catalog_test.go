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
