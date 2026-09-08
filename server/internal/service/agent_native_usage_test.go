package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentNativeUsageReasoningIsNotDoubleBilled(t *testing.T) {
	for _, kind := range []string{"native_sdk", "codex", "opencode"} {
		t.Run(kind, func(t *testing.T) {
			normalized, err := aiusage.NormalizeTokens(agentRunTokenTelemetry(&model.AgentRun{RuntimeKind: kind}, agentRuntimeUsagePayload{InputTokens: 200, CachedInputTokens: 50, OutputTokens: 100, ReasoningOutputTokens: 20}))
			if err != nil {
				t.Fatal(err)
			}
			wantOutput := int64(80)
			if kind == "opencode" {
				wantOutput = 100
			}
			if normalized.OutputTokens != wantOutput || normalized.ReasoningTokens != 20 || normalized.UncachedInputTokens != 150 || normalized.CacheReadTokens != 50 {
				t.Fatalf("incorrect token classes: %+v", normalized)
			}
		})
	}
}

func TestAgentNativeContextPolicyOnlyAcceptedForNative(t *testing.T) {
	for _, kind := range []string{"native_sdk", "codex", "opencode"} {
		agent := &model.Agent{RuntimeKind: kind, ExecutionConfig: model.JSONBlob(`{"native_context":{"enabled":true,"context_window":128000}}`)}
		config, err := parseAndValidateExecutionConfig(agent)
		if kind == "native_sdk" {
			if err != nil || config.NativeContext == nil {
				t.Fatalf("native config lost: %v", err)
			}
		} else if err == nil {
			t.Fatalf("accepted context policy for %s", kind)
		}
	}
}
