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
| Underline input/control | `QuietUnderlineInput`, `quietUnderlineControlClassName` |
| Detail property | `QuietPropertyRow` |
| Stacked scan row | `QuietListRow` |
| Inline facts/provenance | `QuietMetaLine` |
| Dot plus status word | `QuietStatusText` |
| Working empty state | `QuietEmptyState` |

These components own visual invariants. Pages own data, navigation, permissions, and domain actions.

Use `QuietPageHeader` for normal authenticated index and settings routes. Use `QuietDetailHeader` when a detail page needs ancestor breadcrumbs, a single editable identity/document title, metadata, status, save state, and page-specific actions. Its left side renders one compact line beneath the title with metadata first and semantic `status` last. Its right side renders actions with transient save/operation `state` directly underneath. Use `QuietDetailAction` for those actions so phones receive round icon controls and larger screens receive labels without page-owned responsive class strings; use its `iconOnly` option for controls such as the Docs details toggle that should remain icon-only at every breakpoint. The header owns layout and sidebar clearance; pages retain navigation, permissions, validation, and persistence. Keep purpose-built editor or workflow toolbars only when their controls cannot fit this composition, and apply `workspaceSidebarSafeInsetClassName` to every top/loading toolbar state instead of recreating the collapsed-sidebar spacing class.

## Existing behavior primitives

Keep using the established components below. Prefer the listed quiet presentation rather than rebuilding behavior.

- Overlays: `Sheet`, `Dialog`, `Popover`, `DropdownMenu`, `Tooltip`, and `QuickTooltip` from `@/components/ui`.
- Form behavior: `Input` with `variant="plain"`, existing Select/Command components, `DatePicker`, `TiptapEditor` with `variant="divider"`, and react-hook-form/zod where already used.
- Identity: `UserAvatar`; provide `fallbackColorSeed` when a stable email is available.
- PM: `SidebarPopoverSelect`, `MemberPickerPopover`, `SaveIndicator`, `Attachments`, `DetailDescriptionEditorActions`, `DetailDescriptionEditButton`, `TaskUpdatesView`, `EpicUpdatesView`, and routing helpers.
- CRM: `EntitySummaryCard` with `presentation="overview"`, `EntitySignals`, `ActivityTimeline` and `EmailTimeline` borderless presentations, `CompanyDetailCollections`, `LinkedTasksPanel`, and enrichment/association components.
- Automation: retain flow composers, agent editors, run drawers, tool selectors, query hooks, permission checks, and billing/error handling. Centralize only their page chrome and recurring visual patterns.

## Canonical reference surfaces

- Task sheet: `TaskDetailPanel`, opened through `GlobalTaskPanel`.
- Epic detail: `EpicDetailPage`.
- CRM identity/detail: `ContactDetailPage` and `CompanyDetailPage`.
- Automation routes: Flows, Activity, Agents, Trigger Catalog, Skill Catalog, and Tool Catalog.

Reference surfaces demonstrate composition and domain behavior; the Quiet primitives and design reference are authoritative when an older local class or visual treatment conflicts.

## Component decisions

- Do not use `Card` to structure a new page section. Use `QuietSection` or a hairline list.
- Use `QuietMetricGrid` and `QuietMetricBlock` when a small set of earned operational numbers needs the shared Automation Activity hairline treatment; do not recreate separate metric cards.
- Do not use `Badge` for ordinary status, counts, lifecycle, filters, or metadata. Use `QuietStatusText` or inline text.
- Do not use default `Button` colors or boxed `Input`/`Tabs` styling for Quiet page chrome. Use the Quiet wrappers.
- `QuietPrimaryAction` must preserve the centralized Helpin `Button size="sm"` geometry and curvature; only its color and hierarchy differ.
- Do not duplicate section-heading, tab, row, action, property-row, or empty-state class strings in a page.
- Domain components may retain a compact chip only when the shape itself is established user data or a removal affordance, such as an editable label/token collection.
