# PM shared dropdown migration plan

Status: proposed implementation plan.

Baseline: local `develop` at `9f52f1f2d`, 2026-09-09. The adaptive-search implementation passes TypeScript checks and 2,481 frontend tests across 415 files.

## Outcome

Move every PM option-selection dropdown onto the implementation behind the new shared dropdown. Filters, assignments, right rails, inline table/board editing, creation forms, saved views, and display-property selection must use the same option list, search behavior, typography, selection indicators, and keyboard handling.

Adoption means sharing the implementation, not merely copying its classes or wrapping an existing custom list in a new component name.

The migration must fix these confirmed problems:

- Same-named epics in the right-rail picker collide because CommandItem values use labels. Two options become active together, and keyboard navigation cannot reach the second ID.
- Option-count thresholds prevent search from appearing on overflowing grouped lists. The audit reproduced 429px of content inside a 225px list with no search input.
- Teams, owners, epics and sprints use different selection controls across pages and tables.
- Some custom menus bypass the shared 12.2px dropdown typography.

## Scope

All option selectors are included, even short lists such as priority, health, grouping, zoom, period and sorting. Search stays hidden when these lists fit.

| Area | Included controls |
| --- | --- |
| Epics list | Filters, owner overflow, grouping, state/health/team/owner/objective/label table cells, display properties |
| Epic detail and sheet | Editable metadata, objectives, owner, repository and agent selection; embedded task table |
| Task detail and right rail | Team, state, owners, requester, priority, severity, type, epic, sprint, labels, estimate scale, repository and agent selection |
| Task list and board | Filters, toolbar team/grouping, inline editors, owners, epic badges, saved views and display settings |
| Sprints | Status filter, backlog priority/state/assignee, detail team selector and task table |
| Creation and templates | Task/template, epic, sprint and objective fields; template selection; sprint cadence/day/count selectors |
| Objectives | Local metadata pickers, owner selection and Add Epics |
| Other PM pages | Roadmap filters/grouping/zoom, label/template scope, Reports team/period/sprint, PM Support Search filters/sort |
| Shared tables | Every option picker supplied through column renderers; table containers do not own field selection |

Excluded from this migration:

- **Bulk edit:** keep its current controls, staging and Apply flow. Do not migrate TaskBulkActionsBar. Shared adapter APIs must remain compatible with its existing consumers.

Specialized interactions retain their purpose:

- Calendars, date ranges, color palettes and free-form numeric estimate inputs remain specialized controls.
- Archive, delete, duplicate, copy-link and lifecycle action menus retain action-menu semantics and shared menu typography.
- Relationship search dialogs remain dialogs, including task-to-epic/sprint linking. Remote search remains available even with few loaded results.
- Creatable label dropdowns migrate to the shared list while preserving the search and create action.
- Read-only metadata in the compact epic sheet does not become editable as part of this migration.
- My Work inherits changes through the task-detail flow; no new dropdown is required on the page itself.

## Shared component design

Extract the reusable selection implementation from `QuietFilterDropdown` into **`QuietDropdown`** in the design-system layer. Keep `QuietFilterDropdown` as a thin filter presentation wrapper, preserving existing consumers.

Existing domain components such as `MemberPickerPopover`, `LabelPicker`, and `SidebarPopoverSelect` become thin adapters over the same core. They may own domain data, save callbacks and trigger content, but must not own separate option-list, search, Popover or keyboard implementations.

The core must support:

| Contract | Requirement |
| --- | --- |
| Identity | Stable option IDs as values; labels and optional keywords used for matching. Identically named records remain independently selectable. |
| Selection | Single and multiple selection, checked and partially selected states, disabled options, controlled values and open state. |
| Clearing | Explicit clear/None behavior. Adapters preserve existing API representations of empty, null, undefined, mixed and unchanged values. |
| Single-value behavior | Ordinary reselection and toggle-to-clear are explicit policies. Preserve the existing board/list epic toggle-to-clear behavior. |
| Grouping | Ordered groups with stable IDs and optional headings; omit empty groups; preserve lifecycle order and the None group. |
| Rendering | Custom triggers, leading avatars/icons, badges, option content and accessible secondary actions such as pinning a saved view. |
| Search | Explicit `auto`, `always`, and `off` modes. Ordinary local selectors use `auto`; remote lookup and creation use `always`. |
| Search state | Support controlled queries without assuming they are remote. The shared core must explicitly choose visibility mode instead of relying on the current callback-presence heuristic. |
| States | Loading, empty results, errors with retry, disabled controls and optional footer/create actions. |
| Layout | Shared 12.2px option typography, current search styling and added bottom spacing, bounded width/height, scrolling constrained to the available viewport. |
| Interaction | Stable active option, Arrow/Home/End/Enter/Escape behavior, selection versus checked-state distinction, and focus restoration. |
| Performance | Lazy menu mounting in virtualized lists/boards; measurement observers only while content is mounted. |

Use the existing Command and Popover behavior primitives. Extend the current overflow measurement rather than building another search-visibility mechanism.

In `auto` mode, measure the complete list including headings and additional rows. Account for space released by hiding search, keep search visible during an active query, and remeasure after data/size changes without oscillation or focus loss. Remove option-count gates.

## Capability parity before each migration

For each adapter or page, record its current behavior and check it after conversion:

- Entry points and custom trigger content, including inline and sheet variants.
- Current value, defaults, None/clear behavior, archived/missing selected records, disabled choices and permissions.
- Group ordering, avatars, presence indicators, badges and state icons.
- Single/multiple selection, partial selection, and whether the menu closes after a choice.
- Controlled query state, remote loading, creation, empty states and retry behavior.
- Exact save payloads, optimistic updates, errors, query invalidation and rollback.
- URL parameters, persisted filters, display preferences and saved-view behavior.
- Compatibility with consumers outside the migration, including existing bulk-edit adapter usage.
- Keyboard interaction and protection against opening or dragging the containing task row/card.

No capability is scheduled for removal. Preserve the removed Epic team filter, the existing owner-first order, and the existing distinction between filter selections and assignment edits.

## Implementation sequence

### 1. Build and verify the shared foundation

- [ ] Extract `QuietDropdown` from `frontend/src/components/design-system/quiet-select.tsx` and export it through the design-system entry point.
- [ ] Add the contract above without changing existing QuietFilterDropdown behavior.
- [ ] Centralize ID-based matching, None handling, active/checked indicators and grouped rendering.
- [ ] Integrate explicit search modes with `CommandInput` / `useDropdownSearch`.
- [ ] Bound the list by available viewport height, including search/header/footer space.
- [ ] Establish behavioral tests and an isolated browser fixture before migrating consumers.

Exit: existing filter tests pass; duplicate labels, grouped overflow and active-query visibility work through the new core.

### 2. Migrate domain adapters and right rails

- [ ] Replace both branches of `SidebarPopoverSelect` with the shared core; remove `searchThreshold` and update its callers.
- [ ] Migrate MemberPickerPopover/MultiMemberPickerPopover, OwnerAvatarFilterRow, ObjectivePicker, InlineEpicCell and LabelPicker.
- [ ] Migrate configured EstimatePicker options; retain its free-form numeric variant.
- [ ] Keep AgentPickerCard as a domain wrapper using the migrated field picker.
- [ ] Remove ObjectiveDetail's local SidebarPopoverSelect and use the shared adapter.
- [ ] Verify task detail/create, epic detail, sprint detail, repositories and agent selectors through those adapters.

Exit: right rails no longer choose a different implementation by count, and same-named records work with mouse and keyboard throughout the migrated adapters.

### 3. Migrate tables and board cards

- [ ] Replace page-owned Command lists in TaskListView, TaskCard and Epics with the shared core/domain adapters.
- [ ] Preserve task/epic lifecycle grouping, colored badges, avatars, custom cell triggers and lazy mounting.
- [ ] Verify embedded task tables in EpicDetail and SprintDetail, plus sheet openings from My Work and other tables.

Exit: the same field uses the same list behavior in a table, board card and detail rail. Inline edits do not trigger task navigation or dragging.

### 4. Migrate filters, toolbars and saved views

- [ ] Make PMFilterControls delegate to QuietFilterDropdown/shared core while preserving staged filter-category navigation.
- [ ] Remove duplicate filter dropdown implementations from TaskListView.
- [ ] Migrate task toolbar team/grouping and retain Epics filters on the same core.
- [ ] Migrate sprint status and backlog priority/state/assignee; render member avatars in assignee options.
- [ ] Migrate Roadmap grouping/zoom, Reports team/period/sprint, label/template scope, and PM Support Search sort.
- [ ] Remove the ViewBar `> 10` gate and migrate its grouped options and pin actions. Pinning must not open the view.

Exit: filters preserve URL/persistence semantics and clear behavior, and all enumerated selectors use the shared core even when search is hidden.

### 5. Finish creation, objectives and display settings

- [ ] Migrate GlobalCreateModals' remaining Select controls, including short cadence/day/count and objective state lists.
- [ ] Remove CreateTaskModal's local GroupedSidebarPopoverSelect after migrating its sprint behavior.
- [ ] Migrate ObjectiveDetail Add Epics to the shared list, retaining asynchronous loading and the exclusion of linked epics.
- [ ] Migrate BoardDisplayMenu, ListDisplayMenu and DisplayPropertiesPopover to shared multi-select options with their existing custom triggers.
- [ ] Preserve per-team hidden properties, stored column choices, and the board's Show empty columns switch through an appropriate footer slot.
- [ ] Standardize typography on genuine action/settings menus that intentionally retain their specialized primitives.

Exit: display choices use the shared option rows rather than separate chip-grid implementations; all existing settings remain available.

### 6. Remove legacy implementations and validate all surfaces

- [ ] Remove dead count gates, duplicate option renderers and obsolete props/imports.
- [ ] Refresh the PM dropdown inventory, tracing imports so a local component with the same name cannot be mistaken for the shared one.
- [ ] Review remaining direct Select/native-select/Command/custom option-list usage in PM pages and components. Every remaining use must be a documented exception, including the excluded bulk editor; sharing typography alone does not qualify as migrated.
- [ ] Confirm non-PM users of shared components still work; migration scope does not justify breaking existing consumers elsewhere.
- [ ] Run the final checks below and document results against the resulting commit.

## Validation

Core regression scenarios:

1. Zero, one, few and many options; duplicate names with different IDs; disabled and missing selected records.
2. Several group headings causing overflow despite a small option count.
3. Search hidden when the complete list fits; visible when it overflows; stable at the threshold and after resize/data changes.
4. Search remains available during filtering, no results, remote loading and label creation; clearing restores the complete list.
5. Mouse and keyboard selection of duplicate names; no competing active-option identities; focus preserved when search hides and when the menu closes/reopens.
6. Single selection, toggling the current epic to None, multi-selection, partial selection and adapter compatibility for mixed/no-change values.
7. Long labels, avatars, badges, narrow viewports, constrained height and both themes.

Representative integration flows:

- Edit the same task's epic, state and owners from board, list and detail rail.
- Edit epic metadata and objectives in list/detail; open tasks from epic and sprint tables.
- Filter sprint backlog by an owner with an avatar; verify selected values and persisted filters.
- Create and assign a label from a short list; search and select asynchronously loaded records.
- Change saved views and pin them; toggle display columns and Show empty columns.
- Exercise loading, save failure and disabled permission states through existing domain behavior.

Run focused behavior tests per phase, then `pnpm --dir frontend test` and the frontend TypeScript check after integration. Use isolated browser checks without starting extra frontend servers. Complete authenticated page checks where an environment with suitable data/access is available, and clearly report any remaining environment-dependent checks.

## Completion criteria

- [ ] Every in-scope PM option selector delegates to the shared implementation, including short fixed lists.
- [ ] No label-based entity identity or option-count search gate remains in migrated selectors.
- [ ] All named pages, rails, table cells, board cards and creation flows are covered in the final inventory.
- [ ] Avatars, badges, grouping, creation, permissions, persistence and save behavior retain parity.
- [ ] Shared dropdown text is 12.2px; the search-to-options spacing and both themes are consistent.
- [ ] Keyboard, viewport-fit and remote/creatable scenarios pass; existing bulk-edit tests retain compatibility coverage.
- [ ] Full frontend tests and TypeScript checks pass; browser verification is documented.
- [ ] Each specialized exception has a concrete reason and location.

Implement in the order above with reviewable commits per phase. This document plans the migration; the application migration has not started.
