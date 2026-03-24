package worker

import "testing"

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
