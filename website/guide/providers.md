---
title: Providers
description: Managing AI API providers in WebRouter
---

# Providers

## Overview

Providers represent your AI API sources. WebRouter supports multiple provider types, each with different capabilities.

## Provider Types

| Type | Description | Health | Latency | Cost Data | Channel Mgmt |
|------|-------------|:------:|:-------:|:---------:|:------------:|
| `direct` | Official APIs (OpenAI, Anthropic, Google, DeepSeek, etc.) | ✅ | ✅ | — | — |
| `aggregate` | Aggregator platforms (OpenRouter, API2D, OhMyGPT, etc.) | ✅ | ✅ | Manual | — |
| `newapi` | Self-hosted New-API / One-API | ✅ | ✅ | ✅ | ✅ |
| `litellm` | LiteLLM proxy | ✅ | ✅ | — | — |
| `custom` | Any OpenAI-compatible gateway | ✅ | ✅ | — | — |

## Adding a Provider

1. Navigate to **Providers** → **+ Add**
2. Select the provider type
3. Enter the Base URL and API Key
4. Specify available models (comma-separated)
5. Configure weight and priority for routing
6. Click **Save**

## Health Checks

WebRouter automatically checks provider health every 5 minutes:

- **Healthy:** Provider responds within normal latency
- **Warning:** High latency or intermittent failures
- **Dead:** Provider unresponsive, enters cooldown

Dead providers are automatically excluded from routing until they recover.

## Provider Status

| Status | Meaning |
|--------|---------|
| `healthy` | Responding normally |
| `warning` | High latency or degraded |
| `dead` | Unreachable (in cooldown) |
| `unknown` | Not yet checked |
| `auth_failed` | Authentication error |