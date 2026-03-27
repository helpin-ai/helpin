package worker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ExecutionContext holds all state for a single agent run execution.
type ExecutionContext struct {
	Context                context.Context
	WorkDir                string // path to cloned repo on disk
	WorkspaceID            string
	AgentID                string
	RunID                  string
	TargetType             string
	TargetID               string
	StoryID                string
	ConversationID         string
	Agent                  *model.Agent
	Story                  *model.PMStory
	Epic                   *model.PMEpic
	EpicStories            []model.PMStory
	Conversation           *model.SupportConversation
	GitIntegration         *model.GitIntegration
	GitAccessToken         string
	Repo                   string // e.g. "owner/repo"
	BaseBranch             string
	WorkingBranch          string
	InitialInstructions    string
	PlanningStage          string
	PlanningMethodology    string
	PlanningSpecDocumentID string
	PlanningSpecVersionID  string
	Config                 *WorkflowConfig
	ResolvedProfile        ResolvedProfile
	AllowedTools           map[string]bool
	Services               *ServiceBridge
	PendingSupportDraft    *SupportDraftReply
	LatestPRMetadata       *PRMetadata
	Heartbeat              func(stage string) error
	OnExecutionEvent       func(event ExecutionEvent)
	OnGitPush              func(branch, sha string) error
	OnPROpen               func(metadata PRMetadata, title string) error
	PlanningTurnKind       string
	PlanningTurnAttempt    int
	RunFacts               map[string]string
	ArtifactContext        *ArtifactContext
	ProviderContinuation   *ProviderContinuation
	ConversationHistory    []ExecutionMessage
	LastExecutionResult    *ExecutionResult
	ToolFileState          *ToolFileState
	toolFileStateMu        sync.Mutex
	PublishedPreviews      map[string]PublishedPreview
	CurrentAssistantText   string
}

type ArtifactContext struct {
	Entries []ArtifactContextEntry
}

type ArtifactContextEntry struct {
	Label   string
	Source  string
	Status  string
	Format  string
	Content string
	// PreserveFull marks canonical artifacts that should not be trimmed from prompt context.
	PreserveFull bool
}

type ProviderContinuation struct {
	Provider           string
	ResponseID         string
	PreviousResponseID string
	AfterSequenceNo    int
}

const (
	PlanningTurnKindInitial = "initial"
	PlanningTurnKindMessage = "message"
)

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
	ExecuteInternalCommand func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error)

	// PM / Stories
	AddComment           func(ctx context.Context, workspaceID, storyID, agentID, content string) error
	UpdateStoryState     func(ctx context.Context, workspaceID, storyID, stateID string) error
	ListChecklist        func(ctx context.Context, workspaceID, storyID string) ([]model.PMChecklistItem, error)
	CreateStoryBatch     func(ctx context.Context, workspaceID, epicID, actorID string, stories []model.ProposedStory) (CreateStoryBatchResult, error)
	AssignStoryAgent     func(ctx context.Context, workspaceID, actorID, storyID, agentID string) error
	SetStoryDependencies func(ctx context.Context, workspaceID, actorID string, dependencies []StoryDependencyLink) error
	ListEpicStories      func(ctx context.Context, workspaceID, epicID string) ([]EpicStorySummary, error)
	ListWorkspaceTeams   func(ctx context.Context, workspaceID string) ([]WorkspaceTeamSummary, error)
	ApproveEpicSpec      func(ctx context.Context, workspaceID, epicID, actorID string, versionID *string) (*model.ApprovedSpecSummary, error)

	// Support
	ListConversationMessages func(ctx context.Context, workspaceID, conversationID string) ([]model.SupportMessage, error)
	UpdateConversationStatus func(ctx context.Context, workspaceID, conversationID, status string) error

	// CRM
	ListDeals        func(ctx context.Context, workspaceID string, limit int) ([]model.CRMDeal, error)
	GetDeal          func(ctx context.Context, id string) (*model.CRMDeal, error)
	UpdateDealStage  func(ctx context.Context, dealID, stageID string) error
	AddDealNote      func(ctx context.Context, workspaceID, dealID, agentID, content string) error
	ListContacts     func(ctx context.Context, workspaceID string, limit int) ([]model.CRMContact, error)
	ListBuyerSignals func(ctx context.Context, workspaceID string, dealID *string, limit int) ([]model.CRMBuyerSignal, error)

	// Docs
	GetDocument          func(ctx context.Context, id string) (*model.DocsDocument, error)
	ListDocuments        func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsDocument, error)
	SearchDocuments      func(ctx context.Context, workspaceID, query string, limit int) ([]DocsSearchHit, error)
	EnsureEpicSpecDoc    func(ctx context.Context, workspaceID, epicID, actorID string) (*model.DocsDocument, error)
	EnsureStoryPlanDoc   func(ctx context.Context, workspaceID, storyID, actorID string) (*model.DocsDocument, error)
	GetDocumentContent   func(ctx context.Context, documentID string) (string, error)
	WriteDocumentContent func(ctx context.Context, workspaceID, documentID string, content json.RawMessage) error
	LinkDocumentToObject func(ctx context.Context, workspaceID, documentID, linkedObjectType, linkedObjectID, linkContext, actorID string) error
}

// DocsSearchHit is a simplified search result for tool responses.
type DocsSearchHit struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

type StoryDependencyLink struct {
	SourceStoryID string `json:"source_story_id"`
	TargetStoryID string `json:"target_story_id"`
}

type CreateStoryBatchStoryResult struct {
	Ref     string `json:"ref,omitempty"`
	StoryID string `json:"story_id"`
	Name    string `json:"name"`
}

type CreateStoryBatchResult struct {
	Stories []CreateStoryBatchStoryResult `json:"stories"`
}

// EpicStorySummary is a simplified story for the list_epic_stories tool.
type EpicStorySummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	StoryType       string  `json:"story_type"`
	Status          string  `json:"status"` // "not_started", "in_progress", "done"
	Estimate        *int    `json:"estimate,omitempty"`
	Priority        string  `json:"priority"`
	AssignedAgentID *string `json:"assigned_agent_id,omitempty"`
}

type WorkspaceTeamSummary struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Handle           *string `json:"handle,omitempty"`
	TeamType         string  `json:"team_type,omitempty"`
	DefaultStoryType string  `json:"default_story_type,omitempty"`
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
