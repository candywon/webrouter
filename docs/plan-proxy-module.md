# P0: WebRouter 一体化 AI API 网关 — 最终实现计划

## 产品定位

**WebRouter — 离用户最近的 AI API 网关**

面向：中小企业 / 个体创业者 / 自媒体 / 开发者 / 个人
部署：局域网 / 个人电脑 / 工作站
规模：3-20 人同时使用（峰值 30 并发）

核心场景：
1. **企业内部分配** — 给每个员工/部门发独立 API Key，追踪谁用了多少
2. **用量管控** — 设额度、限速，防止某人跑空账户
3. **智能调度** — 多个 API 源自动选最优，挂了自动切
4. **预警** — 额度将尽、Provider 故障、异常用量及时通知
5. **成本透明** — 每个模型每个用户花了多少钱一目了然

合规：不碰翻墙，只做正规 API 管理。

## 架构

```
客户端 (Cursor/ChatGPT-Next-Web/自研App/脚本)
    │
    │  POST /v1/chat/completions
    │  Authorization: Bearer sk-wr-xxxx
    ▼
┌─────────────────────────────────────────┐
│  WebRouter (Flask + gevent :5050)        │
│                                          │
│  /api/*  — 管理后台 (Python, 已有)        │
│    Provider CRUD, Token 管理,            │
│    仪表盘, 告警, 计费, 团队              │
│                                          │
│  /v1/*   — 代理转发 (Python, 新增)        │
│    ① 鉴权 WR Token                       │
│    ② 调度选 Provider                     │
│    ③ 异步转发 (aiohttp)                  │
│    ④ 流式 SSE 透传                       │
│    ⑤ 计量写 RequestLog                   │
│    ⑥ 配额/限速 拦截                      │
│                                          │
│  SQLite DB (共享)                         │
│    wr_providers                          │
│    wr_tokens                             │ ← 新增
│    wr_request_logs                       │ ← 新增
│    wr_cost_records                       │
│    wr_team_quotas                        │
│    wr_alert_rules / wr_alert_history     │
│    wr_channel_health                     │
└─────────────────────────────────────────┘
         │
         │ 根据 provider.base_url + api_key 转发
         ▼
    上游 AI API (OpenAI / Claude / 聚合平台 / ...)

未来扩展（按需）:
    当规模超过 Python 舒适区(50+并发)时，
    /v1/* 可替换为 Go sidecar (wr-proxy :5051)，
    管理后台和 API 接口不变。
```

## 数据模型变更

### wr_providers（已有，扩展字段）
```sql
ALTER TABLE wr_providers ADD COLUMN proxy_enabled BOOLEAN DEFAULT 1;
ALTER TABLE wr_providers ADD COLUMN rate_limit_rpm INTEGER DEFAULT 0;
ALTER TABLE wr_providers ADD COLUMN timeout_seconds INTEGER DEFAULT 30;
ALTER TABLE wr_providers ADD COLUMN max_retries INTEGER DEFAULT 2;
ALTER TABLE wr_providers ADD COLUMN cost_multiplier REAL DEFAULT 1.0;
```

### wr_tokens（新增）— 对外 API Key，核心管控单元
```sql
CREATE TABLE wr_tokens (
    id INTEGER PRIMARY KEY,
    name VARCHAR(100) NOT NULL,          -- Token 名称（如"张三-研发部"）
    key VARCHAR(64) NOT NULL UNIQUE,     -- sk-wr-xxxxxxxxxxxx
    user_id INTEGER,                     -- 关联团队用户
    models TEXT,                         -- JSON: 允许的模型 ["gpt-4o","claude-3"]
    provider_ids TEXT,                   -- JSON: 允许的 Provider ID（空=全部）
    quota_total BIGINT DEFAULT 0,        -- 总额度(分), 0=不限
    quota_used BIGINT DEFAULT 0,         -- 已用额度(分)
    rate_limit_rpm INTEGER DEFAULT 0,    -- 每分钟限速, 0=不限
    subnet_whitelist TEXT,               -- JSON: IP 白名单
    enabled BOOLEAN DEFAULT 1,
    expires_at DATETIME,                 -- 过期时间
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### wr_request_logs（新增）— 核心计量表
```sql
CREATE TABLE wr_request_logs (
    id INTEGER PRIMARY KEY,
    request_id VARCHAR(36) NOT NULL,     -- UUID
    token_id INTEGER NOT NULL,           -- 关联 WR Token
    token_name VARCHAR(100),             -- 冗余，方便查询
    provider_id INTEGER NOT NULL,        -- 实际转发的 Provider
    provider_name VARCHAR(100),          -- 冗余
    model_name VARCHAR(100) NOT NULL,
    endpoint VARCHAR(200) NOT NULL,      -- /v1/chat/completions 等
    input_tokens BIGINT DEFAULT 0,
    output_tokens BIGINT DEFAULT 0,
    status_code INTEGER,                 -- 上游 HTTP 状态
    latency_ms INTEGER,                  -- 端到端延迟
    cost_cents INTEGER DEFAULT 0,        -- 估算成本(分)
    is_stream BOOLEAN DEFAULT 0,
    is_retry BOOLEAN DEFAULT 0,          -- 是否重试请求
    error_message TEXT,
    client_ip VARCHAR(45),               -- 客户端 IP
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_rlog_token ON wr_request_logs(token_id, created_at);
CREATE INDEX idx_rlog_provider ON wr_request_logs(provider_id, created_at);
CREATE INDEX idx_rlog_model ON wr_request_logs(model_name, created_at);
CREATE INDEX idx_rlog_created ON wr_request_logs(created_at);
```

## 核心服务（Python）

### 1. services/router.py — 智能调度引擎
```python
class Router:
    def select_provider(model: str, token: dict, 
                       strategy='priority') -> Optional[Provider]:
        """
        根据策略选择最优 Provider
        1. 过滤: enabled + healthy + 支持 model + token 白名单
        2. 策略:
           - priority:      按 priority DESC, 同级按 weight 随机
           - round_robin:   同优先级加权轮询
           - least_latency: 选最近延迟最低的
           - cost_first:    选 cost_multiplier 最低的
        3. 返回 None = 无可用 Provider → 503
        """

    def handle_failover(model: str, token: dict, 
                       failed_provider_id: int) -> Optional[Provider]:
        """失败降级：排除 failed_provider，重新选择"""
```

### 2. services/proxy.py — 代理转发核心
```python
class ProxyService:
    async def forward(provider, endpoint, headers, body, 
                      is_stream) -> Response:
        """
        异步转发请求到上游 Provider
        - 构造上游 URL: provider.base_url + endpoint
        - 替换 Authorization: 用 provider.api_key
        - 流式: SSE 逐 chunk 透传 + 提取 usage
        - 非流式: 完整转发 + 解析 usage
        - 超时: provider.timeout_seconds
        """

    def _parse_usage(response_body, is_stream) -> dict:
        """从响应中提取 token 用量"""

    def _stream_generator(upstream_resp, request_id, ...) -> Generator:
        """SSE 流式生成器 — 逐行透传"""
```

### 3. services/meter.py — 计量服务
```python
class MeterService:
    def record_request(...) -> RequestLog:
        """写入请求日志 + 扣配额 + 累计统计"""

    def check_quota(token_id) -> bool:
        """检查配额是否充足"""

    def check_rate_limit(token_id) -> bool:
        """检查 RPM 限速（内存计数，无需 Redis）"""
```

### 4. services/cost_calculator.py — 成本估算
```python
class CostCalculator:
    # 主流模型定价表 (分/千token)
    PRICING = {
        'gpt-4o':         {'input': 0.18, 'output': 0.54},
        'gpt-4o-mini':    {'input': 0.012, 'output': 0.048},
        'claude-3.5-sonnet': {'input': 0.21, 'output': 1.05},
        'claude-3-haiku': {'input': 0.018, 'output': 0.09},
        'deepseek-chat':  {'input': 0.009, 'output': 0.027},
        # ... 可配置扩展
    }

    def calculate(model, input_tokens, output_tokens, 
                  multiplier=1.0) -> int:
        """估算成本(分)"""
```

## API 路由

### routes/proxy.py — 代理入口 (url_prefix='/v1')
```
POST /v1/chat/completions     代理聊天
POST /v1/completions          代理补全
POST /v1/embeddings           代理嵌入
POST /v1/images/generations   代理图片生成
GET  /v1/models               聚合模型列表
```

流程:
1. 鉴权: Bearer sk-wr-xxx → 查 WR Token → enabled + 未过期
2. 限速: MeterService.check_rate_limit()
3. 配额: MeterService.check_quota()
4. 路由: Router.select_provider(model, token)
5. 转发: ProxyService.forward(provider, ...)
6. 计量: MeterService.record_request(...)
7. 返回: 透传上游响应

### routes/tokens.py — Token 管理 (url_prefix='/api/tokens')
```
GET    /              Token 列表
POST   /              创建 Token
GET    /<id>          Token 详情（含用量摘要）
PUT    /<id>          更新 Token
DELETE /<id>          删除 Token
POST   /<id>/reset-quota  重置配额
GET    /<id>/usage    Token 用量明细
GET    /<id>/cost     Token 成本明细
```

### 修改 routes/billing.py
- 数据源改为 wr_request_logs
- 按 Token / Provider / Model / 时间 四维聚合
- 移除 New-API DB 依赖和 demo_data

### 修改 routes/dashboard.py
- 仪表盘数据改为自有的 wr_request_logs
- 新增: Token 用量排行、部门用量对比

## 移除 New-API

### 删除
- models/newapi_adapter.py
- services/demo_data.py
- services/stats_collector.py → 被 MeterService 替代
- deploy/update-newapi.sh, scripts/build-newapi.sh
- Provider 中 admin_token, db_uri 字段
- start.py 中 New-API 启动/停止逻辑

### Provider 类型简化
- direct    → 保持（直连官方）
- aggregate → 保持（聚合平台）
- litellm   → 保持
- custom    → 保持
- newapi    → 废弃，迁移提示改用 custom
- oneapi    → 废弃，同上

## 企业场景特色功能

### 1. 部门/员工维度追踪
- Token 绑定 user_id + 部门标签
- 仪表盘按部门聚合用量
- 导出 CSV 报表

### 2. 额度预警
- 额度使用 80%/95% 时自动告警
- 额度耗尽可选择: 拒绝 / 降级到便宜模型 / 通知管理员

### 3. 模型白名单
- 每个 Token 可限定可用模型
- 防止员工用 gpt-4o 烧钱，限定 gpt-4o-mini

### 4. 成本预算
- 每月设定预算上限
- 超预算自动限制或通知

### 5. 审计日志
- 请求元数据（不含内容）完整记录
- 可追溯谁在什么时间用了什么模型

## 实现顺序

### Phase 1: 核心代理 — 最小可用（P0）
1. DB: WRToken + RequestLog 模型
2. services/router.py — 调度引擎
3. services/proxy.py — 非流式代理
4. services/meter.py — 计量
5. services/cost_calculator.py — 成本估算
6. routes/proxy.py — /v1/* 入口
7. routes/tokens.py — Token CRUD
8. curl 端到端测试

### Phase 2: 流式 + 企业功能（P1）
9. proxy.py 加 SSE 流式透传
10. 限速（RPM 内存计数）
11. 失败重试 + failover
12. 额度预警（80%/95%）
13. 模型白名单
14. billing/stats 对接 RequestLog

### Phase 3: 清理 + 前端（P2）
15. 删除 New-API 相关代码
16. 更新 start.py（单进程）
17. 前端 Token 管理页面
18. 前端用量/成本看板
19. 更新 README + 部署脚本
20. 导出 CSV 报表

### 未来扩展（按需）
21. wr-proxy Go sidecar（当规模超过 50 并发时）
22. 多租户（SaaS 模式）
23. 对接企业微信/飞书通知
