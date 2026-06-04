package worker

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
)

func TestCreateTaskBatchToolSchemaRequiresStructuredImplementationBrief(t *testing.T) {
	registry := NewToolRegistry(nil)

	var schema map[string]interface{}
	for _, def := range registry.Definitions() {
		if def.Name == "create_task_batch" {
			var ok bool
			schema, ok = def.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatalf("expected object schema, got %#v", def.InputSchema)
			}
			break
		}
	}
	if schema == nil {
		t.Fatal("expected create_task_batch tool definition")
	}

	properties := schema["properties"].(map[string]interface{})
	tasks := properties["tasks"].(map[string]interface{})
	taskItems := tasks["items"].(map[string]interface{})
	taskProperties := taskItems["properties"].(map[string]interface{})
	brief := taskProperties["implementation_brief"].(map[string]interface{})
	briefProperties := brief["properties"].(map[string]interface{})
	filesToModify := briefProperties["files_to_modify"].(map[string]interface{})
	fileItems := filesToModify["items"].(map[string]interface{})
	fileProperties := fileItems["properties"].(map[string]interface{})

	for _, field := range []string{"approach", "files_to_modify", "test_strategy"} {
		if _, ok := briefProperties[field]; !ok {
			t.Fatalf("expected implementation_brief schema field %q, got %#v", field, briefProperties)
		}
	}
	for _, field := range []string{"path", "action", "description"} {
		if _, ok := fileProperties[field]; !ok {
			t.Fatalf("expected file change schema field %q, got %#v", field, fileProperties)
		}
	}
}

func TestCreateTaskBatchToolSchemaAllowsArrayTestStrategy(t *testing.T) {
	registry := NewToolRegistry(nil)

	var schema map[string]interface{}
	for _, def := range registry.Definitions() {
		if def.Name == "create_task_batch" {
			var ok bool
			schema, ok = def.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatalf("expected object schema, got %#v", def.InputSchema)
			}
			break
		}
	}
	if schema == nil {
		t.Fatal("expected create_task_batch tool definition")
	}

	properties := schema["properties"].(map[string]interface{})
	tasks := properties["tasks"].(map[string]interface{})
	taskItems := tasks["items"].(map[string]interface{})
	taskProperties := taskItems["properties"].(map[string]interface{})
	brief := taskProperties["implementation_brief"].(map[string]interface{})
	briefProperties := brief["properties"].(map[string]interface{})
	testStrategy := briefProperties["test_strategy"].(map[string]interface{})
	anyOf := testStrategy["anyOf"].([]map[string]interface{})

	if len(anyOf) != 2 {
		t.Fatalf("expected test_strategy anyOf schema, got %#v", testStrategy)
	}
}

func TestToolCatalogUsesSharedCommandToolMetadataForCategories(t *testing.T) {
	catalog := ListToolCatalog()

	categories := make(map[string]string, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		categories[tool.Name] = tool.Category
	}

	for toolName, want := range map[string]string{
		"approve_epic_spec":      "PM / Tasks",
		"write_document_content": "Docs",
		"update_deal_stage":      "CRM",
		"enrich_crm_contact":     "CRM",
	} {
		if got := categories[toolName]; got != want {
			t.Fatalf("expected tool %q to use shared category %q, got %q", toolName, want, got)
		}
	}
}

func TestToolRegistryIncludesAllSharedCommandTools(t *testing.T) {
	registry := NewToolRegistry(nil)

	definitions := make(map[string]struct{}, len(registry.Definitions()))
	for _, def := range registry.Definitions() {
		definitions[def.Name] = struct{}{}
	}

	for _, meta := range commandtools.AllRuntimeToolMetadata() {
		if _, ok := definitions[meta.Alias]; !ok {
			t.Fatalf("expected shared command-backed tool %q to be registered", meta.Alias)
		}
	}
}

func TestCRMEnrichmentToolSchemasAreStrict(t *testing.T) {
	for _, alias := range []string{"ensure_crm_contact_company", "enrich_crm_contact", "enrich_crm_company"} {
		meta, ok := commandtools.ToolMetadataForAlias(alias)
		if !ok {
			t.Fatalf("missing metadata for %s", alias)
		}
		if got := meta.InputSchema["additionalProperties"]; got != false {
			t.Fatalf("%s additionalProperties = %#v, want false", alias, got)
		}
		required, ok := meta.InputSchema["required"].([]string)
		if !ok {
			t.Fatalf("%s required has unexpected type %#v", alias, meta.InputSchema["required"])
		}
		if len(required) < 2 {
			t.Fatalf("%s required = %#v, want required fields", alias, required)
		}
		if alias == "ensure_crm_contact_company" {
			continue
		}
		if required[1] != "fields" {
			t.Fatalf("%s required = %#v, want object id and fields", alias, required)
		}
		properties := meta.InputSchema["properties"].(map[string]any)
		fields := properties["fields"].(map[string]any)
		item := fields["items"].(map[string]any)
		if got := item["additionalProperties"]; got != false {
			t.Fatalf("%s field item additionalProperties = %#v, want false", alias, got)
		}
	}
}
