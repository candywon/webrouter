---
title: 安装指南
description: WebRouter 详细安装说明
---

# 安装指南

## 环境要求

- **Python** 3.8+（必需）
- **Go** 1.21+（仅从源码编译 wr-proxy 时需要，已含预编译二进制）
- **内存** 2 GB+
- **磁盘** 500 MB+

## 手动安装

### 1. 克隆仓库

```bash
git clone https://github.com/candywon/webrouter.git
cd webrouter
```

### 2. 配置 Python 环境

```bash
python3 -m venv venv
source venv/bin/activate
pip install -r backend/requirements.txt
```

### 3. 编译 wr-proxy（可选）

```bash
cd wr-proxy && make build && cd ..
```

### 4. 配置

在项目根目录创建 `.env` 文件：

```bash
cat > .env << 'EOF'
SESSION_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))")
FLASK_ENV=production
WR_PORT=5050
WR_PROXY_PORT=5051
EOF
```

### 5. 启动服务

```bash
python3 backend/start.py start
```

查看状态：

```bash
python3 backend/start.py status
```

## Docker Compose

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

## 进程管理

```bash
python3 backend/start.py start     # 启动所有服务
python3 backend/start.py stop      # 停止所有服务
python3 backend/start.py restart   # 重启
python3 backend/start.py status    # 查看状态
python3 backend/start.py logs      # 查看日志
```