---
title: 智能路由
description: 基于复杂度的智能模型调度与自动故障切换
---

# 智能路由

## 概述

wr-proxy 智能路由引擎实时分析每个请求，做出两个关键决策：

1. **选哪个模型** — 基于请求复杂度评估
2. **选哪个数据源** — 基于健康状态、延迟和可用性

设置 `model: "auto"`，全部交给 WebRouter。

## 复杂度评估

请求到达时，wr-proxy 运行多维度分析：

| 维度 | 检测内容 | 影响 |
|------|----------|------|
| **输入长度** | 提示词 Token 数 | 越长 → 越高等级 |
| **多轮对话** | 上下文轮数 | 深层对话 → premium |
| **代码检测** | 代码块、语言关键词 | 代码生成 → 推理模型 |
| **工具/函数调用** | 请求中的 tool 定义 | 工具调用 → 高能力模型 |
| **推理信号** | "解释"、"分析"、"对比" | 分析请求 → premium |
| **系统提示词** | 长度和复杂度 | 复杂指令 → 更高等级 |

所有阈值可通过 `smart_complexity_config` 系统设置配置。

```json
{
  "input_length": {"enabled": true, "levels": [
    {"max_tokens": 500, "grade": "economy"},
    {"max_tokens": 2000, "grade": "standard"},
    {"max_tokens": 999999, "grade": "premium"}
  ]},
  "multi_turn": {"enabled": true, "levels": [
    {"max_rounds": 3, "grade": "standard"},
    {"max_rounds": 999, "grade": "premium"}
  ]},
  "code_detection": {"enabled": true, "score": 25},
  "reasoning": {"enabled": true, "score": 20},
  "tools_detection": {"enabled": true, "tools_score": 30, "functions_score": 20},
  "system_prompt": {"enabled": true, "max_chars": 500, "score": 15}
}
```

## 模型分级

定义模型能力层级：

| 等级 | 示例模型 | 适用场景 |
|------|----------|----------|
| `premium` | gpt-4o, claude-sonnet-4, deepseek-reasoner | 复杂推理、代码生成 |
| `standard` | gpt-4o-mini, deepseek-chat, qwen-plus | 通用场景 |
| `economy` | claude-haiku-4-5, qwen-turbo | 简单问答、分类 |

在 **系统设置** → **模型分级** 中配置。

## 模型别名

为应用创建稳定的别名：

```
gpt4 → gpt-4o
claude-sonnet → claude-sonnet-4
deepseek-v3 → deepseek-chat
```

## 自动切换数据源

Provider 故障时，wr-proxy 自动处理：

1. **重试** — 指数退避，最多可配置次数
2. **故障转移** — 路由到下一个健康 Provider
3. **冷却** — 故障 Provider 进入 30 分钟冷却，流量自动切换
4. **智能降级** — 高端模型不可用时自动降级到经济型

无需人工干预。查看冷却状态：`GET /admin/cooldowns`。

## 成本感知选择

多 Provider 提供相同模型时，按优先级选择：

1. **健康状态** — healthy > warning > dead
2. **延迟** — 优先低延迟
3. **成本** — 能力相同时选更便宜的
4. **权重** — 遵循配置的 Provider 权重

## 降级与重试配置

| 设置项 | 默认值 | 说明 |
|--------|--------|------|
| `max_retry_count` | 2 | 每次请求最大重试次数 |
| `max_failover` | 3 | 最大 Provider 切换次数 |
| `default_timeout` | 60s | 请求超时 |
| `routing_strategy` | `smart` | 策略：smart/priority/round_robin/least_latency/cost_first |
