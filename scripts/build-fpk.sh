#!/bin/bash
# ============================================================
# ClawBot Gateway FPK 构建脚本
# ============================================================
# 构建 clawbot-gateway 的飞牛 fnOS 应用包 (.fpk)
# 使用方法：
#   ./scripts/build-fpk.sh              # 构建当前版本
#   ./scripts/build-fpk.sh --version 1.0.0  # 指定版本
#   ./scripts/build-fpk.sh --arch arm       # 构建 ARM 版本
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${SCRIPT_DIR}"

# ---- 解析参数 ----
VERSION=""
ARCH="x86_64"
FNPLATFORM="x86"
NODE_ARCH="x64"

while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --arch)
            case "$2" in
                x86_64|x86|amd64)
                    ARCH="x86_64"
                    FNPLATFORM="x86"
                    NODE_ARCH="x64"
                    ;;
                arm64|arm|aarch64)
                    ARCH="arm64"
                    FNPLATFORM="arm"
                    NODE_ARCH="arm64"
                    ;;
                *)
                    echo "不支持的架构: $2 (可选: x86_64, arm64)"
                    exit 1
                    ;;
            esac
            shift 2
            ;;
        --help)
            echo "用法: $0 [--version VERSION] [--arch x86_64|arm64]"
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

# ---- 确认版本号 ----
if [ -z "${VERSION}" ]; then
    # 从 version.go 读取
    VERSION=$(grep 'Version\s*=' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
    if [ "${VERSION}" = "dev" ]; then
        VERSION="1.0.0"
        echo "⚠  未设置版本号，使用默认: ${VERSION}"
    fi
fi

echo "============================================"
echo "  构建 ClawBot Gateway FPK"
echo "  版本: ${VERSION}"
echo "  架构: ${ARCH}"
echo "============================================"

# ---- 1. 构建前端 ----
echo ""
echo "📦 构建前端..."
cd web
npm install --silent 2>/dev/null
npm run build
cd "${SCRIPT_DIR}"
echo "✅ 前端构建完成"

# ---- 2. 构建 Go 后端 ----
echo ""
echo "🔨 构建 Go 后端 (linux/amd64)..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
    -ldflags="-X 'clawbot-gateway/internal/version.Version=${VERSION}' -X 'clawbot-gateway/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)' -X 'clawbot-gateway/internal/version.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')'" \
    -o "fpk/app/server/clawbot-gateway" .
echo "✅ Go 后端构建完成: $(du -h fpk/app/server/clawbot-gateway | cut -f1)"

# ---- 3. 复制前端文件 ----
echo ""
echo "📂 复制前端文件..."
rm -rf fpk/app/server/web
mkdir -p fpk/app/server/web && cp -r web/dist fpk/app/server/web/
echo "✅ 前端文件已复制"

# 复制桌面图标（fnOS 要求 icon-{size}.png 格式）
rm -rf fpk/app/ui/images
mkdir -p fpk/app/ui/images
cp fpk/ICON.PNG fpk/app/ui/images/icon_64.png
cp fpk/ICON_256.PNG fpk/app/ui/images/icon_256.png
echo "✅ 前端资源已复制"

# ---- 4. 更新 manifest 版本号和平台 ----
echo ""
echo "📝 更新 manifest..."
sed -i "s/^version.*/version = ${VERSION}/" fpk/manifest
sed -i "s/^platform.*/platform = ${FNPLATFORM}/" fpk/manifest
echo "✅ manifest 已更新"

# ---- 5. 修复脚本权限 ----
echo ""
echo "🔧 修复脚本权限..."
for f in fpk/cmd/main fpk/cmd/install_init fpk/cmd/install_callback fpk/cmd/uninstall_init fpk/cmd/uninstall_callback fpk/cmd/upgrade_init fpk/cmd/upgrade_callback fpk/cmd/config_init fpk/cmd/config_callback; do
    [ -f "$f" ] && chmod +x "$f"
done
echo "✅ 脚本权限已修复"

# ---- 6. 下载 fnpack 并打包 ----
echo ""
echo "📦 下载 fnpack..."
FNPACK="/tmp/fnpack"
if [ ! -f "${FNPACK}" ]; then
    curl -fsSL -o "${FNPACK}" "https://static2.fnnas.com/fnpack/fnpack-1.2.1-linux-${NODE_ARCH}"
    chmod +x "${FNPACK}"
fi
echo "✅ fnpack 就绪"

echo ""
echo "🏗️  打包 FPK..."
cd fpk
"${FNPACK}" build --directory .
if [ -f clawbot-gateway.fpk ]; then
    mv clawbot-gateway.fpk "clawbot-gateway-${FNPLATFORM}.fpk"
    echo "✅ FPK 构建完成: $(ls -lh clawbot-gateway-${FNPLATFORM}.fpk)"
else
    echo "❌ FPK 构建失败"
    ls -la
    exit 1
fi

echo ""
echo "============================================"
echo "  构建完成！"
echo "  输出文件: fpk/clawbot-gateway-${FNPLATFORM}.fpk"
echo "============================================"