package model

import "time"

// CRMPlaybookAgentUsage is a CRM-readable dependency, without saved Agent prompts.
type CRMPlaybookAgentUsage struct {
	PlaybookID string `json:"playbook_id"`
	Name       string `json:"name"`
}

// CRMPlaybookAutomationActivity projects existing immutable receipts/normal runs.
type CRMPlaybookAutomationActivity struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	SituationID    *string   `json:"situation_id,omitempty"`
	SituationTitle string    `json:"situation_title,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type CRMPlaybookAutomationActivityPage struct {
	Data  []CRMPlaybookAutomationActivity `json:"data"`
	Total int64                           `json:"total"`
	Page  int                             `json:"page"`
}
