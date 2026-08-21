package ai.helpin.mobile.plugin.push

import android.Manifest
import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.webkit.WebView
import androidx.core.content.ContextCompat
import app.tauri.annotation.Command
import app.tauri.annotation.Permission
import app.tauri.annotation.PermissionCallback
import app.tauri.annotation.TauriPlugin
import app.tauri.plugin.Invoke
import app.tauri.plugin.JSObject
import app.tauri.plugin.Plugin
import com.google.firebase.messaging.FirebaseMessaging

/**
 * Bridges FCM push tokens and notification-tap payloads into the Helpin
 * Support webview.
 *
 * SPIKE-VERIFY (whole file): never compiled — no Android SDK/NDK/Rust
 * toolchain in this environment. Structure mirrors the `app.tauri.plugin.Plugin`
 * base class shape used by official mobile plugins (annotation-driven
 * `@Command` methods, `Invoke.resolve(JSObject)`, `trigger(event, data)` for
 * native -> JS events). Confirm the exact `app.tauri.plugin.*` /
 * `app.tauri.annotation.*` import paths and method signatures against the
 * generated `gen/android` project's `tauri-android` core module once
 * `pnpm tauri android init` has run on a real machine (see Task 19a
 * findings doc and the app README's device checklist).
 */
@TauriPlugin(
    permissions = [
        Permission(
            alias = "postNotifications",
            strings = [Manifest.permission.POST_NOTIFICATIONS],
        ),
    ],
)
class PushPlugin(private val activity: Activity) : Plugin(activity) {

    companion object {
        /**
         * The last unconsumed notification-tap payload. Written in `load()`
         * (cold start: launch intent carries the FCM data extras before any
         * JS listener exists) and in `onNewIntent` (warm tap that no
         * listener has consumed yet); returned-and-cleared by the
         * `takePendingTap` command. This is a deliberate pull model: the JS
         * side registers its live listener first, then drains this buffer
         * once, so cold-start delivery never races webview/listener
         * readiness. Static because `HelpinMessagingService` (an
         * OS-instantiated `FirebaseMessagingService`) has no direct handle
         * to this plugin instance — see `HelpinMessagingService.onNewToken`
         * for the same pattern applied to token refresh.
         *
         * SPIKE-VERIFY: confirm no stale payload leaks into a later,
         * unrelated launch — the launch intent (and thus its extras) can be
         * redelivered by the OS on activity recreation (e.g. rotation,
         * process restore), which would re-populate this buffer with an
         * already-handled tap. If that happens in practice, the fix is to
         * mark the intent consumed (e.g. `intent.removeExtra(...)` or an
         * `intent.identifier` check) rather than to change the pull model.
         */
        private var pendingTapPayload: JSObject? = null

        /** Live plugin instance, set in `load()`, used by the messaging
         * service to forward token-refresh events without a direct
         * dependency. Nullable: null whenever no activity/webview is
         * currently attached. */
        internal var instance: PushPlugin? = null
    }

    override fun load(webView: WebView) {
        super.load(webView)
        instance = this
        readTapPayloadFromIntent(activity.intent)?.let { payload ->
            // Cold start from a notification tap: buffer only. The JS
            // listener cannot have registered yet at load() time, so a
            // trigger() here would be lost; `takePendingTap` delivers it
            // deterministically once the JS side is ready.
            pendingTapPayload = payload
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        // Warm tap: activity already running, `onNewIntent` delivers the
        // fresh launcher intent with the notification's extras. Buffer AND
        // trigger (belt and braces): the live event gives instant delivery
        // when a listener is attached, the buffer covers a tap landing
        // before `onPushTapped` has finished registering. The JS side
        // dedupes if the same payload arrives through both paths.
        readTapPayloadFromIntent(intent)?.let { payload ->
            pendingTapPayload = payload
            trigger("push-tapped", payload)
        }
    }

    @Command
    fun takePendingTap(invoke: Invoke) {
        val ret = JSObject()
        ret.put("tap", pendingTapPayload)
        pendingTapPayload = null
        invoke.resolve(ret)
    }

    @Command
    fun getPushToken(invoke: Invoke) {
        // SPIKE-VERIFY: confirm this is the correct Tauri Android plugin API
        // for a runtime permission request triggered from a command handler
        // (as opposed to only from `load()`/activity lifecycle callbacks),
        // and that FCM's token is still obtainable even if the user denies
        // POST_NOTIFICATIONS (Android only gates *displaying* notifications
        // on that permission, not FCM registration/token issuance).
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(activity, Manifest.permission.POST_NOTIFICATIONS)
            != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissionForAlias(
                "postNotifications",
                invoke,
                "postNotificationsPermissionCallback",
            )
            // NOTE: requestPermissionForAlias is async; the actual token
            // fetch continues in postNotificationsPermissionCallback below
            // regardless of grant result (see comment above).
            return
        }
        fetchAndResolveToken(invoke)
    }

    @PermissionCallback
    private fun postNotificationsPermissionCallback(invoke: Invoke) {
        // Regardless of grant/deny result: FCM registration/token issuance
        // is not gated by POST_NOTIFICATIONS (that permission only gates
        // *displaying* a notification tray entry), so we always continue to
        // fetch the token. SPIKE-VERIFY this assumption on a real device.
        fetchAndResolveToken(invoke)
    }

    private fun fetchAndResolveToken(invoke: Invoke) {
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            val ret = JSObject()
            if (task.isSuccessful) {
                ret.put("token", task.result)
            } else {
                ret.put("token", null)
            }
            invoke.resolve(ret)
        }
    }

    /**
     * Reads the FCM notification-tap extras off the launcher/new intent.
     *
     * SPIKE-VERIFY: confirm FCM's notification-tap intent extras land as
     * flat string key/value pairs directly on the intent (documented
     * behavior for the default `PendingIntent` FCM/the OS builds for a
     * notification+data message), and that they aren't nested under a
     * reserved key that needs unwrapping first.
     */
    private fun readTapPayloadFromIntent(intent: Intent?): JSObject? {
        val extras = intent?.extras ?: return null
        val payload = JSObject()
        var found = false
        for (key in extras.keySet()) {
            val value = extras.get(key)
            if (value is String) {
                payload.put(key, value)
                found = true
            }
        }
        return if (found) payload else null
    }

    /**
     * Called by `HelpinMessagingService.onNewToken` (a separately
     * OS-instantiated component) to forward the refreshed token as a
     * `push-token-changed` event, since only a live `Plugin` instance can
     * call `trigger()`.
     */
    fun emitTokenChanged(token: String) {
        val payload = JSObject()
        payload.put("token", token)
        trigger("push-token-changed", payload)
    }

    /** Delivers a notification received while the app is foregrounded to JS. */
    fun emitPushReceived(payload: JSObject) {
        trigger("push-received", payload)
    }
}
