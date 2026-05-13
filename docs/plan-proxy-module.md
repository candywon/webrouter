# P0: WebRouter 一体化网关 — 实现计划

## 核心决策：移除 New-API 依赖

WebRouter 从"New-API 的管理面板"升级为"独立的 AI API 网关"。
不再依赖 New-API sidecar，所有功能自建。

## 架构变更

### Before（双系统割裂）
```
浏览器 → WebRouter(:5050) → 读 New-API DB
客户端 → New-API(:3000)  → 上游 API
              ↑ Go 黑盒，不可定制
```

### After（单系统闭环）
```
浏览器 → WebRouter(:5050/api/*)  → 管理后台
客户端 → WebRouter(:5050/v1/*)   → 代理转发 → 上游 API
              ↑ Python 全栈，完全可控
```

## 数据模型

### wr_providers（已有，扩展字段）
```
+ proxy_enabled     BOOL    是否纳入代理池
+ rate_limit_rpm    INT     每分钟请求上限
+ timeout_seconds   INT     请求超时(默认30)
+ max_retries       INT     最大重试次数(默认2)
+ cost_multiplier   FLOAT   成本倍率(默认1.0)
```

### wr_tokens（新增）— 对外 API Key
```
id, name, key (sk-wr-xxxx)
user_id
models          TEXT    JSON: 允许的模型 ["gpt-4o","claude-3"]
providers       TEXT    JSON: 允许的 Provider ID（空=全部）
quota_total     BIGINT  总额度(分)
quota_used      BIGINT  已用额度(分)
rate_limit_rpm  INT     每分钟限速
subnet_whitelist TEXT   JSON: IP 白名单
enabled         BOOL
expires_at      DATETIME
created_at
```

### wr_request_logs（新增）— 核心计量表
```
id, request_id   UUID
token_id         INT     关联 WR Token
provider_id      INT     实际转发的 Provider
model_name       VARCHAR
endpoint         VARCHAR /v1/chat/completions 等
input_tokens     BIGINT
output_tokens    BIGINT
status_code      INT     上游 HTTP 状态
latency_ms       INT     端到端延迟
cost_cents       INT     估算成本(分)
is_stream        BOOL
is_retry         BOOL    是否重试请求
error_message    TEXT
created_at       DATETIME  INDEX
```

### wr_channel_health（已有，不变）
### wr_alert_rules（已有，不变）
### wr_alert_history（已有，不变）
### wr_team_quotas（已有，对接 Token 后生效）
### wr_cost_records（已有，数据源从 New-API DB 改为 wr_request_logs）

### 删除的模型
- NewAPIAdapter → 不再需要，删除 newapi_adapter.py
- Provider 中 newapi/oneapi 专有字段(admin_token, db_uri) → 废弃

## 核心服务

### 1. services/router.py — 智能调度引擎
```python
class Router:
    def select_provider(model, token, strategy='priority') -> Provider:
        """根据策略选择最优 Provider"""
        # 1. 过滤: enabled + healthy + 支持 model + token 白名单
        # 2. 策略选择:
        #    priority:   按 priority DESC, 同级按 weight 随机
        #    round_robin: 同优先级加权轮询
        #    least_latency: 选最近延迟最低的
        #    cost_first: 选 cost_multiplier 最低的
        # 3. 返回 None = 无可用 Provider (返回 503)

    def handle_failover(model, token, failed_provider) -> Provider:
        """失败降级：排除 failed_provider，重新选择"""
```

### 2. services/proxy.py — 代理转发核心
```python
class ProxyService:
    def forward(provider, endpoint, headers, body, is_stream) -> Response:
        """
        转发请求到上游 Provider
        - 构造上游 URL: provider.base_url + endpoint
        - 替换 Authorization: 用 provider.api_key
        - 流式: SSE 逐 chunk 透传 + 提取 usage
        - 非流式: 完整转发 + 解析 usage
        - 超时: provider.timeout_seconds
        """

    def _parse_usage(response_body, is_stream) -> dict:
        """从响应中提取 token 用量"""
        # 非流式: response.usage
        # 流式: 最后一个 data chunk 的 usage

    def _stream_generator(upstream_resp, request_id, provider_id, model):
        """SSE 流式生成器 — 逐行透传"""
        # yield 每行 SSE
        # 识别 stream_end 或 usage 段
        # 写 RequestLog
```

### 3. services/meter.py — 计量服务
```python
class MeterService:
    def record_request(token_id, provider_id, model, endpoint,
                       input_tokens, output_tokens, latency_ms,
                       status_code, is_stream, error) -> RequestLog:
        """写入请求日志 + 更新配额"""

    def check_quota(token_id) -> bool:
        """检查配额是否充足"""

    def check_rate_limit(token_id) -> bool:
        """检查 RPM 限速（Redis 计数）"""
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

流程：
1. 鉴权: Bearer sk-wr-xxx → 查 WR Token → 验证 enabled + expires
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
GET    /<id>          Token 详情
PUT    /<id>          更新 Token
DELETE /<id>          删除 Token
POST   /<id>/reset    重置配额
GET    /<id>/usage    Token 用量
```

### 修改 routes/billing.py
- 数据源改为 wr_request_logs（不再依赖 New-API DB）
- 移除 _has_newapi() 检查
- 移除 demo_data 回退

### 修改 routes/providers.py
- 新增 proxy_enabled / rate_limit_rpm / timeout_seconds 等字段
- 移除 newapi/oneapi 专有渠道获取逻辑

## 移除 New-API 相关代码

### 删除
- models/newapi_adapter.py — 不再直读 New-API DB
- Provider 中 admin_token, db_uri 字段 — 废弃
- services/stats_collector.py — 被 MeterService 替代
- services/demo_data.py — 不再需要 demo 数据
- deploy/install.sh 中 New-API 二进制下载逻辑
- deploy/update-newapi.sh
- scripts/build-newapi.sh
- bin/ 目录

### 简化 Provider 类型
- direct    → 保持（直连官方）
- aggregate → 保持（聚合平台）
- newapi    → 废弃或降级为 "custom"（如果用户已有 New-API 实例，当 custom 接入）
- oneapi    → 同上
- litellm   → 保持（当普通上游接入）
- custom    → 保持

实际代理转发只关心：base_url + api_key + models，类型只影响健康检测策略。

## 实现顺序

### Phase 1: 核心代理（最小可用）
1. DB: RequestLog + WRToken 模型 → wr_models.py
2. services/router.py — 调度引擎
3. services/proxy.py — 非流式代理
4. services/meter.py — 计量
5. routes/proxy.py — /v1/* 入口
6. routes/tokens.py — Token 管理
7. curl 端到端测试

### Phase 2: 流式 + 增强
8. proxy.py 加 SSE 流式透传
9. 限速（Redis RPM 计数）
10. 失败重试 + failover
11. billing/stats 对接 RequestLog

### Phase 3: 清理
12. 删除 New-API 相关代码
13. 删除 newapi/oneapi Provider 类型
14. 更新 README + 部署脚本
15. 前端 Token 管理页面

## 合规设计
- 请求 body 不落盘，只提取 usage 字段
- 日志保留周期可配置（默认 90 天）
- Token key 加密存储（AES-256）
- IP 白名单限制
