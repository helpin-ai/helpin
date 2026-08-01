package worker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHelpinMCPRuntimeToolNamePrefixesHelpinTools(t *testing.T) {
	for _, alias := range []string{
		ToolUpdatePlan,
		ToolRequestUserInput,
		ToolPublishTaskPlan,
		"list_tasks",
		"read_file",
	} {
		got := HelpinMCPRuntimeToolName(alias)
		if alias == "read_file" {
			if got != "read_file" {
				t.Fatalf("expected local tool to stay unprefixed, got %q", got)
			}
			continue
		}
		want := HelpinMCPToolPrefix + alias
		if got != want {
			t.Fatalf("expected %q runtime name %q, got %q", alias, want, got)
		}
	}
}

func TestCanonicalToolNameStripsHelpinMCPPrefixAndLegacyAliases(t *testing.T) {
	for raw, want := range map[string]string{
		HelpinMCPToolPrefix + ToolUpdatePlan:        ToolUpdatePlan,
		HelpinMCPToolPrefix + ToolRequestHumanInput: ToolRequestUserInput,
		"helpin/" + ToolPublishTaskPlanDoc:          ToolPublishTaskPlanDoc,
		"agent_runtime/" + ToolRequestApproval:      ToolRequestApproval,
		"mcp__future_app__custom__approval":         "custom__approval",
		ToolRequestHumanApproval:                    ToolRequestApproval,
		"read_file":                                 "read_file",
	} {
		if got := CanonicalToolName(raw); got != want {
			t.Fatalf("CanonicalToolName(%q)=%q, want %q", raw, got, want)
		}
	}
}

func TestRenderRuntimeToolNamesInInstructionsForCodexUsesLogicalNames(t *testing.T) {
	rendered := RenderRuntimeToolNamesInInstructionsForRuntime(
		"Publish with `mcp__helpin__publish_task_plan_doc`, then call `mcp__helpin__request_approval`.",
		"codex",
	)
	for _, toolName := range []string{"`publish_task_plan_doc`", "`request_approval`"} {
		if !strings.Contains(rendered, toolName) {
			t.Fatalf("expected logical tool name %s, got %q", toolName, rendered)
		}
	}
	if strings.Contains(rendered, "mcp__helpin__") {
		t.Fatalf("expected Codex prompt to remove MCP qualification, got %q", rendered)
	}
}

func TestHelpinMCPRuntimeToolDefinitionsRenamesDiscoveredTools(t *testing.T) {
	defs := helpinMCPRuntimeToolDefinitions([]ToolDefinition{
		{Name: ToolUpdatePlan},
		{Name: "list_tasks"},
	})

	got := map[string]bool{}
	for _, def := range defs {
		got[def.Name] = true
	}
	for _, want := range []string{
		HelpinMCPToolPrefix + ToolUpdatePlan,
		HelpinMCPToolPrefix + "list_tasks",
	} {
		if !got[want] {
			t.Fatalf("expected runtime definition %q, got %#v", want, got)
		}
	}
}

func TestFilterNativeDirectMCPBackedToolDefinitionsRemovesOnlyDiscoveredMCPTools(t *testing.T) {
	defs := filterNativeDirectMCPBackedToolDefinitions([]ToolDefinition{
		{Name: HelpinMCPRuntimeToolName(ToolUpdatePlan)},
		{Name: "publish_document_change_proposal"},
		{Name: "read_file"},
	}, map[string]bool{
		ToolUpdatePlan: true,
	})

	got := map[string]bool{}
	for _, def := range defs {
		got[def.Name] = true
	}
	if got[HelpinMCPRuntimeToolName(ToolUpdatePlan)] {
		t.Fatalf("expected MCP-backed update_plan to be removed, got %#v", got)
	}
	if !got["publish_document_change_proposal"] || !got["read_file"] {
		t.Fatalf("expected undiscovered/local tools to remain, got %#v", got)
	}
}

func TestExecuteAllowedRoutesPrefixedHelpinMCPToolToCanonicalBridgeName(t *testing.T) {
	registry := NewToolRegistry(nil)
	var calledName string
	ctx := &ExecutionContext{
		AllowedTools: map[string]bool{ToolUpdatePlan: true},
		MCPToolNames: map[string]bool{
			ToolUpdatePlan:                           true,
			HelpinMCPRuntimeToolName(ToolUpdatePlan): true,
		},
		CallMCPTool: func(name string, input json.RawMessage) (string, error) {
			calledName = name
			return `{"ok":true}`, nil
		},
	}

	out, err := registry.ExecuteAllowed(ctx, HelpinMCPRuntimeToolName(ToolUpdatePlan), json.RawMessage(`{"plan":[{"step":"Inspect","status":"in_progress"}]}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if out != `{"ok":true}` {
		t.Fatalf("unexpected MCP output %q", out)
	}
	if calledName != ToolUpdatePlan {
		t.Fatalf("expected canonical MCP bridge call %q, got %q", ToolUpdatePlan, calledName)
	}
}
