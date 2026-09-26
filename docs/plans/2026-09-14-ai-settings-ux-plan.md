# AI settings pages: restyle and UX plan


This historical implementation plan records the September AI settings redesign.
Use the current notes before following its component names or visual acceptance
criteria; the page structure has evolved since the proposed card layout.

## Current implementation and limits

Source-compared on 2026-09-18. The original live-dev observations and verification
checklist are historical, not a fresh browser or test report.

- [AISettingsPage](../../frontend/src/pages/settings/AISettingsPage.tsx) uses
  Workspace/Personal tabs in one settings frame and table-based connection/profile
  rows. `AIConnectionRow` and `AIProfileRow` replace the proposed card components.
  The personal route initializes the Personal tab; it is not a separate page
  implementation. Workspace management is gated by `workspace.update`, while
  the personal tab passes its personal-management capability separately.
- The [settings registry](../../frontend/src/lib/settingsSections.tsx) uses
  `workspace.read` for both AI route entries, rather than the proposed
  `settings.read` on workspace AI. Frontend visibility is not a substitute for
  backend authorization.
- Query hooks, provider metadata, the connection dialog, status badge, route
  fields, and [model combobox](../../frontend/src/components/agents/AIModelCombobox.tsx)
  exist. The combobox offers catalog suggestions and custom identifiers; a
  selectable custom string does not establish provider support for that model.
- [AIProfileEditor](../../frontend/src/components/agents/AIProfileEditor.tsx)
  scopes primary/fallback choices to the page scope, excludes the primary
  connection from fallback choices, and validates missing fields and duplicate
  primary/fallback connections. Connection disconnect and profile deletion use
  confirmation dialogs in their respective row components.
- [Device login](../../frontend/src/components/agents/ChatGPTDeviceLogin.tsx)
  implements polling and expiry handling. Poll intervals have a five-second
  minimum, so “poll at interval_seconds” below is not exact for smaller values.
  This review did not initiate authentication or verify external provider flows.
- [Community pricing text](../../frontend/src/edition/community/ai.tsx) is returned
  only when the policy mode is `community`; otherwise the helper returns null.
  The plan's claim that the fee line is never empty is too broad.

Current tests include row-oriented tests instead of the proposed card tests.
The old build, edition, mobile-width, dark-mode, and accessibility checklist still
requires explicit execution to establish those results; source availability
alone does not prove all of it passed.

## Original implementation plan

## Context

The AI profiles branch added two settings pages, Personal → AI connections (`/w/$slug/settings/ai-connections`) and Workspace → AI (`/w/$slug/settings/ai`), plus the "Manage connections" dialog and the profile editor. They work but look and behave unlike the rest of Helpin. Verified against the live dev site and the code:

- The page wraps everything in `QuietSection` (full-bleed hairlines) inside the already padded settings viewport, and repeats the registry description as a body paragraph. Neighbouring pages (Skill Catalog, External MCP, Git connections) use bordered card rows with an icon tile, status badge, muted meta line, and hover actions.
- Providers render as raw slugs on the page (`openai_chatgpt`) but friendly labels in the dialog. Status is lowercase text with no colour. There are no provider icons anywhere in the app.
- The page list is read-only; reconnect/disconnect live only inside a nested dialog. Delete profile and Disconnect fire with no confirmation. Forms show no validation messages; buttons are just disabled. "Create profile" and the workspace default select are disabled for several reasons with no explanation. Errors are inline `role=alert` paragraphs instead of sonner toasts. Data fetching uses ad-hoc `useQuery` keys. Both sidebar entries share one icon and have no `requiredPermission`. The model field is free text although `generated/aiModels.ts` has a provider-grouped catalog.

Decisions made with the user: adopt Skill Catalog card styling; manage connections inline on the page with an "Add connection" header action; model field becomes a combobox with catalog suggestions and free text.

Backend contract is untouched: `/ai-connections` list/create/poll/reconnect/disconnect/endpoints, `/ai-profiles` list/save/remove, `/ai-settings` get/setDefault.

## Reference patterns to reuse (do not invent new primitives)

- Card row: `frontend/src/pages/automation/SkillCatalog.tsx` `SkillCard` (container `rounded-lg border border-border/70 bg-card px-4 py-3.5 hover:border-border hover:shadow-sm`, hover-revealed icon actions, uppercase group label with count at `SourceSection`), dialog chrome (bordered `px-6 py-4` header, scrollable body, `border-t` footer with `Button size="sm"` outline Cancel + primary), `RequiredFieldLabel`, manual `useState` + `toast.error` validation.
- Icon tile and status badge map: `frontend/src/components/automation/ExternalMCPConnections.tsx` (`h-9 w-9 rounded-lg border bg-muted/30` tile, `STATUS_COPY` emerald/amber/destructive `Badge variant="outline"`, dashed hero empty state, `useConfirm` from `@/components/ui/confirm-dialog`).
- Settings shell: `frontend/src/pages/settings/SettingsPageFrame.tsx` (registry-driven header, `headerAction` prop, Skeleton loading); dedicated route files like `routes/_authenticated/w/$slug/settings/mcp.tsx`.
- Form fields: `QuietUnderlineInput`, `Select` from `design-system/quiet-dropdown-select` with `variant="underline"` (the adapter renders `SelectItem` children as rich option content, so disabled items can carry a reason line). Combobox: `QuietDropdown` with `searchMode="always"`, `groups`, `empty` slot.
- Monochrome brand icon rendering: CSS mask + `bg-current` pattern in `frontend/src/components/docs/helpcenter/SocialPlatformIcon.tsx`; local SVG asset import pattern in `components/settings/ImportTab.tsx`. Design rule: no second icon library. Hugeicons has no OpenAI/OpenRouter glyph, so ship local SVG marks.
- Query hooks: copy `hooks/queries/useExternalMCP.ts` shape; keys in `src/lib/queryKeys.ts`.

## Implementation

### PR 1: data layer, routes, registry (no visual change)

- `src/lib/queryKeys.ts`: add `ai.root/connections/endpoints/profiles/settings(ws)`.
- New `src/hooks/queries/useAIConnections.ts` (list, endpoints, create, reconnect, poll, disconnect) and `useAIProfiles.ts` (profiles, settings, save, delete, setDefault); export from `hooks/queries/index.ts`. Mutations invalidate `queryKeys.ai.root`.
- Migrate `AIProfilePicker.tsx`, `AIProfileLabel.tsx`, `AISettingsPage.tsx` off raw `useQuery`; update tests seeding `['ai-profiles', ws]` (`CustomAgentCreatePanel.test.tsx`, `crmAutomationTargets.test.tsx`, `AIProfilePicker.test.tsx`) to `queryKeys.ai.profiles`.
- Add route files `settings/ai.tsx` and `settings/ai-connections.tsx`; remove the `ai` branch from `settings/$section.tsx`; point `AISettingsLink.tsx` at the static routes.
- `lib/settingsSections.tsx`: distinct icons (`Key01Icon` for personal connections, `AiMagicIcon` or `AiNetworkIcon` for workspace AI), `requiredPermission` (`workspace.read` / `settings.read`), sharper descriptions; fix the off-pattern indentation. Extend `settingsSections.test.ts`.

### PR 2: provider metadata and icons

- New `src/lib/aiProviders.ts`: `providerMeta` as the single source of truth per provider key (`label`, `shortLabel`, `icon`, `description`, `needsApiKey`, `supportsControls {reasoningEffort, serviceTier, openrouterQuantizations}`, `scopes`), `PROVIDER_ORDER`, `connectionStatusMeta` (connected → Connected/emerald, pending → Awaiting login/amber, reauthorization_required → Reconnect required/amber, disconnected → Disconnected/muted), `modelCatalogFor(provider)` grouping `AI_MODELS.models` by tier, `catalogLabel(provider, model)`. Replaces the private `providerLabels` map in `AIConnectionsDialog.tsx` and inline provider branching in the editor.
- New `src/assets/ai-providers/{openai,anthropic,openrouter,chatgpt,endpoint}.svg` (single-path monochrome) and `components/agents/ProviderIcon.tsx` (`ProviderIcon`, `ProviderIconTile`) using the mask pattern; unknown provider falls back to the endpoint mark.
- New `components/agents/AIConnectionStatusBadge.tsx`.
- Use `providerShortLabel` in `AIProfilePicker` route summary and `AIExecutionDetails`. Give the community edition real copy in `edition/community/ai.tsx` ("Usage is billed by your provider account. No Helpin token fee.") so the fee line is never empty.

### PR 3: Add/Reconnect connection dialog and ChatGPT device login

- New `components/agents/AIConnectionDialog.tsx` replacing `AIConnectionsDialog.tsx` (delete it and its test; port the no-auth endpoint test). Modes: `add` and `reconnect(connection)`. Skill Catalog dialog chrome.
  - Add: Provider select (icon + label + one-line description; filtered by scope and model availability; ChatGPT personal-only), Name (auto-filled from provider short label), then per provider: API key with helper "Stored encrypted"; ChatGPT: no key, primary button "Start device login"; compatible: Approved endpoint select with base URL and "No API key is sent." when `auth_mode === 'none'`, key field only for `api_key` endpoints; endpoint loading/empty/error states inline.
  - Reconnect: name/provider read-only in header; key field only; ChatGPT restarts device login.
  - Validation via `toast.error` in order (name, provider, endpoint, key); primary button disabled only while pending. Success toasts, invalidate `queryKeys.ai.connections`.
- New `components/agents/ChatGPTDeviceLogin.tsx` with `useChatGPTDeviceLogin`: explicit states waiting/connected/failed/expired/cancelled; `setTimeout` polling at `interval_seconds`; countdown from `expires_at` with no non-null assertion; copy-code button; Cancel stops polling and leaves the connection `pending` (card shows "Continue login"); "Start again" on expiry.

### PR 4: page restyle and inline management

- Rewrite `pages/settings/AISettingsPage.tsx`: `SettingsPageFrame` with `headerAction` "Add connection" (hidden when not manageable); body `space-y-7` composed of new `components/settings/ai/AIConnectionsSection.tsx`, `AIWorkspaceDefaultSection.tsx` (workspace only), `AIProfilesSection.tsx`, and `AISectionLabel.tsx` (uppercase label · count · optional right action). No duplicated description, no `QuietSection`/`QuietListRow`, no page-global busy flag.
- New `components/agents/AIConnectionCard.tsx`: icon tile, name, status badge, "Managed" badge, meta "Provider label · Personal|Workspace", endpoint line for compatible, fee/policy line, expiry line; hover actions Continue login / Reconnect / Disconnect (always visible when not connected); Disconnect via `useConfirm` with consequences copy; managed connections show no actions and say "Managed by your administrator".
- New `components/agents/AIProfileCard.tsx`: primary provider tile, name, "Workspace default" badge, "Needs attention" badge when the primary connection is missing or not connected, Primary and Fallback route lines ("Provider · Model label (id) · via Connection name"), policy lines; hover actions Set as default (workspace, when allowed), Edit, Delete via `useConfirm`.
- States: page Skeleton grid; `!enabled` dashed hero; query errors as `Alert` with Retry; connections empty hero with "Add your first connection"; profiles empty hero with "Create profile" or, with zero connections, "Add a connection first" plus an "Add connection" action; read-only `Alert` for members without manage permission.
- Disabled-state reasons: "Create profile" gets a tooltip and the hero text "Add a connection before creating a profile"; the default-profile select gets one `aria-describedby` reason line chosen in order (no permission, profiles error, settings error, no shared profiles, saving, else informational); policy-blocked options render a second muted line with the policy message inside the dropdown.
- All mutation feedback through sonner toasts.

### PR 5: profile editor

- New `components/agents/AIModelCombobox.tsx` on `QuietDropdown`: tier-grouped suggestions from `modelCatalogFor(provider)` (ChatGPT uses the OpenAI catalog, compatible has none), always-on search, "Use "<query>"" custom option, helper text showing tier description for catalog matches or "Custom model" otherwise; disabled with "Choose a connection to see suggested models" until a connection is picked.
- New `components/agents/AIRouteFields.tsx` extracted from `ProfileRouteFields`: connection select with provider icon and disabled reasons (reconnect required, awaiting login, disconnected, policy message), the combobox, and controls gated by `providerMeta.supportsControls` in a two-column grid.
- Rewrite `AIProfileEditor.tsx`: Skill Catalog chrome; Primary route and Fallback route sections; fallback list scoped to the page scope and excluding the primary connection (fixes the unscoped fallback list); validation toasts in order; success/conflict toasts.

### PR 6: polish

Dark mode and narrow widths for cards and dialogs, `aria-describedby` wiring, hover-action visibility rules, remove leftover Quiet list imports, confirm `routeTree.gen.ts` regenerates on build.

## Tests to add (Vitest, jsdom, mirroring `AIConnectionsDialog.test.tsx` style)

- `lib/__tests__/aiProviders.test.ts`: labels, fallback for unknown key, catalog grouping and `enabled` filter, status map completeness.
- `components/agents/__tests__/ProviderIcon.test.tsx`, `AIConnectionDialog.test.tsx` (ported no-auth case, empty-name toast, ChatGPT hidden for workspace scope, reconnect payload), `ChatGPTDeviceLogin.test.tsx` (fake timers: poll, connected, expired stops polling, cancel clears timer, missing `expires_at` safe), `AIModelCombobox.test.tsx`, `AIProfileEditor.test.tsx` (scoped fallback list, disabled reasons, validation, save payload), `AIConnectionCard.test.tsx`, `AIProfileCard.test.tsx` (badges, managed has no actions, confirm gating).
- `pages/settings/__tests__/AISettingsPage.test.tsx`: not-configured hero, empty connections with disabled "Create profile" reason, default-profile reason with zero shared profiles, description not duplicated.
- `edition/community/__tests__/ai.test.tsx`, extended `settingsSections.test.ts`.

## Verification

1. `pnpm --dir frontend build` and `pnpm --dir frontend build:ee` (typecheck both editions; 4 GB heap), `pnpm --dir frontend test`, `bash scripts/check-community-frontend.sh`.
2. On the dev site (agent-browser, workspace `helpin`): Settings → AI and → AI connections in light and dark mode, 1440 and 390 px widths. Check: header action, card rows with icons and status badges, hover actions, empty heroes, read-only alert as a non-admin, default select reason text, confirm dialogs on Disconnect and Delete, toasts on error and success.
3. Dialog flows: add OpenAI key with empty name (toast), add compatible no-auth endpoint (no key field, "No API key is sent."), ChatGPT device login shows code, countdown, cancel, and "Continue login" on the card afterwards; reconnect a key; create a profile with a catalog model and with a custom id; fallback list excludes the primary connection.
4. Regression: Ask Agent dock, Run now dialog, agent form profile pickers still render names with friendly provider labels; `AIExecutionDetails` in run details shows provider short label.
