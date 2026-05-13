package main

// 代理转发核心：流式 SSE 透传 + 非流式转发

import (
	"bufio"
	"bytes"
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
	StatusCode   int
	InputTokens  int64
	OutputTokens int64
	IsStream     bool
	LatencyMs    int
	Error        string
}

// Forward 转发请求到上游 Provider
func (ps *ProxyService) Forward(provider *Provider, endpoint string,
	req *http.Request, body []byte) (*http.Response, *ProxyResult) {

	start := time.Now()
	isStream := isStreamRequest(body)

	// 构造上游 URL
	upstreamURL := provider.BaseURL + endpoint

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

	// 设置超时
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = cfg.DefaultTimeout
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

	return resp, result
}

// StreamResponse 流式 SSE 响应写入器（包级函数）
func StreamResponse(w http.ResponseWriter, resp *http.Response,
	reqID string, provider *Provider, token *Token,
	model, endpoint, clientIP string) *ProxyResult {

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
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			result.Error = fmt.Sprintf("stream read: %v", err)
			break
		}

		// 写入客户端
		w.Write(line)
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
			}
		}
	}

	result.LatencyMs = int(time.Since(start).Milliseconds())

	// 从最后一个 chunk 提取 usage
	usage := parseStreamUsage(lastChunk.String())
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens

	return result
}

// NonStreamResponse 非流式响应写入器（包级函数）
func NonStreamResponse(w http.ResponseWriter, resp *http.Response,
	reqID string, provider *Provider, token *Token,
	model, endpoint, clientIP string) *ProxyResult {

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
		result.LatencyMs = int(time.Since(start).Milliseconds())
		return result
	}

	// 提取 usage
	usage := parseNonStreamUsage(body)
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens

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

// --- Usage 解析 ---

type UsageInfo struct {
	InputTokens  int64
	OutputTokens int64
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
	}
	if err := json.Unmarshal(usageRaw, &usage); err != nil {
		return UsageInfo{}
	}
	return UsageInfo{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
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
	}
	if err := json.Unmarshal(usageRaw, &usage); err != nil {
		return UsageInfo{}
	}
	return UsageInfo{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
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
