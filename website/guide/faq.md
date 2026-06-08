---
title: FAQ
description: Frequently asked questions about WebRouter
---

# FAQ

## General

### What is WebRouter?

WebRouter is an open-source AI API gateway that provides a unified management interface for multiple LLM providers. It handles provider health monitoring, smart routing, cost tracking, team management, and privacy desensitization.

### Who is WebRouter for?

- **Developers** who want a single API key for multiple AI providers
- **Engineering teams** that need cost visibility and quota management
- **Companies** that need to manage and monitor AI API usage across their organization

### Do I need to use wr-proxy?

wr-proxy is the high-performance Go proxy that handles request forwarding, retry, and desensitization. You can run the Flask admin panel without it for configuration and monitoring, but wr-proxy is needed for actual API proxying.

## Licensing

### What license does WebRouter use?

The Community Edition (CE) uses the **Business Source License 1.1 (BSL 1.1)**. On **June 1, 2029**, it automatically converts to Apache License 2.0.

### Can I use WebRouter for free?

Yes. The CE is free for:
- Personal use
- Internal production use within an organization
- Learning and research
- Modifying and contributing code

### What is not allowed under BSL 1.1?

- Selling WebRouter or derivative works as a commercial product
- Offering paid hosted/managed services of WebRouter
- OEM embedding in a closed-source product

### What is the Change Date?

June 1, 2029. After this date, the Community Edition automatically becomes Apache 2.0, removing all commercial restrictions.

### What about the Enterprise Edition?

The Enterprise Edition (EE) uses a proprietary EULA and includes:
- SSO/SAML/OIDC integration
- Advanced audit logging
- Cluster deployment
- Enhanced alerting channels (DingTalk, Feishu, Slack, PagerDuty)
- Commercial support with SLA

## Technical

### What databases are supported?

SQLite is the default (zero configuration). MySQL and PostgreSQL are also supported by setting `DATABASE_URI`.

### Does WebRouter support streaming?

Yes. wr-proxy supports streaming responses (SSE).

### Can I run WebRouter without Docker?

Yes. The install script (`deploy/install.sh`) handles everything on bare metal.

### How are API keys managed?

WebRouter generates unique API keys per token. Each token has configurable model access, quotas, rate limits, and desensitization settings.

### What provider types are supported?

`direct`, `aggregate`, `litellm`, and `custom` — any OpenAI-compatible provider.

## Troubleshooting

### Health checks show "dead" for my provider

1. Verify the Base URL is correct
2. Check that the API key is valid
3. Ensure the provider is accessible from your server (no firewall blocks)
4. Check the provider status page for outages

### Tokens not working

1. Verify the token is enabled
2. Check quota hasn't been exhausted
3. Ensure the requested model is in the token's whitelist
4. Check rate limit hasn't been exceeded

### How do I reset the admin password?

Set the environment variable and restart:

```bash
WEBROUTER_ADMIN_PASSWORD=newpassword
```