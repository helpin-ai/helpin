package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	skillspkg "github.com/helpin-ai/helpin/server/skills"
)

type PresetSkillBundle struct {
	Preamble           string
	SystemPrompt       string
	SkillKeys          []string
	CoreSkillKeys      []string
	AvailableSkillKeys []string
}

var builtInSkillDefinitions = mustLoadBuiltInSkillDefinitions()

var builtInSkillAliases = map[string]string{
	"api_doc_writing":                  "api_reference_doc_writing",
	"api_docs_maintenance":             "api_docs_maintenance",
	"approval_protocol":                "prd_task_plan_approval",
	"code_builder":                     "code_implementation",
	"crm_operator":                     "crm_record_operations",
	"dependency_auditor":               "dependency_audit",
	"docs_api_maintenance":             "api_docs_maintenance",
	"docs_api_reference_writing":       "api_reference_doc_writing",
	"docs_information_architecture":    "docs_architecture_review",
	"docs_internal_maintenance":        "internal_docs_maintenance",
	"docs_public_help_article_writing": "public_help_doc_writing",
	"docs_public_help_maintenance":     "public_help_docs_maintenance",
	"docs_release_update":              "post_release_docs_update",
	"docs_support_gap_update":          "support_gap_docs_update",
	"engineering_code_implementation":  "code_implementation",
	"engineering_code_review":          "code_review",
	"engineering_dependency_auditor":   "dependency_audit",
	"engineering_security_triage":      "security_triage",
	"engineering_task_decomposition":   "coding_task_decomposition",
	"engineering_task_planning_doc":    "coding_task_planning",
	"epic_state_routing":               "epic_planning_state_routing",
	"external_help_doc_writing":        "public_help_doc_writing",
	"general_agent_behavior":           "engineering_planner_operating_rules",
	"marketing_ads_creative":           "ads_creative_planning",
	"marketing_community_partnerships": "community_partnerships_planning",
	"marketing_competitive":            "competitive_positioning",
	"marketing_context":                "marketing_context_setup",
	"marketing_conversion":             "conversion_optimization",
	"marketing_customer_research":      "customer_research_synthesis",
	"marketing_distribution_research":  "distribution_research",
	"marketing_external_research":      "market_research",
	"marketing_launch":                 "launch_marketing",
	"marketing_lead_generation":        "lead_generation_strategy",
	"marketing_lifecycle":              "lifecycle_messaging",
	"marketing_monetization":           "monetization_strategy",
	"marketing_outbound":               "outbound_campaign_planning",
	"marketing_release_marketing":      "release_marketing",
	"marketing_revops":                 "marketing_revops_planning",
	"marketing_seo_content":            "seo_content_strategy",
	"marketing_seo_research":           "seo_research",
	"planning_agent_operating_rules":   "engineering_planner_operating_rules",
	"planning_approval_protocol":       "prd_task_plan_approval",
	"prd_authorship":                   "product_prd_authorship",
	"release_notes_writer":             "release_notes_writing",
	"release_to_docs_update":           "post_release_docs_update",
	"review_agent":                     "code_review",
	"support_agent":                    "support_triage_response",
	"support_gap_to_docs":              "support_gap_docs_update",
	"support_reply_triage":             "support_triage_response",
	"task_decomposition":               "coding_task_decomposition",
	"task_planner_context":             "coding_task_planning",
}

var builtInPresetSkillBundles = map[string]PresetSkillBundle{
	model.AgentPresetEpicPlanner: {
		Preamble:      "You are Atlas, the workspace epic planner. You run the full PRD-to-tasks loop inside a single interactive agent run.",
		SkillKeys:     []string{"prd_task_plan_approval", "product_prd_authorship", "coding_task_decomposition", "epic_planning_state_routing", "engineering_planner_operating_rules"},
		CoreSkillKeys: []string{"prd_task_plan_approval", "product_prd_authorship", "coding_task_decomposition", "epic_planning_state_routing", "engineering_planner_operating_rules"},
	},
	model.AgentPresetTaskPlanner: {
		Preamble:      "You are Scribe, the workspace task planner. You run a focused planning conversation for one task or work item.",
		SkillKeys:     []string{"coding_task_planning", "prd_task_plan_approval", "engineering_planner_operating_rules"},
		CoreSkillKeys: []string{"coding_task_planning", "prd_task_plan_approval", "engineering_planner_operating_rules"},
	},
	model.AgentPresetCRMOperator: {
		Preamble:      "You are Beacon, the workspace CRM operator. You help manage customer records, deal workflows, and sales signals across the workspace.",
		SkillKeys:     []string{"crm_record_operations"},
		CoreSkillKeys: []string{"crm_record_operations"},
	},
	model.AgentPresetSupportAgent: {
		Preamble:      "You are Echo, the workspace support agent. You help triage support conversations, draft replies, and route customer issues.",
		SkillKeys:     []string{"support_triage_response"},
		CoreSkillKeys: []string{"support_triage_response"},
	},
	model.AgentPresetDocumentationAgent: {
		Preamble:     "You are Quill, the workspace documentation agent. You help create, update, and organize internal docs, public help docs, and API docs.",
		SystemPrompt: quillSystemPrompt,
		SkillKeys: []string{
			"docs_architecture_review",
			"public_help_doc_writing",
			"api_reference_doc_writing",
			"internal_docs_maintenance",
			"public_help_docs_maintenance",
			"api_docs_maintenance",
			"post_release_docs_update",
			"support_gap_docs_update",
		},
		AvailableSkillKeys: []string{
			"docs_architecture_review",
			"public_help_doc_writing",
			"api_reference_doc_writing",
			"internal_docs_maintenance",
			"public_help_docs_maintenance",
			"api_docs_maintenance",
			"post_release_docs_update",
			"support_gap_docs_update",
		},
	},
	model.AgentPresetMarketer: {
		Preamble:     "You are Mira, the workspace marketer. You help with positioning, campaigns, copy, lifecycle messaging, launches, conversion ideas, and marketing research.",
		SystemPrompt: miraSystemPrompt,
		SkillKeys: []string{
			"marketing_context_setup",
			"marketing_plan",
			"customer_research_synthesis",
			"marketing_copywriting",
			"conversion_optimization",
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
		},
		AvailableSkillKeys: []string{
			"marketing_context_setup",
			"marketing_plan",
			"customer_research_synthesis",
			"marketing_copywriting",
			"conversion_optimization",
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
		},
	},
	model.AgentPresetCodeBuilder: {
		Preamble:      "You are Forge, the workspace code builder. You use the relevant engineering instructions and skills to make focused, reviewable progress in the repository.",
		SkillKeys:     []string{"code_implementation"},
		CoreSkillKeys: []string{"code_implementation"},
	},
	model.AgentPresetReviewAgent: {
		Preamble:      "You are Lens, the workspace reviewer. You use the relevant review instructions and skills to identify findings, risks, and verification gaps.",
		SkillKeys:     []string{"code_review"},
		CoreSkillKeys: []string{"code_review"},
	},
}

const quillSystemPrompt = `You are Quill, the workspace documentation agent.

Work as a flexible documentation operator across internal docs, public help docs, API docs, release documentation, support-driven documentation, information architecture, and documentation maintenance.

Start by understanding the audience, documentation surface, source of truth, and intended outcome. Use available workspace context, existing docs, tasks, releases, support evidence, API behavior, repository context, public sources, and explicit user input when available. Separate confirmed facts from assumptions, hypotheses, and unresolved gaps.

Choose the right documentation action for the job instead of forcing every request into one format. You may create, update, reorganize, summarize, audit, draft, propose, or identify missing documentation. Prefer strengthening the existing source of truth over creating duplicate or disconnected content.

## Documentation Modes

- Internal docs: preserve useful operational, product, process, and implementation knowledge for the workspace team.
- Public help docs: produce customer-facing education, how-to guidance, troubleshooting, and product explanations.
- API docs: document integration behavior, authentication, permissions, schemas, examples, errors, and edge cases.
- Release docs: translate shipped work into clear documentation updates and customer-visible change notes.
- Support-driven docs: turn repeated customer questions, missing answers, and weak articles into better documentation.
- Information architecture: improve placement, naming, structure, navigation, and discoverability.

## Skill Selection

When available, use list_available_skills or search_available_skills to inspect relevant skill options, then use read_skill only for the specific skill guidance the task needs.
Use information architecture skills when docs need structure, placement, naming, or reorganization.
Use internal docs skills for team-facing operational, product, process, or implementation knowledge.
Use public help docs skills for customer-facing how-to, troubleshooting, onboarding, and product education.
Use API docs skills for endpoints, schemas, authentication, permissions, examples, errors, and integration behavior.
Use release documentation skills when shipped work, changelogs, tasks, or epics require documentation updates.
Use support-gap skills when customer questions or support evidence reveal missing, stale, or weak documentation.

## Operating Rules

Use tools directly when they are available, and stay within the tool and approval policy for the run. If the target surface, source of truth, or requested outcome is unclear, ask before making documentation changes.

For broad, risky, or customer-facing changes, prefer review-ready drafts or proposals before final publication. Make documentation easy to scan, correctly placed, and ready for human review.`

const miraSystemPrompt = `You are Mira, the workspace marketer.

Work as a flexible marketing operator across strategy, research, positioning, messaging, campaigns, lifecycle, launch, content, conversion, competitive, outbound, RevOps, monetization, and release marketing.

Start from available workspace context and user intent. Use product facts, customer language, CRM signals, support signals, docs, tasks, releases, public research, and explicit user input when available. Separate confirmed facts, assumptions, hypotheses, and open questions.

Choose the right marketing angle for the job instead of forcing every request into one format. Produce the artifact that best advances the work: a brief, plan, draft, audit, recommendation, research summary, campaign concept, task backlog, or decision memo.

## Marketing Modes

- Strategy and positioning: clarify audience, category, differentiation, proof, objections, and messaging angles.
- Research: synthesize customer, competitor, market, SEO, distribution, and public-source evidence.
- Copy and campaigns: draft messaging, ads, outbound, lifecycle, launch, release, and content assets.
- Conversion and monetization: analyze funnels, landing pages, activation, pricing, packaging, upgrades, and retention.
- RevOps and sales handoff: connect marketing work to CRM lifecycle, qualification, routing, scoring, pipeline, and sales follow-up.
- Release marketing: translate shipped work into customer-facing narratives, launch plans, changelog entries, and enablement notes.

## Skill Selection

When available, use list_available_skills or search_available_skills to inspect relevant skill options, then use read_skill only for the specific skill guidance the task needs.
Use marketing context skills for product, ICP, personas, positioning, proof points, customer language, and brand voice.
Use planning skills for marketing strategy, 90-day plans, campaign roadmaps, and task backlogs.
Use research skills for customer insights, competitive intelligence, market/category learning, SEO, distribution, and public-source synthesis.
Use copy and campaign skills for messaging, ads, outbound, lifecycle, launch, release marketing, and content briefs.
Use conversion and monetization skills for funnels, landing pages, activation, pricing, packaging, upgrade paths, retention, and win-back work.
Use RevOps skills when marketing work touches CRM lifecycle, qualification, routing, scoring, attribution, sales handoff, or pipeline process.

## Operating Rules

Use tools directly when they are available, and stay within the tool and approval policy for the run. Do not imply access to analytics, ad platforms, SEO tools, email platforms, enrichment tools, private communities, or private external data unless those tools are available in the run.

For broad, risky, or customer-facing work, prefer review-ready drafts, plans, or recommendations before final execution. Make outputs specific, evidence-aware, and easy for a human team to act on.`

func mustLoadBuiltInSkillDefinitions() map[string]SkillDefinition {
	loaded, err := LoadBuiltInSkills(skillspkg.BuiltIn, skillspkg.BuiltInRoot)
	if err != nil {
		panic(fmt.Sprintf("load built-in skills: %v", err))
	}
	skillsByKey := make(map[string]SkillDefinition, len(loaded))
	for _, skill := range loaded {
		skillsByKey[skill.Key] = skill
	}
	return skillsByKey
}

func GetBuiltInSkill(key string) (SkillDefinition, bool) {
	normalized := strings.TrimSpace(key)
	if canonical, ok := builtInSkillAliases[normalized]; ok {
		normalized = canonical
	}
	skill, ok := builtInSkillDefinitions[normalized]
	return skill, ok
}

func ListBuiltInSkills() []SkillDefinition {
	keys := make([]string, 0, len(builtInSkillDefinitions))
	for key := range builtInSkillDefinitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]SkillDefinition, 0, len(keys))
	for _, key := range keys {
		out = append(out, builtInSkillDefinitions[key])
	}
	return out
}

func BuiltInPresetSkillBundleForPreset(presetKey string) (PresetSkillBundle, bool) {
	bundle, ok := builtInPresetSkillBundles[strings.TrimSpace(presetKey)]
	if !ok {
		return PresetSkillBundle{}, false
	}
	bundle.SkillKeys = append([]string(nil), bundle.SkillKeys...)
	bundle.CoreSkillKeys = append([]string(nil), bundle.CoreSkillKeys...)
	if len(bundle.AvailableSkillKeys) == 0 {
		bundle.AvailableSkillKeys = bundle.SkillKeys
	}
	bundle.AvailableSkillKeys = append([]string(nil), bundle.AvailableSkillKeys...)
	return bundle, true
}

func coreSkillKeysForPresetBundle(bundle PresetSkillBundle) []string {
	if len(bundle.CoreSkillKeys) > 0 {
		return bundle.CoreSkillKeys
	}
	return nil
}

func CompileInstructionModules(moduleKeys []string) string {
	sections := make([]string, 0, len(moduleKeys))
	for _, key := range moduleKeys {
		skill, ok := GetBuiltInSkill(key)
		if !ok {
			continue
		}
		instructions := RenderRuntimeToolNamesInInstructions(skill.Instructions)
		if instructions == "" {
			continue
		}
		sections = append(sections, instructions)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func CompilePresetInstructions(preamble string, moduleKeys []string) string {
	sections := make([]string, 0, len(moduleKeys)+1)
	if strings.TrimSpace(preamble) != "" {
		sections = append(sections, strings.TrimSpace(preamble))
	}
	if compiled := CompileInstructionModules(moduleKeys); compiled != "" {
		sections = append(sections, compiled)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func CompilePresetInstructionsWithAvailableSkills(preamble string, coreSkillKeys, availableSkillKeys []string) string {
	sections := make([]string, 0, 3)
	if compiled := CompilePresetInstructions(preamble, coreSkillKeys); compiled != "" {
		sections = append(sections, compiled)
	}
	if len(SortedUniqueStrings(availableSkillKeys)) > 0 {
		sections = append(sections, AvailableSkillPromptGuidance())
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func AvailableSkillPromptGuidance() string {
	return strings.TrimSpace(`## Available Skills

This agent has available skills it can use when they would help or are required for the task. Use the runtime's skill access mechanism to inspect and apply only the specific skill instructions the task needs. Do not load every available skill by default.`)
}

func InstructionTemplateVersionForPreset(preamble string, moduleKeys []string) string {
	compiled := CompilePresetInstructions(preamble, moduleKeys)
	sum := sha256.Sum256([]byte(compiled))
	return hex.EncodeToString(sum[:])[:12]
}

func InstructionTemplateVersionForPresetWithAvailableSkills(preamble string, coreSkillKeys, availableSkillKeys []string) string {
	compiled := CompilePresetInstructionsWithAvailableSkills(preamble, coreSkillKeys, availableSkillKeys)
	sum := sha256.Sum256([]byte(compiled))
	return hex.EncodeToString(sum[:])[:12]
}

func compiledPromptForPresetBundle(bundle PresetSkillBundle) string {
	if strings.TrimSpace(bundle.SystemPrompt) != "" {
		return strings.TrimSpace(bundle.SystemPrompt)
	}
	return CompilePresetInstructionsWithAvailableSkills(bundle.Preamble, coreSkillKeysForPresetBundle(bundle), bundle.AvailableSkillKeys)
}

func BuiltInPresetPrompt(presetKey string) *string {
	bundle, ok := BuiltInPresetSkillBundleForPreset(presetKey)
	if !ok {
		return nil
	}
	compiled := compiledPromptForPresetBundle(bundle)
	return &compiled
}

func BuiltInPresetInstructionTemplateVersion(presetKey string) string {
	bundle, ok := BuiltInPresetSkillBundleForPreset(presetKey)
	if !ok {
		return ""
	}
	compiled := compiledPromptForPresetBundle(bundle)
	sum := sha256.Sum256([]byte(compiled))
	return hex.EncodeToString(sum[:])[:12]
}

func ListSkillCatalog() model.SkillCatalogResponse {
	skills := ListBuiltInSkills()
	entries := make([]model.SkillCatalogEntry, 0, len(skills))
	for _, skill := range skills {
		presets := make([]string, 0, len(builtInPresetSkillBundles))
		for presetKey, bundle := range builtInPresetSkillBundles {
			if slices.Contains(bundle.SkillKeys, skill.Key) {
				presets = append(presets, presetKey)
			}
		}
		sort.Strings(presets)
		entries = append(entries, model.SkillCatalogEntry{
			Key:               skill.Key,
			Title:             skill.Title,
			Description:       skill.Description,
			Instructions:      skill.Instructions,
			SourceKind:        skill.SourceKind,
			RequiredTools:     append([]string(nil), skill.RequiredTools...),
			SupportedRuntimes: append([]string(nil), skill.SupportedRuntimes...),
			Presets:           presets,
		})
	}
	return model.SkillCatalogResponse{Skills: entries}
}
