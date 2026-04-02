# Story to Task Hard-Cut Progress Tracker

> **Tracking rule:** update this file as work lands. Keep statuses limited to `Pending`, `In Progress`, `Blocked`, or `Done`.

## Overall Status

| Workstream | Status | Notes |
| --- | --- | --- |
| Schema and migrations | In Progress | Forward hard-cut migrations `202604010002` and `202604010003` now exist; live verification exposed and fixed a dual-schema drift case where empty task tables already existed before the rename |
| Data backfill rewrites | In Progress | Core entity/preset rewrites are in the migration; automation/prompt payload sweep still pending |
| Backend contracts | In Progress | `/pm/tasks` and `/pm/task-templates` route slice landed; task-era model/repository/service/handler aliases and DI/router field wiring are now in place, but deeper internal renames are still pending |
| Agent and automation surfaces | In Progress | `task_planner` preset family and default target-type slice landed; canonical internal command names are now task-first (`pm.create_task_batch`, `pm.update_task_state`, `pm.assign_task_agent`, `pm.set_task_dependencies`, `docs.ensure_task_plan_doc`), task-era planner prompts/approval phases are partially landed, and runtime migration verification is clean; deeper worker/model/service internals still remain |
| Frontend routes and PM UI | In Progress | Canonical `/pm/tasks` routes now include the renamed `$taskId` detail route, `task-templates`, `pmTaskService`, `pmTaskLinks`, task-era query keys, `useTasks`, task association service methods, task PM comment/attachment entity types, task aliases in shared PM types, task-era board/store type usage, `taskRouteNavigation`, `taskPanelStore`, `?task=` URL state, task DOM events, `CreateTaskModal`, `GlobalTaskPanel`, `TaskDetailPanel`, `TaskListView`, `TaskCard`, `TaskDeliveryPanel`, `TaskGitPanel`, `TaskRelationshipsSection`, `TaskSidebarIdRow`, the full `task-detail/` utility directory, recurring-task summary/settings task copy, and `SprintPlanningTaskCard`; broader Story->Task component/store/type rename still pending |
| Cross-product integrations | In Progress | Notification links, docs/support/search visible route/copy slice landed; CRM/docs/support task object-type compatibility and task-aware link/create flows are partially landed, with deeper backend object-type rewrites still pending |
| Documentation | Pending | CLAUDE.md, AGENTS.md, internal docs |
| Validation and rollout | In Progress | Forward and rollback SQL exist, live environment verification completed, reconciliation migration fixed the dual-schema case, `frontend` and `server` builds are green, and `migrate status`/`migrate up` are clean in the configured environment; staging rehearsal and deployment cutover rules still remain |

## Checklist

### 0. Migration runner foundation

- [x] Add `server/cmd/migrate` runner binary
- [x] Add `server/internal/dbmigrate` runtime migration package with `schema_migrations`, embedded SQL loading, checksums, and advisory lock
- [x] Add bootstrap runtime migration SQL file under `server/internal/dbmigrate/sql/`
- [x] Build `./migrate` into the server image
- [x] Add ArgoCD `PreSync` migration Job manifests in `k8s/stage/server-migrate.yaml` and `k8s/prod/server-migrate.yaml`
- [x] Add `RUN_AUTO_MIGRATE` flag support to API startup
- [x] Change the stage/prod migration Jobs from bootstrap-safe fallback mode to strict `./migrate up` mode now that the deployment image contains the migration binary

### 1. Schema and migrations

- [x] Add the Story -> Task forward rename SQL file(s) under `server/internal/dbmigrate/sql/`
- [x] Create the SQL migration set for table renames (`pm_stories` -> `pm_tasks`, related join/template/link tables)
- [x] Rename FK columns (`story_id` -> `task_id`, `linked_story_id` -> `linked_task_id`, `active_story_id` -> `active_task_id`, recurring lineage columns, etc.)
- [x] Rename `story_type` to `task_type`
- [x] Rename `workspace_teams.default_story_type` to `default_task_type`
- [x] Rename sequence `pm_story_display_id_seq` -> `pm_task_display_id_seq` and re-point column default
- [x] Add reconciliation migration for environments where task-era tables were auto-created before the rename and the guarded table renames therefore skipped live story data
- [ ] Rename or recreate indexes, constraints, and FK names for task-era clarity
- [x] Update `pm_comments.entity_type` CHECK constraint from `('story', 'epic', 'doc')` to `('task', 'epic', 'doc')`
- [ ] Rehearse migration on staging data with existing story records
- [x] Confirm historical migration files (013, 014, 017, 020, 024, 038, 040, 042, 043, 044, 045, 057) are left as-is

### 1b. Data backfill rewrites

- [x] `pm_comments` rows: `entity_type = 'story'` -> `'task'`
- [x] `notifications` rows: `entity_type = 'story'` -> `'task'`
- [x] Follower/association rows: `object_type = 'story'` -> `'task'`
- [x] `agents.allowed_targets` JSONB: `"story"` -> `"task"`
- [ ] Automation trigger strings: `story.*` -> `task.*`
- [x] Preset keys: `story_planner` -> `task_planner`
- [ ] Rewrite any other persisted JSONB/text values containing `story` -> `task`

### 2. Backend contracts

- [ ] Rename backend PM work-item models and request/response DTOs from Story to Task
  Current state: task-era alias types now exist in `server/internal/model/pm_task_aliases.go`; underlying canonical structs are still story-era.
- [ ] Rename handlers, services, repositories, and DI wiring in `cmd/api/main.go` to task-era names
  Current state: task-era repository/service/handler aliases landed and API/router wiring now uses `PMTask*` fields and constructors, but the underlying implementation files are still `pm_story*`.
- [x] Move PM routes from `/pm/stories/...` to `/pm/tasks/...`
- [x] Move template routes from `/pm/story-templates` to `/pm/task-templates`
- [x] Move relationship routes from `/pm/story-relationships/{id}` to `/pm/task-relationships/{id}`
- [ ] Rename sprint/epic statistics fields (`story_count` -> `task_count`, `done_story_count` -> `done_task_count`)
- [ ] Rename filter parameter `story_type` -> `task_type` in query params and filter structs
- [ ] Rename notification event names and entity types to `task.*` / `task`
- [ ] Rename websocket entity and parent type usage from `story` to `task`
- [ ] Update support/CRM/docs association handlers and generic entity references
- [ ] Update backend tests to task-era naming and contracts

### 3. Agent and automation surfaces

- [x] Rename preset key `story_planner` to `task_planner`
- [x] Rename model constant `AgentPresetStoryPlanner` -> `AgentPresetTaskPlanner` in `model/agent.go`
- [x] Rename preset/UI label to `Task Planner`
- [ ] Rename agent target type `story` to `task`
- [ ] Rename agent-run task/story linkage columns and fields to task equivalents
- [ ] Rename worker execution context: `execCtx.Story` -> `execCtx.Task`, `execCtx.StoryID` -> `execCtx.TaskID` in `worker/eino_executor.go`
- [ ] Rename webhook event `"pm.story_completion_followups"` -> `"pm.task_completion_followups"`
- [ ] Update `BuildSystemPrompt()` to receive Task object instead of Story
- [ ] Update `repository/agent_schema.go` inline SQL enum value `'story'` -> `'task'`
- [ ] Update planner schemas and payload keys (`proposed_tasks`, `task_refs`, etc.)
  Current state: live planner preview validation now accepts canonical `proposed_tasks` while preserving legacy `proposed_stories`.
- [ ] Update system prompts, tool descriptions, inventory text, and internal command registry wording
  Current state: task-first command names, prompt text, approval phases (`tasks`, `task_doc`), and Temporal planner summaries landed for the active runtime path; deeper compatibility names remain in worker/model internals and tests.
- [ ] Migrate existing system/workspace agent rows to task-era preset keys and target types

### 4. Frontend PM surfaces

#### 4a. Types and query infrastructure
- [ ] Rename all Story types in `pmTypes.ts` (Story, StoryDetail, StoryRecurringSummary, StoryType, StoryStateColumn, StoryMemberColumn, StoryGroup, StoryTemplate, CreateStoryRequest, UpdateStoryRequest, CreateStoryTemplateRequest, UpdateStoryTemplateRequest, CreateStoryRelationshipRequest, StoryUserLinkRequest, StoryLabelLinkRequest)
  Current state: task-era aliases now exist for the main task DTOs and board shapes, and consuming services/stores have started switching to them.
- [ ] Rename `crmTypes.ts` CRMObjectType `'story'` literal -> `'task'`
- [ ] Rename `types.ts` fields: `story_type` feature flag boolean, `default_story_type`
- [x] Rename all 11 story-prefixed query keys in `queryKeys.ts` and their `'stories'` path segments
- [x] Rename `useStories.ts` -> `useTasks.ts` (14 hooks)
- [ ] Rename story hooks in `useAssociations.ts` (useStoryAssociations, useCreateStoryRelationship, useDeleteStoryRelationship, `'story'` object type checks)
  Current state: query hook names and service method names are task-era; object-type literals still need follow-through.
- [x] Rename `useComments.ts` entity type literal `'story'` -> `'task'`

#### 4b. Services
- [x] Rename `pmStoryService.ts` -> `pmTaskService.ts` (CRUD, owners, labels, followers)
- [x] Rename `pmStoryTemplateService.ts` -> `pmTaskTemplateService.ts` (endpoint paths `/pm/story-templates`)
- [x] Rename `associationsService.ts` methods (`.listByStory()`, `.createStoryRelationship()`, `.deleteStoryRelationship()`) and endpoint path `/pm/story-relationships/`

#### 4c. Stores
- [x] Rename `storyPanelStore.ts` -> `taskPanelStore.ts` (storyId, lastClosedStoryId, openStory, closeStory, etc.)
- [ ] Rename story functions in `pmBoardStore.ts` (createStory, moveStory, patchStory, moveMemberStory, filter/sort)
  Current state: board/store type imports and board column/task shapes are task-era; method/function names remain story-era.
- [ ] Rename story references in `boardDisplayStore.ts` and `globalCreateStore.ts`

#### 4d. Routes and navigation
- [x] Rename canonical PM routes to `/pm/tasks` and `/pm/tasks/:id`
- [x] Rename task detail route parameter `$storyId` -> `$taskId` and sync `routeTree.gen.ts`
- [x] Rename settings route `story-templates` -> `task-templates` and `StoryTemplatesSettingsPage.tsx`
- [x] Rename `pmStoryLinks.ts` -> `pmTaskLinks.ts` (buildTaskPath, buildTaskUrl, buildTaskCopyUrl)
- [x] Rename `storyRouteNavigation.ts` -> `taskRouteNavigation.ts`
- [x] Rename URL query parameter `?story=<id>` -> `?task=<id>` (panel state + window.history)
- [x] Rename `url.searchParams.delete('story')` -> `'task'`

#### 4e. Components (44+ files)
- [ ] Rename core panels: `StoryDetailPanel.tsx`, `GlobalStoryPanel.tsx` (entity: `'story'`), `StoryCard.tsx`
  Current state: `TaskDetailPanel.tsx`, `GlobalTaskPanel.tsx`, and `TaskCard.tsx` landed.
- [ ] Rename feature panels: `StoryDeliveryPanel.tsx`, `StoryGitPanel.tsx`, `StoryRelationshipsSection.tsx`, `StoryListView.tsx`
  Current state: `TaskListView.tsx`, `TaskDeliveryPanel.tsx`, `TaskGitPanel.tsx`, and `TaskRelationshipsSection.tsx` landed.
- [x] Rename modals: `CreateStoryModal.tsx` (mode: `'story'` -> `'task'`)
- [ ] Rename filters: `StoryFilters.tsx` (story_type filter field), `StoryCard.sortable.ts`
  Current state: `TaskCard.sortable.ts` landed; `StoryFilters.tsx` remains.
- [ ] Rename sprint: `SprintPlanningStoryCard.tsx`
  Current state: `SprintPlanningTaskCard.tsx` landed.
- [x] Rename settings: `StoryTemplatesSettings.tsx`, `RecurringTemplateList.tsx` (lastGeneratedStory)
- [x] Rename `StorySidebarIdRow.tsx`
- [ ] Rename sidebar config in `layout/sidebar/config.ts`: `{ key: 'story', label: 'Story', pages: ['stories'] }` -> task equivalents
- [x] Rename `story-detail/` directory and all 13 utility files (storyFilterMembers, storyOverlayDismiss, storyLabelSync, storyPendingPatch, storyListGrouping, storyOverlayState, storyPlanningScope, storyListPinnedOffsets, storyDetailEventPayload, StoryStateSelectContent, StoryRouteFallbackBackground, etc.)
- [x] Rename `RecurringTemplatesSettings.tsx` openStoryRoute reference

#### 4f. DOM events
- [x] Rename DOM custom events: `story-created`, `story-panel-updated`, `story-panel-archived` -> task equivalents

#### 4g. Tests (24+ files)
- [x] Rename `__tests__/CreateStoryModal.test.tsx`, `StoryCard.sortable.test.tsx`
- [x] Rename `story-detail/__tests__/` (13 test files matching utility modules)
- [x] Rename `lib/__tests__/pmStoryLinks.test.ts`
- [ ] Rename `stores/__tests__/pmBoardStore.test.ts`

#### 4h. Cross-module UI
- [ ] Update PM pages: board, list, epic, sprint, my-work flows to Task naming
- [x] Update PM list/detail route callers and recurring-task summary/settings call sites to task-era prop names where those routes/components are now task-first
- [x] Update task template and recurring task surfaces

### 5. Cross-product integrations

- [ ] Rename support linked-story contracts and UI to task equivalents
  Current state: task-aware support link/create flows landed in the sidebar and service layer, but canonical API/contracts remain story-era.
- [ ] Rename CRM association object type `story` to `task` (CRM `AssociationsList.tsx`, `EmailAccountConnect.tsx`)
  Current state: CRM UI now accepts task/story object types and labels task-facing UI accordingly.
- [ ] Rename docs linked-object type `story` to `task`
  Current state: docs frontend types and link panel now accept/create `task`, with backend docs-link object-type storage still pending.
- [x] Update notifications UI and routing to open task detail routes
- [x] Update search and shared-link helpers from story paths to task paths

### 6. Documentation

- [ ] Update `CLAUDE.md` (root) — logging examples (`"story created"`, `"story_id"`), route references (`/pm/stories`)
- [ ] Update `server/CLAUDE.md` — `/stories` route definition, story creation log examples
- [ ] Update `AGENTS.md` — story references in log output examples
- [ ] Update `docs/prd-shortcut-importer.md` — Shortcut import mappings referencing `pm_stories`, story type/team counts
- [ ] Update `docs/PRD-stories-scale-and-performance.md` — performance requirements terminology
- [ ] Update any internal PRDs or plan docs that describe story as the current product term

### 7. Validation and rollout

- [x] Write reverse migration SQL script (tables/columns back to story-era names)
- [ ] Add migration verification test coverage for seeded existing story data
- [x] Dry-run the forward SQL against the configured database inside a transaction and fix current-schema mismatches before merge
- [x] Dry-run the forward + reverse SQL together inside a single transaction to validate rollback shape
- [ ] Run `./migrate status` in staging after applying the Story -> Task SQL and confirm the version is recorded in `schema_migrations`
- [x] Run `./migrate status` and `./migrate up` against the configured environment and verify all three runtime migrations are recorded in `schema_migrations`
- [x] Verify live database ended with task-era tables populated and legacy story-era tables removed after reconciliation
- [x] Re-run `./migrate status` and `./migrate up` after the latest task-era runtime rename pass to confirm the environment stays clean and migrations remain idempotent
- [ ] Verify end-to-end task CRUD and board flows against migrated data
- [ ] Verify agent assignment/run and automation triggers against `task`
- [ ] Verify no primary-contract `/pm/stories`, `story_id`, `story_type`, `story_planner`, or `story.*` identifiers remain
- [x] Keep `frontend` and `server` build-green while the manual rename lands (`npm run build`, `GOCACHE=/tmp/go-build go build ./cmd/...`)
- [ ] Prepare release notes for the hard cutover and bookmark/API breakage
- [ ] Add temporary 301 redirect from `/pm/stories/*` -> `/pm/tasks/*` (remove after one release cycle)
- [ ] Take database snapshot/backup immediately before production migration
- [ ] Execute production rollout with migration-first deployment sequence (`PreSync` Job runs `./migrate up`, then task-era pods deploy with `RUN_AUTO_MIGRATE=false`)

## Open Risks To Watch

- Hard cutover means bookmarked `/pm/stories/...` URLs and any external API consumers on old paths will break immediately. Mitigated by temporary 301 redirect.
- Persisted JSONB/text rewrites are easy to miss and must be enumerated carefully before rollout. Now tracked separately in section 1b.
- Agent/runtime rows and automation rules store story-era identifiers in multiple places and need explicit migration coverage.
- `AutoMigrate` must not be treated as sufficient for the rename; schema and data rewrites need explicit SQL.
- `pm_story_display_id_seq` sequence is not cascaded by `ALTER TABLE RENAME` — requires explicit rename and column default re-point.
- `pm_comments.entity_type` CHECK constraint is hardcoded — must be altered, not just data-backfilled.
- The forward SQL migration is now executable, so merging it before task-era application code is ready would cause the ArgoCD `PreSync` hook to attempt the hard cutover early.
- No rollback path without a pre-tested reverse migration script and database snapshot.
- The rollback SQL now exists, but it is manual-only and still needs a real staged rollback rehearsal against a task-era deployment before production use.
- Worker execution context (`eino_executor.go`) passes Story object to LLM prompt builder — a missed rename here breaks all agent runs silently.
- Frontend still has a large Story-era surface in type names, stores, modal/component names, and DOM events even though the canonical task routes are now working.
- `repository/agent_schema.go` contains inline SQL with `'story'` enum — easy to miss as it's not in the migrations directory.

## File Impact Summary

| Layer | Files to modify | Key risk areas |
| --- | --- | --- |
| SQL migrations (new) | 1 migration file | Sequence rename, CHECK constraints, data backfills |
| Backend Go | ~33 files | Worker execution context, agent schema, DI wiring |
| Frontend TS/TSX | ~80+ files | 3,000+ references, 24 test files, 13 utility modules |
| Documentation | ~6 files | CLAUDE.md, AGENTS.md, PRDs |
