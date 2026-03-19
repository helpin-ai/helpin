package model

import (
	"encoding/json"
	"time"
)

// FlowTemplate status constants.
const (
	FlowTemplateStatusActive   = "active"
	FlowTemplateStatusArchived = "archived"
)

// FlowTemplate is a DB-backed flow template definition.
// It defines a reusable node graph that can be instantiated as a FlowRun.
type FlowTemplate struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     *string   `json:"workspace_id,omitempty" gorm:"type:uuid;index"`
	Name            string    `json:"name" gorm:"not null"`
	Description     *string   `json:"description"`
	TemplateSlug    string    `json:"template_slug" gorm:"not null"`
	Version         int       `json:"version" gorm:"not null;default:1"`
	TargetType      string    `json:"target_type" gorm:"not null"`
	InitialNodeSlug string    `json:"initial_node_slug" gorm:"not null"`
	IsBuiltin       bool      `json:"is_builtin" gorm:"not null;default:false"`
	Status          string    `json:"status" gorm:"not null;default:'active'"`
	CreatedBy       *string   `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	Nodes []FlowTemplateNode `json:"nodes,omitempty" gorm:"foreignKey:TemplateID;references:ID"`
}

func (FlowTemplate) TableName() string { return "flow_templates" }

// FlowTemplateNode is a single node within a flow template.
type FlowTemplateNode struct {
	ID                   string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TemplateID           string          `json:"template_id" gorm:"type:uuid;not null;index"`
	NodeSlug             string          `json:"node_slug" gorm:"not null"`
	Label                string          `json:"label" gorm:"not null"`
	NodeType             string          `json:"node_type" gorm:"not null"`
	Position             int             `json:"position" gorm:"not null;default:0"`
	NextNodeSlug         *string         `json:"next_node_slug,omitempty"`
	LoopbackNodeSlug     *string         `json:"loopback_node_slug,omitempty"`
	AgentInputKey        *string         `json:"agent_input_key,omitempty"`
	DefaultAgentID       *string         `json:"default_agent_id,omitempty" gorm:"type:uuid"`
	SystemPrompt         *string         `json:"system_prompt,omitempty"`
	AllowedTools         json.RawMessage `json:"allowed_tools" gorm:"type:jsonb;not null;default:'[]'"`
	OutputTag            *string         `json:"output_tag,omitempty"`
	Actions              json.RawMessage `json:"actions" gorm:"type:jsonb;not null;default:'[]'"`
	Retryable            bool            `json:"retryable" gorm:"not null;default:false"`
	CommandName          *string         `json:"command_name,omitempty"`
	ApproveCommandName   *string         `json:"approve_command_name,omitempty"`
	FeedbackFromNode     *string         `json:"feedback_from_node,omitempty"`
	AdditionalContextKey *string         `json:"additional_context_key,omitempty"`
	FallbackAgentKey     *string         `json:"fallback_agent_key,omitempty"`
	CreatedAt            time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (FlowTemplateNode) TableName() string { return "flow_template_nodes" }

// AllowedToolsList parses the allowed_tools JSONB into a string slice.
func (n *FlowTemplateNode) AllowedToolsList() []string {
	if len(n.AllowedTools) == 0 || string(n.AllowedTools) == "null" {
		return nil
	}
	var tools []string
	if err := json.Unmarshal(n.AllowedTools, &tools); err != nil {
		return nil
	}
	return tools
}

// ActionsList parses the actions JSONB into a string slice.
func (n *FlowTemplateNode) ActionsList() []string {
	if len(n.Actions) == 0 || string(n.Actions) == "null" {
		return nil
	}
	var actions []string
	if err := json.Unmarshal(n.Actions, &actions); err != nil {
		return nil
	}
	return actions
}

// RequiredMode returns the invocation mode required for this node type.
func (n *FlowTemplateNode) RequiredMode() string {
	switch n.NodeType {
	case FlowNodeTypeInteractiveAgent:
		return InvocationModeInteractive
	case FlowNodeTypeAgentTask:
		return InvocationModeAutonomous
	default:
		return ""
	}
}

// --- Request / Response DTOs ---

// CreateFlowTemplateRequest is the payload for creating a flow template.
type CreateFlowTemplateRequest struct {
	Name            string                         `json:"name"`
	Description     *string                        `json:"description"`
	TemplateSlug    string                         `json:"template_slug"`
	TargetType      string                         `json:"target_type"`
	InitialNodeSlug string                         `json:"initial_node_slug"`
	Nodes           []CreateFlowTemplateNodeRequest `json:"nodes"`
}

// CreateFlowTemplateNodeRequest is the payload for creating a node within a template.
type CreateFlowTemplateNodeRequest struct {
	NodeSlug             string          `json:"node_slug"`
	Label                string          `json:"label"`
	NodeType             string          `json:"node_type"`
	Position             int             `json:"position"`
	NextNodeSlug         *string         `json:"next_node_slug,omitempty"`
	LoopbackNodeSlug     *string         `json:"loopback_node_slug,omitempty"`
	AgentInputKey        *string         `json:"agent_input_key,omitempty"`
	DefaultAgentID       *string         `json:"default_agent_id,omitempty"`
	SystemPrompt         *string         `json:"system_prompt,omitempty"`
	AllowedTools         json.RawMessage `json:"allowed_tools,omitempty"`
	OutputTag            *string         `json:"output_tag,omitempty"`
	Actions              json.RawMessage `json:"actions,omitempty"`
	Retryable            bool            `json:"retryable"`
	CommandName          *string         `json:"command_name,omitempty"`
	ApproveCommandName   *string         `json:"approve_command_name,omitempty"`
	FeedbackFromNode     *string         `json:"feedback_from_node,omitempty"`
	AdditionalContextKey *string         `json:"additional_context_key,omitempty"`
	FallbackAgentKey     *string         `json:"fallback_agent_key,omitempty"`
}

// UpdateFlowTemplateRequest is the payload for updating a flow template.
type UpdateFlowTemplateRequest struct {
	Name            *string `json:"name,omitempty"`
	Description     *string `json:"description,omitempty"`
	InitialNodeSlug *string `json:"initial_node_slug,omitempty"`
	Status          *string `json:"status,omitempty"`
}

// UpdateFlowTemplateNodeRequest is the payload for updating a node.
type UpdateFlowTemplateNodeRequest struct {
	Label                *string          `json:"label,omitempty"`
	NodeType             *string          `json:"node_type,omitempty"`
	Position             *int             `json:"position,omitempty"`
	NextNodeSlug         *string          `json:"next_node_slug,omitempty"`
	LoopbackNodeSlug     *string          `json:"loopback_node_slug,omitempty"`
	AgentInputKey        *string          `json:"agent_input_key,omitempty"`
	DefaultAgentID       *string          `json:"default_agent_id,omitempty"`
	SystemPrompt         *string          `json:"system_prompt,omitempty"`
	AllowedTools         *json.RawMessage `json:"allowed_tools,omitempty"`
	OutputTag            *string          `json:"output_tag,omitempty"`
	Actions              *json.RawMessage `json:"actions,omitempty"`
	Retryable            *bool            `json:"retryable,omitempty"`
	CommandName          *string          `json:"command_name,omitempty"`
	ApproveCommandName   *string          `json:"approve_command_name,omitempty"`
	FeedbackFromNode     *string          `json:"feedback_from_node,omitempty"`
	AdditionalContextKey *string          `json:"additional_context_key,omitempty"`
	FallbackAgentKey     *string          `json:"fallback_agent_key,omitempty"`
}

// FlowTemplateView is the API response for a flow template with its nodes.
type FlowTemplateView struct {
	Template FlowTemplate       `json:"template"`
	Nodes    []FlowTemplateNode `json:"nodes"`
}

// FlowTemplateListResponse is a paginated list of templates.
type FlowTemplateListResponse struct {
	Data  []FlowTemplate `json:"data"`
	Total int64          `json:"total"`
}
