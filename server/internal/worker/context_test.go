package worker

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDefaultWorkflowConfigForAgent_UsesHigherLimitForNativePlanners(t *testing.T) {
	tests := []struct {
		name  string
		agent *model.Agent
		want  int
	}{
		{
			name: "atlas epic planner",
			agent: &model.Agent{
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			want: plannerWorkflowMaxIterations,
		},
		{
			name: "scribe task planner",
			agent: &model.Agent{
				PresetKey:   model.AgentPresetTaskPlanner,
				RuntimeKind: "native_sdk",
			},
			want: plannerWorkflowMaxIterations,
		},
		{
			name: "support agent unchanged",
			agent: &model.Agent{
				PresetKey:   model.AgentPresetSupportAgent,
				RuntimeKind: "native_sdk",
			},
			want: defaultWorkflowMaxIterations,
		},
		{
			name: "planner on non native runtime unchanged",
			agent: &model.Agent{
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "codex",
			},
			want: defaultWorkflowMaxIterations,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultWorkflowConfigForAgent(tt.agent)
			if config.MaxIterations != tt.want {
				t.Fatalf("MaxIterations = %d, want %d", config.MaxIterations, tt.want)
			}
		})
	}
}
