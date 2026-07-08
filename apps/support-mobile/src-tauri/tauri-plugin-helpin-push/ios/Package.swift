// swift-tools-version:5.7
import PackageDescription

// SPIKE-VERIFY (whole file): mirrors the SPM package shape used by official
// Tauri v2 iOS mobile plugins (a `Tauri` local package dependency resolved
// relative to the generated `gen/apple` project, plus one external SPM
// dependency here — Firebase). Never resolved by `swift build`/Xcode in
// this environment (Linux, no Xcode). Confirm once `pnpm tauri ios init`
// has run on macOS:
// - the relative path to the `Tauri` package matches what the generator
//   actually lays out under `gen/apple/`
// - the firebase-ios-sdk version pin matches Task 19a's findings doc
// - `.iOS(.v13)` is sufficient for the FirebaseMessaging version pinned, or
//   needs bumping to v14/v15 (this app's `tauri.conf.json` already targets
//   `minimumSystemVersion: "15.0"`, so `.v13` here is conservative and
//   should not be the binding constraint)
let package = Package(
    name: "tauri-plugin-helpin-push",
    platforms: [.iOS(.v13)],
    products: [
        .library(
            name: "tauri-plugin-helpin-push",
            type: .static,
            targets: ["tauri-plugin-helpin-push"]
        )
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api"),
        .package(url: "https://github.com/firebase/firebase-ios-sdk", from: "10.29.0"),
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
