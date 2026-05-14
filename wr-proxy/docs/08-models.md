# 08 - 数据模型 (models.go)

## Provider 结构

```
type Provider struct {
    // ── 基础信息 ──
    ID             int       `json:"id"`
    Name           string    `json:"name"`
    Type           string    `json:"type"`            // direct/aggregate/litellm/custom
    BaseURL        string    `json:"base_url"`
    APIKey         string    `json:"api_key,omitempty"`
    Models         string    `json:"models"`          // JSON array: ["gpt-4o","claude-3"]
    Tags           string    `json:"tags"`            // JSON array

    // ── 调度参数 ──
    Priority       int       `json:"priority"`        // 90+主力, 50-89热备, 1-49冷备, 0禁用
    Weight         int       `json:"weight"`          // 调度权重 0-100
    ProxyEnabled   bool      `json:"proxy_enabled"`   // 是否纳入代理池
    RateLimitRPM   int       `json:"rate_limit_rpm"`  // 每分钟请求上限, 0=不限
    TimeoutSeconds int       `json:"timeout_seconds"` // 请求超时
    MaxRetries     int       `json:"max_retries"`     // 最大重试次数
    CostMultiplier float64   `json:"cost_multiplier"` // 成本倍率

    // ── 状态 ──
    Enabled        bool      `json:"enabled"`
    Status         string    `json:"status"`          // healthy/warning/dead/rate_limited/disabled/auth_failed
    LastCheckAt    *time.Time `json:"last_check_at"`
    LastLatencyMs  int       `json:"last_latency_ms"`
    LastError      string    `json:"last_error,omitempty"`

    // ── 额度 ──
    QuotaTotal     int64     `json:"quota_total"`     // 总额度(分), 0=不限
    QuotaUsed      int64     `json:"quota_used"`      // 已用额度(分)
    QuotaSource    string    `json:"quota_source"`    // manual/api/unknown

    // ── 能力 ──
    SupportsTools  bool      `json:"supports_tools"`  // 支持function calling

    // ── 运行时(不持久化) ──
    HealthStatus   string    // 缓存健康状态
    ConsecFails    int       // 连续失败次数
    CooldownUntil  *time.Time // 冷却截止时间
}
```

### Provider 方法

| 方法 | 逻辑 |
|------|------|
| IsAvailable(model) | Enabled && ProxyEnabled && Status∉{dead,disabled,auth_failed} && modelInList && 非冷却期 |
| PriorityGroup() | >=90→"primary", >=50→"hot", else→"cold" |
| QuotaRatio() | QuotaTotal>0 ? (QuotaTotal-QuotaUsed)/QuotaTotal : 1.0 |
| QuotaRemaining() | QuotaTotal-QuotaUsed, 或-1(不限) |

## Token 结构

```
type Token struct {
    ID              int        `json:"id"`
    Name            string     `json:"name"`
    Key             string     `json:"key"`             // sk-wr-...xxxx
    UserID          int        `json:"user_id"`
    Models          string     `json:"models"`          // JSON array, 空=全部
    ProviderIDs     string     `json:"provider_ids"`    // JSON array, 空=全部
    QuotaTotal      int64      `json:"quota_total"`     // 总额度(分), 0=不限
    QuotaUsed       int64      `json:"quota_used"`      // 已用额度(分)
    RateLimitRPM    int        `json:"rate_limit_rpm"`  // 每分钟限速, 0=不限
    SubnetWhitelist string     `json:"subnet_whitelist"`// JSON array
    SmartDowngrade  bool       `json:"smart_downgrade"` // 允许自动降级
    Enabled         bool       `json:"enabled"`
    ExpiresAt       *time.Time `json:"expires_at"`
    CreatedAt       time.Time  `json:"created_at"`
}
```

### Token 方法

| 方法 | 逻辑 |
|------|------|
| IsExpired() | ExpiresAt非空 && now.After(ExpiresAt) |
| QuotaRemaining() | QuotaTotal-QuotaUsed, 或-1(不限) |
| QuotaRatio() | QuotaTotal>0 ? (QuotaTotal-QuotaUsed)/QuotaTotal : 1.0 |
| CanUseModel(model) | Models为空 || model在列表中 |
| CanUseProvider(id) | ProviderIDs为空 || id在列表中 |

## RequestLog 结构

```
type RequestLog struct {
    ID            int64     `json:"id"`
    RequestID     string    `json:"request_id"`
    TokenID       int       `json:"token_id"`
    TokenName     string    `json:"token_name"`
    ProviderID    int       `json:"provider_id"`
    ProviderName  string    `json:"provider_name"`
    ModelName     string    `json:"model_name"`
    Endpoint      string    `json:"endpoint"`
    InputTokens   int64     `json:"input_tokens"`
    OutputTokens  int64     `json:"output_tokens"`
    StatusCode    int       `json:"status_code"`
    LatencyMs     int       `json:"latency_ms"`
    CostCents     int64     `json:"cost_cents"`
    IsStream      bool      `json:"is_stream"`
    IsRetry       bool      `json:"is_retry"`
    ErrorMessage  string    `json:"error_message,omitempty"`
    ErrorType     string    `json:"error_type,omitempty"` // quota_exhausted/rate_limited/timeout/truncated/unknown
    ClientIP      string    `json:"client_ip"`
    CreatedAt     time.Time `json:"created_at"`
}
```

## QuotaPrediction 结构

```
type QuotaPrediction struct {
    ProviderID           int     `json:"provider_id"`
    QuotaRemaining       int64   `json:"quota_remaining"`
    DailyBurnRate        float64 `json:"daily_burn_rate"`
    DaysUntilExhaust     float64 `json:"days_until_exhaust"`
    PredictedExhaustDate string  `json:"predicted_exhaust_date"`
    Trend                string  `json:"trend"` // increasing/stable/decreasing
    Confidence           float64 `json:"confidence"`
    AlertLevel           string  `json:"alert_level"` // green/yellow/orange/red/black
}
```
