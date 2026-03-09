package model

import (
	"encoding/json"
	"testing"
)

func TestJSONB_Value_Nil(t *testing.T) {
	var j JSONB
	val, err := j.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if s != "{}" {
		t.Fatalf("expected '{}', got %q", s)
	}
	// Verify it's valid JSON
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("nil JSONB produced invalid JSON: %v", err)
	}
}

func TestJSONB_Value_Empty(t *testing.T) {
	j := JSONB{}
	val, err := j.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if s != "{}" {
		t.Fatalf("expected '{}', got %q", s)
	}
}

func TestJSONB_Value_WithData(t *testing.T) {
	j := JSONB{"title": "Test Story", "count": float64(42)}
	val, err := j.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("produced invalid JSON: %v", err)
	}
	if m["title"] != "Test Story" {
		t.Fatalf("expected title 'Test Story', got %v", m["title"])
	}
	if m["count"] != float64(42) {
		t.Fatalf("expected count 42, got %v", m["count"])
	}
}

func TestJSONB_Scan_Nil(t *testing.T) {
	var j JSONB
	if err := j.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j == nil {
		t.Fatal("expected non-nil JSONB after scanning nil")
	}
	if len(j) != 0 {
		t.Fatalf("expected empty map, got %v", j)
	}
}

func TestJSONB_Scan_Bytes(t *testing.T) {
	var j JSONB
	if err := j.Scan([]byte(`{"key":"value"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j["key"] != "value" {
		t.Fatalf("expected key='value', got %v", j["key"])
	}
}

func TestJSONB_Scan_String(t *testing.T) {
	var j JSONB
	if err := j.Scan(`{"key":"value"}`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j["key"] != "value" {
		t.Fatalf("expected key='value', got %v", j["key"])
	}
}

func TestJSONB_Scan_InvalidType(t *testing.T) {
	var j JSONB
	if err := j.Scan(12345); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestJSONB_Roundtrip(t *testing.T) {
	original := JSONB{"title": "Test WS", "nested": map[string]interface{}{"a": float64(1)}}
	val, err := original.Value()
	if err != nil {
		t.Fatalf("unexpected error on Value: %v", err)
	}

	var restored JSONB
	if err := restored.Scan(val.(string)); err != nil {
		t.Fatalf("unexpected error on Scan: %v", err)
	}

	if restored["title"] != "Test WS" {
		t.Fatalf("expected title 'Test WS', got %v", restored["title"])
	}
}
