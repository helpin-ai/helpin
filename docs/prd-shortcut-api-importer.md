# PRD: Shortcut API Importer

**Author:** Engineering
**Date:** 2026-04-26
**Status:** Implementation in progress
**Supersedes:** `docs/prd-shortcut-importer.md` for new implementation work

## 1. Summary

Build a complete, repeatable Shortcut import that uses the Shortcut REST API v3 as the source of truth. The importer should replace the current CSV-first flow, which is incomplete and brittle, with a token-based, previewable, resumable import that pulls Shortcut workspace data directly from the API and writes it into Helpin's project-management domain.

The CSV importer remains useful only as legacy implementation reference. The product path is API-only:

1. Admin enters a Shortcut API token.
2. Helpin validates the token and scans the Shortcut workspace.
3. Admin reviews entity counts, destination team mappings, workflow/state mappings, user mappings, and import options.
4. Helpin executes the import as a durable job with progress, warnings, retry-safe writes, and a final report.

## 2. Problem

The current Shortcut migration path depends on CSV export plus partial API enrichment. That makes the import flaky because CSV export shape can vary, CSV lacks several important entity relationships, and any CSV parsing error blocks the whole migration. It also misses or weakly handles important data that Shortcut's API exposes directly, including:

- Story comments and comment authors.
- Story tasks/checklist items with IDs, positions, owners, and timestamps.
- Story links/relationships.
- Files and linked files.
- Accurate member, workflow, workflow state, iteration, epic, objective, group, project, and label metadata.
- API-provided IDs and timestamps that support idempotent imports.

Customers moving from Shortcut need Helpin to preserve structure, ownership, and history well enough that they can continue work in Helpin immediately after import.

## 3. Goals

- Import Shortcut data through REST API v3, not CSV, for the primary migration flow.
- Preserve project-management hierarchy: objectives, epics, iterations/sprints, workflows/states, teams/groups, labels, stories/tasks, relationships, comments, files, and checklist items.
- Provide a preview before writing Helpin data.
- Make imports idempotent and retry-safe, keyed by Shortcut entity IDs and import run metadata.
- Show progress by phase and final counts/warnings.
- Respect Shortcut API authentication and rate limits.
- Avoid leaking Shortcut API tokens into logs, database plaintext fields, frontend error messages, or job results.
- Keep current Helpin PM behavior intact by using import-specific write paths that do not emit normal user activity, automations, or websocket noise unless explicitly intended.

## 4. Non-Goals

- Ongoing two-way sync with Shortcut.
- Creating or modifying Shortcut data.
- Perfect recreation of Shortcut analytics, Shortcut search ranking, or historical activity feeds.
- Importing every Git/VCS integration object as first-class Helpin records in v1.
- Migrating Shortcut Docs into Helpin Docs in this project. Story descriptions, comments, files, and links are in scope.
- Requiring customers to produce a CSV file.

## 5. Source API Facts

Primary docs: https://developer.shortcut.com/api/rest/v3

Implementation-relevant constraints:

- Base API host: `https://api.app.shortcut.com/api/v3`.
- Auth uses the `Shortcut-Token` header. Query-token auth exists but is deprecated and must not be used.
- API traffic is JSON over HTTPS.
- Rate limit is 200 requests per minute.
- Search endpoints support `page_size` up to 250 and `next` pagination.
- Search result sets can fail above 1000 results, so the importer must partition story search queries for large workspaces.
- REST API v3 is the current version as of 2026-04-26.

## 6. Users And Permissions

Primary user:

- Workspace owner/admin importing a Shortcut workspace into an existing Helpin workspace.

Permission requirements:

- Only workspace admins/owners can validate Shortcut tokens, preview imports, execute imports, or view import job status.
- The Shortcut token gives broad access to the Shortcut account. Treat it as a secret:
  - Never log it.
  - Never include it in job result JSON.
  - Never return it from status endpoints.
  - Store only encrypted token material if a long-running import requires persistence beyond request memory.

## 7. Current State

Existing implementation:

- `server/internal/service/pm_import.go` implements CSV preview and execution.
- `server/internal/service/shortcut_csv.go` parses Shortcut CSV rows.
- `server/internal/service/shortcut_api.go` validates API tokens and fetches partial enrichment for labels, iterations, workflows, epics, objectives, members, and story comments.
- `frontend/src/components/pm/ShortcutImportWizard.tsx` uses API token input and API progress steps.
- `server/internal/model/pm_import.go` has `PMImportJob` and Shortcut import DTOs.

Gaps:

- CSV is no longer exposed in the product importer.
- API client is enrichment-only, not a complete extractor.
- No robust API pagination, partitioning, retry, or 429 handling.
- No import snapshot/staging model for resumability.
- No API-native preview response.
- No durable secret handling for long-running API jobs.
- Story relationships, files, linked files, sub-tasks, and full comments are not treated as first-class import phases.

## 8. Product Experience

### 8.1 Entry Point

Route: `/w/{slug}/settings/import`

The Shortcut card should be an API import. CSV must not be exposed as a fallback in the importer UI.

### 8.2 Wizard Steps

#### Step 1: Connect Shortcut

Inputs:

- Shortcut API token.
- Optional import name.

Actions:

- Validate token with `GET /member`.
- Fetch current Shortcut workspace/member metadata.
- Show connected workspace name, authenticated Shortcut member, and token validation status.

Errors:

- Invalid/expired token.
- Shortcut API unavailable.
- Rate limited during validation.

#### Step 2: Scan And Preview

Actions:

- Start a preview scan job or run a bounded synchronous scan if the workspace is small.
- Fetch reference entities and estimate/import counts.
- Show counts by entity type and key warnings.

Preview should include:

- Stories total, by type, state, workflow, owner, group/team, epic, iteration, and archived/completed status.
- Objectives, epics, iterations, labels, groups, projects, workflows/states, members.
- Comments count estimate.
- Files/linked files count estimate.
- Story links count estimate.
- Duplicate count against Helpin external IDs.
- Token scope/access warnings.

#### Step 3: Map Teams

Admins must explicitly choose where Shortcut teams/groups land before stories are written:

- Shortcut `group_id` maps to a Helpin team when group data is available.
- Shortcut `project.team_id` can be used as a secondary team signal.
- Each Shortcut group can create a new Helpin team or map into an existing Helpin team.
- If a story has no group/team, import with `team_id = null`.
- Rerunning an import with changed team mappings must update already-imported Shortcut stories instead of leaving them in the old team.

#### Step 4: Map Workflows And States

Reuse the existing workflow/state mapping UI, but source data from Shortcut API workflows:

- Shortcut workflow -> new Helpin workflow or existing Helpin workflow.
- Shortcut workflow state -> Helpin state.
- When creating new Helpin workflows, create a separate workflow per mapped destination team so team-specific boards do not share one workspace-level workflow accidentally.
- Preserve Shortcut state positions.
- Require every imported story's `workflow_state_id` to resolve.
- Require at least one `done` state per created workflow.

#### Step 5: Map Users

Members:

- Auto-match Shortcut members to Helpin workspace members by email.
- Allow manual mapping to a Helpin member.
- Allow skip.
- Allow invite-and-map if the existing invite flow supports creating a pending member immediately.

#### Step 6: Configure Import

Options:

- Import archived stories: default on.
- Import completed stories: default on.
- Import comments: default on.
- Import files: default on.
- Import linked files as external links: default on.
- Import story links: default on.
- Import sub-tasks/checklist items: default on.
- Rewrite Shortcut app URLs inside descriptions/comments to Helpin task links after import: default on.
- Preserve original created/updated/completed timestamps: default on.

#### Step 6: Execute And Review

Show:

- Progress bar by phase.
- Current entity type and processed/total counts.
- Latest non-sensitive warning.
- Cancel button if cancellation is implementable before commit boundaries.
- Final report with created, skipped, updated, failed, and warning counts.

## 9. API Extraction Plan

### 9.1 Required Shortcut Endpoints

Reference entities:

- `GET /member` - validate token and show authenticated Shortcut user/workspace.
- `GET /members` - member list and profile emails.
- `GET /workflows` - workflows and states.
- `GET /labels` - label names/colors.
- `GET /groups` - Shortcut teams/groups, if available in the workspace.
- `GET /projects` - project metadata and team association.
- `GET /iterations` - sprints.
- `GET /objectives` - objectives.
- `GET /epics` - epics.
- `GET /custom-fields` - optional v1 import metadata and diagnostics.

Stories:

- Prefer `GET /search?query=...&entity_types=story&detail=full&page_size=250`.
- Follow `next` pagination until exhausted.
- Partition search queries when a result set exceeds Shortcut's search result limit.
- Fallback for project-scoped workspaces: `GET /projects/{project-public-id}/stories`.
- For a precise story reload or retry: `GET /stories/{story-public-id}`.

Story children and relationships:

- `GET /stories/{story-public-id}/comments` when comments are not already present in full search payload.
- Story payload `tasks` for Shortcut story tasks/checklists.
- Story payload `story_links` for blocks, duplicates, and relates-to.
- Story payload `files` and `linked_files`.
- Story payload `external_links`.

### 9.2 Search Partitioning

The importer must not rely on a single unbounded search query. Use one or more partition strategies:

- Primary: partition by updated date ranges, using Shortcut search operators.
- Secondary: partition by workflow/project/group when date range still exceeds limits.
- Last resort: project story listing plus per-story detail fetch.

Each partition must record:

- Query string.
- Expected total if returned.
- Page count.
- `next` tokens consumed.
- Shortcut story IDs seen.

Deduplicate stories by Shortcut numeric `id` across partitions.

### 9.3 Rate Limiting And Retries

Client behavior:

- Global limiter at or below 200 requests/minute.
- Small burst allowed only if limiter still ensures minute cap.
- Retry transient 429/5xx/network failures with exponential backoff and jitter.
- Respect `Retry-After` when present.
- Do not retry 400/401/403/404/422 except where the entity is optional and can become a warning.
- Use request context cancellation.
- Cap response bodies in errors to prevent memory/log issues and token leaks.

### 9.4 Data Normalization

Build an in-memory or staged normalized graph before writing PM entities:

```text
ShortcutWorkspaceSnapshot
  Members
  Groups
  Projects
  Workflows
  WorkflowStates
  Labels
  Iterations
  Objectives
  Epics
  Stories
    Comments
    StoryTasks
    StoryLinks
    Files
    LinkedFiles
    ExternalLinks
```

All normalized objects carry:

- `shortcut_id`
- `entity_type`
- source timestamps
- source URL when available
- raw source payload hash
- warning list

## 10. Mapping Specification

### 10.1 IDs And Idempotency

Use deterministic external IDs:

| Shortcut entity | Helpin storage |
|---|---|
| Story `id` | `pm_tasks.external_id = shortcut:story:{id}` |
| Epic `id` | `pm_epics.external_id = shortcut:epic:{id}` |
| Objective `id` | `pm_objectives.external_id = shortcut:objective:{id}` |
| Iteration `id` | `pm_sprints.external_id = shortcut:iteration:{id}` |
| Label `id` | Add external ref row or match by name/color |
| Workflow `id` | Add external ref row or import mapping table |
| Workflow state `id` | Add external ref row or import mapping table |
| Story comment `id` | New external ref required |
| Story task `id` | `pm_checklist_items.external_id` if added, otherwise external ref |
| File `id` | New external ref required |
| Story link `id` | New external ref required |

Recommendation: add a generic `pm_import_external_refs` table instead of adding `external_id` columns to every model.

```sql
pm_import_external_refs(
  id uuid primary key,
  workspace_id uuid not null,
  import_job_id uuid,
  source text not null,
  source_entity_type text not null,
  source_entity_id text not null,
  target_entity_type text not null,
  target_entity_id uuid not null,
  source_hash text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique(workspace_id, source, source_entity_type, source_entity_id)
)
```

### 10.2 Story To Task

| Shortcut field | Helpin field |
|---|---|
| `id` | `external_id` / external ref |
| `name` | `pm_tasks.name` |
| `description` | `pm_tasks.description` after markdown/HTML normalization |
| `story_type` | `pm_tasks.task_type` |
| `workflow_id` | `pm_tasks.workflow_id` via mapping |
| `workflow_state_id` | `pm_tasks.workflow_state_id` via mapping |
| `epic_id` | `pm_tasks.epic_id` |
| `iteration_id` | `pm_tasks.sprint_id` |
| `group_id` | `pm_tasks.team_id` |
| `owner_ids[0]` | `pm_tasks.owner_id` / `owner_member_id` |
| `owner_ids` | `pm_task_owners` |
| `requested_by_id` | `pm_tasks.requester_id` / `requester_member_id` |
| `estimate` | `pm_tasks.estimate` |
| `deadline` | `pm_tasks.deadline` |
| `blocked` | `pm_tasks.blocked` |
| `blocker` | `pm_tasks.blocker` when textual reason exists |
| `archived` | `pm_tasks.archived` |
| `completed` | `pm_tasks.completed` |
| `started_at` / overrides | `pm_tasks.started_at` |
| `completed_at` / overrides | `pm_tasks.completed_at` |
| `moved_at` | `pm_tasks.moved_at` |
| `created_at` | `pm_tasks.created_at` |
| `updated_at` | `pm_tasks.updated_at` |
| `position` | `pm_tasks.position` |

Task type mapping:

- `feature` -> `feature`
- `bug` -> `bug`
- `chore` -> `chore`
- unknown/empty -> `feature` with warning

Priority/severity:

- Shortcut's API story payload does not expose the CSV-only priority/severity columns consistently. If those are custom fields, v1 should either map configured custom fields or store them in import notes. Do not invent priority/severity values.

### 10.3 Objectives

Shortcut objective -> Helpin objective:

- Name, description, archived, started/completed state, dates, position.
- Map Shortcut completed state to `closed`; started/in-progress to `active`; otherwise `not_started`.
- Link epics through `pm_epic_objectives`.

### 10.4 Epics

Shortcut epic -> Helpin epic:

- Name, description, archived, started/completed state, planned start, deadline, owner, labels, position.
- Epic state should map into existing Helpin epic workflow states. If no direct API state mapping exists, derive from Shortcut `started`/`completed` booleans.
- Link labels through `pm_epic_labels`.
- Link objectives through `pm_epic_objectives`.

### 10.5 Iterations

Shortcut iteration -> Helpin sprint:

- Name, description if available, start date, end date, status, labels, group/team association.
- Unlike CSV flow, API dates should be trusted over parsing names.
- Nullable start/end dates remain supported if Shortcut returns missing values.

### 10.6 Labels

Shortcut labels -> Helpin labels:

- Match existing Helpin labels by normalized name within workspace.
- Preserve Shortcut color when creating a new label.
- Store Shortcut label ID in external refs.
- Apply to stories, epics, objectives, and sprints where Helpin supports the join table.

### 10.7 Members

Shortcut member -> Helpin member mapping:

- Auto-match by lowercased email.
- If no email exists, match cannot be automatic.
- Disabled/imported Shortcut members can still be shown as source authors/owners but should not create active Helpin users automatically unless admin chooses invite-and-map.
- Comments or ownership from skipped members should use the importing actor as technical creator only where Helpin requires a non-null user, while preserving original author text in the comment body prefix or metadata.

### 10.8 Story Comments

Shortcut comment -> Helpin comment:

- Entity type: `task`.
- Entity ID: mapped Helpin task ID.
- Body: Shortcut comment text, converted to Helpin-compatible rich text/markdown format.
- Author: mapped Helpin user if available; otherwise importing actor with preserved attribution note.
- Parent comment: map Shortcut `parent_id` to Helpin `parent_id` after parent comment exists.
- Skip `deleted = true` comments by default; optionally preserve as tombstone text if product wants full audit history.
- Store external ref for idempotency.

### 10.9 Story Tasks / Checklist Items

Shortcut story `tasks` -> Helpin checklist items:

- Description -> checklist text.
- Complete -> completed.
- Owner IDs -> future enhancement unless Helpin checklist item ownership exists.
- Position -> checklist item position.
- Timestamps -> checklist item timestamps if model supports them; otherwise omit with warning.
- Add external refs or an `external_id` column for exact idempotency.

### 10.10 Story Links

Shortcut story links -> Helpin task links:

| Shortcut verb | Helpin link type | Direction |
|---|---|---|
| `blocks` | `blocks` | subject blocks object |
| `duplicates` | `duplicates` | subject duplicates object |
| `relates to` | `relates_to` | subject relates to object |

Only create links when both Shortcut stories were imported or already mapped in Helpin. Missing targets become warnings.

### 10.11 Files, Linked Files, And External Links

Files:

- If `import_files` is enabled, download Shortcut file URLs and upload to Helpin storage through the existing attachment path.
- Attach to task, epic, or comment based on source association when known.
- Preserve file name, content type, size, and uploader when possible.
- Avoid downloading files above configured max size; create an external link instead with a warning.

Linked files:

- Import as `pm_external_links` unless Helpin supports richer linked file models.
- Use linked file name/title and URL.

External links:

- Import story external links as `pm_external_links`.
- Add the Shortcut app URL for every imported task as an external link titled `Shortcut Story`.

### 10.12 Description And Link Rewriting

- Preserve Shortcut markdown where possible.
- Convert HTML fragments only when source content is HTML.
- Rewrite internal Shortcut story URLs to Helpin task URLs after all stories are mapped.
- Keep unknown Shortcut URLs as external links.
- Truncate only when Helpin storage constraints require it, and report the exact count.

## 11. Backend Design

### 11.1 Packages

Refactor toward the code-organization plan:

```text
server/internal/pm/import/
  service.go
  jobs.go
  mapping.go
  refs.go
  shortcut/
    client.go
    types.go
    extract.go
    partition.go
    normalize.go
    preview.go
    execute.go
    media.go
```

Transitional approach:

- Keep existing `PMImportService` API to avoid router and handler churn.
- Move Shortcut API internals behind a new package.
- Keep CSV parser only as fallback.

### 11.2 New/Changed Tables

Required:

- `pm_import_external_refs` for generic source-to-target mapping.
- `pm_import_warnings` for queryable warning records instead of only JSON blobs.
- `pm_import_snapshots` or object storage-backed snapshot metadata for normalized preview/execution handoff.

Recommended:

- `pm_import_job_events` for progress events and debug trace.
- `pm_checklist_items.external_id` if simpler than generic refs for checklist idempotency.

### 11.3 Import Job Lifecycle

Statuses:

- `pending`
- `scanning`
- `ready`
- `processing`
- `completed`
- `failed`
- `canceled`

Preview lifecycle:

1. Create import job with status `scanning`.
2. Validate token.
3. Extract reference data and story summaries/full payloads as needed.
4. Normalize snapshot.
5. Store snapshot metadata.
6. Return preview with `import_id`.
7. Mark job `ready`.

Execute lifecycle:

1. Load ready snapshot by `import_id`.
2. Validate mappings and options.
3. Start DB transaction per safe phase or use staged commit groups.
4. Upsert/match references.
5. Create entities in dependency order.
6. Rewrite links after all task IDs are known.
7. Mark completed or failed.

### 11.4 Dependency Order

1. Import job and snapshot.
2. Members/user mappings.
3. Groups/teams.
4. Workflows and workflow states.
5. Labels.
6. Objectives.
7. Epics.
8. Epic-objective links.
9. Iterations/sprints.
10. Stories/tasks.
11. Task owners/followers/labels.
12. Checklist items.
13. Comments and comment replies.
14. Task links/story relationships.
15. Files/attachments.
16. Linked files/external links.
17. Description/comment link rewrites.
18. Final report.

### 11.5 Transaction Strategy

Avoid one giant transaction for large API imports because file downloads and retries can be slow. Use this strategy:

- Extract/normalize outside write transactions.
- Write core PM entities in bounded DB transactions by phase.
- Every write must be idempotent through external refs and unique constraints.
- If a phase fails, rerunning execute should skip already-created refs and continue.
- Media import can run after core entities and record per-file failures as warnings unless configured as strict.

### 11.6 Status API

Existing endpoints can evolve:

```text
POST /api/workspaces/{workspace_id}/import/shortcut/api/preview
POST /api/workspaces/{workspace_id}/import/shortcut/api/{import_id}/execute
GET  /api/workspaces/{workspace_id}/import/shortcut/api/{import_id}
POST /api/workspaces/{workspace_id}/import/shortcut/api/{import_id}/cancel
```

Preview request:

```json
{
  "api_token": "secret",
  "options": {
    "include_archived": true,
    "include_completed": true,
    "include_comments": true,
    "include_files": true
  }
}
```

Preview response:

```json
{
  "import_id": "uuid",
  "status": "ready",
  "shortcut_workspace": {
    "name": "Acme",
    "url_slug": "acme"
  },
  "summary": {
    "stories": 1626,
    "epics": 107,
    "objectives": 17,
    "iterations": 74,
    "labels": 10,
    "members": 32,
    "comments": 940,
    "files": 84,
    "story_links": 41,
    "duplicates": 0
  },
  "users": [],
  "teams": [],
  "workflows": [],
  "warnings": []
}
```

Execute request:

```json
{
  "user_mappings": {
    "shortcut-member-uuid": "helpin-member-or-user-id"
  },
  "team_mappings": {
    "shortcut-group-uuid": "helpin-team-id"
  },
  "workflow_state_mappings": [],
  "options": {
    "import_archived": true,
    "import_completed": true,
    "import_comments": true,
    "import_files": true,
    "import_story_links": true,
    "rewrite_links": true
  }
}
```

### 11.7 Observability

Log with `slog`:

- `workspace_id`
- `import_id`
- `phase`
- `source_entity_type`
- counts
- duration
- request status for Shortcut calls

Never log:

- Shortcut token.
- Full descriptions/comments.
- Download URLs when they may contain secrets.

Metrics to add later:

- Import duration by phase.
- Shortcut requests count/status.
- Retry count.
- Warning/error count.
- Entities created/skipped.

## 12. Frontend Design

Reuse `ShortcutImportWizard` but change its state model from file-based to import-job-based.

Required UI changes:

- Remove CSV upload from the importer flow.
- Add token validation step.
- Start preview scan and poll status when scan is async.
- Use `import_id` for execute instead of resubmitting a file.
- Add team mapping UI.
- Add options for comments, files, story links, and linked files.
- Show API extraction warnings separately from write warnings.
- Do not show CSV fallback in the Shortcut importer.

## 13. Acceptance Criteria

Functional:

- Admin can validate a Shortcut API token.
- Admin can preview a Shortcut workspace without uploading CSV.
- Preview reports counts for all in-scope entities.
- Admin can map workflows/states, users, and teams.
- Admin can choose create-new or existing-team destination for every detected Shortcut team/group before import.
- Execute imports objectives, epics, iterations, labels, workflows/states, stories, comments, checklist items, story links, external links, and files according to selected options.
- Re-running the same import does not create duplicates.
- Re-running after changed team mappings remaps previously imported Shortcut stories to the selected destination team/workflow.
- Re-running after a failed phase continues safely or skips already imported refs.
- Imported Shortcut story URLs are stored as external links.
- Shortcut internal story links in descriptions/comments are rewritten when target tasks are imported.

Reliability:

- Honors 200 requests/minute limit.
- Retries transient API failures.
- Handles workspaces with more than 1000 stories via search partitioning.
- Shows actionable warnings for skipped optional data.
- Does not log or persist plaintext Shortcut tokens.

Data quality:

- Task names, descriptions, types, states, owners, requesters, epics, sprints, labels, dates, archived/completed flags, estimates, and positions match Shortcut where supported.
- Comments preserve author attribution and threading where possible.
- Story links preserve direction and verb.
- Files are either imported or represented as links with warnings.

## 14. Test Plan

Backend unit tests:

- Shortcut client auth/header handling.
- 429/5xx retry and `Retry-After`.
- Search pagination and `next` handling.
- Search partition deduplication.
- Normalizer mapping for every entity type.
- Workflow/state validation.
- User/team mapping validation.
- Idempotent external refs.
- Story link direction mapping.
- Comment parent mapping.
- File fallback behavior.

Backend integration tests:

- API preview using mocked Shortcut server.
- Execute full small fixture.
- Execute with duplicate rerun.
- Execute with partial prior refs.
- Execute with missing optional entities.
- Execute large partitioned fixture over 1000 stories.

Frontend tests:

- Token validation states.
- Async preview polling.
- Workflow mapping validation.
- Team/user mapping.
- Execute payload shape.
- Progress and final report rendering.

Manual QA:

- Import a real small Shortcut workspace.
- Import a real workspace with archived/completed stories.
- Import with comments/files off and on.
- Re-run same import.
- Force Shortcut 429 and 500 responses in a mock server.

## 15. Rollout

Phase 1: Backend API extractor and preview behind feature flag.

Phase 2: Execute core entities without files:

- members, teams, workflows, labels, objectives, epics, iterations, stories, checklist items, comments, story links, external links.

Phase 3: File import and link rewriting.

Phase 4: Frontend primary API wizard.

Phase 5: Remove CSV from the product importer UI.

Feature flag:

- `shortcut_api_import_enabled`

Operational controls:

- Max stories per import.
- Max file size.
- Max total file bytes.
- Strict vs best-effort media import.
- Import cancellation.

## 16. Open Questions

- Does Shortcut expose `GET /groups` in all workspaces we care about, and what exact ID type maps to story `group_id`?
- Should skipped/unmapped comment authors use importing actor with attribution text, or should Helpin support external/imported-only authors?
- Should deleted Shortcut comments be omitted or imported as tombstones?
- Should Shortcut custom fields become Helpin labels, metadata-only warnings, or future custom fields?
- Do we need to import Shortcut Docs in a later dedicated Docs importer?
- Should the import support update mode, or only create/skip mode in v1?
