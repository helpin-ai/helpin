# Streaming chat UI parity plan

## Status

Historical implementation plan comparing the March 31 coding-session UI with a local AG-UI checkout. The temporary `/tmp/ag-ui` paths are original research references, not repository dependencies.

This plan is specifically about:

- streaming assistant replies
- streaming reasoning or thinking
- streaming tool-call lifecycle and argument deltas
- attaching tool results to the current assistant turn
- rendering a chat UI that feels like AG-UI

This plan is not primarily about approvals and human-input interruptions. That work is already covered by:

- [docs/plans/2026-03-31-codex-native-interaction-contract-plan.md](2026-03-31-codex-native-interaction-contract-plan.md)
- [docs/plans/2026-03-31-coding-session-ui-and-runtime-contract-plan.md](2026-03-31-coding-session-ui-and-runtime-contract-plan.md)

This plan explains the original structured streaming model for assistant text,
reasoning, and tool calls. Use it for design rationale; current rendering and
runtime ownership have moved beyond the worker adapters proposed below.

## Source review — 2026-09-18

- [Stream payloads](../../server/internal/model/agent_run_message.go) contain the
  proposed parent-message, argument, activity, and reasoning fields. Current
  [Runtime projection](../../server/internal/service/agent_runtime_projection.go)
  maps assistant/reasoning lifecycle and tool argument/result events, including
  failure on a finished tool event with an error.
- The Eino/Codex/OpenCode worker adapter files named below are absent. Their
  historical rollout checklist is not the current executor architecture or proof
  that every external runtime produces every event variant.
- [The stream builder](../../frontend/src/components/pm/CodingSession/codingSessionStream.ts)
  maintains structured live assistant, reasoning, and tool-call state. Its public
  builder is `buildCodingSessionStreamState`; the suggested parser/function names
  below are sketches. `plan.updated`/`activity.updated` are recognized; the
  proposed `activity.snapshot`/`activity.delta` pair is not the current mapping.
- [The page](../../frontend/src/pages/pm/CodingSession.tsx) delegates to
  `CodingSessionSurface`. The old page-owned flat `liveAssistantText` problem no
  longer describes this entry point.
- Shared transcript rendering is described in the
  [later renderer plan](2026-06-15-unified-agent-transcript-renderer-plan.md).
  [CodingActivityRail](../../frontend/src/components/pm/CodingSession/CodingActivityRail.tsx)
  still supports completed tool calls alongside events, so its component contract
  is not strictly the sidecar-only rail proposed here.
- No AG-UI checkout comparison, provider stream run, or browser parity test was
  performed during this source review. The acceptance cases below remain dated
  intent rather than fresh validation results.

## Original plan

## Reference Model

The relevant AG-UI pieces are:

- event contract in `/tmp/ag-ui/sdks/typescript/packages/core/src/events.ts`
- message and tool-call state types in `/tmp/ag-ui/sdks/typescript/packages/core/src/types.ts`
- chunk normalization in `/tmp/ag-ui/sdks/typescript/packages/client/src/chunks/transform.ts`
- client-side event reducer in `/tmp/ag-ui/sdks/typescript/packages/client/src/apply/default.ts`
- reasoning producer example in `/tmp/ag-ui/integrations/langgraph/typescript/src/agent.ts`

The important takeaway is not AG-UI's visual shell. It is the state model:

- assistant text is streamed into a specific message by `messageId`
- tool calls are streamed independently by `toolCallId`
- tool results are attached to the matching tool call, not rendered as generic status noise
- reasoning is a distinct message family, not assistant prose
- activity cards are distinct from transcript messages

## Current Helpin Gap

Helpin now has typed interruption cards, but the streaming chat model is still too flat.

Current backend state:

- live stream events are published from `server/internal/temporalapp/activities.go` (historical path; absent from this checkout)
- `codingSessionEventTypeFromExecutionEvent` currently emits:
  - `assistant.message.started`
  - `assistant.message.delta`
  - `assistant.message.completed`
  - `tool.call.started`
  - `tool.call.completed`
  - `tool.call.failed`
- stream payloads are currently too thin in [server/internal/model/agent_run_message.go](../../server/internal/model/agent_run_message.go)

Current frontend state:

- [frontend/src/pages/pm/CodingSession.tsx](../../frontend/src/pages/pm/CodingSession.tsx) keeps one flat `liveAssistantText`
- transcript only uses completed messages
- [frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx](../../frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx) renders streaming text as one global block
- [frontend/src/components/pm/CodingSession/CodingActivityRail.tsx](../../frontend/src/components/pm/CodingSession/CodingActivityRail.tsx) renders all non-completed events in one side rail

This loses the turn structure that makes AG-UI feel correct:

- no stable live assistant message entity
- no reasoning entity
- no streamed tool argument buffer
- no tool result block attached to a tool call
- no notion of "assistant resumed after tool"

## Goal

Introduce a first-class streaming turn model for coding sessions where:

- assistant turns stream in place
- reasoning streams separately and renders as subdued "thinking"
- tool calls render inline inside the current assistant turn
- tool results attach to the matching tool call
- approvals, auth, and human-input remain in the interruption panel instead of polluting the transcript
- repo diff and repo state remain in the repo pane, not the chat pane

## Recommendation

Do not import AG-UI directly.

Do copy its event application model closely:

- normalize chunked events into explicit start or delta or end events
- apply those events into a structured session reducer keyed by stable ids
- render transcript from finalized turn state plus in-progress live state

## Target Event Model

Add or standardize these coding-session live event families.

### Assistant message lifecycle

- `assistant.message.started`
- `assistant.message.delta`
- `assistant.message.completed`

Required fields:

- `message_id`
- `parent_message_id` optional
- `role`
- `delta` for delta events
- `content` for completed events
- `sequence_no` when persisted

### Reasoning lifecycle

- `reasoning.message.started`
- `reasoning.message.delta`
- `reasoning.message.completed`

Required fields:

- `message_id`
- `parent_message_id` optional
- `delta`
- `content` for completed or snapshot events
- optional `encrypted_value` if a runtime supports redacted reasoning

Rules:

- reasoning is not assistant prose
- reasoning must not be merged into the assistant message buffer

### Tool-call lifecycle

- `tool.call.started`
- `tool.call.args.delta`
- `tool.call.completed`
- `tool.call.failed`
- `tool.call.result`

Required fields:

- `tool_call_id`
- `parent_message_id`
- `tool_name`
- `args_delta` for argument streaming
- `args_text` optional snapshot
- `result_message_id` for result attachment
- `content` or `output_summary`
- `duration_ms`
- `error`

Rules:

- `tool.call.started` creates the call shell
- `tool.call.args.delta` appends into the argument buffer
- `tool.call.result` creates a result row attached to the tool call
- `tool.call.completed` or `tool.call.failed` closes the call lifecycle

### Activity cards

- `activity.snapshot`
- `activity.delta`

Required fields:

- `activity_id`
- `activity_type`
- `content` for snapshot
- `patch` for delta

Rules:

- this is for plans, run status, and structured runtime progress
- this is not a transcript message

## Backend Changes

### 1. Expand stream payload shape

Update [server/internal/model/agent_run_message.go](../../server/internal/model/agent_run_message.go).

Add fields to `AgentRunStreamEvent`:

- `ParentMessageID`
- `ActivityID`
- `ActivityType`
- `ArgsDelta`
- `ArgsText`
- `Content`
- `ResultMessageID`
- `EncryptedValue`

Keep existing fields for compatibility:

- `Text`
- `ToolCallID`
- `ToolName`
- `ToolInput`
- `OutputSummary`
- `DurationMs`
- `Error`

Recommendation:

- prefer explicit new fields
- keep old ones temporarily as compatibility aliases

### 2. Expand internal execution event contract

Update `server/internal/worker/eino_exec.go` (historical path; absent from this checkout).

Expand `ExecutionEvent` so the worker layer can express:

- `MessageID`
- `ParentMessageID`
- `ArgsDelta`
- `Content`
- `ResultMessageID`
- `ActivityID`
- `ActivityType`
- `EncryptedValue`

This should become the normalized live event structure shared by:

- Eino
- Codex
- OpenCode

### 3. Make Codex emit structured tool and message ids

Update `server/internal/worker/codex_event_mapper.go` (historical path; absent from this checkout).

Current behavior:

- streams assistant deltas
- emits tool start and finish
- does not emit structured tool argument deltas
- does not emit message ids on assistant streaming events

Required changes:

- create a stable `message_id` for the current assistant turn
- include `message_id` on assistant start or delta or completed events
- set `parent_message_id` on tool calls so they attach to the current assistant turn
- emit `tool.call.args.delta` when Codex exposes incremental tool input
- emit `tool.call.result` when a concrete tool result body exists
- keep `tool.call.completed` or `failed` as lifecycle close events

### 4. Make Eino emit the same structure

Update `server/internal/worker/eino_exec.go` (historical path; absent from this checkout).

Current behavior:

- emits `assistant_message_delta`
- emits `tool_call_started`
- emits `tool_call_finished`

Required changes:

- generate and carry stable `message_id` for the assistant turn
- emit `tool.call.args.delta` while assembling tool args
- emit `tool.call.result` from executed tool output blocks
- optionally emit reasoning events if the provider exposes them

### 5. Make OpenCode conform to the same event shape

Update `server/internal/worker/opencode_stream.go` (historical path; absent from this checkout).

Goal:

- map OpenCode stream semantics into the same `ExecutionEvent` contract as Codex and Eino

### 6. Publish the richer live event contract into coding sessions

Update `server/internal/temporalapp/activities.go` (historical path; absent from this checkout).

Required changes:

- extend `publishRunStreamEvent`
- extend `codingSessionEventTypeFromExecutionEvent`
- include the new event payload fields when forwarding to `coding_session_event`

Replace the coarse mapping with:

- `assistant.message.started`
- `assistant.message.delta`
- `assistant.message.completed`
- `reasoning.message.started`
- `reasoning.message.delta`
- `reasoning.message.completed`
- `tool.call.started`
- `tool.call.args.delta`
- `tool.call.result`
- `tool.call.completed`
- `tool.call.failed`
- `activity.snapshot`
- `activity.delta`

### 7. Keep persisted session event reconstruction compatible

Update [server/internal/service/coding_session.go](../../server/internal/service/coding_session.go).

This file should continue to reconstruct durable history from:

- completed `agent_run_messages`
- interaction records
- artifacts
- run status

Do not try to reconstruct fine-grained live deltas from persisted rows in phase 1.

Recommendation:

- keep `ListCodingSessionEvents` as the durable history API
- treat websocket `coding_session_event` as the live incremental feed
- on refresh, rebuild finalized transcript from persisted rows, not from old deltas

## Frontend Changes

### 1. Introduce a stream reducer

Add a new frontend reducer layer, for example:

- `frontend/src/components/pm/CodingSession/codingSessionStream.ts`

This should be the Helpin equivalent of AG-UI's reducer in `/tmp/ag-ui/sdks/typescript/packages/client/src/apply/default.ts`.

Recommended reducer state:

```ts
type LiveAssistantMessage = {
  messageId: string
  role: "assistant"
  content: string
}

type LiveReasoningMessage = {
  messageId: string
  role: "reasoning"
  content: string
  encryptedValue?: string
}

type LiveToolCall = {
  toolCallId: string
  parentMessageId?: string
  toolName: string
  argsText: string
  status: "running" | "completed" | "failed"
  result?: {
    messageId?: string
    content: string
    error?: string
  }
  durationMs?: number
}

type LiveActivityCard = {
  activityId: string
  activityType: string
  content: Record<string, unknown>
}

type CodingSessionStreamState = {
  finalizedMessages: CodingSessionMessage[]
  liveAssistantMessage?: LiveAssistantMessage
  liveReasoningMessage?: LiveReasoningMessage
  liveToolCalls: Record<string, LiveToolCall>
  liveActivities: Record<string, LiveActivityCard>
}
```

### 2. Expand frontend session types

Update [frontend/src/lib/pm-types/codingSession.ts](../../frontend/src/lib/pm-types/codingSession.ts).

Add typed stream event payload shapes for:

- assistant deltas
- reasoning deltas
- tool-call args deltas
- tool-call results
- activity cards

Avoid using only `Record<string, unknown>` for the live-render path.

### 3. Add stream event parsers

Update [frontend/src/components/pm/CodingSession/codingSessionUtils.ts](../../frontend/src/components/pm/CodingSession/codingSessionUtils.ts).

Add helpers:

- `parseAssistantStreamEvent`
- `parseReasoningStreamEvent`
- `parseToolCallStreamEvent`
- `parseActivityStreamEvent`
- `applyCodingSessionStreamEvent`

`latestPendingCodingSessionInteraction` should remain separate. It solves a different problem.

### 4. Refactor the page state model

Update [frontend/src/pages/pm/CodingSession.tsx](../../frontend/src/pages/pm/CodingSession.tsx).

Remove:

- flat `liveAssistantText`

Replace with:

- stream reducer state
- finalized transcript data
- live turn state

Recommended page responsibilities:

- bootstrap finalized session history from REST
- ingest websocket `coding_session_event` stream
- apply live deltas into `CodingSessionStreamState`
- derive render models for transcript, inline tool cards, reasoning, and sidecar activities

### 5. Replace transcript rendering

Update [frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx](../../frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx).

New behavior:

- render finalized user and assistant messages as durable chat bubbles
- render the in-progress assistant message in place
- render reasoning as a subdued collapsible block attached to the active turn
- render tool calls inline under the active assistant message
- render tool results under the matching tool call

Do not render tool results as plain assistant prose.

### 6. Narrow the activity rail

Update [frontend/src/components/pm/CodingSession/CodingActivityRail.tsx](../../frontend/src/components/pm/CodingSession/CodingActivityRail.tsx).

New behavior:

- show only sidecar session activity:
  - plan cards
  - activity snapshots
  - run state transitions
  - auth and interaction checkpoints if you still want a passive audit trail

Do not use the rail as the catch-all destination for tool execution or live assistant state.

### 7. Keep interruptions separate

Keep `frontend/src/components/pm/CodingSession/CodingInterruptionPanel.tsx` (historical path; absent from this checkout) focused on:

- auth
- typed interactions
- cancel or safety controls

Do not merge streaming reasoning or tool lifecycle into that panel.

## Suggested UI Composition

### Transcript pane

The transcript should be turn-based:

- user bubble
- assistant bubble
- inline reasoning strip
- inline tool call cards
- inline tool result cards

Tool calls should appear visually nested under the current assistant turn, not as top-level transcript turns.

### Reasoning

Render reasoning as:

- muted background
- smaller type
- collapsed by default after completion

It should read like "thinking", not like an answer.

### Tool cards

Each tool card should show:

- tool name
- running or completed state
- streamed arguments as they arrive
- compact result body or summary
- failure state if applicable

### Activity rail

Keep it for:

- plan updates
- run phases
- repo status changes
- session checkpoints

## Rollout Plan

### Phase 1: Contract and producer changes

Files:

- [server/internal/model/agent_run_message.go](../../server/internal/model/agent_run_message.go)
- `server/internal/worker/eino_exec.go` (historical path; absent from this checkout)
- `server/internal/worker/codex_event_mapper.go` (historical path; absent from this checkout)
- `server/internal/worker/opencode_stream.go` (historical path; absent from this checkout)
- `server/internal/temporalapp/activities.go` (historical path; absent from this checkout)

Outcome:

- richer coding-session live events with stable ids and tool or reasoning structure

### Phase 2: Frontend reducer and types

Files:

- [frontend/src/lib/pm-types/codingSession.ts](../../frontend/src/lib/pm-types/codingSession.ts)
- [frontend/src/components/pm/CodingSession/codingSessionUtils.ts](../../frontend/src/components/pm/CodingSession/codingSessionUtils.ts)
- new `codingSessionStream.ts`
- [frontend/src/pages/pm/CodingSession.tsx](../../frontend/src/pages/pm/CodingSession.tsx)

Outcome:

- AG-UI-style state assembly in Helpin

### Phase 3: Transcript and tool UI

Files:

- [frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx](../../frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx)
- [frontend/src/components/pm/CodingSession/CodingActivityRail.tsx](../../frontend/src/components/pm/CodingSession/CodingActivityRail.tsx)

Optional new components:

- `CodingReasoningStrip.tsx`
- `CodingToolCallCard.tsx`
- `CodingToolResultRow.tsx`

Outcome:

- proper streaming chat UX instead of flat text plus generic rail

### Phase 4: Runtime parity and cleanup

Files:

- runtime-specific worker adapters
- coding-session tests

Outcome:

- Codex, OpenCode, and Eino-backed `native_sdk` all stream through one session model
- old flat `liveAssistantText` logic is removed

## Testing

Backend tests to add:

- producer tests for Codex tool-call args or result events
- producer tests for Eino tool-call result attachment
- `activities.go` tests for richer coding-session stream event forwarding

Frontend tests to add:

- reducer tests for assistant start or delta or completed
- reducer tests for reasoning start or delta or completed
- reducer tests for tool start or args delta or result or complete
- page tests confirming a tool call is rendered inline with the current assistant turn

## Non-Goals

- adopting AG-UI's library directly
- persisting every live delta as a durable row in phase 1
- rendering raw runtime-specific payloads in the transcript

## Final Recommendation

Treat this as the next major coding-session slice after typed interactions.

Typed interactions solved:

- pauses
- approvals
- user input
- auth and review checkpoints

This plan solves:

- streaming chat shape
- reasoning visibility
- tool-call visibility
- turn continuity

That is the missing piece required for Helpin's coding-session UI to actually feel like AG-UI rather than a run log with a better shell.
