package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestNumericGroundingMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, evidence, answer string
		pass                   bool
	}{
		// Correct pricing answers that the strict matcher blocked in production.
		{"per-seat plans", "Foundation $13 /user/mo. Pro $25 /user/mo. Business $42. Enterprise $59.", "Plans cost $13, $25, $42 and $59 per user per month.", true},
		{"percent free", "Pirate Ship is 100% free. No monthly fees.", "Pirate Ship is 100% free, with no monthly fees.", true},
		{"price split across table cells", "Starter\n$\n335\n/year", "Starter costs $335/year.", true},
		{"billing words differ", "$27/mo billed annually", "It costs $27 per month, billed annually.", true},
		{"period on its own line", "Inspector\n$109\nper month", "The Inspector plan is $109/month.", true},
		// Formatting that does not change the number.
		{"thousands separator", "Allows 1,000 users.", "Allows 1000 users.", true},
		{"trailing decimal zeros", "Costs $49.00.", "Costs $49.", true},
		{"list numbering", "Enable SSO.", "1. Enable SSO.", true},
		{"bold list numbering", "Use SAML 2.0.", "**1. In your identity provider**\nUse SAML 2.0.", true},
		{"version", "Use TLS 1.3.", "Use TLS 1.3.", true},
		// Numbers the evidence does not state.
		{"invented price", "Costs $49 per month.", "Costs $99 per month.", false},
		{"calculated total", "Costs $13 per user per month.", "For 10 users that is $130 per month.", false},
		{"annual equivalent", "Costs $49 per month.", "That is $588 a year.", false},
		{"decimal comma is not thousands", "Costs 1,5 euros.", "Costs 15 euros.", false},
		{"version dots matter", "Use TLS 1.3.", "Use TLS 13.", false},
		{"invented limit", "Includes 20 users.", "Includes 30 users.", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &AIResponseContract{Content: tc.answer, Claims: []AIResponseClaim{{Text: tc.answer, EvidenceIDs: []string{"e"}}}}
			got := validateSupportAnswer(SupportQueryPlanContract{}, supportEvidenceCoverage{}, []KnowledgeSearchResult{{ID: "e", Content: tc.evidence}}, response)
			if (got.Outcome == supportValidationPass) != tc.pass {
				t.Fatalf("outcome=%s reasons=%v want pass=%v", got.Outcome, got.Reasons, tc.pass)
			}
		})
	}
}

func TestNumericMayComeFromAnyRetrievedEvidence(t *testing.T) {
	// The claim cites the help article; the price is on the pricing page that
	// was retrieved alongside it.
	response := &AIResponseContract{Content: "Pro costs $25 per user and includes SSO.", Claims: []AIResponseClaim{
		{Text: "Pro costs $25 per user and includes SSO.", EvidenceIDs: []string{"article"}},
	}}
	results := []KnowledgeSearchResult{{ID: "article", Content: "SSO is on the Pro plan."}, {ID: "pricing", Content: "Pro $25 /user/mo"}}
	if got := validateSupportAnswer(SupportQueryPlanContract{}, supportEvidenceCoverage{}, results, response); got.Outcome != supportValidationPass {
		t.Fatalf("outcome=%s reasons=%v", got.Outcome, got.Reasons)
	}
}

func BenchmarkSupportNumericEvidenceSizes(b *testing.B) {
	for _, count := range []int{1, 20, 100} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			evidence := make([]KnowledgeSearchResult, count)
			for i := range evidence {
				evidence[i] = KnowledgeSearchResult{ID: fmt.Sprint(i), Content: strings.Repeat("The Pro plan costs $49 per month and includes 1000 users. ", 20)}
			}
			response := &AIResponseContract{Content: "The Pro plan costs $49 per month.", Claims: []AIResponseClaim{{Text: "The Pro plan costs $49 per month.", EvidenceIDs: []string{"0"}}}}
			b.ReportAllocs()
			for b.Loop() {
				validateSupportAnswer(SupportQueryPlanContract{}, supportEvidenceCoverage{}, evidence, response)
			}
		})
	}
}
