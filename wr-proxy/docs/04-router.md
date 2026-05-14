# 04 - Provider 路由调度 (router.go)

## Router 结构
```
type Router struct {
    mu        sync.RWMutex
    providers []*Provider
    strategy  string  // smart/priority/round_robin/least_latency/cost_first
}
```

## SelectProvider 主函数

```
输入: model, token, excludeIDs
输出: *Provider (nil=无可用)
```

### 1. 过滤条件

| 条件 | 逻辑 |
|------|------|
| Provider可用 | p.IsAvailable(model): Enabled && ProxyEnabled && Status非dead/disabled/auth_failed && model在列表中 && 非冷却期 |
| Token可用 | token.CanUseProvider(p.ID): ProviderIDs为空或包含p.ID |
| 排除已失败 | p.ID 不在 excludeIDs 中 |
| 额度紧急 | QuotaTotal > 0 && QuotaRatio() < QuotaCriticalThreshold(0.05) → 跳过 |

### 2. 优先级分组 (groupByPriority)

```
主力组(primary): priority >= 90
热备组(hot):     priority 50~89
冷备组(cold):    priority 1~49
```

### 3. 调度策略

按分组顺序(主力→热备→冷备)，每组内用策略函数选择:

| 策略名 | 函数 | 逻辑 |
|--------|------|------|
| smart (默认) | selectSmart | 额度充裕度>0.5优先 → 同级别比延迟 → 前3名加权随机 |
| least_latency | selectLeastLatency | LastLatencyMs最小的 |
| cost_first | selectCostFirst | CostMultiplier最小的 |
| round_robin | selectWeightedRandom | 按Weight加权随机 |
| priority | selectSmart | 同smart |

### selectSmart 详细
```
1. 按额度充裕度排序: QuotaRatio > 0.5 的排前面
2. 同级别比延迟: LastLatencyMs 小的排前面
3. 取前3名，按权重加权随机选择(避免全走一个)
4. 只有1个 → 直接返回
```

### selectWeightedRandom
```
1. 按每个Provider的Weight构建累积权重数组
2. rand.Intn(总权重) → 落在哪个区间就选哪个
3. Weight=0的Provider不参与
```

## 热插拔

```
RefreshProviders(providers)  → 加锁替换整个列表
GetProviders()               → 加锁复制快照返回
```
