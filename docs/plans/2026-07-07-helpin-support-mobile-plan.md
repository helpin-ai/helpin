# Helpin Support mobile implementation plan

This historical plan explains the original mobile support app: an independent Tauri interface using shared support data and actions. It preserves the initial design and acceptance criteria; use the [mobile README](../../apps/support-mobile/README.md) for current setup and the [QA record](../../apps/support-mobile/QA.md) for dated validation and pending device checks.

## Source review — September 18, 2026

- The app, shared mutations, Rust shell, native push plugin, registration API, and Firebase sender exist. Current manifests request Tauri `2.11.1`; the app product name is `Helpin`, identifier `ai.helpin.mobile`, and development port 5176. Treat dependency versions in the scaffold below as historical examples.
- Current navigation has Inbox, Mine, Search, and Settings tabs. Search, canned responses, AI rewrite, attachments, tag actions, contacts/CRM context, linked tasks, and agent interactions have expanded the original scope. The deferred-feature list below is not a current missing-feature inventory; see `src/router.tsx`, `src/navigation/tab-bar.tsx`, and `src/thread/composer.tsx`.
- UI remains independently implemented, but `vite.config.ts` now explicitly aliases selected pure web support logic and widget emoji helpers. The original claim that only support-core/shared imports are allowed is obsolete; there is still no blanket alias to the web UI.
- Native session storage uses `LazyStore` at `auth/session.json`, backed by an in-memory adapter. This code does not establish Keychain/Keystore encryption. Browser development uses the browser session adapter. Do not equate the Tauri store with a secure credential vault.
- Push registration routes are authenticated and user-scoped. `support_notification.go` checks recipient access and existing `in_app` preferences before delivery. `push-registration.ts` unregisters before logout but allows failures or a three-second timeout; sign-out therefore does not guarantee immediate device-row deletion or cessation of all queued notifications.
- `push_devices` is included in the versioned core foundation schema. Community installations use versioned migrations; adding an AutoMigrate model alone is insufficient. The plan's native push spike findings file was not located. Source files and a desktop-host Cargo CI check do not prove signed iOS/Android builds, store publication, hardware push delivery, or the performance/accessibility budgets below.

This review inspected source and CI configuration only. No native build, hardware test, Firebase request, app-store check, or release operation was performed. Original checkboxes and expected results remain historical requirements, not completed-test evidence.

## Original implementation sequence

**Goal:** Ship a first-class native mobile app (iOS + Android) for Helpin Support agents — triage, read, and reply to support conversations from a phone with push notifications — with UI/UX quality on par with the best-built consumer apps.

**Architecture:** A new self-contained Tauri 2 mobile app at `apps/support-mobile` with a purpose-built mobile-first React UI. All domain logic (auth/session, support queries, realtime, mutations) comes from `packages/support-core` — the same package the desktop app and web app already share. Zero UI reuse from `frontend/src` (desktop-shaped layouts would guarantee mediocrity on a phone); instead, a small mobile UI kit is built from scratch against the same design tokens. Push notifications are delivered via FCM (both platforms) with a new `push_devices` backend surface and a thin custom Tauri plugin.

**Tech Stack:** Tauri 2.9 (mobile targets), React 19, Vite 7, TypeScript 5.9, Tailwind CSS 4, TanStack Router + Query, Zustand, `motion` (Framer Motion v12) for transitions, `vaul` for sheets, `@tanstack/react-virtual` for lists, lucide-react icons, Inter Variable font. Backend: Go + Chi + GORM, `firebase.google.com/go/v4` for FCM.

## Global Constraints

- Tauri crate versions must match the desktop app: `tauri = "2.9.1"`, `tauri-build = "2.5.1"`, `@tauri-apps/cli ^2.8.4`, `@tauri-apps/api ^2.8.0`.
- React `19.2.5`, TypeScript `~5.9.3`, Vite `^7.3.1`, Tailwind `^4.2.1` — same as `apps/support-desktop`.
- App identifier: `ai.helpin.mobile`. Product name: `Helpin Support`. Deep-link scheme: `helpin://`.
- Dev server port: **5176** (desktop uses 5175).
- The mobile app must NOT alias `@` → `frontend/src`. Only `@helpin-ai/support-core` and `@helpin-ai/shared` are shared imports. `@mobile` aliases `apps/support-mobile/src`.
- All JSON API fields are `snake_case`; all backend Go code follows handler → service → repository layering; new tables via GORM AutoMigrate (register the model in `cmd/api/main.go`).
- Every interactive element ≥ 44×44 pt touch target. All screens support light + dark themes and `prefers-reduced-motion`.
- Do not log or persist tokens anywhere except the Tauri store session adapter.
- Branch: create `feat/support-mobile` off `develop`. Commit after every task at minimum.

---

## Part A — Product & Design Specification

This section is the quality bar. Every UI task below references it. Deviations require explicit sign-off, not silent judgment calls.

### A1. North star

**"Triage anywhere in under 10 seconds."** An agent gets a push notification, opens straight into the conversation, reads context at a glance, replies or reassigns, and is done. Every design decision optimizes time-to-first-response. This is an agent's *second screen* — it complements desktop, so V1 is deliberately support-only and conversation-centric.

### A2. Information architecture

```
Login → Workspace picker → Tab shell
  Tab 1: Inbox
    InboxScreen (conversation list)
      → ConversationScreen (thread + composer)
          → ContextSheet (customer/visitor info, assignment, tags, status)
  Tab 2: You (profile, workspace switch, theme, notifications, sign out)
```

Two tabs in V1. Search ships in V1.1 — never ship a dead tab. Deep link `helpin://w/{slug}/support/{conversationId}` (from push taps) lands directly on ConversationScreen with back → InboxScreen.

### A3. Screen-by-screen specification

**Login** — Full-bleed background using the brand radial gradient (same recipe as the desktop shell: `radial-gradient(circle_at_20%_20%, rgba(188,214,231,.75), rgba(245,248,251,.92) 45%, rgba(187,210,229,.55) 100%)`, dark-mode equivalent with deep navy stops). Logo centered in top third. Email + password fields 52px tall, 12px radius, floating labels. Primary button full-width 52px. Inline field-level error text (no toasts for validation). Button shows inline spinner while authenticating; the whole card never jumps. Keyboard never covers the focused input (scroll-into-view + `--keyboard-inset` padding).

**Workspace picker** — List of workspace rows (44px avatar with `getInitials` fallback, name, member role caption). Rows stagger-fade in (30ms increments, 250ms total cap). Single workspace → skip this screen entirely and go straight to Inbox.

**Inbox** — The heart of the app.
- Header: large title "Inbox" (28px/700) that collapses to a 44px compact bar on scroll (title shrinks to 17px/600 centered). Right side: avatar button → You tab shortcut is NOT here (it's a tab); instead a mailbox-switcher button. Tapping the title area opens the mailbox sheet (all mailboxes + unread counts per mailbox).
- Filter row under the header: segmented control `Mine · Unassigned · All`, mapping to `ConversationFilters.filter` values `my_inbox` / `unassigned` / `all` (verify exact strings against `SupportInboxLayout.tsx` usage of `filter` before wiring; the unread stats meta uses `my_inbox` and `unassigned` keys, so these are the expected values). Counts shown inline in each segment from `useUnreadStats`.
- Conversation cell (84px): 44px customer avatar with a 16px channel glyph badge (mail icon for `source=email`, chat bubble for `widget`) bottom-right; line 1: customer name (15px/600) + relative timestamp right-aligned (13px, tabular numerals, `text-muted-foreground`); line 2–3: last message preview, 2 lines max, ellipsized (14px, muted; 15px/500 + full foreground when unread); leading 8px unread dot (primary color) vertically centered, only when unread; state badges (Waiting, Resolved) as 11px pills only when not `open`.
- Swipe gestures on cells: swipe right reveals blue "Read/Unread" action; swipe left reveals green "Resolve". Committing past 96px triggers the action with a light impact haptic; the row springs back. Full-swipe (>60% width) auto-commits.
- Pull-to-refresh: custom rubber-band with the Helpin spinner; triggers at 70px pull with a haptic tick; refetches conversations + unread stats.
- Loading: 8 skeleton cells matching the exact cell geometry (never spinners for lists). Empty state: centered illustration-free composition — 48px icon in a soft circle, "Inbox zero" headline, one-line body, subtle confetti-free. Error state: retry button.
- Scroll: virtualized (`@tanstack/react-virtual`), 60fps budget, no layout thrash (fixed cell heights).

**Conversation** — 
- Header (compact 52px + safe-area): back chevron (44px target, also edge-swipe-back), customer name (17px/600) with presence dot (green when `visitor_online`), subtitle line showing status + assignee (13px muted). Tapping the header opens ContextSheet. Right: overflow button (also opens ContextSheet — one sheet, no menu maze).
- Thread: chronological, newest at bottom, auto-scrolled to bottom on open. Day separators ("Today", "Yesterday", else "Mon, Jul 6"). Messages grouped: consecutive same-sender messages within 3 minutes collapse into one cluster (avatar + name shown once). Customer messages left-aligned (surface bubble), agent/AI messages right-aligned (primary-tinted bubble), internal notes full-width amber-tinted cards labeled "Internal note". Email messages render as cards: subject line (semibold), sanitized HTML body (DOMPurify), quoted history collapsed behind a "•••" expander. AI messages get a small sparkle glyph next to the sender name.
- New-message pill: when scrolled up >300px and a new message arrives, a floating "↓ New message" pill appears above the composer; tapping scrolls to bottom.
- Typing indicator: three-dot bubble driven by the presence store's typing state.
- Mark-read on open (existing `useMarkConversationRead`).
- Composer: pinned to bottom, respects keyboard inset and safe area. Reply/Note segmented toggle (Note mode tints the composer amber and changes placeholder to "Internal note…"). Auto-growing textarea, 1–6 lines. Send button is a 36px circle that transitions disabled(30% opacity) → active(primary) → sending(spinner) → sent(checkmark, 400ms) — no layout shift. Optimistic append: message appears instantly with 70% opacity + 12px rise animation, solidifies on server ack; on failure it gets a red "Retry" chip and an error haptic.
- Perceived open time < 250ms: the cell's data (name, preview) pre-populates the header via query cache while messages load behind message skeletons.

**ContextSheet** (vaul bottom sheet, 2 detents: 55% and 92%) —
- Customer block: avatar, name, email (tap-to-copy), last-active line.
- Actions row: `Resolve`/`Reopen` (status), `Assign` (opens teammate picker list inline in the sheet), `Tags` (tag picker chips).
- Visitor context: browser/OS/location rows from `useVisitorContext`.
- Every action optimistic with rollback on error + toast.

**You** — Grouped list (iOS Settings-style): profile row (avatar, name, email); Workspace row (current workspace, tap → workspace picker); Appearance (Light/Dark/System segmented); Notifications (push toggle + link to OS settings when denied); Sign out (destructive red row, confirm dialog); footer: app version + build.

### A4. Design tokens

- **Color:** copy the exact oklch token block (light + dark `:root` variables `--background`, `--foreground`, `--primary`, `--muted`, `--destructive`, `--border`, etc.) from `frontend/src/index.css` so brand color is pixel-identical across surfaces. Mobile adds: `--surface-raised` (cards/bubbles), `--overlay-scrim` (40% black), `--unread-dot` (= primary).
- **Typography:** Inter Variable. Scale: 28/34 large title (700), 20/25 title (600), 17/22 headline (600), 15/20 body (400/500), 13/18 footnote (400), 11/13 caption (500, tracking +0.06em). Timestamps and counters use `font-variant-numeric: tabular-nums`.
- **Spacing:** 4px base grid. Screen gutter 16px. List row internal padding 12px vertical.
- **Radius:** 10px controls (matches web `0.625rem`), 16px cards/bubbles, 20px sheet top corners, full circle for send button/avatars.
- **Elevation:** borders-first (hairline `--border`), shadows only on floating elements (pill, sheet): `0 8px 24px rgba(0,0,0,.12)`.

### A5. Motion system

| Interaction | Spec |
|---|---|
| Stack push | Incoming screen translateX 100%→0, spring `{ stiffness: 380, damping: 38 }`; outgoing parallax to −25% with scrim fading to 12% |
| Stack pop / edge swipe | Reverse of push; gesture-driven, tracks finger 1:1, commits past 35% width or velocity > 0.5 px/ms |
| Sheet | vaul defaults (spring, drag-to-dismiss) |
| Press feedback | scale 0.97 + opacity 0.85, 100ms ease-out, on every Pressable |
| Tab switch | Crossfade 150ms; icon does a 1.0→0.85→1.0 scale tick |
| Optimistic message | translateY 12px→0 + opacity 0→1, 220ms ease-out |
| Skeleton | 1.4s shimmer loop |
| Pull-to-refresh | Rubber-band resistance ×0.5, spinner rotation tied to pull distance until armed |

`prefers-reduced-motion: reduce` → all slides/springs become 120ms crossfades; shimmer becomes static.

### A6. Haptics map

| Event | Haptic |
|---|---|
| Tab / segment change | `selection` |
| Swipe action armed (passes threshold) | `impactLight` |
| Pull-to-refresh armed | `impactLight` |
| Send success | `notificationSuccess` |
| Send failure / error toast | `notificationError` |
| Long-press (future) | `impactMedium` |

### A7. Performance budgets (hard gates for Task 22)

- Cold start → interactive inbox: **< 2.0s** on Pixel 6a, **< 1.5s** on iPhone 12.
- Initial JS bundle **< 900KB gzip** (no mermaid/excalidraw/tiptap — none of these are dependencies of this app).
- Inbox scroll: sustained 60fps with 500 conversations.
- Conversation open: header paint < 1 frame (cache-fed), messages < 800ms on LTE.

### A8. Accessibility bar

- WCAG AA contrast in both themes; verify muted-on-background combos.
- Text respects OS font scaling to 120% without clipping critical UI (allow preview truncation).
- VoiceOver/TalkBack: cells announce "«name», «preview», «time», unread"; send button announces state; sheet focus-traps.
- All hit targets ≥ 44pt (cells, tab items, header buttons, segmented controls).

---

## Part B — Architecture Decisions (ADRs)

1. **New app `apps/support-mobile`, not mobile targets on `apps/support-desktop`.** The desktop app aliases `@` → `frontend/src` and mounts the desktop `SupportInboxLayout`; its bundle carries web-app weight (mermaid, excalidraw chunks visible in its `dist/`). World-class mobile requires a mobile-first UI and an independent performance budget. The Rust shell is ~100 lines; duplicating it is cheap. Logic reuse happens where it should: `support-core`.
2. **All new domain logic goes into `packages/support-core`, not the app.** V1 needs three mutations support-core lacks (`sendMessage`, `updateConversationStatus`, `assignConversationUser`). Adding them to support-core benefits web + desktop too and keeps the mobile app UI-only.
3. **FCM for both platforms.** One backend integration (`firebase.google.com/go/v4`); iOS delivery goes APNs-via-FCM. Requires a Firebase project + APNs key upload (ops prerequisite, Task 17).
4. **Custom thin Tauri push plugin** (`tauri-plugin-helpin-push`): the Tauri ecosystem has no official push plugin; community ones are unmaintained. Ours exposes exactly three things: get token, token-refresh event, notification-tap event. The API surface is small, but the native integration (Firebase SDK setup, Gradle/Xcode wiring, APNs token handoff, cold-start payload delivery) is the highest-risk work in this plan — Task 19a spikes it on real hardware before Task 19 builds it properly.
5. **TanStack Router (code-based routes, like desktop) + a custom `ScreenStack`** for native-feeling transitions. No react-navigation equivalents exist for DOM; a ~150-line stack built on `motion` gives gesture-driven back with full control.
6. **Plain-text composer in V1** (no TipTap). Agents on mobile send short replies; TipTap costs ~300KB and keyboard-handling complexity. Canned responses/AI rewrite are V1.1.
7. **Copy, don't share, the tiny host services** (`authService.ts`, `workspacesService.ts`, `sessionStorage.ts` — each < 40 lines, already duplicated in desktop by that team's deliberate call to defer extraction). Consolidation into support-core is follow-up work after mobile ships, tracked outside this plan.

## Part C — Phase map

| Phase | Tasks | Deliverable |
|---|---|---|
| 1. Host bootstrap | 1–2 | App runs on Android emulator + iOS simulator showing themed hello screen |
| 2. Design system | 3–5 | Tokens, primitives, haptics/motion utilities — all unit-tested |
| 3. Navigation shell | 6–7 | Stack + tabs with gesture back, deep-linkable routes |
| 4. Auth & workspace | 8–9 | Real login → workspace pick → empty inbox, session survives restart |
| 5. Inbox | 10–12 | Production inbox: virtualized, filters, swipe actions, pull-to-refresh |
| 6. Thread | 13–15 | Full conversation read/reply/note/status/assign experience |
| 7. Realtime & lifecycle | 16 | Live updates, resume-from-background reconnect, offline banner |
| 8. Push | 17, 18, 19a (spike), 19, 20 | Backend devices + FCM sender, hardware spike, plugin, permission priming, tap-through |
| 9. Distribution & hardening | 21–22 | Deep links, icons/splash, CI, perf/a11y gates, QA matrix |

Phases 5, 6, and 8 are independently shippable checkpoints: after Phase 6 the app is dogfoodable via TestFlight/internal track without push.

---

## Part D — Tasks

### Task 1: Scaffold `apps/support-mobile` (Vite + TS + Tailwind + Vitest)

**Files:**
- Create: `apps/support-mobile/package.json`
- Create: `apps/support-mobile/vite.config.ts`
- Create: `apps/support-mobile/index.html`
- Create: `apps/support-mobile/tsconfig.json`, `apps/support-mobile/tsconfig.app.json`, `apps/support-mobile/tsconfig.node.json`
- Create: `apps/support-mobile/src/main.tsx`, `apps/support-mobile/src/index.css`, `apps/support-mobile/src/vite-env.d.ts`
- Create: `apps/support-mobile/vitest.config.ts`, `apps/support-mobile/src/__tests__/smoke.test.tsx`
- Modify: `pnpm-workspace.yaml` — confirm `apps/*` is included (desktop already lives there; if the globs only list `packages/*`, add `apps/*`).

**Interfaces:**
- Produces: the app package `support-mobile` with scripts `dev`, `build`, `typecheck`, `test`, `tauri`; alias `@mobile/*` → `src/*`.

- [ ] **Step 1: Create branch**

```bash
# From the repository root
git checkout develop && git pull origin develop && git checkout -b feat/support-mobile
```

- [ ] **Step 2: Write package.json**

```json
{
  "name": "support-mobile",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0 --port 5176",
    "build": "NODE_OPTIONS=--max-old-space-size=4096 tsc -b && NODE_OPTIONS=--max-old-space-size=4096 vite build",
    "preview": "vite preview",
    "typecheck": "tsc -b --noEmit",
    "test": "vitest run",
    "test:watch": "vitest",
    "tauri": "tauri"
  },
  "dependencies": {
    "@fontsource-variable/inter": "^5.2.8",
    "@helpin-ai/shared": "workspace:*",
    "@helpin-ai/support-core": "workspace:*",
    "@tanstack/react-query": "^5.99.0",
    "@tanstack/react-router": "^1.168.22",
    "@tanstack/react-virtual": "^3.13.23",
    "@tauri-apps/api": "^2.8.0",
    "@tauri-apps/plugin-notification": "^2",
    "@tauri-apps/plugin-store": "^2.4.0",
    "clsx": "^2.1.1",
    "dompurify": "^3.4.0",
    "lucide-react": "^0.575.0",
    "motion": "^12.0.0",
    "next-themes": "^0.4.6",
    "react": "19.2.5",
    "react-dom": "19.2.5",
    "sonner": "^2.0.7",
    "tailwind-merge": "^3.5.0",
    "vaul": "^1.1.2",
    "zustand": "^5.0.11"
  },
  "devDependencies": {
    "@tailwindcss/vite": "^4.2.1",
    "@tauri-apps/cli": "^2.8.4",
    "@testing-library/react": "^16.1.0",
    "@types/node": "^24.10.1",
    "@types/react": "^19.2.7",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^5.1.1",
    "jsdom": "^25.0.1",
    "tailwindcss": "^4.2.1",
    "typescript": "~5.9.3",
    "vite": "^7.3.1",
    "vitest": "^3.0.0"
  }
}
```

- [ ] **Step 3: Write vite.config.ts**

```ts
import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const host = process.env.TAURI_DEV_HOST

export default defineConfig({
  clearScreen: false,
  plugins: [react(), tailwindcss()],
  resolve: {
    dedupe: ['react', 'react-dom', '@tanstack/react-query', 'zustand'],
    alias: [
      { find: '@mobile', replacement: path.resolve(__dirname, './src') },
      {
        find: '@helpin-ai/support-core',
        replacement: path.resolve(__dirname, '../../packages/support-core/src/index.ts'),
      },
      {
        find: '@helpin-ai/shared',
        replacement: path.resolve(__dirname, '../../packages/shared/src/index.ts'),
      },
    ],
  },
  server: {
    port: 5176,
    strictPort: true,
    host: host || '0.0.0.0',
    hmr: host ? { protocol: 'ws', host, port: 5177 } : undefined,
    watch: { ignored: ['**/src-tauri/**'] },
  },
})
```

- [ ] **Step 4: Write index.html** (viewport-fit is load-bearing for safe areas)

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover" />
    <meta name="theme-color" content="#ffffff" media="(prefers-color-scheme: light)" />
    <meta name="theme-color" content="#0a0a0a" media="(prefers-color-scheme: dark)" />
    <title>Helpin Support</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 5: Write tsconfig files** (copy the three tsconfig files from `apps/support-desktop/` and change the `paths` mapping from `@desktop/*` to `@mobile/*` pointing at `./src/*`; remove the `@/*` → `frontend/src` mapping entirely — this app must not compile against `frontend/src`).

- [ ] **Step 6: Minimal boot + smoke test**

`src/main.tsx`:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'

export function App() {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-background text-foreground">
      <h1 className="text-xl font-semibold">Helpin Support</h1>
    </div>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

`src/index.css`:

```css
@import 'tailwindcss';
@import '@fontsource-variable/inter';
```

`vitest.config.ts`:

```ts
import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config'

export default mergeConfig(viteConfig, defineConfig({
  test: { environment: 'jsdom', globals: true },
}))
```

`src/__tests__/smoke.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { App } from '../main'

test('renders app shell', () => {
  render(<App />)
  expect(screen.getByText('Helpin Support')).toBeDefined()
})
```

Note: `main.tsx` calls `createRoot` at module scope; for the test to import `App` without mounting, guard the mount: `const rootEl = document.getElementById('root'); if (rootEl) createRoot(rootEl).render(...)` — jsdom test files have no `#root`.

- [ ] **Step 7: Install, verify test fails then passes, typecheck**

```bash
# From the repository root
pnpm install
cd apps/support-mobile && pnpm test    # expect: 1 passed
pnpm typecheck                          # expect: exit 0
```

- [ ] **Step 8: Commit**

```bash
git add apps/support-mobile pnpm-workspace.yaml pnpm-lock.yaml
git commit -m "feat(mobile): scaffold support-mobile app package"
```

---

### Task 2: Tauri mobile shell (Android + iOS targets)

**Files:**
- Create: `apps/support-mobile/src-tauri/Cargo.toml`, `src-tauri/tauri.conf.json`, `src-tauri/build.rs`, `src-tauri/src/main.rs`, `src-tauri/src/lib.rs`, `src-tauri/capabilities/default.json`
- Create (generated): `src-tauri/gen/android/**`, `src-tauri/gen/apple/**` via `tauri android init` / `tauri ios init`
- Create: `apps/support-mobile/README.md` — device/emulator dev-loop instructions

**Interfaces:**
- Produces: `pnpm tauri android dev` and `pnpm tauri ios dev` run the app; Rust command `mobile_shell_info()` returns `{ runtime, platform, app_version }` (mirrors desktop's `desktop_shell_info`).

- [ ] **Step 1: Write src-tauri/Cargo.toml**

```toml
[package]
name = "helpin-support-mobile"
version = "0.1.0"
description = "Helpin Support Mobile"
authors = ["Helpin"]
edition = "2021"

[lib]
name = "helpin_support_mobile_lib"
crate-type = ["staticlib", "cdylib", "rlib"]

[build-dependencies]
tauri-build = { version = "2.5.1", features = [] }

[dependencies]
serde = { version = "1", features = ["derive"] }
serde_json = "1"
tauri = { version = "2.9.1", features = [] }
tauri-plugin-notification = "2"
tauri-plugin-store = "2.4.0"
```

`build.rs`:

```rust
fn main() {
    tauri_build::build()
}
```

- [ ] **Step 2: Write tauri.conf.json**

```json
{
  "$schema": "https://schema.tauri.app/config/2",
  "productName": "Helpin Support",
  "version": "0.1.0",
  "identifier": "ai.helpin.mobile",
  "build": {
    "beforeDevCommand": "pnpm dev",
    "beforeBuildCommand": "pnpm build",
    "devUrl": "http://localhost:5176",
    "frontendDist": "../dist"
  },
  "app": {
    "windows": [{ "label": "main", "title": "Helpin Support" }],
    "security": { "csp": null }
  },
  "bundle": {
    "active": true,
    "targets": "all",
    "iOS": { "minimumSystemVersion": "15.0" },
    "android": { "minSdkVersion": 26 }
  }
}
```

- [ ] **Step 3: Write src/lib.rs and src/main.rs**

`src/lib.rs`:

```rust
use serde::Serialize;

#[derive(Serialize)]
struct MobileShellInfo {
    runtime: &'static str,
    platform: &'static str,
    app_version: String,
}

#[tauri::command]
fn mobile_shell_info(app: tauri::AppHandle) -> MobileShellInfo {
    MobileShellInfo {
        runtime: "tauri",
        platform: std::env::consts::OS,
        app_version: app.package_info().version.to_string(),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_store::Builder::default().build())
        .invoke_handler(tauri::generate_handler![mobile_shell_info])
        .run(tauri::generate_context!())
        .expect("error while running helpin support mobile");
}
```

`src/main.rs`:

```rust
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    helpin_support_mobile_lib::run();
}
```

`capabilities/default.json`:

```json
{
  "$schema": "../gen/schemas/mobile-schema.json",
  "identifier": "default",
  "description": "Default capability for the Helpin Support mobile webview.",
  "windows": ["main"],
  "permissions": [
    "core:default",
    "notification:default",
    "notification:allow-is-permission-granted",
    "notification:allow-request-permission",
    "notification:allow-notify",
    "store:default"
  ]
}
```

- [ ] **Step 4: Initialize mobile targets**

```bash
cd apps/support-mobile
pnpm tauri android init     # requires ANDROID_HOME + NDK; generates src-tauri/gen/android
pnpm tauri ios init         # macOS only; generates src-tauri/gen/apple
```

Environment prerequisites (document in the README written this step): Rust with `aarch64-linux-android`/`aarch64-apple-ios` targets, Android Studio + SDK/NDK, Xcode 15+. iOS steps run on a macOS machine; Android everywhere. If this execution environment lacks the SDKs, mark the on-device steps as handoff items in the PR description rather than skipping the config work.

- [ ] **Step 5: Run on emulator/simulator and verify**

```bash
pnpm tauri android dev   # expect: app boots on emulator showing "Helpin Support" centered
pnpm tauri ios dev       # expect: same on iOS simulator
```

Verify: text is visible in both light and dark OS themes; no white flash on boot; status bar area is not overlapped incorrectly (safe-area handling arrives in Task 3).

- [ ] **Step 6: Commit**

```bash
git add apps/support-mobile/src-tauri apps/support-mobile/README.md
git commit -m "feat(mobile): tauri 2 shell with android/ios targets"
```

---

### Task 3: Design tokens, theme, and safe-area foundation

**Files:**
- Modify: `apps/support-mobile/src/index.css`
- Create: `apps/support-mobile/src/ui/theme-provider.tsx`
- Create: `apps/support-mobile/src/lib/cn.ts`
- Test: `apps/support-mobile/src/ui/__tests__/theme.test.tsx`

**Interfaces:**
- Produces: CSS custom properties identical to web (`--background`, `--foreground`, `--primary`, `--muted`, `--muted-foreground`, `--border`, `--destructive`, `--card`, etc.) in light+dark; safe-area vars `--safe-top/right/bottom/left`; `--keyboard-inset` (0px default); type utility classes `text-large-title`, `text-headline`, `text-body`, `text-footnote`, `text-caption`; `cn()` helper; `<AppThemeProvider>` wrapping next-themes with `attribute="class"`.

- [ ] **Step 1: Port the token block.** Open `frontend/src/index.css`, copy the full `:root { … }` and `.dark { … }` oklch variable blocks verbatim into `src/index.css`, then register them with Tailwind v4 the same way frontend does (copy frontend's `@theme inline` mapping block too). This is a copy task — do not re-derive colors.

- [ ] **Step 2: Add the mobile layer on top** (append to `src/index.css`):

```css
:root {
  --safe-top: env(safe-area-inset-top, 0px);
  --safe-right: env(safe-area-inset-right, 0px);
  --safe-bottom: env(safe-area-inset-bottom, 0px);
  --safe-left: env(safe-area-inset-left, 0px);
  --keyboard-inset: 0px;
  --surface-raised: var(--card);
  --overlay-scrim: rgb(0 0 0 / 0.4);
}

html, body, #root { height: 100%; }

body {
  font-family: 'Inter Variable', system-ui, sans-serif;
  background: var(--background);
  color: var(--foreground);
  overscroll-behavior: none;               /* the app owns pull-to-refresh */
  -webkit-tap-highlight-color: transparent;
  -webkit-touch-callout: none;
  user-select: none;                        /* re-enabled per message body */
  -webkit-user-select: none;
}

.selectable { user-select: text; -webkit-user-select: text; }

@utility text-large-title { font-size: 1.75rem; line-height: 2.125rem; font-weight: 700; letter-spacing: -0.02em; }
@utility text-title { font-size: 1.25rem; line-height: 1.5625rem; font-weight: 600; letter-spacing: -0.01em; }
@utility text-headline { font-size: 1.0625rem; line-height: 1.375rem; font-weight: 600; }
@utility text-body { font-size: 0.9375rem; line-height: 1.25rem; }
@utility text-footnote { font-size: 0.8125rem; line-height: 1.125rem; }
@utility text-caption { font-size: 0.6875rem; line-height: 0.8125rem; font-weight: 500; letter-spacing: 0.06em; }
@utility tnum { font-variant-numeric: tabular-nums; }

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation-duration: 0.12s !important; transition-duration: 0.12s !important; }
}
```

- [ ] **Step 3: `src/lib/cn.ts`**

```ts
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

- [ ] **Step 4: Theme provider** — `src/ui/theme-provider.tsx`:

```tsx
import { ThemeProvider } from 'next-themes'
import type { ReactNode } from 'react'

export function AppThemeProvider({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      {children}
    </ThemeProvider>
  )
}
```

- [ ] **Step 5: Test + verify on device.** Test asserts the provider renders children and toggling `document.documentElement.classList` between `''`/`dark` is what next-themes drives (smoke-level). On the emulator: flip OS dark mode; the app background/text flips with no flash.

- [ ] **Step 6: Commit** — `git commit -m "feat(mobile): design tokens, theme, safe-area foundation"`

---

### Task 4: UI primitives (Pressable, Avatar, Badge, Skeleton, SegmentedControl, Sheet, TopBar, EmptyState)

**Files:**
- Create: `apps/support-mobile/src/ui/pressable.tsx`, `avatar.tsx`, `badge.tsx`, `skeleton.tsx`, `segmented-control.tsx`, `sheet.tsx`, `top-bar.tsx`, `empty-state.tsx`, `spinner.tsx`
- Test: `apps/support-mobile/src/ui/__tests__/segmented-control.test.tsx`, `__tests__/pressable.test.tsx`

**Interfaces:**
- Produces:
  - `Pressable({ onPress, haptic?, className, children, disabled? })` — button with press-scale feedback, min 44px targets
  - `Avatar({ name, src?, size?: number })` — image with `getInitials(name)` fallback (implement `getInitials` locally in `avatar.tsx`: first letters of first two words, uppercased)
  - `Badge({ tone: 'neutral' | 'primary' | 'warning' | 'success' | 'destructive', children })` — 11px caption pill
  - `Skeleton({ className })` — shimmer block
  - `SegmentedControl<T extends string>({ segments: { value: T; label: string; count?: number }[], value: T, onChange: (v: T) => void })` — animated thumb, selection haptic
  - `Sheet({ open, onOpenChange, children, detents? })` — vaul wrapper with 20px top radius, grabber, safe-bottom padding
  - `TopBar({ title, subtitle?, onBack?, trailing?, large? })` — 52px bar + safe-top padding; `large` renders the collapsing large-title variant (scroll-linked via a `scrollY` motion value prop)
  - `EmptyState({ icon, title, body, action? })`

- [ ] **Step 1: Write failing tests** for SegmentedControl (renders segments, fires `onChange` with the tapped value, marks selected via `aria-pressed`) and Pressable (fires `onPress`, does not fire when `disabled`, has `min-h-[44px]`).

```tsx
// __tests__/segmented-control.test.tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { SegmentedControl } from '../segmented-control'

test('fires onChange with tapped segment value', () => {
  const onChange = vi.fn()
  render(
    <SegmentedControl
      segments={[{ value: 'mine', label: 'Mine' }, { value: 'all', label: 'All' }]}
      value="mine"
      onChange={onChange}
    />,
  )
  fireEvent.click(screen.getByRole('button', { name: /All/ }))
  expect(onChange).toHaveBeenCalledWith('all')
})
```

- [ ] **Step 2: Run tests, expect FAIL** (`pnpm test` → module not found).

- [ ] **Step 3: Implement the primitives.** Pressable (the pattern every other primitive builds on):

```tsx
// src/ui/pressable.tsx
import { motion } from 'motion/react'
import type { ReactNode } from 'react'
import { cn } from '@mobile/lib/cn'
import { haptic, type HapticKind } from '@mobile/lib/haptics'

interface PressableProps {
  onPress?: () => void
  haptic?: HapticKind
  disabled?: boolean
  className?: string
  children: ReactNode
  'aria-label'?: string
}

export function Pressable({ onPress, haptic: hapticKind, disabled, className, children, ...rest }: PressableProps) {
  return (
    <motion.button
      type="button"
      disabled={disabled}
      whileTap={disabled ? undefined : { scale: 0.97, opacity: 0.85 }}
      transition={{ duration: 0.1 }}
      className={cn('min-h-[44px] min-w-[44px] touch-manipulation disabled:opacity-40', className)}
      onClick={() => {
        if (disabled) return
        if (hapticKind) haptic(hapticKind)
        onPress?.()
      }}
      {...rest}
    >
      {children}
    </motion.button>
  )
}
```

SegmentedControl uses a `motion.div` thumb with `layoutId="segment-thumb"` sliding between segments; each segment is a Pressable with `haptic="selection"`, counts rendered `tnum text-footnote`. Sheet wraps `vaul`'s `Drawer.Root/Portal/Content` with `bg-background rounded-t-[20px] pb-[max(var(--safe-bottom),16px)]` and a 36×5px grabber. TopBar renders `pt-[var(--safe-top)]`, a 52px row, back chevron as Pressable when `onBack` is set; the `large` variant renders a 28px title below the bar that scales/fades into the compact title driven by the passed `scrollY` motion value (`useTransform(scrollY, [0, 44], [1, 0])`). Skeleton is a `bg-muted` block with a CSS shimmer overlay (1.4s loop). Note: `haptics.ts` arrives in Task 5 — create a stub `export type HapticKind = 'selection'|'impactLight'|'impactMedium'|'notificationSuccess'|'notificationError'; export function haptic(_k: HapticKind) {}` in this task so Pressable compiles; Task 5 fills it in.

- [ ] **Step 4: Run tests, expect PASS.** `pnpm test && pnpm typecheck`

- [ ] **Step 5: Commit** — `git commit -m "feat(mobile): core ui primitives"`

---

### Task 5: Haptics + motion presets

**Files:**
- Modify: `apps/support-mobile/src/lib/haptics.ts` (replace Task-4 stub)
- Create: `apps/support-mobile/src/lib/motion.ts`
- Modify: `apps/support-mobile/src-tauri/Cargo.toml`, `src-tauri/src/lib.rs`, `src-tauri/capabilities/default.json`, `package.json` — add the official `tauri-plugin-haptics`
- Test: `apps/support-mobile/src/lib/__tests__/haptics.test.ts`

**Interfaces:**
- Produces: `haptic(kind: HapticKind): void` — fire-and-forget, no-ops silently off-device; `stackSpring = { type: 'spring', stiffness: 380, damping: 38 }`, `pressTransition = { duration: 0.1 }`, `riseIn = { initial: { opacity: 0, y: 12 }, animate: { opacity: 1, y: 0 }, transition: { duration: 0.22, ease: 'easeOut' } }` exported from `motion.ts`.

- [ ] **Step 1: Add the plugin.** `pnpm add @tauri-apps/plugin-haptics`; in Cargo.toml add `tauri-plugin-haptics = "2"`; in `lib.rs` add `.plugin(tauri_plugin_haptics::init())`; in capabilities add `"haptics:default"`.

- [ ] **Step 2: Failing test** — mock `@tauri-apps/plugin-haptics`, assert `haptic('selection')` calls `selectionFeedback()` and that errors are swallowed (calling `haptic` when the plugin throws must not throw).

- [ ] **Step 3: Implement `haptics.ts`**

```ts
import {
  impactFeedback,
  notificationFeedback,
  selectionFeedback,
} from '@tauri-apps/plugin-haptics'

export type HapticKind =
  | 'selection'
  | 'impactLight'
  | 'impactMedium'
  | 'notificationSuccess'
  | 'notificationError'

const isTauri = () => typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window

export function haptic(kind: HapticKind): void {
  if (!isTauri()) return
  const fire = async () => {
    switch (kind) {
      case 'selection': return selectionFeedback()
      case 'impactLight': return impactFeedback('light')
      case 'impactMedium': return impactFeedback('medium')
      case 'notificationSuccess': return notificationFeedback('success')
      case 'notificationError': return notificationFeedback('error')
    }
  }
  void fire().catch(() => {})
}
```

- [ ] **Step 4: Tests pass, on-device spot check** (tap feedback felt on a physical phone; simulators don't vibrate — verify no crashes there).

- [ ] **Step 5: Commit** — `git commit -m "feat(mobile): haptics and motion presets"`

---

### Task 6: Router + ScreenStack with gesture-driven back

**Files:**
- Create: `apps/support-mobile/src/router.tsx`
- Create: `apps/support-mobile/src/navigation/screen-stack.tsx`
- Create: `apps/support-mobile/src/navigation/use-edge-swipe-back.ts`
- Create: placeholder screens `apps/support-mobile/src/screens/login-screen.tsx`, `workspaces-screen.tsx`, `inbox-screen.tsx`, `conversation-screen.tsx`, `you-screen.tsx` (each renders a TopBar + name; replaced by later tasks)
- Test: `apps/support-mobile/src/navigation/__tests__/screen-stack.test.tsx`

**Interfaces:**
- Consumes: `TopBar`, `stackSpring` from Tasks 4–5.
- Produces:
  - Routes: `/login`, `/workspaces`, `/w/$slug/support` (inbox), `/w/$slug/support/$conversationId` (thread), `/w/$slug/you`
  - `RouterContext { auth: { user: User | null; loading: boolean; serverUnreachable: boolean } }` — same shape as desktop's `router.tsx`
  - `<ScreenStack>` component: renders the current route match, animates push/pop, exposes edge-swipe back
  - `navDirection(): 'push' | 'pop' | 'replace'` — module-level tracker

- [ ] **Step 1: Failing test** — direction tracker: simulate history index increasing → `'push'`, decreasing → `'pop'`. Extract the pure function so it is testable:

```ts
// screen-stack.tsx exports this pure helper
export function resolveDirection(prevIndex: number, nextIndex: number): 'push' | 'pop' | 'replace' {
  if (nextIndex > prevIndex) return 'push'
  if (nextIndex < prevIndex) return 'pop'
  return 'replace'
}
```

```tsx
// __tests__/screen-stack.test.tsx
import { backFallbackPath, resolveDirection } from '../screen-stack'

test('history index changes map to stack direction', () => {
  expect(resolveDirection(0, 1)).toBe('push')
  expect(resolveDirection(2, 1)).toBe('pop')
  expect(resolveDirection(1, 1)).toBe('replace')
})

test('back with no history falls back to inbox or workspaces', () => {
  expect(backFallbackPath('/w/acme/support/conv-1')).toBe('/w/acme/support')
  expect(backFallbackPath('/login')).toBe('/workspaces')
})
```

- [ ] **Step 2: Implement the route tree** in `router.tsx` following the desktop pattern (code-based `createRoute` calls, `createRootRouteWithContext<RouterContext>()`, `requireAuth` beforeLoad that redirects to `/login`). Root component:

```tsx
function RootComponent() {
  return (
    <AppThemeProvider>
      <ScreenStack />
      {/* TabBar rendered inside ScreenStack chrome for tab-level routes only */}
    </AppThemeProvider>
  )
}
```

- [ ] **Step 3: Implement ScreenStack.** Core structure (complete file, ~140 lines):

```tsx
// src/navigation/screen-stack.tsx
import { useEffect, useRef, useState } from 'react'
import { Outlet, useRouter, useRouterState } from '@tanstack/react-router'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { stackSpring } from '@mobile/lib/motion'
import { useEdgeSwipeBack } from './use-edge-swipe-back'

export function resolveDirection(prevIndex: number, nextIndex: number): 'push' | 'pop' | 'replace' {
  if (nextIndex > prevIndex) return 'push'
  if (nextIndex < prevIndex) return 'pop'
  return 'replace'
}

/** Where "back" goes when there is no prior history entry (cold-start deep link). */
export function backFallbackPath(pathname: string): string {
  const match = pathname.match(/^\/w\/([^/]+)\//)
  return match ? `/w/${match[1]}/support` : '/workspaces'
}

/** Tab-level roots crossfade instead of sliding. */
const TAB_ROOTS = [/^\/w\/[^/]+\/support$/, /^\/w\/[^/]+\/you$/]
const isTabRoot = (path: string) => TAB_ROOTS.some((re) => re.test(path))

export function ScreenStack() {
  const router = useRouter()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const historyIndex = useRouterState({
    select: (s) => (s.location.state as { __TSR_index?: number }).__TSR_index ?? 0,
  })
  const prevIndexRef = useRef(historyIndex)
  const [direction, setDirection] = useState<'push' | 'pop' | 'replace'>('replace')

  useEffect(() => {
    setDirection(resolveDirection(prevIndexRef.current, historyIndex))
    prevIndexRef.current = historyIndex
  }, [historyIndex])

  const reduced = useReducedMotion()
  const crossfade = reduced || (isTabRoot(pathname) && direction !== 'pop')

  const swipeEnabled = !isTabRoot(pathname) && pathname !== '/login' && pathname !== '/workspaces'
  const goBack = () => {
    if (router.history.canGoBack()) router.history.back()
    else router.navigate({ to: backFallbackPath(pathname) })
  }
  const { swipeRef, gestureX } = useEdgeSwipeBack({ enabled: swipeEnabled, onBack: goBack })

  const variants = crossfade
    ? {
        initial: { opacity: 0 },
        animate: { opacity: 1 },
        exit: { opacity: 0 },
      }
    : {
        initial: { x: direction === 'pop' ? '-25%' : '100%' },
        animate: { x: 0 },
        exit: { x: direction === 'pop' ? '100%' : '-25%' },
      }

  return (
    <div className="relative h-dvh overflow-hidden bg-background">
      <AnimatePresence initial={false} mode="popLayout">
        {/* Outer div: Motion owns its transform (push/pop variants).
            Inner div: the gesture owns its transform (a plain motion value).
            Never let both write to the same element. */}
        <motion.div
          key={pathname}
          className="absolute inset-0 bg-background"
          variants={variants}
          initial="initial"
          animate="animate"
          exit="exit"
          transition={crossfade ? { duration: 0.15 } : stackSpring}
        >
          <motion.div ref={swipeRef} style={{ x: gestureX }} className="h-full">
            <Outlet />
          </motion.div>
        </motion.div>
      </AnimatePresence>
    </div>
  )
}
```

- [ ] **Step 4: Implement `use-edge-swipe-back.ts`** (complete). The gesture drives a Motion value on a dedicated inner element (the outer element's transform belongs to the variant animation — never write to both), uses a non-passive `touchmove` so a horizontal intent lock can `preventDefault()` vertical scroll, and animates via Motion's `animate()` instead of raw style writes:

```ts
import { useEffect, useRef } from 'react'
import { animate, useMotionValue } from 'motion/react'
import { stackSpring } from '@mobile/lib/motion'

const EDGE_PX = 28
const INTENT_LOCK_PX = 10
const COMMIT_RATIO = 0.35
const COMMIT_VELOCITY = 0.5 // px per ms

export function useEdgeSwipeBack({ enabled, onBack }: { enabled: boolean; onBack: () => void }) {
  const swipeRef = useRef<HTMLDivElement | null>(null)
  const gestureX = useMotionValue(0)

  useEffect(() => {
    const el = swipeRef.current
    if (!el || !enabled) return

    let startX = 0
    let startY = 0
    let startT = 0
    let tracking = false
    let locked: 'horizontal' | 'vertical' | null = null

    const onStart = (e: TouchEvent) => {
      const t = e.touches[0]
      if (t.clientX > EDGE_PX) return
      tracking = true
      locked = null
      startX = t.clientX
      startY = t.clientY
      startT = performance.now()
    }
    const onMove = (e: TouchEvent) => {
      if (!tracking) return
      const t = e.touches[0]
      const dx = t.clientX - startX
      const dy = t.clientY - startY
      if (!locked && (Math.abs(dx) > INTENT_LOCK_PX || Math.abs(dy) > INTENT_LOCK_PX)) {
        locked = Math.abs(dx) > Math.abs(dy) ? 'horizontal' : 'vertical'
      }
      if (locked !== 'horizontal') return
      e.preventDefault() // needs { passive: false } — stops vertical scroll fighting the gesture
      gestureX.set(Math.max(0, dx))
    }
    const onEnd = (e: TouchEvent) => {
      if (!tracking) return
      tracking = false
      if (locked !== 'horizontal') return
      const dx = Math.max(0, e.changedTouches[0].clientX - startX)
      const dt = Math.max(1, performance.now() - startT)
      const commit = dx > el.clientWidth * COMMIT_RATIO || dx / dt > COMMIT_VELOCITY
      if (commit) {
        animate(gestureX, el.clientWidth, { duration: 0.18, ease: 'easeOut' }).then(() => {
          onBack()
          gestureX.set(0)
        })
      } else {
        animate(gestureX, 0, stackSpring)
      }
    }

    el.addEventListener('touchstart', onStart, { passive: true })
    el.addEventListener('touchmove', onMove, { passive: false })
    el.addEventListener('touchend', onEnd, { passive: true })
    el.addEventListener('touchcancel', onEnd, { passive: true })
    return () => {
      el.removeEventListener('touchstart', onStart)
      el.removeEventListener('touchmove', onMove)
      el.removeEventListener('touchend', onEnd)
      el.removeEventListener('touchcancel', onEnd)
    }
  }, [enabled, onBack, gestureX])

  return { swipeRef, gestureX }
}
```

- [ ] **Step 5: Wire `main.tsx`** to `RouterProvider` + `QueryClientProvider` (create `src/lib/queryClient.ts` mirroring desktop: `staleTime: 30_000`, `retry: 1`). Placeholder screens render `<TopBar title="…" />`.

- [ ] **Step 6: Verify.** `pnpm test && pnpm typecheck`; then on emulator: navigate placeholder Inbox → Conversation (temporary Link), confirm slide-in; edge-swipe from left returns; enable OS reduced-motion → crossfades. If `__TSR_index` is absent in this router version, fall back to tracking `router.history.length` semantics — confirm against `node_modules/@tanstack/history` types and adjust the selector, keeping `resolveDirection` unchanged. Likewise confirm `router.history.canGoBack()` exists in the installed `@tanstack/history`; if not, maintain a depth counter next to the direction tracker. Also verify the cold-start case explicitly: launch the app directly on a conversation URL (deep-link, Task 21 makes this routine) and confirm back lands on the inbox, not a dead end.

- [ ] **Step 7: Commit** — `git commit -m "feat(mobile): screen stack navigation with gesture back"`

---

### Task 7: Tab bar

**Files:**
- Create: `apps/support-mobile/src/navigation/tab-bar.tsx`
- Modify: `apps/support-mobile/src/screens/inbox-screen.tsx`, `you-screen.tsx` — render inside a shared `TabShell` layout that includes the TabBar
- Test: `apps/support-mobile/src/navigation/__tests__/tab-bar.test.tsx`

**Interfaces:**
- Consumes: `useUnreadStats` (support-core), `Pressable`, router `Link`.
- Produces: `<TabBar activeTab: 'inbox' | 'you'>` — 49px + safe-bottom bar; Inbox tab shows unread badge (count from `unread.meta` total, "99+" cap).

- [ ] **Step 1: Failing test** — badge formatting helper `formatBadgeCount(n)`: `0 → null`, `5 → "5"`, `120 → "99+"`.
- [ ] **Step 2: Implement** — fixed bottom bar `pb-[var(--safe-bottom)]`, two tab items (Inbox = `Inbox` lucide icon, You = `CircleUser`), active tint primary, inactive `text-muted-foreground`, selection haptic, icon scale-tick on switch (`whileTap` + `animate` on active change). Badge: absolute-positioned 16px pill, `bg-destructive text-white text-caption tnum`.
- [ ] **Step 3: Tests pass; verify on emulator** (badge renders with mock count; safe-area respected on notched device).
- [ ] **Step 4: Commit** — `git commit -m "feat(mobile): tab bar with unread badge"`

---

### Task 8: API client, session storage, auth store, boot sequence

**Files:**
- Create: `apps/support-mobile/src/lib/api.ts` — copy `apps/support-desktop/src/lib/api.ts` verbatim, then change the `onUnauthorized` handler to `router.navigate({ to: '/login' })` (import lazily to avoid a cycle: `import('@mobile/router').then(m => m.router.navigate({ to: '/login' }))`).
- Create: `apps/support-mobile/src/lib/session-storage.ts` — copy `apps/support-desktop/src/lib/sessionStorage.ts` verbatim (it is host-agnostic Tauri store code).
- Create: `apps/support-mobile/src/lib/types.ts` — copy the `User` and `AuthResponse` interfaces from `frontend/src/lib/types.ts` (only those two), plus `Workspace` fields used by the picker (`id`, `name`, `slug`, `logo_url`).
- Create: `apps/support-mobile/src/lib/services/auth-service.ts`, `workspaces-service.ts` — copy from `apps/support-desktop/src/lib/services/` (authService is 8 lines: `signin`, `me`), fixing the `@/lib` imports to `@mobile/lib`.
- Create: `apps/support-mobile/src/stores/auth-store.ts`
- Modify: `apps/support-mobile/src/main.tsx` — boot sequence
- Test: `apps/support-mobile/src/stores/__tests__/auth-store.test.ts`

**Interfaces:**
- Consumes: `configureSessionStorage`, `hydrateSessionStorage`, `getAccessToken`, `getRefreshToken`, `clearSession` from support-core; `createTauriSessionStorage` local.
- Produces: `useAuthStore` zustand store `{ user: User | null, loading: boolean, serverUnreachable: boolean }`; `bootstrapAuth(): Promise<void>`; `signOut(): Promise<void>`.

- [ ] **Step 1: Failing tests** for `bootstrapAuth` with a mocked `auth-service`: (a) no stored tokens → `user: null, loading: false`, `me()` never called; (b) tokens present + `me()` succeeds → user set; (c) `me()` network-errors → `serverUnreachable: true` and user kept null without clearing the session.

- [ ] **Step 2: Implement `auth-store.ts`**

```ts
import { create } from 'zustand'
import {
  clearSession,
  getAccessToken,
  getRefreshToken,
  hydrateSessionStorage,
} from '@helpin-ai/support-core'
import { authService } from '@mobile/lib/services/auth-service'
import type { User } from '@mobile/lib/types'

interface AuthState {
  user: User | null
  loading: boolean
  serverUnreachable: boolean
}

export const useAuthStore = create<AuthState>(() => ({
  user: null,
  loading: true,
  serverUnreachable: false,
}))

export async function bootstrapAuth(): Promise<void> {
  await hydrateSessionStorage()
  if (!getAccessToken() && !getRefreshToken()) {
    useAuthStore.setState({ user: null, loading: false })
    return
  }
  const { data, error, isNetworkError } = await authService.me()
  if (data && !error) {
    useAuthStore.setState({ user: data, loading: false, serverUnreachable: false })
  } else if (isNetworkError) {
    // Server unreachable — keep tokens, do NOT clear the session
    useAuthStore.setState({ loading: false, serverUnreachable: true })
  } else {
    // Genuine auth failure (401, invalid token) — clear session
    await clearSession()
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false })
  }
}

export async function signOut(): Promise<void> {
  await clearSession()
  useAuthStore.setState({ user: null })
}
```

The shared client reports network failure as `isNetworkError: true` with no `status` field (`packages/support-core/src/auth-api.ts:158`); the branching above mirrors `frontend/src/stores/authStore.ts`'s `initialize()` exactly. Getting this wrong logs users out on offline startup — test (c) exists to prevent that.

- [ ] **Step 3: Boot sequence in `main.tsx`** — mirror desktop `main.tsx` exactly, minus tray/badge/menu: `configureSessionStorage(isTauri() ? createTauriSessionStorage() : createBrowserSessionStorage())`, call `bootstrapAuth()`, start/stop token-refresh timers on user presence, `router.invalidate()` on auth changes, pass `{ auth }` router context.

- [ ] **Step 4: Tests pass; typecheck; commit** — `git commit -m "feat(mobile): auth, session persistence, boot sequence"`

---

### Task 9: Login screen, workspace picker, You screen

**Files:**
- Modify: `apps/support-mobile/src/screens/login-screen.tsx`, `workspaces-screen.tsx`, `you-screen.tsx`
- Create: `apps/support-mobile/src/ui/text-field.tsx`
- Create: `apps/support-mobile/src/lib/use-keyboard-inset.ts`
- Test: `apps/support-mobile/src/lib/__tests__/use-keyboard-inset.test.ts`

**Interfaces:**
- Consumes: `authService.signin`, `writeSession` (support-core), `bootstrapAuth`, `workspacesService.list`, `Avatar`, `Pressable`.
- Produces: `TextField({ label, type?, value, onChange, error?, autoComplete? })` — 52px floating-label input; `useKeyboardInset()` hook that syncs `--keyboard-inset` from `window.visualViewport`.

- [ ] **Step 1: Failing test for `useKeyboardInset`** — mock `visualViewport` (height 800 → 500 with `window.innerHeight` 800), assert `document.documentElement.style` gets `--keyboard-inset: 300px`, and `0px` on restore.

- [ ] **Step 2: Implement `use-keyboard-inset.ts`**

```ts
import { useEffect } from 'react'

export function useKeyboardInset(): void {
  useEffect(() => {
    const vv = window.visualViewport
    if (!vv) return
    const update = () => {
      const inset = Math.max(0, window.innerHeight - vv.height - vv.offsetTop)
      document.documentElement.style.setProperty('--keyboard-inset', `${inset}px`)
    }
    vv.addEventListener('resize', update)
    vv.addEventListener('scroll', update)
    update()
    return () => {
      vv.removeEventListener('resize', update)
      vv.removeEventListener('scroll', update)
      document.documentElement.style.setProperty('--keyboard-inset', '0px')
    }
  }, [])
}
```

Call it once in `RootComponent`.

- [ ] **Step 3: Build the Login screen to the A3 spec.** Brand gradient background (light + dark variants via Tailwind `dark:`), centered card (max-w-sm, gutter 16px), TextFields for email/password with per-field error strings from state, submit → `authService.signin(email, password, true)`; on success `writeSession({ accessToken, refreshToken, rememberMe: true })` using the response's token field names (verify against `AuthResponse` in `frontend/src/lib/types.ts` and how `apps/support-desktop/src/pages/login-page.tsx` writes the session — copy that call exactly), then `bootstrapAuth()` and `router.navigate({ to: '/workspaces' })`. Error from API → single error line above the button (not a toast). Button disabled while pending with inline spinner. The card sits in a container with `pb-[var(--keyboard-inset)]` so the keyboard never covers it.

- [ ] **Step 4: Build the Workspace picker to the A3 spec.** `workspacesService.list()` → rows (Avatar 44px, name headline, role footnote if present in the response), stagger animation (`motion.div` with `transition={{ delay: i * 0.03 }}`, cap 10 rows), tap → persist choice (`LazyStore` `prefs/workspace.json` key `last_workspace_slug`) → navigate to `/w/$slug/support`. On mount: if exactly one workspace, or a stored `last_workspace_slug` matches an available workspace, redirect immediately (before paint).

- [ ] **Step 4.5: Build the You screen to spec A3.** Grouped list sections using plain divs + hairline dividers: profile row (Avatar 44px, name headline, email footnote); Workspace row (current workspace name, chevron, tap → `/workspaces`); Appearance row with a Light/Dark/System `SegmentedControl` wired to `useTheme()` from next-themes; Sign out as a destructive red Pressable row with a confirm step (inline "Tap again to confirm" state for 3s — no modal); footer caption with app version from `mobile_shell_info` (Task 2's Rust command via `invoke`). The Notifications row is added by Task 20 — leave that section absent for now, not stubbed.

- [ ] **Step 5: End-to-end verify on emulator** against staging or local API (`VITE_API_URL` in `.env.local`; on Android emulator use `http://10.0.2.2:8080/api` for a local server — document in README): real login lands on placeholder Inbox; kill + relaunch app → still signed in, skips login and picker. Both themes checked.

- [ ] **Step 6: Commit** — `git commit -m "feat(mobile): login and workspace picker"`

---

### Task 10: support-core mutations (send, status, assign) + vitest setup

**Files:**
- Modify: `packages/support-core/package.json` — add `"test": "vitest run"`, devDeps `vitest ^3`, `@tanstack/react-query` already present
- Modify: `packages/support-core/src/support-service.ts`
- Modify: `packages/support-core/src/use-support-core.ts`
- Test: `packages/support-core/src/__tests__/support-mutations.test.ts`

**Interfaces:**
- Consumes: existing `getApi()`, `supportQueryKeys`, `updateConversationListUnreadCount` cache helpers.
- Produces (exact signatures later tasks depend on):
  - `supportService.sendMessage(workspaceId: string, conversationId: string, payload: SendMessagePayload)` → `ApiResponse<SupportMessage>` where `SendMessagePayload` mirrors the web composer's POST body to `/support/inbox/conversations/{id}/messages` — before implementing, read the web sender (grep `conversations/${conversationId}/messages` in `frontend/src/lib/services/`, line ~215 of the support inbox service) and `SupportInboxHandler.CreateConversationMessage`'s request struct in `server/internal/handler/`, and copy the exact field names (expect `content` plus an internal-note flag and optional attachments).
  - `supportService.updateConversationStatus(workspaceId, conversationId, status: ConversationStatus)` → PUT `/support/inbox/conversations/{id}/status` body `{ status }` (confirmed at `frontend` service line 228). **Prerequisite:** the `ApiLike` adapter at `support-service.ts:14` only exposes `get`/`post` today — widen it with `put: <T>(path: string, body?: unknown) => Promise<ApiResponse<T>>`. The shared `createApiClient` already implements `put`, so web/desktop hosts satisfy the widened type with no changes.
  - `supportService.assignConversationUser(workspaceId, conversationId, payload: { user_id: string | null })` → POST `/support/inbox/conversations/{id}/assign-user` (verify body field against the web caller, line ~245).
  - `supportService.listConversationAssignees(workspaceId, conversationId)` → GET `/support/inbox/conversations/{id}/assignees` (route confirmed at `server/internal/router/router.go:754`, handler `ListConversationAssignableUsers` — take the response type from that handler). Task 15's assign sheet depends on this.
  - Query key: add `assignees: (wsId, conversationId)` to `supportQueryKeys` following the existing key shapes in `support-query-keys.ts`.
  - Hooks: `useSendMessage(workspaceId, conversationId)` (optimistic append to the `supportQueryKeys.messages(workspaceId, conversationId)` cache — note the key is named `messages`, not `conversationMessages` — with a `pending: true` client-side flag, rollback on error), `useUpdateConversationStatus(workspaceId)`, `useAssignConversationUser(workspaceId)`, `useConversationAssignees(workspaceId, conversationId)` — mutations invalidate the conversation + list queries on settle.

- [ ] **Step 1: Add vitest to support-core; write failing tests** using a fake api injected via `configureSupportApi`: sendMessage posts to the right path with the right body; useSendMessage optimistically appends then replaces on success and removes + surfaces error on failure (use `@tanstack/react-query`'s `QueryClientProvider` test harness with `renderHook` from `@testing-library/react`).
- [ ] **Step 2: Run — FAIL.** `cd packages/support-core && pnpm test`
- [ ] **Step 3: Implement service methods + hooks.** Optimistic message object: `{ id: 'pending-' + crypto.randomUUID(), conversation_id, content, sender_type: 'user', created_at: new Date().toISOString(), pending: true }` cast through the `SupportMessage` type — add `pending?: boolean` to `SupportMessage` in `support-types.ts` (client-only field; harmless for web/desktop).
- [ ] **Step 4: Tests pass; `pnpm typecheck` in support-core, frontend, and support-desktop** (all three consume this package — the CI desktop gate exists precisely for this).
- [ ] **Step 5: Commit** — `git commit -m "feat(support-core): send/status/assign mutations with optimistic send"`

---

### Task 11: Inbox screen (virtualized list, filters, mailbox sheet, states)

**Files:**
- Modify: `apps/support-mobile/src/screens/inbox-screen.tsx`
- Create: `apps/support-mobile/src/inbox/conversation-cell.tsx`
- Create: `apps/support-mobile/src/inbox/mailbox-sheet.tsx`
- Create: `apps/support-mobile/src/inbox/inbox-helpers.ts`
- Create: `apps/support-mobile/src/stores/workspace-store.ts` (current workspace id+slug, set by the inbox route loader via `workspacesService.getBySlug`)
- Test: `apps/support-mobile/src/inbox/__tests__/inbox-helpers.test.ts`

**Interfaces:**
- Consumes: `useConversations(workspaceId, { filter, mailbox_id })`, `useUnreadStats`, `useSupportMailboxes`, `SupportConversation` type, `SegmentedControl`, `Sheet`, `Avatar`, `Badge`, `Skeleton`, `EmptyState`, `TopBar large`.
- Produces: `ConversationCell({ conversation, onPress })` (84px, spec A3); helpers `formatRelativeTime(iso: string, now?: Date): string` ("Now" <60s, "12m", "3h", "Yesterday", "Jul 2"), `previewText(conversation: SupportConversation): string` (verify which field carries the last-message preview by reading the `SupportConversation` interface at `packages/support-core/src/support-types.ts:32-78` and using the same field `ConversationRow.tsx` renders), `isUnread(conversation): boolean` (same source of truth as `ConversationRow.tsx`'s unread visual state).

- [ ] **Step 1: Failing tests** for `formatRelativeTime` (five cases above, fixed `now`), `previewText` (strips HTML tags, collapses whitespace, empty fallback "No messages yet"), `formatBadgeCount` reuse.
- [ ] **Step 2: Implement helpers; tests pass.**
- [ ] **Step 3: Build ConversationCell** exactly to spec A3 (fixed 84px height — the virtualizer depends on it). Unread state: 8px primary dot in a fixed 16px leading gutter (layout identical read/unread — only opacity of the dot changes; no text shifting), name/preview weight+color shift, timestamp `tnum`.
- [ ] **Step 4: Build the screen.** Layout: TopBar(large, title = current mailbox name or "Inbox", scroll-linked) → SegmentedControl (Mine/Unassigned/All with counts from `useUnreadStats` meta) → virtualized list (`useVirtualizer` with `estimateSize: () => 84`, `overscan: 8`) inside the single scroll container that also drives the TopBar collapse. Mailbox button in TopBar trailing opens MailboxSheet (list of mailboxes from `useSupportMailboxes` + "All inboxes" row; selection updates local filter state and closes). Loading → 8 `<Skeleton>` cells with the exact cell geometry. Empty → `EmptyState(icon: Inbox, title: "Inbox zero", body: "New conversations will appear here.")`. Error → EmptyState with retry action calling `refetch`.
- [ ] **Step 5: Cell press** → `router.navigate({ to: '/w/$slug/support/$conversationId', params })`.
- [ ] **Step 6: Verify on emulator with staging data**: scroll a long list smoothly; filters switch with correct counts; mailbox sheet works; skeleton→content has no layout jump; dark mode. `pnpm test && pnpm typecheck`.
- [ ] **Step 7: Commit** — `git commit -m "feat(mobile): inbox screen with virtualized conversation list"`

---

### Task 12: Pull-to-refresh + swipeable cells

**Files:**
- Create: `apps/support-mobile/src/inbox/use-pull-to-refresh.ts`
- Create: `apps/support-mobile/src/inbox/swipeable-row.tsx`
- Modify: `apps/support-mobile/src/screens/inbox-screen.tsx`, `src/inbox/conversation-cell.tsx`
- Test: `apps/support-mobile/src/inbox/__tests__/swipe-logic.test.ts`

**Interfaces:**
- Consumes: `useMarkConversationRead/Unread`, `useUpdateConversationStatus` (Task 10), `haptic`.
- Produces:
  - `usePullToRefresh({ scrollRef, onRefresh: () => Promise<void> })` → `{ pullDistance, refreshing }` (rubber-band ×0.5, arm at 70px + `impactLight`, release → spinner until promise settles)
  - `SwipeableRow({ leading: SwipeAction, trailing: SwipeAction, children })` where `SwipeAction = { label, icon, tone: 'primary' | 'success', onCommit }` — commit threshold 96px (haptic on arm), auto-commit past 60% width, springs closed after commit
  - Pure helpers exported for tests: `swipeState(dx: number, width: number): 'idle' | 'armed' | 'auto'` and `rubberBand(dy: number): number`

- [ ] **Step 1: Failing tests**: `swipeState(50, 400) === 'idle'`, `swipeState(120, 400) === 'armed'`, `swipeState(280, 400) === 'auto'`; `rubberBand(140) === 70`.
- [ ] **Step 2: Implement.** SwipeableRow uses `motion.div` with `drag="x"`, `dragConstraints`, action underlays revealed behind the cell (blue "Read" leading with `MailOpen` icon, green "Resolve" trailing with `Check`); on commit run the mutation optimistically (list cache updates come from the mutation hooks). Only one row open at a time (module-level "close others" event). Pull-to-refresh renders a 0-height container above the list that expands with `pullDistance`, spinner rotation = `pullDistance * 2.6deg` until armed, then continuous spin.
- [ ] **Step 3: Tests pass; on-device verify**: swipe feel (tracks finger, no scroll-vs-swipe conflict — horizontal intent lock after 10px dx dominance), pull-to-refresh arms with haptic, refreshes conversations+stats; resolve swipe removes the row from `Mine` filter with a smooth exit animation.
- [ ] **Step 4: Commit** — `git commit -m "feat(mobile): pull-to-refresh and swipeable conversation cells"`

---

### Task 13: Conversation screen — thread rendering

**Files:**
- Modify: `apps/support-mobile/src/screens/conversation-screen.tsx`
- Create: `apps/support-mobile/src/thread/message-list.tsx`, `message-bubble.tsx`, `email-body.tsx`, `thread-helpers.ts`
- Test: `apps/support-mobile/src/thread/__tests__/thread-helpers.test.ts`

**Interfaces:**
- Consumes: `useConversation`, `useConversationMessages`, `useMarkConversationRead`, `useSupportPresenceStore` (typing/viewers), `SupportMessage`, `MessageSenderType`, `TopBar`, `riseIn`.
- Produces:
  - `groupMessages(messages: SupportMessage[]): ThreadItem[]` where `ThreadItem = { kind: 'day'; label: string } | { kind: 'cluster'; senderType: MessageSenderType; senderName: string; messages: SupportMessage[] }` — day boundaries by local date ("Today"/"Yesterday"/"EEE, MMM d"), clusters break on sender change or >3min gap
  - `MessageBubble({ message, align: 'left' | 'right' })` — chat bubble; internal notes render the amber full-width variant; email messages delegate to `EmailBody`
  - `EmailBody({ html, subject? })` — DOMPurify-sanitized render in a `.selectable` container, quoted history (`blockquote`, `gmail_quote` div patterns) collapsed behind a "•••" Pressable

- [ ] **Step 1: Failing tests** for `groupMessages`: day separators inserted correctly across a 3-day fixture; clustering breaks at 3min/sender change; empty input → `[]`. And for the quote-collapse splitter `splitQuotedHtml(html): { visible: string; quoted: string | null }` (fixture with a `gmail_quote` block).
- [ ] **Step 2: Implement helpers; tests pass.**
- [ ] **Step 3: Build the screen to spec A3.** Header from `useConversation` (cache-primed by the list so it paints immediately); presence dot from the presence store's visitor-online state; status+assignee subtitle. Thread: plain scroll container (messages arrive fully — virtualization unnecessary at typical thread sizes; revisit if profiling in Task 22 says otherwise), auto-scroll to bottom on mount and on own-sends; "New message" pill when scrolled up >300px and new items append (uses a `scrollBottomDistance` ref). Message skeletons while loading (3 alternating-width bubbles). Mark-read on mount via the existing hook. Typing indicator bubble below the last cluster when the presence store reports an active typer for this conversation.
- [ ] **Step 4: Determine message field names from `SupportMessage`** (`packages/support-core/src/support-types.ts:169+`) — content/html fields, `sender_type`, internal-note flag, attachments — and render `SupportAttachmentGallery`-equivalent minimal attachment rows (filename + size, image thumbnails full-bleed in bubble, tap → OS viewer via `window.open` presigned URL).
- [ ] **Step 5: Verify on device** with a real email conversation and a widget-chat conversation from staging: email quoted-history collapse works; images render; day separators correct; scroll feel native; VoiceOver reads clusters sensibly.
- [ ] **Step 6: Commit** — `git commit -m "feat(mobile): conversation thread rendering"`

---

### Task 14: Composer (reply/note, optimistic send, keyboard)

**Files:**
- Create: `apps/support-mobile/src/thread/composer.tsx`, `send-button.tsx`, `draft-store.ts`
- Modify: `apps/support-mobile/src/screens/conversation-screen.tsx`
- Test: `apps/support-mobile/src/thread/__tests__/draft-store.test.ts`, `__tests__/send-button.test.tsx`

**Interfaces:**
- Consumes: `useSendMessage` (Task 10), `haptic`, `useKeyboardInset` (already global), `SegmentedControl`.
- Produces: `useDraftStore` — zustand map `conversationId → { text: string; mode: 'reply' | 'note' }`, persisted to `sessionStorage` so drafts survive route changes but not app restarts; `SendButton({ state: 'disabled' | 'active' | 'sending' | 'sent', onPress })`.

- [ ] **Step 1: Failing tests**: draft store set/get/clear per conversation + mode preserved; SendButton renders spinner in `sending` and fires nothing while `sending`.
- [ ] **Step 2: Implement composer to spec A3.** Container: `pb-[max(var(--safe-bottom),8px)] mb-[var(--keyboard-inset)]` with 150ms ease-out on the margin so it moves with the keyboard; hairline top border; Reply/Note toggle (note mode: `bg-amber-500/10` composer tint, placeholder "Internal note…"); auto-grow textarea (shadow div measurement or `field-sizing: content` with a max-height of 6 lines); send: `useSendMessage.mutate` → optimistic bubble (`riseIn`, 70% opacity) → on success `notificationSuccess` haptic + button `sent` state 400ms; on error `notificationError`, message row gets a destructive "Retry" chip wired to re-`mutate`. Clear draft on confirmed send only.
- [ ] **Step 3: Tests pass; on-device verify**: keyboard opens → composer rides above it, thread stays scrolled to bottom; rotate/dismiss keyboard cleanly; send round-trips against staging; airplane mode → failed state + retry works after reconnect.
- [ ] **Step 4: Commit** — `git commit -m "feat(mobile): composer with optimistic send and notes"`

---

### Task 15: ContextSheet — customer info, status, assign, tags

**Files:**
- Create: `apps/support-mobile/src/thread/context-sheet.tsx`, `assign-list.tsx`
- Modify: `apps/support-mobile/src/screens/conversation-screen.tsx` (header tap + overflow open the sheet)
- Test: `apps/support-mobile/src/thread/__tests__/context-sheet.test.tsx`

**Interfaces:**
- Consumes: `useVisitorContext`, `useConversation`, `useConversationAssignees` (Task 10 — assignee candidates from `/inbox/conversations/{id}/assignees`), `useUpdateConversationStatus`, `useAssignConversationUser`, `Sheet`, `Avatar`, `Badge`.
- Produces: `ContextSheet({ workspaceId, conversationId, open, onOpenChange })`.

- [ ] **Step 1: Failing test**: sheet renders customer name/email from a mocked conversation; Resolve button calls `useUpdateConversationStatus().mutate` with `resolved`; assign row tap calls assign mutation with the teammate's id.
- [ ] **Step 2: Implement to spec A3**: customer block (tap-to-copy email via `navigator.clipboard` + "Copied" toast), action row (Resolve/Reopen from current `status`, Assign expands an inline teammate list with avatars + "Unassign" row, Tags section reads conversation tags as chips — tag *editing* is V1.1; render read-only), visitor context rows (browser/OS/location — take field names from `VisitorContextResponse` in `visitor-types.ts`). All mutations optimistic; error → rollback + sonner toast.
- [ ] **Step 3: Tests pass; device verify** both detents, drag-dismiss, actions reflected in inbox list after back-navigation.
- [ ] **Step 4: Commit** — `git commit -m "feat(mobile): conversation context sheet with status/assign"`

---

### Task 16: Realtime, lifecycle resume, offline banner

**Files:**
- Create: `apps/support-mobile/src/lib/use-mobile-realtime.ts`, `src/ui/offline-banner.tsx`
- Modify: `apps/support-mobile/src/screens/inbox-screen.tsx` (mount realtime at workspace level), `src/screens/conversation-screen.tsx` (pass `selectedConversationId`)
- Test: `apps/support-mobile/src/lib/__tests__/use-mobile-realtime.test.ts`

**Interfaces:**
- Consumes: `useSupportRealtime({ apiBase, workspaceId, selectedConversationId, onEvent })`, `useSupportRealtimeStore` (connection status), `setupVisibilityRefresh` (already running from Task 8).
- Produces: `useMobileRealtime(workspaceId, selectedConversationId)` — wraps `useSupportRealtime` and adds mobile lifecycle: on `visibilitychange` → visible, force a reconnect check + invalidate conversations/unread-stats if hidden longer than 60s (the shared controller already has wake/stale recovery with `RESUME_GAP_MS = 60_000`; verify it fires on `visibilitychange` in the webview and only add what is missing); `<OfflineBanner>` — slim amber bar under the TopBar shown when the realtime store reports disconnected + `navigator.onLine === false`, with "Reconnecting…" state.

- [ ] **Step 1: Failing test**: banner visibility logic pure function `bannerState(online: boolean, realtimeStatus: string): 'hidden' | 'offline' | 'reconnecting'`.
- [ ] **Step 2: Implement; mount; tests pass.**
- [ ] **Step 3: Device verify (the money test)**: background the app 2+ minutes → foreground: new messages appear within 2s without manual refresh; airplane mode toggling shows/hides the banner; a message arriving while on the thread screen appends live with the new-message pill when scrolled up.
- [ ] **Step 4: Commit** — `git commit -m "feat(mobile): realtime with lifecycle resume and offline banner"`

---

### Task 17: Backend — push device registration

**Files:**
- Create: `server/internal/model/push_device.go`
- Create: `server/internal/repository/push_device_repository.go`
- Create: `server/internal/service/push_device.go`
- Create: `server/internal/handler/push_device_handler.go`
- Modify: `server/internal/router/router.go` (user-scoped routes next to `/user/notification-settings`, ~line 408)
- Modify: `server/cmd/api/main.go` (AutoMigrate registration + DI wiring)
- Test: `server/internal/service/push_device_test.go`

**Interfaces:**
- Produces:
  - `model.PushDevice { ID, UserID, Platform, Token, AppVersion, LastSeenAt, CreatedAt, UpdatedAt }`, table `push_devices`
  - `POST /api/user/push-devices` body `{ "platform": "ios"|"android", "token": string, "app_version": string }` → 200 with the device (upsert by token: re-registering an existing token reassigns it to the current user and bumps `last_seen_at` — phones change owners/accounts)
  - `DELETE /api/user/push-devices` body `{ "token": string }` → 204 (called on sign-out)
  - `PushDeviceRepository.ListByUserIDs(ctx, userIDs []string) ([]model.PushDevice, error)` — consumed by Task 18

- [ ] **Step 1: Write the model**

```go
package model

import "time"

type PushDevice struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string    `json:"user_id" gorm:"type:uuid;index;not null"`
	Platform   string    `json:"platform" gorm:"not null"`
	Token      string    `json:"token" gorm:"uniqueIndex;not null"`
	AppVersion string    `json:"app_version"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PushDevice) TableName() string { return "push_devices" }

type RegisterPushDeviceRequest struct {
	Platform   string `json:"platform"`
	Token      string `json:"token"`
	AppVersion string `json:"app_version"`
}

type UnregisterPushDeviceRequest struct {
	Token string `json:"token"`
}
```

- [ ] **Step 2: Failing service tests** (fake repo implementing a small interface):

```go
func TestRegisterDevice_UpsertsByToken(t *testing.T) {
	repo := &fakePushDeviceRepo{}
	svc := service.NewPushDeviceService(repo)

	_, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "ios", Token: "tok-a", AppVersion: "0.1.0"})
	require.NoError(t, err)

	// same token, different user → device moves to user-2
	dev, err := svc.Register(context.Background(), "user-2", model.RegisterPushDeviceRequest{Platform: "ios", Token: "tok-a", AppVersion: "0.2.0"})
	require.NoError(t, err)
	require.Equal(t, "user-2", dev.UserID)
	require.Len(t, repo.devices, 1)
}

func TestRegisterDevice_RejectsBadPlatform(t *testing.T) {
	svc := service.NewPushDeviceService(&fakePushDeviceRepo{})
	_, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "web", Token: "t"})
	require.Error(t, err)
}
```

Run: `cd server && go test ./internal/service/ -run TestRegisterDevice -v` — expect FAIL (packages don't exist).

- [ ] **Step 3: Implement repository (GORM upsert on token conflict via `clause.OnConflict{Columns: token, UpdateAll: true}`), service (validates platform ∈ {ios, android}, token non-empty, stamps `LastSeenAt`), handler (decode → service → writeJSON, following any existing handler in `internal/handler/` as the template).** Wire routes inside the authenticated user-scoped block at `router.go:408`:

```go
r.Post("/user/push-devices", h.PushDevice.Register)
r.Delete("/user/push-devices", h.PushDevice.Unregister)
```

DI in `main.go`: repo from `*gorm.DB`, service from repo, handler from service; append `&model.PushDevice{}` to the AutoMigrate call.

- [ ] **Step 4: Tests pass; `go vet ./...` clean.**
- [ ] **Step 5: Commit** — `git commit -m "feat(server): push device registration endpoints"`

---

### Task 18: Backend — FCM sender wired into support notifications

**Files:**
- Create: `server/internal/service/push_sender.go`
- Modify: `server/internal/service/support_notification.go` — `ProcessSupportCustomerReplyNotification` (line ~71)
- Modify: `server/internal/config/` — add `FCM_SERVICE_ACCOUNT_JSON` (optional; push disabled when empty), document in `server/.env.example`
- Modify: `server/cmd/api/main.go` — DI
- Test: `server/internal/service/push_sender_test.go`

**Interfaces:**
- Consumes: `PushDeviceRepository.ListByUserIDs` (Task 17).
- Produces:
  - `type FCMClient interface { Send(ctx context.Context, token string, n PushNotification) error }` (implemented by a `firebaseFCMClient` wrapping `firebase.google.com/go/v4/messaging`, and by fakes in tests)
  - `PushNotification { Title, Body string; Data map[string]string }` — Data carries `workspace_slug`, `conversation_id`, `deep_link` (`helpin://w/{slug}/support/{conversationId}`)
  - `PushSenderService.NotifyUsers(ctx, userIDs []string, n PushNotification)` — fans out to all devices of the users, logs-and-continues per-device errors, deletes devices on FCM "unregistered" errors, no-ops when FCM is not configured

- [ ] **Step 1: `go get firebase.google.com/go/v4` in `server/`.**
- [ ] **Step 2: Failing tests** with a fake FCMClient: NotifyUsers sends one message per device with the data payload intact; a device returning an unregistered error is deleted from the repo; nil client (unconfigured) → no error, no panic.
- [ ] **Step 3: Implement `push_sender.go`.** Firebase init in main.go only when `FCM_SERVICE_ACCOUNT_JSON` is non-empty (`option.WithCredentialsJSON`). Use `slog` per project logging rules (`slog.ErrorContext(ctx, "push send failed", "error", err, "user_id", ...)`; never log tokens).
- [ ] **Step 4: Hook the emit path.** Read `ProcessSupportCustomerReplyNotification` in `support_notification.go:71` fully first; after it determines recipient user IDs for a customer reply (mentions flow at line 36 likewise), call `pushSender.NotifyUsers` with title = customer name, body = plain-text message preview (truncate 140 chars), and the deep-link data. **Slug resolution:** `conv` carries only `WorkspaceID`, not the slug — but `notifService.workspaceRepo` is already a field on the service (used at line 84 for recipient selection). Look up the workspace by `conv.WorkspaceID` through it and use its `Slug` for `workspace_slug` and the `helpin://w/{slug}/support/{id}` deep link. If the lookup fails, log at WARN and send the push with `conversation_id` but no `deep_link` (the client's `routePushTap` fallback needs both `workspace_slug` and `conversation_id`, so a slugless push degrades to opening the app on its default screen — acceptable; never drop the notification over it). Thread the service through existing call sites the same way its current dependencies are threaded (constructor injection, matching the file's existing style). Respect the existing user notification settings: if the recipients list is already filtered for in-app notifications, push inherits that filtering; add a `push_enabled` check only if `user_notification_settings` already models channel preferences — read `user_notification_settings.go` and follow what exists rather than inventing a new preference in this task.
- [ ] **Step 5: Tests pass; `go vet ./...`; manual smoke**: with a real service-account JSON in `.env`, register a real device token (Task 20), send a test customer reply on staging, observe delivery.
- [ ] **Step 6: Commit** — `git commit -m "feat(server): FCM push fan-out on support customer replies"`

---

### Task 19a (spike — run before Task 19): Prove push delivery on real hardware

**Files:**
- Create: `docs/research/2026-07-08-push-spike-findings.md` — findings write-up (the only merged artifact)
- Scratch code on a throwaway branch off `feat/support-mobile`; none of it merges

Push is the highest-integration-risk item in this plan: Firebase SDK versions, Gradle/Xcode wiring, notification permission states, APNs token handoff, and cold-start payload delivery each have failure modes that are invisible until tried on hardware. De-risk before building the real plugin.

- [ ] **Step 1: Ops prerequisite (gates everything push-related):** create the Firebase project `helpin-mobile`, add Android app `ai.helpin.mobile` (download `google-services.json`) and iOS app `ai.helpin.mobile` (download `GoogleService-Info.plist`), upload the APNs auth key (from the Apple Developer account) to Firebase Cloud Messaging settings, and generate the service-account JSON for Task 18. Store artifacts in Doppler, never in git — add both filenames to `.gitignore` and document retrieval in the app README.
- [ ] **Step 2: Minimal token proof.** Scaffold a throwaway plugin (`npx @tauri-apps/cli plugin new`), hard-code the minimum needed to obtain an FCM token on one physical Android device and one physical iPhone, and log it.
- [ ] **Step 3: Delivery + tap proof.** Send test messages from the Firebase console. Verify: background delivery on both platforms; tap launches the app; and — critically — the data payload is retrievable on **cold start** (Android: launcher intent extras; iOS: `didReceive response.userInfo`), since Task 20's tap routing depends on it.
- [ ] **Step 4: Write the findings doc:** exact SDK versions and integration mechanism (SPM vs pods, Gradle plugin versions), required Xcode capabilities, permission-prompt behavior per platform, cold-start payload mechanics, foreground-message behavior, and any Tauri-specific gotchas. Task 19 implements from these findings, not from assumptions.
- [ ] **Step 5: Commit the findings doc** — `git commit -m "docs(mobile): push integration spike findings"`

---

### Task 19: Tauri push plugin (`tauri-plugin-helpin-push`)

**Files:**
- Create: `apps/support-mobile/src-tauri/tauri-plugin-helpin-push/` — scaffolded via `npx @tauri-apps/cli plugin new helpin-push --android --ios --directory apps/support-mobile/src-tauri` (adjust the exact flag syntax to the installed CLI version; the generator produces the Rust crate + `android/` Kotlin + `ios/` Swift + `guest-js/` bindings)
- Modify: `apps/support-mobile/src-tauri/Cargo.toml`, `src/lib.rs`, `capabilities/default.json`
- Modify: `src-tauri/gen/android/**` — add `google-services.json`, Gradle FCM deps; `src-tauri/gen/apple/**` — push capability + `GoogleService-Info.plist`

**Interfaces:**
- Produces (guest-js API consumed by Task 20):
  - `getPushToken(): Promise<string | null>` — current FCM token (requests OS notification permission on iOS first call)
  - `onPushTokenChanged(cb: (token: string) => void): Promise<UnlistenFn>`
  - `onPushTapped(cb: (data: Record<string, string>) => void): Promise<UnlistenFn>` — fires with the `Data` payload both when the app is cold-started from a notification (delivered after webview ready) and when tapped while running

- [ ] **Step 1: Confirm Task 19a is done:** the Firebase project exists, config files are retrievable from Doppler, and `2026-07-08-push-spike-findings.md` records the SDK versions and integration steps. Implement this task per those findings — where the findings contradict the outline below, the findings win.
- [ ] **Step 2: Scaffold the plugin; define the Rust command/event surface** (`get_push_token` command; `push-token-changed`, `push-tapped` events emitted via `app.emit`). Register in `lib.rs` (`.plugin(tauri_plugin_helpin_push::init())`) and grant `helpin-push:default` in capabilities.
- [ ] **Step 3: Android implementation (Kotlin):** add `com.google.firebase:firebase-messaging` via the plugin's `build.gradle.kts` + Google services Gradle plugin in `gen/android`; implement `HelpinMessagingService : FirebaseMessagingService` — `onNewToken` → emit `push-token-changed`; notification display for data-messages while app is foregrounded is suppressed (in-app realtime already covers it); background data+notification messages use FCM's default tray behavior; the launcher intent's extras carry the data payload → plugin reads extras on activity start and emits `push-tapped`.
- [ ] **Step 4: iOS implementation (Swift):** FirebaseMessaging pod/SPM in the plugin's iOS package; `application(_:didRegisterForRemoteNotificationsWithDeviceToken:)` → set APNs token on `Messaging.messaging()`; `MessagingDelegate.didReceiveRegistrationToken` → emit `push-token-changed`; `UNUserNotificationCenterDelegate.didReceive response` → emit `push-tapped` with `userInfo`. Enable Push Notifications + Background Modes (remote-notification) capabilities in the generated Xcode project.
- [ ] **Step 5: Verify on physical devices** (push does not work on simulators/emulators for APNs; Android emulators with Play services do work): `getPushToken()` returns a token on both platforms; sending a test message from the Firebase console reaches the device in background; tapping it opens the app and `onPushTapped` fires with the data payload.
- [ ] **Step 6: Commit** — `git commit -m "feat(mobile): tauri push plugin for FCM/APNs"`

---

### Task 20: Client push registration + permission priming + tap routing

**Files:**
- Create: `apps/support-mobile/src/push/push-registration.ts`, `permission-priming-sheet.tsx`
- Modify: `apps/support-mobile/src/main.tsx` (tap routing), `src/screens/inbox-screen.tsx` (priming trigger), `src/stores/auth-store.ts` (`signOut` unregisters), `src/screens/you-screen.tsx` (notification row)
- Test: `apps/support-mobile/src/push/__tests__/push-registration.test.ts`

**Interfaces:**
- Consumes: plugin API (Task 19), `POST/DELETE /user/push-devices` (Task 17).
- Produces: `registerForPush(): Promise<void>` (token → register endpoint, subscribes to token-changed for re-register), `unregisterPush(): Promise<void>`, `routePushTap(data: Record<string,string>, navigate: (to: string) => void)` — pure, testable: prefers `deep_link`, falls back to building `/w/{workspace_slug}/support/{conversation_id}`.

- [ ] **Step 1: Failing tests** for `routePushTap` (deep_link present, fallback fields present, garbage → no navigation) and for `registerForPush` posting `{ platform, token, app_version }` (mock plugin + api).
- [ ] **Step 2: Implement.** Permission priming (world-class detail — never cold-prompt): first time the user lands on Inbox with a signed-in session, show a Sheet: bell icon, "Never miss a customer" headline, one-line value prop, primary "Enable notifications" → calls `registerForPush()` (which triggers the OS prompt), quiet "Not now" → re-ask no sooner than 7 days (persist decision in the prefs store). You-screen notification row reflects current OS permission state and deep-links to OS settings when denied.
- [ ] **Step 3: Wire tap routing in `main.tsx`**: `onPushTapped(data => routePushTap(data, to => router.navigate({ to })))` — cold-start taps must land on the conversation after auth bootstrap completes (queue the navigation until `loading === false`).
- [ ] **Step 4: Device verify, full loop**: staging customer reply → push arrives (app backgrounded) → tap → app opens directly on that conversation, back goes to Inbox; sign out → device row deleted (verify via DB) → no more pushes.
- [ ] **Step 5: Commit** — `git commit -m "feat(mobile): push registration, priming, and tap-through routing"`

---

### Task 21: Deep links, icons, splash, CI

**Files:**
- Modify: `apps/support-mobile/src-tauri/Cargo.toml`, `tauri.conf.json`, `capabilities/default.json` — `tauri-plugin-deep-link = "2"` with scheme `helpin`
- Create: app icon set via `pnpm tauri icon <path-to-1024px-master>` (request the master asset from design; interim: Helpin logomark on brand-primary rounded square)
- Modify: `src-tauri/gen/android` (splash + adaptive icon), `gen/apple` (launch screen: plain `--background` color — a blank brand-colored launch screen reads as fast; no logo splash that must animate away)
- Modify: `apps/support-mobile/src/main.tsx` — `onOpenUrl` handler routing `helpin://` URLs through the same `routePushTap` path shape
- Modify: `.github/workflows/ci.yml` — add `mobile-check` job: `pnpm install`, `pnpm --dir apps/support-mobile typecheck`, `pnpm --dir apps/support-mobile test`, plus `cargo check` in `src-tauri` (desktop-host check only; full Android builds are release-pipeline work, out of CI-per-PR scope)

**Interfaces:**
- Produces: `helpin://w/{slug}/support/{conversationId}` opens the app to the conversation from any source (push uses it, teammates can paste it in Slack later).

- [ ] **Step 1: Add plugin + config; implement `onOpenUrl` → parse → navigate (reuse `routePushTap` parsing with a URL adapter, unit-tested).**
- [ ] **Step 2: Generate icons; set Android adaptive icon foreground/background; iOS launch storyboard = background color only.**
- [ ] **Step 3: CI job added; verify it passes on the PR.**
- [ ] **Step 4: Device verify:** `adb shell am start -a android.intent.action.VIEW -d "helpin://w/<slug>/support/<id>"` opens the thread; `xcrun simctl openurl booted helpin://…` likewise.
- [ ] **Step 5: Commit** — `git commit -m "feat(mobile): deep links, app icons, ci gate"`

---

### Task 22: Hardening — performance, accessibility, QA matrix

**Files:**
- Create: `apps/support-mobile/QA.md` — the executed checklist below with results
- Modify: whatever the audit findings require (this task is a gate, not a feature)

- [ ] **Step 1: Performance audit against A7 budgets.** Measure cold start (adb logcat timestamps / Xcode Instruments), record bundle size (`pnpm build` → gzip sizes; fail if initial chunk > 900KB — code-split the thread screen if needed), scroll-profile the inbox with 500 seeded conversations (Chrome remote devtools → Performance; no frame > 16.7ms during steady scroll), thread-open timing. Fix regressions before proceeding.
- [ ] **Step 2: Accessibility audit against A8.** VoiceOver + TalkBack pass over all five screens; OS font scaling at 120%; contrast spot-checks in dark mode (muted text on background); `prefers-reduced-motion` full-flow check.
- [ ] **Step 3: Lifecycle/edge QA matrix — execute and record:**

| Scenario | Expected |
|---|---|
| Kill app → push tap | Opens to conversation after auth restore |
| Token expiry while backgrounded overnight | Silent refresh, no logout |
| Airplane mode mid-send | Failed bubble + retry chip; retry succeeds on reconnect |
| Workspace with 0 conversations | Inbox-zero empty state |
| 10k-message thread | Opens without hang (else virtualize — noted in Task 13) |
| Rapid conversation switching | No stale thread flash (query cache keyed correctly) |
| RTL locale smoke test | Layout does not break (full RTL support is V1.1) |
| Low-end Android (2GB RAM) | Usable; no OOM |

- [ ] **Step 4: Fix everything found; commit** — `git commit -m "chore(mobile): performance, a11y, and lifecycle hardening"`

---

## Part E — Explicitly deferred to V1.1 (do not build now)

Search tab; canned responses + AI rewrite in composer; tag editing; attachment *upload* from mobile (receive/view only in V1); biometric app-lock; Android/iOS home-screen widgets; CRM context beyond visitor info; full RTL; store listing assets and release-train automation (needs a human Apple/Google account owner regardless).

## Part F — Ops prerequisites (human-owned, start early — they gate Tasks 18–19)

1. Firebase project + APNs key upload (needs Apple Developer account access) — executed as Task 19a Step 1, but account access must be arranged before Phase 8 starts.
2. Apple Developer + Google Play accounts and signing certificates.
3. `FCM_SERVICE_ACCOUNT_JSON` into Doppler for staging + prod.
4. 1024px app icon master from design.

## Self-review notes

- Spec coverage: every A3 screen maps to Tasks 9, 11–15, 20 (You-screen rows land in Tasks 9/20; theme/sign-out rows are part of Task 9's picker + Task 20's You modifications — the You screen itself is assembled across those tasks and gated by Task 22's QA pass).
- Field names intentionally verified-at-implementation in Tasks 10 and 13 (message payload, assignees response type, preview field) rather than guessed here — each carries an exact file+line to read first.
- Revised 2026-07-08 after external review: `isNetworkError` auth branching, `ApiLike.put` widening, `supportQueryKeys.messages` naming, gesture/Motion transform separation + no-history back fallback, workspace-slug lookup via `notifService.workspaceRepo` for push deep links, assignees moved into Task 10, and the Task 19a hardware spike.
- Type consistency: `SendMessagePayload`, `PushNotification`, `routePushTap`, `resolveDirection`, `swipeState` signatures are each defined once and referenced identically in consuming tasks.

