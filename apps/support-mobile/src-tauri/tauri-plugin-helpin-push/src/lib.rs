//! Tauri plugin bridging FCM (Android) / APNs (iOS) push tokens and tap
//! payloads into the Helpin Support mobile webview.
//!
//! Guest-js contract (see `guest-js/index.ts`, consumed by Task 20):
//! - `getPushToken(): Promise<string | null>`
//! - `onPushTokenChanged(cb): Promise<UnlistenFn>` — native `push-token-changed` event
//! - `onPushTapped(cb): Promise<UnlistenFn>` — native `push-tapped` event
//!
//! The two events are delivered natively via the Tauri mobile plugin's
//! `trigger()` channel (Kotlin/Swift call `trigger("push-token-changed", ...)`
//! / `trigger("push-tapped", ...)` directly) and are picked up on the JS side
//! by `addPluginListener('helpin-push', <event>, cb)` — this crate does not
//! re-emit them itself, mirroring how `tauri-plugin-notification` bridges its
//! `notification` / `actionPerformed` events.

use tauri::{
    plugin::{Builder, TauriPlugin},
    Manager, Runtime,
};

pub use models::*;

#[cfg(desktop)]
mod desktop;
#[cfg(mobile)]
mod mobile;

mod commands;
mod error;
mod models;

pub use error::{Error, Result};

#[cfg(desktop)]
use desktop::HelpinPush;
#[cfg(mobile)]
use mobile::HelpinPush;

/// Extension trait to access the `helpin-push` plugin state from any
/// `Manager` (`AppHandle`, `Window`, etc.), mirroring the accessor pattern
/// used by every official Tauri v2 plugin (e.g. `NotificationExt`).
pub trait HelpinPushExt<R: Runtime> {
    fn helpin_push(&self) -> &HelpinPush<R>;
}

impl<R: Runtime, T: Manager<R>> HelpinPushExt<R> for T {
    fn helpin_push(&self) -> &HelpinPush<R> {
        self.state::<HelpinPush<R>>().inner()
    }
}

/// Initializes the `helpin-push` plugin. Registered in the app's
/// `src/lib.rs` via `.plugin(tauri_plugin_helpin_push::init())`.
pub fn init<R: Runtime>() -> TauriPlugin<R> {
    Builder::new("helpin-push")
        .invoke_handler(tauri::generate_handler![commands::get_push_token])
        .setup(|app, api| {
            #[cfg(mobile)]
            let helpin_push = mobile::init(app, api)?;
            #[cfg(desktop)]
            let helpin_push = desktop::init(app, api)?;
            app.manage(helpin_push);
            Ok(())
        })
        .build()
}
