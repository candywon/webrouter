#!/bin/bash
# ============================================================
#  New-API 交叉编译脚本 — 生成 5 个平台二进制
#  用途：WebRouter 项目内部使用，定期从 New-API 最新源码编译
#  前提：需要 Go 1.21+ 和 Git
# ============================================================
set -e

NEWAPI_REPO="https://github.com/QuantumNous/new-api.git"
BRANCH="${1:-main}"  # 可指定分支或 tag，如 v1.0.0-rc.5
OUTPUT_DIR="$(cd "$(dirname "$0")" && pwd)/../deploy/bin"
VERSION=""

echo "============================================"
echo "  New-API 交叉编译"
echo "============================================"
echo ""

# 检查 Go
command -v go &>/dev/null || { echo "错误: 未安装 Go (https://go.dev/dl/)"; exit 1; }
echo "Go 版本: $(go version)"

# 创建临时目录
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# 克隆源码
echo "[1/4] 克隆 New-API 源码 (branch: $BRANCH)..."
git clone --depth 1 --branch "$BRANCH" "$NEWAPI_REPO" "$TMPDIR/new-api" 2>/dev/null || \
    git clone --depth 1 "$NEWAPI_REPO" "$TMPDIR/new-api"

cd "$TMPDIR/new-api"

# 获取版本号
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev-$(date +%Y%m%d)")
echo "  版本: $VERSION"

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 编译目标列表
TARGETS=(
    "linux/amd64:new-api-linux-amd64"
    "linux/arm64:new-api-linux-arm64"
    "darwin/amd64:new-api-darwin-amd64"
    "darwin/arm64:new-api-darwin-arm64"
    "windows/amd64:new-api-windows-amd64.exe"
)

echo ""
echo "[2/4] 开始交叉编译..."

for target in "${TARGETS[@]}"; do
    IFS=':' read -r GOOS_ARCH BINARY <<< "$target"
    GOOS="${GOOS_ARCH%%/*}"
    GOARCH="${GOOS_ARCH##*/}"

    echo "  编译: $GOOS/$GOARCH → $BINARY"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w -X main.Version=$VERSION" \
        -o "$OUTPUT_DIR/$BINARY" . 2>&1 || {
        echo "  ⚠ 编译失败: $GOOS/$GOARCH (可能需要调整代码)"
        continue
    }
done

echo ""
echo "[3/4] 生成校验文件..."

cd "$OUTPUT_DIR"
sha256sum new-api-* > checksums.txt 2>/dev/null || \
    shasum -a 256 new-api-* > checksums.txt

echo ""
echo "[4/4] 编译结果:"
echo ""
ls -lh new-api-* 2>/dev/null
echo ""
cat checksums.txt
echo ""
echo "============================================"
echo "  编译完成！二进制文件位于:"
echo "  $OUTPUT_DIR/"
echo ""
echo "  将这些文件上传到 GitHub Release 后，"
echo "  install.sh 会按平台自动下载对应文件。"
echo "============================================"
