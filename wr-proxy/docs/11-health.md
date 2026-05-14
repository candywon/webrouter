# 11 - 健康检测 (health.go)

## HealthChecker 结构
```
type HealthChecker struct {
    interval time.Duration  // 检测间隔 (默认5分钟)
    timeout  time.Duration  // 单次超时 (默认15秒)
    stopCh   chan struct{}   // 停止信号
}
```

## 运行逻辑

```
Start() → go run()
1. 首次立即 checkAll()
2. ticker := NewTicker(interval)
3. 循环 select:
   case <-ticker.C: checkAll()
   case <-stopCh: return
```

## checkAll()
遍历所有Provider → checkProvider(p) → UpdateProviderStatus

## checkProvider(p) → (status, latencyMs, errMsg)

按Provider.Type分支:
| Type | 检测方式 |
|------|---------|
| direct | checkDirect → sendTestRequest |
| aggregate | checkAggregate → sendGET /models |
| litellm | checkLiteLLM → sendGET /health |
| custom/其他 | checkGeneric → sendTestRequest |

### sendTestRequest(p, endpoint, body)
- POST endpoint + body
- 用Provider的APIKey
- 超时: HealthTimeout (15s)
- 200 → "healthy", latencyMs
- 401 → "auth_failed"
- 429 → "rate_limited"
- 其他 → "dead" + errMsg

### sendGET(p, endpoint)
- GET endpoint
- 类似错误映射

### getVendorTestConfig(baseURL) → (endpoint, body)

根据BaseURL自动选择测试请求:
| URL包含 | endpoint | body |
|---------|----------|------|
| dashscope | /compatible-mode/v1/chat/completions | {"model":"qwen-turbo","messages":[{"role":"user","content":"hi"}],"max_tokens":1} |
| openai | /v1/chat/completions | {"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1} |
| anthropic/claude | /v1/messages | (Claude格式) |
| 默认 | /v1/chat/completions | 通用格式 |

### parseModelsList(modelsJSON) → []string
解析JSON数组字符串为Go切片
