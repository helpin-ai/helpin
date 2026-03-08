# Phase 1 Frontend: Core Work Tracking UI

**Timeline**: After Phase 1 Backend is complete
**Goal**: Board view, story management, epics, iterations — enough to replace Shortcut daily usage.
**Status**: Not Started
**Depends on**: Phase 1 Backend (all API endpoints working)

---

## Overview

This phase builds the foundational PM user interface: sidebar navigation, Kanban board with drag-and-drop, story cards, story detail panel, epic and iteration pages. The team should be able to do all daily work management through these interfaces.

---

## Task Breakdown

### F1.1 TypeScript Types & API Services

#### Task F1.1.1: PM TypeScript type definitions
- **Status**: [ ] Not Started
- **Description**: Define all TypeScript types for Phase 1 PM entities
- **Details**:
  - Enums: `StoryType` (feature/bug/chore), `StateType` (backlog/unstarted/started/done), `Priority` (none/low/medium/high/urgent), `Severity` (none/low/medium/high/critical), `EpicHealth` (on_track/at_risk/off_track), `IterationStatus` (unstarted/started/done)
  - Interfaces:
    - `Workflow`, `WorkflowState`, `WorkflowWithStates`
    - `EpicWorkflowState`
    - `Label`
    - `Epic`, `EpicWithStats`, `EpicStats`
    - `Iteration`, `IterationWithStats`, `IterationStats`
    - `Story`, `StoryDetail`, `StoryOwner`, `StoryFollower`
    - `Comment`, `CommentWithAuthor`
    - `ActivityLogEntry`
  - Request types:
    - `CreateWorkflowRequest`, `UpdateWorkflowRequest`, `CreateWorkflowStateRequest`, `UpdateWorkflowStateRequest`, `ReorderStatesRequest`
    - `CreateLabelRequest`, `UpdateLabelRequest`
    - `CreateEpicRequest`, `UpdateEpicRequest`, `UpdateEpicHealthRequest`
    - `CreateIterationRequest`, `UpdateIterationRequest`
    - `CreateStoryRequest`, `UpdateStoryRequest`, `MoveStoryRequest`, `ReorderStoryRequest`
    - `CreateCommentRequest`, `UpdateCommentRequest`
  - Pagination types: `PaginatedResponse<T>`, `PaginationParams`
  - Filter types: `StoryFilters`, `EpicFilters`, `IterationFilters`
- **Files**: `frontend/src/lib/pmTypes.ts`

#### Task F1.1.2: PM Workflow API service
- **Status**: [ ] Not Started
- **Description**: API client for workflow endpoints
- **Methods**:
  - `listWorkflows(workspaceId)` → `GET /api/pm/workflows`
  - `getWorkflow(id)` → `GET /api/pm/workflows/{id}`
  - `createWorkflow(data)` → `POST /api/pm/workflows`
  - `updateWorkflow(id, data)` → `PUT /api/pm/workflows/{id}`
  - `deleteWorkflow(id)` → `DELETE /api/pm/workflows/{id}`
  - `createState(workflowId, data)` → `POST /api/pm/workflows/{id}/states`
  - `updateState(workflowId, stateId, data)` → `PUT /api/pm/workflows/{id}/states/{stateId}`
  - `deleteState(workflowId, stateId)` → `DELETE /api/pm/workflows/{id}/states/{stateId}`
  - `reorderStates(workflowId, stateIds)` → `PUT /api/pm/workflows/{id}/states/reorder`
- **Files**: `frontend/src/lib/services/pmWorkflowService.ts`

#### Task F1.1.3: PM Label API service
- **Status**: [ ] Not Started
- **Description**: API client for label endpoints
- **Methods**:
  - `listLabels()` → `GET /api/pm/labels`
  - `createLabel(data)` → `POST /api/pm/labels`
  - `updateLabel(id, data)` → `PUT /api/pm/labels/{id}`
  - `deleteLabel(id)` → `DELETE /api/pm/labels/{id}`
- **Files**: `frontend/src/lib/services/pmLabelService.ts`

#### Task F1.1.4: PM Epic API service
- **Status**: [ ] Not Started
- **Description**: API client for epic endpoints
- **Methods**:
  - `listEpics(filters?)` → `GET /api/pm/epics`
  - `getEpic(id)` → `GET /api/pm/epics/{id}`
  - `createEpic(data)` → `POST /api/pm/epics`
  - `updateEpic(id, data)` → `PUT /api/pm/epics/{id}`
  - `deleteEpic(id)` → `DELETE /api/pm/epics/{id}`
  - `getEpicStories(id)` → `GET /api/pm/epics/{id}/stories`
  - `updateEpicHealth(id, data)` → `PUT /api/pm/epics/{id}/health`
- **Files**: `frontend/src/lib/services/pmEpicService.ts`

#### Task F1.1.5: PM Iteration API service
- **Status**: [ ] Not Started
- **Description**: API client for iteration endpoints
- **Methods**:
  - `listIterations(filters?)` → `GET /api/pm/iterations`
  - `getIteration(id)` → `GET /api/pm/iterations/{id}`
  - `createIteration(data)` → `POST /api/pm/iterations`
  - `updateIteration(id, data)` → `PUT /api/pm/iterations/{id}`
  - `deleteIteration(id)` → `DELETE /api/pm/iterations/{id}`
  - `getIterationStories(id)` → `GET /api/pm/iterations/{id}/stories`
- **Files**: `frontend/src/lib/services/pmIterationService.ts`

#### Task F1.1.6: PM Story API service
- **Status**: [ ] Not Started
- **Description**: API client for story endpoints
- **Methods**:
  - `listStories(filters?, pagination?)` → `GET /api/pm/stories`
  - `getStory(id)` → `GET /api/pm/stories/{id}`
  - `createStory(data)` → `POST /api/pm/stories`
  - `updateStory(id, data)` → `PUT /api/pm/stories/{id}`
  - `deleteStory(id)` → `DELETE /api/pm/stories/{id}`
  - `moveStory(id, stateId, position)` → `PUT /api/pm/stories/{id}/move`
  - `reorderStory(id, position)` → `PUT /api/pm/stories/{id}/reorder`
  - `addOwner(storyId, userId)` → `POST /api/pm/stories/{id}/owners`
  - `removeOwner(storyId, userId)` → `DELETE /api/pm/stories/{id}/owners/{userId}`
  - `followStory(id)` → `POST /api/pm/stories/{id}/followers`
  - `unfollowStory(id)` → `DELETE /api/pm/stories/{id}/followers`
  - `addLabel(storyId, labelId)` → `POST /api/pm/stories/{id}/labels`
  - `removeLabel(storyId, labelId)` → `DELETE /api/pm/stories/{id}/labels/{labelId}`
  - `getActivity(storyId)` → `GET /api/pm/stories/{id}/activity`
- **Files**: `frontend/src/lib/services/pmStoryService.ts`

#### Task F1.1.7: PM Comment API service
- **Status**: [ ] Not Started
- **Description**: API client for comment endpoints
- **Methods**:
  - `listComments(entityType, entityId)` → `GET /api/pm/comments?entity_type=&entity_id=`
  - `createComment(data)` → `POST /api/pm/comments`
  - `updateComment(id, data)` → `PUT /api/pm/comments/{id}`
  - `deleteComment(id)` → `DELETE /api/pm/comments/{id}`
- **Files**: `frontend/src/lib/services/pmCommentService.ts`

---

### F1.2 State Management (Zustand Stores)

#### Task F1.2.1: PM Workflow store
- **Status**: [ ] Not Started
- **Description**: Zustand store for workflow and state data
- **State**:
  - `workflows: Workflow[]` — all workflows for workspace
  - `currentWorkflow: WorkflowWithStates | null` — active workflow with states
  - `epicWorkflowStates: EpicWorkflowState[]` — epic states
  - `isLoading: boolean`
  - `error: string | null`
- **Actions**:
  - `fetchWorkflows(workspaceId)` — load all workflows
  - `fetchWorkflow(id)` — load single workflow with states
  - `setCurrentWorkflow(workflow)` — set active workflow
  - `fetchEpicWorkflowStates(workspaceId)` — load epic states
- **Files**: `frontend/src/stores/pmWorkflowStore.ts`

#### Task F1.2.2: PM Board store
- **Status**: [ ] Not Started
- **Description**: Zustand store for Kanban board state
- **State**:
  - `columns: Map<string, Story[]>` — stories grouped by workflow state ID
  - `stateOrder: string[]` — ordered state IDs for column rendering
  - `dragState: { storyId: string | null, sourceStateId: string | null }` — active drag tracking
  - `isLoading: boolean`
  - `error: string | null`
- **Actions**:
  - `fetchBoardData(workflowId)` — load stories grouped by state
  - `moveStory(storyId, targetStateId, position)` — optimistic move + API call
  - `reorderStory(storyId, newPosition)` — optimistic reorder + API call
  - `addStoryToColumn(stateId, story)` — after create
  - `removeStoryFromColumn(storyId)` — after delete/archive
  - `updateStoryInColumn(story)` — after edit
  - `setDragState(state)` — track drag-and-drop
  - `revertMove(storyId, originalStateId, originalPosition)` — rollback on API error
- **Files**: `frontend/src/stores/pmBoardStore.ts`

#### Task F1.2.3: PM Story store
- **Status**: [ ] Not Started
- **Description**: Zustand store for current story detail and story list
- **State**:
  - `currentStory: StoryDetail | null` — currently open story
  - `isDetailOpen: boolean` — whether detail panel is open
  - `isLoading: boolean`
  - `error: string | null`
- **Actions**:
  - `openStory(storyId)` — fetch and open story detail panel
  - `closeStory()` — close detail panel
  - `updateCurrentStory(updates)` — optimistic update + API call
  - `refreshCurrentStory()` — re-fetch current story
- **Files**: `frontend/src/stores/pmStoryStore.ts`

---

### F1.3 Routing

#### Task F1.3.1: PM route configuration
- **Status**: [ ] Not Started
- **Description**: Set up TanStack Router routes for all PM pages
- **Details**:
  - Route tree under `_authenticated/w/$slug/pm/`:
    ```
    pm/
    ├── index.tsx          → Redirect to /stories
    ├── stories.tsx        → Stories page (board view)
    ├── epics/
    │   ├── index.tsx      → Epics list page
    │   └── $epicId.tsx    → Epic detail page
    ├── iterations/
    │   ├── index.tsx      → Iterations list page
    │   └── $iterationId.tsx → Iteration detail page
    ```
  - All routes wrapped in authenticated layout
  - All routes load workspace context from slug
  - Story detail displayed as overlay panel (not a separate route — controlled by store)
- **Files**:
  - `frontend/src/routes/_authenticated/w.$slug.pm/index.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/stories.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/epics/index.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/epics/$epicId.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/iterations/index.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/iterations/$iterationId.tsx`

---

### F1.4 Navigation

#### Task F1.4.1: Add PM section to workspace sidebar
- **Status**: [ ] Not Started
- **Description**: Add Project Management links to the workspace sidebar navigation
- **Details**:
  - Add a visual separator after existing nav items (Dashboard, Goals, Sprints, Bonus)
  - Add new section "Project Management" with items:
    - **Stories** — icon: `LayoutList` or `Kanban` → `/w/{slug}/pm/stories`
    - **Epics** — icon: `Layers` → `/w/{slug}/pm/epics`
    - **Iterations** — icon: `RefreshCw` or `Timer` → `/w/{slug}/pm/iterations`
  - Active state: highlight current page
  - Use Lucide React icons (already in project via shadcn)
  - Collapse/expand section (optional)
- **Files**: Update existing sidebar component (find and modify)

---

### F1.5 Stories Page & Board View

#### Task F1.5.1: Stories page shell
- **Status**: [ ] Not Started
- **Description**: Main stories page with header and board container
- **Details**:
  - Page header with:
    - Title: "Stories"
    - View switcher: Board (active) | List (disabled, coming in Phase 2)
    - "Create Story" button (primary action)
  - On mount:
    - Fetch default workflow for workspace
    - Fetch stories grouped by workflow state
    - Render Kanban board
  - Story Detail Panel overlays from right when a story is opened
  - URL params: optionally track `?story=TP-123` for direct linking
- **Files**: `frontend/src/pages/pm/Stories.tsx`

#### Task F1.5.2: Kanban Board component
- **Status**: [ ] Not Started
- **Description**: Drag-and-drop board with workflow state columns
- **Details**:
  - **Library**: Use `@dnd-kit/core` + `@dnd-kit/sortable`
  - **Layout**: Horizontal scrollable row of columns
  - **Columns**: One per workflow state, ordered by state position
  - **Column header**:
    - State name
    - Story count badge
    - Total points badge
    - State color indicator (left border or dot)
  - **Column body**:
    - Vertical list of StoryCard components
    - Scrollable when stories overflow
    - Drop zone highlighting when dragging over
    - "+" button at bottom for quick create
  - **Drag behavior**:
    - Horizontal drag between columns → API call to `moveStory(storyId, newStateId, position)`
    - Vertical drag within column → API call to `reorderStory(storyId, newPosition)`
    - Optimistic update: move card immediately in UI
    - On API error: revert card to original position, show error toast
  - **DragOverlay**: Show a ghost card following cursor during drag
  - **Performance**: Consider virtualization if columns have 50+ stories
- **Files**: `frontend/src/components/pm/KanbanBoard.tsx`

#### Task F1.5.3: Story Card component
- **Status**: [ ] Not Started
- **Description**: Card rendered on the Kanban board for each story
- **Details**:
  - **Layout** (compact card, ~120-160px height):
    - Top row: Story type icon + Display ID (TP-123) in muted text
    - Title: 1-2 lines, truncated with ellipsis
    - Bottom row (flex, spaced):
      - Left: Label chips (first 2 + "+N" overflow badge)
      - Right: Owner avatar(s) (first 2 + "+N")
    - Optional indicators (small icons/badges):
      - Priority dot (colored: gray/blue/yellow/orange/red for none/low/medium/high/urgent)
      - Estimate badge (small rounded pill with point number)
      - Due date (text, red if overdue, amber if within 3 days)
      - Blocked icon (red lock) / Blocker icon (yellow warning)
      - Epic name (small muted text above title)
  - **Story type icons**:
    - Feature: `Star` icon (yellow)
    - Bug: `Bug` icon (red)
    - Chore: `Settings` / `Wrench` icon (gray)
  - **Interactions**:
    - Click → open Story Detail Panel via pmStoryStore.openStory()
    - Drag handle → entire card is draggable
  - **Styling**: shadcn Card, subtle shadow, rounded-lg, hover state
- **Files**: `frontend/src/components/pm/StoryCard.tsx`

#### Task F1.5.4: Quick Create Story (inline)
- **Status**: [ ] Not Started
- **Description**: Inline story creation at the bottom of a board column
- **Details**:
  - Click "+" at bottom of column → expands to a mini form
  - Fields: Title input (required), story type selector (feature/bug/chore)
  - Enter → creates story in that column's workflow state
  - Escape → collapse form
  - New story appears at bottom of the column
  - Auto-focus title input when expanded
- **Files**: `frontend/src/components/pm/QuickCreateStory.tsx`

---

### F1.6 Story Detail Panel

#### Task F1.6.1: Story Detail Panel shell
- **Status**: [ ] Not Started
- **Description**: Slide-over panel for viewing/editing story details
- **Details**:
  - **Position**: Right side of screen, overlays board content
  - **Width**: ~500-600px, or responsive (40-50% of screen)
  - **Open/Close**: Animated slide from right. Close via X button, Escape key, or click on backdrop
  - **Header**:
    - Display ID badge (TP-123, clickable to copy link)
    - Story type selector (dropdown: feature/bug/chore)
    - Title: large editable text, auto-save on blur/debounce
    - Close button (X)
  - **Two-column layout inside panel**:
    - Main content (left ~60%): Description, Sub-tasks (Phase 2), Checklists (Phase 2), Comments, Activity
    - Sidebar fields (right ~40%): State, Owner(s), Epic, Iteration, Team, Priority, Severity, Estimate, Labels, Due Date
  - **Auto-save**: All field changes debounced (300ms) and auto-saved via API
  - **Loading**: Skeleton while fetching story detail
- **Files**: `frontend/src/components/pm/StoryDetailPanel.tsx`

#### Task F1.6.2: Story Detail — sidebar fields
- **Status**: [ ] Not Started
- **Description**: Editable sidebar fields in the story detail panel
- **Details**:
  - **State**: Dropdown showing all workflow states, grouped by type. Selecting changes state via `moveStory` API.
  - **Owner(s)**: Multi-select user picker. Shows avatars. Search by name. Add/remove via API.
  - **Epic**: Single-select dropdown listing workspace epics. Clearable. Changes `epic_id`.
  - **Iteration**: Single-select dropdown listing workspace iterations. Group by status (current/upcoming/past). Changes `iteration_id`.
  - **Team**: Single-select dropdown listing workspace teams. Changes `team_id`.
  - **Priority**: Single-select: None, Low, Medium, High, Urgent. Color-coded icons.
  - **Severity**: Single-select (shown only for Bug type): None, Low, Medium, High, Critical.
  - **Estimate**: Number input or button group (1, 2, 3, 5, 8, 13). Clearable.
  - **Labels**: Multi-select with color chips. Search/filter. Create new label inline.
  - **Due Date**: Date picker. Shows "overdue" badge if past.
  - Each field: label on left, value/control on right. Click to edit.
- **Files**: `frontend/src/components/pm/StoryDetailSidebar.tsx`

#### Task F1.6.3: Story Detail — description editor
- **Status**: [ ] Not Started
- **Description**: Description editing area in story detail panel
- **Details**:
  - Textarea with markdown support (Phase 1: plain textarea, Phase 4: upgrade to TipTap)
  - Placeholder: "Add a description..."
  - Auto-save on blur or after 1s debounce
  - Show "Saving..." / "Saved" indicator
  - Support basic markdown preview (optional in Phase 1)
  - Expandable: auto-grow as content increases
- **Files**: `frontend/src/components/pm/StoryDescription.tsx`

#### Task F1.6.4: Story Detail — comments section
- **Status**: [ ] Not Started
- **Description**: Comment thread in story detail panel
- **Details**:
  - **Comment list**: Chronological, newest at bottom
  - Each comment:
    - Author avatar + name + timestamp ("2 hours ago")
    - Body text (markdown rendered)
    - Edit button (own comments only) → inline edit mode
    - Delete button (own comments only) → confirm dialog
  - **New comment input**:
    - Textarea at bottom
    - "Comment" submit button (or Cmd+Enter)
    - Auto-focus not enabled (user clicks to start)
  - **Threading**: Phase 1 flat list. Phase 2 can add threading.
  - Empty state: "No comments yet. Start the conversation."
- **Files**: `frontend/src/components/pm/CommentSection.tsx`

#### Task F1.6.5: Story Detail — activity log section
- **Status**: [ ] Not Started
- **Description**: Chronological activity feed in story detail panel
- **Details**:
  - Toggle: "Comments" / "Activity" / "All" tabs
  - Activity entries:
    - Actor avatar + "Actor Name action_description" + timestamp
    - Examples:
      - "Jane moved this story from Ready for Dev to In Development"
      - "John changed priority from Medium to High"
      - "Jane assigned Mike as owner"
      - "John added label frontend"
      - "Jane set estimate to 5"
  - Compact display: small text, timeline-style layout
  - Load more / paginate for long histories
- **Files**: `frontend/src/components/pm/ActivityLog.tsx`

---

### F1.7 Create Story Modal

#### Task F1.7.1: Create Story modal/dialog
- **Status**: [ ] Not Started
- **Description**: Full story creation dialog
- **Details**:
  - Triggered by: "Create Story" button in page header, or Spaces where quick create isn't enough
  - **Fields**:
    - Title (required, text input)
    - Story Type (feature/bug/chore radio or button group)
    - Description (textarea, optional)
    - Team (dropdown)
    - Epic (dropdown, optional)
    - Iteration (dropdown, optional)
    - Priority (dropdown)
    - Estimate (number input / button group)
    - Owner(s) (multi-select user picker)
    - Labels (multi-select)
    - Due Date (date picker)
  - **Pre-fill behavior**:
    - When opened from a board column: pre-fill workflow state
    - When opened from epic detail: pre-fill epic
    - When opened from iteration detail: pre-fill iteration
  - **Submit**: Create button, or Cmd+Enter
  - **After create**: Close modal, add story to board, show success toast
  - Use shadcn Dialog component
- **Files**: `frontend/src/components/pm/CreateStoryModal.tsx`

---

### F1.8 Epics Pages

#### Task F1.8.1: Epics list page
- **Status**: [ ] Not Started
- **Description**: Page showing all epics with progress
- **Details**:
  - Route: `/w/{slug}/pm/epics`
  - **Page header**: Title "Epics", "Create Epic" button
  - **List view** (card-based or table):
    - Each epic row/card:
      - Epic name (link to detail)
      - State badge (To Do / In Progress / Done) with color
      - Owner avatar + name
      - Team name
      - Progress bar: stories done / total (with point counts)
      - Date range: start → deadline
      - Health badge (on_track green / at_risk amber / off_track red)
    - Group by: State (default), Team
    - Sort by: Position, Name, Updated, Deadline
  - **Empty state**: "No epics yet. Create an epic to organize related stories."
  - **Filtering**: By state, team, owner, label (basic — full filter bar in Phase 2)
- **Files**: `frontend/src/pages/pm/Epics.tsx`

#### Task F1.8.2: Create/Edit Epic modal
- **Status**: [ ] Not Started
- **Description**: Modal for creating or editing an epic
- **Details**:
  - **Fields**: Name (required), Description (textarea), State (dropdown of epic workflow states), Owner (user picker), Team (dropdown), Planned Start Date, Deadline, Color picker, Labels (multi-select)
  - For edit: pre-populate all fields, update on save
  - Use shadcn Dialog
- **Files**: `frontend/src/components/pm/EpicModal.tsx`

#### Task F1.8.3: Epic Detail page
- **Status**: [ ] Not Started
- **Description**: Single epic view with stories list
- **Details**:
  - Route: `/w/{slug}/pm/epics/{epicId}`
  - **Header section**:
    - Epic name (large, editable inline)
    - State dropdown (editable)
    - Owner, Team, Dates (editable inline)
    - Health badge with update action
    - Progress stats: X of Y stories done, X of Y points done
    - Progress bar (visual)
  - **Description section**: Editable textarea (same as story description)
  - **Stories section**:
    - Table of stories in this epic: Display ID, Title, Type, State, Owner, Priority, Estimate
    - Sortable columns
    - Click story → open Story Detail Panel
    - "Add Story" button (opens Create Story modal with epic pre-filled)
    - "Add Existing Story" → search and assign stories to this epic
  - **Comments section**: Same CommentSection component
  - **Breadcrumb**: Epics > Epic Name
- **Files**: `frontend/src/pages/pm/EpicDetail.tsx`

---

### F1.9 Iterations Pages

#### Task F1.9.1: Iterations list page
- **Status**: [ ] Not Started
- **Description**: Page showing all iterations grouped by status
- **Details**:
  - Route: `/w/{slug}/pm/iterations`
  - **Page header**: Title "Iterations", "Create Iteration" button
  - **Grouped sections**:
    - **Current** (status = started): highlighted section at top
    - **Upcoming** (status = unstarted): next section
    - **Completed** (status = done): collapsed by default, expandable
  - Each iteration card:
    - Iteration name
    - Date range: start → end
    - Status badge (started=blue, unstarted=gray, done=green)
    - Team name
    - Progress: X of Y stories done, X of Y points
    - Progress bar
    - Days remaining (for active iterations)
  - Click → navigate to Iteration Detail page
  - **Empty state**: "No iterations yet. Create an iteration to start sprint planning."
- **Files**: `frontend/src/pages/pm/Iterations.tsx`

#### Task F1.9.2: Create/Edit Iteration modal
- **Status**: [ ] Not Started
- **Description**: Modal for creating or editing an iteration
- **Details**:
  - **Fields**: Name (required), Description (textarea), Start Date (required), End Date (required), Team (dropdown)
  - Date validation: end must be after start
  - Duration display: "2 weeks" calculated from dates
  - For edit: pre-populate, update on save
- **Files**: `frontend/src/components/pm/IterationModal.tsx`

#### Task F1.9.3: Iteration Detail page
- **Status**: [ ] Not Started
- **Description**: Single iteration view with stories
- **Details**:
  - Route: `/w/{slug}/pm/iterations/{iterationId}`
  - **Header section**:
    - Iteration name (editable)
    - Date range + status badge
    - Team name
    - Days remaining / "Completed X days ago"
    - Progress stats: stories done, points done, points remaining
    - Progress bar
  - **Description section**: Editable textarea for sprint goals/notes
  - **Stories section**:
    - Mini Kanban board (same board component, scoped to this iteration's stories)
    - Toggle: Board / List view
    - "Add Story" button (pre-fill iteration)
    - "Add Existing Story" → search and assign to iteration
  - **Breadcrumb**: Iterations > Iteration Name
- **Files**: `frontend/src/pages/pm/IterationDetail.tsx`

---

### F1.10 Shared Components

#### Task F1.10.1: User Picker component
- **Status**: [ ] Not Started
- **Description**: Reusable dropdown to select workspace members
- **Details**:
  - Search by name
  - Show: avatar + full name
  - Single-select and multi-select variants
  - Clear button
  - Used in: Story sidebar (owner), Epic modal (owner), Create Story modal
  - Fetches workspace members list (cached)
- **Files**: `frontend/src/components/pm/UserPicker.tsx`

#### Task F1.10.2: Label Picker component
- **Status**: [ ] Not Started
- **Description**: Multi-select dropdown for labels with color chips
- **Details**:
  - Search/filter labels
  - Each option: colored dot + label name
  - Selected labels shown as colored chips
  - "Create new label" action at bottom of dropdown
  - Used in: Story sidebar, Epic modal, Create Story modal
- **Files**: `frontend/src/components/pm/LabelPicker.tsx`

#### Task F1.10.3: Entity Picker components (Epic, Iteration, Team)
- **Status**: [ ] Not Started
- **Description**: Reusable select dropdowns for Epic, Iteration, and Team
- **Details**:
  - **Epic Picker**: Search epics by name, show state badge, clearable
  - **Iteration Picker**: Show iterations grouped by status (current/upcoming/past), clearable
  - **Team Picker**: Simple dropdown of workspace teams, clearable
  - All fetch their data and cache it
- **Files**: `frontend/src/components/pm/EntityPickers.tsx`

#### Task F1.10.4: Priority and Estimate selectors
- **Status**: [ ] Not Started
- **Description**: Reusable selector components for priority and estimates
- **Details**:
  - **Priority Selector**: Dropdown with colored indicators (none=gray, low=blue, medium=yellow, high=orange, urgent=red)
  - **Severity Selector**: Same pattern, only shown for Bug type stories
  - **Estimate Selector**: Button group (1, 2, 3, 5, 8, 13) or number input. Selected value highlighted. Click again to deselect.
- **Files**: `frontend/src/components/pm/PrioritySelector.tsx`, `frontend/src/components/pm/EstimateSelector.tsx`

---

### F1.11 Install Dependencies

#### Task F1.11.1: Install npm packages
- **Status**: [ ] Not Started
- **Description**: Install required npm packages for Phase 1
- **Packages**:
  - `@dnd-kit/core` — drag-and-drop core
  - `@dnd-kit/sortable` — sortable lists (for board columns)
  - `@dnd-kit/utilities` — CSS utilities for drag
  - Verify existing packages: `lucide-react` (icons), `date-fns` (dates), `zustand` (state), `sonner` or shadcn toast (notifications)
- **Files**: `frontend/package.json`

---

## Definition of Done

Phase 1 Frontend is complete when:
- [ ] PM section appears in workspace sidebar navigation
- [ ] Stories page loads and displays Kanban board with workflow state columns
- [ ] Stories can be created via quick create (inline) and full modal
- [ ] Story cards display: type icon, ID, title, owner, priority, estimate, labels, epic
- [ ] Drag-and-drop moves stories between columns (state change) and within columns (reorder)
- [ ] Optimistic updates work — card moves immediately, reverts on API error
- [ ] Story Detail Panel opens from right, shows all fields, auto-saves edits
- [ ] Comments can be added, edited, and deleted on stories
- [ ] Activity log shows chronological changes
- [ ] Epics list page shows epics with progress bars
- [ ] Epic detail page shows stories list with add/assign capability
- [ ] Iterations list page shows iterations grouped by status
- [ ] Iteration detail page shows stories with mini board
- [ ] All pages have loading and empty states
- [ ] No console errors or TypeScript warnings

---

## Component Tree

```
Stories Page
├── PageHeader (title, view switcher, create button)
├── KanbanBoard
│   ├── DndContext (from @dnd-kit)
│   │   ├── BoardColumn (per workflow state)
│   │   │   ├── ColumnHeader (name, count, points)
│   │   │   ├── SortableContext
│   │   │   │   └── StoryCard (per story)
│   │   │   └── QuickCreateStory
│   │   └── DragOverlay (ghost card)
├── StoryDetailPanel (slide-over)
│   ├── StoryDetailHeader (ID, type, title, close)
│   ├── StoryDescription
│   ├── StoryDetailSidebar
│   │   ├── StateSelector
│   │   ├── UserPicker (owners)
│   │   ├── EpicPicker
│   │   ├── IterationPicker
│   │   ├── TeamPicker
│   │   ├── PrioritySelector
│   │   ├── EstimateSelector
│   │   ├── LabelPicker
│   │   └── DatePicker (due date)
│   ├── CommentSection
│   └── ActivityLog
└── CreateStoryModal
```

---

## Design Notes

- **Color scheme**: Follow existing shadcn/ui theme (dark/light mode support)
- **Spacing**: Use Tailwind spacing scale consistently (p-4, gap-3, etc.)
- **Typography**: Match existing app typography (font sizes, weights)
- **Animations**: Subtle transitions for panel open/close, card drag. Use Tailwind `transition` classes.
- **Responsive**: Board scrolls horizontally on smaller screens. Detail panel goes full-width on screens < 1024px.
