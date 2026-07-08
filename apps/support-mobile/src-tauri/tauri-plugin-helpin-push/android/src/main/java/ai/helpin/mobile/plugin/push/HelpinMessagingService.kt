package ai.helpin.mobile.plugin.push

import android.util.Log
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
        // Intentionally not surfaced as an in-app JS event: foreground data
        // messages are suppressed per the task brief because in-app
        // realtime (WebSocket) already covers "new message arrived while
        // the app is open". Background/killed-state tray display is FCM's
        // own default behavior when the message includes a `notification`
        // block, so no manual NotificationCompat building happens here.
        //
        // SPIKE-VERIFY: confirm the backend (Task 18's FCM sender) always
        // includes a `notification` block (not a data-only message) so the
        // OS tray notification appears without any code in this method —
        // a data-only message would arrive here silently with nothing
        // shown to the user.
    }

    companion object {
        private const val TAG = "HelpinPush"
    }
}
