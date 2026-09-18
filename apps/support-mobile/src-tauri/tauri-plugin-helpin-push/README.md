# Helpin native push plugin

This guide is for contributors integrating native push into the Support mobile
app. The plugin returns Firebase Cloud Messaging (FCM) tokens on Android and
iOS, and forwards notification payloads to the webview. On iOS, Firebase also
needs the APNs device token forwarded by the host app. Desktop commands return
no token or pending tap.

## Implementation and verification status

Source reviewed on 2026-09-17. The Rust bridge, Kotlin/Swift implementations,
JavaScript bindings, app registration, and foreground notification handling are
present. This is not proof of successful device delivery or a TestFlight release.
The original hardware-spike checklist remains relevant for native acceptance,
but its claims that nothing had ever been compiled and that registration was
future work are obsolete. The current Swift package records a prior build issue
and uses Firebase iOS SDK `from: "12.0.0"`, not the original `10.29.0` value.

Use [the TestFlight guide](../../TESTFLIGHT.md) for the release workflow.
Record actual device/build results when validating the checklist below; do not
infer acceptance from the existence of source files or mocked tests.

## JavaScript API

The app imports these source bindings through the `@helpin/plugin-push` alias
in its Vite and TypeScript configuration. This is not a published npm package.

```ts
import {
  getPushToken,
  onPushTokenChanged,
  onPushTapped,
  onPushReceived,
} from '@helpin/plugin-push'

const token = await getPushToken() // string | null
const stopToken = await onPushTokenChanged((token) => { /* register token */ })
const stopTap = await onPushTapped((data) => { /* navigate using routing data */ })
const stopReceived = await onPushReceived((data) => { /* show an in-app notice */ })
// Each stop function unregisters its listener when the consumer is disposed.
```

`getPushToken()` can prompt for notification permission. The app's
[registration code](../../src/push/push-registration.ts) reads the current
token and registers it with the backend; it also subscribes to token rotation.
The app's [entry point](../../src/main.tsx) subscribes to notification taps
and foreground messages. The [bindings](guest-js/index.ts) define the exact API.

On Android, token refresh events are forwarded only while a plugin instance is
attached. Reading the current token during registration avoids depending solely
on historical refresh events. Foreground messages forward both notification
text and routing data to the webview. Background display depends on the native
platform and notification payload; validate it on devices.

## Tap buffering

The native code stores the latest tap payload. `onPushTapped` registers the live
listener first, then invokes `take_pending_tap` to read and clear that buffer.
Identical payloads received within three seconds are deduplicated using sorted
key/value entries. This is a single pending payload, not a durable queue of taps.

The buffering mechanism does not establish that every native launch callback
fires as expected. Confirm Android launch-intent/new-intent payloads and iOS
terminated-state notification callbacks on physical devices.

## Native integration checklist

- Build each target and verify Tauri command signatures, plugin registration,
  generated permissions, and Android manifest merging. The `android/` and `ios/`
  directories are wired through [build.rs](build.rs).
- Initialize native projects with `pnpm tauri android init` or
  `pnpm tauri ios init` from `apps/support-mobile`. Generated project presence
  in a local checkout is not evidence of release success.
- Configure Firebase for the application. On Android, place
  `google-services.json` in the generated app module and apply the Google
  services Gradle plugin there, not in this plugin's library module.
- On iOS, include `GoogleService-Info.plist` in the app bundle. The Swift plugin
  returns no token when that file is absent. Confirm the host AppDelegate
  forwards `didRegisterForRemoteNotificationsWithDeviceToken` to
  `Messaging.messaging().apnsToken`.
- Enable Push Notifications and the remote-notification background mode in the
  generated iOS project. Confirm signing capabilities and entitlements.
- Validate the local Tauri Swift package path after generation. Current build
  declarations are in [Package.swift](ios/Package.swift) and
  [build.gradle.kts](android/build.gradle.kts): Firebase iOS `from: "12.0.0"`,
  Android Firebase BOM `33.5.1`, Android compile SDK 34 and minimum SDK 26.
  These declarations do not establish tested device compatibility.
- Test permission approval/denial, fresh token retrieval, rotation, logout,
  foreground notices, background taps, terminated-state taps, and activity
  recreation. Confirm that native redelivery does not repeat navigation.
- Test with the backend's actual notification and routing payload. JavaScript
  mocks cannot prove APNs/FCM delivery, OS tray behavior, or platform lifecycle
  handling.

Keep Firebase configuration and signing credentials outside committed source.
See the app's [QA checklist](../../QA.md) for broader device acceptance.
