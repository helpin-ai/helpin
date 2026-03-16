# Interactive Planning Sessions

## Problem

Epic planning currently works in batch mode: the planner agent generates a full spec in one autonomous run, dumps assumptions and open questions, then the human resolves them in a flat form. If the answers change the spec direction, the only option is a full redraft — a new run that regenerates the entire spec from scratch, losing conversational context and wasting tokens.

The planning phase needs to be a dialogue, not a batch job. The planner should ask questions progressively, propose spec sections incrementally, and evolve the spec based on the human's answers — all within a single persistent conversation.

## Design Principles

1. **Planning is collaborative, execution is autonomous.** The planning session model is parallel to (not a replacement for) the existing autonomous run model. Engineer/reviewer agents keep running autonomously via Temporal + executor tool loop.

2. **The conversation is the process, the spec is the artifact.** The session produces a spec document as its output, but the real value is the structured dialogue that surfaces the right decisions.

3. **Progressive disclosure over batch interrogation.** The planner asks 2-3 gating questions first, then narrows based on answers. Not 15 questions dumped in a wall of text.

4. **Resumable by design.** Human closes laptop, comes back tomorrow. Full conversation history is in the database. The next LLM call picks up exactly where it left off.

5. **Temporal orchestrates lifecycle, not conversation.** Temporal handles session timeout, final artifact generation, and state transitions. The actual back-and-forth runs through a WebSocket-backed service endpoint, not through a Temporal activity.

6. **Backward compatible.** The existing batch `DraftEpicSpec` flow remains available as "auto-draft" mode for teams that prefer fire-and-forget planning. The interactive session is an alternative, not a replacement.

---

## Architecture Overview

```
React (Split View)                    Go API Server
┌─────────────┬──────────┐
│ Chat Panel  │ Spec     │◄──── WebSocket ────► PlanningSessionHandler
│             │ Draft    │                            │
│ Q&A back    │ (live    │      REST (send msg) ──► PlanningSessionService
│ and forth   │  update) │                            │
│             │          │                       Claude API (streaming)
│             │          │                            │
│ [input]     │ [Approve]│                       Message persistence
└─────────────┴──────────┘                            │
                                                 Temporal (lifecycle)
                                                   - timeout/abandon
                                                   - finalize spec
                                                   - transition epic state
```

### Interaction flow

1. Human clicks "Start Planning Session" on the epic
2. API creates a `PlanningSession` record, loads epic context (linked docs, support tickets, repo summary, existing spec if redrafting)
3. API makes the first Claude call with the full context and the planning prompt pack, asking the agent to introduce itself and ask its first prioritized questions
4. The agent's response is saved as a `PlanningSessionMessage` and pushed to the frontend via WebSocket
5. Human types an answer in the chat panel
6. API saves the human message, appends it to the conversation history, makes the next Claude call
7. When the agent proposes a spec section, it's tagged as `message_type: proposal` with structured metadata linking it to a spec section. The frontend renders it in the spec panel
8. Loop continues until the agent determines it has enough to finalize, or the human says "write it up"
9. Agent produces the final spec markdown. API saves it to the Docs module as a new version
10. Session status transitions to `completed`. Epic planning_state transitions to `awaiting_spec_approval`
11. Existing approval flow takes over from here (unchanged)

---

## Data Model

### New table: `planning_sessions`

```sql
CREATE TABLE IF NOT EXISTS planning_sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspaces(id),
    epic_id             UUID NOT NULL REFERENCES pm_epics(id),
    agent_id            UUID NOT NULL REFERENCES pm_agents(id),
    status              TEXT NOT NULL DEFAULT 'active',
        -- active: conversation in progress
        -- paused: human left, can resume
        -- finalizing: agent writing final spec
        -- completed: spec written, session done
        -- abandoned: timed out or cancelled
    planning_methodology TEXT NOT NULL DEFAULT 'structured_v1',
    spec_document_id    UUID REFERENCES docs_documents(id),
    spec_sections       JSONB NOT NULL DEFAULT '[]',
        -- Tracks proposed spec sections and their status
        -- [{ "key": "auth_flow", "title": "Authentication Flow", "status": "proposed|confirmed|revised", "message_id": "uuid" }]
    context_snapshot    JSONB NOT NULL DEFAULT '{}',
        -- Frozen context at session start: epic metadata, linked docs summaries,
        -- support ticket signals, repo structure summary
    token_usage         JSONB NOT NULL DEFAULT '{"input": 0, "output": 0}',
    started_by          UUID,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_planning_sessions_epic ON planning_sessions(epic_id);
CREATE INDEX idx_planning_sessions_workspace ON planning_sessions(workspace_id);
CREATE INDEX idx_planning_sessions_status ON planning_sessions(workspace_id, status);
```

### New table: `planning_session_messages`

```sql
CREATE TABLE IF NOT EXISTS planning_session_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES planning_sessions(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
        -- 'assistant' or 'user'
    content         TEXT NOT NULL,
        -- Markdown content of the message
    message_type    TEXT NOT NULL DEFAULT 'message',
        -- question: agent asking a prioritized question
        -- answer: human response to a question
        -- proposal: agent proposing a spec section (has section_metadata)
        -- confirmation: human confirming/redirecting a proposal
        -- context: agent sharing context it found (repo analysis, ticket patterns)
        -- summary: agent summarizing decisions made so far
        -- final_spec: agent's final spec output
        -- message: general conversation
    section_metadata JSONB,
        -- For proposal messages: { "section_key": "auth_flow", "section_title": "Authentication Flow", "spec_markdown": "..." }
        -- For question messages: { "priority": 1, "gates": ["auth_flow", "data_model"], "category": "scope" }
        -- For context messages: { "source": "support_tickets", "ticket_ids": ["..."] }
    token_usage     JSONB,
        -- { "input": N, "output": N } for assistant messages (the Claude call that produced this)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_planning_session_messages_session ON planning_session_messages(session_id, created_at);
```

### Epic model changes

Add to `PMEpic`:

```go
// New field
ActivePlanningSessionID *string `json:"active_planning_session_id,omitempty" gorm:"type:uuid;index"`
```

### New planning state

Add to existing planning state constants:

```go
EpicPlanningStateInSession = "in_session"  // Interactive planning session active
```

This slots between `not_started` and `awaiting_spec_clarification`. The state machine becomes:

```
not_started
  ├─► in_session (interactive path)
  │     └─► awaiting_spec_approval (session completed, spec written)
  │
  ├─► [draft_spec run] (batch path, unchanged)
  │     ├─► awaiting_spec_clarification
  │     └─► awaiting_spec_approval
  │
  └─► awaiting_spec_approval
        └─► ready_for_story_planning
              └─► awaiting_plan_approval
                    └─► stories_created
                          ├─► execution_started
                          └─► ready_for_execution
```

---

## Go Backend

### New model file: `server/internal/model/planning_session.go`

```go
type PlanningSession struct {
    ID                   string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID          string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
    EpicID               string          `json:"epic_id" gorm:"type:uuid;not null;index"`
    AgentID              string          `json:"agent_id" gorm:"type:uuid;not null"`
    Status               string          `json:"status" gorm:"not null;default:'active'"`
    PlanningMethodology  string          `json:"planning_methodology" gorm:"not null;default:'structured_v1'"`
    SpecDocumentID       *string         `json:"spec_document_id,omitempty" gorm:"type:uuid"`
    SpecSections         json.RawMessage `json:"spec_sections" gorm:"type:jsonb;not null;default:'[]'"`
    ContextSnapshot      json.RawMessage `json:"context_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
    TokenUsage           json.RawMessage `json:"token_usage" gorm:"type:jsonb;not null;default:'{\"input\":0,\"output\":0}'"`
    StartedBy            *string         `json:"started_by,omitempty" gorm:"type:uuid"`
    StartedAt            time.Time       `json:"started_at" gorm:"not null;default:now()"`
    LastActiveAt         time.Time       `json:"last_active_at" gorm:"not null;default:now()"`
    CompletedAt          *time.Time      `json:"completed_at,omitempty"`
    CreatedAt            time.Time       `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt            time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PlanningSession) TableName() string { return "planning_sessions" }

type PlanningSessionMessage struct {
    ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    SessionID       string          `json:"session_id" gorm:"type:uuid;not null;index"`
    Role            string          `json:"role" gorm:"not null"`
    Content         string          `json:"content" gorm:"not null"`
    MessageType     string          `json:"message_type" gorm:"not null;default:'message'"`
    SectionMetadata json.RawMessage `json:"section_metadata,omitempty" gorm:"type:jsonb"`
    TokenUsage      json.RawMessage `json:"token_usage,omitempty" gorm:"type:jsonb"`
    CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (PlanningSessionMessage) TableName() string { return "planning_session_messages" }

// Constants
const (
    PlanningSessionStatusActive     = "active"
    PlanningSessionStatusPaused     = "paused"
    PlanningSessionStatusFinalizing = "finalizing"
    PlanningSessionStatusCompleted  = "completed"
    PlanningSessionStatusAbandoned  = "abandoned"

    PlanningMessageTypeQuestion     = "question"
    PlanningMessageTypeAnswer       = "answer"
    PlanningMessageTypeProposal     = "proposal"
    PlanningMessageTypeConfirmation = "confirmation"
    PlanningMessageTypeContext      = "context"
    PlanningMessageTypeSummary      = "summary"
    PlanningMessageTypeFinalSpec    = "final_spec"
    PlanningMessageTypeMessage      = "message"
)

// Request/Response DTOs
type StartPlanningSessionRequest struct {
    AgentID           string  `json:"agent_id"`
    AdditionalContext *string `json:"additional_context,omitempty"`
}

type SendPlanningMessageRequest struct {
    Content     string  `json:"content"`
    MessageType *string `json:"message_type,omitempty"` // defaults to "answer" or "message"
}

type FinalizePlanningSessionRequest struct {
    // Empty for now; agent writes the spec automatically
    // Could later support: { "sections_to_include": ["auth_flow", "data_model"] }
}
```

### New repository: `server/internal/repository/planning_session.go`

Standard GORM CRUD following existing patterns:

```go
type PlanningSessionRepository struct {
    db *gorm.DB
}

func (r *PlanningSessionRepository) Create(ctx, session) (*PlanningSession, error)
func (r *PlanningSessionRepository) GetByID(ctx, id) (*PlanningSession, error)
func (r *PlanningSessionRepository) GetActiveByEpicID(ctx, epicID) (*PlanningSession, error)
func (r *PlanningSessionRepository) Update(ctx, session) error
func (r *PlanningSessionRepository) UpdateLastActive(ctx, id) error

func (r *PlanningSessionRepository) CreateMessage(ctx, msg) (*PlanningSessionMessage, error)
func (r *PlanningSessionRepository) ListMessages(ctx, sessionID) ([]PlanningSessionMessage, error)
func (r *PlanningSessionRepository) ListMessagesByRole(ctx, sessionID, role) ([]PlanningSessionMessage, error)

// For session timeout detection
func (r *PlanningSessionRepository) ListStale(ctx, workspaceID, staleSince time.Time) ([]PlanningSession, error)
```

### New service: `server/internal/service/planning_session.go`

This is the core service. It manages session lifecycle and drives the Claude conversation.

```go
type PlanningSessionService struct {
    sessionRepo       *repository.PlanningSessionRepository
    epicRepo          *repository.PMEpicRepository
    agentRepo         *repository.PMAgentRepository
    docsContentRepo   *repository.DocsContentRepository
    docsVersionRepo   *repository.DocsVersionRepository
    docsSvc           *DocsService
    claudeClient      *worker.ClaudeClient       // Direct Claude API access (reuse from executor)
    wsPublisher       websocket.Publisher
    settingsRepo      *repository.SettingsRepository
}
```

#### StartSession

```go
func (s *PlanningSessionService) StartSession(ctx context.Context, workspaceID, epicID, actorID string, req model.StartPlanningSessionRequest) (*model.PlanningSession, *model.PlanningSessionMessage, error)
```

1. Validate epic exists, belongs to workspace, has a planning repository
2. Check no active session already exists for this epic
3. Resolve agent (must be `product_planner` class)
4. Resolve planning methodology from workspace AI settings
5. Build context snapshot:
   - Epic metadata (name, description, objectives, labels)
   - Linked docs summaries (fetch from docs service)
   - Linked support ticket patterns (if any)
   - Existing spec content (if redrafting)
   - Repo structure summary (from planning repository, if available — bounded to key directories and file types)
   - Additional context from request
6. Ensure spec document exists (reuse `ensureEpicSpecDocument`)
7. Create `PlanningSession` record
8. Update epic: `planning_state = "in_session"`, `active_planning_session_id = session.ID`
9. Build the initial system prompt (reuse planning prompt pack with modifications for interactive mode)
10. Build the initial user message from context snapshot
11. Call Claude API with system prompt + initial user message
12. Parse response, create `PlanningSessionMessage` (role: assistant, message_type determined from content)
13. Save token usage
14. Publish WebSocket event: `planning_session-created`
15. Return session + first assistant message

#### SendMessage

```go
func (s *PlanningSessionService) SendMessage(ctx context.Context, workspaceID, sessionID, actorID string, req model.SendPlanningMessageRequest) (*model.PlanningSessionMessage, error)
```

1. Load session, validate status is `active`
2. Save user message to `planning_session_messages`
3. Load full message history from DB
4. Convert to Claude API message format (alternating user/assistant)
5. Call Claude API with system prompt + full message history
6. Parse response:
   - If response contains a spec section proposal: extract `section_metadata`, set `message_type = "proposal"`, update `session.spec_sections`
   - If response contains a question: extract priority/category metadata, set `message_type = "question"`
   - If response indicates readiness to finalize: set `message_type = "summary"`
   - Otherwise: `message_type = "message"`
7. Save assistant message to DB
8. Update `session.last_active_at` and `session.token_usage`
9. Publish WebSocket event: `planning_session_message-created` with message data
10. Return the assistant message

**Spec section detection:** The planning prompt instructs the agent to use a structured format when proposing sections:

```
When proposing a spec section, output it in this format:

<spec_section key="auth_flow" title="Authentication Flow">
## Authentication Flow

The system SHALL support OAuth2 with Google and GitHub providers...
</spec_section>
```

The service parses these tags from the response, extracts the markdown, and stores it in `section_metadata`. The raw message content (including the tags) goes to the chat panel. The extracted spec markdown goes to the spec panel.

#### FinalizeSession

```go
func (s *PlanningSessionService) FinalizeSession(ctx context.Context, workspaceID, sessionID, actorID string) (*model.PlanningSession, error)
```

1. Load session, validate status is `active` or `paused`
2. Set status to `finalizing`
3. Load full message history
4. Make one final Claude call with instruction: "Based on our conversation, write the complete product specification in markdown. Include all confirmed sections and decisions."
5. Save the response as `message_type = "final_spec"`
6. Write the spec content to the Docs module:
   - Update doc content via docs service
   - Create a `DocsVersion` with label "AI Draft (Interactive)"
7. Extract and persist `assumptions`, `open_questions`, `risks` from the final output as `SpecClarificationItem` on the epic (for compatibility with the existing approval flow)
8. Update session: `status = "completed"`, `completed_at = now()`
9. Update epic:
   - `planning_state = "awaiting_spec_approval"` (or `"awaiting_spec_clarification"` if clarifications exist)
   - `active_planning_session_id = nil`
   - `last_planning_run_id` — create a lightweight `AgentRun` record for audit trail consistency (target_type=epic, status=completed, output_summary with session reference)
10. Publish WebSocket event: `planning_session-updated`
11. Return updated session

#### ResumeSession

```go
func (s *PlanningSessionService) ResumeSession(ctx context.Context, workspaceID, sessionID, actorID string) (*model.PlanningSession, []model.PlanningSessionMessage, error)
```

1. Load session, validate status is `paused` or `active`
2. Set status to `active`, update `last_active_at`
3. Load all messages
4. Return session + messages (frontend reconstructs the conversation)

#### AbandonSession

```go
func (s *PlanningSessionService) AbandonSession(ctx context.Context, workspaceID, sessionID, actorID string) error
```

1. Set status to `abandoned`
2. Reset epic: `planning_state = "not_started"`, `active_planning_session_id = nil`
3. Publish WebSocket event

#### GetSession

```go
func (s *PlanningSessionService) GetSession(ctx context.Context, workspaceID, sessionID string) (*model.PlanningSession, error)
func (s *PlanningSessionService) GetSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]model.PlanningSessionMessage, error)
func (s *PlanningSessionService) GetActiveSessionByEpicID(ctx context.Context, workspaceID, epicID string) (*model.PlanningSession, error)
```

Standard read operations.

### Planning prompt pack modifications

The existing planning prompt pack in `worker/planning_prompt_pack.go` builds prompts for autonomous runs. For interactive sessions, we need a variant.

Add to `worker/planning_prompt_pack.go` (or a new `planning_session_prompt.go`):

```go
func BuildPlanningSessionSystemPrompt(agent *model.Agent, methodology string) string
```

Key differences from the autonomous `BuildSystemPrompt`:

1. **Identity**: "You are a product planner working interactively with a human product owner."
2. **Interaction style**: "Ask questions progressively. Start with the 2-3 most important scope-gating questions. Based on answers, ask follow-ups. Do not ask all questions at once."
3. **Spec section format**: Instruct the agent to use `<spec_section>` tags when proposing sections (see detection format above).
4. **Decision tracking**: "When the human confirms a decision, acknowledge it and note it as confirmed. When they redirect, update your understanding."
5. **Finalization awareness**: "When you believe you have enough information to write a complete spec, tell the human and summarize the key decisions made. Wait for them to confirm before finalizing."
6. **No JSON schema constraint**: Unlike autonomous runs that must output structured JSON, the interactive session uses natural language with embedded structured tags.

The initial user message is built from the context snapshot:

```go
func BuildPlanningSessionInitialMessage(contextSnapshot json.RawMessage) string
```

This renders the epic metadata, linked docs, support signals, and existing spec into a structured prompt that tells the agent: "Here's what I know. Begin by asking your most important questions."

### Claude API access

The existing `worker.ClaudeClient` (used by `Executor`) makes direct Claude API calls. The planning session service needs the same client but without the tool-loop wrapper.

Two options:
- **Option A**: Extract the Claude API client from `worker/claude.go` into a shared package (e.g., `internal/llm/claude.go`) that both the executor and the planning session service can import.
- **Option B**: Have the planning session service import the worker package and use `ClaudeClient` directly.

**Recommendation: Option A.** The Claude client is a simple HTTP wrapper with no execution-specific logic. Moving it to `internal/llm/` keeps the dependency clean. The executor continues to use it through the new import path.

The planning session service calls Claude with:
- `system`: from `BuildPlanningSessionSystemPrompt`
- `messages`: full conversation history from DB (converted to Claude message format)
- `max_tokens`: 8192 (shorter than autonomous runs since each turn is incremental)
- `tools`: none for v1 (planning is conversation-only; repo analysis happens at session start via context snapshot)

Future: add read-only tools (file read, search) so the agent can look at the codebase mid-conversation. This is a natural extension but not required for v1.

### New handler: `server/internal/handler/planning_session.go`

```go
type PlanningSessionHandler struct {
    sessionService *service.PlanningSessionService
}

func (h *PlanningSessionHandler) Start(w, r)       // POST /pm/epics/{epicId}/planning-session
func (h *PlanningSessionHandler) Get(w, r)          // GET /pm/planning-sessions/{sessionId}
func (h *PlanningSessionHandler) GetMessages(w, r)  // GET /pm/planning-sessions/{sessionId}/messages
func (h *PlanningSessionHandler) SendMessage(w, r)  // POST /pm/planning-sessions/{sessionId}/messages
func (h *PlanningSessionHandler) Finalize(w, r)     // POST /pm/planning-sessions/{sessionId}/finalize
func (h *PlanningSessionHandler) Abandon(w, r)      // POST /pm/planning-sessions/{sessionId}/abandon
```

### Router additions: `server/internal/router/router.go`

```go
// Inside the PM epic routes
r.Route("/epics/{epicId}", func(r chi.Router) {
    // ... existing routes ...
    r.With(requirePerm(PermPMEdit)).
        Post("/planning-session", h.PlanningSession.Start)
})

// New top-level planning session routes
r.Route("/planning-sessions/{sessionId}", func(r chi.Router) {
    r.With(requirePerm(PermPMRead)).
        Get("/", h.PlanningSession.Get)
    r.With(requirePerm(PermPMRead)).
        Get("/messages", h.PlanningSession.GetMessages)
    r.With(requirePerm(PermPMEdit)).
        Post("/messages", h.PlanningSession.SendMessage)
    r.With(requirePerm(PermPMEdit)).
        Post("/finalize", h.PlanningSession.Finalize)
    r.With(requirePerm(PermPMEdit)).
        Post("/abandon", h.PlanningSession.Abandon)
})
```

### WebSocket events

New event types (using existing `websocket.Event` structure):

| Entity | Action | When |
|---|---|---|
| `planning_session` | `created` | Session started |
| `planning_session` | `updated` | Status change (paused, finalizing, completed, abandoned) |
| `planning_session_message` | `created` | New message (assistant or user) |

The `planning_session_message-created` event carries the message ID in `EntityID` and the session ID in `ParentID` with `ParentType = "planning_session"`. The frontend listens for this to append new messages to the chat panel in real time.

### DI wiring: `server/cmd/api/main.go`

```go
planningSessionRepo := repository.NewPlanningSessionRepository(db)
claudeClient := llm.NewClaudeClient(cfg.AnthropicAPIKey)  // extracted from worker package

planningSessionService := service.NewPlanningSessionService(
    planningSessionRepo,
    epicRepo,
    agentRepo,
    docsContentRepo,
    docsVersionRepo,
    docsSvc,
    claudeClient,
    wsPublisher,
    settingsRepo,
)

planningSessionHandler := handler.NewPlanningSessionHandler(planningSessionService)
```

### Temporal: session lifecycle

A lightweight Temporal workflow handles session timeout:

```go
// PlanningSessionTimeoutWorkflow
// Started when a session is created. Checks session activity periodically.
func PlanningSessionTimeoutWorkflow(ctx workflow.Context, sessionID string) error {
    for {
        // Sleep 1 hour
        workflow.Sleep(ctx, 1 * time.Hour)

        // Check session activity
        var isStale bool
        workflow.ExecuteActivity(ctx, CheckSessionActivityActivity, sessionID).Get(ctx, &isStale)

        if isStale {
            // Pause the session (not abandon — human can resume)
            workflow.ExecuteActivity(ctx, PauseSessionActivity, sessionID)
            return nil
        }

        // Check if session is completed or abandoned
        var status string
        workflow.ExecuteActivity(ctx, GetSessionStatusActivity, sessionID).Get(ctx, &status)
        if status == "completed" || status == "abandoned" {
            return nil
        }
    }
}
```

Staleness threshold: 2 hours of inactivity → pause. After 48 hours paused → mark abandoned (separate cron or second timer).

This is intentionally lightweight. The Temporal workflow is just a timer, not an executor.

### Audit trail compatibility

The existing planning flow creates `AgentRun` records that serve as audit trail. Interactive sessions need equivalent traceability.

When a session completes (`FinalizeSession`), create a single `AgentRun` record:

```go
run := &model.AgentRun{
    WorkspaceID:   session.WorkspaceID,
    AgentID:       session.AgentID,
    TargetType:    "epic",
    TargetID:      session.EpicID,
    RuntimeKind:   "native_claude",
    Status:        model.RunStatusCompleted,
    ApprovalState: model.ApprovalStatePending,
    Input:         json.Marshal(planningRunInput{Stage: "interactive_session", SpecDocumentID: *session.SpecDocumentID}),
    OutputSummary: json.Marshal(epicPlanningRunSummary{
        Stage:          "interactive_session",
        SpecDocumentID: *session.SpecDocumentID,
        Summary:        "Interactive planning session",
        // Extracted from final spec
        Assumptions:    extractedAssumptions,
        OpenQuestions:  extractedOpenQuestions,
    }),
    TokensUsed:    totalTokens,
    CompletedAt:   session.CompletedAt,
}
```

This means the existing `EpicOrchestrationPanel` (which lists `agent_runs` for the epic) will show the session as a completed run. The downstream flow (approve spec → plan stories → confirm → kickoff) works unchanged.

---

## Frontend

### New types: `frontend/src/lib/pmTypes.ts`

```typescript
interface PlanningSession {
  id: string;
  workspace_id: string;
  epic_id: string;
  agent_id: string;
  status: 'active' | 'paused' | 'finalizing' | 'completed' | 'abandoned';
  planning_methodology: string;
  spec_document_id?: string;
  spec_sections: SpecSectionEntry[];
  context_snapshot: Record<string, unknown>;
  token_usage: { input: number; output: number };
  started_by?: string;
  started_at: string;
  last_active_at: string;
  completed_at?: string;
}

interface SpecSectionEntry {
  key: string;
  title: string;
  status: 'proposed' | 'confirmed' | 'revised';
  message_id: string;
}

interface PlanningSessionMessage {
  id: string;
  session_id: string;
  role: 'assistant' | 'user';
  content: string;
  message_type: 'question' | 'answer' | 'proposal' | 'confirmation' | 'context' | 'summary' | 'final_spec' | 'message';
  section_metadata?: {
    section_key?: string;
    section_title?: string;
    spec_markdown?: string;
    priority?: number;
    gates?: string[];
    category?: string;
    source?: string;
  };
  token_usage?: { input: number; output: number };
  created_at: string;
}
```

### New service: `frontend/src/lib/services/planningSessionService.ts`

```typescript
export const planningSessionService = {
  start: (workspaceId: string, epicId: string, req: { agent_id: string; additional_context?: string }) =>
    api.post<PlanningSession>(`/pm/epics/${epicId}/planning-session?workspace_id=${workspaceId}`, req),

  get: (workspaceId: string, sessionId: string) =>
    api.get<PlanningSession>(`/pm/planning-sessions/${sessionId}?workspace_id=${workspaceId}`),

  getMessages: (workspaceId: string, sessionId: string) =>
    api.get<PlanningSessionMessage[]>(`/pm/planning-sessions/${sessionId}/messages?workspace_id=${workspaceId}`),

  sendMessage: (workspaceId: string, sessionId: string, req: { content: string; message_type?: string }) =>
    api.post<PlanningSessionMessage>(`/pm/planning-sessions/${sessionId}/messages?workspace_id=${workspaceId}`, req),

  finalize: (workspaceId: string, sessionId: string) =>
    api.post<PlanningSession>(`/pm/planning-sessions/${sessionId}/finalize?workspace_id=${workspaceId}`, {}),

  abandon: (workspaceId: string, sessionId: string) =>
    api.post<void>(`/pm/planning-sessions/${sessionId}/abandon?workspace_id=${workspaceId}`, {}),
};
```

### New query hooks: `frontend/src/hooks/queries/usePlanningSession.ts`

```typescript
export function usePlanningSession(workspaceId: string, sessionId?: string)
export function useActivePlanningSession(workspaceId: string, epicId?: string)
export function usePlanningSessionMessages(workspaceId: string, sessionId?: string)
export function useStartPlanningSession(workspaceId: string)
export function useSendPlanningMessage(workspaceId: string)
export function useFinalizePlanningSession(workspaceId: string)
export function useAbandonPlanningSession(workspaceId: string)
```

Add to barrel export in `frontend/src/hooks/queries/index.ts`.

### Query keys: `frontend/src/lib/queryKeys.ts`

```typescript
planningSessions: {
  byId: (sessionId: string) => ['planning-sessions', sessionId] as const,
  activeByEpic: (epicId: string) => ['planning-sessions', 'active', epicId] as const,
  messages: (sessionId: string) => ['planning-sessions', sessionId, 'messages'] as const,
}
```

### New component: `frontend/src/components/pm/PlanningSessionPanel.tsx`

Split-view panel that replaces the current `DraftSpecStep` when an interactive session is active.

**Layout:**

```
┌─ PlanningSessionPanel ──────────────────────────────────────────────────┐
│ ┌─ Header ────────────────────────────────────────────────────────────┐ │
│ │ Planning Session · Epic Name          [Finalize] [Abandon] tokens  │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌─ ChatPanel (flex-1) ──────────┬─ SpecPanel (flex-1) ──────────────┐ │
│ │                               │                                    │ │
│ │  🤖 AssistantMessage          │  ## Overview                       │ │
│ │  (question, priority=1)       │  [from proposal msg #3]            │ │
│ │                               │                                    │ │
│ │  👤 UserMessage               │  ## Authentication Flow            │ │
│ │                               │  [from proposal msg #5]            │ │
│ │  🤖 AssistantMessage          │                                    │ │
│ │  (proposal, section=auth)     │  ## Data Model                     │ │
│ │                               │  [pending — needs Q answered]      │ │
│ │  ...                          │                                    │ │
│ │                               │                                    │ │
│ │ ┌─ InputBar ────────────────┐ │                                    │ │
│ │ │ [Type your answer...]  [➤]│ │                                    │ │
│ │ └───────────────────────────┘ │                                    │ │
│ └───────────────────────────────┴────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────────┘
```

**Component structure:**

```typescript
// Main panel
function PlanningSessionPanel({ epic, workspaceId }: Props)
  // Manages session state, message list, sending

// Chat sub-components
function ChatPanel({ messages, onSend, sending }: Props)
function ChatMessage({ message }: Props)
  // Renders differently based on message_type:
  //   question: highlighted with priority badge
  //   proposal: with "Confirm" / "Revise" inline actions
  //   context: collapsible with source badge
  //   summary: highlighted with decision list

// Spec sub-components
function SpecPanel({ sections, messages }: Props)
  // Assembles spec sections from proposal messages
  // Shows proposed/confirmed/pending status per section
  // Scrolls to section when user clicks a proposal message

// Input
function SessionInput({ onSend, disabled }: Props)
  // Textarea with send button
  // Disabled when waiting for agent response
```

**Message rendering by type:**

| message_type | Chat Panel rendering | Spec Panel effect |
|---|---|---|
| `question` | Highlighted card with priority badge, category tag | None |
| `answer` | User bubble (standard) | None |
| `proposal` | Agent bubble with embedded spec preview + "Looks good" / "Change this" actions | Section added/updated in spec panel |
| `confirmation` | User bubble (standard) | Section status → confirmed |
| `context` | Collapsible card with source icon (support ticket, repo file) | None |
| `summary` | Highlighted decision summary card | None |
| `final_spec` | "Spec finalized" system message | Full spec rendered |
| `message` | Standard agent/user bubble | None |

### Integration with `EpicOrchestrationPanel`

The existing `EpicOrchestrationPanel` renders steps based on `computeCurrentStep()`. Modify `planningStepUtils.ts`:

```typescript
// In computeCurrentStep():
// Add check for active planning session
if (epic.planning_state === 'in_session' && epic.active_planning_session_id) {
  return 'draft';  // Show as "draft" step, but render PlanningSessionPanel instead of DraftSpecStep
}
```

In `EpicOrchestrationPanel.tsx`, when `planning_state === 'in_session'`:

```typescript
// Instead of rendering DraftSpecStep, render:
<PlanningSessionPanel epic={epic} workspaceId={workspaceId} />
```

The "Start Planning" UI in the setup step offers two options:

```
┌─────────────────────────────────────────────────────────┐
│ How would you like to plan this epic?                   │
│                                                         │
│ [🗣️ Interactive Session]     [⚡ Auto-Draft]            │
│  Collaborate with the         Agent drafts the full     │
│  planner in real-time         spec autonomously         │
└─────────────────────────────────────────────────────────┘
```

- **Interactive Session** → calls `POST /pm/epics/{id}/planning-session`
- **Auto-Draft** → calls existing `POST /pm/epics/{id}/draft-spec`

### WebSocket integration

In `PlanningSessionPanel`, listen for new messages:

```typescript
useEffect(() => {
  const handler = (e: CustomEvent<WebSocketEvent>) => {
    if (
      e.detail.entity === 'planning_session_message' &&
      e.detail.parent_id === sessionId
    ) {
      // Invalidate messages query to fetch new message
      queryClient.invalidateQueries({
        queryKey: queryKeys.planningSessions.messages(sessionId)
      });
    }
    if (
      e.detail.entity === 'planning_session' &&
      e.detail.entity_id === sessionId
    ) {
      // Invalidate session query (status change)
      queryClient.invalidateQueries({
        queryKey: queryKeys.planningSessions.byId(sessionId)
      });
    }
  };
  window.addEventListener('planning_session_message-created', handler);
  window.addEventListener('planning_session-updated', handler);
  return () => {
    window.removeEventListener('planning_session_message-created', handler);
    window.removeEventListener('planning_session-updated', handler);
  };
}, [sessionId]);
```

### Optimistic message sending

When the user sends a message:

1. Immediately append the user message to the local message list (optimistic)
2. Show a typing indicator for the agent
3. Call `POST /pm/planning-sessions/{id}/messages`
4. On response: replace optimistic message with server response, append agent message, hide typing indicator
5. On error: show error toast, keep user message but mark as failed with retry option

---

## Migration

### SQL migration: `server/migrations/044_planning_sessions.sql`

```sql
-- Planning sessions for interactive epic planning
CREATE TABLE IF NOT EXISTS planning_sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspaces(id),
    epic_id             UUID NOT NULL REFERENCES pm_epics(id),
    agent_id            UUID NOT NULL REFERENCES pm_agents(id),
    status              TEXT NOT NULL DEFAULT 'active',
    planning_methodology TEXT NOT NULL DEFAULT 'structured_v1',
    spec_document_id    UUID,
    spec_sections       JSONB NOT NULL DEFAULT '[]',
    context_snapshot    JSONB NOT NULL DEFAULT '{}',
    token_usage         JSONB NOT NULL DEFAULT '{"input": 0, "output": 0}',
    started_by          UUID,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_planning_sessions_epic ON planning_sessions(epic_id);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_workspace ON planning_sessions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_status ON planning_sessions(workspace_id, status);

CREATE TABLE IF NOT EXISTS planning_session_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES planning_sessions(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    message_type    TEXT NOT NULL DEFAULT 'message',
    section_metadata JSONB,
    token_usage     JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_planning_session_messages_session ON planning_session_messages(session_id, created_at);

-- Add active session reference to epics
ALTER TABLE pm_epics ADD COLUMN IF NOT EXISTS active_planning_session_id UUID;
CREATE INDEX IF NOT EXISTS idx_pm_epics_active_session ON pm_epics(active_planning_session_id) WHERE active_planning_session_id IS NOT NULL;
```

### GORM AutoMigrate

Add `PlanningSession` and `PlanningSessionMessage` to the AutoMigrate call in `cmd/api/main.go`.

---

## Implementation Order

### Phase 1: Backend foundation

1. Create migration `044_planning_sessions.sql`
2. Create model `server/internal/model/planning_session.go`
3. Add `ActivePlanningSessionID` field + `EpicPlanningStateInSession` constant to epic model
4. Create repository `server/internal/repository/planning_session.go`
5. Extract Claude client from `worker/claude.go` to `internal/llm/claude.go` (or keep importing from worker)
6. Create service `server/internal/service/planning_session.go` with `StartSession`, `SendMessage`, `FinalizeSession`, `AbandonSession`, `ResumeSession`, `GetSession`, `GetSessionMessages`
7. Create planning session prompt builder (interactive variant of the planning prompt pack)
8. Create handler `server/internal/handler/planning_session.go`
9. Add routes to `server/internal/router/router.go`
10. Wire DI in `server/cmd/api/main.go`
11. Add WebSocket event types for planning sessions

### Phase 2: Frontend

12. Add TypeScript types to `frontend/src/lib/pmTypes.ts`
13. Create service `frontend/src/lib/services/planningSessionService.ts`
14. Create query hooks `frontend/src/hooks/queries/usePlanningSession.ts`
15. Add query keys to `frontend/src/lib/queryKeys.ts`
16. Build `PlanningSessionPanel` component (chat + spec split view)
17. Integrate into `EpicOrchestrationPanel` — add "Interactive Session" / "Auto-Draft" choice in setup step
18. Update `planningStepUtils.ts` to handle `in_session` state
19. Add WebSocket listeners for planning session events

### Phase 3: Polish and lifecycle

20. Add Temporal workflow for session timeout (pause after inactivity, abandon after extended pause)
21. Add audit trail: create AgentRun record on session finalize for compatibility with existing run history UI
22. Add session token usage display in the UI header
23. Handle edge cases: browser disconnect mid-send, concurrent session prevention, agent error recovery

---

## Token Budget Estimation

Rough per-session estimates (assuming structured_v1 methodology):

| Component | Tokens |
|---|---|
| System prompt | ~2,000 |
| Context snapshot (initial) | ~3,000-8,000 |
| Per turn (avg, including history prefix growth) | ~1,500 input + ~800 output |
| Typical session (15-20 turns) | ~35,000-50,000 total |
| Finalization call | ~8,000 input + ~4,000 output |
| **Total per session** | **~45,000-65,000 tokens** |

Compare to batch redraft: ~15,000-20,000 per draft. Two redrafts = 30,000-40,000. So interactive sessions cost ~50% more in raw tokens but produce dramatically better output because the agent understood the requirements correctly the first time.

For long sessions (30+ turns), the growing conversation prefix becomes expensive. Mitigation: after 25 turns, the service can summarize earlier turns into a compressed context block, preserving key decisions while reducing token count. This is a v2 optimization.

---

## What Changes vs. What Stays the Same

### Changes

| Component | Change |
|---|---|
| Epic model | New `active_planning_session_id` field, new `in_session` planning state |
| Planning flow | New interactive path alongside existing batch path |
| Epic planning UI | New `PlanningSessionPanel` component, choice between interactive/auto-draft |
| Backend | New service, handler, repository, migration for planning sessions |
| WebSocket | New event types for session messages |

### Stays the same

| Component | Why |
|---|---|
| Spec approval flow | Sessions write to the same Docs module. Approval works identically |
| Clarification model | Sessions extract clarification items on finalize. Same approval gate |
| Story planning (`plan_stories`) | Unchanged. Reads the approved spec version regardless of how it was created |
| Story confirmation and creation | Unchanged |
| Execution kickoff | Unchanged |
| Automation rules engine | Unchanged |
| Engineer/reviewer agent runs | Unchanged. Autonomous execution via Temporal + executor |
| Pipeline builder | Unchanged |
| Agent model | Unchanged. Product planner agents work in both modes |

---

## Open Questions

1. **Should the agent have read-only tools during the session?** v1 is conversation-only (context snapshot at start). But allowing the agent to search the codebase mid-conversation ("let me check how auth is currently implemented...") would be valuable. This adds complexity (tool execution within the session loop) but is a natural v2 extension.

2. **Multi-user sessions?** v1 is single-user. But planning often involves multiple stakeholders. A future version could allow multiple users to join a session and the agent mediates between them. The data model supports this (messages have no `user_id` — the `role: user` messages represent whoever is currently chatting).

3. **Session branching?** If the human wants to explore two different directions ("what if we do OAuth? what if we do magic links?"), they currently have to abandon and restart. A branching model (save checkpoint, explore, restore) is powerful but complex. Defer to v2.

4. **Streaming responses?** v1 returns the full agent response after the Claude call completes. For long responses (spec section proposals), this means the user waits. Server-Sent Events or streaming WebSocket messages would improve perceived responsiveness. This requires the Claude streaming API and a chunked response pipeline. Worth doing in v1 if the latency is noticeable (likely 5-15 seconds per turn).
