---
title: 配置说明
description: 环境变量和系统设置参考
---

# 配置说明

## 环境变量

### 核心设置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SESSION_SECRET` | Flask 会话密钥 | 自动生成 |
| `DATABASE_URI` | 数据库连接 | `sqlite:///data/webrouter.db` |
| `REDIS_URL` | Redis 连接（可选，缓存用） | `redis://localhost:6379/0` |
| `FLASK_ENV` | 运行环境 | `production` |
| `TZ` | 时区 | `Asia/Shanghai` |

### 端口配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WR_PORT` | Flask 管理面板端口 | `5050` |
| `WR_PROXY_PORT` | wr-proxy 网关端口 | `5051` |

### 调度器

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ENABLE_SCHEDULER` | Debug 模式强制启用调度器 | `0` |
| `HEALTH_CHECK_INTERVAL` | 健康检测间隔（秒） | `300` |
| `BALANCE_CHECK_INTERVAL` | 余额检测间隔（秒） | `1800` |

### 告警

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ALERT_COOLDOWN` | 告警冷却时间（秒） | `300` |

### 管理员账号

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WEBROUTER_ADMIN_USER` | 默认管理员用户名 | `admin` |
| `WEBROUTER_ADMIN_PASSWORD` | 默认管理员密码 | `admin123456` |

## Demo 模式

设置 `WEBROUTER_DEMO=1` 启用 Demo 模式：
- 自动登录 demo/demo123456
- 核心数据只读保护
- 预置种子数据