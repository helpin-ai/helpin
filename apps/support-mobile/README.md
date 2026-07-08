# Helpin Support Mobile

Tauri 2 mobile shell for Helpin Support, targeting Android and iOS. Shares the
Vite/React frontend at `apps/support-mobile/src` with a thin native shell in
`src-tauri/`, mirroring the pattern established by `apps/support-desktop`.

## Stack

- Frontend: Vite 7 + React 19 + TypeScript, dev server on port `5176`
- Native shell: Tauri 2.9 (`src-tauri/`), plugins: `tauri-plugin-notification`, `tauri-plugin-store`
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

If `cargo check` in `src-tauri/` was not previously verified (no network
access to crates.io, or no Rust toolchain), run it once as a first sanity
pass before attempting `android init` / `ios init`:

```bash
cd apps/support-mobile/src-tauri
cargo check
```

5. **Haptics on-device spot check** (`src/lib/haptics.ts`, backed by
   `tauri-plugin-haptics`): on a physical Android/iOS device, trigger each
   `HapticKind` (e.g. tap a `Pressable`/`SegmentedControl` with a `haptic`
   prop, or call `haptic(...)` from devtools) and confirm you feel the
   corresponding tap. Simulators/emulators don't vibrate — on those, just
   verify no crash/error occurs when `haptic()` fires.
