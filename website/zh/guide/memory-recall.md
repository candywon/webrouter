---
title: 会话记忆召回
description: 通过 @recall 实现会话上下文自动恢复
---

# 会话记忆召回

## 概述

当客户端因重启、网络中断或设备切换丢失上下文时，WebRouter 的会话记忆召回机制自动从服务端恢复对话历史，注入回请求中。

无需客户端存储，无需复杂的状态管理，只需一个简单的触发。

## 工作原理

```
客户端断开
     │
     ▼
┌─────────────┐     POST /v1/chat/completions     ┌──────────────┐
│   客户端      │ ────────────────────────────────→ │   wr-proxy    │
│ (无历史记录)  │    @recall 还记得之前聊的...        │               │
└─────────────┘                                    └──────┬────────┘
                                                          │
                                                   ┌──────▼────────┐
                                                   │ wr_session_    │
                                                   │ messages       │
                                                   │ (SQLite)       │
                                                   └──────┬────────┘
                                                          │
                                                   ┌──────▼────────┐
                                                   │ 将历史注入      │
                                                   │ messages 数组   │
                                                   │ (Token预算感知) │
                                                   └──────┬────────┘
                                                          │
                                                          ▼
                                                    上游 LLM
```

## 触发方式

### 1. `@recall` 魔术词

在用户消息中任意位置包含 `@recall`，wr-proxy 会检测并剥离，然后注入会话历史。

```
User: @recall 我们之前讨论的第三点是什么？
```

`@recall` 标记在消息到达上游模型前被移除。

### 2. 自然语言触发

当客户端无上下文（messages 中没有 assistant 回复）时，以下短语自动触发召回：

| 短语 |
|------|
| 还记得 |
| 继续上次 |
| 继续聊 |
| 接着说 |
| 之前聊过 |
| 上次说到 |
| 接着上次 |

### 3. HTTP Header

发送 `X-Recall-Session: <session_id>` 显式触发特定会话的召回。

## Token 预算控制

召回的历史消息受 Token 预算限制（默认 8000 tokens）。wr-proxy 从最新到最旧累积消息，直到预算用完，确保最近上下文优先保留。

```
最近轮次  ←  优先注入（在预算内）
早期轮次  ←  超出预算时省略
系统提示词 ←  始终保留在顶部
```

通过系统设置 `session_recall_budget` 配置。

## 会话消息存储

每轮对话（user + assistant 对）异步持久化到 `wr_session_messages`：

| 字段 | 说明 |
|------|------|
| `session_id` | 唯一会话标识（来自 `X-Session-Id` 头） |
| `token_id` | 所属 Token |
| `turn_index` | 会话内递增轮次号 |
| `role` | `user` 或 `assistant` |
| `content` | 消息内容（已脱敏） |
| `model` | 本次响应用到的模型 |

## 保留策略

旧会话根据 `session_recall_retention_days` 自动清理（默认 30 天），每 6 小时执行一次。

## 启用会话召回

按 Token 配置：

1. 进入 **令牌管理** → 选择 Token
2. 启用 **会话记忆召回**
3. 客户端请求需携带 `X-Session-Id` 头

```bash
curl http://localhost:5051/v1/chat/completions \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -H "X-Session-Id: my-session-001" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "@recall 继续我们的讨论"}]}'
```

## 架构说明

- **异步落盘**：消息通过缓冲 channel + 单 goroutine 顺序写入，对请求转发零延迟影响
- **脱敏优先**：内容在脱敏规则处理后存储
- **Token 隔离**：每个 Token 的会话严格隔离，不可跨 Token 召回
- **轮次完整性**：turn_index 按会话顺序分配，防止乱序
