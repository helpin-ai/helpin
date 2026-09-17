# Invited users in task assignment

> Source review, 2026-09-17

Historical March assignment proposal. The
[workspace repository](../../server/internal/repository/workspace.go) now has
`UpsertPendingMember` and `ListAssignableMembers`, which includes joined and
pending identities. The opening claim that invitation acceptance is required
before an identity can appear in PM assignment lists is obsolete. Check the
specific service and picker when extending assignment; a pending identity does
not thereby receive authenticated workspace access.

**Author:** Engineering  
**Date:** 2026-03-07  
**Status:** Draft

---

## 1. Problem Statement

Workspace admins can invite users by email, but invited users do not appear in PM owner / assignee pickers until they accept the invite and become active workspace members.

This is a mismatch with the behavior teams expect from tools like Shortcut and Linear, where planning can start before invite acceptance. It is also a modeling problem in Helpin: today, workspace identity is split across:

- `workspace_members` for access
- `workspace_people` for rewards / people records
- PM owner fields that inconsistently behave like user IDs or person IDs

That split makes it hard to answer a simple product question:

“Who can I assign work to in this workspace?”

We need a simpler long-run PM identity model.

---

## 2. Recommendation

For Helpin, the best long-run model is:

- `workspace_members` becomes the single workspace identity model for PM
- `workspace_members` supports both `pending` and `active` lifecycle states
- PM entities reference `workspace_member_id`
- `workspace_invitations` remains the invite token / expiry workflow table
- reward-specific fields move out of `workspace_people` into a separate `reward_profiles` table later

This means:

- invited users and joined users use the same workspace-scoped identity row
- the same picker can later support stories, epics, and objectives
- the product model becomes much easier to explain

---

## 3. Goals

1. Show invited-but-not-yet-joined users in PM owner / assignee pickers.
2. Allow a story to be assigned to an invited user before invite acceptance.
3. Preserve assignments when the user later accepts the invite.
4. Build the solution so the same picker and backend model can be reused for epics and objectives.
5. Simplify workspace identity so PM does not depend on both `workspace_members` and `workspace_people`.

---

## 4. Non-Goals

- Rebuilding the full rewards system in this release.
- Deleting `workspace_people` immediately in v1.
- Refactoring every PM screen in one shipment.
- Supporting assignment to arbitrary emails without an invitation.
- Sending PM notifications to pending invitees before signup.

---

## 5. Why This Is The Best Fit

### 5.1 Why Not Copy Plane

Plane’s assignee model is member-only:

- accepted members appear in assignee pickers
- invitations are separate
- pending invitees are not assignable

That is simpler, but it does not satisfy the product behavior you want.

### 5.2 Why Not Keep `workspace_people` As PM Truth

`workspace_people` can represent non-joined humans, but it is already overloaded with rewards/personnel concerns. If PM becomes the first-priority product surface, continuing to route core PM identity through a rewards-shaped table will stay confusing.

### 5.3 Why Unified `workspace_members` Works Better

If `workspace_members` becomes the canonical workspace identity row, then:

- pending invitees already have a durable workspace identity
- active users keep the same ID after acceptance
- PM ownership, team assignment, and filters can all point to the same model
- the distinction becomes clear:
  - `users` = account
  - `workspace_members` = workspace identity and access lifecycle

That is easier to reason about than a `members + people` split for PM.

---

## 6. Product Decision

We will standardize PM owner / requester / assignee identity on `workspace_members`.

### 6.1 Core Rule

If a human can be assigned work in a workspace, they must have a `workspace_members` row, regardless of whether they have accepted the invite yet.

### 6.2 Identity Split

- `users`
  - authenticated account
- `workspace_members`
  - canonical workspace-scoped identity for PM and access lifecycle
- `workspace_invitations`
  - invite token, expiry, acceptance workflow
- `reward_profiles`
  - future reward / compensation / job metadata attached to a workspace member

### 6.3 Rollout Strategy

- **v1**
  - story create modal
  - story detail owner / requester pickers
  - shared assignable-member picker
- **v2**
  - epic owner picker
  - objective owner picker(s)
  - PM filters and quick assign surfaces
- **v3**
  - cleanup of legacy `workspace_people` usage
  - full migration of rewards / people data to dedicated profile tables

---

## 7. User Experience

### 7.1 Shared Picker

Create a reusable `AssignableMemberPicker` UI component used by:

- story create modal
- story detail / panel
- epic owner selection later
- objective owner selection later

### 7.2 Display Rules

Show active members as:

- full name
- fallback email

Show invited pending members as:

- `email@example.com`
- or `Name (email@example.com)` when available
- with a `Pending invite` badge

### 7.3 Story v1

In story create and story detail:

- `Owner` picker shows active and pending workspace members
- `Requester` picker shows active and pending workspace members
- pending invitees can be selected and saved

### 7.4 Epic / Objective Later

Yes, this same picker can be reused later for:

- epic owner
- objective owners

That is the main reason to make `workspace_members` the canonical identity row now.

---

## 8. Current State

### 8.1 Relevant Tables

- `workspace_members`
  - currently access membership only
  - requires `user_id`
- `workspace_invitations`
  - pending invitation state by email
- `workspace_people`
  - workspace-scoped people records, currently used by settings / rewards
- `team_user_memberships`
  - team assignment by `user_id`
- `team_memberships`
  - team assignment by `person_id`
- `pm_stories`
  - `owner_id`, `requester_id`
- `pm_epics`
  - `owner_id`
- `pm_objective_owners`
  - owner join table

### 8.2 Current Gaps

1. `workspace_members` cannot represent pending invitees.
2. Story owner / requester pickers only read joined members.
3. Team assignment is split across user-based and person-based tables.
4. PM owner identity is inconsistent across stories, epics, and objectives.
5. Rewards data living in `workspace_people` keeps pressure on PM to reuse that model.

---

## 9. Target Data Model

### 9.1 `workspace_members`

`workspace_members` becomes the canonical workspace identity row.

Required changes:

- `user_id` becomes nullable
- add `email`
- add `display_name`
- add `status`
- add `invited_by`
- add `invited_at`
- add `accepted_at`

Proposed shape:

```text
workspace_members
- id
- workspace_id
- user_id nullable
- email
- display_name nullable
- role
- status: pending | active | revoked | inactive
- invited_by nullable
- invited_at nullable
- accepted_at nullable
- created_at
- updated_at
```

Constraints:

- unique `(workspace_id, lower(email))`
- unique `(workspace_id, user_id)` where `user_id` is not null

### 9.2 `workspace_invitations`

Keep `workspace_invitations` for token and invite lifecycle only.

Add:

- `workspace_member_id`

This makes the relationship explicit:

- one pending / accepted member identity row
- one or more invitation lifecycle records if needed

### 9.3 Team Membership

Introduce a new canonical team membership table:

```text
team_workspace_memberships
- id
- team_id
- workspace_member_id
- role
- created_at
- updated_at
```

This replaces the current split between:

- `team_user_memberships`
- `team_memberships`

`team_workspace_memberships` is the correct long-run PM/team relation because it works for:

- pending invitees
- active joined members

### 9.4 PM Ownership

PM entities should converge on `workspace_member_id`.

#### Story

- `pm_stories.owner_member_id`
- `pm_stories.requester_member_id`
- `pm_story_owners.workspace_member_id`

#### Epic

- `pm_epics.owner_member_id`

#### Objective

- `pm_objective_owners.workspace_member_id`

Recommendation:

- introduce explicit new member-based columns first
- backfill from current user/person based fields
- switch reads to prefer member-based fields
- remove old fields later

### 9.5 Reward Profiles

Rewards should not block PM simplification.

Instead of forcing PM to stay coupled to `workspace_people`, add a future table:

```text
reward_profiles
- id
- workspace_member_id unique
- job_role
- manager_member_id nullable
- hire_date
- base_salary
- active_for_bonus
- active_for_evaluation
- is_account_owner
- created_at
- updated_at
```

This lets us:

- simplify PM around `workspace_members`
- preserve a path for rewards later
- migrate off `workspace_people` over time

`reward_profiles` is **not required for story v1**, but it is the intended long-run replacement for reward/personnel fields currently living on `workspace_people`.

---

## 10. Functional Requirements

### 10.1 Invitation Send Flow

When a workspace invitation is created:

1. Upsert a `workspace_members` row by `(workspace_id, email)`.
2. If no row exists:
   - create `workspace_members.status = pending`
   - `user_id = null`
   - `display_name = null` unless provided
3. Create `workspace_invitations` linked to that `workspace_member_id`.
4. If team preassignments are chosen, create `team_workspace_memberships` rows immediately.

### 10.2 Invitation Acceptance Flow

When the invite is accepted:

1. Find the `workspace_members` row by invitation.
2. Link `workspace_members.user_id = accepted_user_id`.
3. Set `status = active`.
4. Set `accepted_at`.
5. Keep the same `workspace_member_id` so PM references remain stable.

### 10.3 Authorization Rule

Access checks must use:

- `workspace_members.user_id = current_user_id`
- `workspace_members.status = active`

Pending members are assignable in PM but do not have workspace access.

### 10.4 Assignable Members Source

Add one canonical backend source for PM pickers:

- active workspace members
- pending invited workspace members
- optional team filter

This source must power:

- story owner / requester
- later epic owner
- later objective owner(s)

---

## 11. API Design

### 11.1 New Endpoint

Add:

`GET /api/workspaces/{id}/assignable-members`

Response:

```json
[
  {
    "id": "workspace_member_uuid",
    "user_id": null,
    "email": "jane@example.com",
    "display_name": null,
    "role": "member",
    "status": "pending",
    "is_pending": true,
    "team_ids": ["team_1"]
  }
]
```

### 11.2 Story APIs

Create / update story payloads must accept:

- `owner_member_id`
- `requester_member_id`
- `owner_member_ids` where multi-owner applies

### 11.3 Epic APIs Later

Epic create / update payloads must later accept:

- `owner_member_id`

### 11.4 Objective APIs Later

Objective create / update payloads must later accept:

- `owner_member_ids`

---

## 12. Backend Changes Required

### 12.1 Schema Changes Required For v1

1. Update `workspace_members`
   - make `user_id` nullable
   - add `email`
   - add `display_name`
   - add `status`
   - add `invited_by`
   - add `invited_at`
   - add `accepted_at`
2. Update `workspace_invitations`
   - add `workspace_member_id`
3. Add `team_workspace_memberships`
4. Add member-based PM fields:
   - `pm_stories.owner_member_id`
   - `pm_stories.requester_member_id`
   - `pm_story_owners.workspace_member_id`

### 12.2 Service / Repository Changes For v1

1. Workspace repository:
   - active-member access queries must filter by `status = active`
   - member lookups by email
2. Invite service:
   - create / upsert pending workspace member on invite send
   - link same member row on accept
3. Team settings:
   - write `team_workspace_memberships`
   - stop depending on `team_user_memberships` for PM behavior
4. PM story repository / service:
   - write member-based owner / requester fields
   - read owner names from `workspace_members.display_name` or `email`
   - stop assuming story owner IDs map directly to `users`

### 12.3 New Read Models / Queries

Need queries for:

- assignable members by workspace
- assignable members filtered by team
- story detail owner / requester display from member rows
- board/list owner display from member rows

### 12.4 Backfill Required

Backfill:

1. active `workspace_members.email` and `display_name` from linked users
2. pending workspace members for existing invitations
3. story owner / requester links from current user-based fields to member-based fields

---

## 13. Frontend Changes

### 13.1 Shared Picker

Create `AssignableMemberPicker`:

- single select
- multi-select
- pending badge
- display name / email fallback
- team-aware filtering support

### 13.2 Story v1

Replace member-only sources in:

- create story modal
- story detail page
- story detail panel

### 13.3 Epic / Objective Later

Reuse the same picker in:

- epic owner selector
- objective owner selector

---

## 14. Migration Strategy

### Phase 1: Foundation

1. Expand `workspace_members` schema.
2. Add `workspace_member_id` to invitations.
3. Add `team_workspace_memberships`.
4. Backfill pending / active workspace member rows.

### Phase 2: Story v1

1. Add member-based story owner / requester fields.
2. Backfill existing story data.
3. Update story APIs and story UI pickers.

### Phase 3: Epic / Objective v2

1. Add member-based epic owner.
2. Add member-based objective owners.
3. Reuse same picker and endpoint.

### Phase 4: Reward Cleanup

1. Add `reward_profiles`.
2. Backfill reward/personnel fields from `workspace_people`.
3. Remove PM dependencies on `workspace_people`.
4. Deprecate `workspace_people` later if no longer needed.

---

## 15. Edge Cases

### 15.1 Existing Joined User

Inviting an email already linked to an active workspace member should be rejected.

### 15.2 Existing Pending Member

Inviting the same email again should reuse the same pending `workspace_member` row and either:

- reuse the open invitation
- or replace the invitation token while preserving the member identity

### 15.3 Revoked / Expired Invite

If an invitation is revoked or expires:

- keep the `workspace_member` row
- mark status appropriately (`revoked` or keep `pending` plus non-assignable flag if needed)
- do not orphan PM assignments

### 15.4 Notifications

Pending members should not receive PM notifications until they have a linked `user_id` and active membership.

---

## 16. Success Metrics

1. Invited emails appear in story owner / requester pickers immediately after invite creation.
2. A story can be assigned to an invited user before signup.
3. After invite acceptance, the same `workspace_member_id` remains attached to the story.
4. The same assignable-member foundation can later be reused for epic and objective owner pickers.
5. PM identity becomes easier to reason about because it uses one workspace-level member model.

---

## 17. Acceptance Criteria

1. Inviting `jane@example.com` creates:
   - a pending `workspace_members` row
   - a linked `workspace_invitations` row
2. `jane@example.com` appears in the story owner / requester picker as `Pending invite`.
3. A story can be created with Jane selected before she joins.
4. When Jane accepts:
   - her existing `workspace_members` row is linked to `user_id`
   - status becomes `active`
   - the story still points to the same member row
5. The same picker / API shape can later be reused for epic and objective owner assignment.

---

## 18. Open Questions

1. Do we want to replace `team_user_memberships` immediately or keep it for compatibility while `team_workspace_memberships` rolls out?
2. Should revoked / expired members remain assignable if they are already referenced by PM entities?
3. Should requester move in the same release as owner, or is it acceptable to stage requester one sprint later?
4. When `reward_profiles` is introduced, do we also want a more generic `member_profiles` table for non-reward metadata, or is reward-only enough?

---

## 19. Final Recommendation

If the goal is long-run clarity and PM-first modeling:

- stop making PM choose between `workspace_members` and `workspace_people`
- standardize PM on `workspace_members`
- let invitations create pending member rows
- keep invite lifecycle in `workspace_invitations`
- move reward-specific fields to `reward_profiles` later

That gives you the simplest model to explain and the best path to story first, epic next, objective after.
