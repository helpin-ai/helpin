package model

import "time"

// CRMSignalRuleCandidate is bounded aggregate evidence emitted by an evaluator.
// It intentionally excludes raw event and message histories.
type CRMSignalRuleCandidate struct {
	WorkspaceID            string
	ContactID              *string
	DealID                 *string
	CompanyID              *string
	RuleKey                string
	SignalType             string
	SignalDomain           string
	Polarity               string
	SourceType             string
	SourceID               *string
	Summary                string
	EvidenceExcerpt        string
	EvidenceFingerprint    string
	EvidenceIdentityMethod string
	EvidenceIdentityTrust  string
	ObservedAt             time.Time
	Metadata               JSONB

	// Behavioral identity fields are resolved against Postgres before storage.
	AnonymousID       string
	ExternalUserID    string
	CompanyExternalID string
}

// CRMSignalRuleSweepResult summarizes one evaluator cadence.
type CRMSignalRuleSweepResult struct {
	Cadence        string `json:"cadence"`
	RulesEvaluated int    `json:"rules_evaluated"`
	Candidates     int    `json:"candidates"`
	Inserted       int    `json:"inserted"`
	FailedRules    int    `json:"failed_rules"`
}
