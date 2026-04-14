use serde::Serialize;

#[derive(Serialize)]
struct DesktopShellInfo {
    runtime: &'static str,
    platform: &'static str,
    app_version: String,
}

#[tauri::command]
fn desktop_shell_info(app: tauri::AppHandle) -> DesktopShellInfo {
    DesktopShellInfo {
        runtime: "tauri",
        platform: std::env::consts::OS,
        app_version: app.package_info().version.to_string(),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_store::Builder::default().build())
        .invoke_handler(tauri::generate_handler![desktop_shell_info])
        .run(tauri::generate_context!())
        .expect("error while running Helpin Support desktop");
}
