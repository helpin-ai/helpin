package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentRunInputPayloadSetTaskTargetUsesCanonicalContract(t *testing.T) {
	payload := &AgentRunInputPayload{}
	payload.SetTarget("task", "task-1")

	if payload.Target == nil || payload.Target.TargetType != "task" || payload.Target.TargetID != "task-1" {
		t.Fatalf("target = %#v, want canonical task target", payload.Target)
	}
	if payload.TaskID != "task-1" {
		t.Fatalf("task_id = %q, want task-1", payload.TaskID)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if strings.Contains(string(raw), "story") {
		t.Fatalf("canonical payload contains Story-era contract: %s", raw)
	}
}

func TestAgentRunInputPayloadRejectsStoryTarget(t *testing.T) {
	payload := &AgentRunInputPayload{}
	payload.SetTarget("story", "legacy-1")

	if payload.Target != nil || payload.TaskID != "" {
		t.Fatalf("Story-era target must be rejected, got %#v", payload)
	}
}
