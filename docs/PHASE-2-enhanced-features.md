# Phase 2: Enhanced Features

**Timeline**: Weeks 5-8
**Goal**: Feature parity with team's daily Shortcut usage.
**Status**: Not Started
**Depends on**: Phase 1 complete

---

## Overview

This phase adds the List/Table view, filtering system, saved views, sub-tasks, checklists, story links/dependencies, templates, custom fields, notifications, search, and bulk operations. By the end of Phase 2, the PM module covers everything the team used in Shortcut on a daily basis.

---

## Task Breakdown

### 2.1 Database Migrations

#### Task 2.1.1: Create PM Sub-tasks table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_sub_tasks` table
- **Details**:
  - `pm_sub_tasks`: id, story_id, name, description, workflow_state_id, owner_id, estimate, position, completed, completed_at, timestamps
  - FK: story_id → pm_stories, workflow_state_id → pm_workflow_states, owner_id → users
  - Index on story_id
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/016_pm_sub_tasks.sql`

#### Task 2.1.2: Create PM Checklists tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_checklists` and `pm_checklist_items` tables
- **Details**:
  - `pm_checklists`: id, story_id, name, position, created_at
  - `pm_checklist_items`: id, checklist_id, text, completed, assignee_id, position, timestamps
  - FKs, indexes on story_id and checklist_id
- **Files**: `server/migrations/017_pm_checklists.sql`

#### Task 2.1.3: Create PM Story Links table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_story_links` table
- **Details**:
  - `pm_story_links`: id, workspace_id, source_story_id, target_story_id, link_type (blocks/relates_to/duplicates), created_by, created_at
  - CHECK: source_story_id != target_story_id
  - UNIQUE: (source_story_id, target_story_id, link_type)
  - Indexes on source_story_id, target_story_id
- **Files**: `server/migrations/018_pm_story_links.sql`

#### Task 2.1.4: Create PM Custom Fields tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_custom_fields`, `pm_custom_field_options`, `pm_custom_field_values` tables
- **Details**:
  - `pm_custom_fields`: id, workspace_id, name, description, field_type, icon, position, enabled, timestamps
  - `pm_custom_field_options`: id, custom_field_id, value, color, position
  - `pm_custom_field_values`: id, story_id, custom_field_id, option_id, text_value, number_value
  - UNIQUE: (story_id, custom_field_id) on values table
- **Files**: `server/migrations/019_pm_custom_fields.sql`

#### Task 2.1.5: Create PM Story Templates table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_story_templates` table
- **Details**:
  - `pm_story_templates`: id, workspace_id, name, description, story_type, template_description, default_team_id, default_epic_id, default_workflow_state_id, default_priority, default_estimate, default_labels (jsonb), default_custom_fields (jsonb), checklist_template (jsonb), created_by, timestamps
- **Files**: `server/migrations/020_pm_templates.sql`

#### Task 2.1.6: Create PM Saved Views table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_saved_views` table
- **Details**:
  - `pm_saved_views`: id, workspace_id, name, view_type (board/list/timeline), filters (jsonb), group_by, sort_by, sort_order, column_config (jsonb), is_default, team_id, created_by, is_shared, position, timestamps
- **Files**: `server/migrations/021_pm_saved_views.sql`

#### Task 2.1.7: Create PM Notifications table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_notifications` table
- **Details**:
  - `pm_notifications`: id, workspace_id, user_id, entity_type, entity_id, notification_type, title, body, actor_id, read, read_at, created_at
  - Index on (user_id, read, created_at DESC)
- **Files**: `server/migrations/022_pm_notifications.sql`

---

### 2.2 Backend — Sub-tasks

#### Task 2.2.1: Sub-task models
- **Status**: [ ] Not Started
- **Description**: Go structs for SubTask
- **Details**:
  - GORM model with JSON tags
  - CreateSubTaskRequest, UpdateSubTaskRequest, MoveSubTaskRequest, ReorderSubTasksRequest
- **Files**: `server/internal/model/pm_sub_task.go`

#### Task 2.2.2: Sub-task repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for sub-tasks
- **Methods**:
  - `ListByStory(ctx, storyID)` — list sub-tasks for a story
  - `GetByID(ctx, id)` — get sub-task
  - `Create(ctx, subTask)` — create sub-task
  - `Update(ctx, subTask)` — update sub-task
  - `Delete(ctx, id)` — delete sub-task
  - `MoveToState(ctx, id, stateID)` — change workflow state
  - `Reorder(ctx, storyID, subTaskIDs)` — reorder within story
  - `SumEstimates(ctx, storyID)` — total estimate points for parent rollup
  - `AllCompleted(ctx, storyID)` — check if all sub-tasks are done
  - `AnyStarted(ctx, storyID)` — check if any sub-task is started
- **Files**: `server/internal/repository/pm_sub_task.go`

#### Task 2.2.3: Sub-task service
- **Status**: [ ] Not Started
- **Description**: Business logic for sub-tasks
- **Logic**:
  - Sub-task inherits workflow from parent story
  - Estimate rollup: sub-task points added to parent total
  - Auto-transition parent: when any sub-task → Started, parent moves to first Started state (if not already started)
  - Auto-transition parent: when ALL sub-tasks → Done, parent moves to first Done state
  - Log activity on parent story
- **Files**: `server/internal/service/pm_sub_task.go`

#### Task 2.2.4: Sub-task handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for sub-task CRUD
- **Endpoints**:
  - `GET /api/pm/stories/{storyId}/sub-tasks`
  - `POST /api/pm/stories/{storyId}/sub-tasks`
  - `PUT /api/pm/sub-tasks/{id}`
  - `DELETE /api/pm/sub-tasks/{id}`
  - `PUT /api/pm/sub-tasks/{id}/move`
  - `PUT /api/pm/sub-tasks/reorder`
- **Files**: `server/internal/handler/pm_sub_task.go`

---

### 2.3 Backend — Checklists

#### Task 2.3.1: Checklist models
- **Status**: [ ] Not Started
- **Description**: Go structs for Checklist and ChecklistItem
- **Files**: `server/internal/model/pm_checklist.go`

#### Task 2.3.2: Checklist repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for checklists and items
- **Methods**:
  - `ListByStory(ctx, storyID)` — checklists with items
  - `CreateChecklist(ctx, checklist)`, `UpdateChecklist`, `DeleteChecklist`
  - `CreateItem(ctx, item)`, `UpdateItem`, `DeleteItem`
  - `ToggleItem(ctx, id, completed)` — toggle completion
  - `ReorderItems(ctx, checklistID, itemIDs)`
- **Files**: `server/internal/repository/pm_checklist.go`

#### Task 2.3.3: Checklist service
- **Status**: [ ] Not Started
- **Description**: Business logic for checklists
- **Logic**:
  - Promote checklist item to sub-task (create sub-task, delete item)
- **Files**: `server/internal/service/pm_checklist.go`

#### Task 2.3.4: Checklist handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for checklist CRUD
- **Endpoints**:
  - `POST /api/pm/stories/{storyId}/checklists`
  - `PUT /api/pm/checklists/{id}`
  - `DELETE /api/pm/checklists/{id}`
  - `POST /api/pm/checklists/{id}/items`
  - `PUT /api/pm/checklist-items/{id}`
  - `DELETE /api/pm/checklist-items/{id}`
  - `POST /api/pm/checklist-items/{id}/promote`
  - `PUT /api/pm/checklists/{id}/items/reorder`
- **Files**: `server/internal/handler/pm_checklist.go`

---

### 2.4 Backend — Story Links & Dependencies

#### Task 2.4.1: Story Link models
- **Status**: [ ] Not Started
- **Description**: Go structs for StoryLink
- **Files**: `server/internal/model/pm_story_link.go`

#### Task 2.4.2: Story Link repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for story links
- **Methods**:
  - `ListByStory(ctx, storyID)` — all links where story is source or target
  - `Create(ctx, link)` — create link
  - `Delete(ctx, id)` — delete link
  - `IsBlocked(ctx, storyID)` — check if story has active blockers (uncompleted blocking stories)
  - `GetBlockers(ctx, storyID)` — list stories blocking this one
  - `GetBlocking(ctx, storyID)` — list stories this one blocks
- **Files**: `server/internal/repository/pm_story_link.go`

#### Task 2.4.3: Story Link service
- **Status**: [ ] Not Started
- **Description**: Business logic for story links
- **Logic**:
  - When creating a `blocks` link: update target story's `blocked = true`
  - When blocking story completes: update target story's `blocked = false` (if no other blockers)
  - Prevent circular dependencies (A blocks B, B blocks A)
  - Log activity on both stories
- **Files**: `server/internal/service/pm_story_link.go`

#### Task 2.4.4: Story Link handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for story links
- **Endpoints**:
  - `POST /api/pm/stories/{id}/links` — create link
  - `DELETE /api/pm/stories/{id}/links/{linkId}` — delete link
- **Files**: `server/internal/handler/pm_story_link.go` (or add to pm_story.go)

---

### 2.5 Backend — Custom Fields

#### Task 2.5.1: Custom Field models
- **Status**: [ ] Not Started
- **Description**: Go structs for CustomField, CustomFieldOption, CustomFieldValue
- **Files**: `server/internal/model/pm_custom_field.go`

#### Task 2.5.2: Custom Field repository
- **Status**: [ ] Not Started
- **Description**: Data access layer
- **Methods**:
  - `ListFields(ctx, workspaceID)` — fields with options
  - `CreateField`, `UpdateField`, `DeleteField`
  - `CreateOption`, `UpdateOption`, `DeleteOption`
  - `SetValue(ctx, storyID, fieldID, optionID)` — set/update value
  - `GetValues(ctx, storyID)` — all custom field values for a story
  - `ClearValue(ctx, storyID, fieldID)` — remove value
- **Files**: `server/internal/repository/pm_custom_field.go`

#### Task 2.5.3: Custom Field service & handler
- **Status**: [ ] Not Started
- **Description**: Business logic and HTTP handlers
- **Endpoints**:
  - `GET /api/pm/custom-fields`
  - `POST /api/pm/custom-fields`
  - `PUT /api/pm/custom-fields/{id}`
  - `DELETE /api/pm/custom-fields/{id}`
  - `POST /api/pm/custom-fields/{id}/options`
  - `PUT /api/pm/custom-field-options/{id}`
  - `DELETE /api/pm/custom-field-options/{id}`
- **Files**: `server/internal/service/pm_custom_field.go`, `server/internal/handler/pm_custom_field.go`

---

### 2.6 Backend — Templates

#### Task 2.6.1: Template models, repository, service, handler
- **Status**: [ ] Not Started
- **Description**: Full CRUD for story templates
- **Endpoints**:
  - `GET /api/pm/templates`
  - `POST /api/pm/templates`
  - `GET /api/pm/templates/{id}`
  - `PUT /api/pm/templates/{id}`
  - `DELETE /api/pm/templates/{id}`
- **Logic**:
  - When creating story from template, populate all pre-filled fields
  - Requester always set to current user
- **Files**: `server/internal/model/pm_template.go`, `server/internal/repository/pm_template.go`, `server/internal/service/pm_template.go`, `server/internal/handler/pm_template.go`

---

### 2.7 Backend — Saved Views (Spaces)

#### Task 2.7.1: Saved View models, repository, service, handler
- **Status**: [ ] Not Started
- **Description**: Full CRUD for saved views
- **Endpoints**:
  - `GET /api/pm/views`
  - `POST /api/pm/views`
  - `PUT /api/pm/views/{id}`
  - `DELETE /api/pm/views/{id}`
- **Logic**:
  - List returns: user's personal views + shared views for workspace
  - Filter configuration stored as JSONB
  - Only creator or admin can edit/delete shared views
- **Files**: `server/internal/model/pm_saved_view.go`, `server/internal/repository/pm_saved_view.go`, `server/internal/service/pm_saved_view.go`, `server/internal/handler/pm_saved_view.go`

---

### 2.8 Backend — Notifications

#### Task 2.8.1: Notification models, repository, service, handler
- **Status**: [ ] Not Started
- **Description**: In-app notification system
- **Endpoints**:
  - `GET /api/pm/notifications` — paginated, filter by read/unread
  - `GET /api/pm/notifications/unread-count`
  - `PUT /api/pm/notifications/{id}/read`
  - `PUT /api/pm/notifications/read-all`
- **Logic**:
  - Create notifications when:
    - Story assigned to user
    - @mentioned in comment/description
    - Comment added on followed entity
    - State changed on followed story
    - Deadline approaching (24h) or passed
    - Story blocked/unblocked
  - Notification creator service called from story, comment, and link services
  - Don't notify the actor themselves
- **Files**: `server/internal/model/pm_notification.go`, `server/internal/repository/pm_notification.go`, `server/internal/service/pm_notification.go`, `server/internal/handler/pm_notification.go`

---

### 2.9 Backend — Search & Bulk Operations

#### Task 2.9.1: Story search endpoint
- **Status**: [ ] Not Started
- **Description**: Search stories with operators
- **Endpoint**: `GET /api/pm/stories/search?q={query}`
- **Logic**:
  - Parse search query for operators: `type:`, `is:`, `has:`, `label:`, `epic:`, `owner:`, `team:`, `estimate:`, `priority:`, `iteration:`, `created:`
  - Free text searches title, description, comments
  - Build dynamic SQL query from parsed operators
  - Return paginated results
- **Files**: Add to `server/internal/handler/pm_story.go`, new `server/internal/service/pm_search.go`

#### Task 2.9.2: Bulk update endpoint
- **Status**: [ ] Not Started
- **Description**: Bulk update multiple stories at once
- **Endpoint**: `POST /api/pm/stories/bulk`
- **Request Body**: `{ story_ids: [], updates: { state_id?, epic_id?, iteration_id?, owner_id?, priority?, labels_add?, labels_remove? } }`
- **Logic**:
  - Validate all story_ids belong to workspace
  - Apply updates to all stories
  - Log activity for each story
  - Return updated stories
- **Files**: Add to story handler and service

---

### 2.10 Frontend — List/Table View

#### Task 2.10.1: Story Table component
- **Status**: [ ] Not Started
- **Description**: Table view with sortable columns and inline editing
- **Details**:
  - Columns: checkbox, display ID, type icon, title, state, priority, owner(s), epic, iteration, estimate, labels, due date, updated_at
  - Sortable by clicking column headers
  - Inline editing: click cell to edit state, owner, priority, estimate, labels
  - Row click → open Story Detail Panel
  - Group by: Epic, Iteration, Owner, Team, Label, State, Priority
  - Column visibility toggle
  - Use TanStack Table for features
- **Files**: `frontend/src/components/pm/StoryTable.tsx`

#### Task 2.10.2: View switcher
- **Status**: [ ] Not Started
- **Description**: Toggle between Board and List view
- **Details**:
  - Tab buttons or icon toggle in Stories page header
  - Persist selection in localStorage
  - Both views share the same filter state
- **Files**: Update `frontend/src/pages/pm/Stories.tsx`

---

### 2.11 Frontend — Filtering System

#### Task 2.11.1: FilterBar component
- **Status**: [ ] Not Started
- **Description**: Reusable filter component with AND/OR logic
- **Details**:
  - "Add Filter" button → dropdown of available filter types
  - Each filter: type selector + value selector (multi-select dropdown)
  - Filter types: Team, Epic, Iteration, Owner, Requester, Label, Story Type, State, Priority, Custom Fields, Estimate range, Due Date range
  - Default group: "Matches All" (AND)
  - Optional: "Matches Any" (OR) group
  - Toggle individual filters on/off without removing
  - Clear all filters button
  - Filter count badge
- **Files**: `frontend/src/components/pm/FilterBar.tsx`

#### Task 2.11.2: Filter state management
- **Status**: [ ] Not Started
- **Description**: Zustand store for filter state
- **Details**:
  - Active filters with AND/OR groups
  - Serialize/deserialize to URL params
  - Serialize to saved view JSONB
  - Apply filters to API requests
- **Files**: `frontend/src/stores/pmFilterStore.ts`

---

### 2.12 Frontend — Saved Views (Spaces)

#### Task 2.12.1: Spaces tabs component
- **Status**: [ ] Not Started
- **Description**: Tab bar at top of Stories page for saved views
- **Details**:
  - List saved views as tabs
  - "+" button to save current view as a Space
  - Click tab → apply view's filters, grouping, sort, view type
  - Right-click tab → rename, share, delete
  - Distinguish personal vs shared views (shared icon)
  - Drag to reorder tabs
- **Files**: `frontend/src/components/pm/SpacesTabs.tsx`

#### Task 2.12.2: Save View modal
- **Status**: [ ] Not Started
- **Description**: Modal to save current view configuration
- **Details**:
  - Name input
  - Share toggle (personal or shared with workspace)
  - Captures: view_type, filters, group_by, sort_by, sort_order, column_config
- **Files**: `frontend/src/components/pm/SaveViewModal.tsx`

---

### 2.13 Frontend — Sub-tasks & Checklists

#### Task 2.13.1: Sub-tasks section in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: Sub-task management within story detail
- **Details**:
  - "Sub-tasks" section in Story Detail Panel
  - List sub-tasks with: name, state badge, owner avatar, estimate
  - Inline add new sub-task
  - Click sub-task → expand inline editor (name, description, state, owner, estimate)
  - Drag to reorder
  - Progress bar: completed / total sub-tasks
  - Delete sub-task (confirm)
- **Files**: `frontend/src/components/pm/SubTaskList.tsx`

#### Task 2.13.2: Checklists section in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: Checklist management within story detail
- **Details**:
  - "Checklists" section in Story Detail Panel
  - Add new checklist (name input)
  - Add items to checklist (text input)
  - Toggle item completed (checkbox)
  - Assign item to user
  - Reorder items via drag
  - "Promote to Sub-task" action on item (three-dot menu)
  - Delete item, delete checklist
  - Progress: X of Y items completed
- **Files**: `frontend/src/components/pm/ChecklistSection.tsx`

---

### 2.14 Frontend — Story Links & Dependencies

#### Task 2.14.1: Relationships section in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: View and manage story links
- **Details**:
  - "Relationships" section in Story Detail Panel
  - List existing links grouped by type:
    - Blocks: stories this one blocks
    - Blocked by: stories blocking this one
    - Relates to: related stories
    - Duplicates: duplicate stories
  - "Add Relationship" button → select type + search for story
  - Each linked story shows: display ID, title, state badge
  - Click linked story → navigate to it
  - Remove relationship (X button)
  - Blocked indicator on story card (board view)
- **Files**: `frontend/src/components/pm/StoryRelationships.tsx`

---

### 2.15 Frontend — Custom Fields

#### Task 2.15.1: Custom Fields in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: Display and edit custom field values on stories
- **Details**:
  - Custom fields section in Story Detail Panel sidebar
  - Each field: label + dropdown selector for value
  - Clear value option
  - Fields appear in order set by admin
  - Only show enabled fields
- **Files**: `frontend/src/components/pm/CustomFieldsSection.tsx`

#### Task 2.15.2: Custom Fields admin page
- **Status**: [ ] Not Started
- **Description**: Manage custom fields in PM settings
- **Details**:
  - Route: `/w/{slug}/pm/settings` (Custom Fields tab)
  - List existing fields with name, type, options count, enabled toggle
  - Create new field: name, description, icon, options (add/remove/reorder)
  - Edit field: modify options, colors, ordering
  - Delete field (with confirmation)
  - Reorder fields (drag)
- **Files**: `frontend/src/pages/pm/Settings.tsx`

#### Task 2.15.3: Custom Fields in Table View columns
- **Status**: [ ] Not Started
- **Description**: Show custom fields as columns in list/table view
- **Details**:
  - Column visibility toggle includes custom fields
  - Each custom field shows as a dropdown cell
  - Sortable and groupable by custom field
- **Files**: Update `frontend/src/components/pm/StoryTable.tsx`

---

### 2.16 Frontend — Templates

#### Task 2.16.1: Template selector in Create Story modal
- **Status**: [ ] Not Started
- **Description**: Select template when creating a new story
- **Details**:
  - "Use Template" dropdown at top of Create Story modal
  - Selecting a template populates all pre-filled fields
  - User can override any field before creating
- **Files**: Update `frontend/src/components/pm/CreateStoryModal.tsx`

#### Task 2.16.2: Template management page
- **Status**: [ ] Not Started
- **Description**: Manage story templates in PM settings
- **Details**:
  - List templates with name, story type, description preview
  - Create/edit template: all story fields as configuration
  - "Save current story as template" action in story detail
  - Delete template
- **Files**: `frontend/src/pages/pm/SettingsTemplates.tsx`

---

### 2.17 Frontend — Notifications

#### Task 2.17.1: Notification dropdown
- **Status**: [ ] Not Started
- **Description**: Bell icon with notification list in header
- **Details**:
  - Bell icon in top-right navigation area
  - Unread count badge (red dot with number)
  - Click → dropdown list of notifications
  - Each notification: actor avatar, title, timestamp, entity link
  - Click notification → navigate to entity, mark as read
  - "Mark all as read" button
  - Notification types with icons:
    - Assigned (user icon)
    - Mentioned (@ icon)
    - Comment (message icon)
    - State changed (arrow icon)
    - Blocked (warning icon)
  - Poll for unread count every 30 seconds
- **Files**: `frontend/src/components/pm/NotificationDropdown.tsx`

---

### 2.18 Frontend — Search

#### Task 2.18.1: Search bar component
- **Status**: [ ] Not Started
- **Description**: Global search with operator autocomplete
- **Details**:
  - Search input in PM header/navigation
  - As user types, show autocomplete suggestions for operators
  - Operator prefix detection: `type:`, `is:`, `label:`, `epic:`, `owner:`, etc.
  - After operator prefix, show relevant values (e.g., `label:` → show label list)
  - Free text search on Enter → navigate to search results page or filter current view
  - Recent searches history
- **Files**: `frontend/src/components/pm/SearchBar.tsx`

---

### 2.19 Frontend — Bulk Operations

#### Task 2.19.1: Bulk selection and actions
- **Status**: [ ] Not Started
- **Description**: Select multiple stories and apply bulk actions
- **Details**:
  - Checkboxes on table view rows and board view cards
  - Select all / deselect all
  - Floating action bar when items selected: "X selected" + action buttons
  - Bulk actions: Change State, Assign Owner, Move to Epic, Move to Iteration, Add Label, Remove Label, Change Priority, Archive
  - Confirmation dialog before applying
  - Show success/failure count
- **Files**: `frontend/src/components/pm/BulkActionBar.tsx`

---

### 2.20 Frontend API Services (Phase 2 additions)

#### Task 2.20.1: Sub-task service
- **Files**: `frontend/src/lib/services/pmSubTaskService.ts`

#### Task 2.20.2: Checklist service
- **Files**: `frontend/src/lib/services/pmChecklistService.ts`

#### Task 2.20.3: Story Link service
- **Files**: `frontend/src/lib/services/pmStoryLinkService.ts`

#### Task 2.20.4: Custom Field service
- **Files**: `frontend/src/lib/services/pmCustomFieldService.ts`

#### Task 2.20.5: Template service
- **Files**: `frontend/src/lib/services/pmTemplateService.ts`

#### Task 2.20.6: Saved View service
- **Files**: `frontend/src/lib/services/pmViewService.ts`

#### Task 2.20.7: Notification service
- **Files**: `frontend/src/lib/services/pmNotificationService.ts`

---

## Definition of Done

Phase 2 is complete when:
- [ ] List/Table view renders with sortable columns and inline editing
- [ ] Filter bar works with AND/OR logic across all filter types
- [ ] Saved views can be created, loaded, shared, and deleted
- [ ] Sub-tasks can be created, assigned, estimated, and moved through states
- [ ] Sub-task auto-transitions work (any started → parent started, all done → parent done)
- [ ] Checklists can be added with items, toggled, and promoted to sub-tasks
- [ ] Story links (blocks, relates_to, duplicates) work with blocked indicators
- [ ] Custom fields can be created, configured, and used on stories
- [ ] Story templates can be created and used when creating stories
- [ ] In-app notifications work for assignments, mentions, and state changes
- [ ] Search with operators returns correct results
- [ ] Bulk operations work for common actions (state change, assign, label)

---

## Dependencies

- Phase 1 must be fully complete (all core entities and board view)
- `@tanstack/react-table` for table view
- Consider `cmdk` or similar for search autocomplete

---

## Risk & Mitigations

| Risk | Mitigation |
|------|-----------|
| Filter complexity | Start with simple AND filters, add OR groups later if needed |
| Notification volume | Batch notifications, respect follow rules, add quiet hours later |
| Search performance | Index story title, use pg_trgm for fuzzy matching |
| Custom field schema changes | JSONB for flexibility, limit to single_select initially |
