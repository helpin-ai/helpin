package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestRuntimeTurnPolicy(t *testing.T) {
	askAgent := &model.Agent{PresetKey: model.AgentPresetAskAgent}
	commandAgent := &model.Agent{PresetKey: model.AgentPresetCommandAgent}

	tests := []struct {
		name            string
		run             *model.AgentRun
		agent           *model.Agent
		mode            string
		idleSeconds     int
		wantMode        string
		wantIdleSeconds int
		wantCompletion  string
		wantCorrections int
	}{
		{
			name:     "nil run completes on finish",
			run:      nil,
			agent:    askAgent,
			mode:     model.InvocationModeInteractive,
			wantMode: agentRuntimeTurnCompleteOnFinish,
		},
		{
			name:            "ask_agent uses chat loop with default idle timeout",
			run:             &model.AgentRun{TargetType: "workspace"},
			agent:           askAgent,
			mode:            model.InvocationModeInteractive,
			wantMode:        agentRuntimeTurnPauseAfterAssist,
			wantIdleSeconds: defaultDockChatIdleTimeoutSeconds,
			wantCompletion:  agentRuntimeTurnCompletionExplicit,
			wantCorrections: askAgentCompletionCorrections,
		},
		{
			name:            "ask_agent chat loop regardless of invocation mode",
			run:             &model.AgentRun{TargetType: "workspace"},
			agent:           askAgent,
			mode:            model.InvocationModeAutonomous,
			wantMode:        agentRuntimeTurnPauseAfterAssist,
			wantIdleSeconds: defaultDockChatIdleTimeoutSeconds,
			wantCompletion:  agentRuntimeTurnCompletionExplicit,
			wantCorrections: askAgentCompletionCorrections,
		},
		{
			name:            "ask_agent honors idle override",
			run:             &model.AgentRun{TargetType: "workspace"},
			agent:           askAgent,
			mode:            model.InvocationModeInteractive,
			idleSeconds:     600,
			wantMode:        agentRuntimeTurnPauseAfterAssist,
			wantIdleSeconds: 600,
			wantCompletion:  agentRuntimeTurnCompletionExplicit,
			wantCorrections: askAgentCompletionCorrections,
		},
		{
			name: "support chat trigger gets chat loop with 24h idle timeout",
			run: &model.AgentRun{
				TargetType: "support_conversation",
				Input:      []byte(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`),
			},
			agent:           commandAgent,
			mode:            model.InvocationModeInteractive,
			wantMode:        agentRuntimeTurnPauseAfterAssist,
			wantIdleSeconds: defaultSupportChatIdleTimeoutSeconds,
		},
		{
			name: "manual support run keeps complete_on_finish when autonomous",
			run: &model.AgentRun{
				TargetType: "support_conversation",
				Input:      []byte(`{"trigger":{"source":"manual","trigger_type":"manual"}}`),
			},
			agent:    commandAgent,
			mode:     model.InvocationModeAutonomous,
			wantMode: agentRuntimeTurnCompleteOnFinish,
		},
		{
			name:     "interactive support conversation uses chat loop without idle timeout",
			run:      &model.AgentRun{TargetType: "support_conversation"},
			agent:    commandAgent,
			mode:     model.InvocationModeInteractive,
			wantMode: agentRuntimeTurnPauseAfterAssist,
		},
		{
			name:     "autonomous support conversation completes on finish",
			run:      &model.AgentRun{TargetType: "support_conversation"},
			agent:    commandAgent,
			mode:     model.InvocationModeAutonomous,
			wantMode: agentRuntimeTurnCompleteOnFinish,
		},
		{
			name:     "interactive non-conversational target completes on finish",
			run:      &model.AgentRun{TargetType: "task"},
			agent:    commandAgent,
			mode:     model.InvocationModeInteractive,
			wantMode: agentRuntimeTurnCompleteOnFinish,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runtimeTurnPolicy(tt.run, tt.agent, tt.mode, tt.idleSeconds)
			if got.Mode != tt.wantMode {
				t.Errorf("runtimeTurnPolicy() mode = %q, want %q", got.Mode, tt.wantMode)
			}
			if got.IdleTimeoutSeconds != tt.wantIdleSeconds {
				t.Errorf("runtimeTurnPolicy() idle = %d, want %d", got.IdleTimeoutSeconds, tt.wantIdleSeconds)
			}
			if got.CompletionMode != tt.wantCompletion || got.MaxCompletionCorrections != tt.wantCorrections {
				t.Errorf("runtimeTurnPolicy() completion = %q/%d, want %q/%d", got.CompletionMode, got.MaxCompletionCorrections, tt.wantCompletion, tt.wantCorrections)
			}
		})
	}
}

func TestRuntimeResumeTurnPolicyOnlyUpgradesDockChatRuns(t *testing.T) {
	chatID := "chat-1"
	policy := runtimeResumeTurnPolicy(&model.AgentRun{DockChatID: &chatID})
	if policy == nil || policy.Mode != agentRuntimeTurnPauseAfterAssist || policy.CompletionMode != agentRuntimeTurnCompletionExplicit || policy.MaxCompletionCorrections != askAgentCompletionCorrections {
		t.Fatalf("unexpected Ask resume policy: %#v", policy)
	}
	if got := runtimeResumeTurnPolicy(&model.AgentRun{TargetType: "support_conversation"}); got != nil {
		t.Fatalf("support chat must preserve implicit completion on resume, got %#v", got)
	}
}
