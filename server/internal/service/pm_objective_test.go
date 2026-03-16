package service

import (
	"testing"
	"time"

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

func TestComputeSuggestedHealth(t *testing.T) {
	t.Parallel()

	makeObjective := func(start, end time.Time, keyResultCount int, avgProgress float64) *model.ObjectiveWithDetails {
		return &model.ObjectiveWithDetails{
			Objective: model.PMObjective{
				PlannedStartDate: &start,
				Deadline:         &end,
			},
			Stats: model.PMObjectiveStats{
				KeyResultCount:  keyResultCount,
				KeyResultAvgPct: avgProgress,
			},
		}
	}

	t.Run("no dates returns on_track", func(t *testing.T) {
		objective := &model.ObjectiveWithDetails{
			Stats: model.PMObjectiveStats{KeyResultCount: 3, KeyResultAvgPct: 0},
		}
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("no key results returns on_track", func(t *testing.T) {
		objective := &model.ObjectiveWithDetails{
			Stats: model.PMObjectiveStats{KeyResultCount: 0},
		}
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("before start returns on_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 27, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 4, 0)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("start day with no progress remains on_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 2, 0)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 18, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("deadline day is not automatically overdue", func(t *testing.T) {
		start := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 2, 50)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("after deadline with incomplete work returns off_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 4, 75)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 8, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOffTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOffTrack)
		}
	})

	t.Run("gap thresholds map to on_track at_risk and off_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 19, 0, 0, 0, 0, time.UTC)
		now := time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC)

		onTrack := makeObjective(start, end, 4, 40) // expected 50%, actual 40%, gap 10
		if result := computeSuggestedHealthAt(onTrack, now); result != model.PMObjectiveHealthOnTrack {
			t.Errorf("on_track got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}

		atRisk := makeObjective(start, end, 4, 30) // expected 50%, actual 30%, gap 20
		if result := computeSuggestedHealthAt(atRisk, now); result != model.PMObjectiveHealthAtRisk {
			t.Errorf("at_risk got %q, want %q", result, model.PMObjectiveHealthAtRisk)
		}

		offTrack := makeObjective(start, end, 4, 20) // expected 50%, actual 20%, gap 30
		if result := computeSuggestedHealthAt(offTrack, now); result != model.PMObjectiveHealthOffTrack {
			t.Errorf("off_track got %q, want %q", result, model.PMObjectiveHealthOffTrack)
		}
	})
}
