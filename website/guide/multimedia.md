---
title: Multimedia Support
description: Handle audio, image, and video content through wr-proxy
---

# Multimedia Support

## Overview

wr-proxy provides first-class support for multimedia API endpoints — audio synthesis, speech transcription, image generation, and image editing — all routed through the same unified gateway with automatic failover and provider selection.

## Supported Endpoints

| Endpoint | Method | Content Type | Description |
|----------|--------|-------------|-------------|
| `/v1/audio/speech` | POST | JSON → Binary Audio | Text-to-speech synthesis |
| `/v1/audio/transcriptions` | POST | Multipart → JSON | Speech-to-text transcription |
| `/v1/images/generations` | POST | JSON → JSON | Image generation (DALL·E, etc.) |
| `/v1/images/edits` | POST | Multipart → JSON | Image editing |
| `/v1/images/variations` | POST | Multipart → JSON | Image variations |

## How Binary Content Is Handled

Multimedia endpoints use a dedicated fast path (`handleBinaryProxy`) that differs from the standard chat completions flow:

1. **Authentication** — same token-based auth as chat endpoints
2. **Model extraction** — parsed from JSON body or multipart form field
3. **Provider selection** — uses the same router with health-aware selection
4. **Binary passthrough** — response body is streamed directly without JSON parsing

### What's Skipped

The binary proxy intentionally skips these steps (not applicable to binary content):
- Complexity assessment (no text to analyze)
- Desensitization (no PII in audio/image bytes)
- Request sanitization (format is endpoint-specific)
- Knowledge capture (binary content not capturable)

### What's Preserved

- Token authentication and model whitelist checks
- Provider health-aware selection with failover
- Cooldown mechanism (quota exhaustion, rate limiting)
- Request logging and cost metering
- Response Content-Type passthrough

## Audio Speech (TTS)

Convert text to speech using any TTS model available through your providers.

```bash
curl http://localhost:5051/v1/audio/speech \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tts-1",
    "input": "Hello, welcome to WebRouter!",
    "voice": "alloy"
  }' \
  --output speech.mp3
```

## Audio Transcription (STT)

Transcribe audio files to text. Supports multipart form upload.

```bash
curl http://localhost:5051/v1/audio/transcriptions \
  -H "Authorization: Bearer <token>" \
  -F "file=@recording.mp3" \
  -F "model=whisper-1"
```

## Image Generation

Generate images through any OpenAI-compatible provider.

```bash
curl http://localhost:5051/v1/images/generations \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dall-e-3",
    "prompt": "A serene mountain lake at sunset",
    "n": 1,
    "size": "1024x1024"
  }'
```

## Provider Compatibility

Any provider that offers OpenAI-compatible endpoints for these operations is supported. Common providers:

| Provider | TTS | STT | Image Gen |
|----------|-----|-----|-----------|
| OpenAI | tts-1, tts-1-hd | whisper-1 | dall-e-3 |
| Azure OpenAI | Yes | Yes | Yes |
| Custom (OpenAI-compatible) | Yes | Yes | Yes |

## Failover Behavior

Multimedia requests follow the same failover logic as chat requests:

1. If the selected provider fails, wr-proxy tries the next healthy provider with the same model
2. Rate-limited providers enter cooldown (configurable duration)
3. Quota-exhausted providers are cooled for 30 minutes
4. Configurable via `max_failover` and `max_retry_count` settings
