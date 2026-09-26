# Shortcut API import progress tracker

**Date:** 2026-04-26
**PRD:** `docs/prds/shortcut-api-importer.md`
**Status:** Historical progress snapshot; see current source comparison below.

## Current source comparison — 2026-09-18

This tracker records April implementation progress for contributors. Its unchecked
boxes are not a reliable current backlog, and checked tests are historical results.

The [Shortcut client](../../server/internal/service/shortcut_api.go) still lives in
`service/`, rather than the proposed new package. It implements a per-client
200-request/minute limiter with burst 10, bounded response reads, retry delay handling,
and `POST /stories/search`. This configured limit is not a verified current vendor
quota or a global limit shared across all imports.

[API imports](../../server/internal/service/pm_import_shortcut_api.go) now persist
preview jobs, provide polling status, and can retain encrypted preview metadata.
Preview scans still run in a goroutine; durable import execution uses Temporal and
an encrypted payload. These are different lifecycle guarantees. The stored preview
snapshot omits full story payloads, so it is not an immutable full-data snapshot.

The [router](../../server/internal/router/router.go) gates imports with `pm.import`.
The actual execute path is `/api/workspaces/{id}/import/shortcut/api/execute`,
without an import-ID path segment. Cancellation and retry routes also exist.
The [wizard](../../frontend/src/components/pm/ShortcutImportWizard.tsx) has preview
polling as well as WebSocket progress, contrary to the unchecked polling item.
The proposed `shortcut_api_import_enabled` flag was not found in the router or
configuration; do not assume it protects rollout.

[Comment import](../../server/internal/service/pm_import.go) falls back to the
importing actor for unmapped authors and adds attribution when a source name is
available. [Media import](../../server/internal/service/shortcut_media_import.go)
has bounded download handling and storage-dependent behavior; the old unchecked
media tasks do not mean media support is absent. Successful transfer of every file
or a total-job byte guarantee is not established by this review.

Legacy CSV backend routes and code remain. The original large-workspace, real-data,
and publication checks below remain historical open questions unless separately
verified. No live Shortcut requests, imports, or runtime tests were run here.

## Legend

- `[ ]` Not started
- `[~]` In progress
- `[x]` Done
- `[!]` Blocked/risk

## Milestone 0: Decisions And Scope

- [x] Confirm Shortcut API import replaces CSV as the product path.
- [x] Remove CSV fallback from the Shortcut importer UI.
- [x] Confirm v1 import mode is create/skip for new entities, with placement remap for existing imported Shortcut stories.
- [ ] Confirm whether unmapped comment authors use importing actor plus attribution text.
- [ ] Confirm deleted Shortcut comments policy: skip or tombstone.
- [ ] Confirm file size and total media import limits.
- [ ] Confirm feature flag name: `shortcut_api_import_enabled`.

## Milestone 1: API Client

- [ ] Move Shortcut API code out of `server/internal/service/shortcut_api.go` into `server/internal/pm/import/shortcut/`.
- [ ] Add typed client for `GET /member`.
- [ ] Add typed client for `GET /members`.
- [ ] Add typed client for `GET /workflows`.
- [ ] Add typed client for `GET /labels`.
- [ ] Add typed client for `GET /groups` or document fallback if unavailable.
- [ ] Add typed client for `GET /projects`.
- [ ] Add typed client for `GET /iterations`.
- [ ] Add typed client for `GET /objectives`.
- [ ] Add typed client for `GET /epics`.
- [ ] Add typed client for `GET /custom-fields`.
- [ ] Add typed client for `GET /search` with `entity_types=story`.
- [ ] Add typed client for `GET /stories/{story-public-id}`.
- [ ] Add typed client for `GET /stories/{story-public-id}/comments`.
- [ ] Add rate limiter capped at 200 requests/minute.
- [ ] Add retry handling for 429/5xx/network errors.
- [ ] Respect `Retry-After` when present.
- [ ] Add response body caps for errors and normal payloads.
- [ ] Ensure token is never logged.

## Milestone 2: Extraction And Normalization

- [ ] Define Shortcut source DTOs for members, workflows, labels, groups, projects, iterations, objectives, epics, stories, comments, tasks, files, linked files, story links, and custom fields.
- [ ] Define normalized snapshot structs.
- [ ] Fetch reference data concurrently under the rate limiter.
- [ ] Implement story search with `page_size=250`.
- [ ] Follow `next` pagination.
- [ ] Implement search partitioning for more than 1000 results.
- [ ] Deduplicate stories by Shortcut story ID.
- [ ] Hydrate missing comments through story comment endpoint when needed.
- [ ] Normalize story checklist tasks.
- [ ] Normalize story links.
- [ ] Normalize files and linked files.
- [ ] Normalize internal Shortcut URLs discovered in descriptions/comments.
- [ ] Generate preview counts and warnings.

## Milestone 3: Persistence Model

- [ ] Create migration for `pm_import_external_refs`.
- [ ] Create migration for `pm_import_warnings`.
- [ ] Create migration for `pm_import_job_events` if chosen.
- [ ] Create snapshot persistence strategy: DB JSONB, object storage, or temp encrypted blob.
- [ ] Add repository helpers for external refs.
- [ ] Add repository helpers for warnings.
- [ ] Add unique constraint for source refs: workspace/source/entity type/entity ID.
- [ ] Add idempotent upsert helpers.
- [ ] Decide whether `pm_checklist_items.external_id` is needed or generic refs are enough.

## Milestone 4: Preview API

- [ ] Add route `POST /api/workspaces/{workspace_id}/import/shortcut/api/preview`.
- [ ] Validate workspace admin/owner access.
- [ ] Validate Shortcut token via `GET /member`.
- [ ] Start preview job with status `scanning`.
- [ ] Run extraction/normalization.
- [ ] Persist snapshot metadata.
- [ ] Return `import_id`, connected workspace, summary, users, teams, workflows, and warnings.
- [ ] Add status support for `scanning` and `ready`.
- [ ] Add tests for invalid token.
- [ ] Add tests for preview scan success.
- [ ] Add tests for preview scan API failure.

## Milestone 5: Mapping Validation

- [ ] Validate every Shortcut workflow maps to a Helpin workflow.
- [ ] Validate every imported Shortcut workflow state maps to a Helpin state.
- [ ] Validate created workflows have at least one `done` state.
- [ ] Validate Shortcut members map to valid Helpin members or explicit skip/invite choices.
- [x] Validate Shortcut groups map to valid Helpin teams or explicit create choices.
- [ ] Validate import options.
- [ ] Validate snapshot belongs to workspace and is `ready`.
- [ ] Add tests for all validation failures.

## Milestone 6: Execute Core Import

- [ ] Add route `POST /api/workspaces/{workspace_id}/import/shortcut/api/{import_id}/execute`.
- [ ] Load normalized snapshot.
- [x] Create/import teams.
- [x] Create/import workflows and workflow states, scoped per destination team for new workflows.
- [ ] Create/import labels.
- [ ] Create/import objectives.
- [ ] Create/import epics.
- [ ] Create epic-objective links.
- [ ] Create/import iterations as sprints.
- [ ] Create/import stories as tasks.
- [ ] Create task owners/followers/labels.
- [ ] Create checklist items.
- [ ] Create comments and replies.
- [ ] Create task links from Shortcut story links.
- [ ] Create external links.
- [ ] Store Shortcut app URL as an external link per task.
- [ ] Write external refs for every imported entity.
- [x] Update progress after each phase.
- [ ] Produce final result JSON.

## Milestone 7: Media Import

- [ ] Add file download service with size limits.
- [ ] Upload Shortcut files to Helpin storage.
- [ ] Attach imported files to tasks/comments/epics where supported.
- [ ] Import linked files as external links.
- [ ] Fall back to links for files above limit or failed downloads.
- [ ] Add per-file warnings.
- [ ] Add tests for success, too-large files, 404 files, and download timeout.

## Milestone 8: Link Rewriting

- [ ] Build Shortcut story ID to Helpin task URL map.
- [ ] Rewrite Shortcut story URLs in task descriptions.
- [ ] Rewrite Shortcut story URLs in comments.
- [ ] Leave unknown Shortcut URLs unchanged.
- [ ] Add warning count for unresolved links.
- [ ] Add tests for URL rewrite behavior.

## Milestone 9: Frontend Wizard

- [x] Change primary Shortcut wizard from CSV upload to API token connection.
- [ ] Add token validation/connected workspace UI.
- [x] Add live preview scan status over workspace WebSocket events.
- [ ] Add async preview scan polling.
- [ ] Replace file-backed state with `import_id`-backed state.
- [x] Keep existing workflow/state mapping UI, sourced from API preview.
- [x] Add team mapping UI.
- [ ] Update user mapping UI to use Shortcut member IDs plus emails.
- [x] Add story date-scope, epic date-scope, objective date-scope, archived/completed, and max-story controls.
- [ ] Add options for comments, files, linked files, story links, and link rewriting.
- [ ] Update execute call to use `import_id`.
- [x] Update progress labels and result summary.
- [x] Remove CSV flow from the Shortcut importer UI.
- [x] Add frontend service tests for API preview/execute payloads.

## Milestone 10: Reliability And Operations

- [ ] Add structured `slog` logging for import phases.
- [ ] Add cancellation endpoint if implementation supports it.
- [x] Add max stories safety limit.
- [ ] Add max file size and max total file bytes controls.
- [ ] Add strict vs best-effort media mode.
- [ ] Add admin-facing warning export or copyable final report.
- [ ] Add cleanup for old snapshots.
- [ ] Add feature flag checks.

## Milestone 11: Testing And QA

- [x] Unit test Shortcut client scoped story query behavior.
- [x] Unit test explicit Shortcut team mapping and team-scoped workflow creation.
- [x] Unit test rerun remaps existing imported Shortcut stories to the selected team/workflow.
- [ ] Unit test rate limit/retry behavior.
- [ ] Unit test search pagination.
- [ ] Unit test search partitioning over 1000 stories.
- [ ] Unit test normalizer mapping.
- [x] Integration test preview with mocked Shortcut server.
- [x] Integration test execute small fixture.
- [x] Integration test duplicate rerun.
- [ ] Integration test partial previous import refs.
- [x] Integration test comments.
- [x] Integration test story links.
- [x] Integration test external file-link fallback.
- [x] Frontend service tests.
- [ ] Manual QA with real small Shortcut workspace.
- [ ] Manual QA with archived/completed data.
- [ ] Manual QA with comments/files enabled.

## Milestone 12: Rollout

- [ ] Ship behind `shortcut_api_import_enabled`.
- [ ] Enable for internal workspace.
- [ ] Run one real migration dry run.
- [ ] Fix data-quality issues from dry run.
- [ ] Enable for selected customer workspace.
- [x] Keep Shortcut importer UI API-only.
- [ ] Update internal runbook.
- [ ] Update customer-facing import copy.
- [ ] Mark old CSV PRD as superseded by API PRD.

## Current Risks

- [~] Shortcut full-text search has a 1,000-result cap; current implementation avoids it by querying active/archived stories through `POST /stories/search` and fetching full details by ID, but this still needs a real large-workspace validation.
- [!] Large file imports can make jobs slow and expensive without strict limits.
- [!] Helpin comments require non-null authors; unmapped Shortcut authors need a deliberate attribution policy.
- [~] Exact Shortcut group/project/team semantics are implemented from `group_id` with `project.team_id` fallback, but still need verification against a real large workspace payload.
- [~] Legacy CSV importer code remains backend-compatible, but it is no longer exposed in the Shortcut importer UI.

## Implementation Notes

- Prefer adding the generic `pm_import_external_refs` table before expanding model-specific `external_id` columns.
- Keep extraction pure and testable. The API extractor should produce a normalized snapshot without writing PM domain tables.
- Keep media import after core entity import so failed file downloads do not invalidate the whole migration.
- Keep legacy CSV backend code only as compatibility/reference unless a separate removal task is scheduled.
- Review follow-up on 2026-04-27 replaced the fragile `/search` multi-entity scan with `POST /stories/search` plus `GET /stories/{id}` detail fetches, reused full-story comments, and backfilled API external links for already-imported stories.
