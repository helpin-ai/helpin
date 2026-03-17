# PM Team Mentions Design

## Summary

Give `@team-handle` real product semantics across PM surfaces.

Today, team handles are stored and surfaced in editors, but team mentions are effectively just authored text. The backend only resolves `@mentions` to user recipients, and only in some PM services. The goal of this design is to make `@team-handle` notify all current members of that team who can read the item, using the existing mention notification model.

## Goals

- Make `@team-handle` notify all current team members with access to the mentioned item.
- Support team mentions everywhere mentions are currently authored in PM.
- Reuse the existing mention-notification model rather than inventing a separate broadcast system.
- Keep the current editor authoring experience intact for the first version.

## Non-Goals

- Granting access to users because they were team-mentioned.
- Migrating to a fully structured mention-entity schema.
- Changing current PM editor UX beyond team-aware rendering.
- Retroactively reprocessing historical mentions when teams or memberships change.

## Current Behavior

- Team handles are validated, normalized, and stored uniquely per workspace team.
- Workspace setup and team settings expose team handles and describe them as mention identifiers.
- PM editors suggest teams in the `@mention` picker using `team.handle`.
- Backend mention fan-out exists today for user mentions in:
  - stories
  - comments
  - checklist items
- Epic, sprint, and objective descriptions currently allow authoring `@mentions` in editors, but do not have equivalent backend mention fan-out.
- Mention rendering is member-oriented today; unresolved handles are shown as plain highlighted `@text`.

## Product Decision

`@team-handle` means: notify all current members of that team who can read the story/comment/entity, using mention notifications.

It should behave as a real mention, not as plain text, a generic update, or a follow-only side effect.

## Approaches Considered

### 1. Recommended: Shared backend mention-resolution service

Keep raw `@mention` text in entity bodies and descriptions, but add a shared backend layer that resolves user and team mentions into recipient user IDs.

Why this is preferred:

- Matches the current codebase, which already parses raw mentions in PM services.
- Avoids duplicating team expansion logic across story, comment, checklist, epic, sprint, and objective services.
- Delivers the required behavior without adding a new mention schema.

Trade-offs:

- Mention semantics remain save-time derived from raw text rather than first-class stored objects.

### 2. Per-service team mention expansion

Extend each PM entity service independently.

Advantages:

- Smaller local changes in each file.

Disadvantages:

- Repeats logic and makes behavior drift likely.
- Increases maintenance cost as more mentionable entities are added.

### 3. Structured mention entities

Persist mentions as explicit database entities rather than deriving them from text.

Advantages:

- Strongest long-term model for rendering, auditing, and future behavior.

Disadvantages:

- Larger schema, service, and editor change set than needed for the current goal.

## Approved Design

### Architecture

- Add a shared PM mention-resolution layer on the backend.
- That layer parses raw `@handles`, resolves them as user or team mentions, expands teams to current member user IDs, filters out users who cannot read the item, and returns a deduped recipient list.
- Reuse the same layer from all mentionable PM surfaces:
  - story create/update
  - comment create/update
  - checklist item create/update
  - epic create/update
  - sprint create/update
  - objective create/update
- Keep the existing frontend authoring model for now. Editors already insert `@team-handle`.

### Resolution Rules

- Parse raw `@handle` text on save.
- For each handle:
  - if it matches a user mention, add that user
  - if it matches a team handle, expand to current active members of that team
- Expansion must use current persisted team membership data, not cached frontend editor state.
- Deduplicate recipients across:
  - direct user mentions
  - expanded team mentions
  - any other explicit recipient sources
- Exclude the acting user from mention recipients.

### Access Rules

- `@team` must only notify members who can read the entity being mentioned.
- Mentioning a team must not grant access.
- In the current PM model, this will usually resolve to the whole team because PM access is workspace-scoped, but the access filter should still exist in backend logic for future safety.

### Notification Rules

- Team mentions use mention notifications, not generic update notifications.
- Fan-out is to users, not to a team object.
- If `@design` expands to six users, those six users receive mention notifications.
- If a user is both directly mentioned and included via team expansion, they still receive only one mention notification for that action.
- Mention notifications should use the existing mention-event style:
  - existing: `story.mention`, `comment.mention`, `checklist.mention`
  - add: `epic.mention`, `sprint.mention`, `objective.mention`
- All new mention event types must be wired into the notification category mapping, Mentions tab filtering, unread counts, and related notification tests so they behave like existing mention events in the UI.

### Supported Surfaces

Team mentions should work everywhere mentions are currently authored in PM:

- story descriptions
- story comments
- checklist item text
- epic descriptions
- sprint descriptions
- objective descriptions

### Rendering

- Keep authoring UX the same for the first version: editors still autocomplete `@team-handle`.
- On display, resolve team mentions as distinct team chips rather than generic highlighted text.
- If a team mention cannot be resolved at render time, fall back to plain `@handle` text rather than breaking the UI.
- Read-time team mention rendering should use the existing workspace team list on the frontend, passed into mention-rendering components as optional team metadata, rather than introducing a new mention-resolution API just for display.

### Collision Rule

- If a handle could match both a user and a team, treat it as the team mention.

Reason:

- Team handles are explicit persisted identifiers.
- User handles are currently inferred from names rather than stored as first-class unique identifiers.

### Time Semantics

- Team membership is evaluated at mention time only.
- Historical notifications are not recalculated when team membership changes later.
- Renaming a team handle affects future mentions only, not past notifications.

## Implementation Shape

### Backend

- Add a shared PM mention-resolution helper/service that can:
  - extract raw handles
  - resolve explicit team handles
  - expand team members
  - resolve user mentions
  - dedupe recipients
  - apply access filtering
- Replace duplicated direct user-only resolution logic in current PM services with that shared layer.
- Extend epic, sprint, and objective services to emit mention notifications on create/update when their description fields contain mentions.

### Frontend

- Keep existing mention suggestion behavior.
- Extend mention-rendering components to accept optional team metadata sourced from the existing workspace team list, so team mentions can display distinctly from person mentions without changing PM entity response shapes.
- Do not block backend rollout on richer rendering; notification correctness is the first priority.

## Testing

### Backend

- Add unit tests for:
  - handle extraction
  - team-handle resolution
  - team member expansion
  - recipient deduplication
  - access filtering
  - actor exclusion
- Add service-level tests covering create/update mention fan-out for:
  - stories
  - comments
  - checklist items
  - epics
  - sprints
  - objectives
- Add regression tests proving `@team` emits mention notifications, not generic update notifications.

### Frontend

- Add follow-up tests for rendering team mentions distinctly once display behavior is implemented.
- Rendering tests are lower priority than backend notification correctness.

## Risks

- Team handles are first-class persisted identifiers, but user handles are inferred from names. The collision rule above avoids ambiguity for the first version, but user-handle semantics may need to be formalized later.
- Rendering and backend resolution can drift if the frontend keeps treating all mentions as member mentions only. This is acceptable for the first rollout as long as unresolved mentions degrade safely.

## Open Questions

- None. The approved behavior is:
  - `@team-handle` works everywhere mentions are authored in PM
  - it notifies all readable current team members
  - it uses mention notifications
  - it does not grant access
