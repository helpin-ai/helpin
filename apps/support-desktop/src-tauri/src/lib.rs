use serde::Serialize;
use tauri::menu::{MenuBuilder, MenuItemBuilder, SubmenuBuilder};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder};
use tauri::{Emitter, Manager};

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
        .on_window_event(|window, event| {
            if window.label() != "main" {
                return;
            }

            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .plugin(tauri_plugin_notification::init())
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

            // ── System tray ──────────────────────────────────────
            let tray_open = MenuItemBuilder::with_id("tray_open", "Open Helpin Support")
                .build(app)?;
            let tray_quit = MenuItemBuilder::with_id("tray_quit", "Quit")
                .build(app)?;

            let tray_menu = MenuBuilder::new(app)
                .item(&tray_open)
                .separator()
                .item(&tray_quit)
                .build()?;

            TrayIconBuilder::new()
                .icon(app.default_window_icon().cloned().expect("app icon"))
                .tooltip("Helpin Support")
                .menu(&tray_menu)
                .on_menu_event(|app_handle, event| {
                    match event.id().0.as_str() {
                        "tray_open" => {
                            if let Some(w) = app_handle.get_webview_window("main") {
                                let _ = w.unminimize();
                                let _ = w.show();
                                let _ = w.set_focus();
                            }
                        }
                        "tray_quit" => {
                            app_handle.exit(0);
                        }
                        _ => {}
                    }
                })
                .on_tray_icon_event(|tray, event| {
                    if let tauri::tray::TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        let app_handle = tray.app_handle();
                        if let Some(w) = app_handle.get_webview_window("main") {
                            let _ = w.unminimize();
                            let _ = w.show();
                            let _ = w.set_focus();
                        }
                    }
                })
                .build(app)?;

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running Helpin Support desktop");
}
