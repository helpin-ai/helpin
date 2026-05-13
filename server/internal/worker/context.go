package worker

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// executionContextSync holds the mutexes and shared mutable caches for an
// ExecutionContext. It lives behind a pointer so that struct-copying an
// ExecutionContext (used by cloneExecutionContext for short-lived post-run
// contexts) shares the synchronization primitives instead of copying them —
// a `go vet` lock-copy error and a real race hazard.
type executionContextSync struct {
	toolFileStateMu        sync.Mutex
	securityScannerCacheMu sync.Mutex
	securityScannerCache   map[string]securityScannerCacheEntry
}

// ExecutionContext holds all state for a single agent run execution.
type ExecutionContext struct {
	Context                    context.Context
	WorkDir                    string // path to cloned repo on disk
	WorkspaceID                string
	AgentID                    string
	RunID                      string
	TargetType                 string
	TargetID                   string
	TaskID                     string
	ConversationID             string
	Agent                      *model.Agent
	Task                       *model.PMTask
	Epic                       *model.PMEpic
	EpicTasks                  []model.PMTask
	Conversation               *model.SupportConversation
	GitIntegration             *model.GitIntegration
	GitAccessToken             string
	Repo                       string // e.g. "owner/repo"
	BaseBranch                 string
	WorkingBranch              string
	BranchSyncStatus           string
	BranchSyncConflictFiles    []string
	InitialInstructions        string
	PhaseGuidance              string
	RepairGuidance             string
	RepairGuidanceSource       string
	RepairGuidanceClass        string
	PlanningStage              string
	PlanningMethodology        string
	PlanningSpecDocumentID     string
	PlanningSpecVersionID      string
	RunInput                   *model.AgentRunInputPayload
	Config                     *WorkflowConfig
	ResolvedProfile            ResolvedProfile
	RuntimeSkillRefs           model.AgentSkillRefs
	ActiveRuntimeSkillRefs     model.AgentSkillRefs
	ActiveSkillInstructions    string
	SkillPolicy                SkillPolicy
	NativeSelectivePathEnabled bool
	AllowedTools               map[string]bool
	Services                   *ServiceBridge
	PendingSupportDraft        *SupportDraftReply
	LatestPRMetadata           *PRMetadata
	LocalGitCommit             *GitCommitMetadata
	Heartbeat                  func(stage string) error
	OnExecutionEvent           func(event ExecutionEvent)
	OnGitPush                  func(branch, sha string) error
	OnPROpen                   func(metadata PRMetadata, title string) error
	HeartbeatStageProvider     func() string
	HandleInteractivePause     func(result *ExecutionResult) (*LiveExecutionResumeSignal, error)
	PlanningTurnKind           string
	PlanningTurnAttempt        int
	TurnLocalInstructions      string
	RunFacts                   map[string]string
	ArtifactContext            *ArtifactContext
	ProviderContinuation       *ProviderContinuation
	ConversationHistory        []ExecutionMessage
	LastExecutionResult        *ExecutionResult
	StagedRuntimeSkillRoot     string
	ToolFileState              *ToolFileState
	// syncPtr is *executionContextSync, accessed atomically. We use
	// unsafe.Pointer (rather than atomic.Pointer[T]) because atomic.Pointer
	// embeds a noCopy marker that go vet's copylocks analyzer flags when
	// `clone := *execCtx` copies the struct in cloneExecutionContext.
	// unsafe.Pointer is safe to copy by value — both original and clone
	// then point to the same underlying executionContextSync.
	syncPtr              unsafe.Pointer
	PublishedPreviews    map[string]PublishedPreview
	CurrentAssistantText string
}

// ensureSync atomically initializes (if needed) and returns the
// synchronization block. Safe under concurrent access — multiple goroutines
// racing on first init produce a single shared *executionContextSync via CAS.
func (e *ExecutionContext) ensureSync() *executionContextSync {
	if p := atomic.LoadPointer(&e.syncPtr); p != nil {
		return (*executionContextSync)(p)
	}
	fresh := &executionContextSync{}
	if atomic.CompareAndSwapPointer(&e.syncPtr, nil, unsafe.Pointer(fresh)) {
		return fresh
	}
	// Lost the race: another goroutine installed its struct first.
	return (*executionContextSync)(atomic.LoadPointer(&e.syncPtr))
}

type LiveExecutionResumeSignal struct {
	Intent          string
	Content         string
	ResponsePayload json.RawMessage
	Acknowledge     func() error
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

const (
	defaultWorkflowMaxIterations  = 50
	plannerWorkflowMaxIterations  = 300
	defaultWorkflowTimeoutMinutes = 30
	defaultWorkflowCommandTimeout = 2 * time.Minute
)

// DefaultWorkflowConfig returns sensible defaults.
func DefaultWorkflowConfig() *WorkflowConfig {
	return DefaultWorkflowConfigForAgent(nil)
}

// DefaultWorkflowConfigForAgent returns runtime defaults, including higher
// tool budgets for custom agents and native planners.
func DefaultWorkflowConfigForAgent(agent *model.Agent) *WorkflowConfig {
	maxIterations := defaultWorkflowMaxIterations
	if isHighToolBudgetAgent(agent) {
		maxIterations = plannerWorkflowMaxIterations
	}
	return &WorkflowConfig{
		MaxIterations:  maxIterations,
		TimeoutMinutes: defaultWorkflowTimeoutMinutes,
		CommandTimeout: defaultWorkflowCommandTimeout,
	}
}

func isHighToolBudgetAgent(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	if !agent.IsSystem {
		return true
	}
	return isHighToolBudgetNativePlanner(agent)
}

func isHighToolBudgetNativePlanner(agent *model.Agent) bool {
	if agent == nil || strings.TrimSpace(agent.RuntimeKind) != "native_sdk" {
		return false
	}
	switch strings.TrimSpace(agent.EffectivePresetKey()) {
	case model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner:
		return true
	default:
		return false
	}
}

// ServiceBridge provides access to Helpin services from within tool execution.
type ServiceBridge struct {
	ExecuteInternalCommand func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error)

	// PM / Tasks
	AddComment          func(ctx context.Context, workspaceID, taskID, agentID, content string) error
	UpdateTaskState     func(ctx context.Context, workspaceID, taskID, stateID string) error
	ListChecklist       func(ctx context.Context, workspaceID, taskID string) ([]model.PMChecklistItem, error)
	CreateTaskBatch     func(ctx context.Context, workspaceID, epicID, actorID string, tasks []model.ProposedTask) (CreateTaskBatchResult, error)
	AssignTaskAgent     func(ctx context.Context, workspaceID, actorID, taskID, agentID string) error
	SetTaskDependencies func(ctx context.Context, workspaceID, actorID string, dependencies []TaskDependencyLink) error
	ListEpicTasks       func(ctx context.Context, workspaceID, epicID string) ([]EpicTaskSummary, error)
	ListWorkspaceTeams  func(ctx context.Context, workspaceID string) ([]WorkspaceTeamSummary, error)
	ListTeamWorkflows   func(ctx context.Context, workspaceID string, teamID *string) ([]TeamWorkflowSummary, error)
	ApproveEpicSpec     func(ctx context.Context, workspaceID, epicID, actorID string, versionID *string) (*model.ApprovedSpecSummary, error)
	CreateTask          func(ctx context.Context, workspaceID, actorID string, req CreateTaskToolRequest) (*CreateTaskToolResult, error)

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
	GetDocument                   func(ctx context.Context, id string) (*model.DocsDocument, error)
	GetDocumentKey                func(ctx context.Context, workspaceID, keyType, key string) (*model.DocsDocumentKey, error)
	ListCollections               func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsCollection, error)
	ListDocuments                 func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsDocument, error)
	SearchDocuments               func(ctx context.Context, workspaceID, query string, limit int) ([]DocsSearchHit, error)
	CreateDocument                func(ctx context.Context, workspaceID, userID string, req model.CreateDocsDocumentRequest, content json.RawMessage) (*model.DocsDocument, error)
	EnsureEpicSpecDoc             func(ctx context.Context, workspaceID, epicID, actorID string) (*model.DocsDocument, error)
	EnsureTaskPlanDoc             func(ctx context.Context, workspaceID, taskID, actorID string) (*model.DocsDocument, error)
	GetDocumentContent            func(ctx context.Context, documentID string) (string, error)
	ListDocumentBlocks            func(ctx context.Context, documentID string) ([]model.DocsBlock, error)
	PublishAISectionCandidate     func(ctx context.Context, workspaceID, documentID, blockID, agentID, runID string, currentContent, candidateContent json.RawMessage, candidateText string, sourceRefs model.JSONB, prompt *string, promptHash *string, modelName *string) (*model.DocsAISectionCandidate, error)
	PublishDocumentChangeProposal func(ctx context.Context, workspaceID string, req model.CreateDocsChangeProposalRequest) (*model.DocsChangeProposal, error)
	UpdateDocumentBlock           func(ctx context.Context, documentID, blockID string, revision int, content json.RawMessage, actorID string) (*model.DocsContent, error)
	WriteDocumentContent          func(ctx context.Context, workspaceID, documentID string, content json.RawMessage) error
	LinkDocumentToObject          func(ctx context.Context, workspaceID, documentID, linkedObjectType, linkedObjectID, linkContext, actorID string) error
	UpsertDocumentKey             func(ctx context.Context, record *model.DocsDocumentKey) error

	// Release facts
	GetReleaseContext      func(ctx context.Context, workspaceID string, req model.GetReleaseContextRequest) (*model.ReleaseContextResult, error)
	FindTasksForGitChanges func(ctx context.Context, workspaceID string, req model.FindTasksForGitChangesRequest) (*model.FindTasksForGitChangesResult, error)
	GetTaskContext         func(ctx context.Context, workspaceID string, req model.GetTaskContextRequest) (*model.GetTaskContextResult, error)
}

// DocsSearchHit is a simplified search result for tool responses.
type DocsSearchHit struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

type TaskDependencyLink struct {
	SourceTaskID string `json:"source_task_id"`
	TargetTaskID string `json:"target_task_id"`
}

type CreateTaskBatchTaskResult struct {
	Ref    string `json:"ref,omitempty"`
	TaskID string `json:"task_id"`
	Name   string `json:"name"`
}

type CreateTaskBatchResult struct {
	Tasks []CreateTaskBatchTaskResult `json:"tasks"`
}

// EpicTaskSummary is a simplified task for the list_epic_tasks tool.
type EpicTaskSummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	TaskType        string  `json:"task_type"`
	Status          string  `json:"status"` // "not_started", "in_progress", "done"
	Estimate        *int    `json:"estimate,omitempty"`
	Priority        string  `json:"priority"`
	AssignedAgentID *string `json:"assigned_agent_id,omitempty"`
}

type WorkspaceTeamSummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Handle          *string `json:"handle,omitempty"`
	TeamType        string  `json:"team_type,omitempty"`
	DefaultTaskType string  `json:"default_task_type,omitempty"`
}

type WorkflowStageSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StateType string `json:"state_type"`
	Position  int    `json:"position"`
	IsDefault bool   `json:"is_default"`
}

type TeamWorkflowSummary struct {
	TeamID           string                 `json:"team_id"`
	TeamName         string                 `json:"team_name"`
	WorkflowID       string                 `json:"workflow_id"`
	WorkflowName     string                 `json:"workflow_name"`
	DefaultStateID   *string                `json:"default_state_id,omitempty"`
	UsesTeamWorkflow bool                   `json:"uses_team_workflow"`
	Stages           []WorkflowStageSummary `json:"stages"`
}

type CreateTaskToolRequest struct {
	Name           string     `json:"name"`
	Description    *string    `json:"description,omitempty"`
	TaskType       string     `json:"task_type,omitempty"`
	Estimate       *int       `json:"estimate,omitempty"`
	Priority       *string    `json:"priority,omitempty"`
	EpicID         *string    `json:"epic_id,omitempty"`
	TeamID         string     `json:"team_id"`
	WorkflowID     *string    `json:"workflow_id,omitempty"`
	StateID        *string    `json:"state_id,omitempty"`
	OwnerMemberIDs []string   `json:"owner_member_ids,omitempty"`
	LabelIDs       []string   `json:"label_ids,omitempty"`
	Deadline       *time.Time `json:"deadline,omitempty"`
}

type CreateTaskToolResult struct {
	TaskID      string  `json:"task_id"`
	DisplayID   int     `json:"display_id,omitempty"`
	TaskKey     string  `json:"task_key,omitempty"`
	Name        string  `json:"name"`
	TeamID      *string `json:"team_id,omitempty"`
	WorkflowID  string  `json:"workflow_id"`
	StateID     string  `json:"state_id"`
	StateName   string  `json:"state_name,omitempty"`
	WorkspaceID string  `json:"workspace_id,omitempty"`
	EpicID      *string `json:"epic_id,omitempty"`
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

// GitCommitMetadata captures a local commit produced during a coding run.
type GitCommitMetadata struct {
	Branch        string   `json:"branch"`
	CommitSHA     string   `json:"commit_sha"`
	CommitMessage string   `json:"commit_message"`
	ChangedFiles  []string `json:"changed_files,omitempty"`
}
