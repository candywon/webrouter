#!/bin/bash
# ============================================================
#  更新 New-API 二进制 — 从 GitHub Release 下载最新版
#  用法: ./update-newapi.sh [版本号]
# ============================================================
set -e

NEWAPI_REPO="QuantumNous/new-api"
VERSION="${1:-latest}"
INSTALL_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$INSTALL_DIR/bin"

echo "============================================"
echo "  更新 New-API"
echo "============================================"

# 检测平台
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)   PLATFORM="linux" ;;
    darwin)  PLATFORM="darwin" ;;
    mingw*|msys*|cygwin*) PLATFORM="windows" ;;
esac

case "$ARCH" in
    x86_64|amd64)  ARCHEXT="amd64" ;;
    arm64|aarch64) ARCHEXT="arm64" ;;
esac

echo "平台: ${PLATFORM}/${ARCHEXT}"

# 获取最新版本号
if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${NEWAPI_REPO}/releases/latest" | \
        grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    echo "最新版本: $VERSION"
fi

# Mac 特殊处理
if [ "$PLATFORM" = "darwin" ]; then
    echo ""
    echo "⚠ New-API 暂无 macOS 官方预编译二进制"
    echo "请选择:"
    echo "  1) 从 WebRouter 自建 Release 下载 (如有)"
    echo "  2) 本地编译 (需要 Go 1.21+)"
    echo "  3) 使用 Docker"
    echo ""
    read -p "请选择 [1/2/3]: " CHOICE
    case "$CHOICE" in
        2)
            if command -v go &>/dev/null; then
                TMPDIR=$(mktemp -d)
                git clone --depth 1 --branch "$VERSION" \
                    "https://github.com/${NEWAPI_REPO}.git" "$TMPDIR/new-api"
                cd "$TMPDIR/new-api"
                CGO_ENABLED=0 GOOS=darwin GOARCH=$ARCHEXT \
                    go build -trimpath -ldflags="-s -w" \
                    -o "$BIN_DIR/new-api" .
                rm -rf "$TMPDIR"
                echo "✓ 编译完成: $BIN_DIR/new-api"
            else
                echo "✗ 未找到 Go，请安装: https://go.dev/dl/"
                exit 1
            fi
            ;;
        3)
            echo "使用 Docker 运行 New-API..."
            docker pull calciumion/new-api:latest
            echo "✓ 镜像已更新，用 docker compose 启动"
            ;;
        *)
            echo "跳过 New-API 更新"
            ;;
    esac
    exit 0
fi

# Linux / Windows 下载
case "$PLATFORM" in
    linux)
        case "$ARCHEXT" in
            amd64) DL_NAME="new-api-${VERSION}" ;;
            arm64) DL_NAME="new-api-arm64-${VERSION}" ;;
        esac
        OUT_NAME="new-api"
        ;;
    windows)
        DL_NAME="new-api-${VERSION}.exe"
        OUT_NAME="new-api.exe"
        ;;
esac

DL_URL="https://github.com/${NEWAPI_REPO}/releases/download/${VERSION}/${DL_NAME}"
OUT_PATH="$BIN_DIR/$OUT_NAME"

echo "下载: $DL_URL"
mkdir -p "$BIN_DIR"
curl -fSL --progress-bar -o "$OUT_PATH" "$DL_URL"
chmod +x "$OUT_PATH"

echo ""
echo "✓ New-API 已更新到 $VERSION"
echo "  文件: $OUT_PATH"
echo "  重启生效: cd $INSTALL_DIR && ./stop.sh && ./start.sh"
