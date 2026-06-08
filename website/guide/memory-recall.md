---
title: Session Memory Recall
description: Automatic conversation context recovery with @recall
---

# Session Memory Recall

## Overview

When a client loses context — due to a restart, network interruption, or switching devices — WebRouter's session memory recall mechanism automatically recovers conversation history from the server and injects it back into the request.

No client-side storage needed. No complex state management. Just a simple trigger.

## How It Works

```
Client disconnects
     │
     ▼
┌─────────────┐     POST /v1/chat/completions     ┌──────────────┐
│   Client     │ ────────────────────────────────→ │   wr-proxy    │
│ (no history) │    @recall 还记得之前聊的...        │               │
└─────────────┘                                    └──────┬────────┘
                                                          │
                                                   ┌──────▼────────┐
                                                   │ wr_session_    │
                                                   │ messages       │
                                                   │ (SQLite)       │
                                                   └──────┬────────┘
                                                          │
                                                   ┌──────▼────────┐
                                                   │ Inject history │
                                                   │ into messages  │
                                                   │ (token budget  │
                                                   │  aware)        │
                                                   └──────┬────────┘
                                                          │
                                                          ▼
                                                    Upstream LLM
```

## Trigger Methods

### 1. `@recall` Magic Word

Include `@recall` anywhere in your user message. wr-proxy detects it, strips it from the message, and injects the session history.

```
User: @recall what was the third point from our earlier discussion?
```

The `@recall` token is removed before the message reaches the upstream model.

### 2. Natural Language Phrases

For Chinese-language sessions, these phrases also trigger recall (when the client has no assistant messages in context):

| Phrase | Meaning |
|--------|---------|
| 还记得 | Do you remember |
| 继续上次 | Continue from last time |
| 继续聊 | Keep chatting |
| 接着说 | Go on |
| 之前聊过 | We talked before |
| 上次说到 | Last time we mentioned |
| 接着上次 | Following up from last time |

### 3. HTTP Header

Send `X-Recall-Session: <session_id>` to explicitly trigger recall for a specific session.

## Token Budget Control

Recalled history is injected with a configurable token budget (default: 8000 tokens). wr-proxy accumulates messages from newest to oldest until the budget is exhausted, ensuring the most recent context is preserved.

```
Recent turns  ←  Injected first (within budget)
Old turns     ←  Omitted if budget exceeded
System prompt ←  Always preserved at top
```

Configure via system setting `session_recall_budget`.

## Session Message Storage

Every turn (user + assistant pair) is persisted asynchronously to `wr_session_messages`:

| Field | Description |
|-------|-------------|
| `session_id` | Unique session identifier (from `X-Session-Id` header) |
| `token_id` | Token that owns this session |
| `turn_index` | Monotonic turn number within the session |
| `role` | `user` or `assistant` |
| `content` | Message content (post-desensitization) |
| `model` | Model used for the assistant response |

## Retention Policy

Old sessions are automatically cleaned up based on `session_recall_retention_days` (default: 30 days). The cleanup runs every 6 hours.

## Enabling Session Recall

Per token configuration:

1. Go to **Tokens** → select a token
2. Enable **Session Recall**
3. Client must send `X-Session-Id` header with requests

```bash
curl http://localhost:5051/v1/chat/completions \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -H "X-Session-Id: my-session-001" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "@recall continue our discussion"}]}'
```

## Architecture Notes

- **Async persistence**: Messages are written via a buffered channel + single goroutine worker, ensuring zero latency impact on request forwarding
- **Desensitization-first**: Content is stored after desensitization rules are applied
- **Token isolation**: Each token's sessions are strictly isolated — no cross-token recall
- **Multi-turn integrity**: Turn index is assigned sequentially per session to prevent ordering issues
