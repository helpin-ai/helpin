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
│ Chat Panel  │ Spec     │◄── WebSocket (stream) ──► PlanningSessionHandler
│ (streaming) │ Draft    │                                │
│ Q&A back    │ (live    │    REST (send msg) ────────► PlanningSessionService
│ and forth   │  update) │                                │
│             │          │                          Claude Streaming API
│ Tool use    │          │                                │
│ indicators  │          │                          Tool Execution (read-only)
│             │          │                            read_file, search_files,
│ [input]     │ [Approve]│                            ripgrep, list_directory,
└─────────────┴──────────┘                            list_symbols, web_search
                                                          │
                                                     Message persistence
                                                          │
                                                     Temporal (lifecycle)
                                                       - workspace clone/cleanup
                                                       - timeout/abandon
```

### Key architectural decisions

**Streaming via WebSocket**: The `SendMessage` endpoint returns immediately with a `202 Accepted` + the user message ID. The agent's response streams token-by-token through a dedicated WebSocket channel. This means the frontend sees tokens appearing in real time (like ChatGPT), not waiting 5-15 seconds for the full response.

**Read-only tools from day one**: The planning agent can read files, search code, list directories, and search the web during the conversation. This lets it say "let me check how auth is currently implemented..." and actually look. Tools are a read-only subset of the existing `ToolRegistry` — no `write_file`, `run_command`, `commit_and_push`, or `open_pr`. Tool executions are visible in the chat as collapsible "tool use" blocks.

**Tool loop within streaming**: When Claude returns a `tool_use` block during streaming, the service pauses streaming, executes the tool, sends a `tool_executing` WebSocket event (so the frontend shows a spinner), then resumes the Claude call with the tool result. This may produce multiple streaming segments per turn — the frontend handles this transparently.

### Interaction flow

1. Human clicks "Start Planning Session" on the epic
2. API creates a `PlanningSession` record, loads epic context (linked docs, support tickets, repo summary, existing spec if redrafting)
3. API makes the first Claude streaming call with the full context, read-only tools, and the planning prompt pack
4. Response tokens stream through WebSocket to the frontend in real time. If Claude invokes tools (e.g., `read_file` to check existing code), tool execution happens server-side and a `tool_executing` event is sent to the frontend, followed by resuming the Claude stream with tool results
5. The complete agent message (including any tool use blocks) is saved as a `PlanningSessionMessage`
6. Human types an answer in the chat panel
7. API saves the human message, loads full message history (including tool use/result blocks), makes the next Claude streaming call with tools
8. When the agent proposes a spec section, it's tagged as `message_type: proposal` with structured metadata. The frontend renders it in the spec panel
9. Loop continues. The agent can search the codebase at any point to ground its proposals in existing code
10. The spec is built up progressively in `session.spec_draft` via `<spec_draft>` tags during conversation. Users can ask for changes in the chat and the agent updates the draft
11. Human clicks "Finalize" — API synchronously creates a Docs document (if the epic doesn't have one yet), copies `session.spec_draft` to it, and completes the session. No extra agent turn
12. Session status transitions directly to `completed`. Epic `planning_state` transitions to `awaiting_spec_approval`. No clarification step
13. Existing approval flow takes over from here (unchanged)

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
        -- finalizing: (legacy, no longer used — finalize is synchronous)
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
        -- tool_use: agent invoking a read-only tool (rendered as collapsible block in chat)
        -- message: general conversation
    section_metadata JSONB,
        -- For proposal messages: { "section_key": "auth_flow", "section_title": "Authentication Flow", "spec_markdown": "..." }
        -- For question messages: { "priority": 1, "gates": ["auth_flow", "data_model"], "category": "scope" }
        -- For context messages: { "source": "support_tickets", "ticket_ids": ["..."] }
    tool_invocations JSONB,
        -- For assistant messages that used tools during the turn:
        -- [{ "tool_name": "read_file", "input": {"path": "server/internal/auth/..."}, "output_summary": "148 lines, JWT middleware", "duration_ms": 120 }]
        -- Stored for audit and for reconstructing the Claude message history on resume
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
  │     └─► awaiting_spec_approval (session completed, spec + doc created)
  │           (no clarification step — conversation already resolved ambiguities)
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
    ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    SessionID        string          `json:"session_id" gorm:"type:uuid;not null;index"`
    Role             string          `json:"role" gorm:"not null"`
    Content          string          `json:"content" gorm:"not null"`
    MessageType      string          `json:"message_type" gorm:"not null;default:'message'"`
    SectionMetadata  json.RawMessage `json:"section_metadata,omitempty" gorm:"type:jsonb"`
    ToolInvocations  json.RawMessage `json:"tool_invocations,omitempty" gorm:"type:jsonb"`
    // Raw Claude content blocks for faithful conversation reconstruction on resume.
    // Stores the full []ContentBlock (text + tool_use + tool_result) so the next
    // Claude call gets exact history without lossy text-only reconstruction.
    ContentBlocks    json.RawMessage `json:"content_blocks,omitempty" gorm:"type:jsonb"`
    TokenUsage       json.RawMessage `json:"token_usage,omitempty" gorm:"type:jsonb"`
    CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

// ToolInvocation records a single tool use within a planning session turn.
type ToolInvocation struct {
    ToolName      string `json:"tool_name"`
    Input         json.RawMessage `json:"input"`
    OutputSummary string `json:"output_summary"` // Truncated for display
    DurationMs    int64  `json:"duration_ms"`
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
    PlanningMessageTypeToolUse      = "tool_use"
    PlanningMessageTypeMessage      = "message"
)

// Read-only tools available during planning sessions.
// These are a subset of the full ToolRegistry — no write, command, or git mutation tools.
var PlanningSessionAllowedTools = map[string]bool{
    "read_file":       true,
    "read_file_range": true,
    "list_directory":  true,
    "search_files":    true,
    "ripgrep":         true,
    "grep":            true,
    "list_symbols":    true,
    "web_search":      true, // if enabled in workspace settings
}

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
    toolRegistry      *worker.ToolRegistry        // Read-only tools for codebase exploration
    wsHub             *websocket.Hub              // Direct hub access for streaming
    wsPublisher       websocket.Publisher          // Standard event publishing
    settingsRepo      *repository.SettingsRepository
    gitIntegrationSvc *GitIntegrationService      // For resolving repo workspace paths
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
9. Resolve planning repository workspace path (clone if needed for tool access)
10. Build the initial system prompt (reuse planning prompt pack with modifications for interactive mode + tool instructions)
11. Build the initial user message from context snapshot
12. Return session to HTTP caller immediately (the first agent message streams via WebSocket)
13. Launch `runAgentTurn` goroutine — streams the first response with tool access
14. Publish WebSocket event: `planning_session-created`

#### SendMessage (streaming + tool loop)

```go
func (s *PlanningSessionService) SendMessage(ctx context.Context, workspaceID, sessionID, actorID string, req model.SendPlanningMessageRequest) (*model.PlanningSessionMessage, error)
```

The `SendMessage` handler returns the persisted user message immediately (HTTP 200). The agent response is produced asynchronously via streaming WebSocket. This is the core interaction loop:

1. Load session, validate status is `active`
2. Save user message to `planning_session_messages` (with `content_blocks` = nil for user messages)
3. Return the user message to the HTTP caller immediately
4. **Launch goroutine** for the agent turn:

**Agent turn goroutine (streaming + tool loop):**

```go
func (s *PlanningSessionService) runAgentTurn(ctx context.Context, session *model.PlanningSession, workspaceID string) {
    // 1. Load full message history from DB
    messages := s.sessionRepo.ListMessages(ctx, session.ID)

    // 2. Convert to Claude API message format
    //    - User messages: role="user", content=string
    //    - Assistant messages with tools: role="assistant", content=[]ContentBlock (from content_blocks column)
    //    - Tool results: injected as role="user" messages with tool_result content blocks
    claudeMessages := s.buildClaudeMessages(messages)

    // 3. Resolve tools: read-only subset from ToolRegistry
    tools := s.toolRegistry.DefinitionsFor(model.PlanningSessionAllowedTools)
    // Remove web_search if not enabled in workspace settings
    if !s.isWebSearchEnabled(ctx, workspaceID) {
        tools = filterOutTool(tools, "web_search")
    }

    // 4. Prepare execution context for tool calls
    execCtx := s.buildToolExecutionContext(ctx, session)

    // 5. TOOL LOOP: may iterate multiple times if Claude invokes tools
    var allContentBlocks []worker.ContentBlock
    var toolInvocations []model.ToolInvocation
    var totalUsage worker.Usage
    maxToolRounds := 10

    for round := 0; round < maxToolRounds; round++ {
        // 6. Stream Claude API call
        streamCh := s.claudeClient.CreateMessageStream(ctx, worker.CreateMessageRequest{
            System:   s.buildSystemPrompt(session),
            Messages: claudeMessages,
            Tools:    tools,
        })

        // 7. Forward text delta events to frontend via WebSocket
        var turnBlocks []worker.ContentBlock
        var textAccum strings.Builder

        for event := range streamCh {
            switch event.Type {
            case "content_block_delta":
                if event.Delta.Type == "text_delta" {
                    textAccum.WriteString(event.Delta.Text)
                    // Stream token to frontend
                    s.wsHub.SendToSession(session.ID, websocket.StreamEvent{
                        Type:      "token",
                        SessionID: session.ID,
                        Text:      event.Delta.Text,
                    })
                }
            case "content_block_start":
                if event.ContentBlock.Type == "tool_use" {
                    // Notify frontend: tool execution starting
                    s.wsHub.SendToSession(session.ID, websocket.StreamEvent{
                        Type:      "tool_start",
                        SessionID: session.ID,
                        ToolName:  event.ContentBlock.Name,
                    })
                }
            case "content_block_stop":
                turnBlocks = append(turnBlocks, event.ContentBlock)
            case "message_delta":
                totalUsage.InputTokens += event.Usage.InputTokens
                totalUsage.OutputTokens += event.Usage.OutputTokens
            case "error":
                // Send error to frontend, abort
                s.wsHub.SendToSession(session.ID, websocket.StreamEvent{
                    Type:      "error",
                    SessionID: session.ID,
                    Error:     event.Error.Message,
                })
                return
            }
        }

        allContentBlocks = append(allContentBlocks, turnBlocks...)

        // 8. Check if any tool_use blocks need execution
        toolUseBlocks := filterToolUseBlocks(turnBlocks)
        if len(toolUseBlocks) == 0 {
            // No tools — agent turn is complete
            break
        }

        // 9. Execute tools and collect results
        var toolResultBlocks []worker.ContentBlock
        for _, toolBlock := range toolUseBlocks {
            start := time.Now()
            result, err := s.toolRegistry.ExecuteAllowed(execCtx, toolBlock.Name, toolBlock.Input)
            duration := time.Since(start)

            if err != nil {
                result = fmt.Sprintf("Error: %s", err.Error())
            }

            // Truncate large results for display
            displayResult := truncate(result, 500)

            toolInvocations = append(toolInvocations, model.ToolInvocation{
                ToolName:      toolBlock.Name,
                Input:         toolBlock.Input,
                OutputSummary: displayResult,
                DurationMs:    duration.Milliseconds(),
            })

            toolResultBlocks = append(toolResultBlocks, worker.ContentBlock{
                Type:      "tool_result",
                ToolUseID: toolBlock.ID,
                Content:   result,
            })

            // Notify frontend: tool completed
            s.wsHub.SendToSession(session.ID, websocket.StreamEvent{
                Type:          "tool_result",
                SessionID:     session.ID,
                ToolName:      toolBlock.Name,
                OutputSummary: displayResult,
                DurationMs:    duration.Milliseconds(),
            })
        }

        // 10. Append assistant message + tool results to conversation for next round
        claudeMessages = append(claudeMessages,
            worker.Message{Role: "assistant", Content: turnBlocks},
            worker.Message{Role: "user", Content: toolResultBlocks},
        )
        allContentBlocks = append(allContentBlocks, toolResultBlocks...)
    }

    // 11. Signal stream complete to frontend
    s.wsHub.SendToSession(session.ID, websocket.StreamEvent{
        Type:      "turn_complete",
        SessionID: session.ID,
    })

    // 12. Extract text content for the persisted message
    fullText := extractTextFromBlocks(allContentBlocks)

    // 13. Detect message type from content
    messageType, sectionMeta := s.classifyMessage(fullText)

    // 14. Persist the assistant message with full content blocks
    msg := &model.PlanningSessionMessage{
        SessionID:       session.ID,
        Role:            "assistant",
        Content:         fullText,
        MessageType:     messageType,
        SectionMetadata: sectionMeta,
        ToolInvocations: marshalJSON(toolInvocations),
        ContentBlocks:   marshalJSON(allContentBlocks),
        TokenUsage:      marshalJSON(totalUsage),
    }
    s.sessionRepo.CreateMessage(ctx, msg)

    // 15. Update session
    s.sessionRepo.UpdateLastActive(ctx, session.ID)
    s.updateSessionTokenUsage(ctx, session, totalUsage)
    if sectionMeta != nil {
        s.updateSessionSpecSections(ctx, session, sectionMeta, msg.ID)
    }

    // 16. Publish standard WebSocket event for query invalidation
    s.wsPublisher.Publish(websocket.Event{
        Action: "created", Entity: "planning_session_message",
        EntityID: msg.ID, ParentType: "planning_session", ParentID: session.ID,
        WorkspaceID: session.WorkspaceID,
    })
}
```

### Streaming WebSocket protocol

The planning session uses a **dedicated WebSocket message channel** alongside the existing event system. Stream events are sent to clients subscribed to a specific session ID.

**WebSocket stream event types:**

| Type | Fields | Purpose |
|---|---|---|
| `token` | `session_id`, `text` | Incremental text token from Claude |
| `tool_start` | `session_id`, `tool_name` | Tool execution beginning (show spinner) |
| `tool_result` | `session_id`, `tool_name`, `output_summary`, `duration_ms` | Tool completed (show result) |
| `turn_complete` | `session_id` | Agent turn finished (enable input) |
| `error` | `session_id`, `error` | Error occurred during turn |

The frontend accumulates `token` events into a growing message bubble. On `turn_complete`, it replaces the accumulated content with the final persisted message (fetched via query invalidation from the standard `planning_session_message-created` event).

### Claude Streaming API integration

Add to `worker/claude.go`:

```go
// StreamEvent represents a single SSE event from the Claude streaming API.
type StreamEvent struct {
    Type         string        `json:"type"`
    ContentBlock *ContentBlock `json:"content_block,omitempty"`
    Delta        *StreamDelta  `json:"delta,omitempty"`
    Usage        *Usage        `json:"usage,omitempty"`
    Error        *StreamError  `json:"error,omitempty"`
    Index        int           `json:"index,omitempty"`
}

type StreamDelta struct {
    Type  string `json:"type"`
    Text  string `json:"text,omitempty"`
}

type StreamError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
}

// CreateMessageStream sends a streaming request to the Claude Messages API.
// Returns a channel that yields StreamEvents. The channel closes when the stream ends.
func (c *ClaudeClient) CreateMessageStream(ctx context.Context, req CreateMessageRequest) <-chan StreamEvent {
    ch := make(chan StreamEvent, 64)

    go func() {
        defer close(ch)

        req.Stream = true  // Add "stream": true to the request
        // ... same HTTP setup as CreateMessage ...
        // Parse SSE lines from response body
        // For each "data: {json}" line, unmarshal and send to channel
        // Handle "event: message_stop" as stream end
    }()

    return ch
}
```

The streaming request adds `"stream": true` to the existing `CreateMessageRequest`. The `anthropic-version` header stays the same. The response is an SSE stream that the client reads line by line.

### Read-only tool execution context

Planning session tools execute against the **epic's planning repository** clone. The session service resolves the repo path at session start (reusing `GitIntegrationService` to get the local clone path or creating a shallow clone if needed).

```go
func (s *PlanningSessionService) buildToolExecutionContext(ctx context.Context, session *model.PlanningSession) *worker.ExecutionContext {
    // Resolve the planning repository's local workspace path
    workDir := s.resolveRepoWorkDir(ctx, session)

    return &worker.ExecutionContext{
        WorkDir:        workDir,
        AllowedTools:   model.PlanningSessionAllowedTools,
        RuntimeProfile: worker.RuntimeProfile{Name: "planning_session"},
        // No git credentials needed — read-only
        // No story/ticket context — this is epic-level planning
    }
}
```

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

Finalization is **synchronous** — no extra agent turn, no Temporal signal. The spec was already built up progressively in `session.spec_draft` during conversation.

1. Load session, validate status is `active` or `paused`
2. Validate `session.spec_draft` is non-empty (reject if no draft exists)
3. Ensure a Docs document exists for the epic via `ensureEpicSpecDocument` (creates "Product Specs" space + document + DocsLink if the epic doesn't have a `SpecDocumentID` yet)
4. Write `session.spec_draft` to the doc content, create a `DocsVersion` with label "AI Draft (Interactive)"
5. Update session: `status = "completed"`, `completed_at = now()`, `spec_document_id = doc.ID`
6. Update epic: `planning_state = "awaiting_spec_approval"`, `active_planning_session_id = nil`
7. Signal Temporal workflow to **abandon** (cleanup workspace only — no agent turn)
8. Publish WebSocket events: `planning_session-updated`, `epic-updated`
9. Return updated session immediately

No clarification step — the interactive conversation already resolved ambiguities. No `AgentRun` record — the session itself is the audit trail.

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
7. **Tool usage guidance**: "You have read-only access to the codebase. Use tools proactively to ground your proposals in existing code. When discussing a feature that touches existing modules, read the relevant files first. Mention what you found — e.g., 'I checked server/internal/auth/middleware.go and the current JWT implementation uses...' This builds trust and ensures the spec is realistic. Available tools: read_file, read_file_range, list_directory, search_files, ripgrep, grep, list_symbols, web_search (if enabled)."
8. **Tool etiquette**: "Don't use tools excessively — read what's relevant, not the entire codebase. If you need to check something, explain why briefly before using the tool."

The initial user message is built from the context snapshot:

```go
func BuildPlanningSessionInitialMessage(contextSnapshot json.RawMessage) string
```

This renders the epic metadata, linked docs, support signals, and existing spec into a structured prompt that tells the agent: "Here's what I know. Begin by asking your most important questions."

### Claude API access

The existing `worker.ClaudeClient` (used by `Executor`) makes direct Claude API calls. The planning session service needs the same client with two additions: **streaming support** and the ability to pass **read-only tools**.

Two options:
- **Option A**: Extract the Claude API client from `worker/claude.go` into a shared package (e.g., `internal/llm/claude.go`) that both the executor and the planning session service can import.
- **Option B**: Have the planning session service import the worker package and use `ClaudeClient` directly.

**Recommendation: Option B for now.** The `worker` package already has `ClaudeClient`, `ToolRegistry`, `ToolDefinition`, `ContentBlock`, and `ExecutionContext` — everything the planning session service needs. Extracting to `internal/llm/` is a future cleanup.

The planning session service calls Claude with:
- `system`: from `BuildPlanningSessionSystemPrompt`
- `messages`: full conversation history from DB (including `content_blocks` for faithful tool use/result reconstruction)
- `max_tokens`: 8192 (shorter than autonomous runs since each turn is incremental)
- `tools`: read-only subset from `ToolRegistry.DefinitionsFor(PlanningSessionAllowedTools)`
- `stream`: true (SSE streaming for real-time token delivery)

Add `CreateMessageStream` method to `ClaudeClient` (see streaming protocol section above).

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

Two WebSocket channels are used:

**1. Standard entity events** (existing `websocket.Event` structure, for query invalidation):

| Entity | Action | When |
|---|---|---|
| `planning_session` | `created` | Session started |
| `planning_session` | `updated` | Status change (paused, finalizing, completed, abandoned) |
| `planning_session_message` | `created` | New message persisted (assistant or user) |

**2. Streaming events** (new `websocket.StreamEvent` structure, for real-time token delivery):

| Type | Fields | When |
|---|---|---|
| `token` | `session_id`, `text` | Each text token from Claude streaming API |
| `tool_start` | `session_id`, `tool_name` | Agent invokes a read-only tool |
| `tool_result` | `session_id`, `tool_name`, `output_summary`, `duration_ms` | Tool execution completed |
| `turn_complete` | `session_id` | Agent turn finished (all tool loops done) |
| `error` | `session_id`, `error` | Error during agent turn |

Streaming events are targeted to clients subscribed to a specific session. The hub needs a `SendToSession(sessionID, event)` method that filters to WebSocket clients who have subscribed via a `subscribe_session` message.

**Frontend WebSocket subscription flow:**
1. When `PlanningSessionPanel` mounts, it sends `{ type: "subscribe_session", session_id: "..." }` to the WebSocket
2. The hub registers the client for that session's stream events
3. On unmount, sends `{ type: "unsubscribe_session", session_id: "..." }`

### DI wiring: `server/cmd/api/main.go`

```go
planningSessionRepo := repository.NewPlanningSessionRepository(db)
// Reuse existing ClaudeClient and ToolRegistry from worker package
claudeClient := worker.NewClaudeClient(cfg.AnthropicAPIKey)
toolRegistry := worker.NewToolRegistry(webSearchClient) // same as executor's

planningSessionService := service.NewPlanningSessionService(
    planningSessionRepo,
    epicRepo,
    agentRepo,
    docsContentRepo,
    docsVersionRepo,
    docsSvc,
    claudeClient,
    toolRegistry,
    wsHub,          // direct hub access for streaming
    wsPublisher,    // standard event publishing
    settingsRepo,
    gitIntegrationSvc,
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

The existing planning flow creates `AgentRun` records that serve as audit trail. Interactive sessions don't create a separate `AgentRun` — the `PlanningSession` record itself (with its messages, token usage, and timestamps) serves as the audit trail. The `EpicOrchestrationPanel` shows the session status directly.

The downstream flow (approve spec → plan stories → confirm → kickoff) works unchanged because the session writes to the same Docs module and sets the same `awaiting_spec_approval` state.

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
  message_type: 'question' | 'answer' | 'proposal' | 'confirmation' | 'context' | 'summary' | 'final_spec' | 'tool_use' | 'message';
  section_metadata?: {
    section_key?: string;
    section_title?: string;
    spec_markdown?: string;
    priority?: number;
    gates?: string[];
    category?: string;
    source?: string;
  };
  tool_invocations?: ToolInvocation[];
  token_usage?: { input: number; output: number };
  created_at: string;
}

interface ToolInvocation {
  tool_name: string;
  input: Record<string, unknown>;
  output_summary: string;
  duration_ms: number;
}

// WebSocket stream events for real-time token delivery
interface PlanningStreamEvent {
  type: 'token' | 'tool_start' | 'tool_result' | 'turn_complete' | 'error';
  session_id: string;
  text?: string;          // for 'token'
  tool_name?: string;     // for 'tool_start', 'tool_result'
  output_summary?: string; // for 'tool_result'
  duration_ms?: number;   // for 'tool_result'
  error?: string;         // for 'error'
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
| `tool_use` | Collapsible tool invocation blocks (file path, search query, result preview) | None (but tool results inform subsequent proposals) |
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

The `PlanningSessionPanel` uses two WebSocket channels:

**1. Streaming channel** — for real-time token delivery during agent turns:

```typescript
// Custom hook: usePlanningStream
function usePlanningStream(sessionId: string | undefined) {
  const [streamingText, setStreamingText] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [activeToolCall, setActiveToolCall] = useState<{ name: string } | null>(null);
  const [toolResults, setToolResults] = useState<ToolInvocation[]>([]);

  useEffect(() => {
    if (!sessionId) return;

    // Subscribe to session stream via existing WebSocket connection
    const ws = getWebSocket(); // reuse existing WS connection
    ws.send(JSON.stringify({ type: 'subscribe_session', session_id: sessionId }));

    const handler = (event: MessageEvent) => {
      const data: PlanningStreamEvent = JSON.parse(event.data);
      if (data.session_id !== sessionId) return;

      switch (data.type) {
        case 'token':
          setIsStreaming(true);
          setStreamingText(prev => prev + data.text);
          break;
        case 'tool_start':
          setActiveToolCall({ name: data.tool_name! });
          break;
        case 'tool_result':
          setActiveToolCall(null);
          setToolResults(prev => [...prev, {
            tool_name: data.tool_name!,
            input: {},
            output_summary: data.output_summary!,
            duration_ms: data.duration_ms!,
          }]);
          break;
        case 'turn_complete':
          setIsStreaming(false);
          setStreamingText('');
          setToolResults([]);
          break;
        case 'error':
          setIsStreaming(false);
          toast.error(data.error);
          break;
      }
    };

    ws.addEventListener('message', handler);
    return () => {
      ws.send(JSON.stringify({ type: 'unsubscribe_session', session_id: sessionId }));
      ws.removeEventListener('message', handler);
    };
  }, [sessionId]);

  return { streamingText, isStreaming, activeToolCall, toolResults };
}
```

**2. Standard entity events** — for query invalidation when messages are persisted:

```typescript
useEffect(() => {
  const handler = (e: CustomEvent<WebSocketEvent>) => {
    if (
      e.detail.entity === 'planning_session_message' &&
      e.detail.parent_id === sessionId
    ) {
      queryClient.invalidateQueries({
        queryKey: queryKeys.planningSessions.messages(sessionId)
      });
    }
    if (
      e.detail.entity === 'planning_session' &&
      e.detail.entity_id === sessionId
    ) {
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

### Message sending and streaming display

When the user sends a message:

1. Immediately append the user message to the local message list (optimistic)
2. Call `POST /pm/planning-sessions/{id}/messages` (returns user message, agent response streams via WS)
3. While `isStreaming` is true, render a streaming message bubble:
   - Shows `streamingText` accumulating in real time
   - Shows `activeToolCall` as an inline spinner ("Reading server/internal/auth/middleware.go...")
   - Shows completed `toolResults` as collapsible blocks above the streaming text
4. On `turn_complete`: the streaming bubble disappears, replaced by the persisted message (fetched via query invalidation)
5. On error: show error toast, enable input for retry

**Streaming message bubble component:**

```tsx
function StreamingMessage({ text, activeToolCall, toolResults }: Props) {
  return (
    <div className="flex gap-2">
      <Bot className="h-5 w-5 text-violet-500 shrink-0 mt-0.5" />
      <div className="space-y-2 min-w-0 flex-1">
        {/* Completed tool calls */}
        {toolResults.map((tool, i) => (
          <div key={i} className="flex items-center gap-1.5 text-xs text-muted-foreground rounded bg-muted/50 px-2 py-1">
            <FileSearch className="h-3 w-3" />
            <span className="font-mono">{tool.tool_name}</span>
            <span className="truncate">{tool.output_summary}</span>
            <span className="shrink-0">{tool.duration_ms}ms</span>
          </div>
        ))}

        {/* Active tool call */}
        {activeToolCall && (
          <div className="flex items-center gap-1.5 text-xs text-violet-600 animate-pulse">
            <Loader2 className="h-3 w-3 animate-spin" />
            <span>Reading {activeToolCall.name}...</span>
          </div>
        )}

        {/* Streaming text */}
        {text && (
          <div className="prose prose-sm dark:prose-invert max-w-none">
            <ReactMarkdown>{text}</ReactMarkdown>
            <span className="inline-block w-1.5 h-4 bg-violet-500 animate-pulse ml-0.5" />
          </div>
        )}
      </div>
    </div>
  );
}
```

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
    tool_invocations JSONB,
    content_blocks  JSONB,
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
2. Create model `server/internal/model/planning_session.go` (with `ToolInvocation`, `PlanningSessionAllowedTools`, `ContentBlocks` field)
3. Add `ActivePlanningSessionID` field + `EpicPlanningStateInSession` constant to epic model
4. Create repository `server/internal/repository/planning_session.go`
5. Add `CreateMessageStream` method to `worker/claude.go` (SSE streaming support)
6. Add `SendToSession` / session subscription to `websocket/hub.go` (streaming channel)
7. Add `StreamEvent` type to `websocket/` package
8. Create service `server/internal/service/planning_session.go`:
   - `StartSession` (with initial agent turn via streaming + tools)
   - `SendMessage` (async agent turn via `runAgentTurn` goroutine)
   - `runAgentTurn` (streaming + tool loop core)
   - `FinalizeSession` (synchronous: validate spec_draft, ensureEpicSpecDocument, write to docs, complete session)
   - `AbandonSession`, `ResumeSession`
   - `GetSession`, `GetSessionMessages`, `GetActiveSessionByEpicID`
   - `buildClaudeMessages` (reconstruct from `content_blocks` column)
   - `buildToolExecutionContext` (read-only, against planning repo)
   - `classifyMessage` (detect spec sections, questions, etc.)
9. Create planning session prompt builder (interactive variant with tool instructions)
10. Create handler `server/internal/handler/planning_session.go`
11. Add routes to `server/internal/router/router.go`
12. Wire DI in `server/cmd/api/main.go`

### Phase 2: Frontend

13. Add TypeScript types to `frontend/src/lib/pmTypes.ts` (`PlanningSession`, `PlanningSessionMessage`, `ToolInvocation`, `PlanningStreamEvent`)
14. Create service `frontend/src/lib/services/planningSessionService.ts`
15. Create query hooks `frontend/src/hooks/queries/usePlanningSession.ts`
16. Add query keys to `frontend/src/lib/queryKeys.ts`
17. Create `usePlanningStream` hook (WebSocket streaming subscription)
18. Build `PlanningSessionPanel` component:
    - `ChatPanel` with streaming message bubble
    - `StreamingMessage` component (live tokens + tool call indicators)
    - `ChatMessage` component (persisted messages with tool invocation blocks)
    - `SpecPanel` (live spec assembly from proposals)
    - `SessionInput` (textarea, disabled during streaming)
19. Integrate into `EpicOrchestrationPanel` — add "Interactive Session" / "Auto-Draft" choice in setup step
20. Update `planningStepUtils.ts` to handle `in_session` state

### Phase 3: Polish and lifecycle

21. Add Temporal workflow for session timeout (pause after inactivity, abandon after extended pause)
22. Add session token usage display in the UI header
24. Handle edge cases: browser disconnect mid-stream, concurrent session prevention, agent error recovery, stream reconnection

---

## Token Budget Estimation

Rough per-session estimates (assuming structured_v1 methodology with tools):

| Component | Tokens |
|---|---|
| System prompt (with tool definitions) | ~3,500 |
| Context snapshot (initial) | ~3,000-8,000 |
| Tool definitions (8 read-only tools) | ~1,500 |
| Per turn (avg, including history prefix growth) | ~2,000 input + ~1,000 output |
| Tool use per turn (avg 1-2 tool calls) | ~500 input + ~2,000 output (file contents) |
| Typical session (15-20 turns, ~30% with tool use) | ~50,000-75,000 total |
| Finalization | 0 (synchronous copy, no agent call) |
| **Total per session** | **~50,000-75,000 tokens** |

Compare to batch redraft: ~15,000-20,000 per draft. Two redrafts = 30,000-40,000. Interactive sessions cost ~2x more in raw tokens but produce dramatically better output because:
- The agent understood the requirements correctly through dialogue
- The agent grounded proposals in actual codebase exploration
- No wasted full-spec regeneration cycles

For long sessions (30+ turns), the growing conversation prefix becomes expensive. Mitigation options:
- Summarize tool result blocks (replace file contents with summaries after they've been discussed)
- After 25 turns, compress earlier conversation into a decisions summary
- These are v2 optimizations — the 1M context window on Opus provides ample room for most planning sessions

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
| Clarification model | Not used by interactive sessions (conversation resolves ambiguities). Batch path still uses it |
| Story planning (`plan_stories`) | Unchanged. Reads the approved spec version regardless of how it was created |
| Story confirmation and creation | Unchanged |
| Execution kickoff | Unchanged |
| Automation rules engine | Unchanged |
| Engineer/reviewer agent runs | Unchanged. Autonomous execution via Temporal + executor |
| Pipeline builder | Unchanged |
| Agent model | Unchanged. Product planner agents work in both modes |

---

## Open Questions

1. ~~**Should the agent have read-only tools during the session?**~~ **RESOLVED: Yes, from day one.** Read-only tools (`read_file`, `search_files`, `ripgrep`, `list_directory`, `list_symbols`, `grep`, `web_search`) are available during planning sessions. The agent can explore the codebase mid-conversation to ground proposals in existing code.

2. **Multi-user sessions?** v1 is single-user. But planning often involves multiple stakeholders. A future version could allow multiple users to join a session and the agent mediates between them. The data model supports this (messages have no `user_id` — the `role: user` messages represent whoever is currently chatting).

3. **Session branching?** If the human wants to explore two different directions ("what if we do OAuth? what if we do magic links?"), they currently have to abandon and restart. A branching model (save checkpoint, explore, restore) is powerful but complex. Defer to v2.

4. ~~**Streaming responses?**~~ **RESOLVED: Yes, from day one.** Agent responses stream token-by-token via WebSocket using Claude's streaming API. Tool invocations are visible in real time as collapsible blocks. The `turn_complete` event signals the frontend to finalize the message.
