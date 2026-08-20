# Helpin Support Mobile

Tauri 2 mobile shell for Helpin Support, targeting Android and iOS. Shares the
Vite/React frontend at `apps/support-mobile/src` with a thin native shell in
`src-tauri/`, mirroring the pattern established by `apps/support-desktop`.

## Stack

- Frontend: Vite 7 + React 19 + TypeScript, dev server on port `5176`
- Native shell: Tauri 2.9 (`src-tauri/`), plugins: `tauri-plugin-notification`, `tauri-plugin-store`, `tauri-plugin-deep-link`, `tauri-plugin-opener`
- Rust command: `mobile_shell_info()` returns `{ runtime, platform, app_version }` (mirrors desktop's `desktop_shell_info`)

## Environment prerequisites

Install these before attempting any of the dev-loop commands below:

- **Rust toolchain** via [rustup](https://rustup.rs), plus mobile targets:
  ```bash
  rustup target add aarch64-linux-android armv7-linux-androideabi i686-linux-android x86_64-linux-android
  rustup target add aarch64-apple-ios x86_64-apple-ios aarch64-apple-ios-sim
  ```
- **Android**: Android Studio with SDK + NDK installed. Set:
  ```bash
  export ANDROID_HOME="$HOME/Android/Sdk"
  export NDK_HOME="$ANDROID_HOME/ndk/<version>"
  ```
  Android SDK/NDK setup works on Linux, macOS, and Windows.
- **iOS**: Xcode 15+ with the iOS 15+ simulator runtime. iOS steps require macOS — they cannot run on Linux or Windows.
- **Node/pnpm**: already required by the rest of the monorepo (see root `CLAUDE.md`).

## Local API URL

The app talks to the Helpin API via `VITE_API_URL`. When running against a
locally hosted API:

- **Web / iOS simulator**: `VITE_API_URL=http://localhost:8080/api` (the iOS
  simulator shares the host's network namespace).
- **Android emulator**: `VITE_API_URL=http://10.0.2.2:8080/api` — the Android
  emulator maps `10.0.2.2` to the host machine's `localhost`; using
  `localhost` directly from the emulator will fail to connect.

Set this in `apps/support-mobile/.env.local` (create it if it doesn't exist).

Set `VITE_WEB_APP_URL` to the matching Helpin web application origin (for
example `https://stage.helpin.ai`). Linked PM tasks created from a support
conversation open at that origin in the system browser. When omitted, the
current page origin is used, which is suitable only when that origin also
serves the main Helpin web routes.

## Google and passkey sign-in

Google uses the existing backend callback configured by
`GOOGLE_AUTH_REDIRECT_URL`. The native app opens Google in the system browser;
the callback returns a five-minute, single-use code to
`helpin://auth/google`, and the app exchanges it for tokens without placing
access or refresh tokens in the URL. The hosted mobile build returns through
its HTTPS login page. Configure the API with:

```bash
MOBILE_APP_BASE_URL=https://azhar.dev.helpin.ai
CORS_ORIGINS=https://azhar.dev.helpin.ai,...
```

For passkeys in the DNS-hosted build, include that HTTPS origin in the
comma-separated `WEBAUTHN_RP_ORIGIN` setting while keeping a compatible
`WEBAUTHN_RP_ID` (for example, `helpin.ai` for Helpin subdomains). Native
webviews expose WebAuthn differently by OS/version, so the login screen checks
device support and disables the passkey button when the platform does not
provide it; Google and password login remain available.

## Dev loop

From `apps/support-mobile`:

```bash
pnpm install                # once, from repo root is also fine (pnpm workspaces)
pnpm tauri android dev      # boots the app on a connected device or running emulator
pnpm tauri ios dev          # boots the app on a running iOS simulator (macOS only)
```

Both commands run `pnpm dev` (the Vite dev server on port 5176) as
`beforeDevCommand` and load `http://localhost:5176` inside the native
webview, so edits to `src/` hot-reload the same way they do in the browser.

To produce release builds:

```bash
pnpm tauri android build
pnpm tauri ios build
```

## Device handoff checklist

This machine has no Rust toolchain, no Android SDK/NDK, and is Linux (no
Xcode), so the steps below could not be executed here. The `src-tauri/`
config and source files are complete and verbatim per spec; a machine with
the prerequisites above must run the following to finish mobile bring-up.

1. **Initialize native projects** (generates `src-tauri/gen/android` and
   `src-tauri/gen/apple`; these are checked into git, following the same
   pattern as `apps/support-desktop/src-tauri/gen`, and are not part of this
   commit since they can't be generated on this machine):
   ```bash
   cd apps/support-mobile
   pnpm tauri android init     # requires ANDROID_HOME + NDK
   pnpm tauri ios init         # macOS only
   ```
   Expected result: `src-tauri/gen/android/` and `src-tauri/gen/apple/`
   appear, each a native project (Gradle / Xcode) wrapping the Rust core.
   This also generates `src-tauri/gen/schemas/mobile-schema.json`, which
   `capabilities/default.json`'s `$schema` field points at — that reference
   is intentionally forward-looking until this step runs.

2. **Run on emulator/simulator**:
   ```bash
   pnpm tauri android dev   # expect: app boots on emulator showing "Helpin Support" centered
   pnpm tauri ios dev       # expect: same on iOS simulator
   ```
   Verify:
   - Text is visible and legible in both light and dark OS themes.
   - No white flash on boot (splash/background should match the app theme).
   - The status bar area is not overlapped incorrectly by app content (full
     safe-area handling arrives in a later task — this is a basic sanity
     check only, not final polish).

3. **Sanity-check the native command bridge**: from the webview devtools
   console (or a temporary button in the app), call
   `invoke('mobile_shell_info')` and confirm it resolves to
   `{ runtime: "tauri", platform: "android" | "ios", app_version: "0.1.0" }`.

4. **Commit the generated `gen/android` and `gen/apple` directories** once
   initialized (see point 1 above).

If `cargo check` in `src-tauri/` was not previously verified locally (no
network access to crates.io, or no Rust toolchain), run it once as a first
sanity pass before attempting `android init` / `ios init`:

```bash
cd apps/support-mobile/src-tauri
cargo check
```

Note: CI's `mobile-check` job (`.github/workflows/ci.yml`) runs this same
`cargo check` on every PR that touches the mobile app — but for the
**desktop (linux host) target only**; it does not exercise Android/iOS
`#[cfg(mobile)]` code paths, so the local check above is still worth running
before mobile `init`.

5. **Haptics on-device spot check** (`src/lib/haptics.ts`, backed by
   `tauri-plugin-haptics`): on a physical Android/iOS device, trigger each
   `HapticKind` (e.g. tap a `Pressable`/`SegmentedControl` with a `haptic`
   prop, or call `haptic(...)` from devtools) and confirm you feel the
   corresponding tap. Simulators/emulators don't vibrate — on those, just
   verify no crash/error occurs when `haptic()` fires.

6. **Navigation / ScreenStack device check** (`src/navigation/screen-stack.tsx`,
   `src/navigation/use-edge-swipe-back.ts`) — these need a real touchscreen or
   at minimum an emulator with touch input; jsdom cannot exercise gestures or
   the reduced-motion media query end-to-end:
   - Navigate placeholder Inbox → Conversation (temporary `Link` or
     `router.navigate`) and confirm the new screen slides in from the right
     with a spring feel (not linear/robotic), and the previous screen
     partially parallaxes left.
   - From the Conversation screen, swipe right starting within ~28px of the
     left edge and confirm it pops back to Inbox; confirm swipes starting
     mid-screen do NOT trigger back (edge-only gesture).
   - Confirm a vertical scroll started near the edge does not get hijacked by
     the horizontal gesture (intent-lock check — scroll the conversation list
     with a mostly-vertical touch starting at x < 28px).
   - Enable OS-level "Reduce Motion" (Settings > Accessibility on iOS/Android)
     and confirm push/pop transitions crossfade instead of sliding.
   - Cold-start directly on a conversation deep link (`/w/{slug}/support/{id}`,
     routine once Task 21 lands) with no prior in-app history, then trigger
     back — confirm it lands on the Inbox (`backFallbackPath`), not a dead
     end or a crash from `router.history.back()` on an empty stack.

7. **Login / workspace picker / You screen end-to-end check** (Task 9 —
   `src/screens/login-screen.tsx`, `workspaces-screen.tsx`, `you-screen.tsx`):
   point `VITE_API_URL` at staging or a local API (`http://10.0.2.2:8080/api`
   on the Android emulator; see "Local API URL" above), then on a real
   emulator/simulator:
   - Sign in with a real account/password; confirm field-level errors show
     for empty email, malformed email, and empty password before any request
     fires, and that a bad password shows a single error line above the
     button (no toast) without the card resizing.
   - Confirm the primary button shows an inline spinner while the request is
     in flight and the keyboard never covers the focused field (`TextField` /
     `--keyboard-inset`).
   - On success, land on the workspace picker; if the account has exactly one
     workspace (or a previously-picked one matches), confirm it skips
     straight to the Inbox instead of flashing the list.
   - Kill and relaunch the app; confirm it skips both login and the picker
     and returns straight to the last workspace's Inbox (persisted session +
     `last_workspace_slug`).
   - On the You tab: confirm the Appearance segmented control flips the app
     between light/dark immediately, the Workspace row navigates back to the
     picker, and Sign Out requires a second tap ("Tap again to confirm" for
     ~3s, no modal) before it actually signs out and redirects to Login.
   - Repeat the visual pass in both OS light and dark themes.

8. **Pull-to-refresh / swipeable-row on-device check** (Task 12 —
   `src/inbox/use-pull-to-refresh.ts`, `src/inbox/swipeable-row.tsx`,
   `src/screens/inbox-screen.tsx`) — jsdom only exercises the pure
   `swipeState`/`rubberBand` math; the actual gesture feel needs a real
   touchscreen:
   - At the very top of the Inbox, pull down slowly and confirm the spinner
     tracks the pull (rubber-banded, roughly half speed) and visibly rotates
     as you drag; past ~70px it should snap to a continuous spin with a light
     haptic tap ("armed"). Release before 70px and confirm it springs back
     with no refresh. Release past 70px and confirm conversations + unread
     counts refetch and the spinner clears once that settles.
   - Confirm pull-to-refresh does **not** engage when the list is scrolled
     down (only at scrollTop 0), including right after the large title has
     collapsed and you scroll back up to exactly the top.
   - On a conversation row, swipe right (leading) and left (trailing) and
     confirm the underlay tracks your finger 1:1 with no visible lag/jump,
     a light haptic at ~96px ("armed"), and that a full swipe past ~60% of
     the row's width commits before you even lift your finger.
   - Confirm a mostly-vertical drag on a row scrolls the list normally and
     never reveals the swipe underlay (10px horizontal-dominance lock).
   - Swipe to reveal one row, then start swiping a different row, and
     confirm the first one snaps closed (only one row open at a time).
   - In the "Mine" segment, swipe a row's Resolve action and confirm it
     fades/shrinks in place rather than abruptly vanishing, and that it
     disappears from "Mine" once the status update lands; toggle the
     leading Read/Unread action and confirm the dot and typography update to
     match without navigating away from the row.
   - Switch segments (Mine/Unassigned/All) and confirm the previous list
     stays visible (dimmed, not skeletons) while the new page loads, with
     skeletons appearing only on the very first load of the screen.

9. **Push plugin (Task 19 — `src-tauri/tauri-plugin-helpin-push/`)**: written
   pre-spike, entirely unverified — no Rust/Android/iOS toolchain exists in
   this environment. Full SPIKE-VERIFY list lives in
   `src-tauri/tauri-plugin-helpin-push/README.md`; do not proceed with any
   of the below until Task 19a's findings doc
   (`docs/superpowers/plans/2026-07-08-push-spike-findings.md`) exists and
   this plugin has been reconciled against it. Once a real toolchain and
   `gen/android`/`gen/apple` exist:
   - `cd apps/support-mobile/src-tauri && cargo check` — first sanity pass;
     confirm the crate compiles for desktop (`#[cfg(desktop)]` path) before
     attempting mobile targets.
   - Add `google-services.json` (Android) / `GoogleService-Info.plist`
     (iOS) from Doppler per Task 19a Step 1; both are gitignored.
   - Apply the `com.google.gms.google-services` Gradle plugin in
     `gen/android/app/build.gradle.kts` (the **application** module —
     deliberately not applied in the plugin's library module, where it is
     documented-ineffective), alongside where `google-services.json` lands
     (see plugin README item 10).
   - Manually enable Push Notifications + Background Modes
     (remote-notification) capabilities in the generated Xcode project, and
     wire `Messaging.messaging().apnsToken` in the generated AppDelegate's
     `didRegisterForRemoteNotificationsWithDeviceToken` — the plugin cannot
     do this itself (see plugin README item 13).
   - `getPushToken()` from devtools console (or a temporary button) on both
     a physical Android device and a physical iPhone (push does not reach
     simulators/emulators reliably for APNs; Android emulators with Play
     services do work) — confirm it resolves to a non-null token and that
     the OS permission prompt appears on first call.
   - Send a test message from the Firebase console: confirm background
     delivery on both platforms, tapping opens the app, and
     `onPushTapped`'s callback fires with the data payload on **both** a
     cold start (app was fully killed) and a warm tap (app already
     running), and fires exactly **once** per tap (the same payload can
     reach JS via both the live event and the `take_pending_tap` drain —
     guest-js dedupes). Cold start's remaining risk is whether the launch
     intent (Android) / `didReceive` (iOS) actually carries the data
     payload — the buffer/drain mechanism itself is deterministic (see
     plugin README items 4 and 14).
   - Confirm `onPushTokenChanged` fires when Firebase issues a fresh token
     (e.g. after clearing app data and relaunching).

10. **Push registration, priming, and tap-through routing (Task 20 —
    `src/push/push-registration.ts`, `src/push/permission-priming-sheet.tsx`,
    `src/main.tsx` tap wiring, `src/screens/you-screen.tsx` notifications
    row)**: `routePushTap`, `shouldShowPriming`, and `registerForPush`/
    `unregisterPush`'s request-shaping logic are unit-tested (mocked
    plugin/API), but the end-to-end device loop needs a real toolchain,
    real push delivery, and a real backend — none of which exist in this
    environment. Once Task 19's SPIKE-VERIFY items above are confirmed on
    device:
    - **First-run priming**: sign in fresh (or clear the app's prefs store)
      and land on Inbox; confirm the priming sheet appears once ("Never miss
      a customer" / bell icon), tapping "Enable notifications" triggers the
      real OS permission prompt (via `getPushToken()`), and after accepting,
      the You-screen Notifications row shows "Enabled". Relaunch the app and
      confirm the sheet does NOT reappear.
    - **"Not now" cooldown**: on a fresh prefs state, tap "Not now"; confirm
      the sheet does not reappear on subsequent Inbox visits/relaunches
      until 7 days have elapsed (or fast-forward by editing the persisted
      `push_priming` entry in the Tauri store file directly, since there's
      no dev-only clock override).
    - **You-screen manual enable**: with no priming decision yet made (or
      after "Not now"), open the You tab directly and confirm the
      Notifications row offers "Enable notifications"; tapping it triggers
      registration the same as the priming sheet and flips the row to
      "Enabled".
    - **Registration + backend row**: after enabling, confirm (via DB or an
      internal admin view) that a `push_devices` row exists for the signed-in
      user with the correct `platform` (`ios`/`android`), a non-empty
      `token`, and `app_version` matching the build.
    - **Full tap-through loop**: background the app (do not force-kill),
      have a customer reply to a conversation on staging, confirm the push
      notification arrives, tap it, and confirm the app opens directly on
      that conversation (not the Inbox) — then confirm the back gesture/
      button returns to the Inbox, not a dead end.
    - **Cold-start tap-through**: fully kill the app, trigger a customer
      reply push, tap it from a killed state, and confirm the app boots
      straight through to that conversation once auth bootstrap completes
      (not a flash of Inbox first, not a hang).
    - **Sign-out unregisters**: sign out, then confirm (via DB) the
      corresponding `push_devices` row is deleted; send another test push
      to the same device/token and confirm nothing arrives.
    - **Degraded payload**: if the backend ever sends a payload with only
      `conversation_id` (no `deep_link`, no `workspace_slug` — e.g. an older
      app version's queued notification), confirm tapping it does NOT crash
      or navigate anywhere (current documented behavior — see
      `routePushTap`'s doc comment for why a slugless payload is a no-op
      rather than a best-effort guess).

11. **OS-level deep links (Task 21 — `tauri-plugin-deep-link`,
    `src/main.tsx`'s `onOpenUrl` + `getCurrent()` wiring, `routeDeepLinkUrl`
    and `routeColdStartUrls` in `src/push/push-registration.ts`)**: config
    was added text-only (no Rust toolchain, no `gen/android`/`gen/apple` on
    this machine — see SPIKE-VERIFY notes below); the pure URL-parsing
    wrappers are unit-tested, but the OS→app handoff itself needs a real
    device/emulator and generated native projects. Warm opens arrive via
    `onOpenUrl`; cold starts (app launched BY the link) are picked up via a
    one-shot `getCurrent()` at startup, since the plugin's `onOpenUrl` only
    fires while the app is already running. Once `gen/android` and
    `gen/apple` exist (item 1 above) and the app is installed on a
    device/emulator/simulator:
    - **Android**:
      ```bash
      adb shell am start -a android.intent.action.VIEW -d "helpin://w/<slug>/support/<id>"
      ```
      Confirm this opens the app directly on that conversation (app backgrounded
      or fully killed — try both). Confirm a plain `https://` URL is NOT
      intercepted by this app (no `host`/App Link config was registered —
      `helpin://` custom scheme only, see `SPIKE-VERIFY` below).
    - **iOS**:
      ```bash
      xcrun simctl openurl booted "helpin://w/<slug>/support/<id>"
      ```
      Same expectations as Android: opens directly on the conversation, both
      backgrounded and cold-start.
    - **Malformed link**: try `helpin://w//support/` (empty slug) and confirm
      the app opens to whatever its default landing screen is (Inbox/Login)
      rather than crashing or showing a blank screen — `routeDeepLinkUrl`
      should no-op and let normal routing take over.
    - **Auth-gated queueing**: force-quit the app first (cold start), fire the
      `adb`/`xcrun` command above, and confirm the navigation waits for auth
      bootstrap and then either lands on the conversation (signed in) or is
      silently dropped (signed out) — same contract as a cold-start push tap,
      since both share `queueTapNavigation`/`flushPendingTap` in `main.tsx`.

    **SPIKE-VERIFY** (config written text-only against the Tauri v2
    deep-link plugin docs — https://v2.tauri.app/plugin/deep-linking/ — not
    exercised on a real build):
    - `tauri.conf.json`'s `plugins.deep-link.mobile` block uses the
      documented "Custom scheme on mobile (no server required)" shape
      (`{ "scheme": ["helpin"], "appLink": false }`, no `host`/`pathPrefix`).
      Confirm `pnpm tauri android init`/`ios init` + a build actually
      generates the Android intent-filter and iOS `CFBundleURLTypes` entries
      for the `helpin` scheme from this config once `gen/` exists.
    - `capabilities/default.json` adds `deep-link:default` but NOT an
      explicit `core:event:default` — the existing capability already has
      `core:default`, and `onPushTapped` (Task 19/20) successfully listens
      for plugin events under that same `core:default` today, so the
      assumption is it already covers `deep-link`'s `onOpenUrl` event too.
      Confirm this holds; add `core:event:default` explicitly if `onOpenUrl`
      doesn't fire on device.
    - No desktop `plugins.deep-link.desktop` block was added — this app
      targets Android/iOS only (see `README.md`'s Stack section), so desktop
      runtime registration (`app.deep_link().register(...)`) was intentionally
      left out per the plugin docs' guidance that iOS/Android must be
      config-registered (no dynamic runtime registration on those platforms
      anyway) and desktop isn't a shipped target for this app.

### Views drawer (on-device verification)

- [ ] Tap the menu (hamburger) button in the inbox top bar → drawer slides in from the left with groups Views / AI / Custom views / Team inboxes.
- [ ] Swipe in from the very left edge of the inbox → drawer opens (does not fight vertical pull-to-refresh or row swipe actions).
- [ ] Each row shows its unread count badge; counts match the web app for the same view.
- [ ] Select a view (e.g. Waiting) → drawer closes, top-bar title updates, and the conversation list shows the SAME conversations as that view on web (spot-check Inbox, Mine, Waiting, Resolved side by side with web).
- [ ] Select a team inbox and a custom view → list scopes correctly.
- [ ] Active view is highlighted in the drawer; scrim tap and left-swipe both dismiss it.
- [ ] With OS "Reduce Motion" on, the drawer crossfades instead of sliding.
