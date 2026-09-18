# PM Recurring Work Design

> Source review, 2026-09-17

Historical recurring-work design. The implemented domain now uses tasks, with
[recurring templates](../../server/internal/service/pm_recurring_template.go),
a [scheduler](../../server/internal/service/pm_recurring_template_scheduler.go), and
[a Temporal workflow](../../server/internal/temporalapp/pm_recurring_workflow.go).
The workflow declares a five-minute cron interval. The story-named examples and
proposed file changes below are not a current implementation checklist. Source
presence does not establish that a deployment's scheduler is running.

## Summary

Helpin needs a first-class recurring work system for PM, not a single `repeat` checkbox on stories. The correct model is a recurring template that owns recurrence rules and lifecycle state, and generates normal story instances over time.

This design keeps generated stories fully editable and historically independent, while giving operators one place to manage schedules, failures, pause/resume state, and occurrence history.

## Product Shape

### Core model

- A recurring template is the source of truth.
- Each run creates a new normal `PMStory`.
- Generated stories keep a link back to the recurring template and a run/occurrence identifier.
- Past generated stories are never mutated when the recurring template changes.

### User entry points

Recurring configuration should exist in all of these places:

1. Story create flow
   - Create a normal story.
   - Turn it into recurring during creation.
2. Story edit/detail flow
   - Edit recurrence from story detail.
   - Add a quick action such as `Make recurring`.
3. Workspace settings
   - Add `Settings -> Recurring Tasks` for centralized management.

### Management view

The recurring management surface should show:

- active, paused, stopped, and failed templates
- recurrence rule summary
- next run
- team
- assignee / primary owner
- last generated story
- last run status / last error
- actions: edit, pause, resume, stop, skip next, generate now, duplicate

### Generated story behavior

Each generated story should:

- be a normal `PMStory`
- preserve its own comments, attachments, checklist state, activity, and history
- show a recurring badge / icon
- link back to the parent recurring template
- optionally expose an occurrence number in the recurring metadata

## Scope

### In scope

- recurring templates that generate stories/tasks
- time-based recurrence
- completion-based recurrence
- start/end controls
- pause/resume/stop/skip/generate-now/duplicate controls
- dedicated recurring management page
- story create/edit/detail integration
- optional sprint assignment logic for generated stories
- occurrence history
- retry-safe background generation
- duplicate prevention and last-error visibility

### Explicit boundary for v1

Recurring templates generate stories only.

Epics, objectives, and sprints remain supported as linked context on generated stories:

- a generated story may link to an epic
- a generated story may link to an objective through its epic/story associations
- a generated story may optionally auto-assign to a sprint

This avoids over-generalizing the engine into recurring epics/objectives/sprints before the product proves that need.

### Not reusing `pm_story_templates`

`pm_story_templates` should remain the manual story-template feature. It is not the right persistence model for recurrence because it lacks:

- recurrence rules
- lifecycle state
- runtime metadata
- occurrence history
- failure tracking
- skip/pause controls

The recurring editor may reuse some story-template UI patterns, but the data model should be separate.

## Scheduling Model

### Time-based rules

The system should support:

- daily
- weekly
- monthly
- yearly
- every X days
- every X weeks
- every X months
- selected weekdays
- day of month

The scheduler should store structured rule fields rather than a user-facing cron string. Cron is an execution detail, not the product model.

### Completion-based rules

The system should also support templates that advance only after the current occurrence finishes:

- generate next when current story becomes completed
- generate next when current story moves to a done workflow state
- optionally create the next story immediately on completion and set due date by offset

### Start/end controls

Templates should support:

- starts on date
- no end date
- ends on date
- ends after N generated occurrences

### Generation rules

Each template should capture when the next instance is created:

- on schedule date
- only after current completion
- immediately after completion with due date offset

### Due date behavior

Recurring due dates should be relative, not copied as a fixed historical date. The template should support:

- no due date
- due on scheduled date
- due N days after generation / completion

## Data Model

### New tables

Add a dedicated recurring subsystem:

1. `pm_recurring_templates`
2. `pm_recurring_runs`

### `pm_recurring_templates`

This table should hold:

- `id`
- `workspace_id`
- `team_id`
- `status`
- `title`
- `description`
- `seed_payload` as JSON for story defaults
- schedule fields
- generation mode fields
- sprint assignment mode
- start/end fields
- `next_run_at`
- `last_run_at`
- `last_generated_story_id`
- `last_error`
- `failure_count`
- `generated_count`
- `skipped_until`
- `created_by`
- `updated_by`
- timestamps

`seed_payload` should carry the story defaults that are actually supported today, including:

- name/title
- description
- story type
- priority
- severity
- team
- owner/requester fields
- label ids
- epic/sprint linkage inputs where allowed
- checklist template items
- external links
- other current story-create inputs

The payload should be structured JSON instead of the current stringified-json pattern used by `pm_story_templates`.

### `pm_recurring_runs`

This table should hold execution history:

- `id`
- `workspace_id`
- `template_id`
- `occurrence_number`
- `trigger_type`
- `scheduled_for`
- `started_at`
- `finished_at`
- `status`
- `generated_story_id`
- `dedupe_key`
- `error`
- timestamps

This is the source for:

- occurrence history
- auditability
- duplicate prevention
- operational debugging

### Story linkage

Add new fields to `pm_stories` for recurring generation:

- `recurring_template_id`
- `recurring_run_id`
- `recurring_occurrence_number`

Do not overload the existing `template_id` field because it is already used by the manual story-template flow.

## Runtime Architecture

### Source of truth

The database is the scheduling source of truth:

- the template stores rule configuration and `next_run_at`
- the run log stores every generation attempt
- generated stories reference the template/run that created them

### Background execution

Use a singleton Temporal cron workflow that wakes up on a short interval and processes due templates.

This fits the current codebase better than one Temporal schedule per recurring template because:

- completion-based templates still need DB-driven advancement
- editing many templates does not require creating/tearing down many Temporal schedules
- pause/resume/stop changes stay inside normal row updates
- the management screen can read everything from DB state directly

The runner should:

1. claim due templates transactionally
2. insert a run record with a unique dedupe key
3. create the story instance
4. update template runtime fields
5. compute and persist the next run
6. record failure state when generation fails

### Completion-triggered advancement

Story completion and move-to-done flows should also call into the recurring service:

- if the story belongs to a completion-based recurring template
- and it is the active/latest occurrence for that template
- then the service should create the next run or schedule it immediately based on the template rule

### Duplicate prevention

Prevent duplicates at the database layer:

- unique run dedupe key per template occurrence / trigger
- transactional generation
- idempotent generation service

Temporal retries should be safe because re-running the same due template should see the existing run and avoid a second story.

## API Shape

Add dedicated recurring endpoints rather than overloading story-template routes.

Expected backend surfaces:

- list recurring templates
- get recurring template detail
- create recurring template
- update recurring template
- pause recurring template
- resume recurring template
- stop recurring template
- skip next occurrence
- generate now
- duplicate recurring template
- list recurring runs / occurrences for a template
- get recurring summary for a story

Story create/update endpoints should also accept recurrence configuration helpers where appropriate so the UI can create a story and recurring template in one flow.

## Frontend Shape

### Story create/edit/detail

Add a recurrence section to the story create/edit UI that supports:

- enable recurring
- configure cadence/trigger
- configure start/end
- configure due-date behavior
- configure sprint assignment behavior

Detail views should show:

- whether the story was generated from recurrence
- the parent recurring template
- quick access to edit recurring behavior

### Recurring management screen

Add `Settings -> Recurring Tasks`.

This should be the operator surface for:

- browsing templates
- filtering by status/team
- searching templates
- seeing next/last run
- seeing last generated story
- taking lifecycle actions

### Filters and badges

Generated stories should expose:

- a recurring badge in story list/board/detail
- a link to the parent recurring template

Recurring-template-specific searching and filtering should live in the dedicated management surface. Story list/board filtering can add a simple recurring-origin filter once the base system is in place.

## Reliability and Auditability

The system should expose:

- last run
- next run
- last generated story
- last error
- failure count
- current template status
- occurrence history

Actions such as pause/resume/stop/skip/generate-now should emit PM activity or recurring-run history so operators can understand why generation happened or stopped.

## Product References

This design intentionally borrows from the current product patterns of:

- Linear recurring issues for issue-level conversion plus a dedicated management screen: <https://linear.app/docs/creating-issues>
- ClickUp recurring tasks for time-based and completion-based rule flexibility: <https://help.clickup.com/hc/en-us/articles/7246322970519-Use-recurring-tasks-on-mobile>
- Plane recurring issues for the template-plus-history direction: <https://plane.so/changelog/release-v2-2-1-bug-fix-page-lock-and-unlock>

## Testing Strategy

### Backend

- recurrence rule validation tests
- next-run calculation tests
- duplicate-prevention tests
- due-template processing tests
- completion-trigger generation tests
- pause/resume/skip/generate-now tests
- sprint-assignment tests

### Frontend

- recurrence form behavior in story create/edit
- recurring management screen list and actions
- recurring badge / parent-link rendering
- story-detail recurring summary

### End-to-end behavior

- create recurring story template from story UI
- scheduled generation produces a normal story
- completion-based template generates the next story
- pausing stops generation
- generate-now creates exactly one story
- failed generation records error state without duplicating runs
