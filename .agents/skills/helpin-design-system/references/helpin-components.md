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

Use `QuietPageHeader` for normal authenticated index and settings routes. Use `QuietDetailHeader` when a detail page needs ancestor breadcrumbs, a single editable identity/document title, metadata, status, save state, and page-specific actions. Its left side renders one compact line beneath the title with metadata first and semantic `status` last. Its right side renders actions with transient save/operation `state` directly underneath. At desktop widths, that action column spans the breadcrumb and content rows so taller controls do not inflate the status row; mobile retains the compact inline arrangement. Use `QuietDetailAction` for those actions so phones receive round icon controls and larger screens receive labels without page-owned responsive class strings; use its `iconOnly` option for controls such as the Docs details toggle that should remain icon-only at every breakpoint. The header owns layout and sidebar clearance; pages retain navigation, permissions, validation, and persistence. Keep purpose-built editor or workflow toolbars only when their controls cannot fit this composition, and apply `workspaceSidebarSafeInsetClassName` to every top/loading toolbar state instead of recreating the collapsed-sidebar spacing class.

Docs documents are an exception to header-owned titles: both internal and external-space documents show their editable name in `DocsEditor`, above the content. Keep the shared header's breadcrumbs, metadata, status, save indicator, and actions, passing no document title. Both use the established pinned desktop header and responsive editor scrolling. External-space document canvases use `contentWidth="external"` (4xl); internal documents retain `contentWidth="standard"` (7xl). This width rule does not apply to space or collection pages.

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
- Epic option menus use `EPIC_PICKER_WIDTH` (384px), with the shared viewport cap on narrow screens. Truncated option text gets its full label on hover through `QuietDropdownItem`; do not add unconditional titles or page-local tooltip logic.
- Leave `searchMode="auto"` as the default: hide search when every option fits without scrolling and show it when the list overflows. The shared implementation measures available space; do not use an item-count threshold. An active query must remain visible when its results fit.
- Dropdown search uses the shared quiet `CommandInput` presentation, including the gap before the first option. Do not insert a second `QuietSearchInput` into the menu. Use `searchMode="always"` when typing is needed independently of overflow, such as remote lookup or creating an option; use `"off"` only for an intentionally non-searchable control.
- Use stable domain IDs as option values and human-readable labels/keywords for search. Preserve avatars, epic colors and lifecycle groups, disabled/partial selections, and explicit None/Unassigned choices. Match the reference filter category icons and semantic option icons/colors in both toolbar menus and applied-filter editors; carry `icon`/`leading` and `labelClassName` through adapters into `PMFilterControls`.
- Keep persistence and selection semantics in the caller: single versus multiple selection, clearing, permission checks, and bulk edit's staged Cancel/Apply behavior. Preserve keyboard navigation and focus return, including table cells that replace the trigger after selection.
- Keep specialized date/calendar, color, and action menus on their behavior-specific components. This dropdown standard covers option selection, not every overlay.

### Show applied list filters

For collection pages such as Tasks, Epics, and Objectives, use a full-height flex shell with a non-scrolling `QuietPageHeader variant="shell"` and its divider. Place the full-width search/filter toolbar and applied-filter row below it, outside the content scroller. Only the table, board, or cards scroll. Do not wrap the header and toolbar in `QuietPageViewport` or a centered content-width container.

Use `PMFilterBar` and `PMFilterPill` from `@/components/pm/PMFilterControls` for the established Tasks/Epics presentation. Despite the module path, these components accept generic filter definitions and values. Reuse their presentation when adding comparable filters elsewhere; retain any existing query-builder operators and domain semantics.

- Render applied filters below the toolbar as **field → is → editable value → remove**, followed by **Clear all**. The compact pill is an explicit functional exception to the rule against decorative chips.
- Feed `definitions`, `values`, and `visibleKeys` from the page's filter state; route `onToggle`, `onRemove`, and `onClearAll` back to that same state so dropdowns, owner avatars, pills, results, and persisted views stay synchronized. Do not keep a second selection state for the pills.
- Keep Owner first when present, and preserve member avatars in owner option lists. Reuse `OwnerAvatarFilterRow` for the existing toolbar avatar selector.
- For a toolbar with direct category selectors, show pills only for categories with selected values. For Tasks-style add-then-choose filtering, preserve the visible empty pill while its value is being chosen. `PMFilterTrigger` provides that category-adding flow.
- Removing one pill clears only that field. Clear all clears the page's filters according to its established reset behavior; avoid a duplicate toolbar clear action while the applied-filter row is visible. Search text and standalone toggles may retain their own controls, as on Epics.
- Grouping is a view setting, not an applied filter. Use `QuietFilterDropdown label="Group by" showLabel="inline"` so the visible `Group by:` prefix and selected value sit together inside the trigger on Tasks and Epics.

Reference compositions: `frontend/src/pages/pm/EpicFilterBar.tsx`, `frontend/src/components/pm/TaskFilters.tsx`, and `frontend/src/components/pm/TaskListGroupingDropdown.tsx`.

## PM detail composition

Use `frontend/src/pages/pm/EpicDetail.tsx` as the visual reference for PM detail headers, property rails, and description editing. Preserve each entity's navigation contract: Objectives retains its card index and full detail page; adopting this composition does not turn a page into a sheet.

- **Back navigation:** pass both `onBack` and an entity-specific `backLabel` to `QuietBreadcrumbs`, alongside its breadcrumb items. A clickable “Objectives” or “Epics” label does not replace the visible back arrow. Keep it in loading and error headers too.
- **Title:** use `QuietTitleInput presentation="header"` inside a `flex min-w-0 items-center` wrapper in the header's `title` slot. Match Epic's `max-w-[42rem] border-b-transparent hover:border-quiet-field focus-visible:border-quiet-text-primary`. The wrapper matters: `QuietDetailHeader` gives direct children `max-w-full`, which can override a direct input's width cap. The underline is invisible at rest and bounded to the title control on hover/focus; it must not extend across the header. Match the header and body insets (`lg:px-10` on the reference page).
- **Header actions:** keep `SaveIndicator presentation="quiet"` below the actions and use `FollowButton presentation="detail-header"`. Reuse the existing responsive detail actions for other controls.
- **PM properties:** use `DetailMetadataRow` from `@/components/pm/DetailMetadataRow`, shared by Epic and Objective detail. Its parent uses `grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5`. The component owns 12px muted labels, matching icons, optional label tooltips, and value typography. Do not substitute the larger generic `QuietPropertyRow` when matching this PM rail. Preserve the reference rail's padding and section dividers.
- **Colored options:** use `SidebarPopoverSelect` and pass each option's semantic color through `className`, as well as coloring the selected trigger. A colored trigger with uncolored menu choices is incomplete. Objective health uses the same green/yellow/red treatment as Epic health, while retaining Objective's supported values; do not add unsupported states such as “No health” merely to copy the menu.
- **Description display/editing:** use `RichTextMentionContent variant="pm"`, `DetailDescriptionEditButton` inside `group/desc`, and `TiptapEditor variant="divider" contentVariant="pm"` with `min-h-[320px] [&_.tiptap]:min-h-[250px] [&_.tiptap]:p-0`. Wrap the editor in `group/description-editor` and use `DetailDescriptionEditorActions` for Cancel/Done. Do not recreate an inline text edit link, a separate Done button, or local editor chrome.
- **Description behavior:** retain permissions, mentions, uploads, and attachment handling. Cancel restores the description captured at the start of editing, even if autosave has already run. Do not delete attachments needed by that snapshot before the edit is accepted. Failed saves retain the draft and allow retry.
- **Epic markers:** when a compact row shows an epic’s color beside its name, use `EpicColorSwatch`, as in the linked-epic rows on Objective cards and detail pages, including the link-epic picker. Do not substitute a colored outline icon. Epic selection options use a color square followed by a plain, truncating name across task list, board, detail, bulk edit, and filters. Keep existing `EpicBadge` presentations for selected values outside the option list.
- **Progress:** preserve existing bars, labels, numbers, and calculations when replacing cards or summaries with shared metrics. On Objective detail, show epic completion and elapsed time toward the target date as bars; keep the target-date bar conditional on a date being set. A percentage or date alone does not replace these indicators.
- **Responsive layout:** desktop main content and the rail may scroll separately. On narrow screens, use one vertically scrollable column whose sections retain their content height. Do not let constrained grid rows overlap the rail with the main content. Verify by scrolling to the final property, not only by checking the initial viewport.

## Settings section composition

Use `frontend/src/pages/NotificationSettings.tsx` as the reference for a settings page with collapsible groups and `frontend/src/components/settings/ConversationRoutingTab.tsx` for persistent section containers.

- Keep `QuietPageHeader` for the route header. Group related controls into full-width, vertically stacked sections with subtle borders, rounded corners, clear headings, and consistent padding. Avoid nested cards for individual rows.
- Reuse `SettingsSection` from `@/components/settings/SettingsSection` for collapsible groups. It owns the container, header, chevron, focus treatment, and body divider; content remains mounted when collapsed. For always-visible groups, use the existing settings `Card`/`CardContent` composition rather than a disclosure that cannot collapse. These are explicit exceptions to the generic no-card rule.
- Keep section titles readable with `text-sm font-semibold text-quiet-text-primary`. Use secondary text for useful supporting scope, never placeholder-level contrast for navigation or section headings. Preserve this hierarchy in dark mode.
- On dense settings pages, open the general or most frequently needed group by default and collapse optional module/advanced groups. Short forms may stay open. Keep group titles visible, and reveal a section containing a validation error or a search/deep-link target so the relevant control can be reached.
- Match the settings sidebar's grouping and order where the same categories appear on settings home or a settings page. Preserve personal/workspace/organization scope where it changes the effect of a control.
- Use the [content rules](quiet-hairline.md#content-and-progressive-disclosure) to decide whether `description` is needed. The prop is optional; do not fill it with a paraphrase of the title.

## Canonical reference surfaces

- Task sheet: `TaskDetailPanel`, opened through `GlobalTaskPanel`.
- Epic detail: `EpicDetailPage`.
- Objective cards and full detail page: `ObjectivesPage` and `ObjectiveDetailPage`; preserve the card layout and the detail’s existing route.
- CRM identity/detail: `ContactDetailPage` and `CompanyDetailPage`.
- Automation routes: Flows, Activity, Agents, Trigger Catalog, Skill Catalog, and Tool Catalog.

Reference surfaces demonstrate composition and domain behavior; the Quiet primitives and design reference are authoritative when an older local class or visual treatment conflicts.

## Component decisions

- Use `QuietSection` or a hairline list for general page sections. Settings forms follow the explicit [settings section composition](#settings-section-composition), including its shared bordered containers.
- Use `QuietMetricGrid` and `QuietMetricBlock` when a small set of earned operational numbers needs the shared Automation Activity hairline treatment; do not recreate separate metric cards.
- Do not use `Badge` for ordinary status, counts, lifecycle, filters, or metadata. Use `QuietStatusText` or inline text. Editable applied filters use the shared `PMFilterPill` exception above, not a custom Badge.
- Do not use default `Button` colors or boxed `Input`/`Tabs` styling for Quiet page chrome. Use the Quiet wrappers.
- Search is the exception to underline-only form controls: use `QuietSearchInput` throughout product surfaces. It centralizes the Skill Catalog's compact bordered treatment and leading icon. Do not recreate it or use `QuietUnderlineInput` for search; set only `containerClassName` when page layout requires a narrower width and use its `trailing` slot for clear/close actions. Keep behavior-owned `CommandInput`, editor search/replace, and content-preview controls on their specialized primitives.
- Relationship pickers use the centralized responsive dialog (576px at `sm`, 672px at `lg`, and 768px at `xl`) and contained results primitives. Result rows must shrink with `min-w-0`, hide horizontal overflow, and truncate their primary/secondary text; do not let unbounded names determine the overlay width.
- `QuietPrimaryAction` must preserve the centralized Helpin `Button size="sm"` geometry and curvature; only its color and hierarchy differ.
- Do not duplicate section-heading, tab, row, action, property-row, or empty-state class strings in a page.
- Domain components may retain a compact chip only when the shape itself is established user data or a removal affordance, such as an editable label/token collection.
- Task and epic comments and CRM email bodies/replies use the centralized conversation-composer primitives. The primitives mirror the Support `ReplyComposer` reference and own the Support-derived shell and blue emphasized border, complete rich-text action order, AI rewrite menu, keyboard hint, and send-action geometry. Keep the heavily used Support implementation behavior-stable; mirror intentional visual-contract changes between it and the primitives rather than moving Support-specific drafts, presence, shortcuts, recipient rules, or uploads into the design system. Domain components own editor extensions and content, identity/mode headers, mention panels, permissions, billing errors, and submission. Do not migrate Docs comment composers implicitly.

### Form dialogs

Use `frontend/src/components/crm/CreateDealDialog.tsx` as the default field presentation for create/edit form dialogs, including Objective Add Key Result. Text, date, and numeric fields use `QuietUnderlineInput`; selectors use the shared dropdown adapter with `SelectTrigger variant="underline" className="w-full px-0.5"`. Keep labels above fields and associate them with control IDs. Use `QuietTextAction` for Cancel and `QuietPrimaryAction` for the submit action.

Focused form fields emphasize their bottom border while retaining a transparent background. Reuse the component’s focus styling; do not add rectangular `outline-2`, box rings, or compact filled/ghost field triggers. Dropdown option selection and search retain their own shared behavior and styling. Verify the actual focused text field and open selector in light and dark mode.
