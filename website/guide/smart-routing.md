---
title: Smart Routing
description: How WebRouter intelligently routes requests to the best model
---

# Smart Routing

## Overview

WebRouter's smart routing feature automatically selects the optimal model based on request complexity. Instead of hardcoding a model name, you can set `model: "auto"` and let WebRouter decide.

## How It Works

When a request comes in with `model: "auto"`, WebRouter:

1. **Analyzes** the request — prompt length, complexity indicators, system prompt presence
2. **Scores** each available model on capability vs. cost
3. **Selects** the best match: simple requests get fast/cheap models, complex reasoning gets powerful ones
4. **Routes** the request to the selected provider

## Model Grades

Model grades let you define tiers of model capability:

| Grade | Examples | Use Case |
|-------|----------|----------|
| `premium` | gpt-4o, claude-sonnet-4 | Complex reasoning, code generation |
| `standard` | gpt-4o-mini, deepseek-chat | General purpose |
| `economy` | qwen-turbo, claude-haiku | Simple Q&A, classification |

Configure model grades in **Settings** → **Model Grades**.

## Model Aliases

Create aliases for common model names:

```yaml
gpt4 → gpt-4o
claude-sonnet → claude-sonnet-4
deepseek-v3 → deepseek-chat
```

This allows you to use short, stable names in your application while WebRouter maps them to the actual provider model names.

## Fallback & Retry

When a provider fails, WebRouter automatically:

1. **Retries** the request with exponential backoff (up to 3 attempts)
2. **Falls back** to the next available provider with the same model
3. **Smart downgrade** — if configured, uses a cheaper model to complete the request