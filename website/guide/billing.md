---
title: Billing
description: Cost tracking, quotas, and billing reports
---

# Billing

## Overview

WebRouter tracks token usage and costs in real time. The billing dashboard provides:

- Per-model cost breakdown
- Per-token and per-organization usage
- Historical trends (30 days)
- Quota management
- Budget alerts

## Cost Tracking

### Per-Model Pricing

Configure pricing in **Settings** → **Model Pricing**:

```json
{
  "model": "gpt-4o",
  "input_price": 2.50,
  "output_price": 10.00,
  "unit": "per million tokens"
}
```

Default pricing is pre-configured for common models and can be customized.

### Request Logs

Every request processed by wr-proxy is logged with:

- Model, provider, token used
- Input/output token counts
- Cost (calculated from pricing)
- Latency, status, error info

30 days of request history are maintained and viewable in the billing dashboard.

## Quotas

Quotas can be set at the token level:

| Field | Description |
|-------|-------------|
| Total quota | Maximum tokens allowed |
| Used | Consumed tokens (auto-tracked) |
| Remaining | Auto-calculated |

When quota is depleted, the token stops working (unless smart downgrade is enabled).

## Reports

Export billing data as CSV:

- **Token report** — usage per token over a date range
- **Provider report** — usage per provider
- **Model report** — usage per model

Data can be filtered by date range and organization.