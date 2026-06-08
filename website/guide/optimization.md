---
title: Optimization Features
description: wr-proxy request optimization — token compression, session compression, and dynamic content reordering
---

# Optimization Features

## Overview

wr-proxy includes built-in request optimization features that transform outgoing requests before they reach upstream providers — reducing token costs, improving cache hit rates, and handling long conversations gracefully.

All features are controlled via system settings and can be toggled independently without restarting wr-proxy.

## Feature Toggles

| Feature | Setting Key | Default | Description |
|---------|------------|---------|-------------|
| Dynamic Content Reordering | `feature_dynamic_content_last` | off | Move dynamic content to end of context |
| Token Compression | `feature_token_compression` | off | Compress long system prompts |
| Session Compression | `feature_session_compression` | off | Summarize long conversation history |

Toggle via **Settings** in the admin panel or `PUT /api/settings`.

## 1. Dynamic Content Reordering

### Problem

When dynamic content (URLs, timestamps, hash values) appears early in the conversation, it prevents upstream prompt caching from working effectively. Each request looks "different" even when the core instruction hasn't changed.

### Solution

wr-proxy detects messages containing dynamic patterns and reorders them within their role group — pushing dynamic messages to the end:

| Pattern | Example |
|---------|---------|
| URLs | `https://api.example.com/v1/...` |
| Dates | `2026-06-07` |
| Times | `14:30:00` |
| Hashes/UUIDs | `a1b2c3d4e5f6...` |
| Long numbers | `1234567890123` |

```
Before:                          After:
[system: "You are..."]           [system: "You are..."]
[user: "Check https://..."]      [user: "What's the status?"]
[user: "What's the status?"]     [user: "Check https://..."]  ← moved to end
```

Result: the static prefix stays cacheable upstream.

## 2. Token Compression (RTK)

### Problem

Long system prompts (2000+ characters) consume tokens on every request, even though the instruction rarely changes.

### Solution

When enabled, system prompts exceeding the threshold are compressed in-place:

```
Original (3000 chars):
"You are an expert code reviewer. Follow these guidelines:
 1. Check for security vulnerabilities...
 2. Verify error handling...
 ... (2000 more characters) ...
 99. Ensure consistent naming..."

Compressed:
"You are an expert code reviewer. Follow these guidelines:
 1. Check for security vulnerabilities...
 2. Verify error handling...

[... 2300 characters omitted ...]

 98. Use type hints consistently.
 99. Ensure consistent naming."
```

The model receives a compact representation while preserving the critical first and last instructions.

## 3. Session Compression

### Problem

Long conversations (10+ turns) accumulate significant token costs. Early messages may be irrelevant to the current topic.

### Solution

When a conversation exceeds the compression threshold, wr-proxy:

1. Preserves the most recent 5 turns intact
2. Summarizes earlier turns into a concise system message
3. Injects the summary before the recent messages

```
Before (15 turns):                 After:
[system: "You are..."]             [system: "You are..."]
[turn 1: user/assistant]           [system: "[Summary of turns 1-10]
[turn 2: user/assistant]            共 10 轮对话，约 5000 字符。
 ...                               主要话题：部署流程、数据库配置..."]
[turn 10: user/assistant]          [turn 11: user/assistant]  ← preserved
[turn 11: user/assistant]          [turn 12: user/assistant]
 ...                               ...
[turn 15: user/assistant]          [turn 15: user/assistant]
```

## Session Context Awareness

Beyond the three toggleable features, wr-proxy's smart model selection also considers session context:

- **Large context** (>16K tokens accumulated): complexity score +0.15, may trigger premium model selection
- **Medium context** (>8K tokens): complexity score +0.08

This ensures that as conversations grow deeper, the model selection adapts accordingly.

## API Compatibility

All three optimization features also support Anthropic (`/v1/messages`) and Cohere (`/v1/chat`) API formats through format translation before optimization is applied.

## Admin Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /admin/features` | View current feature toggle states |
| `POST /admin/features` | Reload feature toggles from DB |
| `POST /admin/conversation_compress` | Manually trigger conversation compression |
