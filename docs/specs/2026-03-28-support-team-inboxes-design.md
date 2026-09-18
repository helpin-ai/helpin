# Support team inboxes: original access and routing design

This historical specification records the March team-inbox proposal for contributors investigating support routing and access. Current membership, navigation, setup, and archive behavior differ materially; use the source review below instead of treating the original access rules as a current security contract.

## Source review — 2026-09-18

- Nullable `mailbox_id`, mailbox records, membership records, assignment modes, and routing settings exist in [support models](../../server/internal/model/support_inbox.go). The shared scope remains represented by a null mailbox ID, but the [frontend store](../../frontend/src/stores/supportInboxStore.ts) now defaults to `all` and migrates older persisted `shared` selections to `all`.
- Linked-team membership is now part of live access, not merely a one-time import helper. [Repository access conditions](../../server/internal/repository/support_mailbox.go) admit active explicit members **or active linked-team members**, and count the union. Removing an explicit membership does not revoke access if that person still belongs to the linked team.
- The [mailbox service](../../server/internal/service/support_mailbox.go) grants elevated mailbox access to workspace owner/admin roles. Route-level `support.read`, `support.edit`, and `support.admin` checks are separate. Do not equate a permission name with the role bypass used by the repository/service helpers.
- `ArchiveMailbox` currently sets `active=false` without checking for or moving active conversations. Ordinary membership checks require an active mailbox; elevated access is handled separately. The proposed requirement to move active conversations before archive is not implemented by this method.
- The AI handoff mailbox helper uses an explicitly configured `ai_handoff_mailbox_id`, otherwise returns no override. It does not implement the original helper-level fallback to `default_mailbox_id`; later routing behavior depends on its caller. Default inbound mailbox resolution remains a separate helper.
- [TeamInboxDialog](../../frontend/src/components/support/TeamInboxDialog.tsx) includes linked-team members automatically and accepts extra members. Its [three-step flow](../../frontend/src/components/support/teamInboxDialogFlow.ts) covers details, members/assignment, and routing, superseding the single compact form proposal. [SupportRailNav](../../frontend/src/components/layout/sidebar/SupportRailNav.tsx) contains current team-inbox navigation.
- List/detail query access uses mailbox-aware conditions, and administration/move routes exist. This source review does not establish complete parity across notifications, mentions, realtime delivery, or all clients. Those cross-surface requirements and the manual UX expectations below remain historical acceptance criteria, not a fresh security or browser test result.

## Original specification

## Summary

Add optional Team Inboxes to Support without changing the current shared workspace inbox behavior.

Today, support is effectively one workspace-wide inbox behind one workspace widget. That behavior should remain intact for v1. Team Inboxes should be an additional internal routing and access-control layer for conversations that need specialist or private handling, similar to Crisp sub-inboxes sitting alongside the main inbox.

The core product rule is:

- the workspace widget remains a single workspace-level entry point
- the current shared inbox remains the default inbox experience
- Team Inboxes are optional, member-scoped inboxes layered on top
- conversations can live either in the shared inbox or in one Team Inbox

The widget is not the access-control mechanism. Inbox membership is the access-control mechanism.

This avoids a risky migration to a generated `General` inbox and keeps rollout behavior stable for existing workspaces.

## Goals

- Preserve the current shared support inbox as the default workspace experience.
- Add private Team Inboxes for selected groups such as Billing, Technical Support, and VIP.
- Restrict Team Inbox conversations to inbox members plus elevated admins/owners.
- Let admins link an inbox to a workspace team for setup convenience without making team membership the access model.
- Let admins add members directly during inbox creation.
- Add an icon per inbox so sidebar navigation is easier to scan.
- Keep the design compatible with later queue models without blocking the first release.

## Current Behavior

- Support conversations are workspace-scoped today.
- The main support UI assumes one shared list of conversations.
- Routing and availability can narrow to a team in some flows, but the core read model is still workspace-wide.
- Existing customers already understand the current shared list as the default inbox.

That makes a forced migration into a generated mailbox the wrong v1 shape.

## Product Decision

### Product concepts

This feature has three separate concepts that should not be conflated:

- `Widget`
  - the customer-facing entry point for support conversations
  - remains workspace-level in v1
  - is not scoped to selected members or selected teams
- `Shared Inbox`
  - the existing internal workspace-wide support inbox
  - remains the default support destination unless routing sends a conversation into a Team Inbox
- `Team Inbox`
  - a private internal sub-inbox for selected members
  - can optionally link to a workspace team for setup convenience

In other words:

- customers enter through the widget
- conversations land in an inbox
- inbox membership determines which agents can work those conversations

### Inbox model

Support will have two inbox concepts:

- `Shared Inbox`
  - implicit system inbox
  - backed by the existing workspace-wide conversation model
  - not represented as a user-managed mailbox record
- `Team Inbox`
  - explicit user-created mailbox
  - private to selected members
  - optional linked team for setup and import convenience

### Conversation ownership

- A conversation belongs to either:
  - the shared inbox
  - one Team Inbox
- In storage, `support_conversations.mailbox_id` is nullable.
- `NULL mailbox_id` means the conversation is in the shared inbox.

### Access rules

- `owner` and `support.admin` can access:
  - the shared inbox
  - all Team Inboxes
  - inbox management features
- users with `support.read` or `support.edit` can access:
  - the shared inbox
  - Team Inboxes where they are members
- private Team Inboxes do not appear in navigation for non-members
- direct URL access to an inaccessible Team Inbox conversation must fail with not found / forbidden behavior and redirect back to an accessible inbox
- the widget itself is not hidden or shown based on Team Inbox membership

### Team linking

- A Team Inbox may optionally link to one workspace team.
- Linking a team is a setup helper, not the durable ACL source.
- Team Inbox membership is managed independently from workspace team membership.
- Admins can import linked-team members during creation or later from edit settings.

### Inbox creation

Creating a Team Inbox should capture:

- name
- icon
- handle
- description
- linked workspace team
- initial members
- assignment mode

The modal should support both:

- selecting a linked team and importing members
- manually adding extra members who are not in that team

## Scope

### V1

- Create, edit, archive, and reorder Team Inboxes.
- Add and remove Team Inbox members.
- Add members during Team Inbox creation.
- Select an icon during Team Inbox creation.
- Optionally link a Team Inbox to a workspace team.
- Import linked-team members on create or edit.
- Show accessible Team Inboxes in the support sidebar under a collapsible heading.
- Keep one workspace widget for support entry.
- Keep the current shared inbox visible as the default inbox.
- Route new conversations to the shared inbox by default.
- Allow an optional workspace default Team Inbox for routing if admins configure one.
- Allow manual move of a conversation:
  - shared inbox -> Team Inbox
  - Team Inbox -> shared inbox
  - Team Inbox -> Team Inbox
- Make unread counts, notifications, assignment, and mentions mailbox-aware.
- Let AI escalation route into a configured Team Inbox, otherwise fall back to the shared inbox.

## UX Design

### Sidebar information architecture

The support navigation should keep today’s shared filters, then introduce a collapsible `Team Inboxes` group below them.

The shared inbox remains first-class and always visible. Team Inboxes are a secondary grouping, not a shell rewrite.

ASCII sketch:

```text
+--------------------------------------------------+
| Support                                          |
+--------------------------------------------------+
| [ + New Conversation ]                           |
|                                                  |
| Inbox                                            |
| > Shared Inbox                         [ 24 ]    |
|                                                  |
| Filters                                          |
|   All                                  [ 24 ]    |
|   My Inbox                              [ 6 ]    |
|   Unassigned                            [ 9 ]    |
|   Mentions                              [ 2 ]    |
|                                                  |
| AI                                               |
|   All AI                                [ 5 ]    |
|   Escalated                             [ 2 ]    |
|   Pending                               [ 3 ]    |
|   Resolved                              [ 0 ]    |
|                                                  |
| Team Inboxes                               [v]   |
|   # Billing                              [ 4 ]   |
|   W Technical Support                    [ 7 ]   |
|   * VIP                                  [ 1 ]   |
|                                                  |
|   + Create Team Inbox                            |
+--------------------------------------------------+
```

Behavior:

- `Shared Inbox` switches conversation scope to `mailbox_id IS NULL`
- selecting a Team Inbox switches conversation scope to that mailbox
- existing filters like `All`, `My Inbox`, `Unassigned`, `Mentions`, and AI states apply within the selected inbox scope
- Team Inboxes only render if the user can access at least one of them
- archived Team Inboxes are hidden from normal navigation

### Team Inbox creation flow

Creating an inbox should be a compact modal or sheet, not a separate multi-step wizard.

ASCII sketch:

```text
+------------------------------------------------------------------+
| Create Team Inbox                                           [x]  |
+------------------------------------------------------------------+
| Name *                                                           |
| [ Technical Support___________________________________________ ] |
|                                                                  |
| Icon *                                                           |
| [ W ]  Choose icon                                               |
|                                                                  |
| Handle *                                                         |
| [ technical-support___________________________________________ ] |
|                                                                  |
| Description                                                      |
| [ Specialist support for product and API issues______________ ]  |
|                                                                  |
| Linked Team                                                      |
| [ Engineering Support_____________________________ v ] [Import]  |
|                                                                  |
| Members                                                          |
| [ Search workspace members___________________________________ ]  |
| [x] Aisha Khan      [x] Ben Carter      [x] Priya Shah           |
|                                                                  |
| Assignment Mode                                                  |
| ( ) Manual     ( ) Round robin                                   |
|                                                                  |
|                                     [ Cancel ] [ Create Inbox ]  |
+------------------------------------------------------------------+
```

Behavior:

- linked team is optional
- `Import` pulls active members from the linked team into the member picker as a one-time action
- imported users remain manually removable
- icon selection should use the project’s existing icon system rather than freeform uploads for v1

### Thread-level move action

Conversations need an explicit move action in thread or detail UI.

ASCII sketch:

```text
+--------------------------------------------------------------+
| Conversation #1042                                           |
| Acme Corp billing issue                                      |
|                                                              |
| Inbox: Shared Inbox                  [ Move... ]             |
| Assignee: Unassigned                                          |
| Status: Open                                                  |
+--------------------------------------------------------------+

Move conversation

  Shared Inbox
  Billing
  Technical Support
  VIP
```

Behavior:

- moving a conversation changes its visibility immediately
- if the current user loses access after the move, the UI should navigate away from that thread
- unread state should be recalculated for the destination inbox audience

### Settings surface

Support admins should manage Team Inboxes under Support settings.

The management table should show:

- icon
- name
- handle
- linked team
- member count
- assignment mode
- active / archived state
- actions

## Routing and Assignment

### Routing defaults

- The support widget remains workspace-level in v1.
- New inbound conversations should continue routing to the shared inbox by default.
- Add `default_mailbox_id` to support installation settings as an optional override.
- If `default_mailbox_id` is null, the destination is the shared inbox.
- Add `ai_handoff_mailbox_id` as an optional AI escalation override.
- If `ai_handoff_mailbox_id` is null, AI escalation falls back to:
  - configured `default_mailbox_id`, if present
  - otherwise the shared inbox

This means:

- widget visibility is still controlled at the workspace support-installation level
- inbox visibility is controlled by shared-inbox access or Team Inbox membership
- routing chooses the destination inbox after the conversation enters through the widget

### Assignment

- Shared inbox keeps the current workspace-wide assignment behavior for v1.
- Team Inbox assignment mode is mailbox-owned.
- Supported Team Inbox assignment modes in v1:
  - `manual`
  - `round_robin`
- Round robin selects from active eligible Team Inbox members, not from linked-team membership.

### Notifications

- Shared inbox notifications keep current workspace behavior.
- Team Inbox notifications go only to:
  - assigned owner
  - eligible Team Inbox members
  - elevated admins/owners where appropriate
- Mentions inside a Team Inbox are restricted to:
  - Team Inbox members
  - elevated admins/owners

## Data Model

### `support_mailboxes`

Add a new mailbox table for Team Inboxes:

- `id`
- `workspace_id`
- `name`
- `handle`
- `icon`
- `description`
- `linked_team_id`
- `visibility_mode`
- `assignment_mode`
- `position`
- `active`
- `created_by_id`
- `created_at`
- `updated_at`

Recommended rules:

- `visibility_mode` exists for future-proofing
- only `members_only` is exposed in v1 for Team Inboxes
- the shared inbox is still represented by `NULL mailbox_id`, not by a mailbox row

### `support_mailbox_memberships`

- `id`
- `mailbox_id`
- `workspace_member_id`
- `created_at`
- `updated_at`

Uniqueness:

- unique on `mailbox_id + workspace_member_id`

### `support_conversations`

Add:

- `mailbox_id NULL`

Semantics:

- `NULL` => shared inbox
- non-null => Team Inbox conversation

### support installation settings

Extend support settings JSON with:

- `default_mailbox_id`
- `ai_handoff_mailbox_id`

Both fields are nullable.

## Backend Design

### Read model

- mailbox list endpoint returns:
  - shared inbox
  - accessible Team Inboxes
- conversation list and get endpoints must enforce mailbox-aware access rules
- unread stats must be scoped to the currently selected inbox

### Write model

Add Team Inbox endpoints for:

- create inbox
- update inbox
- archive inbox
- reorder inboxes
- add members
- remove members
- import linked-team members
- list accessible inboxes
- move conversation to another inbox

### Access enforcement

- shared inbox conversations remain readable to users with current support access
- Team Inbox conversations require:
  - Team Inbox membership
  - or elevated admin/owner role
- all mailbox access checks must happen server-side, not only in frontend filtering

### Migration

No conversation backfill is required.

Schema migration should only:

- create mailbox tables
- add nullable `mailbox_id` to `support_conversations`
- extend support settings for mailbox routing

Existing workspaces should behave exactly as they do today immediately after deploy.

## Frontend Design

### State and query model

- Add mailbox types to frontend support types.
- Add query keys and hooks for:
  - mailbox list
  - mailbox admin management
  - mailbox members
- Add selected inbox state alongside the existing support nav filter state.
- Keep route shape stable; inbox selection can live in query params or state in the first version.

### Conversation list behavior

- conversation queries should include the selected inbox scope
- existing local filters remain unchanged conceptually, but operate within the selected inbox
- empty states should be inbox-aware

### Admin UI

- add Team Inbox management under Support settings
- add create/edit forms with icon selection, linked team import, and manual member editing

## Implementation Shape

### Phase 1

- mailbox schema
- nullable `mailbox_id`
- mailbox access control
- mailbox list API
- sidebar selector with Team Inboxes section
- shared inbox default behavior preserved

### Phase 2

- create/edit/archive/reorder Team Inboxes
- member management
- linked-team import
- icon selection

### Phase 3

- move conversation between inboxes
- Team Inbox round robin
- mailbox-aware notifications and AI handoff routing cleanup

## Risks

- Shared inbox semantics and Team Inbox semantics must be enforced consistently across list, detail, unread, notifications, mentions, and assignment. Partial adoption would create confusing access leaks.
- Using `NULL mailbox_id` for shared inbox is the simplest rollout model, but all mailbox-aware queries must be reviewed carefully so `NULL` is treated as a real inbox scope rather than “missing data.”
- If inbox selection is kept only in local state, deep-linking and browser navigation may feel incomplete. Query-param support may be worth adding early even if route paths stay unchanged.

## Open Questions

- Should archiving a Team Inbox require moving all active conversations back to the shared inbox first, or should archived inbox conversations remain readable only to admins? The recommended v1 behavior is:
  - archived inbox is hidden from normal agent navigation
  - admins can still access historical conversations
  - active conversations must be moved before archive is finalized
