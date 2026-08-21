package ai.helpin.mobile.plugin.push

import android.util.Log
import app.tauri.plugin.JSObject
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * Registered as the FCM service in `AndroidManifest.xml` (merged into the
 * generated app manifest via Gradle manifest merging — SPIKE-VERIFY there's
 * no conflicting `<service>` entry once `gen/android` exists).
 *
 * The OS instantiates this class directly (it is not a Tauri plugin and has
 * no `Invoke`/webview access), so token refresh is forwarded to the live
 * `PushPlugin` instance via `PushPlugin.instance`, mirroring the same
 * "companion-object bridge" pattern documented on that class.
 */
class HelpinMessagingService : FirebaseMessagingService() {

    override fun onNewToken(token: String) {
        super.onNewToken(token)
        Log.d(TAG, "FCM token refreshed")
        // SPIKE-VERIFY: if no PushPlugin instance is currently attached
        // (e.g. token refresh happens while the app is fully backgrounded
        // with the activity destroyed), this event is silently dropped
        // rather than buffered. Per the task brief only `push-tapped` needs
        // cold-start buffering (Task 20 reads the *current* token via
        // `getPushToken()` on launch instead of relying on having caught
        // every historical `push-token-changed` event) — confirm that
        // assumption holds once Task 20 is implemented.
        PushPlugin.instance?.emitTokenChanged(token)
    }

    override fun onMessageReceived(message: RemoteMessage) {
        super.onMessageReceived(message)
        // FCM calls this for a notification received while the app is in the
        // foreground. The OS does not draw its tray banner in that state, so
        // forward both notification content and routing data to the webview
        // for a Helpin-styled in-app banner. Background/killed-state display
        // remains FCM's default behavior because the backend includes a
        // `notification` block as well as `data`.
        val payload = JSObject()
        for ((key, value) in message.data) {
            payload.put(key, value)
        }
        message.notification?.title?.let { payload.put("title", it) }
        message.notification?.body?.let { payload.put("body", it) }
        PushPlugin.instance?.emitPushReceived(payload)
    }

    companion object {
        private const val TAG = "HelpinPush"
    }
}
