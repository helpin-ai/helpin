import FirebaseCore
import FirebaseMessaging
import Tauri
import UIKit
import UserNotifications
import WebKit

/// Bridges APNs/FCM push tokens and notification-tap payloads into the
/// Helpin Support webview.
///
/// SPIKE-VERIFY (whole file): never compiled — no Xcode/macOS in this
/// environment. Structure mirrors the `Tauri.Plugin` base class shape used
/// by official iOS mobile plugins (`@objc` command methods taking an
/// `Invoke`, `trigger(_:data:)` for native -> JS events, `load(webview:)`
/// override for setup). Confirm exact `Tauri` module API surface (method
/// names/signatures) against the real `Tauri` SPM package once
/// `pnpm tauri ios init` has run on macOS.
class PushPlugin: Plugin, MessagingDelegate, UNUserNotificationCenterDelegate {

    /// Cold-start tap payload, buffered until `load(webview:)` attaches the
    /// webview (mirrors the Android companion-object buffer in
    /// `PushPlugin.kt` — same reasoning: `didReceive response` can fire
    /// before the JS `onPushTapped` listener has registered).
    private static var bufferedTapPayload: [String: String]?

    override init() {
        super.init()
        // SPIKE-VERIFY: confirm `FirebaseApp.configure()` belongs here
        // (guarded so repeated `Plugin` instantiation across
        // window/scene reloads doesn't double-configure) versus requiring
        // the generated `gen/apple` AppDelegate to call it before Tauri's
        // plugin registration runs — Tauri iOS plugins don't get their own
        // AppDelegate lifecycle hook, so this may need to move to a manual
        // edit of `gen/apple/<App>/AppDelegate.swift` instead (documented
        // as a manual Xcode step in the plugin README either way, since
        // `GoogleService-Info.plist` also has to be added to the Xcode
        // project manually).
        if FirebaseApp.app() == nil {
            FirebaseApp.configure()
        }
        Messaging.messaging().delegate = self
        UNUserNotificationCenter.current().delegate = self
    }

    override func load(webview: WKWebView) {
        super.load(webview: webview)
        if let payload = PushPlugin.bufferedTapPayload {
            trigger("push-tapped", data: payload)
            PushPlugin.bufferedTapPayload = nil
        }
    }

    @objc public func getPushToken(_ invoke: Invoke) throws {
        UNUserNotificationCenter.current().requestAuthorization(
            options: [.alert, .sound, .badge]
        ) { _, _ in
            // Requesting authorization is what triggers the OS permission
            // prompt on first call, matching the guest-js contract
            // regardless of the grant/deny outcome (APNs registration and
            // FCM token issuance are not blocked by a denied notification
            // permission — only the ability to *display* a notification is).
            DispatchQueue.main.async {
                UIApplication.shared.registerForRemoteNotifications()
            }

            // SPIKE-VERIFY: `registerForRemoteNotifications()` is
            // asynchronous — the actual APNs device token arrives later via
            // `application(_:didRegisterForRemoteNotificationsWithDeviceToken:)`
            // on the host app's `AppDelegate`, which must forward it with
            // `Messaging.messaging().apnsToken = deviceToken`. This plugin
            // cannot intercept that AppDelegate callback itself, so a
            // manual edit to the generated `gen/apple` AppDelegate is
            // required (documented in this plugin's README) — until that
            // wiring exists, `Messaging.messaging().token` below may never
            // resolve on a real device.
            Messaging.messaging().token { token, error in
                let ret = JSObject()
                if let token = token, error == nil {
                    ret["token"] = token
                } else {
                    ret["token"] = nil
                }
                invoke.resolve(ret)
            }
        }
    }

    // MARK: - MessagingDelegate

    func messaging(_ messaging: Messaging, didReceiveRegistrationToken fcmToken: String?) {
        guard let token = fcmToken else { return }
        trigger("push-token-changed", data: ["token": token])
    }

    // MARK: - UNUserNotificationCenterDelegate

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let payload = Self.flattenUserInfo(response.notification.request.content.userInfo)
        // SPIKE-VERIFY: confirm this delegate callback fires for a
        // cold-start tap too (delivered once the app finishes launching),
        // and not only for warm taps — if cold-start delivery instead
        // requires reading
        // `launchOptions[.remoteNotification]` in the host AppDelegate's
        // `application(_:didFinishLaunchingWithOptions:)`, that value would
        // need forwarding into `PushPlugin.bufferedTapPayload` from there,
        // same as the APNs device token forwarding above.
        trigger("push-tapped", data: payload)
        completionHandler()
    }

    // Foreground presentation: let the OS default apply (no banner
    // override) — matches the Android side suppressing foreground display,
    // since in-app realtime already covers the foreground case.
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([])
    }

    private static func flattenUserInfo(_ userInfo: [AnyHashable: Any]) -> [String: String] {
        var result: [String: String] = [:]
        for (key, value) in userInfo {
            guard let stringKey = key as? String else { continue }
            if let stringValue = value as? String {
                result[stringKey] = stringValue
            } else {
                // SPIKE-VERIFY: confirm the backend never sends non-string
                // APNs custom payload values in practice; this fallback
                // exists only so the `Record<string, string>` guest-js
                // contract never gets a non-string, not because it's an
                // expected code path.
                result[stringKey] = "\(value)"
            }
        }
        return result
    }
}

/// C symbol Tauri's Rust `ios_plugin_binding!(init_plugin_helpin_push)`
/// macro (see `../../src/mobile.rs`) looks up to construct the plugin.
@_cdecl("init_plugin_helpin_push")
func initPlugin() -> Plugin {
    return PushPlugin()
}
