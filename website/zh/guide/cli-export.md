---
title: CLI 导出
description: 生成开发工具配置
---

# CLI 导出

## 概述

一键生成主流 AI 开发工具的配置。

## 支持的工具

| 工具 | 配置格式 |
|------|----------|
| Claude Code | 环境变量 |
| Codex | CLI 参数 |
| Cursor | `~/.cursor/config.json` |
| Continue | `~/.continue/config.json` |

## 使用方法

1. 进入 **CLI 导出**
2. 选择 Token
3. 点击 **生成**
4. 复制配置

## 示例：Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:5051
export ANTHROPIC_API_KEY=<your-webrouter-token>
```

## 示例：Cursor

```json
{
  "baseUrl": "http://localhost:5051",
  "apiKey": "<your-webrouter-token>"
}
```