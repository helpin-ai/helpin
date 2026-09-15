package service

import (
	"context"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func terminalUsageEvent(kind string, input, output int) AgentRuntimeEventEnvelope {
	return AgentRuntimeEventEnvelope{RunID: "runtime", Type: kind, Data: map[string]any{"usage_semantic": agentruntime.UsageSemanticCumulative, "usage": map[string]any{"input_tokens": input, "output_tokens": output, "total_tokens": input + output}}}
}

type conflictingTerminalProjectionRepo struct {
	*fakeAgentRuntimeProjectionRunRepo
}

func (r *conflictingTerminalProjectionRepo) UpdateRuntimeProjection(context.Context, *model.AgentRun) error {
	return repository.ErrAIUsageWatermarkChanged
}
