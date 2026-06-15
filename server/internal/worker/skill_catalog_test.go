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
		"dependency_auditor",
		"security_triage",
		"external_help_doc_writing",
		"api_doc_writing",
		"internal_docs_maintenance",
		"public_help_docs_maintenance",
		"api_docs_maintenance",
		"docs_information_architecture",
		"release_to_docs_update",
		"support_gap_to_docs",
	} {
		if !containsString(keys, key) {
			t.Fatalf("expected built-in skill %q in registry, got %v", key, keys)
		}
	}
}

func TestDocumentationSkillsDeclareExpectedGuidance(t *testing.T) {
	cases := map[string][]string{
		"external_help_doc_writing": {
			"Write for customers and end users",
			"Do not publish directly",
			"Place the article in the most specific existing collection",
		},
		"api_doc_writing": {
			"Document authentication, permissions, request shape, response shape, errors, and examples",
			"Do not invent endpoints, fields, limits, or SDK behavior",
			"Include at least one realistic request example and one realistic response example",
		},
		"internal_docs_maintenance": {
			"Internal docs may include implementation details",
			"Prefer updating the existing source of truth",
			"Preserve operational details",
		},
		"public_help_docs_maintenance": {
			"Preserve stable public URLs and slugs unless a redirect plan exists",
			"Avoid exposing internal implementation details",
			"Do not silently publish customer-facing changes",
		},
		"api_docs_maintenance": {
			"Check for changed endpoints, parameters, response fields, errors, auth, rate limits, pagination, and version notes",
			"Mark deprecations and breaking changes explicitly",
			"Keep examples synchronized with the documented schema",
		},
		"docs_information_architecture": {
			"Organize docs into spaces, collections, and subcollections",
			"Avoid duplicate articles unless the audience or workflow is genuinely different",
			"Maintain naming, ordering, and related-link consistency",
		},
		"release_to_docs_update": {
			"Map shipped changes to internal docs, public help docs, and API docs",
			"Separate user-visible behavior from internal operational changes",
			"Call out uncertainty instead of filling gaps with guesses",
		},
		"support_gap_to_docs": {
			"Read the gap evidence before deciding what to write",
			"Decide whether the gap needs a new article, an update to an existing article, or an information architecture change",
			"Do not close or mark a gap resolved until the doc work is actually created, updated, or explicitly handed off",
		},
	}

	for key, snippets := range cases {
		skill, ok := GetBuiltInSkill(key)
		if !ok {
			t.Fatalf("expected %s built-in skill", key)
		}
		if !containsString(skill.SupportedRuntimes, "native_sdk") {
			t.Fatalf("expected %s to support native_sdk, got %v", key, skill.SupportedRuntimes)
		}
		for _, snippet := range snippets {
			if !strings.Contains(skill.Instructions, snippet) {
				t.Fatalf("expected %s instructions to contain %q\n%s", key, snippet, skill.Instructions)
			}
		}
	}
}

func TestSecurityTriageSkillReferencesScannerWorkflow(t *testing.T) {
	skill, ok := GetBuiltInSkill("security_triage")
	if !ok {
		t.Fatal("expected security_triage built-in skill")
	}
	for _, snippet := range []string{
		"`scan_semgrep`",
		"`scan_trivy`",
		"`scan_gitleaks`",
		"Do not run scanner CLIs through `run_command`",
		"Repository file tools such as `list_directory`, `read_file`, and `search_files` are scoped to the checked-out repository",
		"Let the scanner tools handle bundled rules, caches, and scanner-native fallbacks",
		"Scanner output is evidence, not truth",
		"Create tasks only for findings classified as `applicable`",
		"prewarms Trivy vulnerability databases",
		"seed writable runtime caches under `/tmp/helpin-security-cache`",
	} {
		if !strings.Contains(skill.Instructions, snippet) {
			t.Fatalf("expected security triage instructions to contain %q\n%s", snippet, skill.Instructions)
		}
	}
}

func TestDependencyAuditorSkillReferencesExecutionDocs(t *testing.T) {
	skill, ok := GetBuiltInSkill("dependency_auditor")
	if !ok {
		t.Fatal("expected dependency_auditor built-in skill")
	}
	for _, snippet := range []string{
		"ecosystems/go.md",
		"ecosystems/rust.md",
		"ecosystems/python.md",
		"ecosystems/node.md",
		"ecosystems/java.md",
		"verification.md",
		"Compare against direct manifest declarations",
	} {
		if !strings.Contains(skill.Instructions, snippet) {
			t.Fatalf("expected dependency auditor instructions to contain %q\n%s", snippet, skill.Instructions)
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
		"Approval requests happen inline in the same chat.",
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

func TestTaskPlannerBundleUsesTaskPlanDocSkillStack(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetTaskPlanner)
	if !ok {
		t.Fatal("expected task planner bundle")
	}
	if !containsString(bundle.SkillKeys, "task_planner_context") {
		t.Fatalf("expected task planner context skill, got %v", bundle.SkillKeys)
	}
	if containsString(bundle.SkillKeys, "task_decomposition") {
		t.Fatalf("did not expect epic task-plan skill in task planner bundle, got %v", bundle.SkillKeys)
	}
}

func TestReviewAgentBundleDoesNotIncludePlannerSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetReviewAgent)
	if !ok {
		t.Fatal("expected review bundle")
	}
	for _, forbidden := range []string{"prd_authorship", "task_decomposition", "epic_state_routing", "task_planner_context"} {
		if containsString(bundle.SkillKeys, forbidden) {
			t.Fatalf("did not expect planner skill %q in review bundle: %v", forbidden, bundle.SkillKeys)
		}
	}
}

func TestSupportAgentBundleDoesNotIncludePlannerSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetSupportAgent)
	if !ok {
		t.Fatal("expected support bundle")
	}
	for _, forbidden := range []string{"prd_authorship", "task_decomposition", "epic_state_routing", "task_planner_context"} {
		if containsString(bundle.SkillKeys, forbidden) {
			t.Fatalf("did not expect planner skill %q in support bundle: %v", forbidden, bundle.SkillKeys)
		}
	}
}

func TestDocumentationAgentBundleIncludesDocumentationSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetDocumentationAgent)
	if !ok {
		t.Fatal("expected documentation agent bundle")
	}
	for _, expected := range []string{
		"docs_information_architecture",
		"external_help_doc_writing",
		"api_doc_writing",
		"internal_docs_maintenance",
		"public_help_docs_maintenance",
		"api_docs_maintenance",
		"release_to_docs_update",
		"support_gap_to_docs",
		"general_agent_behavior",
	} {
		if !containsString(bundle.SkillKeys, expected) {
			t.Fatalf("expected documentation skill %q in bundle: %v", expected, bundle.SkillKeys)
		}
	}
	if strings.Contains(bundle.Preamble, "Helpin") || strings.Contains(bundle.Preamble, "helpin") {
		t.Fatalf("documentation preamble must use workspace context, got %q", bundle.Preamble)
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
		if !containsString(skill.RequiredTools, ToolRequestApproval) {
			t.Fatalf("expected request approval requirement, got %v", skill.RequiredTools)
		}
		if !containsString(skill.Presets, model.AgentPresetEpicPlanner) || !containsString(skill.Presets, model.AgentPresetTaskPlanner) {
			t.Fatalf("expected planner preset mappings, got %v", skill.Presets)
		}
	}
	if !found {
		t.Fatal("expected approval_protocol in skill catalog")
	}
}

func TestReviewAgentSkillDeclaresCompletionInteractionPolicy(t *testing.T) {
	skill, ok := GetBuiltInSkill("review_agent")
	if !ok {
		t.Fatal("expected review_agent built-in skill")
	}
	if len(skill.RequiredTools) != 0 {
		t.Fatalf("expected review_agent to avoid transport-specific required tools, got %v", skill.RequiredTools)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, model.AgentRunInteractionKindReviewCheckpoint) {
		t.Fatalf("expected review checkpoint completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, ToolRequestUserInput) {
		t.Fatalf("expected user input completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	contract, ok := skill.Policy.InteractionContract(InteractionKindReviewCheckpoint)
	if !ok {
		t.Fatal("expected review_checkpoint interaction contract")
	}
	if contract.Schema != "review_checkpoint_v1" {
		t.Fatalf("expected review checkpoint schema, got %q", contract.Schema)
	}
	if contract.Transports["native_sdk"].ToolName != HelpinMCPRuntimeToolName(ToolRequestReviewCheckpoint) {
		t.Fatalf("expected native_sdk review checkpoint tool transport, got %+v", contract.Transports["native_sdk"])
	}
	if contract.Transports["codex"].BlockLabel != "helpin-review" {
		t.Fatalf("expected codex review checkpoint block label, got %+v", contract.Transports["codex"])
	}
	inputContract, ok := skill.Policy.InteractionContract(InteractionKindRequestUserInput)
	if !ok {
		t.Fatal("expected request_user_input interaction contract")
	}
	if inputContract.Schema != "request_user_input_v1" {
		t.Fatalf("expected request user input schema, got %q", inputContract.Schema)
	}
	if inputContract.Transports["native_sdk"].ToolName != HelpinMCPRuntimeToolName(ToolRequestUserInput) {
		t.Fatalf("expected native_sdk request_user_input tool transport, got %+v", inputContract.Transports["native_sdk"])
	}
	if inputContract.Transports["codex"].Type != InteractionTransportTypeRuntimeBridge {
		t.Fatalf("expected codex request_user_input runtime bridge transport, got %+v", inputContract.Transports["codex"])
	}
}

func TestApprovalProtocolSkillDeclaresPlannerCompletionInteractionPolicy(t *testing.T) {
	skill, ok := GetBuiltInSkill("approval_protocol")
	if !ok {
		t.Fatal("expected approval_protocol built-in skill")
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, InteractionKindApprovalRequest) {
		t.Fatalf("expected approval request completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, InteractionKindRequestUserInput) {
		t.Fatalf("expected user input completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	contract, ok := skill.Policy.InteractionContract(InteractionKindApprovalRequest)
	if !ok {
		t.Fatal("expected approval_request interaction contract")
	}
	if contract.Transports["native_sdk"].ToolName != HelpinMCPRuntimeToolName(ToolRequestApproval) {
		t.Fatalf("expected native_sdk approval request tool transport, got %+v", contract.Transports["native_sdk"])
	}
	inputContract, ok := skill.Policy.InteractionContract(InteractionKindRequestUserInput)
	if !ok {
		t.Fatal("expected request_user_input interaction contract")
	}
	if inputContract.Transports["native_sdk"].ToolName != HelpinMCPRuntimeToolName(ToolRequestUserInput) {
		t.Fatalf("expected native_sdk request_user_input tool transport, got %+v", inputContract.Transports["native_sdk"])
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
