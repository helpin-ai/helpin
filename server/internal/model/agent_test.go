package model

import (
	"encoding/json"
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
	if err := json.Unmarshal([]byte(`["approval_protocol","review_agent"]`), &refs); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Key != "approval_protocol" || refs[1].Key != "review_agent" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestAgentSkillRefsValueRoundTrip(t *testing.T) {
	refs := AgentSkillRefs{{Key: "approval_protocol"}, {SkillID: strPtr("skill-1"), Key: "workspace_skill", VersionKey: strPtr("v1")}}
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

func strPtr(value string) *string {
	return &value
}
