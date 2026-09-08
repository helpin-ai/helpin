package model

import (
	"encoding/json"
	"time"
)

// CRMPlaybookConnectionSelection identifies server-owned settings to review.
type CRMPlaybookConnectionSelection struct {
	PlaybookVersionID string `json:"playbook_version_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	FlowID            string `json:"flow_id"`
	AgentID           string `json:"agent_id"`
}

// PublishCRMPlaybookConnectionRequest approves an exact preview, not client-supplied instructions.
type PublishCRMPlaybookConnectionRequest struct {
	PlaybookVersionID         string `json:"playbook_version_id"`
	ExpectedRevision          int64  `json:"expected_revision"`
	FlowID                    string `json:"flow_id"`
	AgentID                   string `json:"agent_id"`
	CommandKey                string `json:"command_key"`
	ExpectedConnectionVersion int64  `json:"expected_connection_version"`
	ReviewFingerprint         string `json:"review_fingerprint"`
}

// Selection returns the settings identities without publication or approval fields.
func (r PublishCRMPlaybookConnectionRequest) Selection() CRMPlaybookConnectionSelection {
	return CRMPlaybookConnectionSelection{PlaybookVersionID: r.PlaybookVersionID, ExpectedRevision: r.ExpectedRevision, FlowID: r.FlowID, AgentID: r.AgentID}
}

// CRMPlaybookConnectionSource is loaded atomically from trusted workspace records.
type CRMPlaybookConnectionSource struct {
	Playbook          CRMPlaybook
	Policy            CRMPlaybookVersion
	Flow              AutomationRule
	Agent             Agent
	ConnectionVersion int64
}

// CRMPlaybookConnectionSnapshot freezes configuration, not live execution authority.
// Agent retains host-owned limits/access selectors; RuntimeAgent is the existing
// runtime projection. These private settings exclude counters and active run state.
type CRMPlaybookConnectionSnapshot struct {
	SchemaVersion     int                       `json:"schema_version"`
	WorkspaceID       string                    `json:"workspace_id"`
	PlaybookID        string                    `json:"playbook_id"`
	PlaybookRevision  int64                     `json:"playbook_revision"`
	PolicyVersionID   string                    `json:"policy_version_id"`
	PolicyFingerprint string                    `json:"policy_fingerprint"`
	Flow              AutomationRule            `json:"flow"`
	Agent             Agent                     `json:"agent"`
	RuntimeAgent      json.RawMessage           `json:"runtime_agent"`
	Specialization    CRMPlaybookSpecialization `json:"specialization"`
}

// CRMPlaybookConnection is an append-only approved setup and its idempotent receipt.
// No publication can enable execution. Future activation needs a separate guarded gate.
type CRMPlaybookConnection struct {
	ID                  string                        `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID         string                        `json:"-" gorm:"type:uuid;not null"`
	PlaybookID          string                        `json:"playbook_id" gorm:"type:uuid;not null"`
	PlaybookVersionID   string                        `json:"playbook_version_id" gorm:"type:uuid;not null"`
	Version             int64                         `json:"version"`
	CommandKey          string                        `json:"-"`
	CommandFingerprint  string                        `json:"-"`
	Fingerprint         string                        `json:"fingerprint"`
	Snapshot            CRMPlaybookConnectionSnapshot `json:"-" gorm:"serializer:json;type:jsonb"`
	ExecutionEnabled    bool                          `json:"execution_enabled"`
	PublishedByMemberID string                        `json:"published_by_member_id" gorm:"type:uuid"`
	PublishedAt         time.Time                     `json:"published_at"`
}

// TableName keeps immutable connection publication separate from editable Flows.
func (CRMPlaybookConnection) TableName() string { return "crm_playbook_connections" }

// CRMPlaybookConnectionReview exposes useful settings without raw instructions or packages.
type CRMPlaybookConnectionReview struct {
	PlaybookVersionID string                    `json:"playbook_version_id"`
	ExpectedRevision  int64                     `json:"expected_revision"`
	FlowID            string                    `json:"flow_id"`
	AgentID           string                    `json:"agent_id"`
	ConnectionVersion int64                     `json:"connection_version"`
	ReviewFingerprint string                    `json:"review_fingerprint"`
	FlowName          string                    `json:"flow_name"`
	AgentName         string                    `json:"agent_name"`
	RuntimeKind       string                    `json:"runtime_kind"`
	Skills            []CRMPlaybookSkillSummary `json:"skills"`
	ExecutionEnabled  bool                      `json:"execution_enabled"`
	Explanation       string                    `json:"explanation"`
}

// CRMPlaybookConnectionResult returns the original receipt on an identical retry.
type CRMPlaybookConnectionResult struct {
	Connection CRMPlaybookConnection `json:"connection"`
	Replayed   bool                  `json:"replayed"`
}

// CRMPlaybookConnections lists publication receipts, newest first, without private snapshots.
type CRMPlaybookConnections struct {
	Data              []CRMPlaybookConnection `json:"data"`
	NextBeforeVersion *int64                  `json:"next_before_version"`
}
