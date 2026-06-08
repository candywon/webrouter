---
title: Tokens & Teams
description: Managing API keys and team access
---

# Tokens & Teams

## API Tokens

Tokens are the API keys clients use to authenticate with wr-proxy. Each token can have:

- **Model whitelist** — restrict which models this token can use
- **Quota** — total token limit
- **Rate limit** — requests per minute
- **Smart downgrade** — automatically use a cheaper model on errors
- **Desensitization** — enable/disable PII stripping

### Creating a Token

1. Go to **Tokens** → **+ Create**
2. Enter a name and associate it with an organization
3. Set model access (whitelist or allow all)
4. Configure quota and rate limits
5. Enable smart downgrade if desired

## Organizations

Organizations represent your company structure. They can be nested:

```
Engineering (company)
├── Backend (department)
└── Frontend (department)
ML Research (company)
└── LLM Ops (department)
Product (company)
```

Each token belongs to an organization, enabling per-organization quota tracking and billing reports.

## Quota Management

Quotas can be set per token:

- **Total quota** — maximum tokens the token can consume
- **Used quota** — consumed tokens (auto-tracked)
- **Remaining** — auto-calculated

When a token's quota is nearly exhausted, alerts can be triggered.

## Team Members

Each token has a member email for identification. Members can be assigned to organizations, and the admin can view per-member usage statistics from the billing dashboard.