#!/bin/bash
# ============================================================
#  WebRouter 一键安装脚本 — 支持 Linux / macOS / Windows(WSL/Git Bash)
#  无需 Docker，自动下载对应平台 New-API 二进制
# ============================================================
set -e

VERSION="1.0.0"
NEWAPI_VERSION="v1.0.0-rc.5"
INSTALL_DIR="${1:-$HOME/webrouter}"

# 颜色
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; exit 1; }
step()  { echo -e "${CYAN}[$1]${NC} $2"; }

echo ""
echo "============================================"
echo "  WebRouter ${VERSION} - AI中转站管理平台"
echo "============================================"
echo ""

# ----------------------------------------------------------
# 1. 检测操作系统和架构
# ----------------------------------------------------------
step "1/7" "检测系统环境..."

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)   PLATFORM="linux" ;;
    darwin)  PLATFORM="darwin" ;;
    mingw*|msys*|cygwin*) PLATFORM="windows" ;;
    *)       error "不支持的操作系统: $OS" ;;
esac

case "$ARCH" in
    x86_64|amd64)  ARCHEXT="amd64" ;;
    arm64|aarch64) ARCHEXT="arm64" ;;
    *)             error "不支持的架构: $ARCH" ;;
esac

info "系统: ${PLATFORM}/${ARCHEXT}"

# ----------------------------------------------------------
# 2. 检查 Python3
# ----------------------------------------------------------
step "2/7" "检查 Python3..."

if command -v python3 &>/dev/null; then
    PYVER=$(python3 --version 2>&1 | awk '{print $2}')
    info "Python ${PYVER} 已安装"
elif command -v python &>/dev/null; then
    PYVER=$(python --version 2>&1 | awk '{print $2}')
    # 检查是否是 Python 3
    case "$PYVER" in
        3.*) info "Python ${PYVER} 已安装"; PYCMD="python" ;;
        *)   warn "Python 2 已找到，需要 Python 3.8+"; PYCMD="" ;;
    esac
else
    PYCMD=""
fi

if [ -z "${PYCMD:-}" ]; then
    PYCMD="python3"
    warn "未找到 Python3，尝试安装..."
    case "$PLATFORM" in
        linux)
            if command -v apt-get &>/dev/null; then
                sudo apt-get update -qq && sudo apt-get install -y -qq python3 python3-pip python3-venv
            elif command -v yum &>/dev/null; then
                sudo yum install -y python3 python3-pip
            elif command -v dnf &>/dev/null; then
                sudo dnf install -y python3 python3-pip
            else
                error "无法自动安装 Python3，请手动安装: https://www.python.org/downloads/"
            fi
            ;;
        darwin)
            if command -v brew &>/dev/null; then
                brew install python3
            else
                error "请先安装 Homebrew (https://brew.sh) 后运行: brew install python3"
            fi
            ;;
        windows)
            error "请手动安装 Python3: https://www.python.org/downloads/ (勾选 Add to PATH)"
            ;;
    esac
    command -v python3 &>/dev/null || error "Python3 安装失败"
fi

# ----------------------------------------------------------
# 3. 创建安装目录
# ----------------------------------------------------------
step "3/7" "创建安装目录..."

mkdir -p "$INSTALL_DIR"/{bin,data,logs}
cd "$INSTALL_DIR"

# 复制项目文件（如果从源码安装）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/../backend/app.py" ]; then
    # 从源码安装
    cp -r "$SCRIPT_DIR/../backend" "$INSTALL_DIR/backend"
    cp -r "$SCRIPT_DIR/../docs" "$INSTALL_DIR/docs" 2>/dev/null || true
    info "项目文件已复制"
else
    # 需要下载 release
    if [ ! -f "$INSTALL_DIR/backend/app.py" ]; then
        warn "从 GitHub 下载 WebRouter..."
        curl -fsSL "https://github.com/user/webrouter/archive/refs/heads/main.tar.gz" | tar xz --strip-components=1 2>/dev/null || \
            error "下载失败，请手动下载: https://github.com/user/webrouter"
    fi
fi

# ----------------------------------------------------------
# 4. 安装 Python 依赖
# ----------------------------------------------------------
step "4/7" "安装 Python 依赖..."

# 创建虚拟环境
if [ ! -d "$INSTALL_DIR/venv" ]; then
    python3 -m venv "$INSTALL_DIR/venv"
    info "虚拟环境已创建"
fi

# 激活虚拟环境
source "$INSTALL_DIR/venv/bin/activate" 2>/dev/null || \
    source "$INSTALL_DIR/venv/Scripts/activate" 2>/dev/null || \
    error "虚拟环境激活失败"

# 安装依赖
pip install --quiet --upgrade pip 2>/dev/null
pip install --quiet -r "$INSTALL_DIR/backend/requirements.txt"
info "Python 依赖已安装"

# ----------------------------------------------------------
# 5. 下载 New-API 二进制
# ----------------------------------------------------------
step "5/7" "下载 New-API ${NEWAPI_VERSION}..."

NEWAPI_BIN="$INSTALL_DIR/bin/new-api"

if [ -f "$NEWAPI_BIN" ]; then
    info "New-API 已存在，跳过下载 (如需更新请删除 bin/new-api 后重跑)"
else
    # 构建下载 URL
    case "$PLATFORM" in
        linux)
            case "$ARCHEXT" in
                amd64) DL_NAME="new-api-${NEWAPI_VERSION}" ;;
                arm64) DL_NAME="new-api-arm64-${NEWAPI_VERSION}" ;;
            esac
            ;;
        darwin)
            # New-API 没有官方 Mac 二进制，从第三方构建或提示 Docker
            warn "New-API 暂无 macOS 官方二进制"
            warn "方案1: 使用 Docker 运行 New-API（推荐）"
            warn "方案2: 自行从源码编译 (需要 Go 1.21+)"
            echo ""
            read -p "是否使用 Docker 运行 New-API? [Y/n] " USE_DOCKER
            USE_DOCKER="${USE_DOCKER:-Y}"
            if [[ "$USE_DOCKER" =~ ^[Yy] ]]; then
                USE_DOCKER_NEWAPI=1
                DL_NAME=""
            else
                # 检查 Go 是否可用，尝试编译
                if command -v go &>/dev/null; then
                    info "检测到 Go，从源码编译 New-API..."
                    TMPDIR=$(mktemp -d)
                    git clone --depth 1 --branch "$NEWAPI_VERSION" \
                        https://github.com/QuantumNous/new-api.git "$TMPDIR/new-api" 2>/dev/null || \
                        git clone --depth 1 \
                        https://github.com/QuantumNous/new-api.git "$TMPDIR/new-api"
                    cd "$TMPDIR/new-api"
                    CGO_ENABLED=0 GOOS=darwin GOARCH=$ARCHEXT go build -o "$NEWAPI_BIN" .
                    cd "$INSTALL_DIR"
                    rm -rf "$TMPDIR"
                    info "New-API 编译完成"
                    DL_NAME=""
                else
                    error "请安装 Go (https://go.dev/dl/) 后重试，或选择 Docker 模式"
                fi
            fi
            ;;
        windows)
            DL_NAME="new-api-${NEWAPI_VERSION}.exe"
            NEWAPI_BIN="$INSTALL_DIR/bin/new-api.exe"
            ;;
    esac

    if [ -n "$DL_NAME" ]; then
        DL_URL="https://github.com/QuantumNous/new-api/releases/download/${NEWAPI_VERSION}/${DL_NAME}"
        info "下载: $DL_URL"
        curl -fSL --progress-bar -o "$NEWAPI_BIN" "$DL_URL" || \
            error "下载失败，请检查网络或手动下载: $DL_URL"
        chmod +x "$NEWAPI_BIN"
        info "New-API 下载完成"
    fi
fi

# ----------------------------------------------------------
# 6. 生成配置
# ----------------------------------------------------------
step "6/7" "生成配置文件..."

if [ ! -f "$INSTALL_DIR/.env" ]; then
    SESSION_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))" 2>/dev/null || echo "changeme-$(date +%s)")
    cat > "$INSTALL_DIR/.env" << ENVEOF
# WebRouter 环境配置 — 生成于 $(date)
SESSION_SECRET=${SESSION_SECRET}
NEWAPI_ADMIN_TOKEN=
DATABASE_URI=
REDIS_URL=
FLASK_ENV=production
ENVEOF
    info ".env 已生成"
else
    info ".env 已存在，跳过"
fi

# 生成启动脚本
cat > "$INSTALL_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
cd "$(dirname "$0")"
source venv/bin/activate 2>/dev/null || source venv/Scripts/activate 2>/dev/null

# 加载环境变量
[ -f .env ] && export $(grep -v '^#' .env | xargs)

# 启动 New-API（如果不用 Docker）
if [ -z "$USE_DOCKER_NEWAPI" ] && [ -f bin/new-api ]; then
    echo "启动 New-API..."
    nohup bin/new-api --port 3000 > logs/newapi.log 2>&1 &
    echo $! > logs/newapi.pid
    sleep 2
fi

# 启动 WebRouter
echo "启动 WebRouter..."
cd backend
DATABASE_URI="${DATABASE_URI:-sqlite:///$PWD/../data/webrouter.db}" \
NEWAPI_URL="${NEWAPI_URL:-http://localhost:3000}" \
nohup python3 -c "from app import create_app; app = create_app(); app.run(host='0.0.0.0', port=5000)" \
    > ../logs/webrouter.log 2>&1 &
echo $! > ../logs/webrouter.pid

echo ""
echo "WebRouter 已启动！"
echo "  管理后台: http://localhost:5000"
echo "  New-API:  http://localhost:3000"
echo "  日志目录: $(pwd)/../logs/"
echo ""
echo "  停止服务: ./stop.sh"
STARTEOF
chmod +x "$INSTALL_DIR/start.sh"

# 生成停止脚本
cat > "$INSTALL_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
cd "$(dirname "$0")"
[ -f logs/newapi.pid ] && kill "$(cat logs/newapi.pid)" 2>/dev/null && echo "New-API 已停止" && rm logs/newapi.pid
[ -f logs/webrouter.pid ] && kill "$(cat logs/webrouter.pid)" 2>/dev/null && echo "WebRouter 已停止" && rm logs/webrouter.pid
STOPEOF
chmod +x "$INSTALL_DIR/stop.sh"

info "启动脚本已生成"

# ----------------------------------------------------------
# 7. 启动服务
# ----------------------------------------------------------
step "7/7" "启动服务..."

cd "$INSTALL_DIR"
bash start.sh

echo ""
echo "============================================"
echo "  安装完成！"
echo "============================================"
echo ""
echo "  管理后台:  http://localhost:5000"
echo "  New-API:   http://localhost:3000"
echo "  安装目录:  $INSTALL_DIR"
echo "  配置文件:  $INSTALL_DIR/.env"
echo ""
echo "  启动服务:  cd $INSTALL_DIR && ./start.sh"
echo "  停止服务:  cd $INSTALL_DIR && ./stop.sh"
echo "  查看日志:  tail -f $INSTALL_DIR/logs/*.log"
echo ""
