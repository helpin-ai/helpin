# Historical repair of team-workflow task assignments

> Historical planning artifact, source-compared on 2026-09-17. This page
> explains an older workflow migration for contributors. “Stories,” proposed
> filenames, approximate line numbers, and database instructions below are from
> that period; do not execute them as a current repair procedure.

## Current implementation and differences

The [workflow service](../../server/internal/service/pm_workflow.go) now calls
`migrateTeamTasks` from workflow seeding and copying. Its mapping prefers
`(state_type, position)`, then state type, then the configured default or first
new state. Migration errors are logged without failing the already-created
workflow; success of seeding/copying does not prove that all tasks moved.

The [task repository](../../server/internal/repository/pm_task.go) implements
`MigrateTasksToWorkflow`. It matches the old workflow and state, excludes archived
tasks, includes both the target team's tasks and tasks with null `team_id`, and
sets their team to the target. That is broader than the team-only proposal below.
Each state mapping is updated separately without a transaction around this loop,
so failures can leave a partially migrated set.

The actual [legacy SQL](../../server/migrations/045_migrate_team_stories_to_team_workflows.sql)
is numbered **045**, not the proposed 043 filename. It still uses the old
`pm_stories` table and handles unassigned stories only when their workspace has
exactly one team. That SQL rule differs from the current repository helper.
Current migrations belong under `server/internal/dbmigrate/sql`; a repair for a
present installation must be based on its actual schema and migration state,
not direct replay of this archived SQL. No database mutation was run for this review.

## Original repair proposal

## Context

After introducing team-scoped workflows, old stories in statuses like "Ready for Dev" and "In Development" no longer appear on the Kanban board. They still show in the epics list.

**Root cause**: `SeedTeamWorkflow()` calls `CopyWorkflow()` which creates a NEW workflow with fresh state UUIDs. Existing stories still reference the OLD workspace-level workflow's state IDs. The board query `WHERE workflow_state_id IN (new_team_state_ids)` excludes them. The epics list query has no workflow filter, so stories appear there.

## Plan

### Step 1: Add `MigrateStoriesToWorkflow` repository method

**File**: `server/internal/repository/pm_story.go`

Add method that accepts a state mapping (`map[string]string` of oldStateID → newStateID) and bulk-updates stories:
- For each mapping entry, `UPDATE pm_stories SET workflow_id = newWfID, workflow_state_id = newStateID WHERE team_id = ? AND workflow_state_id = oldStateID AND archived = false`
- Returns total rows affected
- Uses the repo's `db` directly (no separate transaction param needed - caller can wrap if desired)

### Step 2: Add migration helpers to workflow service

**File**: `server/internal/service/pm_workflow.go`

Add two private helpers:
- `buildStateMapping(oldStates, newStates []model.PMWorkflowState, defaultStateID *string) map[string]string` — maps old→new by `(state_type, position)`, falling back to `state_type` only, then to default state
- `migrateTeamStories(ctx, teamID string, oldWf, newWf *WorkflowWithStates) (int64, error)` — builds mapping and calls `storyRepo.MigrateStoriesToWorkflow`

### Step 3: Call migration in `SeedTeamWorkflow`

**File**: `server/internal/service/pm_workflow.go` (line ~508-514)

After `CopyWorkflow` succeeds, call `s.migrateTeamStories(ctx, teamID, defaultWf, newWf)`. Log result. Non-fatal on error (log warning, don't fail workflow seeding).

### Step 4: Call migration in `CopyToTeam`

**File**: `server/internal/service/pm_workflow.go` (line ~481-487)

Same pattern — after `CopyWorkflow` succeeds, migrate stories. This prevents the issue when manually copying a workflow to a team.

### Step 5: SQL data migration for existing orphaned stories

**File**: `server/migrations/043_migrate_team_stories_to_team_workflows.sql`

One-time migration to fix already-orphaned stories:
- Find stories where `team_id` matches a team with its own workflow, but `workflow_id` still points to a different (old) workflow
- Map each story's old state to the new workflow's corresponding state by `(state_type, position)`, falling back to `state_type`, then default state
- Update `workflow_id`, `workflow_state_id`, and `updated_at`

## Files to Modify

| File | Change |
|------|--------|
| `server/internal/repository/pm_story.go` | Add `MigrateStoriesToWorkflow` method |
| `server/internal/service/pm_workflow.go` | Add `buildStateMapping`, `migrateTeamStories`; modify `SeedTeamWorkflow` and `CopyToTeam` |
| `server/migrations/043_migrate_team_stories_to_team_workflows.sql` | New migration file |

## Verification

1. Run the SQL migration against the database
2. Check that old stories now appear on the Kanban board for their respective teams
3. Create a new team → verify auto-seeded workflow also migrates any stories assigned to that team
4. Run `go build ./cmd/api` to verify compilation
5. Run existing tests: `go test ./internal/...`
