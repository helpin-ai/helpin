# Command runs rail persistence and scoping plan

**Date:** 2026-04-29
**Status:** Historical proposal; backend portions implemented, original frontend superseded.
**Scope:** Tighten what happens when a user closes/refreshes the rail, so the rail reliably reflects "my recent command runs" without depending on incidental WebSocket events.


## Current implementation review

Source-compared on 2026-09-17. Use this page to understand the original rail
persistence decisions, not to implement its checklist unchanged.

The [router](../../server/internal/router/router.go) exposes recent runs at
`GET /api/pm/agent-runs/recent?workspace_id=…&limit=20`, guarded by `pm.read`.
The [handler](../../server/internal/handler/agent.go) gets the actor from the
request context. The [repository](../../server/internal/repository/agent.go)
filters by workspace and triggering user. Invalid limits, including values above
50, reset to 20; they are not clamped to 50. This query has neither a last-day
cutoff nor the proposed join excluding runs from dismissed plans.

[Plan dismissal](../../server/internal/service/command_bar_plans.go) and the
[versioned migration](../../server/internal/dbmigrate/sql/202604290001_command_bar_plan_dismissals.sql)
exist. The actual routes use `/api/command-bar/plans/dismiss` and
`/api/command-bar/plans/{id}/dismiss`, with workspace supplied separately.
Dismissal skips unknown or non-owned plans; listing filters dismissed IDs after
fetching records. Its fetch window is capped at 50, so many dismissed records can
still shrink the returned visible window. Entity-specific plan listing deliberately
uses different visibility rules and ignores personal dismissal.

The original `CommandBarRunRail` and `useCommandBarRunStore` are no longer the
frontend entry points. The current [Dock store](../../frontend/src/stores/dockStore.ts)
persists collapsed state and workspace-specific tab/chat/run selection with
best-effort localStorage writes. It does not implement the proposed
`helpin:cmdk-rail` Zustand persist slice. Current Dock run queries separate chat
runs from non-chat activity; the legacy recent-runs endpoint is not sufficient
to describe that surface.

The absolute isolation statement and acceptance criteria below are historical
requirements, not a completed security audit or fresh test results. In particular,
actor-filtered database queries do not by themselves prove every WebSocket and
client cache path. The old Clear all behavior, rollback, 404 fallback, and restore
UI should not be assumed to describe the current Dock.

## Original proposal

## Problem

Today the rail's in-memory store is rehydrated only from `GET /command-bar/plans` (latest 10 plans for the actor). This causes three real gaps:

1. **Standalone agent runs** (runs not tied to a command-bar plan — automations, "run agent" actions, follow-ups outside a plan) only enter the rail through `agent_run-created` / `agent_run-updated` WebSocket events. After a refresh they are gone until a new event for them arrives.
2. **The ✕ "Clear all" header button** clears the store on the current device but does not stick — the next mount re-pulls the same plans from the server, so it is not really "clear".
3. **Tab counts, filter selection, and rail mode** are partially persisted (rail mode goes to `localStorage`) but the rest is in-memory only — the user gets inconsistent state across tabs and across refreshes.

The rail is already user-scoped server-side (plans are filtered by `actor_id`, every mutation runs through `commandBarPlanOwnedByActor`), so the gaps are purely about reseeding and persistence, not authorization.

## Goal

After this work:

- A user closing or refreshing the rail does not lose visibility of their recent runs (plans **and** standalone runs).
- "Clear all" is a real action — dismissed items stay dismissed for that user.
- The rail rehydrates deterministically on mount from server data, not from whichever events happen to land first.
- No information from another user's runs ever appears in this user's rail (existing guarantee, preserved).

## Non-Goals

- Cross-device live sync of rail UI state (filter, scroll position).
- Persisting the rail across organizations or workspaces other than the active one.
- Replacing the WebSocket event stream — events stay as the live update channel; persistence/rehydrate is the cold-start path.
- Building a full "agent run history" page. Rail stays a recent-activity surface; longer history lives on the agent runs page.

## Backend Changes

### 1. Recent agent runs endpoint (per actor)

Add a thin endpoint that lists the current user's recent runs in the workspace, regardless of whether they are part of a command-bar plan:

- **Route**: `GET /api/workspaces/{workspace_id}/agent-runs/recent?limit=20`
- **Handler**: new method on `AgentRunHandler`
- **Service**: `AgentRunService.ListRecentForActor(ctx, workspaceID, actorID, limit)`
- **Repo**: `AgentRunRepository.ListRecentForActor(ctx, workspaceID, actorID, limit)` — `WHERE workspace_id = ? AND triggered_by_user_id = ? ORDER BY created_at DESC LIMIT ?`
- **Permission**: `PermAgentRunRead` (already exists) plus an in-handler check that the resolved actor matches the requested filter (the actor is always self — no `user_id` query param).
- **Response shape**: `{ runs: AgentRun[] }` — same shape the WebSocket and existing list endpoints use, so the frontend store update path is unchanged.

Limit defaults to 20, hard-capped at 50.

### 2. Per-user dismissal of command-bar plans

Add server-backed dismissal so "Clear all" persists across devices:

- **Migration** (dbmigrate): create `command_bar_plan_dismissals(workspace_id uuid, plan_id uuid, user_id uuid, dismissed_at timestamptz, primary key (workspace_id, plan_id, user_id))`. Indexed on `(workspace_id, user_id)` for the list join.
- **Repo**: `DismissPlans(ctx, workspaceID, userID, planIDs)`, `ListDismissedPlanIDs(ctx, workspaceID, userID, since)`.
- **Service**: extend `CommandBarService.ListPlans` to left-join (or post-filter) dismissed plan IDs and exclude them. Add `DismissPlans(ctx, workspaceID, actorID, planIDs)` and `RestorePlans` (undo, optional).
- **Routes**:
  - `POST /api/workspaces/{workspace_id}/command-bar/plans/dismiss` — body `{ plan_ids: string[] }` (used by "Clear all" — pass current visible plan IDs).
  - `POST /api/workspaces/{workspace_id}/command-bar/plans/{id}/dismiss` — single dismissal from a plan card overflow menu.
- **Auth**: must be the actor on each plan (`commandBarPlanOwnedByActor`).

Dismissal is per-user, so two engineers in the same workspace clearing their rails do not affect each other.

### 3. (Optional, deferred) Standalone run dismissal

Same shape as `command_bar_plan_dismissals` but for `agent_run` IDs. Defer until after we see whether plan-level dismissal already covers the common case — most rail noise comes from completed plans.

## Frontend Changes

### 4. Rehydrate standalone runs on mount

Where: `CommandBarRunRail` mount effect.

- After the existing `commandBarService.listPlans(...)` call, fan out a parallel `agentRunService.listRecent(workspaceId, 20)`.
- Merge results into the store via the existing `addRuns` action so runs that are part of a hydrated plan are deduped by ID.
- Active runs (status in `ACTIVE_RUN_STATUSES`) get priority in render order; the rail filter logic already handles the rest.

### 5. Wire "Clear all" to backend dismissal

- Replace the local-only `clear` store action with a service call: `commandBarService.dismissPlans(workspaceId, currentVisiblePlanIds)`.
- On success, remove those plans from the in-memory store **and** keep an in-memory `dismissedPlanIds` set for this session so a quick re-mount before the server roundtrip lands does not flash the plans back.
- On next mount, the server already excludes them from `listPlans`, so the rail comes up clean.
- Add a per-card overflow action ("Hide from rail") that calls the single-plan dismiss endpoint, for surgical removal.

### 6. Persist rail UI state

Apply `zustand/middleware/persist` to a narrow slice of `useCommandBarRunStore`:

- Persisted: `railMode`, `filter` (currently component-local — promote to store), `lastSeenAt` (timestamp of last hydrate).
- **Not** persisted: `runIds`, `runsById`, `planIds`, `plansById`. We rehydrate those from the server on every mount — local persistence of run snapshots invites stale-status bugs (e.g. a run that completed while the tab was closed would still render as running until an event lands).
- Storage key: `helpin:cmdk-rail` (single key, JSON). Versioned via the persist middleware `version` to allow future migrations.

This makes the existing `localStorage`-only rail-mode persistence consistent with the rest of the rail's UI state.

### 7. Tab/filter counts driven by hydrated data only

Already true today; verify it stays true after the standalone-run rehydrate lands. No new code expected, but worth a manual QA pass: open rail → close tab → reopen → counts match server state, not events received.

## Data & State Flow After Changes

On mount:

1. `listPlans` → seeds plans + their child runs. Excludes user-dismissed plan IDs server-side.
2. `listRecentRuns` → seeds standalone runs (and any plan runs we missed; deduped by ID).
3. WebSocket events (`agent_run-created`, `agent_run-updated`) update store as they arrive — unchanged.
4. UI state (`railMode`, `filter`) read from persisted slice.

On "Clear all":

1. Frontend optimistically removes plans from store.
2. `POST /command-bar/plans/dismiss` with the visible plan IDs.
3. On success, those plans stay gone next mount because the server excludes them.
4. On failure, restore from store snapshot and toast the error.

## Migration / Rollout

- Backend migration creates `command_bar_plan_dismissals`. Idempotent SQL — `CREATE TABLE IF NOT EXISTS`, indexes guarded.
- New endpoints are additive — no breaking changes.
- Frontend gracefully degrades if the new endpoints 404 (older backend): falls back to existing `listPlans`-only behavior.

## Acceptance Criteria

- After a hard refresh, a user's standalone agent runs from the last day still appear in the rail.
- "Clear all" removes plans for the current user and they do not return on refresh.
- Two users in the same workspace see independent rails — neither user's dismissals or runs leak.
- TypeScript build, `go vet`, and `go build ./...` pass.
- No regressions in existing rail tests; new tests cover dismissal repo + service.

## Open Questions

- Should we expose a "Restore dismissed" UI? Probably yes as a small footer link, with a 30-day server-side retention window on the dismissals table. Defer to follow-up if we want to ship the persistence first.
- Should standalone runs respect plan-level dismissal (i.e. dismissing a plan also hides its runs)? Yes — `listRecentRuns` should exclude runs whose `parent_run_id`'s plan is in the dismissals table. Implement with a join in the recent-runs query rather than a follow-up filter on the frontend.

## Implementation Order

1. Backend: `agent_runs/recent` endpoint (handler + service + repo + tests).
2. Frontend: rehydrate standalone runs on mount (consume new endpoint).
3. Backend: `command_bar_plan_dismissals` table, repo, list-filter, dismiss endpoints.
4. Frontend: wire "Clear all" + per-card "Hide" to dismiss endpoint, optimistic UI.
5. Frontend: `zustand/persist` for `railMode` + `filter`.
6. QA pass against the acceptance criteria.
