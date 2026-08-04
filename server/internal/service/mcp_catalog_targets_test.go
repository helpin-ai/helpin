package service

import "testing"

func TestStartAgentRunCatalogIncludesCanonicalPlanningTargets(t *testing.T) {
	var startRun *MCPToolDefinition
	for _, definition := range specialMCPToolDefinitions() {
		if definition.Name == "start_agent_run" {
			definition := definition
			startRun = &definition
			break
		}
	}
	if startRun == nil {
		t.Fatal("start_agent_run tool definition not found")
	}
	properties, ok := startRun.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties schema = %#v", startRun.InputSchema["properties"])
	}
	targetType, ok := properties["target_type"].(map[string]any)
	if !ok {
		t.Fatalf("target_type schema = %#v", properties["target_type"])
	}
	targetValues, ok := targetType["enum"].([]any)
	if !ok {
		t.Fatalf("target enum = %#v", targetType["enum"])
	}
	targets := make([]string, 0, len(targetValues))
	for _, value := range targetValues {
		target, ok := value.(string)
		if !ok {
			t.Fatalf("target enum value = %#v", value)
		}
		targets = append(targets, target)
	}

	for _, wanted := range []string{"sprint", "objective"} {
		found := false
		for _, target := range targets {
			if target == wanted {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("target enum %v does not contain %q", targets, wanted)
		}
	}
}
