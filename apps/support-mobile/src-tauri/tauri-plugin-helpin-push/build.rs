const COMMANDS: &[&str] = &["get_push_token"];

fn main() {
    // Mirrors the build.rs shape used by every official Tauri v2 mobile
    // plugin (e.g. tauri-plugin-notification, tauri-plugin-haptics): the
    // `tauri-plugin` build-dependency generates the `mobile`/`desktop` cfg
    // flags used throughout `src/`, wires the Android Gradle module at
    // `android/` and the iOS SPM package at `ios/` into the app's generated
    // native projects, and emits the permission command allowlist consumed
    // by `permissions/`.
    //
    // SPIKE-VERIFY: this has never been run through `cargo build` in this
    // environment (no Rust toolchain here) — confirm it actually resolves
    // `android_path`/`ios_path` relative to this crate's own directory once
    // a machine with Rust + the mobile targets builds it for the first time.
    tauri_plugin::Builder::new(COMMANDS)
        .android_path("android")
        .ios_path("ios")
        .build();
}
