package model

import "time"

const (
	CRMSignalDismissIncorrectEvidence = "incorrect_evidence"
	CRMSignalDismissWrongEntity       = "wrong_entity"
	CRMSignalDismissDuplicate         = "duplicate"
	CRMSignalDismissIrrelevant        = "irrelevant"
	CRMSignalDismissHandled           = "handled"
	CRMSignalDismissBadTiming         = "bad_timing"

	CRMSignalFeedbackReviewed  = "reviewed"
	CRMSignalFeedbackDismissed = "dismissed"
	CRMSignalFeedbackActed     = "acted"

	CRMSignalDeliveryFeed         = "feed"
	CRMSignalDeliveryNotification = "notification"
	CRMSignalDeliveryDigest       = "digest"

	CRMSignalDeliveryPending = "pending"
	CRMSignalDeliverySending = "sending"
	CRMSignalDeliverySent    = "sent"
	CRMSignalDeliveryFailed  = "failed"
)

// CRMSignalFeedback is an immutable review or action event used for quality measurement.
type CRMSignalFeedback struct {
	ID                     string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SignalID               string    `json:"signal_id" gorm:"type:uuid;not null;index"`
	MemberID               string    `json:"member_id" gorm:"type:uuid;not null;index"`
	Action                 string    `json:"action" gorm:"not null;index"`
	DismissalReason        *string   `json:"dismissal_reason,omitempty" gorm:"index"`
	RuleKey                *string   `json:"rule_key,omitempty" gorm:"index"`
	RuleVersion            *int      `json:"rule_version,omitempty" gorm:"index"`
	SignalDomain           string    `json:"signal_domain" gorm:"not null;index"`
	IdentityMethod         string    `json:"identity_method" gorm:"not null;index"`
	DetectedAt             time.Time `json:"detected_at" gorm:"not null"`
	OccurredAt             time.Time `json:"occurred_at" gorm:"not null;index"`
	DetectionToEventMillis int64     `json:"detection_to_event_millis" gorm:"not null"`
	CreatedAt              time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalFeedback) TableName() string { return "crm_signal_feedback" }

// CRMSignalRoutingPolicy is immutable. A new version replaces prior routing behavior.
type CRMSignalRoutingPolicy struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Version           int       `json:"version" gorm:"not null"`
	Enabled           bool      `json:"enabled" gorm:"not null;default:true"`
	MinimumPriority   float64   `json:"minimum_priority" gorm:"not null;default:12"`
	RequiredTrust     string    `json:"required_trust" gorm:"not null;default:'verified'"`
	RouteToOwner      bool      `json:"route_to_owner" gorm:"not null;default:true"`
	DestinationTeamID *string   `json:"destination_team_id,omitempty" gorm:"type:uuid"`
	Channels          JSONBlob  `json:"channels" gorm:"type:jsonb;not null;default:'[\"feed\"]'"`
	CreatedByMemberID string    `json:"created_by_member_id" gorm:"type:uuid;not null"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalRoutingPolicy) TableName() string { return "crm_signal_routing_policies" }

type CreateCRMSignalRoutingPolicyRequest struct {
	MinimumPriority   float64  `json:"minimum_priority"`
	RequiredTrust     string   `json:"required_trust"`
	RouteToOwner      *bool    `json:"route_to_owner"`
	DestinationTeamID *string  `json:"destination_team_id"`
	Channels          []string `json:"channels"`
}

// CRMSignalDelivery provides channel-level idempotency and an audit trail.
type CRMSignalDelivery struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SignalID          string     `json:"signal_id" gorm:"type:uuid;not null;index"`
	PolicyID          string     `json:"policy_id" gorm:"type:uuid;not null;index"`
	PolicyVersion     int        `json:"policy_version" gorm:"not null"`
	Channel           string     `json:"channel" gorm:"not null"`
	RecipientMemberID *string    `json:"recipient_member_id,omitempty" gorm:"type:uuid"`
	DestinationTeamID *string    `json:"destination_team_id,omitempty" gorm:"type:uuid"`
	Status            string     `json:"status" gorm:"not null;default:'pending'"`
	DeliveredAt       *time.Time `json:"delivered_at,omitempty"`
	Attempts          int        `json:"attempts" gorm:"not null;default:0"`
	LastAttemptedAt   *time.Time `json:"last_attempted_at,omitempty"`
	LastError         *string    `json:"last_error,omitempty"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalDelivery) TableName() string { return "crm_signal_deliveries" }

type CRMSignalPrecisionRow struct {
	WorkspaceID         string  `json:"workspace_id"`
	RuleKey             string  `json:"rule_key"`
	RuleVersion         int     `json:"rule_version"`
	SignalDomain        string  `json:"signal_domain"`
	IdentityMethod      string  `json:"identity_method"`
	ReviewedCount       int64   `json:"reviewed_count"`
	ValidCount          int64   `json:"valid_count"`
	IncorrectCount      int64   `json:"incorrect_count"`
	ActedCount          int64   `json:"acted_count"`
	Precision           float64 `json:"precision"`
	AverageReviewMillis int64   `json:"average_review_millis"`
	AverageActionMillis int64   `json:"average_action_millis"`
}

// CRMSignalOutcomeCalibrationRow joins risk detections to later subscription outcomes.
type CRMSignalOutcomeCalibrationRow struct {
	WorkspaceID      string  `json:"workspace_id"`
	RuleKey          string  `json:"rule_key"`
	RuleVersion      int     `json:"rule_version"`
	CommercialMotion string  `json:"commercial_motion"`
	IdentityMethod   string  `json:"identity_method"`
	MaturedSignals   int64   `json:"matured_signals"`
	OutcomeMatched   int64   `json:"outcome_matched"`
	OutcomePrecision float64 `json:"outcome_precision"`
	HorizonDays      int     `json:"horizon_days"`
}

type CRMSignalBrief struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	WhatChanged     []string         `json:"what_changed"`
	Priority        float64          `json:"business_priority"`
	Severity        string           `json:"severity"`
	ScoreVersion    int              `json:"score_version"`
	Sources         []CRMBuyerSignal `json:"sources"`
	ActivationReady bool             `json:"activation_ready"`
}
