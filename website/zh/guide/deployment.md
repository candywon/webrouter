---
title: 部署方案
description: WebRouter 生产部署方案
---

# 部署方案

## Docker Compose（推荐生产使用）

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

### 配置

在 `.env` 文件中设置环境变量：

```bash
DATABASE_URI=sqlite:///data/webrouter.db
# 或使用 MySQL：
# DATABASE_URI=mysql+pymysql://user:pass@host/webrouter
REDIS_URL=redis://redis:6379
FLASK_ENV=production
```

## Nginx 反向代理

参见 `deploy/nginx.conf`。

## 数据库选择

### SQLite（默认）

- 位置：`data/webrouter.db`
- 适合单实例部署
- 无需额外配置

### MySQL / PostgreSQL

设置 `DATABASE_URI`：

```bash
# MySQL
DATABASE_URI=mysql+pymysql://user:password@host:3306/webrouter
# PostgreSQL
DATABASE_URI=postgresql://user:password@host:5432/webrouter
```

## 调度器

生产环境默认启用健康检测和告警评估。在 debug 模式下强制启用：

```bash
ENABLE_SCHEDULER=1
```