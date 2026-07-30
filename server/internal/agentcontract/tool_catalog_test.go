package agentcontract

import (
	"strings"
	"testing"
)

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
		for _, field := range []string{"epic_id", "workflow_id", "state_id"} {
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
