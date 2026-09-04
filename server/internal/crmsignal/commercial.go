package crmsignal

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CommercialMeaning is the server-owned interpretation of a qualified commercial event.
type CommercialMeaning struct {
	Motion       string
	Polarity     string
	Relationship string
	NeedsContext bool
	ActionKey    string
	ActionLabel  string
}

// CustomerRelationship derives the relationship from CRM facts rather than model claims.
func CustomerRelationship(c model.SignalCommercialContext) string {
	switch c.SubscriptionStatus {
	case "active", "past_due", "cancel_scheduled", "canceled":
		return "customer"
	case "trialing":
		return "prospect"
	}
	if c.DealStageType == model.CRMStageTypeWon || c.DealMotion == model.CRMDealMotionExpansion || c.DealMotion == model.CRMDealMotionRenewal {
		return "customer"
	}
	switch c.LifecycleStage {
	case model.CRMLifecycleCustomer, model.CRMLifecycleEvangelist:
		return "customer"
	case model.CRMLifecycleLead, model.CRMLifecycleMarketingQualified, model.CRMLifecycleSalesQualified, model.CRMLifecycleOpportunity:
		return "prospect"
	}
	if c.DealStageType == model.CRMStageTypeOpen {
		return "prospect"
	}
	return "unknown"
}

// QualifyCommercialSignal requires an offering match and a specific commercial consequence.
// Routine support activity and ambiguous model classifications do not qualify.
func QualifyCommercialSignal(a model.SignalCommercialAssessment, c model.SignalCommercialContext) (CommercialMeaning, bool) {
	relationship := CustomerRelationship(c)
	meaning := CommercialMeaning{Relationship: relationship, NeedsContext: relationship == "unknown", Polarity: model.CRMSignalPolarityPositive}
	if a.Relevance != "relevant" || strings.TrimSpace(c.ProductContext) == "" || strings.TrimSpace(a.OfferingMatch) == "" || strings.TrimSpace(a.Consequence) == "" {
		return meaning, false
	}
	switch a.Event {
	case "purchase", "purchase_deadline", "purchase_blocker":
		switch {
		case relationship == "customer":
			meaning.Motion = model.CRMCommercialMotionExpansion
		case c.DealStageType == model.CRMStageTypeOpen || c.LifecycleStage == model.CRMLifecycleOpportunity:
			meaning.Motion = model.CRMCommercialMotionConversion
		case relationship == "prospect":
			meaning.Motion = model.CRMCommercialMotionProspecting
		default:
			meaning.Motion = model.CRMCommercialMotionNeedsContext
		}
		meaning.ActionKey, meaning.ActionLabel = "follow_up_on_purchase", "Follow up on the purchase request"
		if a.Event == "purchase_blocker" {
			meaning.Polarity = model.CRMSignalPolarityNegative
			meaning.ActionKey, meaning.ActionLabel = "resolve_purchase_blocker", "Review the blocker to this purchase"
		}
	case "expansion":
		meaning.Motion = model.CRMCommercialMotionExpansion
		meaning.NeedsContext = relationship != "customer"
		meaning.ActionKey, meaning.ActionLabel = "review_expansion", "Review the upgrade or add-on request"
	case "renewal", "renewal_deadline":
		meaning.Motion = model.CRMCommercialMotionRenewal
		meaning.NeedsContext = relationship != "customer"
		meaning.ActionKey, meaning.ActionLabel = "prepare_renewal", "Review renewal terms and timing"
	case "cancellation", "cancellation_deadline", "retention_risk":
		meaning.Motion, meaning.Polarity = model.CRMCommercialMotionRetention, model.CRMSignalPolarityNegative
		meaning.NeedsContext = relationship != "customer"
		meaning.ActionKey, meaning.ActionLabel = "review_retention_risk", "Review the cancellation or revenue risk"
	case "payment_recovery":
		meaning.Motion, meaning.Polarity = model.CRMCommercialMotionRetention, model.CRMSignalPolarityNegative
		meaning.NeedsContext = relationship != "customer"
		meaning.ActionKey, meaning.ActionLabel = "resolve_payment_blocker", "Help the customer resolve the payment blocker"
	default:
		return meaning, false
	}
	if meaning.Motion == model.CRMCommercialMotionNeedsContext {
		meaning.ActionKey, meaning.ActionLabel = "resolve_customer_context", "Confirm the account and customer relationship"
	}
	return meaning, true
}

// ValidCommercialEvent keeps invalid model output from reconciling away old evidence.
func ValidCommercialEvent(event string) bool {
	switch event {
	case "purchase", "purchase_deadline", "purchase_blocker", "expansion", "renewal", "renewal_deadline", "cancellation", "cancellation_deadline", "retention_risk", "payment_recovery":
		return true
	default:
		return false
	}
}
