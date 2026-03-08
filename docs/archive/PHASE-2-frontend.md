# Phase 2 Frontend: Enhanced Features UI

**Timeline**: After Phase 2 Backend is complete
**Goal**: Table view, filtering, saved views, sub-tasks, checklists, dependencies, custom fields, templates, notifications, search, bulk operations.
**Status**: Not Started
**Depends on**: Phase 1 Frontend complete, Phase 2 Backend complete

---

## Overview

This phase adds the second major view (List/Table), the full filtering system, saved views (Spaces), and all the secondary story features: sub-tasks, checklists, story links, custom fields, templates, notifications, and search. By the end, the PM module matches the team's daily Shortcut usage.

---

## Task Breakdown

### F2.1 API Services & Types (Phase 2 additions)

#### Task F2.1.1: Update PM types for Phase 2 entities
- **Status**: [ ] Not Started
- **Description**: Add TypeScript types for all Phase 2 entities
- **Details**:
  - Interfaces: `SubTask`, `Checklist`, `ChecklistItem`, `StoryLink`, `CustomField`, `CustomFieldOption`, `CustomFieldValue`, `StoryTemplate`, `SavedView`, `Notification`
  - Request types: `CreateSubTaskRequest`, `UpdateSubTaskRequest`, `MoveSubTaskRequest`, `CreateChecklistRequest`, `CreateChecklistItemRequest`, `CreateStoryLinkRequest`, `CreateCustomFieldRequest`, `CreateTemplateRequest`, `CreateSavedViewRequest`, `BulkUpdateStoriesRequest`
  - Enums: `LinkType` (blocks/relates_to/duplicates), `NotificationType`, `ViewType` (board/list/timeline)
  - Filter types: `FilterGroup`, `FilterCondition`, `FilterOperator`
- **Files**: Update `frontend/src/lib/pmTypes.ts`

#### Task F2.1.2: Sub-task API service
- **Status**: [ ] Not Started
- **Description**: API client for sub-task endpoints
- **Methods**:
  - `listSubTasks(storyId)` → `GET /api/pm/stories/{storyId}/sub-tasks`
  - `createSubTask(storyId, data)` → `POST /api/pm/stories/{storyId}/sub-tasks`
  - `updateSubTask(id, data)` → `PUT /api/pm/sub-tasks/{id}`
  - `deleteSubTask(id)` → `DELETE /api/pm/sub-tasks/{id}`
  - `moveSubTask(id, stateId)` → `PUT /api/pm/sub-tasks/{id}/move`
  - `reorderSubTasks(storyId, subTaskIds)` → `PUT /api/pm/sub-tasks/reorder`
- **Files**: `frontend/src/lib/services/pmSubTaskService.ts`

#### Task F2.1.3: Checklist API service
- **Status**: [ ] Not Started
- **Description**: API client for checklist endpoints
- **Methods**:
  - `createChecklist(storyId, data)` → `POST /api/pm/stories/{storyId}/checklists`
  - `updateChecklist(id, data)` → `PUT /api/pm/checklists/{id}`
  - `deleteChecklist(id)` → `DELETE /api/pm/checklists/{id}`
  - `createItem(checklistId, data)` → `POST /api/pm/checklists/{id}/items`
  - `updateItem(id, data)` → `PUT /api/pm/checklist-items/{id}`
  - `deleteItem(id)` → `DELETE /api/pm/checklist-items/{id}`
  - `promoteToSubTask(itemId)` → `POST /api/pm/checklist-items/{id}/promote`
  - `reorderItems(checklistId, itemIds)` → `PUT /api/pm/checklists/{id}/items/reorder`
- **Files**: `frontend/src/lib/services/pmChecklistService.ts`

#### Task F2.1.4: Story Link API service
- **Status**: [ ] Not Started
- **Description**: API client for story link endpoints
- **Methods**:
  - `createLink(storyId, data)` → `POST /api/pm/stories/{id}/links`
  - `deleteLink(storyId, linkId)` → `DELETE /api/pm/stories/{id}/links/{linkId}`
- **Files**: `frontend/src/lib/services/pmStoryLinkService.ts`

#### Task F2.1.5: Custom Field API service
- **Status**: [ ] Not Started
- **Description**: API client for custom field endpoints
- **Methods**:
  - `listFields()` → `GET /api/pm/custom-fields`
  - `createField(data)` → `POST /api/pm/custom-fields`
  - `updateField(id, data)` → `PUT /api/pm/custom-fields/{id}`
  - `deleteField(id)` → `DELETE /api/pm/custom-fields/{id}`
  - `createOption(fieldId, data)` → `POST /api/pm/custom-fields/{id}/options`
  - `updateOption(id, data)` → `PUT /api/pm/custom-field-options/{id}`
  - `deleteOption(id)` → `DELETE /api/pm/custom-field-options/{id}`
- **Files**: `frontend/src/lib/services/pmCustomFieldService.ts`

#### Task F2.1.6: Template API service
- **Status**: [ ] Not Started
- **Description**: API client for template endpoints
- **Methods**:
  - `listTemplates()` → `GET /api/pm/templates`
  - `getTemplate(id)` → `GET /api/pm/templates/{id}`
  - `createTemplate(data)` → `POST /api/pm/templates`
  - `updateTemplate(id, data)` → `PUT /api/pm/templates/{id}`
  - `deleteTemplate(id)` → `DELETE /api/pm/templates/{id}`
- **Files**: `frontend/src/lib/services/pmTemplateService.ts`

#### Task F2.1.7: Saved View API service
- **Status**: [ ] Not Started
- **Description**: API client for saved view endpoints
- **Methods**:
  - `listViews()` → `GET /api/pm/views`
  - `createView(data)` → `POST /api/pm/views`
  - `updateView(id, data)` → `PUT /api/pm/views/{id}`
  - `deleteView(id)` → `DELETE /api/pm/views/{id}`
- **Files**: `frontend/src/lib/services/pmViewService.ts`

#### Task F2.1.8: Notification API service
- **Status**: [ ] Not Started
- **Description**: API client for notification endpoints
- **Methods**:
  - `listNotifications(page?)` → `GET /api/pm/notifications`
  - `getUnreadCount()` → `GET /api/pm/notifications/unread-count`
  - `markAsRead(id)` → `PUT /api/pm/notifications/{id}/read`
  - `markAllAsRead()` → `PUT /api/pm/notifications/read-all`
- **Files**: `frontend/src/lib/services/pmNotificationService.ts`

#### Task F2.1.9: Search API service
- **Status**: [ ] Not Started
- **Description**: API client for story search
- **Methods**:
  - `searchStories(query, pagination?)` → `GET /api/pm/stories/search?q={query}`
  - `bulkUpdateStories(data)` → `POST /api/pm/stories/bulk`
- **Files**: Update `frontend/src/lib/services/pmStoryService.ts`

---

### F2.2 State Management (Phase 2 additions)

#### Task F2.2.1: Filter store
- **Status**: [ ] Not Started
- **Description**: Zustand store for filter state management
- **State**:
  - `filterGroups: FilterGroup[]` — AND/OR filter groups
  - `activeFilters: FilterCondition[]` — flattened active filters for API
  - `filterCount: number` — badge count
- **Actions**:
  - `addFilter(group, condition)` — add filter to group
  - `removeFilter(group, conditionId)` — remove filter
  - `toggleFilter(conditionId)` — enable/disable without removing
  - `clearAllFilters()` — reset
  - `setFilterGroup(group)` — set AND/OR mode
  - `serializeToParams()` — convert to URL query params
  - `serializeToJSON()` — convert to JSONB for saved views
  - `loadFromParams(params)` — restore from URL
  - `loadFromJSON(json)` — restore from saved view
- **Files**: `frontend/src/stores/pmFilterStore.ts`

#### Task F2.2.2: Notification store
- **Status**: [ ] Not Started
- **Description**: Zustand store for notification state
- **State**:
  - `notifications: Notification[]`
  - `unreadCount: number`
  - `isLoading: boolean`
- **Actions**:
  - `fetchNotifications(page?)` — load notifications
  - `fetchUnreadCount()` — poll unread count
  - `markAsRead(id)` — mark single read
  - `markAllAsRead()` — mark all read
  - `startPolling()` — poll unread count every 30s
  - `stopPolling()` — cleanup
- **Files**: `frontend/src/stores/pmNotificationStore.ts`

---

### F2.3 List/Table View

#### Task F2.3.1: Story Table component
- **Status**: [ ] Not Started
- **Description**: Full-featured table view for stories
- **Details**:
  - **Library**: `@tanstack/react-table` for sorting, grouping, column visibility
  - **Default columns**: Checkbox, Display ID, Type Icon, Title, State, Priority, Owner(s), Epic, Iteration, Estimate, Labels, Due Date, Updated
  - **Column features**:
    - Click header to sort (asc/desc/none)
    - Resize columns by dragging header border
    - Show/hide columns via column visibility menu (gear icon)
  - **Inline editing**: Click cell to edit:
    - State → dropdown
    - Priority → dropdown
    - Owner → user picker
    - Estimate → number input
    - Labels → label picker
    - Epic → epic picker
    - Iteration → iteration picker
  - **Row interactions**:
    - Click row → open Story Detail Panel
    - Checkbox → select for bulk actions
    - Right-click → context menu (Open, Copy ID, Move to Epic, Archive)
  - **Grouping**: Dropdown to group by: None, Epic, Iteration, Owner, Team, Label, State, Priority
  - **Performance**: Virtualize rows for large lists (use `@tanstack/react-virtual`)
- **Files**: `frontend/src/components/pm/StoryTable.tsx`

#### Task F2.3.2: View switcher update
- **Status**: [ ] Not Started
- **Description**: Enable Board/List toggle on Stories page
- **Details**:
  - Toggle buttons: Board (Kanban icon) | List (Table icon)
  - Persist selection in localStorage key: `pm-view-type`
  - Both views share: filters, current workflow
  - Board uses pmBoardStore, List uses pmStoryService.listStories()
  - Smooth transition between views
- **Files**: Update `frontend/src/pages/pm/Stories.tsx`

#### Task F2.3.3: Install TanStack Table
- **Status**: [ ] Not Started
- **Packages**: `@tanstack/react-table`, `@tanstack/react-virtual`

---

### F2.4 Filtering System

#### Task F2.4.1: Filter Bar component
- **Status**: [ ] Not Started
- **Description**: Reusable filter bar for stories page
- **Details**:
  - Positioned below page header, above board/table
  - **"Add Filter" button**: Opens dropdown of filter types:
    - Team, Epic, Iteration, Owner, Requester, Label, Story Type, Workflow State, Priority, Severity, Custom Fields, Estimate, Due Date
  - **Each active filter**: Chip showing "Type: Value" with X to remove
  - **Filter value selection**: Each filter type has its own value selector:
    - Entity-based (Epic, Iteration, Team, Owner): searchable multi-select dropdown
    - Enum-based (Type, State, Priority): checkbox list
    - Range-based (Estimate): min/max number inputs
    - Date-based (Due Date): date range picker (from/to)
    - Custom Field: per-field option selector
  - **Filter logic toggle**: "Matches All" (AND) / "Matches Any" (OR) switch
  - **Toggle filters**: Click chip to disable/enable without removing (dimmed visual)
  - **Clear all**: Button to remove all filters
  - **Filter count badge**: Shows on "Filters" button when collapsed
  - Filters applied to both Board and Table views
  - Filters synced to URL query params for sharing
- **Files**: `frontend/src/components/pm/FilterBar.tsx`

#### Task F2.4.2: Filter value selector components
- **Status**: [ ] Not Started
- **Description**: Individual filter value selectors per type
- **Details**:
  - `EntityFilterSelect` — for Epic, Iteration, Team, Owner, Requester (searchable multi-select with avatars/names)
  - `EnumFilterSelect` — for Story Type, State, Priority, Severity (checkbox list)
  - `RangeFilterInput` — for Estimate (min/max number)
  - `DateRangeFilterInput` — for Due Date, Created Date (from/to date pickers)
  - `CustomFieldFilterSelect` — dynamic per custom field (option checkboxes)
  - All emit standardized `FilterCondition` objects
- **Files**: `frontend/src/components/pm/filters/` (directory with individual components)

---

### F2.5 Saved Views (Spaces)

#### Task F2.5.1: Spaces tabs component
- **Status**: [ ] Not Started
- **Description**: Tab bar for saved views at top of Stories page
- **Details**:
  - Horizontal tab bar below page title, above filter bar
  - Tabs: "All Stories" (default) + saved views
  - Each tab: name + shared icon (if shared)
  - Click tab → load view config (filters, group_by, sort, view_type, columns)
  - Active tab highlighted
  - "+" button at end to save current view
  - Right-click / three-dot menu on tab: Rename, Share/Unshare, Duplicate, Delete
  - Drag tabs to reorder
  - Overflow: horizontal scroll or "more" dropdown
- **Files**: `frontend/src/components/pm/SpacesTabs.tsx`

#### Task F2.5.2: Save View dialog
- **Status**: [ ] Not Started
- **Description**: Dialog to save current view configuration as a Space
- **Details**:
  - Fields: Name (required), Share with workspace (toggle)
  - Captures current: view_type, filters (serialized), group_by, sort_by, sort_order, column_config
  - Saves via API
  - New tab appears in Spaces tabs
- **Files**: `frontend/src/components/pm/SaveViewDialog.tsx`

---

### F2.6 Story Detail Panel Additions

#### Task F2.6.1: Sub-tasks section
- **Status**: [ ] Not Started
- **Description**: Sub-task management in Story Detail Panel
- **Details**:
  - Section title: "Sub-tasks" with count badge and progress (e.g., "2/5")
  - Progress bar above list (completed/total)
  - **Sub-task list**: Each item shows:
    - State badge (colored dot matching state type)
    - Name (editable inline on click)
    - Owner avatar (clickable → user picker)
    - Estimate badge (clickable → estimate selector)
    - Three-dot menu: Edit, Delete
  - **Add sub-task**: Input at bottom of list, Enter to create
  - **Drag to reorder**: Vertical drag within list
  - **State change**: Click state badge → dropdown of workflow states
  - **Expand/collapse**: Click sub-task name → expand to show description field
- **Files**: `frontend/src/components/pm/SubTaskSection.tsx`

#### Task F2.6.2: Checklists section
- **Status**: [ ] Not Started
- **Description**: Checklist management in Story Detail Panel
- **Details**:
  - Section title: "Checklists"
  - **Add checklist button**: "Add Checklist" → name input → creates new checklist
  - **Each checklist**:
    - Checklist name (editable, bold)
    - Progress: "3 of 7" with mini progress bar
    - Items list:
      - Checkbox (toggle complete)
      - Text (editable on click, markdown supported)
      - Assignee avatar (small, clickable → user picker)
      - Three-dot menu: Promote to Sub-task, Delete
    - "Add item" input at bottom of each checklist
    - Drag items to reorder
  - **Checklist actions**: Three-dot menu on checklist header: Rename, Delete (with confirm)
  - Visual: completed items show strikethrough text, dimmed
- **Files**: `frontend/src/components/pm/ChecklistSection.tsx`

#### Task F2.6.3: Relationships section
- **Status**: [ ] Not Started
- **Description**: Story links and dependencies in Story Detail Panel
- **Details**:
  - Section title: "Relationships"
  - **Grouped by type**:
    - "Blocks" — stories this one blocks
    - "Blocked by" — stories blocking this one
    - "Relates to" — related stories
    - "Duplicates" / "Duplicated by"
  - **Each linked story**: Display ID (TP-123) + Title (truncated) + State badge + X remove button
  - **Click linked story** → navigate to that story (update detail panel)
  - **"Add Relationship" button**:
    - Step 1: Select type (Blocks, Blocked by, Relates to, Duplicates)
    - Step 2: Search for story (by ID or title)
    - Step 3: Confirm → creates link
  - **Blocked indicator**: If story has active "blocked by" links, show red "Blocked" badge at top of detail panel
- **Files**: `frontend/src/components/pm/RelationshipsSection.tsx`

#### Task F2.6.4: Custom Fields section
- **Status**: [ ] Not Started
- **Description**: Custom field values in Story Detail Panel sidebar
- **Details**:
  - Appears in sidebar below built-in fields
  - Each custom field: Label + value selector
  - **Single select**: Dropdown showing field options with colored dots
  - **Clear value**: "None" option at top of dropdown
  - Only show enabled fields
  - Fields ordered by position
  - On change: API call to set/update value
- **Files**: `frontend/src/components/pm/CustomFieldsSection.tsx`

---

### F2.7 Templates

#### Task F2.7.1: Template selector in Create Story modal
- **Status**: [ ] Not Started
- **Description**: Add template selection to story creation
- **Details**:
  - "Use Template" dropdown at top of Create Story modal
  - Lists all workspace templates with name and story type icon
  - Selecting a template populates: type, description, team, epic, state, priority, estimate, labels, custom fields, checklist items
  - User can modify any pre-filled field before creating
  - "No Template" as default/first option
- **Files**: Update `frontend/src/components/pm/CreateStoryModal.tsx`

#### Task F2.7.2: Template management in PM Settings
- **Status**: [ ] Not Started
- **Description**: Admin page for managing story templates
- **Details**:
  - Route: `/w/{slug}/pm/settings` (Templates tab)
  - **List**: Template name, story type, description preview, created by, actions (edit/delete)
  - **Create/Edit form**: Same fields as Create Story modal, but saved as template
  - **Delete**: Confirm dialog, doesn't affect stories already created from template
  - **"Save as Template" action**: Available in Story Detail Panel three-dot menu → saves current story config as new template
- **Files**: `frontend/src/pages/pm/PMSettings.tsx` (Templates tab)

---

### F2.8 Custom Fields Admin

#### Task F2.8.1: Custom Fields management in PM Settings
- **Status**: [ ] Not Started
- **Description**: Admin page for managing custom fields
- **Details**:
  - Route: `/w/{slug}/pm/settings` (Custom Fields tab)
  - **List**: Field name, type, option count, enabled toggle, drag to reorder, actions (edit/delete)
  - **Create field form**:
    - Name (required, max 64 chars)
    - Description (optional)
    - Icon selector
    - Options list: add/remove/reorder options. Each option has: value text, color picker
  - **Edit field**: Same form, pre-populated. Can add/remove/reorder options.
  - **Delete field**: Confirm dialog warning "This will remove this field from all stories"
  - **Enable/disable toggle**: Disabled fields hidden from story forms but data preserved
- **Files**: `frontend/src/pages/pm/PMSettings.tsx` (Custom Fields tab)

#### Task F2.8.2: Custom Fields in Table View
- **Status**: [ ] Not Started
- **Description**: Show custom fields as columns in the table view
- **Details**:
  - Column visibility menu includes custom fields
  - Each custom field renders as a colored badge in its column
  - Click cell → dropdown to change value
  - Sortable by custom field value
  - Group by custom field supported
- **Files**: Update `frontend/src/components/pm/StoryTable.tsx`

---

### F2.9 Notifications

#### Task F2.9.1: Notification dropdown component
- **Status**: [ ] Not Started
- **Description**: Bell icon with notification list in app header
- **Details**:
  - **Bell icon**: In top-right area of the app layout (near user avatar)
  - **Unread badge**: Red dot with count (max "99+")
  - **Click → dropdown panel**:
    - Header: "Notifications" + "Mark all as read" link
    - Scrollable list of notifications
    - Each notification:
      - Actor avatar (small)
      - Title text: "Jane assigned you to TP-123"
      - Timestamp: "2 hours ago" (relative)
      - Unread indicator: blue dot
      - Notification type icon (assigned, mentioned, comment, state_changed, blocked)
    - Click notification → navigate to entity, mark as read
    - Empty state: "You're all caught up!"
  - **Polling**: Fetch unread count every 30 seconds
  - **Dropdown positioning**: Below bell icon, right-aligned
  - Max height with scroll for long lists
- **Files**: `frontend/src/components/pm/NotificationDropdown.tsx`

#### Task F2.9.2: Notification integration in app layout
- **Status**: [ ] Not Started
- **Description**: Add notification bell to the authenticated layout
- **Details**:
  - Place bell icon in the top navigation bar
  - Initialize polling on mount (when in PM context)
  - Stop polling on unmount
  - Share across all PM pages
- **Files**: Update authenticated layout / app header component

---

### F2.10 Search

#### Task F2.10.1: Search bar component
- **Status**: [ ] Not Started
- **Description**: Global search with operator autocomplete
- **Details**:
  - **Location**: PM header area (above board/table, next to filter bar)
  - **Input**: Text field with search icon, placeholder "Search stories..."
  - **Autocomplete**:
    - Detect operator prefix (e.g., typing "type:" shows story type options)
    - Supported operators: `type:`, `is:`, `has:`, `label:`, `epic:`, `owner:`, `team:`, `estimate:`, `priority:`, `iteration:`, `created:`
    - After prefix, show relevant value suggestions
    - Free text: show "Search for '{text}'" option
  - **Enter**: Execute search, display results
  - **Results**: Replace current story list/board with search results
  - **Clear search**: X button returns to normal view
  - **Keyboard**: `Cmd+K` or `/` to focus search bar
  - Use `cmdk` library or custom implementation
- **Files**: `frontend/src/components/pm/SearchBar.tsx`

---

### F2.11 Bulk Operations

#### Task F2.11.1: Bulk selection and action bar
- **Status**: [ ] Not Started
- **Description**: Select multiple stories and apply bulk actions
- **Details**:
  - **Selection**:
    - Table view: checkbox column, header checkbox for select all
    - Board view: hold Cmd/Ctrl + click to multi-select cards (selected cards get blue border)
  - **Floating action bar**: Appears at bottom of screen when items selected
    - Left: "X stories selected" + "Deselect all"
    - Right: Action buttons:
      - "Move to State" → state dropdown
      - "Assign Owner" → user picker
      - "Set Epic" → epic picker
      - "Set Iteration" → iteration picker
      - "Add Label" → label picker
      - "Set Priority" → priority dropdown
      - "Archive" → confirm dialog
  - **Applying action**: Shows progress, then success toast "Updated X stories"
  - **Error handling**: "Failed to update Y stories" with retry
  - Animated slide-up/down of action bar
- **Files**: `frontend/src/components/pm/BulkActionBar.tsx`

---

### F2.12 PM Settings Page

#### Task F2.12.1: PM Settings page shell
- **Status**: [ ] Not Started
- **Description**: Settings page with tabs for PM configuration
- **Details**:
  - Route: `/w/{slug}/pm/settings`
  - **Tabs**: Workflows | Custom Fields | Templates
  - Only accessible to owner/admin roles
  - Add "PM Settings" link to sidebar navigation (under Settings or as sub-item)
- **Files**: `frontend/src/pages/pm/PMSettings.tsx`

#### Task F2.12.2: Workflow settings tab
- **Status**: [ ] Not Started
- **Description**: Manage workflows and states
- **Details**:
  - List workflows with name, team, state count
  - Select workflow → show states editor:
    - States grouped by type (Backlog, Unstarted, Started, Done)
    - Each state: name (editable), color picker, WIP limit input
    - Drag to reorder within type
    - Add state button within each type group
    - Delete state (blocked if stories exist in that state)
  - Create new workflow: name, team assignment
  - **Epic workflow states**: Separate section for managing epic states
- **Files**: `frontend/src/components/pm/settings/WorkflowSettings.tsx`

---

### F2.13 Install Dependencies

#### Task F2.13.1: Install npm packages for Phase 2
- **Status**: [ ] Not Started
- **Packages**:
  - `@tanstack/react-table` — table view
  - `@tanstack/react-virtual` — row virtualization
  - `cmdk` — command palette / search autocomplete (optional, can build custom)

---

## Definition of Done

Phase 2 Frontend is complete when:
- [ ] Table view renders with sortable columns, inline editing, and grouping
- [ ] View toggle (Board/List) switches seamlessly with shared filters
- [ ] Filter bar adds/removes/toggles filters with AND/OR logic
- [ ] Filters sync to URL params and apply to both views
- [ ] Saved views (Spaces) can be created, loaded, shared, deleted as tabs
- [ ] Sub-tasks display in story detail with state tracking, estimates, and auto-transitions
- [ ] Checklists work with toggle, reorder, promote-to-subtask
- [ ] Story relationships (blocks, relates_to, duplicates) display correctly with blocked indicator
- [ ] Custom fields appear in story sidebar and table columns
- [ ] Story templates populate fields when creating stories
- [ ] PM Settings page manages workflows, custom fields, and templates
- [ ] Notification bell shows unread count and dropdown list
- [ ] Search with operators returns filtered results
- [ ] Bulk operations apply changes to multiple selected stories
- [ ] No console errors or TypeScript warnings
