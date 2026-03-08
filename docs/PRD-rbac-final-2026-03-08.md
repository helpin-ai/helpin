# PRD: RBAC And Workspace Authorization

**Product**: TeamPulse  
**Date**: March 8, 2026  
**Status**: Final PRD  
**Supersedes**:
- [PRD-rbac-members-and-roles.md](/root/teampulse/docs/PRD-rbac-members-and-roles.md)
- [RBAC-final-plan-2026-03-06.md](/root/teampulse/docs/RBAC-final-plan-2026-03-06.md)

## 1. Review Summary

This review found that the earlier RBAC docs mix two different versions of the product:

- the legacy identity model centered on `workspace_people`, `team_memberships`, and `team_user_memberships`
- the current identity model centered on `workspace_members`, `team_workspace_memberships`, and `reward_profiles`

The old documents are therefore partially correct on authorization risks, but stale on schema, member lifecycle, team membership storage, and some implementation references.

Primary source files reviewed for this PRD:

- workspace identity and statuses: [workspace.go](/root/teampulse/server/internal/model/workspace.go)
- team membership and compatibility DTOs: [settings.go](/root/teampulse/server/internal/model/settings.go)
- unified identity migration and legacy-table hard cut: [workspace_member_schema.go](/root/teampulse/server/internal/repository/workspace_member_schema.go), [020_workspace_member_identity.sql](/root/teampulse/server/migrations/020_workspace_member_identity.sql), [021_workspace_identity_hard_cut.sql](/root/teampulse/server/migrations/021_workspace_identity_hard_cut.sql)
- invitation lifecycle on top of `workspace_members`: [invite.go](/root/teampulse/server/internal/service/invite.go)
- current route and middleware shape: [router.go](/root/teampulse/server/internal/router/router.go), [auth.go](/root/teampulse/server/internal/middleware/auth.go), [workspace.go](/root/teampulse/server/internal/middleware/workspace.go)
- current frontend role heuristics: [sessionStore.ts](/root/teampulse/frontend/src/stores/sessionStore.ts), [Settings.tsx](/root/teampulse/frontend/src/pages/Settings.tsx), [Sidebar.tsx](/root/teampulse/frontend/src/components/layout/Sidebar.tsx)

### 1.1 Key corrections vs earlier docs

- `workspace_members` is already the canonical workspace identity and access table.
- `workspace_people` is no longer an active source of truth for RBAC and should not appear in new RBAC design.
- `team_workspace_memberships` is the canonical team membership table.
- `team_memberships` and `team_user_memberships` are legacy tables and should not be used as the basis of future authorization logic.
- PM identity has already started moving to `workspace_member_id` fields instead of user-only or people-only references.
- invitation acceptance already activates a pending `workspace_members` row rather than creating a second identity model.
- the current member status vocabulary in code is `pending`, `active`, `inactive`, `revoked`, not `invited`, `suspended`, `left`.
- the main remaining gap is centralized authorization, not another schema migration.

## 2. Final Scope Decision

Phase 1 RBAC will harden authorization around the storage model that already exists in the codebase. It will not begin with new roles, guest access, or another identity-table redesign.

Final decisions:

- `workspace_members` remains the canonical workspace identity row.
- `team_workspace_memberships` remains the canonical team membership row.
- `reward_profiles` remains the reward- and HR-specific extension of a workspace member.
- workspace roles remain `owner`, `admin`, `manager`, `member`, `viewer`.
- team roles remain `owner`, `member`.
- member statuses remain `pending`, `active`, `inactive`, `revoked`.
- role strings stay unchanged in storage for Phase 1.
- compatibility DTOs may continue to exist in APIs where needed, but they are not the storage truth.

Deferred beyond Phase 1:

- `guest`
- `team_owner` rename
- custom roles
- per-feature role builder
- team privacy modes such as `open`, `restricted`, `private`
- broad terminology renames for statuses or roles

## 3. Current System Reality

### 3.1 Identity and membership schema

| Entity | Current role | Notes |
|---|---|---|
| `workspace_members` | canonical workspace identity and access row | `user_id` is nullable so pending invitees can exist before signup/acceptance |
| `workspace_invitations` | invitation record | linked to `workspace_member_id` when a pending member identity exists |
| `team_workspace_memberships` | canonical team membership join table | stores team role `owner` or `member` against `workspace_member_id` |
| `reward_profiles` | reward-specific metadata | keeps compensation/manager/job-role data out of RBAC core |
| `workspace_people` | legacy table | should be considered retired for RBAC design |
| `team_memberships` | legacy table | should be considered retired for RBAC design |
| `team_user_memberships` | legacy table | should be considered retired for RBAC design |

Important compatibility note:

- the Settings API still exposes shapes like `people`, `memberships`, and `user_memberships`
- those response shapes are compatibility DTOs derived from current tables
- they must not be mistaken for the canonical authorization model

### 3.2 What is already improved

The codebase has already completed meaningful identity cleanup:

- invite creation creates or updates a pending `workspace_members` row
- invite acceptance activates that same membership row
- team memberships are written to `team_workspace_memberships`
- team creation auto-adds the creator as team `owner`
- settings "people" data is synthesized from `workspace_members` plus `reward_profiles`
- PM stories, epics, and objectives already support workspace-member-based ownership in the schema

### 3.3 What is still missing

Authorization remains incomplete and inconsistent:

- JWT authentication exists, but most route groups do not enforce workspace membership centrally.
- `RequireWorkspaceID` only copies a client-supplied workspace ID into context.
- many handlers and repositories still trust raw `workspace_id`, slug, or entity IDs without proving the actor belongs to that workspace
- some services contain local role checks, but there is no shared permission catalog or policy engine
- WebSocket access trusts any valid token plus any supplied `workspace_id`
- frontend UI uses local role heuristics instead of backend-resolved permissions

## 4. Problem Statement

TeamPulse has mostly finished the identity-table migration that older RBAC docs were still planning for. The remaining product risk is not missing a new table or missing a guest role. The remaining risk is that authorization is not consistently enforced server-side.

Today this creates five concrete problems:

1. Workspace-scoped routes are not uniformly protected by workspace membership checks.
2. Privileged actions are enforced through scattered role checks instead of a central policy layer.
3. Some repositories still operate on raw IDs instead of workspace-scoped IDs, which creates cross-workspace access risk.
4. Frontend route access and editability are inferred from local role labels instead of backend permissions.
5. Existing docs encourage future work on stale tables and stale lifecycle terminology.

## 5. Goals

1. Make backend authorization the source of truth for every workspace-scoped action.
2. Keep the already-shipped identity model centered on `workspace_members`.
3. Define one permission vocabulary reused by backend middleware, services, and frontend UI.
4. Enforce workspace access before route handlers touch workspace data.
5. Enforce privileged actions through permissions, not ad hoc role checks.
6. Keep team RBAC compatible with current storage while leaving room for richer team controls later.
7. Ship one final PRD that reflects the current schema and current risk accurately.

## 6. Non-Goals

- redesigning identity around `workspace_people`
- adding guest users in Phase 1
- adding customizable roles in Phase 1
- renaming `owner` to `workspace_owner` in storage
- renaming team `owner` to `team_owner` in storage
- introducing a generalized policy editor UI
- solving billing-plan gating, SCIM, or SSO group sync

## 7. Canonical Role And Status Model

### 7.1 Workspace roles

| Role | Scope | Phase 1 meaning |
|---|---|---|
| `owner` | workspace | full administrative access including owner-only destructive actions |
| `admin` | workspace | full operational administration except owner-only actions |
| `manager` | workspace | can operate day-to-day PM and applicable management surfaces, but cannot administer workspace membership or core workspace settings |
| `member` | workspace | standard collaborator with edit access to day-to-day product work |
| `viewer` | workspace | read-only workspace access |

### 7.2 Team roles

| Role | Scope | Phase 1 meaning |
|---|---|---|
| `owner` | team | delegated team-level owner for team settings and team membership actions once enforced |
| `member` | team | standard team participant |

Rules:

- workspace `owner` and `admin` override team checks
- team creator becomes team `owner`
- no guest team membership exists in Phase 1
- no separate `team_owner` storage value is introduced in Phase 1

### 7.3 Member statuses

| Status | Meaning | Access effect |
|---|---|---|
| `pending` | invited identity exists but is not yet active | cannot authenticate into the workspace |
| `active` | live workspace member | standard access based on role |
| `inactive` | no longer an active participant, but kept for continuity | no live access by default |
| `revoked` | access explicitly removed | no live access and excluded from normal operational pickers |

Status rules:

- only `active` memberships qualify for runtime workspace access
- `pending` members may appear in assignment or invitation-related UX where appropriate
- `inactive` and `revoked` members remain important for historical attribution
- lifecycle behavior must be enforced centrally in authorization, not left to individual handlers

## 8. Permission Model

### 8.1 Canonical permission catalog

Phase 1 will define a shared permission vocabulary at minimum:

Workspace permissions:

- `workspace.read`
- `workspace.update`
- `workspace.delete`
- `workspace.members.read`
- `workspace.members.manage`
- `workspace.invites.manage`
- `workspace.roles.manage`

Settings and team permissions:

- `settings.read`
- `settings.manage`
- `team.read`
- `team.manage`
- `team.members.manage`

PM permissions:

- `pm.read`
- `pm.edit`
- `pm.admin.workflows`
- `pm.admin.labels`
- `pm.admin.automations`
- `pm.import`

Other workspace-scoped permissions:

- `rewards.read`
- `rewards.manage`
- `search.read`
- `ws.connect`

### 8.2 Minimum role mapping for Phase 1

| Permission | Owner | Admin | Manager | Member | Viewer |
|---|---|---|---|---|---|
| `workspace.read` | Yes | Yes | Yes | Yes | Yes |
| `settings.read` | Yes | Yes | Yes | Yes | Yes |
| `workspace.members.read` | Yes | Yes | Yes | Yes | Yes |
| `pm.read` | Yes | Yes | Yes | Yes | Yes |
| `search.read` | Yes | Yes | Yes | Yes | Yes |
| `ws.connect` | Yes | Yes | Yes | Yes | Yes |
| `pm.edit` | Yes | Yes | Yes | Yes | No |
| `rewards.read` | Yes | Yes | Yes | Yes | No |
| `settings.manage` | Yes | Yes | No | No | No |
| `workspace.update` | Yes | Yes | No | No | No |
| `workspace.members.manage` | Yes | Yes | No | No | No |
| `workspace.invites.manage` | Yes | Yes | No | No | No |
| `workspace.roles.manage` | Yes | Yes, except owner transfer | No | No | No |
| `pm.admin.workflows` | Yes | Yes | No | No | No |
| `pm.admin.labels` | Yes | Yes | No | No | No |
| `pm.admin.automations` | Yes | Yes | No | No | No |
| `pm.import` | Yes | Yes | No | No | No |
| `workspace.delete` | Yes | No | No | No | No |

Notes:

- team-level settings and membership permissions will additionally allow team `owner`
- `admin` may not transfer or assign the workspace `owner` role
- reward-route hardening should adopt the same pattern: read permissions separate from manage permissions

## 9. Backend Design

### 9.1 Introduce a centralized authorization package

Add a dedicated package such as:

- `server/internal/authorization/permissions.go`
- `server/internal/authorization/policy.go`
- `server/internal/authorization/context.go`
- `server/internal/authorization/middleware.go`

This layer will:

- resolve the actor's active workspace membership
- attach actor context to the request
- map workspace roles to permissions
- resolve team memberships for the active workspace when needed
- expose helpers such as `Can(actor, permission)` and `CanManageTeam(actor, teamID)`

### 9.2 Replace `RequireWorkspaceID` as the security boundary

`RequireWorkspaceID` should stop being treated as authorization middleware. It may remain as an internal workspace-ID extraction helper, but it must not be the route guard.

New route guards should be:

- `RequireWorkspaceAccess`
- `RequireWorkspacePermission(permission)`
- `RequireTeamPermission(permission)`

Required behavior:

- resolve workspace ID from path, header, or query in one place
- load the caller's membership from `workspace_members`
- reject non-members and non-`active` members
- store resolved actor context for downstream handlers and services

### 9.3 Enforce authorization at the router level

All workspace-scoped routes must be moved behind workspace-access middleware, including:

- workspace detail, update, delete, member, and import routes
- settings routes
- rewards routes
- PM routes
- search routes
- support routes
- git integration routes when a workspace is involved
- WebSocket connection

Specific corrections required:

- `GET /api/workspaces/by-slug/{slug}` must verify the caller belongs to that workspace
- `GET /api/workspaces/{id}/my-membership` must not be the only membership check a page relies on
- routes that currently take `workspace_id` in query string must still prove the actor belongs to that workspace

### 9.4 Stop trusting raw entity IDs

Repository and service methods must become workspace-scoped.

Replace patterns like:

- `GetByID(id)`
- `Update(id, ...)`
- `Delete(id)`

with patterns like:

- `GetByID(workspaceID, id)`
- `Update(workspaceID, id, ...)`
- `Delete(workspaceID, id)`

This applies first to:

- stories
- epics
- sprints
- objectives
- workflows and workflow states
- labels
- views
- comments
- attachments
- checklist items
- external links
- automations
- support tickets and messages where workspace scoping matters

### 9.5 Keep invitations and lifecycle aligned with canonical identity

Invitation and lifecycle rules for Phase 1:

1. Send invite:
   - create or update a `pending` `workspace_members` row
   - create invitation linked to `workspace_member_id`
2. Accept invite:
   - activate the same `workspace_members` row
   - link `user_id`
   - preserve historical continuity of ownership and assignment references
3. Revoke invite or revoke access:
   - mark membership `revoked` where appropriate
   - ensure future access checks reject the member

No new parallel identity model should be introduced.

### 9.6 Add a backend access endpoint for frontend consumption

Add a stable endpoint:

- `GET /api/workspaces/{id}/me`

Response should include:

- workspace ID
- resolved workspace membership
- resolved workspace role
- effective permissions
- team memberships in that workspace

Suggested shape:

```json
{
  "workspace_id": "ws_123",
  "membership": {
    "id": "wm_123",
    "role": "admin",
    "status": "active"
  },
  "permissions": [
    "workspace.read",
    "settings.manage",
    "pm.edit"
  ],
  "team_memberships": [
    {
      "team_id": "team_123",
      "role": "owner"
    }
  ]
}
```

This becomes the only supported source of truth for frontend access control.

### 9.7 WebSocket hardening

WebSocket connection must require:

- valid JWT
- resolved workspace membership
- `ws.connect` permission

No socket should be registered from token validation alone.

### 9.8 Backend tests required before rollout

Add automated coverage for:

- active vs pending/inactive/revoked membership access
- owner/admin/manager/member/viewer permission mapping
- team owner overrides where applicable
- route middleware rejection of cross-workspace requests
- repository methods rejecting raw cross-workspace entity access
- invite acceptance preserving the same `workspace_member_id`
- WebSocket rejection for unauthorized workspace IDs

## 10. Frontend Design

### 10.1 Replace local role heuristics with permission-based access

The current frontend approach is too local:

- `sessionStore` loads one membership row and derives booleans like `isAdmin()`
- pages and components gate editability with local role checks
- navigation is largely static

Replace this with a workspace access store backed by `GET /api/workspaces/{id}/me`.

Recommended store shape:

- `membership`
- `permissions`
- `teamMemberships`
- `has(permission)`
- `hasAny(permission[])`

### 10.2 Add route guards and permission-aware navigation

Frontend must:

- block settings/admin routes when the user lacks the required permission
- show read-only sections only when the user can actually view them
- render edit controls only when `has(permission)` is true
- treat `403` as access denied, not as a generic data-loading problem

Examples:

- settings section mutations require `settings.manage`
- member and invite actions require `workspace.members.manage` or `workspace.invites.manage`
- workflow, label, automation, and import surfaces require the matching PM admin permissions

### 10.3 Keep the current API shapes during transition if needed

The frontend may continue consuming compatibility response shapes during migration, but:

- access control must move to permission checks
- role labels alone must not remain the source of truth
- slug-based workspace bootstrap must handle forbidden access cleanly

### 10.4 Frontend tests required before rollout

Add tests for:

- route guards
- settings editability by permission
- nav visibility by permission
- forbidden workspace bootstrap and forbidden settings access

## 11. Data And Migration Rules

Phase 1 should avoid unnecessary schema churn.

Required rules:

- do not reintroduce `workspace_people` into RBAC design
- do not build new team authorization on top of `team_memberships` or `team_user_memberships`
- do not rename storage role values in Phase 1
- do not create a second workspace identity table
- keep compatibility DTOs only as an API transition aid

Documentation rule:

- any new RBAC doc must describe `workspace_members` as the canonical identity table

## 12. Delivery Phases

### Phase 1: Authorization foundation

- add permission catalog
- add actor-context and workspace-access middleware
- add backend tests for membership and permission resolution

Exit criteria:

- backend has one canonical authorization layer

### Phase 2: Route and repository hardening

- apply workspace authorization to all workspace-scoped routes
- refactor repositories and services to workspace-scoped entity access
- harden WebSocket and slug lookup

Exit criteria:

- changing `workspace_id`, slug, or raw entity ID cannot cross workspace boundaries

### Phase 3: Frontend permission integration

- ship `GET /api/workspaces/{id}/me`
- replace frontend role heuristics with permission checks
- update route guards, nav, and page editability

Exit criteria:

- frontend only offers actions the backend would allow

### Phase 4: Team RBAC expansion and deferred model work

- expand team-specific permission enforcement
- add guest support if still needed
- add richer team privacy or access-mode controls if still needed

Exit criteria:

- team-scoped authorization is enforced without changing the core identity model

## 13. Acceptance Criteria

RBAC is considered complete for this Phase 1 PRD when all of the following are true:

- every workspace-scoped backend route resolves and enforces active workspace membership
- every privileged backend action uses centralized permission checks
- every entity read/write path is workspace-scoped
- WebSocket access is authorized by workspace membership and permission
- frontend consumes backend-resolved permissions instead of only local role labels
- settings, member-management, and PM admin surfaces are permission-gated end to end
- no new RBAC work relies on `workspace_people`, `team_memberships`, or `team_user_memberships`
- automated tests cover role differences, lifecycle status differences, and cross-workspace denial

## 14. Final Conclusion

The correct final RBAC plan for TeamPulse is:

- keep the unified identity model already built around `workspace_members`
- stop treating old tables as active design inputs
- build centralized server-side authorization next
- then move the frontend to backend-resolved permissions
- only after that, consider guest access or richer team RBAC

That sequence fixes the real security gap while preserving the identity simplification already completed in the codebase.
