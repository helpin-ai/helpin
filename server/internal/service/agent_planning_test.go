package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateSpecClarificationsRequiresAnswersAndReasons(t *testing.T) {
	current := []model.SpecClarificationItem{
		{ID: "open_question_1", Kind: model.SpecClarificationKindOpenQuestion, Prompt: "Who gets access?"},
		{ID: "assumption_1", Kind: model.SpecClarificationKindAssumption, Prompt: "Launch for paid plans only"},
	}

	_, _, err := validateSpecClarifications([]model.SpecClarificationItem{
		{ID: "open_question_1", Disposition: model.SpecClarificationDispositionAnswered, Response: ""},
		{ID: "assumption_1", Disposition: model.SpecClarificationDispositionRejected, Response: ""},
	}, current)
	if err == nil {
		t.Fatal("expected validation error for missing answer/rejection reason")
	}
}

func TestValidateSpecClarificationsCountsPendingItems(t *testing.T) {
	current := []model.SpecClarificationItem{
		{ID: "open_question_1", Kind: model.SpecClarificationKindOpenQuestion, Prompt: "Who gets access?"},
		{ID: "assumption_1", Kind: model.SpecClarificationKindAssumption, Prompt: "Launch for paid plans only"},
	}

	updated, pendingCount, err := validateSpecClarifications([]model.SpecClarificationItem{
		{ID: "open_question_1", Disposition: model.SpecClarificationDispositionAnswered, Response: "Workspace admins only"},
		{ID: "assumption_1", Disposition: model.SpecClarificationDispositionPending},
	}, current)
	if err != nil {
		t.Fatalf("validateSpecClarifications returned error: %v", err)
	}
	if pendingCount != 1 {
		t.Fatalf("expected 1 pending clarification, got %d", pendingCount)
	}
	if len(updated) != 2 || updated[0].Response != "Workspace admins only" {
		t.Fatalf("unexpected updated clarifications: %#v", updated)
	}
}

func TestUpsertSpecClarificationsSectionReplacesExistingSection(t *testing.T) {
	markdown := strings.Join([]string{
		"# Problem",
		"",
		"Base spec body",
		"",
		"## Clarifications",
		"",
		"### Open Questions Resolved",
		"",
		"- Old question -> old answer",
	}, "\n")

	updated := upsertSpecClarificationsSection(markdown, []model.SpecClarificationItem{
		{
			ID:          "open_question_1",
			Kind:        model.SpecClarificationKindOpenQuestion,
			Prompt:      "Who gets access?",
			Disposition: model.SpecClarificationDispositionAnswered,
			Response:    "Workspace admins only",
		},
	})

	if strings.Count(updated, "## Clarifications") != 1 {
		t.Fatalf("expected one clarifications section, got %q", updated)
	}
	if !strings.Contains(updated, "Workspace admins only") {
		t.Fatalf("expected updated clarification answer in markdown, got %q", updated)
	}
	if strings.Contains(updated, "old answer") {
		t.Fatalf("expected old clarifications section to be replaced, got %q", updated)
	}
}
