package model

import (
	"encoding/json"
	"testing"
)

func TestAgentNativeContextConfigRoundTrip(t *testing.T) {
	var config AgentExecutionConfig
	if err := json.Unmarshal([]byte(`{"native_context":{"enabled":true,"context_window":128000,"max_total_tokens":1000000,"user_anchor_tokens":2000}}`), &config); err != nil {
		t.Fatal(err)
	}
	if config.IsZero() || config.NativeContext == nil {
		t.Fatal("native context lost")
	}
	if err := config.NativeContext.Validate(); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var restored AgentExecutionConfig
	if err := json.Unmarshal(body, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.NativeContext == nil || *restored.NativeContext != *config.NativeContext {
		t.Fatalf("roundtrip: %s", body)
	}
	for _, bad := range []AgentNativeContextConfig{{Enabled: true}, {Enabled: true, ContextWindow: 4096, InputLimit: 4096}, {MaxTotalTokens: -1}, {UserAnchorTokens: -1}, {Enabled: true, ContextWindow: 32000, UserAnchorTokens: 64000}, {Enabled: true, ContextWindow: 32000, TriggerTokens: 500, KeepRecentTokens: 500}} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("unsafe config accepted: %+v", bad)
		}
	}
}
