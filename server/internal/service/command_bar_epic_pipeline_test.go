package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestEpicPipelineDeliveryTargetIsMerged(t *testing.T) {
	merged := "merged"
	open := "open"
	epicA := "epic-a"
	epicB := "epic-b"

	tests := []struct {
		name   string
		target model.TaskDeliveryTarget
		epicID string
		want   bool
	}{
		{
			name:   "merged state without source epic counts",
			target: model.TaskDeliveryTarget{DeliveryState: "merged"},
			epicID: epicA,
			want:   true,
		},
		{
			name:   "merged PR status counts",
			target: model.TaskDeliveryTarget{ActivePRStatus: &merged},
			epicID: epicA,
			want:   true,
		},
		{
			name:   "merged into another epic does not count",
			target: model.TaskDeliveryTarget{DeliveryState: "merged", SourceEpicID: &epicB},
			epicID: epicA,
			want:   false,
		},
		{
			name:   "merged into the same epic counts",
			target: model.TaskDeliveryTarget{DeliveryState: "merged", SourceEpicID: &epicA},
			epicID: epicA,
			want:   true,
		},
		{
			name:   "open target does not count",
			target: model.TaskDeliveryTarget{DeliveryState: "open", ActivePRStatus: &open},
			epicID: epicA,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := epicPipelineDeliveryTargetIsMerged(tt.target, tt.epicID); got != tt.want {
				t.Errorf("epicPipelineDeliveryTargetIsMerged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEpicPipelineTaskTitle(t *testing.T) {
	if got := epicPipelineTaskTitle(model.PMTask{DisplayID: 12, Name: "Ship it"}); got != "#12 Ship it" {
		t.Errorf("epicPipelineTaskTitle() = %q, want %q", got, "#12 Ship it")
	}
	if got := epicPipelineTaskTitle(model.PMTask{Name: "Untracked"}); got != "Untracked" {
		t.Errorf("epicPipelineTaskTitle() = %q, want %q", got, "Untracked")
	}
}
