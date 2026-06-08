<p align="center">
  <img src="docs/images/logo.svg" alt="WebRouter" width="120" />
</p>

<p align="center">
  <a href="https://demo.webrouter.tech">
    <img src="https://img.shields.io/badge/Try%20Demo-Online-9cf?style=for-the-badge" alt="Try Live Demo" />
  </a>
</p>

<h1 align="center">WebRouter</h1>

<p align="center">
  <strong>Unified AI API Gateway</strong> — Manage all your LLM providers in one place.<br/>
  One API key → OpenAI, Anthropic, Google, DeepSeek, Qwen, and more.
</p>

<p align="center">
  <a href="README.zh-CN.md">中文文档</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#features">Features</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Python-3.8+-blue" alt="Python 3.8+" />
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8" alt="Go 1.21+" />
  <img src="https://img.shields.io/badge/License-BSL%201.1-blue" alt="License: BSL 1.1" />
  <img src="https://img.shields.io/badge/Status-Active-brightgreen" alt="Status: Active" />
</p>

---

## Why WebRouter?

Managing multiple AI API providers is painful — scattered keys, no visibility into costs, no failover when a provider goes down. WebRouter gives you a **single control plane** for all your LLM traffic.

- **Tired of hardcoding provider URLs?** → One gateway endpoint, auto-routed to the best provider
- **Worried about provider outages?** → Automatic health checks, cooldowns, and failover
- **No idea how much you're spending?** → Per-model cost tracking, quotas, and billing reports
- **Sharing API keys across the team?** → Token management with per-member quotas and model whitelists

## Features

### 🧠 Smart Routing
Set `model: "auto"` and WebRouter picks the optimal model based on request complexity — simple queries get fast/cheap models, complex reasoning gets powerful ones.

### 💓 Health Monitoring
Automatic health checks with latency tracking. Dead providers enter cooldown; traffic shifts to healthy alternatives — no manual intervention needed.

### 💰 Cost Tracking
Real-time cost accounting per model, per token, per team. Billing reports, quota management, and budget alerts built in.

### 🔐 Privacy & Desensitization
Built-in desensitization engine strips PII (phone numbers, ID cards, emails) from requests before they reach upstream providers.

### 👥 Team Management
Invite team members, assign quotas, restrict model access. Each member gets a unique API key with scoped permissions.

### ⚡ High-Performance Proxy
The `wr-proxy` Go gateway handles request forwarding, retry with backoff, streaming, and metering — all with minimal latency overhead.

### 📡 Multi-Provider Support
| Type | Description | Health | Latency | Cost |
|------|-------------|:------:|:-------:|:----:|
| `direct` | Official APIs (OpenAI, Anthropic, Google...) | ✅ | ✅ | — |
| `aggregate` | Aggregator platforms (OhMyGPT, API2D...) | ✅ | ✅ | Manual |
| `litellm` | LiteLLM proxy | ✅ | ✅ | — |
| `custom` | Any OpenAI-compatible gateway | ✅ | ✅ | — |

### 🛠️ CLI Config Export
One-click export of environment variables and config for Claude Code, Codex, Cursor, Continue, and more.

---

## Quick Start

### Try it Online

Don't want to install anything? Try the live demo at **[demo.webrouter.tech](https://demo.webrouter.tech)** (login: `demo` / `demo123456`).

### Prerequisites

- Python 3.8+
- Go 1.21+ (only if building wr-proxy from source; pre-built binaries included)
- 2 GB+ RAM

### Install & Run

```bash
git clone https://github.com/<org>/webrouter.git
cd webrouter
bash deploy/install.sh
```

The install script auto-detects your OS and architecture, sets up a virtual environment, installs dependencies, and starts both services.

### Open the Dashboard

```bash
open http://localhost:5050
# Default login: admin / admin123
```

### Add Your First Provider

1. Go to **Providers** → **+ Add**
2. Select type `direct`, paste your OpenAI base URL and API key
3. Click **🔍 Check** to verify connectivity
4. Your gateway is ready at `http://localhost:5051/v1/chat/completions`

### Docker

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

---

## Screenshots

<p align="center">
  <img src="docs/images/webrouter-1.png" alt="WebRouter dashboard" width="800" />
</p>

<p align="center">
  <img src="docs/images/webrouter-2.png" alt="WebRouter provider management" width="800" />
</p>

<p align="center">
  <img src="docs/images/webrouter-3.png" alt="WebRouter monitoring" width="800" />
</p>

<p align="center">
  <img src="docs/images/webrouter-4.png" alt="WebRouter billing analytics" width="800" />
</p>

<p align="center">
  <img src="docs/images/webrouter-5.png" alt="WebRouter system settings" width="800" />
</p>

---

## Architecture

```
┌─────────────┐     HTTP      ┌─────────────────┐
│  Browser/CLI │ ───────────→ │    WebRouter     │
│             │ ←──────────── │    (Flask)       │
└─────────────┘               │    :5050         │
                              └──────┬──────────┘
                                     │
                              ┌──────▼──────┐
                              │  wr-proxy    │
                              │  (Go) :5051  │
                              └──────┬──────┘
                                     │
                     ┌───────────────┼───────────────┐
                     │               │               │
              ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
              │   direct    │ │  aggregate  │ │   custom    │
              │ (Official)  │ │ (Aggregator)│ │ (Gateway)   │
              └─────────────┘ └─────────────┘ └─────────────┘
```

| Component | Stack | Role |
|-----------|-------|------|
| **WebRouter** (backend) | Python Flask | Admin panel, REST API, database models, scheduler |
| **wr-proxy** | Go 1.22 | High-performance API proxy: routing, retry, desensitization, metering |

Both components share a SQLite database (MySQL/PostgreSQL also supported) for configuration and request logs.

---

## Project Structure

```
webrouter/
├── backend/                # Flask backend
│   ├── app.py             # Application factory
│   ├── config.py          # Configuration
│   ├── models/            # Database models
│   ├── routes/            # 12 API blueprints (/api/*)
│   ├── services/          # Business logic
│   ├── static/            # Frontend SPA
│   │   ├── index.html
│   │   ├── js/            # 21 page modules
│   │   ├── css/
│   │   └── i18n/          # en.json, zh-CN.json
│   └── start.py           # Process manager
├── wr-proxy/               # Go proxy gateway
│   ├── main.go
│   ├── proxy.go            # HTTP forwarding
│   ├── smart_model.go      # Smart routing
│   ├── retry.go            # Retry with backoff
│   ├── desensitize.go      # PII stripping
│   ├── meter.go            # Cost tracking
│   └── ...
├── deploy/                 # Deployment configs
│   ├── install.sh
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── nginx.conf
├── docs/                   # Documentation
├── data/                   # Runtime data
└── .env                    # Environment config (auto-generated)
```

---

## Configuration

All settings are managed via the `.env` file (auto-generated on first install):

| Variable | Description | Default |
|----------|-------------|---------|
| `SESSION_SECRET` | Flask session key | Auto-generated |
| `DATABASE_URI` | Database connection string | SQLite |
| `REDIS_URL` | Redis connection (optional, for caching) | — |
| `FLASK_ENV` | Runtime environment | `production` |
| `FLASK_HOST` | Listen address | `0.0.0.0` |
| `FLASK_PORT` | Flask port | `5050` |
| `WR_PROXY_PORT` | wr-proxy port | `5051` |
| `ENABLE_SCHEDULER` | Run health checks & alerts on schedule | `0` (off in debug) |

### Process Management

```bash
python3 backend/start.py start     # Start all services
python3 backend/start.py stop      # Stop all services
python3 backend/start.py restart   # Restart
python3 backend/start.py status    # Check status
python3 backend/start.py logs      # Tail logs
```

Or use the generated shell scripts:

```bash
./start.sh    # Start
./stop.sh     # Stop
```

---

## Roadmap

- [ ] **Plugin SDK** — Extensible plugin interface for EE modules
- [ ] **SSO / SAML / OIDC** — Enterprise single sign-on
- [ ] **Audit logging** — Tamper-proof operation audit trail
- [ ] **Cluster mode** — Multi-instance with shared state
- [ ] **Cloud hosted version** — Zero-ops managed service
- [ ] **Advanced routing DSL** — Custom routing rules by department, project, or tag

> See [LICENSING.md](LICENSING.md) for the Community vs Enterprise edition feature matrix.

---

## Contributing

We welcome contributions! Before submitting a PR, please:

1. Sign the [Contributor License Agreement (CLA)](CONTRIBUTING.md) — this grants us the right to re-license the project in the future
2. Follow the existing code style
3. Test your changes locally

See [CONTRIBUTING.md](CONTRIBUTING.md) for full guidelines.

---

## License

WebRouter Community Edition is licensed under the **Business Source License 1.1**.

- ✅ Free for personal and internal production use
- ❌ Commercial redistribution (reselling, managed SaaS, OEM) is prohibited before the Change Date
- 🔄 On **2029-06-01**, the license automatically converts to **Apache-2.0**

See [LICENSE](LICENSE) for the full text and [LICENSING.md](LICENSING.md) for the dual-edition strategy.

---

## Acknowledgments

WebRouter is built with:

- [Flask](https://flask.palletsprojects.com/) — Python web framework
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — Pure-Go SQLite (no CGO)
- [APScheduler](https://apscheduler.readthedocs.io/) — Job scheduling
- [Font Awesome](https://fontawesome.com/) — Icons

---

<p align="center">
  <strong>One gateway. All AI APIs.</strong>
</p>
