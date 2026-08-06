# Dock Chat Transcript Ordering Fix Plan

**Date:** 2026-08-06
**Bug:** In the Ask Agents dock chat, user and agent messages render out of order —
user messages cluster at the top, agent messages cluster together below.

## Root cause

The dock transcript sorts by `agent_run_messages.created_at`
(`frontend/src/components/agents/dock/dockChatState.ts:117`, fed by
`model.CodingSessionEventFromAgentRunMessage`, `server/internal/model/coding_session.go:123`).

- **User messages** are persisted at send time by `AgentService.createRunMessage`
  (`server/internal/service/agent.go:6160`) → conversation-true `created_at`.
- **Assistant messages** are mirrored from runtime events by
  `mirrorAssistantMessageCompleted`
  (`server/internal/service/agent_runtime_projection.go:987`), which never sets
  `CreatedAt`, so GORM `autoCreateTime` stamps **helpin's insert time**.

When assistant events arrive late — reconciliation sweep replay
(`ReconcileMappedRuns` → `replayV2Events` → `ApplyEvent`) after the NATS consumer
missed live events — all assistant rows are inserted in one burst with near-identical,
late timestamps. Every user message then sorts before the assistant clump.
The `sequence_no` tiebreak can't help: it is also assigned in projection-arrival
order (`NextSequence` at insert), and the timestamps aren't equal anyway.

The correct time already exists and is discarded: the v2 event envelope carries
`SentAt`, stamped at emission by the runtime and persisted with the event
(`agent-runtime/internal/engine/persisted_event_sink.go:33`,
`internal/store/gorm.go:278`), so replayed events keep their original time.
The projection even has a helper — `eventTime(event)`
(`agent_runtime_projection.go:2141`) — and the reconcile-transcript path
(`createRuntimeMessage`, line 1353) already honors the runtime store's
`CreatedAt`. Only the assistant-message mirror path ignores it.

---

## Phase 1 — helpin-only fix (ships immediately, fixes the reported bug)

**1.1** `server/internal/service/agent_runtime_projection.go` —
`mirrorAssistantMessageCompleted` (~line 987): set

```go
message.CreatedAt = s.eventTime(event)
```

on the constructed `model.AgentRunMessage` before `runMessageRepo.Create`.
(`eventTime` falls back to `nowUTC()` when `SentAt` is zero, preserving current
behavior for envelopes without a timestamp.)

**1.2** Regression test in `agent_runtime_projection_test.go`: apply an
`assistant.message.completed` envelope with `SentAt` in the past; assert the
persisted message's `CreatedAt` equals `SentAt` (UTC), not now. Add a second
case with zero `SentAt` asserting the fallback still stamps a non-zero time.

**1.3** Frontend guard test in
`frontend/src/components/agents/dock/__tests__/dockChatState.test.ts`
(if not already covered): a transcript where assistant `timestamp`s are
conversation-true but `sequence_no`s are arrival-ordered must sort
user → assistant → user → assistant.

**Scope note:** fixes all newly projected messages. Existing broken rows keep
their wrong `created_at` (see Phase 3 backfill option).

## Phase 2 — v2 streaming protocol: message-level `created_at`

Envelope `SentAt` on a `completed` event is emission time; make the message's own
creation time explicit so projections are independent of transport timing.

**2.1** `agent-runtime` — add `created_at` to the assistant message event payload:
- `AssistantMessageEventData` gains `CreatedAt time.Time \`json:"created_at,omitempty"\``.
- Emitters in `internal/runtime/` (native, codex, opencode paths that publish
  `assistant.message.*`) stamp it from the runtime store message row (the same
  clock `AppendMessage` persists).

**2.2** `agent-runtime-go` client — mirror the field on
`AssistantMessageEventData` (`events.go:94`); tag and release (v0.2.x → v0.3.0
if other v2 changes land together).

**2.3** helpin — bump `github.com/helpin-ai/agent-runtime-go`;
`mirrorAssistantMessageCompleted` prefers `data.CreatedAt` when non-zero,
falling back to `s.eventTime(event)` from Phase 1.

## Phase 3 — durable ordering by runtime sequence (optional, hardening)

Wall-clock ordering is still clock-skew-sensitive. The runtime store assigns a
monotonic per-run `SequenceNo` to every message (`agent-runtime/internal/store/gorm.go`
`messageRecord`), already exposed via `ListMessages`.

**3.1** helpin model: add nullable `RuntimeSequenceNo *int` to
`model.AgentRunMessage` (AutoMigrate handles the column).

**3.2** Populate it:
- `createRuntimeMessage` (reconcile path): from `runtimeMessage.SequenceNo`.
- `mirrorAssistantMessageCompleted`: from the `lookupRuntimeStoreMessage` result
  when reachable.
- Dedup path: when `agentRunMessageMatchesRuntimeMessage` matches a
  helpin-persisted user row to its runtime copy
  (`agent_runtime_projection.go:1594`), backfill `RuntimeMessageID` and
  `RuntimeSequenceNo` onto the existing row instead of returning early.

**3.3** Surface it through `CodingSessionEventFromAgentRunMessage` payload →
frontend transcript message type → `transformDockStream`: sort primarily by
`runtime_sequence_no` **when both compared messages have it**, else fall back to
the existing timestamp-then-sequence comparison.

**3.4 (optional backfill)** Extend `reconcileRuntimeTranscript` to repair
existing rows: when a matched runtime store message has a `CreatedAt` differing
from the helpin row by more than a threshold (e.g. 30s), update the helpin row's
`CreatedAt`. This heals historical jumbled transcripts on the next reconcile
sweep without a data migration.

## Addendum (2026-08-06): actual root cause in the reported repro

After Phase 1 shipped, the jumbling persisted. DB inspection showed all
`agent_run_messages` rows (timestamps AND sequences) were already correct for
the affected chats — the corruption was client-side:

`collectSegments` (`frontend/src/components/agents/transcript/segments.ts`)
treats the cumulative live snapshot timeline as the authoritative ordering
while a run is active, hoisting every persisted segment matched in
`live_turn_segments` to the tail. The server snapshot
(`model.ApplyCodingSessionStreamEvent`) never resets between chat turns, and
dock chat runs stay alive (paused `awaiting_user_message`) across turns — so
the live timeline accumulates ALL assistant segments while user messages only
exist in the transcript. Result: every user message stacked on top, all
assistant content clumped below.

**Fix:** persisted segments positioned before the last user message are
"settled" — they keep their transcript position and their live counterparts
are dropped; only the current turn's tail (after the last user message)
follows the live timeline ordering. Covered by
`segments.test.ts` "keeps earlier turns interleaved when the live timeline
spans multiple chat turns".

Phase 1's timestamp fix remains correct and necessary for the replay-projection
path; Phases 2–3 are unchanged.

## Rollout order & verification

1. Phase 1 → deploy helpin (`develop` → stage). Verify: open a dock chat, send
   several messages across agent turns, kill/restart the NATS consumer mid-run
   to force sweep replay, confirm ordering stays interleaved.
2. Phase 2 → deploy agent-runtime first (additive field, old helpin ignores it),
   then helpin with the bumped client.
3. Phase 3 → helpin + frontend together (frontend change is backward-compatible
   with rows lacking `runtime_sequence_no`).

Tests to run: `cd server && go test ./internal/service/...`,
`cd frontend && pnpm vitest run src/components/agents/dock`.
