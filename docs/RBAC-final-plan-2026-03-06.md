# Final RBAC Plan

**Product**: Helpin  
**Date**: March 6, 2026  
**Status**: Final planning doc  
**Supersedes**:
- [PRD-rbac-members-and-roles.md](/root/helpin/docs/PRD-rbac-members-and-roles.md)
- [PRD-current-state-review-2026-03-05.md](/root/helpin/docs/PRD-current-state-review-2026-03-05.md)

## 1. Decision

Helpin does **not** have RBAC properly implemented today.

What exists today is:

- authenticated access via JWT
- workspace membership records with role labels
- a few isolated role checks in invitation flows
- frontend helpers that hide or disable some actions

What is missing is the part that matters most:

- centralized server-side authorization
- consistent workspace-membership enforcement
- route-level and entity-level permission checks
- consistent frontend permission consumption

The implementation plan must therefore start with **authorization hardening of the current model**, not with adding more roles first.

## 2. Current State Summary

### Backend reality

- Most protected routes only use JWT auth, not authorization:
  - [router.go](/root/helpin/server/internal/router/router.go#L68)
- PM and search routes only require a client-supplied workspace ID:
  - [router.go](/root/helpin/server/internal/router/router.go#L151)
  - [router.go](/root/helpin/server/internal/router/router.go#L157)
  - [workspace.go](/root/helpin/server/internal/middleware/workspace.go#L5)
- Workspace routes do not verify membership or role before returning/updating/deleting data:
  - [workspace.go](/root/helpin/server/internal/handler/workspace.go#L56)
  - [workspace.go](/root/helpin/server/internal/handler/workspace.go#L68)
  - [workspace.go](/root/helpin/server/internal/handler/workspace.go#L87)
  - [workspace.go](/root/helpin/server/internal/handler/workspace.go#L126)
- Settings routes accept raw workspace IDs or team IDs and perform no actor authorization:
  - [settings.go](/root/helpin/server/internal/handler/settings.go#L23)
  - [settings.go](/root/helpin/server/internal/handler/settings.go#L57)
  - [settings.go](/root/helpin/server/internal/handler/settings.go#L75)
  - [settings.go](/root/helpin/server/internal/handler/settings.go#L94)
  - [settings.go](/root/helpin/server/internal/handler/settings.go#L233)
  - [settings.go](/root/helpin/server/internal/handler/settings.go#L321)
- PM services still trust raw IDs for many entity reads and writes:
  - [pm_story.go](/root/helpin/server/internal/handler/pm_story.go#L140)
  - [pm_story.go](/root/helpin/server/internal/service/pm_story.go#L44)
  - [pm_story.go](/root/helpin/server/internal/repository/pm_story.go#L91)
  - [pm_story.go](/root/helpin/server/internal/repository/pm_story.go#L117)
  - [pm_story.go](/root/helpin/server/internal/repository/pm_story.go#L158)
  - [pm_story.go](/root/helpin/server/internal/repository/pm_story.go#L169)
- Board access is driven by `workflow_id` without workspace scoping:
  - [pm_story.go](/root/helpin/server/internal/handler/pm_story.go#L72)
  - [pm_story.go](/root/helpin/server/internal/repository/pm_story.go#L302)
- Other PM repositories also fetch by raw ID:
  - [pm_workflow.go](/root/helpin/server/internal/repository/pm_workflow.go#L47)
  - [pm_attachment.go](/root/helpin/server/internal/repository/pm_attachment.go#L31)
- WebSocket connections trust any supplied `workspace_id` after token validation:
  - [handler.go](/root/helpin/server/internal/websocket/handler.go#L23)

### Frontend reality

- Frontend permission logic is local role checking only:
  - [sessionStore.ts](/root/helpin/frontend/src/stores/sessionStore.ts#L15)
- Workspace selection is based on `getBySlug`, which currently exposes any workspace by slug:
  - [workspaceStore.ts](/root/helpin/frontend/src/stores/workspaceStore.ts#L35)
  - [workspacesService.ts](/root/helpin/frontend/src/lib/services/workspacesService.ts#L4)
- Settings pages are loaded for any workspace once the route is entered, then editability is toggled in UI:
  - [Settings.tsx](/root/helpin/frontend/src/pages/Settings.tsx#L120)
  - [Settings.tsx](/root/helpin/frontend/src/pages/Settings.tsx#L176)
- Settings navigation is shown unconditionally:
  - [Sidebar.tsx](/root/helpin/frontend/src/components/layout/Sidebar.tsx#L173)
  - [Sidebar.tsx](/root/helpin/frontend/src/components/layout/Sidebar.tsx#L217)
- API clients pass `workspace_id` in query strings directly, which mirrors the backend trust problem:
  - [api.ts](/root/helpin/frontend/src/lib/api.ts#L10)
  - [settingsService.ts](/root/helpin/frontend/src/lib/services/settingsService.ts#L30)
  - [pmStoryService.ts](/root/helpin/frontend/src/lib/services/pmStoryService.ts#L32)
  - [searchService.ts](/root/helpin/frontend/src/lib/services/searchService.ts#L22)
- WebSocket connects with arbitrary workspace ID from the client:
  - [useWebSocket.ts](/root/helpin/frontend/src/hooks/useWebSocket.ts#L19)

## 3. Final Scope Decision

### 3.1 What we will implement first

Phase 1 RBAC will formalize and enforce the roles already in product storage:

- `owner`
- `admin`
- `manager`
- `member`
- `viewer`

Team roles in Phase 1:

- `owner`
- `member`

This avoids a large schema migration while fixing the actual security problem first.

### 3.2 What we will not block Phase 1 on

These remain planned, but are **not** prerequisites for the first proper RBAC rollout:

- guest users
- membership lifecycle states (`invited`, `suspended`, `left`)
- team owner rename to `team_owner`
- custom permission builders
- per-feature exceptions across the product

### 3.3 Naming decision

Keep `owner` in storage for now and present it as **Workspace Owner** in UI and docs.

Do not rename the DB role value in Phase 1.

## 4. RBAC Principles

1. Backend is the source of truth.
2. Every workspace-scoped route must prove workspace membership server-side.
3. Every privileged action must prove permission server-side.
4. Entity reads and writes must be scoped by workspace, not just by raw ID.
5. Frontend may hide actions, but hidden UI is never authorization.
6. One permission vocabulary must be reused across backend and frontend.

## 5. Target Permission Model

### 5.1 Workspace permissions for Phase 1

Define a centralized permission catalog with at least:

- `workspace.view`
- `workspace.update`
- `workspace.delete`
- `workspace.members.read`
- `workspace.members.manage`
- `workspace.invites.manage`
- `settings.read`
- `settings.manage`
- `rewards.view`
- `rewards.manage`
- `pm.view`
- `pm.edit`
- `pm.manage_workflows`
- `pm.manage_labels`
- `pm.manage_automations`
- `search.use`
- `ws.connect`

### 5.2 Role mapping for Phase 1

| Role | Baseline |
|------|----------|
| `owner` | Full workspace access |
| `admin` | Full operational access except destructive owner-only actions |
| `manager` | Read workspace/settings/rewards/PM, edit day-to-day PM/rewards, no member management |
| `member` | Standard collaboration access |
| `viewer` | Read-only access |

### 5.3 Team permission model for Phase 1

Keep current team roles:

- `owner`
- `member`

But enforce them only for team-scoped settings and future team-private PM actions.

Workspace `owner` and `admin` override team checks.

## 6. Backend Implementation Plan

### 6.1 Add a centralized authorization package

Create a new backend authorization layer, for example:

- `server/internal/authorization/permissions.go`
- `server/internal/authorization/policy.go`
- `server/internal/authorization/context.go`
- `server/internal/authorization/middleware.go`

Core responsibilities:

- map workspace role to permissions
- resolve actor membership for the active workspace
- expose helpers like `Can(actor, permission)`
- expose team helpers like `CanManageTeam(actor, teamID)`

### 6.2 Add actor context middleware

Add middleware that, for any workspace-scoped request:

- reads workspace ID from path/header/query in one place
- loads the caller’s workspace membership
- rejects non-members with `403`
- stores resolved actor context in request context

Required middleware set:

- `RequireWorkspaceAccess`
- `RequireWorkspacePermission(permission)`
- `RequireTeamPermission(permissionResolver)`

`RequireWorkspaceID` alone should be removed or downgraded to an internal helper.

### 6.3 Lock down all workspace-scoped routes

Apply workspace authorization middleware to:

- workspace detail/update/delete/member routes
- all settings routes
- all rewards routes
- all PM routes
- search routes
- WebSocket connect

Expected rule:

- no route should accept a workspace ID without verifying caller membership in that workspace

### 6.4 Stop trusting raw entity IDs

Repository/service methods must be changed from:

- `GetByID(id)`
- `Update(id)`
- `Delete(id)`

to workspace-scoped forms such as:

- `GetByID(workspaceID, id)`
- `Update(workspaceID, id, ...)`
- `Delete(workspaceID, id)`

This applies first to:

- stories
- workflows and workflow states
- labels
- epics
- sprints
- objectives
- comments
- attachments
- views
- checklist items
- external links
- automations

### 6.5 Add route-to-permission mapping

Minimum backend policy for Phase 1:

- read settings/search/PM/rewards data requires `workspace.view`
- settings mutations require `settings.manage`
- invitation and member management requires `workspace.members.manage`
- workflow/label/automation mutations require `pm.manage_workflows`, `pm.manage_labels`, or `pm.manage_automations`
- story/epic/sprint/objective/comment edits require `pm.edit`
- workspace delete requires owner-only permission
- WebSocket connect requires `ws.connect`

### 6.6 Add backend permission endpoints

Add one stable source for frontend consumption:

- `GET /api/workspaces/{id}/me`

Response should include:

- workspace membership
- resolved workspace role
- effective permissions
- team memberships for the workspace

This should replace ad hoc frontend role inference over time.

### 6.7 Keep invite and membership logic aligned

Current invite checks are the only real RBAC checks today:

- [invite.go](/root/helpin/server/internal/service/invite.go#L54)

Retain that behavior, but move it under centralized permission checks instead of hardcoding owner/admin logic in service methods.

### 6.8 WebSocket hardening

WebSocket connect must:

- validate JWT
- validate workspace membership
- reject unauthorized workspace connections

Do not register a socket client from token validation alone.

### 6.9 Backend tests required before rollout

Add table-driven tests for:

- workspace membership middleware
- permission mapping
- owner/admin/manager/member/viewer access differences
- settings authorization
- PM authorization
- cross-workspace ID access denial
- WebSocket workspace rejection

## 7. Frontend Implementation Plan

### 7.1 Replace local role heuristics with effective permissions

Current frontend role helpers:

- [sessionStore.ts](/root/helpin/frontend/src/stores/sessionStore.ts#L15)

should be replaced by a store that loads backend-resolved permissions from `GET /api/workspaces/{id}/me`.

New frontend store shape:

- `membership`
- `permissions`
- `team_memberships`
- `has(permission)`

### 7.2 Add route guards

The frontend should stop rendering settings/admin surfaces purely based on route access.

Required guards:

- block settings sections that require `settings.manage`
- block member management screens that require `workspace.members.manage`
- block workflow/label/automation screens if user lacks PM admin permissions

Profile remains public to any authenticated user in a workspace.

### 7.3 Update navigation from permission data

Sidebar and other entry points should render from effective permissions, not from fixed nav definitions:

- [Sidebar.tsx](/root/helpin/frontend/src/components/layout/Sidebar.tsx#L173)

Examples:

- hide Members/Teams admin actions for non-admins
- hide workflow/automation settings for users without PM admin permissions
- keep read-only routes visible only when the user can actually access them

### 7.4 Keep UI disable/hide behavior, but only as UX

Existing `editable={isAdmin()}` usage in settings:

- [Settings.tsx](/root/helpin/frontend/src/pages/Settings.tsx#L176)

should be migrated to `has(permission)` checks.

This remains useful for UX, but all backend endpoints must already reject unauthorized calls.

### 7.5 Stop treating workspace slug lookup as authorization

Frontend workspace bootstrap currently trusts workspace lookup by slug:

- [workspaceStore.ts](/root/helpin/frontend/src/stores/workspaceStore.ts#L35)

After backend hardening:

- `GET /workspaces/by-slug/{slug}` must only return workspaces the caller belongs to
- frontend should treat `403` as access denied, not as a partial workspace load

### 7.6 Frontend tests required before rollout

Add tests for:

- route guards
- nav visibility by permission
- settings page edit/read-only behavior
- forbidden responses causing redirect or error state

## 8. Delivery Phases

### Phase 0: Authorization Foundation

- add permission catalog
- add actor context middleware
- add workspace membership resolution
- add backend tests for policy and middleware

Exit criteria:

- backend has one canonical permission layer

### Phase 1: Lock Down Existing Product

- apply middleware to workspace/settings/rewards/PM/search/WebSocket
- scope repositories by workspace
- fix raw-ID access paths
- expose `GET /api/workspaces/{id}/me`

Exit criteria:

- a user cannot access another workspace by changing `workspace_id`, slug, UUID, or WebSocket params

### Phase 2: Frontend Permission Integration

- replace role helper checks with permission store
- add route guards
- update sidebar and page-level action visibility

Exit criteria:

- frontend shows only actions the backend would allow

### Phase 3: Team RBAC Expansion

- introduce team-level permission helpers
- enforce team-owner behavior
- add team-scoped PM restrictions where needed

Exit criteria:

- team settings and future private-team flows are enforced server-side

### Phase 4: Guest And Membership Lifecycle

- add `guest`
- add membership statuses
- add suspension/offboarding behavior
- add audit logging for RBAC-sensitive changes

Exit criteria:

- guest and suspended users behave correctly across backend and frontend

## 9. Non-Goals For Phase 1

- custom roles
- SCIM
- SSO group sync
- billing plan enforcement
- per-feature permission editor
- broad schema renames for existing roles

## 10. Acceptance Criteria

RBAC is considered properly implemented for Phase 1 only when all of the following are true:

- every workspace-scoped backend route enforces workspace membership
- every privileged backend action enforces a centralized permission check
- every entity read/write path is workspace-scoped
- WebSocket connections are workspace-authorized
- frontend consumes effective permissions from backend, not only local role labels
- navigation and route guards align with backend permissions
- automated tests cover cross-workspace denial and role differences

## 11. Execution Order

Recommended execution order:

1. Build backend authorization primitives and tests.
2. Migrate workspace/settings/rewards/PM/search/WebSocket routes onto those primitives.
3. Refactor repositories to workspace-scoped entity access.
4. Ship the backend `me/permissions` payload.
5. Replace frontend role helpers and add route/nav guards.
6. Only then extend the model with guest access, lifecycle states, and richer team RBAC.

## 12. Final Conclusion

The correct final plan is:

- do **not** start by adding new roles
- first turn the current membership model into real server-side RBAC
- then move the frontend to consume backend-resolved permissions
- then extend the model with team ownership, guest access, and lifecycle states

That order fixes the real security gap and keeps migration risk contained.
