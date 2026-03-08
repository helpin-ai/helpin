> Superseded by [PRD-rbac-final-2026-03-08.md](/root/teampulse/docs/PRD-rbac-final-2026-03-08.md). This file is retained as an earlier draft.

# PRD: RBAC For Members And Roles

**Product**: Helpin  
**Date**: March 5, 2026  
**Status**: Draft  
**Owner**: Product / Engineering  
**Inspired by**: Linear member and role model, adapted for Helpin

## 1. Summary

Helpin needs a formal RBAC system for workspace membership, member lifecycle, and team-scoped administration. Today the product already supports workspace roles in the shape of `owner`, `admin`, `manager`, `member`, and `viewer`, plus workspace invitations.

This PRD defines the next version of RBAC so Helpin can:

- manage members safely at workspace and team level
- support suspended members and historical visibility
- add a true team-owner role
- add guest users with team-scoped access
- enforce permissions consistently across frontend, backend, and settings UI

The design should feel familiar to teams coming from Linear, but should fit Helpin’s current architecture and terminology.

## 2. Current State

### Existing capabilities

- Workspace membership exists in `workspace_members`
- Current workspace roles are:
  - `owner`
  - `admin`
  - `manager`
  - `member`
  - `viewer`
- Invitations support:
  - `admin`
  - `manager`
  - `member`
  - `viewer`
- Settings already has a Members section and invitation flow

### Existing code references

- Workspace member model: [workspace.go](/root/helpin/server/internal/model/workspace.go:16)
- Workspace member migration: [002_workspaces.sql](/root/helpin/server/migrations/002_workspaces.sql:12)
- Invitation model: [invitation.go](/root/helpin/server/internal/model/invitation.go:5)
- Invitation migration: [016_workspace_invitations.sql](/root/helpin/server/migrations/016_workspace_invitations.sql:5)
- Current frontend role type: [types.ts](/root/helpin/frontend/src/lib/types.ts:21)
- Current session helpers: [sessionStore.ts](/root/helpin/frontend/src/stores/sessionStore.ts:1)

## 3. Problem

The current membership system is too shallow for the product Helpin is becoming.

Current gaps:

- no explicit member status model beyond invitations
- no suspension flow at membership level
- no guest role
- no team owner role
- no team-scoped permission controls
- no team handle model for `@team` mentions
- no team-scoped PM configuration model
- no centralized permission matrix across product surfaces
- inconsistent authorization patterns in the backend

## 4. Goals

1. Introduce a durable RBAC model for workspace and team access.
2. Preserve current roles where possible to minimize migration risk.
3. Make member lifecycle states explicit: invited, active, suspended, left.
4. Support delegated team administration without granting workspace-wide admin access.
5. Support external collaborators through guest accounts with team-scoped access.
6. Make authorization enforceable server-side for every workspace and team-scoped action.

## 5. Non-Goals

- Full SCIM implementation in the first release
- Billing-plan enforcement in the first release
- Fine-grained per-feature custom role builder
- OAuth app approval workflows
- SSO group sync

## 6. Role Model

## 6.1 Workspace Roles

Helpin will support these workspace-level roles:

| Role | Scope | Summary |
|------|-------|---------|
| `workspace_owner` | Workspace | Highest privilege, including billing, security, exports, audit, role policy |
| `admin` | Workspace | Operational workspace admin, but below owner |
| `manager` | Workspace | Business/team leadership permissions, but not full admin |
| `member` | Workspace | Standard full collaborator |
| `viewer` | Workspace | Read-only workspace access |
| `guest` | Team-scoped | Limited collaborator with access only to explicitly granted teams |

### Note on migration

Current `owner` should map to `workspace_owner`.

To reduce migration churn, implementation may either:

1. Rename `owner` to `workspace_owner`, or
2. Keep `owner` in storage and treat it as the workspace-owner canonical value

Recommendation: keep `owner` in v1 storage and expose user-facing label “Workspace Owner”.

## 6.2 Team Roles

Team membership should support a second dimension of access:

| Team Role | Scope | Summary |
|-----------|-------|---------|
| `team_owner` | Team | Delegated admin for team settings, team membership, and restricted team operations |
| `team_member` | Team | Standard team participant |
| `guest_team_member` | Team | Guest-level participant in an allowed team |

Rules:

- Workspace owners and admins are implicitly team owners for all teams they can access
- Team creators become team owners by default
- Team owners of a parent team can be treated as team owners of child teams if Helpin adds hierarchical teams
- Guests cannot become team owners

### Team identity requirements

Each team should also have a collaboration identity inside the workspace:

- `name` for display
- `handle` for mentions, e.g. `@growth`
- optional `description`
- access mode and permission settings

Team handles must be unique within a workspace.

## 7. Member Status Model

Each workspace membership should have a status:

| Status | Meaning |
|--------|---------|
| `invited` | Invite exists but membership not yet active |
| `active` | User has access |
| `suspended` | User loses access immediately but remains visible for history |
| `left` | User is no longer active in the workspace, kept for historical attribution |

### Behavior

- Suspended users lose all access immediately
- Suspended users remain visible in members list, issue activity, comments, ownership history, and reporting
- Left users remain historically visible but cannot authenticate into the workspace
- Invitations are separate records, but activation should result in an `active` workspace membership

## 8. Permission Model

## 8.1 Workspace-Level Permissions

| Permission | Owner | Admin | Manager | Member | Viewer | Guest |
|------------|-------|-------|---------|--------|--------|-------|
| View workspace | Yes | Yes | Yes | Yes | Yes | Limited |
| Manage billing | Yes | No | No | No | No | No |
| Manage security settings | Yes | No | No | No | No | No |
| View audit log | Yes | Optional | No | No | No | No |
| Export workspace data | Yes | Optional | No | No | No | No |
| Manage members | Yes | Yes | No | No | No | No |
| Change workspace roles | Yes | Yes, except owner | No | No | No | No |
| Suspend members | Yes | Yes | No | No | No | No |
| Create teams | Yes | Yes | Optional | No | No | No |
| Manage global PM settings | Yes | Yes | Limited | No | No | No |
| Access workspace-wide views/reports | Yes | Yes | Yes | Yes | Read-only | No |

## 8.2 Team-Level Permissions

| Permission | Workspace Owner/Admin | Team Owner | Team Member | Viewer | Guest |
|------------|-----------------------|------------|-------------|--------|-------|
| View team data | Yes | Yes | Yes | Yes | Yes, only assigned teams |
| Join open team | Yes | Yes | Configurable | No | No |
| Invite member to team | Yes | Configurable | Configurable | No | No |
| Invite guest to team | Yes | Yes | No | No | No |
| Manage team settings | Yes | Configurable | Configurable | No | No |
| Manage team labels/templates | Yes | Configurable | Configurable | No | No |
| Delete team | Yes | Yes | No | No | No |
| Make team private | Yes | Yes | No | No | No |
| Change team parent | Yes | Yes | No | No | No |

## 8.3 PM-Specific Permissions

| PM Action | Owner/Admin | Manager | Member | Viewer | Guest |
|----------|-------------|---------|--------|--------|-------|
| View stories/epics/sprints/objectives | Yes | Yes | Yes | Yes | Only allowed teams |
| Create/edit stories | Yes | Yes | Yes | No | Yes, in allowed teams |
| Create/edit epics | Yes | Yes | Yes | No | Team-scoped only |
| Create/edit objectives | Yes | Yes | Optional | No | No |
| Manage workflows | Yes | Yes | No | No | No |
| Manage custom fields/templates | Yes | Yes | No | No | No |
| Comment | Yes | Yes | Yes | No | Yes, in allowed teams |
| Upload attachments | Yes | Yes | Yes | No | Yes, in allowed teams |
| View workspace-wide reports | Yes | Yes | Yes | Read-only | No |

## 9. Team Access Controls

Each team should support access and permission controls similar to Linear:

### 9.1 Team Access Mode

| Mode | Behavior |
|------|----------|
| `open` | Any active workspace member can join |
| `restricted` | Only invited/assigned members can join |
| `private` | Only explicitly added members and owners can access |

### 9.2 Team Permission Switches

Per team, configure whether these are allowed for `all_team_members` or `team_owners_only`:

- label management
- template management
- team settings management
- member management
- workflow management
- automation management

Rule:

- Adding guests is always restricted to workspace owners/admins and team owners

## 9.3 Team Collaboration Settings

The following PM capabilities should be team-scoped by default:

- team handle / mention identity
- team labels
- team workflows and states
- team templates
- team automations
- team triage and intake rules

The following remain workspace-scoped:

- workspace roles
- invites and member lifecycle
- billing/security/audit
- integrations
- global defaults

## 10. Guest Access Model

Guests are intended for contractors, advisors, clients, and external collaborators.

Guests can:

- access issues, docs, attachments, comments, and projects for teams they are explicitly assigned to
- take the same day-to-day actions as members inside those teams, unless team settings restrict them

Guests cannot:

- access workspace-wide settings
- access cross-workspace search results outside their teams
- access workspace-wide reports, finance, bonus, or administration surfaces
- manage members or change roles
- be promoted to workspace owner/admin
- be promoted to team owner

## 11. Member Lifecycle Flows

## 11.1 Invite Member

1. Admin or owner opens Settings > Members
2. Clicks Invite Member
3. Enters email
4. Chooses role
5. If guest, chooses one or more teams
6. System creates invitation record
7. On accept, system creates active membership and team assignments

## 11.2 Change Member Role

1. Admin or owner opens overflow menu on member row
2. Chooses Change role
3. System validates:
   - owner cannot be removed if they are last owner
   - guest cannot be upgraded to team owner directly without team membership flow
4. System records audit log entry

## 11.3 Suspend Member

1. Admin or owner opens overflow menu
2. Chooses Suspend member
3. Membership status becomes `suspended`
4. Sessions are revoked immediately
5. User remains visible in member directory and historical objects

## 11.4 Reinstate Member

1. Admin or owner opens suspended member row
2. Chooses Reinstate
3. Membership status becomes `active`
4. Prior team assignments can be restored optionally

## 11.5 Team Owner Promotion

1. Team owner/admin opens Team settings > Members
2. Promotes an active team member to team owner
3. Audit event is recorded

## 12. Data Model Changes

## 12.1 Workspace Membership

Extend `workspace_members`:

- `role`
- `status`
- `suspended_at`
- `suspended_by`
- `left_at`
- `last_active_at` optional

Suggested enums:

- `role IN ('owner','admin','manager','member','viewer','guest')`
- `status IN ('active','suspended','left')`

Note: `invited` should continue to live in `workspace_invitations`, not in `workspace_members`.

## 12.2 Team Membership

Add or extend a team-membership table to include:

- `workspace_id`
- `team_id`
- `user_id`
- `team_role` (`member`, `owner`)
- `access_source` (`direct`, `inherited`, `guest_assignment`, `workspace_admin`)
- `created_at`
- `updated_at`

This should be tied to authenticated workspace members, not to HR/performance-only people records.

## 12.3 Team Permission Settings

Add team-level settings table, for example `team_access_policies`:

- `team_id`
- `access_mode`
- `label_management_scope`
- `template_management_scope`
- `team_settings_scope`
- `member_management_scope`
- `workflow_management_scope`
- `automation_management_scope`

## 12.4 Team Identity And Mentioning

Extend teams with:

- `handle`
- `is_private`
- `access_mode`

Add mention support that resolves `@handle` to a team entity and fans out notifications to active team members with access.

## 12.5 PM Label Scope

PM labels should support team ownership.

Recommended model:

- `scope` enum: `workspace` or `team`
- nullable `team_id`

Behavior:

- team labels are visible within their team context
- workspace labels are optional shared taxonomy
- duplicate label names are allowed across teams
- duplicate label names are allowed between a team label and a workspace label if desired

## 12.6 Audit Log

Track all RBAC-sensitive actions:

- invite sent
- invite accepted
- role changed
- member suspended
- member reinstated
- member removed
- team owner promoted/demoted
- team access policy changed
- team handle changed
- team label policy changed

## 13. Backend Requirements

## 13.1 Authorization Layer

Introduce centralized permission helpers/middleware:

- `RequireWorkspaceMembership`
- `RequireWorkspaceRole(minRole or allowedRoles)`
- `RequireTeamAccess`
- `RequireTeamOwnerOrAdmin`
- `CanManageMember(actor, target)`

Authorization must be enforced server-side only. Frontend checks are convenience only.

## 13.2 Required Backend APIs

### Workspace member APIs

- `GET /api/workspaces/{id}/members`
- `GET /api/workspaces/{id}/members/{memberId}`
- `PATCH /api/workspaces/{id}/members/{memberId}/role`
- `PATCH /api/workspaces/{id}/members/{memberId}/status`
- `POST /api/workspaces/{id}/members/{memberId}/reinstate`
- `DELETE /api/workspaces/{id}/members/{memberId}`

### Invitation APIs

- extend existing invite create endpoint to support `guest` and team assignments
- `GET /api/workspaces/{id}/invitations`
- `PATCH /api/invitations/{id}`
- `DELETE /api/invitations/{id}`

### Team access APIs

- `GET /api/teams/{id}/members`
- `POST /api/teams/{id}/members`
- `PATCH /api/teams/{id}/members/{userId}`
- `DELETE /api/teams/{id}/members/{userId}`
- `GET /api/teams/{id}/permissions`
- `PUT /api/teams/{id}/permissions`
- `PATCH /api/teams/{id}/handle`

### Team-scoped PM APIs

- `GET /api/pm/teams/{id}/labels`
- `POST /api/pm/teams/{id}/labels`
- `PUT /api/pm/teams/{id}/labels/{labelId}`
- `DELETE /api/pm/teams/{id}/labels/{labelId}`
- `GET /api/pm/teams/{id}/workflows`
- `PUT /api/pm/teams/{id}/settings`
- mention resolution endpoint or unified search support for team handles

## 14. Frontend Requirements

## 14.1 Members Page

Add filters:

- Active
- Pending invites
- Suspended
- Left
- Role
- Team

Each member row should show:

- avatar
- full name
- email
- workspace role
- status
- team count
- last active optional

Row overflow actions:

- Change role
- Suspend
- Reinstate
- Remove from workspace
- View profile/history

## 14.2 Invite Modal

Fields:

- email
- role
- team assignments when role = `guest`
- optional message

Validation:

- guests require at least one team
- viewers and guests cannot be granted admin-only permissions

## 14.3 Team Settings > Members

Add:

- promote/demote team owner
- add/remove members
- add/remove guest access
- manage team access policy

## 14.4 Team Settings > General

Add:

- team name
- team handle
- team visibility/access mode
- team-scoped PM toggles

## 14.5 Team Settings > Labels

Add:

- team label CRUD
- optional toggle to include workspace labels in pickers
- label ownership indicator: team or workspace

## 14.6 Team Mentions

Add:

- `@team-handle` autocomplete in editor
- mention chips rendering for team entities
- notification fanout to team members

## 14.7 UI Gating

Frontend should hide or disable actions based on effective permission:

- settings navigation
- members management actions
- PM configuration pages
- team-level restricted actions
- team label/workflow settings

## 15. Migration Strategy

### Phase 1

- Keep existing workspace roles
- Add member `status`
- Add backend membership authorization
- Add suspend/reinstate flows

### Phase 2

- Add `guest`
- Add team-scoped guest access
- Add team owner role
- Add team access policies
- Add team handle and mentions

### Phase 3

- Add stronger audit reporting
- Add team-scoped labels, workflows, templates, and automations
- Add SCIM/IdP compatibility layer if needed

## 16. Compatibility Mapping

| Current Helpin | New User-Facing Label | Notes |
|------------------|-----------------------|-------|
| `owner` | Workspace Owner | Keep DB value if easier |
| `admin` | Admin | No change |
| `manager` | Manager | Keep as Helpin-specific role |
| `member` | Member | No change |
| `viewer` | Viewer | Read-only |
| n/a | Guest | New |
| n/a | Team Owner | Team-level role, not workspace role |

## 17. Risks

1. Authorization drift if permission logic is duplicated across handlers.
2. Guest access can leak data if search, integrations, or cross-team joins are not scoped correctly.
3. Suspension is incomplete if sessions/tokens are not revoked immediately.
4. Migration risk if role renaming is done at the DB layer too early.
5. Teams remain second-class if membership stays tied to `workspace_people` instead of authenticated members.

## 18. Success Criteria

- Workspace admins can manage roles and statuses without direct DB intervention
- Suspended users lose access immediately and remain visible historically
- Team owners can manage their teams without workspace-wide admin powers
- Guests only see explicitly granted teams and data
- Teams can be mentioned by stable handles
- Teams can manage their own PM labels and settings
- All restricted backend actions enforce RBAC server-side
- Audit log records every sensitive member and role action

## 19. Implementation Recommendation

Recommended implementation order:

1. Lock down backend authorization with centralized membership checks
2. Add `status` to workspace memberships
3. Add member-management APIs and settings UI actions
4. Add guest role and team-scoped access
5. Add team owner, team handles, and configurable team permissions
6. Add team-scoped labels and PM team settings

This order reduces security risk first, then expands capability.
