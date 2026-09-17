# Helpin project management module

**Version**: 1.0
**Date**: March 3, 2026
**Status**: Draft
**Author**: Engineering Team

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Goals & Objectives](#2-goals--objectives)
3. [Architecture Principles](#3-architecture-principles)
4. [Entity Hierarchy](#4-entity-hierarchy)
5. [Feature Specifications](#5-feature-specifications)
   - 5.1 [Objectives (OKRs)](#51-objectives-okrs)
   - 5.2 [Epics](#52-epics)
   - 5.3 [Stories](#53-stories)
   - 5.4 [Sub-tasks](#54-sub-tasks)
   - 5.5 [Checklists](#55-checklists)
   - 5.6 [Iterations (Sprints)](#56-iterations-sprints)
   - 5.7 [Workflows & States](#57-workflows--states)
   - 5.8 [Labels](#58-labels)
   - 5.9 [Custom Fields](#59-custom-fields)
   - 5.10 [Story Links & Dependencies](#510-story-links--dependencies)
   - 5.11 [Comments & Activity](#511-comments--activity)
   - 5.12 [File Attachments](#512-file-attachments)
   - 5.13 [Templates](#513-templates)
   - 5.14 [Docs](#514-docs)
6. [Views & UI](#6-views--ui)
   - 6.1 [Board View (Kanban)](#61-board-view-kanban)
   - 6.2 [List/Table View](#62-listtable-view)
   - 6.3 [Timeline/Roadmap View](#63-timelineroadmap-view)
   - 6.4 [Filtering & Search](#64-filtering--search)
   - 6.5 [Saved Views (Spaces)](#65-saved-views-spaces)
7. [Reports & Analytics](#7-reports--analytics)
8. [Notifications](#8-notifications)
9. [Teams & Permissions](#9-teams--permissions)
10. [Integrations](#10-integrations)
11. [Database Schema](#11-database-schema)
12. [API Design](#12-api-design)
13. [Frontend Structure](#13-frontend-structure)
14. [Relationship with Existing Bonus System](#14-relationship-with-existing-bonus-system)
15. [Implementation Phases](#15-implementation-phases)
16. [Non-Goals & Future Considerations](#16-non-goals--future-considerations)

---

## 1. Executive Summary

Helpin is expanding from a performance-based quarterly bonus system to include a full-featured project management module. This module replaces Shortcut.com ($6,000/year), providing work tracking (stories, epics, iterations) integrated within the same platform where performance is evaluated.

**Key principle**: The PM module is built as **separate, independent tables** (prefixed with `pm_`) alongside the existing bonus system. No existing tables or features are modified. Future integration between PM data and bonus scoring will be handled via a mapping layer in a later phase.

### What We're Building

A project management system modeled after Shortcut.com's feature set, covering:

- **Objectives** — Strategic and tactical quarterly goals with optional Key Results (OKR)
- **Epics** — Large initiatives containing multiple stories
- **Stories** — Atomic work items (features, bugs, chores) with full workflow tracking
- **Sub-tasks** — Lightweight child stories with workflow states
- **Iterations** — Time-boxed sprints with velocity and burndown tracking
- **Workflows** — Customizable state machines per team
- **Board, List, Timeline views** — Multiple ways to visualize and manage work
- **Reports** — Velocity, burndown, cumulative flow, cycle time, lead time
- **Docs** — Rich-text documents linked to work items
- **Labels, Custom Fields, Templates, Comments, Attachments** — Full collaboration toolkit

---

## 2. Goals & Objectives

### Primary Goals

1. **Eliminate Shortcut.com dependency** — Save $6,000/year by replacing external tool
2. **Unified platform** — Work tracking and performance evaluation in one system
3. **Team familiarity** — Mirror Shortcut concepts the team already knows
4. **Future integration** — Design for eventual connection between PM metrics and bonus scoring

### Success Metrics

- Team fully migrated off Shortcut.com within 60 days of launch
- Feature parity with team's actual Shortcut usage (not 100% of Shortcut features)
- Zero disruption to existing bonus/scoring workflows

---

## 3. Architecture Principles

### 3.1 Separation from Existing System

```
Existing Tables (UNTOUCHED):        New PM Tables:
├── users                           ├── pm_objectives
├── workspaces                      ├── pm_key_results
├── workspace_members               ├── pm_epics
├── workspace_settings              ├── pm_stories
├── workspace_people                ├── pm_sub_tasks
├── workspace_teams                 ├── pm_checklists
├── team_memberships                ├── pm_checklist_items
├── workspace_managers              ├── pm_iterations
├── quarters                        ├── pm_workflows
├── sprints                         ├── pm_workflow_states
├── company_goals                   ├── pm_labels
├── goal_team_contributions         ├── pm_story_labels
├── sprint_goals                    ├── pm_custom_fields
├── goal_drafts                     ├── pm_custom_field_values
├── individual_checks               ├── pm_custom_field_options
├── job_role_criteria               ├── pm_story_links
├── bonus_tiers                     ├── pm_comments
├── quarterly_bonus_calculations    ├── pm_attachments
├── quarterly_finance_settings      ├── pm_story_templates
├── bonus_audit_log                 ├── pm_docs
                                    ├── pm_doc_links
                                    ├── pm_saved_views
                                    ├── pm_notifications
                                    └── pm_activity_log
```

### 3.2 Shared Resources

The PM module **reuses** existing tables where appropriate (no duplication):

- **users** — Same authenticated users
- **workspaces** — PM entities are scoped to the same workspaces
- **workspace_members** — Same role-based access (owner, admin, manager, member, viewer)

### 3.3 Tech Stack Consistency

- **Backend**: Go + Chi router + GORM + PostgreSQL (same patterns as existing code)
- **Frontend**: React + TypeScript + shadcn/ui + TanStack Router (same stack)
- **Naming**: All new Go handlers in `pm_*.go`, repositories in `pm_*.go`, etc.
- **API prefix**: All new endpoints under `/api/pm/`

---

## 4. Entity Hierarchy

```
Objective (Strategic Goal)
├── Key Results (measurable outcomes — boolean, percent, numeric)
└── Epics (large initiatives — can belong to multiple Objectives)
     └── Stories (feature | bug | chore)
          ├── Sub-tasks (child stories with workflow tracking)
          ├── Checklists (lightweight to-do lists)
          ├── Comments (discussion thread)
          ├── Attachments (files)
          └── Story Links (blocks, relates to, duplicates)

Iteration (time-boxed sprint)
└── Stories (cross-epic, cross-team)

Workflow (per-team state machine)
└── Workflow States (Backlog → Unstarted → Started → Done)
     └── Stories move through states

Labels → apply to Stories, Epics, Iterations
Custom Fields → apply to Stories
Docs → link to Stories, Epics, Iterations, Objectives
```

---

## 5. Feature Specifications

### 5.1 Objectives (OKRs)

Objectives represent company or team-level strategic goals, typically aligned with quarters.

#### Two Types

| Type | Description |
|------|-------------|
| **Tactical** | Groups and tracks Epics by theme or release. No Key Results. Simple progress tracking based on Epic completion. |
| **Strategic** | Full OKR system with Key Results measuring outcomes. Epics attach to the Objective or to a specific Key Result. |

A Tactical Objective can be elevated to Strategic by adding a Key Result.

#### Objective Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(256) | yes | Objective title |
| description | text | no | Markdown supported |
| objective_type | enum | yes | `tactical` or `strategic` |
| state | varchar(50) | yes | Current workflow state |
| planned_start_date | date | no | Planned start |
| deadline | date | no | Target completion date |
| started | boolean | auto | Computed from epic progress |
| started_at | timestamptz | auto | When first epic started |
| completed | boolean | auto | Computed from epic progress |
| completed_at | timestamptz | auto | When all epics completed |
| owner_id | UUID | no | FK → users |
| position | integer | yes | Sort order |
| archived | boolean | no | Default false |
| created_by | UUID | yes | FK → users |
| created_at | timestamptz | auto | Creation timestamp |
| updated_at | timestamptz | auto | Update timestamp |

#### Key Result Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| objective_id | UUID | yes | FK → pm_objectives |
| name | varchar(256) | yes | Key result title |
| result_type | enum | yes | `boolean`, `percent`, `numeric` |
| initial_value | numeric | no | Starting value |
| current_value | numeric | no | Current measured value |
| target_value | numeric | no | Target value |
| progress | integer | auto | Computed 0-100 |
| position | integer | yes | Sort order |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Behaviors

- An Epic can belong to **multiple** Objectives (many-to-many via `pm_epic_objectives`)
- Objective progress auto-computes from Epic story completion percentages
- Strategic Objective progress factors in Key Result progress
- Objectives page shows: list grouped by state, progress bars, Epic counts
- Objectives appear on the Timeline/Roadmap view when dates are set

---

### 5.2 Epics

Epics represent large bodies of work — a user problem broken into multiple stories.

#### Epic Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(256) | yes | Epic title |
| description | text | no | Markdown supported (100K char limit) |
| epic_state | varchar(50) | yes | Current state (from epic workflow) |
| owner_id | UUID | no | FK → users |
| team_id | UUID | no | FK → workspace_teams (primary team) |
| planned_start_date | date | no | Planned start |
| deadline | date | no | Target completion |
| started | boolean | auto | Has any story been started |
| started_at | timestamptz | auto | When first story started |
| completed | boolean | auto | All stories done |
| completed_at | timestamptz | auto | When last story completed |
| position | integer | yes | Sort order |
| color | varchar(7) | no | Hex color #RRGGBB |
| archived | boolean | no | Default false |
| created_by | UUID | yes | FK → users |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Epic Workflow States

Epics have their own independent workflow system (separate from story workflows):

**Default states**: `To Do` (Unstarted) → `In Progress` (Started) → `Done` (Done)

Custom epic states can be added. Each custom state maps to a parent type: Unstarted, Started, or Done.

#### Epic Stats (computed, not stored)

| Metric | Description |
|--------|-------------|
| num_stories_total | All stories in this epic |
| num_stories_backlog | Stories in backlog states |
| num_stories_unstarted | Stories in unstarted states |
| num_stories_started | Stories in started states |
| num_stories_done | Stories in done states |
| num_stories_unestimated | Stories with no estimate |
| num_points | Total story points |
| num_points_done | Completed story points |

#### Epic Health

Each Epic has an optional health status:
- `on_track` — Work is progressing as expected
- `at_risk` — Potential issues that may affect delivery
- `off_track` — Significant issues, behind schedule
- `null` — No health set

Health updates include a text description/comment explaining the status.

---

### 5.3 Stories

Stories are the atomic unit of work. Every piece of work the team does is represented as a Story.

#### Story Types

| Type | Icon | Description |
|------|------|-------------|
| **feature** | ⭐ | Work that changes the product — new functionality, UI updates, API endpoints |
| **bug** | 🐛 | Things that are broken — defects requiring fixes |
| **chore** | ⚙️ | Work that must be done but doesn't change the product — tech debt, deps, DevOps |

#### Story Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| display_id | serial | auto | Human-readable numeric ID (TP-1, TP-2, ...) |
| name | varchar(512) | yes | Story title |
| description | text | no | Markdown supported (100K char limit) |
| story_type | enum | yes | `feature`, `bug`, `chore`. Default: `feature` |
| workflow_id | UUID | yes | FK → pm_workflows |
| workflow_state_id | UUID | yes | FK → pm_workflow_states |
| epic_id | UUID | no | FK → pm_epics |
| iteration_id | UUID | no | FK → pm_iterations |
| team_id | UUID | no | FK → workspace_teams |
| owner_id | UUID | no | Primary owner (FK → users) |
| requester_id | UUID | yes | Who created/requested (FK → users) |
| estimate | integer | no | Story points (null = unestimated) |
| priority | enum | no | `none`, `low`, `medium`, `high`, `urgent` |
| severity | enum | no | Bug-only: `none`, `low`, `medium`, `high`, `critical` |
| deadline | date | no | Due date |
| position | integer | yes | Sort order within workflow state |
| started | boolean | auto | Whether story entered a Started state |
| started_at | timestamptz | auto | When story first entered Started |
| completed | boolean | auto | Whether story reached Done |
| completed_at | timestamptz | auto | When story reached Done |
| moved_at | timestamptz | auto | Last workflow state change |
| blocked | boolean | auto | Is this story blocked by another |
| blocker | boolean | auto | Is this story blocking another |
| archived | boolean | no | Default false |
| template_id | UUID | no | FK → pm_story_templates (if created from template) |
| external_id | varchar(128) | no | External system mapping |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Story Owners (many-to-many)

A story can have **multiple owners** tracked via `pm_story_owners` join table:

| Field | Type | Description |
|-------|------|-------------|
| story_id | UUID | FK → pm_stories |
| user_id | UUID | FK → users |

#### Story Followers (many-to-many)

Users who receive notifications for this story, tracked via `pm_story_followers`:

| Field | Type | Description |
|-------|------|-------------|
| story_id | UUID | FK → pm_stories |
| user_id | UUID | FK → users |

Requesters and owners are auto-added as followers.

#### Computed Metrics

| Metric | Description |
|--------|-------------|
| lead_time | Seconds from creation to completion |
| cycle_time | Seconds from first Started state to completion |

---

### 5.4 Sub-tasks

Sub-tasks are lightweight child stories that live inside a parent story. They have their own workflow tracking.

#### Sub-task Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| story_id | UUID | yes | FK → pm_stories (parent) |
| name | varchar(512) | yes | Sub-task title |
| description | text | no | Markdown supported |
| workflow_state_id | UUID | yes | FK → pm_workflow_states (inherits parent's workflow) |
| owner_id | UUID | no | FK → users |
| estimate | integer | no | Story points (rolls up to parent total) |
| position | integer | yes | Sort order within parent |
| completed | boolean | auto | Whether sub-task reached Done |
| completed_at | timestamptz | auto | When sub-task completed |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Behaviors

- Sub-task estimates **roll up** to parent story's total (additive)
- When any sub-task moves to Started → parent auto-moves to first Started state (if not already)
- When ALL sub-tasks reach Done → parent auto-moves to first Done state
- Sub-tasks inherit Epic and Team from the parent story
- Checklist items can be **promoted** to sub-tasks

---

### 5.5 Checklists

Checklists are lightweight to-do lists within a story. Not full work items — no workflow tracking.

#### Checklist Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| story_id | UUID | yes | FK → pm_stories |
| name | varchar(256) | yes | Checklist name (e.g., "QA Steps") |
| position | integer | yes | Sort order within story |
| created_at | timestamptz | auto | |

#### Checklist Item Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| checklist_id | UUID | yes | FK → pm_checklists |
| text | text | yes | Item description (Markdown supported) |
| completed | boolean | no | Default false |
| assignee_id | UUID | no | FK → users |
| position | integer | yes | Sort order |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Use Cases

- QA verification steps
- Code review checklist
- Launch checklist
- Items taking under an hour

---

### 5.6 Iterations (Sprints)

An Iteration is a time-boxed period for a collection of Stories. Stories can span multiple Epics and Teams within a single Iteration.

#### Iteration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(128) | yes | Iteration name (e.g., "Sprint 12") |
| description | text | no | Sprint goals/notes |
| start_date | date | yes | Sprint start |
| end_date | date | yes | Sprint end |
| status | enum | auto | `unstarted`, `started`, `done` — computed from dates |
| team_id | UUID | no | FK → workspace_teams (primary team) |
| archived | boolean | no | Default false |
| created_by | UUID | yes | FK → users |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Iteration Status (auto-computed)

| Status | Condition |
|--------|-----------|
| `unstarted` | today < start_date |
| `started` | start_date ≤ today ≤ end_date |
| `done` | today > end_date |

#### Iteration Stats (computed, not stored)

| Metric | Description |
|--------|-------------|
| num_stories_total | All stories in this iteration |
| num_stories_backlog | Stories in backlog states |
| num_stories_unstarted | Stories in unstarted states |
| num_stories_started | Stories in started states |
| num_stories_done | Stories completed |
| num_stories_unestimated | Stories with no estimate |
| num_points | Total story points |
| num_points_done | Completed points |
| average_cycle_time | Avg seconds from start to complete |
| average_lead_time | Avg seconds from creation to complete |

#### Velocity Tracking

- Track completed story points per iteration over time
- Overall average velocity (all-time)
- Trailing 4-iteration average for capacity planning
- Breakdown by story type (feature/bug/chore)

---

### 5.7 Workflows & States

A Workflow is the state machine through which Stories move. Each team can have its own workflow.

#### Workflow Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(128) | yes | Workflow name (e.g., "Engineering") |
| description | text | no | What this workflow is for |
| team_id | UUID | no | FK → workspace_teams |
| default_state_id | UUID | no | State for new stories |
| auto_assign_owner | boolean | no | Auto-assign when unowned story starts |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Workflow State Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workflow_id | UUID | yes | FK → pm_workflows |
| name | varchar(128) | yes | State name (e.g., "In Review") |
| state_type | enum | yes | `backlog`, `unstarted`, `started`, `done` |
| position | integer | yes | Order within workflow (0 = leftmost) |
| color | varchar(7) | no | Hex color #RRGGBB |
| description | text | no | What stories belong here |
| wip_limit | integer | no | Max stories allowed in this state (null = unlimited) |
| is_default | boolean | no | Whether this is the default state for new stories |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### The Four State Types

| Type | Description | Example States |
|------|-------------|---------------|
| **backlog** | Unrefined ideas, not ready for work | "Backlog", "Icebox" |
| **unstarted** | Refined and ready, not yet in progress | "Ready for Dev", "To Do" |
| **started** | Actively being worked on | "In Development", "In Review", "In QA" |
| **done** | Finished | "Completed", "Deployed" |

States can only be reordered **within** their type.

#### Default Workflow (created for each new team)

1. **Backlog** (backlog)
2. **Ready for Development** (unstarted) — default state
3. **In Development** (started)
4. **In Review** (started)
5. **Done** (done)

#### Epic Workflow States

Stored in `pm_epic_workflow_states`:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(128) | yes | State name |
| state_type | enum | yes | `unstarted`, `started`, `done` |
| position | integer | yes | Sort order |
| color | varchar(7) | no | Hex color |
| is_default | boolean | no | Default state for new epics |
| created_at | timestamptz | auto | |

Default epic states: "To Do" (unstarted), "In Progress" (started), "Done" (done).

---

### 5.8 Labels

Labels are flexible tags for categorizing Stories, Epics, and Iterations.

#### Label Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(128) | yes | Unique within workspace |
| description | text | no | Optional description |
| color | varchar(7) | no | Hex color #RRGGBB |
| archived | boolean | no | Hidden from pickers when true |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Label Associations (many-to-many)

Three join tables for polymorphic label assignment:

**pm_story_labels**: story_id + label_id
**pm_epic_labels**: epic_id + label_id
**pm_iteration_labels**: iteration_id + label_id

#### Usage

- Filter stories, epics, iterations by label
- Labels appear on story cards in board view
- Each label has a reporting page showing story distribution

---

### 5.9 Custom Fields

Custom Fields add structured, searchable attributes to Stories.

#### Built-in Fields (always available)

| Field | Applies To | Values |
|-------|-----------|--------|
| **Priority** | All stories | none, low, medium, high, urgent |
| **Severity** | Bug stories only | none, low, medium, high, critical |

These are implemented as columns on the `pm_stories` table (not in the custom fields system).

#### User-Defined Custom Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(64) | yes | Field name |
| description | text | no | Field description |
| field_type | enum | yes | `single_select` (extensible to `text`, `number`, `date` later) |
| icon | varchar(32) | no | Icon identifier |
| position | integer | yes | Sort order in forms |
| enabled | boolean | no | Default true |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Custom Field Options (for single_select)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| custom_field_id | UUID | yes | FK → pm_custom_fields |
| value | varchar(128) | yes | Display text |
| color | varchar(7) | no | Hex color |
| position | integer | yes | Sort order |

#### Custom Field Values (story ↔ field ↔ option)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| story_id | UUID | yes | FK → pm_stories |
| custom_field_id | UUID | yes | FK → pm_custom_fields |
| option_id | UUID | no | FK → pm_custom_field_options (for single_select) |
| text_value | text | no | For future text fields |
| number_value | numeric | no | For future number fields |

Unique constraint on (story_id, custom_field_id).

---

### 5.10 Story Links & Dependencies

Stories can be linked to express relationships and dependencies.

#### Story Link Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| source_story_id | UUID | yes | FK → pm_stories |
| target_story_id | UUID | yes | FK → pm_stories |
| link_type | enum | yes | `blocks`, `relates_to`, `duplicates` |
| created_by | UUID | yes | FK → users |
| created_at | timestamptz | auto | |

#### Link Types & Behavior

| Type | Forward | Inverse | Behavior |
|------|---------|---------|----------|
| **blocks** | "A blocks B" | "B is blocked by A" | Target story shows blocked indicator |
| **relates_to** | "A relates to B" | "B relates to A" | Symmetric — informational only |
| **duplicates** | "A duplicates B" | "B is duplicated by A" | Duplicate expected to be archived |

#### Auto-computed Story Fields

- `story.blocked = true` when any story has a `blocks` link targeting this story (and blocking story is not Done)
- `story.blocker` is reserved for external non-story blocker notes
- stories that block other stories are surfaced through derived `blocking` relationship views, not a separate boolean
- When blocking story reaches Done → blocked indicator automatically clears

#### Dependency Visualization

On Epic and Iteration detail pages, render a dependency graph showing all blocking/blocked-by relationships as a directed graph.

---

### 5.11 Comments & Activity

#### Comment Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| entity_type | enum | yes | `story`, `epic`, `doc` |
| entity_id | UUID | yes | FK → corresponding entity |
| author_id | UUID | yes | FK → users |
| body | text | yes | Markdown supported |
| parent_id | UUID | no | FK → pm_comments (for threaded replies) |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### @Mentions

- `@username` in comment body or descriptions triggers notification
- `@team-name` notifies all team members
- Mentioned user IDs are extracted and stored for notification delivery

#### Activity Log

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| entity_type | enum | yes | `story`, `epic`, `iteration`, `objective`, `doc` |
| entity_id | UUID | yes | FK → corresponding entity |
| actor_id | UUID | yes | FK → users (who made the change) |
| action | varchar(64) | yes | Action type (see below) |
| field_name | varchar(64) | no | Which field changed |
| old_value | text | no | Previous value |
| new_value | text | no | New value |
| metadata | jsonb | no | Additional context |
| created_at | timestamptz | auto | |

#### Action Types

- `created` — Entity created
- `updated` — Field value changed
- `state_changed` — Workflow state transition
- `comment_added` — New comment
- `owner_changed` — Owner assignment changed
- `epic_changed` — Story moved to different epic
- `iteration_changed` — Story added/removed from iteration
- `label_added` / `label_removed`
- `attachment_added` / `attachment_removed`
- `link_added` / `link_removed` — Story link created/removed
- `archived` / `unarchived`

---

### 5.12 File Attachments

#### Attachment Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| entity_type | enum | yes | `story`, `epic`, `comment`, `doc` |
| entity_id | UUID | yes | FK → corresponding entity |
| uploaded_by | UUID | yes | FK → users |
| filename | varchar(512) | yes | Original filename |
| file_size | bigint | yes | Size in bytes |
| content_type | varchar(128) | yes | MIME type |
| storage_path | text | yes | Path in object storage |
| thumbnail_path | text | no | Path to thumbnail (for images) |
| created_at | timestamptz | auto | |

#### Constraints

- Maximum file size: 50MB per upload
- Any file type supported
- Images render inline as previews
- Files stored in object storage (S3-compatible or local filesystem)

---

### 5.13 Templates

#### Story Template Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(256) | yes | Template name |
| description | text | no | Template description |
| story_type | enum | no | feature, bug, chore |
| template_description | text | no | Pre-filled story description (Markdown) |
| default_team_id | UUID | no | Default team assignment |
| default_epic_id | UUID | no | Default epic |
| default_workflow_state_id | UUID | no | Default state |
| default_priority | enum | no | Default priority |
| default_estimate | integer | no | Default estimate |
| default_labels | jsonb | no | Array of label IDs |
| default_custom_fields | jsonb | no | Map of field_id → option_id |
| checklist_template | jsonb | no | Pre-defined checklist items |
| created_by | UUID | yes | FK → users |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Usage

- Templates populate all story fields on creation
- Requester always defaults to the creating user
- Templates are workspace-scoped and available to all members

---

### 5.14 Docs

Rich-text documents integrated with the project management workflow.

#### Doc Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| title | varchar(512) | yes | Document title |
| content | text | no | Rich text content (stored as Markdown/HTML) |
| author_id | UUID | yes | FK → users |
| pinned | boolean | no | Pinned to navigation |
| archived | boolean | no | Default false |
| access_level | enum | no | `workspace` (default), `team`, `private` |
| team_id | UUID | no | FK → workspace_teams (for team-scoped docs) |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Doc Links (many-to-many to entities)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| doc_id | UUID | yes | FK → pm_docs |
| entity_type | enum | yes | `story`, `epic`, `iteration`, `objective` |
| entity_id | UUID | yes | FK → corresponding entity |
| created_at | timestamptz | auto | |

#### Features

- Full rich text editing (headings, bold, italic, lists, code blocks, tables)
- @mention support in doc content
- Link docs to Stories, Epics, Iterations, Objectives
- Linked work items show live status within the doc
- Pinned docs appear in sidebar navigation
- Comment threads on docs

#### Use Cases

- PRDs and Technical Design Docs
- Sprint Planning notes
- Retrospective notes
- Meeting notes
- Architecture Decision Records (ADRs)

---

## 6. Views & UI

### 6.1 Board View (Kanban)

The primary view for day-to-day work management.

#### Layout

- **Columns** = Workflow states (left to right: Backlog → Unstarted → Started → Done)
- **Story cards** within columns, ordered by position (top = highest priority)
- Column headers show: state name, story count, point total, WIP limit indicator

#### Story Card Display

Each card shows:
- Story type icon (feature/bug/chore)
- Story display ID (TP-123)
- Story title (truncated if long)
- Owner avatar(s)
- Estimate badge (story points)
- Label chips (colored)
- Priority indicator
- Due date (if set, with overdue highlighting)
- Blocked/Blocker icon
- Epic name (small text)

#### Interactions

- **Drag & drop** horizontally between columns → updates workflow state
- **Drag & drop** vertically within column → updates priority order
- **Click card** → opens story detail panel/modal
- **Quick create** → add story directly within a column
- **Multi-select** → bulk move, bulk edit

#### WIP Limits

- Configurable per workflow state
- Visual warning when column is at limit
- Visual error when column exceeds limit

---

### 6.2 List/Table View

Alternative view showing stories as rows in a sortable table.

#### Default Columns

- Checkbox (for bulk selection)
- Display ID
- Type icon
- Title
- State
- Priority
- Owner(s)
- Epic
- Iteration
- Estimate
- Labels
- Due Date
- Updated At

#### Features

- **Sortable** by any column (click header)
- **Group by**: Epic, Iteration, Owner, Team, Label, Workflow State, Priority, Requester
- **Inline editing**: Click a cell to edit (state, owner, priority, estimate, labels)
- **Bulk actions**: Select multiple rows → bulk edit state, owner, epic, iteration, labels
- **Column visibility**: Show/hide columns via settings
- **Custom field columns**: Custom fields appear as additional columns

---

### 6.3 Timeline/Roadmap View

Gantt-style horizontal timeline for high-level planning.

#### Layout

- **X-axis**: Calendar timeline (weeks/months/quarters)
- **Y-axis**: Epics as horizontal bars
- **Bar length**: planned_start_date → deadline
- **Color coding**: By team or by epic state

#### Features

- **Drag bars** to change dates
- **Resize bars** to adjust duration
- **Group by**: Team, Objective
- **Filter by**: Team, Objective, Epic State, Labels, Owner
- **Zoom levels**: Week, Month, Quarter
- **Objective markers**: Show Objective deadlines as vertical milestone lines
- **Epic health**: Color-coded health indicator on each bar
- **Today line**: Vertical line showing current date

---

### 6.4 Filtering & Search

#### Filter System

Available on Board, List, and Timeline views:

| Filter | Type | Description |
|--------|------|-------------|
| Team | multi-select | Filter by team assignment |
| Epic | multi-select | Filter by epic |
| Iteration | multi-select | Filter by iteration |
| Owner | multi-select | Filter by owner(s) |
| Requester | multi-select | Filter by requester |
| Label | multi-select | Filter by labels |
| Story Type | multi-select | feature, bug, chore |
| Workflow State | multi-select | Filter by specific states |
| Priority | multi-select | none, low, medium, high, urgent |
| Custom Fields | per-field select | Filter by any custom field value |
| Estimate | range | min/max story points |
| Due Date | date range | Filter by deadline |
| Created | date range | Filter by creation date |
| Archived | toggle | Show/hide archived |

#### Filter Logic

- **Matches All** (AND): Default group — all conditions must be true
- **Matches Any** (OR): Optional group — any condition can be true
- Filters can be toggled on/off without removal

#### Search

Global search bar with operators:

```
type:bug                    — Filter by story type
is:started                  — Filter by status
is:blocked                  — Show blocked stories
has:comment                 — Stories with comments
!has:epic                   — Stories without an epic
label:"frontend"            — Filter by label
epic:"Auth System"          — Filter by epic name
owner:@username             — Filter by owner
team:"Frontend"             — Filter by team
estimate:5                  — Filter by estimate
priority:high               — Filter by priority
iteration:"Sprint 12"       — Filter by iteration
created:2026-01-01..2026-03-31  — Date range
```

Search covers story titles, descriptions, and comments.

---

### 6.5 Saved Views (Spaces)

Any filtered/configured view can be saved as a **Space**.

#### Space Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| name | varchar(128) | yes | Space name |
| view_type | enum | yes | `board`, `list`, `timeline` |
| filters | jsonb | no | Serialized filter configuration |
| group_by | varchar(64) | no | Grouping field |
| sort_by | varchar(64) | no | Sort field |
| sort_order | enum | no | `asc`, `desc` |
| column_config | jsonb | no | Visible columns (for list view) |
| is_default | boolean | no | Default space for the workspace |
| team_id | UUID | no | Team-scoped space (null = workspace-wide) |
| created_by | UUID | yes | FK → users |
| is_shared | boolean | no | Visible to all workspace members |
| position | integer | yes | Tab order |
| created_at | timestamptz | auto | |
| updated_at | timestamptz | auto | |

#### Behaviors

- Spaces appear as tabs at the top of the Stories page
- Personal spaces are visible only to the creator
- Shared spaces are visible to all workspace members (or team members if team-scoped)
- Each space remembers its view type, filters, grouping, sorting, and column configuration

---

## 7. Reports & Analytics

### 7.1 Velocity Chart

- **Bar chart** showing stories/points completed per iteration
- **Two trend lines**:
  - Dotted line: Overall average velocity (all-time)
  - Solid line: Trailing 4-iteration average
- **Color breakdown** by story type: Feature (green), Bug (red), Chore (blue)
- **Toggle**: View by story count or story points
- **Scope**: Team-specific or workspace-wide

### 7.2 Burndown Chart

- Available for: Iterations, Epics, Objectives
- **X-axis**: Time (start → end date)
- **Y-axis**: Remaining work (stories or points)
- **Ideal line**: Linear burn from total → 0 by end date
- **Actual line**: Real remaining work over time
- **Projection line** (when no end date): Forward projection based on current velocity
- **Toggle**: View by story count or story points

### 7.3 Cumulative Flow Diagram

- Available for: Iterations, Epics
- **X-axis**: Time
- **Y-axis**: Number of stories
- **Stacked area chart**: One band per workflow state
- Useful for identifying bottlenecks (growing bands indicate WIP accumulation)

### 7.4 Cycle Time Chart

- **Scatter plot**: Each completed story as a dot
- **X-axis**: Completion date
- **Y-axis**: Cycle time (hours/days from Started → Done)
- **Trend lines**:
  - Dotted: Overall average
  - Solid: 7-day trailing average
- **Filters**: Story type, workflow state range
- **Scale**: Linear or logarithmic

### 7.5 Lead Time Chart

- Same format as Cycle Time but measures creation → completion
- Useful for understanding total request-to-delivery time

### 7.6 Reports Filtering

All reports can be scoped by:
- Date range
- Team
- Epic
- Iteration
- Owner
- Label
- Story type

---

## 8. Notifications

### 8.1 In-App Notifications

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | UUID | auto | Primary key |
| workspace_id | UUID | yes | Workspace scope |
| user_id | UUID | yes | Notification recipient (FK → users) |
| entity_type | enum | yes | `story`, `epic`, `iteration`, `objective`, `doc`, `comment` |
| entity_id | UUID | yes | FK → corresponding entity |
| notification_type | varchar(64) | yes | See types below |
| title | text | yes | Notification title |
| body | text | no | Notification body |
| actor_id | UUID | yes | Who triggered (FK → users) |
| read | boolean | no | Default false |
| read_at | timestamptz | no | When marked as read |
| created_at | timestamptz | auto | |

#### Notification Types

- `mentioned` — @mentioned in a comment or description
- `comment_added` — New comment on a followed entity
- `state_changed` — Workflow state changed on a followed story
- `assigned` — Story assigned to you
- `unassigned` — Story unassigned from you
- `deadline_approaching` — Due date within 24 hours
- `deadline_passed` — Due date has passed
- `blocked` — A story you own is now blocked
- `unblocked` — A story you own is now unblocked
- `iteration_starting` — An iteration you're in starts tomorrow
- `iteration_ending` — An iteration you're in ends tomorrow

### 8.2 Notification Delivery

- **In-app**: Bell icon with unread count badge, dropdown list of notifications
- **Email** (future): Configurable per notification type
- **Webhook** (future): For external integrations

### 8.3 Follow Rules

- Story requesters and owners are auto-followed
- Commenting on a story auto-follows it
- Users can manually follow/unfollow any entity
- Team mentions follow all team members

---

## 9. Teams & Permissions

### 9.1 Team Model

The PM module reuses the existing `workspace_teams` table. Each team can have:

- Its own **Workflow** (customizable states)
- Its own **Iterations** (sprint cadence)
- Team-specific **Spaces** (saved views)
- Team-scoped **Docs**

### 9.2 Permission Model

Leverages existing `workspace_members` roles:

| Role | PM Permissions |
|------|---------------|
| **owner** | Full access. Manage workflows, custom fields, workspace-level settings. |
| **admin** | Same as owner within workspace. Manage workflows, templates, custom fields. |
| **manager** | Create/edit all entities. Manage team workflows and iterations. View reports. |
| **member** | Create/edit stories, comments, docs. Can't modify workflows or custom fields. |
| **viewer** | Read-only. Can view boards, stories, reports. Cannot create or edit. |

### 9.3 Entity-Level Access

- Stories, Epics, Iterations: Visible to all workspace members
- Docs: Configurable (workspace, team, or private)
- Saved Views (Spaces): Personal or shared
- No per-story or per-epic permission restrictions (follows Shortcut's open model)

---

## 10. Integrations

### 10.1 GitHub Integration (Phase 2)

- Link stories to branches using naming convention: `tp-{display_id}-description`
- Commits with `[tp-123]` in message auto-link to story
- PR status shown on story card (open, merged, closed)
- Auto-state transitions: PR opened → "In Review", PR merged → "Done"

### 10.2 Slack Integration (Phase 3)

- @mention notifications delivered to Slack
- Story updates posted to team channels
- Reply in Slack → posts as comment on story

### 10.3 Webhook Support (Phase 2)

- Outgoing webhooks for story events (created, updated, state_changed, completed)
- Configurable per workspace
- Used for custom automations and external tool integration

---

## 11. Database Schema

### Complete Table Definitions

```sql
-- ================================================
-- PM MODULE MIGRATIONS
-- All tables prefixed with pm_ to isolate from existing system
-- ================================================

-- ------------------------------------------
-- Workflows & States
-- ------------------------------------------

CREATE TABLE pm_workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    default_state_id UUID, -- set after states created
    auto_assign_owner BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pm_workflow_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES pm_workflows(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    state_type VARCHAR(20) NOT NULL CHECK (state_type IN ('backlog', 'unstarted', 'started', 'done')),
    position INTEGER NOT NULL DEFAULT 0,
    color VARCHAR(7),
    description TEXT,
    wip_limit INTEGER,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE pm_workflows
    ADD CONSTRAINT fk_default_state
    FOREIGN KEY (default_state_id) REFERENCES pm_workflow_states(id);

CREATE TABLE pm_epic_workflow_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    state_type VARCHAR(20) NOT NULL CHECK (state_type IN ('unstarted', 'started', 'done')),
    position INTEGER NOT NULL DEFAULT 0,
    color VARCHAR(7),
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ------------------------------------------
-- Objectives & Key Results
-- ------------------------------------------

CREATE TABLE pm_objectives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL,
    description TEXT,
    objective_type VARCHAR(20) NOT NULL DEFAULT 'tactical' CHECK (objective_type IN ('tactical', 'strategic')),
    state VARCHAR(50) NOT NULL DEFAULT 'To Do',
    planned_start_date DATE,
    deadline DATE,
    started BOOLEAN DEFAULT false,
    started_at TIMESTAMPTZ,
    completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMPTZ,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    position INTEGER NOT NULL DEFAULT 0,
    archived BOOLEAN DEFAULT false,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pm_key_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    objective_id UUID NOT NULL REFERENCES pm_objectives(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL,
    result_type VARCHAR(20) NOT NULL CHECK (result_type IN ('boolean', 'percent', 'numeric')),
    initial_value NUMERIC,
    current_value NUMERIC,
    target_value NUMERIC,
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ------------------------------------------
-- Epics
-- ------------------------------------------

CREATE TABLE pm_epics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL,
    description TEXT,
    epic_state_id UUID REFERENCES pm_epic_workflow_states(id) ON DELETE SET NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    planned_start_date DATE,
    deadline DATE,
    started BOOLEAN DEFAULT false,
    started_at TIMESTAMPTZ,
    completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMPTZ,
    position INTEGER NOT NULL DEFAULT 0,
    color VARCHAR(7),
    health VARCHAR(20) CHECK (health IN ('on_track', 'at_risk', 'off_track')),
    health_comment TEXT,
    archived BOOLEAN DEFAULT false,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pm_epic_objectives (
    epic_id UUID NOT NULL REFERENCES pm_epics(id) ON DELETE CASCADE,
    objective_id UUID NOT NULL REFERENCES pm_objectives(id) ON DELETE CASCADE,
    PRIMARY KEY (epic_id, objective_id)
);

CREATE TABLE pm_epic_labels (
    epic_id UUID NOT NULL REFERENCES pm_epics(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (epic_id, label_id)
);

-- ------------------------------------------
-- Iterations
-- ------------------------------------------

CREATE TABLE pm_iterations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE
            WHEN CURRENT_DATE < start_date THEN 'unstarted'
            WHEN CURRENT_DATE > end_date THEN 'done'
            ELSE 'started'
        END
    ) STORED,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    archived BOOLEAN DEFAULT false,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT valid_dates CHECK (end_date > start_date)
);

CREATE TABLE pm_iteration_labels (
    iteration_id UUID NOT NULL REFERENCES pm_iterations(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (iteration_id, label_id)
);

-- ------------------------------------------
-- Labels
-- ------------------------------------------

CREATE TABLE pm_labels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    color VARCHAR(7),
    archived BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);

-- ------------------------------------------
-- Custom Fields
-- ------------------------------------------

CREATE TABLE pm_custom_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    description TEXT,
    field_type VARCHAR(20) NOT NULL DEFAULT 'single_select',
    icon VARCHAR(32),
    position INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pm_custom_field_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    custom_field_id UUID NOT NULL REFERENCES pm_custom_fields(id) ON DELETE CASCADE,
    value VARCHAR(128) NOT NULL,
    color VARCHAR(7),
    position INTEGER NOT NULL DEFAULT 0
);

-- ------------------------------------------
-- Stories
-- ------------------------------------------

CREATE SEQUENCE pm_story_display_id_seq;

CREATE TABLE pm_stories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    display_id INTEGER NOT NULL DEFAULT nextval('pm_story_display_id_seq'),
    name VARCHAR(512) NOT NULL,
    description TEXT,
    story_type VARCHAR(20) NOT NULL DEFAULT 'feature' CHECK (story_type IN ('feature', 'bug', 'chore')),
    workflow_id UUID NOT NULL REFERENCES pm_workflows(id),
    workflow_state_id UUID NOT NULL REFERENCES pm_workflow_states(id),
    epic_id UUID REFERENCES pm_epics(id) ON DELETE SET NULL,
    iteration_id UUID REFERENCES pm_iterations(id) ON DELETE SET NULL,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    requester_id UUID NOT NULL REFERENCES users(id),
    estimate INTEGER,
    priority VARCHAR(20) DEFAULT 'none' CHECK (priority IN ('none', 'low', 'medium', 'high', 'urgent')),
    severity VARCHAR(20) DEFAULT 'none' CHECK (severity IN ('none', 'low', 'medium', 'high', 'critical')),
    deadline DATE,
    position INTEGER NOT NULL DEFAULT 0,
    started BOOLEAN DEFAULT false,
    started_at TIMESTAMPTZ,
    completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMPTZ,
    moved_at TIMESTAMPTZ,
    blocked BOOLEAN DEFAULT false,
    blocker BOOLEAN DEFAULT false,
    archived BOOLEAN DEFAULT false,
    template_id UUID,
    external_id VARCHAR(128),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_pm_stories_display_id ON pm_stories(workspace_id, display_id);
CREATE INDEX idx_pm_stories_workspace ON pm_stories(workspace_id);
CREATE INDEX idx_pm_stories_workflow_state ON pm_stories(workflow_state_id);
CREATE INDEX idx_pm_stories_epic ON pm_stories(epic_id);
CREATE INDEX idx_pm_stories_iteration ON pm_stories(iteration_id);
CREATE INDEX idx_pm_stories_team ON pm_stories(team_id);
CREATE INDEX idx_pm_stories_owner ON pm_stories(owner_id);

CREATE TABLE pm_story_owners (
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (story_id, user_id)
);

CREATE TABLE pm_story_followers (
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (story_id, user_id)
);

CREATE TABLE pm_story_labels (
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (story_id, label_id)
);

CREATE TABLE pm_custom_field_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    custom_field_id UUID NOT NULL REFERENCES pm_custom_fields(id) ON DELETE CASCADE,
    option_id UUID REFERENCES pm_custom_field_options(id) ON DELETE SET NULL,
    text_value TEXT,
    number_value NUMERIC,
    UNIQUE(story_id, custom_field_id)
);

-- ------------------------------------------
-- Sub-tasks
-- ------------------------------------------

CREATE TABLE pm_sub_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    name VARCHAR(512) NOT NULL,
    description TEXT,
    workflow_state_id UUID NOT NULL REFERENCES pm_workflow_states(id),
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    estimate INTEGER,
    position INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ------------------------------------------
-- Checklists
-- ------------------------------------------

CREATE TABLE pm_checklists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL DEFAULT 'Checklist',
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pm_checklist_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    checklist_id UUID NOT NULL REFERENCES pm_checklists(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    completed BOOLEAN DEFAULT false,
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ------------------------------------------
-- Story Links
-- ------------------------------------------

CREATE TABLE pm_story_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    target_story_id UUID NOT NULL REFERENCES pm_stories(id) ON DELETE CASCADE,
    link_type VARCHAR(20) NOT NULL CHECK (link_type IN ('blocks', 'relates_to', 'duplicates')),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT no_self_link CHECK (source_story_id != target_story_id),
    UNIQUE(source_story_id, target_story_id, link_type)
);

-- ------------------------------------------
-- Comments
-- ------------------------------------------

CREATE TABLE pm_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(20) NOT NULL CHECK (entity_type IN ('story', 'epic', 'doc')),
    entity_id UUID NOT NULL,
    author_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    parent_id UUID REFERENCES pm_comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_pm_comments_entity ON pm_comments(entity_type, entity_id);

-- ------------------------------------------
-- Attachments
-- ------------------------------------------

CREATE TABLE pm_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(20) NOT NULL CHECK (entity_type IN ('story', 'epic', 'comment', 'doc')),
    entity_id UUID NOT NULL,
    uploaded_by UUID NOT NULL REFERENCES users(id),
    filename VARCHAR(512) NOT NULL,
    file_size BIGINT NOT NULL,
    content_type VARCHAR(128) NOT NULL,
    storage_path TEXT NOT NULL,
    thumbnail_path TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_pm_attachments_entity ON pm_attachments(entity_type, entity_id);

-- ------------------------------------------
-- Story Templates
-- ------------------------------------------

CREATE TABLE pm_story_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL,
    description TEXT,
    story_type VARCHAR(20),
    template_description TEXT,
    default_team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    default_epic_id UUID REFERENCES pm_epics(id) ON DELETE SET NULL,
    default_workflow_state_id UUID REFERENCES pm_workflow_states(id) ON DELETE SET NULL,
    default_priority VARCHAR(20),
    default_estimate INTEGER,
    default_labels JSONB DEFAULT '[]',
    default_custom_fields JSONB DEFAULT '{}',
    checklist_template JSONB DEFAULT '[]',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ------------------------------------------
-- Docs
-- ------------------------------------------

CREATE TABLE pm_docs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title VARCHAR(512) NOT NULL,
    content TEXT,
    author_id UUID NOT NULL REFERENCES users(id),
    pinned BOOLEAN DEFAULT false,
    archived BOOLEAN DEFAULT false,
    access_level VARCHAR(20) DEFAULT 'workspace' CHECK (access_level IN ('workspace', 'team', 'private')),
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pm_doc_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doc_id UUID NOT NULL REFERENCES pm_docs(id) ON DELETE CASCADE,
    entity_type VARCHAR(20) NOT NULL CHECK (entity_type IN ('story', 'epic', 'iteration', 'objective')),
    entity_id UUID NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(doc_id, entity_type, entity_id)
);

-- ------------------------------------------
-- Saved Views (Spaces)
-- ------------------------------------------

CREATE TABLE pm_saved_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    view_type VARCHAR(20) NOT NULL CHECK (view_type IN ('board', 'list', 'timeline')),
    filters JSONB DEFAULT '{}',
    group_by VARCHAR(64),
    sort_by VARCHAR(64),
    sort_order VARCHAR(4) DEFAULT 'asc' CHECK (sort_order IN ('asc', 'desc')),
    column_config JSONB,
    is_default BOOLEAN DEFAULT false,
    team_id UUID REFERENCES workspace_teams(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    is_shared BOOLEAN DEFAULT false,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ------------------------------------------
-- Activity Log
-- ------------------------------------------

CREATE TABLE pm_activity_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    entity_type VARCHAR(20) NOT NULL,
    entity_id UUID NOT NULL,
    actor_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(64) NOT NULL,
    field_name VARCHAR(64),
    old_value TEXT,
    new_value TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_pm_activity_entity ON pm_activity_log(entity_type, entity_id);
CREATE INDEX idx_pm_activity_workspace ON pm_activity_log(workspace_id, created_at DESC);

-- ------------------------------------------
-- Notifications
-- ------------------------------------------

CREATE TABLE pm_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_type VARCHAR(20) NOT NULL,
    entity_id UUID NOT NULL,
    notification_type VARCHAR(64) NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    actor_id UUID NOT NULL REFERENCES users(id),
    read BOOLEAN DEFAULT false,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_pm_notifications_user ON pm_notifications(user_id, read, created_at DESC);

-- ------------------------------------------
-- Updated-at triggers for all tables
-- ------------------------------------------

-- Apply set_updated_at() trigger (already exists from migration 001)
-- to all pm_ tables with updated_at columns.
```

---

## 12. API Design

All PM endpoints live under `/api/pm/` prefix with JWT authentication and `X-Workspace-ID` header.

### 12.1 Workflows

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/workflows` | List workflows for workspace |
| POST | `/api/pm/workflows` | Create workflow |
| GET | `/api/pm/workflows/{id}` | Get workflow with states |
| PUT | `/api/pm/workflows/{id}` | Update workflow |
| DELETE | `/api/pm/workflows/{id}` | Delete workflow |
| POST | `/api/pm/workflows/{id}/states` | Add workflow state |
| PUT | `/api/pm/workflows/{id}/states/{stateId}` | Update workflow state |
| DELETE | `/api/pm/workflows/{id}/states/{stateId}` | Delete workflow state |
| PUT | `/api/pm/workflows/{id}/states/reorder` | Reorder states |

### 12.2 Objectives

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/objectives` | List objectives (filterable) |
| POST | `/api/pm/objectives` | Create objective |
| GET | `/api/pm/objectives/{id}` | Get objective with key results & epics |
| PUT | `/api/pm/objectives/{id}` | Update objective |
| DELETE | `/api/pm/objectives/{id}` | Delete objective |
| POST | `/api/pm/objectives/{id}/key-results` | Add key result |
| PUT | `/api/pm/objectives/{id}/key-results/{krId}` | Update key result |
| DELETE | `/api/pm/objectives/{id}/key-results/{krId}` | Delete key result |

### 12.3 Epics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/epics` | List epics (filterable) |
| POST | `/api/pm/epics` | Create epic |
| GET | `/api/pm/epics/{id}` | Get epic with stats |
| PUT | `/api/pm/epics/{id}` | Update epic |
| DELETE | `/api/pm/epics/{id}` | Delete/archive epic |
| GET | `/api/pm/epics/{id}/stories` | List stories in epic |
| PUT | `/api/pm/epics/{id}/health` | Update epic health status |
| GET | `/api/pm/epics/{id}/burndown` | Epic burndown data |
| GET | `/api/pm/epics/{id}/cumulative-flow` | Epic CFD data |

### 12.4 Stories

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/stories` | List stories (filterable, paginated) |
| POST | `/api/pm/stories` | Create story |
| GET | `/api/pm/stories/{id}` | Get full story detail |
| PUT | `/api/pm/stories/{id}` | Update story |
| DELETE | `/api/pm/stories/{id}` | Delete/archive story |
| PUT | `/api/pm/stories/{id}/move` | Move to different state (with position) |
| PUT | `/api/pm/stories/{id}/reorder` | Change position within state |
| POST | `/api/pm/stories/{id}/owners` | Add owner |
| DELETE | `/api/pm/stories/{id}/owners/{userId}` | Remove owner |
| POST | `/api/pm/stories/{id}/followers` | Follow story |
| DELETE | `/api/pm/stories/{id}/followers` | Unfollow story |
| POST | `/api/pm/stories/{id}/labels` | Add label |
| DELETE | `/api/pm/stories/{id}/labels/{labelId}` | Remove label |
| POST | `/api/pm/stories/{id}/links` | Create story link |
| DELETE | `/api/pm/stories/{id}/links/{linkId}` | Remove story link |
| GET | `/api/pm/stories/{id}/activity` | Get story activity log |
| POST | `/api/pm/stories/bulk` | Bulk update stories |
| GET | `/api/pm/stories/search` | Search stories with operators |

### 12.5 Sub-tasks

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/stories/{storyId}/sub-tasks` | List sub-tasks |
| POST | `/api/pm/stories/{storyId}/sub-tasks` | Create sub-task |
| PUT | `/api/pm/sub-tasks/{id}` | Update sub-task |
| DELETE | `/api/pm/sub-tasks/{id}` | Delete sub-task |
| PUT | `/api/pm/sub-tasks/{id}/move` | Change sub-task state |
| PUT | `/api/pm/sub-tasks/reorder` | Reorder sub-tasks |

### 12.6 Checklists

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/pm/stories/{storyId}/checklists` | Create checklist |
| PUT | `/api/pm/checklists/{id}` | Update checklist |
| DELETE | `/api/pm/checklists/{id}` | Delete checklist |
| POST | `/api/pm/checklists/{id}/items` | Add item |
| PUT | `/api/pm/checklist-items/{id}` | Update item (toggle, text, assignee) |
| DELETE | `/api/pm/checklist-items/{id}` | Delete item |
| POST | `/api/pm/checklist-items/{id}/promote` | Promote to sub-task |
| PUT | `/api/pm/checklists/{id}/items/reorder` | Reorder items |

### 12.7 Iterations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/iterations` | List iterations (filterable) |
| POST | `/api/pm/iterations` | Create iteration |
| GET | `/api/pm/iterations/{id}` | Get iteration with stats |
| PUT | `/api/pm/iterations/{id}` | Update iteration |
| DELETE | `/api/pm/iterations/{id}` | Delete iteration |
| GET | `/api/pm/iterations/{id}/stories` | List stories in iteration |
| GET | `/api/pm/iterations/{id}/burndown` | Iteration burndown data |
| GET | `/api/pm/iterations/{id}/cumulative-flow` | Iteration CFD data |

### 12.8 Labels

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/labels` | List labels |
| POST | `/api/pm/labels` | Create label |
| PUT | `/api/pm/labels/{id}` | Update label |
| DELETE | `/api/pm/labels/{id}` | Delete label |

### 12.9 Custom Fields

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/custom-fields` | List custom fields |
| POST | `/api/pm/custom-fields` | Create custom field |
| PUT | `/api/pm/custom-fields/{id}` | Update custom field |
| DELETE | `/api/pm/custom-fields/{id}` | Delete custom field |
| POST | `/api/pm/custom-fields/{id}/options` | Add option |
| PUT | `/api/pm/custom-field-options/{id}` | Update option |
| DELETE | `/api/pm/custom-field-options/{id}` | Delete option |

### 12.10 Comments

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/comments?entity_type=story&entity_id={id}` | List comments |
| POST | `/api/pm/comments` | Create comment |
| PUT | `/api/pm/comments/{id}` | Edit comment |
| DELETE | `/api/pm/comments/{id}` | Delete comment |

### 12.11 Attachments

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/pm/attachments` | Upload file (multipart) |
| GET | `/api/pm/attachments/{id}` | Download file |
| DELETE | `/api/pm/attachments/{id}` | Delete file |

### 12.12 Templates

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/templates` | List story templates |
| POST | `/api/pm/templates` | Create template |
| GET | `/api/pm/templates/{id}` | Get template |
| PUT | `/api/pm/templates/{id}` | Update template |
| DELETE | `/api/pm/templates/{id}` | Delete template |

### 12.13 Docs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/docs` | List docs (filterable) |
| POST | `/api/pm/docs` | Create doc |
| GET | `/api/pm/docs/{id}` | Get doc |
| PUT | `/api/pm/docs/{id}` | Update doc |
| DELETE | `/api/pm/docs/{id}` | Delete doc |
| POST | `/api/pm/docs/{id}/links` | Link doc to entity |
| DELETE | `/api/pm/docs/{id}/links/{linkId}` | Unlink |

### 12.14 Saved Views (Spaces)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/views` | List saved views |
| POST | `/api/pm/views` | Create saved view |
| PUT | `/api/pm/views/{id}` | Update saved view |
| DELETE | `/api/pm/views/{id}` | Delete saved view |

### 12.15 Notifications

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/notifications` | List notifications (paginated) |
| GET | `/api/pm/notifications/unread-count` | Get unread count |
| PUT | `/api/pm/notifications/{id}/read` | Mark as read |
| PUT | `/api/pm/notifications/read-all` | Mark all as read |

### 12.16 Reports

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pm/reports/velocity` | Velocity chart data |
| GET | `/api/pm/reports/burndown` | Burndown data (scope: iteration/epic/objective) |
| GET | `/api/pm/reports/cumulative-flow` | CFD data |
| GET | `/api/pm/reports/cycle-time` | Cycle time scatter data |
| GET | `/api/pm/reports/lead-time` | Lead time scatter data |

All report endpoints accept query params: `team_id`, `epic_id`, `iteration_id`, `label_id`, `story_type`, `date_from`, `date_to`.

---

## 13. Frontend Structure

### 13.1 New Routes

```
/w/{slug}/pm/                       — PM Dashboard (redirects to stories)
/w/{slug}/pm/stories                — Stories page (board/list/timeline views)
/w/{slug}/pm/stories/{displayId}    — Story detail (modal/panel over stories page)
/w/{slug}/pm/epics                  — Epics list page
/w/{slug}/pm/epics/{id}             — Epic detail page (stories, burndown, CFD)
/w/{slug}/pm/iterations             — Iterations list page
/w/{slug}/pm/iterations/{id}        — Iteration detail page (stories, burndown, velocity)
/w/{slug}/pm/objectives             — Objectives page
/w/{slug}/pm/objectives/{id}        — Objective detail page
/w/{slug}/pm/roadmap                — Timeline/Roadmap view
/w/{slug}/pm/reports                — Reports dashboard
/w/{slug}/pm/docs                   — Docs list page
/w/{slug}/pm/docs/{id}              — Doc editor page
/w/{slug}/pm/settings               — PM-specific settings (workflows, custom fields, templates)
```

### 13.2 Navigation

Add a **"Project Management"** section to the workspace sidebar:

```
Workspace: {name}
├── Dashboard         (existing)
├── Goals             (existing)
├── Sprints           (existing)
├── Bonus             (existing)
├── ─────────────────
├── Stories           (NEW — board/list views)
├── Epics             (NEW)
├── Iterations        (NEW)
├── Objectives        (NEW)
├── Roadmap           (NEW)
├── Reports           (NEW)
├── Docs              (NEW)
├── ─────────────────
├── Settings          (existing, with new PM tab)
```

### 13.3 Key Components

| Component | Description |
|-----------|-------------|
| `KanbanBoard` | Drag-and-drop board with columns per workflow state |
| `StoryCard` | Card component showing story summary on board |
| `StoryDetailPanel` | Slide-over panel with full story editing |
| `StoryTable` | List/table view with sortable columns and inline editing |
| `TimelineView` | Gantt-style roadmap with draggable epic bars |
| `EpicDetailPage` | Epic overview with stories list, burndown, CFD |
| `IterationDetailPage` | Sprint overview with stories, burndown, velocity |
| `ObjectivesPage` | OKR view with key results and progress bars |
| `FilterBar` | Reusable filter component with AND/OR logic |
| `SpacesTabs` | Tab bar for switching between saved views |
| `RichTextEditor` | Markdown editor for descriptions, docs, comments |
| `CommentThread` | Threaded comment display and input |
| `VelocityChart` | Bar chart with trend lines |
| `BurndownChart` | Line chart with ideal vs actual |
| `CumulativeFlowDiagram` | Stacked area chart |
| `CycleTimeChart` | Scatter plot with trend lines |
| `NotificationDropdown` | Bell icon with notification list |
| `SearchBar` | Global search with operator autocomplete |

### 13.4 State Management

New Zustand stores:

| Store | Purpose |
|-------|---------|
| `pmWorkflowStore` | Current workflow and states |
| `pmStoryStore` | Story list, filters, pagination |
| `pmBoardStore` | Board state (column order, positions, drag state) |
| `pmEpicStore` | Epic list and current epic |
| `pmIterationStore` | Iteration list and current iteration |
| `pmObjectiveStore` | Objective list |
| `pmViewStore` | Current view configuration (filters, sort, group) |
| `pmNotificationStore` | Notification count and list |

### 13.5 API Service Files

New service files in `frontend/src/lib/services/`:

| File | Purpose |
|------|---------|
| `pmWorkflowService.ts` | Workflow and state CRUD |
| `pmStoryService.ts` | Story CRUD, move, bulk ops, search |
| `pmEpicService.ts` | Epic CRUD, stats, burndown |
| `pmIterationService.ts` | Iteration CRUD, stats |
| `pmObjectiveService.ts` | Objective and key result CRUD |
| `pmLabelService.ts` | Label CRUD |
| `pmCommentService.ts` | Comment CRUD |
| `pmAttachmentService.ts` | File upload/download |
| `pmTemplateService.ts` | Template CRUD |
| `pmDocService.ts` | Doc CRUD |
| `pmViewService.ts` | Saved view CRUD |
| `pmNotificationService.ts` | Notification operations |
| `pmReportService.ts` | Report data fetching |
| `pmCustomFieldService.ts` | Custom field management |

---

## 14. Relationship with Existing Bonus System

### Phase 1: Complete Separation (This PRD)

- PM module operates independently
- No foreign keys between PM tables and existing bonus tables
- Same users and workspaces, different data

### Phase 2: Light Integration (Future)

Potential connections:

| PM Metric | Bonus Impact |
|-----------|-------------|
| Stories completed per sprint | Feed into individual performance scoring |
| Sprint velocity (team) | Feed into team quality index (TQI) |
| Bug count / fix rate | Factor into evaluation criteria |
| Objective completion | Align with company goals for bonus weighting |

Implementation approach:
- **Mapping table**: `pm_bonus_mapping` linking PM iterations to bonus sprints
- **Computed views**: SQL views that aggregate PM data for scoring formulas
- **API bridge**: Endpoints that expose PM metrics in bonus-system-compatible format

### Phase 3: Full Unification (Future)

If PM sprints prove to fully replace bonus sprints:
- Migrate sprint data
- Drop old sprint tables
- Single sprint/iteration concept
- Auto-scoring directly from PM activity

---

## 15. Implementation Phases

### Phase 1: Core Work Tracking (Weeks 1-4)

**Goal**: Replace Shortcut for daily work management. Cancel subscription.

| Feature | Priority | Effort |
|---------|----------|--------|
| Workflows & States | P0 | M |
| Stories CRUD | P0 | L |
| Board View (Kanban) | P0 | L |
| Story Detail Panel | P0 | L |
| Epics CRUD | P0 | M |
| Iterations CRUD | P0 | M |
| Labels | P0 | S |
| Story type (feature/bug/chore) | P0 | S |
| Priority & Severity | P0 | S |
| Estimates (story points) | P0 | S |
| Comments | P0 | M |
| Activity Log | P1 | M |

**Deliverable**: Teams can create stories, organize on board, assign to epics and iterations.

### Phase 2: Enhanced Features (Weeks 5-8)

**Goal**: Feature parity with team's Shortcut usage.

| Feature | Priority | Effort |
|---------|----------|--------|
| List/Table View | P0 | L |
| Filtering System | P0 | L |
| Saved Views (Spaces) | P1 | M |
| Sub-tasks | P1 | M |
| Checklists | P1 | S |
| Story Links & Dependencies | P1 | M |
| Story Templates | P1 | M |
| Custom Fields | P1 | L |
| Notifications (in-app) | P1 | L |
| Search with operators | P1 | L |
| Bulk operations | P1 | M |

**Deliverable**: Full daily workflow coverage matching Shortcut.

### Phase 3: Planning & Reporting (Weeks 9-12)

**Goal**: Strategic planning and data-driven insights.

| Feature | Priority | Effort |
|---------|----------|--------|
| Objectives (OKRs) | P1 | L |
| Key Results | P1 | M |
| Timeline/Roadmap View | P1 | XL |
| Velocity Chart | P1 | M |
| Burndown Chart | P1 | M |
| Cumulative Flow Diagram | P2 | M |
| Cycle Time & Lead Time Charts | P2 | M |
| Epic Health | P2 | S |
| Epic Burndown/CFD | P2 | M |

**Deliverable**: Full planning and reporting capabilities.

### Phase 4: Collaboration & Polish (Weeks 13-16)

**Goal**: Team collaboration tools and UX polish.

| Feature | Priority | Effort |
|---------|----------|--------|
| Docs | P2 | XL |
| Rich Text Editor | P2 | L |
| File Attachments | P2 | M |
| WIP Limits | P2 | S |
| Dependency Graph Visualization | P2 | L |
| @Mention notifications | P2 | M |
| Keyboard shortcuts | P3 | M |
| GitHub Integration | P3 | XL |

**Deliverable**: Complete collaboration platform.

### Effort Legend

- **S** (Small): < 1 day
- **M** (Medium): 1-3 days
- **L** (Large): 3-5 days
- **XL** (Extra Large): 1-2 weeks

---

## 16. Non-Goals & Future Considerations

### Non-Goals (not in scope for V1)

- **Jira import/sync** — No migration from Shortcut needed (manual transition)
- **Mobile app** — Web-only for internal use
- **Marketplace integrations** — Beyond GitHub and Slack
- **Time tracking** — Not required for internal tool
- **SAML/SSO** — Existing JWT auth is sufficient
- **Real-time collaboration** — Polling-based updates are sufficient initially
- **AI features** — Auto-story generation, AI summaries, etc.
- **Multiple workspaces** — Team operates in a single workspace

### Future Considerations

- **WebSocket updates** for real-time board collaboration
- **Import from Shortcut** if historical data migration is needed
- **Automation rules** (when state changes → trigger actions)
- **API tokens** for external tool integration
- **Figma embed previews** in story descriptions
- **Sprint retrospective** templates and workflows
- **Capacity planning** based on team size and velocity
- **Custom estimate scales** (Fibonacci, linear, powers of 2)

---

## Appendix A: Glossary

| Term | Definition |
|------|-----------|
| **Story** | Atomic unit of work (feature, bug, or chore) |
| **Epic** | Large initiative containing multiple stories |
| **Iteration** | Time-boxed sprint period |
| **Objective** | Strategic or tactical goal (OKR) |
| **Key Result** | Measurable outcome under a Strategic Objective |
| **Workflow** | State machine defining how stories progress |
| **Workflow State** | A stage in a workflow (e.g., "In Review") |
| **State Type** | Category of a state: backlog, unstarted, started, done |
| **Space** | Saved view configuration (filters, grouping, view type) |
| **Label** | Flexible tag for categorizing entities |
| **Custom Field** | User-defined structured attribute on stories |
| **WIP Limit** | Maximum stories allowed in a workflow state |
| **Cycle Time** | Duration from story start to completion |
| **Lead Time** | Duration from story creation to completion |
| **Velocity** | Story points completed per iteration |
| **CFD** | Cumulative Flow Diagram — stacked area chart of story distribution |
| **Display ID** | Human-readable story identifier (TP-1, TP-2, ...) |

---

## Appendix B: Entity Relationship Summary

```
users ──────────┬──── pm_stories (owner, requester)
                ├──── pm_epics (owner, created_by)
                ├──── pm_objectives (owner, created_by)
                ├──── pm_comments (author)
                ├──── pm_activity_log (actor)
                └──── pm_notifications (user, actor)

workspaces ─────┬──── pm_workflows
                ├──── pm_objectives
                ├──── pm_epics
                ├──── pm_iterations
                ├──── pm_stories
                ├──── pm_labels
                ├──── pm_custom_fields
                ├──── pm_story_templates
                ├──── pm_docs
                ├──── pm_saved_views
                └──── pm_notifications

workspace_teams ┬──── pm_workflows (team_id)
                ├──── pm_epics (team_id)
                ├──── pm_iterations (team_id)
                └──── pm_stories (team_id)

pm_workflows ───┬──── pm_workflow_states
                └──── pm_stories

pm_objectives ──┬──── pm_key_results
                └──── pm_epic_objectives ──── pm_epics

pm_epics ───────┬──── pm_stories
                └──── pm_epic_labels ──── pm_labels

pm_iterations ──┬──── pm_stories
                └──── pm_iteration_labels ──── pm_labels

pm_stories ─────┬──── pm_sub_tasks
                ├──── pm_checklists ──── pm_checklist_items
                ├──── pm_story_owners
                ├──── pm_story_followers
                ├──── pm_story_labels ──── pm_labels
                ├──── pm_custom_field_values ──── pm_custom_field_options
                ├──── pm_story_links
                ├──── pm_comments
                ├──── pm_attachments
                └──── pm_activity_log
```
