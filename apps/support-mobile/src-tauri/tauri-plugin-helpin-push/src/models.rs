use std::collections::HashMap;

use serde::{Deserialize, Serialize};

/// Response for the `get_push_token` command.
///
/// `token` is `None` on desktop (no-op target), and also `None` on mobile if
/// the OS notification permission was denied or the FCM/APNs handshake
/// hasn't completed yet.
///
/// SPIKE-VERIFY: confirm the native side actually resolves this promptly
/// (FCM token retrieval is asynchronous on both platforms) rather than
/// blocking the JS caller indefinitely — findings doc should record observed
/// latency.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GetPushTokenResponse {
    pub token: Option<String>,
}

/// Payload emitted on the `push-token-changed` event.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TokenChangedPayload {
    pub token: String,
}

/// Response for the `take_pending_tap` command — the buffered
/// notification-tap data payload, if one is waiting.
///
/// `tap` is the raw FCM/APNs data payload as flat string key/value pairs
/// (matching the guest-js contract `Record<string, string>`). It is `None`
/// on desktop (no-op target) and on mobile when no unconsumed tap is
/// buffered. The native side returns-and-clears: a second call after a
/// successful take yields `None` again. This pull model makes cold-start
/// tap delivery deterministic — the JS side registers its live listener
/// first, then drains this buffer once, instead of racing native event
/// emission against webview/listener readiness.
///
/// SPIKE-VERIFY: confirm neither platform ever needs to surface non-string
/// values here (FCM data payloads are documented as string-only; APNs
/// `userInfo` can technically carry nested/non-string values sent via a
/// custom aps dictionary — the Swift side flattens with `"\(value)"` as a
/// fallback, see `ios/Sources/PushPlugin/PushPlugin.swift`).
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TakePendingTapResponse {
    pub tap: Option<HashMap<String, String>>,
}
