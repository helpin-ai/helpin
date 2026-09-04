package model

// SignalCommercialContext supplies seller context and authoritative CRM facts to detection.
// It contains no credentials and is scoped to the source workspace.
type SignalCommercialContext struct {
	WorkspaceName      string  `json:"workspace_name"`
	ProductContext     string  `json:"company_product_context"`
	WebsiteURL         string  `json:"website_url,omitempty"`
	LifecycleStage     string  `json:"lifecycle_stage,omitempty"`
	SubscriptionStatus string  `json:"subscription_status,omitempty"`
	DealMotion         string  `json:"deal_motion,omitempty"`
	DealStageType      string  `json:"deal_stage_type,omitempty"`
	CompanyID          *string `json:"company_id,omitempty"`
}

// SignalCommercialAssessment is the model's evidence-backed commercial classification.
// The server determines motion and polarity from the event and CRM relationship.
type SignalCommercialAssessment struct {
	Relevance     string `json:"relevance"`
	Event         string `json:"event"`
	OfferingMatch string `json:"offering_match"`
	Consequence   string `json:"consequence"`
}

// CRMCommercialMotionNeedsContext marks relevant evidence with an unknown relationship.
const CRMCommercialMotionNeedsContext = "needs_context"

// CRMSignalCommercialDetectorVersion identifies commercially qualified conversation extraction.
const CRMSignalCommercialDetectorVersion = 4
