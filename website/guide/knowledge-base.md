---
title: Knowledge Base
description: Enterprise knowledge management with RAG-powered context injection
---

# Knowledge Base

## Overview

WebRouter's knowledge base system captures, stores, and retrieves organizational knowledge — then automatically injects relevant context into LLM requests. It combines automatic knowledge capture from conversations with RAG (Retrieval-Augmented Generation) for real-time context injection.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Knowledge    │     │  Knowledge    │     │  RAG          │
│  Capture      │     │  Extraction   │     │  Injection    │
│  (auto)       │     │  (LLM)        │     │  (real-time)  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       ▼                    ▼                    ▼
┌──────────────────────────────────────────────────────┐
│                  wr_knowledge_raw                     │
│  Raw conversation snippets captured from chat         │
└──────────────────────┬───────────────────────────────┘
                       │ LLM extraction
                       ▼
┌──────────────────────────────────────────────────────┐
│                  wr_agent_memory                      │
│  Structured facts & preferences extracted from raw    │
└──────────────────────┬───────────────────────────────┘
                       │ Embedding
                       ▼
┌──────────────────────────────────────────────────────┐
│                  wr_knowledge_vectors                 │
│  Vector embeddings for semantic search                │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
               RAG Context Injection
           (injected into system prompt)
```

## Key Components

### 1. Knowledge Capture (Automatic)

When a token has `KnowledgeCaptureEnabled`, every conversation turn is automatically captured:

- **Non-streaming**: Full request/response pairs saved after completion
- **Streaming**: Accumulated content saved after stream ends
- Content is stored in `wr_knowledge_raw` for later extraction

Enable per token under **Tokens** → **Knowledge Capture**.

### 2. Knowledge Extraction (LLM-Powered)

Raw captured conversations are processed by an LLM to extract structured knowledge:

- Facts, preferences, decisions, and procedures
- Stored in `wr_agent_memory` as structured entries
- Supports batch extraction and incremental processing

Trigger extraction via:
- **Admin panel**: Knowledge → Extract
- **API**: `POST /admin/knowledge_extract`

### 3. RAG Injection (Real-Time)

Before each request, if RAG is enabled for the token:

1. Extract the user's query from the request
2. Vectorize and search `wr_knowledge_vectors`
3. Retrieve top-K relevant chunks
4. Inject matching context into the system prompt

```
System: You are a helpful assistant.

[Knowledge Base Context]
Relevant information from the knowledge base:
- Fact 1: ...
- Fact 2: ...

User: What's our deployment process?
```

## Configuration Per Token

| Setting | Default | Description |
|---------|---------|-------------|
| `rag_enabled` | false | Enable RAG context injection |
| `rag_top_k` | 3 | Number of chunks to retrieve |
| `rag_min_relevance` | 0.7 | Minimum similarity threshold |
| `rag_hybrid_alpha` | 0 | Hybrid search weight (0 = pure vector, >0 = BM25 + vector) |
| `rag_reranker` | none | Reranking model for result refinement |
| `system_prompt_knowledge` | "" | Static knowledge to always inject |
| `knowledge_department` | "" | Department filter for domain-scoped search |
| `knowledge_capture_enabled` | false | Auto-capture conversations |

## Chunking Strategies

Documents are chunked before embedding using configurable strategies:

| Strategy | Description |
|----------|-------------|
| `fixed` | Split by fixed character count with overlap |
| `sentence` | Split at sentence boundaries |
| `paragraph` | Split at paragraph boundaries |

## Hybrid Search

When `rag_hybrid_alpha > 0`, wr-proxy combines:

- **Vector search** — semantic similarity via embeddings
- **BM25 keyword search** — exact term matching

The alpha parameter controls the blend (0 = pure vector, 1 = pure BM25).

## Department Scoping

Organize knowledge by department for multi-team deployments:

1. Set `knowledge_department` on each token (e.g., "engineering", "marketing")
2. RAG searches are scoped to the token's department
3. Falls back to global search if department results are empty

## Management Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /admin/knowledge_stats` | Knowledge base statistics |
| `POST /admin/knowledge_analyze` | Analyze knowledge quality |
| `POST /admin/knowledge_extract` | Trigger LLM extraction |
| `POST /admin/knowledge_embedding_backfill` | Backfill vector embeddings |
| `GET /admin/knowledge_rag_stats` | RAG hit/miss statistics |
| `GET /admin/knowledge_prompt_preview` | Preview injected prompt |
