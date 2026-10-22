package service

import (
	"regexp"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
)

// Audit authored instructions rather than legacy migration markers. Workspace
// edits are not bundled here and retain their independent validation policy.
func TestBuiltInInstructionToolNames(t *testing.T) {
	known := map[string]bool{}
	for _, tool := range agentcontract.ListToolCatalog().Tools {
		known[tool.Name] = true
	}
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	for _, def := range commands.ToolDefinitions() {
		known[def.Tool.Alias] = true
	}
	identifiers := regexp.MustCompile(`\b[a-z]+(?:_[a-z]+)+\b`)
	check := func(name, instructions string) {
		t.Helper()
		for _, token := range identifiers.FindAllString(instructions, -1) {
			if canonical := agentcontract.CanonicalToolName(token); canonical != token {
				t.Errorf("%s uses obsolete tool %q; use %q", name, token, canonical)
			}
		}
	}
	for _, skill := range agentcontract.ListBuiltInSkills() {
		check("skill "+skill.Key, skill.Instructions)
		for _, tool := range skill.RequiredTools {
			if !known[tool] {
				t.Errorf("skill %s requires unknown tool %q", skill.Key, tool)
			}
		}
	}
	for _, journey := range []string{"buying_intent", "renewal_recovery", "sales_handoff"} {
		snapshot, err := agentcontract.CaptureCRMPlaybookSpecialization(journey)
		if err != nil {
			t.Fatal(err)
		}
		check("playbook "+journey, snapshot.PresetPrompt)
		for _, skill := range snapshot.Skills {
			for _, file := range skill.Files {
				check(journey+"/"+file.Path, string(file.Data))
			}
		}
	}
	t.Logf("audited %d built-in skills, %d preset definitions and 3 CRM playbooks", len(agentcontract.ListBuiltInSkills()), len(ListAgentPresets()))
	for _, preset := range ListAgentPresets() {
		if preset.SystemPrompt != nil {
			check("preset "+preset.Key, *preset.SystemPrompt)
		}
		for _, tool := range preset.AllowedTools {
			if !known[tool] {
				t.Errorf("preset %s grants unknown tool %q", preset.Key, tool)
			}
		}
	}
}
