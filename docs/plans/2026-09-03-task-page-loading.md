# Task Page Loading Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce task board and list cold-load payloads and request latency without changing filtering, ordering, grouping, pagination, drag-and-drop, or task-detail behavior.

**Architecture:** Task collection queries will project summary fields and exclude rich task descriptions and implementation briefs before enrichment and serialization. The tasks screen will use one workflow source, resolve an existing team workflow locally with a server fallback, use shared React Query reference hooks, and apply compression only to task collection GET routes.

**Tech Stack:** Go 1.24, Chi middleware, GORM, SQLite repository tests, React 19, TypeScript, Zustand, TanStack Query, Vitest.

---

### Task 1: Lightweight task collection read models

**Files:**
- Modify: `server/internal/model/pm_task.go`
- Modify: `server/internal/repository/pm_task.go`
- Test: `server/internal/repository/pm_task_member_board_test.go`

- [x] **Step 1: Write failing repository tests**

Seed tasks with large `description` and non-empty `implementation_brief`, then verify `List`, `ListByWorkflowState`, `ListColumnTasks`, `ListByMember`, and `ListMemberColumnTasks` return both fields empty while retaining IDs, names, owners, labels, state data, totals, and ordering.

- [x] **Step 2: Run the focused repository tests and verify RED**

Run: `go test ./internal/repository -run 'TaskSummary|MemberBoard' -count=1`

Expected: summary assertions fail because collection queries currently return the full task fields.

- [x] **Step 3: Implement the summary projection**

Add a repository scope that omits `description` and `implementation_brief` from board queries and a dedicated `ListSummary` path for the HTTP task collection. Keep the existing full `List` path for internal CRM summaries and agent excerpts. Remove explicit `pm_tasks.*` selections that would defeat the projection. Change the description JSON tag to `json:"description,omitempty"` so collection payloads omit the empty property; full detail responses retain populated descriptions.

- [x] **Step 4: Run focused tests and verify GREEN**

Run: `go test ./internal/repository -run 'TaskSummary|MemberBoard' -count=1`

Expected: PASS.

### Task 2: Task collection response compression

**Files:**
- Create: `server/internal/middleware/compress_json.go`
- Create: `server/internal/middleware/compress_json_test.go`
- Modify: `server/internal/router/router.go`

- [x] **Step 1: Write a failing middleware test**

Wrap an `application/json` handler, request gzip, and assert the response declares gzip and decompresses to the original JSON. Also assert a request without gzip receives the original uncompressed response.

- [x] **Step 2: Run the test and verify RED**

Run: `go test ./internal/middleware -run CompressJSON -count=1`

Expected: build failure because `CompressJSON` does not exist.

- [x] **Step 3: Implement scoped JSON compression**

Expose `middleware.CompressJSON` as a small wrapper around Chi's compressor. Apply it only to `GET /pm/tasks`, `/pm/tasks/board`, `/pm/tasks/board/column`, `/pm/tasks/board/members`, and `/pm/tasks/board/members/column`; do not wrap websocket, streaming, write, or unrelated routes.

- [x] **Step 4: Run middleware and router build verification**

Run: `go test ./internal/middleware ./internal/router -count=1`

Expected: PASS.

### Task 3: Deduplicate workflow discovery

**Files:**
- Modify: `frontend/src/stores/pmBoardStore.ts`
- Modify: `frontend/src/components/pm/KanbanBoard.tsx`
- Test: `frontend/src/stores/__tests__/pmBoardStore.loading.test.ts`

- [x] **Step 1: Write failing store tests**

Verify `loadBoard` and `setTeamFilter` choose a workflow whose `team_id` matches the selected team without calling `resolveTeamWorkflow`. Verify they call `resolveTeamWorkflow` when the fetched workflow list has no matching team workflow.

- [x] **Step 2: Run the focused Vitest file and verify RED**

Run: `npm test -- --run src/stores/__tests__/pmBoardStore.loading.test.ts`

Expected: the existing-team test fails because the store always calls the resolver.

- [x] **Step 3: Implement local selection with fallback**

Add a deterministic team-workflow selector used by both `loadBoard` and `setTeamFilter`. Keep the resolver only as the missing-workflow fallback. Read `workflows` from the board store in `KanbanBoard` and remove the parallel `useWorkflows` query.

- [x] **Step 4: Run focused tests and verify GREEN**

Run: `npm test -- --run src/stores/__tests__/pmBoardStore.loading.test.ts`

Expected: PASS.

### Task 4: Share label, epic, and sprint query caches

**Files:**
- Modify: `frontend/src/components/pm/KanbanBoard.tsx`
- Test: `frontend/src/components/pm/__tests__/KanbanBoard.loadingSources.test.ts`

- [x] **Step 1: Write a failing source-contract test**

Assert `KanbanBoard` obtains labels, epics, and sprints from `useLabels`, `useEpics`, and `useSprints`, and no longer directly invokes their services from a mount effect.

- [x] **Step 2: Run the test and verify RED**

Run: `npm test -- --run src/components/pm/__tests__/KanbanBoard.loadingSources.test.ts`

Expected: FAIL because the component still imports and invokes the three services.

- [x] **Step 3: Replace manual fetching with shared hooks**

Use default empty arrays from the established hooks. Preserve the existing filter provider and task-list props, so reference data may populate progressively without blocking board rendering.

- [x] **Step 4: Run the test and verify GREEN**

Run: `npm test -- --run src/components/pm/__tests__/KanbanBoard.loadingSources.test.ts`

Expected: PASS.

### Task 5: Full regression verification

**Files:**
- Modify only if verification identifies a regression.

- [x] **Step 1: Run backend tests**

Run: `go test ./...`

Expected: PASS.

- [x] **Step 2: Run frontend tests**

Run: `npm test -- --run`

Expected: PASS.

- [x] **Step 3: Run production builds**

Run: `go build ./...` from `server`, then `npm run build` from `frontend`.

Expected: PASS.

- [x] **Step 4: Inspect the final patch**

Run: `git diff --check`, `git status --short`, and inspect the diff to confirm only planned files changed and task-detail responses still return full descriptions.
