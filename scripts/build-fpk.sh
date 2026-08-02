#!/bin/bash
# ============================================================
# ClawBot Gateway FPK 构建脚本
# ============================================================
# 构建 clawbot-gateway 的飞牛 fnOS 应用包 (.fpk)
#
# FPK 脚手架已迁移至 https://github.com/EzFavorites/FnDepot
# 此脚本构建二进制产物后，将其打包到 FnDepot 仓库的 scaffold 中。
#
# 使用方法：
#   ./scripts/build-fpk.sh --fndepot ../FnDepot   # 指定 FnDepot 仓库路径并打包
#   ./scripts/build-fpk.sh                          # 仅构建产物到 build/ 目录
#   ./scripts/build-fpk.sh --version 1.0.0          # 指定版本
#   ./scripts/build-fpk.sh --arch arm               # 构建 ARM 版本
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${SCRIPT_DIR}"

# ---- 解析参数 ----
VERSION=""
ARCH="x86_64"
GOARCH="amd64"
FNPACK_ARCH="amd64"
FNPLATFORM="x86"
FNDEPOT_DIR=""

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
                    GOARCH="amd64"
                    FNPACK_ARCH="amd64"
                    ;;
                arm64|arm|aarch64)
                    ARCH="arm64"
                    FNPLATFORM="arm"
                    GOARCH="arm64"
                    FNPACK_ARCH="arm64"
                    ;;
                *)
                    echo "不支持的架构: $2 (可选: x86_64, arm64)"
                    exit 1
                    ;;
            esac
            shift 2
            ;;
        --fndepot)
            FNDEPOT_DIR="$2"
            shift 2
            ;;
        --help)
            echo "用法: $0 [--version VERSION] [--arch x86_64|arm64] [--fndepot PATH]"
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

# ---- 确认 FnDepot 仓库 ----
if [ -n "${FNDEPOT_DIR}" ]; then
    FNDEPOT_DIR="$(cd "${FNDEPOT_DIR}" && pwd)"
    if [ ! -d "${FNDEPOT_DIR}/clawbot-gateway" ]; then
        echo "❌ FnDepot 仓库路径无效: ${FNDEPOT_DIR}/clawbot-gateway 不存在"
        exit 1
    fi
    echo "📁 FnDepot 仓库: ${FNDEPOT_DIR}"
fi

# ---- 1. 构建前端 ----
echo ""
echo "📦 构建前端..."
cd web
npm install --include=dev --silent 2>/dev/null
npm run build
cd "${SCRIPT_DIR}"
echo "✅ 前端构建完成"

# ---- 2. 构建 Go 后端 ----
echo ""
echo "🔨 构建 Go 后端 (linux/${GOARCH})..."
BUILD_DIR="${SCRIPT_DIR}/build"
mkdir -p "${BUILD_DIR}/server"
GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 go build \
    -ldflags="-X 'clawbot-gateway/internal/version.Version=${VERSION}' -X 'clawbot-gateway/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)' -X 'clawbot-gateway/internal/version.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')'" \
    -o "${BUILD_DIR}/server/clawbot-gateway" .
echo "✅ Go 后端构建完成: $(du -h "${BUILD_DIR}/server/clawbot-gateway" | cut -f1)"

# ---- 3. 复制前端文件 ----
echo ""
echo "📂 复制前端文件..."
mkdir -p "${BUILD_DIR}/server/web"
cp -r web/dist "${BUILD_DIR}/server/web/"
echo "✅ 前端文件已复制"

# ---- 如果指定了 FnDepot 路径，打包 FPK ----
if [ -n "${FNDEPOT_DIR}" ]; then
    APP_DIR="${FNDEPOT_DIR}/clawbot-gateway"

    echo ""
    echo "📦 打包到 FnDepot 仓库..."

    # 复制二进制和前端
    cp "${BUILD_DIR}/server/clawbot-gateway" "${APP_DIR}/app/server/clawbot-gateway"
    chmod +x "${APP_DIR}/app/server/clawbot-gateway"
    rm -rf "${APP_DIR}/app/server/web"
    cp -r "${BUILD_DIR}/server/web" "${APP_DIR}/app/server/web"

    # 复制桌面图标
    mkdir -p "${APP_DIR}/app/ui/images"
    cp "${APP_DIR}/ICON.PNG" "${APP_DIR}/app/ui/images/icon_64.png"
    cp "${APP_DIR}/ICON_256.PNG" "${APP_DIR}/app/ui/images/icon_256.png"

    # 更新 manifest 版本号和平台
    sed -i "s/^version.*/version = ${VERSION}/" "${APP_DIR}/manifest"
    sed -i "s/^platform.*/platform = ${FNPLATFORM}/" "${APP_DIR}/manifest"

    # 修复脚本权限
    for f in main install_init install_callback uninstall_init uninstall_callback upgrade_init upgrade_callback config_init config_callback; do
        [ -f "${APP_DIR}/cmd/$f" ] && chmod +x "${APP_DIR}/cmd/$f"
    done

    # 下载 fnpack 并打包
    echo ""
    echo "📦 下载 fnpack..."
    FNPACK="/tmp/fnpack"
    if [ ! -f "${FNPACK}" ]; then
        curl -fsSL -o "${FNPACK}" "https://static2.fnnas.com/fnpack/fnpack-1.2.1-linux-${FNPACK_ARCH}"
        chmod +x "${FNPACK}"
    fi
    echo "✅ fnpack 就绪"

    echo ""
    echo "🏗️  打包 FPK..."
    cd "${APP_DIR}"
    "${FNPACK}" build --directory .
    if [ -f clawbot-gateway.fpk ]; then
        mv clawbot-gateway.fpk "clawbot-gateway-${FNPLATFORM}.fpk"
        echo "✅ FPK 构建完成: $(ls -lh clawbot-gateway-${FNPLATFORM}.fpk)"
    else
        echo "❌ FPK 构建失败"
        ls -la
        exit 1
    fi

    # 清理产物（保留 FPK）
    rm -f "${APP_DIR}/app/server/clawbot-gateway"
    rm -rf "${APP_DIR}/app/server/web"
    rm -rf "${APP_DIR}/app/ui/images"

    echo ""
    echo "============================================"
    echo "  构建完成！"
    echo "  输出文件: ${APP_DIR}/clawbot-gateway-${FNPLATFORM}.fpk"
    echo "============================================"
else
    echo ""
    echo "============================================"
    echo "  构建完成！"
    echo "  产物目录: ${BUILD_DIR}/"
    echo "  用法: ./scripts/build-fpk.sh --fndepot ../FnDepot 打包 FPK"
    echo "============================================"
fi