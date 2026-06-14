#!/bin/bash
# ============================================================
#  WebRouter 一键安装脚本 — 支持 Linux / macOS / Windows(WSL/Git Bash)
#  包含 wr-proxy (Go) + WebRouter (Flask)
# ============================================================
set -e

VERSION="1.0.0"
INSTALL_DIR="${1:-$HOME/webrouter}"

# 颜色
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; exit 1; }
step()  { echo -e "${CYAN}[$1]${NC} $2"; }

echo ""
echo "============================================"
echo "  WebRouter ${VERSION} - AI-API综合管理平台"
echo "============================================"
echo ""

# ----------------------------------------------------------
# 1. 检测操作系统和架构
# ----------------------------------------------------------
step "1/6" "检测系统环境..."

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
step "2/6" "检查 Python3..."

if command -v python3 &>/dev/null; then
    PYVER=$(python3 --version 2>&1 | awk '{print $2}')
    info "Python ${PYVER} 已安装"
elif command -v python &>/dev/null; then
    PYVER=$(python --version 2>&1 | awk '{print $2}')
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
step "3/6" "创建安装目录..."

mkdir -p "$INSTALL_DIR"/{data,logs}
cd "$INSTALL_DIR"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/../backend/app.py" ]; then
    cp -r "$SCRIPT_DIR/../backend" "$INSTALL_DIR/backend"
    cp -r "$SCRIPT_DIR/../wr-proxy" "$INSTALL_DIR/wr-proxy"
    cp -r "$SCRIPT_DIR/../docs" "$INSTALL_DIR/docs" 2>/dev/null || true
    info "项目文件已复制"
else
    if [ ! -f "$INSTALL_DIR/backend/app.py" ]; then
        warn "从 GitHub 下载 WebRouter..."
        curl -fsSL "https://github.com/candywon/webrouter/archive/refs/heads/main.tar.gz" | tar xz --strip-components=1 2>/dev/null || \
            error "下载失败，请手动下载: https://github.com/candywon/webrouter"
    fi
fi

# ----------------------------------------------------------
# 4. 安装 Python 依赖
# ----------------------------------------------------------
step "4/6" "安装 Python 依赖..."

if [ ! -d "$INSTALL_DIR/venv" ]; then
    python3 -m venv "$INSTALL_DIR/venv"
    info "虚拟环境已创建"
fi

source "$INSTALL_DIR/venv/bin/activate" 2>/dev/null || \
    source "$INSTALL_DIR/venv/Scripts/activate" 2>/dev/null || \
    error "虚拟环境激活失败"

pip install --quiet --upgrade pip 2>/dev/null
pip install --quiet -r "$INSTALL_DIR/backend/requirements.txt"
info "Python 依赖已安装"

# ----------------------------------------------------------
# 5. 编译 wr-proxy (Go)
# ----------------------------------------------------------
step "5/6" "编译 wr-proxy..."

WR_PROXY_BIN="$INSTALL_DIR/wr-proxy/wr-proxy"

if command -v go &>/dev/null; then
    cd "$INSTALL_DIR/wr-proxy"
    go build -o "$WR_PROXY_BIN" .
    cd "$INSTALL_DIR"
    info "wr-proxy 编译完成"
else
    warn "未找到 Go，跳过 wr-proxy 编译"
    warn "wr-proxy 需要 Go 1.21+，可从 https://go.dev/dl/ 安装"
    warn "已安装的可执行文件可直接放入 wr-proxy/ 目录"
fi

# ----------------------------------------------------------
# 6. 生成配置
# ----------------------------------------------------------
step "6/6" "生成配置文件..."

if [ ! -f "$INSTALL_DIR/.env" ]; then
    SESSION_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))" 2>/dev/null || echo "changeme-$(date +%s)")
    cat > "$INSTALL_DIR/.env" << ENVEOF
# WebRouter 环境配置 — 生成于 $(date)
SESSION_SECRET=${SESSION_SECRET}
DATABASE_URI=
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

# 启动 wr-proxy
if [ -f wr-proxy/wr-proxy ]; then
    echo "启动 wr-proxy..."
    cd wr-proxy
    nohup ./wr-proxy > ../logs/wr-proxy.log 2>&1 &
    echo $! > ../logs/wr-proxy.pid
    cd ..
    sleep 1
fi

# 启动 WebRouter
echo "启动 WebRouter..."
cd backend
DATABASE_URI="${DATABASE_URI:-sqlite:///$PWD/../data/webrouter.db}" \
nohup python3 -c "from app import create_app; app = create_app(); app.run(host='0.0.0.0', port=5000)" \
    > ../logs/webrouter.log 2>&1 &
echo $! > ../logs/webrouter.pid

echo ""
echo "WebRouter 已启动！"
echo "  管理后台: http://localhost:5000"
echo "  代理网关: http://localhost:5051"
echo "  日志目录: $(pwd)/../logs/"
echo ""
echo "  停止服务: ./stop.sh"
STARTEOF
chmod +x "$INSTALL_DIR/start.sh"

# 生成停止脚本
cat > "$INSTALL_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
cd "$(dirname "$0")"
[ -f logs/wr-proxy.pid ] && kill "$(cat logs/wr-proxy.pid)" 2>/dev/null && echo "wr-proxy 已停止" && rm logs/wr-proxy.pid
[ -f logs/webrouter.pid ] && kill "$(cat logs/webrouter.pid)" 2>/dev/null && echo "WebRouter 已停止" && rm logs/webrouter.pid
STOPEOF
chmod +x "$INSTALL_DIR/stop.sh"

info "启动脚本已生成"

echo ""
echo "============================================"
echo "  安装完成！"
echo "============================================"
echo ""
echo "  管理后台:  http://localhost:5000"
echo "  代理网关:  http://localhost:5051"
echo "  安装目录:  $INSTALL_DIR"
echo "  配置文件:  $INSTALL_DIR/.env"
echo ""
echo "  启动服务:  cd $INSTALL_DIR && ./start.sh"
echo "  停止服务:  cd $INSTALL_DIR && ./stop.sh"
echo "  查看日志:  tail -f $INSTALL_DIR/logs/*.log"
echo ""
