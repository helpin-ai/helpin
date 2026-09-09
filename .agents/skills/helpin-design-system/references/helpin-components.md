# Helpin component map

Use this map for product UI under `frontend/`. Inspect the live source before extending a component.

## Centralized Quiet primitives

Import from `@/components/design-system/quiet`.

| Need | Component |
| --- | --- |
| Scrollable route canvas | `QuietPageViewport` |
| Route title, optional parenthetical scope, breadcrumb/navigation, description, actions | `QuietPageHeader` |
| Sidebar-safe inset for specialized editor/workflow toolbars | `workspaceSidebarSafeInsetClassName` |
| Person/company identity | `QuietIdentityHeader` |
| Breadcrumb-led editable entity/document header | `QuietDetailHeader`, `QuietBreadcrumbs`, `QuietDetailAction` |
| Main + rail detail shell | `QuietDetailLayout`, `QuietDetailRail` |
| Primary line tabs | `Tabs`, `TabsList variant="quiet"`, `TabsTrigger` from `@/components/ui/tabs` |
| Divider-led section | `QuietSection`, `QuietSectionHeader` |
| Hairline metric overview | `QuietMetricGrid`, `QuietMetricBlock` |
| Default secondary action | `QuietTextAction` |
| Compact icon action | `QuietIconAction` |
| One dark primary action | `QuietPrimaryAction` |
| Entity title input | `QuietTitleInput` (`entity` is 26px; `header` is explicitly 20px at every breakpoint) |
| Long document title control | `QuietTitleTextarea` (`header` is autosizing 20px) |
| Search field | `QuietSearchInput` (the canonical Skill Catalog treatment) |
| Option-selection dropdown | `QuietDropdown` |
| Toolbar filter or grouping selector | `QuietFilterDropdown` |
| Relationship picker overlay/results | `QuietRelationshipDialogContent`, `QuietRelationshipResults`, `quietRelationshipResultRowClassName` |
| Underline input/control | `QuietUnderlineInput`, `quietUnderlineControlClassName` |
| Detail property | `QuietPropertyRow` |
| Stacked scan row | `QuietListRow` |
| Inline facts/provenance | `QuietMetaLine` |
| Dot plus status word | `QuietStatusText` |
| Working empty state | `QuietEmptyState` |
| Conversation authoring shell | `QuietConversationComposer`, `QuietComposerEditorSurface` |
| Conversation formatting/actions | `QuietComposerToolbar`, `QuietComposerAITools`, `QuietComposerFormatButton` |

These components own visual invariants. Pages own data, navigation, permissions, and domain actions.

Use `QuietPageHeader` for normal authenticated index and settings routes. Use `QuietDetailHeader` when a detail page needs ancestor breadcrumbs, a single editable identity/document title, metadata, status, save state, and page-specific actions. Its left side renders one compact line beneath the title with metadata first and semantic `status` last. Its right side renders actions with transient save/operation `state` directly underneath. Use `QuietDetailAction` for those actions so phones receive round icon controls and larger screens receive labels without page-owned responsive class strings; use its `iconOnly` option for controls such as the Docs details toggle that should remain icon-only at every breakpoint. The header owns layout and sidebar clearance; pages retain navigation, permissions, validation, and persistence. Keep purpose-built editor or workflow toolbars only when their controls cannot fit this composition, and apply `workspaceSidebarSafeInsetClassName` to every top/loading toolbar state instead of recreating the collapsed-sidebar spacing class.

The Task detail sheet follows the same header contract. Its optional breadcrumb chain is Tasks → Objective → Epic → Sprint; its metadata line contains task key, team, recurrence, and workflow status; save feedback stays beneath the right-side utility actions. Agent runs are entered through the Delivery tab rather than duplicated as a header action.

## Existing behavior primitives

Keep using the established components below. Prefer the listed quiet presentation rather than rebuilding behavior.

- Overlays: `Sheet`, `Dialog`, `Popover`, `DropdownMenu`, `Tooltip`, and `QuickTooltip` from `@/components/ui`.
- Form behavior: `Input` with `variant="plain"`, the shared dropdown and adapters below, `DatePicker`, `TiptapEditor` with `variant="divider"`, and react-hook-form/zod where already used. Preserve specialized Command surfaces such as the command palette.
- Identity: `UserAvatar`; provide `fallbackColorSeed` when a stable email is available.
- PM: `SidebarPopoverSelect`, `MemberPickerPopover`, `SaveIndicator`, `Attachments`, `DetailDescriptionEditorActions`, `DetailDescriptionEditButton`, `TaskUpdatesView`, `EpicUpdatesView`, and routing helpers.
- CRM: `EntitySummaryCard` with `presentation="overview"`, `EntitySignals`, `ActivityTimeline` and `EmailTimeline` borderless presentations, `CompanyDetailCollections`, `LinkedTasksPanel`, and enrichment/association components.
- Automation: retain flow composers, agent editors, run drawers, tool selectors, query hooks, permission checks, and billing/error handling. Centralize only their page chrome and recurring visual patterns.

## Dropdowns and applied filters

### Choose the shared dropdown

Use the shared option-selection implementation across list and board views, table cells, detail pages, right rails, create/edit forms, and bulk edit. Reuse an existing domain picker when it already handles the field; extend the shared implementation when a capability is missing instead of introducing a page-local Select/Command list.

| Need | Implementation |
| --- | --- |
| Data-driven single or multi-select, grouped options, custom trigger | `QuietDropdown` from `@/components/design-system/quiet` |
| Custom picker composition, headers, or footers | `QuietDropdownRoot`, `QuietDropdownTrigger`, `QuietDropdownContent`, `QuietDropdownOptions`, and item/group/empty/separator exports from `@/components/design-system/quiet-dropdown` |
| Existing declarative Select markup | `Select`, `SelectTrigger`, `SelectContent`, `SelectItem`, and related exports from `@/components/design-system/quiet-dropdown-select` |
| Compact toolbar filter or Group by control | `QuietFilterDropdown` from `@/components/design-system/quiet` |
| Domain-specific selection | Existing adapters such as `SidebarPopoverSelect`, `MemberPickerPopover`, and `InlineEpicCell` under `@/components/pm` |

- Dropdown content uses the centralized **12.2px** typography token. Keep the shared content wrapper and its `data-dropdown-content` marker; do not add page-specific font sizes or rebuild its search spacing.
- Leave `searchMode="auto"` as the default: hide search when every option fits without scrolling and show it when the list overflows. The shared implementation measures available space; do not use an item-count threshold. An active query must remain visible when its results fit.
- Dropdown search uses the shared quiet `CommandInput` presentation, including the gap before the first option. Do not insert a second `QuietSearchInput` into the menu. Use `searchMode="always"` when typing is needed independently of overflow, such as remote lookup or creating an option; use `"off"` only for an intentionally non-searchable control.
- Use stable domain IDs as option values and human-readable labels/keywords for search. Preserve avatars, epic colors and lifecycle groups, disabled/partial selections, and explicit None/Unassigned choices.
- Keep persistence and selection semantics in the caller: single versus multiple selection, clearing, permission checks, and bulk edit's staged Cancel/Apply behavior. Preserve keyboard navigation and focus return, including table cells that replace the trigger after selection.
- Keep specialized date/calendar, color, and action menus on their behavior-specific components. This dropdown standard covers option selection, not every overlay.

### Show applied list filters

Use `PMFilterBar` and `PMFilterPill` from `@/components/pm/PMFilterControls` for the established Tasks/Epics presentation. Despite the module path, these components accept generic filter definitions and values. Reuse their presentation when adding comparable filters elsewhere; retain any existing query-builder operators and domain semantics.

- Render applied filters below the toolbar as **field → is → editable value → remove**, followed by **Clear all**. The compact pill is an explicit functional exception to the rule against decorative chips.
- Feed `definitions`, `values`, and `visibleKeys` from the page's filter state; route `onToggle`, `onRemove`, and `onClearAll` back to that same state so dropdowns, owner avatars, pills, results, and persisted views stay synchronized. Do not keep a second selection state for the pills.
- Keep Owner first when present, and preserve member avatars in owner option lists. Reuse `OwnerAvatarFilterRow` for the existing toolbar avatar selector.
- For a toolbar with direct category selectors, show pills only for categories with selected values. For Tasks-style add-then-choose filtering, preserve the visible empty pill while its value is being chosen. `PMFilterTrigger` provides that category-adding flow.
- Removing one pill clears only that field. Clear all clears the page's filters according to its established reset behavior; avoid a duplicate toolbar clear action while the applied-filter row is visible. Search text and standalone toggles may retain their own controls, as on Epics.
- Grouping is a view setting, not an applied filter. Use `QuietFilterDropdown label="Group by" showLabel="inline"` so the visible `Group by:` prefix and selected value sit together inside the trigger on Tasks and Epics.

Reference compositions: `frontend/src/pages/pm/EpicFilterBar.tsx`, `frontend/src/components/pm/TaskFilters.tsx`, and `frontend/src/components/pm/TaskListGroupingDropdown.tsx`.

## Canonical reference surfaces

- Task sheet: `TaskDetailPanel`, opened through `GlobalTaskPanel`.
- Epic detail: `EpicDetailPage`.
- CRM identity/detail: `ContactDetailPage` and `CompanyDetailPage`.
- Automation routes: Flows, Activity, Agents, Trigger Catalog, Skill Catalog, and Tool Catalog.

Reference surfaces demonstrate composition and domain behavior; the Quiet primitives and design reference are authoritative when an older local class or visual treatment conflicts.

## Component decisions

- Do not use `Card` to structure a new page section. Use `QuietSection` or a hairline list.
- Use `QuietMetricGrid` and `QuietMetricBlock` when a small set of earned operational numbers needs the shared Automation Activity hairline treatment; do not recreate separate metric cards.
- Do not use `Badge` for ordinary status, counts, lifecycle, filters, or metadata. Use `QuietStatusText` or inline text. Editable applied filters use the shared `PMFilterPill` exception above, not a custom Badge.
- Do not use default `Button` colors or boxed `Input`/`Tabs` styling for Quiet page chrome. Use the Quiet wrappers.
- Search is the exception to underline-only form controls: use `QuietSearchInput` throughout product surfaces. It centralizes the Skill Catalog's compact bordered treatment and leading icon. Do not recreate it or use `QuietUnderlineInput` for search; set only `containerClassName` when page layout requires a narrower width and use its `trailing` slot for clear/close actions. Keep behavior-owned `CommandInput`, editor search/replace, and content-preview controls on their specialized primitives.
- Relationship pickers use the centralized responsive dialog (576px at `sm`, 672px at `lg`, and 768px at `xl`) and contained results primitives. Result rows must shrink with `min-w-0`, hide horizontal overflow, and truncate their primary/secondary text; do not let unbounded names determine the overlay width.
- `QuietPrimaryAction` must preserve the centralized Helpin `Button size="sm"` geometry and curvature; only its color and hierarchy differ.
- Do not duplicate section-heading, tab, row, action, property-row, or empty-state class strings in a page.
- Domain components may retain a compact chip only when the shape itself is established user data or a removal affordance, such as an editable label/token collection.
- Task and epic comments and CRM email bodies/replies use the centralized conversation-composer primitives. The primitives mirror the Support `ReplyComposer` reference and own the Support-derived shell and blue emphasized border, complete rich-text action order, AI rewrite menu, keyboard hint, and send-action geometry. Keep the heavily used Support implementation behavior-stable; mirror intentional visual-contract changes between it and the primitives rather than moving Support-specific drafts, presence, shortcuts, recipient rules, or uploads into the design system. Domain components own editor extensions and content, identity/mode headers, mention panels, permissions, billing errors, and submission. Do not migrate Docs comment composers implicitly.
