package service

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
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
	if agent.RuntimeKind != "codex" {
		t.Fatalf("expected default runtime codex, got %q", agent.RuntimeKind)
	}
}

func TestAgentDefaultsDerivedFromPreset(t *testing.T) {
	if got := defaultRoleForPresetKey(model.AgentPresetReviewAgent); got != "QA & Code Reviewer" {
		t.Fatalf("expected review preset role label, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetEpicPlanner); got != "codex" {
		t.Fatalf("expected planner runtime default codex, got %q", got)
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
		t.Fatalf("expected task planner preset, got %q", preset.Key)
	}
	if preset.VersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetTaskPlanner) {
		t.Fatalf("expected fallback version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetTaskPlanner), preset.VersionKey)
	}
}

func TestCommandAgentPresetNormalizesLegacyResearcherKey(t *testing.T) {
	preset, ok := agentPresetVersionDefinition(model.AgentPresetResearcher, "")
	if !ok {
		t.Fatal("expected legacy researcher preset key to resolve")
	}
	if preset.Key != model.AgentPresetCommandAgent {
		t.Fatalf("expected legacy researcher key to resolve to %q, got %q", model.AgentPresetCommandAgent, preset.Key)
	}
	if preset.VersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetCommandAgent) {
		t.Fatalf("expected command agent default version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetCommandAgent), preset.VersionKey)
	}
}

func TestCommandAgentPresetNormalizesLegacyResearcherVersion(t *testing.T) {
	preset, ok := agentPresetVersionDefinition(model.AgentPresetCommandAgent, "researcher_default")
	if !ok {
		t.Fatal("expected legacy researcher default version to resolve")
	}
	if preset.Key != model.AgentPresetCommandAgent {
		t.Fatalf("expected command agent preset key %q, got %q", model.AgentPresetCommandAgent, preset.Key)
	}
	if preset.VersionKey != "command_agent_default" {
		t.Fatalf("expected canonical command agent version, got %q", preset.VersionKey)
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
		if preset.RuntimeKind != "codex" {
			t.Fatalf("expected epic planner runtime codex, got %q", preset.RuntimeKind)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected epic planner default mode interactive, got %q", preset.DefaultInvocationMode)
		}
	}
	if !found {
		t.Fatal("expected epic planner preset in catalog")
	}
}

func TestListAgentPresetsUseProductDefaultRouting(t *testing.T) {
	presets := ListAgentPresets()
	if len(presets) == 0 {
		t.Fatal("expected preset catalog")
	}
	for _, preset := range presets {
		if preset.Key == model.AgentPresetTaskPlanner {
			if preset.RuntimeKind != "native_sdk" {
				t.Errorf("preset %q runtime = %q, want native_sdk", preset.Key, preset.RuntimeKind)
			}
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
				t.Errorf("preset %q provider = %+v, want openrouter", preset.Key, preset.Provider)
			}
			if preset.Model == nil || *preset.Model != defaultScribeAgentModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, defaultScribeAgentModel)
			}
			continue
		}
		if preset.Key == model.AgentPresetAskAgent || preset.Key == model.AgentPresetSupportAgent {
			// The dock orchestrator and support agent default to a flash-tier
			// OpenRouter model.
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
				t.Errorf("preset %q provider = %+v, want openrouter", preset.Key, preset.Provider)
			}
			if preset.Model == nil || *preset.Model != defaultAskAgentModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, defaultAskAgentModel)
			}
			continue
		}
		if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenAI {
			t.Errorf("preset %q provider = %+v, want openai", preset.Key, preset.Provider)
		}
		if preset.Model == nil || *preset.Model != "gpt-5.6-terra" {
			t.Errorf("preset %q model = %+v, want gpt-5.6-terra", preset.Key, preset.Model)
		}
	}
}

func TestListAgentPresetsIncludesInteractiveReviewAgent(t *testing.T) {
	presets := ListAgentPresets()
	for _, preset := range presets {
		if preset.Key != model.AgentPresetReviewAgent {
			continue
		}
		if preset.RuntimeKind != "codex" {
			t.Fatalf("expected review agent runtime codex, got %q", preset.RuntimeKind)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected review agent default mode interactive, got %q", preset.DefaultInvocationMode)
		}
		return
	}
	t.Fatal("expected review agent preset in catalog")
}

func TestListAgentPresetsIncludesDocumentationAgent(t *testing.T) {
	presets := ListAgentPresets()
	for _, preset := range presets {
		if preset.Key != model.AgentPresetDocumentationAgent {
			continue
		}
		if preset.VersionKey != defaultPresetVersionKeyForPresetKey(model.AgentPresetDocumentationAgent) {
			t.Fatalf("expected documentation default version %q, got %q", defaultPresetVersionKeyForPresetKey(model.AgentPresetDocumentationAgent), preset.VersionKey)
		}
		if preset.RuntimeKind != "codex" {
			t.Fatalf("expected documentation runtime codex, got %q", preset.RuntimeKind)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected documentation default mode interactive, got %q", preset.DefaultInvocationMode)
		}
		for _, targetType := range []string{"workspace", "document", "support_conversation", "support_coverage_gap", "task", "epic", "repository"} {
			if !slices.Contains(preset.AllowedTargetTypes, targetType) {
				t.Fatalf("expected documentation target %q in %v", targetType, preset.AllowedTargetTypes)
			}
		}
		for _, toolName := range []string{"list_repositories", "checkout_repository", "checkout_repositories", "read_file", "list_documents", "create_document", "write_document_content", "publish_document_change_proposal", "list_conversation_messages", "get_release_context"} {
			if !slices.Contains(preset.AllowedTools, toolName) {
				t.Fatalf("expected documentation tool %q in %v", toolName, preset.AllowedTools)
			}
		}
		return
	}
	t.Fatal("expected documentation agent preset in catalog")
}

func TestAskAgentCanInspectItsCapabilitiesSkillsAndRepositories(t *testing.T) {
	preset, ok := agentPresetDefinition(model.AgentPresetAskAgent)
	if !ok {
		t.Fatal("ask agent preset not found")
	}
	for _, toolName := range []string{
		"get_my_capabilities",
		"list_available_skills", "search_available_skills", "read_skill",
		"list_repositories", "checkout_repository", "checkout_repositories",
		"ripgrep", "search_files", "list_symbols", "read_file", "read_file_range",
	} {
		if !slices.Contains(preset.AllowedTools, toolName) {
			t.Errorf("Ask Agent is missing required self-execution tool %q", toolName)
		}
	}
}

func TestManagedAskAgentCapabilitiesUpgradePinnedSnapshots(t *testing.T) {
	preset := enforceManagedAskAgentCapabilities(model.AgentPresetDefinition{
		Key:                model.AgentPresetAskAgent,
		AllowedTools:       []string{"list_agents", "list_commits"},
		AllowedTargetTypes: []string{"document"},
		ExecutionConfig:    model.JSONBlob(`{"reasoning_effort":"high","workspace":{"mode":"repository"}}`),
	})
	for _, toolName := range []string{
		"checkout_repository", "ripgrep", "read_file",
		"list_available_skills", "read_skill", "update_plan",
		"get_my_capabilities", "create_document", "prepare_dock_execution",
	} {
		if !slices.Contains(preset.AllowedTools, toolName) {
			t.Errorf("managed Ask capability %q was not restored to pinned preset: %v", toolName, preset.AllowedTools)
		}
	}
	if !slices.Contains(preset.AllowedTargetTypes, "workspace") {
		t.Fatalf("managed Ask workspace target missing from %v", preset.AllowedTargetTypes)
	}
	var executionConfig map[string]interface{}
	if err := json.Unmarshal(preset.ExecutionConfig, &executionConfig); err != nil {
		t.Fatalf("decode managed Ask execution config: %v", err)
	}
	if executionConfig["reasoning_effort"] != "high" {
		t.Fatalf("unrelated execution settings were not preserved: %#v", executionConfig)
	}
	if _, ok := executionConfig["workspace"]; ok {
		t.Fatalf("managed Ask config retained repository workspace mode: %#v", executionConfig)
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
		for _, productTool := range []string{"ensure_task_plan_doc", "write_document_content"} {
			if !slices.Contains(preset.AllowedTools, productTool) {
				t.Fatalf("expected task planner preset to include %q, got %v", productTool, preset.AllowedTools)
			}
		}
		return
	}
	t.Fatal("expected task planner preset in catalog")
}

func TestListAgentPresetsPlannersIncludeExaSearch(t *testing.T) {
	presets := ListAgentPresets()
	expected := map[string]bool{
		model.AgentPresetEpicPlanner: false,
		model.AgentPresetTaskPlanner: false,
	}

	for _, preset := range presets {
		_, ok := expected[preset.Key]
		if !ok {
			continue
		}
		if !slices.Contains(preset.AllowedTools, "web_search_exa") {
			t.Fatalf("expected preset %q to include web_search_exa, got %v", preset.Key, preset.AllowedTools)
		}
		expected[preset.Key] = true
	}

	for presetKey, found := range expected {
		if !found {
			t.Fatalf("expected preset %q in catalog", presetKey)
		}
	}
}

func TestBuiltInPresetPMToolExpansionMatrix(t *testing.T) {
	newPMReadTools := []string{
		"list_workspace_members",
		"list_pm_labels",
		"get_task",
		"list_epics",
		"get_epic",
		"list_sprints",
		"get_sprint",
		"list_sprint_tasks",
		"list_objectives",
		"get_objective",
	}
	newPMWriteTools := []string{
		"update_task",
		"create_task_checklist_item",
		"update_task_checklist_item",
		"add_pm_comment",
		"create_epic",
		"update_epic",
		"create_sprint",
		"update_sprint",
		"create_objective",
		"update_objective",
		"create_key_result",
		"update_key_result",
	}
	allNewPMTools := append(slices.Clone(newPMReadTools), newPMWriteTools...)

	tests := []struct {
		presetKey string
		want      []string
	}{
		{presetKey: model.AgentPresetAskAgent, want: allNewPMTools},
		{presetKey: model.AgentPresetCommandAgent, want: allNewPMTools},
		{presetKey: model.AgentPresetEpicPlanner, want: []string{"list_workspace_members", "list_pm_labels", "get_task", "list_epics", "get_epic"}},
		{presetKey: model.AgentPresetTaskPlanner, want: []string{"list_workspace_members", "list_pm_labels", "get_task", "list_sprints", "get_sprint", "list_sprint_tasks"}},
		{presetKey: model.AgentPresetMarketer, want: newPMReadTools},
		{presetKey: model.AgentPresetCRMOperator, want: []string{}},
		{presetKey: model.AgentPresetSupportAgent, want: []string{}},
		{presetKey: model.AgentPresetDocumentationAgent, want: []string{}},
		{presetKey: model.AgentPresetCodeBuilder, want: []string{}},
		{presetKey: model.AgentPresetReviewAgent, want: []string{}},
	}

	presets := ListAgentPresets()
	for _, tc := range tests {
		t.Run(tc.presetKey, func(t *testing.T) {
			var preset *model.AgentPresetDefinition
			for idx := range presets {
				if presets[idx].Key == tc.presetKey {
					preset = &presets[idx]
					break
				}
			}
			if preset == nil {
				t.Fatalf("preset %q not found", tc.presetKey)
			}

			got := make([]string, 0, len(tc.want))
			for _, toolName := range allNewPMTools {
				if slices.Contains(preset.AllowedTools, toolName) {
					got = append(got, toolName)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("preset %q new PM tools = %v, want exactly %v", tc.presetKey, got, tc.want)
			}

			seen := make(map[string]struct{}, len(preset.AllowedTools))
			for _, toolName := range preset.AllowedTools {
				if _, ok := seen[toolName]; ok {
					t.Fatalf("preset %q contains duplicate tool %q in %v", tc.presetKey, toolName, preset.AllowedTools)
				}
				seen[toolName] = struct{}{}
			}
		})
	}
}

func TestCommandAgentPresetPMToolTargets(t *testing.T) {
	preset, ok := agentPresetDefinition(model.AgentPresetCommandAgent)
	if !ok {
		t.Fatal("command agent preset not found")
	}
	for _, targetType := range []string{"sprint", "objective"} {
		if !slices.Contains(preset.AllowedTargetTypes, targetType) {
			t.Fatalf("command agent target types = %v, want %q for its PM tools", preset.AllowedTargetTypes, targetType)
		}
	}
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

func TestValidateRuntimeProviderCompatibilityAllowsCodexOpenAIWithoutManagedOAuth(t *testing.T) {
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
	if err := svc.validateRuntimeProviderCompatibility(agent); err != nil {
		t.Fatalf("expected codex openai provider to be accepted without backend OAuth preflight, got %v", err)
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

func TestListModelProvidersIncludesExecutionCapabilities(t *testing.T) {
	svc := &AgentService{
		anthropicAPIKey:  "anthropic-secret",
		openAIAPIKey:     "openai-secret",
		openRouterAPIKey: "openrouter-secret",
	}

	options := svc.ListModelProviders()
	for _, option := range options {
		switch option.Value {
		case model.AgentModelProviderAnthropic:
			if option.DefaultModel != "claude-opus-4-8" {
				t.Fatalf("expected anthropic default model claude-opus-4-8, got %#v", option)
			}
		case model.AgentModelProviderOpenAI:
			if option.DefaultModel != "gpt-5.6-terra" {
				t.Fatalf("expected openai default model gpt-5.6-terra, got %#v", option)
			}
			if !option.SupportsReasoningEffort || !option.SupportsServiceTier {
				t.Fatalf("expected openai provider capabilities, got %#v", option)
			}
		case model.AgentModelProviderOpenRouter:
			if option.DefaultModel != "openai/gpt-5.6-terra" {
				t.Fatalf("expected openrouter default model openai/gpt-5.6-terra, got %#v", option)
			}
			if !option.SupportsReasoningEffort || option.SupportsServiceTier {
				t.Fatalf("expected openrouter provider capabilities, got %#v", option)
			}
		}
	}
}

func TestValidateModelRoutingRejectsCodexServiceTierForOpenRouter(t *testing.T) {
	openRouter := model.AgentModelProviderOpenRouter
	agent := &model.Agent{
		PresetKey:       model.AgentPresetCodeBuilder,
		RuntimeKind:     "codex",
		Provider:        &openRouter,
		ExecutionConfig: model.JSONBlob(`{"service_tier":"fast"}`),
	}

	svc := &AgentService{openRouterAPIKey: "openrouter-secret"}
	if err := svc.validateModelRouting(agent); err == nil {
		t.Fatal("expected openrouter codex service tier to be rejected")
	}
}

func TestValidateModelRoutingAllowsCodexReasoningConfig(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		PresetKey:       model.AgentPresetCodeBuilder,
		RuntimeKind:     "codex",
		Provider:        &openAI,
		ExecutionConfig: model.JSONBlob(`{"reasoning_effort":"high","service_tier":"fast"}`),
	}

	svc := &AgentService{
		openAIAPIKey: "openai-secret",
	}
	if err := svc.validateModelRouting(agent); err != nil {
		t.Fatalf("expected codex openai execution config to validate, got %v", err)
	}
	if strings.TrimSpace(string(agent.ExecutionConfig)) != `{"reasoning_effort":"high","service_tier":"fast"}` {
		t.Fatalf("expected normalized execution config to persist, got %s", agent.ExecutionConfig)
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

func TestValidateRuntimeForAgentAllowsCustomCodexPolicy(t *testing.T) {
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

	if err := validateRuntimeForAgent(agent); err != nil {
		t.Fatalf("expected custom codex tool policy to be allowed, got %v", err)
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
	materializeAgentSystemPrompt(agent)

	if agent.SystemPrompt == nil {
		t.Fatal("expected normalized system prompt")
	}
	if !strings.Contains(*agent.SystemPrompt, "You are Forge, the workspace code builder.") {
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
		AllowedTools:   json.RawMessage(`["request_human_approval","publish_preview","create_task_batch"]`),
		AllowedTargets: json.RawMessage(`["epic"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	for _, required := range []string{
		agentcontract.ToolUpdatePlan,
		agentcontract.ToolRequestApproval,
		agentcontract.ToolPublishPRDDraft,
		agentcontract.ToolPublishTaskPlan,
	} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected migrated tool list to contain %q, got %v", required, tools)
		}
	}
	for _, unexpected := range []string{
		agentcontract.ToolPublishPreview,
		agentcontract.ToolPreviewMarkdown,
		agentcontract.ToolPreviewJSON,
		agentcontract.ToolPublishTaskPlanDoc,
	} {
		if slices.Contains(tools, unexpected) {
			t.Fatalf("expected migrated tool list to exclude %q, got %v", unexpected, tools)
		}
	}
}

func TestNormalizeAgentRecordStripsGenericPreviewToolsFromTaskPlanner(t *testing.T) {
	agent := &model.Agent{
		IsSystem:       true,
		PresetKey:      model.AgentPresetTaskPlanner,
		TriggerMode:    "manual",
		RuntimeKind:    "native_sdk",
		AllowedTools:   json.RawMessage(`["request_human_approval","preview_md","publish_task_plan_doc","write_document_content","search_documents"]`),
		AllowedTargets: json.RawMessage(`["story"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	if !slices.Contains(tools, agentcontract.ToolPublishTaskPlanDoc) {
		t.Fatalf("expected sanitized tool list to keep %q, got %v", agentcontract.ToolPublishTaskPlanDoc, tools)
	}
	if !slices.Contains(tools, agentcontract.ToolUpdatePlan) {
		t.Fatalf("expected sanitized tool list to keep %q, got %v", agentcontract.ToolUpdatePlan, tools)
	}
	if !slices.Contains(tools, agentcontract.ToolRequestApproval) {
		t.Fatalf("expected sanitized tool list to keep %q, got %v", agentcontract.ToolRequestApproval, tools)
	}
	for _, productTool := range []string{"ensure_task_plan_doc", "write_document_content"} {
		if !slices.Contains(tools, productTool) {
			t.Fatalf("expected sanitized tool list to keep %q, got %v", productTool, tools)
		}
	}
	for _, unexpected := range []string{
		agentcontract.ToolPreviewMarkdown,
		agentcontract.ToolPreviewJSON,
		agentcontract.ToolPublishPreview,
		agentcontract.ToolPublishPRDDraft,
		agentcontract.ToolPublishTaskPlan,
	} {
		if slices.Contains(tools, unexpected) {
			t.Fatalf("expected sanitized tool list to exclude %q, got %v", unexpected, tools)
		}
	}
	if !slices.Contains(tools, "search_documents") {
		t.Fatalf("expected non-preview tools to be preserved, got %v", tools)
	}
}

func TestNormalizeAgentRecordStripsTaskPlannerPreviewToolsFromEpicPlanner(t *testing.T) {
	agent := &model.Agent{
		IsSystem:       true,
		PresetKey:      model.AgentPresetEpicPlanner,
		TriggerMode:    "manual",
		RuntimeKind:    "native_sdk",
		AllowedTools:   json.RawMessage(`["request_human_approval","preview_md","publish_task_plan_doc","publish_task_plan","publish_prd_draft","create_task_batch"]`),
		AllowedTargets: json.RawMessage(`["epic"]`),
	}

	normalizeAgentRecord(agent)

	tools := parseJSONStringSlice(agent.AllowedTools)
	for _, required := range []string{
		agentcontract.ToolUpdatePlan,
		agentcontract.ToolRequestApproval,
		agentcontract.ToolPublishPRDDraft,
		agentcontract.ToolPublishTaskPlan,
	} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected sanitized tool list to keep %q, got %v", required, tools)
		}
	}
	for _, unexpected := range []string{
		agentcontract.ToolPreviewMarkdown,
		agentcontract.ToolPreviewJSON,
		agentcontract.ToolPublishPreview,
		agentcontract.ToolPublishTaskPlanDoc,
		"create_task_batch",
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
			for _, required := range []string{"read_file", "search_documents", agentcontract.ToolUpdatePlan, agentcontract.ToolRequestApproval} {
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
	for _, required := range []string{"read_file", "search_documents", agentcontract.ToolUpdatePlan, agentcontract.ToolRequestApproval, agentcontract.ToolPublishTaskPlanDoc} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected sanitized tool list to keep %q, got %v", required, tools)
		}
	}
}
