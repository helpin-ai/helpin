# Story to Task Hard-Cut Rename Plan

## Status

Draft plan for renaming the PM work item domain from **Story** to **Task** across product, API, database, automations, and agent/runtime surfaces.

This plan assumes:

- the rename is a **full rename**, not just UI copy changes
- the rollout is a **hard cutover**, not a long-lived dual-support migration
- existing records are ported **in place** by schema/data migration, not re-created with new IDs
- `story_type` also renames to `task_type`
- `story_planner` and `story` agent/runtime identifiers also rename to `task_planner` and `task`

## Goal

Replace Story/Stories with Task/Tasks everywhere that matters:

- PM UI, routes, copy, docs, and settings
- backend models, handlers, repositories, services, and route contracts
- database tables, columns, constraints, indexes, and JSONB payloads
- notification events, websocket entity names, association types, follower object types, and automation triggers
- agent presets, target types, preset system prompts, planner outputs, tool names, and related runtime identifiers

The result should be internally consistent: after rollout, the system should not expose story-era identifiers as the primary contract.

## Core Decisions

### 1. Migration mechanism: SQL migrations, not AutoMigrate

Do **not** use GORM `AutoMigrate` as the primary mechanism for this rename.

This repo now has a dedicated runtime migration path and the rename should use that path:

- `server/cmd/migrate` is the migration runner binary
- `server/internal/dbmigrate` owns the `schema_migrations` ledger, embedded SQL loading, checksums, and advisory lock
- ArgoCD should execute migrations through the `PreSync` Jobs in `k8s/stage/server-migrate.yaml` and `k8s/prod/server-migrate.yaml`
- `RUN_AUTO_MIGRATE` in `cmd/api/main.go` must be set to `false` for the Story -> Task cutover release so the app does not recreate story-era tables/columns after the rename
- startup `AutoMigrate` remains acceptable only for normal additive schema evolution outside this hard-cut rename

Use explicit numbered SQL migrations for:

- table renames like `pm_stories` -> `pm_tasks`
- column renames like `story_id` -> `task_id`
- join-table renames such as `pm_story_owners` -> `pm_task_owners`
- index, foreign-key, and constraint updates
- persisted string rewrites inside JSONB/text fields
- event/preset/target-type backfills such as `story` -> `task` and `story_planner` -> `task_planner`

`AutoMigrate` may remain enabled for normal additive schema evolution after the cutover, but it must not be trusted to carry the rename itself.

Execution model for this rename:

1. Add the forward Story -> Task SQL file under `server/internal/dbmigrate/sql/`
2. Rehearse `./migrate up` against staging data
3. Deploy the cutover release with the ArgoCD `PreSync` migration Job enabled
4. Set `RUN_AUTO_MIGRATE=false` on the application Deployment for that release
5. Let ArgoCD run the migration Job before rolling the new API pods
6. Re-enable `RUN_AUTO_MIGRATE=true` later only if desired for additive-only post-cutover changes

### 2. Port existing stories by in-place rename

Existing story records become tasks by renaming the underlying schema and references:

- preserve UUID primary keys
- preserve `display_id`
- preserve workflow state placement and ordering
- preserve labels, comments, attachments, checklists, followers, owners, external links, recurring links, git delivery links, agent runs, and associations

This is an in-place transformation, not a copy/import job.

### 3. Hard cutover

After release:

- PM canonical routes become `/pm/tasks` and `/pm/tasks/:id`
- PM API endpoints move from `/pm/stories/...` to `/pm/tasks/...`
- primary entity/event/target strings become `task`
- preset key becomes `task_planner`

No permanent compatibility layer is planned. If operationally needed during rollout, any bridge should be temporary and removed immediately after deployment validation.

## Implementation Plan

### 1. Database and schema layer

Rename schema objects in place.

The Story -> Task rename SQL should be implemented as one or more new versioned files in `server/internal/dbmigrate/sql/`, not by editing historical files in `server/migrations/` and not by relying on startup `AutoMigrate`.

Primary expected changes:

- `pm_stories` -> `pm_tasks`
- `pm_story_owners` -> `pm_task_owners`
- `pm_story_followers` -> `pm_task_followers`
- `pm_story_labels` -> `pm_task_labels`
- `pm_story_links` -> `pm_task_links`
- `pm_story_templates` -> `pm_task_templates`
- story-scoped columns like `story_id`, `linked_story_id`, `active_story_id`, `created_from_story_id`, `last_generated_story_id`, `generated_story_id` -> task equivalents
- `story_type` -> `task_type`
- `workspace_teams.default_story_type` -> `workspace_teams.default_task_type` (lives on teams table, not pm_stories family)

Sequence rename:

- `pm_story_display_id_seq` -> `pm_task_display_id_seq` (requires explicit `ALTER SEQUENCE ... RENAME TO`, plus re-pointing the column default — `ALTER TABLE RENAME` does not cascade to sequences)

Migration duties:

- rename indexes and constraints where needed for clarity and maintainability
- rewrite JSONB/text payloads that persist story-era keys or values (notably `agents.allowed_targets` which stores `["story"]`)
- update the `pm_comments.entity_type` CHECK constraint from `('story', 'epic', 'doc')` to `('task', 'epic', 'doc')`
- update check constraints and enum-like text checks that mention `story`
- update any SQL that references `pm_stories`, `story_id`, or story-era route/entity strings

Data backfill rewrites (separate from schema renames):

- `pm_comments` rows where `entity_type = 'story'` -> `'task'`
- `notifications` rows where `entity_type = 'story'` -> `'task'`
- follower/association rows where `object_type = 'story'` -> `'task'`
- `agents.allowed_targets` JSONB values containing `"story"` -> `"task"`
- automation trigger strings containing `story.` -> `task.`
- preset keys containing `story_planner` -> `task_planner`

Historical migration files (013, 014, 017, 020, 024, 038, 040, 042, 043, 044, 045, 057) reference story-era names. These are **left as-is** — they are historical artifacts documenting the state at the time they ran. Do not modify old migration files; the rename migration runs after them.

### 2. Backend contracts and business logic

Rename backend symbols and contracts from Story to Task:

- `PMStory` -> `PMTask`
- `CreateStoryRequest` -> `CreateTaskRequest`
- `UpdateStoryRequest` -> `UpdateTaskRequest`
- `MoveStoryRequest` -> `MoveTaskRequest`
- `StoryDetail` -> `TaskDetail`
- equivalent repository/service/handler names
- sprint/epic statistics fields: `story_count` -> `task_count`, `done_story_count` -> `done_task_count` (these are API contract fields consumed by frontend)
- filter parameter `story_type` -> `task_type` (used in query params and filter structs)

DI wiring (`cmd/api/main.go`):

- All story-era model registrations, repo/service/handler instantiations, and variable names must rename to task equivalents

Route changes:

- `/pm/stories` -> `/pm/tasks`
- `/pm/stories/display/{displayID}` -> `/pm/tasks/display/{displayID}`
- `/pm/story-templates` -> `/pm/task-templates` (template CRUD endpoints)
- `/pm/story-relationships/{id}` -> `/pm/task-relationships/{id}` (association endpoints)
- all nested endpoints move to task-based paths, including activity, checklist, links, associations, recurring-template, move, reorder, labels, owners, followers, git links, delivery target, assign-agent, and run-agent

Behavioral renames:

- notification event names: `story.*` -> `task.*`
- websocket entity and parent types: `story` -> `task`
- CRM/docs/support association object types: `story` -> `task`
- automation triggers such as `story.state_entered` -> `task.state_entered`
- follower object type and generic entity references move from `story` to `task`

### 3. Agent, automation, and runtime surfaces

Rename agent/runtime identifiers fully:

- preset key `story_planner` -> `task_planner`
- preset label `Story Planner` -> `Task Planner`
- allowed target type `story` -> `task`
- run target type `story` -> `task`
- `active_story_id` -> `active_task_id`
- any `story_id` fields in agent runs, orchestration payloads, planning contracts, and generated artifacts -> task equivalents
- model constant `AgentPresetStoryPlanner = "story_planner"` in `model/agent.go` -> `AgentPresetTaskPlanner = "task_planner"`

Worker execution context (`internal/worker/eino_executor.go`):

- `execCtx.Story` field and `execCtx.StoryID` -> `execCtx.Task` / `execCtx.TaskID`
- `BuildSystemPrompt()` receives the Story object for LLM template generation — must pass Task
- webhook event name `"pm.story_completion_followups"` -> `"pm.task_completion_followups"`

Repository agent schema (`internal/repository/agent_schema.go`):

- inline SQL enum value `'story'` in migration/schema logic -> `'task'`

Update system prompts, planning schemas, and structured outputs:

- `proposed_stories` -> `proposed_tasks`
- `story_refs` -> `task_refs`
- prompt wording and tool descriptions refer to tasks instead of stories
- internal command registry and automation inventory language must no longer use Story as the product term

Existing system agents and workspace-customized preset references must be migrated in DB so no stored rows continue pointing at `story_planner` or `story` target types after cutover.

### 4. Frontend and route tree

Rename frontend routes and modules:

- `/w/:slug/pm/stories` -> `/w/:slug/pm/tasks`
- `/w/:slug/pm/stories/:id` -> `/w/:slug/pm/tasks/:id`
- `/w/:slug/settings/story-templates` -> `/w/:slug/settings/task-templates`
- story detail surfaces, stores, services, query keys, hooks, and custom events rename to task equivalents

Expected surface changes include:

Services:

- `pmStoryService.ts` -> `pmTaskService.ts` (core CRUD, owners, labels, followers)
- `pmStoryTemplateService.ts` -> `pmTaskTemplateService.ts` (template CRUD at `/pm/story-templates`)
- `associationsService.ts` — rename `.listByStory()`, `.createStoryRelationship()`, `.deleteStoryRelationship()` methods and `/pm/story-relationships/` endpoint paths

Stores:

- `storyPanelStore.ts` -> `taskPanelStore.ts` (storyId, lastClosedStoryId, openStory, closeStory, etc.)
- `pmBoardStore.ts` — createStory, moveStory, patchStory, moveMemberStory, story filter/sort functions
- `boardDisplayStore.ts`, `globalCreateStore.ts` — story references in grouping logic

Hooks:

- `useStories.ts` -> `useTasks.ts` (14 hooks: useStories, useStory, useStoryByDisplayId, useCreateStory, useUpdateStory, useArchiveStory, useDeleteStory, useStoryActivity, useAddStoryOwner, useRemoveStoryOwner, useAddStoryLabel, useRemoveStoryLabel, useSyncStoryLabels, useAddStoryFollower, useRemoveStoryFollower)
- `useAssociations.ts` — useStoryAssociations, useCreateStoryRelationship, useDeleteStoryRelationship, `'story'` object type checks
- `useComments.ts` — entity type literal `'story'` in type union

Query keys (`queryKeys.ts`):

- 11 story-prefixed keys: story, storyByDisplayId, storyActivity, storyAssociations, storyRelationships, storyRecurringTemplate, comments, checklists, attachments, externalLinks, storyLinks — all containing `'stories'` path segments

Types:

- `pmTypes.ts` — Story, StoryDetail, StoryRecurringSummary, StoryType, StoryStateColumn, StoryMemberColumn, StoryGroup, StoryTemplate, CreateStoryRequest, UpdateStoryRequest, CreateStoryTemplateRequest, UpdateStoryTemplateRequest, CreateStoryRelationshipRequest, StoryUserLinkRequest, StoryLabelLinkRequest
- `crmTypes.ts` — `CRMObjectType` union includes `'story'` literal -> `'task'`
- `types.ts` — `story_type` feature flag boolean, `default_story_type` field

Routes and navigation:

- `pmStoryLinks.ts` -> `pmTaskLinks.ts` (buildStoryPath, buildStoryUrl, buildStoryCopyUrl)
- `storyRouteNavigation.ts` -> `taskRouteNavigation.ts`
- `StoryTemplatesSettingsPage.tsx` -> `TaskTemplatesSettingsPage.tsx`
- settings route `story-templates` -> `task-templates`

Components (44+ files):

- core panels: `StoryDetailPanel.tsx`, `GlobalStoryPanel.tsx` (entity: `'story'`), `StoryCard.tsx`
- feature panels: `StoryDeliveryPanel.tsx`, `StoryGitPanel.tsx`, `StoryRelationshipsSection.tsx`, `StoryListView.tsx`
- modals: `CreateStoryModal.tsx` (mode: `'story'` | `'template'`)
- filters: `StoryFilters.tsx` (story_type filter field), `StoryCard.sortable.ts`
- sprint: `SprintPlanningStoryCard.tsx`
- settings: `StoryTemplatesSettings.tsx`, `RecurringTemplateList.tsx` (lastGeneratedStory)
- sidebar: `StorySidebarIdRow.tsx`
- sidebar config: `layout/sidebar/config.ts` — `{ key: 'story', label: 'Story', pages: ['stories'] }` -> task equivalents
- `story-detail/` utilities (13 files): storyFilterMembers, storyOverlayDismiss, storyLabelSync, storyPendingPatch, storyListGrouping, storyOverlayState, storyPlanningScope, storyListPinnedOffsets, storyDetailEventPayload, StoryStateSelectContent, StoryRouteFallbackBackground
- `RecurringTemplatesSettings.tsx` — openStoryRoute reference

DOM events and URL params:

- DOM events: `story-created`, `story-panel-updated`, `story-panel-archived` -> task equivalents
- URL query parameter `?story=<id>` -> `?task=<id>` (panel state + `window.history` manipulation)
- URL param cleanup: `url.searchParams.delete('story')` -> `'task'`

Tests (24+ files):

- `__tests__/CreateStoryModal.test.tsx`, `StoryCard.sortable.test.tsx`
- `story-detail/__tests__/` (13 test files matching utility modules)
- `lib/__tests__/pmStoryLinks.test.ts`
- `stores/__tests__/pmBoardStore.test.ts`

Cross-module UI:

- support, CRM, docs, notifications, and agent run UI surfaces use Task labels and task entity types

### 5. Existing data porting

Existing records should port like this:

- every current `pm_stories` row becomes a `pm_tasks` row with the same `id`
- every referencing child row follows the renamed FK column/table without changing the referenced ID
- existing `display_id` values remain unchanged
- no business data is re-authored or regenerated

This preserves:

- board order and workflow state
- history and comments
- agent run attachments to the target entity
- recurring task lineage
- support linked work
- CRM/docs associations
- git delivery and branch linkage

## Validation

### Backend

- task CRUD, move, reorder, board, member board
- task checklist, attachments, links, activity, recurring, associations, git delivery
- automation triggers work under `task.*`
- notifications emit `task.*` names
- agent assignment/run works against `task` target type and `task_planner`
- migration tests prove existing seeded story data survives rename in place

### Frontend

- `/pm/tasks` board and `/pm/tasks/:id` detail flow
- create/open/edit/archive/move/reorder task
- task routes work from PM, epics, sprints, support, CRM, notifications, and search
- agent pages render `Task Planner` and task target types
- no story-era route or contract remains in active product flows

### Search/grep acceptance

Before rollout, verify there are no remaining primary-contract story-era identifiers in app code except:

- historical migration filenames and comments describing the old state
- deliberate archival docs if retained

Specifically clear:

- `/pm/stories`
- `story_id`
- `active_story_id`
- `story_type`
- `story_planner`
- target/entity type `story`
- notification event prefixes `story.`

### 6. Documentation

Update repo-level documentation that references "story" as the canonical term:

- `CLAUDE.md` (root) — logging examples (`"story created"`, `"story_id"`), route references (`/pm/stories`)
- `server/CLAUDE.md` — `/stories` route definition, story creation log examples
- `AGENTS.md` — story references in log output examples
- `docs/prd-shortcut-importer.md` — Shortcut import mappings referencing `pm_stories`, story type counts, team story counts
- `docs/PRD-stories-scale-and-performance.md` — performance requirements for stories board and scaling
- Any internal PRDs or plan docs that describe story as the current product term should note the rename

## Rollout Notes

- This is a coordinated release and should not be partially deployed.
- Migration execution order must be documented and rehearsed against a seeded staging database.
- Existing bookmarked `/pm/stories/...` URLs will break after cutover by design, so release notes and internal comms should call out the route change.

### Migration runner sequence

For this rename, Kubernetes/ArgoCD rollout should be:

1. Build and publish the server image that contains `./migrate` and the new Story -> Task SQL migration files
2. Ensure the `PreSync` Job manifest points at that exact image tag
3. Set `RUN_AUTO_MIGRATE=false` on the API Deployment for the cutover release
4. Let ArgoCD run the `PreSync` migration Job (`./migrate up`) before applying the new Deployment
5. Roll the new task-era API pods only after the migration Job succeeds

Do not rely on API startup to perform the rename. The runner is now the canonical path for this class of migration.

### Temporary redirects (recommended)

Consider adding a low-cost 301 redirect from `/pm/stories/*` -> `/pm/tasks/*` at the backend router level for 1-2 release cycles. This is a single middleware rule and significantly improves the transition experience for users with bookmarked URLs and any external integrations pointing at old paths. Remove the redirect after one release cycle.

### Rollback plan

If the migration fails or a critical issue is discovered post-deploy:

1. **Pre-migration**: take a database snapshot/backup immediately before running the rename migration
2. **Migration failure**: restore from snapshot; the application code has not been deployed yet, so the old code continues to work
3. **Post-deploy critical issue**: if discovered within the deployment window, restore the database snapshot and roll back the application deployment; if discovered later and data has been written to the new schema, a reverse migration script should be prepared in advance (rename tables/columns back, rewrite entity types back to `story`)

A reverse migration SQL script should be written alongside the forward migration and tested during staging rehearsal, even if it is never expected to run.
