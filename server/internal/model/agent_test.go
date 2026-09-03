package model

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestJSONBlobValueReturnsJSONString(t *testing.T) {
	value, err := JSONBlob(`{"reasoning_effort":"high"}`).Value()
	if err != nil {
		t.Fatalf("Value() returned error: %v", err)
	}

	text, ok := value.(string)
	if !ok {
		t.Fatalf("Value() type = %T, want string", value)
	}
	if text != `{"reasoning_effort":"high"}` {
		t.Fatalf("Value() = %q, want %q", text, `{"reasoning_effort":"high"}`)
	}
}

func TestAgentSkillRefsUnmarshalLegacyStringArray(t *testing.T) {
	var refs AgentSkillRefs
	if err := json.Unmarshal([]byte(`["prd_task_plan_approval","code_review"]`), &refs); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Key != "prd_task_plan_approval" || refs[1].Key != "code_review" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestAgentSkillRefsValueRoundTrip(t *testing.T) {
	refs := AgentSkillRefs{{Key: "prd_task_plan_approval"}, {SkillID: strPtr("skill-1"), Key: "workspace_skill", VersionKey: strPtr("v1")}}
	value, err := refs.Value()
	if err != nil {
		t.Fatalf("Value returned error: %v", err)
	}
	var restored AgentSkillRefs
	if err := restored.Scan(value); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(restored) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(restored))
	}
	if restored[1].SkillID == nil || *restored[1].SkillID != "skill-1" {
		t.Fatalf("expected skill_id to round-trip, got %+v", restored[1])
	}
}

func TestAgentExecutionConfigNormalizesOpenRouterQuantizations(t *testing.T) {
	config, err := ParseAgentExecutionConfig([]byte(`{
		"openrouter":{"provider":{"quantizations":[" fp8 ","FP16","bf16","fp32","fp8",""]}}
	}`))
	if err != nil {
		t.Fatalf("ParseAgentExecutionConfig returned error: %v", err)
	}
	if config.OpenRouter == nil || config.OpenRouter.Provider == nil {
		t.Fatalf("OpenRouter config = %#v", config.OpenRouter)
	}
	want := []string{"fp8", "fp16", "bf16", "fp32"}
	if !slices.Equal(config.OpenRouter.Provider.Quantizations, want) {
		t.Fatalf("quantizations = %v, want %v", config.OpenRouter.Provider.Quantizations, want)
	}
	if got := string(MarshalAgentExecutionConfig(config)); got != `{"openrouter":{"provider":{"quantizations":["fp8","fp16","bf16","fp32"]}}}` {
		t.Fatalf("MarshalAgentExecutionConfig = %s", got)
	}
}

func strPtr(value string) *string {
	return &value
}
