package agentcontract

import "testing"

func TestCanonicalToolNameUsesBareCanonicalNames(t *testing.T) {
	for alias, want := range map[string]string{
		ToolUpdatePlan:       ToolUpdatePlan,
		ToolRequestUserInput: ToolRequestUserInput,
		ToolPublishTaskPlan:  ToolPublishTaskPlan,
		"list_tasks":         "list_tasks",
		"read_file":          "read_files",
		HelpinMCPToolPrefix + "create_collection": "create_collection",
	} {
		if got := CanonicalToolName(alias); got != want {
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

func TestCanonicalToolNameCoversEveryWriteSideMapping(t *testing.T) {
	for alias, canonical := range legacyToolAliases {
		if got := CanonicalToolName(alias); got != canonical {
			t.Fatalf("canonical name for %q=%q, want %q", alias, got, canonical)
		}
	}
}
