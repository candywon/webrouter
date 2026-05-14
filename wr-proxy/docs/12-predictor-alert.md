# 12 - 预测与告警 (predictor.go + alert.go)

## predictor.go 额度消耗预测

### Predictor 结构
```
type Predictor struct{}  // 无状态，依赖DB和Provider数据
```

### PredictExhaustion(provider) → *QuotaPrediction

```
1. getDailyCosts(providerID, 7天) → 每日成本数组
2. calculateBurnRate(dailyCosts) → 日均消耗率
3. detectTrend(dailyCosts) → increasing/stable/decreasing
4. DaysUntilExhaust = QuotaRemaining / DailyBurnRate
5. alertLevel(quotaRatio, daysUntilExhaust) → green/yellow/orange/red/black
6. calculateConfidence(dailyCosts) → 0~1
7. exhaustDate(daysUntilExhaust) → 日期字符串
```

### calculateBurnRate(dailyCosts)
- 去掉0值(无消耗的天)
- 取最近7天均值
- 无数据 → 0

### detectTrend(dailyCosts)
- 最近3天均值 vs 前4天均值
- 差异 > 20% → increasing
- 差异 < -20% → decreasing
- 其他 → stable

### alertLevel(quotaRatio, daysUntilExhaust)

| quotaRatio | daysUntil | level |
|------------|-----------|-------|
| <=0 | any | black (已耗尽) |
| <0.05 | <1 | red |
| <0.05 | <3 | orange |
| <0.2 | <7 | yellow |
| 其他 | - | green |

### calculateConfidence(dailyCosts)
- 数据天数 >= 7 → 0.9
- 3-6天 → 0.7
- 1-2天 → 0.4
- 0天 → 0.1

### PredictAll() → []*QuotaPrediction
遍历所有有额度信息的Provider

## alert.go 告警引擎

### AlertEngine 结构
```
type AlertEngine struct {
    mu       sync.Mutex
    cooldown map[string]time.Time  // 告警冷却 key: "providerID:alertType"
}
```

### AlertEvent 结构
```
type AlertEvent struct {
    ProviderID   int    `json:"provider_id"`
    ProviderName string `json:"provider_name"`
    Type         string `json:"type"`    // quota_critical/quota_warning/health_dead/health_degraded/error_spike
    Level        string `json:"level"`   // red/orange/yellow/green
    Message      string `json:"message"`
    Timestamp    string `json:"timestamp"`
}
```

### EvaluateAll() → []AlertEvent

遍历所有Provider，检测:

| 条件 | alertType | level | 消息 |
|------|-----------|-------|------|
| Status=="dead" | health_dead | red | "Provider X 已宕机" |
| Status=="rate_limited" | health_degraded | orange | "Provider X 被限速" |
| Status=="auth_failed" | health_degraded | red | "Provider X 认证失败" |
| ConsecFails >= 3 | error_spike | orange | "Provider X 连续N次失败" |
| QuotaRatio < 0.05 | quota_critical | red | "Provider X 额度紧急(<5%)" |
| QuotaRatio < 0.2 | quota_warning | yellow | "Provider X 额度预警(<20%)" |

### filterCooldown(events)
- 同一 ProviderID:alertType 在5分钟内只触发一次
- 告警冷却避免重复通知

### NotifyAlerts(events)
- 当前仅日志输出: LogWarn("ALERT: ...")
- 可扩展: Webhook/邮件/钉钉等
