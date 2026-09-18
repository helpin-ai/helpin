# Mobile icons and launch screens

Use this guide when updating the support app's brand assets. The repository now
contains an [SVG source](src-tauri/icons/helpin-app-icon.svg), desktop-size icons,
and generated Android/iOS icon sets under [src-tauri/icons](src-tauri/icons/).
The earlier statement that no icon asset exists is obsolete. Asset presence does
not establish App Store approval or visual verification on a device.

## Update icons

From `apps/support-mobile`, regenerate the set from the source asset:

```sh
pnpm tauri icon src-tauri/icons/helpin-app-icon.svg
```

Review the generated diff and confirm that the iOS store icon is opaque and the
Android adaptive icon retains legible padding. Keep generated platform assets
consistent with the source; do not hand-edit one size without regenerating the
others. Check the actual Android/iOS build output before publishing.

Native projects are generated under `src-tauri/gen` when initialized. They are
absent from this checkout; the TestFlight workflow initializes the Apple project
when needed. See [native setup](README.md#device-handoff-checklist) and the
[TestFlight guide](TESTFLIGHT.md) for the release process.

## Launch screen verification

The original design called for a plain brand-colored launch screen without a
logo. Treat that as design intent until verified in the generated native project.
Check launch, light/dark appearance, and safe-area transitions on a device;
checked-in icon assets alone do not verify the splash screen.
