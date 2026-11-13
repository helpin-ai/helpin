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
		{"currency spacing", "Costs $ 49.", "Costs $49.", true},
		{"percent word", "Saves 20%.", "Saves 20 percent.", true},
		{"percent word reverse", "Saves 20 percent.", "Saves 20%.", true},
		{"percentage points differ", "Saves 20 percentage points.", "Saves 20 percent.", false},
		{"percent spacing", "Saves 20 %.", "Saves 20%.", true},
		{"decimal ambiguity", "Costs 1,5 euros.", "Costs 15 euros.", false},
		{"upper bound removed", "Includes up to 20 users.", "Includes 20 users.", false},
		{"upper bound preserved", "Includes up to 20 users.", "Includes up to 20 users.", true},
		{"plain occurrence alongside bound", "Supports up to 20 users. Includes 20 users.", "Includes 20 users.", true},
		{"plain occurrence before bound", "Includes 20 users. Supports up to 20 users.", "Includes 20 users.", true},
		{"matching bound among alternatives", "Supports up to 20 users. Requires at least 20 users.", "Supports up to 20 users.", true},
		{"different unit cannot excuse bound", "Supports up to 20 users. Includes 20 days.", "Includes 20 users.", false},
		{"same claim drops one qualifier", "Supports up to 20 users.", "Supports up to 20 users and includes 20 users.", false},
		{"plain currency alongside bound", "Costs up to $49. Setup costs $49.", "Setup costs $49.", true},
		{"currency qualifier does not leak to adjacent quantity", "Up to $49 20 users included.", "20 users included.", true},
		{"qualified currency billing aliases", "Costs up to USD 49 per month.", "Costs up to $49/month.", true},
		{"currency prefix bound removed", "Costs up to USD 49.", "Costs $49.", false},
		{"currency suffix bound removed", "Costs up to 49 USD.", "Costs $49.", false},
		{"qualified currency wrong period", "Costs up to $49/month.", "Costs up to $49/year.", false},
		{"plain price wrong period with bound", "Costs up to $49/month or $49/year.", "Costs $49/month.", false},
		{"plain percent alongside bound", "Save up to 20%. Annual billing saves 20%.", "Annual billing saves 20%.", true},
		{"upper bound added", "Includes 20 users.", "Includes up to 20 users.", false},
		{"lower bound reversed", "Includes at least 20 users.", "Includes up to 20 users.", false},
		{"decimal formatting", "Costs $49.00 per month.", "Costs $49 per month.", true},
		{"range formatting", "Includes 10-20 users.", "Includes 10 to 20 users.", true},
		{"brackets are not automatically citations", "Includes 10 users.", "Use [99] users.", false},
		{"currency aliases", "Costs 49 USD.", "Costs $49.", true},
		{"currency prefix aliases", "Costs USD 49.", "Costs $49.", true},
		{"currency prefix mismatch", "Costs EUR 49.", "Costs USD 49.", false},
		{"discount bound removed", "Save up to 20%.", "Save 20%.", false},
		{"written quantity", "Includes 20 users.", "Includes twenty users.", true},
		{"written compound quantity", "Includes 21 users.", "Includes twenty-one users.", true},
		{"wrong written quantity", "Includes 20 users.", "Includes thirty users.", false},
		{"ambiguous written arithmetic", "Includes 100 users.", "Includes one hundred users.", false},
		{"written literal support", "Includes one hundred users.", "Includes one hundred users.", true},
		{"written currency", "Costs $49.", "Costs forty-nine dollars.", true},
		{"generic version distinction", "Requires version 1.10.", "Requires version 1.1.", false},
		{"price", "Costs $49 per month.", "Costs $49 per month.", true},
		{"invented price", "Costs $49 per month.", "Costs $99 per month.", false},
		{"currency", "Costs $49.", "Costs €49.", false},
		{"seats versus days", "Includes 10 seats.", "Includes 10 days.", false},
		{"supported version", "Use SAML 2.0.", "Use SAML 2.0.", true},
		{"invented version", "Use SAML 2.0.", "Use SAML 3.0.", false},
		{"certificate spelling", "Use an X.509 certificate.", "Use an x509 certificate.", true},
		{"certificate reverse spelling", "Use an x509 certificate.", "Use an X.509 certificate.", true},
		{"unsupported certificate", "Use a certificate.", "Use an X.509 certificate.", false},
		{"punctuation", "Allows 20 users.", "Allows 20 users!", true},
		{"bold section numbering", "Use SAML 2.0.", "**1. In your identity provider**\nUse SAML 2.0.", true},
		{"bold ordinal", "Enable SSO.", "**1.** Enable SSO.", true},
		{"version dots matter", "Use TLS 1.3.", "Use TLS 13.", false},
		{"date order matters", "Expires 2026-11-10.", "Expires 2026-10-11.", false},
		{"range order matters", "Includes 10-20 users.", "Includes 20-10 users.", false},
		{"list numbering", "Enable SSO.", "1. Enable SSO.", true},
		{"percent versus count", "Includes 20 users.", "Saves 20%.", false},
		{"billing slash to words", "Costs $49/month.", "Costs $49 per month.", true},
		{"billing words to slash", "Costs $49 per month.", "Costs $49/month.", true},
		{"billing short aliases", "Costs $49/mo.", "Costs $49 each month.", true},
		{"billing slash spacing", "Costs $49 / month.", "Costs $49 per month.", true},
		{"billing seat period reversed", "Costs $49 per user per month.", "Costs $49 per user per year.", false},
		{"billing axes reordered", "Costs $49 per user per month.", "Costs $49/month/user.", true},
		{"monthly yearly", "Costs $49 per month.", "Costs $49 per year.", false},
		{"negative quantity", "Changes by -5 users.", "Changes by 5 users.", false},
		{"invented negative", "Changes by 5 users.", "Changes by -5 users.", false},
		{"unicode negative", "Changes by −5 users.", "Changes by -5 users.", true},
		{"scale removed", "Includes 5 million users.", "Includes 5 users.", false},
		{"scale invented", "Includes 5 users.", "Includes 5 million users.", false},
		{"scale preserved", "Includes 5 million users.", "Includes 5 million users.", true},
		{"negative currency", "Refund is -$49.", "Refund is $49.", false},
		{"negative currency alias", "Refund is -49 USD.", "Refund is -$49.", true},
		{"currency scale", "Costs $5 million.", "Costs $5.", false},
		{"currency scale invented", "Costs $5.", "Costs $5 million.", false},
		{"billing price swap", "Costs $49 per month or $499 per year.", "Costs $49 per year.", false},
		{"billing price correct", "Costs $49 per month or $499 per year.", "Costs $499 per year.", true},
		{"bound unrelated unit", "Includes up to 20 users and a 20 days trial.", "Includes a 20 days trial.", true},
		{"currency bound removed", "Costs up to $49.", "Costs $49.", false},
		{"currency bound preserved", "Costs up to $49.00.", "Costs up to $49.", true},
		{"range sign removed", "Changes by -5 to 5 users.", "Changes by 5 to 5 users.", false},
		{"signed range preserved", "Changes by -5 to 5 users.", "Changes by -5–5 users.", true},
		{"conjunction before unit", "Configure session settings.", "Configure authentication and session settings.", true},
		{"quantity after conjunction", "Includes 20 users.", "Includes access and thirty users.", false},
		{"thousands", "Allows 1,000 users.", "Allows 1000 users.", true},
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

func TestNumericCannotBorrowAnotherClaimCitation(t *testing.T) {
	response := &AIResponseContract{Content: "Pro includes 50 users. Enterprise includes 50 users.", Claims: []AIResponseClaim{
		{Text: "Pro includes 50 users.", EvidenceIDs: []string{"pro"}},
		{Text: "Enterprise includes 50 users.", EvidenceIDs: []string{"enterprise"}},
	}}
	got := validateSupportAnswer(SupportQueryPlanContract{}, supportEvidenceCoverage{}, []KnowledgeSearchResult{{ID: "pro", Content: "Pro includes 10 users."}, {ID: "enterprise", Content: "Enterprise includes 50 users."}}, response)
	if got.Outcome == supportValidationPass {
		t.Fatal("borrowed a number from another claim's evidence")
	}
}

func BenchmarkSupportNumericValidation(b *testing.B) {
	evidence := []KnowledgeSearchResult{{ID: "e", Content: "The Pro plan costs $49 per month and includes 1000 users. Use SAML 2.0 with an X.509 certificate."}}
	response := &AIResponseContract{Content: evidence[0].Content, Claims: []AIResponseClaim{{Text: evidence[0].Content, EvidenceIDs: []string{"e"}}}}
	b.ReportAllocs()
	for b.Loop() {
		validateSupportAnswer(SupportQueryPlanContract{}, supportEvidenceCoverage{}, evidence, response)
	}
}

func FuzzNormalizeNumericToken(f *testing.F) {
	for _, value := range []string{"$ 49.", "1,000.00 users", "1,000 users", "1,5", "20 %", "TLS 1.3", "\u00a020 users", "2026-11-10"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		once := normalizeNumericToken(value)
		if twice := normalizeNumericToken(once); once != twice {
			t.Fatalf("normalization is not stable: %q -> %q -> %q", value, once, twice)
		}
	})
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
