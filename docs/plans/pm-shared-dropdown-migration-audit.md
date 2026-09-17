# PM shared dropdown migration — implementation audit

Date: 2026-09-09. Includes bulk edit.

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

The refreshed JSX inventory traces imports, rather than counting component names alone. It found **45 declarative Select sites**, all importing the shared adapter, **27 SidebarPopoverSelect sites**, **18 shared compound option lists**, **12 direct QuietDropdown sites**, and **11 QuietFilterDropdown sites** within PM pages/components. Domain adapter usages and inherited table/detail flows are additional consumers.

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

Final automated check results are recorded in the migration plan. Browser checks cover isolated components; authenticated page behavior is covered by the frontend regression suite and code review, not a live deployed-app walkthrough.
