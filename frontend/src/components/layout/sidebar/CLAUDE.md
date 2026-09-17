# Claude Sidebar Context

This folder is the local architecture boundary for the workspace sidebar.

## What matters here

- `../Sidebar.tsx` should stay thin and orchestrate:
  - router state
  - workspace/auth/support hooks
  - sidebar-level UI state
  - composition of subcomponents from this folder
- This folder owns the sidebar presentation structure, not global app settings models.

## Design Intent

- Split by concern, not by arbitrary file count.
- Shared settings metadata lives in `@/lib/settingsSections`.
- Sidebar-specific persistence lives in `state.ts`.
- Pure active-link/navigation logic lives in `navigation.ts`.
- Module-specific rendering lives in dedicated components.

## Preferred change pattern

1. If the change is static nav structure, update `config.ts`.
2. If the change is a settings nav item, update `@/lib/settingsSections`.
3. If the change is local UI persistence, update `state.ts`.
4. If the change is route/activity matching, update `navigation.ts`.
5. If the change grows one rail section noticeably, extract a focused component in this folder instead of expanding `Sidebar.tsx`.

## Avoid

- Reintroducing a monolithic `Sidebar.tsx`.
- Copy-pasting settings section definitions into sidebar/header/page files.
- Mixing data-fetching business logic into the leaf sidebar components.
- Building an overly generic rail framework that makes support/docs/projects harder to read.

## Good outcomes

- `Sidebar.tsx` reads like a controller/composer.
- Each rail-specific component is easy to scan.
- Header/settings/sidebar labels stay in sync because they share one metadata source.

## Validation

Use:

```bash
cd frontend
pnpm exec tsc -b
pnpm run build
```
