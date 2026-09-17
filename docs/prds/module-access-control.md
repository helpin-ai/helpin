# Module access control by team and member

**Product**: Helpin  
**Date**: April 7, 2026  
**Status**: Draft  
**Owner**: Product / Engineering

## 1. Summary

Helpin should show workspace modules only to the people who actually need them.

The recommended model is:

- module access granted primarily by team
- direct member grants for exceptions
- workspace `owner` and `admin` retain full access
- Support keeps its existing inbox/mailbox restriction layer inside the module

This PRD defines a simple, scalable access model for:

- `pm`
- `docs`
- `crm`
- `support`

The main product goal is to stop treating module visibility as a frontend-only concern and make it a real backend-enforced authorization layer.

## 1.1 V1 Scope Decision

V1 of this system applies to:

- `crm`
- `support`

PM and Docs remain broadly available in v1 and are not required to use explicit module grants yet.

### Why

This keeps the first rollout focused on the modules that are operationally specialized and most in need of tighter visibility rules.

## 2. Current State

### What exists today

- workspace access is resolved through `RequireWorkspaceAccess`, which loads current workspace membership and current team memberships on each request
- effective permissions returned by `/workspaces/{id}/me` are built from workspace role only
- the frontend has a temporary module visibility allowlist for some modules
- Support already has mailbox-level access control through linked teams and explicit mailbox members

### Practical consequence

Today module access is inconsistent:

- some modules are hidden in the UI
- backend authorization is still broad for role-based access
- module visibility is not yet a first-class product concept

This is acceptable as a temporary phase, but not as the long-term model.

## 3. Problem

Not every workspace member should see or use every module.

Examples:

- engineers may not need CRM
- sales may not need Support
- many members may not need either
- some operators or founders may still need exceptions

Without a proper module access layer:

- the sidebar becomes noisy
- users see tools they do not need
- route-level access is harder to reason about
- frontend visibility and backend authorization can drift apart

## 4. Goals

1. Show modules only to users who should have them.
2. Grant access primarily by team.
3. Allow direct member grants for exceptions.
4. Keep module definitions in code, not in the database.
5. Enforce access in both frontend and backend.
6. Preserve Support mailbox/inbox restrictions as a second layer.
7. Make team membership changes take effect immediately.
8. Give admins a dedicated Access Page to manage module grants.

## 5. Non-Goals

- billing-plan-based module enablement
- custom permission builders for every module in v1
- deep CRM record-level scoping in v1
- generalizing Support mailbox scoping to all modules in v1
- replacing workspace role RBAC entirely

## 6. Product Principles

1. Workspace is the security boundary.
2. Team is the default operational boundary.
3. Module access should be simple to explain.
4. Direct user grants should exist, but only as exceptions.
5. Frontend should reflect access, but backend must enforce it.
6. Support is special because inbox access is narrower than module access.

## 7. Core Product Decision

The product should use:

- team-based module access by default
- direct member grants for exceptions
- admin override for workspace `owner` and `admin`

This means:

- if a team has CRM access, its members see CRM
- if a team has Support access, its members see Support
- if a specific member is granted CRM or Support directly, they also see it
- if a member has no grant through team or direct assignment, they do not see the module

## 8. Module List

The module list must be code-defined, not DB-defined.

Canonical module IDs:

- `pm`
- `docs`
- `crm`
- `support`

The database stores access grants only. It does not store a mutable table of module definitions.

### Why

This avoids:

- orphaned module rows
- admin-managed module catalog drift
- deployment complexity when adding or removing modules

## 9. Default Behavior For New Workspaces

Recommended defaults:

- `pm` visible to all active workspace members in v1 without explicit grants
- `docs` visible to all active workspace members in v1 without explicit grants
- `crm` visible to `owner` and `admin` only until grants are added
- `support` visible to `owner` and `admin` only until grants are added

### Why

PM and Docs are core workspace surfaces.

CRM and Support are more operationally specialized. Making them broadly visible by default would create noise for most workspaces and most users.

### Explicit v1 implementation choice

PM and Docs do **not** use the grant table in v1.

This means:

- no sentinel `all members` grant is needed
- no seeding of grants for every member is needed
- PM and Docs remain outside the explicit module-grant evaluation path in v1

This keeps the rollout small and aligned with the Phase 3 recommendation to optionally adopt the same model for PM and Docs later.

## 10. Access Model

## 10.1 Decision Order

A user can access a module if any of these are true:

1. user is workspace `owner`
2. user is workspace `admin`
3. user has a direct member grant for the module
4. user belongs to a team that has a grant for the module

Otherwise access is denied.

## 10.2 Support Special Case

Support uses two layers:

1. module access decides whether the user can enter Support
2. mailbox/inbox access decides which conversations the user can actually see

This is the correct boundary for Support and should be preserved.

CRM and Docs do not need a second layer in v1.

## 10.3 Team Removal Behavior

If a user is removed from a team:

- module access must be recomputed on the next request
- no stale long-lived access should remain

This fits the current request-time membership resolution model already used by workspace access middleware.

## 11. ASCII Diagram

```text
                   +----------------------------------+
                   |          Workspace User          |
                   +----------------+-----------------+
                                    |
                                    v
                   +----------------------------------+
                   |   Is user owner/admin in this    |
                   |            workspace?            |
                   +-----------+----------------------+
                               |
                     yes ------+------> allow all modules
                               |
                               no
                               |
                               v
          +------------------------------------------------------+
          | Check module access grants for this module           |
          |                                                      |
          | - direct member grant?                               |
          | - any team grant for one of the user's teams?        |
          +----------------------+-------------------------------+
                                 |
                       yes ------+------> module visible + allowed
                                 |
                                 no
                                 |
                                 v
                           module hidden + denied
```

Support flow:

```text
User has Support module access
             |
             v
      Enter Support module
             |
             v
   Check mailbox / inbox access
             |
    +--------+---------+
    |                  |
 allowed            not allowed
    |                  |
    v                  v
see allowed      do not show those
inboxes only     inboxes/conversations
```

Admin management flow:

```text
Access Page
    |
    +--> Module: CRM
    |      +--> grant to team: Sales
    |      +--> grant to team: Customer Success
    |      +--> grant to member: Founder
    |
    +--> Module: Support
           +--> grant to team: Support
           +--> grant to member: Ops Lead
```

## 12. Access Page

Yes, the product should have an **Access Page**.

This should be the main admin surface for module access management.

Recommended location:

- Workspace Settings
- Section name: `Access`

Recommended responsibilities:

- show each module and who has access
- add/remove team grants
- add/remove direct member grants
- explain the difference between:
  - module access
  - Support inbox access

### Why a dedicated page is the right approach

Without an Access Page:

- admins will not have a clear place to manage module visibility
- module access logic will become scattered across module settings pages
- Support and CRM onboarding will be harder to explain

### What the Access Page should not do

- it should not manage Support mailbox membership directly
- it should not manage billing/plan enablement
- it should not redefine the module catalog

Mailbox and inbox restrictions should remain in Support settings where they belong.

## 13. User Stories

- As a workspace admin, I want Sales to have CRM automatically through team membership.
- As a workspace admin, I want Support staff to have Support automatically through team membership.
- As a workspace admin, I want to grant CRM or Support directly to a specific person without moving them to another team.
- As a workspace member, I should only see modules relevant to my role.
- As a Support agent, I should only see the inboxes I am allowed to use.
- As an admin, when I remove someone from a team, their module access should update immediately.

## 14. UX Behavior

## 14.1 Sidebar

The sidebar is the main frontend consumer.

It should:

- show only modules the current user can access
- hide inaccessible modules entirely

## 14.2 Route Guards

If a user navigates directly to a module route they cannot access:

- frontend should redirect or show forbidden state
- backend must still return `403`

## 14.3 Admin Experience

On the Access Page, admins should be able to:

- pick a module
- grant access to one or more teams
- grant access to individual members
- revoke team or direct grants
- understand default behavior clearly

## 14.4 Support Experience

A user with Support access but no mailbox access:

- may enter the Support shell only if product chooses to allow that state
- should never see restricted conversations

Preferred v1 behavior:

- if user has zero accessible inbox scope, show an empty restricted state inside Support

## 15. Functional Requirements

1. Module access must be a backend-enforced authorization layer.
2. Module access must support grants to teams.
3. Module access must support grants to individual workspace members.
4. Workspace `owner` and `admin` must bypass module grants.
5. Module list must be code-defined.
6. Sidebar must render from real module access state.
7. Route-level backend protection must exist regardless of sidebar state.
8. Support mailbox access must remain a second layer inside Support.
9. Team membership changes must affect module access immediately.
10. Access management must be available from a dedicated Access Page in workspace settings.

## 16. Proposed Data Model

Recommended table:

```text
workspace_module_grants
- id
- workspace_id
- module
- subject_type        // team | workspace_member
- subject_id
- created_by_id
- created_at
- updated_at
```

Recommended uniqueness:

```text
unique(workspace_id, module, subject_type, subject_id)
```

### Notes

- `module` values are validated against code constants
- `subject_type` is intentionally narrow in v1
- no separate `modules` table is needed

### Subject identity decision

`subject_type` must be:

- `team`
- `workspace_member`

`subject_id` must always hold a **workspace-scoped ID**:

- for `team`, use `workspace_teams.id`
- for `workspace_member`, use `workspace_members.id`

It must **not** store global `users.id`.

### Why this matters

Using `workspace_member.id` instead of `user.id` makes the model consistent with workspace-scoped access and lifecycle operations:

- direct grants can be resolved using the same workspace membership object already used by authorization
- grants can be cleaned up automatically when a workspace member is removed
- joins stay workspace-local and predictable

### Referential cleanup

Grant rows should be protected from orphaning.

Recommended behavior:

- foreign key from `subject_id` to `workspace_teams.id` when `subject_type = team`
- foreign key from `subject_id` to `workspace_members.id` when `subject_type = workspace_member`
- cleanup on member or team deletion should cascade

If polymorphic foreign keys are inconvenient in the first migration, the implementation must still guarantee cleanup in service/repository code on:

- team deletion
- workspace member deletion

## 17. Backend Design

Introduce a dedicated module access service with functions like:

- `CanAccessModule(actor, module)`
- `ListAccessibleModules(actor)`

Introduce route middleware:

- `RequireModuleAccess(module)`

Recommended request chain:

```text
RequireWorkspaceAccess -> RequireModuleAccess("crm") -> CRM handler
RequireWorkspaceAccess -> RequireModuleAccess("support") -> Support handler
```

Support routes keep their existing mailbox checks after module access is granted.

### New permission for grant management

Add a dedicated permission:

- `module_access.manage`

This permission should gate:

- Access Page visibility
- grant create/update/delete APIs

### Why

`settings.manage` is broader than the feature actually needs.

Using a dedicated permission keeps module access management explicit and allows future narrowing if needed.

## 18. Frontend Design

The frontend follows existing app patterns: `SettingsPageFrame` layout, `Dialog` for mutations, `Table` for lists, `Popover` + checkboxes for multi-select, and sidebar filtering via `access.team_memberships`.

### 18.1 Data Source

Expose accessible modules in the existing `/workspaces/{id}/me` response.

This is the preferred v1 shape because it gives:

- one fetch
- one cache key
- one invalidation path
- one source of truth for workspace access

Recommended response shape:

```json
{
  "modules": ["pm", "docs", "support"]
}
```

Recommended semantics:

- the server returns only modules the user can access
- the frontend keeps the canonical ordered module catalog in code
- the frontend renders only the intersection of:
  - known app modules
  - accessible modules from `/me`

Frontend consumers:

- sidebar
- route guards
- empty/forbidden states
- Access Page management UI

### 18.2 UI Surface Priority

| Surface | Priority | What it does |
|---------|----------|-------------|
| Access Page (Settings) | P0 | Primary grant management |
| Sidebar filtering | P0 | Hide inaccessible modules |
| Route guard / 403 page | P0 | Block direct navigation |
| Team detail module tab | P1 | Secondary grant management |
| Member profile access view | P2 | Read-only visibility |

### 18.3 Access Page (Settings → Access)

New section in `SettingsPageFrame`, routed as `/w/$slug/settings/access`.

PM and Docs show as collapsed rows with "All members" badge (open by default in v1). CRM and Support show expandable cards with team/member grant tables.

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Settings Rail Nav          │  Access                                 │
│ ─────────────────          │  Manage which teams and members can     │
│ General                    │  access each module.                    │
│ Members                    │                                         │
│ Teams                      │  ┌──────────────────────────────────┐   │
│ Labels                     │  │ PM                    All members │   │
│ Workflows                  │  └──────────────────────────────────┘   │
│ ▸ Access  ← (new)         │                                         │
│ Notifications              │  ┌──────────────────────────────────┐   │
│                            │  │ Docs                  All members │   │
│                            │  └──────────────────────────────────┘   │
│                            │                                         │
│                            │  ┌──────────────────────────────────┐   │
│                            │  │ CRM                    [Manage →] │   │
│                            │  │                                  │   │
│                            │  │  Teams                           │   │
│                            │  │  ┌────────────────────────────┐  │   │
│                            │  │  │ Name           │ Actions   │  │   │
│                            │  │  ├────────────────┼───────────┤  │   │
│                            │  │  │ Sales          │    ×      │  │   │
│                            │  │  │ Cust. Success  │    ×      │  │   │
│                            │  │  └────────────────┴───────────┘  │   │
│                            │  │  [+ Add team]                    │   │
│                            │  │                                  │   │
│                            │  │  Members                         │   │
│                            │  │  ┌────────────────────────────┐  │   │
│                            │  │  │ Name           │ Actions   │  │   │
│                            │  │  ├────────────────┼───────────┤  │   │
│                            │  │  │ Sarah (CEO)    │    ×      │  │   │
│                            │  │  └────────────────┴───────────┘  │   │
│                            │  │  [+ Add member]                  │   │
│                            │  │                                  │   │
│                            │  │  ℹ Owners and admins always      │   │
│                            │  │    have access.                  │   │
│                            │  └──────────────────────────────────┘   │
│                            │                                         │
│                            │  ┌──────────────────────────────────┐   │
│                            │  │ Support                [Manage →] │   │
│                            │  │                                  │   │
│                            │  │  Teams                           │   │
│                            │  │  ┌────────────────────────────┐  │   │
│                            │  │  │ Support Team   │    ×      │  │   │
│                            │  │  └────────────────┴───────────┘  │   │
│                            │  │  [+ Add team]                    │   │
│                            │  │                                  │   │
│                            │  │  Members                         │   │
│                            │  │  ┌────────────────────────────┐  │   │
│                            │  │  │ Ops Lead       │    ×      │  │   │
│                            │  │  └────────────────┴───────────┘  │   │
│                            │  │  [+ Add member]                  │   │
│                            │  │                                  │   │
│                            │  │  ℹ Inbox-level access is managed │   │
│                            │  │    in Support → Settings.        │   │
│                            │  └──────────────────────────────────┘   │
│                            │                                         │
└────────────────────────────┴─────────────────────────────────────────┘
```

### 18.4 Add Team Dialog

Triggered by `[+ Add team]` on the Access Page. Uses `Dialog` + `Popover` with checkboxes, consistent with existing multi-select patterns.

```text
┌──────────────────────────────────────┐
│  Add team to CRM                  ×  │
├──────────────────────────────────────┤
│                                      │
│  Select teams                        │
│  ┌──────────────────────────────┐    │
│  │ 🔍 Search teams...           │    │
│  ├──────────────────────────────┤    │
│  │ ☑ Sales                      │    │
│  │ ☑ Customer Success           │    │
│  │ ☐ Engineering                │    │
│  │ ☐ Support Team               │    │
│  │ ☐ Design                     │    │
│  └──────────────────────────────┘    │
│                                      │
│  Already added teams are checked     │
│  and disabled.                       │
│                                      │
│              [Cancel]  [Add teams]   │
└──────────────────────────────────────┘
```

### 18.5 Add Member Dialog

Triggered by `[+ Add member]`. Members who already have access via team grants are shown but not selectable.

```text
┌──────────────────────────────────────┐
│  Add member to CRM               ×  │
├──────────────────────────────────────┤
│                                      │
│  Select members                      │
│  ┌──────────────────────────────┐    │
│  │ 🔍 Search members...         │    │
│  ├──────────────────────────────┤    │
│  │ ☐ Alice (alice@acme.com)     │    │
│  │ ☐ Bob (bob@acme.com)         │    │
│  │ ☐ Dave (dave@acme.com)       │    │
│  │ ── already have access ──    │    │
│  │ ☑ Sarah (via direct grant)   │    │
│  │ ░ Carol (via Sales team)     │    │
│  └──────────────────────────────┘    │
│                                      │
│  Members with team access are shown  │
│  but not selectable.                 │
│                                      │
│             [Cancel]  [Add members]  │
└──────────────────────────────────────┘
```

### 18.6 Team Detail Dialog — Module Access Tab

New tab in the existing team detail `Dialog` alongside Details, Estimates, Sprints. PM and Docs are shown checked and locked since they are open to all members in v1.

```text
┌───────────────────────────────────────────────┐
│  Team: Sales                               ×  │
├───────────────────────────────────────────────┤
│                                               │
│  Details │ Estimates │ Sprints │ Modules ← new│
│  ─────────────────────────────  ───────       │
│                                               │
│  Module access                                │
│  This team's members will see these modules.  │
│                                               │
│  ☑ PM               (all members — locked)    │
│  ☑ Docs             (all members — locked)    │
│  ☑ CRM                                       │
│  ☐ Support                                    │
│                                               │
│  ℹ PM and Docs are currently visible to all   │
│    workspace members.                         │
│                                               │
│                           [Cancel]  [Save]    │
└───────────────────────────────────────────────┘
```

### 18.7 Sidebar — Module Visibility

The sidebar already filters teams by `access.team_memberships`. Module filtering extends this by checking the `modules` list from `/workspaces/{id}/me`. Inaccessible modules are not rendered (no locked icons or upsell states in v1).

```text
ADMIN sees:                     SALES MEMBER sees:
┌─────────────────────┐        ┌─────────────────────┐
│ 🏢 Acme Inc          │        │ 🏢 Acme Inc          │
│                     │        │                     │
│ Projects            │        │ Projects            │
│  ▸ Sales            │        │  ▸ Sales            │
│  ▸ Engineering      │        │                     │
│  ▸ Design           │        │ Docs                │
│                     │        │                     │
│ Docs                │        │ CRM                 │
│                     │        │  Contacts           │
│ CRM                 │        │  Deals              │
│  Contacts           │        │  Pipeline           │
│  Deals              │        │                     │
│  Pipeline           │        │                     │
│                     │        │                     │
│ Support             │        │                     │
│  Inboxes            │        │                     │
│  Tickets            │        │                     │
│                     │        │                     │
│ Agents              │        │ Agents              │
│                     │        │                     │
│ ──────────          │        │ ──────────          │
│ ⚙ Settings          │        │                     │
└─────────────────────┘        └─────────────────────┘
```

### 18.8 Forbidden State

If a user navigates directly to a module URL they cannot access, the frontend shows a centered message inside the main content area. The sidebar remains visible. The backend returns `403`.

```text
┌──────────────────────────────────────────────────┐
│  Sidebar  │                                      │
│           │                                      │
│  Projects │                                      │
│  Docs     │     🔒                               │
│  CRM      │                                      │
│           │     You don't have access to Support  │
│           │                                      │
│           │     Ask a workspace admin to grant    │
│           │     you access.                      │
│           │                                      │
│           │     [Back to Dashboard]               │
│           │                                      │
│           │                                      │
└───────────┴──────────────────────────────────────┘
```

## 19. Support Scope Boundary

Support is the correct place to keep a second access boundary.

The product should explicitly distinguish:

- module access: "Can you use Support?"
- inbox access: "Which Support inboxes can you use?"

This distinction should be visible in admin copy and product help text.

## 20. Rollout Plan

Phase 1:

- create grant table
- add backend service for module access
- expose accessible modules from `/workspaces/{id}/me`
- build Access Page UI

Phase 2:

- wire sidebar to real module access
- add route-level backend module checks for CRM and Support
- keep existing Support mailbox restrictions

Phase 3:

- remove temporary frontend email allowlists
- optionally adopt same module access model more explicitly for PM and Docs

## 21. Risks

### Risk 1: confusion between module access and Support inbox access

Mitigation:

- explain the difference in the Access Page
- keep mailbox configuration in Support settings

### Risk 2: stale frontend visibility after grant changes

Mitigation:

- invalidate workspace access/module-access queries after updates

### Risk 3: too much complexity too early

Mitigation:

- keep v1 narrow
- module-level access for CRM and Support first
- mailbox-level extra scope only for Support

## 22. Acceptance Criteria

- Members only see CRM and Support when granted directly or through team access.
- Workspace `owner` and `admin` see all modules.
- CRM and Support routes return `403` when user lacks module access.
- Sidebar hides modules user cannot access.
- Team membership changes affect module access on the next request.
- Support users only see inboxes and conversations they have mailbox access to.
- Module catalog is defined in code, not in database rows.
- Admins can manage grants from a dedicated Access Page.
- Accessible modules are returned from `/workspaces/{id}/me`.
- Direct grants are stored against `workspace_member.id`, not `user.id`.
- Grant cleanup happens when a referenced team or workspace member is removed.

## 23. Recommendation

Ship v1 with:

- code-defined module constants
- team-based grants
- direct member grants
- owner/admin bypass
- a dedicated Access Page
- a dedicated `module_access.manage` permission for the Access Page and grant APIs
- `/workspaces/{id}/me` returning only the accessible module list
- Support mailbox restrictions preserved as the second layer

This is the simplest product model that is easy to explain:

- team access gives the module by default
- direct grants handle exceptions
- Support inbox access adds operational scoping where needed
