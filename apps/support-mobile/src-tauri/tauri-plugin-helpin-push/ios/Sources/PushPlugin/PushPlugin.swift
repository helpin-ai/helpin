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

    /// The last unconsumed notification-tap payload. Written by every
    /// `didReceive response` (cold-start and warm taps alike) and
    /// returned-and-cleared by the `takePendingTap` command. Deliberate
    /// pull model mirroring the Android buffer in `PushPlugin.kt`: the JS
    /// side registers its live listener first, then drains this buffer
    /// once, so a cold-start tap (which fires `didReceive` before the JS
    /// `onPushTapped` listener exists) is never lost to event timing.
    /// Static so a hypothetical plugin re-instantiation across webview
    /// reloads doesn't drop an unconsumed payload.
    static var bufferedTapPayload: [String: String]?

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

    @objc public func takePendingTap(_ invoke: Invoke) throws {
        var ret = JSObject()
        if let payload = PushPlugin.bufferedTapPayload {
            // `payload` is `[String: String]`; JSObject's values are `any
            // JSValue`, so a plain String-keyed/valued Dictionary doesn't
            // satisfy it directly even though String itself conforms to
            // JSValue — map it explicitly into a JSValue-valued dictionary.
            ret["tap"] = payload.mapValues { $0 as JSValue }
        } else {
            ret["tap"] = nil
        }
        PushPlugin.bufferedTapPayload = nil
        invoke.resolve(ret)
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
                var ret = JSObject()
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
        // `trigger` can throw; this delegate method's signature is fixed by
        // the MessagingDelegate protocol (not `throws`), and a failure to
        // emit this event isn't fatal (the JS side just misses one token
        // refresh), so discard the error rather than propagate it.
        try? trigger("push-token-changed", data: ["token": token])
    }

    // MARK: - UNUserNotificationCenterDelegate

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let payload = Self.flattenUserInfo(response.notification.request.content.userInfo)
        // Buffer AND trigger: the live event gives instant delivery to an
        // already-registered JS listener (warm tap); the buffer is drained
        // deterministically by `takePendingTap` once the JS side registers
        // (cold-start tap, where this delegate fires before the webview/JS
        // listener exists). The JS side dedupes if the same payload arrives
        // through both paths.
        PushPlugin.bufferedTapPayload = payload
        // Same throws/type-mismatch reasoning as the token-changed trigger
        // and takePendingTap above: `trigger` can throw from a non-throwing
        // delegate method (discard via `try?`), and `payload` (`[String:
        // String]`) needs mapping into a JSValue-valued dictionary.
        try? trigger("push-tapped", data: payload.mapValues { $0 as JSValue })
        // SPIKE-VERIFY: confirm this delegate callback fires for a
        // cold-start tap too (delivered once the app finishes launching),
        // and not only for warm taps — if cold-start delivery instead
        // requires reading
        // `launchOptions[.remoteNotification]` in the host AppDelegate's
        // `application(_:didFinishLaunchingWithOptions:)`, that value would
        // need forwarding into `PushPlugin.bufferedTapPayload` from there,
        // same as the APNs device token forwarding above.
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
