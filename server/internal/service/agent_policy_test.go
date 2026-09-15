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
			target:    "task",
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
	if agent.RuntimeKind != "native_sdk" {
		t.Fatalf("expected default runtime native_sdk, got %q", agent.RuntimeKind)
	}
}

func TestAgentDefaultsDerivedFromPreset(t *testing.T) {
	if got := defaultRoleForPresetKey(model.AgentPresetReviewAgent); got != "QA & Code Reviewer" {
		t.Fatalf("expected review preset role label, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetEpicPlanner); got != "native_sdk" {
		t.Fatalf("expected planner runtime default native_sdk, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetTaskPlanner); got != "native_sdk" {
		t.Fatalf("expected Scribe runtime default native_sdk, got %q", got)
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
		if preset.RuntimeKind != "native_sdk" {
			t.Fatalf("expected epic planner runtime native_sdk, got %q", preset.RuntimeKind)
		}
		if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
			t.Fatalf("expected epic planner provider openrouter, got %+v", preset.Provider)
		}
		if preset.Model == nil || *preset.Model != defaultAtlasAgentModel {
			t.Fatalf("expected epic planner model %s, got %+v", defaultAtlasAgentModel, preset.Model)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected epic planner default mode interactive, got %q", preset.DefaultInvocationMode)
		}
		for _, productTool := range []string{"ensure_epic_spec_doc", "write_document_content", "approve_epic_spec", "create_task_batch"} {
			if !slices.Contains(preset.AllowedTools, productTool) {
				t.Fatalf("expected epic planner preset to include %q, got %v", productTool, preset.AllowedTools)
			}
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
		usesFastOpenRouterDefault := preset.Key == model.AgentPresetEpicPlanner ||
			preset.Key == model.AgentPresetDocumentationAgent ||
			preset.Key == model.AgentPresetSupportAgent
		if usesFastOpenRouterDefault {
			config, err := model.ParseAgentExecutionConfig(preset.ExecutionConfig)
			if err != nil {
				t.Errorf("preset %q execution config: %v", preset.Key, err)
			} else if config.OpenRouter == nil || config.OpenRouter.Provider == nil ||
				!slices.Equal(config.OpenRouter.Provider.Quantizations, defaultFastOpenRouterQuantizations) {
				t.Errorf("preset %q execution config = %s, want quantizations %v", preset.Key, preset.ExecutionConfig, defaultFastOpenRouterQuantizations)
			}
		}
		if preset.Key == model.AgentPresetEpicPlanner {
			if preset.RuntimeKind != "native_sdk" {
				t.Errorf("preset %q runtime = %q, want native_sdk", preset.Key, preset.RuntimeKind)
			}
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
				t.Errorf("preset %q provider = %+v, want openrouter", preset.Key, preset.Provider)
			}
			if preset.Model == nil || *preset.Model != defaultAtlasAgentModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, defaultAtlasAgentModel)
			}
			continue
		}
		if preset.Key == model.AgentPresetTaskPlanner {
			if preset.RuntimeKind != "native_sdk" {
				t.Errorf("preset %q runtime = %q, want native_sdk", preset.Key, preset.RuntimeKind)
			}
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenAI {
				t.Errorf("preset %q provider = %+v, want openai", preset.Key, preset.Provider)
			}
			if preset.Model == nil || *preset.Model != defaultScribeAgentModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, defaultScribeAgentModel)
			}
			continue
		}
		if preset.Key == model.AgentPresetDocumentationAgent {
			if preset.RuntimeKind != "native_sdk" {
				t.Errorf("preset %q runtime = %q, want native_sdk", preset.Key, preset.RuntimeKind)
			}
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
				t.Errorf("preset %q provider = %+v, want openrouter", preset.Key, preset.Provider)
			}
			if preset.Model == nil || *preset.Model != defaultQuillAgentModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, defaultQuillAgentModel)
			}
			continue
		}
		if preset.Key == model.AgentPresetSupportAgent {
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
				t.Errorf("preset %q provider = %+v, want openrouter", preset.Key, preset.Provider)
			}
			if preset.Model == nil || *preset.Model != defaultFastOpenRouterAgentModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, defaultFastOpenRouterAgentModel)
			}
			continue
		}
		if preset.Key == model.AgentPresetAskAgent || preset.Key == model.AgentPresetCommandAgent {
			if preset.RuntimeKind != "native_sdk" {
				t.Errorf("preset %q runtime = %q, want native_sdk", preset.Key, preset.RuntimeKind)
			}
			if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
				t.Errorf("preset %q provider = %+v, want openrouter", preset.Key, preset.Provider)
			}
			wantModel := defaultAskAgentModel
			if preset.Key == model.AgentPresetCommandAgent {
				wantModel = defaultCommandAgentModel
			}
			if preset.Model == nil || *preset.Model != wantModel {
				t.Errorf("preset %q model = %+v, want %s", preset.Key, preset.Model, wantModel)
			}
			config, err := model.ParseAgentExecutionConfig(preset.ExecutionConfig)
			if err != nil {
				t.Errorf("preset %q execution config: %v", preset.Key, err)
			} else if config.MaxToolSteps == nil || *config.MaxToolSteps != managedAssistantMaxToolSteps || config.OpenRouter == nil || config.OpenRouter.Provider == nil ||
				!slices.Equal(config.OpenRouter.Provider.Quantizations, defaultFastOpenRouterQuantizations) {
				t.Errorf("preset %q execution config = %s, want FP8-or-higher routing and max_tool_steps=%d", preset.Key, preset.ExecutionConfig, managedAssistantMaxToolSteps)
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
		if preset.RuntimeKind != "native_sdk" {
			t.Fatalf("expected review agent runtime native_sdk, got %q", preset.RuntimeKind)
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
		if preset.RuntimeKind != "native_sdk" {
			t.Fatalf("expected documentation runtime native_sdk, got %q", preset.RuntimeKind)
		}
		if preset.Provider == nil || *preset.Provider != model.AgentModelProviderOpenRouter {
			t.Fatalf("expected documentation provider openrouter, got %+v", preset.Provider)
		}
		if preset.Model == nil || *preset.Model != defaultQuillAgentModel {
			t.Fatalf("expected documentation model %s, got %+v", defaultQuillAgentModel, preset.Model)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected documentation default mode interactive, got %q", preset.DefaultInvocationMode)
		}
		for _, targetType := range []string{"workspace", "document", "support_conversation", "support_coverage_gap", "task", "epic", "repository"} {
			if !slices.Contains(preset.AllowedTargetTypes, targetType) {
				t.Fatalf("expected documentation target %q in %v", targetType, preset.AllowedTargetTypes)
			}
		}
		for _, toolName := range []string{
			"list_repositories", "checkout_repositories", "read_files", "list_documents",
			"create_document", "write_document_content", "edit_document", "insert_document_artifact",
			"browser_open", "browser_screenshot", "browser_record",
			"publish_document_change_proposal", "list_conversation_messages",
			"get_release_context", "list_task_checklist", "list_epic_tasks",
			"get_pull_request_diff", "search_knowledge",
		} {
			if !slices.Contains(preset.AllowedTools, toolName) {
				t.Fatalf("expected documentation tool %q in %v", toolName, preset.AllowedTools)
			}
		}
		return
	}
	t.Fatal("expected documentation agent preset in catalog")
}

func TestOperationalPresetsExposeRelevantSafeTools(t *testing.T) {
	expected := map[string][]string{
		model.AgentPresetCRMOperator:        {"get_crm_contact", "get_crm_company", "get_crm_deal", "list_crm_companies", "list_crm_pipelines", "list_crm_associations", "create_crm_deal", "update_crm_contact", "update_crm_company", "update_crm_deal", "add_crm_activity", "link_crm_objects", "unlink_crm_association", "set_primary_contact_company"},
		model.AgentPresetDocumentationAgent: {"update_document_metadata"},
		model.AgentPresetSupportAgent:       {"list_support_conversations", "assign_support_conversation", "update_support_conversation_subject"},
		model.AgentPresetCommandAgent:       {"get_crm_contact", "create_crm_deal", "update_crm_contact", "list_support_conversations", "assign_support_conversation", "link_support_conversation_task", "update_document_metadata"},
		model.AgentPresetAskAgent:           {"get_crm_contact", "list_crm_pipelines", "create_crm_deal"},
	}
	byKey := map[string]model.AgentPresetDefinition{}
	for _, preset := range ListAgentPresets() {
		byKey[preset.Key] = preset
	}
	for key, tools := range expected {
		preset, ok := byKey[key]
		if !ok {
			t.Fatalf("preset %s missing", key)
		}
		for _, tool := range tools {
			if !slices.Contains(preset.AllowedTools, tool) {
				t.Errorf("preset %s missing %s", key, tool)
			}
		}
	}
}

func TestAskAgentRequiresExplicitDealPlacementChoices(t *testing.T) {
	prompt := askAgentSystemPrompt()
	for _, required := range []string{"call list_crm_pipelines", "multiple pipelines", "always ask which stage", "never silently choose a stage"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Ask Agent deal creation guidance missing %q", required)
		}
	}
}

func TestAskAgentCanInspectItsCapabilitiesSkillsAndRepositories(t *testing.T) {
	preset, ok := agentPresetDefinition(model.AgentPresetAskAgent)
	if !ok {
		t.Fatal("ask agent preset not found")
	}
	if preset.ApprovalMode != "risk_based" {
		t.Fatalf("Ask Agent approval mode = %q, want risk_based", preset.ApprovalMode)
	}
	for _, toolName := range []string{
		"get_my_capabilities",
		"create_task", "list_task_checklist", "list_epic_tasks", "ensure_task_label",
		"assign_task_agent", "set_task_dependencies", "update_task_state",
		"update_task_delivery_target", "update_epic_delivery_target",
		"create_document", "write_document_content", "edit_document", "insert_document_block",
		"insert_document_artifact", "preview_md", "preview_json",
		"browser_open", "browser_snapshot", "browser_act", "browser_screenshot", "browser_record",
		"list_conversation_messages",
		"search_workspace", "search_documents",
		"find_skills", "read_skill",
		"list_repositories", "checkout_repositories",
		"repository_search", "list_symbols", "read_files",
		"read_symbol", "trace_symbol", "get_pull_request_diff", "get_check_run_logs",
		"get_release_context", "find_tasks_for_git_changes",
	} {
		if !slices.Contains(preset.AllowedTools, toolName) {
			t.Errorf("Ask Agent is missing required self-execution tool %q", toolName)
		}
	}
	if len(preset.AvailableSkills) != 32 {
		t.Fatalf("Ask Agent default must expose 32 curated optional skills, got %d: %v", len(preset.AvailableSkills), preset.AvailableSkills)
	}
	for _, skillKey := range []string{"internal_docs_maintenance", "marketing_plan", "crm_record_operations", "competitors_changelog_tracking_report", "simplediag", "mermaid", "document_editing"} {
		if !slices.Contains(preset.AvailableSkills, skillKey) {
			t.Errorf("Ask Agent is missing available skill %q", skillKey)
		}
	}
	for _, skillKey := range []string{
		"code_implementation",
		"code_review",
		"support_triage_response",
		"security_triage",
		"task_plan_publishing",
		"release_notes_writing",
		"release_marketing",
	} {
		if slices.Contains(preset.AvailableSkills, skillKey) {
			t.Errorf("role-bound or tool-incompatible skill %q must not be available to Ask Agent", skillKey)
		}
	}
	if preset.SystemPrompt == nil || !strings.Contains(*preset.SystemPrompt, "primary execution agent") {
		t.Fatalf("Ask Agent skill availability must preserve its managed Dock prompt, got %v", preset.SystemPrompt)
	}
	if !strings.Contains(*preset.SystemPrompt, "Repository inspection is read-only") || !strings.Contains(*preset.SystemPrompt, "Sensitive or destructive tools are paused by the runtime") {
		t.Fatalf("Ask Agent prompt is missing domain execution guidance: %s", *preset.SystemPrompt)
	}
	if !strings.Contains(*preset.SystemPrompt, "start with the newest 20") || !strings.Contains(*preset.SystemPrompt, "Inspect image attachment URLs") {
		t.Fatalf("Ask Agent prompt is missing support transcript and image guidance: %s", *preset.SystemPrompt)
	}
	for _, required := range []string{
		"Every direct sub-agent launch needs an explicit target",
		"saved preset agents and Sub-agents use the same target contract",
		"Never switch a failed entity-specific launch to workspace",
		"Do not create or attach an epic",
		"update_task_delivery_target with task_id and repository_id",
		"retry the same task-targeted launch once",
		"If several repositories remain plausible",
	} {
		if !strings.Contains(*preset.SystemPrompt, required) {
			t.Errorf("Ask Agent prompt is missing launch recovery guidance %q", required)
		}
	}
	for _, skillKey := range preset.AvailableSkills {
		skill, ok := agentcontract.GetBuiltInSkill(skillKey)
		if !ok {
			t.Errorf("Ask Agent available skill %q is not registered", skillKey)
			continue
		}
		if len(skill.SupportedRuntimes) > 0 && !slices.Contains(skill.SupportedRuntimes, preset.RuntimeKind) {
			t.Errorf("Ask Agent available skill %q does not support runtime %q", skillKey, preset.RuntimeKind)
		}
		for _, requiredTool := range agentcontract.NormalizeToolNames(skill.RequiredTools) {
			if !slices.Contains(preset.AllowedTools, requiredTool) {
				t.Errorf("Ask Agent available skill %q requires unavailable tool %q", skillKey, requiredTool)
			}
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
	if preset.ApprovalMode != "risk_based" {
		t.Fatalf("managed Ask approval mode = %q, want risk_based", preset.ApprovalMode)
	}
	for _, toolName := range []string{
		"checkout_repositories", "repository_search", "read_files",
		"read_symbol", "trace_symbol",
		"find_skills", "read_skill", "update_plan",
		"get_my_capabilities", "search_workspace", "search_documents", "create_document",
		"insert_document_block", "preview_md", "preview_json", "list_task_checklist",
		"list_epic_tasks", "ensure_task_label", "assign_task_agent", "set_task_dependencies",
		"update_task_state", "update_task_delivery_target", "update_epic_delivery_target",
		"get_pull_request_diff", "get_check_run_logs", "get_release_context",
		"find_tasks_for_git_changes", "prepare_dock_execution",
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

func TestManagedDocumentationAgentCapabilitiesUpgradePinnedSnapshots(t *testing.T) {
	preset := enforceManagedDocumentationAgentCapabilities(model.AgentPresetDefinition{
		Key:          model.AgentPresetDocumentationAgent,
		AllowedTools: []string{"read_document", "write_document_content"},
	})
	for _, toolName := range []string{
		"insert_document_artifact", "edit_document", "list_task_checklist", "list_epic_tasks",
		"get_pull_request_diff", "search_knowledge",
	} {
		if !slices.Contains(preset.AllowedTools, toolName) {
			t.Fatalf("managed Documentation Agent capability %q was not restored: %v", toolName, preset.AllowedTools)
		}
	}
	if got := enforceManagedDocumentationAgentCapabilities(model.AgentPresetDefinition{
		Key:          model.AgentPresetSupportAgent,
		AllowedTools: []string{"read_document"},
	}); slices.Contains(got.AllowedTools, "insert_document_artifact") {
		t.Fatalf("artifact capability leaked into unrelated preset: %v", got.AllowedTools)
	}
}

func TestCommandAgentExposesGeneralWorkerToolsAndSkills(t *testing.T) {
	preset, ok := agentPresetDefinition(model.AgentPresetCommandAgent)
	if !ok {
		t.Fatal("command agent preset not found")
	}
	for _, toolName := range []string{
		"find_skills", "read_skill",
		"browser_open", "browser_snapshot", "browser_act", "browser_screenshot", "browser_record",
		"create_space", "create_collection", "update_space", "update_collection", "move_document",
		"insert_document_block", "insert_document_artifact", "preview_md", "preview_json",
		"list_task_checklist", "list_epic_tasks", "ensure_task_label", "assign_task_agent",
		"set_task_dependencies", "update_task_state", "update_task_delivery_target",
		"update_epic_delivery_target", "get_pull_request_diff", "get_check_run_logs",
		"get_release_context", "find_tasks_for_git_changes", "scan_semgrep", "scan_trivy",
		"scan_gitleaks", "search_knowledge", "draft_support_reply", "update_conversation_status",
	} {
		if !slices.Contains(preset.AllowedTools, toolName) {
			t.Errorf("Sub-agent is missing worker tool %q", toolName)
		}
	}
	if !slices.Equal(preset.AvailableSkills, commandAgentAvailableSkills()) {
		t.Fatalf("Sub-agent available skills = %v, want %v", preset.AvailableSkills, commandAgentAvailableSkills())
	}
	for _, forbidden := range []string{
		"run_command", "write_file", "apply_patch", "commit_and_push", "open_pr",
		"list_agents", "start_agent_run", "send_support_reply", "escalate_to_human",
	} {
		if slices.Contains(preset.AllowedTools, forbidden) {
			t.Errorf("Sub-agent must not expose specialist or orchestration tool %q", forbidden)
		}
	}
}

func TestManagedCommandAgentCapabilitiesUpgradePinnedSnapshots(t *testing.T) {
	preset := enforceManagedCommandAgentCapabilities(model.AgentPresetDefinition{
		Key:             model.AgentPresetCommandAgent,
		AllowedTools:    []string{"read_files"},
		AvailableSkills: []string{"workspace_skill"},
	})
	for _, toolName := range []string{
		"browser_open", "insert_document_block", "update_task_state",
		"get_pull_request_diff", "scan_semgrep", "search_knowledge",
	} {
		if !slices.Contains(preset.AllowedTools, toolName) {
			t.Errorf("managed Sub-agent capability %q was not restored: %v", toolName, preset.AllowedTools)
		}
	}
	if !slices.Contains(preset.AvailableSkills, "workspace_skill") {
		t.Fatalf("managed Sub-agent dropped an existing available skill: %v", preset.AvailableSkills)
	}
	for _, skillKey := range commandAgentAvailableSkills() {
		if !slices.Contains(preset.AvailableSkills, skillKey) {
			t.Errorf("managed Sub-agent available skill %q was not restored: %v", skillKey, preset.AvailableSkills)
		}
	}

	unrelated := enforceManagedCommandAgentCapabilities(model.AgentPresetDefinition{
		Key:          model.AgentPresetSupportAgent,
		AllowedTools: []string{"search_knowledge"},
	})
	if slices.Contains(unrelated.AllowedTools, "scan_semgrep") {
		t.Fatalf("Sub-agent worker tools leaked into unrelated preset: %v", unrelated.AllowedTools)
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
		if !slices.Contains(preset.AllowedTools, "web_search") {
			t.Fatalf("expected preset %q to include web_search, got %v", preset.Key, preset.AllowedTools)
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

func TestWorkspaceSearchPresetAlignment(t *testing.T) {
	searchPresets := map[string]bool{
		model.AgentPresetAskAgent:           true,
		model.AgentPresetEpicPlanner:        true,
		model.AgentPresetTaskPlanner:        true,
		model.AgentPresetCRMOperator:        true,
		model.AgentPresetMarketer:           true,
		model.AgentPresetDocumentationAgent: true,
		model.AgentPresetCommandAgent:       true,
	}
	excludedPresets := map[string]bool{
		model.AgentPresetSupportAgent: true,
		model.AgentPresetCodeBuilder:  true,
		model.AgentPresetReviewAgent:  true,
	}

	for _, preset := range ListAgentPresets() {
		hasSearch := slices.Contains(preset.AllowedTools, "search_workspace")
		switch {
		case searchPresets[preset.Key]:
			if !hasSearch {
				t.Errorf("preset %q is missing search_workspace", preset.Key)
			}
			if preset.SystemPrompt == nil || !strings.Contains(*preset.SystemPrompt, "next_offset") {
				t.Errorf("preset %q is missing workspace search pagination guidance", preset.Key)
			}
			if !strings.Contains(*preset.SystemPrompt, "## Workspace Discovery") {
				t.Errorf("preset %q is missing shared workspace discovery guidance", preset.Key)
			}
			if preset.Key != model.AgentPresetAskAgent {
				if !slices.Contains(preset.AllowedTools, "search_documents") {
					t.Errorf("specialist preset %q must retain search_documents", preset.Key)
				}
			}
		case excludedPresets[preset.Key]:
			if hasSearch {
				t.Errorf("narrow preset %q must not expose search_workspace", preset.Key)
			}
		}
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
	valid := []string{"native_sdk"}
	for _, runtimeKind := range valid {
		if err := validateRuntimeKind(runtimeKind); err != nil {
			t.Fatalf("expected runtime %q to be valid, got %v", runtimeKind, err)
		}
	}

	invalid := []string{"codex", "opencode", "native_claude", "claude_code", "openclaw", "zeroclaw"}
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
			name: "native_sdk code builder supports interactive",
			agent: model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "native_sdk",
			},
			supported: true,
		},
		{
			name: "legacy preset reconciles to native interactive",
			agent: model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "opencode",
			},
			supported: true,
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

func TestValidateRuntimeProviderCompatibilityAllowsNativeAnthropic(t *testing.T) {
	anthropic := model.AgentModelProviderAnthropic
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "native_sdk",
		Provider:    &anthropic,
	}

	svc := &AgentService{}
	if err := svc.validateRuntimeProviderCompatibility(agent); err != nil {
		t.Fatalf("native supports Anthropic: %v", err)
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

func TestValidateModelRoutingRejectsNativeServiceTierForOpenRouter(t *testing.T) {
	openRouter := model.AgentModelProviderOpenRouter
	agent := &model.Agent{
		PresetKey:       model.AgentPresetCodeBuilder,
		RuntimeKind:     "native_sdk",
		Provider:        &openRouter,
		ExecutionConfig: model.JSONBlob(`{"service_tier":"fast"}`),
	}

	svc := &AgentService{openRouterAPIKey: "openrouter-secret"}
	if err := svc.validateModelRouting(agent); err == nil {
		t.Fatal("expected openrouter native_sdk service tier to be rejected")
	}
}

func TestValidateModelRoutingAllowsNativeReasoningConfig(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		PresetKey:       model.AgentPresetCodeBuilder,
		RuntimeKind:     "native_sdk",
		Provider:        &openAI,
		ExecutionConfig: model.JSONBlob(`{"reasoning_effort":"high","service_tier":"fast"}`),
	}

	svc := &AgentService{
		openAIAPIKey: "openai-secret",
	}
	if err := svc.validateModelRouting(agent); err != nil {
		t.Fatalf("expected native_sdk openai execution config to validate, got %v", err)
	}
	if strings.TrimSpace(string(agent.ExecutionConfig)) != `{"reasoning_effort":"high","service_tier":"fast"}` {
		t.Fatalf("expected normalized execution config to persist, got %s", agent.ExecutionConfig)
	}
}

func TestParseAndValidateExecutionConfigAllowsNativeToolStepLimit(t *testing.T) {
	agent := &model.Agent{
		RuntimeKind:     "native_sdk",
		ExecutionConfig: model.JSONBlob(`{"max_tool_steps":640}`),
	}

	config, err := parseAndValidateExecutionConfig(agent)
	if err != nil {
		t.Fatalf("expected native tool step limit to validate, got %v", err)
	}
	if config.MaxToolSteps == nil || *config.MaxToolSteps != 640 {
		t.Fatalf("MaxToolSteps = %#v, want 640", config.MaxToolSteps)
	}
	if got := strings.TrimSpace(string(model.MarshalAgentExecutionConfig(config))); got != `{"max_tool_steps":640}` {
		t.Fatalf("normalized execution config = %s, want max_tool_steps", got)
	}
}

func TestParseAndValidateExecutionConfigAllowsOpenRouterQuantizations(t *testing.T) {
	openRouter := model.AgentModelProviderOpenRouter
	agent := &model.Agent{
		RuntimeKind: "native_sdk",
		Provider:    &openRouter,
		ExecutionConfig: model.JSONBlob(
			`{"openrouter":{"provider":{"quantizations":["fp8","fp16","bf16","fp32"]}}}`,
		),
	}

	config, err := parseAndValidateExecutionConfig(agent)
	if err != nil {
		t.Fatalf("expected OpenRouter quantizations to validate, got %v", err)
	}
	if config.OpenRouter == nil || config.OpenRouter.Provider == nil ||
		!slices.Equal(config.OpenRouter.Provider.Quantizations, defaultFastOpenRouterQuantizations) {
		t.Fatalf("OpenRouter config = %#v", config.OpenRouter)
	}
}

func TestParseAndValidateExecutionConfigRejectsOpenRouterConfigForOtherProvider(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		RuntimeKind: "native_sdk",
		Provider:    &openAI,
		ExecutionConfig: model.JSONBlob(
			`{"openrouter":{"provider":{"quantizations":["fp8"]}}}`,
		),
	}

	_, err := parseAndValidateExecutionConfig(agent)
	if err == nil || !strings.Contains(err.Error(), "only supported for provider openrouter") {
		t.Fatalf("parseAndValidateExecutionConfig() error = %v", err)
	}
}

func TestParseAndValidateExecutionConfigRejectsInvalidNativeToolStepLimit(t *testing.T) {
	tests := []struct {
		name        string
		runtimeKind string
		config      model.JSONBlob
		wantError   string
	}{
		{
			name:        "zero",
			runtimeKind: "native_sdk",
			config:      model.JSONBlob(`{"max_tool_steps":0}`),
			wantError:   "must be between 1 and 2000",
		},
		{
			name:        "above maximum",
			runtimeKind: "native_sdk",
			config:      model.JSONBlob(`{"max_tool_steps":2001}`),
			wantError:   "must be between 1 and 2000",
		},
		{
			name:        "retired runtime",
			runtimeKind: "codex",
			config:      model.JSONBlob(`{"max_tool_steps":500}`),
			wantError:   "only supported for runtime_kind native_sdk",
		},
		{
			name:        "retired model control",
			runtimeKind: "codex",
			config:      model.JSONBlob(`{"reasoning_effort":"high"}`),
			wantError:   "model controls are only supported for runtime_kind native_sdk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &model.Agent{RuntimeKind: tt.runtimeKind, ExecutionConfig: tt.config}
			_, err := parseAndValidateExecutionConfig(agent)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("parseAndValidateExecutionConfig() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestValidateRuntimeForAgentAllowsReviewAgentNativePreset(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		IsSystem:              true,
		PresetKey:             model.AgentPresetReviewAgent,
		RuntimeKind:           "native_sdk",
		Provider:              &openAI,
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	normalizeAgentRecord(agent)

	if err := validateRuntimeForAgent(agent); err != nil {
		t.Fatalf("expected review_agent native_sdk runtime to be allowed, got %v", err)
	}
}

func TestValidateRuntimeForAgentAllowsCustomNativePolicy(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		IsSystem:              true,
		PresetKey:             model.AgentPresetCodeBuilder,
		RuntimeKind:           "native_sdk",
		Provider:              &openAI,
		AllowedTools:          mustJSONStringSlice([]string{"read_file"}),
		AllowedCommands:       mustJSONStringSlice([]string{"go"}),
		AllowedTargets:        mustJSONStringSlice([]string{"task"}),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}

	if err := validateRuntimeForAgent(agent); err != nil {
		t.Fatalf("expected custom native_sdk tool policy to be allowed, got %v", err)
	}
}

func TestNormalizeAgentRecordResetsInvalidRuntimeForPreset(t *testing.T) {
	openAI := model.AgentModelProviderOpenAI
	agent := &model.Agent{
		IsSystem:    true,
		PresetKey:   model.AgentPresetReviewAgent,
		RuntimeKind: "native_sdk",
		Provider:    &openAI,
	}

	normalizeAgentRecord(agent)

	if agent.RuntimeKind != "native_sdk" {
		t.Fatalf("expected review agent runtime to preserve native_sdk, got %q", agent.RuntimeKind)
	}
}

func TestNormalizeAgentRecordUpgradesAskAgentToRiskBasedApproval(t *testing.T) {
	agent := &model.Agent{
		IsSystem:     true,
		PresetKey:    model.AgentPresetAskAgent,
		RuntimeKind:  "native_sdk",
		ApprovalMode: "never",
	}

	normalizeAgentRecord(agent)

	if agent.ApprovalMode != "risk_based" {
		t.Fatalf("Ask Agent approval mode = %q, want risk_based", agent.ApprovalMode)
	}
}

func TestNormalizeAgentRecordRefreshesLegacyCodeBuilderPrompt(t *testing.T) {
	legacyPrompt := "You are Builder, an AI coding agent. You write clean, correct code and follow existing project conventions."
	agent := &model.Agent{
		IsSystem:     true,
		PresetKey:    model.AgentPresetCodeBuilder,
		RuntimeKind:  "native_sdk",
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
		"create_task_batch",
		"ensure_epic_spec_doc",
		"write_document_content",
		"approve_epic_spec",
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
		AllowedTargets: json.RawMessage(`["task"]`),
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
		"create_task_batch",
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
			for _, required := range []string{"read_files", "search_documents", agentcontract.ToolUpdatePlan, agentcontract.ToolRequestApproval} {
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
	for _, required := range []string{"read_files", "search_documents", agentcontract.ToolUpdatePlan, agentcontract.ToolRequestApproval, agentcontract.ToolPublishTaskPlanDoc} {
		if !slices.Contains(tools, required) {
			t.Fatalf("expected sanitized tool list to keep %q, got %v", required, tools)
		}
	}
}
