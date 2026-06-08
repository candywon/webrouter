---
title: API Reference
description: WebRouter REST API endpoints
---

# API Reference

## Base URL

All API endpoints are prefixed with `/api/`. The admin panel is available at:

- **Admin Panel:** `http://localhost:5050`
- **Proxy Gateway:** `http://localhost:5051`

## Authentication

Most endpoints require authentication. Use the admin credentials to log in.

### `POST /api/auth/login`

Log in with username and password.

```json
{
  "username": "admin",
  "password": "admin123456"
}
```

### `GET /api/auth/check`

Check if the current session is authenticated.

## Dashboard

### `GET /api/dashboard/stats`

System overview statistics:

- Total providers, tokens, teams
- Request counts (today, this week, this month)
- Cost summary
- Provider status distribution

## Providers

### `GET /api/providers`

List all providers.

### `POST /api/providers`

Create a new provider.

### `GET /api/providers/<id>`

Get provider details.

### `PUT /api/providers/<id>`

Update provider.

### `DELETE /api/providers/<id>`

Delete provider.

### `POST /api/providers/<id>/check`

Trigger a health check for a specific provider.

## Channels

### `GET /api/providers/<id>/channels`

List channels for a provider.

### `POST /api/providers/<provider_id>/channels`

Add a channel.

## Monitoring

### `GET /api/monitor/health`

Get health status of all providers.

### `GET /api/monitor/history/<provider_id>`

Get 14-day health history for a provider.

## Alert Rules

### `GET /api/alerts/rules`

List all alert rules.

### `POST /api/alerts/rules`

Create an alert rule.

### `PUT /api/alerts/rules/<id>`

Update an alert rule.

### `DELETE /api/alerts/rules/<id>`

Delete an alert rule.

### `GET /api/alerts/history`

Get alert history.

## Billing

### `GET /api/billing/stats`

Get billing statistics (total cost, token usage, provider breakdown).

### `GET /api/billing/requests`

Get request logs with filtering (date range, provider, model, token).

### `GET /api/billing/export`

Export billing data as CSV.

## Tokens

### `GET /api/tokens`

List all tokens.

### `POST /api/tokens`

Create a new token.

### `PUT /api/tokens/<id>`

Update token configuration.

### `DELETE /api/tokens/<id>`

Delete a token.

### `POST /api/tokens/<id>/reset-key`

Regenerate the token's API key.

## Teams

### `GET /api/team`

List organizations (tree structure).

### `POST /api/team`

Create an organization.

### `PUT /api/team/<id>`

Update organization.

### `DELETE /api/team/<id>`

Delete organization.

## Settings

### `GET /api/settings`

Get all system settings.

### `PUT /api/settings`

Update system settings (batch).

## Model Pricing

### `GET /api/pricing`

List model pricing.

### `PUT /api/pricing`

Update model pricing.

## Model Grades

### `GET /api/modelgrades`

List model grades.

### `POST /api/modelgrades`

Create a model grade.

### `DELETE /api/modelgrades/<id>`

Delete a model grade.

## Model Aliases

### `GET /api/modelaliases`

List model aliases.

### `POST /api/modelaliases`

Create a model alias.

### `DELETE /api/modelaliases/<id>`

Delete a model alias.

## Desensitization

### `GET /api/desensitize`

List desensitization rules.

### `POST /api/desensitize`

Create a desensitization rule.

### `PUT /api/desensitize/<id>`

Update a rule.

### `DELETE /api/desensitize/<id>`

Delete a rule.

## CLI Export

### `GET /api/cli/export`

Generate CLI configuration for tools like Claude Code, Codex, Cursor, Continue.

## Demo

### `GET /api/demo/status`

Check if demo mode is active.

### `POST /api/demo/reset`

Reset demo seed data.

### `GET /api/demo/auto-login`

Auto-login with demo account.

## Proxy Gateway Endpoint

### `POST /v1/chat/completions`

The primary proxy endpoint. Use with a WebRouter API key.

```bash
curl http://localhost:5051/v1/chat/completions \
  -H "Authorization: Bearer <webrouter-token>" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

### `GET /health`

Proxy health check.