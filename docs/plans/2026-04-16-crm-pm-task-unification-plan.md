# CRM and PM task unification plan

This historical plan records the move from task-shaped CRM activity to shared PM
tasks. It is useful migration context for contributors; the proposed sales-only
creation flow and blanket sequence-deletion scope no longer describe the product.

## Source review — 2026-09-18

- [CRM activity types](../../server/internal/model/crm_activity.go) now contain note, call, meeting, and email; PM tasks are the follow-up primitive. [LinkedTasksPanel](../../frontend/src/components/crm/LinkedTasksPanel.tsx) lists and creates shared PM tasks, with contact/deal filters and `company_rollup_id` for company context.
- The panel selects from existing accessible teams and remembers a workspace-specific choice in browser `localStorage`. It does not require a sales team or silently create one. Without PM edit access or an accessible team it disables creation. Task creation and CRM association are separate API calls, so an association failure can leave a created task unlinked.
- [SettingsService.EnsureDefaultTeam](../../server/internal/service/settings.go) exists separately. It chooses the earliest-created team of the requested type and resolves handle uniqueness races by re-querying, rather than the proposed row lock or alphabetical preference. The actor is added as team owner on creation. Its existence does not mean the CRM task panel invokes it.
- [The activity migration](../../server/internal/dbmigrate/sql/202604170001_crm_task_activity_to_pm_task.sql) creates task/association records then deletes task-type activities. Its actual insert does not preserve the full source row in `custom_properties` as the risk mitigation proposed, and it leaves `completed` false even when choosing the workflow's `Lost` state for past occurrences. The final deletion targets all task activities, not only the inserted mapping; do not treat this historical SQL as a reusable recovery procedure.
- [The trim migration](../../server/internal/dbmigrate/sql/202604170002_drop_crm_hubspot_bloat.sql) removed the old property/list/sequence tables. Later [CRM outreach models](../../server/internal/model/crm_outreach.go) introduce email sequences, enrollments, and deliveries. The old “delete sequences” instruction is therefore not a current cleanup task.
- Schema rollout, tenant migration results, and production data-loss claims were not verified. Preserve versioned migrations; use current authorization and schema tooling rather than replaying the historical hard-cutover checklist. The old `story.state_entered` wording is legacy naming, not a reason to add another CRM task event.

## Original plan

## Status

Draft plan covering two closely related concerns surfaced during CRM module review:

1. **Tasks**: "sales tasks" and "PM tasks" should be the same primitive. Today CRM has no real task model — `crm_activities` rows with `activity_type="task"` stand in for it — while `pm_tasks` already has the fields sales needs (owner, deadline, workflow state, priority, labels, followers).
2. **Data model scope**: the CRM is carrying several entities copied from HubSpot (property definitions, property groups, smart lists, sequences) that duplicate capabilities the rest of the platform already provides or that no customer has asked for.

This plan assumes:

- the canonical task primitive for every team — engineering, marketing, **and sales** — is `PMTask` in `server/internal/model/pm_task.go`
- `WorkspaceTeam.team_type` (currently free-form string defaulting to `'engineering'`) becomes the seam that distinguishes sales from engineering work, with team-scoped workflow states
- `CRMActivity` remains the log of things-that-happened (call, meeting, email, note) — the `task` activity type is **removed**
- `crm_associations` stays as the many-to-many link layer between `pm_tasks` and `contacts` / `companies` / `deals`; no new join table is introduced
- the rollout is a **hard cutover** for tasks (small data volume — `crm_activities` where `activity_type='task'` are migrated into `pm_tasks`) and **dead-code deletion** for unused HubSpot-parity tables
- no change to Pipelines, Stages, Deals, Contacts, Companies, or the signal-detection / enrichment stack — those are working and not the target of this plan

## Goal

Make the CRM module behave like every other module in Helpin: it composes on top of shared primitives (teams, tasks, workflows, labels) rather than building parallel ones.

Concretely:

- a salesperson viewing a contact or a deal sees a list of `pm_tasks` scoped to that CRM object — not a list of "task activities" with no assignee
- a sales team appears in `/w/:slug/pm` alongside engineering and marketing teams, with its own workflow (e.g. `To call → Contacted → Follow-up → Closed` instead of `Backlog → In progress → Done`)
- the same task shows up on both the sales kanban and the linked deal's right-rail — one row, two surfaces
- the CRM data model shrinks to what actually drives value: objects, pipeline, stages, activities, associations, signals, enrichment. HubSpot-parity bolt-ons that nobody uses get deleted.

## Core Decisions

### 1. PMTask is the single task primitive across all teams

No new `crm_tasks` table. No polymorphic task parent. `PMTask` already has:

- `OwnerID` / `OwnerMemberID` + `PMTaskOwner` join for multi-owner (`server/internal/model/pm_task.go`)
- `WorkflowID` / `WorkflowStateID` for kanban-style status
- `TeamID` for team scoping
- `Priority`, `Severity`, `Deadline`, `Estimate`, `Position`
- `Labels` (many-to-many), `Followers`
- `RequesterID` / `RequesterMemberID`
- `display_id` + computed `task_key` (`HLP-123`) for human-readable references
- `ContactID` / `CompanyID` / `DealID` filter fields on `PMTaskFilters` (`server/internal/model/pm_task.go:115-117`)

These are exactly the fields a sales task needs. The gap is not in the model — it is in the UI and the team/workflow defaults.

### 2. Sales is a `team_type`, not a new module

`WorkspaceTeam.team_type` is already a free-form string column (`server/internal/model/settings.go:46`). Promote the set of recognized values to constants:

```go
const (
    TeamTypeEngineering = "engineering"
    TeamTypeMarketing   = "marketing"
    TeamTypeSales       = "sales"
    TeamTypeSupport     = "support"
)
```

Seed sensible defaults per team type when a team is created:

| team_type     | default_task_type | default workflow states                                          |
|---------------|-------------------|------------------------------------------------------------------|
| engineering   | feature           | Backlog → In progress → In review → Done                         |
| marketing     | chore             | Backlog → In progress → In review → Done                         |
| sales         | chore             | To contact → Contacted → Meeting booked → Proposal → Won / Lost  |
| support       | chore             | New → In progress → Waiting → Done                               |

Team-type is purely a **default seed** — after creation the team owns its workflow and nothing about `team_type` constrains what it can do. This preserves the existing model's flexibility while giving sales users a meaningful starting point.

### 3. CRM "task activity" is retired

Delete the `task` value from the CRM activity-type enum:

- `server/internal/model/crm_activity.go:11` — remove `CRMActivityTask`
- `frontend/src/components/crm/ActivityTimeline.tsx:34,37` — remove the `task` entry from the create menu and the filter set

`CRMActivity` continues to represent **things that happened** (note, call, meeting, email). It stops pretending to represent **things to do**.

Migration: every `crm_activities` row with `activity_type='task'` is converted into a `pm_tasks` row in the workspace's default sales team, with:

- `pm_tasks.name` ← `crm_activities.subject` (fallback: first 80 chars of body)
- `pm_tasks.description` ← `crm_activities.body`
- `pm_tasks.owner_member_id` ← `crm_activities.owner_member_id`
- `pm_tasks.deadline` ← `crm_activities.occurred_at` (treated as due date, not occurrence date)
- `pm_tasks.workflow_state_id` ← the new sales workflow's "To contact" state (or "Done" if `occurred_at < now()`)
- A `crm_associations` row is created linking the new `pm_tasks` row to the activity's `contact_id` / `company_id` / `deal_id` where present

The activity rows themselves are **deleted** after the backfill succeeds. There is no dual-write period — this is a hard cut.

### 4. Render PM tasks inside CRM surfaces

The association plumbing already exists:

- `PMTaskFilters.ContactID` / `CompanyID` / `DealID` (`server/internal/model/pm_task.go:115-117`)
- Handler layer already accepts `contact_id` / `company_id` / `deal_id` query params on task list
- `crm_associations` table already contains `task` as a valid `from_object_type` / `to_object_type`

What changes is purely frontend composition:

- Contact detail right-rail: add a "Tasks" panel backed by `GET /api/pm/tasks?contact_id=...`
- Company detail: same, with `company_id`
- Deal detail: same, with `deal_id`
- "Add task" button on each CRM object creates a `pm_task` with the association pre-populated, defaulting to the workspace's sales team + workflow

Under the hood, creation goes through the existing `PMTaskService.Create` — no CRM-specific task service is introduced. The CRM UI is a caller, not a model owner.

### 5. No sales team at task-creation time: lazy auto-create

Not every workspace will have a sales team when a user first clicks "Add task" from a CRM surface. Possible causes:

- workspace existed before this plan shipped and never had a sales team (Migration A only auto-creates for workspaces with CRM activity rows to backfill; empty CRM workspaces get skipped)
- CRM is enabled post-onboarding, after any team-seeding step has already run
- a workspace admin deleted the sales team

The rule: **the first write to a sales-scoped surface lazily creates the sales team.** The user never sees a dead-end "you need to create a team first" state.

Resolution order when the CRM "Add task" action fires on a workspace:

1. If any `WorkspaceTeam` with `team_type='sales'` exists → use it. If there is more than one, respect the user's last-selected sales team (persisted on `WorkspacePerson.preferences` or equivalent); otherwise the alphabetically-first one.
2. Otherwise → call `WorkspaceTeamService.EnsureDefault(ctx, workspaceID, TeamTypeSales)`, which:
   - creates a `workspace_teams` row with `name='Sales'`, `handle='sales'` (deduped with a numeric suffix if taken), `team_type='sales'`, `default_task_type='chore'`
   - seeds the sales default workflow + states (Decision 2)
   - adds the current user as a team member with a sensible role (member, or admin if they have `PermSettingsManage`)
   - returns the new team ID
3. Proceed with task creation against the resolved team.

This path is **idempotent and side-effect-safe**: `EnsureDefault` uses a `SELECT ... FOR UPDATE` on `workspace_teams` keyed by `(workspace_id, team_type)` to prevent two concurrent "Add task" clicks from creating two sales teams. The same helper is used by Migration A so the two code paths converge on identical seed state.

Surfaces that trigger lazy creation:

| Trigger                                                | Behavior                                                                              |
|--------------------------------------------------------|---------------------------------------------------------------------------------------|
| "Add task" on a CRM contact / company / deal           | Silent lazy-create + open the task modal pre-filled                                   |
| Manual PM task create with "Sales" team typed / picked | Not applicable — the team picker only offers existing teams; this path is irrelevant  |
| CRM settings "Sales team" dropdown                     | Shows "+ Create sales team" as a first-class option with the same `EnsureDefault` call |
| Sales-team-scoped automation rule fires                | Rule execution is skipped with a WARN log if no sales team exists; rules do not auto-create teams (avoids silent team creation from background events) |

Surfaces that do **not** trigger lazy creation:

- Read paths (listing tasks by `contact_id`): return an empty list cleanly. No team is inferred.
- The autonomy / signal pipeline: if a deal-automation workflow wants to spawn a follow-up task and no sales team exists, it logs and proceeds without the task rather than silently creating workspace infrastructure from a background job.

UX copy on the first lazy-create: a single non-blocking toast — "Created your Sales team" — with a "Configure" action that deep-links to `/w/:slug/pm/teams/:handle/settings`. No modal, no confirmation step. The user can rename / reconfigure the team after the fact; nothing about it is irreversible.

**Permission gate**: lazy team creation requires the caller to have `PermSettingsManage` (team creation today already requires this). If the caller lacks the permission:

- the CRM "Add task" button is disabled with a tooltip: "Ask your workspace admin to create a Sales team to track this work"
- the CRM settings dropdown shows the "+ Create sales team" option grayed out with the same tooltip
- no API call is made; the 403 case is prevented at the UI layer

This keeps the fast path fast for admins (who run onboarding and typically click the first "Add task") while giving viewers/members a clear signal rather than a silent failure.

### 6. HubSpot-parity bloat to delete

The following tables are present, backed by GORM models and migrations, but are not load-bearing for the signal-detection / enrichment / deal-automation flows that actually drive CRM value. They should be removed:

| Entity                      | Model file                                | Why it goes                                                                                                   |
|-----------------------------|-------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| `CRMPropertyDefinition`     | `server/internal/model/crm_property.go`   | Every CRM object already has `custom_properties JSONB`. The schema registry adds UI complexity with no customer ask — an AI-first product doesn't need a no-code field builder. |
| `CRMPropertyGroup`          | `server/internal/model/crm_property.go`   | Only exists to organize property definitions in a settings UI that does not exist.                            |
| `CRMList` (smart + static)  | `server/internal/model/crm_list.go`       | The query-builder layer (CLAUDE.md "Query Builder Conventions") is the canonical segmentation primitive. Adding a second one bifurcates the story. |
| `CRMSequence` + enrollments | `server/internal/model/crm_sequence.go`   | Outbound email cadences are a whole product surface. Until a customer asks, the agent/automation layer (`agents-and-automation.md`) is a better substrate for any cadence-like behavior. |

What **stays**:

- `CRMContact`, `CRMCompany`, `CRMDeal`, `Pipeline`, `PipelineStage`
- `CRMActivity` (minus `task` type)
- `CRMAssociation`
- `CRMSignal`, `CRMSignalSource`, `CRMSuggestion`, `CRMSummary`, `CRMAutonomySettings`, `CRMEnrichment`, `CRMWritingProfile`
- Gmail / calendar / email sync infrastructure (`crm_email*.go`, `crm_calendar.go`)
- `custom_properties JSONB` columns on Contact / Company / Deal — extensibility stays; the formal registry goes

### 7. Authorization

Sales-team tasks reuse the existing PM RBAC surface (`PermPMEdit`, `PermPMRead`). A separate `pm.sales.edit` permission is **not** introduced — the team-membership check already gates who can act on a team's tasks.

CRM-side permissions (`PermCRMEdit`, `PermCRMRead`) continue to gate the CRM surfaces (contact/deal pages, associations). Creating a PM task from a CRM surface requires **both** `PermCRMRead` (to view the CRM object) and membership in the target sales team (to create the task).

## Migration Plan

Two dbmigrate SQL files, applied in order by `server/internal/dbmigrate`:

### Migration A — `YYYYMMDDNNNN_crm_task_activity_to_pm_task.sql`

1. For each workspace with CRM enabled and no existing sales team: create one (`team_type='sales'`), seed the sales workflow + states.
2. For each `crm_activities` row with `activity_type='task'`:
   - insert a `pm_tasks` row mapped as described in Decision 3
   - insert `pm_task_owners` row if owner_member_id present
   - insert `crm_associations` rows linking the new task to the activity's contact / company / deal
3. Delete migrated `crm_activities` rows.
4. Add a CHECK constraint forbidding `activity_type='task'` going forward (defensive — app code also drops the enum value).

### Migration B — `YYYYMMDDNNNN_drop_crm_hubspot_bloat.sql`

1. `DROP TABLE IF EXISTS crm_property_definitions, crm_property_groups CASCADE;`
2. `DROP TABLE IF EXISTS crm_lists, crm_list_memberships CASCADE;`
3. `DROP TABLE IF EXISTS crm_sequences, crm_sequence_steps, crm_sequence_enrollments CASCADE;`
4. Remove any FK columns that referenced these tables from retained tables.

Both migrations are idempotent. They run through the existing `dbmigrate` runner with ArgoCD PreSync Jobs; `RUN_AUTO_MIGRATE` stays true since the additive parts (sales team defaults, constants) are all covered by existing `AutoMigrate` semantics.

## Backend Changes

### Models

- `server/internal/model/crm_activity.go`: remove `CRMActivityTask` constant. Update doc comment.
- `server/internal/model/settings.go`: add `TeamType*` constants. Document default workflows per team type in comment on `WorkspaceTeam`.
- Delete: `server/internal/model/crm_property.go`, `crm_list.go`, `crm_sequence.go`.

### Services

- `server/internal/service/crm_activity.go`: reject `activity_type='task'` at create time (belt-and-braces after enum removal).
- `server/internal/service/workspace_team.go` (or equivalent): when creating a team with `team_type='sales'`, seed the sales default workflow + states via the existing workflow-seeding path.
- Delete: `crm_property_*.go`, `crm_list_*.go`, `crm_sequence_*.go` under `service/`, `repository/`, and `handler/`.

### Handlers / Routes

- Remove `/api/crm/properties`, `/api/crm/property-groups`, `/api/crm/lists`, `/api/crm/sequences` route groups from `server/internal/router/router.go`.
- Ensure `/api/pm/tasks` already supports `?contact_id=...&company_id=...&deal_id=...` filters end-to-end (should be true today; verify).

### Automation / Agents

- `docs/agents-and-automation.md` lists `story.state_entered` as an automation trigger. That event already fires on `pm_tasks` post-rename; it will now fire for sales-team tasks too, giving the CRM autonomy layer a native way to react to task state changes without a CRM-specific event type.

## Frontend Changes

### Types

- `frontend/src/lib/crmTypes.ts`: remove `CRMPropertyDefinition`, `CRMPropertyGroup`, `CRMList`, `CRMSequence*`, and `CRMActivityType = 'task'` member. Update `CRMActivity` union.
- `frontend/src/lib/pmTypes.ts`: no schema change — `Task` already has everything. Add `TeamType` enum mirror for typed team switching.

### Components

- `frontend/src/components/crm/ActivityTimeline.tsx` (`:27,34,37`): remove `task` icon entry, create-menu entry, and filter option.
- New: `frontend/src/components/crm/LinkedTasksPanel.tsx` — renders `useTasks({ contactId | companyId | dealId })` in the right-rail of Contact / Company / Deal pages. "Add task" button opens the existing PM task-create modal pre-filled with the association + the sales team.
- Delete: any `PropertyDefinitionEditor`, `ListBuilder`, `SequenceEditor` components under `frontend/src/components/crm/`.

### Routes

- `/w/:slug/pm` team switcher: nothing to change — it already enumerates `WorkspaceTeam` rows; sales teams will appear automatically once a workspace has one.
- Remove `/w/:slug/crm/settings/properties`, `/lists`, `/sequences` routes from `frontend/src/routes/`.

### Query Hooks

- `frontend/src/hooks/queries/`: delete `useCRMProperties`, `useCRMLists`, `useCRMSequences` and their mutations.
- `useTasks` (existing) already accepts `contactId` / `companyId` / `dealId` — confirm the shape and extend if needed.

## Out of Scope

- Cross-workspace task references
- A dedicated "sales pipeline board" view that interleaves deal stages with task columns (deals and tasks stay separate surfaces; tasks link to deals, not the other way)
- Reviving sequences under the automation/agent substrate — that is a separate design, not a prerequisite
- Custom-property UX redesign. The JSONB column stays; how users edit arbitrary fields is unchanged (field-level UI lives with each object's detail view).
- Any change to signal detection, enrichment, or deal automation — those already work on the entities we are keeping.

## Risks

- **Lost data in the activity→task migration**: CRM activity rows of type `task` may carry ad-hoc fields in `metadata` that don't map onto `pm_tasks`. Mitigation: preserve the full original row as JSON in `pm_tasks.custom_properties` under `crm_activity_origin` during migration. This is a one-time escape hatch; no code reads it.
- **Workspaces that have invested in CRM lists / sequences**: in principle none exist today (these surfaces were never shipped to the UI in a material way), but validate via a production row-count check before running Migration B. If any tenant has non-trivial list/sequence data, split Migration B off into a later phase and gate it behind a per-workspace flag.
- **Sales team default workflow collisions**: if a workspace already has a team named "Sales" with a custom workflow, do not overwrite it. The migration only seeds a sales team when none exists.
- **Authorization drift**: confirm `PermPMEdit` / `PermPMRead` are actually granted to the CRM-user roles that need to create sales tasks. If not, that is a seed-data fix, not a new permission.

## Rollout Order

1. Add `TeamType*` constants + sales-team workflow seeding (backend, additive, no migration)
2. Ship `LinkedTasksPanel` behind a feature flag, reading PM tasks with CRM filters
3. Run Migration A (activity→task backfill) in staging, verify counts
4. Remove `task` from activity-type enum (backend + frontend), drop the feature flag
5. Run Migration A in production
6. Separately: Migration B (drop property/list/sequence tables) — can ship in a later release
