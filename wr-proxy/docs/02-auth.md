# 02 - 鉴权与限速 (auth.go)

## 数据结构

### RateLimiter
```
type RateLimiter struct {
    mu       sync.Mutex
    counters map[int]*rateCounter  // key: tokenID
}

type rateCounter struct {
    count       int        // 当前窗口请求数
    windowStart time.Time  // 窗口起始时间
}
```

### AuthResult
```
type AuthResult struct {
    Token      *Token
    Error      string
    StatusCode int
}
```

## 核心函数

### CheckRateLimit(tokenID int, rpm int) bool
- rpm <= 0 → 不限速，直接返回true
- 滑动窗口：1分钟内计数
- 超出rpm → 返回false
- 新窗口重置计数

### Authenticate(r *http.Request, token *Token) *AuthResult
校验链（按顺序）:
1. **Token启用** → token.Enabled == false → 401 "Token 已禁用"
2. **Token过期** → token.IsExpired() → 401 "Token 已过期"
3. **Token配额** → QuotaTotal > 0 && QuotaUsed >= QuotaTotal → 429 "Token 配额已用完"
4. **IP白名单** → SubnetWhitelist非空 → isIPAllowed → 不在白名单 → 403
5. **RPM限速** → CheckRateLimit → 超出 → 429 "RPM limit exceeded"

### extractClientIP(r *http.Request) string
优先级: X-Real-IP → X-Forwarded-For第一个 → r.RemoteAddr

### isIPAllowed(ip string, whitelistJSON string) bool
- 解析白名单JSON数组: ["10.0.0.0/8", "192.168.0.0/16"]
- 支持 CIDR 子网匹配 (net.Contains)
- 白名单为空 → 允许所有

## 全局变量

```
var limiter = &RateLimiter{counters: make(map[int]*rateCounter)}
```
