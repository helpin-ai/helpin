# App Icons & Splash — Helpin Support Mobile

## Status: blocked on design asset

No 1024px icon master exists yet (owned by design — see the plan's Part F,
item 4). **Do not generate placeholder icon binaries** and commit them; a
half-finished icon set that "just works" silently is worse than a build that
visibly has no icons yet, because it tends to ship by accident. This doc
records the exact command to run and the interim visual decision so bring-up
isn't blocked once the asset lands.

## When the master asset arrives

1. Get a **1024×1024px PNG** from design. Requirements:
   - Square, no transparency/alpha channel for the iOS variant (`pnpm tauri
     icon` derives the iOS set from the same master but Apple's App Store
     icon must be fully opaque — a master with alpha will produce an iOS
     icon with visible/undefined edge artifacts).
   - Content should have safe padding — Android adaptive icons crop
     the foreground layer into a circle/squircle/rounded-square depending on
     the launcher, so keep the important part of the mark within the inner
     ~66% of the canvas.
2. From `apps/support-mobile/`, run:
   ```bash
   pnpm tauri icon <path-to-1024px-master.png>
   ```
   This generates the full Android (`src-tauri/gen/android/app/src/main/res/mipmap-*`,
   adaptive icon foreground/background/monochrome layers) and iOS
   (`src-tauri/gen/apple/Assets.xcassets/AppIcon.appiconset`) icon sets in one
   pass. Requires `src-tauri/gen/android` and `src-tauri/gen/apple` to already
   exist (`pnpm tauri android init` / `pnpm tauri ios init` — see the
   "Device handoff checklist" in `README.md`, item 1), since the command
   writes into those generated project trees.
3. Commit the generated assets alongside the `gen/` directories (they're
   checked into git, matching `apps/support-desktop/src-tauri/gen`).

## Interim decision (documented now, applied once `gen/` exists)

Until the design asset lands, the agreed placeholder concept for anyone who
ends up needing *something* on-device sooner is:

- **Mark**: the Helpin logomark (wordmark-free, icon-only version), centered.
- **Background**: solid brand-primary `#3e63dd`, no gradient.
- **Shape**: plain rounded square at the OS-default corner radius — no
  custom shape mask; let Android/iOS apply their own icon masking (adaptive
  icon squircle on Android, superellipse on iOS) rather than baking in a
  shape that fights the OS's own mask.

This is explicitly a placeholder, not a design decision to keep — swap it
for the design-provided master via step 2 above as soon as it exists.

## iOS launch screen

Per the controller resolution for this task: the iOS launch screen (Task
19a's `gen/apple` storyboard) should be **plain background color only** —
`--background` set to the same brand-primary `#3e63dd`, no logo. A blank
brand-colored launch screen reads as fast (it's gone before anyone consciously
registers it); a logo splash reads as slow because it must render, then hold,
then animate away. This is a generation-time step (`pnpm tauri ios init`
scaffolds `LaunchScreen.storyboard`) — there is nothing to hand-author here
until `gen/apple` exists on a macOS machine with Xcode.

## Android adaptive icon

`pnpm tauri icon` populates the adaptive icon's foreground/background layers
directly from the master (step 2). No manual XML editing should be needed
for the default case — only revisit `ic_launcher.xml` /
`ic_launcher_round.xml` under `gen/android/app/src/main/res/mipmap-anydpi-v26/`
by hand if the generated adaptive icon needs a monochrome variant tuned
separately from the default one (Android 13+ themed icons), which is out of
scope until the real master exists.
