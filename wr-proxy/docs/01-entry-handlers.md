# 01 - 入口与主流程 (main.go + handlers.go)

## main.go 启动序列

```
1. LoadConfig()         → 环境变量加载配置
2. InitDB()             → SQLite 初始化 + 迁移
3. LoadProviders()      → 从DB加载Provider列表
4. RefreshPricing()     → 从DB加载定价表到内存
5. LoadChannels()       → 展开Channel为独立调度项
6. NewProxyService()    → 初始化HTTP客户端连接池
7. HealthChecker.Start()→ 启动后台健康检测(5min间隔)
8. 告警评估goroutine    → 每分钟 EvaluateAll()
9. 缓存清理goroutine    → 每10分钟 CleanStale()
10. HTTP服务启动         → :5051 + CORS中间件
11. 优雅关闭            → SIGINT/SIGTERM → 5秒超时
```

## handlers.go HTTP路由

### 路由注册 (RegisterHandlers)

| 路径 | 处理函数 | 说明 |
|------|---------|------|
| /v1/chat/completions | handleProxy | Chat补全(主入口) |
| /v1/completions | handleProxy | Text补全 |
| /v1/embeddings | handleProxy | 向量嵌入 |
| /v1/images/generations | handleProxy | 图片生成 |
| /v1/models | handleModels | 模型列表聚合 |
| /health | handleHealth | 健康检查 |
| /admin/reload | handleReload | 重载Provider |
| /admin/reload_pricing | handleReloadPricing | 重载定价 |
| /admin/stats | handleStats | 实时统计 |

### handleProxy 主流程（核心中的核心）

```
1. 鉴权
   authenticateRequest(r) → Token + AuthResult
   失败 → 401/429

2. 读取请求体
   io.ReadAll(LimitReader, MaxBodySize=10MB)

3. 提取model
   extractModel(body) → model字段
   缺失 → 400

4. 智能模型选择
   SmartModelSelect(model, body, token)
   → ResolvedModel, Downgraded
   auto/smart → 按复杂度选模型
   Downgraded=true → replaceModelInBody更新body

5. Token模型白名单
   token.CanUseModel(model)
   不允许 → 403

6. Failover循环 (attempt 0..MaxFailover=3)
   ┌──────────────────────────────────────────────┐
   │ a. 选Provider                                 │
   │    router.SelectProvider(model, token, excludeIDs) │
   │    无可用 → 503                               │
   │                                               │
   │ b. 请求清洗                                    │
   │    SanitizeRequest(provider, endpoint, body)   │
   │    不合法 → 400                                │
   │                                                │
   │ c. 转发                                        │
   │    proxySvc.Forward(provider, endpoint, r, body, model) │
   │                                                │
   │ d. 判断是否failover                            │
   │    ShouldFailover(result, tokenID, model, body) │
   │                                                │
   │ e. 如果需要failover:                           │
   │    - 截断 → 同Provider + IncreaseMaxTokens重试  │
   │    - 额度用完 → 冷却30min + exclude + continue  │
   │    - 长时限流 → 冷却到预计恢复 + exclude + continue │
   │    - 短时限流 → 同Provider重试(0.5s递增)        │
   │    - 超时 → 同Provider重试                      │
   │    - 其他 → exclude + continue                  │
   │                                                │
   │ f. 成功 → StreamResponse/NonStreamResponse      │
   │    流式中断 → 补录日志 + RecordRequestFailure    │
   └──────────────────────────────────────────────┘

7. 所有降级都失败 → 返回最后一个错误
```

### handleModels 模型列表

- 聚合所有可用Provider的模型
- Token白名单过滤
- 追加虚拟模型: "auto", "smart"

### handleStats 实时统计

每个Provider输出:
- provider_id, name, status, priority, last_latency
- quota_ratio, supports_tools
- minute_count, minute_valid_count, minute_tokens, minute_cost
- cooldown_remaining_sec (如有冷却)
