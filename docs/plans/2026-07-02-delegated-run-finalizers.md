# Delegated Run Finalizers (agent-runtime → Helpin product side effects)

Status: implemented 2026-07-02. Scope: `AgentRuntimeProjectionService` terminal-event
handling, new `AgentRunFinalizerService`, temporal-worker wiring, `HandoffRun` delegated guard.

## Problem

Temporal-executed runs fire product side effects inside `temporalapp/activities.go`
(`markAgentIdle`, `evaluateRunCompletedRules`, `finalizeSupportConversationRun`,
`finalizePlanningRun`). Delegated runs (`external_runtime='agent-runtime'`) only project
status/messages/artifacts back via NATS — none of these side effects fire, so agents stay
"working", `agent_run.completed` automations never trigger, and support drafts are never
turned into messages.

## (a) Terminal-transition trigger rule

`ApplyEvent` resolves the local run row before applying an event, so it knows the prior
persisted status. The dispatch signal is threaded explicitly:

- `wasTerminal := isTerminalAgentRunStatus(run.Status)` captured at entry.
- `transitioned := isTerminalRuntimeEvent(event.Type) && !wasTerminal &&
  isTerminalAgentRunStatus(run.Status)` after the event is applied.
- Finalizers are dispatched **after the event switch but before the final
  `runRepo.Update` persists the terminal status**. Ordering rationale: if the process
  crashes mid-dispatch, the NATS message is unacked and the local row is still
  non-terminal, so redelivery recomputes `transitioned == true` and re-enters the
  dispatch; per-finalizer idempotency markers (below) make already-completed finalizers
  no-ops. This is what makes the markers crash-safe "between finalizers".
- Redelivered or reconciled terminal events on an **already-terminal** run compute
  `transitioned == false` and never dispatch — replay no-op guaranteed at the dispatch
  gate, independent of markers.
- The reconciliation sweep funnels through the same `ApplyEvent`, so a missed terminal
  NATS event still produces exactly one transition when the sweep synthesizes it.

Known accepted edge: the automation-rule finalizer evaluates `agent_run.completed` while
the DB row is committed one write later (Temporal persists first, then evaluates). A rule
action that immediately queries "active run on this target" inside the same millisecond
can still see the old row. Accepted: the window is a single sequential write in the same
consumer, and the alternative (persist-first) destroys crash-replay of the finalizer set.

Finalizer failures are isolated (billing precedent 6d8bcf04): each finalizer error is
logged at ERROR with workspace/run ids and the loop continues; `ApplyEvent` never returns
a finalizer error, so status projection and the NATS ack are never blocked by a finalizer.

## (b) Idempotency key per finalizer

Markers live in `agent_runs.output_summary` (same pattern as
`agent_runtime_usage_consumed`) and are persisted with the targeted
`AgentRunRepository.UpdateOutputSummary` — never a full-row `Save` — so they survive a
crash between finalizers without touching projection-owned status/pause fields. All
host-reserved keys use the `agent_runtime_` prefix.

| Finalizer | Idempotency guard | Marker write order |
|---|---|---|
| 1. Agent idle + monthly tokens | `agent_runtime_finalizer_agent_idle` | after the agent update (mirrors usage pattern: consume, then mark). Crash window = worst case one duplicated monthly-token increment; a stuck-"working" agent (the bug being fixed) is the failure mode we refuse. The marker also protects live agents: a late replayed terminal event can never flip an agent that has since started a new run back to idle. |
| 2. `agent_run.completed` automation rules | `agent_runtime_finalizer_automation_rules` | **before** `EvaluateEvent` (at-most-once). Automation actions are user-visible; a crash may lose one evaluation but replay can never double-fire rules. |
| 3. Support draft finalization | natural: reply `SupportMessage.ID == run.ID` (create-if-not-exists via `GetByID`) plus `sent_message_id` recorded back into `output_summary`, exactly like the Temporal original. No extra marker needed. |
| 4. Planning / flow-output finalization | `agent_runtime_finalizer_planning`, written after. Epic pointer write (`epic.last_planning_run_id = run.ID`) is naturally idempotent; flow-output validation is read-only. |

Dispatch is transition-gated (see (a)), so markers are a second line of defense for
crash-replay, not the primary replay filter.

## (c) OutputSummary input contract

For delegated runs the adapter-produced run summary lives on the **runtime** side
(`AgentRuntimeRun.OutputSummary`); the runtime preserves adapter output through
cumulative merging, so the terminal snapshot contains everything the adapter wrote.
The local row only carries host markers. On the terminal **transition** (only), the
projection fetches the runtime run once and merges its `OutputSummary` into the local
summary before dispatch:

- merge rule: runtime keys overwrite local keys, **except** keys prefixed
  `agent_runtime_` (host-reserved markers always win);
- the merged summary is persisted atomically with the terminal status by the final
  `runRepo.Update`;
- if the fetch fails, summary-dependent finalizers (3 and the flow-output branch of 4)
  are skipped **without** writing their markers and the error is logged at ERROR, so a
  later duplicate terminal event from the runtime can retry them; summary-independent
  finalizers (1, 2, epic branch of 4) still run.

Contract keys each finalizer reads:

- **Finalizer 3 (support draft)** reads `output_summary.draft_reply`:
  `{"content": string (required, non-empty), "is_internal": bool,
  "sender_display_name": string?, "approval_required": bool}` and writes back
  `sent_message_id: string`. Runs only for `target_type == "support_conversation"`,
  `approval_state != "pending"`, on the **completed** transition. The delegated adapter
  must write `draft_reply` into the runtime run OutputSummary for a reply to be sent.
- **Finalizer 4 (flow output)** reads the whole `output_summary` as either
  `model.TaskCompletionAssessment` (`flow_output_kind == "pm.task_completion_followups"`)
  or `model.CRMDealReviewActionPlan` (`crm.deal_review_actions`); both require a
  non-empty `summary` field.
- **Finalizer 4 (planning preset)** reads no summary keys; it is keyed off
  `run.input.flow_output_kind` and `run.target_type == "epic"`.
- **Finalizers 1–2** read no summary keys (only run token counters / task linkage).

## Finalizer scope decisions

1. **Agent status bookkeeping** — port of `activities.markAgentIdle`: `status=idle`,
   `active_task_id=nil`, `tokens_used_this_month += run.tokens_used`. Fires on every
   terminal transition (completed/failed/cancelled). `CancelRun` already idles the agent
   on the cancel write-path but does not add tokens; the marker makes the pair converge.
2. **Automation `agent_run.completed` rules** — port of
   `activities.evaluateRunCompletedRules`: requires `task_id` and
   `target_type in (task, story)`; loads the task and feeds
   `model.TriggerAgentRunCompleted` into the `AutomationRuleEngine` (already constructed
   in `cmd/temporal-worker/main.go`). Completed transition only.
3. **Support draft finalization** — port of `activities.finalizeSupportConversationRun`
   including websocket `SupportMessageEvent` publish and visitor conversation-list
   refresh. Completed transition only. Conversation is loaded from
   `run.conversation_id` (fallback `target_id`).
4. **Planner/flow output** — SCOPED port of `finalizePlanningRun`/`finalizeFlowOutputRun`:
   - implemented: epic-target planner pointer (`epic.last_planning_run_id = run.id`);
     flow-output decode validation for `pm.task_completion_followups` and
     `crm.deal_review_actions` (validation failure is logged at ERROR — the projection
     cannot fail a runtime-owned run, see deferred list).
   - **deferred (documented gaps, not half-ported):**
     - failing the run on invalid flow output (`failRun`) — run status is owned by the
       runtime for delegated runs; the host must not overwrite it. Gap: an invalid
       delegated flow-output run completes "green" with an ERROR log instead of failing.
     - `applyApprovedInteractivePreview` (create_tasks / persist_prd / persist_task_doc
       application) — depends on `resolvedRunState`, docs/task/git repositories, preview
       artifacts and interaction plumbing inside `temporalapp` (~45 deps), not reachable
       from the service-layer projection worker.
     - `preparePlanningRepository`, `captureTranscriptPlanningArtifacts`,
       `enforceCompletionInteractionPolicy` / `retryInvalidCompletionTurn` /
       `synthesizeCompletionInteractionFallback` — execution-time machinery that has no
       meaning after a delegated run already reached terminal state in the runtime.
     - delivery-target state transitions tied to git execution (`state.deliveryTarget`).
5. **Notifications** — verified: the Temporal path's only `NotificationEmitter.Emit`
   site is `task.agent_attention_required` (run_interaction_persistence.go:164), fired on
   *pending interactions*, not on terminal transitions. There is **no run-completion
   notification in the Temporal path**, so emitting one only for delegated runs would
   diverge behavior. Finalizer 5 is therefore an intentional no-op for parity;
   attention-required notifications for delegated interactions are a separate
   (interaction-projection) surface, out of scope here.

## Handoff gap

The agent-runtime `/v1` API exposes no handoff route (checked
`/root/agent-runtime/internal/api/` and the Go SDK — handoff only exists inside the
runtime's internal Temporal workflow). `AgentService.HandoffRun` previously signaled
Temporal unconditionally, a silent no-op for delegated runs that still wrote handoff
records. It now rejects delegated runs up front with
"handoff is not supported for delegated agent runtime runs" before any writes.
All other `runEngine` signal sites in agent.go (`SignalResume` ×2, `CancelRun`) already
branch on `agentRuntimeRunID(run)`.

## Wiring

`AgentRuntimeProjectionService.SetRunFinalizers(*AgentRunFinalizerService)` follows the
`SetOverageDependencies`/`SetTranscriptRepositories` setter pattern. In
`cmd/temporal-worker/main.go` the projection consumer construction/start moved after the
`AutomationRuleEngine` is built so the finalizer service can be injected with: agent,
task (story), epic, support-conversation and support-message repositories, the rule
engine, the ws publisher, and the run repository (for targeted summary updates).
