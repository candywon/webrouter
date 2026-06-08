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
}

// Forward 转发请求到上游 Provider
func (ps *ProxyService) Forward(provider *Provider, endpoint string,
	req *http.Request, body []byte, model string) (*http.Response, *ProxyResult) {

	start := time.Now()
	isStream := isStreamRequest(body)

	// 构造上游 URL — 智能处理 /v1 重复问题
	// 如果 BaseURL 已包含 /v1（如 DashScope compatible-mode/v1），则 endpoint 去掉 /v1 前缀
	upstreamURL := provider.BaseURL + endpoint
	if strings.HasSuffix(provider.BaseURL, "/v1") && strings.HasPrefix(endpoint, "/v1/") {
		upstreamURL = provider.BaseURL + endpoint[3:] // 去掉 endpoint 的 /v1 前缀
	}

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
			timeout = cfg.DefaultTimeout
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

// ForwardBinary 转发二进制/多媒体请求（跳过 JSON 解析和流式检测）
// 用于 audio/image 端点，body 可以是 JSON 或 multipart
func (ps *ProxyService) ForwardBinary(provider *Provider, endpoint string,
	req *http.Request, body []byte, model string, contentType string, isMultipart bool) (*http.Response, *ProxyResult) {

	start := time.Now()

	// 构造上游 URL
	upstreamURL := provider.BaseURL + endpoint
	if strings.HasSuffix(provider.BaseURL, "/v1") && strings.HasPrefix(endpoint, "/v1/") {
		upstreamURL = provider.BaseURL + endpoint[3:]
	}

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
	if timeout <= 0 {
		timeout = cfg.DefaultTimeout
	}
	// 二进制请求可能需要更长超时（大文件上传）
	if timeout < cfg.DefaultTimeout {
		timeout = cfg.DefaultTimeout
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
	var streamContentBuf strings.Builder // 累积流式内容（用于知识捕获，上限 10KB）
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

				// 累积流式内容（用于知识捕获）
				if streamContentBuf.Len() < maxStreamCapture {
					if delta := extractStreamDeltaContent(data); delta != "" {
						streamContentBuf.WriteString(delta)
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

	// 保存流式内容（用于知识捕获）
	result.StreamContent = streamContentBuf.String()

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
func extractStreamDeltaContent(data string) string {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return ""
	}
	if len(chunk.Choices) > 0 {
		return chunk.Choices[0].Delta.Content
	}
	return ""
}
