package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestNormalizeObjectiveState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "canonical not_started", input: "not_started", want: model.PMObjectiveStateNotStarted, ok: true},
		{name: "legacy to_do", input: "to_do", want: model.PMObjectiveStateNotStarted, ok: true},
		{name: "label to do", input: "To Do", want: model.PMObjectiveStateNotStarted, ok: true},
		{name: "canonical active", input: "active", want: model.PMObjectiveStateActive, ok: true},
		{name: "legacy in_progress", input: "in_progress", want: model.PMObjectiveStateActive, ok: true},
		{name: "label in progress", input: "In Progress", want: model.PMObjectiveStateActive, ok: true},
		{name: "canonical closed", input: "closed", want: model.PMObjectiveStateClosed, ok: true},
		{name: "legacy done", input: "done", want: model.PMObjectiveStateClosed, ok: true},
		{name: "label completed", input: "Completed", want: model.PMObjectiveStateClosed, ok: true},
		{name: "invalid", input: "paused", want: "", ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := normalizeObjectiveState(tt.input)
			if ok != tt.ok {
				t.Fatalf("normalizeObjectiveState(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("normalizeObjectiveState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
