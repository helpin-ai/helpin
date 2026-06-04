package temporalapp

import (
	"os"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func nativeSelectivePlannerPathRolloutEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED"))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func resolveNativeSelectivePlannerPathEnabled(run *model.AgentRun, agent *model.Agent) bool {
	if run == nil || agent == nil {
		return false
	}
	if !nativeSelectivePlannerPathRolloutEnabled() {
		return false
	}
	if !agent.IsSystem {
		return false
	}
	runtimeKind := firstNonEmptyString(strings.TrimSpace(run.RuntimeKind), strings.TrimSpace(agent.RuntimeKind))
	if runtimeKind != "native_sdk" {
		return false
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetEpicPlanner:
		return strings.TrimSpace(run.TargetType) == "epic"
	case model.AgentPresetTaskPlanner:
		return strings.TrimSpace(run.TargetType) == "task"
	case model.AgentPresetDocumentationAgent:
		return true
	default:
		return false
	}
}

func runtimeSkillRefKeys(refs model.AgentSkillRefs) []string {
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		key := strings.TrimSpace(ref.Key)
		if key == "" && ref.SkillID != nil && strings.TrimSpace(*ref.SkillID) != "" {
			key = "workspace:" + strings.TrimSpace(*ref.SkillID)
		}
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func selectNativeActiveSkills(state *resolvedRunState, planningStage string) agentskills.NativeActiveSelection {
	if state == nil {
		return agentskills.NativeActiveSelection{}
	}
	if !state.nativeSelectivePathEnabled {
		return agentskills.NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), state.runtimeSkillRefs...),
			Definitions:  append([]workerpkg.SkillDefinition(nil), state.runtimeSkillDefinitions...),
			Instructions: agentskills.CompileInstructions(state.runtimeSkillDefinitions),
		}
	}
	planningStage = nativeActiveSkillPlanningStage(state, planningStage)
	return agentskills.SelectNativeActiveSkills(state.runtimeSkillRefs, state.runtimeSkillDefinitions, agentskills.NativeActiveSelectionContext{
		PresetKey:     strings.TrimSpace(state.agent.EffectivePresetKey()),
		TargetType:    strings.TrimSpace(state.run.TargetType),
		PlanningStage: strings.TrimSpace(planningStage),
	})
}

func nativeActiveSkillPlanningStage(state *resolvedRunState, planningStage string) string {
	planningStage = strings.TrimSpace(planningStage)
	if planningStage != "" || state == nil || state.run == nil {
		return planningStage
	}

	switch strings.TrimSpace(state.run.TargetType) {
	case "epic":
		if state.epic == nil {
			return planningStage
		}
		if strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID)) != "" {
			return model.PlanningStagePlanTasks
		}
		switch strings.TrimSpace(state.epic.PlanningState) {
		case model.EpicPlanningStateReadyForTaskPlanning,
			model.EpicPlanningStateAwaitingPlanApproval,
			model.EpicPlanningStateTasksCreated,
			model.EpicPlanningStateExecutionStarted,
			model.EpicPlanningStateReadyForExecution:
			return model.PlanningStagePlanTasks
		default:
			return model.PlanningStageDraftSpec
		}
	case "task":
		if state.task != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetTaskPlanner {
			return model.PlanningStageTaskPlanDoc
		}
	}

	return planningStage
}

func effectiveExecutionSkillPolicy(state *resolvedRunState, selection agentskills.NativeActiveSelection) workerpkg.SkillPolicy {
	if state == nil || !state.nativeSelectivePathEnabled {
		if state == nil {
			return workerpkg.SkillPolicy{}
		}
		return state.skillPolicy
	}
	if len(selection.Definitions) == 0 && len(state.runtimeSkillDefinitions) > 0 {
		return state.skillPolicy
	}
	return agentskills.AggregatePolicy(selection.Definitions)
}
