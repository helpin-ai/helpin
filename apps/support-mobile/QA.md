# Task 22 — Hardening QA: performance, accessibility, lifecycle matrix

Status: gate executed 2026-07-08 in an environment with **no physical
devices/emulators/simulators** (confirmed — see README's "Environment
prerequisites"). Every check that genuinely requires hardware (touch input,
a real WebView, OS accessibility services, real network radios) is marked
**PENDING-DEVICE** below with exact instructions and pass thresholds, to be
executed once Task 19a's hardware spike lands a real toolchain. Everything
that could be verified from source, `pnpm build`, and `pnpm test` was
measured or code-reviewed directly, and gaps found were fixed in this same
pass (see "Fixes applied").

---

## 1. Performance audit against A7 budgets

### 1a. Bundle size (measured — HARD GATE, executed here)

Budget: **initial JS < 900KB gzip**.

`pnpm build` output, before this pass (single chunk — nothing route-level
lazy-loaded):

| Asset | Raw | Gzip |
|---|---|---|
| `index-*.js` (everything) | 711.62 kB | **226.46 kB** |
| `index-*.css` | 36.95 kB | 7.94 kB |

That already clears the 900KB gzip gate with ~4x headroom (226/900 ≈ 25%
utilized). Not a failing gate, but the brief calls out the thread/conversation
screen by name as a likely split candidate, and this app has real code to
give it (composer, message list, context sheet, thread-helpers, send-button,
email-body) — 5 route components share one entry chunk today for no reason
tied to first paint. Split it via `@tanstack/react-router`'s
`lazyRouteComponent` on the `/w/$slug/support/$conversationId` route
(`src/router.tsx`).

**After** the split:

| Asset | Raw | Gzip |
|---|---|---|
| `index-*.js` (entry: login/workspaces/inbox/you + shared deps) | 665.98 kB | **210.46 kB** |
| `conversation-screen-*.js` (lazy chunk) | 47.02 kB | 17.08 kB |
| `index-*.css` | 36.95 kB | 7.94 kB |

Confirmed `dist/index.html` references only the entry JS + CSS — no
`modulepreload` for the conversation chunk, so it is genuinely deferred
(fetched on navigation; `defaultPreload: 'intent'` on the router prefetches
it the moment a conversation row is pressed, so there's no added spinner on
the common path). **Initial JS gzip: 226.46 kB → 210.46 kB (−16 kB, −7%).**
Gate passes both before and after with large margin; the split is a genuine
improvement, not a required fix.

Checked what makes up the bulk of the entry chunk (no mermaid/excalidraw/
tiptap — confirmed absent from `package.json`): `react`/`react-dom` 19,
`@tanstack/react-query` + `react-router` + `react-virtual`, `motion` 12,
`zustand`, `vaul`, `dompurify`, `sonner`, `lucide-react` (tree-shaken —
icons are imported individually throughout, not as a barrel/namespace
import), `@fontsource-variable/inter` (fonts are separate `.woff2` assets,
not inlined into JS). Nothing unexpected; no further easy wins identified
without diminishing returns (e.g. splitting `you-screen`/`workspaces-screen`
would save only a few KB each against a 210KB base that's already 23% of
budget).

### 1b. Cold start, scroll fps, thread-open timing — PENDING-DEVICE

Cannot be measured without a device/emulator (needs `adb logcat`, Xcode
Instruments, or Chrome remote DevTools attached to a real WebView process).

- [ ] **PENDING-DEVICE: Cold start → interactive inbox.**
  Threshold: **< 2.0s** on Pixel 6a, **< 1.5s** on iPhone 12.
  Instructions: `adb logcat -s ActivityTaskManager:I` and grep for
  `Displayed <package>` timestamp, diffed against the launch-intent
  timestamp (`am start` or a cold tap on the icon); on iOS, use Xcode
  Instruments' "App Launch" template, or Console.app filtering the process
  and diffing `didFinishLaunching` against first meaningful paint (Inbox
  skeleton → real rows). Run 5x cold (force-stop / swipe-kill between runs)
  and report median + worst.

- [ ] **PENDING-DEVICE: Inbox scroll — sustained 60fps with 500 conversations.**
  Threshold: **no frame > 16.7ms** during steady scroll.
  Instructions: seed 500 conversations in a test workspace (support-core
  fixtures or a seed script against a local API), open the Inbox, attach
  Chrome remote DevTools (`chrome://inspect` for Android WebView; Safari Web
  Inspector for iOS WKWebView) → Performance panel → record a ~3s continuous
  scroll from top to bottom → inspect the frame chart for any bar exceeding
  16.7ms. The inbox already uses `@tanstack/react-virtual` (confirmed in
  `src/screens/inbox-screen.tsx`), so this should pass, but the virtualizer's
  `overscan: 8` and swipeable-row's per-cell `ResizeObserver` are exactly the
  kind of thing that can regress fps and need on-device confirmation.

- [ ] **PENDING-DEVICE: Conversation open timing.**
  Threshold: header paint < 1 frame (cache-fed — confirmed in code via
  `useConversationFromListCache`, see `src/screens/conversation-screen.tsx`),
  messages < 800ms on LTE.
  Instructions: same Performance-panel recording, trigger a tap from the
  Inbox on a conversation cell, and measure (a) time from tap to header text
  appearing (should be sub-frame — it's reading an already-cached list
  entry) and (b) time from tap to the message list's first real bubble
  (`messagesQuery.isPending` flipping false), throttled to "Fast 3G"/LTE
  profile in DevTools network conditions.

- [ ] **PENDING-DEVICE: 10k-message thread open (lifecycle matrix row).**
  Threshold: opens without hang. `MessageList` is a plain scroll container
  (Task 13/21 decision — "revisit if profiling says otherwise", see plan
  line 1219) — this is the one A7/lifecycle item most likely to actually
  fail on real hardware. If it hangs or the scroll-to-bottom effect visibly
  janks, virtualize the thread (Task 13 already anticipates this as a
  fallback) — do not ship un-virtualized to a workspace with a real 10k+
  thread without running this check first.

---

## 2. Accessibility audit against A8

### 2a. Code-level sweep (executed here) — gaps found + fixed

| # | Gap | File | Fix |
|---|---|---|---|
| 1 | `ConversationCell` (`role="button"`) had no explicit `aria-label` — its accessible name would fall back to subtree text concatenation, which also picks up `Avatar`'s rendered initials text (e.g. "AL") ahead of the customer name, and never announces unread state. A8 requires cells to announce "«name», «preview», «time», unread". | `src/inbox/conversation-cell.tsx` | Added a composed `aria-label` = `"{name}, {preview}, {time}[, unread]"`. Covered by 2 new tests in `src/inbox/__tests__/conversation-cell.test.tsx`. |
| 2 | `OfflineBanner` had no `aria-live` region — the offline/reconnecting text change was silent to screen readers. | `src/ui/offline-banner.tsx` | Wrapped in a **persistent** `role="status" aria-live="polite"` container (persistent, not just on the animated child, so the live region exists before the text ever changes). Covered by a new test in `src/ui/__tests__/offline-banner.test.tsx`. |
| 3 | The "New message" pill (conversation screen) had no `aria-live` — same issue, worse because it's the primary way a screen-reader user would learn a new message arrived while scrolled up. | `src/screens/conversation-screen.tsx` | Same `role="status" aria-live="polite"` persistent-wrapper pattern. No dedicated test added (this screen has no existing render-test harness — see "Known test gaps" below); verified by code inspection only. |
| 4 | `SendButton` announced a static `"Send message"` in every state; A8 requires the send button to announce its state. | `src/thread/send-button.tsx` | Per-state `aria-label`: `disabled`/`active` → "Send message", `sending` → "Sending message", `sent` → "Message sent". Existing `send-button.test.tsx` updated for the new labels + 2 assertions renamed to check the state-specific name. |

### 2b. Verified already-fine (no change needed)

- **Icon-only Pressables**: `TopBar`'s back chevron (`aria-label="Back"`),
  the mailbox chooser (`aria-label="Choose mailbox"`), the composer's
  dismiss-failed-send `X` (`aria-label="Dismiss failed message"`),
  `context-sheet.tsx`'s copy-email button (`aria-label="Copy email"`) — all
  already labeled. `TabBar` items are NOT icon-only (they render a visible
  text caption under the icon), so their accessible name comes from that
  text — no separate label needed.
- **Swipe-action underlays** (`inbox/swipeable-row.tsx`'s `ActionUnderlay`):
  already `aria-hidden="true"` — correct, since the underlying row content
  (with its own now-labeled `ConversationCell`) is the real interactive
  element; the underlay is purely decorative/visual feedback during a drag.
- **Dialog/sheet semantics** (`ui/sheet.tsx`, wrapping vaul's `Drawer`):
  `Drawer.Title` + `Drawer.Description` are both rendered `sr-only` with a
  real `title` prop on every call site — satisfies Radix's requirement that
  a `Dialog.Content` have an accessible name and description. Focus-trapping
  is Radix's `Dialog` default behavior under vaul; not re-implemented here.
- **Focus-visible**: grepped `index.css` for `outline`/`focus-visible`/
  `:focus` — no global suppression exists. `Sheet`'s own `outline-none` is
  scoped to the drawer's own outer content box (standard — focus moves to
  content inside it, not to the box itself), not a blanket override.
- **Hit targets ≥ 44pt** (cells, tab items, header buttons, segmented
  controls — the four categories A8 actually lists): `Pressable` enforces
  `min-h-[44px] min-w-[44px]` as its base class, and every listed category
  goes through `Pressable` (tab items stretch to the tab bar's 49px row;
  header buttons and segmented-control segments use the unmodified 44px
  floor). `ConversationCell` doesn't use `Pressable` but has a fixed 84px
  row height (`CONVERSATION_CELL_HEIGHT`), well over 44pt. The one
  deliberate exception is `SendButton` (36×36, `min-h-0 min-w-0` override,
  documented in its own comment) — A8's hit-target list does not include
  the send button, so this is in-spec, not a gap.
- **Text respects OS font scaling to 120% without clipping critical UI,
  allow preview truncation**: `ConversationCell`'s preview is already
  `line-clamp-2` (explicitly allowed to truncate per A8's own wording). See
  §4 for the honest caveat on whether OS font scaling reaches this app's
  CSS at all in a Tauri WebView.

### 2c. Contrast — computed (not device-rendered, but exact)

All color tokens in `src/index.css` are achromatic OKLCH (`oklch(L 0 0)`),
so contrast can be computed exactly from the token values via an
OKLCH → linear-sRGB → WCAG relative-luminance conversion (not eyeballed):

| Pair | Theme | Contrast ratio | WCAG AA (4.5:1 normal text) |
|---|---|---|---|
| `--foreground` / `--background` | light | 16.45:1 | pass (huge margin) |
| `--muted-foreground` / `--background` | light | **4.73:1** | pass, ~5% margin — tightest pair in the system |
| `--foreground` / `--background` | dark | 13.79:1 | pass (huge margin) |
| `--muted-foreground` / `--background` | dark | 7.63:1 | pass (also clears AAA's 7:1) |
| `--destructive` text / `--destructive/10` overlay (dark, approximated over page bg) | dark | 6.84:1 | pass |

Every combination clears AA. The one worth a design note: light-theme
`muted-foreground` on `background` sits at 4.73:1 against a 4.5:1 floor —
comfortably passing today, but with the least headroom of any pair in the
system, and used for footnote/caption-sized text (`text-footnote` 13px,
`text-caption` 11px) which does NOT qualify for the "large text" 3:1
exception. Not a regression introduced by this app (tokens are copied
verbatim from the shared frontend token set per `index.css`'s own header
comment) — flagged for awareness, not fixed here (changing a shared design
token is out of scope for a mobile-only hardening pass).

### 2d. VoiceOver / TalkBack / OS font scaling — PENDING-DEVICE

- [ ] **PENDING-DEVICE: VoiceOver pass (iOS), all 5 screens** (Login,
  Workspaces, Inbox, Conversation, You). Enable VoiceOver
  (Settings → Accessibility → VoiceOver), swipe through each screen, and
  confirm: (a) every `ConversationCell` announces "«name», «preview»,
  «time»[, unread]" in one swipe (aria-label just added — verify VoiceOver
  actually reads it as one unit, not per-child); (b) the send button
  announces "Send message" → "Sending message" → "Message sent" as a real
  send goes through; (c) the offline banner and new-message pill are
  announced without needing manual focus (aria-live polite — verify no
  interruption of in-progress speech, since `polite` should queue rather
  than interrupt); (d) sheets (mailbox picker, permission-priming, context
  sheet) trap focus and announce their title on open.
- [ ] **PENDING-DEVICE: TalkBack pass (Android)** — same checklist as
  VoiceOver above; TalkBack's rotor/swipe model differs enough from
  VoiceOver's that both need independent runs, not just one assumed to
  cover the other.
- [ ] **PENDING-DEVICE: OS font scaling at 120%.** iOS: Settings →
  Accessibility → Display & Text Size → Larger Text, or Settings → Display
  & Brightness → Text Size. Android: Settings → Display → Font size. After
  setting to ~120%, revisit all 5 screens and confirm no critical UI clips
  (buttons, headers, tab bar) — preview text truncating further is
  explicitly allowed. See §4 below for why this is a genuinely open
  question in a Tauri WebView, not a rubber-stamp check.

---

## 3. Reduced-motion audit

Global CSS clamp exists (`src/index.css`):
```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation-duration: 0.12s !important; transition-duration: 0.12s !important; }
}
```
This only affects **CSS** `animation`/`transition` properties — it cannot
touch `motion`/`framer-motion`-style JS-driven animations (motion values,
`animate()` imperative calls, spring transitions), which is why every
`motion/react` usage was individually grepped and checked:

| File | Animation | Guarded? |
|---|---|---|
| `navigation/screen-stack.tsx` | Push/pop slide vs. crossfade | **Yes** — `useReducedMotion()` forces `crossfade` + short (0.15s) duration. Verified pre-existing. |
| `screens/workspaces-screen.tsx` | Staggered fade+rise entrance on the workspace list (up to ~500ms total across 10 rows) | **Fixed this pass** — was unguarded; added `useReducedMotion()` and skip the stagger/initial-offset when reduced motion is requested (same idiom as `screen-stack.tsx`). |
| `inbox/conversation-cell.tsx` | 0.2s opacity/scale fade on Resolve-swipe exit | Not guarded, but 0.2s and a direct result of the user's own swipe commit — not a decorative/ambient animation. Judged in-spec (short, functional feedback). |
| `inbox/swipeable-row.tsx` | Drag-follows-finger + spring snap-back | Not guarded — this is 1:1 gesture tracking (WCAG 2.3.3 exempts animation that is a direct, essential result of user interaction). Snap-back uses `stackSpring` (settles quickly). |
| `navigation/use-edge-swipe-back.ts` | Edge-swipe-back gesture + spring | Same as above — direct-manipulation gesture, exempt. |
| `navigation/tab-bar.tsx` | 0.1s icon scale on tab select | Short, not guarded, judged acceptable (barely perceptible even at full speed). |
| `ui/offline-banner.tsx` | 0.2s height/opacity | Short, acceptable. |
| `ui/pressable.tsx` | 0.1s `whileTap` scale | Short, acceptable — also a direct-interaction (press) animation. |
| `ui/segmented-control.tsx` | Spring-driven sliding thumb (`layoutId`) | Not guarded — a `layoutId` shared-element transition; 0.3s spring, but tied to a direct user action (segment tap), not ambient. |
| `ui/top-bar.tsx` | Scroll-linked large-title collapse (`useTransform` off scroll position) | Not an independent timed animation — 1:1 with scroll position (equivalent to "instant," since it has no duration of its own); exempt. |
| `thread/message-bubble.tsx` | 0.22s `riseIn` entrance per bubble | Short, acceptable. |

**Full-flow PENDING-DEVICE check**: static analysis confirms every
animation is either guarded, short (<0.3s), or a direct result of user
input (gesture-tracking) — but `prefers-reduced-motion` needs to actually be
toggled at the OS level and each of the 5 screens + push/pop navigation +
the swipe gestures re-walked on a real device/emulator to confirm the media
query itself is honored inside the Tauri WebView (this is a reasonable
assumption, not something jsdom can verify).

- [ ] **PENDING-DEVICE: full-flow reduced-motion check.** Enable
  "Reduce Motion" (iOS: Settings → Accessibility → Motion; Android:
  Settings → Accessibility → Remove animations), then walk Login →
  Workspaces → Inbox → Conversation → You, triggering navigation, a
  conversation swipe-to-resolve, and the edge-swipe-back gesture. Confirm
  screen transitions crossfade (not slide) and nothing feels newly janky
  from the CSS clamp fighting a JS-driven spring.

---

## 4. Dynamic type (OS font scaling) — honest note

All type sizes in `src/index.css` (`text-large-title` through
`text-caption`) are defined in **`rem`**, which in principle scale with the
document root font-size. In practice, whether OS-level "larger text"
accessibility settings actually change that root font-size **depends
entirely on the native WebView**, and this app has done nothing (yet) to
wire that up explicitly:

- **Android** (Tauri uses the system WebView, Chromium-based): Android
  WebView has a `textZoom` setting (`WebSettings.setTextZoom()`) that some
  embedders tie to `Configuration.fontScale`; Tauri's Android shell does not
  currently set this explicitly (not found in `src-tauri/`), so the default
  WebView behavior applies — this is genuinely uncertain without a device,
  since defaults have shifted across Android/WebView versions.
- **iOS** (WKWebView): does **not** automatically honor the system Dynamic
  Type setting for arbitrary CSS `rem`/`px` sizing — Dynamic Type only
  propagates to native UIKit text and to a page's CSS if the page opts in
  via `-webkit-text-size-adjust` combined with Safari-specific Reader
  heuristics, neither of which this app uses. The realistic expectation is
  **no automatic scaling on iOS** without further work.

**V1.1 plan** (not built now — flagged honestly rather than silently
shipped as "done"): read the OS text-scale factor via a small Tauri command
(`UIApplication.shared.preferredContentSizeCategory` on iOS,
`Configuration.fontScale` on Android) at startup, and apply it as a
multiplier on a single `:root { --font-scale: 1.0 }` custom property that
every `@utility text-*` rule multiplies into its `font-size`. This is a
contained, one-PR-sized change once the toolchain exists to test it against
real OS settings — deliberately not attempted blind in this environment.

- [ ] **PENDING-DEVICE**: confirm the above assumptions against real
  Android/iOS OS font-scale settings once a device is available (see §2d).

---

## 5. Lifecycle / edge QA matrix (plan verbatim) — all PENDING-DEVICE

None of these can be exercised without hardware, real push delivery, or
airplane-mode control, which this environment has none of. Each row below
is the plan's table (verbatim) plus exact execution notes.

| Scenario | Expected | Execution notes |
|---|---|---|
| Kill app → push tap | Opens to conversation after auth restore | Requires Task 19a's push spike findings + a real device (push doesn't reach simulators/emulators reliably for APNs — see README item 9). Force-quit the app, send a push from the Firebase console, tap the notification, confirm `onPushTapped`'s cold-start path (`take_pending_tap` drain, per plugin README) fires exactly once and lands on the right conversation after `bootstrapAuth()` resolves (`main.tsx`'s `flushPendingTap` queue). |
| Token expiry while backgrounded overnight | Silent refresh, no logout | Background the app with a session close to expiry (or shorten the token TTL in a test env), leave backgrounded 8+ hours (or fast-forward via a debug endpoint), foreground, confirm `setupVisibilityRefresh`/`startTokenRefreshTimer` (`main.tsx`) silently refreshes rather than bouncing to `/login`. |
| Airplane mode mid-send | Failed bubble + retry chip; retry succeeds on reconnect | Start a send, enable airplane mode before it resolves, confirm the composer's `attemptSend` catch path fires (`src/thread/composer.tsx`) — failed-send chip renders, tapping Retry after disabling airplane mode re-attempts via the same `attemptSend(retryId)` path and clears the chip on success. |
| Workspace with 0 conversations | Inbox-zero empty state | Point at a workspace with no support conversations; confirm `InboxScreen`'s `showEmpty` branch renders "Inbox zero" (already code-verified to exist — `src/screens/inbox-screen.tsx`); device run confirms no false-positive skeleton flash first. |
| 10k-message thread | Opens without hang (else virtualize — noted in Task 13) | See §1b — the one item here most likely to need follow-up work (thread virtualization) rather than just confirmation. |
| Rapid conversation switching | No stale thread flash (query cache keyed correctly) | Rapidly tap between 3+ conversations; confirm each `ConversationScreen` mount (keyed by `conversationId` via the composer's `key={conversationId}` and TanStack Query's per-conversation cache keys) shows the right messages immediately with no flash of the previous thread's content. |
| RTL locale smoke test | Layout does not break (full RTL support is V1.1) | Force an RTL locale (device language → Arabic/Hebrew, or a debug override), walk all 5 screens, confirm nothing visually inverts incorrectly or overlaps — full RTL mirroring is explicitly out of scope (Part E), this is only a "doesn't break" smoke test. |
| Low-end Android (2GB RAM) | Usable; no OOM | Needs a real or closely-matched low-RAM device/emulator profile; watch `adb shell dumpsys meminfo <package>` while exercising Inbox scroll + conversation open + backgrounding/foregrounding a few times, confirm no OOM kill in logcat. |

---

## 6. Consolidated device-handoff checklist

This QA pass adds **zero new device-only items** beyond what's already
tracked — everything above either got fixed here (bundle split, the 4 a11y
gaps, the reduced-motion guard) or folds into the existing, more detailed
device checklist already in `apps/support-mobile/README.md` under
"Device handoff checklist," items **1–11** (native project init through
Task 21's deep-link on-device checks). See that file for the authoritative,
step-by-step instructions for those items — not duplicated here. This
document's §1b/§2d/§3/§5 PENDING-DEVICE items are the Task 22-specific
additions (performance thresholds, the accessibility screen-reader/font-
scale pass, the reduced-motion full-flow walk, and the lifecycle matrix) —
run them in the same device session as README items 1–11 once real
hardware/toolchain access exists.

---

## 7. Fixes applied this pass (summary)

1. `src/push/permission-priming-sheet.tsx` + `src/screens/you-screen.tsx`:
   `handleEnable` now catches a rejected `registerForPush()` (e.g. the
   native plugin's `getPushToken()` throwing) and shows the
   "Couldn't enable notifications" toast instead of leaking an unhandled
   promise rejection with no user feedback. New test:
   `src/push/__tests__/permission-priming-sheet.test.tsx`.
2. `src/inbox/conversation-cell.tsx`: composed `aria-label` for VoiceOver/
   TalkBack. Tests added.
3. `src/ui/offline-banner.tsx` + `src/screens/conversation-screen.tsx`:
   persistent `role="status" aria-live="polite"` regions. Offline-banner
   test added; new-message pill verified by inspection only (no existing
   render-test harness for `ConversationScreen`).
4. `src/thread/send-button.tsx`: per-state `aria-label`. Existing test
   updated.
5. `src/screens/workspaces-screen.tsx`: guarded the staggered entrance
   animation behind `useReducedMotion()`.
6. `src/router.tsx`: lazy-loaded the conversation route
   (`lazyRouteComponent`), −16KB gzip off the initial bundle.

## 8. Known test-coverage gaps (honest, not silently skipped)

- No render-level test exists for `ConversationScreen`'s new-message pill
  `aria-live` wiring, or for `you-screen.tsx`'s `handleEnable` catch path —
  both would require mocking the full router/store/query stack, which no
  existing screen test in this codebase does (screens are tested via pure
  helper-function extraction only — see `login-screen.test.ts`,
  `workspaces-screen.test.ts`). `permission-priming-sheet.test.tsx` covers
  the identical `registerForPush()`-rejects-→-toast contract in a component
  that IS practical to render standalone, and `you-screen.tsx`'s
  `handleEnable` is structurally identical (same try/catch/finally shape) —
  judged sufficient given the pattern match, but flagged here rather than
  claimed as tested.
