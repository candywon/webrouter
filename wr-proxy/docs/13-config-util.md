# 13 - 配置与工具 (config.go + util.go)

## config.go 配置加载

### Config 结构
```
type Config struct {
    // ── 服务 ──
    ListenAddr string        // 默认 :5051
    DBPath     string        // SQLite路径
    FlaskURL   string        // Flask管理后台地址

    // ── 代理 ──
    DefaultTimeout  time.Duration  // 请求超时 默认60s
    MaxRetryCount   int            // 同Provider最大重试 默认2
    MaxFailover     int            // 最大降级次数 默认3
    IdleConnTimeout time.Duration  // 连接池空闲超时 默认90s
    MaxIdleConns    int            // 连接池最大连接 默认100
    MaxBodySize     int64          // 非流式最大body 默认10MB

    // ── 调度 ──
    RoutingStrategy string        // smart/priority/round_robin/least_latency/cost_first

    // ── 额度 ──
    QuotaWarnThreshold     float64  // 额度预警阈值 默认0.2(20%)
    QuotaCriticalThreshold float64  // 额度紧急阈值 默认0.05(5%)
    PredictionDays         int     // 预测用近N天数据 默认7

    // ── 健康检测 ──
    HealthCheckInterval time.Duration  // 默认5min
    HealthTimeout       time.Duration  // 默认15s

    // ── 告警 ──
    AlertCooldown time.Duration  // 同一告警冷却 默认5min
}
```

### 环境变量映射

| 环境变量 | 字段 | 默认值 |
|----------|------|--------|
| WR_PORT | ListenAddr | :5051 |
| WR_DB_PATH | DBPath | data/webrouter.db |
| WR_FLASK_URL | FlaskURL | http://localhost:5050 |
| WR_TIMEOUT | DefaultTimeout | 60s |
| WR_MAX_RETRY | MaxRetryCount | 2 |
| WR_MAX_FAILOVER | MaxFailover | 3 |
| WR_STRATEGY | RoutingStrategy | smart |
| WR_QUOTA_WARN | QuotaWarnThreshold | 0.2 |
| WR_QUOTA_CRITICAL | QuotaCriticalThreshold | 0.05 |
| WR_PREDICTION_DAYS | PredictionDays | 7 |
| WR_HEALTH_INTERVAL | HealthCheckInterval | 5m |
| WR_HEALTH_TIMEOUT | HealthTimeout | 15s |
| WR_ALERT_COOLDOWN | AlertCooldown | 5m |
| WR_MAX_BODY | MaxBodySize | 10485760(10MB) |

### 辅助函数
- envStr(key, fallback) → 字符串
- envInt(key, fallback) → 整数
- envFloat(key, fallback) → 浮点
- envDuration(key, fallback) → 时间段(支持30s/5m/1h)

## util.go 工具函数

### 日志
```
LogInfo(format, args...)
LogWarn(format, args...)
LogError(format, args...)
```
格式: `[WR-PROXY] 2026-05-14 12:00:00 [INFO] message`

### containsJSONString(jsonStr, target) → bool
在JSON数组字符串中查找目标值（避免每请求JSON parse）

### min(a, b int) → int
