# Coding Session UI and Runtime Contract Plan

## Status

Draft plan for replacing the current agent-run drawer/log-viewer experience with a first-class Helpin coding session UI.

This plan assumes:

- Codex, OpenCode, and `native_sdk` remain supported runtimes
- Helpin owns the product UX, persistence, auth surfaces, approval flows, and repo-facing session state
- runtimes are adapter-backed execution engines behind one Helpin session contract

Current execution references (the retired in-process rollout plans remain in Git history):

- [Coding Agent Runtime Flow](../CODING_AGENT_RUNTIME_FLOW.md)
- [Agents and automation](../AGENTS_AND_AUTOMATION.md)

## Goal

Introduce a proper coding-session product surface where:

- Codex, OpenCode, and `native_sdk` all appear through the same Helpin-owned session model
- the frontend consumes one normalized live event contract
- the backend persists one durable session/message/artifact model
- runtime-specific differences are exposed as capabilities and adapter mappings, not as different UI architectures

## Recommendation

Build Helpin's own coding UI.

Do not adopt AG-UI as the product contract directly.

Do borrow heavily from AG-UI's event model:

- assistant text start/delta/end
- tool lifecycle events
- activity/status events
- state snapshot and state delta semantics
- interrupt and resume concepts

Do not build a formal AG-UI adapter in phase 1.

Keep the internal contract close enough to AG-UI concepts that an adapter would be straightforward later if interoperability ever matters.

## Core Decisions

### 1. One product contract across all coding runtimes

Yes, Codex, OpenCode, and `native_sdk` should all implement the same outer contract.

That contract should cover:

- session creation and resume
- transcript
- assistant streaming
- tool execution events
- repo state and diff state
- approval and human-input pauses
- auth-required pauses
- preview/published artifacts
- completion and failure

Runtimes may differ internally in:

- how they stream
- what tool events they can expose
- whether they own a thread/session natively
- whether they support approvals/auth natively
- how much repo detail they can emit incrementally

Those differences should be surfaced through capability flags and optional fields, not different frontend architectures.

### 2. Browser transport should be WebSocket or SSE, not webhooks

Do not use webhooks to drive the frontend.

Reason:

- webhooks are server-to-server delivery
- browsers cannot safely or directly act as webhook receivers
- Helpin already has a workspace WebSocket path and realtime event model

Recommended transport:

- browser uses Helpin WebSocket for live session events
- REST remains for initial load, pagination, and explicit actions
- Temporal and workers publish normalized events into Helpin's realtime hub

If needed later:

- internal backend fan-out can be event-bus or webhook-like
- frontend should still consume WebSocket or SSE only

### 3. Build a real `CodingSession` product surface, not a bigger `AgentRunDrawer`

The current drawer in [AgentRunDrawer.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDrawer.tsx) is still a mixed transcript/artifact/log surface.

That was useful for rollout, but it is not the right final architecture for coding work.

The new UI should be a dedicated session workspace with:

- transcript pane
- live tool/activity rail
- repo/diff pane
- preview/published panel surface
- interruption/auth/approval cards
- run controls and session metadata

The existing drawer can continue as a compatibility shell or compact summary view, but the main experience should move to dedicated `CodingSession` components.

## Non-Goals

- replacing all non-coding agents with the coding session UI
- making runtime provider-side state authoritative
- exposing runtime-specific raw protocols directly to the browser
- building AG-UI interoperability first
- rewriting all PM run surfaces at once

## Problem Summary

Current issues in the existing implementation:

- the UI still reconstructs meaning from `run/messages/artifacts` plus ad hoc stream events
- Codex/OpenCode/native runs do not feel like one coherent product
- auth, approval, and human-input experiences are bolted onto the run drawer
- repo changes, changed files, and previews are not first-class session state
- autonomous and interactive coding behavior still leak runtime-specific logic into UX
- the current transport is partly polling, partly custom event streaming

## Target Architecture

```text
CodingSession page
  -> Helpin REST for session bootstrap + actions
  -> Helpin WebSocket for live session events
    -> Helpin API/service/session layer
      -> Temporal workflow / run orchestration
        -> runtime adapter (codex | opencode | native_sdk)
          -> runtime-native transport
             codex app-server / opencode process / native in-process SDK
```

## Product Model

Introduce a first-class coding-session concept on top of `agent_runs`.

Recommended rule:

- `agent_run` remains the durable workflow/execution record
- `coding_session` becomes the product-facing interaction shell for coding agents

Short-term compatibility option:

- keep `agent_run` as the backing record
- add a session-focused API and frontend model without a new DB table immediately

Long-term preferred shape:

- `coding_sessions`
- `coding_session_events`
- `coding_session_state_snapshots`

If schema expansion is deferred, emulate these through:

- `agent_runs`
- `agent_run_messages`
- `agent_run_artifacts`
- `agent_run_stream` websocket events

## Unified Runtime Contract

## Contract Principles

1. Helpin owns product-visible truth.
2. Every coding runtime maps into the same session event model.
3. Runtime-specific details are allowed only under `runtime_metadata` or capability flags.
4. A missing runtime capability should degrade gracefully in UI.
5. Interruptions are first-class state, not implied by prose.

## Core Session Object

Recommended frontend/session shape:

```ts
type CodingSession = {
  id: string
  runId: string
  workspaceId: string
  targetType: string
  targetId: string
  agentId: string
  runtimeKind: "codex" | "opencode" | "native_sdk"
  invocationMode: "interactive" | "autonomous"
  status: "queued" | "running" | "paused" | "completed" | "failed" | "cancelled"
  pauseReason?: "human_input" | "human_approval" | "authentication" | "none"
  title: string
  summary?: string
  capabilities: CodingSessionCapabilities
  repo: CodingSessionRepoState
  authState?: CodingSessionAuthState
  createdAt: string
  updatedAt: string
}
```

Recommended capability shape:

```ts
type CodingSessionCapabilities = {
  liveTextStreaming: boolean
  toolStreaming: boolean
  repoDiffStreaming: boolean
  planStreaming: boolean
  approvals: boolean
  humanInput: boolean
  authentication: boolean
  previews: boolean
  terminalOutput: boolean
  checkpoints: boolean
}
```

## Event Model

Use one normalized event stream for all runtimes.

Recommended event families:

- `session.started`
- `session.updated`
- `session.paused`
- `session.resumed`
- `session.completed`
- `session.failed`
- `assistant.message.started`
- `assistant.message.delta`
- `assistant.message.completed`
- `assistant.reasoning.delta`
- `tool.call.started`
- `tool.call.delta`
- `tool.call.completed`
- `tool.call.failed`
- `activity.started`
- `activity.updated`
- `activity.completed`
- `repo.state.updated`
- `repo.diff.updated`
- `preview.updated`
- `approval.requested`
- `input.requested`
- `auth.requested`
- `auth.updated`

Recommended envelope:

```ts
type CodingSessionEvent = {
  id: string
  sessionId: string
  runId: string
  sequenceNo: number
  timestamp: string
  type: string
  runtimeKind: "codex" | "opencode" | "native_sdk"
  payload: Record<string, unknown>
  runtimeMetadata?: Record<string, unknown>
}
```

The sequence number is important.

Do not rely on client receive order alone.

## Action Contract

Recommended session actions:

- `POST /api/pm/coding-sessions/:id/message`
- `POST /api/pm/coding-sessions/:id/resume`
- `POST /api/pm/coding-sessions/:id/approve`
- `POST /api/pm/coding-sessions/:id/request-changes`
- `POST /api/pm/coding-sessions/:id/cancel`
- `POST /api/pm/coding-sessions/:id/auth/device-code/start`
- `POST /api/pm/coding-sessions/:id/auth/device-code/cancel`

Compatibility rule:

- these can route to existing `agent_run` services/workflows initially
- frontend should move to session endpoints as soon as practical

## Runtime Adapter Contract

All runtime adapters should implement one common shape.

Recommended Go interface:

```go
type CodingRuntimeAdapter interface {
    Start(ctx context.Context, req StartCodingSessionRequest) (*StartCodingSessionResult, error)
    Resume(ctx context.Context, req ResumeCodingSessionRequest) error
    Cancel(ctx context.Context, req CancelCodingSessionRequest) error
    Capabilities() CodingRuntimeCapabilities
}
```

Runtime adapters are responsible for:

- translating runtime-native events into normalized Helpin session events
- mapping runtime-native pauses into Helpin pause reasons
- persisting runtime checkpoints or session ids as runtime-local metadata
- exposing repo state, diff state, and artifacts where supported

Runtime adapters are not responsible for:

- owning the frontend contract
- deciding product-visible statuses
- inventing their own approval or auth UX

## Runtime Mapping Rules

### Codex

Codex should remain the richest external runtime and the main reference for the coding-session contract.

Codex mapping examples:

- app-server text stream -> `assistant.message.*`
- tool requests/results -> `tool.call.*`
- approval requests -> `approval.requested`
- user input requests -> `input.requested`
- auth-required -> `auth.requested`
- thread/checkpoint state -> runtime metadata

### OpenCode

OpenCode should map into the same contract even if its native protocol is poorer.

Minimum OpenCode contract:

- assistant text
- tool execution status
- repo mutation/diff state
- completion/failure

If OpenCode cannot emit a rich event class natively, the adapter should synthesize normalized events from logs, process boundaries, tool wrappers, and repo inspections.

### Native SDK

`native_sdk` should implement the same contract for coding-style runs.

For coding-session participation, `native_sdk` should expose:

- assistant stream
- tool lifecycle
- structured pause reasons
- repo state and file mutation artifacts
- preview events where relevant

This does not require the planner/support agents to move into the coding UI.

It means that any `native_sdk` coding agent must speak the same session contract if it appears in the coding surface.

## Persistence Model

## Durable State

Persist these as first-class durable state:

- session status
- pause reason
- assistant messages
- tool invocations and results
- approval/input/auth requests
- repo status snapshots
- diff summaries
- published preview metadata
- runtime checkpoint references
- auth state artifacts

## Runtime Local State

Allow runtimes to keep local execution state such as:

- Codex thread ids and home directories
- OpenCode process/session metadata
- native provider response continuation ids

But those are execution aids only.

Helpin DB remains authoritative for product state.

## Frontend UX Plan

## Main Surface

Create a dedicated component tree under:

- `frontend/src/components/pm/CodingSession/*`

Recommended structure:

- `CodingSessionPage`
- `CodingSessionHeader`
- `CodingTranscriptPane`
- `CodingActivityRail`
- `CodingRepoPane`
- `CodingDiffView`
- `CodingPreviewPane`
- `CodingInterruptionCard`
- `CodingAuthCard`
- `CodingRunControls`

## Layout

Recommended desktop layout:

- left: transcript and interruptions
- center: live activity and tool flow
- right: repo, diff, preview, changed files

Recommended mobile behavior:

- transcript remains primary
- repo/diff/preview become tabs or drawers

## First-Class UI States

The UI must treat these as explicit states:

- `running`
- `paused:human_input`
- `paused:human_approval`
- `paused:authentication`
- `completed`
- `failed`

Do not force the user to infer state from transcript prose or hidden artifacts.

## Realtime Delivery Plan

## Recommendation

Use Helpin WebSocket events for live session updates.

Keep REST for:

- bootstrap
- pagination/history fetch
- explicit commands
- reload/recovery

Do not use browser webhooks.

## Event Delivery Shape

Recommended workspace event names:

- `coding_session-created`
- `coding_session-updated`
- `coding_session_event-created`

Short-term compatibility:

- these can coexist with existing `agent_run-*` events
- the coding UI can subscribe to both during migration

Long-term:

- coding session UI should depend only on session events, not generic run events

## Session Bootstrap

Recommended page load flow:

1. `GET /api/pm/coding-sessions/:id`
2. `GET /api/pm/coding-sessions/:id/events?after=...` for replay if needed
3. subscribe to WebSocket live events
4. reconcile gaps with `sequenceNo`

This avoids the current overreliance on polling.

## Workstreams

## Workstream 1: Define the Shared Session Contract

### Backend

Files likely touched:

- `server/internal/model/agent.go`
- `server/internal/service/agent.go`
- `server/internal/handler/agent.go`
- `server/internal/temporalapp/workflow.go`
- `server/internal/temporalapp/activities.go`
- new files under `server/internal/service` and `server/internal/model`

Tasks:

1. Define normalized session status, pause reasons, and capabilities.
2. Define normalized event families and payload shapes.
3. Add a session-oriented read API over existing run data.
4. Add sequence numbers and replay semantics for live events.
5. Document runtime metadata rules.

Exit criteria:

- one contract document exists in code and docs
- all coding runtimes can map into it without runtime-specific frontend branches

## Workstream 2: Build Runtime Adapters to the Shared Contract

### Backend

Files likely touched:

- `server/internal/worker/codex*.go`
- `server/internal/worker/opencode*.go`
- `server/internal/worker/eino*.go`
- `server/internal/worker/runtime_adapter.go`
- `server/internal/worker/runtime_factory.go`

Tasks:

1. Extract runtime-native event translation into one normalized event publisher.
2. Ensure Codex, OpenCode, and `native_sdk` emit the same session events.
3. Add capability reporting per runtime.
4. Add parity tests that compare event mapping across runtimes.

Exit criteria:

- all three runtimes emit the same outer contract
- missing runtime features are exposed through capabilities, not UI breakage

## Workstream 3: Ship the Dedicated Coding Session UI

### Frontend

Files likely added:

- `frontend/src/components/pm/CodingSession/*`
- `frontend/src/pages/pm/CodingSession.tsx`
- `frontend/src/lib/services/codingSessionService.ts`
- `frontend/src/lib/pm-types/codingSession.ts`

Tasks:

1. Build the dedicated coding session page and layout.
2. Move auth, approval, input, and repo state into first-class panels.
3. Replace artifact parsing as the primary UX model.
4. Keep the old drawer as a fallback/summary shell during migration.

Exit criteria:

- Codex and OpenCode no longer depend on the legacy drawer for normal use
- the UI renders off session state and live session events, not ad hoc artifact heuristics

## Workstream 4: Realtime Contract Migration

### Backend and Frontend

Tasks:

1. Move live coding UX from mixed polling plus run events to session events.
2. Add replay endpoints for missed events.
3. Add reconnection and gap recovery using `sequenceNo`.
4. Keep polling only as a resilience fallback.

Exit criteria:

- coding sessions remain coherent across reconnects and refreshes
- live session state does not require aggressive polling

## Workstream 5: Repo State and Diff as First-Class Product State

### Backend and Frontend

Tasks:

1. Add normalized repo-state snapshots:
   - branch
   - dirty state
   - changed file count
   - changed file list
2. Add normalized diff summaries and file-level diff fetch APIs.
3. Allow runtimes to update repo state incrementally.
4. Render changed files and diff independent of transcript text.

Exit criteria:

- repo changes are visible even if assistant prose is sparse
- completion does not depend on transcript text to explain code modifications

## Workstream 6: Interruptions as a Shared State Machine

### Backend and Frontend

Tasks:

1. Normalize `human_input`, `human_approval`, and `authentication`.
2. Give each interruption a stable id and resolution status.
3. Attach interruption records to the assistant turn and session event stream.
4. Make resume/approve/request-changes/auth actions flow through one session action model.

Exit criteria:

- interruptions behave consistently across Codex, OpenCode, and `native_sdk`
- no runtime needs bespoke pause UX

## Rollout Plan

## Phase 1

- define shared session types
- expose session read APIs backed by existing run records
- emit normalized session events for Codex first
- build a read-only coding session page

## Phase 2

- make Codex use the new session UI as primary
- add auth, approval, input, repo, and diff panels
- add OpenCode adapter parity

## Phase 3

- add `native_sdk` parity for coding-style runs
- move from mixed polling to sequence-based live session streaming
- make the old drawer secondary

## Phase 4

- introduce first-class DB tables for session events if needed
- decide whether interoperability warrants an AG-UI adapter

## Explicit Recommendation on AG-UI

Use AG-UI as a design reference, not as the product contract.

Specifically copy the good parts:

- event-oriented runtime thinking
- state snapshot/delta mindset
- tool lifecycle semantics
- clean interrupt model

Do not copy blindly:

- protocol naming as product surface
- generic abstractions that weaken Helpin-specific UX
- draft features as hard dependencies

## Risks

- trying to preserve the old drawer as the main experience too long
- leaking runtime-specific quirks back into the session contract
- treating artifacts as the primary UX state instead of durable session state
- over-indexing on protocol purity before shipping the actual coding UI
- using polling as the default forever instead of proper live session streaming

## Open Questions

- whether `coding_session` should become a new DB table immediately or remain a session-shaped API over `agent_runs` first
- whether session events should be persisted in their own append-only table or materialized from messages/artifacts plus stream logs in phase 1
- whether `native_sdk` coding participation should begin with only `code_builder`-style flows or include review-style flows immediately

## Success Criteria

- Codex, OpenCode, and `native_sdk` coding runs all render through one Helpin-owned session UI
- the browser consumes one normalized live event contract
- auth, approval, and human-input are first-class session states
- repo state and diff are visible without reading assistant prose
- reconnect and refresh preserve coherent live session state
- adding an AG-UI adapter later would be straightforward, but unnecessary for Helpin to ship

## Phase 1 Implementation Checklist

This section is the concrete execution sequence for the first shippable version.

Phase 1 goal:

- keep existing `agent_run` persistence
- add a session-shaped read and event contract on top of it
- make Codex the first runtime on the new contract
- ship a dedicated read-first coding session page without waiting for all runtimes to be perfect

## Phase 1 Scope

In scope:

- new session-shaped backend read models
- new coding session REST endpoints backed by `agent_run`
- new live websocket event envelope for coding sessions
- Codex adapter mapping into the new session events
- dedicated `CodingSession` page and layout
- compatibility bridge from old run records/artifacts into session state

Out of scope:

- new DB tables
- full OpenCode parity
- full `native_sdk` parity
- complete removal of the old run drawer
- AG-UI adapter work

## Phase 1 Deliverables

1. A `coding_session` API shape backed by current `agent_run` records.
2. A `coding_session_event` live transport and replay shape.
3. A dedicated `CodingSession` page in the PM UI.
4. Codex as the first runtime publishing normalized session events.
5. A compatibility translator that derives session state from current messages and artifacts when no direct runtime event exists yet.

## Contract Definitions

## REST Read Contract

### `GET /api/pm/coding-sessions/:id`

Response:

```json
{
  "id": "session_123",
  "run_id": "run_123",
  "workspace_id": "ws_123",
  "target_type": "story",
  "target_id": "story_123",
  "agent_id": "agent_123",
  "runtime_kind": "codex",
  "invocation_mode": "interactive",
  "status": "paused",
  "pause_reason": "authentication",
  "title": "Track payload validation errors",
  "summary": "Implement 4xx payload validation tracking in rust-capture",
  "capabilities": {
    "live_text_streaming": true,
    "tool_streaming": true,
    "repo_diff_streaming": true,
    "plan_streaming": true,
    "approvals": true,
    "human_input": true,
    "authentication": true,
    "previews": false,
    "terminal_output": true,
    "checkpoints": true
  },
  "repo": {
    "repo_name": "usermaven/events-pipeline",
    "branch": "tp-77-track-payload-validation-errors-400-and-404-not-found-errors",
    "is_dirty": true,
    "changed_file_count": 3
  },
  "auth_state": {
    "provider": "openai",
    "auth_mode": "chatgpt_device_code",
    "state": "pending",
    "verification_url": "https://...",
    "user_code": "ABCD-EFGH",
    "updated_at": "2026-03-31T00:00:00Z"
  },
  "created_at": "2026-03-31T00:00:00Z",
  "updated_at": "2026-03-31T00:00:10Z"
}
```

### `GET /api/pm/coding-sessions/:id/events?after=123`

Response:

```json
{
  "events": [
    {
      "id": "evt_124",
      "session_id": "session_123",
      "run_id": "run_123",
      "sequence_no": 124,
      "timestamp": "2026-03-31T00:00:05Z",
      "type": "assistant.message.delta",
      "runtime_kind": "codex",
      "payload": {
        "message_id": "msg_12",
        "text": "I'll start by exploring the codebase"
      }
    }
  ],
  "next_sequence_no": 124
}
```

### `GET /api/pm/coding-sessions/:id/repo`

Response:

```json
{
  "repo_name": "usermaven/events-pipeline",
  "branch": "tp-77-track-payload-validation-errors-400-and-404-not-found-errors",
  "is_dirty": true,
  "changed_file_count": 3,
  "changed_files": [
    {
      "path": "rust-capture/src/router.rs",
      "status": "modified"
    }
  ]
}
```

### `GET /api/pm/coding-sessions/:id/diff?path=rust-capture/src/router.rs`

Response:

```json
{
  "path": "rust-capture/src/router.rs",
  "diff": "@@ ...",
  "is_truncated": false
}
```

## REST Action Contract

Use top-level JSON objects with `snake_case` fields.

### `POST /api/pm/coding-sessions/:id/message`

```json
{
  "content": "Please continue and implement the metrics changes.",
  "attachments": []
}
```

### `POST /api/pm/coding-sessions/:id/resume`

```json
{
  "intent": "reply",
  "content": "Continue coding now"
}
```

`intent` enum:

- `reply`
- `approve`
- `request_changes`

### `POST /api/pm/coding-sessions/:id/auth/device-code/start`

```json
{}
```

### `POST /api/pm/coding-sessions/:id/auth/device-code/cancel`

```json
{}
```

## WebSocket Event Contract

Recommended workspace event names:

- `coding_session-created`
- `coding_session-updated`
- `coding_session_event-created`

Event detail shape:

```json
{
  "entity_id": "session_123",
  "parent_id": "run_123",
  "data": {
    "id": "evt_124",
    "session_id": "session_123",
    "run_id": "run_123",
    "sequence_no": 124,
    "timestamp": "2026-03-31T00:00:05Z",
    "type": "tool.call.started",
    "runtime_kind": "codex",
    "payload": {
      "tool_call_id": "tool_1",
      "tool_name": "run_command",
      "tool_input": {
        "cmd": "rg -n \"payload validation\" rust-capture/src"
      }
    }
  }
}
```

Rules:

- every event must include `sequence_no`
- `sequence_no` must increase monotonically per session
- clients must be able to reconcile with `GET /events?after=...`
- no event type should require parsing transcript prose or opaque artifact text

## File-Level Phase 1 Work

## Backend

### 1. Add session read models

Add:

- `server/internal/model/coding_session.go`

Recommended types:

- `CodingSession`
- `CodingSessionCapabilities`
- `CodingSessionRepoState`
- `CodingSessionAuthState`
- `CodingSessionEvent`
- `CodingSessionEventListResponse`

### 2. Add service layer

Add:

- `server/internal/service/coding_session.go`

Responsibilities:

- load session-shaped data from `agent_runs`
- translate run/artifact state into `CodingSession`
- expose event replay API
- expose repo and diff read APIs
- route action requests to existing run services

### 3. Add handler endpoints

Add to:

- `server/internal/handler/agent.go`
- `server/internal/router/router.go`

Routes:

- `GET /api/pm/coding-sessions/{id}`
- `GET /api/pm/coding-sessions/{id}/events`
- `GET /api/pm/coding-sessions/{id}/repo`
- `GET /api/pm/coding-sessions/{id}/diff`
- `POST /api/pm/coding-sessions/{id}/message`
- `POST /api/pm/coding-sessions/{id}/resume`
- `POST /api/pm/coding-sessions/{id}/approve`
- `POST /api/pm/coding-sessions/{id}/request-changes`
- `POST /api/pm/coding-sessions/{id}/cancel`
- `POST /api/pm/coding-sessions/{id}/auth/device-code/start`
- `POST /api/pm/coding-sessions/{id}/auth/device-code/cancel`

### 4. Add event publisher

Add:

- `server/internal/service/coding_session_events.go`

Responsibilities:

- map runtime events into `CodingSessionEvent`
- assign `sequence_no`
- publish to websocket hub
- optionally persist short replay window if needed

Phase 1 persistence option:

- derive replay from persisted `agent_run_messages` plus selected artifacts and emitted stream events

If that is too lossy, add a lightweight append-only event table in phase 2.

### 5. Add Codex adapter mapping first

Touch:

- `server/internal/worker/codex_event_mapper.go`
- `server/internal/worker/codex_session_host.go`
- `server/internal/temporalapp/activities.go`

Tasks:

- publish normalized `assistant.message.*`
- publish normalized `tool.call.*`
- publish `auth.requested`, `auth.updated`
- publish `approval.requested`, `input.requested`
- publish `repo.state.updated` when repo snapshots change

### 6. Add compatibility translators

Add:

- `server/internal/service/coding_session_compat.go`

Responsibilities:

- parse current `human_input_request`
- parse current `human_approval_request`
- parse current `codex_auth_state`
- parse runtime stdout/stderr artifacts where needed
- synthesize session state when direct event history is incomplete

## Frontend

### 1. Add session types

Add:

- `frontend/src/lib/pm-types/codingSession.ts`

Types:

- `CodingSession`
- `CodingSessionEvent`
- `CodingSessionRepoState`
- `CodingSessionAuthState`
- `CodingSessionCapabilities`

### 2. Add service adapter

Add:

- `frontend/src/lib/services/codingSessionService.ts`

Methods:

- `get`
- `listEvents`
- `getRepo`
- `getDiff`
- `sendMessage`
- `resume`
- `approve`
- `requestChanges`
- `cancel`
- `startDeviceCodeAuth`
- `cancelDeviceCodeAuth`

### 3. Add dedicated page

Add:

- `frontend/src/pages/pm/CodingSession.tsx`

Route shape recommendation:

- `/w/$slug/pm/coding-sessions/$sessionId`

### 4. Add component tree

Add under:

- `frontend/src/components/pm/CodingSession/`

Initial components:

- `CodingSessionLayout.tsx`
- `CodingSessionHeader.tsx`
- `CodingTranscriptPane.tsx`
- `CodingActivityRail.tsx`
- `CodingRepoPane.tsx`
- `CodingInterruptionCard.tsx`
- `CodingAuthCard.tsx`

Phase 1 note:

- the diff view can be simple at first
- reuse existing transcript and artifact rendering pieces where sensible
- do not let the new page depend on the legacy drawer container

### 5. Add websocket bridge

Touch:

- existing realtime sync hook or websocket event hub files

Tasks:

- subscribe to `coding_session-*`
- append events by `sequence_no`
- reconcile gaps with `listEvents`
- keep polling only as fallback

## Concrete Build Order

### Step 1

Define TS and Go session types first.

Reason:

- the contract must stabilize before more UI work happens

### Step 2

Ship read-only backend endpoints.

Reason:

- the frontend can start using the new page without waiting for action parity

### Step 3

Wire Codex event publishing into the session event contract.

Reason:

- Codex already has the richest runtime event shape and is the best anchor

### Step 4

Build the dedicated `CodingSession` page with transcript, activity rail, and auth/input cards.

### Step 5

Route existing Codex action buttons through session endpoints.

### Step 6

Add repo and diff panels.

### Step 7

Bridge OpenCode into the same event contract.

### Step 8

Add `native_sdk` parity for coding-oriented runs.

## Validation Checklist

## Backend tests

Add tests for:

- session state derivation from `agent_run`
- event sequence ordering
- replay from `after`
- Codex event mapping into normalized events
- pause reason mapping
- auth state mapping
- repo state read endpoints

Likely files:

- `server/internal/service/coding_session_test.go`
- `server/internal/service/coding_session_events_test.go`
- `server/internal/worker/codex_event_mapper_test.go`
- `server/internal/handler/agent_test.go`

## Frontend tests

Add tests for:

- session page render by status
- auth interruption rendering
- approval interruption rendering
- live event append and reconciliation
- repo pane file list
- diff fetch and display

Likely files:

- `frontend/src/components/pm/CodingSession/__tests__/*`
- `frontend/src/pages/pm/__tests__/CodingSession.test.tsx`

## Migration Checklist

1. Keep [AgentRunDrawer.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDrawer.tsx) working during migration.
2. Add a “Open coding session” entry point from existing run surfaces.
3. Use the new page for Codex first behind a feature flag if needed.
4. Keep existing run APIs intact until the session page reaches parity.
5. Once Codex is stable, make the drawer a compact summary and fallback view.

## Phase 1 Exit Criteria

- a coding session can be opened on its own route
- Codex runs render through the new page
- auth, input, and approval show as first-class interruption cards
- live assistant and tool events stream over websocket with replay support
- repo state and changed files are visible in a dedicated pane
- no part of the new page depends on parsing assistant prose to infer run state
