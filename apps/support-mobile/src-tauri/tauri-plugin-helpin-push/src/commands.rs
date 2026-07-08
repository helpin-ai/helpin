use tauri::{command, AppHandle, Runtime};

use crate::models::*;
use crate::HelpinPushExt;
use crate::Result;

#[command]
pub(crate) async fn get_push_token<R: Runtime>(
    app: AppHandle<R>,
) -> Result<GetPushTokenResponse> {
    app.helpin_push().get_push_token()
}
