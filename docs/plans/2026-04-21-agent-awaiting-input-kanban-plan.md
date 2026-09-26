# Agent attention badges and notification dispatch

**Date:** 2026-04-21
**Author:** azhar
**Branch:** `feature/live-chat-events-pipeline` (or new branch off `develop`)
**Status:** Historical plan; source-compared 2026-09-18


This plan records the original proposal for making paused agent runs visible.
Use the current notes to distinguish implemented UI from unverified notification
production; the original “verified” section describes an earlier checkout.

## Current implementation and limits

- [PMTask](../../server/internal/model/pm_task.go) includes
  `latest_run_pause_reason` with `gorm:"-"`: it is a response enrichment, not
  the proposed persisted task column. The
  [repository](../../server/internal/repository/pm_task.go) loads latest-run
  metadata from `agent_runs`, choosing by started-or-created recency. It does
  not select only active runs, so a newer terminal run can hide an older pause.
  The proposed column migration and backfill are not the current design.
- [TaskCard](../../frontend/src/components/pm/TaskCard.tsx) derives an attention
  label when the latest run is paused. It supports human input, approval,
  authentication, and `awaiting_user_message`; unknown nonempty reasons fall
  back to “Awaiting your input.” Its memo comparison includes pause reason.
- [Realtime sync](../../frontend/src/hooks/useRealtimeSync.ts) invalidates
  notification queries and shows a ten-second attention toast for matching
  `created` events addressed to the signed-in user, excluding self-actor events.
  Its body is generic, not a task title plus reason-specific subtitle. The CTA
  uses `parent_task_id` and opens the task URL without an explicit run parameter;
  the plan's promise that this CTA opens the paused run is not established.
- [NotificationCenter](../../frontend/src/components/notifications/NotificationCenter.tsx)
  gives attention rows amber styling. Its navigation separately supports a
  task/run target when the notification contains the required metadata.
- The old `server/internal/temporalapp/activities.go` emitter no longer exists.
  A search of current `server/internal` Go sources found the exact event type in
  category/notification definitions and tests, but did not establish a current
  producer emitting it for all pause reasons. UI handling alone does not prove
  delivery or the proposed one-to-two-second latency.
- [Coding-session cleanup](../../server/internal/service/coding_session.go)
  calls [MarkAgentAttentionResolved](../../server/internal/service/notification.go),
  which marks matching unread notifications as read. This is not deletion or
  archival and does not by itself prove every terminal path invokes cleanup.

No application tests, live pauses, notification sends, or browser verification
were performed for this source review. The original rollout and test instructions
are historical; nullable fields or AutoMigrate assumptions alone do not establish
safe deployment. Use the current versioned migration process for actual schema
changes.

## Original plan

## Problem

When an agent run pauses and requests input from a user, there is **no prominent surface** in the product that flags this as actionable. Specifically:

- On the PM **kanban board**, a task whose agent is awaiting input looks visually identical to one whose agent is running. Both show the subtle "Last run X ago" text with a spinning agent badge.
- Users have no reliable signal that their attention is required unless they open the task and drill into the agent run.
- The `task.agent_attention_required` notification event **is** already emitted from the temporal activity (`server/internal/temporalapp/activities.go:1920–1954`), but we have not verified it reaches the **NotificationCenter** reliably, and there is no toast/sound/badge cue on arrival.

The first surface we want to fix is the **kanban board** (card view). List view and detail view can follow.

## Current state (verified)

### Backend

- `agent_runs.status = "paused"` + `pause_reason ∈ {"human_input", "human_approval", "authentication"}` is how "awaiting input" is modeled (`server/internal/model/agent.go:326–338`).
- `pm_tasks` already denormalizes `latest_run_id`, `latest_run_agent_id`, `latest_run_status`, `latest_run_at` — but **not `latest_run_pause_reason`**.
- Notification event constant `taskAgentAttentionRequiredEventType = "task.agent_attention_required"` (`server/internal/service/notification.go:110`).
- Category mapping: `"task.agent_attention_required" → NotifCategoryAgentAttention` (`server/internal/model/notification.go:184`).
- Emission point: `server/internal/temporalapp/activities.go:1920–1954` on interaction pause. Metadata includes `run_id`, `task_id`, `interaction_id`, `pause_reason`, `agent_id`. Priority `"high"`.
- Resolution helper: `NotificationService.MarkAgentAttentionResolved(ctx, wsID, runID)`.
- WebSocket agent-run broadcast (`server/internal/websocket/run_notifier.go`) already carries `pause_reason`.

### Frontend

- `TaskCard.tsx:580–622` renders `TaskCardAgentBadge` when `latest_run_status ∈ {queued, running, paused}`. It spins when running, goes green/red for completed/failed. **Pause reason is not surfaced**.
- `Task` type (`frontend/src/lib/pm-types/project.ts:311–314`) has `latest_run_*` fields but no `latest_run_pause_reason`.
- `NotificationCenter` (`frontend/src/components/notifications/NotificationCenter.tsx`) is mounted in the sidebar header (`Sidebar.tsx:239`). Uses `useNotifications(wsId, filter)` (infinite query, 30s staleTime) and `useUnreadCount()` (30s refetch interval). Filter tabs: `all | mentions | assigned`. Agent-attention category constant: `'agent_attention'` (`frontend/src/lib/notificationTypes.ts:128`).
- **No dedicated toast or sound on new notification arrival** in the current code path — the bell updates via polling.

## Goals

1. **Kanban card**: when a task's latest run is `paused` and requires user action, show a prominent, unmistakable badge like `⚠ Awaiting input` (amber/orange) in place of the subtle "Last run ago" row.
2. **NotificationCenter**: ensure `task.agent_attention_required` notifications show up, are visually distinguishable (priority = high already), and drive an actionable toast on arrival so users don't have to click the bell to discover them.
3. **Consistency**: pause reasons `human_approval` and `authentication` get their own labels ("Awaiting approval", "Needs auth") using the same badge treatment.
4. **Resolution**: when the run resumes or completes, the badge and notification auto-clear (via existing `MarkAgentAttentionResolved`).

Non-goals for this pass:

- Changing the list view or board filters (follow-up).
- Changing the agent run detail drawer.
- Adding sound (we'll use toast only — sound can be a later opt-in).

## Plan

### Phase 1 — Backend: denormalize pause reason on task

**Files to touch:**

- `server/internal/model/pm_task.go` (or wherever `latest_run_*` fields live) — add `LatestRunPauseReason *string \`json:"latest_run_pause_reason" gorm:"column:latest_run_pause_reason"\``. AutoMigrate handles the column add (nullable).
- Find the call site that sets `latest_run_status` on the task when an agent_run transitions — likely in `server/internal/service/agent_run*.go` or the temporal activity. Update it to write `latest_run_pause_reason` alongside (value = `agent_run.pause_reason` when status is `paused`, `nil` otherwise).
- **Backfill**: add a `dbmigrate` entry (`server/internal/dbmigrate/sql/YYYYMMDDNNNN_backfill_task_pause_reason.sql`) that populates `latest_run_pause_reason` for existing paused tasks by joining to `agent_runs` on `latest_run_id`. Idempotent, `IF EXISTS` guards.

**Acceptance:**

- New column exists on `pm_tasks`.
- Fetching a task whose latest run is paused returns `latest_run_pause_reason` = `"human_input"` (or other).
- Unit test in `pm_task_service_test.go` covers: status transitions to `paused` → pause_reason set; transitions to `running`/`completed`/`failed` → pause_reason cleared.

### Phase 2 — Backend: confirm notification emission covers all pause reasons

**Files to touch:**

- Read `server/internal/temporalapp/activities.go:1920–1954` to verify `task.agent_attention_required` is emitted for all three pause reasons (`human_input`, `human_approval`, `authentication`). If it currently only fires for `human_input`, broaden the trigger.
- Verify the notification `metadata.pause_reason` is set (used by the frontend to pick the title/icon).
- Verify `MarkAgentAttentionResolved` is called when the run resumes, completes, fails, or is cancelled — so stale notifications don't linger.

**Acceptance:**

- Emitting a paused state for all three pause reasons produces a notification row with `event_type = "task.agent_attention_required"`, `latest_event_category = "agent_attention"`, `priority = "high"`, `metadata.pause_reason` populated.
- Run resumption marks matching notifications as resolved (status = `read` or archived per existing logic).

### Phase 3 — Frontend: surface pause state on kanban card

**Files to touch:**

- `frontend/src/lib/pm-types/project.ts` — add `latest_run_pause_reason?: AgentRunPauseReason | null` to `Task`.
- `frontend/src/lib/pm-types/agents.ts` — `AgentRunPauseReason` type already exists; re-export if not already available in `project.ts` imports.
- `frontend/src/components/pm/TaskCard.tsx`:
  - In the existing agent-badge block (around line 580–622), when `latest_run_status === "paused"` AND `latest_run_pause_reason && latest_run_pause_reason !== "none"`, render an **amber pill** in place of the "Last run ago" line. Copy:
    - `human_input` → `⚠ Awaiting your input`
    - `human_approval` → `⚠ Awaiting approval`
    - `authentication` → `⚠ Needs auth`
  - Pill styling: `bg-amber-100 dark:bg-amber-500/15 text-amber-900 dark:text-amber-200 border-amber-300`, rounded-full, `text-xs`, with `AlertCircle` icon from `lucide-react`.
  - Keep the agent avatar on the left; replace only the status text/dot.
  - Optionally: add a thin amber left-border accent on the whole card (`border-l-2 border-l-amber-500`) to draw the eye from across the board.

**Acceptance:**

- A paused task with `pause_reason = "human_input"` shows the amber "Awaiting your input" pill.
- A running task shows the existing spinner badge (unchanged).
- Toggling theme (light/dark) keeps contrast acceptable.
- Manual test: create a run, pause it via the backend, confirm badge updates in real time via WebSocket invalidation.

### Phase 4 — Frontend: notification toast on arrival

**Files to touch:**

- `frontend/src/hooks/queries/useNotifications.ts` — no change to fetchers, but…
- `frontend/src/hooks/useRealtimeSync.ts` (or wherever the WebSocket listener lives) — on receiving a `notification.created` event with `event_type === "task.agent_attention_required"`, dispatch a `sonner` toast:
  - Title: "Agent needs your attention"
  - Body: the notification `title` (task key + name) + pause reason subtitle
  - Action button: "Open task" → `navigate({ to: "/w/$slug/pm/tasks", search: { task: taskKey } })`
  - Duration: 10s (high-priority, not ephemeral)
- Invalidate `queryKeys.notifications.*` on the same event so the bell updates immediately without waiting for the 30s poll.

**Acceptance:**

- Backend emits a new agent_attention notification → user sees a toast within 1–2s with an "Open task" CTA.
- Clicking the CTA opens the task drawer with the run visible.
- Toast does not fire for the user's own actions (e.g., if they triggered the pause themselves — skip if `actor_id === currentUserId`).

### Phase 5 — NotificationCenter visual polish

**Files to touch:**

- `frontend/src/components/notifications/NotificationCenter.tsx` (and/or the `NotificationRow` component).
- Agent-attention category rows should render with:
  - The amber icon/color treatment matching the kanban pill (consistency).
  - A small inline "Open run" or "Resolve" action per row.
  - Priority `high` notifications get a subtle left-border accent inside the popover.

**Acceptance:**

- Agent-attention rows are visually distinct from assignment / comment rows.
- Clicking the row navigates to the task + opens the run.

## Testing

### Backend

- `go test ./internal/service/...` — unit tests for pause_reason denormalization and notification emission coverage of all three pause reasons.
- `go test ./internal/temporalapp/...` — integration test (if pattern exists) that a paused run emits the expected notification metadata.
- `go vet ./... && go build ./...`.

### Frontend

- Manual: run `pnpm dev`, pause a run via backend, verify kanban badge + toast + bell update.
- Type check: `pnpm exec tsc -b`.
- No unit tests in `frontend/` for TaskCard today — add a light Vitest snapshot if the component has a test file, otherwise rely on manual verification (per project norm).

## Migration / Rollout

- Column is nullable and added via AutoMigrate — safe to deploy without downtime.
- Backfill migration is idempotent.
- Frontend changes are purely additive (extra field on `Task` type is optional).
- No feature flag needed — behavior degrades gracefully if `latest_run_pause_reason` is absent (card falls back to current "Last run ago" display).

## Open questions

1. Should the amber pill also appear on the **list view** in this pass, or is kanban-first enough? *(User asked kanban-first — defer list view to follow-up.)*
2. Should we add a "Resolve" action directly on the kanban card, or keep it in the task drawer? *(Default: drawer only — keep the card read-only.)*
3. Sound on new agent-attention notification — opt-in setting? *(Out of scope for this plan.)*
4. Should `actor_id === currentUserId` skip apply to the bell too, or only the toast? *(Default: only toast. The user should still see a historical record in the bell.)*

## Follow-ups (not in this plan)

- **Emit `task.agent_attention_required` for `pause_reason="authentication"`** — currently only `human_input` + `human_approval` emit (via `maybeNotifyAgentAttentionRequired` at `activities.go:1876`). Auth pause is set at `activities.go:719` without emitting. Kanban pill from Phase 3 still surfaces it, but the bell does not.
- List view badge (same logic, smaller footprint).
- Agent run detail drawer styling for paused state.
- Filter chip on kanban board: "Awaiting input" quick filter.
- Opt-in sound for high-priority notifications.
- Surface pause reason on CRM / support cards if they ever gain agent runs.
