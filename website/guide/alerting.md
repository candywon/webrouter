---
title: Alerting
description: Configuring alert rules and notification channels
---

# Alerting

## Overview

WebRouter's alert engine evaluates rules every minute and sends notifications when conditions are met.

## Alert Rules

### Error Rate Alert

Triggered when a provider's error rate exceeds a threshold in a time window:

| Field | Description |
|-------|-------------|
| Threshold | Error rate (e.g., 0.1 = 10%) |
| Window | Evaluation period in minutes |

### Quota Remaining Alert

Triggered when a token's remaining quota drops below a percentage:

| Field | Description |
|-------|-------------|
| Threshold | Remaining percentage (e.g., 0.2 = 20%) |
| Scope | `token` or `organization` |

### Provider Down Alert

Triggered when a provider is consistently failing:

| Field | Description |
|-------|-------------|
| Max Retries | Consecutive failures before alert |
| Window | Evaluation period in minutes |

## Alert Levels

| Level | Color | Behavior |
|-------|-------|----------|
| `critical` | Red | All configured channels |
| `warning` | Yellow | Notify but no escalation |
| `info` | Blue | Informational only |

## Notification Channels

### Email

Configure SMTP settings in **Settings** → **Alert Settings**:

- SMTP host, port, user, password
- From address and recipient list
- TLS support

### WeChat

Use ServerChan (PushBear) for WeChat notifications:

- Get a SendKey from [sct.ftqq.com](https://sct.ftqq.com)
- Enter it in **Settings** → **Alert Settings**

## Cooldown

To prevent alert fatigue, alerts have a cooldown period (default: 5 minutes). The same alert will not fire again until the cooldown expires.