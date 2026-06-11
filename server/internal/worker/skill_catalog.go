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
	Preamble     string
	SystemPrompt string
	SkillKeys    []string
}

var builtInSkillDefinitions = mustLoadBuiltInSkillDefinitions()

var builtInPresetSkillBundles = map[string]PresetSkillBundle{
	model.AgentPresetEpicPlanner: {
		Preamble:  "You are Atlas, the workspace epic planner. You run the full PRD-to-tasks loop inside a single interactive agent run.",
		SkillKeys: []string{"approval_protocol", "prd_authorship", "task_decomposition", "epic_state_routing", "general_agent_behavior"},
	},
	model.AgentPresetTaskPlanner: {
		Preamble:  "You are Scribe, the workspace task planner. You run a focused planning conversation for one task or work item.",
		SkillKeys: []string{"task_planner_context", "approval_protocol", "general_agent_behavior"},
	},
	model.AgentPresetCRMOperator: {
		Preamble:  "You are Beacon, the workspace CRM operator. You help manage customer records, deal workflows, and sales signals across the workspace.",
		SkillKeys: []string{"crm_operator"},
	},
	model.AgentPresetSupportAgent: {
		Preamble:  "You are Echo, the workspace support agent. You help triage support conversations, draft replies, and route customer issues.",
		SkillKeys: []string{"support_agent"},
	},
	model.AgentPresetDocumentationAgent: {
		Preamble:     "You are Quill, the workspace documentation agent. You help create, update, and organize internal docs, public help docs, and API docs.",
		SystemPrompt: quillSystemPrompt,
		SkillKeys: []string{
			"docs_information_architecture",
			"external_help_doc_writing",
			"api_doc_writing",
			"internal_docs_maintenance",
			"public_help_docs_maintenance",
			"api_docs_maintenance",
			"release_to_docs_update",
			"support_gap_to_docs",
			"general_agent_behavior",
		},
	},
	model.AgentPresetMarketer: {
		Preamble:     "You are Mira, the workspace marketer. You help with positioning, campaigns, copy, lifecycle messaging, launches, conversion ideas, and marketing research.",
		SystemPrompt: miraSystemPrompt,
		SkillKeys: []string{
			"marketing_context",
			"marketing_plan",
			"marketing_customer_research",
			"marketing_copywriting",
			"marketing_conversion",
			"marketing_lifecycle",
			"marketing_launch",
			"marketing_seo_content",
			"marketing_competitive",
			"marketing_lead_generation",
			"marketing_outbound",
			"marketing_ads_creative",
			"marketing_community_partnerships",
			"marketing_revops",
			"marketing_monetization",
			"marketing_external_research",
			"marketing_competitor_research",
			"marketing_distribution_research",
			"marketing_seo_research",
			"marketing_release_marketing",
			"general_agent_behavior",
		},
	},
	model.AgentPresetCodeBuilder: {
		Preamble:  "You are Forge, the workspace code builder. You use the relevant engineering instructions and skills to make focused, reviewable progress in the repository.",
		SkillKeys: []string{"code_builder"},
	},
	model.AgentPresetReviewAgent: {
		Preamble:  "You are Lens, the workspace reviewer. You use the relevant review instructions and skills to identify findings, risks, and verification gaps.",
		SkillKeys: []string{"review_agent"},
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
	skill, ok := builtInSkillDefinitions[strings.TrimSpace(key)]
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
	return bundle, true
}

func CompileInstructionModules(moduleKeys []string) string {
	sections := make([]string, 0, len(moduleKeys))
	for _, key := range moduleKeys {
		skill, ok := GetBuiltInSkill(key)
		if !ok {
			continue
		}
		instructions := strings.TrimSpace(skill.Instructions)
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

func InstructionTemplateVersionForPreset(preamble string, moduleKeys []string) string {
	compiled := CompilePresetInstructions(preamble, moduleKeys)
	sum := sha256.Sum256([]byte(compiled))
	return hex.EncodeToString(sum[:])[:12]
}

func compiledPromptForPresetBundle(bundle PresetSkillBundle) string {
	if strings.TrimSpace(bundle.SystemPrompt) != "" {
		return strings.TrimSpace(bundle.SystemPrompt)
	}
	return CompilePresetInstructions(bundle.Preamble, bundle.SkillKeys)
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
