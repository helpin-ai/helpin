package service

import "testing"

func TestDetectForwardedEmailAttributionGmailBlock(t *testing.T) {
	result := detectForwardedEmailAttribution(forwardedEmailDetectionInput{
		ForwarderEmail: "founder@company.com",
		ForwarderName:  "Founder",
		RecipientEmails: []string{
			"support@acme.on.helpin.email",
		},
		ReplyDomain: "replies.helpin.ai",
		RouteDomain: "on.helpin.email",
		Text: `Can someone handle this?

---------- Forwarded message ---------
From: Jane Customer <jane@customer.example>
Date: Tue, Jun 2, 2026 at 10:14 AM
Subject: Billing question
To: Founder <founder@company.com>

I need help with my invoice.`,
	})

	if !result.Applied {
		t.Fatalf("expected forwarded attribution to apply, got %#v", result)
	}
	if result.OriginalEmail != "jane@customer.example" {
		t.Fatalf("original email = %q, want jane@customer.example", result.OriginalEmail)
	}
	if result.OriginalName != "Jane Customer" {
		t.Fatalf("original name = %q, want Jane Customer", result.OriginalName)
	}
	if result.ForwardedByEmail != "founder@company.com" || result.ForwardedByName != "Founder" {
		t.Fatalf("unexpected forwarder: %#v", result)
	}
	if result.Confidence < forwardedEmailDefaultMinConfidence || result.ConfidenceLevel != "high" {
		t.Fatalf("expected high confidence, got %#v", result)
	}
}

func TestDetectForwardedEmailAttributionRejectsSupportDomainCandidate(t *testing.T) {
	result := detectForwardedEmailAttribution(forwardedEmailDetectionInput{
		ForwarderEmail: "founder@company.com",
		RecipientEmails: []string{
			"support@acme.on.helpin.email",
		},
		ReplyDomain: "replies.helpin.ai",
		RouteDomain: "on.helpin.email",
		Text: `---------- Forwarded message ---------
From: Support <support@acme.on.helpin.email>
Date: Tue, Jun 2, 2026 at 10:14 AM
Subject: Billing question`,
	})

	if result.Applied {
		t.Fatalf("expected support-domain candidate to be rejected, got %#v", result)
	}
	if result.Reason == "" {
		t.Fatalf("expected rejection reason, got %#v", result)
	}
}

func TestDetectForwardedEmailAttributionRejectsConflictingCandidates(t *testing.T) {
	result := detectForwardedEmailAttribution(forwardedEmailDetectionInput{
		ForwarderEmail: "founder@company.com",
		ReplyDomain:    "replies.helpin.ai",
		RouteDomain:    "on.helpin.email",
		Text: `---------- Forwarded message ---------
From: Jane Customer <jane@customer.example>
From: Other Customer <other@customer.example>
Date: Tue, Jun 2, 2026 at 10:14 AM
Subject: Billing question`,
	})

	if result.Applied {
		t.Fatalf("expected conflicting candidates to be rejected, got %#v", result)
	}
}
