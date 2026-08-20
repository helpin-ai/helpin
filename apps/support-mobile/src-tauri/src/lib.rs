use serde::Serialize;

#[derive(Serialize)]
struct MobileShellInfo {
    runtime: &'static str,
    platform: &'static str,
    app_version: String,
}

#[tauri::command]
fn mobile_shell_info(app: tauri::AppHandle) -> MobileShellInfo {
    MobileShellInfo {
        runtime: "tauri",
        platform: std::env::consts::OS,
        app_version: app.package_info().version.to_string(),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_deep_link::init())
        .plugin(tauri_plugin_haptics::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_store::Builder::default().build())
        .plugin(tauri_plugin_helpin_push::init())
        .invoke_handler(tauri::generate_handler![mobile_shell_info])
        .run(tauri::generate_context!())
        .expect("error while running helpin support mobile");
}
