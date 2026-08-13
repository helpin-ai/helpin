package agentcontract

import (
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
			if got != "read_files" {
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
