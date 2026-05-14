# 06 - 代理转发 (proxy.go)

## ProxyService 结构
```
type ProxyService struct {
    client *http.Client
}
```
HTTP客户端连接池:
- Timeout: cfg.DefaultTimeout (60s)
- MaxIdleConns: 100
- IdleConnTimeout: 90s
- MaxIdleConnsPerHost: 10

## ProxyResult 结构
```
type ProxyResult struct {
    StatusCode    int
    InputTokens   int64
    OutputTokens  int64
    IsStream      bool
    LatencyMs     int
    Error         string
    UpstreamError UpstreamErrorDetail  // 语义错误详情
    StreamAborted bool                 // 流式中途被错误中断
    Truncated     bool                 // 响应被截断
}
```

## Forward 函数

```
输入: provider, endpoint, req, body, model
输出: (*http.Response, *ProxyResult)
```

### 流程

```
1. 构造上游URL
   upstreamURL = BaseURL + endpoint
   智能去重: BaseURL以/v1结尾 + endpoint以/v1/开头 → 去掉endpoint的/v1
   
2. 构造上游请求
   复制Header(排除Host/Authorization)
   替换Authorization为Provider的APIKey
   
3. 设置超时
   provider.TimeoutSeconds > 0 → 用Provider的
   否则 → cfg.DefaultTimeout
   
4. 发送请求
   失败 → 502 + "upstream request failed"

5. 响应处理

   A. HTTP >= 400:
      - ReadAll(LimitReader 4KB)
      - DetectUpstreamError(statusCode, body) → 语义分析
      - 有语义错误 → 写result.Error + 日志
      - 还原body供上层使用
      - 更新Provider.ConsecFails
      
   B. HTTP 200 + 非流式:
      - ReadAll(LimitReader 4KB) 做轻量检测
      - DetectUpstreamError(200, body) → 检测200但含错误JSON
      - 有语义错误 → 标记(这是最坑的情况)
      - 还原body
      
   C. HTTP 200 + 流式 / 无错误:
      - 直接返回resp + result
      - 流式由StreamResponse处理
```

## StreamResponse 函数

```
输入: w, resp, reqID, provider, token, model, endpoint, clientIP
输出: *ProxyResult
```

### 流程

```
1. 设置SSE响应头
2. 逐行透传(reader.ReadBytes('\n'))
3. 每行:
   a. 写入客户端 + Flush
   b. data: 开头 → 缓存到lastChunk
   c. 提取finishReason → 跟踪lastFinishReason
   d. DetectStreamError(data) → 检测流中错误
      有错误 → StreamAborted=true, 记录UpstreamError, 不中断流继续读

4. 读取结束:
   - io.EOF → 正常结束
   - 其他错误 → StreamAborted=true, UpstreamError.Type=timeout

5. 后处理:
   - parseStreamUsage(lastChunk) → 提取InputTokens/OutputTokens
   - lastFinishReason=="length" → Truncated=true
```

## NonStreamResponse 函数

```
输入: 同StreamResponse
输出: *ProxyResult
```

### 流程

```
1. ReadAll(LimitReader, MaxBodySize)
   失败 → Truncated=true
2. parseNonStreamUsage(body) → 提取tokens
3. 截断检测(HTTP 200时):
   - checkFinishReasonLength(body) → Truncated=true
   - !json.Valid(body) → Truncated=true (不完整JSON)
4. 复制响应头 + 写入客户端
```

## 辅助函数

### checkFinishReasonLength(body) → bool
解析 choices[0].finish_reason == "length"

### extractFinishReason(data string) → string
从SSE chunk提取 choices[0].finish_reason (nullable)

### parseNonStreamUsage(body) → UsageInfo
解析 resp.usage.prompt_tokens / completion_tokens

### parseStreamUsage(data) → UsageInfo
解析最后一个SSE chunk的 usage.prompt_tokens / completion_tokens

### isStreamRequest(body) → bool
解析 req.stream == true
