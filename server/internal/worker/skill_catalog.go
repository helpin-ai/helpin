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
	Preamble  string
	SkillKeys []string
}

var builtInSkillDefinitions = mustLoadBuiltInSkillDefinitions()

var builtInPresetSkillBundles = map[string]PresetSkillBundle{
	model.AgentPresetEpicPlanner: {
		Preamble:  "You are Epic Planner. You run the full PRD-to-tasks loop inside a single interactive agent run.",
		SkillKeys: []string{"approval_protocol", "prd_authorship", "task_decomposition", "epic_state_routing", "general_agent_behavior"},
	},
	model.AgentPresetTaskPlanner: {
		Preamble:  "You are Task Planner. Run a single interactive planning conversation for one task.",
		SkillKeys: []string{"task_planner_context", "approval_protocol", "general_agent_behavior"},
	},
	model.AgentPresetCRMOperator: {
		Preamble:  "You are CRM Operator.",
		SkillKeys: []string{"crm_operator"},
	},
	model.AgentPresetSupportAgent: {
		Preamble:  "You are Support Agent.",
		SkillKeys: []string{"support_agent"},
	},
	model.AgentPresetDocumentationAgent: {
		Preamble: "You are Documentation Agent. Keep the current workspace's internal docs, public help docs, and API docs accurate, organized, and current. First identify the documentation surface: internal docs, public help center, API docs, or multiple surfaces. If the surface or source of truth is ambiguous, ask for clarification before changing docs. Use the workspace name from runtime context when a product, company, or workspace name is needed. Prefer drafts, proposals, and review checkpoints before customer-facing publication.",
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
	model.AgentPresetCodeBuilder: {
		Preamble:  "You are Code Builder. Use the relevant available engineering instructions and skills for the task, then make focused, reviewable progress in the repository.",
		SkillKeys: []string{"code_builder"},
	},
	model.AgentPresetReviewAgent: {
		Preamble:  "You are Review Agent. Use the relevant available review instructions and skills for the task, and prioritize clear findings, risks, and verification gaps.",
		SkillKeys: []string{"review_agent"},
	},
}

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

func BuiltInPresetPrompt(presetKey string) *string {
	bundle, ok := BuiltInPresetSkillBundleForPreset(presetKey)
	if !ok {
		return nil
	}
	compiled := CompilePresetInstructions(bundle.Preamble, bundle.SkillKeys)
	return &compiled
}

func BuiltInPresetInstructionTemplateVersion(presetKey string) string {
	bundle, ok := BuiltInPresetSkillBundleForPreset(presetKey)
	if !ok {
		return ""
	}
	return InstructionTemplateVersionForPreset(bundle.Preamble, bundle.SkillKeys)
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
