---
title: CLI Export
description: Generate CLI configuration for popular development tools
---

# CLI Export

## Overview

WebRouter's CLI export feature generates ready-to-use configuration for popular AI development tools. Instead of manually configuring each tool, select your token and export the config.

## Supported Tools

| Tool | Configuration Format |
|------|---------------------|
| Claude Code | Environment variables |
| Codex | CLI flags |
| Cursor | `~/.cursor/config.json` |
| Continue | `~/.continue/config.json` |

## How to Use

1. Go to **CLI Export** in the admin panel
2. Select a token from the dropdown
3. Click **Generate**
4. Copy the generated configuration

## Example: Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:5051
export ANTHROPIC_API_KEY=<your-webrouter-token>
export CLAUDE_MODEL=claude-sonnet-4
```

## Example: Cursor

```json
{
  "baseUrl": "http://localhost:5051",
  "apiKey": "<your-webrouter-token>",
  "models": [
    { "name": "claude-sonnet-4", "provider": "openai" },
    { "name": "gpt-4o", "provider": "openai" }
  ]
}
```

## Smart URL Resolution

When generating config, WebRouter automatically resolves the correct wr-proxy URL based on your deployment (localhost for development, server address for production).