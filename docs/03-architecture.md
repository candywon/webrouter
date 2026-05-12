# 技术架构文档

## 一、整体架构

### 1.1 系统架构图

```
                    ┌─────────────┐
                    │   Nginx     │
                    │  :80/:443   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────▼─────┐ ┌───▼────┐ ┌────▼─────┐
        │ WebRouter  │ │New-API │ │  静态资源  │
        │ (Flask)    │ │ (Go)   │ │  (前端)   │
        │ :5000      │ │ :3000  │ │          │
        └─────┬──────┘ └───┬────┘ └──────────┘
              │            │
        ┌─────▼────────────▼─────┐
        │     MySQL / SQLite     │
        │     共享数据库           │
        └───────────┬────────────┘
                    │
              ┌─────▼─────┐
              │   Redis    │
              │   :6379    │
              └───────────┘
```

### 1.2 职责分工

| 组件 | 职责 | 技术 |
|------|------|------|
| Nginx | 反向代理、SSL、静态资源 | Nginx |
| New-API | API网关核心：渠道管理、Key轮换、格式转换、负载均衡 | Go + React |
| WebRouter | 管理增强：监控、告警、计费、团队、对接 | Python Flask |
| MySQL/SQLite | 共享数据存储 | 兼容New-API表结构 |
| Redis | 缓存、会话、实时统计 | Redis |

### 1.3 核心原则

1. **不修改New-API源码** — 通过数据库共享 + 管理API集成
2. **数据层复用** — 直读New-API数据库，不重复存储
3. **侧车模式** — WebRouter与New-API独立部署，互不影响
4. **故障隔离** — WebRouter挂了不影响API转发，New-API挂了WebRouter仍可展示历史数据

---

## 二、后端架构

### 2.1 目录结构

```
backend/
├── app.py                 # Flask主应用入口
├── config.py              # 配置管理
├── extensions.py          # Flask扩展初始化
├── models/
│   ├── __init__.py
│   ├── newapi_adapter.py  # New-API数据库适配器
│   ├── monitor.py         # 监控数据模型
│   ├── alert.py           # 告警规则模型
│   └── billing.py         # 计费模型
├── routes/
│   ├── __init__.py
│   ├── dashboard.py       # 仪表盘API
│   ├── monitor.py         # 监控API
│   ├── alert.py           # 告警API
│   ├── billing.py         # 计费API
│   ├── team.py            # 团队管理API
│   ├── cli_export.py      # CLI配置导出API
│   └── settings.py        # 设置API
├── services/
│   ├── __init__.py
│   ├── health_checker.py  # Key健康检测
│   ├── alert_engine.py    # 告警引擎
│   ├── stats_collector.py # 统计采集
│   ├── cli_generator.py   # CLI配置生成
│   └── deploy_helper.py   # 部署辅助
├── static/                # 前端静态文件
└── templates/             # Flask模板（如需SSR）
```

### 2.2 New-API数据库适配

WebRouter直读New-API数据库，关键表映射：

| New-API表 | 用途 | WebRouter读取方式 |
|-----------|------|------------------|
| channels | 渠道/Key管理 | 读取状态、余额、模型列表 |
| tokens | API Key管理 | 读取额度、使用量 |
| users | 用户管理 | 读取用户信息、角色 |
| logs | 请求日志 | 读取调用量、错误率、延迟 |
| options | 系统配置 | 读取全局设置 |

适配器设计：

```python
# models/newapi_adapter.py
class NewAPIAdapter:
    """New-API数据库只读适配器"""

    def __init__(self, db_uri):
        self.engine = create_engine(db_uri)

    def get_channels_status(self):
        """获取所有渠道健康状态"""
        sql = """
        SELECT id, name, type, status, priority,
               models, created_time, test_time
        FROM channels
        ORDER BY priority DESC
        """
        return pd.read_sql(sql, self.engine)

    def get_usage_stats(self, hours=24):
        """获取用量统计"""
        sql = """
        SELECT model_name, channel_id,
               COUNT(*) as request_count,
               SUM(prompt_tokens) as input_tokens,
               SUM(completion_tokens) as output_tokens,
               AVG(duration) as avg_duration,
               SUM(CASE WHEN code != 200 THEN 1 ELSE 0 END) as error_count
        FROM logs
        WHERE created_at >= datetime('now', '-%d hours')
        GROUP BY model_name, channel_id
        """ % hours
        return pd.read_sql(sql, self.engine)
```

### 2.3 监控服务设计

```python
# services/health_checker.py
class HealthChecker:
    """渠道健康检测引擎"""

    CHECK_INTERVAL = 300  # 5分钟检测一次

    # 每种渠道类型的测试提示词
    TEST_PROMPTS = {
        "openai": {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "hi"}], "max_tokens": 1},
        "anthropic": {"model": "claude-3-haiku", "messages": [{"role": "user", "content": "hi"}], "max_tokens": 1},
        "gemini": {"model": "gemini-2.0-flash", "contents": [{"parts": [{"text": "hi"}]}]},
    }

    async def check_channel(self, channel: dict) -> dict:
        """检测单个渠道健康状态"""
        result = {
            "channel_id": channel["id"],
            "timestamp": datetime.utcnow(),
            "status": "unknown",
            "latency_ms": 0,
            "error": None
        }
        try:
            start = time.monotonic()
            response = await self._send_test_request(channel)
            result["latency_ms"] = (time.monotonic() - start) * 1000
            if response.status_code == 200:
                result["status"] = "healthy"
            elif response.status_code == 429:
                result["status"] = "rate_limited"
            elif response.status_code == 401:
                result["status"] = "auth_failed"
            else:
                result["status"] = "unhealthy"
                result["error"] = f"HTTP {response.status_code}"
        except Exception as e:
            result["status"] = "dead"
            result["error"] = str(e)

        return result
```

### 2.4 告警引擎设计

```python
# services/alert_engine.py
class AlertEngine:
    """告警规则引擎"""

    def __init__(self, config: dict):
        self.rules = config.get("rules", [])
        self.channels = config.get("channels", {})

    async def evaluate(self, event: dict):
        """评估告警规则"""
        for rule in self.rules:
            if self._match(rule["condition"], event):
                await self._fire(rule, event)

    async def _fire(self, rule: dict, event: dict):
        """触发告警"""
        message = self._format_message(rule, event)
        for ch in rule.get("channels", []):
            handler = self.channels.get(ch)
            if handler:
                await handler.send(message)

# 告警通道实现
class WeChatAlertChannel:
    """Server酱微信告警"""
    def __init__(self, sendkey: str):
        self.url = f"https://sctapi.ftqq.com/{sendkey}.send"

    async def send(self, message: str):
        async with aiohttp.ClientSession() as session:
            await session.post(self.url, json={
                "title": "WebRouter 告警",
                "desp": message
            })

class EmailAlertChannel:
    """邮件告警"""
    def __init__(self, smtp_host, smtp_port, sender, password, receivers):
        self.conf = (smtp_host, smtp_port, sender, password, receivers)

    async def send(self, message: str):
        # 异步发送邮件
        pass

class WebhookAlertChannel:
    """自定义Webhook告警"""
    def __init__(self, url: str):
        self.url = url

    async def send(self, message: str):
        async with aiohttp.ClientSession() as session:
            await session.post(self.url, json={"content": message})
```

### 2.5 API路由设计

```
# 仪表盘
GET  /api/dashboard/overview        # 总览数据
GET  /api/dashboard/trends          # 趋势数据(7天/30天)
GET  /api/dashboard/channels        # 渠道列表+状态

# 监控
GET  /api/monitor/channels          # 渠道健康状态
POST /api/monitor/check/:id         # 手动触发检测
GET  /api/monitor/history/:id       # 检测历史

# 告警
GET  /api/alerts/rules              # 告警规则列表
POST /api/alerts/rules              # 创建告警规则
PUT  /api/alerts/rules/:id          # 更新规则
DELETE /api/alerts/rules/:id        # 删除规则
GET  /api/alerts/history            # 告警历史
PUT  /api/alerts/channels           # 配置告警通道

# 计费
GET  /api/billing/usage             # 用量统计
GET  /api/billing/cost              # 成本分析
GET  /api/billing/daily             # 每日明细

# 团队
GET  /api/team/members              # 成员列表
POST /api/team/members              # 邀请成员
PUT  /api/team/members/:id          # 更新成员额度
DELETE /api/team/members/:id        # 移除成员
GET  /api/team/members/:id/usage    # 成员用量

# CLI对接
GET  /api/cli/export/:tool          # 导出配置(claude-code/codex/openclaw/hermes)
POST /api/cli/test                  # 测试连接

# 设置
GET  /api/settings                  # 系统设置
PUT  /api/settings                  # 更新设置
POST /api/settings/backup           # 创建备份
POST /api/settings/restore          # 恢复备份
```

---

## 三、前端架构

### 3.1 页面结构

```
frontend/
├── index.html              # 管理后台SPA入口
├── css/
│   ├── variables.css       # CSS变量(主题色)
│   ├── layout.css          # 布局
│   ├── dashboard.css       # 仪表盘样式
│   ├── monitor.css         # 监控样式
│   └── components.css      # 组件样式
├── js/
│   ├── app.js              # 应用入口、路由
│   ├── api.js              # API请求封装
│   ├── dashboard.js        # 仪表盘逻辑
│   ├── monitor.js          # 监控逻辑
│   ├── alert.js            # 告警逻辑
│   ├── billing.js          # 计费逻辑
│   ├── team.js             # 团队逻辑
│   ├── cli-export.js       # CLI导出逻辑
│   └── utils.js            # 工具函数
└── assets/
    └── icons/              # SVG图标
```

### 3.2 前端路由(Hash路由)

```
#/                  → 仪表盘总览
#/channels          → 渠道管理
#/monitor           → 健康监控
#/alerts            → 告警规则
#/billing           → 计费统计
#/team              → 团队管理
#/cli               → CLI对接
#/settings          → 系统设置
```

### 3.3 图表方案

使用轻量级Chart.js，按需加载：

```javascript
// js/dashboard.js
class Dashboard {
    async loadOverview() {
        const data = await API.get('/api/dashboard/overview');
        this.renderCards(data);
        this.renderTrendChart(data.trends);
        this.renderChannelStatus(data.channels);
    }

    renderTrendChart(trends) {
        new Chart(document.getElementById('trendChart'), {
            type: 'line',
            data: {
                labels: trends.dates,
                datasets: [{
                    label: '调用量',
                    data: trends.counts,
                    borderColor: '#6366f1',
                    tension: 0.3,
                    fill: true,
                    backgroundColor: 'rgba(99,102,241,0.1)'
                }]
            },
            options: {
                responsive: true,
                plugins: { legend: { display: false } },
                scales: { y: { beginAtZero: true } }
            }
        });
    }
}
```

---

## 四、部署架构

### 4.1 Docker Compose配置

```yaml
# deploy/docker-compose.yml
version: '3.8'

services:
  # API网关核心
  new-api:
    image: calciumion/new-api:latest
    container_name: webrouter-newapi
    restart: always
    ports:
      - "3000:3000"
    environment:
      - TZ=Asia/Shanghai
      - SQL_DSN=${DB_DSN:-}
      - REDIS_CONN_STRING=redis://redis:6379
      - SESSION_SECRET=${SESSION_SECRET}
    volumes:
      - newapi_data:/data
    depends_on:
      - redis

  # WebRouter管理后台
  webrouter:
    build:
      context: ..
      dockerfile: deploy/Dockerfile.flask
    container_name: webrouter-app
    restart: always
    ports:
      - "5000:5000"
    environment:
      - TZ=Asia/Shanghai
      - DATABASE_URI=${DB_DSN:-sqlite:///data/new-api.db}
      - NEWAPI_URL=http://new-api:3000
      - NEWAPI_ADMIN_TOKEN=${NEWAPI_ADMIN_TOKEN}
      - REDIS_URL=redis://redis:6379
    volumes:
      - webrouter_data:/app/data
    depends_on:
      - new-api
      - redis

  # Redis缓存
  redis:
    image: redis:7-alpine
    container_name: webrouter-redis
    restart: always
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  # Nginx反向代理
  nginx:
    image: nginx:alpine
    container_name: webrouter-nginx
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
    depends_on:
      - webrouter
      - new-api

volumes:
  newapi_data:
  webrouter_data:
  redis_data:
```

### 4.2 Nginx配置

```nginx
# deploy/nginx/nginx.conf
upstream webrouter {
    server webrouter:5000;
}
upstream newapi {
    server new-api:3000;
}

server {
    listen 80;
    server_name _;

    # WebRouter管理后台
    location / {
        proxy_pass http://webrouter;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # New-API管理接口
    location /api/ {
        proxy_pass http://newapi;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # AI API转发(用户调用)
    location /v1/ {
        proxy_pass http://newapi;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
        proxy_read_timeout 300s;
    }
}
```

### 4.3 Flask Dockerfile

```dockerfile
# deploy/Dockerfile.flask
FROM python:3.11-slim

WORKDIR /app

COPY backend/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY backend/ .
COPY frontend/ ./static/

EXPOSE 5000

CMD ["gunicorn", "--bind", "0.0.0.0:5000", "--workers", "2", "--timeout", "120", "app:create_app()"]
```

### 4.4 一键部署脚本

```bash
#!/bin/bash
# scripts/install.sh

set -e

echo "=== WebRouter 一键部署 ==="

# 1. 检查Docker
if ! command -v docker &> /dev/null; then
    echo "安装 Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
fi

# 2. 检查Docker Compose
if ! docker compose version &> /dev/null; then
    echo "安装 Docker Compose 插件..."
    apt-get update && apt-get install -y docker-compose-plugin
fi

# 3. 创建目录
INSTALL_DIR="${1:-$HOME/webrouter}"
mkdir -p "$INSTALL_DIR" && cd "$INSTALL_DIR"

# 4. 生成配置
cat > .env << EOF
SESSION_SECRET=$(openssl rand -hex 32)
NEWAPI_ADMIN_TOKEN=$(openssl rand -hex 16)
DB_DSN=
EOF

# 5. 下载docker-compose.yml
curl -fsSL https://raw.githubusercontent.com/USER/webrouter/main/deploy/docker-compose.yml \
    -o docker-compose.yml

# 6. 启动
docker compose up -d

# 7. 等待启动
echo "等待服务启动..."
sleep 10

# 8. 输出信息
PUBLIC_IP=$(curl -s ifconfig.me || echo "YOUR_SERVER_IP")
echo ""
echo "=== 部署完成 ==="
echo "管理后台: http://$PUBLIC_IP"
echo "New-API:  http://$PUBLIC_IP:3000"
echo "默认密码已写入 .env 文件"
echo "=================="
```

---

## 五、数据模型

### 5.1 WebRouter自有表(追加到New-API数据库)

```sql
-- 告警规则
CREATE TABLE wr_alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,
    condition_type VARCHAR(50) NOT NULL,  -- key_failed/balance_low/error_rate/usage_spike
    condition_config TEXT NOT NULL,         -- JSON配置
    level VARCHAR(20) NOT NULL,            -- critical/warning/info
    channels TEXT NOT NULL,                -- JSON: ["wechat","email"]
    enabled BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 告警历史
CREATE TABLE wr_alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER REFERENCES wr_alert_rules(id),
    event_data TEXT NOT NULL,               -- JSON事件数据
    message TEXT NOT NULL,
    level VARCHAR(20) NOT NULL,
    channels_sent TEXT,                     -- JSON: 已发送通道
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 渠道健康记录
CREATE TABLE wr_channel_health (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL,            -- healthy/warning/dead/disabled
    latency_ms INTEGER,
    error_message TEXT,
    checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_channel_time (channel_id, checked_at)
);

-- 团队额度分配
CREATE TABLE wr_team_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    quota_total BIGINT DEFAULT 0,          -- 总额度(tokens)
    quota_used BIGINT DEFAULT 0,           -- 已用额度
    period VARCHAR(20) DEFAULT 'monthly',  -- monthly/weekly/daily
    reset_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

-- 成本记录
CREATE TABLE wr_cost_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    channel_id INTEGER,
    model_name VARCHAR(100),
    input_tokens BIGINT DEFAULT 0,
    output_tokens BIGINT DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,           -- 成本(分)
    recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_time (user_id, recorded_at),
    INDEX idx_model_time (model_name, recorded_at)
);

-- CLI配置模板
CREATE TABLE wr_cli_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_name VARCHAR(50) NOT NULL,         -- claude-code/codex/openclaw/hermes
    template_content TEXT NOT NULL,          -- 配置模板
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tool_name)
);
```

---

## 六、技术选型

| 层 | 技术 | 选型理由 |
|----|------|---------|
| API网关 | New-API (Go) | 3.2万Star，功能最全，兼容One-API数据 |
| 管理后端 | Flask (Python) | 技术栈统一，开发快 |
| 前端 | 原生HTML/CSS/JS | 与包装站一致，无需构建工具 |
| 图表 | Chart.js | 轻量(60KB)，按需加载 |
| 数据库 | SQLite→MySQL | 开发用SQLite，生产平滑迁移MySQL |
| 缓存 | Redis | New-API已依赖Redis，复用 |
| 部署 | Docker Compose | 一键部署，与New-API一致 |
| 反向代理 | Nginx | SSL、路由分发、静态资源 |
| 进程管理 | Gunicorn | Flask生产级WSGI服务器 |
| 定时任务 | APScheduler | 健康检测、统计采集、告警评估 |

---

## 七、安全设计

| 措施 | 说明 |
|------|------|
| HTTPS | Nginx终止SSL，Let's Encrypt自动续期 |
| Key加密存储 | 复用New-API的加密机制 |
| 管理后台认证 | JWT Token，与New-API用户体系打通 |
| API限流 | Nginx层限流，防止暴力请求 |
| 数据隔离 | 团队版用户数据严格隔离，按user_id过滤 |
| 备份加密 | 备份文件AES加密，传输用HTTPS |
