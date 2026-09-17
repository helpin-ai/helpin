# PM shared dropdown migration — implementation audit

Original audit: 2026-09-09. Includes bulk edit. Source review: 2026-09-17.
This historical migration report is for contributors maintaining shared PM
selection controls. Inventory counts and browser results below describe the
original audit, not a fresh visual verification of every surface.

## Current verification boundary

The [declarative adapter](../../frontend/src/components/design-system/quiet-dropdown-select.tsx)
still renders through [QuietDropdown](../../frontend/src/components/design-system/quiet-dropdown.tsx).
Its explicit-undefined handling preserves controlled mixed-value fields. Shared
option search uses `hideWhenFits` in auto mode, while always/off modes remain
explicit. [SidebarPopoverSelect](../../frontend/src/components/pm/SidebarPopoverSelect.tsx)
and [PMFilterControls](../../frontend/src/components/pm/PMFilterControls.tsx)
use this shared core.

A fresh scan of 133 production TSX files in `components/pm` and `pages/pm` found
zero matches for the [regression test's](../../frontend/src/components/pm/__tests__/pmDropdownMigration.test.ts)
legacy import/native-select pattern. That test scans these two folders, excludes
`__tests__`, and recognizes the specified direct import syntax. It does not trace
every indirect wrapper or prove visual/keyboard parity throughout the app. The
exact site counts below remain the September 9 inventory.

[Bulk Apply](../../frontend/src/components/pm/TaskBulkActionsBar.tsx) submits
per-task updates with `Promise.allSettled`; partial success is possible. Shared
controls do not make the batch transactional. The original isolated Chromium
checks and full-suite results were not rerun during this documentation comparison.


## Implementation

`QuietDropdown` owns the option-list interaction behind `QuietFilterDropdown`. Its compound API supports domain rows and filter-category navigation through the same `QuietDropdownOptions` search/list renderer. The declarative `quiet-dropdown-select` adapter preserves existing form value callbacks, triggers and grouped JSX while rendering options through this core.

| Surface | Result |
| --- | --- |
| Task/epic/sprint detail and right rails | Shared SidebarPopoverSelect, member/multi-member, objective, label and configured-estimate adapters; repository and agent selectors inherit the same implementation. |
| Task list, task board, epic table | Page-owned Command/search/list wrappers removed. Stable IDs distinguish duplicate labels. Colored epic badges and lifecycle groups retained. Lazy cells restore focus to replacement triggers. |
| Bulk editing | All six declarative single-value fields use the shared adapter; owners and labels use shared domain adapters. Epic choices include colored badges and lifecycle groups. Explicit undefined values preserve mixed/unchanged placeholders. Cancel discards staging; Apply performs existing per-task updates. |
| Filters | PMFilterControls uses the shared compound API; TaskListView's duplicate filter UI delegates to it. Category searches reset before showing values. Owner overflow includes avatars, presence and email search. |
| Toolbars and other PM pages | Task team/grouping, sprint/backlog filters, roadmap grouping/zoom, report team/period/sprint, label/template scope and Support Search sorting use the shared core. |
| Creation | Declarative fields in GlobalCreateModals and templates migrated. CreateTaskModal's local grouped sprint picker removed. ObjectiveDetail's local sidebar picker removed; Add Epics retains asynchronous loading and linked-epic exclusion, with loading/error handling. |
| Saved views | Grouped options use the shared list, with no count threshold. Pin clicks and keyboard activation do not open the view. |
| Display settings | Board, list and generic display-property selectors use shared multiple-selection rows. Property persistence, team visibility exclusions and Show empty columns are retained. |

## Inventory and exceptions

The September 9 JSX inventory traced imports, rather than counting component names alone. It found **45 declarative Select sites**, all importing the shared adapter, **27 SidebarPopoverSelect sites**, **18 shared compound option lists**, **12 direct QuietDropdown sites**, and **11 QuietFilterDropdown sites** within PM pages/components. Domain adapter usages and inherited table/detail flows are additional consumers.

No PM production file imports the old UI Select or Command directly, and no native HTML select remains. A regression test enforces this boundary.

Remaining specialized interactions are intentional:

- Date/calendar/range pickers; color palettes; free-form numeric estimate input.
- Bulk edit's outer form popover and calendar. Its option fields are migrated.
- Comment emoji/reaction pickers and task-ID copy controls.
- Action menus for archive/delete/duplicate, copy/open links, saved-view management and coding-session actions.
- Relationship dialogs and mixed relationship action menus, including type-change commands and removal. These remain relationship-management interactions.
- Read-only compact epic metadata and My Work's inherited task-detail flow.

## Validation

Regression coverage includes duplicate-name keyboard selection, explicit None/empty values, disabled and partial options, controlled mixed-value resets, grouped declarative choices, category-search reset, label creation search, and actual bulk Cancel/Apply behavior.

Isolated Chromium checks use the real components and compiled application styles, without a frontend server. Both light and dark mode pass: search hiding for fitting lists, overflowing grouped lists in a 360×320 viewport, active/empty queries, changing option counts, always-visible search, 12.2px typography, duplicate IDs, normal focus restoration, lazy epic-cell focus restoration, saved-view pin keyboard activation, display-property toggling and the Show empty columns footer.

Original automated check results are recorded in the [migration plan](pm-shared-dropdown-migration-plan.md). Browser checks cover isolated components; authenticated page behavior is covered by the frontend regression suite and code review, not a live deployed-app walkthrough.
