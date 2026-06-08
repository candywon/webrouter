---
title: API 参考
description: WebRouter REST API 接口文档
---

# API 参考

## 基础 URL

所有 API 端点前缀为 `/api/`。

## 认证

### `POST /api/auth/login`

### `GET /api/auth/check`

## 仪表盘

### `GET /api/dashboard/stats`

## 数据源

### `GET /api/providers` — 列表
### `POST /api/providers` — 创建
### `GET /api/providers/<id>` — 详情
### `PUT /api/providers/<id>` — 更新
### `DELETE /api/providers/<id>` — 删除
### `POST /api/providers/<id>/check` — 触发健康检测

## 渠道管理

### `GET /api/providers/<id>/channels`
### `POST /api/providers/<provider_id>/channels`

## 监控

### `GET /api/monitor/health`
### `GET /api/monitor/history/<provider_id>`

## 告警

### `GET /api/alerts/rules`
### `POST /api/alerts/rules`
### `PUT /api/alerts/rules/<id>`
### `DELETE /api/alerts/rules/<id>`
### `GET /api/alerts/history`

## 计费

### `GET /api/billing/stats`
### `GET /api/billing/requests`
### `GET /api/billing/export`

## 令牌

### `GET /api/tokens`
### `POST /api/tokens`
### `PUT /api/tokens/<id>`
### `DELETE /api/tokens/<id>`
### `POST /api/tokens/<id>/reset-key`

## 团队

### `GET /api/team`
### `POST /api/team`
### `PUT /api/team/<id>`
### `DELETE /api/team/<id>`

## 系统设置

### `GET /api/settings`
### `PUT /api/settings`

## 模型定价 / 分级 / 别名

### `GET/PUT /api/pricing`
### `GET/POST/DELETE /api/modelgrades`
### `GET/POST/DELETE /api/modelaliases`

## 脱敏规则

### `GET/POST /api/desensitize`
### `PUT/DELETE /api/desensitize/<id>`

## CLI 导出

### `GET /api/cli/export`

## Demo

### `GET /api/demo/status`
### `POST /api/demo/reset`
### `GET /api/demo/auto-login`

## 代理网关

### `POST /v1/chat/completions`
### `GET /health`