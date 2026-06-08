---
title: Configuration
description: Environment variables and system settings reference
---

# Configuration

## Environment Variables

All settings can be configured via `.env` file in the project root or through environment variables.

### Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `SESSION_SECRET` | Flask session encryption key | Auto-generated |
| `DATABASE_URI` | Database connection string | `sqlite:///data/webrouter.db` |
| `REDIS_URL` | Redis connection (optional, for caching) | `redis://localhost:6379/0` |
| `FLASK_ENV` | Runtime environment | `production` |
| `TZ` | Timezone | `Asia/Shanghai` |

### Port Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `WR_PORT` | Flask admin panel port | `5050` |
| `WR_PROXY_PORT` | wr-proxy gateway port | `5051` |
| `NEWAPI_URL` | New-API sidecar URL | `http://localhost:3000` |

### Scheduler

| Variable | Description | Default |
|----------|-------------|---------|
| `ENABLE_SCHEDULER` | Force-enable scheduler in debug mode | `0` |
| `HEALTH_CHECK_INTERVAL` | Provider health check interval (seconds) | `300` |
| `BALANCE_CHECK_INTERVAL` | Balance check interval (seconds) | `1800` |

### Alerting

| Variable | Description | Default |
|----------|-------------|---------|
| `ALERT_COOLDOWN` | Alert cooldown period (seconds) | `300` |

### Admin Account

| Variable | Description | Default |
|----------|-------------|---------|
| `WEBROUTER_ADMIN_USER` | Default admin username | `admin` |
| `WEBROUTER_ADMIN_PASSWORD` | Default admin password | `admin123456` |

## Demo Mode

Set `WEBROUTER_DEMO=1` to enable demo mode:

- Auto-login with `demo` / `demo123456`
- Read-only protection for core data
- Pre-seeded sample data (providers, tokens, alerts, request logs)

## System Settings (Admin Panel)

The following settings can be configured through the admin panel under **Settings**:

| Setting | Description |
|---------|-------------|
| Gateway Mode | Enable/disable wr-proxy routing |
| Smart Routing | Enable/disable auto model selection |
| Retry Count | Number of retry attempts |
| Timeout | Request timeout in seconds |
| Alert SMTP | Email server configuration |
| Alert WeChat | ServerChan SendKey for WeChat notifications |
| Alert Email | Alert notification email address |