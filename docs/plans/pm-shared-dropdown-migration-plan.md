# PM shared dropdown migration plan

Status: historical implementation record, including bulk edit. The September 9 validation record is preserved below; it is not a fresh test result.

Baseline: local `develop` at `9f52f1f2d`, 2026-09-09. The baseline record reported passing TypeScript checks and 2,481 frontend tests across 415 files.

This plan explains the shared PM option-picker contract and the migration completed
in the dated implementation record. Use the current source and linked audit when
adding a selector; checked boxes are historical evidence, not current page QA.

## Source review — 2026-09-18

- [QuietDropdown](../../frontend/src/components/design-system/quiet-dropdown.tsx)
  supplies shared search/list primitives with explicit `auto`, `always`, and `off`
  search modes. `QuietFilterDropdown` delegates to it, while the declarative
  `quiet-dropdown-select` adapter supports existing form-style consumers.
- [Global creation](../../frontend/src/components/pm/GlobalCreateModals.tsx)
  imports that shared Select adapter. A component named `Select` is therefore
  not evidence that a caller still uses the legacy primitive; inspect imports.
- [The migration regression test](../../frontend/src/components/pm/__tests__/pmDropdownMigration.test.ts)
  scans production TSX in `components/pm` and `pages/pm` for particular direct
  legacy imports and native `<select>` syntax. It does not prove all indirect
  wrappers, keyboard behavior, or authenticated page flows are correct.
- [Bulk actions](../../frontend/src/components/pm/TaskBulkActionsBar.tsx) retain
  staging and per-task operations using `Promise.allSettled`, including partial
  failure feedback. Shared selectors do not make Apply transactional.
- The historical test counts, Chromium checks, and production build below were
  not rerun for this documentation review. The recorded absence of authenticated
  deployed-page walkthroughs remains an explicit validation limit.

## Original implementation record

## Outcome

Move every PM option-selection dropdown onto the implementation behind the new shared dropdown. Filters, assignments, right rails, inline table/board editing, bulk editing, creation forms, saved views, and display-property selection must use the same option list, search behavior, typography, selection indicators, and keyboard handling.

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
| Bulk edit | Team, state, owners, priority, severity, epic, sprint and labels; preserve staging, mixed values, Cancel and Apply |
| Sprints | Status filter, backlog priority/state/assignee, detail team selector and task table |
| Creation and templates | Task/template, epic, sprint and objective fields; template selection; sprint cadence/day/count selectors |
| Objectives | Local metadata pickers, owner selection and Add Epics |
| Other PM pages | Roadmap filters/grouping/zoom, label/template scope, Reports team/period/sprint, PM Support Search filters/sort |
| Shared tables | Every option picker supplied through column renderers; table containers do not own field selection |

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

- [x] Extract `QuietDropdown` from `frontend/src/components/design-system/quiet-select.tsx` and export it through the design-system entry point.
- [x] Add the contract above without changing existing QuietFilterDropdown behavior.
- [x] Centralize ID-based matching, None handling, active/checked indicators and grouped rendering.
- [x] Integrate explicit search modes with `CommandInput` / `useDropdownSearch`.
- [x] Bound the list by available viewport height, including search/header/footer space.
- [x] Establish behavioral tests and an isolated browser fixture before migrating consumers.

Exit: existing filter tests pass; duplicate labels, grouped overflow and active-query visibility work through the new core.

### 2. Migrate domain adapters and right rails

- [x] Replace both branches of `SidebarPopoverSelect` with the shared core; remove `searchThreshold` and update its callers.
- [x] Migrate MemberPickerPopover/MultiMemberPickerPopover, OwnerAvatarFilterRow, ObjectivePicker, InlineEpicCell and LabelPicker.
- [x] Migrate configured EstimatePicker options; retain its free-form numeric variant.
- [x] Keep AgentPickerCard as a domain wrapper using the migrated field picker.
- [x] Remove ObjectiveDetail's local SidebarPopoverSelect and use the shared adapter.
- [x] Verify task detail/create, epic detail, sprint detail, repositories and agent selectors through those adapters.

Exit: right rails no longer choose a different implementation by count, and same-named records work with mouse and keyboard throughout the migrated adapters.

### 3. Migrate tables, board cards and bulk editing

- [x] Replace page-owned Command lists in TaskListView, TaskCard and Epics with the shared core/domain adapters.
- [x] Preserve task/epic lifecycle grouping, colored badges, avatars, custom cell triggers and lazy mounting.
- [x] Migrate TaskBulkActionsBar selectors and verify mixed/no-change values, staging, Cancel and Apply.
- [x] Verify embedded task tables in EpicDetail and SprintDetail, plus sheet openings from My Work and other tables.

Exit: the same field uses the same list behavior in a table, board card and detail rail. Inline edits do not trigger task navigation or dragging.

### 4. Migrate filters, toolbars and saved views

- [x] Make PMFilterControls delegate to QuietFilterDropdown/shared core while preserving staged filter-category navigation.
- [x] Remove duplicate filter dropdown implementations from TaskListView.
- [x] Migrate task toolbar team/grouping and retain Epics filters on the same core.
- [x] Migrate sprint status and backlog priority/state/assignee; render member avatars in assignee options.
- [x] Migrate Roadmap grouping/zoom, Reports team/period/sprint, label/template scope, and PM Support Search sort.
- [x] Remove the ViewBar `> 10` gate and migrate its grouped options and pin actions. Pinning must not open the view.

Exit: filters preserve URL/persistence semantics and clear behavior, and all enumerated selectors use the shared core even when search is hidden.

### 5. Finish creation, objectives and display settings

- [x] Migrate GlobalCreateModals' remaining Select controls, including short cadence/day/count and objective state lists.
- [x] Remove CreateTaskModal's local GroupedSidebarPopoverSelect after migrating its sprint behavior.
- [x] Migrate ObjectiveDetail Add Epics to the shared list, retaining asynchronous loading and the exclusion of linked epics.
- [x] Migrate BoardDisplayMenu, ListDisplayMenu and DisplayPropertiesPopover to shared multi-select options with their existing custom triggers.
- [x] Preserve per-team hidden properties, stored column choices, and the board's Show empty columns switch through an appropriate footer slot.
- [x] Standardize typography on genuine action/settings menus that intentionally retain their specialized primitives.

Exit: display choices use the shared option rows rather than separate chip-grid implementations; all existing settings remain available.

### 6. Remove legacy implementations and validate all surfaces

- [x] Remove dead count gates, duplicate option renderers and obsolete props/imports.
- [x] Refresh the PM dropdown inventory, tracing imports so a local component with the same name cannot be mistaken for the shared one.
- [x] Review remaining direct Select/native-select/Command/custom option-list usage in PM pages and components. Every remaining use must be a documented exception, for specialized interactions; sharing typography alone does not qualify as migrated.
- [x] Confirm non-PM users of shared components still work; migration scope does not justify breaking existing consumers elsewhere.
- [x] Run the final checks below and document results against the resulting commit.

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
- Stage bulk changes, cancel, then apply; verify exact payloads and team/state dependencies.
- Create and assign a label from a short list; search and select asynchronously loaded records.
- Change saved views and pin them; toggle display columns and Show empty columns.
- Exercise loading, save failure and disabled permission states through existing domain behavior.

Run focused behavior tests per phase, then `pnpm --dir frontend test` and the frontend TypeScript check after integration. Use isolated browser checks without starting extra frontend servers. Complete authenticated page checks where an environment with suitable data/access is available, and clearly report any remaining environment-dependent checks.

## Completion criteria

- [x] Every in-scope PM option selector delegates to the shared implementation, including short fixed lists.
- [x] No label-based entity identity or option-count search gate remains in migrated selectors.
- [x] All named pages, rails, table cells, board cards, bulk editing and creation flows are covered in the final inventory.
- [x] Avatars, badges, grouping, creation, permissions, persistence and save behavior retain parity.
- [x] Shared dropdown text is 12.2px; the search-to-options spacing and both themes are consistent.
- [x] Keyboard, viewport-fit and remote/creatable scenarios pass; bulk staging and Apply scenarios pass.
- [x] Full frontend tests and TypeScript checks pass; browser verification is documented.
- [x] Each specialized exception has a concrete reason and location.

Implementation coverage and exceptions are documented in [the completed audit](pm-shared-dropdown-migration-audit.md).

## Final validation — 2026-09-09

- Frontend suite: **2,491 tests passed across 417 files**.
- TypeScript: `tsc -b --pretty false` passed.
- ESLint: shared dropdown, Select adapter, focus helper and filter wrapper passed.
- Isolated Chromium: light/dark themes, fit/overflow, grouped/narrow viewport, active queries, duplicate IDs, focus restoration, saved-view pinning and display settings passed.
- Production build: `pnpm --dir frontend run build` passed (existing chunk-size warning only).
- No frontend servers were started. Authenticated deployed-page walkthroughs were not performed; the audit distinguishes component/browser checks from page regression coverage.
