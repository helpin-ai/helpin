package worker

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestListBuiltInSkillsContainsExpectedKeys(t *testing.T) {
	skills := ListBuiltInSkills()
	keys := make([]string, 0, len(skills))
	for _, skill := range skills {
		keys = append(keys, skill.Key)
	}
	for _, key := range []string{
		"approval_protocol",
		"prd_authorship",
		"task_decomposition",
		"epic_state_routing",
		"general_agent_behavior",
		"task_planner_context",
		"code_builder",
		"review_agent",
		"crm_operator",
		"support_agent",
	} {
		if !containsString(keys, key) {
			t.Fatalf("expected built-in skill %q in registry, got %v", key, keys)
		}
	}
}

func TestCompilePresetInstructionsIncludesPreambleAndSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetEpicPlanner)
	if !ok {
		t.Fatal("expected epic planner bundle")
	}
	compiled := CompilePresetInstructions(bundle.Preamble, bundle.SkillKeys)
	for _, snippet := range []string{
		"You are Epic Planner.",
		"Approval checkpoints happen inline in the same chat.",
		"## PRD Work",
		"## Current Facts And Next-Step Rules",
		"## General Rules",
	} {
		if !strings.Contains(compiled, snippet) {
			t.Fatalf("expected compiled prompt to contain %q\n%s", snippet, compiled)
		}
	}
}

func TestInstructionTemplateVersionForPresetIsDeterministic(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetReviewAgent)
	if !ok {
		t.Fatal("expected review bundle")
	}
	versionA := InstructionTemplateVersionForPreset(bundle.Preamble, bundle.SkillKeys)
	versionB := InstructionTemplateVersionForPreset(bundle.Preamble, bundle.SkillKeys)
	if versionA == "" {
		t.Fatal("expected non-empty template version")
	}
	if versionA != versionB {
		t.Fatalf("expected deterministic version, got %q and %q", versionA, versionB)
	}
}

func TestListSkillCatalogReturnsBuiltInSkills(t *testing.T) {
	catalog := ListSkillCatalog()
	if len(catalog.Skills) == 0 {
		t.Fatal("expected built-in skills in catalog")
	}
	found := false
	for _, skill := range catalog.Skills {
		if skill.Key != "approval_protocol" {
			continue
		}
		found = true
		if skill.SourceKind != "built_in" {
			t.Fatalf("expected built_in source kind, got %q", skill.SourceKind)
		}
		if !containsString(skill.RequiredTools, ToolRequestReviewCheckpoint) {
			t.Fatalf("expected request review checkpoint requirement, got %v", skill.RequiredTools)
		}
		if !containsString(skill.Presets, model.AgentPresetEpicPlanner) || !containsString(skill.Presets, model.AgentPresetTaskPlanner) {
			t.Fatalf("expected planner preset mappings, got %v", skill.Presets)
		}
	}
	if !found {
		t.Fatal("expected approval_protocol in skill catalog")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
