package agentcontract

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
		"prd_task_plan_approval",
		"product_prd_authorship",
		"coding_task_decomposition",
		"epic_planning_state_routing",
		"engineering_planner_operating_rules",
		"coding_task_planning",
		"code_implementation",
		"code_review",
		"crm_record_operations",
		"support_triage_response",
		"dependency_audit",
		"security_triage",
		"public_help_doc_writing",
		"api_reference_doc_writing",
		"internal_docs_maintenance",
		"public_help_docs_maintenance",
		"api_docs_maintenance",
		"docs_architecture_review",
		"post_release_docs_update",
		"support_gap_docs_update",
		"release_notes_writing",
		"competitors_changelog_tracking_report",
		"marketing_context_setup",
		"marketing_plan",
		"marketing_copywriting",
		"conversion_optimization",
		"customer_research_synthesis",
		"lifecycle_messaging",
		"launch_marketing",
		"seo_content_strategy",
		"competitive_positioning",
		"lead_generation_strategy",
		"outbound_campaign_planning",
		"ads_creative_planning",
		"community_partnerships_planning",
		"marketing_revops_planning",
		"monetization_strategy",
		"market_research",
		"competitor_research",
		"distribution_research",
		"seo_research",
		"release_marketing",
	} {
		if !containsString(keys, key) {
			t.Fatalf("expected built-in skill %q in registry, got %v", key, keys)
		}
	}
}

func TestGetBuiltInSkillAcceptsCompetitiveDigestAlias(t *testing.T) {
	skill, ok := GetBuiltInSkill("competitive_intelligence_digest")
	if !ok {
		t.Fatal("expected legacy competitive intelligence skill alias to resolve")
	}
	if skill.Key != "competitors_changelog_tracking_report" {
		t.Fatalf("expected canonical competitors changelog skill, got %q", skill.Key)
	}
	if CanonicalBuiltInSkillKey("system/competitive_intelligence_digest") != "competitors_changelog_tracking_report" {
		t.Fatalf("expected canonical key alias for system/competitive_intelligence_digest")
	}
}

func TestMiraBundleIncludesMarketingSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetMarketer)
	if !ok {
		t.Fatal("expected Mira marketer bundle")
	}
	for _, expected := range []string{
		"marketing_context_setup",
		"marketing_plan",
		"marketing_copywriting",
		"conversion_optimization",
		"customer_research_synthesis",
		"lifecycle_messaging",
		"launch_marketing",
		"seo_content_strategy",
		"competitive_positioning",
		"lead_generation_strategy",
		"outbound_campaign_planning",
		"ads_creative_planning",
		"community_partnerships_planning",
		"marketing_revops_planning",
		"monetization_strategy",
		"market_research",
		"competitor_research",
		"distribution_research",
		"seo_research",
		"release_marketing",
	} {
		if !containsString(bundle.SkillKeys, expected) {
			t.Fatalf("expected Mira skill %q in bundle: %v", expected, bundle.SkillKeys)
		}
	}
	if containsString(bundle.SkillKeys, "engineering_planner_operating_rules") {
		t.Fatalf("Mira bundle should not include engineering planner rules: %v", bundle.SkillKeys)
	}
	prompt := BuiltInPresetPrompt(model.AgentPresetMarketer)
	if prompt == nil {
		t.Fatal("expected Mira marketer prompt")
	}
	for _, snippet := range []string{
		"You are Mira, the workspace marketer.",
		"## Marketing Modes",
		"## Skill Selection",
		"When available, use list_available_skills or search_available_skills to inspect relevant skill options, then use read_skill only for the specific skill guidance the task needs.",
		"Use marketing context skills for product, ICP, personas, positioning, proof points, customer language, and brand voice.",
		"Use RevOps skills when marketing work touches CRM lifecycle",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected Mira prompt to contain %q\n%s", snippet, *prompt)
		}
	}
	for _, fullSkillBodySnippet := range []string{
		"Use this skill when Mira needs foundational marketing context",
		"Capture these sections:",
		"Use AARRR as the default structure:",
		"For each sequence:",
		"Do not send emails or claim an email-platform integration exists.",
	} {
		if strings.Contains(*prompt, fullSkillBodySnippet) {
			t.Fatalf("expected Mira prompt to omit full skill body snippet %q\n%s", fullSkillBodySnippet, *prompt)
		}
	}
}

func TestEchoSystemPromptPrefersCanonicalWebsiteSourcesOverBlogs(t *testing.T) {
	for _, guidance := range []string{
		"Search results include their source URL and authority when available",
		"Prefer curated and canonical evidence over standard and secondary evidence",
		"comparison, alternative, blog, news, announcement, campaign, or audience pages",
	} {
		if !strings.Contains(echoSystemPrompt, guidance) {
			t.Fatalf("Echo prompt missing source precedence guidance %q:\n%s", guidance, echoSystemPrompt)
		}
	}
}

func TestEchoSystemPromptUsesCustomerSafeResearchFallback(t *testing.T) {
	for _, guidance := range []string{
		"never mention a knowledge base",
		"search only the official product website",
		"implementation-specific behavior",
		"Call start_agent_run first",
		"Do not send the interim reply before launching",
		"required_confidence",
		"best_possible_grounded_confidence",
	} {
		if !strings.Contains(echoSystemPrompt, guidance) {
			t.Fatalf("Echo prompt missing fallback guidance %q:\n%s", guidance, echoSystemPrompt)
		}
	}
}

func TestEnsureSupportRuntimeDeliveryContractProtectsWorkspacePromptCopies(t *testing.T) {
	staleSnapshot := "You are Echo.\n\n## Answer quality\nAnswer from evidence."
	prompt := EnsureSupportRuntimeDeliveryContract(model.AgentPresetSupportAgent, staleSnapshot)
	for _, required := range []string{
		staleSnapshot,
		"Required live-support delivery contract",
		"one successful call to send_support_reply",
		"Plain assistant text is never delivered",
		"status sent, escalated, or suppressed is terminal",
		"Never mention a knowledge base",
		"official website",
		"implementation-specific questions",
		"required_confidence",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("support runtime prompt missing %q:\n%s", required, prompt)
		}
	}
	if duplicated := EnsureSupportRuntimeDeliveryContract(model.AgentPresetSupportAgent, prompt); duplicated != prompt {
		t.Fatal("support runtime contract should be idempotent")
	}
	if got := EnsureSupportRuntimeDeliveryContract(model.AgentPresetMarketer, staleSnapshot); got != staleSnapshot {
		t.Fatalf("non-support prompt changed: %q", got)
	}
}

func TestDocumentationSkillsDeclareExpectedGuidance(t *testing.T) {
	cases := map[string][]string{
		"public_help_doc_writing": {
			"Write for customers and end users",
			"Do not publish directly",
			"Place the article in the most specific existing collection",
		},
		"api_reference_doc_writing": {
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
		"docs_architecture_review": {
			"Organize docs into spaces, collections, and subcollections",
			"Avoid duplicate articles unless the audience or workflow is genuinely different",
			"Maintain naming, ordering, and related-link consistency",
		},
		"post_release_docs_update": {
			"Map shipped changes to internal docs, public help docs, and API docs",
			"Separate user-visible behavior from internal operational changes",
			"Call out uncertainty instead of filling gaps with guesses",
		},
		"support_gap_docs_update": {
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
	skill, ok := GetBuiltInSkill("dependency_audit")
	if !ok {
		t.Fatal("expected dependency_audit built-in skill")
	}
	for _, snippet := range []string{
		"ecosystems/go.md",
		"ecosystems/rust.md",
		"ecosystems/python.md",
		"ecosystems/node.md",
		"ecosystems/java.md",
		"verification.md",
		"Compare against direct manifest declarations",
		"Use `fetch_url` for exact public registry and advisory API GET requests",
		"Do not invoke `curl` or `wget` through `run_command`",
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
		"You are Atlas, the workspace epic planner.",
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

func TestBuiltInPresetBundleDoesNotDefaultCoreSkillsIntoAvailableSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetEpicPlanner)
	if !ok {
		t.Fatal("expected epic planner bundle")
	}
	if len(bundle.CoreSkillKeys) == 0 {
		t.Fatalf("expected core skill keys, got %#v", bundle)
	}
	for _, coreKey := range bundle.CoreSkillKeys {
		if containsString(bundle.AvailableSkillKeys, coreKey) {
			t.Fatalf("core skill %q should not be advertised as available, got %v", coreKey, bundle.AvailableSkillKeys)
		}
	}
}

func TestTaskPlannerBundleUsesTaskPlanDocSkillStack(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetTaskPlanner)
	if !ok {
		t.Fatal("expected task planner bundle")
	}
	if !containsString(bundle.SkillKeys, "coding_task_planning") {
		t.Fatalf("expected task planner context skill, got %v", bundle.SkillKeys)
	}
	if containsString(bundle.SkillKeys, "coding_task_decomposition") {
		t.Fatalf("did not expect epic task-plan skill in task planner bundle, got %v", bundle.SkillKeys)
	}
	skill, ok := GetBuiltInSkill("coding_task_planning")
	if !ok || !containsString(skill.SupportedRuntimes, "codex") {
		t.Fatalf("task planning skill must support Codex, got %#v", skill.SupportedRuntimes)
	}
}

func TestReviewAgentBundleDoesNotIncludePlannerSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetReviewAgent)
	if !ok {
		t.Fatal("expected review bundle")
	}
	for _, forbidden := range []string{"product_prd_authorship", "coding_task_decomposition", "epic_planning_state_routing", "coding_task_planning"} {
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
	for _, forbidden := range []string{"product_prd_authorship", "coding_task_decomposition", "epic_planning_state_routing", "coding_task_planning"} {
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
		"docs_architecture_review",
		"public_help_doc_writing",
		"api_reference_doc_writing",
		"internal_docs_maintenance",
		"public_help_docs_maintenance",
		"api_docs_maintenance",
		"post_release_docs_update",
		"support_gap_docs_update",
	} {
		if !containsString(bundle.SkillKeys, expected) {
			t.Fatalf("expected documentation skill %q in bundle: %v", expected, bundle.SkillKeys)
		}
	}
	if containsString(bundle.SkillKeys, "engineering_planner_operating_rules") {
		t.Fatalf("documentation bundle should not include engineering planner rules: %v", bundle.SkillKeys)
	}
	if strings.Contains(bundle.Preamble, "Helpin") || strings.Contains(bundle.Preamble, "helpin") {
		t.Fatalf("documentation preamble must use workspace context, got %q", bundle.Preamble)
	}
}

func TestDocumentationAgentPromptUsesCuratedSkillSelectionGuide(t *testing.T) {
	prompt := BuiltInPresetPrompt(model.AgentPresetDocumentationAgent)
	if prompt == nil {
		t.Fatal("expected documentation agent prompt")
	}
	for _, snippet := range []string{
		"You are Quill, the workspace documentation agent.",
		"## Documentation Modes",
		"## Skill Selection",
		"When available, use list_available_skills or search_available_skills to inspect relevant skill options, then use read_skill only for the specific skill guidance the task needs.",
		"Use information architecture skills when docs need structure",
		"Use public help docs skills for customer-facing how-to",
		"Use API docs skills for endpoints, schemas, authentication",
		"Use support-gap skills when customer questions or support evidence reveal missing, stale, or weak documentation.",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected Quill prompt to contain %q\n%s", snippet, *prompt)
		}
	}
	for _, fullSkillBodySnippet := range []string{
		"Use this skill when organizing or reorganizing documentation.",
		"Place the article in the most specific existing collection",
		"Document authentication, permissions, request shape, response shape, errors, and examples",
		"Do not close or mark a gap resolved until the doc work is actually created, updated, or explicitly handed off.",
	} {
		if strings.Contains(*prompt, fullSkillBodySnippet) {
			t.Fatalf("expected Quill prompt to omit full skill body snippet %q\n%s", fullSkillBodySnippet, *prompt)
		}
	}
}

func TestCompilePresetInstructionsWithAvailableSkillsUsesRuntimeNeutralGuidance(t *testing.T) {
	prompt := CompilePresetInstructionsWithAvailableSkills(
		"You are a code agent.",
		[]string{"code_implementation"},
		[]string{"dependency_audit"},
	)

	for _, forbidden := range []string{"list_available_skills", "search_available_skills", "read_skill"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("compiled preset prompt should not mention native-only tool %q\n%s", forbidden, prompt)
		}
	}
	for _, expected := range []string{
		"## Available Skills",
		"Use the runtime's skill access mechanism",
		"Do not load every available skill by default.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected compiled prompt to contain %q\n%s", expected, prompt)
		}
	}
}

func TestSystemAgentPresetPreamblesUsePersonaIdentities(t *testing.T) {
	expected := map[string]string{
		model.AgentPresetEpicPlanner:        "You are Atlas, the workspace epic planner. You run the full PRD-to-tasks loop inside a single interactive agent run.",
		model.AgentPresetTaskPlanner:        "You are Scribe, the workspace task planner. You run a focused planning conversation for one task or work item.",
		model.AgentPresetCodeBuilder:        "You are Forge, the workspace code builder. You use the relevant engineering instructions and skills to make focused, reviewable progress in the repository.",
		model.AgentPresetReviewAgent:        "You are Lens, the workspace reviewer. You use the relevant review instructions and skills to identify findings, risks, and verification gaps.",
		model.AgentPresetCRMOperator:        "You are Beacon, the workspace CRM operator. You help manage customer records, deal workflows, and sales signals across the workspace.",
		model.AgentPresetSupportAgent:       "You are Echo, the workspace support agent. You help triage support conversations, draft replies, and route customer issues.",
		model.AgentPresetDocumentationAgent: "You are Quill, the workspace documentation agent. You help create, update, and organize internal docs, public help docs, and API docs.",
		model.AgentPresetMarketer:           "You are Mira, the workspace marketer. You help with positioning, campaigns, copy, lifecycle messaging, launches, conversion ideas, and marketing research.",
	}

	for presetKey, want := range expected {
		bundle, ok := BuiltInPresetSkillBundleForPreset(presetKey)
		if !ok {
			t.Fatalf("expected bundle for %s", presetKey)
		}
		if bundle.Preamble != want {
			t.Fatalf("preamble for %s = %q, want %q", presetKey, bundle.Preamble, want)
		}
	}
}

func TestEchoPromptDefaultsGenericProductReferencesToWorkspace(t *testing.T) {
	prompt := BuiltInPresetPrompt(model.AgentPresetSupportAgent)
	if prompt == nil {
		t.Fatal("expected Echo system prompt")
	}
	for _, expected := range []string{
		"product whose website the visitor is currently using",
		`"your plans" to the current workspace`,
		"Do not ask which product they mean unless they explicitly named or compared another product.",
	} {
		if !strings.Contains(*prompt, expected) {
			t.Fatalf("Echo prompt does not contain %q\n%s", expected, *prompt)
		}
	}
}

func TestListSkillCatalogReturnsBuiltInSkills(t *testing.T) {
	catalog := ListSkillCatalog()
	if len(catalog.Skills) == 0 {
		t.Fatal("expected built-in skills in catalog")
	}
	found := false
	for _, skill := range catalog.Skills {
		if skill.Key != "prd_task_plan_approval" {
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
		t.Fatal("expected prd_task_plan_approval in skill catalog")
	}
}

func TestReviewAgentSkillDeclaresCompletionInteractionPolicy(t *testing.T) {
	skill, ok := GetBuiltInSkill("code_review")
	if !ok {
		t.Fatal("expected code_review built-in skill")
	}
	if len(skill.RequiredTools) != 0 {
		t.Fatalf("expected code_review to avoid transport-specific required tools, got %v", skill.RequiredTools)
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
	if contract.Transports["codex"].Type != InteractionTransportTypeToolCall || contract.Transports["codex"].ToolName != HelpinMCPRuntimeToolName(ToolRequestReviewCheckpoint) {
		t.Fatalf("expected codex review checkpoint tool transport, got %+v", contract.Transports["codex"])
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
	skill, ok := GetBuiltInSkill("prd_task_plan_approval")
	if !ok {
		t.Fatal("expected prd_task_plan_approval built-in skill")
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
