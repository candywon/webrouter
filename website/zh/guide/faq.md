---
title: 常见问题
description: 常见问题解答
---

# 常见问题

## 通用

### WebRouter 是什么？

WebRouter 是开源 AI API 网关，统一管理多个大模型 Provider。提供健康监控、智能路由、成本追踪、团队管理和隐私脱敏。

### 适合谁用？

- 希望一把 API Key 管理多个 AI Provider 的开发者
- 需要成本可视化和额度管理的工程团队
- 需要统一管理公司 AI API 使用量的企业

## 许可证

### WebRouter 用什么许可证？

社区版（CE）使用 **Business Source License 1.1 (BSL 1.1)**。2029 年 6 月 1 日自动转为 Apache License 2.0。

### 免费使用吗？

是的。CE 免费用于个人使用、企业内部生产部署、学习研究。

### BSL 1.1 禁止什么？

- 将 WebRouter 或衍生品作为商业产品售卖
- 提供付费托管服务
- OEM 嵌入闭源产品转售

### 企业版呢？

企业版（EE）使用专有 EULA，包含 SSO、高级审计日志、集群部署、商业支持等。

## 技术

### 支持什么数据库？

SQLite（默认零配置），也支持 MySQL 和 PostgreSQL。

### 支持流式响应吗？

支持。wr-proxy 支持 SSE 流式传输。

### 支持什么 Provider 类型？

`direct`、`aggregate`、`newapi`、`oneapi`、`litellm`、`custom`——任何 OpenAI 兼容的 Provider。

## 故障排除

### 健康检测显示不可用

1. 验证 Base URL 是否正确
2. 检查 API Key 是否有效
3. 确认服务器到 Provider 的网络连通性
4. 查看 Provider 状态页是否有故障

### Token 不工作

1. 检查 Token 是否启用
2. 确认配额是否耗尽
3. 确保请求的模型在 Token 白名单中
4. 检查速率限制

### 如何重置管理员密码？

设置环境变量后重启：

```bash
WEBROUTER_ADMIN_PASSWORD=newpassword
```