---
title: 数据源管理
description: Provider 类型和配置
---

# 数据源管理

## 支持的类型

| 类型 | 说明 | 健康 | 延迟 | 成本 | 渠道管理 |
|------|------|:----:|:----:|:----:|:--------:|
| `direct` | 官方直连（OpenAI、Claude、Gemini、DeepSeek 等） | ✅ | ✅ | — | — |
| `aggregate` | 聚合平台（OpenRouter、API2D、OhMyGPT 等） | ✅ | ✅ | 手动 | — |
| `litellm` | LiteLLM 代理 | ✅ | ✅ | — | — |
| `custom` | 任何 OpenAI 兼容网关 | ✅ | ✅ | — | — |

## 添加数据源

1. 进入 **数据源** → **+ 添加**
2. 选择 Provider 类型
3. 填写 Base URL 和 API Key
4. 指定可用模型
5. 配置权重和优先级
6. 点击 **保存**

## 健康检测

WebRouter 每 5 分钟自动检测 Provider 健康状态：

- **健康：** 正常响应
- **警告：** 延迟较高或偶发失败
- **不可用：** 无响应，进入冷却

不可用的 Provider 自动排除出路由。

## Provider 状态

| 状态 | 含义 |
|------|------|
| `healthy` | 正常 |
| `warning` | 高延迟或性能下降 |
| `dead` | 不可达（冷却中） |
| `unknown` | 尚未检测 |
| `auth_failed` | 认证错误 |