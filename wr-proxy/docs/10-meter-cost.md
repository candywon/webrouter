# 10 - 统计计量 (meter.go + cost.go)

## meter.go 请求计量

### Meter 结构
```
type Meter struct {
    mu             sync.Mutex
    providerMinute map[int]*minuteBucket  // key: providerID
}

type minuteBucket struct {
    count      int     // 请求总数
    validCount int     // 有效请求数
    tokens     int64   // token总量
    costCents  int64   // 成本(分)
    start      time.Time // 窗口起始
}
```

### RecordRequest(rlog *RequestLog)

```
1. 写DB日志: InsertRequestLog(rlog)
2. 扣Token配额: DeductTokenQuota(tokenID, costCents) — 仅status<400时
3. 累加Provider用量: UpdateProviderQuota(providerID, costCents)
4. 内存缓存(实时统计):
   - 当前分钟窗口 → 累加count/validCount/tokens/costCents
   - 超过1分钟 → 新窗口
```

### GetProviderMinuteStats(providerID) → (count, validCount, tokens, cost)

### BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, isFailover) → *RequestLog

构建RequestLog:
- TokenID/TokenName/ProviderID/ProviderName 从token/provider取
- InputTokens/OutputTokens/LatencyMs 从result取
- IsRetry = isFailover
- StatusCode 从result取
- CostCents = CalculateCost(model, inputTokens, outputTokens, provider.CostMultiplier)
- ErrorType 从result.UpstreamError.Type取
- ErrorMessage 从result.Error取

**有效请求判定**: status_code < 400 AND is_retry = false AND error_message = ""

## cost.go 定价与成本

### ModelPricing
```
type ModelPricing struct {
    Input  float64  // 输入价格(每1K tokens, 单位:分)
    Output float64  // 输出价格(每1K tokens, 单位:分)
}
```

### PricingCache
```
type PricingCache struct {
    mu      sync.RWMutex
    table   map[string]ModelPricing
    default_ ModelPricing  // 未知模型默认: {Input: 0.015, Output: 0.06}
}
```

### RefreshPricing() → error
从DB wr_model_pricing 表加载所有定价到内存缓存

### CalculateCost(model, inputTokens, outputTokens, multiplier) → int64 (分)
```
cost = (inputTokens/1000 * Input + outputTokens/1000 * Output) * multiplier
四舍五入到整数分
```

### GetModelPricing(model) → (ModelPricing, bool)
从缓存查定价，未知模型返回default_

### GetAllPricing() → map
返回完整定价表快照
