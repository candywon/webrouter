# 技术架构文档

## 一、整体架构

### 1.1 系统架构图

```
                    ┌─────────────────────────────────┐
                    │         终端工具 / IDE           │
                    │  Claude Code · Codex · Hermes   │
                    └───────────────┬─────────────────┘
                                    │
                    ┌───────────────▼─────────────────┐
                    │            Nginx                 │
                    │           :80/:443               │
                    └───────────────┬─────────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
        ┌─────▼─────┐   ┌──────────▼──────────┐   ┌─────▼─────┐
        │ WebRouter  │   │    静态资源 (前端)    │   │  任意      │
        │ (Flask)    │   │                     │   │  Provider  │
        │ :5050      │   │                     │   │  实例       │
        └─────┬──────┘   └─────────────────────┘   └─────┬─────┘
              │                                          │
        ┌─────▼──────────────────────────────────────────▼──┐
        │              wr-proxy (Go 代理网关)                 │
        │  智能路由 · 脱敏 · 重试 · 计量 · 二进制流支持       │
        │                                                   │
        │  ┌────────┐ ┌────────┐ ┌─────────┐ ┌─────────┐  │
        │  │直连官方 │ │聚合平台 │ │LiteLLM  │ │自定义    │  │
        │  │(HTTP)  │ │(HTTP)  │ │(HTTP)   │ │(HTTP)   │  │
        │  └────────┘ └────────┘ └─────────┘ └─────────┘  │
        └─────────────────────────┬─────────────────────────┘
                                  │
                          ┌───────▼───────┐
                          │    SQLite      │
                          │  (wr-proxy.db) │
                          └───────┬───────┘
                                  │
                          ┌───────▼───────┐
                          │    Redis       │
                          │    :6379       │
                          └───────────────┘
```

### 1.2 职责分工

| 组件 | 职责 | 技术 |
|------|------|------|
| Nginx | 反向代理、SSL、静态资源 | Nginx |
| WebRouter (Flask) | 管理增强：Provider 管理、统一监控、告警、计费、团队、对接 | Python Flask |
| wr-proxy (Go) | 代理网关核心：智能路由、请求脱敏、自动重试、成本计量、二进制流 | Go |
| Provider 层 | API 源抽象：统一注册、统一检测、统一调度 | 可插拔适配器 |
| SQLite | 共享数据存储（wr-provider.db + webrouter.db） | 纯 Go/Python |
| Redis | 缓存、会话、实时统计 | Redis |

### 1.3 核心原则

1. **Provider 抽象优先** — 所有 API 源（直连、聚合、自建）统一为 Provider 概念
2. **wr-proxy 内置网关** — 自研 Go 代理，不依赖任何第三方网关，自主可控
3. **数据能力分级** — 不同 Provider 类型获取不同深度的数据，但健康检测是基线能力
4. **故障隔离** — 单个 Provider 挂了不影响其他 Provider 的监控和告警
5. **渐进式接入** — 用户可以先只注册直连 Provider（最简模式），再逐步添加外部网关

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
│   ├── provider.py        # Provider数据模型
│   ├── wr_models.py       # WebRouter自有表模型
│   └── provider_factory.py # Provider适配器工厂
├── routes/
│   ├── __init__.py
│   ├── dashboard.py       # 仪表盘API
│   ├── providers.py       # Provider管理API
│   ├── monitor.py         # 监控API
│   ├── alert.py           # 告警API
│   ├── billing.py         # 计费API
│   ├── team.py            # 团队管理API
│   ├── cli_export.py      # CLI配置导出API
│   └── settings.py        # 设置API
├── services/
│   ├── __init__.py
│   ├── provider_manager.py # Provider生命周期管理
│   ├── health_checker.py  # 统一健康检测
│   ├── alert_engine.py    # 告警引擎
│   ├── stats_collector.py # 统计采集
│   ├── cli_generator.py   # CLI配置生成
│   └── demo_data.py       # 演示数据
├── static/                # 前端静态文件
│   ├── js/
│   │   ├── providers.js   # Provider管理页面JS
│   │   └── ...
│   └── ...
└── data/                  # SQLite数据目录

wr-proxy/
├── main.go                # Go代理入口
├── handlers.go            # 请求处理（含多媒体端点）
├── proxy.go               # HTTP代理转发
├── router.go              # 路由注册
├── retry.go               # 重试引擎
├── desensitize.go         # 脱敏引擎
├── meter.go               # 成本计量
├── auth.go                # Token认证
├── config.go              # 配置管理
├── models.go              # 数据模型
├── smart_model.go         # 智能模型选择
├── sanitizer.go           # 响应清理
├── predictor.go           # 模型预测
├── health.go              # 健康检查
├── alert.go               # 告警集成
├── util.go                # 工具函数
├── data/                  # SQLite数据库
└── docs/                  # wr-proxy详细文档
```

### 2.2 Provider 数据模型

#### 2.2.1 wr_providers 表

```sql
CREATE TABLE wr_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,           -- 数据源名称
    type VARCHAR(20) NOT NULL,            -- direct/aggregate/litellm/custom
    base_url VARCHAR(500) NOT NULL,       -- API Base URL
    api_key VARCHAR(500),                 -- API Key (AES加密存储)
    api_key_masked VARCHAR(50),           -- 脱敏显示 sk-xxx...xxxx

    -- litellm 专有
    master_key VARCHAR(500),              -- LiteLLM Master Key

    -- custom 专有
    health_endpoint VARCHAR(500),         -- 自定义健康检测端点

    -- 通用配置
    models TEXT,                          -- JSON: 支持的模型列表
    tags TEXT,                            -- JSON: 标签 ["主力","备用"]
    weight INTEGER DEFAULT 100,           -- 调度权重 (0-100)
    priority INTEGER DEFAULT 0,           -- 优先级 (越高越优先)
    check_interval INTEGER DEFAULT 300,   -- 健康检测间隔(秒)
    enabled BOOLEAN DEFAULT TRUE,

    -- 状态(由系统自动维护)
    status VARCHAR(20) DEFAULT 'unchecked', -- healthy/warning/dead/disabled/unchecked
    last_check_at DATETIME,
    last_latency_ms INTEGER,
    last_error TEXT,

    -- 元数据
    notes TEXT,                           -- 备注
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### 2.2.2 Provider 适配器基类

```python
# models/provider_factory.py

class BaseProviderAdapter:
    """Provider 适配器基类 — 所有类型必须实现"""

    PROVIDER_TYPE = None  # 子类覆盖

    def __init__(self, provider: dict):
        self.provider = provider
        self.base_url = provider['base_url']
        self.api_key = provider.get('api_key', '')

    def check_health(self) -> dict:
        """健康检测 — 所有类型必须实现"""
        raise NotImplementedError

    def get_models(self) -> list:
        """获取支持的模型列表 — 可选"""
        return self.provider.get('models', [])


class DirectProviderAdapter(BaseProviderAdapter):
    """直连官方 API"""
    PROVIDER_TYPE = 'direct'

    def check_health(self) -> dict:
        # 根据 base_url 识别厂商，发送对应格式的测试请求
        ...


class AggregateProviderAdapter(BaseProviderAdapter):
    """聚合平台"""
    PROVIDER_TYPE = 'aggregate'

    def check_health(self) -> dict:
        # 聚合平台通常兼容 OpenAI 格式，走 /v1/chat/completions
        ...


class LiteLLMProviderAdapter(BaseProviderAdapter):
    """LiteLLM 代理"""
    PROVIDER_TYPE = 'litellm'

    def get_models(self) -> list:
        # GET /v1/models 自动发现
        ...


class CustomProviderAdapter(BaseProviderAdapter):
    """自定义网关"""
    PROVIDER_TYPE = 'custom'

    def check_health(self) -> dict:
        # 使用自定义 health_endpoint
        ...


class ProviderFactory:
    """Provider 适配器工厂"""

    _adapters = {
        'direct': DirectProviderAdapter,
        'aggregate': AggregateProviderAdapter,
        'litellm': LiteLLMProviderAdapter,
        'custom': CustomProviderAdapter,
    }

    @classmethod
    def create(cls, provider: dict) -> BaseProviderAdapter:
        provider_type = provider.get('type', 'custom')
        adapter_class = cls._adapters.get(provider_type, CustomProviderAdapter)
        return adapter_class(provider)
```

### 2.3 统一健康检测（支持多 Provider 类型）

```python
# services/health_checker.py
class HealthChecker:
    """统一健康检测引擎 — 支持所有 Provider 类型"""

    CHECK_INTERVAL = 300  # 默认5分钟

    def check_provider(self, provider: dict) -> dict:
        """检测单个 Provider — 根据类型自动选择检测策略"""
        from models.provider_factory import ProviderFactory
        adapter = ProviderFactory.create(provider)
        return adapter.check_health()

    def check_all_providers(self) -> list:
        """检测所有已启用的 Provider"""
        providers = Provider.query.filter_by(enabled=True).all()
        results = []
        for p in providers:
            result = self.check_provider(p.to_dict())
            # 更新 Provider 状态
            p.status = result['status']
            p.last_check_at = datetime.utcnow()
            p.last_latency_ms = result.get('latency_ms')
            p.last_error = result.get('error')
            # 写入健康历史
            health = ChannelHealth(
                provider_id=p.id,
                status=result['status'],
                latency_ms=result.get('latency_ms'),
                error_message=result.get('error'),
            )
            db.session.add(health)
            results.append(result)
        db.session.commit()
        return results
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
# Provider 管理
GET    /api/providers                    # Provider列表
POST   /api/providers                    # 注册新Provider
GET    /api/providers/:id                # Provider详情
PUT    /api/providers/:id                # 更新Provider配置
DELETE /api/providers/:id                # 删除Provider
POST   /api/providers/:id/check          # 手动触发单个Provider健康检测
POST   /api/providers/check_all          # 手动触发全量检测
GET    /api/providers/:id/models         # Provider支持的模型列表
GET    /api/providers/types              # 获取支持的Provider类型定义

# 仪表盘
GET  /api/dashboard/overview              # 总览数据(跨所有Provider聚合)
GET  /api/dashboard/trends                # 趋势数据(7天/30天)
GET  /api/dashboard/providers             # Provider列表+状态

# 监控
GET  /api/monitor/providers               # 所有Provider健康状态
GET  /api/monitor/history/:provider_id    # 检测历史

# 告警
GET  /api/alerts/rules                    # 告警规则列表
POST /api/alerts/rules                    # 创建告警规则
PUT  /api/alerts/rules/:id                # 更新规则
DELETE /api/alerts/rules/:id              # 删除规则
GET  /api/alerts/history                  # 告警历史
PUT  /api/alerts/channels                 # 配置告警通道

# 计费
GET  /api/billing/usage                   # 用量统计(跨Provider聚合)
GET  /api/billing/cost                    # 成本分析
GET  /api/billing/daily                   # 每日明细

# 团队
GET  /api/team/members                    # 成员列表
POST /api/team/members                    # 邀请成员
PUT  /api/team/members/:id                # 更新成员额度
DELETE /api/team/members/:id              # 移除成员
GET  /api/team/members/:id/usage          # 成员用量

# CLI对接
GET  /api/cli/export/:tool                # 导出配置(claude-code/codex/openclaw/hermes)
POST /api/cli/test                        # 测试连接

# 设置
GET  /api/settings                        # 系统设置
PUT  /api/settings                        # 更新设置
POST /api/settings/backup                 # 创建备份
POST /api/settings/restore                # 恢复备份
```

---

## 三、前端架构

### 3.1 页面结构

```
frontend/
├── index.html              # 管理后台SPA入口
├── css/
│   ├── style.css           # 统一样式（明亮主题）
└── js/
    ├── router.js           # Hash路由
    ├── api.js              # API请求封装
    ├── dashboard.js        # 仪表盘逻辑
    ├── providers.js        # Provider管理
    ├── channels.js         # 渠道管理
    ├── monitor.js          # 健康监控
    ├── alert.js            # 告警逻辑
    ├── billing.js          # 计费逻辑
    ├── team.js             # 团队逻辑
    ├── tokens.js           # 令牌管理
    ├── pricing.js          # 模型定价
    ├── modelgrades.js      # 模型分级
    ├── desensitize.js      # 脱敏规则
    ├── cli-export.js       # CLI导出逻辑
    └── settings.js         # 系统设置
```

### 3.2 前端路由(Hash路由)

```
#/                  → 仪表盘总览
#/providers         → 数据源管理
#/channels          → 渠道管理
#/monitor           → 健康监控
#/alerts            → 告警规则
#/billing           → 计费统计
#/team              → 团队管理
#/tokens            → 令牌管理
#/pricing           → 模型定价
#/modelgrades       → 模型分级
#/desensitize       → 脱敏规则
#/cli               → CLI对接
#/reqcache          → 请求缓存
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
  # wr-proxy 代理网关
  wr-proxy:
    build:
      context: ../wr-proxy
      dockerfile: ../deploy/Dockerfile-proxy
    container_name: webrouter-proxy
    restart: always
    ports:
      - "5051:5051"
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - proxy_data:/app/data
    depends_on:
      - webrouter

  # WebRouter管理后台
  webrouter:
    build:
      context: ../backend
      dockerfile: ../deploy/Dockerfile
    container_name: webrouter-app
    restart: always
    ports:
      - "5050:5050"
    environment:
      - TZ=Asia/Shanghai
      - DATABASE_URI=${DB_DSN:-sqlite:///data/webrouter.db}
      - REDIS_URL=redis://redis:6379
    volumes:
      - webrouter_data:/app/data
    depends_on:
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

volumes:
  proxy_data:
  webrouter_data:
  redis_data:
```

### 4.2 Nginx配置

```nginx
# deploy/nginx.conf
upstream webrouter {
    server webrouter:5050;
}
upstream wrproxy {
    server wr-proxy:5051;
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

    # WebRouter API
    location /api/ {
        proxy_pass http://webrouter;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # AI API转发（用户调用）
    location /v1/ {
        proxy_pass http://wrproxy;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
        proxy_read_timeout 300s;
        # SSE支持
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
    }
}
```

### 4.3 一键部署脚本

```bash
#!/bin/bash
# deploy/install.sh

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
echo "管理后台: http://$PUBLIC_IP:5050"
echo "代理网关: http://$PUBLIC_IP:5051"
echo "默认密码已写入 .env 文件"
echo "=================="
```

---

## 五、数据模型

### 5.1 WebRouter自有表

```sql
-- Provider 数据源
CREATE TABLE wr_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,           -- 数据源名称
    type VARCHAR(20) NOT NULL,            -- direct/aggregate/litellm/custom
    base_url VARCHAR(500) NOT NULL,       -- API Base URL
    api_key VARCHAR(500),                 -- API Key (AES加密存储)
    api_key_masked VARCHAR(50),           -- 脱敏显示 sk-xxx...xxxx
    master_key VARCHAR(500),              -- Master Key(litellm, AES加密)
    health_endpoint VARCHAR(500),         -- 自定义健康端点(custom)
    models TEXT,                          -- JSON: 支持的模型列表
    tags TEXT,                            -- JSON: 标签 ["主力","备用"]
    weight INTEGER DEFAULT 100,           -- 调度权重(0-100)
    priority INTEGER DEFAULT 0,           -- 优先级
    check_interval INTEGER DEFAULT 300,   -- 健康检测间隔(秒)
    enabled BOOLEAN DEFAULT TRUE,
    status VARCHAR(20) DEFAULT 'unchecked', -- healthy/warning/dead/disabled/unchecked
    last_check_at DATETIME,
    last_latency_ms INTEGER,
    last_error TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

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
    provider_id INTEGER,                    -- 关联 Provider ID
    channel_id INTEGER,                     -- 渠道ID
    status VARCHAR(20) NOT NULL,            -- healthy/warning/dead/disabled
    latency_ms INTEGER,
    error_message TEXT,
    checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_provider_time (provider_id, checked_at),
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
| API网关 | wr-proxy (Go) | 自主研发，零依赖，支持二进制流/脱敏/重试/智能路由 |
| 管理后端 | Flask (Python) | 技术栈统一，开发快 |
| 前端 | 原生HTML/CSS/JS | 与包装站一致，无需构建工具 |
| 图表 | Chart.js | 轻量(60KB)，按需加载 |
| 数据库 | SQLite | 纯 Go/Python 驱动，无需 CGO，单文件部署 |
| 缓存 | Redis | 可选，用于会话和实时统计 |
| 部署 | Docker Compose | 一键部署 |
| 反向代理 | Nginx | SSL、路由分发、静态资源 |
| 进程管理 | Gunicorn / start.py | Flask生产级WSGI + 双进程管理 |
| 定时任务 | APScheduler | 健康检测、统计采集、告警评估 |

---

## 七、安全设计

| 措施 | 说明 |
|------|------|
| HTTPS | Nginx终止SSL，Let's Encrypt自动续期 |
| Key加密存储 | API Key 加密后存储 |
| 管理后台认证 | JWT Token |
| API限流 | Nginx层限流，防止暴力请求 |
| 数据隔离 | 团队版用户数据严格隔离，按user_id过滤 |
| 备份加密 | 备份文件AES加密，传输用HTTPS |
