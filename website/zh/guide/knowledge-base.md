---
title: 知识库
description: 企业知识管理与 RAG 驱动的上下文注入
---

# 知识库

## 概述

WebRouter 的知识库系统捕获、存储和检索组织知识，并自动将相关上下文注入 LLM 请求。它结合了自动对话知识捕获与 RAG（检索增强生成）技术，实现实时上下文注入。

## 架构

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  知识捕获      │     │  知识提取      │     │  RAG 注入     │
│  (自动)       │     │  (LLM)       │     │  (实时)       │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       ▼                    ▼                    ▼
┌──────────────────────────────────────────────────────┐
│                  wr_knowledge_raw                     │
│  从对话中捕获的原始对话片段                               │
└──────────────────────┬───────────────────────────────┘
                       │ LLM 提取
                       ▼
┌──────────────────────────────────────────────────────┐
│                  wr_agent_memory                      │
│  从原始数据提取的结构化事实和偏好                           │
└──────────────────────┬───────────────────────────────┘
                       │ 向量嵌入
                       ▼
┌──────────────────────────────────────────────────────┐
│                  wr_knowledge_vectors                 │
│  用于语义搜索的向量嵌入                                  │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
               RAG 上下文注入
           （注入到 system prompt）
```

## 核心组件

### 1. 知识捕获（自动）

当 Token 启用了 `KnowledgeCaptureEnabled`，每轮对话自动捕获：

- **非流式**：完整请求/响应对在完成后保存
- **流式**：累积内容在流结束后保存
- 内容存储在 `wr_knowledge_raw` 中供后续提取

在 **令牌管理** → **知识捕获** 中按 Token 启用。

### 2. 知识提取（LLM 驱动）

捕获的原始对话由 LLM 处理，提取结构化知识：

- 事实、偏好、决策和流程
- 存储为 `wr_agent_memory` 中的结构化条目
- 支持批量提取和增量处理

触发提取：
- **管理面板**：知识库 → 提取
- **API**：`POST /admin/knowledge_extract`

### 3. RAG 注入（实时）

每次请求前，若 Token 启用了 RAG：

1. 从请求中提取用户查询
2. 向量化并搜索 `wr_knowledge_vectors`
3. 检索 Top-K 相关片段
4. 将匹配的上下文注入 system prompt

```
System: 你是一个有用的助手。

[知识库上下文]
来自知识库的相关信息：
- 事实 1：...
- 事实 2：...

User: 我们的部署流程是什么？
```

## Token 配置项

| 设置项 | 默认值 | 说明 |
|--------|--------|------|
| `rag_enabled` | false | 启用 RAG 上下文注入 |
| `rag_top_k` | 3 | 检索片段数量 |
| `rag_min_relevance` | 0.7 | 最低相似度阈值 |
| `rag_hybrid_alpha` | 0 | 混合搜索权重（0 = 纯向量，>0 = BM25 + 向量） |
| `rag_reranker` | none | 结果重排序模型 |
| `system_prompt_knowledge` | "" | 始终注入的静态知识 |
| `knowledge_department` | "" | 部门过滤，限定搜索范围 |
| `knowledge_capture_enabled` | false | 自动捕获对话 |

## 分块策略

文档在嵌入前按可配置策略分块：

| 策略 | 说明 |
|------|------|
| `fixed` | 按固定字符数切分，带重叠 |
| `sentence` | 按句子边界切分 |
| `paragraph` | 按段落边界切分 |

## 混合搜索

当 `rag_hybrid_alpha > 0` 时，wr-proxy 结合：

- **向量搜索** — 通过嵌入进行语义相似度匹配
- **BM25 关键词搜索** — 精确词项匹配

alpha 参数控制混合比例（0 = 纯向量，1 = 纯 BM25）。

## 部门范围

为多团队部署组织知识：

1. 为每个 Token 设置 `knowledge_department`（如 "engineering"、"marketing"）
2. RAG 搜索范围限定在 Token 所属部门
3. 部门搜索结果为空时，回退到全局搜索

## 管理端点

| 端点 | 说明 |
|------|------|
| `GET /admin/knowledge_stats` | 知识库统计 |
| `POST /admin/knowledge_analyze` | 分析知识质量 |
| `POST /admin/knowledge_extract` | 触发 LLM 提取 |
| `POST /admin/knowledge_embedding_backfill` | 回填向量嵌入 |
| `GET /admin/knowledge_rag_stats` | RAG 命中/未命中统计 |
| `GET /admin/knowledge_prompt_preview` | 预览注入的 prompt |
