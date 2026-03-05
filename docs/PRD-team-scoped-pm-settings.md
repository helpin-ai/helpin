# PRD: Team-Scoped PM Settings, Team Mentions, And Team Labels

**Product**: TeamPulse  
**Date**: March 5, 2026  
**Status**: Draft  
**Owner**: Product / Engineering

## 1. Summary

TeamPulse should move to a hybrid model:

- workspace-scoped identity, membership, RBAC, security, and administration
- team-scoped PM behavior, collaboration settings, and team identity

This PRD defines the implementation direction for:

- team membership management
- team handles for `@team` mentions
- team-scoped labels
- team-scoped PM settings

The goal is to make teams first-class operational units, similar to Linear, without turning the entire product into a team-only system.

## 2. Current State

### What exists today

- teams exist as `workspace_teams`: [settings.go](/root/teampulse/server/internal/model/settings.go:20)
- team memberships exist, but are attached to `WorkspacePerson`, not authenticated workspace members: [settings.go](/root/teampulse/server/internal/model/settings.go:33), [settings.go](/root/teampulse/server/internal/model/settings.go:55)
- workspace members exist separately in `workspace_members`: [workspace.go](/root/teampulse/server/internal/model/workspace.go:18)
- PM labels are workspace-wide only: [pm_label.go](/root/teampulse/server/internal/model/pm_label.go:5)
- the editor does not yet implement mention extensions or team handles: [tiptap-editor.tsx](/root/teampulse/frontend/src/components/ui/tiptap-editor.tsx:1)

### Practical consequence

Today teams behave more like settings/HR groupings than PM collaboration units.

That is why:

- you can create teams, but team member management is not a first-class workspace-member flow
- you cannot mention a team with `@team`
- labels cannot be owned by individual teams
- PM configuration cannot be delegated cleanly to teams

## 3. Problem

As PM features expand, workspace-wide settings alone are too coarse.

Different teams need:

- their own labels
- their own handles
- their own workflows and templates
- their own automations
- their own membership and visibility rules

Without team-scoped settings:

- PM becomes cluttered in multi-team workspaces
- labels become noisy and inconsistent
- collaboration flows like team mentions do not exist
- access control remains awkward

## 4. Goals

1. Make teams first-class PM units.
2. Let teams manage their own labels and PM settings safely.
3. Add team handles for mentions and routing.
4. Base team access on authenticated workspace members, not only `workspace_people`.
5. Preserve workspace-level governance.

## 5. Non-Goals

- cross-workspace teams
- custom role-builder for every team permission
- org chart / reporting-chain redesign
- Slack-style arbitrary user groups separate from teams

## 6. Product Principles

1. Workspace is the security boundary.
2. Team is the operational boundary.
3. Team PM settings should be isolated by default.
4. Global defaults should exist, but teams may override them.
5. Mentions must resolve to real access-controlled entities.

## 7. Scope Split

## 7.1 Workspace-Scoped

- workspace members
- workspace roles and RBAC
- invitations
- billing
- security
- audit
- integrations
- optional global PM defaults

## 7.2 Team-Scoped

- team members
- team owners
- team handle
- team labels
- team workflows/statuses
- team templates
- team automations
- team access mode
- team-specific PM defaults

## 8. Team Membership Model

## 8.1 Source Of Truth

Team membership should be based on authenticated workspace members.

Recommended direction:

- keep `workspace_people` for bonus/performance/HR metadata
- introduce or evolve a team membership model keyed by `workspace_members.user_id`

### Why

The current team membership table points to `person_id`, not `user_id`, which is insufficient for:

- RBAC
- mentions
- notifications
- PM permissions
- guest access

## 8.2 Team Membership Roles

Each team member can have:

- `owner`
- `member`

Guests can be assigned to teams but cannot be team owners.

## 8.3 Team Access Modes

Each team should support:

| Mode | Meaning |
|------|---------|
| `open` | Any active workspace member can join |
| `restricted` | Only invited/assigned members can join |
| `private` | Only explicitly assigned members and owners can access |

## 9. Team Handles And Mentions

## 9.1 Team Handle

Add `handle` to teams.

Requirements:

- unique per workspace
- lowercase slug-like format
- editable by team owners or workspace admins/owners
- visible in team settings and team picker UIs

Example:

- Team name: `Growth Marketing`
- Team handle: `growth`
- Mention: `@growth`

## 9.2 Mention Behavior

When a user types `@growth`:

1. editor suggests the matching team
2. selecting it inserts a team mention entity
3. mention renders as a chip in the editor
4. notification fanout goes to active team members with access

## 9.3 Mention Rules

- only resolvable for teams visible to the current user
- guests can mention only teams they belong to
- notifications should not fan out to suspended members
- mentions should resolve by stable team ID, not by raw string after insertion

## 10. Team Labels

## 10.1 Recommendation

Move to team-scoped labels by default.

Recommended label scope model:

- `team`
- `workspace`

## 10.2 Why

Different teams will reuse names like:

- `blocked`
- `backend`
- `design`
- `urgent`

Workspace-wide labels alone create naming collisions and taxonomy sprawl.

## 10.3 Behavior

### Team labels

- owned by a single team
- visible in that team’s stories/epics/sprints by default
- manageable from Team Settings > Labels

### Workspace labels

- optional shared taxonomy
- visible across teams if enabled
- manageable only by workspace admins/owners

## 10.4 Label Picker UX

In team context:

- show team labels first
- optionally show shared workspace labels in a separate group
- visually mark label scope

In cross-team views:

- show both, but always indicate origin team/scope

## 11. Team-Scoped PM Settings

Each team should own the following PM settings:

- labels
- workflows and statuses
- templates
- triage rules
- automations
- member access controls

These settings should live under Team Settings rather than only global Settings.

## 12. UX Requirements

## 12.1 Team Settings Navigation

Each team should have:

- General
- Members
- Access & Permissions
- Labels
- Workflow
- Templates
- Automations

## 12.2 Team Settings > General

Fields:

- team name
- handle
- description
- visibility/access mode

## 12.3 Team Settings > Members

Actions:

- add workspace member to team
- remove member from team
- promote to team owner
- demote team owner
- add guest to team

## 12.4 Team Settings > Labels

Actions:

- create team label
- edit team label
- archive team label
- optionally allow workspace labels in pickers

## 12.5 Editor Mention UX

Requirements:

- autocomplete users and teams
- teams show handle and team icon
- insert structured mention node
- read-only render remains clickable/filterable

## 13. Data Model Changes

## 13.1 Teams

Extend teams with:

- `handle`
- `access_mode`
- `is_private`

## 13.2 Team Members

Add or evolve a user-based membership table:

- `workspace_id`
- `team_id`
- `user_id`
- `role` (`owner`, `member`)
- `access_source`
- timestamps

## 13.3 Labels

Extend `pm_labels`:

- `scope`
- `team_id` nullable

Suggested rules:

- if `scope = 'team'`, `team_id` is required
- if `scope = 'workspace'`, `team_id` is null

Suggested uniqueness:

- unique `(workspace_id, lower(name))` for workspace labels
- unique `(team_id, lower(name))` for team labels

## 13.4 Team PM Settings

Add a `pm_team_settings` style table or separate tables for:

- workflow policy
- label visibility policy
- template policy
- automation policy

## 14. API Requirements

## 14.1 Team Membership

- `GET /api/teams/{id}/members`
- `POST /api/teams/{id}/members`
- `PATCH /api/teams/{id}/members/{userId}`
- `DELETE /api/teams/{id}/members/{userId}`

## 14.2 Team General Settings

- `GET /api/teams/{id}`
- `PUT /api/teams/{id}`
- `PATCH /api/teams/{id}/handle`
- `PATCH /api/teams/{id}/access`

## 14.3 Team Labels

- `GET /api/pm/teams/{id}/labels`
- `POST /api/pm/teams/{id}/labels`
- `PUT /api/pm/teams/{id}/labels/{labelId}`
- `DELETE /api/pm/teams/{id}/labels/{labelId}`

## 14.4 Mentions

Either:

- a dedicated mention lookup endpoint

or:

- extend the existing search/autocomplete endpoint to return teams with handles

## 15. Authorization Rules

- workspace owner/admin can manage any team
- team owner can manage only their team
- member can act within team permissions
- guest can only access explicitly granted teams
- viewer cannot change team PM settings

## 16. Migration Plan

### Phase 1

- add team handle
- add team settings pages
- add user-based team membership APIs

### Phase 2

- implement team mention extension
- add notification fanout for team mentions

### Phase 3

- migrate PM labels to support `team` and `workspace` scope
- add team-scoped workflows/templates/automations

## 17. Success Criteria

- admins and team owners can add/remove members from teams
- teams can be referenced by `@handle`
- team labels are managed without polluting other teams
- PM configuration becomes team-owned where appropriate
- workspace-level governance remains intact

## 18. Recommendation

TeamPulse should follow Linear’s direction here.

Not by making every setting team-scoped, but by making teams first-class PM configuration boundaries inside a workspace-scoped security model.

That is the right model for:

- team membership
- team mentions
- team labels
- team workflows and templates

