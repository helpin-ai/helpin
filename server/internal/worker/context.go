package worker

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ExecutionContext holds all state for a single agent run execution.
type ExecutionContext struct {
	Context                   context.Context
	WorkDir                   string // path to cloned repo on disk
	WorkspaceID               string
	AgentID                   string
	RunID                     string
	TargetType                string
	TargetID                  string
	StoryID                   string
	ConversationID            string
	Agent                     *model.Agent
	Story                     *model.PMStory
	Epic                      *model.PMEpic
	EpicStories               []model.PMStory
	Conversation              *model.SupportConversation
	GitIntegration            *model.GitIntegration
	GitAccessToken            string
	Repo                      string // e.g. "owner/repo"
	BaseBranch                string
	WorkingBranch             string
	InitialInstructions       string
	PlanningStage             string
	PlanningMethodology       string
	PlanningSpecDocumentID    string
	PlanningSpecVersionID     string
	PlanningWebSearchEnabled  bool
	PlanningWebSearchProvider string
	Config                    *WorkflowConfig
	RuntimeProfile            model.RuntimeProfile
	AllowedTools              map[string]bool
	Services                  *ServiceBridge
	PendingSupportDraft       *SupportDraftReply
	LatestPRMetadata          *PRMetadata
	Heartbeat                 func(stage string) error
	OnGitPush                 func(branch, sha string) error
	OnPROpen                  func(metadata PRMetadata, title string) error
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
	// PM / Stories
	AddComment         func(ctx context.Context, workspaceID, storyID, agentID, content string) error
	UpdateStoryState   func(ctx context.Context, workspaceID, storyID, stateID string) error
	ListChecklist      func(ctx context.Context, workspaceID, storyID string) ([]model.PMChecklistItem, error)

	// Support
	ListConversationMessages func(ctx context.Context, workspaceID, conversationID string) ([]model.SupportMessage, error)
	UpdateConversationStatus func(ctx context.Context, workspaceID, conversationID, status string) error

	// CRM
	ListDeals          func(ctx context.Context, workspaceID string, limit int) ([]model.CRMDeal, error)
	GetDeal            func(ctx context.Context, id string) (*model.CRMDeal, error)
	UpdateDealStage    func(ctx context.Context, dealID, stageID string) error
	AddDealNote        func(ctx context.Context, workspaceID, dealID, agentID, content string) error
	ListContacts       func(ctx context.Context, workspaceID string, limit int) ([]model.CRMContact, error)
	ListBuyerSignals   func(ctx context.Context, workspaceID string, dealID *string, limit int) ([]model.CRMBuyerSignal, error)

	// Docs
	GetDocument        func(ctx context.Context, id string) (*model.DocsDocument, error)
	ListDocuments      func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsDocument, error)
	SearchDocuments    func(ctx context.Context, workspaceID, query string, limit int) ([]DocsSearchHit, error)
}

// DocsSearchHit is a simplified search result for tool responses.
type DocsSearchHit struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
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
