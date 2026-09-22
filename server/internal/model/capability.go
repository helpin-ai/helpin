package model

import "time"

// Capability status values. A status is only "ready" when the server has
// evidence that the capability works; configuration alone is never enough.
const (
	// CapabilityReady means the capability is configured and has been verified.
	CapabilityReady = "ready"
	// CapabilityNeedsSetup means a configuration step is missing or the last check failed.
	CapabilityNeedsSetup = "needs_setup"
	// CapabilityUnableToVerify means the capability is configured but has not been
	// verified, or verification needs a check the server did not perform.
	CapabilityUnableToVerify = "unable_to_verify"
	// CapabilityUnavailable means the edition, deployment, or module selection
	// does not offer the capability.
	CapabilityUnavailable = "unavailable"
)

// Capability action kinds tell clients which control resolves a status.
const (
	// CapabilityActionOpenSettings opens the workspace settings page in Path.
	CapabilityActionOpenSettings = "open_settings"
	// CapabilityActionServerConfig requires an operator to change server configuration.
	CapabilityActionServerConfig = "server_config"
	// CapabilityActionSendTestEmail sends a test email (POST /workspaces/{id}/email/test).
	CapabilityActionSendTestEmail = "send_test_email"
	// CapabilityActionTestAIConnection tests the connection in ConnectionID.
	CapabilityActionTestAIConnection = "test_ai_connection"
)

// Capability keys reported by the capability endpoints.
const (
	CapabilityKeyAIChat              = "ai_chat"
	CapabilityKeyAIEmbeddings        = "ai_embeddings"
	CapabilityKeyEmailOutbound       = "email_outbound"
	CapabilityKeySupportWidget       = "support_widget"
	CapabilityKeySupportEmailInbound = "support_email_inbound"
	CapabilityKeyGitHub              = "github"
	CapabilityKeyObjectStorage       = "object_storage"
	CapabilityKeyWorkers             = "workers"
)

// CapabilityAction describes the next step that resolves a capability status.
// Path is relative to the workspace route (for example "settings/ai-connections").
type CapabilityAction struct {
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	Path         string `json:"path,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
}

// Capability is one reported capability. Detail is user-facing text that never
// contains secrets, hostnames, or raw provider errors.
type Capability struct {
	Key       string            `json:"key"`
	Status    string            `json:"status"`
	Detail    string            `json:"detail"`
	Required  bool              `json:"required"`
	CheckedAt *time.Time        `json:"checked_at,omitempty"`
	Action    *CapabilityAction `json:"action,omitempty"`
}

// CapabilitiesResponse lists capabilities for a workspace or the instance.
type CapabilitiesResponse struct {
	Edition      string       `json:"edition"`
	Capabilities []Capability `json:"capabilities"`
}

// InstanceCapabilityCheck is the last explicit check of an instance capability.
// ConfigFingerprint identifies the tested configuration without storing secrets.
type InstanceCapabilityCheck struct {
	Key               string    `gorm:"primaryKey"`
	OK                bool      `gorm:"column:ok;not null"`
	Error             *string   `gorm:"column:error"`
	ConfigFingerprint string    `gorm:"not null"`
	CheckedBy         *string   `gorm:"type:uuid"`
	CheckedAt         time.Time `gorm:"not null"`
}

// TableName returns the table for instance capability checks.
func (InstanceCapabilityCheck) TableName() string { return "instance_capability_checks" }

// TestEmailResult reports a test email attempt. Error is a fixed, redacted message.
type TestEmailResult struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Recipient string `json:"recipient,omitempty"`
}
