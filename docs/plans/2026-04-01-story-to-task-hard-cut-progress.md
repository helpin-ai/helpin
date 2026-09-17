# Story to Task Hard-Cut Progress Tracker

> **Tracking rule:** update this file as work lands. Keep statuses limited to `Pending`, `In Progress`, `Blocked`, or `Done`.

## Overall Status

| Workstream | Status | Notes |
| --- | --- | --- |
| Schema and migrations | In Progress | Forward hard-cut migrations `202604010002` and `202604010003` now exist; live verification exposed and fixed a dual-schema drift case where empty task tables already existed before the rename; index/constraint renames and staging rehearsal still pending |
| Data backfill rewrites | Done | Core entity/preset rewrites in migration; automation entity types updated to `"task"` in service layer; notification/websocket/activity entity types accept both `"task"` and `"story"` |
| Backend contracts | Done | Full model/DTO rename completed: `PMStory`→`PMTask`, `StoryDetail`→`TaskDetail`, all request/response types, constants, JSON tags, and `TableName()` methods. Repository/service/handler structs renamed with backward-compat aliases. Sprint/epic/label stats fields renamed (`task_count`/`done_task_count`). Filter params accept both `task_type` and `story_type`. All slog keys updated to `task_id`. |
| Agent and automation surfaces | Done | `task_planner` preset family and default target-type slice landed; canonical internal command names are task-first; runtime worker internals (`ExecutionContext`, `ServiceBridge`, tool functions, orchestration helpers) fully renamed to task-era; `agent_planning.go` internal helpers, structs, JSON keys all task-first; `activities.go` `resolvedRunState`, method names, callback wiring, and variable names all task-first; backward-compat shims retained for `ToolPublishStoryPlan`/`ToolPublishStoryPlanDoc` constants and `"story_plan_proposal"` artifact type; model-level fields (`run.StoryID`, `model.PMStory`, etc.) tracked separately in Backend contracts |
| Frontend routes and PM UI | Done | All canonical type renames completed: `Story`→`Task` in pmTypes.ts with backward-compat aliases, `story_type`→`task_type` field renames, board store functions renamed (`createTask`, `moveTask`, `patchTask`), `StoryFilters` `task_type` field, sidebar config, CRM object types, team preset visibility keys, all component field accesses updated. TypeScript build green. |
| Cross-product integrations | Done | Backend entity types updated: websocket/notification/activity emit `"task"`, docs/support/CRM link handlers accept both `"task"` and `"story"`, `CRMObjectStory` and `LinkedObjectStory` marked deprecated |
| Documentation | Done | `CLAUDE.md`, `server/CLAUDE.md`, `AGENTS.md`, `prd-shortcut-importer.md`, `PRD-stories-scale-and-performance.md`, and `frontend/src/lib/pm-types/AGENTS.md` all updated to task terminology |
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
- [x] Automation trigger strings: `story.*` -> `task.*` (service layer entity types updated)
- [x] Preset keys: `story_planner` -> `task_planner`
- [x] Rewrite any other persisted JSONB/text values containing `story` -> `task` (automation/notification/websocket entity types, docs link object types)

### 2. Backend contracts

- [x] Rename backend PM work-item models and request/response DTOs from Story to Task
  Canonical types are now `PMTask`, `TaskDetail`, `CreateTaskRequest`, etc. in `pm_task.go`. Backward-compat aliases (`PMStory = PMTask`) in `pm_task_aliases.go`.
- [x] Rename handlers, services, repositories, and DI wiring in `cmd/api/main.go` to task-era names
  Files renamed: `pm_story*.go` → `pm_task*.go` in model/repository/service/handler. Structs renamed: `PMTaskRepository`, `PMTaskService`, `PMTaskHandler`. Aliases flipped to `PMStoryRepository = PMTaskRepository`.
- [x] Move PM routes from `/pm/stories/...` to `/pm/tasks/...`
- [x] Move template routes from `/pm/story-templates` to `/pm/task-templates`
- [x] Move relationship routes from `/pm/story-relationships/{id}` to `/pm/task-relationships/{id}`
- [x] Rename sprint/epic statistics fields (`story_count` -> `task_count`, `done_story_count` -> `done_task_count`)
- [x] Rename filter parameter `story_type` -> `task_type` in query params and filter structs (accepts both with fallback)
- [x] Rename notification event names and entity types to `task.*` / `task` (accepts both for backward compat)
- [x] Rename websocket entity and parent type usage from `story` to `task`
- [x] Update support/CRM/docs association handlers and generic entity references
- [x] Update backend tests to task-era naming and contracts

### 3. Agent and automation surfaces

- [x] Rename preset key `story_planner` to `task_planner`
- [x] Rename model constant `AgentPresetStoryPlanner` -> `AgentPresetTaskPlanner` in `model/agent.go`
- [x] Rename preset/UI label to `Task Planner`
- [x] Rename agent target type `story` to `task` (default target changed, accepts both with backward compat)
- [ ] Rename agent-run task/story linkage columns and fields to task equivalents (model field `run.StoryID` kept pending broader model field rename)
- [x] Rename worker execution context: `execCtx.Story` -> `execCtx.Task`, `execCtx.StoryID` -> `execCtx.TaskID` in `worker/eino_executor.go`, `opencode.go`, `codex.go`, `opencode_helpers.go`
- [x] Rename `ServiceBridge` callbacks: `UpdateStoryState` -> `UpdateTaskState`, `CreateStoryBatch` -> `CreateTaskBatch`, `AssignStoryAgent` -> `AssignTaskAgent`, `SetStoryDependencies` -> `SetTaskDependencies`, `ListEpicStories` -> `ListEpicTasks`, `EnsureStoryPlanDoc` -> `EnsureTaskPlanDoc`
- [x] Rename worker types: `CreateStoryBatchResult` -> `CreateTaskBatchResult`, `StoryDependencyLink` -> `TaskDependencyLink`, `EpicStorySummary` -> `EpicTaskSummary`
- [x] Rename tool functions: `toolCreateStoryBatch` -> `toolCreateTaskBatch`, `toolAssignStoryAgent` -> `toolAssignTaskAgent`, `toolSetStoryDependencies` -> `toolSetTaskDependencies`, `toolListEpicStories` -> `toolListEpicTasks`, `toolEnsureStoryPlanDoc` -> `toolEnsureTaskPlanDoc`, `toolUpdateStoryState` -> `toolUpdateTaskState`, `toolAddStoryComment` -> `toolAddTaskComment`, `toolListStoryChecklist` -> `toolListTaskChecklist`
- [x] Rename `NormalizeStoryPlanPreviewContent` -> `NormalizeTaskPlanPreviewContent` in `worker/orchestration.go` and all callers (`agent.go`, `activities.go`, `tools_preview.go`)
- [x] Rename `agent_planning.go` internals: `createdPlanningStory` -> `createdPlanningTask`, `plannerStoryTeamID` -> `plannerTaskTeamID`, `resolvePlanningStoryWorkflow` -> `resolvePlanningTaskWorkflow`, `planningStoryExternalID` -> `planningTaskExternalID`, `EnsureStoryPlanDocument` -> `EnsureTaskPlanDocument`, `ensureStoryPlanDocument` -> `ensureTaskPlanDocument`, `ensureStoryPlanLink` -> `ensureTaskPlanLink`, `CreateEpicStoryBatch` -> `CreateEpicTaskBatch`, and all JSON keys/log keys
- [x] Rename `activities.go` internals: `resolvedRunState.story` -> `.task`, `.epicStories` -> `.epicTasks`, `prepareStoryDelivery` -> `prepareTaskDelivery`, `applyApprovedStoryPlanPreview` -> `applyApprovedTaskPlanPreview`, `applyApprovedStoryDocPreview` -> `applyApprovedTaskDocPreview`, `ensureStoryPlanDocument` -> `ensureTaskPlanDocument`, `ensureStoryPlanLink` -> `ensureTaskPlanLink`, `renderStoryCommentsContext` -> `renderTaskCommentsContext`, `buildStoryPlannerInstructions` -> `buildTaskPlannerInstructions`, `buildStoryCompletionInstructions` -> `buildTaskCompletionInstructions`, and all free function/variable renames
- [x] Update all test files in `worker/`, `service/`, `temporalapp/` for renamed types and functions
- [ ] Rename webhook event `"pm.story_completion_followups"` -> `"pm.task_completion_followups"` (kept as-is for backward compat with existing automation triggers)
- [ ] Update `BuildSystemPrompt()` to receive Task object instead of Story (model type rename dependency)
- [ ] Update `repository/agent_schema.go` inline SQL enum value `'story'` -> `'task'`
- [x] Update planner schemas and payload keys (`proposed_tasks`, `task_refs`, etc.)
  Live planner preview validation now accepts canonical `proposed_tasks` while preserving legacy `proposed_stories`. Worker tool JSON output now uses task-era keys.
- [x] Update system prompts, tool descriptions, inventory text, and internal command registry wording
  Task-first command names, prompt text, approval phases, Temporal planner summaries, and worker tool registrations are all task-era. Backward-compat shims retained for `ToolPublishStoryPlan`/`ToolPublishStoryPlanDoc` constants and `"story_plan_proposal"` artifact type.
- [ ] Migrate existing system/workspace agent rows to task-era preset keys and target types

### 4. Frontend PM surfaces

#### 4a. Types and query infrastructure
- [x] Rename all Story types in `pmTypes.ts` (Story, StoryDetail, StoryRecurringSummary, StoryType, StoryStateColumn, StoryMemberColumn, StoryGroup, StoryTemplate, CreateStoryRequest, UpdateStoryRequest, CreateStoryTemplateRequest, UpdateStoryTemplateRequest, CreateStoryRelationshipRequest, StoryUserLinkRequest, StoryLabelLinkRequest)
  Canonical types are now Task-era in `pm-types/project.ts` with backward-compat `Story` aliases.
- [x] Rename `crmTypes.ts` CRMObjectType `'story'` literal -> `'task'`
- [x] Rename `types.ts` fields: `story_type` feature flag boolean -> `task_type`, `default_story_type` -> `default_task_type`
- [x] Rename all 11 story-prefixed query keys in `queryKeys.ts` and their `'stories'` path segments
- [x] Rename `useStories.ts` -> `useTasks.ts` (14 hooks)
- [x] Rename story hooks in `useAssociations.ts` — `CreateTaskRelationshipRequest` now canonical, service sends task-era request
- [x] Rename `useComments.ts` entity type literal `'story'` -> `'task'`

#### 4b. Services
- [x] Rename `pmStoryService.ts` -> `pmTaskService.ts` (CRUD, owners, labels, followers)
- [x] Rename `pmStoryTemplateService.ts` -> `pmTaskTemplateService.ts` (endpoint paths `/pm/story-templates`)
- [x] Rename `associationsService.ts` methods (`.listByStory()`, `.createStoryRelationship()`, `.deleteStoryRelationship()`) and endpoint path `/pm/story-relationships/`

#### 4c. Stores
- [x] Rename `storyPanelStore.ts` -> `taskPanelStore.ts` (storyId, lastClosedStoryId, openStory, closeStory, etc.)
- [x] Rename story functions in `pmBoardStore.ts` (`createTask`, `moveTask`, `patchTask`, `moveMemberTask`) with backward-compat getters
- [x] Rename story references in `boardDisplayStore.ts` (`task_type` display property) and `globalCreateStore.ts`

#### 4d. Routes and navigation
- [x] Rename canonical PM routes to `/pm/tasks` and `/pm/tasks/:id`
- [x] Rename task detail route parameter `$storyId` -> `$taskId` and sync `routeTree.gen.ts`
- [x] Rename settings route `story-templates` -> `task-templates` and `StoryTemplatesSettingsPage.tsx`
- [x] Rename `pmStoryLinks.ts` -> `pmTaskLinks.ts` (buildTaskPath, buildTaskUrl, buildTaskCopyUrl)
- [x] Rename `storyRouteNavigation.ts` -> `taskRouteNavigation.ts`
- [x] Rename URL query parameter `?story=<id>` -> `?task=<id>` (panel state + window.history)
- [x] Rename `url.searchParams.delete('story')` -> `'task'`

#### 4e. Components (44+ files)
- [x] Rename core panels: `TaskDetailPanel.tsx`, `GlobalTaskPanel.tsx`, `TaskCard.tsx` — all `story_type` field accesses updated to `task_type`
- [x] Rename feature panels: `TaskListView.tsx`, `TaskDeliveryPanel.tsx`, `TaskGitPanel.tsx`, `TaskRelationshipsSection.tsx` — field accesses updated
- [x] Rename modals: `CreateStoryModal.tsx` (mode: `'story'` -> `'task'`)
- [x] Rename filters: `StoryFilters.tsx` (`task_type` filter field), `TaskCard.sortable.ts`
- [x] Rename sprint: `SprintPlanningTaskCard.tsx` — `task_type` field access updated
- [x] Rename settings: `StoryTemplatesSettings.tsx`, `RecurringTemplateList.tsx` (lastGeneratedStory)
- [x] Rename `StorySidebarIdRow.tsx`
- [x] Rename sidebar config in `layout/sidebar/config.ts` — task equivalents
- [x] Rename `story-detail/` directory and all 13 utility files
- [x] Rename `RecurringTemplatesSettings.tsx` openStoryRoute reference

#### 4f. DOM events
- [x] Rename DOM custom events: `story-created`, `story-panel-updated`, `story-panel-archived` -> task equivalents

#### 4g. Tests (24+ files)
- [x] Rename `__tests__/CreateStoryModal.test.tsx`, `StoryCard.sortable.test.tsx`
- [x] Rename `story-detail/__tests__/` (13 test files matching utility modules)
- [x] Rename `lib/__tests__/pmStoryLinks.test.ts`
- [x] Rename `stores/__tests__/pmBoardStore.test.ts` — function names and mock service updated

#### 4h. Cross-module UI
- [x] Update PM pages: board, list, epic, sprint, my-work flows to Task naming (field accesses, component props updated)
- [x] Update PM list/detail route callers and recurring-task summary/settings call sites to task-era prop names where those routes/components are now task-first
- [x] Update task template and recurring task surfaces

### 5. Cross-product integrations

- [x] Rename support linked-story contracts and UI to task equivalents — backend `LinkedTaskID` field, service layer accepts both `"task"` and `"story"`
- [x] Rename CRM association object type `story` to `task` — `CRMObjectStory` deprecated, CRM UI updated
- [x] Rename docs linked-object type `story` to `task` — `LinkedObjectStory` deprecated, docs link handler accepts both
- [x] Update notifications UI and routing to open task detail routes
- [x] Update search and shared-link helpers from story paths to task paths

### 6. Documentation

- [x] Update `CLAUDE.md` (root) — logging examples, route references updated to task terminology
- [x] Update `server/CLAUDE.md` — `/tasks` route definition, task creation log examples
- [x] Update `AGENTS.md` — task references in log output examples
- [x] Update `docs/prds/prd-shortcut-importer.md` — `pm_tasks` table references, task type/team counts
- [x] Update `docs/prds/PRD-stories-scale-and-performance.md` — performance requirements terminology updated to tasks
- [x] Update internal PRDs and plan docs

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
