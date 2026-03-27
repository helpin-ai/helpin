package worker

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
)

func TestCreateStoryBatchToolSchemaRequiresStructuredImplementationBrief(t *testing.T) {
	registry := NewToolRegistry(nil)

	var schema map[string]interface{}
	for _, def := range registry.Definitions() {
		if def.Name == "create_story_batch" {
			var ok bool
			schema, ok = def.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatalf("expected object schema, got %#v", def.InputSchema)
			}
			break
		}
	}
	if schema == nil {
		t.Fatal("expected create_story_batch tool definition")
	}

	properties := schema["properties"].(map[string]interface{})
	stories := properties["stories"].(map[string]interface{})
	storyItems := stories["items"].(map[string]interface{})
	storyProperties := storyItems["properties"].(map[string]interface{})
	brief := storyProperties["implementation_brief"].(map[string]interface{})
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

func TestCreateStoryBatchToolSchemaAllowsArrayTestStrategy(t *testing.T) {
	registry := NewToolRegistry(nil)

	var schema map[string]interface{}
	for _, def := range registry.Definitions() {
		if def.Name == "create_story_batch" {
			var ok bool
			schema, ok = def.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatalf("expected object schema, got %#v", def.InputSchema)
			}
			break
		}
	}
	if schema == nil {
		t.Fatal("expected create_story_batch tool definition")
	}

	properties := schema["properties"].(map[string]interface{})
	stories := properties["stories"].(map[string]interface{})
	storyItems := stories["items"].(map[string]interface{})
	storyProperties := storyItems["properties"].(map[string]interface{})
	brief := storyProperties["implementation_brief"].(map[string]interface{})
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
		"approve_epic_spec":      "PM / Stories",
		"write_document_content": "Docs",
		"update_deal_stage":      "CRM",
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
