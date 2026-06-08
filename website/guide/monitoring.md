---
title: Monitoring
description: Health monitoring and provider status tracking
---

# Monitoring

## Health Dashboard

The monitoring dashboard provides a real-time overview of all your providers:

- **Status** — healthy, warning, dead, unknown
- **Latency** — response time in milliseconds
- **Uptime** — 14-day historical chart
- **Error rate** — percentage of failed requests

## Health Check Details

WebRouter runs health checks every 5 minutes against each provider:

```json
{
  "provider": "OpenAI",
  "status": "healthy",
  "latency_ms": 850,
  "checked_at": "2026-06-07T10:30:00Z"
}
```

## History

The system maintains 14 days of health check history, visualized as:

- **Latency trends** — line chart showing response times over time
- **Status distribution** — pie chart of healthy vs. degraded providers
- **Provider timeline** — per-provider timeline of status changes

## Provider Detail View

Click on any provider to see:

- Current status and latency
- 14-day health history
- Recent error messages