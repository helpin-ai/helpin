package temporalapp

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

const (
	executionContractEpicPlanning = "epic_planning"
	executionContractTaskPlanning = "task_planning"
)

func resolveExecutionContract(run *model.AgentRun, agent *model.Agent) string {
	if run == nil || agent == nil {
		return ""
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetEpicPlanner:
		if strings.TrimSpace(run.TargetType) == "epic" {
			return executionContractEpicPlanning
		}
	case model.AgentPresetTaskPlanner:
		if strings.TrimSpace(run.TargetType) == "task" {
			return executionContractTaskPlanning
		}
	}
	return ""
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

func selectActiveContractSkills(state *resolvedRunState, planningStage string) agentskills.NativeActiveSelection {
	if state == nil {
		return agentskills.NativeActiveSelection{}
	}
	if !state.executionContractActive {
		return agentskills.NativeActiveSelection{
			Refs:         append(model.AgentSkillRefs(nil), state.runtimeSkillRefs...),
			Definitions:  append([]workerpkg.SkillDefinition(nil), state.runtimeSkillDefinitions...),
			Instructions: agentskills.CompileInstructions(state.runtimeSkillDefinitions),
		}
	}
	planningStage = contractSkillPlanningStage(state, planningStage)
	return agentskills.SelectNativeActiveSkills(state.runtimeSkillRefs, state.runtimeSkillDefinitions, agentskills.NativeActiveSelectionContext{
		PresetKey:     strings.TrimSpace(state.agent.EffectivePresetKey()),
		TargetType:    strings.TrimSpace(state.run.TargetType),
		PlanningStage: strings.TrimSpace(planningStage),
	})
}

func contractSkillPlanningStage(state *resolvedRunState, planningStage string) string {
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
	if state == nil || !state.executionContractActive {
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
