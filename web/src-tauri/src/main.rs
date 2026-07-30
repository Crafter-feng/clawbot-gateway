use tauri::Manager;
use tauri_plugin_shell::ShellExt;
use tauri_plugin_shell::process::CommandEvent;

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            None,
        ))
        .setup(|app| {
            // 启动 Go sidecar
            let sidecar = app.shell().sidecar("clawbot-gateway")
                .expect("sidecar binary not found");

            let (rx, _child) = sidecar.spawn()
                .expect("failed to start gateway");

            // 监听 sidecar 输出
            tauri::async_runtime::spawn(async move {
                let mut rx = rx;
                while let Some(event) = rx.recv().await {
                    match event {
                        CommandEvent::Stdout(line) => {
                            println!("[gateway] {}", String::from_utf8_lossy(&line));
                        }
                        CommandEvent::Stderr(line) => {
                            eprintln!("[gateway] {}", String::from_utf8_lossy(&line));
                        }
                        _ => {}
                    }
                }
            });

            // 等待后端启动后加载管理页面
            let window = app.get_webview_window("main").unwrap();
            let _handle = app.handle().clone();
            std::thread::spawn(move || {
                std::thread::sleep(std::time::Duration::from_secs(2));
                let _ = window.eval("window.location.href = 'http://localhost:6798'");
            });

            Ok(())
        })
        .on_window_event(|window, event| {
            // 关闭窗口时隐藏到托盘，而非退出
            if let tauri::WindowEvent::CloseRequested { .. } = event {
                window.hide().unwrap();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}