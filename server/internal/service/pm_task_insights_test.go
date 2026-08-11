package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTaskUpdateCursorRoundTrip(t *testing.T) {
	want := time.Date(2026, time.August, 11, 12, 34, 56, 789, time.UTC)
	cursor := encodeTaskUpdateCursor(want)
	got, err := decodeTaskUpdateCursor(cursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("decoded time = %v, want %v", got, want)
	}
}

func TestNormalizeTaskStandingBriefOutput(t *testing.T) {
	output := normalizeTaskStandingBriefOutput(taskStandingBriefGenerationOutput{
		Narrative: "  A grounded summary.  ",
		Suggestions: []model.TaskStandingBriefSuggestion{
			{Label: "Retry the failed run", Action: model.TaskStandingBriefSuggestionAction{Type: "retry_run", RunID: "run-1"}},
			{Label: "Invent an action", Action: model.TaskStandingBriefSuggestionAction{Type: "delete_task"}},
			{Label: "   ", Action: model.TaskStandingBriefSuggestionAction{Type: "open_related_object"}},
		},
	})

	if output.Narrative != "A grounded summary." {
		t.Fatalf("narrative = %q", output.Narrative)
	}
	if len(output.Suggestions) != 1 {
		t.Fatalf("suggestion count = %d, want 1", len(output.Suggestions))
	}
	if output.Suggestions[0].Key == "" {
		t.Fatal("expected a stable suggestion key")
	}
	wantKey := output.Suggestions[0].Key
	again := normalizeTaskStandingBriefOutput(taskStandingBriefGenerationOutput{Suggestions: []model.TaskStandingBriefSuggestion{
		{Label: "Retry the failed run", Action: model.TaskStandingBriefSuggestionAction{Type: "retry_run", RunID: "run-1"}},
	}})
	if again.Suggestions[0].Key != wantKey {
		t.Fatalf("suggestion key changed: %q != %q", again.Suggestions[0].Key, wantKey)
	}
}

func TestIsCommentLifecycleActivity(t *testing.T) {
	for _, action := range []string{"comment_created", "comment_updated", "block_commented"} {
		if !isCommentLifecycleActivity(action) {
			t.Fatalf("expected %q to be treated as comment lifecycle activity", action)
		}
	}
	if isCommentLifecycleActivity("workflow_state_changed") {
		t.Fatal("state changes must remain in the updates feed")
	}
}
