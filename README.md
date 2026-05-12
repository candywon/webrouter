# WebRouter - AI API 中转站一站式管理平台

> 卖铲子，不淘金。为想搭建 AI 中转站的开发者/自媒体/企业提供开箱即用的管理工具。

## 项目定位

WebRouter 不是中转站本身，而是中转站的"建站与管理工具"。用户自带上游资源（API Key、订阅账号、逆向渠道），WebRouter 提供部署、监控、计费、团队协作等全套能力。

**核心价值主张：** 把开源工具的部署复杂度和运维黑箱封装起来，让用户只管填自己的 Key 就能跑起来。

## 文档索引

| 文档 | 路径 | 内容 |
|------|------|------|
| 需求分析 | [docs/01-requirements.md](docs/01-requirements.md) | 用户画像、痛点分析、功能需求 |
| 产品方案 | [docs/02-product-plan.md](docs/02-product-plan.md) | 功能设计、定价策略、获客路径 |
| 技术架构 | [docs/03-architecture.md](docs/03-architecture.md) | 技术选型、系统架构、部署方案 |

## 技术栈

- **后端：** Python (Flask) + New-API (Go, Docker)
- **前端：** 原生 HTML/CSS/JS（与包装外贸站技术栈一致）
- **数据库：** SQLite (开发) / MySQL (生产)，复用 New-API 数据库
- **部署：** Docker Compose 一键部署
- **监控：** 自建轻量监控 + 告警

## 目录结构

```
webrouter/
├── docs/                    # 项目文档
├── frontend/                # 前端静态文件
│   ├── index.html           # 管理后台入口
│   ├── css/
│   ├── js/
│   └── assets/
├── backend/                 # Flask 后端
│   ├── app.py               # 主应用
│   ├── config.py            # 配置
│   ├── models/              # 数据模型
│   ├── routes/              # API 路由
│   ├── services/            # 业务逻辑
│   └── static/              # 静态资源
├── deploy/                  # 部署配置
│   ├── docker-compose.yml
│   ├── Dockerfile.flask
│   └── nginx/
├── scripts/                 # 运维脚本
│   ├── install.sh           # 一键部署脚本
│   ├── backup.sh            # 备份脚本
│   └── monitor.py           # 监控脚本
└── README.md
```
