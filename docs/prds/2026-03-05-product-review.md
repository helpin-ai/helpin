# Product review and remediation

**Product**: Helpin  
**Review date**: March 5, 2026  
**Reviewer**: Codex  
**Scope**: Current repository state in `/root/helpin`, including frontend, backend, PM module, and shared auth/workspace infrastructure

## 1. Executive Summary

Helpin has a substantial amount of product-management functionality implemented across both frontend and backend. Core CRUD surfaces exist for workflows, labels, stories, epics, sprints, objectives, comments, attachments, saved views, checklist items, external links, and automations. The reward/quarter/bonus platform is also present and compiles on the backend.

The current codebase is **not release-ready** for the latest PM feature set. The largest problems are:

1. **Critical backend data loss risk**: server startup drops `pm_epic_objectives` on every boot.
2. **Critical authorization gap**: PM/search routes trust client-supplied `workspace_id` and several repositories fetch entities by raw ID without workspace membership enforcement.
3. **Frontend build is broken**: `npm run build` fails, including route/type mismatches in login, search, WebSocket, and story detail code.
4. **Feature completeness is uneven**: Docs, Roadmap, and Reports are linked routes but still placeholder pages.
5. **Quality gates are weak**: backend has no tests, frontend has no test script, and lint currently reports a large error backlog.

## 2. Review Method

This review used:

- Repository structure inspection across `frontend/`, `server/`, and `docs/`
- API surface inspection through [router.go](../../server/internal/router/router.go)
- Frontend and backend source review of major implemented PM features
- Validation runs on the current worktree:
  - `go test ./...`
  - `npm run build`
  - `npm run lint`
  - `npm test`

Note: the worktree is currently dirty in `frontend/src/pages/Settings.tsx` (historical path; absent from this checkout), [pm_automation.go](../../server/internal/service/pm_automation.go), and `docs/mattermost-integration.md`. Findings below describe the current state on disk.

## 3. Validation Results

### Backend

- `go test ./...`: passes, but every package reports `[no test files]`
- Result: backend compiles, but there is effectively no automated regression coverage

### Frontend

- `npm run build`: fails
- `npm run lint`: fails with 85 errors and 6 warnings
- `npm test`: fails because no `test` script exists in [frontend/package.json](../../frontend/package.json)

## 4. Implemented Feature Inventory

### 4.1 Shared Platform

| Area | Status | Notes |
|------|--------|-------|
| Auth | Implemented | JWT auth exists; authorization is insufficient for workspace-scoped PM APIs |
| Workspaces | Implemented | Slug-based routing and membership endpoints exist |
| Search | Implemented | Backend search exists; frontend command palette currently breaks build |
| WebSocket sync | Implemented | Event transport exists; frontend hook currently fails type-checking |

### 4.2 PM Backend

| Feature | Status | Notes |
|--------|--------|-------|
| Workflows and states | Implemented | CRUD routed and service/repo layers present |
| Labels | Implemented | CRUD plus stats |
| Stories | Implemented | CRUD, board view, move, reorder, owners/followers/labels, activity |
| Epics | Implemented | CRUD, story listing, health updates |
| Sprints | Implemented | CRUD, stories, overlap rules, automation hooks |
| Objectives and key results | Implemented | CRUD plus links to teams, owners, epics |
| Comments | Implemented | Entity comments supported |
| Attachments | Implemented | Presigned upload flow exists |
| Checklist items | Implemented | CRUD exists |
| External links | Implemented | CRUD exists |
| Views | Implemented | Saved/shared views exist |
| Automations | Implemented | Epic and sprint automations exist |

### 4.3 PM Frontend

| Feature | Status | Notes |
|--------|--------|-------|
| Stories board/list/detail | Implemented with defects | Core UI exists; build/type/runtime issues remain |
| Epics list/detail | Implemented | Usable surface, but part of the failing lint/build baseline |
| Sprints list/detail | Implemented | Usable surface, same quality concerns |
| Objectives list/detail | Implemented | Substantial UI exists, but part of failing lint/build baseline |
| Labels UI | Implemented | Present, also contributes to lint/type errors |
| Saved views | Implemented | Board store supports views |
| Docs | Placeholder | Route exists but only shows “Coming Soon” |
| Roadmap | Placeholder | Route exists but only shows “Coming Soon” |
| Reports | Placeholder | Route exists but only shows “Coming Soon” |

## 5. Priority Findings

### P0. Server startup drops objective/epic relationship data on every boot

**Severity**: Critical  
**Area**: Backend, data integrity

The API entrypoint unconditionally executes `DROP TABLE IF EXISTS pm_epic_objectives` before `AutoMigrate`, which recreates the same table name afterward. That means all objective-to-epic links are destroyed whenever the server restarts.

- Evidence: [main.go](../../server/cmd/api/main.go)
- Supporting model: [pm_epic.go](../../server/internal/model/pm_epic.go)

**Impact**

- Every restart can silently wipe linked planning data
- Objective progress becomes incorrect after restart
- Users lose roadmap/planning relationships with no recovery path in-app

**Requirement**

- Remove the destructive startup DDL
- Replace with an explicit one-time migration if schema repair is actually needed
- Add migration/backfill verification before next deploy

### P0. Workspace authorization is missing across PM and search APIs

**Severity**: Critical  
**Area**: Backend, security

PM and search routes only require a `workspace_id` header/query parameter, but they do not verify that the authenticated user belongs to that workspace. Several repositories also fetch records by raw entity ID without scoping to workspace, which makes cross-workspace access easier once an ID is known.

- Routes only require workspace ID: [router.go](../../server/internal/router/router.go), [router.go](../../server/internal/router/router.go)
- Middleware only copies `workspace_id` into context: [workspace.go](../../server/internal/middleware/workspace.go)
- Story fetches are by raw ID: `server/internal/repository/pm_story.go` (historical path; absent from this checkout), `server/internal/repository/pm_story.go` (historical path; absent from this checkout)
- Attachment fetch/list also ignore workspace scoping: [pm_attachment.go](../../server/internal/repository/pm_attachment.go), [pm_attachment.go](../../server/internal/repository/pm_attachment.go)

**Impact**

- Any authenticated user can probe other workspaces by supplying another `workspace_id`
- Direct object access by UUID can cross tenant boundaries
- Attachment and PM entity metadata can leak between workspaces

**Requirement**

- Add workspace-membership authorization middleware for workspace-scoped routes
- Scope all repository reads/updates/deletes by `workspace_id`
- Reject any mismatch between path/query workspace and entity workspace

### P1. Frontend is not in a shippable state because the build currently fails

**Severity**: High  
**Area**: Frontend, release readiness

The frontend does not pass TypeScript build checks. The current failure set includes invalid route targets, incorrect router search params, incorrect service call signatures, and incorrect `useRef` initialization.

Representative failures:

- Invalid post-login route target: [Login.tsx](../../frontend/src/pages/Login.tsx)
- Search command palette writes unsupported search params and uses uninitialized refs: [SearchCommandPalette.tsx](../../frontend/src/components/search/SearchCommandPalette.tsx), [SearchCommandPalette.tsx](../../frontend/src/components/search/SearchCommandPalette.tsx)
- Story detail calls comment service with the wrong arguments: `frontend/src/pages/pm/StoryDetail.tsx` (historical path; absent from this checkout), [pmCommentService.ts](../../frontend/src/lib/services/pmCommentService.ts)
- WebSocket hook has invalid `useRef` initialization under current TS settings: [useWebSocket.ts](../../frontend/src/hooks/useWebSocket.ts)

**Impact**

- Production build cannot complete
- Route safety is being bypassed in places
- Real-time refresh logic is partially broken even before runtime QA

**Requirement**

- Make `npm run build` green before adding more PM UI surface
- Remove `as any` route escapes where possible
- Add CI gating on `npm run build`

### P1. Docs, Roadmap, and Reports are exposed in navigation but are still placeholders

**Severity**: High  
**Area**: Product completeness

These routes are present but only render “Coming Soon” screens:

- Reports: [reports.tsx](../../frontend/src/routes/_authenticated/w/$slug/pm/reports.tsx)
- Roadmap: [roadmap.tsx](../../frontend/src/routes/_authenticated/w/$slug/pm/roadmap.tsx)
- Docs: [docs.tsx](../../frontend/src/routes/_authenticated/w/$slug/docs.tsx)

**Impact**

- Product appears broader than it actually is
- Users can navigate to unfinished surfaces
- PRD expectations and shipped UX are misaligned

**Requirement**

- Either hide these routes behind feature flags/navigation guards
- Or finish MVP implementations before calling them part of the released PM module

### P1. Quality gates are too weak to catch regressions

**Severity**: High  
**Area**: Engineering process

- Frontend has no `test` script: [frontend/package.json](../../frontend/package.json)
- Backend has no test packages in `go test ./...`
- Lint baseline is failing across core PM components and pages

**Impact**

- Regressions land unnoticed
- Refactors are unsafe
- Runtime bug fixing becomes reactive instead of preventative

**Requirement**

- Add at least smoke tests for auth, workspace access, stories, epics, sprints, and objectives
- Add frontend component/integration coverage for board navigation, search, and detail views
- Make lint/build/test mandatory in CI

### P2. Story display ID allocation is race-prone under concurrent create load

**Severity**: Medium  
**Area**: Backend, data integrity

Story creation computes `MAX(display_id) + 1` inside an application transaction rather than relying on a database sequence or row lock. Under concurrent requests for the same workspace, two inserts can calculate the same next ID and collide with the unique constraint.

- Evidence: `server/internal/repository/pm_story.go` (historical path; absent from this checkout)
- Supporting schema uniqueness: [013_pm_stories.sql](../../server/migrations/013_pm_stories.sql)

**Impact**

- Intermittent create failures under concurrent use
- Hard-to-reproduce bug during imports, bulk create, or active team usage

**Requirement**

- Move to a DB-managed per-workspace allocation strategy
- Or use a locking strategy that guarantees uniqueness without retries

## 6. Secondary Findings

### 6.1 Frontend lint backlog is signaling real behavioral risks

The lint output is not just stylistic debt. It contains:

- sync `setState` in effects
- refs being mutated during render
- missing dependencies in hooks
- incompatible memoization patterns

Those patterns are concentrated in board, story-detail, objective, label, attachment, settings, and sidebar code. They increase the likelihood of stale UI, double renders, incorrect real-time refresh, and hard-to-debug state drift.

### 6.2 Search UX is partially implemented but not production-ready

The backend search service is structurally fine, but the frontend command palette is not ready. It currently breaks type-checking and uses a route search param that does not exist on the story listing route.

- Backend service: [search.go](../../server/internal/service/search.go)
- Frontend issue: [SearchCommandPalette.tsx](../../frontend/src/components/search/SearchCommandPalette.tsx)

### 6.3 Startup schema strategy is operationally risky

The backend relies on `AutoMigrate` during server boot for primary schema management instead of using the SQL migrations directory as the source of truth. The comment in [main.go](../../server/cmd/api/main.go) explicitly says the migration files are now “reference documentation.”

That approach reduces migration discipline, obscures schema drift, and makes destructive changes easier to ship accidentally.

## 7. Frontend PRD: Remediation Requirements

### 7.1 Release Gate

The PM frontend is releasable only when:

- `npm run build` passes
- `npm run lint` passes or is reduced to an agreed warning-only baseline
- Search, login redirect, story detail, objective detail, and sidebar interactions are validated manually

### 7.2 Required Work

1. Fix route definitions and typed navigation
2. Fix service-call signature mismatches
3. Repair WebSocket hook typing and lifecycle safety
4. Remove placeholder PM routes from primary navigation or finish them
5. Add smoke tests for:
   - login redirect
   - workspace switching
   - story board load
   - story detail reload
   - objective detail edit flow
   - search navigation

## 8. Backend PRD: Remediation Requirements

### 8.1 Release Gate

The PM backend is releasable only when:

- destructive startup DDL is removed
- workspace authorization is enforced
- entity fetches are workspace-scoped
- at least basic API tests exist for authz and core CRUD

### 8.2 Required Work

1. Remove `DROP TABLE IF EXISTS pm_epic_objectives`
2. Introduce workspace-membership authorization middleware
3. Require workspace scoping in repository `GetByID`/`Update`/`Delete` paths
4. Add authz tests covering cross-workspace access attempts
5. Replace race-prone story display ID allocation
6. Move back toward explicit migrations as the authoritative schema history

## 9. Recommended Delivery Plan

### Phase 1: Stop-the-bleeding

- Remove destructive table drop
- Lock down workspace authorization
- Fix frontend build failures

### Phase 2: Stabilize core PM

- Add API tests for stories, epics, sprints, objectives, attachments
- Add frontend smoke tests
- Clear lint baseline on core PM pages/components

### Phase 3: Finish or hide incomplete PM areas

- Hide Docs/Roadmap/Reports until functional
- Or implement MVP versions with real data and acceptance criteria

## 10. Release Recommendation

**Recommendation**: Do not ship the current PM feature set as production-complete.

### Ship blockers

- P0 data-loss issue
- P0 authorization issue
- failing frontend build
- incomplete PM routes exposed as available features

### Safe claim after remediation

After P0/P1 fixes, Helpin can credibly position the current PM module as:

- core work tracking
- sprint planning
- epic management
- objective tracking
- comments, attachments, views, and automations

It should not yet be marketed as fully complete for docs, roadmap, or reports.

