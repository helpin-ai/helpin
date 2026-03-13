package llm

import (
	"testing"
)

func TestUnmarshalResponse_DirectJSON(t *testing.T) {
	var result []map[string]any
	err := UnmarshalResponse(`[{"key":"value"}]`, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0]["key"] != "value" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnmarshalResponse_FencedJSON(t *testing.T) {
	input := "```json\n{\"name\":\"test\"}\n```"
	var result map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "test" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnmarshalResponse_TextBeforeJSON(t *testing.T) {
	input := "Here is the analysis:\n\n[{\"signal_type\":\"buying_intent\",\"confidence\":0.9}]"
	var result []map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0]["signal_type"] != "buying_intent" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnmarshalResponse_TextAfterJSON(t *testing.T) {
	input := "[]\n\nNo signals were detected in this email."
	var result []map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got: %v", result)
	}
}

func TestUnmarshalResponse_TextAroundJSON(t *testing.T) {
	input := "Looking at this email...\n\n{\"summary\":\"test\"}\n\nThis appears to be a normal message."
	var result map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["summary"] != "test" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnmarshalResponse_NestedJSON(t *testing.T) {
	input := `Some text {"outer":{"inner":[1,2,3]},"key":"val"} more text`
	var result map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "val" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnmarshalResponse_EscapedStrings(t *testing.T) {
	input := `prefix {"evidence":"he said \"hello}\""} suffix`
	var result map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["evidence"] != `he said "hello}"` {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestUnmarshalResponse_Invalid(t *testing.T) {
	err := UnmarshalResponse("this is not json at all", &map[string]any{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshalResponse_EmptyArray(t *testing.T) {
	var result []any
	err := UnmarshalResponse("[]", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got: %v", result)
	}
}

func TestUnmarshalResponse_FencedArray(t *testing.T) {
	input := "```\n[{\"type\":\"a\"}]\n```"
	var result []map[string]any
	err := UnmarshalResponse(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 element, got: %v", result)
	}
}
