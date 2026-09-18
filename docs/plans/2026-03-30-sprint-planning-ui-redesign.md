# Sprint planning UI redesign plan

> Historical redesign plan, source-compared on 2026-09-17. This page records
> early sprint-planning UI ideas for contributors. The current interface has
> evolved; unchecked tasks below are not a current implementation checklist.

## Current implementation

The [empty state](../../frontend/src/components/pm/sprints/SprintPlanningEmptyState.tsx)
has the proposed heading, explanation, and three feature cards, but contains no
Create Sprint button or animated ring. Page-level controls belong to
[Sprints](../../frontend/src/pages/pm/Sprints.tsx), rather than this component.

The [workspace](../../frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx)
flattens upcoming, active, and completed buckets into horizontally scrolling
columns. It does not render the proposed per-bucket section headers and empty
boxes. Drag callbacks assign tasks to a sprint or the backlog, with the page
coordinating optimistic state; source inspection alone does not prove complete
interaction parity or successful browser drag tests.

[Sprint columns](../../frontend/src/components/pm/sprints/SprintPlanningColumn.tsx)
use virtualized task loading rather than a fixed story preview plus “more stories”
link. [Progress](../../frontend/src/components/pm/sprints/sprintProgress.ts) uses
historical closeout totals when available and live stats otherwise. The old
`SprintPlanningStoryCard.tsx` filename is now
[SprintPlanningTaskCard.tsx](../../frontend/src/components/pm/sprints/SprintPlanningTaskCard.tsx):
it uses a two-line title, task key/state, priority icon, estimate, and owner avatar,
not the exact one-line/dot arrangement below.

The [backlog panel](../../frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx)
has collapsible filters and a create-task action. On smaller screens it opens as
a fixed overlay; it is not simply hidden on mobile. The
[filter component](../../frontend/src/components/pm/sprints/SprintPlanningFilters.tsx)
now owns search and sprint-status filtering through shared controls, not the page
heading, team selector, and create button described in Task 6.

No browser, responsive-layout, or runtime test run was performed for this review.
The original visual targets and verification checklist follow as historical context.

## Original redesign record

> **For agentic workers:** Use superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the sprint planning page to feel polished and cohesive, matching the quality of the rest of the app. Fix the bare/wireframe feel without breaking existing functionality (drag-and-drop, optimistic updates, filters).

**Architecture:** Incremental improvements to existing components — no structural rewrites. Each task can be verified independently.

**Tech Stack:** React, Tailwind CSS, shadcn/ui components (Card, Badge, Button, Select, Progress), lucide-react icons.

---

## Principles

- Don't break drag-and-drop, optimistic updates, or permission logic
- Use existing shadcn/ui components and Tailwind patterns from the app
- Each task is independently verifiable
- Preserve all existing props and callbacks

---

### Task 1: Restore and improve the empty state

The original empty state (before our sprint null-safety fixes) had a nice centered layout with icon, title, description, CTA, and feature cards. Restore and refine it.

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningEmptyState.tsx`

- [ ] **Step 1: Update the empty state**

Restore the clean design:
- Centered icon (animated ring like the screenshot — use a `Timer` or `IterationCw` icon inside a rounded primary/10 circle)
- Bold heading: "Create your first sprint"
- Description: "Sprints are time-boxed cycles that help your team plan, focus, and deliver work in a predictable rhythm."
- Primary CTA: "+ Create Sprint" button
- Three feature cards below (3-column grid):
  1. Calendar icon + "Set a cadence" + "Define start and end dates for focused work cycles"
  2. BarChart icon + "Track progress" + "Monitor story and point completion in real time"
  3. CheckCircle icon + "Ship consistently" + "Build momentum with regular delivery milestones"

- [ ] **Step 2: Verify empty state renders**

Visit sprints page with no sprints for a team. Confirm the empty state looks clean.

- [ ] **Step 3: Commit**

---

### Task 2: Improve section headers in workspace view

Replace plain uppercase text headers with styled section headers that match the app's existing patterns.

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`

- [ ] **Step 1: Update bucket section headers**

For each bucket (active, upcoming, completed):
- Use slightly larger text (`text-sm font-semibold` instead of plain text)
- Add a subtle left border accent (`border-l-2 border-primary/30 pl-2`)
- Description as `text-xs text-muted-foreground`
- Show sprint count badge next to the label

- [ ] **Step 2: Improve per-bucket empty states**

Instead of plain text in a bare box:
- Dashed border container with icon + text centered
- Different messaging per bucket:
  - Active: "No active sprints. Create one or wait for an upcoming sprint to start."
  - Upcoming: "No upcoming sprints planned."
  - Completed: "No completed sprints yet."

- [ ] **Step 3: Verify and commit**

---

### Task 3: Enrich sprint column cards

Make sprint cards more informative and visually substantial.

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningColumn.tsx`

- [ ] **Step 1: Improve card header**

- Sprint name: `text-sm font-semibold` (clickable, underline on hover)
- Date range below name: `text-xs text-muted-foreground` with Calendar icon
- Status badge: use existing `Badge` component with color variants (active=emerald, upcoming=blue, completed=muted)
- Progress section: show `X of Y stories done` text next to the progress bar

- [ ] **Step 2: Improve card content area**

- Story preview cards: ensure they show priority dot, title (1-line truncate), assignee avatar
- If more stories than preview limit: show "+N more stories" link at bottom
- Empty sprint state: icon + "Pull work from the backlog" text + dashed border

- [ ] **Step 3: Improve card footer**

- "+ Create story" button with Plus icon, matching app's existing button styles
- Show total estimate points if available

- [ ] **Step 4: Verify drag-and-drop still works**

Test dragging stories between sprints and to backlog. Confirm visual feedback.

- [ ] **Step 5: Commit**

---

### Task 4: Polish story cards

Make individual story cards within sprints and backlog more information-dense.

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningStoryCard.tsx`

- [ ] **Step 1: Update story card layout**

- Left: Priority dot (colored circle, not badge) + Story title (1-line truncate)
- Right: Assignee avatar (or gray circle if unassigned)
- Bottom row: Story display ID (`#123`) in muted text + state type label + estimate badge
- Consistent height for visual alignment in columns

- [ ] **Step 2: Polish hover and drag states**

- Hover: subtle `bg-muted/40` background
- Dragging: `shadow-lg` + `opacity-70` + `rotate-1` for visual lift
- Cursor: `cursor-grab` default, `cursor-grabbing` when dragging

- [ ] **Step 3: Verify compact mode for backlog**

Ensure `compact={true}` still renders a tighter version with less padding.

- [ ] **Step 4: Commit**

---

### Task 5: Improve backlog panel

Make the backlog panel feel more integrated and less like an afterthought.

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx`

- [ ] **Step 1: Update panel header and filters**

- "Backlog" heading with count badge (matching other count badges in the app)
- Filters: use compact inline pills or small Select components, not full-width selects
- Add a "Filters" toggle to collapse/expand filter row (save vertical space)

- [ ] **Step 2: Improve empty backlog state**

- Centered icon + "No backlog stories match the current filters" text
- If no stories at all: "All stories are assigned to sprints" positive message

- [ ] **Step 3: Improve create story button**

- Sticky at bottom of panel
- Match style with sprint column's create button

- [ ] **Step 4: Commit**

---

### Task 6: Polish the top filters bar

Make the filters bar cleaner and more compact.

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningFilters.tsx`

- [ ] **Step 1: Simplify header**

- Title: "Sprint Planning" (text-xl font-semibold, matching other page headers)
- Description: "Plan active and upcoming sprints, then pull work in from the backlog." (text-sm text-muted-foreground)

- [ ] **Step 2: Clean up controls**

- Team selector: compact Select component
- Toggle buttons: use outline variant buttons with icons, consistent sizing
- Create Sprint: primary button with Plus icon
- All controls in a single row, right-aligned

- [ ] **Step 3: Commit**

---

### Task 7: Final verification

- [ ] **Step 1: Test full flow**
  - Empty state (no sprints) renders correctly
  - Creating a sprint works
  - Sprint columns show with stories
  - Drag-and-drop between sprints works
  - Drag to backlog works
  - Filters work (team, completed toggle, backlog toggle)
  - Backlog panel renders and filters properly
  - Create story from sprint works
  - Create story from backlog works

- [ ] **Step 2: Test responsive behavior**
  - Backlog hidden on mobile
  - Horizontal scroll on sprint columns

- [ ] **Step 3: Commit all remaining changes**
