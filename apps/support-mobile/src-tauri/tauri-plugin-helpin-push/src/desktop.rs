use serde::de::DeserializeOwned;
use tauri::{plugin::PluginApi, AppHandle, Runtime};

use crate::models::*;

/// Desktop targets have no FCM/APNs bridge — every API is a deliberate
/// no-op so the app shell (`src-tauri/`) keeps compiling and running for
/// `pnpm tauri dev` on Linux/macOS/Windows dev machines, per the task brief
/// ("the plugin must compile for desktop too since src-tauri builds for
/// dev").
pub fn init<R: Runtime, C: DeserializeOwned>(
    app: &AppHandle<R>,
    _api: PluginApi<R, C>,
) -> crate::Result<HelpinPush<R>> {
    Ok(HelpinPush(app.clone()))
}

pub struct HelpinPush<R: Runtime>(AppHandle<R>);

impl<R: Runtime> HelpinPush<R> {
    pub fn get_push_token(&self) -> crate::Result<GetPushTokenResponse> {
        Ok(GetPushTokenResponse { token: None })
    }

    pub fn take_pending_tap(&self) -> crate::Result<TakePendingTapResponse> {
        Ok(TakePendingTapResponse { tap: None })
    }
}
