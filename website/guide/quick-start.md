---
title: Quick Start
description: Get WebRouter running in minutes
---

# Quick Start

## Try it Online

Don't want to install anything? Visit the **[live demo](https://webrouter-demo.fly.dev)** to explore WebRouter's admin panel (login: `demo` / `demo123456`).

## One-Click Install

```bash
git clone https://github.com/<org>/webrouter.git
cd webrouter
bash deploy/install.sh
```

The install script automatically:
1. Detects your OS and CPU architecture
2. Installs Python 3.8+ if missing
3. Creates a virtual environment and installs dependencies
4. Builds the wr-proxy Go gateway (if Go is available)
5. Generates configuration and startup scripts
6. Starts both services

## First Login

Open `http://localhost:5050` and log in with:

- **Username:** `admin`
- **Password:** `admin123456`

## Docker

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

## Add Your First Provider

1. Go to **Providers** → **+ Add**
2. Select type `direct`, enter OpenAI's Base URL and API Key
3. Click **Check** to verify connectivity
4. Your gateway is ready: `http://localhost:5051/v1/chat/completions`