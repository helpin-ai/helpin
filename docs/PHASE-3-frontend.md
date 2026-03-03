# Phase 3 Frontend: Planning & Reporting UI

**Timeline**: After Phase 3 Backend is complete
**Goal**: Objectives/OKR pages, Timeline/Roadmap view, all reporting charts.
**Status**: Not Started
**Depends on**: Phase 2 Frontend complete, Phase 3 Backend complete

---

## Overview

This phase builds the strategic planning interfaces (Objectives with Key Results) and the full reporting suite (velocity, burndown, cumulative flow, cycle time, lead time). It also adds the Timeline/Roadmap view for visual epic planning.

---

## Task Breakdown

### F3.1 API Services & Types

#### Task F3.1.1: Update PM types for Phase 3 entities
- **Status**: [ ] Not Started
- **Description**: Add TypeScript types for objectives, key results, and report data
- **Details**:
  - Interfaces: `Objective`, `ObjectiveWithDetails`, `KeyResult`, `EpicObjective`
  - Request types: `CreateObjectiveRequest`, `UpdateObjectiveRequest`, `CreateKeyResultRequest`, `UpdateKeyResultRequest`
  - Enums: `ObjectiveType` (tactical/strategic), `KeyResultType` (boolean/percent/numeric)
  - Report types: `VelocityDataPoint`, `BurndownDataPoint`, `CumulativeFlowDataPoint`, `CycleTimeDataPoint`, `LeadTimeDataPoint`, `VelocityAverages`
  - Chart config types: `ReportFilters`, `ChartScope` (iteration/epic/objective)
- **Files**: Update `frontend/src/lib/pmTypes.ts`

#### Task F3.1.2: Objective API service
- **Status**: [ ] Not Started
- **Description**: API client for objective and key result endpoints
- **Methods**:
  - `listObjectives(filters?)` → `GET /api/pm/objectives`
  - `getObjective(id)` → `GET /api/pm/objectives/{id}`
  - `createObjective(data)` → `POST /api/pm/objectives`
  - `updateObjective(id, data)` → `PUT /api/pm/objectives/{id}`
  - `deleteObjective(id)` → `DELETE /api/pm/objectives/{id}`
  - `createKeyResult(objectiveId, data)` → `POST /api/pm/objectives/{id}/key-results`
  - `updateKeyResult(objectiveId, krId, data)` → `PUT /api/pm/objectives/{id}/key-results/{krId}`
  - `deleteKeyResult(objectiveId, krId)` → `DELETE /api/pm/objectives/{id}/key-results/{krId}`
  - `linkEpic(objectiveId, epicId)` → `POST /api/pm/objectives/{id}/epics`
  - `unlinkEpic(objectiveId, epicId)` → `DELETE /api/pm/objectives/{id}/epics/{epicId}`
- **Files**: `frontend/src/lib/services/pmObjectiveService.ts`

#### Task F3.1.3: Report API service
- **Status**: [ ] Not Started
- **Description**: API client for report data endpoints
- **Methods**:
  - `getVelocityData(filters)` → `GET /api/pm/reports/velocity`
  - `getBurndownData(scope, id, unit?)` → `GET /api/pm/reports/burndown`
  - `getCumulativeFlowData(scope, id)` → `GET /api/pm/reports/cumulative-flow`
  - `getCycleTimeData(filters)` → `GET /api/pm/reports/cycle-time`
  - `getLeadTimeData(filters)` → `GET /api/pm/reports/lead-time`
  - `getEpicBurndown(epicId)` → `GET /api/pm/epics/{id}/burndown`
  - `getEpicCumulativeFlow(epicId)` → `GET /api/pm/epics/{id}/cumulative-flow`
  - `getIterationBurndown(iterationId)` → `GET /api/pm/iterations/{id}/burndown`
  - `getIterationCumulativeFlow(iterationId)` → `GET /api/pm/iterations/{id}/cumulative-flow`
- **Files**: `frontend/src/lib/services/pmReportService.ts`

---

### F3.2 State Management

#### Task F3.2.1: Objective store
- **Status**: [ ] Not Started
- **Description**: Zustand store for objectives
- **State**:
  - `objectives: Objective[]`
  - `currentObjective: ObjectiveWithDetails | null`
  - `isLoading: boolean`
- **Actions**:
  - `fetchObjectives(filters?)` — load objectives
  - `fetchObjective(id)` — load single with details
  - `setCurrentObjective(obj)` — set active
- **Files**: `frontend/src/stores/pmObjectiveStore.ts`

---

### F3.3 Routing (Phase 3 additions)

#### Task F3.3.1: Add Phase 3 routes
- **Status**: [ ] Not Started
- **Description**: Add routes for objectives, roadmap, and reports
- **Routes**:
  ```
  pm/
  ├── objectives/
  │   ├── index.tsx       → Objectives list page
  │   └── $objectiveId.tsx → Objective detail page
  ├── roadmap.tsx          → Timeline/Roadmap page
  └── reports.tsx          → Reports dashboard
  ```
- **Files**:
  - `frontend/src/routes/_authenticated/w.$slug.pm/objectives/index.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/objectives/$objectiveId.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/roadmap.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/reports.tsx`

#### Task F3.3.2: Update sidebar navigation
- **Status**: [ ] Not Started
- **Description**: Add Objectives, Roadmap, and Reports links to PM sidebar
- **Details**:
  - Add after existing PM nav items:
    - **Objectives** — icon: `Target` → `/w/{slug}/pm/objectives`
    - **Roadmap** — icon: `GanttChart` or `Map` → `/w/{slug}/pm/roadmap`
    - **Reports** — icon: `BarChart3` → `/w/{slug}/pm/reports`
- **Files**: Update sidebar navigation component

---

### F3.4 Objectives Pages

#### Task F3.4.1: Objectives list page
- **Status**: [ ] Not Started
- **Description**: Page showing all objectives grouped by state
- **Details**:
  - Route: `/w/{slug}/pm/objectives`
  - **Page header**: Title "Objectives", "Create Objective" button
  - **Grouped by state**: To Do, In Progress, Done (collapsible sections)
  - **Each objective card**:
    - Name (link to detail)
    - Type badge: "Tactical" (gray) or "Strategic" (blue)
    - Owner avatar + name
    - Deadline (if set)
    - Progress bar:
      - Tactical: % of linked epic stories completed
      - Strategic: average of key result progress
    - Progress percentage text
    - Epic count: "5 epics"
    - Key result count (strategic only): "3 key results"
  - **Filters**: State, Type (tactical/strategic), Owner
  - **Empty state**: "No objectives yet. Objectives help align work with strategic goals."
- **Files**: `frontend/src/pages/pm/Objectives.tsx`

#### Task F3.4.2: Create/Edit Objective modal
- **Status**: [ ] Not Started
- **Description**: Modal for creating or editing an objective
- **Details**:
  - **Fields**:
    - Name (required)
    - Description (textarea, markdown)
    - Type: Tactical / Strategic (radio buttons)
    - State (dropdown of objective states: To Do, In Progress, Done)
    - Owner (user picker)
    - Planned Start Date (date picker)
    - Deadline (date picker)
  - **Key Results section** (shown when type = Strategic):
    - Add key result: Name + Type (boolean/percent/numeric) + Target Value
    - List added key results with remove button
  - **Link Epics section**:
    - Epic search/select to link
    - List linked epics with unlink button
- **Files**: `frontend/src/components/pm/ObjectiveModal.tsx`

#### Task F3.4.3: Objective Detail page
- **Status**: [ ] Not Started
- **Description**: Single objective view with key results and linked epics
- **Details**:
  - Route: `/w/{slug}/pm/objectives/{objectiveId}`
  - **Header**:
    - Name (editable inline)
    - Type badge, State dropdown, Owner, Dates
    - Overall progress bar with percentage
  - **Description**: Editable textarea
  - **Key Results section** (strategic only):
    - Each key result:
      - Name
      - Type indicator (boolean checkbox / percent bar / numeric value)
      - Current value / Target value (editable)
      - Progress bar
      - Edit button → inline edit
    - "Add Key Result" button
    - Delete key result (confirm)
  - **Linked Epics section**:
    - Table of epics: Name, State, Team, Story Progress, Points Progress
    - Click epic → navigate to epic detail
    - "Link Epic" button → epic search
    - "Unlink" button per epic
  - **Comments section**
  - **Breadcrumb**: Objectives > Objective Name
- **Files**: `frontend/src/pages/pm/ObjectiveDetail.tsx`

#### Task F3.4.4: Key Result progress editor
- **Status**: [ ] Not Started
- **Description**: Inline editor for updating key result values
- **Details**:
  - **Boolean**: Toggle switch (done/not done)
  - **Percent**: Slider or number input (0-100)
  - **Numeric**: Number input showing current/target (e.g., "450 / 1000")
  - Progress bar updates in real-time
  - Auto-save on change
- **Files**: `frontend/src/components/pm/KeyResultEditor.tsx`

---

### F3.5 Timeline/Roadmap View

#### Task F3.5.1: Roadmap page
- **Status**: [ ] Not Started
- **Description**: Gantt-style timeline page showing epics
- **Details**:
  - Route: `/w/{slug}/pm/roadmap`
  - **Page header**:
    - Title: "Roadmap"
    - Zoom controls: Week | Month | Quarter (toggle buttons)
    - Group by: Team | Objective (dropdown)
    - Filter by: Team, Objective, Epic State, Labels, Owner
  - **Layout**:
    - Left panel (~250px): Row labels (epic names or group names)
    - Right panel (scrollable): Calendar grid with epic bars
  - **Epic bars**: See Task F3.5.2
  - **Today line**: Red vertical line at current date
  - **Scroll**: Horizontal scroll for timeline, vertical for many epics
  - **Empty state**: "No epics with dates set. Add start and end dates to epics to see them on the roadmap."
- **Files**: `frontend/src/pages/pm/Roadmap.tsx`

#### Task F3.5.2: Timeline Bar component
- **Status**: [ ] Not Started
- **Description**: Draggable epic bar on the timeline
- **Details**:
  - **Rendering**: Positioned based on start_date and deadline relative to timeline scale
  - **Content**: Epic name (truncated to fit), mini progress bar (% done)
  - **Color**: Based on team color or epic state
  - **Health indicator**: Small dot (green/amber/red) at right end of bar
  - **Drag body**: Move entire bar (updates both start and end dates)
  - **Drag left handle**: Change start date
  - **Drag right handle**: Change end date
  - **Snap**: Snap to week boundaries when dragging
  - **Tooltip on hover**: Full name, dates, team, "X of Y stories done", health status
  - **Visual states**:
    - Unstarted: lighter/dashed
    - Started: solid with progress fill
    - Done: muted/gray
  - **Click**: Navigate to epic detail page
  - Auto-save date changes after drag ends (debounced API call)
- **Files**: `frontend/src/components/pm/TimelineBar.tsx`

#### Task F3.5.3: Timeline Grid component
- **Status**: [ ] Not Started
- **Description**: Calendar grid with date headers for the timeline
- **Details**:
  - **Date headers** (top row):
    - Week zoom: "Mar 3", "Mar 10", "Mar 17"... each column = 1 week
    - Month zoom: "Jan", "Feb", "Mar"... each column = 1 month
    - Quarter zoom: "Q1 2026", "Q2 2026"... each column = 1 quarter
  - **Month/Year labels**: Larger text above date columns
  - **Grid lines**: Vertical lines on column boundaries
  - **Row backgrounds**: Alternating light/dark for readability
  - **Weekend shading**: Subtle background on weekend columns (week zoom)
  - **Today highlight**: Column containing today has subtle highlight
  - **Scroll**: Smooth horizontal scroll with momentum
  - **Auto-scroll**: Auto-scroll to show current date on load
- **Files**: `frontend/src/components/pm/TimelineGrid.tsx`

#### Task F3.5.4: Timeline group rows
- **Status**: [ ] Not Started
- **Description**: Group header rows for team or objective grouping
- **Details**:
  - When grouped by Team: team name as group header, epics indented below
  - When grouped by Objective: objective name as group header
  - Collapse/expand groups
  - Ungrouped epics in "No Team" / "No Objective" group at bottom
- **Files**: `frontend/src/components/pm/TimelineGroupRow.tsx`

---

### F3.6 Reports Dashboard

#### Task F3.6.1: Reports page shell
- **Status**: [ ] Not Started
- **Description**: Reports dashboard with tabs and global filters
- **Details**:
  - Route: `/w/{slug}/pm/reports`
  - **Page header**: Title "Reports"
  - **Tab navigation**: Velocity | Burndown | Cumulative Flow | Cycle Time | Lead Time
  - **Global filter bar** (applies across all tabs):
    - Team selector
    - Date range picker (from/to)
    - Additional per-tab filters (below)
  - Each tab renders its chart component full-width
  - Responsive: charts resize with container
- **Files**: `frontend/src/pages/pm/Reports.tsx`

#### Task F3.6.2: Velocity Chart component
- **Status**: [ ] Not Started
- **Description**: Stacked bar chart showing completed work per iteration
- **Details**:
  - **Library**: Recharts (`BarChart`, `Bar`, `Line`, `XAxis`, `YAxis`, `Tooltip`, `Legend`)
  - **Bars** (stacked):
    - Feature: green
    - Bug: red
    - Chore: blue/gray
  - **Trend lines**:
    - Dotted gray: overall average velocity
    - Solid green: trailing 4-iteration average
  - **Toggle**: "Story Count" / "Story Points" (button group above chart)
  - **X-axis**: Iteration names
  - **Y-axis**: Count or Points
  - **Tooltip**: Iteration name, date range, feature/bug/chore breakdown, total
  - **Legend**: Feature, Bug, Chore, Average, Trailing Avg
  - **Responsive**: Full width, ~400px height
  - **Empty state**: "No completed iterations yet"
- **Files**: `frontend/src/components/pm/charts/VelocityChart.tsx`

#### Task F3.6.3: Burndown Chart component
- **Status**: [ ] Not Started
- **Description**: Line chart showing remaining work over time
- **Details**:
  - **Scope selector**: Dropdown to select Iteration or Epic
  - **Entity selector**: Dropdown to pick specific iteration/epic
  - **Lines**:
    - Ideal (dashed gray): straight line from total work → 0 over date range
    - Actual (solid blue): real remaining work from snapshot data
  - **Toggle**: "Story Count" / "Story Points"
  - **X-axis**: Dates
  - **Y-axis**: Remaining work
  - **Tooltip**: Date, ideal remaining, actual remaining
  - **Shading**: Area under actual line (light blue)
  - **Also used in**: Epic Detail page, Iteration Detail page (embedded version)
  - **Props**: `scope: 'iteration' | 'epic'`, `entityId: string`, `embedded?: boolean`
- **Files**: `frontend/src/components/pm/charts/BurndownChart.tsx`

#### Task F3.6.4: Cumulative Flow Diagram component
- **Status**: [ ] Not Started
- **Description**: Stacked area chart of story distribution over time
- **Details**:
  - **Scope selector**: Iteration or Epic
  - **Stacked areas**: One per workflow state, colored to match state colors
    - Bottom → Top: Done, Started states, Unstarted states, Backlog
  - **X-axis**: Dates
  - **Y-axis**: Number of stories
  - **Tooltip**: Date, count per state
  - **Also used in**: Epic Detail, Iteration Detail (embedded)
  - **Reading guide**: Widening bands = bottleneck. Constant band width = healthy flow.
- **Files**: `frontend/src/components/pm/charts/CumulativeFlowDiagram.tsx`

#### Task F3.6.5: Cycle Time Chart component
- **Status**: [ ] Not Started
- **Description**: Scatter plot of cycle times for completed stories
- **Details**:
  - **Dots**: Each completed story (color by type)
    - Feature: green dot
    - Bug: red dot
    - Chore: blue dot
  - **Trend lines**:
    - Dotted: overall average cycle time
    - Solid: 7-day trailing average
  - **X-axis**: Completion date
  - **Y-axis**: Cycle time (hours or days)
  - **Scale toggle**: Linear / Logarithmic
  - **Filter**: Story type checkboxes (Feature/Bug/Chore)
  - **Tooltip on dot**: Story title, Display ID, cycle time, start date, completion date
  - **Click dot**: Navigate to story
- **Files**: `frontend/src/components/pm/charts/CycleTimeChart.tsx`

#### Task F3.6.6: Lead Time Chart component
- **Status**: [ ] Not Started
- **Description**: Scatter plot of lead times for completed stories
- **Details**:
  - Same layout and features as Cycle Time chart
  - Measures creation → completion instead of start → completion
  - All the same controls: scale toggle, type filter, tooltips
  - Separate data fetch from lead time endpoint
- **Files**: `frontend/src/components/pm/charts/LeadTimeChart.tsx`

---

### F3.7 Epic & Iteration Detail Enhancements

#### Task F3.7.1: Reports tab in Epic Detail page
- **Status**: [ ] Not Started
- **Description**: Add reports/charts to epic detail
- **Details**:
  - Add "Reports" tab alongside "Stories" and "Comments" in Epic Detail
  - Content:
    - Burndown Chart (embedded, scoped to this epic)
    - Cumulative Flow Diagram (embedded, scoped to this epic)
  - Charts fetch data from `/api/pm/epics/{id}/burndown` and `/cumulative-flow`
  - Show "Not enough data" if no snapshots yet
- **Files**: Update `frontend/src/pages/pm/EpicDetail.tsx`

#### Task F3.7.2: Reports tab in Iteration Detail page
- **Status**: [ ] Not Started
- **Description**: Add reports/charts to iteration detail
- **Details**:
  - Add "Reports" tab in Iteration Detail
  - Content:
    - Burndown Chart (embedded, scoped to this iteration)
    - Cumulative Flow Diagram (embedded)
  - Charts fetch from iteration report endpoints
- **Files**: Update `frontend/src/pages/pm/IterationDetail.tsx`

---

### F3.8 Install Dependencies

#### Task F3.8.1: Install charting library
- **Status**: [ ] Not Started
- **Packages**:
  - `recharts` — React charting library
  - Verify `date-fns` is installed for date calculations in timeline

---

## Definition of Done

Phase 3 Frontend is complete when:
- [ ] Objectives list page shows tactical and strategic objectives grouped by state
- [ ] Objective detail page shows key results with editable progress
- [ ] Key results update progress (boolean toggle, percent slider, numeric input)
- [ ] Epics can be linked/unlinked to objectives
- [ ] Roadmap/Timeline renders epics as bars on a calendar grid
- [ ] Timeline supports zoom (week/month/quarter) and grouping (team/objective)
- [ ] Epic bars are draggable to change dates
- [ ] Today line and date headers render correctly
- [ ] Velocity chart shows stacked bars with trend lines
- [ ] Burndown chart shows ideal vs actual for iterations and epics
- [ ] Cumulative Flow Diagram renders stacked areas
- [ ] Cycle Time and Lead Time scatter plots work with filters and tooltips
- [ ] Charts are embedded in Epic and Iteration detail pages
- [ ] All chart components handle empty/loading states
- [ ] No console errors or TypeScript warnings
