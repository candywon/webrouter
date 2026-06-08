---
title: Deployment
description: Production deployment options for WebRouter
---

# Deployment

## Docker Compose (Recommended for Production)

The recommended production deployment uses Docker Compose with three services:

```yaml
services:
  wr-proxy:     # Go proxy gateway (port 5051)
  webrouter:    # Flask admin panel (port 5050)
  redis:        # Cache
```

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

### Configuration

Set environment variables in a `.env` file:

```bash
DATABASE_URI=sqlite:///data/webrouter.db
# Or use MySQL:
# DATABASE_URI=mysql+pymysql://user:pass@host/webrouter
REDIS_URL=redis://redis:6379
FLASK_ENV=production
```

## Nginx Reverse Proxy

A sample nginx configuration is available at `deploy/nginx.conf`:

```nginx
server {
    listen 80;
    server_name webrouter.example.com;

    location / {
        proxy_pass http://127.0.0.1:5050;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /v1/ {
        proxy_pass http://127.0.0.1:5051;
    }
}
```

## Database Options

### SQLite (Default)

- Default location: `data/webrouter.db`
- Suitable for single-instance deployments
- No additional setup required

### MySQL / PostgreSQL

Set `DATABASE_URI` in `.env`:

```bash
# MySQL
DATABASE_URI=mysql+pymysql://user:password@host:3306/webrouter

# PostgreSQL
DATABASE_URI=postgresql://user:password@host:5432/webrouter
```

## Scheduler Configuration

Health checks and alert evaluation are enabled by default in production. To force-enable in debug mode:

```bash
ENABLE_SCHEDULER=1
```