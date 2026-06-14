---
title: Installation
description: Detailed installation instructions for WebRouter
---

# Installation

## System Requirements

- **Python** 3.8+ (required)
- **Go** 1.21+ (only for building wr-proxy from source; pre-built binaries available)
- **Memory** 2 GB+ (RAM)
- **Disk** 500 MB+ (for application and data)

## Manual Installation

### 1. Clone the Repository

```bash
git clone https://github.com/candywon/webrouter.git
cd webrouter
```

### 2. Set Up Python Environment

```bash
python3 -m venv venv
source venv/bin/activate
pip install -r backend/requirements.txt
```

### 3. Build wr-proxy (Optional)

If you have Go installed:

```bash
cd wr-proxy && make build && cd ..
```

Pre-built binaries are included in the repository, so this step is optional.

### 4. Configure

Create a `.env` file in the project root:

```bash
cat > .env << 'EOF'
SESSION_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))")
FLASK_ENV=production
WR_PORT=5050
WR_PROXY_PORT=5051
EOF
```

### 5. Start Services

```bash
python3 backend/start.py start
```

Check status:

```bash
python3 backend/start.py status
```

## Docker Compose

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

This starts three services:
- `webrouter` — Flask admin panel on port 5050
- `wr-proxy` — Go proxy gateway on port 5051
- `redis` — Cache (optional)

## Process Management

```bash
python3 backend/start.py start     # Start all services
python3 backend/start.py stop      # Stop all services
python3 backend/start.py restart   # Restart
python3 backend/start.py status    # Check status
python3 backend/start.py logs      # View logs
```

Or use the generated scripts:

```bash
./start.sh    # Start
./stop.sh     # Stop
```