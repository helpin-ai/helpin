package handler

import "testing"

func TestGitLabPipelineConclusion(t *testing.T) {
	tests := map[string]string{
		"success":   "success",
		"failed":    "failure",
		"FAILED":    "failure",
		"canceled":  "cancelled",
		" skipped ": "skipped",
	}
	for status, want := range tests {
		if got := gitlabPipelineConclusion(status); got != want {
			t.Errorf("gitlabPipelineConclusion(%q) = %q, want %q", status, got, want)
		}
	}
}
