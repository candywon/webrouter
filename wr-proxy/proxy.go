// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// 代理转发核心：流式 SSE 透传 + 非流式转发

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProxyService 代理服务
// buildUpstreamURL 智能拼接上游 URL，去除 BaseURL 和 endpoint 之间的路径重复
//
// 规则：如果 BaseURL 路径以 /vN 结尾（含版本前缀），则去掉 endpoint 中的版本前缀。
//
//   base=.../compatible-mode/v1  + /v1/chat/completions → .../v1/chat/completions
//   base=.../api/v3              + /v1/chat/completions → .../api/v3/chat/completions
//   base=.../api/coding/v3       + /v1/chat/completions → .../api/coding/v3/chat/completions
//   base=.../v1                  + /v1/chat/completions → .../v1/chat/completions
//   base=.../api.openai.com      + /v1/chat/completions → .../api.openai.com/v1/chat/completions
func buildUpstreamURL(baseURL, endpoint string) string {
	base := strings.TrimRight(baseURL, "/")
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		suffix := base[idx+1:]
		// 检查 BaseURL 是否以 /vN 结尾
		if isVersionPrefix(suffix) && isVersionPrefix(endpoint[1:strings.Index(endpoint[1:], "/")+1]) {
			// 去掉 endpoint 的 /vN 部分，只保留后面的路径
			slashIdx := strings.Index(endpoint[1:], "/")
			if slashIdx >= 0 {
				return base + endpoint[1+slashIdx:]
			}
		}
	}
	return base + endpoint
}

// isVersionPrefix 判断字符串是否为版本前缀（如 "v1", "v3", "v2024"）
func isVersionPrefix(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type ProxyService struct {
	client *http.Client
}

var proxySvc *ProxyService

// NewProxyService 创建代理服务
func NewProxyService() *ProxyService {
	return &ProxyService{
		client: &http.Client{
			Timeout: cfg.DefaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.MaxIdleConns,
				IdleConnTimeout:     cfg.IdleConnTimeout,
				MaxIdleConnsPerHost: 10,
				DisableKeepAlives:   false,
			},
		},
	}
}

// ProxyResult 代理结果
type ProxyResult struct {
	StatusCode    int
	InputTokens   int64
	OutputTokens  int64
	CachedTokens  int64
	IsStream      bool
	LatencyMs     int
	Error         string
	UpstreamError UpstreamErrorDetail // 上游语义错误详情（HTTP 200 但 body 含错误时填充）
	StreamAborted bool                // 流式响应中途被错误中断
	Truncated     bool                // 响应被截断（finish_reason=length 或 JSON 不完整）
	StreamContent string              // 流式响应累积内容（用于知识捕获）
	Passthrough   bool                // 响应已是客户端期望的最终格式，跳过解析直接转发
}

// Forward 转发请求到上游 Provider
func (ps *ProxyService) Forward(provider *Provider, endpoint string,
	req *http.Request, body []byte, model string) (*http.Response, *ProxyResult) {

	start := time.Now()
	isStream := isStreamRequest(body)

	// 构造上游 URL — 智能处理 /v1 重复问题
	// 如果 BaseURL 已包含 /v1（如 DashScope compatible-mode/v1），则 endpoint 去掉 /v1 前缀
	upstreamURL := buildUpstreamURL(provider.BaseURL, endpoint)

	// 构造上游请求
	upstreamReq, err := http.NewRequest(req.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, &ProxyResult{
			StatusCode: 502,
			Error:      fmt.Sprintf("build upstream request: %v", err),
			LatencyMs:  int(time.Since(start).Milliseconds()),
		}
	}

	// 复制请求头（排除 Host 和 Authorization）
	for k, vv := range req.Header {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Authorization") {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}

	// 替换为 Provider 的 API Key
	if provider.APIKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	// 设置超时：流式请求需要更长的超时（reasoning 模型生成慢）
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		if isStream {
			timeout = cfg.StreamTimeout
		} else {
			timeout = GetDefaultTimeout()
		}
	}
	// 流式请求至少使用 StreamTimeout
	if isStream && timeout < cfg.StreamTimeout {
		timeout = cfg.StreamTimeout
	}
	ps.client.Timeout = timeout

	// 发送请求
	resp, err := ps.client.Do(upstreamReq)
	if err != nil {
		return nil, &ProxyResult{
			StatusCode: 502,
			Error:      fmt.Sprintf("upstream request failed: %v", err),
			LatencyMs:  int(time.Since(start).Milliseconds()),
		}
	}

	result := &ProxyResult{
		StatusCode: resp.StatusCode,
		IsStream:   isStream,
		LatencyMs:  int(time.Since(start).Milliseconds()),
	}

	// 记录 Provider 连续失败/成功次数
	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		provider.ConsecFails++
	} else if resp.StatusCode < 400 {
		provider.ConsecFails = 0
	}

	// 对错误响应（>= 400）进行语义分析，识别额度用完/限流/超时等
	if resp.StatusCode >= 400 {
		rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096)) // 只读前4KB分析错误
		resp.Body.Close()
		// 若上游声明了 Content-Encoding，先解压以便后续语义分析与展示
		body := decodeResponseBody(rawBody, resp.Header.Get("Content-Encoding"))
		if readErr == nil && len(body) > 0 {
			result.UpstreamError = DetectUpstreamError(resp.StatusCode, body)
			if result.UpstreamError.Type != UpstreamErrNone {
				LogInfo("Forward: %s → %s semantic error: type=%s code=%s msg=%s",
					model, provider.Name, result.UpstreamError.Type,
					result.UpstreamError.Code, result.UpstreamError.Message)
				// 将错误信息写入 result.Error，确保上层 failover 逻辑能触发
				if result.Error == "" {
					result.Error = fmt.Sprintf("upstream semantic error: %s - %s",
						result.UpstreamError.Type, result.UpstreamError.Message)
				}
			}
			// 兜底：即使语义识别失败，也保留 body 摘要供错误展示
			if result.Error == "" {
				snippet := strings.TrimSpace(string(body))
				if len(snippet) > 500 {
					snippet = snippet[:500] + "…"
				}
				result.Error = fmt.Sprintf("upstream HTTP %d: %s", resp.StatusCode, snippet)
			}
		}
		// 重新构造 body reader 供后续使用（保留原始压缩字节，避免破坏 Content-Encoding 语义）
		// 注意：body 已被读取并关闭，上层不应再读取 resp.Body
		// 对于错误响应，上层（handlers.go）不会尝试写入响应体给客户端
		resp.Body = io.NopCloser(bytes.NewReader(rawBody))
		return resp, result
	}

	// 对 200 响应也做轻量检测（某些 API 200 但返回错误 JSON）
	// 仅对非流式做此检测，流式在 StreamResponse 中逐 chunk 检测
	if !isStream && resp.StatusCode == 200 {
		// 先读取完整响应体，避免截断
		fullBody, readErr := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxBodySize))
		if readErr == nil && len(fullBody) > 0 {
			errDetail := DetectUpstreamError(200, fullBody)
			if errDetail.Type != UpstreamErrNone {
				// 200 但含语义错误，这是最坑的情况
				result.UpstreamError = errDetail
				result.Error = fmt.Sprintf("upstream returned 200 but contains error: %s - %s",
					errDetail.Type, errDetail.Message)
				LogWarn("Forward: %s → %s 200-OK but semantic error: type=%s code=%s",
					model, provider.Name, errDetail.Type, errDetail.Code)
				// 还原 body 供上层使用
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(fullBody))
				return resp, result
			}
		}
		// 还原完整 body
		if len(fullBody) > 0 {
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(fullBody))
		}
	}

	return resp, result
}

// ForwardSmart 根据 provider 协议能力选择合适的转发路径
// - provider 支持 anthropic 协议（ApiFormat="anthropic" 或配了 AnthropicBaseURL）：
//     · Anthropic 客户端 → 直通 ForwardAnthropic
//     · OpenAI 客户端 → 翻译 OpenAI→Anthropic 后 ForwardAnthropic
// - 仅 OpenAI 协议：
//     · OpenAI 客户端 → 直通 Forward
//     · Anthropic 客户端 → 翻译 Anthropic→OpenAI 后 Forward
//
// 决策准则：当 provider 同时支持两种协议且客户端是 Anthropic，优先走 Anthropic 路径以保留原生 thinking blocks / tool_use。
func (ps *ProxyService) ForwardSmart(provider *Provider, endpoint string,
	req *http.Request, body []byte, model string) (*http.Response, *ProxyResult) {

	nativeAnth := req.Header.Get("X-WR-Anthropic-Native") == "1"

	// 判断本次请求是否应走 Anthropic 上游路径
	// 1. provider 仅支持 anthropic（无 OpenAI 端点）→ 必须走
	// 2. provider 支持双协议 + 客户端是 Anthropic → 优先走（保留 thinking blocks）
	// 3. 其他情况 → 走 OpenAI 路径
	useAnthropicUpstream := false
	if provider.ApiFormat == "anthropic" {
		useAnthropicUpstream = true
	} else if provider.AnthropicBaseURL != "" && nativeAnth {
		useAnthropicUpstream = true
	}

	if !useAnthropicUpstream {
		if nativeAnth {
			// Anthropic 客户端 → OpenAI 上游：先翻译为 OpenAI 格式
			oaBody, err := TranslateAnthropicToOpenAI(body)
			if err != nil {
				return nil, &ProxyResult{
					StatusCode: 502,
					Error:      fmt.Sprintf("translate anthropic→openai req: %v", err),
				}
			}
			// 端点也需重映射：/v1/messages → /v1/chat/completions
			openAIEndpoint := endpoint
			if strings.Contains(endpoint, "/messages") {
				openAIEndpoint = "/v1/chat/completions"
			}
			return ps.Forward(provider, openAIEndpoint, req, oaBody, model)
		}
		return ps.Forward(provider, endpoint, req, body, model)
	}

	// 仅对 chat.completions/messages 类的请求做翻译；其他端点回退到普通 Forward
	if !strings.Contains(endpoint, "chat/completions") && !strings.Contains(endpoint, "/messages") {
		return ps.Forward(provider, endpoint, req, body, model)
	}

	// Anthropic 上游
	var anthBody []byte
	if nativeAnth {
		// 客户端已经是 Anthropic 格式 → 直通
		anthBody = body
	} else {
		// OpenAI 客户端 → 翻译为 Anthropic
		var err error
		anthBody, err = TranslateOpenAIToAnthropicRequest(body)
		if err != nil {
			return nil, &ProxyResult{
				StatusCode: 502,
				Error:      fmt.Sprintf("translate to anthropic: %v", err),
			}
		}
	}

	resp, result := ps.ForwardAnthropic(provider, req, anthBody, model)
	if resp == nil || result.StatusCode >= 400 {
		return resp, result
	}

	// 原生 Anthropic 直通：响应不做任何转换
	if nativeAnth {
		result.Passthrough = true
		return resp, result
	}

	// 2. 上游成功 → 把 Anthropic 响应译为 OpenAI 格式
	if result.IsStream {
		// 用 io.Pipe 把 Anthropic SSE 实时转换为 OpenAI SSE
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			defer resp.Body.Close()
			if err := AnthropicSSEToOpenAISSE(pw, resp.Body, model); err != nil {
				LogWarn("ForwardSmart: anthropic→openai SSE convert: %v", err)
			}
		}()
		// 用新的 Body 替换原 resp.Body；状态码、header 不变
		newResp := *resp
		newResp.Body = pr
		// Content-Length 已不再准确
		newResp.Header = resp.Header.Clone()
		newResp.Header.Del("Content-Length")
		newResp.Header.Set("Content-Type", "text/event-stream")
		return &newResp, result
	}

	// 非流式：读完整 body，转换后替换
	rawBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		result.Error = fmt.Sprintf("read anthropic body: %v", readErr)
		result.StatusCode = 502
		return resp, result
	}
	// 上游可能返回 gzip/deflate 压缩响应，先解压再翻译
	decoded := decodeResponseBody(rawBody, resp.Header.Get("Content-Encoding"))
	converted, err := TranslateAnthropicToOpenAIResponse(decoded)
	if err != nil {
		LogWarn("ForwardSmart: translate anthropic→openai response: %v (passing raw)", err)
		converted = decoded
	}
	// 解析 usage 写入 result
	result.InputTokens, result.OutputTokens = parseAnthropicUsageFromBody(decoded)
	resp.Body = io.NopCloser(bytes.NewReader(converted))
	resp.Header = resp.Header.Clone()
	resp.Header.Del("Content-Length")
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Type", "application/json")
	return resp, result
}

// parseAnthropicUsageFromBody 提取 Anthropic 响应中的 usage
func parseAnthropicUsageFromBody(body []byte) (int64, int64) {
	var r struct {
		Usage *struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Usage == nil {
		return 0, 0
	}
	return r.Usage.InputTokens, r.Usage.OutputTokens
}

// ForwardAnthropic 转发请求到 Anthropic 协议的上游 Provider
// 使用 x-api-key + anthropic-version header；endpoint 强制走 /v1/messages
func (ps *ProxyService) ForwardAnthropic(provider *Provider,
	req *http.Request, body []byte, model string) (*http.Response, *ProxyResult) {

	start := time.Now()
	isStream := isStreamRequest(body)

	// 优先使用独立配置的 Anthropic 端点（双 URL 场景），否则回退到主 BaseURL
	anthBase := provider.AnthropicBaseURL
	if anthBase == "" {
		anthBase = provider.BaseURL
	}
	upstreamURL := buildUpstreamURL(anthBase, "/v1/messages")

	upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, &ProxyResult{
			StatusCode: 502,
			Error:      fmt.Sprintf("build upstream request: %v", err),
			LatencyMs:  int(time.Since(start).Milliseconds()),
		}
	}

	// 复制必要请求头（去掉 Authorization / x-api-key / Host），保留 Accept/User-Agent 等
	for k, vv := range req.Header {
		if strings.EqualFold(k, "Host") ||
			strings.EqualFold(k, "Authorization") ||
			strings.EqualFold(k, "x-api-key") ||
			strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	// 强制不压缩，便于本端做 SSE/JSON 解析与翻译
	upstreamReq.Header.Set("Accept-Encoding", "identity")
	if provider.APIKey != "" {
		upstreamReq.Header.Set("x-api-key", provider.APIKey)
	}

	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		if isStream {
			timeout = cfg.StreamTimeout
		} else {
			timeout = GetDefaultTimeout()
		}
	}
	if isStream && timeout < cfg.StreamTimeout {
		timeout = cfg.StreamTimeout
	}
	ps.client.Timeout = timeout

	resp, err := ps.client.Do(upstreamReq)
	if err != nil {
		return nil, &ProxyResult{
			StatusCode: 502,
			Error:      fmt.Sprintf("upstream request failed: %v", err),
			LatencyMs:  int(time.Since(start).Milliseconds()),
		}
	}

	result := &ProxyResult{
		StatusCode: resp.StatusCode,
		IsStream:   isStream,
		LatencyMs:  int(time.Since(start).Milliseconds()),
	}

	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		provider.ConsecFails++
	} else if resp.StatusCode < 400 {
		provider.ConsecFails = 0
	}

	if resp.StatusCode >= 400 {
		rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		body := decodeResponseBody(rawBody, resp.Header.Get("Content-Encoding"))
		if readErr == nil && len(body) > 0 {
			result.UpstreamError = DetectUpstreamError(resp.StatusCode, body)
			if result.UpstreamError.Type != UpstreamErrNone && result.Error == "" {
				result.Error = fmt.Sprintf("upstream semantic error: %s - %s",
					result.UpstreamError.Type, result.UpstreamError.Message)
			}
			if result.Error == "" {
				snippet := strings.TrimSpace(string(body))
				if len(snippet) > 500 {
					snippet = snippet[:500] + "…"
				}
				result.Error = fmt.Sprintf("upstream HTTP %d: %s", resp.StatusCode, snippet)
			}
		}
		resp.Body = io.NopCloser(bytes.NewReader(rawBody))
		return resp, result
	}

	return resp, result
}


// 用于 audio/image 端点，body 可以是 JSON 或 multipart
func (ps *ProxyService) ForwardBinary(provider *Provider, endpoint string,
	req *http.Request, body []byte, model string, contentType string, isMultipart bool) (*http.Response, *ProxyResult) {

	start := time.Now()

	// 构造上游 URL
	upstreamURL := buildUpstreamURL(provider.BaseURL, endpoint)

	upstreamReq, err := http.NewRequest(req.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, &ProxyResult{
			StatusCode: 502,
			Error:      fmt.Sprintf("build upstream request: %v", err),
			LatencyMs:  int(time.Since(start).Milliseconds()),
		}
	}

	// 复制原始请求头
	for k, vv := range req.Header {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Authorization") ||
			strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}

	// 设置正确的 Content-Type（multipart 需要含 boundary）
	upstreamReq.Header.Set("Content-Type", contentType)

	// 替换为 Provider 的 API Key
	if provider.APIKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	// 设置超时
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	defaultTimeout := GetDefaultTimeout()
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	// 二进制请求可能需要更长超时（大文件上传）
	if timeout < defaultTimeout {
		timeout = defaultTimeout
	}
	ps.client.Timeout = timeout

	resp, err := ps.client.Do(upstreamReq)
	if err != nil {
		return nil, &ProxyResult{
			StatusCode: 502,
			Error:      fmt.Sprintf("upstream request failed: %v", err),
			LatencyMs:  int(time.Since(start).Milliseconds()),
		}
	}

	result := &ProxyResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  int(time.Since(start).Milliseconds()),
	}

	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		provider.ConsecFails++
	} else if resp.StatusCode < 400 {
		provider.ConsecFails = 0
	}

	// 错误响应语义分析
	if resp.StatusCode >= 400 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if readErr == nil && len(body) > 0 {
			result.UpstreamError = DetectUpstreamError(resp.StatusCode, body)
			if result.UpstreamError.Type != UpstreamErrNone {
				LogInfo("ForwardBinary: %s → %s semantic error: type=%s code=%s msg=%s",
					model, provider.Name, result.UpstreamError.Type,
					result.UpstreamError.Code, result.UpstreamError.Message)
				if result.Error == "" {
					result.Error = fmt.Sprintf("upstream semantic error: %s - %s",
						result.UpstreamError.Type, result.UpstreamError.Message)
				}
			}
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, result
	}

	// 200 响应：仅对 JSON Content-Type 做语义错误检测
	// 二进制响应（audio/image）不能预读，否则会截断 body
	if resp.StatusCode == 200 && !isMultipart && !isBinaryContentType(resp.Header.Get("Content-Type")) {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr == nil && len(body) > 0 {
			errDetail := DetectUpstreamError(200, body)
			if errDetail.Type != UpstreamErrNone {
				result.UpstreamError = errDetail
				result.Error = fmt.Sprintf("upstream returned 200 but contains error: %s - %s",
					errDetail.Type, errDetail.Message)
				LogWarn("ForwardBinary: %s → %s 200-OK but semantic error: type=%s",
					model, provider.Name, errDetail.Type)
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(body))
				return resp, result
			}
		}
		// 还原 body（仅 JSON 响应）
		if len(body) > 0 {
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	return resp, result
}

// StreamResponse 流式 SSE 响应写入器（包级函数）
// 返回 result.StreamAborted=true 表示流中途被错误中断（需触发 failover）
func StreamResponse(w http.ResponseWriter, resp *http.Response,
	reqID string, provider *Provider, token *Token,
	model, endpoint, clientIP string, desensitizeMapping *ReplacementMap) *ProxyResult {

	start := time.Now()
	result := &ProxyResult{
		StatusCode: resp.StatusCode,
		IsStream:   true,
	}

	// 设置 SSE 响应头
	h := w.Header()
	for k, vv := range resp.Header {
		switch strings.ToLower(k) {
		case "content-type", "cache-control", "connection",
			"access-control-allow-origin", "access-control-allow-headers":
			for _, v := range vv {
				h.Set(k, v)
			}
		}
	}

	flusher, canFlush := w.(http.Flusher)

	// 逐行透传
	reader := bufio.NewReader(resp.Body)
	defer resp.Body.Close()

	var lastChunk strings.Builder
	var lastFinishReason string          // 跟踪最后一个 finish_reason
	var streamContentBuf strings.Builder       // 累积流式 content（最终答案，用于知识捕获，上限 10KB）
	var streamReasoningBuf strings.Builder   // 累积流式 reasoning_content（思考过程）
	const maxStreamCapture = 10 * 1024
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			result.Error = fmt.Sprintf("stream read: %v", err)
			// 网络中断/超时，标记为 StreamAborted
			result.StreamAborted = true
			result.UpstreamError = UpstreamErrorDetail{
				Type:    UpstreamErrTimeout,
				Message: result.Error,
			}
			break
		}

		// 写入客户端（还原脱敏标记）
		outLine := line
		if desensitizeMapping != nil {
			outLine = []byte(desensitizeMapping.Restore(string(line)))
		}
		w.Write(outLine)
		if canFlush {
			flusher.Flush()
		}

		// 缓存最后一个 data chunk 以提取 usage
		lineStr := strings.TrimSpace(string(line))
		if strings.HasPrefix(lineStr, "data: ") {
			data := strings.TrimPrefix(lineStr, "data: ")
			if data != "[DONE]" {
				lastChunk.Reset()
				lastChunk.WriteString(data)
					// 累积流式内容（用于知识捕获，分 content/reasoning 双缓冲）
					if streamContentBuf.Len() < maxStreamCapture {
						c, r := extractStreamDeltaContent(data)
						if c != "" {
							streamContentBuf.WriteString(c)
						}
						if r != "" {
							streamReasoningBuf.WriteString(r)
						}
					}

					// 提取 finish_reason（流式）
					if fr := extractFinishReason(data); fr != "" {
						lastFinishReason = fr
					}

					// 检测流中的语义错误（如额度用完、限流等）
					errDetail := DetectStreamError(data)
					if errDetail.Type != UpstreamErrNone {
						LogWarn("StreamResponse: detected error in stream: type=%s code=%s msg=%s",
							errDetail.Type, errDetail.Code, errDetail.Message)
						result.UpstreamError = errDetail
						result.StreamAborted = true
						if result.Error == "" {
							result.Error = fmt.Sprintf("stream error: %s - %s",
								errDetail.Type, errDetail.Message)
						}
						// 继续读取剩余数据（不中断流），但标记为 aborted
						// 上层 handleProxy 会根据 StreamAborted 决定是否 failover
					}
				}
			}
		}

	result.LatencyMs = int(time.Since(start).Milliseconds())

	// 保存流式内容（优先 content/最终答案，无 content 时回退 reasoning_content/思考过程）
	if streamContentBuf.Len() > 0 {
		result.StreamContent = streamContentBuf.String()
	} else if streamReasoningBuf.Len() > 0 {
		result.StreamContent = streamReasoningBuf.String()
	}

	// 从最后一个 chunk 提取 usage
	usage := parseStreamUsage(lastChunk.String())
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens
	result.CachedTokens = usage.CachedTokens

	// 检测流式截断
	if lastFinishReason == "length" {
		result.Truncated = true
		LogInfo("StreamResponse: finish_reason=length detected for %s → %s", model, provider.Name)
	}
	// 没收到 [DONE] 也算截断（连接中断）—— 此时 lastChunk 为空或不含 [DONE]
	// 注意：StreamAborted 已经在循环内处理了 io.Err 的情况

	return result
}

// NonStreamResponse 非流式响应写入器（包级函数）
func NonStreamResponse(w http.ResponseWriter, resp *http.Response,
	reqID string, provider *Provider, token *Token,
	model, endpoint, clientIP string, desensitizeMapping *ReplacementMap) *ProxyResult {

	start := time.Now()
	result := &ProxyResult{
		StatusCode: resp.StatusCode,
		IsStream:   false,
	}

	// 读取上游响应（有上限）
	body, err := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxBodySize))
	resp.Body.Close()
	if err != nil {
		result.Error = fmt.Sprintf("read upstream body: %v", err)
		result.Truncated = true // 读取失败 = 截断
		result.LatencyMs = int(time.Since(start).Milliseconds())
		return result
	}

	// 提取 usage
	usage := parseNonStreamUsage(body)
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens
	result.CachedTokens = usage.CachedTokens

	// 检测 finish_reason=length（非流式）
	if resp.StatusCode == 200 {
		if checkFinishReasonLength(body) {
			result.Truncated = true
			LogInfo("NonStreamResponse: finish_reason=length detected for %s → %s", model, provider.Name)
		} else if !json.Valid(body) {
			// JSON 不完整（被截断）
			result.Truncated = true
			LogInfo("NonStreamResponse: invalid JSON (truncated) for %s → %s", model, provider.Name)
		}
	}

	// 还原脱敏标记
	if desensitizeMapping != nil {
		restored := desensitizeMapping.Restore(string(body))
		body = []byte(restored)
	}
	// 复制响应头
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	// 写入客户端
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	result.LatencyMs = int(time.Since(start).Milliseconds())
	return result
}

// checkFinishReasonLength 检测非流式响应中 finish_reason 是否为 "length"
func checkFinishReasonLength(body []byte) bool {
	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	for _, c := range resp.Choices {
		if c.FinishReason == "length" {
			return true
		}
	}
	return false
}

// extractFinishReason 从 SSE chunk 的 data 中提取 finish_reason
// SSE chunk 格式: {"choices":[{"delta":{},"finish_reason":"stop"}]}
func extractFinishReason(data string) string {
	var chunk struct {
		Choices []struct {
			FinishReason *string `json:"finish_reason"` // nullable
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return ""
	}
	for _, c := range chunk.Choices {
		if c.FinishReason != nil && *c.FinishReason != "" {
			return *c.FinishReason
		}
	}
	return ""
}

// --- Usage 解析 ---

type UsageInfo struct {
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
}

func parseNonStreamUsage(body []byte) UsageInfo {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(body, &resp); err != nil {
		return UsageInfo{}
	}
	usageRaw, ok := resp["usage"]
	if !ok {
		return UsageInfo{}
	}
	var usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
		CachedTokens     int64 `json:"cached_tokens"`
	}
	if err := json.Unmarshal(usageRaw, &usage); err != nil {
		return UsageInfo{}
	}
	return UsageInfo{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		CachedTokens: usage.CachedTokens,
	}
}

func parseStreamUsage(data string) UsageInfo {
	if data == "" {
		return UsageInfo{}
	}
	var chunk map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return UsageInfo{}
	}
	usageRaw, ok := chunk["usage"]
	if !ok {
		return UsageInfo{}
	}
	var usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		CachedTokens     int64 `json:"cached_tokens"`
	}
	if err := json.Unmarshal(usageRaw, &usage); err != nil {
		return UsageInfo{}
	}
	return UsageInfo{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		CachedTokens: usage.CachedTokens,
	}
}

func isStreamRequest(body []byte) bool {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	stream, ok := req["stream"].(bool)
	return ok && stream
}

// decodeResponseBody 根据 Content-Encoding 解压响应体（仅 gzip/deflate）。
// 解压失败时返回原始字节，保证错误展示链路不会因解压异常而中断。
func decodeResponseBody(body []byte, encoding string) []byte {
	if len(body) == 0 {
		return body
	}

	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return body
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, 64*1024))
		if err != nil {
			return body
		}
		return out
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return body
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, 64*1024))
		if err != nil {
			return body
		}
		return out
	}
	return body
}

// extractStreamDeltaContent 从 SSE data chunk 中提取 delta content
func extractStreamDeltaContent(data string) (string, string) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", ""
	}
	if len(chunk.Choices) > 0 {
		d := chunk.Choices[0].Delta
		return d.Content, d.ReasoningContent
	}
	return "", ""
}
