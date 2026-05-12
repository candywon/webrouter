#!/bin/bash
# WebRouter 一键部署脚本
set -e

echo "======================================"
echo "   WebRouter - AI中转站管理平台 部署"
echo "======================================"
echo ""

# 1. 检查Docker
if ! command -v docker &>/dev/null; then
    echo "[1/6] 安装 Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
else
    echo "[1/6] Docker 已安装 ✓"
fi

# 2. 检查Docker Compose
if ! docker compose version &>/dev/null; then
    echo "[2/6] 安装 Docker Compose 插件..."
    apt-get update -qq && apt-get install -y -qq docker-compose-plugin 2>/dev/null || \
    yum install -y docker-compose-plugin 2>/dev/null || {
        echo "安装失败，请手动安装 docker-compose-plugin"
        exit 1
    }
else
    echo "[2/6] Docker Compose 已安装 ✓"
fi

# 3. 创建安装目录
INSTALL_DIR="${1:-$HOME/webrouter}"
echo "[3/6] 安装目录: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

# 4. 复制部署文件
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/docker-compose.yml" ]; then
    cp "$SCRIPT_DIR/docker-compose.yml" "$INSTALL_DIR/"
    cp "$SCRIPT_DIR/Dockerfile" "$INSTALL_DIR/"
    cp "$SCRIPT_DIR/nginx.conf" "$INSTALL_DIR/"
    echo "[4/6] 部署文件已复制 ✓"
else
    echo "[4/6] 下载部署文件..."
    REPO_URL="https://raw.githubusercontent.com/user/webrouter/main/deploy"
    curl -fsSL "$REPO_URL/docker-compose.yml" -o "$INSTALL_DIR/docker-compose.yml"
    curl -fsSL "$REPO_URL/Dockerfile" -o "$INSTALL_DIR/Dockerfile"
    curl -fsSL "$REPO_URL/nginx.conf" -o "$INSTALL_DIR/nginx.conf"
fi

# 5. 生成配置
if [ ! -f "$INSTALL_DIR/.env" ]; then
    SESSION_SECRET=$(openssl rand -hex 32 2>/dev/null || echo "changeme-$(date +%s)")
    ADMIN_TOKEN=$(openssl rand -hex 16 2>/dev/null || echo "admin-$(date +%s)")
    cat > "$INSTALL_DIR/.env" << EOF
# WebRouter 环境配置 — 生成于 $(date)
SESSION_SECRET=$SESSION_SECRET
NEWAPI_ADMIN_TOKEN=$ADMIN_TOKEN
DB_DSN=
EOF
    echo "[5/6] 配置文件已生成 ✓"
else
    echo "[5/6] 配置文件已存在，跳过 ✓"
fi

# 6. 启动
echo "[6/6] 启动服务..."
cd "$INSTALL_DIR"
docker compose up -d --build

echo ""
echo "等待服务启动..."
sleep 10

# 检查状态
if docker compose ps | grep -q "Up\|running"; then
    PUBLIC_IP=$(curl -sf ifconfig.me 2>/dev/null || echo "YOUR_SERVER_IP")
    echo ""
    echo "======================================"
    echo "   部署完成！"
    echo "======================================"
    echo ""
    echo "  管理后台:  http://$PUBLIC_IP"
    echo "  New-API:   http://$PUBLIC_IP:3000"
    echo "  健康检查:  http://$PUBLIC_IP/health"
    echo ""
    echo "  配置文件:  $INSTALL_DIR/.env"
    echo "  查看日志:  cd $INSTALL_DIR && docker compose logs -f"
    echo "  停止服务:  cd $INSTALL_DIR && docker compose down"
    echo ""
else
    echo ""
    echo "⚠ 服务启动异常，请检查日志："
    echo "  cd $INSTALL_DIR && docker compose logs"
fi
