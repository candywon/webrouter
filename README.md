# WebRouter — AI API 中转站管理平台

> 一站式 AI API 网关管理：渠道聚合、健康监控、成本计费、团队协作

## 功能概览

| 模块 | 功能 |
|------|------|
| 仪表盘 | 系统概览、请求统计、渠道健康一览 |
| 渠道管理 | 添加/编辑 API 渠道、负载均衡、优先级 |
| 健康监控 | 实时检测渠道可用性、延迟追踪、自动降级 |
| 告警中心 | 自定义告警规则、多级通知、历史记录 |
| 成本计费 | 用量统计、额度管理、费用报表 |
| 团队管理 | 成员邀请、角色权限、配额分配 |
| CLI 生成 | 一键生成渠道配置命令行 |
| 系统设置 | New-API 连接、告警配置、系统参数 |

## 快速安装

### 系统要求

- Python 3.8+
- 2GB+ 内存
- 网络（首次安装需下载 New-API 二进制）

### 一键安装（推荐）

```bash
# 下载项目
git clone https://github.com/user/webrouter.git
cd webrouter

# 运行安装脚本
bash deploy/install.sh
```

安装脚本会自动完成：
1. 检测操作系统和 CPU 架构
2. 安装 Python3（如缺失）
3. 创建虚拟环境 + 安装依赖
4. 下载对应平台 New-API 二进制
5. 生成配置文件和启动脚本
6. 启动服务

### 安装后

```bash
cd ~/webrouter

# 启动
./start.sh

# 停止
./stop.sh

# 查看日志
tail -f logs/*.log
```

## Docker 安装

如果你更偏好 Docker：

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

Docker 模式下 New-API 自动以容器方式运行，无需手动下载二进制。

## macOS 用户注意

New-API 没有官方 macOS 预编译二进制，安装脚本提供三种方案：

1. **Docker（推荐）** — `docker run -d -p 3000:3000 calciumion/new-api:latest`
2. **本地编译** — 需要 Go 1.21+，脚本自动从源码编译
3. **跳过** — 仅运行 WebRouter，无 New-API 时使用演示数据

## 手动安装

如果一键脚本不适用，可手动安装：

```bash
# 1. 创建目录
mkdir -p ~/webrouter/{bin,data,logs}
cd ~/webrouter

# 2. 复制项目文件
cp -r /path/to/webrouter/backend ./

# 3. 创建虚拟环境
python3 -m venv venv
source venv/bin/activate
pip install -r backend/requirements.txt

# 4. 下载 New-API（Linux 示例）
# 访问 https://github.com/QuantumNous/new-api/releases 获取最新版
curl -L -o bin/new-api https://github.com/QuantumNous/new-api/releases/download/v1.0.0-rc.5/new-api-v1.0.0-rc.5
chmod +x bin/new-api

# 5. 生成配置
cp .env.example .env
# 编辑 .env 设置你的密钥

# 6. 启动服务
python3 backend/start.py start

# 7. 查看状态
python3 backend/start.py status
```

## 进程管理

WebRouter 使用 `start.py` 管理双进程（WebRouter + New-API）：

```bash
# 启动所有服务
python3 backend/start.py start

# 停止所有服务
python3 backend/start.py stop

# 重启
python3 backend/start.py restart

# 查看状态
python3 backend/start.py status

# 查看实时日志
python3 backend/start.py logs
```

也可以使用 shell 脚本（安装时自动生成）：

```bash
./start.sh    # 启动
./stop.sh     # 停止
```

## 配置说明

所有配置通过 `.env` 文件管理（首次安装自动生成）：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| SESSION_SECRET | Flask 会话密钥 | 自动生成 |
| NEWAPI_URL | New-API 地址 | http://localhost:3000 |
| NEWAPI_ADMIN_TOKEN | New-API 管理令牌 | 空 |
| DATABASE_URI | 数据库连接 | SQLite |
| REDIS_URL | Redis 连接（可选） | 空 |
| FLASK_ENV | 运行环境 | production |
| WEBROUTER_PORT | WebRouter 端口 | 5000 |
| NEWAPI_PORT | New-API 端口 | 3000 |
| FLASK_HOST | 监听地址 | 0.0.0.0 |

## 更新 New-API

```bash
# 自动下载最新版
bash deploy/update-newapi.sh

# 指定版本
bash deploy/update-newapi.sh v1.0.0-rc.5

# 重启生效
python3 backend/start.py restart
```

## 交叉编译 New-API（开发者）

如果你需要为所有平台编译 New-API 二进制：

```bash
# 需要 Go 1.21+
bash scripts/build-newapi.sh              # 默认 main 分支
bash scripts/build-newapi.sh v1.0.0-rc.5  # 指定 tag
```

编译产物位于 `deploy/bin/`，包含：
- new-api-linux-amd64
- new-api-linux-arm64
- new-api-darwin-amd64
- new-api-darwin-arm64
- new-api-windows-amd64.exe

上传到 GitHub Release 后，install.sh 会按平台自动下载。

## 架构

```
┌─────────────┐     HTTP      ┌─────────────┐
│  浏览器/CLI  │ ───────────→ │  WebRouter   │
│             │ ←──────────── │  (Flask)     │
└─────────────┘               │  :5000       │
                              └──────┬───────┘
                                     │ HTTP + DB
                              ┌──────▼───────┐
                              │  New-API      │
                              │  (Go sidecar) │
                              │  :3000        │
                              └──────┬───────┘
                                     │
                              ┌──────▼───────┐
                              │  上游 AI API  │
                              │  OpenAI etc.  │
                              └──────────────┘
```

WebRouter 和 New-API 作为独立进程运行，通过 HTTP 和共享数据库通信。
Sidecar 架构确保两者代码隔离，不受 AGPLv3 协议传染。

## 目录结构

```
webrouter/
├── backend/              # Flask 后端
│   ├── app.py           # 应用入口
│   ├── config.py        # 配置
│   ├── models/          # 数据模型
│   ├── routes/          # API 路由（7个蓝图）
│   ├── services/        # 业务逻辑（5个服务）
│   ├── static/          # 前端 SPA
│   │   ├── index.html
│   │   ├── js/          # 原生 JS 模块
│   │   └── css/         # 暗色主题样式
│   └── start.py         # 进程管理器
├── bin/                  # New-API 二进制（安装时下载）
├── data/                 # SQLite 数据库
├── logs/                 # 运行日志
├── deploy/               # 部署配置
│   ├── install.sh       # 一键安装脚本
│   ├── update-newapi.sh # New-API 更新脚本
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── nginx.conf
├── scripts/              # 开发脚本
│   └── build-newapi.sh  # 交叉编译脚本
├── docs/                 # 项目文档
├── .env                  # 环境配置（自动生成）
├── start.sh              # 快捷启动
└── stop.sh               # 快捷停止
```

## 许可证

WebRouter 本体采用 MIT 许可证。
New-API 采用 AGPLv3 许可证，作为独立 sidecar 进程运行，不传染本项目的 MIT 协议。
