---
title: 智能路由
description: 自动选择最优模型
---

# 智能路由

## 概述

智能路由功能根据请求复杂度自动选择最优模型。设置 `model: "auto"`，WebRouter 自动决策。

## 工作原理

当请求携带 `model: "auto"` 时，WebRouter：

1. **分析** 请求——提示长度、复杂度指标、系统提示词
2. **评分** 每个可用模型的能力与成本
3. **选择** 最匹配的模型：简单请求走经济型，复杂推理走高端模型
4. **路由** 请求到选中的 Provider

## 模型分级

定义能力层级：

| 等级 | 示例模型 | 适用场景 |
|------|----------|----------|
| `premium` | gpt-4o, claude-sonnet-4 | 复杂推理、代码生成 |
| `standard` | gpt-4o-mini, deepseek-chat | 通用场景 |
| `economy` | qwen-turbo, claude-haiku | 简单问答、分类 |

## 模型别名

创建常用模型别名：

```
gpt4 → gpt-4o
claude-sonnet → claude-sonnet-4
deepseek-v3 → deepseek-chat
```

在应用中使用简短稳定的名称，WebRouter 自动映射到实际模型。

## 降级与重试

Provider 失败时自动：

1. **重试** — 指数退避（最多 3 次）
2. **回退** — 下一个可用 Provider
3. **智能降级** — 自动切换到经济型模型完成请求