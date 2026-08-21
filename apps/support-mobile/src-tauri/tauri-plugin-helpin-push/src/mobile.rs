use serde::de::DeserializeOwned;
use tauri::{
    plugin::{PluginApi, PluginHandle},
    AppHandle, Runtime,
};

use crate::models::*;

// Must match the Kotlin package declared in
// `android/src/main/java/ai/helpin/mobile/plugin/push/PushPlugin.kt`.
#[cfg(target_os = "android")]
const PLUGIN_IDENTIFIER: &str = "ai.helpin.mobile.plugin.push";

// Must match the `@_cdecl` symbol exported by
// `ios/Sources/PushPlugin/PushPlugin.swift`.
#[cfg(target_os = "ios")]
tauri::ios_plugin_binding!(init_plugin_helpin_push);

// SPIKE-VERIFY: this whole file has never been compiled (no Rust toolchain
// in this environment) — the `register_android_plugin`/`register_ios_plugin`
// signatures and the `ios_plugin_binding!` macro name are reproduced from
// memory of the Tauri v2 mobile plugin convention (matching
// tauri-plugin-barcode-scanner / tauri-plugin-biometric shape); confirm
// against `cargo doc -p tauri --open` (`tauri::plugin::PluginApi`) on the
// first machine with a working toolchain.
pub fn init<R: Runtime, C: DeserializeOwned>(
    _app: &AppHandle<R>,
    api: PluginApi<R, C>,
) -> crate::Result<HelpinPush<R>> {
    #[cfg(target_os = "android")]
    let handle = api.register_android_plugin(PLUGIN_IDENTIFIER, "PushPlugin")?;
    #[cfg(target_os = "ios")]
    let handle = api.register_ios_plugin(init_plugin_helpin_push)?;
    Ok(HelpinPush(handle))
}

/// Mobile-target handle to the native plugin instance.
pub struct HelpinPush<R: Runtime>(PluginHandle<R>);

impl<R: Runtime> HelpinPush<R> {
    /// Blocks on the native `getPushToken` bridge call (Kotlin `@Command` /
    /// Swift `@objc` command). The native side itself performs the
    /// async FCM/APNs token fetch and permission prompt before responding.
    pub fn get_push_token(&self) -> crate::Result<GetPushTokenResponse> {
        self.0
            .run_mobile_plugin("getPushToken", ())
            .map_err(Into::into)
    }

    /// Returns-and-clears the natively buffered notification-tap payload
    /// (cold-start or not-yet-consumed warm tap). See
    /// `TakePendingTapResponse` for the pull-model rationale.
    pub fn take_pending_tap(&self) -> crate::Result<TakePendingTapResponse> {
        self.0
            .run_mobile_plugin("takePendingTap", ())
            .map_err(Into::into)
    }
}
