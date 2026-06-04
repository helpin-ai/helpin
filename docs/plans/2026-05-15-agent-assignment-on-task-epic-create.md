# Agent assignment on task/epic create

**Date:** 2026-05-15
**Branch:** develop
**Status:** Plan — pending approval

## Goal

Make AI agent assignment a first-class option when creating or editing tasks/epics, especially for new users who may not know agents can refine and extend stories using connected repository context. Adds a visually distinct "AGENT" card to the properties sidebar; footer CTA flips from `Create` to `Create & run agent` when an agent is attached.

## Scope (MVP — no LLM suggestion)

- Manual agent picker only. No auto-suggestion, no plan preview, no embeddings/LLM calls.
- No new permission model. Use the same PM edit/run authorization already protecting the PM agent-run routes.
- Repo-aware helper copy only: when a planning repository is connected, tell users the agent can use code context; when none is connected, keep assignment available but mention repo context requires connecting a repo. Do not add repo connection UI in this PR.
- Applies to: PM Task create modal, PM Task detail view, PM Epic create modal, PM Epic detail view.
- Footer CTA flips label + behavior based on whether an agent is attached.
- `×` on the agent chip detaches → button reverts to plain `Create`.
- Persistent assignment: a task/epic can have an assigned agent without running it; the detail view shows a `Run agent` button to trigger later.

## Current state (origin/develop)

**Backend — already in place**
- `PMTask.AssignedAgentID *string` exists (`server/internal/model/pm_task.go`)
- `Agent` + `AgentRun` models (`server/internal/model/agent.go`)
- `AgentRun.TargetType`/`TargetID` — generic launch contract
- Routes:
  - `POST /pm/agent-runs` — generic `StartTargetRun`
  - `POST /pm/tasks/{id}/run-agent` — task shortcut
  - `POST /pm/epics/{id}/run-agent` — epic shortcut

**Frontend — already in place**
- `agentService` in `frontend/src/lib/services/agentService.ts` (`list`, `runTask`, etc.)
- `useAgents` hook in `frontend/src/hooks/queries/useAgents.ts`
- `TaskDetailPanel` properties sidebar with `<MetadataRow>` pattern (`frontend/src/components/pm/TaskDetailPanel.tsx`)
- Existing detail-run surfaces:
  - `AgentRunPanel` already handles task run listing, runnable-agent filtering, active-run blocking, URL run selection, and `agent_run-created/updated` refresh (`frontend/src/components/pm/AgentRunPanel.tsx`).
  - `EpicPlannerPanel` already handles epic run listing, team-scoped planner filtering, active-run blocking, run drawer opening, and `agent_run-created/updated` refresh (`frontend/src/components/pm/EpicPlannerPanel.tsx`).
  - Both detail surfaces use frontend `canEdit`; backend run routes use `PermPMEdit` (`server/internal/router/router.go:959-961`). Reuse this permission model.

**Gaps to close**
1. `PMEpic` is missing `AssignedAgentID` — needs field + migration.
2. `CreateTaskRequest`, `UpdateTaskRequest`, `CreateEpicRequest`, `UpdateEpicRequest` DTOs need `assigned_agent_id`.
3. No reusable agent picker UI component.
4. `CreateTaskModal` has no properties sidebar yet — need to decide between (a) adding the agent card as a new form section, or (b) introducing a properties column.
5. Repository + service layer must persist `assigned_agent_id` and (optionally) auto-launch a run when set on create.

## Decisions (locked after plan review 2026-05-15)

### D1. Create-and-run semantics
Backend handles the combined action, but **truly atomic is not achievable** — starting an agent run has side effects (Temporal workflow, events, agent_run row) that can't be rolled back if the outer DB transaction fails. The honest contract is:

1. **Prevalidate before any write:** confirm the agent exists, belongs to the same workspace, is allowed to run on the target type/team, and the caller has the same PM edit/run authorization already required by the existing PM run-agent routes. Reject the whole request with 400/403 if any check fails — no partial state.
2. **Create task/epic** in a DB transaction.
3. **Start the run** after the commit succeeds. If run start fails *after* task creation (rare — should only be Temporal/infra outage), return `201` with the task plus `agent_run: null` and an `agent_run_error: "<message>"` field. Frontend shows the task as created and surfaces a non-blocking toast: *"Task created, but agent didn't start — try again from the detail view."*
4. Run start failures **must not** roll back the task — the user's writes are precious.

This is "prevalidated + best-effort run" rather than strict atomicity. It matches what's safely achievable and removes the common failure modes (bad agent id, wrong workspace, missing permission) from the partial-state surface area.

### D2. Modal layout — REVERSED
`CreateTaskModal` already has a two-column layout with a right-side properties column (`frontend/src/components/pm/CreateTaskModal.tsx:1125`, `:1188`). `CreateEpicModal` (rendered from `GlobalCreateModals.tsx:395`, `:583`) also has a right sidebar.

**Decision: drop the AGENT card into the existing right-hand properties column** of both modals. No new layout work. The card's elevated styling (tinted bg, rounded border, ✨ header) still makes it visually distinct from the surrounding metadata rows.

### D3. Epic detail surfaces — CONFIRMED, BOTH GET THE CARD
There is no `EpicDetailPanel`. Epic detail lives in two places:
- `frontend/src/components/pm/GlobalEpicPanel.tsx` — drawer surface
- `frontend/src/pages/pm/EpicDetail.tsx` — full page

Both surfaces get the AGENT card + `Run agent` button. The card must be implemented as a reusable component so it lives in exactly one place.

## Implementation plan

### Backend
1. **Migration** — `server/internal/dbmigrate/sql/YYYYMMDDNNNN_pm_epics_assigned_agent.sql`
   - `ALTER TABLE pm_epics ADD COLUMN IF NOT EXISTS assigned_agent_id uuid;`
   - Add index.
2. **Model** — `server/internal/model/pm_epic.go`
   - Add `AssignedAgentID *string` (GORM auto-migrate will pick this up too).
3. **DTOs** — same files
   - `CreateTaskRequest`, `UpdateTaskRequest` (`pm_task.go`): add `AssignedAgentID *string \`json:"assigned_agent_id,omitempty"\``
   - `CreateEpicRequest`, `UpdateEpicRequest` (`pm_epic.go`): same
   - On create, add optional `RunOnCreate bool \`json:"run_on_create,omitempty"\``.
4. **Service** — `internal/service/pm_task.go`, `pm_epic.go`
   - **Prevalidate** when `assigned_agent_id != nil`: use a small exported helper on `AgentService` that wraps the existing `requireRunnableAgent` + `validateAgentTeamScope` logic for `"task"` / `"epic"`. Do not duplicate the validation logic in PM services.
   - Do not introduce a new `agents.run` permission for this MVP; current routes use `PermPMEdit`, and task/epic create already require PM edit permissions. Adding a new permission would require RBAC, route, and frontend permission-surface work outside this goal.
   - Persist `assigned_agent_id` on create/update inside a DB transaction.
   - After commit, if `run_on_create`, call existing `StartTargetRun(ctx, TargetType="task"|"epic", TargetID=newID, AgentID=*assignedID, ...)`. **Use `"task"` / `"epic"`** — those are the canonical values (`server/internal/service/agent.go:2295`, `:2354`). Do NOT use `pm_task`/`pm_epic`.
   - If `StartTargetRun` fails, log + return the task with `agent_run: nil` and `agent_run_error: "<message>"`. Do not roll back the task.
5. **DI wiring** — `cmd/api/main.go`
   - `PMTaskService` already holds an `AgentService` dep (`server/internal/service/pm_task.go:38`).
   - `PMEpicService` currently does NOT (`server/internal/service/pm_epic.go:18`). Add an `agentService *AgentService` field and constructor parameter; thread it through `cmd/api/main.go`.
6. **Repository** — pass the new column through `Create`/`Update` updates maps.
7. **Response DTOs** — explicit shapes, not implicit shape drift.
   - `CreateTaskResponse { task: TaskDetail; agent_run: AgentRun | null; agent_run_error?: string }`
   - `CreateEpicResponse { epic: EpicWithStats; agent_run: AgentRun | null; agent_run_error?: string }`
   - Always return the wrapped shape from create handlers and update frontend service/callers in this PR. Avoid branching response shapes by request mode; conditional response contracts are easy to mishandle.

### Frontend
1. **New component** — `frontend/src/components/pm/AgentPickerCard.tsx`
   - Card with `bg-primary/5` tint, rounded border, `✨ AGENT` header.
   - Two states: empty (`+ Assign an agent ▾` button + helper line) and attached (icon + name + `×` detach).
   - Helper copy is informational, not a recommendation engine:
     - repo connected: "Agents can use your connected repo to refine and extend this story."
     - no repo connected: "Agents can help refine this story. Connect a repo to give them code context."
   - Picker is a popover listing agents from `useAgents()`. Each row: icon, name, one-liner.
   - Props: `value`, `onChange`, `workspaceId`, optional `hasRepoContext`, optional `disabled`, optional `runnableTarget`.
   - Reuse the same runnable-target filtering rules as the existing detail panels:
     - task agents: `allowed_targets` includes `"task"`
     - epic agents: `allowed_targets` includes `"epic"` and team-scoped agents must match an accessible team
   - Default selection helper should be deterministic and shared:
     - for epics, prefer system `preset_key === "epic_planner"` (display name should be Atlas), then any `epic_planner`, then first runnable epic agent
     - for tasks/stories, prefer system `preset_key === "task_planner"` (display name should be Scribe), then any `task_planner`, then first runnable task agent
     - use display-name fallback (`Atlas` / `Scribe`) only if preset keys are missing in legacy data
   - This component is an assignment picker, not a run-history panel. Do not duplicate run timelines, drawers, additional-context controls, or active-run status UI from `AgentRunPanel` / `EpicPlannerPanel`.
2. **Service** — `frontend/src/lib/services/pmTaskService.ts`, `pmEpicService.ts`
   - Extend `create` to accept `assigned_agent_id` + `run_on_create`.
   - Update response type to `CreateTaskResponse` / `CreateEpicResponse` (wrapped shape with `agent_run` + optional `agent_run_error`).
   - Update existing callers (today they expect `TaskDetail` / `EpicWithStats` directly — `frontend/src/lib/services/pmTaskService.ts:38`, `pmEpicService.ts:38`). Either return the wrapped shape always and migrate callers, or branch on `run_on_create`. Pick one and apply consistently.
3. **CreateTaskModal** (`frontend/src/components/pm/CreateTaskModal.tsx`)
   - Add local state `assignedAgentId`.
   - Initialize the picker to Scribe by default using the shared task default-selection helper (`task_planner`, system first). This makes the `Create & run agent` path visible immediately for new users, while still allowing detach.
   - Render `<AgentPickerCard>` inside the existing **right-hand properties column** (lines `:1125`, `:1188`) alongside Type/Status/Owner/Team. Card sits at the bottom of the column.
   - Footer: button label = `assignedAgentId ? "✨ Create & run agent" : "Create"`.
   - Submit: pass `assigned_agent_id` + `run_on_create: !!assignedAgentId` to `onCreate`.
   - On response: if `agent_run` is present, toast "Agent started"; if `agent_run_error` is present, toast non-blocking warning.
4. **CreateEpicModal** (rendered from `frontend/src/components/pm/GlobalCreateModals.tsx:395`, `:583`)
   - Same changes; epic create already has a right sidebar — drop the card into it.
   - Initialize the picker to Atlas by default using the shared epic default-selection helper (`epic_planner`, system first). This mirrors `EpicPlannerPanel`'s current preferred planner selection.
5. **TaskDetailPanel** (`frontend/src/components/pm/TaskDetailPanel.tsx`)
   - Add `<AgentPickerCard>` in the sidebar block. Field updates auto-save through the existing `updateField()` path.
   - When an agent is assigned, show a `Run agent` button that fires `POST /pm/tasks/{id}/run-agent`.
   - Reuse `AgentRunPanel` behavior where possible instead of inventing parallel run state:
     - use `agentService.listTargetRuns(workspaceId, "task", taskId)` if the assignment card needs to know whether an active run exists
     - disable while an active run exists or while the start request is in flight
     - refresh/listen using the same `agent_run-created` / `agent_run-updated` event pattern
   - Use the existing `canEdit` flag for assign/run affordances. Read-only users can see assigned agent state but cannot change or run it.
6. **Epic detail surfaces — BOTH:**
   - `frontend/src/components/pm/GlobalEpicPanel.tsx` (drawer)
   - `frontend/src/pages/pm/EpicDetail.tsx` (full page)
   - Add the same `<AgentPickerCard>` + `Run agent` button to both.
   - Reuse `EpicPlannerPanel` behavior where possible instead of inventing parallel run state:
     - use `agentService.listTargetRuns(workspaceId, "epic", epicId)` if the assignment card needs active-run awareness
     - disable while an active run exists or while the start request is in flight
     - refresh/listen using the same `agent_run-created` / `agent_run-updated` event pattern
   - Use the existing `canEdit` flag for assign/run affordances. Read-only users can see assigned agent state but cannot change or run it.
   - Share state via the existing epic reload/query path so updates in one surface reflect in the other.
7. **Toast on launch** — `sonner` toast "Agent started" with link to the run. Non-blocking warning toast on `agent_run_error`.

### Tests
- Backend: service tests for `Create*` with `assigned_agent_id` + `run_on_create` covering: agent doesn't exist (400), happy path returns run, agent exists but workspace mismatch (403).
- Frontend: vitest for `AgentPickerCard` empty/attached/detach interactions.

## Out of scope (deferred)
- LLM-based agent suggestion + plan preview.
- Per-agent example prompts shown in the picker.
- Sidebar nav "Agents" gallery (separate effort).
- Confidence-based default selection.

## Files touched (estimated)

**Backend (8)**
- `server/internal/dbmigrate/sql/<new>_pm_epics_assigned_agent.sql`
- `server/internal/model/pm_epic.go`
- `server/internal/model/pm_task.go`
- `server/internal/service/pm_task.go`
- `server/internal/service/pm_epic.go` — also adds `agentService` field + constructor param
- `server/internal/repository/pm_task.go`
- `server/internal/repository/pm_epic.go`
- `server/cmd/api/main.go` — DI wiring for `PMEpicService` to receive `AgentService`
- `server/internal/handler/pm_task.go`, `pm_epic.go` — wrapped response shape

**Frontend (7)**
- `frontend/src/components/pm/AgentPickerCard.tsx` (new)
- `frontend/src/components/pm/CreateTaskModal.tsx`
- `frontend/src/components/pm/GlobalCreateModals.tsx` (epic create flow)
- `frontend/src/components/pm/TaskDetailPanel.tsx`
- `frontend/src/components/pm/GlobalEpicPanel.tsx`
- `frontend/src/pages/pm/EpicDetail.tsx`
- `frontend/src/lib/services/pmTaskService.ts`, `pmEpicService.ts`
- `frontend/src/lib/pmTypes.ts` (DTO field additions + response types)

## Risk / things to confirm before coding
- ~~Whether `EpicDetailPanel` exists~~ — confirmed it doesn't; use `GlobalEpicPanel` + `pages/pm/EpicDetail.tsx`.
- ~~Whether `StartTargetRun` accepts target types for tasks/epics~~ — confirmed: canonical values are `"task"` and `"epic"` (`server/internal/service/agent.go:2295`, `:2354`).
- ~~Permission required to start an agent run~~ — there is no `agents.run` permission in `internal/authorization/permissions.go`; use the existing PM edit authorization for this MVP.
- Whether `AgentService` exposes a "load + validate runnable" helper or if one must be added. Add one small helper on `AgentService`; do not inline validation in both PM services.
