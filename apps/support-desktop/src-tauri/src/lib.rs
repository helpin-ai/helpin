use serde::Serialize;
use tauri::menu::{MenuBuilder, MenuItemBuilder, SubmenuBuilder};
use tauri::Emitter;

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
        .setup(|app| {
            let toggle_theme = MenuItemBuilder::with_id("toggle_theme", "Toggle Dark Mode")
                .accelerator("CmdOrCtrl+Shift+D")
                .build(app)?;

            let light_mode = MenuItemBuilder::with_id("light_mode", "Light")
                .build(app)?;
            let dark_mode = MenuItemBuilder::with_id("dark_mode", "Dark")
                .build(app)?;
            let system_mode = MenuItemBuilder::with_id("system_mode", "System")
                .build(app)?;

            let view_menu = SubmenuBuilder::new(app, "View")
                .item(&toggle_theme)
                .separator()
                .item(&light_mode)
                .item(&dark_mode)
                .item(&system_mode)
                .build()?;

            let edit_menu = SubmenuBuilder::new(app, "Edit")
                .undo()
                .redo()
                .separator()
                .cut()
                .copy()
                .paste()
                .select_all()
                .build()?;

            let menu = MenuBuilder::new(app)
                .item(&edit_menu)
                .item(&view_menu)
                .build()?;

            app.set_menu(menu)?;

            app.on_menu_event(move |app_handle, event| {
                let id = event.id().0.as_str();
                match id {
                    "toggle_theme" => {
                        let _ = app_handle.emit("theme-change", "toggle");
                    }
                    "light_mode" => {
                        let _ = app_handle.emit("theme-change", "light");
                    }
                    "dark_mode" => {
                        let _ = app_handle.emit("theme-change", "dark");
                    }
                    "system_mode" => {
                        let _ = app_handle.emit("theme-change", "system");
                    }
                    _ => {}
                }
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running Helpin Support desktop");
}
