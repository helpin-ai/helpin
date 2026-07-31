package agentskills

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type NativeActiveSelectionContext struct {
	PresetKey     string
	TargetType    string
	PlanningStage string
}

type NativeActiveSelection struct {
	Refs         model.AgentSkillRefs
	Definitions  []agentcontract.SkillDefinition
	Instructions string
}

func SelectNativeActiveSkills(refs model.AgentSkillRefs, definitions []agentcontract.SkillDefinition, ctx NativeActiveSelectionContext) NativeActiveSelection {
	if len(refs) == 0 || len(definitions) == 0 || len(refs) != len(definitions) {
		return NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), refs...),
			Definitions:  append([]agentcontract.SkillDefinition(nil), definitions...),
			Instructions: CompileInstructions(definitions),
		}
	}

	activeBuiltIns, ok := activeBuiltInSkillSet(ctx)
	if !ok {
		return NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), refs...),
			Definitions:  append([]agentcontract.SkillDefinition(nil), definitions...),
			Instructions: CompileInstructions(definitions),
		}
	}

	activeRefs := make(model.AgentSkillRefs, 0, len(refs))
	activeDefs := make([]agentcontract.SkillDefinition, 0, len(definitions))
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
				"prd_task_plan_approval":      true,
				"epic_planning_state_routing": true,
				"product_prd_authorship":      true,
			}, true
		case model.PlanningStagePlanTasks:
			return map[string]bool{
				"prd_task_plan_approval":      true,
				"epic_planning_state_routing": true,
				"coding_task_decomposition":   true,
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
				"prd_task_plan_approval": true,
				"coding_task_planning":   true,
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
		"docs_architecture_review": true,
	}
	switch strings.TrimSpace(targetType) {
	case "document":
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
	case "support_conversation", "support_coverage_gap":
		active["support_gap_docs_update"] = true
		active["public_help_doc_writing"] = true
		active["public_help_docs_maintenance"] = true
	case "repository", "task", "epic":
		active["post_release_docs_update"] = true
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
	case "workspace":
		active["public_help_doc_writing"] = true
		active["api_reference_doc_writing"] = true
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
		active["post_release_docs_update"] = true
		active["support_gap_docs_update"] = true
	default:
		active["public_help_doc_writing"] = true
		active["api_reference_doc_writing"] = true
		active["internal_docs_maintenance"] = true
		active["public_help_docs_maintenance"] = true
		active["api_docs_maintenance"] = true
		active["post_release_docs_update"] = true
		active["support_gap_docs_update"] = true
	}
	return active
}

func shouldKeepActiveByDefault(ref model.AgentSkillRef, definition agentcontract.SkillDefinition) bool {
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
	case "prd_task_plan_approval", "product_prd_authorship", "coding_task_decomposition", "epic_planning_state_routing", "coding_task_planning":
		return true
	case "docs_architecture_review", "public_help_doc_writing", "api_reference_doc_writing", "internal_docs_maintenance", "public_help_docs_maintenance", "api_docs_maintenance", "post_release_docs_update", "support_gap_docs_update":
		return true
	default:
		return false
	}
}
