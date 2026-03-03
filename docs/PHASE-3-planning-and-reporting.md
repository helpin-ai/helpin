# Phase 3: Planning & Reporting

**Timeline**: Weeks 9-12
**Goal**: Strategic planning capabilities and data-driven insights.
**Status**: Not Started
**Depends on**: Phase 2 complete

---

## Overview

This phase adds Objectives (OKRs), Key Results, the Timeline/Roadmap view, and all reporting charts (velocity, burndown, cumulative flow, cycle time, lead time). By the end of Phase 3, the team has full planning and performance visibility.

---

## Task Breakdown

### 3.1 Database Migrations

#### Task 3.1.1: Create PM Objectives & Key Results tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_objectives`, `pm_key_results`, and `pm_epic_objectives` tables
- **Details**:
  - `pm_objectives`: id, workspace_id, name, description, objective_type (tactical/strategic), state, planned_start_date, deadline, started, started_at, completed, completed_at, owner_id, position, archived, created_by, timestamps
  - `pm_key_results`: id, objective_id, name, result_type (boolean/percent/numeric), initial_value, current_value, target_value, progress (0-100), position, timestamps
  - `pm_epic_objectives`: epic_id, objective_id (composite PK) — many-to-many link
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/023_pm_objectives.sql`

#### Task 3.1.2: Add snapshot tables for historical reporting
- **Status**: [ ] Not Started
- **Description**: Create tables to capture daily snapshots for burndown/CFD charts
- **Details**:
  - `pm_iteration_snapshots`: id, iteration_id, snapshot_date, stories_backlog, stories_unstarted, stories_started, stories_done, points_backlog, points_unstarted, points_started, points_done
  - `pm_epic_snapshots`: id, epic_id, snapshot_date, stories_backlog, stories_unstarted, stories_started, stories_done, points_backlog, points_unstarted, points_started, points_done
  - UNIQUE: (iteration_id, snapshot_date), (epic_id, snapshot_date)
  - These are populated by a daily cron job or on-demand
- **Files**: `server/migrations/024_pm_snapshots.sql`

---

### 3.2 Backend — Objectives & Key Results

#### Task 3.2.1: Objective models
- **Status**: [ ] Not Started
- **Description**: Go structs for Objective, KeyResult, EpicObjective
- **Details**:
  - GORM models with JSON tags
  - CreateObjectiveRequest, UpdateObjectiveRequest
  - CreateKeyResultRequest, UpdateKeyResultRequest
  - ObjectiveWithDetails (includes key results, epic count, progress)
- **Files**: `server/internal/model/pm_objective.go`

#### Task 3.2.2: Objective repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for objectives and key results
- **Methods**:
  - `List(ctx, workspaceID, filters)` — list objectives (filter by state, type, archived)
  - `GetByID(ctx, id)` — with key results and linked epics
  - `Create(ctx, objective)`, `Update`, `Delete`
  - `CreateKeyResult(ctx, kr)`, `UpdateKeyResult`, `DeleteKeyResult`
  - `LinkEpic(ctx, objectiveID, epicID)` — add epic to objective
  - `UnlinkEpic(ctx, objectiveID, epicID)` — remove epic
  - `ComputeProgress(ctx, objectiveID)` — calculate progress from epics/key results
- **Files**: `server/internal/repository/pm_objective.go`

#### Task 3.2.3: Objective service
- **Status**: [ ] Not Started
- **Description**: Business logic for objectives
- **Logic**:
  - Tactical objective progress: computed from linked epic story completion
  - Strategic objective progress: weighted average of key result progress
  - Auto-compute started/completed from epic states
  - Elevate tactical → strategic when first key result is added
  - Validate key result progress (0-100)
  - Log activity on changes
- **Files**: `server/internal/service/pm_objective.go`

#### Task 3.2.4: Objective handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for objective CRUD
- **Endpoints**:
  - `GET /api/pm/objectives` — list with filters
  - `POST /api/pm/objectives` — create
  - `GET /api/pm/objectives/{id}` — get with key results and epics
  - `PUT /api/pm/objectives/{id}` — update
  - `DELETE /api/pm/objectives/{id}` — delete
  - `POST /api/pm/objectives/{id}/key-results` — add key result
  - `PUT /api/pm/objectives/{id}/key-results/{krId}` — update key result
  - `DELETE /api/pm/objectives/{id}/key-results/{krId}` — delete key result
  - `POST /api/pm/objectives/{id}/epics` — link epic
  - `DELETE /api/pm/objectives/{id}/epics/{epicId}` — unlink epic
- **Files**: `server/internal/handler/pm_objective.go`

---

### 3.3 Backend — Reports

#### Task 3.3.1: Report models
- **Status**: [ ] Not Started
- **Description**: Go structs for report data responses
- **Details**:
  - VelocityDataPoint: iteration_name, start_date, end_date, features_completed, bugs_completed, chores_completed, feature_points, bug_points, chore_points, total_points
  - BurndownDataPoint: date, total_work, remaining_work, ideal_remaining
  - CumulativeFlowDataPoint: date, backlog, unstarted, started, done
  - CycleTimeDataPoint: story_id, display_id, story_name, story_type, completed_at, cycle_time_hours
  - LeadTimeDataPoint: story_id, display_id, story_name, story_type, completed_at, lead_time_hours
- **Files**: `server/internal/model/pm_report.go`

#### Task 3.3.2: Report repository
- **Status**: [ ] Not Started
- **Description**: SQL queries for report data
- **Methods**:
  - `GetVelocityData(ctx, workspaceID, filters)` — completed points per iteration over last N iterations
  - `GetBurndownData(ctx, scopeType, scopeID)` — daily remaining work from snapshots
  - `GetCumulativeFlowData(ctx, scopeType, scopeID)` — daily story distribution from snapshots
  - `GetCycleTimeData(ctx, workspaceID, filters)` — cycle time for completed stories
  - `GetLeadTimeData(ctx, workspaceID, filters)` — lead time for completed stories
  - `GetVelocityAverages(ctx, workspaceID, teamID)` — overall avg and trailing 4-iteration avg
- **Files**: `server/internal/repository/pm_report.go`

#### Task 3.3.3: Snapshot service
- **Status**: [ ] Not Started
- **Description**: Service to capture daily snapshots for burndown/CFD
- **Logic**:
  - `CaptureIterationSnapshots(ctx, workspaceID)` — snapshot all active iterations
  - `CaptureEpicSnapshots(ctx, workspaceID)` — snapshot all active epics
  - Can be called:
    - On demand via API endpoint
    - Periodically via a goroutine timer (daily at midnight UTC)
    - On story state change (lightweight update to today's snapshot)
- **Files**: `server/internal/service/pm_snapshot.go`

#### Task 3.3.4: Report service
- **Status**: [ ] Not Started
- **Description**: Business logic for report data aggregation
- **Logic**:
  - Velocity: Aggregate completed stories/points per iteration, compute averages
  - Burndown: From snapshots, compute ideal line from total → 0 over date range
  - CFD: From snapshots, stacked area data
  - Cycle time: completed_at - started_at for each completed story
  - Lead time: completed_at - created_at for each completed story
  - Apply filters: team, epic, iteration, label, story_type, date_range
- **Files**: `server/internal/service/pm_report.go`

#### Task 3.3.5: Report handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for reports
- **Endpoints**:
  - `GET /api/pm/reports/velocity?team_id=&date_from=&date_to=`
  - `GET /api/pm/reports/burndown?scope=iteration&id=&unit=points|stories`
  - `GET /api/pm/reports/cumulative-flow?scope=iteration&id=`
  - `GET /api/pm/reports/cycle-time?team_id=&story_type=&date_from=&date_to=`
  - `GET /api/pm/reports/lead-time?team_id=&story_type=&date_from=&date_to=`
  - `POST /api/pm/reports/snapshot` — trigger manual snapshot capture
- **Files**: `server/internal/handler/pm_report.go`

#### Task 3.3.6: Add burndown/CFD endpoints to Epic and Iteration handlers
- **Status**: [ ] Not Started
- **Description**: Add report data endpoints on existing entity handlers
- **Endpoints**:
  - `GET /api/pm/epics/{id}/burndown`
  - `GET /api/pm/epics/{id}/cumulative-flow`
  - `GET /api/pm/iterations/{id}/burndown`
  - `GET /api/pm/iterations/{id}/cumulative-flow`
- **Files**: Update `server/internal/handler/pm_epic.go`, `server/internal/handler/pm_iteration.go`

---

### 3.4 Frontend — Objectives Page

#### Task 3.4.1: Objectives list page
- **Status**: [ ] Not Started
- **Description**: Page showing all objectives grouped by state
- **Details**:
  - Route: `/w/{slug}/pm/objectives`
  - Group by state: To Do, In Progress, Done
  - Each objective card: name, type badge (tactical/strategic), owner, deadline, progress bar
  - Progress bar: shows % of epics completed (tactical) or key results progress (strategic)
  - Filter by: state, type, owner
  - Create Objective button
- **Files**: `frontend/src/pages/pm/Objectives.tsx`

#### Task 3.4.2: Objective Detail page
- **Status**: [ ] Not Started
- **Description**: Single objective detail view
- **Details**:
  - Route: `/w/{slug}/pm/objectives/{id}`
  - Header: name, type, state, owner, dates, overall progress
  - Key Results section (strategic only):
    - List key results with: name, type, current/target, progress bar
    - Inline edit key result values
    - Add new key result
  - Linked Epics section:
    - List epics with: name, state, story progress, team
    - Link existing epic button (search and select)
    - Unlink epic
  - Edit objective fields inline
  - Comments section
- **Files**: `frontend/src/pages/pm/ObjectiveDetail.tsx`

#### Task 3.4.3: Create/Edit Objective modal
- **Status**: [ ] Not Started
- **Description**: Modal for creating or editing an objective
- **Details**:
  - Fields: name, description, type (tactical/strategic), state, owner, planned_start_date, deadline
  - If strategic: Key Results section with add/edit/remove
  - Link epics during creation
- **Files**: `frontend/src/components/pm/ObjectiveModal.tsx`

---

### 3.5 Frontend — Timeline/Roadmap View

#### Task 3.5.1: Timeline page
- **Status**: [ ] Not Started
- **Description**: Gantt-style horizontal timeline of epics
- **Details**:
  - Route: `/w/{slug}/pm/roadmap`
  - X-axis: calendar timeline with week/month/quarter zoom levels
  - Y-axis: epics as horizontal bars
  - Bar length: planned_start_date → deadline
  - Bar color: by team color or epic state color
  - Drag bar body: move start and end dates together
  - Drag bar edges: resize to change duration
  - Today line: vertical red line at current date
  - Group by: Team, Objective
  - Filter by: Team, Objective, Epic State, Labels, Owner
  - Epic health badge on each bar
  - Click bar → navigate to Epic detail
  - Zoom controls: Week, Month, Quarter
- **Files**: `frontend/src/pages/pm/Roadmap.tsx`

#### Task 3.5.2: Timeline bar component
- **Status**: [ ] Not Started
- **Description**: Individual epic bar on the timeline
- **Details**:
  - Renders at correct position based on dates
  - Shows: epic name (truncated), progress mini-bar, health dot
  - Draggable (body and handles)
  - Tooltip on hover: full name, dates, team, story progress
  - Visual distinction: started (solid), unstarted (outline/dashed), done (muted)
- **Files**: `frontend/src/components/pm/TimelineBar.tsx`

#### Task 3.5.3: Timeline grid and header
- **Status**: [ ] Not Started
- **Description**: Calendar grid and date headers for timeline
- **Details**:
  - Date headers at top (weeks, months, or quarters depending on zoom)
  - Vertical grid lines on date boundaries
  - Horizontal alternating row backgrounds for readability
  - Scroll horizontally to navigate time
  - Snap-to-week when dragging bars
- **Files**: `frontend/src/components/pm/TimelineGrid.tsx`

---

### 3.6 Frontend — Reports Dashboard

#### Task 3.6.1: Reports page
- **Status**: [ ] Not Started
- **Description**: Reports dashboard with all charts
- **Details**:
  - Route: `/w/{slug}/pm/reports`
  - Tab navigation: Velocity | Burndown | Cumulative Flow | Cycle Time | Lead Time
  - Global filters: Team, Date Range
  - Each tab renders the corresponding chart component
- **Files**: `frontend/src/pages/pm/Reports.tsx`

#### Task 3.6.2: Velocity Chart component
- **Status**: [ ] Not Started
- **Description**: Bar chart showing completed work per iteration
- **Details**:
  - Stacked bars: Feature (green), Bug (red), Chore (blue)
  - Toggle: story count or points
  - Dotted line: overall average
  - Solid line: trailing 4-iteration average
  - Hover: show iteration name, date range, breakdown by type
  - X-axis: iteration names, Y-axis: count or points
  - Use Recharts or Chart.js
- **Files**: `frontend/src/components/pm/charts/VelocityChart.tsx`

#### Task 3.6.3: Burndown Chart component
- **Status**: [ ] Not Started
- **Description**: Line chart showing remaining work over time
- **Details**:
  - Scope selector: Iteration or Epic (dropdown)
  - Two lines: Ideal (dashed gray) and Actual (solid blue)
  - Toggle: story count or points
  - X-axis: dates, Y-axis: remaining work
  - Hover: date, remaining stories/points
  - Also embedded in Iteration Detail and Epic Detail pages
- **Files**: `frontend/src/components/pm/charts/BurndownChart.tsx`

#### Task 3.6.4: Cumulative Flow Diagram component
- **Status**: [ ] Not Started
- **Description**: Stacked area chart of story distribution over time
- **Details**:
  - Scope selector: Iteration or Epic
  - Stacked areas: one per workflow state (colored to match state colors)
  - X-axis: dates, Y-axis: number of stories
  - Hover: date, count per state
  - Widening bands = bottleneck signal
  - Also embedded in Iteration Detail and Epic Detail pages
- **Files**: `frontend/src/components/pm/charts/CumulativeFlowDiagram.tsx`

#### Task 3.6.5: Cycle Time Chart component
- **Status**: [ ] Not Started
- **Description**: Scatter plot of cycle times for completed stories
- **Details**:
  - Dots: each completed story (color by type: feature/bug/chore)
  - X-axis: completion date, Y-axis: cycle time (hours or days)
  - Dotted line: overall average
  - Solid line: 7-day trailing average
  - Toggle: linear or logarithmic scale
  - Hover: story name, display ID, cycle time, dates
  - Filter by story type
- **Files**: `frontend/src/components/pm/charts/CycleTimeChart.tsx`

#### Task 3.6.6: Lead Time Chart component
- **Status**: [ ] Not Started
- **Description**: Scatter plot of lead times for completed stories
- **Details**:
  - Same layout as Cycle Time but measures creation → completion
  - Dots, trend lines, toggles, hover, filters
- **Files**: `frontend/src/components/pm/charts/LeadTimeChart.tsx`

---

### 3.7 Frontend — Epic & Iteration Detail Enhancements

#### Task 3.7.1: Add burndown and CFD to Epic Detail page
- **Status**: [ ] Not Started
- **Description**: Embed chart components in epic detail
- **Details**:
  - Add "Reports" tab/section to Epic Detail page
  - Show burndown chart and cumulative flow diagram scoped to this epic
  - Fetch data from `/api/pm/epics/{id}/burndown` and `/cumulative-flow`
- **Files**: Update `frontend/src/pages/pm/EpicDetail.tsx`

#### Task 3.7.2: Add burndown, CFD, and velocity to Iteration Detail page
- **Status**: [ ] Not Started
- **Description**: Embed chart components in iteration detail
- **Details**:
  - Add "Reports" tab/section to Iteration Detail page
  - Show burndown chart, cumulative flow diagram, and velocity context
  - Fetch data from iteration report endpoints
- **Files**: Update `frontend/src/pages/pm/IterationDetail.tsx`

---

### 3.8 Frontend — API Services & State (Phase 3 additions)

#### Task 3.8.1: Objective service
- **Files**: `frontend/src/lib/services/pmObjectiveService.ts`

#### Task 3.8.2: Report service
- **Files**: `frontend/src/lib/services/pmReportService.ts`

#### Task 3.8.3: Objective store
- **Files**: `frontend/src/stores/pmObjectiveStore.ts`

---

## Definition of Done

Phase 3 is complete when:
- [ ] Objectives page shows tactical and strategic objectives with progress
- [ ] Key results can be created, updated, and show progress
- [ ] Epics can be linked to multiple objectives
- [ ] Timeline/Roadmap shows epics as draggable bars on calendar
- [ ] Timeline supports zoom (week/month/quarter) and grouping (team/objective)
- [ ] Velocity chart shows per-iteration data with trend lines
- [ ] Burndown chart shows ideal vs actual for iterations and epics
- [ ] Cumulative Flow Diagram renders correctly for iterations and epics
- [ ] Cycle Time and Lead Time scatter plots work with filters
- [ ] Daily snapshots capture iteration/epic state for historical charts
- [ ] Reports page provides unified access to all charts

---

## Dependencies

- Phase 2 must be complete (stories, filters, etc.)
- Charting library: `recharts` (React-native, composable, already popular in React ecosystem)
- Consider `date-fns` for date calculations in timeline

---

## Risk & Mitigations

| Risk | Mitigation |
|------|-----------|
| Snapshot data gaps | Backfill snapshots from activity log if daily cron is missed |
| Timeline performance with many epics | Virtualize rows, lazy-load bars outside viewport |
| Chart rendering on large datasets | Aggregate data server-side, limit scatter plots to last 90 days by default |
| Objective progress accuracy | Clearly document how progress is computed, show "last updated" timestamp |
