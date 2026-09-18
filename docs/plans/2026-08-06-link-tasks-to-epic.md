# Link tasks to an epic implementation plan

> Historical implementation plan, source-compared on 2026-09-17. Batch epic
> linking is implemented. This record is for contributors tracing the change;
> unchecked tasks and the branch/test commands below are not current work orders.

## Current implementation

The [dialog](../../frontend/src/components/pm/LinkTasksToEpicDialog.tsx) searches
same-team, non-archived tasks in pages of 50, excludes the target epic's tasks,
preserves selections on API errors, and asks for review before moving tasks.
[EpicDetail](../../frontend/src/pages/pm/EpicDetail.tsx) supplies team context and
handles success. Review is client-side; the API does not require a review token.

The [handler](../../server/internal/handler/pm_epic.go) validates the request,
and the [route](../../server/internal/router/router.go) requires `pm.edit`.
The [service](../../server/internal/service/pm_epic.go) checks workspace, active
epic, team/edit access, task existence, archive status, and same-team membership.
Task membership writes are transactional. Delivery-target inheritance, activity
logging, and websocket publication happen after commit, so their failures do not
roll back successful links. This is not an atomic guarantee across all side effects.

See the [reviewed design](../specs/2026-08-06-link-tasks-to-epic-design.md) for
manual-target preservation and the whole-model-save concurrency limitation.
The old Go version, npm build command, branch name, and verification checklist
remain historical; no feature test run or deployment is claimed by this review.

## Original implementation record

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a team-safe, atomic multi-select workflow for linking or moving existing tasks into an epic.

**Architecture:** Reuse the existing PM task list for team-scoped candidate discovery and add one epic-owned transactional link endpoint. Keep the UI in a focused dialog component, with `EpicDetail` coordinating open/refresh behavior only.

**Tech Stack:** Go 1.24, Chi, GORM, React 19, TypeScript, Vitest, existing shadcn dialog/input/checkbox primitives.

---

### Task 1: Candidate task discovery

**Files:**
- Modify: `server/internal/handler/pm_task.go`
- Modify: `frontend/src/lib/services/pmTaskService.ts`
- Test: `server/internal/handler/pm_task_test.go` or nearest PM task handler test

- [ ] Write a failing test proving `GET /pm/tasks?search=HLP-42&team_id=team-a` forwards both filters.
- [ ] Run the focused handler test and confirm the search assertion fails.
- [ ] Populate `PMTaskFilters.Search` from the `search` query parameter.
- [ ] Add `search` to the frontend task-list filter type.
- [ ] Run focused Go tests and confirm they pass.

### Task 2: Atomic epic task linking service

**Files:**
- Modify: `server/internal/model/pm_epic.go`
- Modify: `server/internal/repository/pm_task.go`
- Modify: `server/internal/service/pm_epic.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/service/pm_epic_test.go`

- [ ] Add failing tests for same-team linking, same-team moving, no-team epic rejection, archived-task rejection, mixed-team atomic rollback, missing-task atomic rollback, and team-access enforcement.
- [ ] Run the focused service tests and confirm they fail because the operation does not exist.
- [ ] Add bounded request/response DTOs with at most 100 task IDs.
- [ ] Add a repository transaction callback/path that loads and updates all selected tasks on one GORM transaction.
- [ ] Implement `PMEpicService.LinkTasks` with deduplication, edit/access checks, exact team validation, and no-op handling for tasks already in the epic.
- [ ] Wire the existing Git service into `PMEpicService` so post-commit delivery-target inheritance matches single-task updates while preserving manual targets.
- [ ] Log epic-link activity and publish task updates after commit.
- [ ] Run focused service tests and confirm they pass.

### Task 3: HTTP endpoint

**Files:**
- Modify: `server/internal/handler/pm_epic.go`
- Modify: `server/internal/router/router.go`
- Test: `server/internal/handler/pm_epic_test.go` or service-backed handler test

- [ ] Add a failing endpoint test for invalid/empty/oversized payloads and a successful request.
- [ ] Run it and confirm the endpoint is unavailable.
- [ ] Add `POST /pm/epics/{id}/tasks/link` behind `pm.edit`.
- [ ] Decode strictly, call `PMEpicService.LinkTasks`, and return the compact result.
- [ ] Run focused handler/router tests and confirm they pass.

### Task 4: Frontend service and dialog behavior

**Files:**
- Create: `frontend/src/components/pm/LinkTasksToEpicDialog.tsx`
- Create: `frontend/src/components/pm/__tests__/LinkTasksToEpicDialog.test.tsx`
- Modify: `frontend/src/lib/pm-types/project.ts`
- Modify: `frontend/src/lib/services/pmEpicService.ts`

- [ ] Write failing component tests for team field rendering, exclusion of the current epic, multi-select, grouping, direct linking, and review-before-moving.
- [ ] Run the focused Vitest file and confirm the missing component failure.
- [ ] Add frontend request/response types and `pmEpicService.linkTasks`.
- [ ] Build the dialog with deferred server search, pagination, persistent selections, table-like rows, and a compact review state.
- [ ] Keep cross-team filtering in the request and render the epic team in every row.
- [ ] Preserve selections on API errors and show concise inline error feedback.
- [ ] Run the focused test and confirm it passes.

### Task 5: Epic detail integration

**Files:**
- Modify: `frontend/src/pages/pm/EpicDetail.tsx`
- Test: `frontend/src/pages/pm/__tests__/EpicDetail.linkTasks.test.tsx` or extracted integration helper test

- [ ] Add a failing test that the Tasks heading exposes **Link tasks**, empty state separates link/create actions, and no-team epics disable linking with guidance.
- [ ] Run it and confirm it fails.
- [ ] Add dialog open state and render **Link tasks** beside **Create task**.
- [ ] Replace the empty-state ghost action with distinct link/create actions.
- [ ] On success, refresh the epic detail tasks and delivery context and show a toast.
- [ ] Run focused component tests and confirm they pass.

### Task 6: Verification and commit

**Files:**
- Review all modified files

- [ ] Run `go test ./internal/service ./internal/handler ./internal/router -count=1` from `server`.
- [ ] Run `go test ./... -count=1` from `server`.
- [ ] Run focused Vitest tests for the new dialog and epic detail integration.
- [ ] Run `npm run build` from `frontend`.
- [ ] Run `git diff --check` and confirm the worktree contains only intended changes.
- [ ] Commit the completed feature on `waqar-fixes`.
