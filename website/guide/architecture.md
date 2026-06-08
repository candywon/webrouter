---
title: Architecture
description: How WebRouter works under the hood
---

# Architecture

## Component Overview

WebRouter consists of two main components that work together:

```
┌─────────────┐     HTTP      ┌─────────────────┐
│   Browser   │ ───────────→ │    WebRouter     │
│   / CLI     │ ←──────────── │    (Flask)       │
└─────────────┘               │    :5050         │
                              └──────┬──────────┘
                                     │
                              ┌──────▼──────┐
                              │  wr-proxy    │
                              │  (Go) :5051  │
                              └──────┬──────┘
                                     │
                     ┌───────────────┼───────────────┐
                     │               │               │
              ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
              │   direct    │ │  aggregate  │ │   custom    │
              │  (Official) │ │ (Aggregator)│ │ (Any OpenAI)│
              └─────────────┘ └─────────────┘ └─────────────┘
```

## Components

### WebRouter (Flask Backend)

The admin panel and REST API server:

- **Port:** 5050
- **Language:** Python (Flask)
- **Responsibilities:** Provider management, token management, health monitoring, billing, team management, settings
- **Database:** SQLite (default) or MySQL/PostgreSQL
- **Scheduler:** APScheduler for periodic health checks and alert evaluation

### wr-proxy (Go Proxy Gateway)

The high-performance request proxy:

- **Port:** 5051
- **Language:** Go 1.22
- **Responsibilities:** Request forwarding, smart routing, retry with backoff, privacy desensitization, streaming, cost metering
- **Database:** Shares the same SQLite database as the Flask backend

## Data Flow

1. **Client sends request** to wr-proxy (`:5051/v1/chat/completions`) with a WebRouter API key
2. **wr-proxy authenticates** the token and selects the best provider based on the model or `auto` routing
3. **Desensitization** strips PII from the request body
4. **Forward** the request to the upstream provider
5. **Meter** token usage and cost
6. **Return** the response to the client, re-sensitizing if needed

## Database

Both components share the same SQLite database file, allowing wr-proxy to read provider/token configuration and write request logs without needing a separate API layer.

## Scheduler

The Flask backend runs two periodic tasks via APScheduler:

| Task | Interval | Description |
|------|----------|-------------|
| Health Check | 5 minutes | Pings all providers and records latency/status |
| Alert Evaluation | 1 minute | Evaluates alert rules against recent health data |