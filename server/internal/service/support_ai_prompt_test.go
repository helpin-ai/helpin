package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildAISystemPrompt_IncludesOutOfScopeFallbackGuidance(t *testing.T) {
	prompt := buildAISystemPrompt(&model.Agent{Name: "Support Bot"}, "")

	if !strings.Contains(prompt, "third-party tool recommendations/comparisons") {
		t.Fatalf("prompt missing out-of-scope comparison guidance: %s", prompt)
	}

	if !strings.Contains(prompt, "acknowledging the limitation and redirecting back to supported questions") {
		t.Fatalf("prompt missing safe fallback guidance: %s", prompt)
	}
}
