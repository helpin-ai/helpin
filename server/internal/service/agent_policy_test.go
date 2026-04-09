package service

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func TestValidateAgentTargetEnforcesPresetTargetMapping(t *testing.T) {
	tests := []struct {
		name      string
		agent     model.Agent
		target    string
		shouldErr bool
	}{
		{
			name:   "epic planner can run on epics",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner},
			target: "epic",
		},
		{
			name:   "epic planner can run on tasks",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner},
			target: "task",
		},
		{
			name:   "epic planner can run on crm deals",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner},
			target: "crm_deal",
		},
		{
			name:   "code builder can run on tasks",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetCodeBuilder},
			target: "task",
		},
		{
			name:   "code builder can run on repositories",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetCodeBuilder},
			target: "repository",
		},
		{
			name:      "code builder cannot run on epics",
			agent:     model.Agent{IsSystem: true, PresetKey: model.AgentPresetCodeBuilder},
			target:    "epic",
			shouldErr: true,
		},
		{
			name:   "review agent can run on tasks",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetReviewAgent},
			target: "task",
		},
		{
			name:   "review agent can run on repositories",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetReviewAgent},
			target: "repository",
		},
		{
			name:   "support agent can run on support conversations",
			agent:  model.Agent{IsSystem: true, PresetKey: model.AgentPresetSupportAgent},
			target: "support_conversation",
		},
		{
			name:      "unknown preset is not runnable",
			agent:     model.Agent{IsSystem: true, PresetKey: "unknown"},
			target:    "story",
			shouldErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAgentTarget(&tc.agent, tc.target)
			if tc.shouldErr && err == nil {
				t.Fatalf("expected error for target %q and preset %q", tc.target, tc.agent.PresetKey)
			}
			if !tc.shouldErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeAgentRecordDefaultsToPreset(t *testing.T) {
	agent := &model.Agent{
		TriggerMode: "manual",
	}

	normalizeAgentRecord(agent)

	if agent.PresetKey != "" {
		t.Fatalf("expected blank custom agent preset_key to remain empty, got %q", agent.PresetKey)
	}
	if agent.PresetVersionKey != "" {
		t.Fatalf("expected blank custom agent preset_version_key to remain empty, got %q", agent.PresetVersionKey)
	}
	if agent.SourcePresetKey != "" {
		t.Fatalf("expected blank custom agent source preset to remain empty, got %q", agent.SourcePresetKey)
	}
	if agent.SourcePresetVersionKey != "" {
		t.Fatalf("expected blank custom agent source preset version to remain empty, got %q", agent.SourcePresetVersionKey)
	}
	if agent.RuntimeKind != "opencode" {
		t.Fatalf("expected default runtime opencode, got %q", agent.RuntimeKind)
	}
}

func TestAgentDefaultsDerivedFromPreset(t *testing.T) {
	if got := defaultRoleForPresetKey(model.AgentPresetReviewAgent); got != "Review Agent" {
		t.Fatalf("expected review preset role label, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetEpicPlanner); got != "native_sdk" {
		t.Fatalf("expected planner runtime default native_sdk, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetSupportAgent); got != "native_sdk" {
		t.Fatalf("expected support runtime default native_sdk, got %q", got)
	}
}

func TestPresetDefinitionForAgentDefaultsFromSystemFlag(t *testing.T) {
	systemAgent := &model.Agent{IsSystem: true}
	preset, ok := presetDefinitionForAgent(systemAgent)
	if !ok {
		t.Fatal("expected preset resolution for system agent")
	}
	if preset.Key != model.AgentPresetEpicPlanner {
		t.Fatalf("expected epic planner preset, got %q", preset.Key)
	}

	regularAgent := &model.Agent{}
	preset, ok = presetDefinitionForAgent(regularAgent)
	if !ok {
		t.Fatal("expected preset resolution for regular agent")
	}
	if preset.Key != model.AgentPresetCodeBuilder {
		t.Fatalf("expected code builder preset, got %q", preset.Key)
	}
}

func TestPresetDefinitionForAgentFallsBackToFamilyDefaultVersion(t *testing.T) {
	agent := &model.Agent{
		PresetKey:        model.AgentPresetTaskPlanner,
		PresetVersionKey: "missing_version",
	}

	preset, ok := presetDefinitionForAgent(agent)
	if !ok {
		t.Fatal("expected preset resolution for invalid version")
	}
	if preset.Key != model.AgentPresetTaskPlanner {
		t.Fatalf("expected story planner preset, got %q", preset.Key)
	}
	if preset.VersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetTaskPlanner) {
		t.Fatalf("expected fallback version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetTaskPlanner), preset.VersionKey)
	}
}

func TestListAgentPresetsIncludesEpicPlanner(t *testing.T) {
	presets := ListAgentPresets()
	if len(presets) == 0 {
		t.Fatal("expected preset catalog")
	}

	found := false
	for _, preset := range presets {
		if preset.Key != model.AgentPresetEpicPlanner {
			continue
		}
		found = true
		if preset.VersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner) {
			t.Fatalf("expected epic planner default version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetEpicPlanner), preset.VersionKey)
		}
		if !preset.IsDefaultVersion {
			t.Fatal("expected epic planner catalog entry to be marked as default version")
		}
		if preset.RuntimeKind != "native_sdk" {
			t.Fatalf("expected epic planner runtime native_sdk, got %q", preset.RuntimeKind)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected epic planner default mode interactive, got %q", preset.DefaultInvocationMode)
		}
	}
	if !found {
		t.Fatal("expected epic planner preset in catalog")
	}
}

func TestListAgentPresetsTaskPlannerExcludesListEpicTasks(t *testing.T) {
	presets := ListAgentPresets()
	for _, preset := range presets {
		if preset.Key != model.AgentPresetTaskPlanner {
			continue
		}
		if slices.Contains(preset.AllowedTools, "list_epic_tasks") {
			t.Fatalf("expected task planner preset to exclude list_epic_tasks, got %v", preset.AllowedTools)
		}
		return
	}
	t.Fatal("expected task planner preset in catalog")
}

func TestValidateRuntimeKindAllowsOnlyImplementedRuntimes(t *testing.T) {
	valid := []string{"opencode", "codex", "native_sdk"}
	for _, runtimeKind := range valid {
		if err := validateRuntimeKind(runtimeKind); err != nil {
			t.Fatalf("expected runtime %q to be valid, got %v", runtimeKind, err)
		}
	}

	invalid := []string{"native_claude", "claude_code", "openclaw", "zeroclaw"}
	for _, runtimeKind := range invalid {
		if err := validateRuntimeKind(runtimeKind); err == nil {
			t.Fatalf("expected runtime %q to be rejected", runtimeKind)
		}
	}
}

func TestAgentSupportsInteractiveRequiresNativeSDK(t *testing.T) {
	tests := []struct {
		name      string
		agent     model.Agent
		supported bool
	}{
		{
			name: "native sdk planner supports interactive",
			agent: model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			supported: true,
		},
		{
			name: "native sdk code builder supports interactive",
			agent: model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "native_sdk",
			},
			supported: true,
		},
		{
			name: "codex code builder supports interactive",
			agent: model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "codex",
			},
			supported: true,
		},
		{
			name: "opencode does not support interactive",
			agent: model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "opencode",
			},
			supported: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalizeAgentRecord(&tc.agent)
			got := agentSupportsMode(&tc.agent, model.InvocationModeInteractive)
			if got != tc.supported {
				t.Fatalf("expected interactive support=%v, got %v (runtime=%q)", tc.supported, got, tc.agent.RuntimeKind)
			}
		})
	}
}

func TestValidateRuntimeProviderCompatibilityRejectsAnthropicForCodex(t *testing.T) {
	anthropic := model.AgentModelProviderAnthropic
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "codex",
		Provider:    &anthropic,
	}

	svc := &AgentService{}
	if err := svc.validateRuntimeProviderCompatibility(agent); err == nil {
		t.Fatal("expected codex anthropic provider to be rejected")
	}
}

func TestValidateRuntimeProviderCompatibilityAllowsCodexOpenAIWithManagedOAuth(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "codex",
		Provider:    &openAI,
	}

	svc := &AgentService{
		codexOpenAIAuthMode:      "chatgpt_oauth",
		codexChatGPTOAuthEnabled: true,
		codexChatGPTAccessToken:  "token",
		codexChatGPTAccountID:    "account-123",
	}
	if err := svc.validateRuntimeProviderCompatibility(agent); err != nil {
		t.Fatalf("expected managed OAuth to satisfy Codex OpenAI runtime validation, got %v", err)
	}
	if err := svc.validateModelRouting(agent); err != nil {
		t.Fatalf("expected managed OAuth to satisfy Codex OpenAI model routing validation, got %v", err)
	}
}

func TestValidateRuntimeProviderCompatibilityRejectsCodexOpenAIWithoutManagedOAuth(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "codex",
		Provider:    &openAI,
	}

	svc := &AgentService{
		codexOpenAIAuthMode:      "chatgpt_oauth",
		codexChatGPTOAuthEnabled: false,
	}
	if err := svc.validateRuntimeProviderCompatibility(agent); err == nil {
		t.Fatal("expected codex openai provider without managed OAuth to be rejected")
	}
}

func TestListModelProvidersIncludesOpenAIForCodexDeviceCodeMode(t *testing.T) {
	svc := &AgentService{
		codexOpenAIAuthMode: "chatgpt_device_code",
	}

	options := svc.ListModelProviders()
	foundOpenAI := false
	for _, option := range options {
		if option.Value == model.AgentModelProviderOpenAI {
			foundOpenAI = true
			break
		}
	}
	if !foundOpenAI {
		t.Fatalf("expected openai provider option when codex device-code mode is enabled, got %#v", options)
	}
}

func TestValidateRuntimeForAgentAllowsReviewAgentCodexPreset(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		IsSystem:              true,
		PresetKey:             model.AgentPresetReviewAgent,
		RuntimeKind:           "codex",
		Provider:              &openAI,
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	normalizeAgentRecord(agent)

	if err := validateRuntimeForAgent(agent); err != nil {
		t.Fatalf("expected review_agent codex runtime to be allowed, got %v", err)
	}
}

func TestValidateRuntimeForAgentRejectsCustomCodexPolicy(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		IsSystem:              true,
		PresetKey:             model.AgentPresetCodeBuilder,
		RuntimeKind:           "codex",
		Provider:              &openAI,
		AllowedTools:          mustJSONStringSlice([]string{"read_file"}),
		AllowedCommands:       mustJSONStringSlice([]string{"go"}),
		AllowedTargets:        mustJSONStringSlice([]string{"story"}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}

	if err := validateRuntimeForAgent(agent); err == nil {
		t.Fatal("expected custom codex tool policy to be rejected")
	}
}

func TestNormalizeAgentRecordResetsInvalidRuntimeForPreset(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		IsSystem:    true,
		PresetKey:   model.AgentPresetReviewAgent,
		RuntimeKind: "codex",
		Provider:    &openAI,
	}

	normalizeAgentRecord(agent)

	if agent.RuntimeKind != "codex" {
		t.Fatalf("expected review agent runtime to preserve codex, got %q", agent.RuntimeKind)
	}
}

func TestNormalizeAgentRecordRefreshesLegacyCodeBuilderPrompt(t *testing.T) {
	legacyPrompt := "You are Builder, an AI coding agent. You write clean, correct code and follow existing project conventions."
	agent := &model.Agent{
		IsSystem:     true,
		PresetKey:    model.AgentPresetCodeBuilder,
		RuntimeKind:  "codex",
		SystemPrompt: &legacyPrompt,
	}

	normalizeAgentRecord(agent)

	if agent.SystemPrompt == nil {
		t.Fatal("expected normalized system prompt")
	}
	if !strings.Contains(*agent.SystemPrompt, "You are Code Builder.") {
		t.Fatalf("expected code builder prompt refresh, got:\n%s", *agent.SystemPrompt)
	}
}

func TestNormalizeAgentRecordClearsPlannerOnlyFieldsForNonEpicPlanner(t *testing.T) {
	planningNotes := "use specs first"
	systemPrompt := "do the work"
	budget := 100
	agent := &model.Agent{
		PresetKey:          model.AgentPresetCodeBuilder,
		RuntimeKind:        "",
		PlanningNotes:      &planningNotes,
		SystemPrompt:       &systemPrompt,
		MonthlyTokenBudget: &budget,
	}

	normalizeAgentRecord(agent)

	if agent.PlanningNotes != nil {
		t.Fatalf("expected non-planner normalization to clear planning notes, got %+v", agent.PlanningNotes)
	}
	if agent.SystemPrompt == nil || *agent.SystemPrompt != systemPrompt {
		t.Fatalf("expected system prompt to be preserved, got %+v", agent.SystemPrompt)
	}
	if agent.MonthlyTokenBudget == nil || *agent.MonthlyTokenBudget != budget {
		t.Fatalf("expected token budget to be preserved, got %+v", agent.MonthlyTokenBudget)
	}
	if agent.RuntimeKind == "" {
		t.Fatal("expected runtime kind default to be populated")
	}
}

func TestNormalizeAgentRecordMigratesLegacyPreviewToolsForPlannerPreset(t *testing.T) {
	agent := &model.Agent{
		IsSystem:       true,
		PresetKey:      model.AgentPresetEpicPlanner,
		TriggerMode:    "manual",
		RuntimeKind:    "native_sdk",
		AllowedTools:   json.RawMessage(`["request_human_approval","publish_preview","create_story_batch"]`),
		AllowedTargets: json.RawMessage(`["epic"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	for _, required := range []string{
		worker.ToolUpdatePlan,
		worker.ToolRequestReviewCheckpoint,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
	} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected migrated tool list to contain %q, got %v", required, tools)
		}
	}
	for _, unexpected := range []string{
		worker.ToolPublishPreview,
		worker.ToolPreviewMarkdown,
		worker.ToolPreviewJSON,
		worker.ToolPublishTaskPlanDoc,
	} {
		if slices.Contains(tools, unexpected) {
			t.Fatalf("expected migrated tool list to exclude %q, got %v", unexpected, tools)
		}
	}
}

func TestNormalizeAgentRecordStripsGenericPreviewToolsFromStoryPlanner(t *testing.T) {
	agent := &model.Agent{
		IsSystem:       true,
		PresetKey:      model.AgentPresetTaskPlanner,
		TriggerMode:    "manual",
		RuntimeKind:    "native_sdk",
		AllowedTools:   json.RawMessage(`["request_human_approval","preview_md","publish_story_plan_doc","write_document_content","search_documents"]`),
		AllowedTargets: json.RawMessage(`["story"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	if !slices.Contains(tools, worker.ToolPublishTaskPlanDoc) {
		t.Fatalf("expected sanitized tool list to keep %q, got %v", worker.ToolPublishTaskPlanDoc, tools)
	}
	if !slices.Contains(tools, worker.ToolUpdatePlan) {
		t.Fatalf("expected sanitized tool list to keep %q, got %v", worker.ToolUpdatePlan, tools)
	}
	if !slices.Contains(tools, worker.ToolRequestReviewCheckpoint) {
		t.Fatalf("expected sanitized tool list to keep %q, got %v", worker.ToolRequestReviewCheckpoint, tools)
	}
	for _, unexpected := range []string{
		worker.ToolPreviewMarkdown,
		worker.ToolPreviewJSON,
		worker.ToolPublishPreview,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
		"write_document_content",
	} {
		if slices.Contains(tools, unexpected) {
			t.Fatalf("expected sanitized tool list to exclude %q, got %v", unexpected, tools)
		}
	}
	if !slices.Contains(tools, "search_documents") {
		t.Fatalf("expected non-preview tools to be preserved, got %v", tools)
	}
}

func TestNormalizeAgentRecordStripsStoryPreviewToolFromEpicPlanner(t *testing.T) {
	agent := &model.Agent{
		IsSystem:       true,
		PresetKey:      model.AgentPresetEpicPlanner,
		TriggerMode:    "manual",
		RuntimeKind:    "native_sdk",
		AllowedTools:   json.RawMessage(`["request_human_approval","preview_md","publish_story_plan_doc","publish_story_plan","publish_prd_draft","create_story_batch"]`),
		AllowedTargets: json.RawMessage(`["epic"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	for _, required := range []string{
		worker.ToolUpdatePlan,
		worker.ToolRequestReviewCheckpoint,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
	} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected sanitized tool list to keep %q, got %v", required, tools)
		}
	}
	for _, unexpected := range []string{
		worker.ToolPreviewMarkdown,
		worker.ToolPreviewJSON,
		worker.ToolPublishPreview,
		worker.ToolPublishTaskPlanDoc,
		"create_story_batch",
	} {
		if slices.Contains(tools, unexpected) {
			t.Fatalf("expected sanitized tool list to exclude %q, got %v", unexpected, tools)
		}
	}
}

func TestNormalizeAgentRecordStripsRepositoryEditToolsFromPlannerPresets(t *testing.T) {
	for _, presetKey := range []string{model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner} {
		t.Run(presetKey, func(t *testing.T) {
			agent := &model.Agent{
				IsSystem:       true,
				PresetKey:      presetKey,
				TriggerMode:    "manual",
				RuntimeKind:    "native_sdk",
				AllowedTools:   json.RawMessage(`["read_file","write_file","edit_file","apply_patch","search_documents"]`),
				AllowedTargets: json.RawMessage(`["task"]`),
			}

			normalizeAgentRecord(agent)

			tools := parseJSONStringSlice(agent.AllowedTools)
			for _, unexpected := range []string{"write_file", "edit_file", "apply_patch"} {
				if slices.Contains(tools, unexpected) {
					t.Fatalf("expected sanitized tool list to exclude %q, got %v", unexpected, tools)
				}
			}
			for _, required := range []string{"read_file", "search_documents", worker.ToolUpdatePlan, worker.ToolRequestReviewCheckpoint} {
				if !slices.Contains(tools, required) {
					t.Fatalf("expected sanitized tool list to keep %q, got %v", required, tools)
				}
			}
		})
	}
}

func TestNormalizeAgentRecordStripsListEpicTasksFromTaskPlanner(t *testing.T) {
	agent := &model.Agent{
		IsSystem:       true,
		PresetKey:      model.AgentPresetTaskPlanner,
		TriggerMode:    "manual",
		RuntimeKind:    "native_sdk",
		AllowedTools:   json.RawMessage(`["read_file","list_epic_tasks","search_documents"]`),
		AllowedTargets: json.RawMessage(`["task"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	if slices.Contains(tools, "list_epic_tasks") {
		t.Fatalf("expected sanitized task planner tool list to exclude list_epic_tasks, got %v", tools)
	}
	for _, required := range []string{"read_file", "search_documents", worker.ToolUpdatePlan, worker.ToolRequestReviewCheckpoint, worker.ToolPublishTaskPlanDoc} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected sanitized tool list to keep %q, got %v", required, tools)
		}
	}
}
