# PRD: RBAC And Workspace Authorization

**Product**: TeamPulse  
**Date**: March 8, 2026  
**Status**: Final PRD  
**Supersedes**: PRD-rbac-members-and-roles.md and RBAC-final-plan-2026-03-06.md (both removed from the repository)

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

### 3.4 Gap analysis: PRD requirements vs current codebase

Backend gaps:

| PRD requirement | Status | Current codebase reality |
|---|---|---|
| `§9.1` authorization package | Missing | there is no `server/internal/authorization/` package today |
| `§9.1` permission catalog | Missing | no canonical permission constants such as `workspace.read` or `pm.edit` exist |
| `§9.1` role-to-permission mapping | Missing | there is no policy engine or shared `Can()` helper |
| `§9.2` `RequireWorkspaceAccess` middleware | Missing | `RequireWorkspaceID` only copies a client-supplied workspace ID into context |
| `§9.3` route-level workspace authorization | Missing | routes are primarily behind JWT auth, not workspace-membership enforcement |
| `§9.4` workspace-scoped repositories | Partial | many list queries are scoped, but a large number of `GetByID`, `Update`, and `Delete` methods still trust raw IDs |
| `§9.5` active-status enforcement | Missing | there is no central middleware or policy check ensuring only `active` members get access |
| `§9.6` `GET /workspaces/{id}/me` with permissions | Partial | `/my-membership` exists, but it returns only the membership row, not effective permissions or team memberships |
| `§9.7` WebSocket hardening | Missing | WebSocket accepts any valid JWT and any supplied `workspace_id` |

Frontend gaps:

| PRD requirement | Status | Current codebase reality |
|---|---|---|
| `§10.1` permission-based access store | Missing | `sessionStore` exposes role heuristics such as `isAdmin()` and `isOwner()` |
| `§10.2` route guards | Missing | settings and admin surfaces rely on page/component behavior, not route-level permission guards |
| `§10.2` 403 handling | Missing | the API client has 401 refresh/logout behavior but no dedicated 403 access-denied flow |
| `§10.1` viewer-role support | Partial | `viewer` exists in types and invitation UI, but there is no consistent read-only enforcement path |
| `§10.2` team-level RBAC in UI | Missing | the frontend has no effective team-role or team-permission awareness |

### 3.5 Critical security risks

The current gaps create four immediate risks:

1. Cross-workspace data access:
   authenticated users can supply arbitrary `workspace_id` values in header or query parameters, and many downstream reads and writes do not prove workspace membership.
2. WebSocket subscription leakage:
   any valid JWT holder can attempt to connect to another workspace's real-time channel by supplying a different `workspace_id`.
3. Unguarded mutations:
   routes such as team creation, team membership changes, team estimate updates, and workspace deletion currently lack centralized permission enforcement.
4. Slug lookup leakage:
   `GET /api/workspaces/by-slug/{slug}` returns workspace data without first proving the caller belongs to that workspace.

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
| `manager` | workspace | can operate day-to-day PM, manage rewards data, and manage team membership for teams they own, but cannot administer workspace membership or core workspace settings |
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
- `team.members.read`
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
| `team.read` | Yes | Yes | Yes | Yes | Yes |
| `team.members.read` | Yes | Yes | Yes | Yes | Yes |
| `pm.edit` | Yes | Yes | Yes | Yes | No |
| `rewards.read` | Yes | Yes | Yes | Yes | No |
| `rewards.manage` | Yes | Yes | Yes | No | No |
| `team.manage` | Yes | Yes | No | No | No |
| `team.members.manage` | Yes | Yes | No | No | No |
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

- team-level `team.manage` and `team.members.manage` are additionally granted to team `owner` for teams they own, even if their workspace role would not normally include those permissions
- `admin` may not transfer or assign the workspace `owner` role; this is enforced by treating owner transfer as a distinct action within `workspace.roles.manage` that requires `owner` role specifically (services must check `role == "owner"` before allowing owner assignment)
- `manager` receives `rewards.manage` to support day-to-day compensation and evaluation workflows without requiring full admin access
- reward-route hardening should adopt the same pattern: read permissions separate from manage permissions

## 9. Backend Design

### 9.1 Introduce a centralized authorization package using Casbin

#### 9.1.1 Framework choice: Casbin

The authorization layer will be built on **Casbin** (`github.com/casbin/casbin/v2`) with the GORM adapter (`github.com/casbin/gorm-adapter/v3`).

Rationale:

- runs in-process inside the Go binary — no new infrastructure to deploy or operate
- GORM adapter stores policies in the existing Neon PostgreSQL database
- supports RBAC with resource roles, which maps directly to the workspace role + team role model
- model and policy are configuration-driven and can evolve to ABAC in later phases without migrating to a different system
- mature ecosystem with Chi middleware helpers and multi-language ports

Alternatives considered and rejected:

- **OpenFGA / ORY Keto**: require deploying and operating a separate authorization service over gRPC, which is unnecessary complexity for a single Go monolith with ~5 workspace roles and ~25 permissions
- **goRBAC**: too basic — no resource-scoped roles, no path to ABAC, would dead-end if Phase 4 needs richer team or per-resource controls

#### 9.1.2 Package structure

Add a dedicated package:

```
server/internal/authorization/
├── permissions.go    # permission string constants
├── model.conf        # Casbin model definition
├── policy.go         # policy loader and role-permission seed
├── enforcer.go       # Casbin enforcer initialization and helpers
├── actor.go          # Actor struct and context helpers
├── middleware.go      # Chi middleware wrappers
```

#### 9.1.3 Casbin model definition (`model.conf`)

```ini
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _    # user -> workspace_role (per workspace domain)

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

- `sub`: the actor's workspace member ID (e.g. `wm_123`)
- `dom`: the workspace ID (e.g. `ws_456`) — Casbin's domain feature provides workspace isolation
- `obj`: the permission resource (e.g. `workspace`, `pm`, `settings`, `team`, `rewards`)
- `act`: the permission action (e.g. `read`, `edit`, `manage`, `delete`, `admin.workflows`)

#### 9.1.4 Policy seeding

Policy seeding follows the same idempotent pattern used elsewhere in the codebase (e.g. `SeedDefaultWorkflow`, `SeedDefaultEpicStates`, `SeedDefaults` for labels). The seed function is called once at startup from `cmd/api/main.go`, after the `AuthzService` is constructed.

**How it works with GORM AutoMigrate:**

1. The GORM adapter (`gormadapter.NewAdapterByDB(db)`) internally calls `db.AutoMigrate(&CasbinRule{})` to create or update the `casbin_rule` table. This happens automatically when the adapter is constructed — no manual AutoMigrate call is needed for the Casbin table.
2. After the enforcer is created, `seedBasePolicy()` is called. This function is **idempotent** — it checks whether the base policies already exist before inserting. On subsequent startups, it detects existing policies and skips the seed.
3. If the policy catalog changes (e.g. a new permission is added in a future release), the seed function detects missing policies and adds only the new ones.

**Seed function pattern:**

```go
func (s *AuthzService) seedBasePolicy() error {
    // Check if base policies already exist (idempotent guard)
    existingPolicies := s.enforcer.GetPolicy()
    if len(existingPolicies) > 0 {
        // Policies exist — sync any new permissions added since last seed
        return s.syncPolicyUpdates()
    }

    // First-time seed: add all role-permission policies
    policies := [][]string{
        // Viewer permissions
        {"viewer", "*", "workspace", "read"},
        {"viewer", "*", "settings", "read"},
        {"viewer", "*", "workspace.members", "read"},
        {"viewer", "*", "pm", "read"},
        {"viewer", "*", "search", "read"},
        {"viewer", "*", "ws", "connect"},
        {"viewer", "*", "team", "read"},
        {"viewer", "*", "team.members", "read"},

        // Member permissions (inherits viewer via grouping)
        {"member", "*", "pm", "edit"},
        {"member", "*", "rewards", "read"},

        // Manager permissions (inherits member)
        {"manager", "*", "rewards", "manage"},

        // Admin permissions (inherits manager)
        {"admin", "*", "settings", "manage"},
        {"admin", "*", "workspace", "update"},
        {"admin", "*", "workspace.members", "manage"},
        {"admin", "*", "workspace.invites", "manage"},
        {"admin", "*", "workspace.roles", "manage"},
        {"admin", "*", "pm", "admin.workflows"},
        {"admin", "*", "pm", "admin.labels"},
        {"admin", "*", "pm", "admin.automations"},
        {"admin", "*", "pm", "import"},
        {"admin", "*", "team", "manage"},
        {"admin", "*", "team.members", "manage"},

        // Owner permissions (inherits admin)
        {"owner", "*", "workspace", "delete"},
    }
    s.enforcer.AddPolicies(policies)

    // Role inheritance chain
    groupingRules := [][]string{
        {"member", "viewer", "*"},
        {"manager", "member", "*"},
        {"admin", "manager", "*"},
        {"owner", "admin", "*"},
    }
    s.enforcer.AddGroupingPolicies(groupingRules)

    return s.enforcer.SavePolicy()
}
```

**`syncPolicyUpdates()` handles rolling upgrades:**

When new permissions are added in a future release, the seed function compares the expected policy set against what exists in the database. It adds missing policies and removes deprecated ones. This avoids requiring a manual migration for policy changes.

**Runtime role assignment:**

When a user accesses a workspace, `RequireWorkspaceAccess` middleware assigns their workspace member ID to their role within that workspace domain:

```
g, wm_123, admin, ws_456
```

This grouping rule is added to the enforcer (and persisted via the adapter) when membership is resolved. The enforcer then evaluates permissions through the role inheritance chain: if `wm_123` is `admin` in `ws_456`, they inherit `manager` → `member` → `viewer` permissions automatically.

#### 9.1.5 GORM adapter and storage

The GORM adapter (`github.com/casbin/gorm-adapter/v3`) handles its own table lifecycle:

- `gormadapter.NewAdapterByDB(db)` internally calls `db.AutoMigrate(&CasbinRule{})` — this creates or updates the `casbin_rule` table automatically, consistent with how the rest of the codebase uses GORM AutoMigrate
- no manual `db.AutoMigrate` call is needed for Casbin in `cmd/api/main.go`
- the adapter reuses the existing `*gorm.DB` connection — no new database or connection pool

The `casbin_rule` table stores two kinds of rows:

| `ptype` | Purpose | Example |
|---|---|---|
| `p` | role-to-permission mappings (global baseline) | `p, admin, *, workspace, update` |
| `g` | role inheritance + per-workspace role assignments | `g, wm_123, admin, ws_456` |

Policy is loaded into memory by the enforcer at startup. The GORM adapter persists changes (new role assignments, policy updates) back to the database so they survive restarts.

Startup sequence in `cmd/api/main.go`:

```go
// Existing: db connection + AutoMigrate for all app models
db.AutoMigrate(&model.Workspace{}, &model.WorkspaceMember{}, /* ... */)

// New: AuthzService construction (adapter auto-creates casbin_rule table)
authzService, err := authorization.NewAuthzService(db, memberRepo)
// seedBasePolicy() is called internally — idempotent, safe on every restart
```

This fits the existing pattern where `main.go` runs AutoMigrate first, then calls idempotent seed functions (e.g. `MigrateWorkspacesToOrganizations`).

#### 9.1.6 Enforcer initialization (`enforcer.go`)

```go
type AuthzService struct {
    enforcer   *casbin.Enforcer
    memberRepo MemberRepository
}

func NewAuthzService(db *gorm.DB, memberRepo MemberRepository) (*AuthzService, error) {
    // Adapter auto-creates casbin_rule table via GORM AutoMigrate
    adapter, err := gormadapter.NewAdapterByDB(db)
    if err != nil {
        return nil, fmt.Errorf("authorization: failed to create adapter: %w", err)
    }

    enforcer, err := casbin.NewEnforcer("internal/authorization/model.conf", adapter)
    if err != nil {
        return nil, fmt.Errorf("authorization: failed to create enforcer: %w", err)
    }

    svc := &AuthzService{enforcer: enforcer, memberRepo: memberRepo}

    // Idempotent seed — safe to call on every startup
    // First run: inserts all role-permission policies and inheritance rules
    // Subsequent runs: detects existing policies, adds only new ones
    if err := svc.seedBasePolicy(); err != nil {
        return nil, fmt.Errorf("authorization: failed to seed base policy: %w", err)
    }

    return svc, nil
}
```

Wiring in `cmd/api/main.go` (placed after existing AutoMigrate and migration calls):

```go
// After: db.AutoMigrate(...)
// After: MigrateWorkspaceMemberSchema(db), MigrateWorkspacesToOrganizations(db), etc.

authzService, err := authorization.NewAuthzService(db, memberRepo)
if err != nil {
    log.Fatalf("Failed to initialize authorization: %v", err)
}

// Pass authzService to router setup
r := router.New(handlers, authzService)
```

#### 9.1.7 Public helpers

The package exposes the following helpers for use by middleware and services:

```go
// Can checks whether the actor has a specific permission in their workspace.
func (s *AuthzService) Can(actor *Actor, permission string) bool

// CanAny checks whether the actor has at least one of the listed permissions.
func (s *AuthzService) CanAny(actor *Actor, permissions ...string) bool

// CanManageTeam checks team-level management permission.
// Returns true if the actor is a workspace owner/admin OR a team owner for the given team.
func (s *AuthzService) CanManageTeam(actor *Actor, teamID string) bool

// IsOwnerOnly checks whether an action requires the workspace owner role specifically.
// Used for owner-transfer and workspace deletion guards.
func (s *AuthzService) IsOwnerOnly(actor *Actor) bool
```

#### 9.1.8 Actor context (`actor.go`)

```go
type Actor struct {
    UserID            string
    WorkspaceID       string
    WorkspaceMemberID string
    Role              string
    Status            string
    TeamMemberships   []TeamRole  // loaded lazily on first CanManageTeam call
}

type TeamRole struct {
    TeamID string
    Role   string
}
```

The `Actor` is resolved once per request by `RequireWorkspaceAccess` middleware and stored in the request context. Downstream handlers and services retrieve it via `authorization.GetActor(ctx)`.

#### 9.1.9 Team owner override

Team-level permissions (`team.manage`, `team.members.manage`) use a two-layer check:

1. Casbin enforcer checks the actor's workspace role — workspace `owner` and `admin` always pass
2. If the workspace role does not grant the permission, check `team_workspace_memberships` for team `owner` role on the specific team

This is implemented inside `CanManageTeam` rather than as additional Casbin policies, because team ownership is per-team and looked up dynamically.

### 9.2 Replace `RequireWorkspaceID` as the security boundary

`RequireWorkspaceID` should stop being treated as authorization middleware. It may remain as an internal workspace-ID extraction helper, but it must not be the route guard.

New Chi middleware functions (in `authorization/middleware.go`):

- `RequireWorkspaceAccess(authzService)` — resolves membership, enforces `active` status, stores `Actor` in context
- `RequirePermission(authzService, permission)` — calls `authzService.Can(actor, permission)`, returns 403 if denied
- `RequireAnyPermission(authzService, permissions...)` — calls `authzService.CanAny(actor, permissions...)`, returns 403 if none match
- `RequireTeamPermission(authzService, permission)` — extracts team ID from path, calls `authzService.CanManageTeam(actor, teamID)`, returns 403 if denied

`RequireWorkspaceAccess` behavior:

1. Resolve workspace ID from a single source per request using the following precedence: (1) path parameter `{id}` or `{slug}`, (2) header `X-Workspace-ID`, (3) query parameter `workspace_id`; only the first match is used
2. Load the caller's membership from `workspace_members` where `user_id` matches the JWT subject and `workspace_id` matches the resolved ID
3. Reject with 403 if no membership exists or membership status is not `active`
4. Assign the actor's workspace role to the Casbin enforcer for the workspace domain (if not already cached): `enforcer.AddGroupingPolicy(memberID, role, workspaceID)`
5. Build an `Actor` struct and store it in the request context via `authorization.WithActor(ctx, actor)`
6. Downstream handlers retrieve the actor via `authorization.GetActor(ctx)`

`RequirePermission` behavior:

1. Retrieve the `Actor` from context (fails with 500 if `RequireWorkspaceAccess` was not applied first)
2. Call `authzService.Can(actor, permission)` which delegates to `enforcer.Enforce(actor.WorkspaceMemberID, actor.WorkspaceID, resource, action)`
3. Return 403 with `{ "error": "forbidden", "reason": "insufficient_permission", "required": "<permission>" }` if denied

Router integration example:

```go
r.Route("/api", func(r chi.Router) {
    r.Use(middleware.RequireAuth)

    r.Route("/workspaces/{id}", func(r chi.Router) {
        r.Use(authorization.RequireWorkspaceAccess(authzService))

        r.Get("/me", h.Workspace.GetMe)  // any active member
        r.With(authorization.RequirePermission(authzService, "workspace.update")).Put("/", h.Workspace.Update)
        r.With(authorization.RequirePermission(authzService, "workspace.delete")).Delete("/", h.Workspace.Delete)
    })

    r.Route("/settings", func(r chi.Router) {
        r.Use(authorization.RequireWorkspaceAccess(authzService))
        r.With(authorization.RequirePermission(authzService, "settings.read")).Get("/", h.Settings.Get)
        r.With(authorization.RequirePermission(authzService, "settings.manage")).Post("/teams", h.Settings.CreateTeam)
    })

    r.Route("/pm", func(r chi.Router) {
        r.Use(authorization.RequireWorkspaceAccess(authzService))
        r.With(authorization.RequirePermission(authzService, "pm.read")).Get("/stories", h.PM.ListStories)
        r.With(authorization.RequirePermission(authzService, "pm.edit")).Post("/stories", h.PM.CreateStory)
        r.With(authorization.RequirePermission(authzService, "pm.admin.workflows")).Put("/workflows/{id}", h.PM.UpdateWorkflow)
    })
})
```

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

High-priority mutation routes to lock down first:

- `POST /api/settings/teams`
- `POST /api/settings/teams/{id}/members`
- `PUT /api/settings/teams/{id}/estimates`
- `DELETE /api/workspaces/{id}`
- WebSocket `/api/ws`

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

Error responses:

- `401 Unauthorized`: JWT is missing or invalid
- `403 Forbidden`: caller is not a member of the workspace, or membership status is not `active` (body should include `{ "error": "forbidden", "reason": "not_a_member" | "membership_inactive" | "membership_revoked" | "membership_pending" }`)
- `404 Not Found`: workspace ID does not exist

### 9.7 WebSocket hardening

WebSocket connection must require:

- valid JWT
- resolved workspace membership (same `workspace_members` lookup as `RequireWorkspaceAccess`)
- `ws.connect` permission check via `authzService.Can(actor, "ws.connect")`

No socket should be registered from token validation alone.

Implementation: the WebSocket handler must resolve an `Actor` using the same logic as `RequireWorkspaceAccess` before upgrading the connection. Since WebSocket uses query parameters (`token`, `workspace_id`) rather than headers, the handler calls `authzService` directly instead of relying on Chi middleware.

### 9.8 Backend tests required before rollout

Add automated coverage for:

- Casbin policy correctness: each role receives exactly the permissions defined in §8.2 (test by calling `enforcer.Enforce` for every role-permission combination)
- Casbin role inheritance: verify `member` inherits `viewer`, `manager` inherits `member`, etc.
- active vs pending/inactive/revoked membership access: `RequireWorkspaceAccess` must reject non-active members
- owner-only actions: workspace deletion and owner transfer must reject `admin` and below
- team owner override: `CanManageTeam` grants access to team owners even when workspace role is `member`
- workspace-admin team override: `CanManageTeam` grants access to workspace `owner`/`admin` for any team
- route middleware rejection of cross-workspace requests
- repository methods rejecting raw cross-workspace entity access
- invite acceptance preserving the same `workspace_member_id`
- WebSocket rejection for unauthorized workspace IDs
- GORM adapter persistence: policies survive enforcer restart

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

- add `github.com/casbin/casbin/v2` and `github.com/casbin/gorm-adapter/v3` dependencies
- create `server/internal/authorization/` package with model, policy seed, enforcer, actor, and middleware
- initialize `AuthzService` in `cmd/api/main.go` DI wiring, passing the existing GORM `*gorm.DB`
- seed the baseline role-to-permission policies and role inheritance from §8.2
- implement `RequireWorkspaceAccess` and `RequirePermission` Chi middleware
- add backend tests for Casbin policy correctness, role inheritance, and membership resolution

Exit criteria:

- backend has one canonical authorization layer powered by Casbin
- `casbin_rule` table exists in PostgreSQL with seeded policies
- all §8.2 role-permission mappings are enforced and tested

### Phase 2: Route and repository hardening

- wire `RequireWorkspaceAccess` and `RequirePermission` into all workspace-scoped route groups in `router.go`
- refactor repositories and services to workspace-scoped entity access
- harden WebSocket handler to resolve `Actor` and check `ws.connect` before upgrading
- protect slug lookup with membership check

Exit criteria:

- changing `workspace_id`, slug, or raw entity ID cannot cross workspace boundaries
- every mutation route requires the appropriate Casbin permission check

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
