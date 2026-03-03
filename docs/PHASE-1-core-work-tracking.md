# Phase 1: Core Work Tracking

**Timeline**: Weeks 1-4
**Goal**: Replace Shortcut.com for daily work management. Cancel subscription.
**Status**: Not Started

---

## Overview

This phase delivers the foundational PM entities and the primary Kanban board view. By the end of Phase 1, teams can create stories, organize them on a board, assign to epics and iterations, and track progress — enough to cancel Shortcut.

---

## Task Breakdown

### 1.1 Database Migrations

#### Task 1.1.1: Create PM Workflows & States tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_workflows`, `pm_workflow_states`, and `pm_epic_workflow_states` tables
- **Details**:
  - `pm_workflows`: id, workspace_id, name, description, team_id, default_state_id, auto_assign_owner, timestamps
  - `pm_workflow_states`: id, workflow_id, name, state_type (backlog/unstarted/started/done), position, color, description, wip_limit, is_default, timestamps
  - `pm_epic_workflow_states`: id, workspace_id, name, state_type (unstarted/started/done), position, color, is_default, timestamps
  - Foreign key from pm_workflows.default_state_id → pm_workflow_states.id
  - Apply `set_updated_at()` trigger to all tables
- **Files**: `server/migrations/009_pm_workflows.sql`

#### Task 1.1.2: Create PM Labels table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_labels` table
- **Details**:
  - `pm_labels`: id, workspace_id, name, description, color, archived, timestamps
  - Unique constraint on (workspace_id, name)
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/010_pm_labels.sql`

#### Task 1.1.3: Create PM Epics tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_epics`, `pm_epic_objectives` (placeholder), and `pm_epic_labels` tables
- **Details**:
  - `pm_epics`: id, workspace_id, name, description, epic_state_id, owner_id, team_id, planned_start_date, deadline, started, started_at, completed, completed_at, position, color, health, health_comment, archived, created_by, timestamps
  - `pm_epic_labels`: epic_id, label_id (composite PK)
  - FK: epic_state_id → pm_epic_workflow_states, owner_id → users, team_id → workspace_teams
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/011_pm_epics.sql`

#### Task 1.1.4: Create PM Iterations table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_iterations` and `pm_iteration_labels` tables
- **Details**:
  - `pm_iterations`: id, workspace_id, name, description, start_date, end_date, status (generated column based on dates), team_id, archived, created_by, timestamps
  - `pm_iteration_labels`: iteration_id, label_id (composite PK)
  - CHECK constraint: end_date > start_date
  - Status auto-computed: unstarted (today < start_date), started (between), done (today > end_date)
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/012_pm_iterations.sql`

#### Task 1.1.5: Create PM Stories tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_stories`, `pm_story_owners`, `pm_story_followers`, `pm_story_labels` tables
- **Details**:
  - `pm_stories`: id, workspace_id, display_id (serial sequence), name, description, story_type (feature/bug/chore), workflow_id, workflow_state_id, epic_id, iteration_id, team_id, owner_id, requester_id, estimate, priority, severity, deadline, position, started, started_at, completed, completed_at, moved_at, blocked, blocker, archived, template_id, external_id, timestamps
  - `pm_story_owners`: story_id, user_id (composite PK)
  - `pm_story_followers`: story_id, user_id (composite PK)
  - `pm_story_labels`: story_id, label_id (composite PK)
  - Create sequence `pm_story_display_id_seq`
  - Indexes: workspace, workflow_state, epic, iteration, team, owner, display_id
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/013_pm_stories.sql`

#### Task 1.1.6: Create PM Comments table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_comments` table
- **Details**:
  - `pm_comments`: id, entity_type (story/epic/doc), entity_id, author_id, body, parent_id (self-ref for threads), timestamps
  - Index on (entity_type, entity_id)
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/014_pm_comments.sql`

#### Task 1.1.7: Create PM Activity Log table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_activity_log` table
- **Details**:
  - `pm_activity_log`: id, workspace_id, entity_type, entity_id, actor_id, action, field_name, old_value, new_value, metadata (jsonb), created_at
  - Indexes: (entity_type, entity_id), (workspace_id, created_at DESC)
- **Files**: `server/migrations/015_pm_activity_log.sql`

---

### 1.2 Backend Models

#### Task 1.2.1: Create PM Workflow models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for Workflow, WorkflowState, EpicWorkflowState
- **Details**:
  - GORM models with JSON tags
  - Request structs: CreateWorkflowRequest, UpdateWorkflowRequest, CreateWorkflowStateRequest, UpdateWorkflowStateRequest, ReorderStatesRequest
  - Response structs: WorkflowWithStates
- **Files**: `server/internal/model/pm_workflow.go`

#### Task 1.2.2: Create PM Label models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for Label
- **Details**:
  - GORM model with JSON tags
  - Request structs: CreateLabelRequest, UpdateLabelRequest
- **Files**: `server/internal/model/pm_label.go`

#### Task 1.2.3: Create PM Epic models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for Epic, EpicLabel
- **Details**:
  - GORM model with JSON tags
  - Request structs: CreateEpicRequest, UpdateEpicRequest, UpdateEpicHealthRequest
  - Response structs: EpicWithStats (including computed story counts/points)
- **Files**: `server/internal/model/pm_epic.go`

#### Task 1.2.4: Create PM Iteration models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for Iteration, IterationLabel
- **Details**:
  - GORM model with JSON tags
  - Request structs: CreateIterationRequest, UpdateIterationRequest
  - Response structs: IterationWithStats (including computed story counts/points)
- **Files**: `server/internal/model/pm_iteration.go`

#### Task 1.2.5: Create PM Story models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for Story, StoryOwner, StoryFollower, StoryLabel
- **Details**:
  - GORM model with JSON tags
  - Request structs: CreateStoryRequest, UpdateStoryRequest, MoveStoryRequest, ReorderStoryRequest
  - Response structs: StoryDetail (with owners, labels, epic name, iteration name)
- **Files**: `server/internal/model/pm_story.go`

#### Task 1.2.6: Create PM Comment models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for Comment
- **Details**:
  - GORM model with JSON tags
  - Request structs: CreateCommentRequest, UpdateCommentRequest
  - Response structs: CommentWithAuthor (with user info)
- **Files**: `server/internal/model/pm_comment.go`

#### Task 1.2.7: Create PM Activity Log models
- **Status**: [ ] Not Started
- **Description**: Define Go structs for ActivityLog
- **Details**:
  - GORM model with JSON tags
  - Response structs: ActivityLogEntry (with actor info)
- **Files**: `server/internal/model/pm_activity.go`

---

### 1.3 Backend Repositories

#### Task 1.3.1: Workflow repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for workflows and workflow states
- **Methods**:
  - `ListByWorkspace(ctx, workspaceID)` — list all workflows for a workspace
  - `GetByID(ctx, id)` — get workflow with states
  - `GetByTeamID(ctx, teamID)` — get workflow for a specific team
  - `Create(ctx, workflow)` — create workflow
  - `Update(ctx, workflow)` — update workflow
  - `Delete(ctx, id)` — delete workflow
  - `CreateState(ctx, state)` — add state to workflow
  - `UpdateState(ctx, state)` — update state
  - `DeleteState(ctx, id)` — delete state
  - `ReorderStates(ctx, workflowID, stateIDs)` — reorder states
  - `GetDefaultWorkflow(ctx, workspaceID)` — get the default workflow
  - `SeedDefaultWorkflow(ctx, workspaceID)` — create default workflow + 5 states
  - `SeedDefaultEpicStates(ctx, workspaceID)` — create default epic workflow states
- **Files**: `server/internal/repository/pm_workflow.go`

#### Task 1.3.2: Label repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for labels
- **Methods**:
  - `ListByWorkspace(ctx, workspaceID)` — list all labels
  - `GetByID(ctx, id)` — get label
  - `Create(ctx, label)` — create label
  - `Update(ctx, label)` — update label
  - `Delete(ctx, id)` — delete label
- **Files**: `server/internal/repository/pm_label.go`

#### Task 1.3.3: Epic repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for epics
- **Methods**:
  - `List(ctx, workspaceID, filters)` — list epics with filters (team, state, label, archived)
  - `GetByID(ctx, id)` — get epic with labels
  - `GetWithStats(ctx, id)` — get epic with computed story/point counts
  - `Create(ctx, epic)` — create epic
  - `Update(ctx, epic)` — update epic
  - `Delete(ctx, id)` — delete/archive epic
  - `UpdateHealth(ctx, id, health, comment)` — update health status
  - `AddLabel(ctx, epicID, labelID)` — add label to epic
  - `RemoveLabel(ctx, epicID, labelID)` — remove label from epic
  - `ComputeStats(ctx, epicID)` — compute story counts and point totals
- **Files**: `server/internal/repository/pm_epic.go`

#### Task 1.3.4: Iteration repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for iterations
- **Methods**:
  - `List(ctx, workspaceID, filters)` — list iterations with filters (team, status, archived)
  - `GetByID(ctx, id)` — get iteration
  - `GetWithStats(ctx, id)` — get iteration with computed story/point counts
  - `Create(ctx, iteration)` — create iteration
  - `Update(ctx, iteration)` — update iteration
  - `Delete(ctx, id)` — delete iteration
  - `GetCurrentIteration(ctx, workspaceID, teamID)` — get the active iteration
  - `ComputeStats(ctx, iterationID)` — compute story counts and point totals
- **Files**: `server/internal/repository/pm_iteration.go`

#### Task 1.3.5: Story repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for stories
- **Methods**:
  - `List(ctx, workspaceID, filters, pagination)` — list stories with filters (team, epic, iteration, state, type, owner, label, priority, archived)
  - `GetByID(ctx, id)` — get full story detail (owners, labels, epic, iteration)
  - `GetByDisplayID(ctx, workspaceID, displayID)` — get story by display ID
  - `Create(ctx, story)` — create story (auto-assign display_id, auto-add requester as follower)
  - `Update(ctx, story)` — update story fields
  - `Delete(ctx, id)` — delete/archive story
  - `MoveToState(ctx, storyID, stateID, position)` — change workflow state and position
  - `Reorder(ctx, storyID, position)` — change position within current state
  - `AddOwner(ctx, storyID, userID)` — add owner
  - `RemoveOwner(ctx, storyID, userID)` — remove owner
  - `AddFollower(ctx, storyID, userID)` — add follower
  - `RemoveFollower(ctx, storyID, userID)` — remove follower
  - `AddLabel(ctx, storyID, labelID)` — add label
  - `RemoveLabel(ctx, storyID, labelID)` — remove label
  - `ListByWorkflowState(ctx, workflowID)` — list stories grouped by state (for board view)
  - `CountByState(ctx, workflowID)` — count stories per state
  - `UpdateStartedCompleted(ctx, storyID)` — compute and update started/completed flags based on state type
- **Files**: `server/internal/repository/pm_story.go`

#### Task 1.3.6: Comment repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for comments
- **Methods**:
  - `List(ctx, entityType, entityID)` — list comments with author info
  - `GetByID(ctx, id)` — get comment
  - `Create(ctx, comment)` — create comment
  - `Update(ctx, comment)` — edit comment
  - `Delete(ctx, id)` — delete comment
- **Files**: `server/internal/repository/pm_comment.go`

#### Task 1.3.7: Activity Log repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for activity log
- **Methods**:
  - `List(ctx, entityType, entityID, pagination)` — list activity for an entity
  - `ListByWorkspace(ctx, workspaceID, filters, pagination)` — workspace-wide activity feed
  - `Create(ctx, entry)` — log an activity entry
- **Files**: `server/internal/repository/pm_activity.go`

---

### 1.4 Backend Services

#### Task 1.4.1: Workflow service
- **Status**: [ ] Not Started
- **Description**: Business logic for workflow management
- **Logic**:
  - Validate state type ordering (backlog < unstarted < started < done)
  - Ensure at least one state of each type exists
  - Prevent deletion of states with active stories
  - Seed default workflow on workspace initialization
  - Seed default epic workflow states on workspace initialization
- **Files**: `server/internal/service/pm_workflow.go`

#### Task 1.4.2: Label service
- **Status**: [ ] Not Started
- **Description**: Business logic for label management
- **Logic**:
  - Validate unique name within workspace
  - Handle archive/unarchive
- **Files**: `server/internal/service/pm_label.go`

#### Task 1.4.3: Epic service
- **Status**: [ ] Not Started
- **Description**: Business logic for epic management
- **Logic**:
  - Auto-compute started/completed from child story states
  - Compute stats (story counts, point counts by state)
  - Handle health updates
  - Handle label associations
- **Files**: `server/internal/service/pm_epic.go`

#### Task 1.4.4: Iteration service
- **Status**: [ ] Not Started
- **Description**: Business logic for iteration management
- **Logic**:
  - Validate date ranges (no overlap within same team)
  - Compute stats from child stories
  - Auto-status based on dates
- **Files**: `server/internal/service/pm_iteration.go`

#### Task 1.4.5: Story service
- **Status**: [ ] Not Started
- **Description**: Business logic for story management
- **Logic**:
  - Auto-assign display_id from sequence
  - Auto-add requester as follower on creation
  - Auto-add owner as follower when assigned
  - Compute started/completed timestamps on state transitions
  - Update moved_at on state change
  - Compute blocked/blocker flags (used later with story links)
  - Log activity on every change
  - Validate workflow_state_id belongs to the story's workflow_id
- **Files**: `server/internal/service/pm_story.go`

#### Task 1.4.6: Comment service
- **Status**: [ ] Not Started
- **Description**: Business logic for comments
- **Logic**:
  - Only author can edit/delete their own comments (or admins)
  - Auto-follow entity when commenting
  - Log activity when comment is added
  - Extract @mentions from body (future: trigger notifications)
- **Files**: `server/internal/service/pm_comment.go`

#### Task 1.4.7: Activity service
- **Status**: [ ] Not Started
- **Description**: Business logic for activity logging
- **Logic**:
  - Helper to create activity entries from service layer
  - Format old/new values for display
- **Files**: `server/internal/service/pm_activity.go`

---

### 1.5 Backend Handlers & Routes

#### Task 1.5.1: Workflow handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for workflow CRUD
- **Endpoints**:
  - `GET /api/pm/workflows` — list workflows
  - `POST /api/pm/workflows` — create workflow
  - `GET /api/pm/workflows/{id}` — get workflow with states
  - `PUT /api/pm/workflows/{id}` — update workflow
  - `DELETE /api/pm/workflows/{id}` — delete workflow
  - `POST /api/pm/workflows/{id}/states` — add state
  - `PUT /api/pm/workflows/{id}/states/{stateId}` — update state
  - `DELETE /api/pm/workflows/{id}/states/{stateId}` — delete state
  - `PUT /api/pm/workflows/{id}/states/reorder` — reorder states
- **Files**: `server/internal/handler/pm_workflow.go`

#### Task 1.5.2: Label handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for label CRUD
- **Endpoints**:
  - `GET /api/pm/labels` — list labels
  - `POST /api/pm/labels` — create label
  - `PUT /api/pm/labels/{id}` — update label
  - `DELETE /api/pm/labels/{id}` — delete label
- **Files**: `server/internal/handler/pm_label.go`

#### Task 1.5.3: Epic handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for epic CRUD
- **Endpoints**:
  - `GET /api/pm/epics` — list epics
  - `POST /api/pm/epics` — create epic
  - `GET /api/pm/epics/{id}` — get epic with stats
  - `PUT /api/pm/epics/{id}` — update epic
  - `DELETE /api/pm/epics/{id}` — archive epic
  - `GET /api/pm/epics/{id}/stories` — list stories in epic
  - `PUT /api/pm/epics/{id}/health` — update health
- **Files**: `server/internal/handler/pm_epic.go`

#### Task 1.5.4: Iteration handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for iteration CRUD
- **Endpoints**:
  - `GET /api/pm/iterations` — list iterations
  - `POST /api/pm/iterations` — create iteration
  - `GET /api/pm/iterations/{id}` — get iteration with stats
  - `PUT /api/pm/iterations/{id}` — update iteration
  - `DELETE /api/pm/iterations/{id}` — delete iteration
  - `GET /api/pm/iterations/{id}/stories` — list stories
- **Files**: `server/internal/handler/pm_iteration.go`

#### Task 1.5.5: Story handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for story CRUD and operations
- **Endpoints**:
  - `GET /api/pm/stories` — list stories (filterable, paginated)
  - `POST /api/pm/stories` — create story
  - `GET /api/pm/stories/{id}` — get full story detail
  - `PUT /api/pm/stories/{id}` — update story
  - `DELETE /api/pm/stories/{id}` — archive story
  - `PUT /api/pm/stories/{id}/move` — move to state
  - `PUT /api/pm/stories/{id}/reorder` — change position
  - `POST /api/pm/stories/{id}/owners` — add owner
  - `DELETE /api/pm/stories/{id}/owners/{userId}` — remove owner
  - `POST /api/pm/stories/{id}/followers` — follow
  - `DELETE /api/pm/stories/{id}/followers` — unfollow
  - `POST /api/pm/stories/{id}/labels` — add label
  - `DELETE /api/pm/stories/{id}/labels/{labelId}` — remove label
  - `GET /api/pm/stories/{id}/activity` — get activity log
- **Files**: `server/internal/handler/pm_story.go`

#### Task 1.5.6: Comment handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for comment CRUD
- **Endpoints**:
  - `GET /api/pm/comments` — list comments (query: entity_type, entity_id)
  - `POST /api/pm/comments` — create comment
  - `PUT /api/pm/comments/{id}` — edit comment
  - `DELETE /api/pm/comments/{id}` — delete comment
- **Files**: `server/internal/handler/pm_comment.go`

#### Task 1.5.7: PM Router setup
- **Status**: [ ] Not Started
- **Description**: Register all PM routes under `/api/pm/` prefix
- **Details**:
  - All routes require JWT authentication
  - All routes require X-Workspace-ID header
  - Group routes by entity under the `/api/pm/` prefix
  - Wire up handler dependencies (services → repositories → DB)
- **Files**: `server/internal/router/router.go` (add PM section)

#### Task 1.5.8: Wire PM dependencies in main.go
- **Status**: [ ] Not Started
- **Description**: Initialize PM repositories, services, and handlers in the application entry point
- **Details**:
  - Add PM model structs to GORM auto-migrate
  - Instantiate PM repositories, services, handlers
  - Pass PM handlers to router setup
- **Files**: `server/cmd/api/main.go`

---

### 1.6 Frontend — API Services

#### Task 1.6.1: PM Workflow service
- **Status**: [ ] Not Started
- **Description**: API client for workflow endpoints
- **Files**: `frontend/src/lib/services/pmWorkflowService.ts`

#### Task 1.6.2: PM Label service
- **Status**: [ ] Not Started
- **Description**: API client for label endpoints
- **Files**: `frontend/src/lib/services/pmLabelService.ts`

#### Task 1.6.3: PM Epic service
- **Status**: [ ] Not Started
- **Description**: API client for epic endpoints
- **Files**: `frontend/src/lib/services/pmEpicService.ts`

#### Task 1.6.4: PM Iteration service
- **Status**: [ ] Not Started
- **Description**: API client for iteration endpoints
- **Files**: `frontend/src/lib/services/pmIterationService.ts`

#### Task 1.6.5: PM Story service
- **Status**: [ ] Not Started
- **Description**: API client for story endpoints (CRUD, move, reorder, owners, followers, labels)
- **Files**: `frontend/src/lib/services/pmStoryService.ts`

#### Task 1.6.6: PM Comment service
- **Status**: [ ] Not Started
- **Description**: API client for comment endpoints
- **Files**: `frontend/src/lib/services/pmCommentService.ts`

#### Task 1.6.7: PM Types
- **Status**: [ ] Not Started
- **Description**: TypeScript type definitions for all PM entities
- **Details**:
  - Workflow, WorkflowState, EpicWorkflowState
  - Epic, EpicWithStats
  - Iteration, IterationWithStats
  - Story, StoryDetail, StoryOwner, StoryFollower
  - Label
  - Comment, CommentWithAuthor
  - ActivityLogEntry
  - All request/response types
  - Enums: StoryType, StateType, Priority, Severity, EpicHealth
- **Files**: `frontend/src/lib/pmTypes.ts`

---

### 1.7 Frontend — State Management

#### Task 1.7.1: PM Board store
- **Status**: [ ] Not Started
- **Description**: Zustand store for Kanban board state
- **Details**:
  - Current workflow and states
  - Stories grouped by state (columns)
  - Drag-and-drop state tracking
  - Optimistic updates for moves
  - Board loading/error state
- **Files**: `frontend/src/stores/pmBoardStore.ts`

#### Task 1.7.2: PM Story store
- **Status**: [ ] Not Started
- **Description**: Zustand store for story list and current story
- **Details**:
  - Story list with pagination
  - Current story detail
  - Active filters
  - Loading/error states
- **Files**: `frontend/src/stores/pmStoryStore.ts`

#### Task 1.7.3: PM Workflow store
- **Status**: [ ] Not Started
- **Description**: Zustand store for workflow configuration
- **Details**:
  - Workflows list for workspace
  - Current workflow and states
  - Epic workflow states
- **Files**: `frontend/src/stores/pmWorkflowStore.ts`

---

### 1.8 Frontend — Pages & Components

#### Task 1.8.1: PM Navigation
- **Status**: [ ] Not Started
- **Description**: Add PM section to workspace sidebar navigation
- **Details**:
  - Add separator after existing items
  - Add: Stories, Epics, Iterations links
  - Icons for each item (using Lucide icons from shadcn)
  - Active state highlighting
  - PM routes nested under `/w/{slug}/pm/`

#### Task 1.8.2: Stories page with Board View
- **Status**: [ ] Not Started
- **Description**: Main stories page with Kanban board as default view
- **Details**:
  - Route: `/w/{slug}/pm/stories`
  - Fetch workflow and stories on mount
  - Render workflow states as columns
  - Story cards within columns
  - Column headers: state name, story count, point total
  - "Add story" button at bottom of each column
  - View switcher (Board / List — List is Phase 2)
- **Files**: `frontend/src/pages/pm/Stories.tsx`, route file

#### Task 1.8.3: Kanban Board component
- **Status**: [ ] Not Started
- **Description**: Drag-and-drop board component
- **Details**:
  - Use `@dnd-kit/core` for drag-and-drop
  - Columns for each workflow state
  - Horizontal drag between columns = state change
  - Vertical drag within column = reorder
  - Optimistic updates (move card immediately, revert on API error)
  - Column header with WIP limit indicator (visual only in Phase 1)
  - Scroll within columns for long lists
  - Empty state per column
- **Files**: `frontend/src/components/pm/KanbanBoard.tsx`

#### Task 1.8.4: Story Card component
- **Status**: [ ] Not Started
- **Description**: Card displayed on the Kanban board
- **Details**:
  - Story type icon (feature star, bug icon, chore gear)
  - Display ID (TP-123)
  - Title (truncated at 2 lines)
  - Owner avatar(s) — show first 2, +N for more
  - Estimate badge
  - Priority indicator (colored dot or icon)
  - Label chips (show first 2, +N)
  - Due date (with overdue red highlight)
  - Blocked/Blocker icon
  - Epic name (small muted text)
  - Click → open Story Detail Panel
- **Files**: `frontend/src/components/pm/StoryCard.tsx`

#### Task 1.8.5: Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: Slide-over panel for viewing/editing a story
- **Details**:
  - Opens from right side when a story card is clicked
  - Sections:
    - Header: display ID, story type selector, title (editable)
    - Description: Markdown editor (textarea in Phase 1, rich editor later)
    - Sidebar fields: State, Owner(s), Epic, Iteration, Team, Priority, Severity, Estimate, Labels, Due Date
    - Comments section: list comments, add new comment
    - Activity log: chronological list of changes
  - All field changes auto-save (debounced API calls)
  - Close via X button or click outside
  - URL updates to include story display ID for direct linking
- **Files**: `frontend/src/components/pm/StoryDetailPanel.tsx`

#### Task 1.8.6: Create Story modal
- **Status**: [ ] Not Started
- **Description**: Modal/dialog for creating a new story
- **Details**:
  - Fields: Title (required), Story Type, Description, Team, Epic, Iteration, Priority, Estimate, Labels, Owner
  - Quick create: just title + type, with defaults for the rest
  - Pre-fill workflow state when created from a specific column
  - Pre-fill epic when created from epic detail page
  - Pre-fill iteration when created from iteration detail page
- **Files**: `frontend/src/components/pm/CreateStoryModal.tsx`

#### Task 1.8.7: Epics page
- **Status**: [ ] Not Started
- **Description**: Epics list page
- **Details**:
  - Route: `/w/{slug}/pm/epics`
  - List view of epics with: name, state, owner, team, story progress bar, dates
  - Group by: state (default), team
  - Click epic → navigate to Epic detail page
  - Create Epic button → modal with: name, description, state, owner, team, dates, color
- **Files**: `frontend/src/pages/pm/Epics.tsx`

#### Task 1.8.8: Epic Detail page
- **Status**: [ ] Not Started
- **Description**: Single epic detail view
- **Details**:
  - Route: `/w/{slug}/pm/epics/{id}`
  - Header: Epic name, state, owner, team, dates, health badge
  - Progress bar: stories done / total, points done / total
  - Stories list: sortable table of stories in this epic
  - Add story to epic button
  - Edit epic fields inline
  - Comments section
- **Files**: `frontend/src/pages/pm/EpicDetail.tsx`

#### Task 1.8.9: Iterations page
- **Status**: [ ] Not Started
- **Description**: Iterations list page
- **Details**:
  - Route: `/w/{slug}/pm/iterations`
  - List of iterations: name, dates, status badge, team, story/point progress
  - Group by: status (current/upcoming/past)
  - Click iteration → navigate to Iteration detail page
  - Create Iteration button → modal with: name, description, start_date, end_date, team
- **Files**: `frontend/src/pages/pm/Iterations.tsx`

#### Task 1.8.10: Iteration Detail page
- **Status**: [ ] Not Started
- **Description**: Single iteration detail view
- **Details**:
  - Route: `/w/{slug}/pm/iterations/{id}`
  - Header: Iteration name, dates, status, team
  - Progress: stories done / total, points done / total
  - Stories in this iteration: board view or list
  - Add story to iteration button
  - Description/goals section
- **Files**: `frontend/src/pages/pm/IterationDetail.tsx`

#### Task 1.8.11: PM Routes setup
- **Status**: [ ] Not Started
- **Description**: Configure TanStack Router routes for all PM pages
- **Details**:
  - Nested under `_authenticated/w/$slug/pm/`
  - Index route redirects to stories
  - Story detail route as modal overlay
  - All routes load workspace context
- **Files**: Multiple route files under `frontend/src/routes/_authenticated/w.$slug.pm/`

---

### 1.9 Workspace Initialization

#### Task 1.9.1: Seed default PM data on workspace creation
- **Status**: [ ] Not Started
- **Description**: When a workspace is created (or when PM module is first accessed), seed default data
- **Details**:
  - Create default Workflow with 5 states (Backlog, Ready for Dev, In Development, In Review, Done)
  - Create default Epic workflow states (To Do, In Progress, Done)
  - Create a few default labels (e.g., "frontend", "backend", "design", "infrastructure")
- **Files**: Update `server/internal/service/pm_workflow.go`, add initialization endpoint or hook

---

## Definition of Done

Phase 1 is complete when:
- [ ] All database migrations run successfully
- [ ] Backend API passes manual testing for all endpoints
- [ ] Frontend board view renders stories in correct columns
- [ ] Stories can be created, edited, moved between states via drag-and-drop
- [ ] Epics can be created and stories assigned to them
- [ ] Iterations can be created and stories assigned to them
- [ ] Labels can be created and assigned to stories/epics
- [ ] Comments can be added to stories
- [ ] Activity log tracks state changes and edits
- [ ] Navigation shows PM section in sidebar
- [ ] Story detail panel shows all fields and allows editing
- [ ] Team is actively using the board for daily work

---

## Dependencies

- Existing `users`, `workspaces`, `workspace_members`, `workspace_teams` tables (no changes needed)
- `@dnd-kit/core` npm package for drag-and-drop
- Lucide React icons (already available via shadcn)

---

## Risk & Mitigations

| Risk | Mitigation |
|------|-----------|
| Display ID sequence conflicts in multi-tenant | Use per-workspace sequence or workspace_id + counter approach |
| Board performance with many stories | Paginate within columns, virtualize with react-window if needed |
| Drag-and-drop state sync issues | Optimistic updates with rollback on error, debounce reorder calls |
| Migration conflicts with existing tables | All tables prefixed with `pm_`, no FK to existing bonus tables |
