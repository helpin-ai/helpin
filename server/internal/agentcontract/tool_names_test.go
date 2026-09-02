package agentcontract

import (
	"strings"
	"testing"
)

func TestHelpinMCPRuntimeToolNameUsesBareCanonicalNames(t *testing.T) {
	for alias, want := range map[string]string{
		ToolUpdatePlan:       ToolUpdatePlan,
		ToolRequestUserInput: ToolRequestUserInput,
		ToolPublishTaskPlan:  ToolPublishTaskPlan,
		"list_tasks":         "list_tasks",
		"read_file":          "read_files",
		HelpinMCPToolPrefix + "create_collection": "create_collection",
	} {
		if got := HelpinMCPRuntimeToolName(alias); got != want {
			t.Fatalf("runtime name for %q=%q, want %q", alias, got, want)
		}
	}
}

func TestCanonicalToolNameStripsHelpinMCPPrefixAndLegacyAliases(t *testing.T) {
	for raw, want := range map[string]string{
		HelpinMCPToolPrefix + ToolUpdatePlan:        ToolUpdatePlan,
		HelpinMCPToolPrefix + ToolRequestHumanInput: ToolRequestUserInput,
		ToolRequestHumanApproval:                    ToolRequestApproval,
		"read_file":                                 "read_files",
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

func TestRenderRuntimeToolNamesInInstructionsForNativeUsesLogicalNames(t *testing.T) {
	rendered := RenderRuntimeToolNamesInInstructionsForRuntime(
		"Create it with `mcp__helpin__create_collection`.",
		"native_sdk",
	)
	if rendered != "Create it with `create_collection`." {
		t.Fatalf("unexpected native instructions: %q", rendered)
	}
}
