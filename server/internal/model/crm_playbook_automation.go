package model

import "time"

// CRMPlaybookSkillFile freezes a relative package file, including referenced resources.
type CRMPlaybookSkillFile struct {
	Path string `json:"path"`
	Data []byte `json:"data"`
}

// CRMPlaybookSkillSnapshot contains job guidance, not permission to execute tools.
type CRMPlaybookSkillSnapshot struct {
	Key     string                 `json:"key"`
	Title   string                 `json:"title"`
	Role    string                 `json:"role"`
	Version string                 `json:"version"`
	Files   []CRMPlaybookSkillFile `json:"files"`
}

// CRMPlaybookSpecialization freezes Beacon guidance for a supported journey.
// It is only the skill/prompt component of a future published execution connection;
// it does not freeze Flow settings, authorize actions, or activate an Agent.
type CRMPlaybookSpecialization struct {
	SchemaVersion int                        `json:"schema_version"`
	PresetKey     string                     `json:"preset_key"`
	PresetPrompt  string                     `json:"preset_prompt"`
	Journey       string                     `json:"journey"`
	Skills        []CRMPlaybookSkillSnapshot `json:"skills"`
	Version       string                     `json:"version"`
}

// CRMPlaybookSkillSummary exposes selected guidance without distributing package contents.
type CRMPlaybookSkillSummary struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Role    string `json:"role"`
	Version string `json:"version"`
}

// CRMPlaybookAutomationPreview describes proposed skill selection, never a runnable connection.
type CRMPlaybookAutomationPreview struct {
	PlaybookID            string                    `json:"playbook_id"`
	PlaybookRevision      int64                     `json:"playbook_revision"`
	PlaybookVersionID     *string                   `json:"playbook_version_id"`
	Scope                 string                    `json:"scope"`
	Journey               string                    `json:"journey"`
	PresetKey             string                    `json:"preset_key"`
	SpecializationVersion string                    `json:"specialization_version"`
	Skills                []CRMPlaybookSkillSummary `json:"skills"`
	Status                string                    `json:"status"`
	Explanation           string                    `json:"explanation"`
	ExecutionEnabled      bool                      `json:"execution_enabled"`
}

// CRMPlaybookContextRequest identifies the exact process and captured skill selection.
// This is an internal preparation contract, not a client-supplied launch request.
type CRMPlaybookContextRequest struct {
	WorkspaceID               string
	PlaybookID                string
	PlaybookVersionID         string
	SituationID               string
	ExpectedSituationRevision int64
	SpecializationVersion     string
	Target                    AgentRunTargetContext
}

// CRMPlaybookRunContext is permission-filtered CRM context for the shared Agent input.
// It is not an approval, execution permit, Flow binding, or separate progress store.
type CRMPlaybookRunContext struct {
	ConnectionID          string                `json:"connection_id,omitempty"`
	ConnectionVersion     int64                 `json:"connection_version,omitempty"`
	ConnectionFingerprint string                `json:"connection_fingerprint,omitempty"`
	SchemaVersion         int                   `json:"schema_version"`
	WorkspaceID           string                `json:"workspace_id"`
	PlaybookID            string                `json:"playbook_id"`
	PlaybookVersionID     string                `json:"playbook_version_id"`
	SituationID           string                `json:"situation_id"`
	SituationRevision     int64                 `json:"situation_revision"`
	SpecializationVersion string                `json:"specialization_version"`
	Journey               string                `json:"journey"`
	Target                AgentRunTargetContext `json:"target"`
	PlaybookObjective     string                `json:"playbook_objective"`
	CustomerObjective     string                `json:"customer_objective"`
	OwnerMemberID         string                `json:"owner_member_id"`
	NextActionOwnerID     string                `json:"next_action_owner_member_id,omitempty"`
	NextStep              string                `json:"next_step"`
	NextCheckpointAt      *time.Time            `json:"next_checkpoint_at"`
}
