package worker

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ExecutionContext holds all state for a single agent run execution.
type ExecutionContext struct {
	Context             context.Context
	WorkDir             string // path to cloned repo on disk
	WorkspaceID         string
	AgentID             string
	RunID               string
	TargetType          string
	TargetID            string
	StoryID             string
	TicketID            string
	Agent               *model.Agent
	Story               *model.PMStory
	Ticket              *model.SupportTicket
	GitIntegration      *model.GitIntegration
	Repo                string // e.g. "owner/repo"
	Config              *WorkflowConfig
	RuntimeProfile      model.RuntimeProfile
	AllowedTools        map[string]bool
	Services            *ServiceBridge
	PendingSupportDraft *SupportDraftReply
	LatestPRMetadata    *PRMetadata
}

// WorkflowConfig holds settings from WORKFLOW.md or defaults.
type WorkflowConfig struct {
	MaxIterations   int
	TimeoutMinutes  int
	AllowedCommands []string
	HandoffState    string
	CommandTimeout  time.Duration
	ExtraPrompt     string // the markdown body of WORKFLOW.md
}

// DefaultWorkflowConfig returns sensible defaults.
func DefaultWorkflowConfig() *WorkflowConfig {
	return &WorkflowConfig{
		MaxIterations:  50,
		TimeoutMinutes: 30,
		CommandTimeout: 2 * time.Minute,
	}
}

// ServiceBridge provides access to Helpin services from within tool execution.
type ServiceBridge struct {
	AddComment         func(ctx context.Context, workspaceID, storyID, agentID, content string) error
	UpdateStoryState   func(ctx context.Context, workspaceID, storyID, stateID string) error
	ListChecklist      func(ctx context.Context, workspaceID, storyID string) ([]model.PMChecklistItem, error)
	ListTicketMessages func(ctx context.Context, workspaceID, ticketID string) ([]model.SupportMessage, error)
	UpdateTicketStatus func(ctx context.Context, workspaceID, ticketID, status string) error
}

// ChecklistItem is a simplified checklist item for tool responses.
type ChecklistItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

// SupportDraftReply is a pending support reply requiring approval.
type SupportDraftReply struct {
	Content           string  `json:"content"`
	IsInternal        bool    `json:"is_internal"`
	SenderDisplayName *string `json:"sender_display_name,omitempty"`
	ApprovalRequired  bool    `json:"approval_required"`
}

// PRMetadata captures the result of an opened pull request.
type PRMetadata struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Number   int    `json:"number"`
	Head     string `json:"head"`
	Base     string `json:"base"`
}
