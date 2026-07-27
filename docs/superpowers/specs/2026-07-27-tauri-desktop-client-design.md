# 方案 B：Tauri 桌面客户端

## 概述

将现有 ClawBot Gateway 的 Web 管理界面包装为原生桌面应用。Go 后端作为 sidecar 进程运行，Tauri 提供原生窗口壳。

## 架构

```
┌─────────────────────────────────────────────────────┐
│                  Tauri 窗口                          │
│  ┌───────────────────────────────────────────────┐  │
│  │          WebView（系统原生渲染引擎）              │  │
│  │                                                │  │
│  │   ┌─────────────────────────────────────────┐  │  │
│  │   │       React SPA（web/dist/）              │  │  │
│  │   │   - 管理页面                              │  │  │
│  │   │   - 设置页（含密码修改）                    │  │  │
│  │   │   - 仪表盘/账号/路由                       │  │  │
│  │   │   - 通知 Token 管理                        │  │  │
│  │   └─────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────┘  │
│                                                      │
│  ┌───────────────────────────────────────────────┐  │
│  │     Go Gateway（sidecar 子进程）                │  │
│  │     - HTTP 服务 localhost:8080                  │  │
│  │     - iLink 客户端（连接腾讯）                   │  │
│  │     - iLink 服务端（透明代理）                   │  │
│  │     - SQLite 数据库（data/clawbot.db）           │  │
│  │     - 路由引擎 / 适配器工厂                      │  │
│  └───────────────────────────────────────────────┘  │
│                                                      │
│  ┌───────────────────────────────────────────────┐  │
│  │     Rust 壳（Tauri 2.0）                       │  │
│  │     - 窗口创建/管理                             │  │
│  │     - 系统托盘（后台运行/退出）                  │  │
│  │     - sidecar 生命周期管理                      │  │
│  │     - 自动更新（tauri-plugin-updater）          │  │
│  │     - 开机自启（tauri-plugin-autostart）        │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### 通信方式

```
WebView (tauri://localhost)  ──fetch──>  Go sidecar (localhost:8080)
                                              │
                                              ├── iLink API (weixin.qq.com)
                                              ├── SQLite (data/clawbot.db)
                                              └── 虚拟 Bot 代理
```

- 前端通过 `fetch('http://localhost:8080/api/...')` 调用 Go 后端
- Tauri 的 `tauri-plugin-shell` 负责启动/停止 sidecar 进程
- 窗口关闭时自动 kill sidecar

## 与现有代码的兼容性

### 前端改动

| 文件 | 改动 | 原因 |
|------|------|------|
| `web/src/api/client.ts` | 加 `baseURL` 常量，Tauri 模式下为 `http://localhost:8080` | WebView 的 origin 是 `tauri://localhost`，相对路径无法路由到 Go 后端 |
| `web/vite.config.ts` | 加 `TAURI_DEV` 环境变量判断，dev 模式保留 proxy | Tauri dev 时 Vite 在 :1420 端口，需要 proxy 到 :8080 |
| `web/package.json` | 加 `tauri` 和 `tauri-build` scripts | Tauri CLI 命令 |

Api client 改动量极小：

```typescript
// 在 Tauri 生产模式下使用绝对 URL
const BASE_URL = (window as any).__TAURI__ ? 'http://localhost:8080' : ''

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, { ... })
}
```

### 后端改动

**零改动**。Go 后端完全不变，只多一个交叉编译步骤。

### 新文件

```
web/
├── src-tauri/                  # Tauri 项目（新增）
│   ├── Cargo.toml              # Rust 依赖
│   ├── tauri.conf.json         # 窗口配置、sidecar 注册
│   ├── build.rs                # Tauri 构建脚本
│   ├── capabilities/
│   │   └── default.json        # 权限声明
│   ├── icons/                  # 应用图标
│   │   ├── icon.png
│   │   ├── icon.ico
│   │   └── icon.icns
│   ├── binaries/               # Go sidecar 二进制（构建时放入）
│   │   └── .gitkeep
│   └── src/
│       └── main.rs             # Rust 入口
```

## 关键实现细节

### 1. Rust 主程序 (`main.rs`)

```rust
use tauri::Manager;

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

            let (rx, child) = sidecar.spawn()
                .expect("failed to start gateway");

            // 监听 sidecar 输出
            tauri::async_runtime::spawn(async move {
                while let Some(event) = rx.recv().await {
                    match event {
                        tauri_plugin_shell::process::CommandEvent::Stdout(line) => {
                            println!("[gateway] {}", String::from_utf8_lossy(&line));
                        }
                        tauri_plugin_shell::process::CommandEvent::Stderr(line) => {
                            eprintln!("[gateway] {}", String::from_utf8_lossy(&line));
                        }
                        _ => {}
                    }
                }
            });

            // 窗口加载完成后打开管理页面
            let window = app.get_webview_window("main").unwrap();
            let handle = app.handle().clone();
            std::thread::spawn(move || {
                std::thread::sleep(std::time::Duration::from_secs(2));
                let _ = window.eval("window.location.href = 'http://localhost:8080'");
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
```

### 2. Sidecar 配置 (`tauri.conf.json`)

```json
{
  "productName": "ClawBot Gateway",
  "version": "1.0.0",
  "identifier": "com.clawbot.gateway",
  "build": {
    "frontendDist": "../dist",
    "devUrl": "http://localhost:1420",
    "beforeDevCommand": "npm run dev",
    "beforeBuildCommand": "npm run build"
  },
  "app": {
    "windows": [
      {
        "title": "ClawBot 网关",
        "width": 1200,
        "height": 800,
        "minWidth": 900,
        "minHeight": 600,
        "center": true,
        "resizable": true
      }
    ],
    "security": {
      "csp": "default-src 'self'; connect-src 'self' http://localhost:8080; style-src 'self' 'unsafe-inline'"
    },
    "trayIcon": {
      "iconPath": "icons/icon.png",
      "iconAsTemplate": true
    }
  },
  "bundle": {
    "active": true,
    "targets": "all",
    "icon": [
      "icons/32x32.png",
      "icons/128x128.png",
      "icons/128x128@2x.png",
      "icons/icon.icns",
      "icons/icon.ico"
    ],
    "externalBin": ["binaries/clawbot-gateway"]
  },
  "plugins": {
    "shell": {
      "sidecar": true,
      "scope": [
        {
          "name": "binaries/clawbot-gateway",
          "sidecar": true
        }
      ]
    },
    "autostart": {
      "enabled": false
    }
  }
}
```

### 3. 权限声明 (`capabilities/default.json`)

```json
{
  "identifier": "default",
  "description": "Default capability",
  "windows": ["main"],
  "permissions": [
    "core:default",
    "shell:allow-open",
    "shell:allow-execute",
    "shell:allow-spawn",
    "autostart:default"
  ]
}
```

### 4. Vite 配置改动

```typescript
// web/vite.config.ts
export default defineConfig({
  // ... 现有配置 ...
  server: {
    port: 1420,  // Tauri 默认端口
    strictPort: true,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/auth': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
```

### 5. Api client 改动

```typescript
// web/src/api/client.ts
const BASE_URL = (window as any).__TAURI_INTERNALS__ ? 'http://localhost:8080' : ''

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { 'Authorization': `Bearer ${token}` } : {}) },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  // ... 其余不变 ...
}
```

## 构建流程

### 开发

```bash
# 1. 启动 Go 后端
cd clawbot-gateway
go run . &

# 2. 启动 Tauri 开发模式
cd web
npx tauri dev
```

Tauri 会启动 Vite dev server（:1420），自动打开窗口，Vite 将 API 请求 proxy 到 Go 后端（:8080）。

### 生产构建

```bash
# 1. 交叉编译 Go sidecar（三平台）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o web/src-tauri/binaries/clawbot-gateway-x86_64-pc-windows-msvc.exe .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o web/src-tauri/binaries/clawbot-gateway-aarch64-apple-darwin .
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o web/src-tauri/binaries/clawbot-gateway-x86_64-unknown-linux-gnu .

# 2. 构建 Tauri 安装包
cd web
npx tauri build
```

### 构建产物

| 平台 | 产物 | 大小 |
|------|------|------|
| Windows | `target/release/ClawBot Gateway.msi` + `.exe` | ~10MB |
| macOS | `target/release/ClawBot Gateway.dmg` + `.app` | ~8MB |
| Linux | `target/release/ClawBot Gateway.deb` + `.AppImage` | ~10MB |

### Sidecar 二进制命名规则

Tauri 要求 sidecar 按平台命名：

```
clawbot-gateway-x86_64-pc-windows-msvc.exe
clawbot-gateway-aarch64-apple-darwin
clawbot-gateway-x86_64-unknown-linux-gnu
```

## 托盘行为

| 操作 | 行为 |
|------|------|
| 点击关闭按钮 | 隐藏窗口到托盘（不退出） |
| 双击托盘图标 | 显示窗口 |
| 右键托盘菜单 | 显示窗口 / 退出 |
| 开机自启 | 默认关闭，可在设置中开启 |

## 自动更新

Tauri 的 `tauri-plugin-updater` 配合静态 JSON 文件：

1. 构建时生成签名
2. 上传安装包到服务器
3. 更新 `https://your-domain.com/updates.json` 中的版本信息
4. 客户端启动时检查更新

```json
{
  "version": "1.0.1",
  "notes": "修复了 XXX",
  "pub_date": "2026-07-27T12:00:00Z",
  "platforms": {
    "windows-x86_64": {
      "signature": "dW50...",
      "url": "https://your-domain.com/releases/ClawBot%20Gateway_1.0.1_x64.msi.zip"
    },
    "darwin-aarch64": {
      "signature": "dW50...",
      "url": "https://your-domain.com/releases/ClawBot%20Gateway_1.0.1_aarch64.dmg"
    },
    "linux-x86_64": {
      "signature": "dW50...",
      "url": "https://your-domain.com/releases/ClawBot%20Gateway_1.0.1_amd64.deb"
    }
  }
}
```

## 风险与注意事项

| 风险 | 说明 | 缓解措施 |
|------|------|---------|
| WebView2 依赖 | Windows 7 无自带 WebView2 | 安装包内嵌 WebView2 引导安装，Win10/11 自带 |
| 端口 8080 冲突 | 其他程序占用 | sidecar 启动时检测端口占用，自动 fallback 到随机端口，通过 stdout 通知 Tauri |
| sidecar 崩溃 | Go 进程异常退出 | Tauri 监听 process exit，自动重启或弹窗提示 |
| 首次启动慢 | 需要初始化数据库、下载 iLink 依赖 | 启动窗口显示 loading 状态 |
| 构建环境 | 需要 Go + Rust + Node.js 三套工具链 | CI 中使用 GitHub Actions 统一构建 |

## 实现步骤

```
Phase 1: 项目初始化
  ├── 安装 Tauri CLI
  ├── 初始化 src-tauri 目录
  ├── 配置 tauri.conf.json
  └── 配置 Rust 依赖

Phase 2: Sidecar 集成
  ├── 编写 Rust main.rs
  ├── 配置 sidecar 权限
  ├── 交叉编译 Go sidecar
  └── 验证 sidecar 启动/停止

Phase 3: 前端适配
  ├── 修改 api/client.ts 加 BASE_URL
  ├── 修改 vite.config.ts 端口
  └── 验证 Tauri dev 模式

Phase 4: 托盘与体验
  ├── 托盘图标
  ├── 关闭到托盘
  ├── 开机自启
  └── 自动更新

Phase 5: 构建与发布
  ├── CI 配置（GitHub Actions）
  ├── 三平台交叉编译
  └── 签名与分发
```