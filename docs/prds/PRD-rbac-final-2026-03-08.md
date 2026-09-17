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

Note: support ticket visibility, knowledge base article access, and other per-object access control are handled by the relation engine (§9.1.11), not by the RBAC permission catalog above. The RBAC catalog governs module-level access (e.g. "can this user access the support module at all?"), while the relation engine governs object-level access (e.g. "can this user view this specific ticket?").

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

### 9.1 Introduce a centralized authorization package

#### 9.1.1 Internal-first architecture

Because TeamPulse is still internal-only and prelaunch, Phase 1 should optimize for correctness, operability, and speed of iteration rather than introducing a policy engine or external authorization service too early.

TeamPulse authorization still requires two fundamentally different kinds of checks:

1. **Coarse-grained RBAC** — "Does this actor have `pm.edit` in this workspace?" — answered by workspace role to permission mapping
2. **Object-level relationship checks** — "Can this agent view this support ticket?", "Can this team member access this article collection?" — answered by explicit relationships between principals and resources

The implementation for Phase 1 is one internal `AuthzService` boundary backed by two in-process components:

```
AuthzService
├── RBAC engine (Go-native)
│   └── workspace roles -> module permissions
│   └── team-admin checks
│
└── Relation engine (internal, GORM-backed)
    └── principal + resource + relation tuples
    └── support ticket visibility
    └── knowledge base article access
    └── future docs/sharing use cases
```

Handlers never know which component answered. The only boundary they call is `AuthzService`.

This intentionally avoids:

- introducing a second persistent membership source of truth
- introducing a new service dependency before the product launches
- scattering custom authorization conditionals across handlers and repositories

#### 9.1.2 Why this is the right Phase 1 choice

Rationale:

- the immediate security problem is missing workspace membership enforcement, not lack of a general-purpose policy engine
- a small fixed workspace role matrix is simpler and safer to implement directly in Go
- the code stays easier to reason about for internal engineers and coding agents
- `workspace_members` and `team_workspace_memberships` remain the only truth for actor identity and role assignment
- the relation engine still gives TeamPulse a clean path to support ticket and knowledge base sharing now

Alternatives considered and deferred:

- **Casbin now**: useful once role and permission rules become more dynamic, but unnecessary complexity for an internal prelaunch system with a small fixed role matrix
- **OpenFGA / ORY Keto now**: strong fit for larger-scale relationship graphs, but too much operational overhead before launch
- **ad-hoc Go checks in handlers/services**: explicitly rejected because it recreates the authorization drift problem the PRD is trying to eliminate

#### 9.1.3 Package structure

Add a dedicated package:

```
server/internal/authorization/
├── authz.go          # AuthzService interface and composite implementation
├── permissions.go    # permission constants and helpers
├── rbac.go           # Go-native role -> permission matrix
├── relations.go      # relation engine: GORM model, queries, relation methods
├── actor.go          # Actor struct and context helpers
├── middleware.go     # Chi middleware wrappers
```

#### 9.1.4 Go-native RBAC engine (`rbac.go`)

The RBAC engine is an in-memory Go implementation of the permission matrix in §8.2. It does not persist policies to the database and does not maintain a second membership store.

Core principles:

- the actor's role comes from `workspace_members`
- the actor's team-owner status comes from `team_workspace_memberships`
- permissions are derived from role in code
- owner-only exceptions remain explicit in Go code

Suggested shape:

```go
type Permission string

const (
    PermissionWorkspaceRead         Permission = "workspace.read"
    PermissionWorkspaceUpdate       Permission = "workspace.update"
    PermissionWorkspaceDelete       Permission = "workspace.delete"
    PermissionWorkspaceMembersRead  Permission = "workspace.members.read"
    PermissionWorkspaceMembersManage Permission = "workspace.members.manage"
    PermissionWorkspaceInvitesManage Permission = "workspace.invites.manage"
    PermissionWorkspaceRolesManage   Permission = "workspace.roles.manage"
    PermissionSettingsRead          Permission = "settings.read"
    PermissionSettingsManage        Permission = "settings.manage"
    PermissionTeamRead              Permission = "team.read"
    PermissionTeamManage            Permission = "team.manage"
    PermissionTeamMembersRead       Permission = "team.members.read"
    PermissionTeamMembersManage     Permission = "team.members.manage"
    PermissionPMRead                Permission = "pm.read"
    PermissionPMEdit                Permission = "pm.edit"
    PermissionPMAdminWorkflows      Permission = "pm.admin.workflows"
    PermissionPMAdminLabels         Permission = "pm.admin.labels"
    PermissionPMAdminAutomations    Permission = "pm.admin.automations"
    PermissionPMImport              Permission = "pm.import"
    PermissionRewardsRead           Permission = "rewards.read"
    PermissionRewardsManage         Permission = "rewards.manage"
    PermissionSearchRead            Permission = "search.read"
    PermissionWSConnect             Permission = "ws.connect"
)

var rolePermissions = map[string]map[Permission]struct{}{
    "viewer": {
        PermissionWorkspaceRead:        {},
        PermissionSettingsRead:         {},
        PermissionWorkspaceMembersRead: {},
        PermissionTeamRead:             {},
        PermissionTeamMembersRead:      {},
        PermissionPMRead:               {},
        PermissionSearchRead:           {},
        PermissionWSConnect:            {},
    },
    "member": {
        PermissionWorkspaceRead:        {},
        PermissionSettingsRead:         {},
        PermissionWorkspaceMembersRead: {},
        PermissionTeamRead:             {},
        PermissionTeamMembersRead:      {},
        PermissionPMRead:               {},
        PermissionPMEdit:               {},
        PermissionRewardsRead:          {},
        PermissionSearchRead:           {},
        PermissionWSConnect:            {},
    },
    // manager, admin, owner continue the same pattern
}
```

RBAC helper behavior:

- `Can(role, permission)` checks the precomputed role-permission map
- `CanAny(role, permissions...)` checks any match
- `IsOwnerOnly(role)` remains an explicit role check for owner-only actions such as workspace deletion and owner transfer

No role inheritance rows or policy rows are stored in the database. The matrix is versioned with application code.

#### 9.1.5 AuthzService interface (`authz.go`)

The `AuthzService` is the single authorization boundary for the application. Handlers and services call only this interface.

```go
type AuthzService interface {
    // --- RBAC methods ---

    // Can checks whether the actor has a specific permission in their workspace.
    Can(actor *Actor, permission string) bool

    // CanAny checks whether the actor has at least one of the listed permissions.
    CanAny(actor *Actor, permissions ...string) bool

    // CanManageTeam checks team-level management permission.
    // Returns true if the actor is a workspace owner/admin OR a team owner for the given team.
    CanManageTeam(actor *Actor, teamID string) bool

    // IsOwnerOnly checks whether an action requires the workspace owner role specifically.
    IsOwnerOnly(actor *Actor) bool

    // --- Relation methods ---

    // CanAccessResource checks whether the actor can perform a relation-level action on a resource.
    CanAccessResource(actor *Actor, resourceType string, resourceID string, relation string) bool

    // GrantRelation creates a relation tuple.
    GrantRelation(principalType string, principalID string, resourceType string, resourceID string, relation string, workspaceID string) error

    // RevokeRelation removes a relation tuple.
    RevokeRelation(principalType string, principalID string, resourceType string, resourceID string, relation string, workspaceID string) error

    // ListAccessible returns resource IDs of a given type that the actor can access with a given relation.
    ListAccessible(actor *Actor, resourceType string, relation string) ([]string, error)
}
```

The composite implementation wraps both internal components:

```go
type authzServiceImpl struct {
    rbac       *RBACEngine
    relations  *RelationEngine
    memberRepo MemberRepository
}
```

#### 9.1.6 Actor context (`actor.go`)

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

#### 9.1.7 Team owner override

Team-level permissions (`team.manage`, `team.members.manage`) use a two-layer check:

1. the RBAC engine checks the actor's workspace role — workspace `owner` and `admin` always pass
2. If the workspace role does not grant the permission, check `team_workspace_memberships` for team `owner` role on the specific team

This is implemented inside `CanManageTeam` because team ownership is per-team and looked up dynamically from current application data.

#### 9.1.8 Relation engine (`relations.go`)

The relation engine handles all object-level access control: support ticket visibility, knowledge base article access, and any future per-resource sharing.

**GORM model:**

```go
type AuthorizationRelation struct {
    ID            string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
    PrincipalType string `gorm:"type:varchar(50);not null;index:idx_authz_rel_principal"`  // "user", "team", "workspace", "public"
    PrincipalID   string `gorm:"type:varchar(100);not null;index:idx_authz_rel_principal"` // wm_123, team_456, "*"
    ResourceType  string `gorm:"type:varchar(50);not null;index:idx_authz_rel_resource"`   // "ticket", "article", "collection", "inbox"
    ResourceID    string `gorm:"type:varchar(100);not null;index:idx_authz_rel_resource"`  // specific ID
    Relation      string `gorm:"type:varchar(50);not null"`                                // "owner", "editor", "viewer", "assignee"
    WorkspaceID   string `gorm:"type:uuid;not null;index:idx_authz_rel_workspace"`
    CreatedAt     time.Time
}

func (AuthorizationRelation) TableName() string {
    return "authorization_relations"
}
```

The `authorization_relations` table is added to the existing `db.AutoMigrate(...)` call in `cmd/api/main.go` alongside all other application models.

**Principal types:**

| Principal type | Meaning | Example |
|---|---|---|
| `user` | a specific workspace member | `PrincipalID = wm_123` |
| `team` | all members of a team | `PrincipalID = team_456` |
| `workspace` | all active members of the workspace | `PrincipalID = ws_789` |
| `public` | unauthenticated visitors (for public KB articles) | `PrincipalID = *` |

**Resource types (Phase 1):**

| Resource type | Use case |
|---|---|
| `ticket` | support ticket visibility and assignment |
| `article` | knowledge base article access |
| `collection` | knowledge base collection/folder access |
| `inbox` | support inbox/queue access per team |

**Relations:**

| Relation | Meaning |
|---|---|
| `owner` | created or owns the resource; full control |
| `editor` | can modify the resource |
| `viewer` | can read the resource |
| `assignee` | assigned to handle the resource (support tickets) |
| `member` | member of a collection or inbox |

**Visibility patterns:**

Support ticket examples:
```
# Agent X is assigned to ticket T
(user, wm_agent, ticket, ticket_123, assignee, ws_789)

# Team A can view all tickets in their inbox
(team, team_A, inbox, inbox_A, member, ws_789)

# Customer sees only their own ticket (created via owner relation)
(user, wm_customer, ticket, ticket_123, owner, ws_789)

# Managers see escalated tickets via team relation
(team, team_mgr, ticket, ticket_456, viewer, ws_789)
```

Knowledge base article examples:
```
# Public article — visible to unauthenticated visitors
(public, *, article, article_789, viewer, ws_789)

# Internal article — visible to all workspace members
(workspace, ws_789, article, article_790, viewer, ws_789)

# Restricted article — visible only to specific team
(team, team_eng, article, article_791, viewer, ws_789)

# Collection restricted to a team
(team, team_eng, collection, col_123, member, ws_789)
# Articles inherit visibility from their parent collection
```

**CanAccessResource resolution order:**

```go
func (r *RelationEngine) CanAccess(actor *Actor, resourceType, resourceID, relation string) bool {
    // 1. Direct user relation
    if r.hasRelation("user", actor.WorkspaceMemberID, resourceType, resourceID, relation, actor.WorkspaceID) {
        return true
    }
    // 2. Team relation (check all teams the actor belongs to)
    for _, tm := range actor.TeamMemberships {
        if r.hasRelation("team", tm.TeamID, resourceType, resourceID, relation, actor.WorkspaceID) {
            return true
        }
    }
    // 3. Workspace-wide relation
    if r.hasRelation("workspace", actor.WorkspaceID, resourceType, resourceID, relation, actor.WorkspaceID) {
        return true
    }
    // 4. Public relation (only for read/view)
    if relation == "viewer" && r.hasRelation("public", "*", resourceType, resourceID, relation, actor.WorkspaceID) {
        return true
    }
    return false
}
```

**ListAccessible for query filtering:**

Handlers that list resources (e.g. "list tickets for this user") should call `ListAccessible(actor, "ticket", "viewer")` to get a set of resource IDs, then pass those IDs as a `WHERE id IN (...)` filter to the repository query. This keeps authorization out of repository SQL and avoids scattered conditionals.

**Collection inheritance:**

Articles can inherit visibility from their parent collection. When checking access to an article, the relation engine first checks direct article relations, then checks whether the actor has `member` access to the article's parent collection. This is implemented in the engine, not in handlers.

#### 9.1.9 Future migration paths

The architecture deliberately keeps the `AuthzService` boundary stable so either internal component can be replaced later without changing handlers or middleware.

If TeamPulse outgrows the current approach:

- **RBAC migration path**:
  if workspace/module permissions become significantly more dynamic, the RBAC engine can later be replaced with Casbin behind the same `Can` and `CanAny` methods
- **relation migration path**:
  if object-level sharing grows into a larger graph with more complex inheritance or scale needs, `RelationEngine` can later be replaced with OpenFGA or Keto

This is not expected in Phase 1 or Phase 2. The internal Go-native implementation is sufficient for the current internal scale.

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
4. Load the actor's team memberships if needed for team-aware checks, or defer that load lazily
5. Build an `Actor` struct and store it in the request context via `authorization.WithActor(ctx, actor)`
6. Downstream handlers retrieve the actor via `authorization.GetActor(ctx)`

`RequirePermission` behavior:

1. Retrieve the `Actor` from context (fails with 500 if `RequireWorkspaceAccess` was not applied first)
2. Call `authzService.Can(actor, permission)` which delegates to the internal RBAC engine using `actor.Role`
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

RBAC engine tests:

- role-permission matrix correctness: each role receives exactly the permissions defined in §8.2
- owner-only actions: workspace deletion and owner transfer must reject `admin` and below
- team owner override: `CanManageTeam` grants access to team owners even when workspace role is `member`
- workspace-admin team override: `CanManageTeam` grants access to workspace `owner`/`admin` for any team
- no second RBAC source of truth: permission decisions are derived from `workspace_members` role plus team membership, not persisted policy assignments

Relation engine tests:

- direct user relation: user with `viewer` on ticket can access it
- team relation: user in team with `member` on inbox can access inbox tickets
- workspace-wide relation: all active members can access `workspace`-principal articles
- public relation: unauthenticated access to public-principal articles
- collection inheritance: article inherits access from parent collection
- `ListAccessible` returns correct filtered set of resource IDs
- relation grant and revoke lifecycle
- cross-workspace isolation: relations in workspace A do not grant access in workspace B

AuthzService integration tests:

- `Can()` delegates to the RBAC engine, `CanAccessResource()` delegates to the relation engine
- active vs pending/inactive/revoked membership access: `RequireWorkspaceAccess` must reject non-active members
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

### Phase 1: Authorization foundation (RBAC + relation engine)

- create `server/internal/authorization/` package with `AuthzService` interface, Go-native RBAC engine, relation engine, actor, and middleware
- add `AuthorizationRelation` model to `db.AutoMigrate(...)` in `cmd/api/main.go`
- initialize `AuthzService` in `cmd/api/main.go` DI wiring, passing the existing GORM `*gorm.DB`
- implement the baseline role-to-permission matrix from §8.2 in code
- implement `RequireWorkspaceAccess` and `RequirePermission` Chi middleware
- add backend tests for RBAC matrix correctness, relation engine tuples, and membership resolution

Exit criteria:

- backend has one canonical `AuthzService` with two internal components (RBAC engine + relation engine)
- `authorization_relations` exists in PostgreSQL
- all §8.2 role-permission mappings are enforced and tested
- relation engine can grant, revoke, check, and list resource access

### Phase 2: Route and repository hardening

- wire `RequireWorkspaceAccess` and `RequirePermission` into all workspace-scoped route groups in `router.go`
- refactor repositories and services to workspace-scoped entity access
- harden WebSocket handler to resolve `Actor` and check `ws.connect` before upgrading
- protect slug lookup with membership check
- integrate relation engine into support ticket handlers: create ticket → `GrantRelation`, list tickets → `ListAccessible` filter

Exit criteria:

- changing `workspace_id`, slug, or raw entity ID cannot cross workspace boundaries
- every mutation route requires the appropriate centralized permission check
- support ticket visibility is enforced through the relation engine, not ad-hoc conditionals

### Phase 3: Frontend permission integration

- ship `GET /api/workspaces/{id}/me`
- replace frontend role heuristics with permission checks
- update route guards, nav, and page editability

Exit criteria:

- frontend only offers actions the backend would allow

### Phase 4: Knowledge base and object-level sharing

- integrate relation engine into knowledge base: article and collection visibility via relation tuples
- implement collection inheritance (articles inherit from parent collection)
- support public / internal / restricted visibility levels via principal types
- add relation management UI for team-scoped and collection-scoped sharing

Exit criteria:

- knowledge base access is enforced through the relation engine
- no ad-hoc visibility conditionals in handlers or services

### Phase 5: Team RBAC expansion and deferred model work

- expand team-specific permission enforcement
- add guest support if still needed
- add richer team privacy or access-mode controls if still needed
- evaluate migration to Casbin for RBAC or OpenFGA/Keto for object relations only if internal usage proves the current implementation is no longer sufficient

Exit criteria:

- team-scoped authorization is enforced without changing the core identity model

## 13. Acceptance Criteria

RBAC is considered complete for this PRD when all of the following are true:

- every workspace-scoped backend route resolves and enforces active workspace membership
- every privileged backend action uses centralized permission checks via `AuthzService`
- every entity read/write path is workspace-scoped
- WebSocket access is authorized by workspace membership and permission
- frontend consumes backend-resolved permissions instead of only local role labels
- settings, member-management, and PM admin surfaces are permission-gated end to end
- support ticket visibility is enforced through the relation engine, not ad-hoc conditionals
- knowledge base article access is enforced through the relation engine with proper visibility levels
- no new authorization logic bypasses the `AuthzService` interface (no scattered `if isPublic || owner || inGroup` in handlers)
- no new RBAC work relies on `workspace_people`, `team_memberships`, or `team_user_memberships`
- automated tests cover role differences, lifecycle status differences, relation tuples, and cross-workspace denial

## 14. Final Conclusion

The correct final authorization plan for TeamPulse is:

- keep the unified identity model already built around `workspace_members`
- stop treating old tables as active design inputs
- build centralized server-side authorization with a single `AuthzService` interface backed by two engines:
  - a Go-native RBAC engine for coarse-grained workspace and module permissions
  - an internal relation engine for object-level access (support tickets, knowledge base articles, future sharing)
- then move the frontend to backend-resolved permissions
- only after that, consider guest access, richer team RBAC, or migration to Casbin/OpenFGA/Keto if real usage justifies it
- never embed authorization decisions directly in handlers or services — all checks go through `AuthzService`

That architecture fixes the real security gap, avoids the two-authorization-system trap, and provides a clean migration path to a Zanzibar-style system if scale demands it later.
