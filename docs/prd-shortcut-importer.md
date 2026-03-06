# PRD: Shortcut CSV Importer

**Author:** Engineering
**Date:** 2026-03-06
**Status:** Draft

---

## 1. Problem Statement

Teams migrating from Shortcut (formerly Clubhouse) to Helpin have no way to bring their existing project data with them. This means they would lose all historical stories, epics, objectives, labels, iterations, and team structure — forcing a painful manual recreation or abandoning institutional knowledge altogether.

Shortcut provides a CSV export of all workspace stories. We need a robust importer that reads this CSV and maps the data into Helpin's existing PM data model, preserving as much context and structure as possible.

---

## 2. Goals

1. **Guided migration** — A workspace admin uploads a Shortcut CSV, reviews mappings, and the importer creates all entities in the correct hierarchy.
2. **Preserve structure** — Objectives, epics, sprints (iterations), labels, workflows/states, teams, and stories are all recreated with their relationships intact.
3. **Preserve history** — Timestamps (created, started, completed), completion status, and archived flags are carried over.
4. **User mapping** — Provide a UI step to map Shortcut email addresses to existing Helpin workspace members.
5. **Safe & idempotent** — The import is wrapped in a transaction. If it fails, nothing is half-created. Re-importing the same CSV does not create duplicates (keyed on Shortcut external IDs stored on imported entities).

---

## 3. Non-Goals

- Real-time sync with Shortcut API (this is a one-time CSV import).
- Importing attachments or embedded images from Shortcut CDN URLs in descriptions.
- Importing comments or activity history (not present in CSV export).
- Importing custom fields beyond what maps to existing Helpin fields.
- Supporting non-CSV Shortcut export formats (JSON API, etc.) in this iteration.

---

## 4. Source Data Analysis

The Shortcut CSV export analyzed (`workspace-60ae676e-...csv`) contains:

| Metric | Value |
|---|---|
| Total rows (stories) | 1,626 |
| Columns | 54 |
| Story types | `feature` (1020), `bug` (415), `chore` (191) |
| Unique epics | 107 |
| Unique objectives | 17 |
| Unique iterations (sprints) | 74 |
| Unique labels | 10 |
| Unique workflows | 2 (`Product Development`, `Docs`) |
| Unique workflow states | 10 |
| Unique teams | 3 (`Dev Team`, `Customer Support`, `Founders`) |
| Stories with epics | 1,111 (68%) |
| Stories with objectives | 1,044 (64%) |
| Stories with iterations | 886 (55%) |
| Stories with descriptions | 1,162 (71%) |
| Stories with tasks (checklists) | 143 (9%) |
| Stories with multiple owners | 240 (15%) |
| Stories with labels | 331 (20%) |
| Completed stories | 1,482 (91%) |
| Archived stories | 68 (4%) |

### 4.1 CSV Columns

| # | Column | Maps to | Notes |
|---|--------|---------|-------|
| 1 | `id` | `pm_stories.external_id` | Shortcut numeric story ID |
| 2 | `name` | `pm_stories.name` | |
| 3 | `type` | `pm_stories.story_type` | Values: `feature`, `bug`, `chore` — direct match |
| 4 | `requester` | `pm_stories.requester_id` | Email — needs user mapping |
| 5 | `owners` | `pm_story_owners` | Semicolon-separated emails — needs user mapping |
| 6 | `description` | `pm_stories.description` | Markdown with embedded Shortcut CDN image URLs |
| 7 | `is_completed` | `pm_stories.completed` | `true`/`false` |
| 8 | `created_at` | `pm_stories.created_at` | Format: `YYYY/MM/DD HH:MM:SS` |
| 9 | `started_at` | `pm_stories.started_at` | |
| 10 | `updated_at` | `pm_stories.updated_at` | |
| 11 | `moved_at` | `pm_stories.moved_at` | |
| 12 | `completed_at` | `pm_stories.completed_at` | |
| 13 | `estimate` | `pm_stories.estimate` | Integer points (sparse — only 63 stories) |
| 14 | `is_blocked` | `pm_stories.blocked` | |
| 15 | `due_date` | `pm_stories.deadline` | Very sparse (2 stories) |
| 16 | `labels` | `pm_story_labels` | Semicolon-separated label names |
| 17 | `tasks` | `pm_checklist_items` | Semicolon-separated; format: `[X] text` or `[ ] text` |
| 18 | `state` | `pm_stories.workflow_state_id` | State name — needs workflow state mapping |
| 19 | `epic_id` | `pm_epics.external_id` | Shortcut epic ID — used to group/deduplicate epics |
| 20 | `epic` | `pm_epics.name` | |
| 21 | `project_id` / `project` | — | Ignored (Shortcut-specific grouping) |
| 22 | `iteration_id` / `iteration` | `pm_sprints.external_id`, `pm_sprints.name`, `pm_sprints.start_date`, `pm_sprints.end_date` | Sprint dates are parsed from iteration name when possible; otherwise set to null |
| 23 | `team_id` / `team` | `pm_stories.team_id` | Team name — needs team mapping; null if missing or intentionally skipped |
| 24 | `is_archived` | `pm_stories.archived` | |
| 25 | `epic_state` | `pm_epics.epic_state_id` | Values: `in progress`, `done` |
| 26 | `epic_is_archived` | `pm_epics.archived` | |
| 27 | `epic_created_at` | `pm_epics.created_at` | |
| 28 | `epic_started_at` | `pm_epics.started_at` | |
| 29 | `epic_due_date` | `pm_epics.deadline` | |
| 30 | `epic_planned_start_date` | `pm_epics.planned_start_date` | |
| 31 | `epic_labels` | `pm_epic_labels` | Semicolon-separated |
| 32 | `objective_id` / `objective` | `pm_objectives.external_id`, `pm_objectives.name` | |
| 33 | `objective_state` | `pm_objectives.state` | `to do` → `not_started`, `in progress` → `active`, `done` → `closed` |
| 34 | `objective_created_at` | `pm_objectives.created_at` | |
| 35 | `objective_started_at` | `pm_objectives.planned_start_date` | |
| 36 | `objective_due_date` | `pm_objectives.deadline` | |
| 37 | `workflow` / `workflow_id` | `pm_workflows.name` | |
| 38 | `priority` | `pm_stories.priority` | See mapping table below |
| 39 | `severity` | `pm_stories.severity` | See mapping table below |
| 40 | `parent_story_id` | — | Not used in dataset (no sub-stories) |
| 41 | `custom_fields` | — | Sparse; key=value format (e.g., `Plan=premium`). Not mapped. |

### 4.2 Enum Mappings

**Priority** (Shortcut → Helpin):

| Shortcut | Helpin |
|---|---|
| `Highest` | `urgent` |
| `High` | `high` |
| `Medium` | `medium` |
| `Low` | `low` |
| `Lowest` | `low` |
| (empty) | `none` |

**Severity** (Shortcut → Helpin):

| Shortcut | Helpin |
|---|---|
| `Severity 0` | `critical` |
| `Severity 1` | `major` |
| `Severity 2` | `minor` |
| (empty) | `none` |

**Story State Type** (Shortcut state → Helpin state type):

| Shortcut State | Helpin State Type |
|---|---|
| `Backlog` | `backlog` |
| `Refinement` | `unstarted` |
| `Up next` | `unstarted` |
| `Ready for Development` | `unstarted` |
| `In Development` | `started` |
| `Ready for Review` | `started` |
| `Ready for Deploy` | `started` |
| `Completed` | `done` |
| `Done` | `done` |
| `Abandoned` | `done` |

**Objective State** (Shortcut → Helpin):

| Shortcut | Helpin |
|---|---|
| `to do` | `not_started` |
| `in progress` | `active` |
| `done` | `closed` |

---

## 5. Import Strategy

### 5.1 Entity Creation Order

The import must create entities in dependency order:

```
1. Teams (from unique `team` values)
2. Workflows + Workflow States (from unique `workflow` + `state` values)
3. Labels (from unique `labels` + `epic_labels` values)
4. Objectives (from unique `objective_id` + objective metadata)
5. Epics (from unique `epic_id` + epic metadata)
6. Epic ↔ Objective links (from `objective_id` on epic rows)
7. Sprints (from unique `iteration_id` + iteration name)
8. Stories (one per CSV row)
9. Story ↔ Owner links (from `owners` column)
10. Story ↔ Label links (from `labels` column)
11. Checklist items (from `tasks` column)
```

### 5.2 Deduplication

- **Epics**: Keyed by Shortcut `epic_id` stored as `pm_epics.external_id`. Multiple CSV rows reference the same epic; only create once.
- **Objectives**: Keyed by Shortcut `objective_id` stored as `pm_objectives.external_id`.
- **Iterations/Sprints**: Keyed by Shortcut `iteration_id` stored as `pm_sprints.external_id`.
- **Labels**: Keyed by name (case-insensitive) within the workspace.
- **Teams**: Keyed by name within the workspace.
- **Workflows**: Keyed by name within the workspace.
- **Stories**: Keyed by Shortcut `id` stored in `external_id`. If a story with the same `external_id` already exists in the workspace, skip it.

### 5.3 User Mapping

Shortcut exports reference users by email. The importer needs a mapping step:

1. Parse the CSV and extract all unique emails from `requester` and `owners` columns.
2. Auto-match emails to existing Helpin workspace members by exact email match.
3. For unmatched emails, the user can:
   - **Manually map** to an existing workspace member via dropdown.
   - **Invite & Map** — sends a workspace invitation email. The system creates the user record immediately (with `pending` status) so that imported stories can reference them by ID. When the invited user accepts and signs up, their account links to the pre-created record and they see their assigned stories.
   - **Skip** — stories from this user import without `requester_id` / owner links.
4. "Invite All Unmatched" performs the invite-and-map action for every unmatched email in one click.
5. The invite flow reuses the existing workspace invitation system (`POST /api/workspaces/{wid}/invitations`).

### 5.4 Transaction & Error Handling

- The entire import runs inside a single database transaction.
- If any critical error occurs (malformed CSV structure, DB constraint violation), the transaction rolls back and the user sees a clear error message.
- Non-critical issues (unmapped user, missing optional field) are collected as warnings and shown in a summary report after import.
- The importer uses a dedicated import path and does not call the normal PM create/update services. This avoids activity logging, websocket broadcasts, automations, and timestamp rewrites during import.

### 5.5 Sprint Import Policy

- Every unique Shortcut iteration is imported as a Helpin sprint.
- `pm_sprints.external_id` stores the Shortcut `iteration_id`.
- `pm_sprints.name` preserves the Shortcut iteration name exactly.
- The importer attempts to parse sprint dates from the Shortcut iteration name.
- If sprint dates cannot be determined confidently, `start_date` and `end_date` are set to null.
- `team_id` is null when the iteration spans multiple teams or the source row has no team.
- Imported sprints with null dates are excluded from date-based automation until manually edited.

---

## 6. API Design

### 6.1 Endpoints

```
POST   /api/workspaces/{workspace_id}/import/shortcut/preview
POST   /api/workspaces/{workspace_id}/import/shortcut/execute
GET    /api/workspaces/{workspace_id}/import/shortcut/status/{import_id}
```

### 6.2 Preview Endpoint

**`POST /api/workspaces/{wid}/import/shortcut/preview`**

Accepts the CSV file as `multipart/form-data`. Parses the file and returns a summary without writing anything to the database.

**Request:**
```
Content-Type: multipart/form-data
file: <csv file>
```

**Response:**
```json
{
  "summary": {
    "total_stories": 1626,
    "stories_by_type": { "feature": 1020, "bug": 415, "chore": 191 },
    "epics_count": 107,
    "objectives_count": 17,
    "sprints_count": 74,
    "labels_count": 10,
    "teams_count": 3,
    "workflows_count": 2,
    "workflow_states_count": 10,
    "checklist_items_count": 312,
    "duplicate_stories": 0
  },
  "users": [
    { "email": "azhar@contentstudio.io", "matched_user_id": "uuid-or-null", "matched_name": "Azhar K" },
    { "email": "sheharyar.khalid@d4interactive.io", "matched_user_id": null, "matched_name": null }
  ],
  "teams": [
    { "name": "Dev Team", "story_count": 1500 },
    { "name": "Customer Support", "story_count": 80 },
    { "name": "Founders", "story_count": 46 }
  ],
  "workflows": [
    {
      "name": "Product Development",
      "story_count": 1580,
      "states": [
        { "name": "Backlog", "suggested_type": "backlog", "story_count": 108 },
        { "name": "Refinement", "suggested_type": "unstarted", "story_count": 18 },
        { "name": "Up next", "suggested_type": "unstarted", "story_count": 8 },
        { "name": "Ready for Development", "suggested_type": "unstarted", "story_count": 1 },
        { "name": "In Development", "suggested_type": "started", "story_count": 2 },
        { "name": "Ready for Review", "suggested_type": "started", "story_count": 6 },
        { "name": "Ready for Deploy", "suggested_type": "started", "story_count": 1 },
        { "name": "Completed", "suggested_type": "done", "story_count": 1429 },
        { "name": "Abandoned", "suggested_type": "done", "story_count": 7 }
      ]
    },
    {
      "name": "Docs",
      "story_count": 46,
      "states": [
        { "name": "Backlog", "suggested_type": "backlog", "story_count": 10 },
        { "name": "Done", "suggested_type": "done", "story_count": 36 }
      ]
    }
  ],
  "warnings": [
    "2 stories have descriptions longer than 64KB and will be truncated"
  ]
}
```

### 6.3 Execute Endpoint

**`POST /api/workspaces/{wid}/import/shortcut/execute`**

Accepts the CSV file plus user mappings and configuration. Runs the import.

**Request:**
```
Content-Type: multipart/form-data
file: <csv file>
user_mappings: JSON string — { "shortcut_email": "helpin_user_id", ... }
invite_emails: JSON string — ["john@oldcompany.com", "jane.contractor@gmail.com"]
workflow_state_mappings: JSON string — see below
options: JSON string — {
  "import_archived": true,
  "import_completed": true
}
```

**`workflow_state_mappings` schema:**

Each Shortcut workflow is configured in one of two modes: **create new** (a new Helpin workflow is created mirroring the Shortcut workflow) or **use existing** (stories are mapped into an already-configured Helpin workflow).

```json
[
  {
    "shortcut_workflow_name": "Product Development",
    "mode": "create_new",
    "new_workflow_name": "Product Development",
    "states": [
      { "shortcut_state": "Backlog", "new_state_name": "Backlog", "state_type": "backlog", "position": 0 },
      { "shortcut_state": "Refinement", "new_state_name": "Refinement", "state_type": "unstarted", "position": 1 },
      { "shortcut_state": "Up next", "new_state_name": "Up next", "state_type": "unstarted", "position": 2 },
      { "shortcut_state": "Ready for Development", "new_state_name": "Ready for Development", "state_type": "unstarted", "position": 3 },
      { "shortcut_state": "In Development", "new_state_name": "In Development", "state_type": "started", "position": 4 },
      { "shortcut_state": "Ready for Review", "new_state_name": "Ready for Review", "state_type": "started", "position": 5 },
      { "shortcut_state": "Ready for Deploy", "new_state_name": "Ready for Deploy", "state_type": "started", "position": 6 },
      { "shortcut_state": "Completed", "new_state_name": "Completed", "state_type": "done", "position": 7 },
      { "shortcut_state": "Abandoned", "new_state_name": "Abandoned", "state_type": "done", "position": 8 }
    ]
  },
  {
    "shortcut_workflow_name": "Docs",
    "mode": "use_existing",
    "existing_workflow_id": "uuid-of-existing-helpin-workflow",
    "states": [
      { "shortcut_state": "Backlog", "existing_state_id": "uuid-of-helpin-state" },
      { "shortcut_state": "Done", "existing_state_id": "uuid-of-helpin-state" }
    ]
  }
]
```

**`create_new` mode fields:**
- `new_workflow_name` — Name for the new Helpin workflow (defaults to Shortcut name, user can edit).
- `states[].new_state_name` — Name for each new state (defaults to Shortcut name, user can rename).
- `states[].state_type` — Required. One of: `backlog`, `unstarted`, `started`, `done`.
- `states[].position` — Display order in the workflow.

**`use_existing` mode fields:**
- `existing_workflow_id` — UUID of an existing Helpin workflow.
- `states[].existing_state_id` — UUID of an existing state within that workflow. Multiple Shortcut states can map to the same Helpin state.

**Validation rules:**
- Every Shortcut workflow must be present in the mapping.
- Every Shortcut state within each workflow must be mapped.
- `create_new`: at least one `done` state per workflow. State type must be one of the four valid values.
- `use_existing`: `existing_workflow_id` and all `existing_state_id` values must exist and belong to the workspace.

**Response:**
```json
{
  "import_id": "uuid",
  "status": "processing"
}
```

### 6.4 Status Endpoint

**`GET /api/workspaces/{wid}/import/shortcut/status/{import_id}`**

Returns current import progress. The frontend polls this during import.

**Response:**
```json
{
  "import_id": "uuid",
  "status": "completed",
  "progress": {
    "current_step": "stories",
    "steps_completed": 8,
    "steps_total": 8,
    "entities_processed": 1626,
    "entities_total": 1626
  },
  "result": {
    "invitations_sent": 2,
    "teams_created": 3,
    "workflows_created": 2,
    "workflow_states_created": 10,
    "labels_created": 10,
    "objectives_created": 17,
    "epics_created": 107,
    "sprints_created": 74,
    "stories_created": 1626,
    "stories_skipped": 0,
    "checklist_items_created": 312,
    "owner_links_created": 1866,
    "label_links_created": 331,
    "warnings": [
      "2 invitations sent — stories assigned, awaiting invite acceptance",
      "11 imported sprints had null dates because iteration names could not be parsed"
    ]
  }
}
```

---

## 7. Backend Design

### 7.1 New Files

| File | Purpose |
|---|---|
| `server/internal/handler/pm_import.go` | HTTP handlers for preview/execute/status |
| `server/internal/service/pm_import.go` | Core import orchestration logic |
| `server/internal/service/shortcut_csv.go` | CSV parsing, validation, and entity extraction |
| `server/internal/model/pm_import.go` | `PMImportJob` model for tracking import status |
| DB migration(s) | Add `external_id` to `pm_epics`, `pm_objectives`, and `pm_sprints`; make `pm_sprints.start_date` and `pm_sprints.end_date` nullable |

### 7.2 Import Job Model

```go
type PMImportJob struct {
    ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
    Source      string     `json:"source" gorm:"not null"` // "shortcut"
    Status      string     `json:"status" gorm:"not null;default:'pending'"` // pending, processing, completed, failed
    FileName    string     `json:"file_name" gorm:"not null"`
    TotalRows   int        `json:"total_rows"`
    Progress    int        `json:"progress" gorm:"default:0"`
    Result      *string    `json:"result"` // JSON blob with counts & warnings
    Error       *string    `json:"error"`
    StartedBy   string     `json:"started_by" gorm:"type:uuid;not null"`
    CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
    CompletedAt *time.Time `json:"completed_at"`
}
```

### 7.3 Processing Flow

```
CSV Upload
    │
    ▼
Parse CSV → Extract unique entities
    │
    ├── Infer sprint dates from `iteration` names when possible
    ├── Collect warnings for null/ambiguous sprint dates
    │
    ▼
Begin Transaction
    │
    ├── Invite & create pending users (for emails marked "invite")
    ├── Create Teams (match existing by name, create missing)
    ├── Create/Resolve Workflows + States (per user mapping: create new workflow with states, or resolve existing workflow/state IDs)
    ├── Create Labels (match existing by name, create missing)
    ├── Create Objectives (keyed by external objective_id)
    ├── Create Epics (keyed by external epic_id)
    │     └── Link epics ↔ objectives
    │     └── Link epics ↔ labels
    ├── Create Sprints (keyed by external iteration_id; preserve name; nullable dates when unparseable)
    ├── Create Stories (keyed by external story id)
    │     └── Map workflow state by name within workflow
    │     └── Map team by name, else null
    │     └── Map requester/owners by email
    │     └── Set timestamps, priority, severity, estimate, etc.
    │     └── Set `owner_id` to first mapped owner when available
    ├── Create Story ↔ Owner links
    ├── Create Story ↔ Label links
    └── Create Checklist Items (parse `[X]`/`[ ]` prefix)
    │
    ▼
Commit Transaction
    │
    ▼
Update Import Job → status: completed, result: { counts }
```

### 7.4 Batch Insertion

For performance with ~1,600 stories:
- Use GORM `CreateInBatches` with batch size of 100.
- Collect all join-table records (story_owners, story_labels) and bulk-insert after stories.
- Estimated import time: < 10 seconds for this dataset size.

### 7.5 Import-Specific Behavior

- Stories, epics, objectives, and sprints persist Shortcut IDs into `external_id` for idempotency.
- Story `owner_id` is set to the first successfully mapped owner email, if any; all mapped owners are still inserted into `pm_story_owners`.
- Story, epic, objective, and sprint `team_id` may be null when the CSV does not provide a team or when the source spans multiple teams.
- Imported timestamps are written directly from CSV values and must not be recomputed from workflow state.

---

## 8. Frontend Design

### 8.1 Entry Point

Add an **"Import"** option in the workspace Settings page under a new **"Import / Export"** section. Accessible only to workspace admins.

Route: `/w/{slug}/settings/import`

### 8.2 Import Wizard (4 Steps)

**Step 1 — Upload & Preview**
- File upload dropzone accepting `.csv` files.
- Source selector (only "Shortcut" for now, designed to be extensible).
- After upload, calls the preview endpoint and shows:
  - Summary card: story count, epic count, objective count, sprint count, label count.
  - Breakdown by story type (feature/bug/chore).
  - Warning badges for any issues detected.

**Step 2 — Map Workflows & States**

This is the critical step. For each Shortcut workflow found in the CSV, the user decides whether to **create a new Helpin workflow** (mirroring the Shortcut workflow) or **map to an existing Helpin workflow**. Either way, every Shortcut state must be assigned a Helpin state type so that boards, metrics, and completion tracking work correctly.

**Per-workflow card** (one collapsible card per Shortcut workflow, e.g., "Product Development — 9 states, 1,580 stories"):

- **One-click default** — Each workflow card starts in **"Create new workflow"** mode with all state names, positions, and state types **already pre-filled** from auto-detection. The user sees a ready-to-go table with green checkmarks. For most imports, no manual editing is needed — the user just reviews and clicks "Next".

- **Mode toggle** at the top of each card:
  - **"Create new workflow"** (default) — A new Helpin workflow is created with the same name and all Shortcut states are recreated as Helpin workflow states. State types are **auto-assigned** from well-known name patterns (e.g., "Backlog" → `backlog`, "In Development" → `started`, "Completed" → `done`). The user can override any state type, rename states, or reorder them, but this is optional.
  - **"Use existing workflow"** — A dropdown lists existing Helpin workflows. When selected, each Shortcut state must be mapped to an existing Helpin workflow state via a dropdown. This is useful if the user already set up their Helpin workflows before importing.

- **Create new workflow mode** — state mapping table:
  - **State Name** — the Shortcut state name (editable — user can rename during import).
  - **Story Count** — how many stories are in this state (helps understand impact).
  - **State Type** — dropdown: `Backlog`, `Unstarted`, `Started`, `Done`.
    Pre-filled with `suggested_type` from the preview response (auto-detected from well-known names). Color-coded pills per state type (gray=backlog, blue=unstarted, yellow=started, green=done).
  - **Position** — drag handle to reorder states (defaults to the order they appear in the CSV).
  - The entire table is pre-populated and valid by default. The user only needs to intervene if auto-detection got a state type wrong.

- **Use existing workflow mode** — state mapping table:
  - **Shortcut State** — the state name from the CSV (read-only).
  - **Story Count** — count of stories in this state.
  - **Helpin State** — dropdown of states from the selected existing workflow. Multiple Shortcut states can map to the same Helpin state (e.g., both "Completed" and "Done" → the existing "Done" state).

- **Validation:**
  - In "create new" mode: at least one `done` state per workflow is required.
  - In "use existing" mode: every Shortcut state must be mapped to a Helpin state.
  - Show a warning if a workflow has no `backlog` or no `started` states (non-blocking).
  - "Reset to suggested" button per workflow to revert manual changes.

**Why this step matters:** Helpin uses `state_type` to determine:
  - Board column grouping (backlog/unstarted/started/done columns)
  - Whether a story is considered "started" or "completed" for metrics
  - Sprint velocity and burndown calculations
  - Epic/objective progress percentages

Getting this wrong would make the entire board and reporting layer incorrect for all imported data.

**Step 3 — Map Users**
- Table showing all unique emails from the CSV with story count per user.
- Auto-matched emails (exact email match to existing workspace members) show a green checkmark.
- **"Match All by Email"** button at the top auto-assigns all exact matches in one click.
- For unmatched emails, three options per row:
  - **Select existing member** — dropdown of workspace members to manually map to.
  - **Invite & Map** — sends a workspace invitation to that email. The invited user is created immediately in the system so that stories can be assigned to them during import. When the user accepts the invite, they gain full access. This ensures no stories are left unassigned.
  - **Skip** — no mapping; stories from this user import without owner/requester.
- **"Invite All Unmatched"** button — one-click to send invitations to all unmatched emails at once, mapping them all in a single action.
- Invited users are shown with a pending badge so the admin can distinguish between already-active members and newly-invited ones.

**Step 4 — Configure & Import**
- Toggle options:
  - **Import archived stories** (default: on)
  - **Import completed stories** (default: on)
- Summary of what will be created (final counts).
- Warning list for inferred sprint dates and sprints imported with null dates.
- "Start Import" button.
- Progress bar with step labels (Teams → Workflows → Labels → Objectives → Epics → Sprints → Stories → Links).
- On completion: success summary with created/skipped counts and any warnings.

### 8.3 ASCII Wireframes

#### Settings Entry Point — Import / Export Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Settings                                                                   │
│                                                                             │
│  ┌──────────────────┐                                                       │
│  │ General          │                                                       │
│  │ Members          │  ┌─────────────────────────────────────────────────┐   │
│  │ Teams            │  │  Import / Export                                │   │
│  │ Workflows        │  │                                                │   │
│  │ Labels           │  │  Import data from another project management   │   │
│  │ Estimate Scales  │  │  tool into this workspace.                     │   │
│  │ ────────────     │  │                                                │   │
│  │ Account          │  │  ┌───────────────────────────────────────────┐  │   │
│  │ ────────────     │  │  │                                           │  │   │
│  │▸Import / Export  │  │  │   📋  Shortcut                            │  │   │
│  │                  │  │  │   Import stories, epics, objectives,      │  │   │
│  │                  │  │  │   and sprints from a Shortcut CSV export.  │  │   │
│  │                  │  │  │                                           │  │   │
│  │                  │  │  │                    [ Start Import ]        │  │   │
│  │                  │  │  │                                           │  │   │
│  │                  │  │  └───────────────────────────────────────────┘  │   │
│  │                  │  │                                                │   │
│  │                  │  │  ┌───────────────────────────────────────────┐  │   │
│  │                  │  │  │                                           │  │   │
│  │                  │  │  │   🔲  Jira           Coming Soon          │  │   │
│  │                  │  │  │                                           │  │   │
│  │                  │  │  └───────────────────────────────────────────┘  │   │
│  │                  │  │                                                │   │
│  │                  │  │  ┌───────────────────────────────────────────┐  │   │
│  │                  │  │  │                                           │  │   │
│  │                  │  │  │   🔲  Linear         Coming Soon          │  │   │
│  │                  │  │  │                                           │  │   │
│  │                  │  │  └───────────────────────────────────────────┘  │   │
│  │                  │  │                                                │   │
│  └──────────────────┘  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Wizard Chrome — Step Indicator (shown at top of all steps)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ● active            ○ pending          ○ pending      ○ pending         │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │                     [ step content here ]                           │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│                                            [ Back ]   [ Next: Workflows ]   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 1 — Upload & Preview (before upload)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ● active            ○                  ○              ○                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  Upload your Shortcut CSV export                                    │     │
│  │                                                                     │     │
│  │  In Shortcut, go to Settings → Export → CSV to download             │     │
│  │  your workspace data.                                               │     │
│  │                                                                     │     │
│  │  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐     │     │
│  │                                                                     │     │
│  │  │         📄  Drag and drop your CSV file here              │     │     │
│  │               or click to browse                                    │     │
│  │  │                                                           │     │     │
│  │               Accepts .csv files up to 50 MB                        │     │
│  │  │                                                           │     │     │
│  │  └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘     │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│                                                    [ Next: Workflows ]      │
│                                                       (disabled)            │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 1 — Upload & Preview (after upload — preview loaded)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ● active            ○                  ○              ○                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  ✓ workspace-60ae676e-...-exported.csv              [ Change File ] │     │
│  │                                                                     │     │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐   │     │
│  │  │   Stories    │ │    Epics    │ │ Objectives  │ │   Sprints   │   │     │
│  │  │             │ │             │ │             │ │             │   │     │
│  │  │    1,626    │ │     107     │ │      17     │ │      74     │   │     │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘   │     │
│  │                                                                     │     │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐   │     │
│  │  │   Labels    │ │    Teams    │ │  Workflows  │ │  Checklists │   │     │
│  │  │             │ │             │ │             │ │             │   │     │
│  │  │      10     │ │       3     │ │       2     │ │     312     │   │     │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘   │     │
│  │                                                                     │     │
│  │  Stories by Type                                                    │     │
│  │  ┌──────────────────────────────────────────────────────────────┐   │     │
│  │  │ ██████████████████████████████████████  feature    1,020     │   │     │
│  │  │ ████████████████                       bug          415     │   │     │
│  │  │ ████████                               chore        191     │   │     │
│  │  └──────────────────────────────────────────────────────────────┘   │     │
│  │                                                                     │     │
│  │  ⚠ 2 stories have descriptions longer than 64KB (will truncate)    │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│                                                    [ Next: Workflows → ]    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 2 — Map Workflows & States (Create New mode — default, pre-filled)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ✓ done              ● active           ○              ○                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  Map Workflows & States                                             │     │
│  │  We auto-detected your Shortcut workflows and assigned state        │     │
│  │  types. Review and adjust if needed, or just continue.              │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │ ▾ Product Development           ✓ Ready     1,580 stories  │     │     │
│  │  │                                                             │     │     │
│  │  │   ( ● Create new workflow ) ( ○ Use existing workflow )     │     │     │
│  │  │                                                             │     │     │
│  │  │   Workflow name: [ Product Development          ]           │     │     │
│  │  │                                                             │     │     │
│  │  │   ┌────┬──────────────────────────┬────────┬─────────────┐  │     │     │
│  │  │   │ ⠿  │ State Name               │ Stories│ State Type   │  │     │     │
│  │  │   ├────┼──────────────────────────┼────────┼─────────────┤  │     │     │
│  │  │   │ ⠿  │ [ Backlog              ] │    108 │ ■ Backlog  ▾│  │     │     │
│  │  │   │ ⠿  │ [ Refinement           ] │     18 │ ■ Unstarted▾│  │     │     │
│  │  │   │ ⠿  │ [ Up next              ] │      8 │ ■ Unstarted▾│  │     │     │
│  │  │   │ ⠿  │ [ Ready for Development] │      1 │ ■ Unstarted▾│  │     │     │
│  │  │   │ ⠿  │ [ In Development       ] │      2 │ ■ Started  ▾│  │     │     │
│  │  │   │ ⠿  │ [ Ready for Review     ] │      6 │ ■ Started  ▾│  │     │     │
│  │  │   │ ⠿  │ [ Ready for Deploy     ] │      1 │ ■ Started  ▾│  │     │     │
│  │  │   │ ⠿  │ [ Completed            ] │  1,429 │ ■ Done     ▾│  │     │     │
│  │  │   │ ⠿  │ [ Abandoned            ] │      7 │ ■ Done     ▾│  │     │     │
│  │  │   └────┴──────────────────────────┴────────┴─────────────┘  │     │     │
│  │  │                                                             │     │     │
│  │  │   ✓ All state types auto-detected        [ Reset suggested ]│     │     │
│  │  │                                                             │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │ ▸ Docs                           ✓ Ready       46 stories  │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│  All workflows are ready. You can continue or expand to customize.          │
│                                                                             │
│                                       [ ← Back ]   [ Next: Users → ]       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 2 — Map Workflows & States (Use Existing mode)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  │  ┌─────────────────────────────────────────────────────────────┐     │    │
│  │  │ ▾ Docs                                       46 stories     │     │    │
│  │  │                                                             │     │    │
│  │  │   ( ○ Create new workflow ) ( ● Use existing workflow )     │     │    │
│  │  │                                                             │     │    │
│  │  │   Existing workflow: [▾ Documentation Workflow     ]        │     │    │
│  │  │                                                             │     │    │
│  │  │   ┌──────────────────────────┬────────┬──────────────────┐  │     │    │
│  │  │   │ Shortcut State           │ Stories│ Helpin State      │  │     │    │
│  │  │   ├──────────────────────────┼────────┼──────────────────┤  │     │    │
│  │  │   │ Backlog                  │     10 │ [▾ To Do       ] │  │     │    │
│  │  │   │ Done                     │     36 │ [▾ Published   ] │  │     │    │
│  │  │   └──────────────────────────┴────────┴──────────────────┘  │     │    │
│  │  │                                                             │     │    │
│  │  │   ✓ All states mapped                                       │     │    │
│  │  │                                                             │     │    │
│  │  └─────────────────────────────────────────────────────────────┘     │    │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 2 — Validation Error State

```
│  │   ┌────┬──────────────────────────┬────────┬─────────────┐  │     │
│  │   │ ⠿  │ State Name               │ Stories│ State Type   │  │     │
│  │   ├────┼──────────────────────────┼────────┼─────────────┤  │     │
│  │   │ ⠿  │ [ Backlog              ] │    108 │ [▾ Backlog ] │  │     │
│  │   │ ⠿  │ [ In Progress          ] │     20 │ [▾ Started ] │  │     │
│  │   │ ⠿  │ [ Review               ] │      5 │ [▾ Started ] │  │     │
│  │   └────┴──────────────────────────┴────────┴─────────────┘  │     │
│  │                                                             │     │
│  │   ✗ This workflow has no Done state. At least one is required.     │
│  │   ⚠ No Unstarted states — new stories will start in Backlog.       │
```

#### Step 3 — Map Users

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ✓ done              ✓ done             ● active       ○                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  Map Users                                                          │     │
│  │  Match Shortcut users to Helpin workspace members. You can          │     │
│  │  invite unmatched users so their stories are properly assigned.     │     │
│  │                                                                     │     │
│  │  7 of 9 users auto-matched                                          │     │
│  │                                                                     │     │
│  │  [ Match All by Email ]   [ Invite All Unmatched (2) ]              │     │
│  │                                                                     │     │
│  │  Matched Members                                                    │     │
│  │  ┌──────────────────────────────────┬────────┬──────────────────┐   │     │
│  │  │ Shortcut User                    │ Stories│ Helpin Member     │   │     │
│  │  ├──────────────────────────────────┼────────┼──────────────────┤   │     │
│  │  │ ✓ azhar@contentstudio.io         │    312 │ Azhar Khan       │   │     │
│  │  │ ✓ amad.ali@usermaven.com         │    287 │ Amad Ali         │   │     │
│  │  │ ✓ sheharyar.khalid@d4int...      │    245 │ Sheharyar K.     │   │     │
│  │  │ ✓ abdurrehman.afridi@d4int...    │    198 │ Abdur Rehman     │   │     │
│  │  │ ✓ adeel.khan@d4interactive.io    │    156 │ Adeel Khan       │   │     │
│  │  │ ✓ ali.raza@d4interactive.io      │    134 │ Ali Raza         │   │     │
│  │  │ ✓ waqar@d4interactive.io         │     98 │ Waqar Ahmed      │   │     │
│  │  └──────────────────────────────────┴────────┴──────────────────┘   │     │
│  │                                                                     │     │
│  │  Unmatched Users (2)                                                │     │
│  │  ┌──────────────────────────────────┬────────┬──────────────────┐   │     │
│  │  │ Shortcut User                    │ Stories│ Action            │   │     │
│  │  ├──────────────────────────────────┼────────┼──────────────────┤   │     │
│  │  │ ⚠ john@oldcompany.com            │     42 │ [▾ Select...   ] │   │     │
│  │  │ ⚠ jane.contractor@gmail.com      │     12 │ [▾ Select...   ] │   │     │
│  │  └──────────────────────────────────┴────────┴──────────────────┘   │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│                                     [ ← Back ]   [ Next: Import → ]        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 3 — Unmatched User Dropdown (expanded)

```
│  │ ⚠ john@oldcompany.com            │     42 │ [▾              ] │   │
│  │                                           │ ── Existing ───  │   │
│  │                                           │  Azhar Khan      │   │
│  │                                           │  Amad Ali        │   │
│  │                                           │  Sheharyar K.    │   │
│  │                                           │  ...             │   │
│  │                                           │ ─────────────── │   │
│  │                                           │  ✉ Invite & Map  │   │
│  │                                           │ ─────────────── │   │
│  │                                           │  ✗ Skip          │   │
│  │                                           └─────────────────┘   │
```

#### Step 3 — After "Invite All Unmatched" clicked

```
│  │  Unmatched Users (0)    ✓ All users mapped                          │     │
│  │  ┌──────────────────────────────────┬────────┬──────────────────┐   │     │
│  │  │ Shortcut User                    │ Stories│ Status            │   │     │
│  │  ├──────────────────────────────────┼────────┼──────────────────┤   │     │
│  │  │ ✉ john@oldcompany.com            │     42 │ Invited (pending)│   │     │
│  │  │ ✉ jane.contractor@gmail.com      │     12 │ Invited (pending)│   │     │
│  │  └──────────────────────────────────┴────────┴──────────────────┘   │     │
│  │                                                                     │     │
│  │  Invitations will be sent when the import starts. Stories will       │     │
│  │  be assigned to these users. They'll see their work once they       │     │
│  │  accept the invite and sign in.                                     │     │
```

#### Step 4 — Configure & Import (before starting)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ✓ done              ✓ done             ✓ done         ● active          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  Review & Import                                                    │     │
│  │                                                                     │     │
│  │  Options                                                            │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  [✓] Import archived stories              68 stories        │     │     │
│  │  │  [✓] Import completed stories           1,482 stories       │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  Summary                                                            │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  Entity                    Will Create                      │     │     │
│  │  │  ─────────────────────────────────────                      │     │     │
│  │  │  Teams                               3                     │     │     │
│  │  │  Workflows                           2 (1 new, 1 existing) │     │     │
│  │  │  Workflow States                    11 (9 new, 2 mapped)   │     │     │
│  │  │  Labels                             10                     │     │     │
│  │  │  Objectives                         17                     │     │     │
│  │  │  Epics                             107                     │     │     │
│  │  │  Sprints                            74                     │     │     │
│  │  │  Stories                          1,626                    │     │     │
│  │  │  Checklist Items                   312                     │     │     │
│  │  │  User Mappings          7 matched, 2 invited (pending)     │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  Warnings                                                           │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  ✉ 2 workspace invitations will be sent during import.       │     │     │
│  │  │  ⚠ 11 sprints have unparseable dates — they will be         │     │     │
│  │  │    imported with null start/end dates.                      │     │     │
│  │  │  ⚠ 2 descriptions exceed 64KB and will be truncated.        │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│                                     [ ← Back ]   [ Start Import ]          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 4 — Import In Progress

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ✓ done              ✓ done             ✓ done         ● active          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  Importing...                                                       │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │                                                             │     │     │
│  │  │  ████████████████████████████████░░░░░░░░░░░░  68%          │     │     │
│  │  │                                                             │     │     │
│  │  │  Step 6 of 8 — Creating stories                             │     │     │
│  │  │  1,108 of 1,626 stories processed                           │     │     │
│  │  │                                                             │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  Steps                                                              │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  ✓ Teams                              3 created             │     │     │
│  │  │  ✓ Workflows & States                 2 workflows, 11 states│     │     │
│  │  │  ✓ Labels                             10 created            │     │     │
│  │  │  ✓ Objectives                         17 created            │     │     │
│  │  │  ✓ Epics                              107 created           │     │     │
│  │  │  ◐ Stories                            1,108 / 1,626         │     │     │
│  │  │  ○ Story Links                        pending               │     │     │
│  │  │  ○ Checklist Items                    pending               │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  Do not close this window while the import is in progress.          │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 4 — Import Complete

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Import from Shortcut                                            [ X ]      │
│                                                                             │
│  ── (1) Upload ──────── (2) Workflows ──── (3) Users ──── (4) Import ──     │
│     ✓ done              ✓ done             ✓ done         ✓ done            │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │                                                                     │     │
│  │  Import Complete                                                    │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │                                                             │     │     │
│  │  │  ████████████████████████████████████████████████  100%     │     │     │
│  │  │                                                             │     │     │
│  │  │  All 8 steps completed successfully.                        │     │     │
│  │  │                                                             │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  Results                                                            │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  Entity                   Created   Skipped                 │     │     │
│  │  │  ──────────────────────────────────────────                 │     │     │
│  │  │  Teams                          3         0                 │     │     │
│  │  │  Workflows                      1         1 (existing)      │     │     │
│  │  │  Workflow States                9         2 (existing)      │     │     │
│  │  │  Labels                        10         0                 │     │     │
│  │  │  Objectives                    17         0                 │     │     │
│  │  │  Epics                        107         0                 │     │     │
│  │  │  Sprints                       74         0                 │     │     │
│  │  │  Stories                    1,626         0                 │     │     │
│  │  │  Checklist Items              312         0                 │     │     │
│  │  │  Owner Links                1,866         0                 │     │     │
│  │  │  Label Links                  331         0                 │     │     │
│  │  │  Invitations Sent               2         —                 │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  Warnings (3)                                                       │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  ✉ 2 invitations sent: john@oldcompany.com,                  │     │     │
│  │  │    jane.contractor@gmail.com — stories assigned, awaiting   │     │     │
│  │  │    invite acceptance.                                       │     │     │
│  │  │  ⚠ 11 sprints imported with null dates (unparseable         │     │     │
│  │  │    iteration names).                                        │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│                                     [ Go to Board ]   [ Close ]            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Step 4 — Import Failed

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  │                                                                     │     │
│  │  Import Failed                                                      │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │                                                             │     │     │
│  │  │  ████████████████████████░░░░░░░░░░░░░░░░░░░░  45%          │     │     │
│  │  │                                                             │     │     │
│  │  │  Failed at step 6 — Creating stories                        │     │     │
│  │  │                                                             │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │  ┌─────────────────────────────────────────────────────────────┐     │     │
│  │  │  ✗ Error: Database constraint violation on story row 847.   │     │     │
│  │  │    Duplicate external_id "85463" detected.                  │     │     │
│  │  │                                                             │     │     │
│  │  │  All changes have been rolled back. No data was imported.   │     │     │
│  │  └─────────────────────────────────────────────────────────────┘     │     │
│  │                                                                     │     │
│  │                               [ ← Back to Settings ]  [ Retry ]    │     │
│  │                                                                     │     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 8.4 Component Structure

```
src/components/import/
  ImportWizard.tsx              — Main wizard container with step navigation
  ImportUploadStep.tsx          — File upload + preview display
  ImportWorkflowMappingStep.tsx — Workflow state → state type mapping per workflow
  ImportUserMappingStep.tsx     — User email → member mapping table
  ImportConfigStep.tsx          — Options + progress + results
  ImportSummaryCard.tsx         — Reusable summary statistics card
```

### 8.4 State Management

Use local React state within the wizard (not a Zustand store) since import is a transient operation. The wizard holds:

```typescript
interface WorkflowStateMapping =
  | {
      shortcutWorkflowName: string;
      mode: "create_new";
      newWorkflowName: string;
      states: {
        shortcutState: string;
        newStateName: string;
        stateType: "backlog" | "unstarted" | "started" | "done";
        position: number;
      }[];
    }
  | {
      shortcutWorkflowName: string;
      mode: "use_existing";
      existingWorkflowId: string;
      states: {
        shortcutState: string;
        existingStateId: string;
      }[];
    };

type UserAction =
  | { type: "matched"; userId: string }   // auto-matched or manually selected
  | { type: "invite" }                     // will invite & map during import
  | { type: "skip" };                      // no mapping

interface ImportState {
  file: File | null;
  preview: ImportPreview | null;
  workflowMappings: WorkflowStateMapping[];
  userActions: Record<string, UserAction>; // email → action
  options: { importArchived: boolean; importCompleted: boolean };
  importId: string | null;
  status: ImportStatus | null;
}
```

---

## 9. Detailed Field Mapping Reference

### 9.1 Story Fields

| CSV Column | Helpin Field | Transform |
|---|---|---|
| `id` | `external_id` | Cast to string |
| `name` | `name` | Direct |
| `type` | `story_type` | Direct (`feature`, `bug`, `chore`) |
| `description` | `description` | Direct (Markdown preserved) |
| `state` | `workflow_state_id` | Resolved via `workflow_state_mappings`: if `create_new`, lookup by new state name; if `use_existing`, lookup by `existing_state_id` |
| `workflow` | `workflow_id` | Resolved via `workflow_state_mappings`: if `create_new`, ID of newly created workflow; if `use_existing`, `existing_workflow_id` |
| `team` | `team_id` | Lookup by team name; null if missing or unmapped |
| `requester` | `requester_id` | User mapping by email |
| `owners` | `owner_id`, `pm_story_owners` | Split by `;`, user mapping by email; first mapped owner becomes `owner_id`, all mapped owners go to `pm_story_owners` |
| `estimate` | `estimate` | Parse int, null if empty |
| `priority` | `priority` | Enum mapping (see 4.2) |
| `severity` | `severity` | Enum mapping (see 4.2) |
| `due_date` | `deadline` | Parse date |
| `is_completed` | `completed` | Parse bool |
| `is_blocked` | `blocked` | Parse bool |
| `is_archived` | `archived` | Parse bool |
| `created_at` | `created_at` | Parse datetime `YYYY/MM/DD HH:MM:SS` with `utc_offset` |
| `started_at` | `started_at` | Parse datetime |
| `completed_at` | `completed_at` | Parse datetime |
| `moved_at` | `moved_at` | Parse datetime |
| `updated_at` | `updated_at` | Parse datetime |
| `epic_id` | `epic_id` | Lookup by mapped epic UUID |
| `iteration_id` | `sprint_id` | Lookup by mapped sprint UUID |
| `labels` | `pm_story_labels` | Split by `;`, lookup by label name |
| `tasks` | `pm_checklist_items` | Split by `;`, parse `[X]`/`[ ]` prefix |

### 9.2 Epic Fields

| CSV Column | Helpin Field | Transform |
|---|---|---|
| `epic_id` | `external_id` | Cast to string |
| `epic` | `name` | Direct |
| `epic_state` | `epic_state_id` | `in progress` → started state, `done` → done state |
| `epic_is_archived` | `archived` | Parse bool |
| `epic_created_at` | `created_at` | Parse datetime |
| `epic_started_at` | `started_at` | Parse datetime |
| `epic_due_date` | `deadline` | Parse date |
| `epic_planned_start_date` | `planned_start_date` | Parse date |
| `epic_labels` | `pm_epic_labels` | Split by `;`, lookup by label name |
| `team_id` / `team` | `team_id` | Lookup by team name when all rows for the epic agree; otherwise null |

### 9.3 Objective Fields

| CSV Column | Helpin Field | Transform |
|---|---|---|
| `objective_id` | `external_id` | Cast to string |
| `objective` | `name` | Direct |
| `objective_state` | `state` | `to do` → `not_started`, `in progress` → `active`, `done` → `closed` |
| `objective_created_at` | `created_at` | Parse datetime |
| `objective_started_at` | `planned_start_date` | Parse datetime |
| `objective_due_date` | `deadline` | Parse date |

### 9.4 Sprint Fields

| CSV Column | Helpin Field | Transform |
|---|---|---|
| `iteration_id` | `external_id` | Cast to string |
| `iteration` | `name` | Preserve exact source name |
| `iteration` | `start_date` / `end_date` | Best-effort parse from known naming patterns; null if parsing fails |
| `team_id` / `team` | `team_id` | Lookup by team name when all rows for the sprint agree; otherwise null |

### 9.5 Checklist Item Parsing

The `tasks` column contains semicolon-separated items in the format:
```
[X] Completed task text;[ ] Incomplete task text
```

Parser logic:
1. Split by `;`
2. Trim whitespace
3. Check prefix: `[X]` → `completed: true`, `[ ]` → `completed: false`
4. Strip the prefix to get `text`
5. Assign sequential `position` values

---

## 10. Authorization & Permissions

- Only **workspace admins** can access the import feature.
- The import runs as the authenticated user (recorded in `started_by`).
- Imported entities belong to the workspace — standard workspace RBAC applies afterward.

---

## 11. Extensibility

The architecture is designed to support future import sources:

- The `PMImportJob.Source` field identifies the import source (`shortcut`, `jira`, `linear`, etc.).
- The CSV parser is isolated in `shortcut_csv.go`. New sources get their own parser file.
- The preview/execute endpoints accept a `source` parameter.
- The frontend source selector can be extended with new options.

---

## 12. Edge Cases & Risks

| Case | Handling |
|---|---|
| CSV has no header row | Reject with clear error |
| CSV missing required columns (`id`, `name`, `type`, `state`) | Reject with list of missing columns |
| Duplicate imported `external_id` in workspace | Skip the matching story/epic/objective/sprint, count it in the import summary |
| Empty story name | Use `"Untitled Story (SC-{id})"` |
| Description > 64KB | Truncate with warning |
| Unknown story type | Default to `feature` with warning |
| Unknown state name | Assign to workflow's default state with warning |
| Sprint date parsing failure | Import the sprint with null `start_date` / `end_date` and warn |
| Story/epic/objective date parsing failure | Set the affected field to null with warning |
| Missing or unmapped story team | Import with `team_id = null` |
| Epic or sprint spans multiple teams | Import with `team_id = null` |
| CSV file > 50MB | Reject at upload with size limit error |
| Concurrent imports on same workspace | Block — only one active import per workspace |
| User cancels mid-import | Transaction rolls back, import job marked `failed` |

---

## 13. Testing Plan

| Test | Type |
|---|---|
| CSV parser handles all 54 columns correctly | Unit |
| Enum mappings (priority, severity, state, objective state) | Unit |
| Checklist item `[X]`/`[ ]` parsing | Unit |
| Semicolon-separated field splitting (labels, owners, tasks) | Unit |
| Deduplication of epics/objectives/sprints across CSV rows | Unit |
| Sprint date parsing from known iteration name formats | Unit |
| Sprint import with null dates when parsing fails | Unit |
| Null team handling for missing or multi-team entities | Unit |
| User email matching (exact, case-insensitive) | Unit |
| Full import with real Shortcut CSV (1,626 rows) | Integration |
| Import idempotency — re-import same file produces 0 new entities | Integration |
| Transaction rollback on simulated failure | Integration |
| Import with no user mappings (all null) | Integration |
| Import with all options disabled (no archived, no completed) | Integration |
| Preview endpoint returns correct counts | API |
| Status endpoint reflects real-time progress | API |
| Frontend wizard complete flow | E2E |

---

## 14. Milestones

| Phase | Scope | Estimate |
|---|---|---|
| **Phase 1 — Backend** | CSV parser, import service, DB transaction, 3 API endpoints, import job model | |
| **Phase 2 — Frontend** | Upload wizard, preview display, user mapping UI, progress tracking | |
| **Phase 3 — Polish** | Error handling, warnings display, idempotency, admin-only gate | |
