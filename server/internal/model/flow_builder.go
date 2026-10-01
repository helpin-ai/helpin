package model

import "encoding/json"

// FlowBuilderState belongs to a private Ask Agent conversation. Drafts have no
// execution side effects; only the approval-gated save tool creates or updates a flow.
type FlowBuilderState struct {
	Timezone        string            `json:"timezone,omitempty"`
	NextRuns        []string          `json:"next_runs,omitempty"`
	CreatesAgent    bool              `json:"creates_agent,omitempty"`
	SourceRule      *AutomationRule   `json:"source_rule,omitempty"`
	TemplateVersion int               `json:"template_version,omitempty"`
	TemplateKey     string            `json:"template_key,omitempty"`
	Draft           *FlowBuilderDraft `json:"draft,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Revision        string            `json:"revision,omitempty"`
	UserMessageID   string            `json:"user_message_id,omitempty"`
	CreatedFlowID   string            `json:"created_flow_id,omitempty"`
	Agent           *Agent            `json:"agent,omitempty"`
}

type FlowBuilderDraft struct {
	Summary           string                            `json:"summary,omitempty"`
	ScheduleTimezone  string                            `json:"schedule_timezone,omitempty"`
	Paused            bool                              `json:"paused,omitempty"`
	SemanticCondition string                            `json:"semantic_condition,omitempty"`
	Name              string                            `json:"name"`
	Description       *string                           `json:"description,omitempty"`
	TeamID            *string                           `json:"team_id,omitempty"`
	WorkflowID        *string                           `json:"workflow_id,omitempty"`
	TriggerType       string                            `json:"trigger_type"`
	TriggerConfig     json.RawMessage                   `json:"trigger_config"`
	ActionType        string                            `json:"action_type"`
	ActionConfig      json.RawMessage                   `json:"action_config"`
	TemplateInputs    map[string]any                    `json:"template_inputs,omitempty"`
	AgentName         string                            `json:"agent_name,omitempty"`
	AgentOverrides    *CreateAgentFromTemplateOverrides `json:"agent_overrides,omitempty"`
}
