package service

import "testing"

func TestStripConversationPII_PreservesPublicSupportEmail(t *testing.T) {
	got := stripConversationPII(
		"Email support@company.com or reach us in live chat.",
		strPtr("customer@example.com"),
		nil,
	)
	want := "Email support@company.com or reach us in live chat."
	if got != want {
		t.Fatalf("stripConversationPII() = %q, want %q", got, want)
	}
}

func TestStripConversationPII_RedactsConversationCustomerContactInfo(t *testing.T) {
	got := stripConversationPII(
		"We emailed customer@example.com and called 415-555-0101. For general help use support@company.com.",
		strPtr("customer@example.com"),
		strPtr("415-555-0101"),
	)
	want := "We emailed [REDACTED] and called [REDACTED]. For general help use support@company.com."
	if got != want {
		t.Fatalf("stripConversationPII() = %q, want %q", got, want)
	}
}
