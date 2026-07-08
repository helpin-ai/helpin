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

/// Payload emitted on the `push-tapped` event — the raw FCM/APNs data
/// payload as flat string key/value pairs, matching the guest-js contract
/// `Record<string, string>`.
///
/// SPIKE-VERIFY: confirm neither platform ever needs to surface non-string
/// values here (FCM data payloads are documented as string-only; APNs
/// `userInfo` can technically carry nested/non-string values sent via a
/// custom aps dictionary — the Swift side flattens with `"\(value)"` as a
/// fallback, see `ios/Sources/PushPlugin/PushPlugin.swift`).
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PushTappedPayload(pub HashMap<String, String>);
