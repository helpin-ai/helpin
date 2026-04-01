# Story to Task Hard-Cut Progress Tracker

> **Tracking rule:** update this file as work lands. Keep statuses limited to `Pending`, `In Progress`, `Blocked`, or `Done`.

## Overall Status

| Workstream | Status | Notes |
| --- | --- | --- |
| Schema and migrations | In Progress | Forward SQL hard-cut migration exists and dry-run validates in a rollback transaction; do not merge until task-era code is ready |
| Data backfill rewrites | In Progress | Core entity/preset rewrites are in the migration; automation/prompt payload sweep still pending |
| Backend contracts | Pending | Models, handlers, services, repositories, routes |
| Agent and automation surfaces | In Progress | `task_planner` preset family and default target-type slice landed; planner payload/task-context renames still pending |
| Frontend routes and PM UI | Pending | `/pm/tasks`, task detail, services, stores, copy |
| Cross-product integrations | Pending | Support, CRM, docs, notifications, search |
| Documentation | Pending | CLAUDE.md, AGENTS.md, internal docs |
| Validation and rollout | In Progress | Forward and rollback SQL exist and validate in rollback transactions; staging rehearsal and app cutover remain |

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
- [ ] Rename handlers, services, repositories, and DI wiring in `cmd/api/main.go` to task-era names
- [ ] Move PM routes from `/pm/stories/...` to `/pm/tasks/...`
- [ ] Move template routes from `/pm/story-templates` to `/pm/task-templates`
- [ ] Move relationship routes from `/pm/story-relationships/{id}` to `/pm/task-relationships/{id}`
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
- [ ] Update system prompts, tool descriptions, inventory text, and internal command registry wording
- [ ] Migrate existing system/workspace agent rows to task-era preset keys and target types

### 4. Frontend PM surfaces

#### 4a. Types and query infrastructure
- [ ] Rename all Story types in `pmTypes.ts` (Story, StoryDetail, StoryRecurringSummary, StoryType, StoryStateColumn, StoryMemberColumn, StoryGroup, StoryTemplate, CreateStoryRequest, UpdateStoryRequest, CreateStoryTemplateRequest, UpdateStoryTemplateRequest, CreateStoryRelationshipRequest, StoryUserLinkRequest, StoryLabelLinkRequest)
- [ ] Rename `crmTypes.ts` CRMObjectType `'story'` literal -> `'task'`
- [ ] Rename `types.ts` fields: `story_type` feature flag boolean, `default_story_type`
- [ ] Rename all 11 story-prefixed query keys in `queryKeys.ts` and their `'stories'` path segments
- [ ] Rename `useStories.ts` -> `useTasks.ts` (14 hooks)
- [ ] Rename story hooks in `useAssociations.ts` (useStoryAssociations, useCreateStoryRelationship, useDeleteStoryRelationship, `'story'` object type checks)
- [ ] Rename `useComments.ts` entity type literal `'story'` -> `'task'`

#### 4b. Services
- [ ] Rename `pmStoryService.ts` -> `pmTaskService.ts` (CRUD, owners, labels, followers)
- [ ] Rename `pmStoryTemplateService.ts` -> `pmTaskTemplateService.ts` (endpoint paths `/pm/story-templates`)
- [ ] Rename `associationsService.ts` methods (`.listByStory()`, `.createStoryRelationship()`, `.deleteStoryRelationship()`) and endpoint path `/pm/story-relationships/`

#### 4c. Stores
- [ ] Rename `storyPanelStore.ts` -> `taskPanelStore.ts` (storyId, lastClosedStoryId, openStory, closeStory, etc.)
- [ ] Rename story functions in `pmBoardStore.ts` (createStory, moveStory, patchStory, moveMemberStory, filter/sort)
- [ ] Rename story references in `boardDisplayStore.ts` and `globalCreateStore.ts`

#### 4d. Routes and navigation
- [ ] Rename canonical PM routes to `/pm/tasks` and `/pm/tasks/:id`
- [ ] Rename settings route `story-templates` -> `task-templates` and `StoryTemplatesSettingsPage.tsx`
- [ ] Rename `pmStoryLinks.ts` -> `pmTaskLinks.ts` (buildStoryPath, buildStoryUrl, buildStoryCopyUrl)
- [ ] Rename `storyRouteNavigation.ts` -> `taskRouteNavigation.ts`
- [ ] Rename URL query parameter `?story=<id>` -> `?task=<id>` (panel state + window.history)
- [ ] Rename `url.searchParams.delete('story')` -> `'task'`

#### 4e. Components (44+ files)
- [ ] Rename core panels: `StoryDetailPanel.tsx`, `GlobalStoryPanel.tsx` (entity: `'story'`), `StoryCard.tsx`
- [ ] Rename feature panels: `StoryDeliveryPanel.tsx`, `StoryGitPanel.tsx`, `StoryRelationshipsSection.tsx`, `StoryListView.tsx`
- [ ] Rename modals: `CreateStoryModal.tsx` (mode: `'story'` -> `'task'`)
- [ ] Rename filters: `StoryFilters.tsx` (story_type filter field), `StoryCard.sortable.ts`
- [ ] Rename sprint: `SprintPlanningStoryCard.tsx`
- [ ] Rename settings: `StoryTemplatesSettings.tsx`, `RecurringTemplateList.tsx` (lastGeneratedStory)
- [ ] Rename `StorySidebarIdRow.tsx`
- [ ] Rename sidebar config in `layout/sidebar/config.ts`: `{ key: 'story', label: 'Story', pages: ['stories'] }` -> task equivalents
- [ ] Rename `story-detail/` directory and all 13 utility files (storyFilterMembers, storyOverlayDismiss, storyLabelSync, storyPendingPatch, storyListGrouping, storyOverlayState, storyPlanningScope, storyListPinnedOffsets, storyDetailEventPayload, StoryStateSelectContent, StoryRouteFallbackBackground, etc.)
- [ ] Rename `RecurringTemplatesSettings.tsx` openStoryRoute reference

#### 4f. DOM events
- [ ] Rename DOM custom events: `story-created`, `story-panel-updated`, `story-panel-archived` -> task equivalents

#### 4g. Tests (24+ files)
- [ ] Rename `__tests__/CreateStoryModal.test.tsx`, `StoryCard.sortable.test.tsx`
- [ ] Rename `story-detail/__tests__/` (13 test files matching utility modules)
- [ ] Rename `lib/__tests__/pmStoryLinks.test.ts`
- [ ] Rename `stores/__tests__/pmBoardStore.test.ts`

#### 4h. Cross-module UI
- [ ] Update PM pages: board, list, epic, sprint, my-work flows to Task naming
- [ ] Update task template and recurring task surfaces

### 5. Cross-product integrations

- [ ] Rename support linked-story contracts and UI to task equivalents
- [ ] Rename CRM association object type `story` to `task` (CRM `AssociationsList.tsx`, `EmailAccountConnect.tsx`)
- [ ] Rename docs linked-object type `story` to `task`
- [ ] Update notifications UI and routing to open task detail routes
- [ ] Update search and shared-link helpers from story paths to task paths

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
- [ ] Verify end-to-end task CRUD and board flows against migrated data
- [ ] Verify agent assignment/run and automation triggers against `task`
- [ ] Verify no primary-contract `/pm/stories`, `story_id`, `story_type`, `story_planner`, or `story.*` identifiers remain
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
- Frontend has 80+ files and 3,000+ story references — recommend automated find-and-replace with manual verification rather than hand-editing.
- `repository/agent_schema.go` contains inline SQL with `'story'` enum — easy to miss as it's not in the migrations directory.

## File Impact Summary

| Layer | Files to modify | Key risk areas |
| --- | --- | --- |
| SQL migrations (new) | 1 migration file | Sequence rename, CHECK constraints, data backfills |
| Backend Go | ~33 files | Worker execution context, agent schema, DI wiring |
| Frontend TS/TSX | ~80+ files | 3,000+ references, 24 test files, 13 utility modules |
| Documentation | ~6 files | CLAUDE.md, AGENTS.md, PRDs |
