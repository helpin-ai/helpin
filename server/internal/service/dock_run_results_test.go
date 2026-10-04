package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBoundedDockSummaryIsRuneSafe(t *testing.T) {
	content := strings.Repeat("界", 8)
	summary, charCount, truncated := boundedDockSummary(content, 5)
	if !truncated {
		t.Fatal("expected summary to be truncated")
	}
	if charCount != 8 {
		t.Fatalf("charCount = %d, want 8", charCount)
	}
	if got := len([]rune(summary)); got != 5 {
		t.Fatalf("summary rune count = %d, want 5 (%q)", got, summary)
	}
	if summary != "界界界界…" {
		t.Fatalf("summary = %q, want rune-safe ellipsis", summary)
	}
}

func TestDockRunResultWindowPaginatesByRune(t *testing.T) {
	first := dockRunResultWindow("a界b界c", 0, 3)
	if first.Content != "a界b" || first.CharCount != 5 || !first.Truncated || first.NextOffset == nil || *first.NextOffset != 3 {
		t.Fatalf("unexpected first result window: %+v", first)
	}
	second := dockRunResultWindow("a界b界c", *first.NextOffset, 3)
	if second.Content != "界c" || second.Truncated || second.NextOffset != nil {
		t.Fatalf("unexpected second result window: %+v", second)
	}
}

func TestDockChildHandoffInstructionIsAppendedOnce(t *testing.T) {
	got := withDockChildHandoffInstruction("Investigate ClickHouse usage.")
	if !strings.Contains(got, "at most 2,500 characters") {
		t.Fatalf("handoff instruction missing: %q", got)
	}
	if twice := withDockChildHandoffInstruction(got); twice != got {
		t.Fatalf("handoff instruction duplicated: %q", twice)
	}
}

func TestSupportChildHandoffInstructionRequiresConfiguredKnowledge(t *testing.T) {
	got := withSupportChildHandoffInstruction("Check the current product pricing.")
	for _, want := range []string{"at most 2,500 characters", "search_knowledge", "third-party", "include_domains", "Stop when", "file paths and symbols", "unconfirmed availability"} {
		if !strings.Contains(got, want) {
			t.Fatalf("support handoff instruction missing %q: %q", want, got)
		}
	}
	if twice := withSupportChildHandoffInstruction(got); twice != got {
		t.Fatalf("support handoff instruction duplicated: %q", twice)
	}
}

func TestDockRunArtifactReferencesExcludeRuntimeInternals(t *testing.T) {
	artifacts := []model.AgentRunArtifact{
		{ID: "tool", ArtifactType: model.AgentRunArtifactTypeToolCall, Format: "json", StorageMode: "inline"},
		{ID: "review", ArtifactType: model.AgentRunArtifactTypeReviewFindings, Format: "json", StorageMode: "inline"},
	}
	got := dockRunArtifactReferences(artifacts)
	if len(got) != 1 || got[0].ArtifactID != "review" {
		t.Fatalf("artifact references = %+v, want only review artifact", got)
	}
}
