package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestToolListAvailableSkillsReturnsOnlyRuntimeSkillsWithoutInstructions(t *testing.T) {
	ctx := availableSkillsTestContext()

	output, err := toolListAvailableSkills(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("toolListAvailableSkills returned error: %v", err)
	}

	var response struct {
		Total  int `json:"total"`
		Skills []struct {
			ID           *string  `json:"id,omitempty"`
			Key          string   `json:"key"`
			Title        string   `json:"title"`
			Description  string   `json:"description"`
			SourceKind   string   `json:"source_kind"`
			RequiredTool []string `json:"required_tools,omitempty"`
			Instructions string   `json:"instructions,omitempty"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if response.Total != 2 || len(response.Skills) != 2 {
		t.Fatalf("expected two available skills, got %#v", response)
	}
	if response.Skills[0].Key != "campaign_planning" || response.Skills[0].Title != "Campaign Planning" {
		t.Fatalf("unexpected first skill %#v", response.Skills[0])
	}
	if response.Skills[0].Instructions != "" {
		t.Fatalf("list output should not include instructions, got %q", response.Skills[0].Instructions)
	}
	if response.Skills[1].ID == nil || *response.Skills[1].ID != "workspace-skill-1" || response.Skills[1].SourceKind != "workspace" {
		t.Fatalf("expected workspace skill id/source, got %#v", response.Skills[1])
	}
}

func TestToolSearchAvailableSkillsSearchesMetadataAndHidesInstructions(t *testing.T) {
	ctx := availableSkillsTestContext()

	output, err := toolSearchAvailableSkills(ctx, json.RawMessage(`{"query":"Lifecycle","limit":5}`))
	if err != nil {
		t.Fatalf("toolSearchAvailableSkills returned error: %v", err)
	}

	var response struct {
		Query  string `json:"query"`
		Total  int    `json:"total"`
		Skills []struct {
			Key          string `json:"key"`
			Instructions string `json:"instructions,omitempty"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if response.Query != "Lifecycle" {
		t.Fatalf("query = %q, want Lifecycle", response.Query)
	}
	if response.Total != 1 || len(response.Skills) != 1 || response.Skills[0].Key != "lifecycle_email" {
		t.Fatalf("expected lifecycle skill only, got %#v", response)
	}
	if response.Skills[0].Instructions != "" {
		t.Fatalf("search output should not include instructions, got %q", response.Skills[0].Instructions)
	}
}

func TestToolReadSkillReturnsInstructionsOnlyForAvailableSkill(t *testing.T) {
	ctx := availableSkillsTestContext()

	output, err := toolReadSkill(ctx, json.RawMessage(`{"key":"campaign_planning"}`))
	if err != nil {
		t.Fatalf("toolReadSkill returned error: %v", err)
	}
	if !strings.Contains(output, `"instructions":"Plan campaigns in phases."`) {
		t.Fatalf("expected instructions in read output, got %s", output)
	}

	if _, err := toolReadSkill(ctx, json.RawMessage(`{"key":"unassigned_skill"}`)); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unavailable skill error, got %v", err)
	}
}

func TestAvailableSkillToolsHideActiveCoreSkills(t *testing.T) {
	ctx := availableSkillsTestContext()
	ctx.ActiveRuntimeSkillRefs = model.AgentSkillRefs{{Key: "campaign_planning"}}
	ctx.ActiveSkillDefinitions = []SkillDefinition{ctx.RuntimeSkillDefinitions[0]}
	ctx.ActiveSkillInstructions = "Plan campaigns in phases."

	output, err := toolListAvailableSkills(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("toolListAvailableSkills returned error: %v", err)
	}
	if strings.Contains(output, `"key":"campaign_planning"`) {
		t.Fatalf("active core skill should not be listed as available, got %s", output)
	}
	if !strings.Contains(output, `"key":"lifecycle_email"`) {
		t.Fatalf("non-active skill should remain available, got %s", output)
	}

	if _, err := toolReadSkill(ctx, json.RawMessage(`{"key":"campaign_planning"}`)); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected active core skill to be unavailable through read_skill, got %v", err)
	}
}

func TestToolReadSkillRequiresSkillIDForAmbiguousKey(t *testing.T) {
	workspaceSkillID := "workspace-skill-override"
	ctx := &ExecutionContext{
		Context: context.Background(),
		RuntimeSkillRefs: model.AgentSkillRefs{
			{Key: "campaign_planning"},
			{Key: "campaign_planning", SkillID: &workspaceSkillID},
		},
		RuntimeSkillDefinitions: []SkillDefinition{
			{
				Key:          "campaign_planning",
				Title:        "Campaign Planning",
				Description:  "Built-in campaign planning.",
				SourceKind:   "built_in",
				Instructions: "Built-in campaign planning instructions.",
			},
			{
				Key:          "campaign_planning",
				Title:        "Workspace Campaign Planning",
				Description:  "Workspace campaign planning.",
				SourceKind:   "workspace",
				Instructions: "Workspace campaign planning instructions.",
			},
		},
	}

	if _, err := toolReadSkill(ctx, json.RawMessage(`{"key":"campaign_planning"}`)); err == nil || !strings.Contains(err.Error(), "multiple available skills") {
		t.Fatalf("expected ambiguous skill key error, got %v", err)
	}

	output, err := toolReadSkill(ctx, json.RawMessage(`{"skill_id":"workspace-skill-override"}`))
	if err != nil {
		t.Fatalf("toolReadSkill by skill_id returned error: %v", err)
	}
	if !strings.Contains(output, "Workspace campaign planning instructions.") {
		t.Fatalf("expected workspace skill instructions by id, got %s", output)
	}
}

func TestToolRegistryRegistersAvailableSkillTools(t *testing.T) {
	registry := NewToolRegistry(nil)
	definitions := make(map[string]struct{}, len(registry.Definitions()))
	for _, def := range registry.Definitions() {
		definitions[def.Name] = struct{}{}
	}
	for _, toolName := range []string{"list_available_skills", "search_available_skills", "read_skill"} {
		if _, ok := definitions[toolName]; !ok {
			t.Fatalf("expected tool %q to be registered", toolName)
		}
	}
}

func availableSkillsTestContext() *ExecutionContext {
	workspaceSkillID := "workspace-skill-1"
	return &ExecutionContext{
		Context: context.Background(),
		RuntimeSkillRefs: model.AgentSkillRefs{
			{Key: "campaign_planning"},
			{Key: "lifecycle_email", SkillID: &workspaceSkillID},
		},
		RuntimeSkillDefinitions: []SkillDefinition{
			{
				Key:               "campaign_planning",
				Title:             "Campaign Planning",
				Description:       "Plan campaigns and launches.",
				SourceKind:        "built_in",
				Instructions:      "Plan campaigns in phases.",
				RequiredTools:     []string{"list_documents"},
				SupportedRuntimes: []string{"native_sdk"},
			},
			{
				Key:               "lifecycle_email",
				Title:             "Lifecycle Email",
				Description:       "Create onboarding and lifecycle messages.",
				SourceKind:        "workspace",
				Instructions:      "Write lifecycle emails from lifecycle stage evidence.",
				SupportedRuntimes: []string{"native_sdk"},
			},
		},
	}
}
