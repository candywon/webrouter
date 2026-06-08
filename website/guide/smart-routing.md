---
title: Smart Routing
description: Complexity-based intelligent model selection and automatic provider failover
---

# Smart Routing

## Overview

WebRouter's smart routing engine analyzes every request in real-time and makes two critical decisions:

1. **Which model** — based on request complexity assessment
2. **Which provider** — based on health, latency, and availability

Set `model: "auto"` and let WebRouter handle everything.

## Complexity Assessment

When a request arrives, wr-proxy runs a multi-dimensional complexity analysis:

| Dimension | What It Detects | Impact |
|-----------|----------------|--------|
| **Input Length** | Token count of the prompt | Longer prompts → higher tier |
| **Multi-Turn Context** | Conversation depth (rounds) | Deep conversations → premium models |
| **Code Detection** | Code blocks, language keywords | Code generation → reasoning models |
| **Tool/Function Calling** | Tool definitions in request | Tool use → capable models |
| **Reasoning Indicators** | "explain", "analyze", "compare" | Analytical requests → premium |
| **System Prompt** | Length and complexity | Complex instructions → higher tier |

All thresholds are configurable via `smart_complexity_config` system setting.

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

## Model Grades

Define capability tiers for your models:

| Grade | Examples | Use Case |
|-------|----------|----------|
| `premium` | gpt-4o, claude-sonnet-4, deepseek-reasoner | Complex reasoning, code generation |
| `standard` | gpt-4o-mini, deepseek-chat, qwen-plus | General purpose |
| `economy` | claude-haiku-4-5, qwen-turbo | Simple Q&A, classification |

Configure in **Settings** → **Model Grades**.

## Model Aliases

Create stable aliases for your applications:

```
gpt4 → gpt-4o
claude-sonnet → claude-sonnet-4
deepseek-v3 → deepseek-chat
```

## Automatic Provider Switching

When a provider fails, wr-proxy handles recovery transparently:

1. **Retry** — exponential backoff, up to configurable attempts
2. **Failover** — routes to next healthy provider with the same model
3. **Cooldown** — failed providers enter 30-minute cooldown, traffic auto-shifts
4. **Smart Downgrade** — falls back to a cheaper model if the premium model is unavailable

No manual intervention needed. View cooldown status at `GET /admin/cooldowns`.

## Cost-Aware Selection

When multiple providers offer the same model, WebRouter picks based on:

1. **Health status** — healthy > warning > dead
2. **Latency** — prefers lower latency
3. **Cost** — prefers cheaper pricing when capability is equal
4. **Weight** — respects your configured provider weights

## Fallback & Retry Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `max_retry_count` | 2 | Max retry attempts per request |
| `max_failover` | 3 | Max provider failover attempts |
| `default_timeout` | 60s | Request timeout |
| `routing_strategy` | `smart` | Strategy: smart/priority/round_robin/least_latency/cost_first |
