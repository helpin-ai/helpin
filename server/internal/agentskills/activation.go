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
		case model.PlanningStageTaskPlanDoc, model.PlanningStageStoryPlanDoc:
			return map[string]bool{
				"approval_protocol":    true,
				"task_planner_context": true,
			}, true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
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
	default:
		return false
	}
}
