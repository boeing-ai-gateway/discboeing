use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    App, AppHandle, Manager, WindowEvent,
};

pub(crate) fn show_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        #[cfg(target_os = "macos")]
        {
            use tauri::ActivationPolicy;
            let _ = app.set_activation_policy(ActivationPolicy::Regular);
        }
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}

pub(crate) fn hide_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.hide();
        #[cfg(target_os = "macos")]
        {
            use tauri::ActivationPolicy;
            let _ = app.set_activation_policy(ActivationPolicy::Accessory);
        }
    }
}

fn toggle_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let is_visible = window.is_visible().unwrap_or(false);
        let is_focused = window.is_focused().unwrap_or(false);

        if is_visible && is_focused {
            hide_window(app);
        } else {
            show_window(app);
        }
    }
}

pub(crate) fn sync_macos_activation_policy(_app: &mut App) {
    #[cfg(target_os = "macos")]
    {
        use tauri::ActivationPolicy;
        if let Some(window) = _app.get_webview_window("main") {
            if window.is_visible().unwrap_or(false) {
                let _ = _app.set_activation_policy(ActivationPolicy::Regular);
            } else {
                let _ = _app.set_activation_policy(ActivationPolicy::Accessory);
            }
        } else {
            let _ = _app.set_activation_policy(ActivationPolicy::Accessory);
        }
    }
}

pub(crate) fn setup_tray(app: &App) -> tauri::Result<()> {
    let show_item = MenuItem::with_id(app, "show", "Show", true, None::<&str>)?;
    let quit_item = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&show_item, &quit_item])?;

    let tray_icon = tauri::image::Image::from_bytes(include_bytes!("../icons/tray-icon@2x.png"))?;
    TrayIconBuilder::new()
        .icon(tray_icon)
        .icon_as_template(true)
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id.as_ref() {
            "show" => show_window(app),
            "quit" => {
                app.exit(0);
            }
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                toggle_window(tray.app_handle());
            }
        })
        .build(app)?;

    Ok(())
}

pub(crate) fn handle_window_event(window: &tauri::Window, event: &WindowEvent) {
    if let WindowEvent::CloseRequested { api, .. } = event {
        hide_window(window.app_handle());
        api.prevent_close();
    }
}
