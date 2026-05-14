# 07 - 智能重试引擎 (retry.go)

## 错误类型体系

### UpstreamErrorType
```
""                  = 无错误
quota_exhausted     = 额度用完
rate_limited        = 频率/时间限制
timeout             = 网络超时/中断
truncated           = 响应被截断
unknown             = 未分类错误
```

### UpstreamErrorDetail
```
type UpstreamErrorDetail struct {
    Type    UpstreamErrorType
    Message string  // 原始错误消息
    Code    string  // 错误代码
}
```

## 三层防御错误识别 (DetectUpstreamError)

```
输入: statusCode int, body []byte
输出: UpstreamErrorDetail
```

### 第1层: HTTP状态码 (body为空时)
| 状态码 | 映射 |
|--------|------|
| 429 | rate_limited |
| 402 | quota_exhausted |
| 403 | quota_exhausted |
| 503 | timeout |

### 第2层: JSON结构通用提取 + 模糊语义匹配
1. extractErrorMessage(body) → msg, code, errType
   - 支持: error.message/error.code/error.type (嵌套)
   - 支持: 顶层 message/code/type (DashScope等)
   - 支持: {"error": "string"} (简化格式)
   - 支持: 非JSON纯文本
2. combined = toLower(msg + code + errType)
3. 匹配优先级: 额度 > 频率 > 超时

### 第3层: 状态码兜底 (body匹配不到时)
- 429 → rate_limited
- 402 → quota_exhausted  
- 403 → quota_exhausted
- 502/503 → timeout

### 关键词匹配表

**quotaExhaustedPatterns** (24项):
```
insufficient_quota, exceeded quota, billing hard limit,
quota exceeded, out of credits, credits exhausted,
account deactivated, account has been suspended,
balance is insufficient, insufficient funds,
plan_limit_exceeded, capacity exceeded,
spender limit, monthly spending limit,
spending limit reached, usage limit reached,
account_limit_exceeded, quota_exceeded,
AccountQuotaExceeded (DashScope), DataInsufficient (DashScope),
InvalidApiKey (DashScope), Forbidden (DashScope),
额度, 余额不足, 已用完
```

**rateLimitPatterns** (17项):
```
rate_limit, rate limit, too many requests,
requests per, rpm limit, tpm limit,
tokens per minute, concurrent requests,
5-hour, 5 hour, daily limit, hour limit, minute limit,
Throttling (DashScope), throttling,
Request was throttled (DashScope), rate_limit_exceeded,
频率限制, 请求过多, 并发
```

**timeoutPatterns** (11项):
```
timeout, timed out, deadline exceeded,
connection reset, connection refused,
network error, no route to host,
i/o timeout, context deadline,
超时, 网络
```

## ShouldFailover 决策

```
输入: result *ProxyResult, tokenID, model, body
输出: (shouldFailover bool, reason string)
```

### 决策链
| 优先级 | 条件 | reason |
|--------|------|--------|
| 1 | result.Error != "" | network_timeout / network_rate_limit / network_error |
| 2 | StatusCode >= 500 | upstream_5xx |
| 3 | StatusCode == 429 | rate_limited_429 |
| 4 | StatusCode >= 400 | upstream_4xx |
| 5 | UpstreamError非空 | quota_exhausted / rate_limited_semantic / timeout_semantic / error_semantic |
| 6 | Truncated=true | response_truncated |
| 7 | 同一Hash之前失败 | same_request_previously_failed |
| 8 | 以上都不满足 | 不failover, reason="" |

## ShouldRetrySameProvider 决策

| 错误类型 | 同Provider重试? | 理由 |
|----------|----------------|------|
| quota_exhausted | ✗ | 重试无意义 |
| rate_limited (长时限流>60s) | ✗ | 等太久 |
| rate_limited (短时限流<60s) | ✓ | 短暂等后可恢复 |
| timeout | ✓ | 可能是偶发 |
| truncated | ✓ | 增大max_tokens重试 |
| unknown/空 | ✗ | 直接换Provider |

## 冷却机制 (handlers.go中调用)

| 场景 | 冷却时间 |
|------|---------|
| 额度用完 | 30分钟 |
| 长时限流(>60s) | ExtractRetryAfter值, 上限2小时 |
| 短时限流/超时 | 不冷却, 短暂等待重试 |

## ExtractRetryAfter 时间提取

支持格式:
1. **精确时间**: "reset at 2026-05-14 12:35:20 +0800 CST" → parseResetAtTime
2. **available in**: "available in 18000 seconds" / "5 minutes" / "2 hours"
3. **retry after**: "retry after 300s" / "10 minutes"
4. **N-hour**: "5-hour limit" / "5 hour"
5. **N-minute**: "10 minute limit"

### parseResetAtTime
- 正则提取 YYYY-MM-DD HH:MM:SS
- 多种格式尝试解析
- 兜底: ParseInLocation(东8区)
- 计算 time.Until(resetTime) → 秒数

## 请求Hash缓存 (RequestCache)

### 结构
```
type requestCacheEntry struct {
    Hash      string    // SHA256前16字符
    Timestamp time.Time
    Success   bool
}

type RequestCache struct {
    entries map[string]*requestCacheEntry  // key: "tokenID:model"
}
```

### 方法
- RecordRequestSuccess → 覆盖Hash, Success=true
- RecordRequestFailure → 覆盖Hash, Success=false
- IsSameRequestFailed → 查询当前Hash是否==上次失败的Hash
- GetEntry → 获取缓存条目
- CleanStale → 清理>30分钟的条目

### 用途
1. 标记token+model的最近请求，判断是否同一请求反复失败
2. 防止同一请求无限重试
3. 统计和日志用途

## IncreaseMaxTokens 截断重试

| 原始max_tokens | 增大后 | 上限 |
|----------------|--------|------|
| 无字段 | 16384 | - |
| 1024 | 2048 | 32768 |
| 20000 | 32768(截断) | 32768 |
| 16384 | 32768 | 32768 |

策略: 翻倍，上限32768
