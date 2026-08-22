const COMMANDS: &[&str] = &["get_push_token", "take_pending_tap"];

fn main() {
    // Mirrors the build.rs shape used by every official Tauri v2 mobile
    // plugin (e.g. tauri-plugin-notification, tauri-plugin-haptics): the
    // `tauri-plugin` build-dependency generates the `mobile`/`desktop` cfg
    // flags used throughout `src/`, wires the Android Gradle module at
    // `android/` and the iOS SPM package at `ios/` into the app's generated
    // native projects, and emits the permission command allowlist consumed
    // by `permissions/`.
    //
    // `tauri ios dev` (unlike `tauri ios build`) does not propagate a
    // MACOSX_DEPLOYMENT_TARGET to plugin build scripts (tauri-apps/tauri
    // #4704, #14083), so the Swift package linking step (swift-rs's
    // SwiftLinker, invoked internally by `tauri_plugin::Builder::build()`)
    // falls back to an old default (10.13) for the macOS side of the
    // build. That conflicts with Firebase's `nanopb` dependency, which
    // requires macOS 10.15+, producing "library X requires macos 10.13,
    // but depends on product 'nanopb' which requires macos 10.15" errors
    // during the first real iOS Simulator build (Task 19a hardware spike).
    // Set an explicit floor here so it's available regardless of whether
    // the CLI passes one through.
    if std::env::var("MACOSX_DEPLOYMENT_TARGET").is_err() {
        std::env::set_var("MACOSX_DEPLOYMENT_TARGET", "10.15");
    }

    // SPIKE-VERIFY: this has never been run through `cargo build` in this
    // environment (no Rust toolchain here) — confirm it actually resolves
    // `android_path`/`ios_path` relative to this crate's own directory once
    // a machine with Rust + the mobile targets builds it for the first time.
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .ios_path("ios")
        .build();
}
