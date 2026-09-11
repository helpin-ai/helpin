package crmsignal

import (
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

func TestQualifyCommercialSignal(t *testing.T) {
	tests := []struct {
		name, event, lifecycle, subscription, dealStage, motion, polarity string
		needsContext                                                      bool
	}{
		{name: "opportunity without attached deal", event: "purchase", lifecycle: "opportunity", motion: "conversion", polarity: "positive"},
		{name: "lead pricing", event: "purchase", lifecycle: "lead", motion: "prospecting", polarity: "positive"},
		{name: "open purchase", event: "purchase_deadline", dealStage: "open", motion: "conversion", polarity: "positive"},
		{name: "customer pricing is expansion", event: "purchase", lifecycle: "customer", motion: "expansion", polarity: "positive"},
		{name: "customer upgrade", event: "expansion", subscription: "active", motion: "expansion", polarity: "positive"},
		{name: "cancel deadline is negative", event: "cancellation_deadline", lifecycle: "customer", motion: "retention", polarity: "negative"},
		{name: "payment recovery", event: "payment_recovery", subscription: "past_due", motion: "retention", polarity: "negative"},
		{name: "buying blocker", event: "purchase_blocker", lifecycle: "lead", motion: "prospecting", polarity: "negative"},
		{name: "renewal deadline", event: "renewal_deadline", subscription: "active", motion: "renewal", polarity: "positive"},
		{name: "unknown buyer", event: "purchase", motion: "needs_context", polarity: "positive", needsContext: true},
		{name: "support subscriber is not a qualified prospect", event: "purchase", lifecycle: "subscriber", motion: "needs_context", polarity: "positive", needsContext: true},
		{name: "cancellation despite subscriber lifecycle", event: "cancellation", lifecycle: "subscriber", motion: "retention", polarity: "negative", needsContext: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.SignalCommercialContext{ProductContext: "We sell team software subscriptions.", LifecycleStage: tt.lifecycle, SubscriptionStatus: tt.subscription, DealStageType: tt.dealStage}
			a := model.SignalCommercialAssessment{Relevance: "relevant", Event: tt.event, OfferingMatch: "Team subscription", Consequence: "Explicit paid subscription decision"}
			got, ok := QualifyCommercialSignal(a, c)
			if !ok || got.Motion != tt.motion || got.Polarity != tt.polarity || got.NeedsContext != tt.needsContext || got.ActionKey == "" || got.ActionLabel == "" {
				t.Fatalf("qualification = %+v, %v", got, ok)
			}
		})
	}
}

func TestCommercialGateRejectsUnqualifiedEvidence(t *testing.T) {
	for _, name := range []string{"irrelevant", "uncertain", "missing offering", "missing consequence", "missing business context", "unsupported event"} {
		t.Run(name, func(t *testing.T) {
			c := model.SignalCommercialContext{ProductContext: "Software subscriptions"}
			a := model.SignalCommercialAssessment{Relevance: "relevant", Event: "purchase", OfferingMatch: "Software", Consequence: "Buying the product"}
			switch name {
			case "irrelevant", "uncertain":
				a.Relevance = name
			case "missing offering":
				a.OfferingMatch = " "
			case "missing consequence":
				a.Consequence = " "
			case "missing business context":
				c.ProductContext = " "
			case "unsupported event":
				a.Event = "support_escalation"
			}
			if got, ok := QualifyCommercialSignal(a, c); ok {
				t.Fatalf("unqualified evidence passed: %+v", got)
			}
		})
	}
}

func TestExistingBusinessRequiresEvidenceForSpecificMotion(t *testing.T) {
	c := model.SignalCommercialContext{ProductContext: "Software subscriptions", DealMotion: "existing_business", DealStageType: "open"}
	if got := CustomerRelationship(c); got != "customer" {
		t.Fatalf("relationship=%s", got)
	}
	for _, tc := range []struct{ event, motion string }{{"expansion", "expansion"}, {"renewal", "renewal"}, {"cancellation", "retention"}, {"purchase", "expansion"}} {
		t.Run(tc.event, func(t *testing.T) {
			got, ok := QualifyCommercialSignal(model.SignalCommercialAssessment{Relevance: "relevant", Event: tc.event, OfferingMatch: "Software", Consequence: "Explicit paid account decision"}, c)
			if !ok || got.Motion != tc.motion || got.NeedsContext {
				t.Errorf("meaning=%+v qualified=%v", got, ok)
			}
		})
	}
	if _, ok := QualifyCommercialSignal(model.SignalCommercialAssessment{Relevance: "relevant", Event: "none"}, c); ok {
		t.Fatal("deal type fabricated commercial intent")
	}
}

func TestExistingBusinessRelationshipSurvivesProductTrial(t *testing.T) {
	if got := CustomerRelationship(model.SignalCommercialContext{DealMotion: "existing_business", SubscriptionStatus: "trialing"}); got != "customer" {
		t.Fatalf("relationship=%s", got)
	}
}
