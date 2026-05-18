# WebRouter — AI-API 综合管理平台

> 一站式 AI API 网关管理：多源聚合、智能路由、健康监控、成本计费、团队协作

## 功能概览

| 模块 | 功能 |
|------|------|
| 数据源管理 | 注册直连/聚合等多种 Provider，统一纳管所有 API 源 |
| 仪表盘 | 系统概览、请求统计、Provider 健康一览 |
| 渠道管理 | Provider 下渠道详情、负载均衡、优先级 |
| 健康监控 | 实时检测 Provider 可用性、延迟追踪、自动降级 |
| 告警中心 | 自定义告警规则、多级通知、历史记录 |
| 成本计费 | 用量统计、额度管理、费用报表 |
| 团队管理 | 成员邀请、配额分配 |
| 令牌管理 | API Key 生成、模型白名单、智能降级、脱敏 |
| CLI 生成 | 一键生成渠道配置命令行 |
| 系统设置 | 代理网关开关、路由策略、超时重试 |

## 支持的 Provider 类型

| 类型 | 说明 | 数据能力 |
|------|------|---------|
| `direct` | 大模型官方直连（OpenAI/Claude/Gemini） | 健康 + 延迟 |
| `aggregate` | 聚合平台（AnyRoute/OhMyGPT/API2D等） | 健康 + 延迟 + 手动成本 |
| `litellm` | 自建 LiteLLM 代理 | 健康 + 延迟 + 模型列表 |
| `custom` | 自研/其他 OpenAI 兼容网关 | 健康 + 延迟 |

## 快速安装

### 系统要求

- Python 3.8+
- Go 1.21+（用于编译 wr-proxy，如已提供二进制可跳过）
- 2GB+ 内存

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
4. 编译 wr-proxy（如 Go 可用）
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

## 进程管理

WebRouter 使用 `start.py` 管理双进程（WebRouter Flask + wr-proxy Go）：

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
| DATABASE_URI | 数据库连接 | SQLite |
| REDIS_URL | Redis 连接（可选） | 空 |
| FLASK_ENV | 运行环境 | production |
| FLASK_HOST | 监听地址 | 0.0.0.0 |

## 架构

```
┌─────────────┐     HTTP      ┌─────────────────┐
│  浏览器/CLI  │ ───────────→ │    WebRouter     │
│             │ ←──────────── │    (Flask)       │
└─────────────┘               │    :5050         │
                              └──────┬──────────┘
                                     │
                              ┌──────▼──────┐
                              │  wr-proxy    │
                              │  (Go) :5051  │
                              └──────┬──────┘
                                     │
                     ┌───────────────┼───────────────┐
                     │               │               │
              ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
              │   direct    │ │  aggregate  │ │   custom    │
              │  (官方直连)  │ │  (聚合平台)  │ │ (自定义网关) │
              └─────────────┘ └─────────────┘ └─────────────┘
```

wr-proxy 是自主研发的高性能 Go 代理网关，支持智能路由、请求脱敏、自动重试、成本计费等能力。WebRouter Flask 后台通过共享 SQLite 数据库与 wr-proxy 协同工作。

## 目录结构

```
webrouter/
├── backend/              # Flask 后端
│   ├── app.py           # 应用入口
│   ├── config.py        # 配置
│   ├── models/          # 数据模型
│   ├── routes/          # API 路由蓝图
│   ├── services/        # 业务逻辑
│   ├── static/          # 前端 SPA（Hash路由）
│   │   ├── index.html
│   │   ├── js/
│   │   └── css/
│   └── start.py         # 进程管理器
├── wr-proxy/             # Go 代理网关
│   ├── main.go          # 入口
│   ├── handlers.go      # 请求处理
│   ├── proxy.go         # HTTP 代理转发
│   ├── retry.go         # 重试引擎
│   ├── desensitize.go   # 脱敏引擎
│   ├── meter.go         # 成本计量
│   └── data/            # SQLite 数据库
├── data/                 # 运行数据
├── logs/                 # 运行日志
├── deploy/               # 部署配置
│   ├── install.sh       # 一键安装脚本
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── nginx.conf
├── docs/                 # 项目文档
└── .env                  # 环境配置（自动生成）
```

## 许可证

WebRouter 采用 MIT 许可证。
