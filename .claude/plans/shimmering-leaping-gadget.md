# Fix: Planning Session Tool Execution — Use Temporal Sandbox

## Context

Planning session tools (`read_file`, `list_directory`, `ripgrep`, etc.) fail with "no execution context available" because `runAgentTurn` is called with `execCtx = nil`. The current implementation runs Claude API calls directly in a goroutine from the API server process — no repo clone, no sandbox, no tenant isolation.

This is a security and correctness problem:
- **No cloned repo** — file tools have nothing to read
- **No tenant isolation** — tools would execute on the API server's filesystem
- **No WorkDir** — all `safePath()` calls would fail or access wrong paths

The regular agent flow solves this correctly via Temporal: `PrepareWorkspace()` clones the repo into a temp dir, creates an `ExecutionContext` with `WorkDir`, and cleans up after.

## Design Decision

The planning session is **interactive** — the human sends messages and the agent responds in a streaming loop that can span minutes or hours. This doesn't fit the standard single-shot `AgentRunWorkflow` pattern.

**Approach: Long-running Temporal workflow with signal-driven message processing.**

The planning session becomes a Temporal workflow that:
1. Clones the repo once on start (via `PrepareWorkspace`)
2. Waits for signals (user messages, finalize, abandon)
3. On each signal, runs the Claude streaming tool loop with the sandboxed `ExecutionContext`
4. Streams tokens back via WebSocket hub (same as now)
5. Cleans up the repo clone when the session ends

The API handler sends Temporal signals instead of spawning goroutines directly.

## Files to Modify

### `server/internal/temporalapp/workflow.go` — Add `PlanningSessionWorkflow`

New workflow:

```go
type PlanningSessionWorkflowInput struct {
    SessionID string
}

type PlanningSessionSignal struct {
    Type      string // "message", "finalize", "abandon"
    Content   string
    ActorID   string
}

func PlanningSessionWorkflow(ctx workflow.Context, input PlanningSessionWorkflowInput) error {
    // Single long-running ExecuteActivity that:
    // 1. Clones repo
    // 2. Loops waiting for signals via a channel
    // 3. On "message": runs agent turn with ExecutionContext
    // 4. On "finalize": runs finalization turn, writes spec, exits
    // 5. On "abandon": exits
    // 6. Cleanup: defer os.RemoveAll(workDir)

    // Use heartbeat timeout of 5 minutes — the activity heartbeats
    // during Claude streaming and between message waits
}
```

**Key insight:** The signal channel is received in the **workflow** and forwarded to the activity via a shared Go channel or by re-scheduling activities. However, Temporal activities can't receive signals directly.

**Better pattern:** Use a **signal-driven workflow loop** where each message signal triggers a new `ExecuteSessionTurnActivity`. The repo clone persists across turns via a session-scoped temp dir managed by the first activity.

**Actually simplest correct pattern:** A single long-running activity with a **signal channel** that the workflow feeds into:

```
Workflow:
  1. Start ExecuteSessionActivity (long timeout, e.g. 4 hours)
  2. In parallel, listen for signals and forward to activity via side-channel

Problem: Activities can't receive signals.
```

**Revised approach — Multiple activity invocations sharing a workspace:**

```
Workflow:
  1. PrepareSessionActivity → clones repo, returns workDir path
  2. Loop:
     - Wait for signal (message/finalize/abandon)
     - On "message": ExecuteSessionTurnActivity(sessionID, workDir)
     - On "finalize": FinalizeSessionActivity(sessionID, workDir)
     - On "abandon": break
  3. CleanupSessionActivity(workDir) — deletes cloned repo
```

But this has a problem: `workDir` is a local filesystem path on the worker machine. Multiple activities may run on different workers. We need all activities to run on the **same worker**.

**Solution: Session-based task queue routing.** Create a unique task queue per session (e.g., `planning-session-{sessionID}`) and have the worker pick it up. But this is complex.

**Simpler solution: Use Temporal's `SessionOptions`** — Temporal sessions pin activities to a single worker, keeping the filesystem state persistent across activity invocations.

### Revised Architecture with Temporal Sessions

```
PlanningSessionWorkflow:
  1. Create Temporal session (pins to a worker)
  2. PrepareSessionActivity → clone repo, return workDir
  3. Signal loop:
     - Receive signal from channel
     - "message" → RunSessionTurnActivity(sessionID, workDir, messageContent)
     - "finalize" → FinalizeSessionActivity(sessionID, workDir)
     - "abandon" → break
  4. CleanupSessionActivity(workDir)
  5. Complete session (release worker)
```

### `server/internal/temporalapp/activities.go` — Add planning session activities

**`PrepareSessionActivity(sessionID string) (string, error)`**
- Load session from DB
- Load epic + git integration
- Call `PrepareWorkspace()` to clone repo
- Return `workDir`

**`RunSessionTurnActivity(sessionID, workDir string) error`**
- Load session, messages, agent from DB
- Build `ExecutionContext` with `WorkDir = workDir`
- Build Claude messages + system prompt
- Run `executeStreamingToolLoop` with the real `execCtx`
- Stream tokens via `wsHub.SendToSession()`
- Persist assistant message
- Heartbeat during streaming

**`FinalizeSessionActivity(sessionID, workDir, actorID string) error`**
- Same as RunSessionTurnActivity but with finalization instruction
- Write spec to doc on completion
- Update session + epic status

**`CleanupSessionActivity(workDir string) error`**
- `os.RemoveAll(workDir)`

### `server/internal/temporalapp/engine.go` — Add `StartPlanningSession`, `SignalPlanningSession`

```go
func (e *RunEngine) StartPlanningSession(ctx context.Context, sessionID string) error
func (e *RunEngine) SignalPlanningSession(ctx context.Context, sessionID string, signal PlanningSessionSignal) error
```

### `server/internal/service/planning_session.go` — Replace goroutines with Temporal signals

**`StartSession`**: Instead of `go s.runAgentTurn(...)`, call `runEngine.StartPlanningSession()`.

**`SendMessage`**: Instead of `go s.runAgentTurn(...)`, call `runEngine.SignalPlanningSession(sessionID, {Type: "message"})`.

**`FinalizeSession`**: Call `runEngine.SignalPlanningSession(sessionID, {Type: "finalize", ActorID: actorID})`.

**`AbandonSession`**: Call `runEngine.SignalPlanningSession(sessionID, {Type: "abandon"})`.

**Move tool execution code** (`executeStreamingToolLoop`, `executeTool`, `buildClaudeMessages`, `buildSystemPrompt`, etc.) from the service into the activity, or keep in the service but call it from the activity with a real `ExecutionContext`.

### `server/internal/service/planning_session.go` — Restructure

The service needs `runEngine *temporalapp.RunEngine` instead of directly holding `claudeClient` and `toolRegistry`. The streaming/tool execution logic moves to the activity.

However, the streaming WebSocket push (`wsHub.SendToSession`) needs to happen from the Temporal worker, which means the worker needs access to the `wsHub`. Since the worker is a separate process from the API server, this requires either:

**Option A:** Worker sends WS events via a shared transport (Redis pub/sub, NATS)
**Option B:** Worker calls back to the API server via HTTP/gRPC to push WS events
**Option C:** Keep the Claude streaming in the API server process (same machine), use Temporal only for workspace setup

**Option C is pragmatic for now:** The Temporal workflow manages the workspace lifecycle (clone, cleanup), but delegates the actual Claude streaming back to the API server via a callback or by having the API server be the Temporal worker for planning sessions.

### Pragmatic approach: API server IS the Temporal worker for planning queues

Looking at `server/cmd/temporal-worker/main.go`, the Temporal worker is a separate binary. But the API server can ALSO register as a Temporal worker for a specific task queue (e.g., `planning-interactive`). This way:

- The API server has access to `wsHub` for streaming
- Temporal manages workspace lifecycle and activity retries
- Activities run in-process with full access to WebSocket hub

### Final Refined Architecture

**`server/cmd/api/main.go`** — Register as Temporal worker for `planning-interactive` queue:
```go
if temporalClient != nil {
    planningWorker := worker.New(temporalClient, "planning-interactive", worker.Options{})
    planningActivities := NewPlanningSessionActivities(...)
    planningWorker.RegisterWorkflow(PlanningSessionWorkflow)
    planningWorker.RegisterActivity(planningActivities)
    go planningWorker.Run(worker.InterruptCh())
}
```

**Workflow + Activities:**

```
PlanningSessionWorkflow(sessionID):
  session = CreateSession(ctx)

  prepareCtx = WithSessionOptions(ctx)  // pin to worker
  workDir = PrepareSessionWorkspace(prepareCtx, sessionID)

  // Signal loop
  signalCh = GetSignalChannel(ctx, "planning_session_signal")
  for {
    signal = signalCh.Receive()
    switch signal.Type:
      "message":
        RunSessionTurn(sessionCtx, sessionID, workDir)
      "finalize":
        FinalizeSessionTurn(sessionCtx, sessionID, workDir, signal.ActorID)
        return  // workflow complete
      "abandon":
        CleanupWorkspace(sessionCtx, workDir)
        return  // workflow complete
      "heartbeat":
        // keep-alive, no-op
  }
  CleanupWorkspace(sessionCtx, workDir)
```

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `server/internal/temporalapp/planning_session_workflow.go` | Create | Workflow + signal types |
| `server/internal/temporalapp/planning_session_activities.go` | Create | Prepare, RunTurn, Finalize, Cleanup activities |
| `server/internal/temporalapp/engine.go` | Modify | Add StartPlanningSession, SignalPlanningSession |
| `server/internal/service/planning_session.go` | Modify | Replace goroutines with Temporal signals; add runEngine dependency |
| `server/cmd/api/main.go` | Modify | Register API server as Temporal worker for planning queue; wire dependencies |

## Key Design Decisions

1. **API server doubles as Temporal worker** for planning sessions — gives activities access to `wsHub` for streaming
2. **Temporal Sessions** pin all activities to the same worker — cloned repo persists across message turns
3. **Signal-driven loop** — each user message/finalize/abandon is a Temporal signal processed by the workflow
4. **Read-only tools only** — planning sessions use `PlanningSessionAllowedTools` (no write, no git push)
5. **Heartbeat timeout** — activities heartbeat during Claude streaming to prevent Temporal timeout
6. **Graceful cleanup** — `CleanupWorkspace` activity runs on abandon/finalize/workflow timeout

## Verification

1. Start a planning session on an epic with a planning repository configured
2. Verify Temporal workflow starts (check Temporal UI or logs)
3. Send a message — agent should be able to use `read_file`, `list_directory`, `ripgrep` on the cloned repo
4. Verify tool results reference actual repo files
5. Finalize — spec is written to docs, session completes, Temporal workflow finishes
6. Abandon — workspace cleaned up, workflow finishes
7. Check no temp directories leak after session ends
