# Sidebar Folder Guide

This folder contains the composable pieces that make up the app sidebar shell in `../Sidebar.tsx`.

## Purpose

- Keep `Sidebar.tsx` as an orchestrator, not a giant UI file.
- Keep route/config/state helpers close to the sidebar components that use them.
- Avoid duplicating settings navigation metadata across the app.

## File Roles

- `config.ts`
  - Static sidebar configuration and lightweight builders.
  - Rail definitions, project create options, support filter items, settings group builders.
- `state.ts`
  - Sidebar-only persistence helpers for localStorage-backed UI state.
- `navigation.ts`
  - Pure navigation helpers such as active-link checks.
- `types.ts`
  - Shared sidebar-only TypeScript types.
- `SidebarRail.tsx`
  - The left icon rail plus theme toggle and account menu slot.
- `SidebarAccountMenu.tsx`
  - User avatar menu and support presence controls.
- `SidebarCreateBar.tsx`
  - Shared top action bar used by module rails such as Projects and Docs.
- `StandardRailNav.tsx`
  - Generic grouped nav renderer for simple rails.
- `SettingsRailNav.tsx`
  - Settings-specific grouped nav with collapsible section groups.
- `SupportRailNav.tsx`
  - Support rail filters, AI section, and Team Inboxes.
- `ProjectsTeamsNav.tsx`
  - Team-scoped project navigation.
- `DocsRailNav.tsx`
  - Active-space picker, scoped creation, quick links, and the Docs collection/document tree.

## Source Of Truth Rules

- Do not hardcode settings sections in this folder.
  - Use `@/lib/settingsSections`.
- Do not duplicate settings labels in the header or settings page.
  - If a settings item changes, update `@/lib/settingsSections` first.
- Keep support mailbox data/query logic outside this folder.
  - This folder renders sidebar UI from hooks/stores provided by the shell.

## Editing Rules

- Prefer extracting a focused component/helper over growing `Sidebar.tsx`.
- Keep helpers pure when possible.
- Keep components module-aware, but avoid over-abstracting every rail into one generic system if the rails differ meaningfully.
- Preserve current route shapes and store contracts unless the change explicitly requires otherwise.
- If you add a new sidebar settings item:
  - add it in `@/lib/settingsSections`
  - let `config.ts` consume it indirectly
  - do not add a separate sidebar-only list

## Verification

After changes in this folder, run:

```bash
cd frontend
npx tsc -b
npm run build
```

## Notes

- The sidebar intentionally mixes static config with dynamic query-backed sections, but rendering and orchestration should stay separated.
- Keep `Sidebar.tsx` responsible for wiring hooks, stores, and navigation only.
