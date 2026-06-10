package service

import (
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMiraPresetDefinitionUsesNativeHelpinTools(t *testing.T) {
	preset, ok := agentPresetDefinition(model.AgentPresetMarketer)
	if !ok {
		t.Fatal("expected Mira marketer preset definition")
	}
	if preset.Label != "Mira" {
		t.Fatalf("label = %q, want Mira", preset.Label)
	}
	if preset.RuntimeKind != "native_sdk" {
		t.Fatalf("runtime = %q, want native_sdk", preset.RuntimeKind)
	}
	if preset.DefaultRole != "Marketer" {
		t.Fatalf("role = %q, want Marketer", preset.DefaultRole)
	}
	if preset.ApprovalMode != "always" {
		t.Fatalf("approval mode = %q, want always", preset.ApprovalMode)
	}
	for _, target := range []string{"workspace", "document", "task", "crm_deal", "crm_contact"} {
		if !slices.Contains(preset.AllowedTargetTypes, target) {
			t.Fatalf("expected target %q in %v", target, preset.AllowedTargetTypes)
		}
	}
	for _, tool := range []string{
		"update_plan",
		"request_user_input",
		"request_approval",
		"list_documents",
		"read_document",
		"create_document",
		"write_document_content",
		"publish_document_change_proposal",
		"list_tasks",
		"create_task",
		"add_task_comment",
		"list_deals",
		"list_contacts",
		"list_buyer_signals",
		"add_deal_note",
		"web_search_exa",
		"fetch_url",
		"crawl_url",
		"get_release_context",
		"find_tasks_for_git_changes",
	} {
		if !slices.Contains(preset.AllowedTools, tool) {
			t.Fatalf("expected tool %q in %v", tool, preset.AllowedTools)
		}
	}
	for _, forbidden := range []string{"web_search_brave", "run_command", "read_file", "write_file", "open_pr"} {
		if slices.Contains(preset.AllowedTools, forbidden) {
			t.Fatalf("did not expect mutating or broad repo tool %q in Mira tools: %v", forbidden, preset.AllowedTools)
		}
	}
}

func TestMiraSystemAgentName(t *testing.T) {
	if got := defaultSystemAgentNameForPresetKey(model.AgentPresetMarketer); got != "Mira" {
		t.Fatalf("default system agent name = %q, want Mira", got)
	}
}
