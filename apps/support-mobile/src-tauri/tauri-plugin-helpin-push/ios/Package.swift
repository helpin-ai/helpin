// swift-tools-version:5.7
import PackageDescription

// Mirrors the SPM package shape used by official Tauri v2 iOS mobile plugins
// (a `Tauri` local package dependency resolved relative to the generated
// `gen/apple` project, plus one external SPM dependency here — Firebase).
//
// firebase-ios-sdk pin history: originally pinned to `from: "10.29.0"`
// (never resolved by `swift build`/Xcode in this environment — Linux, no
// Xcode). First real build on macOS (Task 19a hardware spike) failed with:
//   "the library 'FirebaseAnalyticsWithoutAdIdSupportWrapper' requires
//   macos 10.13, but depends on the product 'nanopb' which requires
//   macos 10.15" (and similarly for several other Firebase-internal
//   libraries) — an internal platform-floor inconsistency within that old
//   firebase-ios-sdk release's own dependency graph. Confirmed by reading
//   firebase-ios-sdk's own Package.swift at the current latest release
//   (12.16.0, as of 2026-07): its root `platforms:` array now consistently
//   declares `.macOS(.v10_15)` (matching nanopb) and `.iOS(.v15)`. Bumped
//   the pin to the 12.x line to pick up that fix, and raised this
//   package's own iOS floor to match (also matches this app's
//   `tauri.conf.json` `minimumSystemVersion: "15.0"`, so no new
//   constraint is introduced).
let package = Package(
    name: "tauri-plugin-helpin-push",
    platforms: [.iOS(.v15)],
    products: [
        .library(
            name: "tauri-plugin-helpin-push",
            type: .static,
            targets: ["tauri-plugin-helpin-push"]
        )
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api"),
        .package(url: "https://github.com/firebase/firebase-ios-sdk", from: "12.0.0"),
    ],
    targets: [
        .target(
            name: "tauri-plugin-helpin-push",
            dependencies: [
                .byName(name: "Tauri"),
                .product(name: "FirebaseMessaging", package: "firebase-ios-sdk"),
            ],
            path: "Sources/PushPlugin"
        )
    ]
)
