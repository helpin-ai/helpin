package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCoverageGapEmbeddingTextUsesStableSemanticFields(t *testing.T) {
	item := model.SupportCoverageGapListItem{
		SupportCoverageGap: model.SupportCoverageGap{
			Title:        "Password reset email never arrives",
			GapKind:      "content",
			GapCategory:  "knowledge",
			FailureMode:  "weak_retrieval",
			SourceSignal: "daily_conversation_analysis",
		},
		CanonicalTitle:   "Password reset email delivery failure",
		CustomerNeedText: "Customers need to recover access when reset emails do not arrive.",
		EvidenceText:     "thanks regards unrelated transcript boilerplate",
	}

	text := coverageGapEmbeddingText(item)

	if !strings.Contains(text, "Customers need to recover access") {
		t.Fatalf("embedding text omitted customer need: %q", text)
	}
	if !strings.Contains(text, "Password reset email delivery failure") {
		t.Fatalf("embedding text omitted canonical title: %q", text)
	}
	for _, noisy := range []string{"thanks regards", "weak_retrieval", "daily_conversation_analysis", "knowledge"} {
		if strings.Contains(text, noisy) {
			t.Fatalf("embedding text included noisy field %q: %q", noisy, text)
		}
	}
}

func TestCoverageEmbeddingTextHashNormalizesWhitespace(t *testing.T) {
	base := coverageEmbeddingTextHash("customers need refunds")
	if base == "" {
		t.Fatal("expected non-empty hash")
	}
	if got := coverageEmbeddingTextHash(" customers   need refunds "); got != base {
		t.Fatalf("hash did not normalize whitespace: got %q want %q", got, base)
	}
	if got := coverageEmbeddingTextHash("customers need invoice exports"); got == base {
		t.Fatalf("hash did not change for different semantic text: got %q", got)
	}
}
