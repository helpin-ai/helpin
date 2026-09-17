# Mobile Support Inbox — Views Drawer & Filter Parity Design

**Date:** 2026-07-12
**Status:** Approved in brainstorming; pending spec review → implementation plan
**Branch:** `feat/support-mobile`

## Plain-language summary (for the product owner)

Today the mobile inbox has three tabs across the top — Mine / Unassigned / All. The web
support module has a much richer set of views in its sidebar. This change replaces the three
tabs with a **slide-out drawer** (the mobile version of the web sidebar) containing the same
grouped views, the same team inboxes, the same custom views, and the **same per-view unread
counts** as the web app. Crucially, tapping a view on mobile shows the **exact same
conversations** as the same view on web, because both apps read one shared "rulebook" for what
each view means — and we do this **without changing the web app**.

## Goal

Bring the mobile inbox's navigation and filtering to parity with the web support module:

1. All web views reachable on mobile: **Inbox, Mine, Waiting, Resolved, Spam** (primary group),
   **AI Handling, AI Resolved** (AI group), **Custom views**, and **Team inboxes**.
2. Each view returns **identical conversations** to the same view on web.
3. Each view shows a **per-view unread count** matching web.
4. **The web app is not modified.** No duplicated filter logic that must be updated twice.

## Non-goals (explicitly out of scope for this change)

- Fine-grained sub-filters *within* a view (extra status/tag/AI toggles). Later.
- Creating / editing / deleting custom views on mobile. The drawer lists existing custom views
  read-only; authoring stays on web. Later.
- Search (already separate and unchanged).
- Any backend change. All endpoints already exist.

## Constraints (from the product owner)

- **C1 — Parity:** same conversations per view as web, never divergent.
- **C2 — Don't touch web:** no edits to `frontend/` source.
- **C3 — No double-maintenance:** exactly one source of truth for filter definitions; a future
  change to a view's meaning must propagate to mobile automatically.

## The web app's structure (reference — what we are matching)

- **View definitions / "rulebook"** live in web code:
  - `frontend/src/lib/supportInboxFilters.ts` — `NavFilter` → default states / assignment / AI
    states (`defaultStatesForNav`, `defaultAIStatesForNav`, `defaultAssignmentForNav`,
    `defaultConversationListFiltersForNav`), plus the `ConversationListFilters` type and the
    conversation predicates it re-exports from `@/components/support/helpers`.
  - `frontend/src/lib/supportInboxRouting.ts` — view-name ↔ `NavFilter` mapping
    (`navFilterFromView`, `viewFromNavFilter`, `buildSupportInboxSearch`, `VIEW_BY_FILTER`,
    `FILTER_BY_VIEW`).
  - `NavFilter` type = `'inbox' | 'mine' | 'waiting' | 'resolved' | 'spam' | 'ai_active' |
    'resolved_by_ai'` (defined in `frontend/src/stores/supportInboxStore.ts`).
- **Conversations endpoint:** `GET /api/support/inbox/conversations` (shared; mobile already
  uses it via `support-core`'s `supportService.listConversations`).
- **Mailboxes (team inboxes):** `GET /api/support/inbox/mailboxes` (mobile already uses via
  `useSupportMailboxes`).
- **Custom views:** `GET /api/support/inbox/views` (web `useSupportInboxViews`).
- **Per-view counts:** `GET /api/support/inbox/views/counts` → `SupportInboxViewCount[]` where
  each item is `{ view_id: string, total_count: int, unread_count: int }`.
  (`view_id` keying — builtin view keys vs custom-view ids vs mailbox scoping — to be traced
  from `frontend/src/components/layout/Sidebar.tsx`, which renders these badges today.)

## Design

### 1. Navigation: left-edge slide-out drawer

- Remove the Mine/Unassigned/All `SegmentedControl` and the separate `MailboxSheet` from the
  inbox screen. Both are absorbed into one drawer.
- Inbox `TopBar` shows the **current view's display name** (e.g. "Inbox", "Mine", or a team
  inbox name) plus a leading **menu (hamburger) button**. The trailing mailbox button is
  removed.
- The drawer opens by (a) tapping the menu button, or (b) a **left-edge swipe** on the inbox.
  - **Gesture reconciliation:** left-edge-swipe currently means "back," but only inside the
    conversation screen (`useEdgeSwipeBack`, enabled on non-tab-root routes). The inbox is a tab
    root with no back target, so the left edge is free for the drawer. No conflict; the drawer's
    edge-swipe is wired only on the inbox screen.
- Drawer is a left-anchored overlay panel (~82% width, max ~360px) with a scrim, spring-in from
  `x: -100% → 0` (reuse `stackSpring`; crossfade under `prefers-reduced-motion`). Dismiss by
  tapping the scrim, swiping left, or selecting an item.
- **Contents (grouped, scrollable), mirroring the web order exactly:**
  - **Views:** Inbox · Mine · Waiting · Resolved · Spam
  - **AI:** AI Handling · AI Resolved
  - **Custom views:** dynamic from `GET /inbox/views`
  - **Team inboxes:** dynamic from `GET /inbox/mailboxes` (plus an "All inboxes" entry)
  - (Exact presence/placement of "Unassigned" / "Mentions" to be matched to the web sidebar's
    rendered list during planning; the group order above is authoritative.)
- Each row: label + a trailing **unread count badge** (from the counts endpoint; hidden when 0).
  The active view is highlighted. Rows are `Pressable` (≥44pt), `selection` haptic on tap.
- Selecting a row: closes the drawer, sets the active view/mailbox, the inbox list re-queries,
  and the TopBar title updates.

### 2. Filter parity — read the web's rulebook in place (satisfies C1, C2, C3)

- The mobile app adds **narrow, read-only build aliases** pointing at the web's existing filter
  modules, so there is exactly one copy (web-owned) that mobile reads:
  - `@/lib/supportInboxFilters` → `frontend/src/lib/supportInboxFilters.ts`
  - `@/lib/supportInboxRouting` → `frontend/src/lib/supportInboxRouting.ts`
  - `@/stores/supportInboxStore` → `frontend/src/stores/supportInboxStore.ts` (for the
    `NavFilter` type; type-only, erased at runtime)
  - plus whatever these transitively require (notably `@/components/support/helpers` — pure
    conversation predicates — and type-only `./pmTypes`). The transitive set is enumerated and
    aliased **explicitly, per-path**; we deliberately do **not** add a blanket `@ → frontend/src`
    alias, so a developer cannot accidentally import web *UI* into the mobile bundle.
- Both `vite.config.ts` and the mobile `tsconfig` get these path aliases (build + typecheck).
- Mobile derives each view's server query from the **same functions** the web uses
  (`defaultConversationListFiltersForNav` / `navFilterFromView` etc.), then converts that to the
  `support-core` `ConversationFilters` shape sent to `listConversations`. **The exact
  web filters→API-query conversion must be reused, not re-derived** — during planning, trace how
  the web turns `ConversationListFilters` into the `/inbox/conversations` query params (in
  `frontend/src/hooks/queries/useSupport.ts` / its service) and reuse that path so the query is
  byte-identical. This is the single most important correctness point in the change.

**Tradeoff (accepted):** mobile gains a build-time dependency on the location/shape of those web
files. If the web relocates or refactors them, the mobile **build fails loudly** at compile
time — never a silent wrong-data condition. It also pulls a small amount of web *logic* (not UI)
into mobile. This intentionally bends the mobile app's original "fully independent from web"
principle; it is the correct trade to satisfy C1+C2+C3 simultaneously. (The textbook-cleanest
alternative — extract the rulebook to `packages/support-core` and have web import it — was
rejected because it requires editing web, violating C2.)

### 3. Per-view unread counts

- Add to `support-core`: `supportService.listInboxViewCounts(workspaceId)` →
  `GET /support/inbox/views/counts`, and a `useSupportInboxViewCounts(workspaceId)` query hook,
  mirroring the web service/hook 1:1. Type `SupportInboxViewCount = { view_id, total_count,
  unread_count }`.
- The drawer maps each row to its `view_id` and renders `unread_count` (reusing
  `formatBadgeCount` for the "99+" cap). Because it is the same endpoint the web uses, the
  numbers match by construction.
- Counts refresh on realtime support events (the drawer's query is invalidated alongside the
  existing conversation/unread-stats invalidations in `useMobileRealtime`).

### 4. State & data flow

- Introduce a mobile `supportViewStore` (zustand): `{ activeView: NavFilter | customViewId,
  selectedMailboxId }`, persisted in-session (survives inbox↔thread navigation, not app restart).
  Replaces the ad-hoc segment/mailbox local state in `inbox-screen.tsx`.
- `inbox-screen.tsx` reads the active view + mailbox from the store, derives filters via the
  reused web rulebook, and passes them to `useConversations`.
- Custom views: selecting one sets `activeView = customViewId`; its saved filter payload
  (from `/inbox/views`) is parsed via the web's `parseSupportInboxViewFilters` and applied.

## Components & files

**Mobile (new):**
- `src/inbox/views-drawer.tsx` — the drawer overlay: grouped list, counts, active state, gestures.
- `src/inbox/view-list-config.ts` — the grouped view model (labels, order, `view_id` mapping)
  built on top of the reused web `NavFilter`/routing definitions.
- `src/stores/support-view-store.ts` — active view / mailbox state.
- `src/inbox/use-inbox-filters.ts` — bridges reused web rulebook → `support-core`
  `ConversationFilters` for the current view.

**Mobile (modified):**
- `src/screens/inbox-screen.tsx` — remove segmented control + mailbox sheet; add TopBar menu
  button + current-view title; mount the drawer; source filters from the store.
- `src/lib/use-mobile-realtime.ts` — also invalidate the view-counts query.
- `vite.config.ts` + `tsconfig.app.json` — the per-path web aliases.
- Remove `src/inbox/mailbox-sheet.tsx` (absorbed into the drawer) — or repurpose its list.

**support-core (new, additive):**
- `listInboxViewCounts` service method + `useSupportInboxViewCounts` hook.
- `listInboxViews` (custom views) service method + hook, if not already present.

**Web:** none. (C2)

## Testing

- **Unit (mobile, vitest):** view-list config (group order, labels, `view_id` mapping); the
  rulebook→`ConversationFilters` bridge for each builtin view (assert the exact params object per
  view — this is the parity guard); `formatBadgeCount` reuse; drawer open/close + active-state
  logic (pure parts).
- **Parity check (planning-time, manual + a recorded fixture):** for each view, capture the
  query the web sends and assert the mobile bridge produces the same params. Where feasible,
  encode a table-driven test mapping `NavFilter → expected query params` derived from the reused
  web functions, so a future web change that alters a view surfaces as a mobile test diff.
- **On-device (handoff, added to `apps/support-mobile/README.md` checklist):** drawer open via
  button + edge-swipe; each view lists the same conversations as web (spot-check 3–4 views side
  by side); counts match; team-inbox + custom-view selection; reduced-motion.

## Open items to pin during planning (no product decision needed)

1. Exact web sidebar view list/order and `view_id` keys — trace `frontend/src/components/layout/
   Sidebar.tsx` + the counts consumer; confirm whether "Unassigned"/"Mentions" are top-level and
   where.
2. The exact web `ConversationListFilters` → `/inbox/conversations` query-param conversion to
   reuse verbatim (the parity crux).
3. Final transitive alias set required to compile the reused web modules (enumerate from the
   import graph of `supportInboxFilters` / `supportInboxRouting` / `helpers`).
