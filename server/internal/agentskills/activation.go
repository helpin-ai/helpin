package agentskills

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type NativeActiveSelectionContext struct {
	PresetKey     string
	TargetType    string
	PlanningStage string
}

type NativeActiveSelection struct {
	Refs         model.AgentSkillRefs
	Definitions  []worker.SkillDefinition
	Instructions string
}

func SelectNativeActiveSkills(refs model.AgentSkillRefs, definitions []worker.SkillDefinition, ctx NativeActiveSelectionContext) NativeActiveSelection {
	if len(refs) == 0 || len(definitions) == 0 || len(refs) != len(definitions) {
		return NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), refs...),
			Definitions:  append([]worker.SkillDefinition(nil), definitions...),
			Instructions: CompileInstructions(definitions),
		}
	}

	activeBuiltIns, ok := activeBuiltInSkillSet(ctx)
	if !ok {
		return NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), refs...),
			Definitions:  append([]worker.SkillDefinition(nil), definitions...),
			Instructions: CompileInstructions(definitions),
		}
	}

	activeRefs := make(model.AgentSkillRefs, 0, len(refs))
	activeDefs := make([]worker.SkillDefinition, 0, len(definitions))
	for idx, ref := range refs {
		definition := definitions[idx]
		if shouldKeepActiveByDefault(ref, definition) || activeBuiltIns[definition.Key] {
			activeRefs = append(activeRefs, ref)
			activeDefs = append(activeDefs, definition)
		}
	}

	return NativeActiveSelection{
		Refs:         activeRefs,
		Definitions:  activeDefs,
		Instructions: CompileInstructions(activeDefs),
	}
}

func activeBuiltInSkillSet(ctx NativeActiveSelectionContext) (map[string]bool, bool) {
	presetKey := strings.TrimSpace(ctx.PresetKey)
	targetType := strings.TrimSpace(ctx.TargetType)
	planningStage := strings.TrimSpace(ctx.PlanningStage)

	switch presetKey {
	case model.AgentPresetEpicPlanner:
		if targetType != "epic" {
			return nil, false
		}
		switch planningStage {
		case model.PlanningStageDraftSpec:
			return map[string]bool{
				"approval_protocol":  true,
				"epic_state_routing": true,
				"prd_authorship":     true,
			}, true
		case model.PlanningStagePlanTasks:
			return map[string]bool{
				"approval_protocol":  true,
				"epic_state_routing": true,
				"task_decomposition": true,
			}, true
		default:
			return nil, false
		}
	case model.AgentPresetTaskPlanner:
		if targetType != "task" {
			return nil, false
		}
		switch planningStage {
		case model.PlanningStageTaskPlanDoc:
			return map[string]bool{
				"approval_protocol":    true,
				"task_planner_context": true,
			}, true
		default:
			return nil, false
		}
	case model.AgentPresetDocumentationAgent:
		return documentationActiveBuiltInSkillSet(targetType), true
	default:
		return nil, false
	}
}

func documentationActiveBuiltInSkillSet(targetType string) map[string]bool {
	active := map[string]bool{
		"docs_information_architecture": true,
		"general_agent_behavior":        true,
	}
	switch strings.TrimSpace(targetType) {
	case "document":
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
	case "support_conversation", "support_coverage_gap":
		active["support_gap_to_docs"] = true
		active["external_help_doc_writing"] = true
		active["public_help_docs_maintenance"] = true
	case "repository", "task", "epic":
		active["release_to_docs_update"] = true
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
	case "workspace":
		active["external_help_doc_writing"] = true
		active["api_doc_writing"] = true
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
		active["release_to_docs_update"] = true
		active["support_gap_to_docs"] = true
	default:
		active["external_help_doc_writing"] = true
		active["api_doc_writing"] = true
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
		active["release_to_docs_update"] = true
		active["support_gap_to_docs"] = true
	}
	return active
}

func shouldKeepActiveByDefault(ref model.AgentSkillRef, definition worker.SkillDefinition) bool {
	if ref.SkillID != nil {
		return true
	}
	if strings.TrimSpace(definition.SourceKind) != "" && strings.TrimSpace(definition.SourceKind) != "built_in" {
		return true
	}
	return !isPhaseSelectableBuiltInSkill(definition.Key)
}

func isPhaseSelectableBuiltInSkill(key string) bool {
	switch strings.TrimSpace(key) {
	case "approval_protocol", "prd_authorship", "task_decomposition", "epic_state_routing", "task_planner_context":
		return true
	case "docs_information_architecture", "external_help_doc_writing", "api_doc_writing", "internal_docs_maintenance", "public_help_docs_maintenance", "api_docs_maintenance", "release_to_docs_update", "support_gap_to_docs":
		return true
	default:
		return false
	}
}
