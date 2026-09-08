package model

// PrepareCRMPlaybookSetupRequest creates reviewable defaults, never publication or activation.
type PrepareCRMPlaybookSetupRequest struct {
	ExpectedRevision  int64  `json:"expected_revision"`
	PlaybookVersionID string `json:"playbook_version_id"`
}

// CRMPlaybookSetup identifies existing editable Automation objects for guided review.
type CRMPlaybookSetup struct {
	FlowID  string `json:"flow_id"`
	AgentID string `json:"agent_id"`
}

// CRMPlaybookAutomationOverview is safe for CRM readers; private prompts and packages stay private.
type CRMPlaybookAutomationOverview struct {
	Settings         *CRMPlaybookAutomationSettings `json:"settings"`
	Connection       *CRMPlaybookConnection         `json:"connection"`
	FlowID           string                         `json:"flow_id,omitempty"`
	FlowName         string                         `json:"flow_name,omitempty"`
	AgentID          string                         `json:"agent_id,omitempty"`
	AgentName        string                         `json:"agent_name,omitempty"`
	RuntimeAvailable bool                           `json:"runtime_available"`
}
