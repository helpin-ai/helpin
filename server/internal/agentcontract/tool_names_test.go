package agentcontract

import "testing"

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
		ToolRequestHumanApproval:                    ToolRequestApproval,
		"read_file":                                 "read_file",
	} {
		if got := CanonicalToolName(raw); got != want {
			t.Fatalf("CanonicalToolName(%q)=%q, want %q", raw, got, want)
		}
	}
}
