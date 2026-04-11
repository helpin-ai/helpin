package service

import (
	"encoding/json"
	"testing"
)

func TestStructuredReviewNextStepResumeContent(t *testing.T) {
	requestPayload, err := json.Marshal(map[string]any{
		"questions": []map[string]any{
			{
				"id":       "next_step",
				"question": "What should I do next with this review?",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	t.Run("implement changes option becomes direct instruction", func(t *testing.T) {
		responsePayload, err := json.Marshal(map[string]any{
			"answers": map[string]any{
				"next_step": map[string]any{
					"answers": []string{"Implement changes"},
				},
			},
		})
		if err != nil {
			t.Fatalf("marshal response payload: %v", err)
		}

		got := structuredReviewNextStepResumeContent(requestPayload, responsePayload)
		if got == "" || got == "Implement changes" {
			t.Fatalf("expected expanded instruction, got %q", got)
		}
	})

	t.Run("other reply is preserved verbatim", func(t *testing.T) {
		responsePayload, err := json.Marshal(map[string]any{
			"answers": map[string]any{
				"next_step": map[string]any{
					"answers": []string{"Focus only on findings 1 and 3, then stop for my review."},
				},
			},
		})
		if err != nil {
			t.Fatalf("marshal response payload: %v", err)
		}

		got := structuredReviewNextStepResumeContent(requestPayload, responsePayload)
		want := "Focus only on findings 1 and 3, then stop for my review."
		if got != want {
			t.Fatalf("structuredReviewNextStepResumeContent() = %q, want %q", got, want)
		}
	})
}
