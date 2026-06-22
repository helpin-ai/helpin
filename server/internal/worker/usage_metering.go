package worker

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentRunUsageRecorder records AI usage for a completed agent runtime run.
// It is optional so non-API workers can run without billing wiring.
type AgentRunUsageRecorder func(ctx context.Context, run *model.AgentRun, agent *model.Agent) error

// AgentRunUsagePreflighter checks whether an agent runtime run may spend AI
// before the runtime starts.
type AgentRunUsagePreflighter func(ctx context.Context, run *model.AgentRun, agent *model.Agent) error

func preflightAgentRunUsage(ctx context.Context, preflighter AgentRunUsagePreflighter, run *model.AgentRun, agent *model.Agent) error {
	if preflighter == nil || run == nil || agent == nil {
		return nil
	}
	return preflighter(ctx, run, agent)
}

func recordAgentRunUsage(ctx context.Context, recorder AgentRunUsageRecorder, run *model.AgentRun, agent *model.Agent) error {
	if recorder == nil || run == nil || agent == nil {
		return nil
	}
	if run.InputTokens <= 0 && run.OutputTokens <= 0 && run.TokensUsed <= 0 {
		return nil
	}
	return recorder(ctx, run, agent)
}
