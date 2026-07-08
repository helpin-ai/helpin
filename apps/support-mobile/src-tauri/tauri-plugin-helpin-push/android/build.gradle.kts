// SPIKE-VERIFY (whole file): mirrors the shape of an official Tauri v2
// mobile plugin's `android/build.gradle.kts` (Android library module,
// namespace matching the Kotlin package used as `PLUGIN_IDENTIFIER` in
// `../src/mobile.rs`). Never run through Gradle in this environment (no
// Android SDK/NDK here). Once `pnpm tauri android init` generates
// `src-tauri/gen/android`, confirm:
// - the generated `gen/android/settings.gradle.kts` includes this module
//   and the `tauri-android` core library dependency it needs is resolvable
//   the same way the app's own `gen/android/app` module resolves it
// - the exact `compileSdk`/`minSdk` here match `tauri.conf.json`'s
//   `bundle.android.minSdkVersion` (26) and whatever `compileSdk` the
//   generated app module uses
// - the Firebase BOM/messaging version pins below match whatever Task 19a's
//   findings doc records as tested-working on real hardware

plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    id("com.google.gms.google-services")
}

android {
    namespace = "ai.helpin.mobile.plugin.push"
    compileSdk = 34

    defaultConfig {
        minSdk = 26
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
}

dependencies {
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.webkit:webkit:1.12.1")
    implementation(platform("com.google.firebase:firebase-bom:33.5.1"))
    implementation("com.google.firebase:firebase-messaging")
}
