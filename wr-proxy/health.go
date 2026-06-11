// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// 健康检测：后台定时探测 Provider 可用性

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HealthChecker 后台健康检测器
type HealthChecker struct {
	interval time.Duration
	timeout  time.Duration
	stopCh   chan struct{}
}

// NewHealthChecker 创建健康检测器
func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		interval: interval,
		timeout:  timeout,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动后台检测
func (hc *HealthChecker) Start() {
	go hc.run()
	LogInfo("HealthChecker: started, interval=%s", hc.interval)
}

// Stop 停止检测
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

func (hc *HealthChecker) run() {
	// 首次立即检测
	hc.checkAll()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkAll()
		case <-hc.stopCh:
			return
		}
	}
}

func (hc *HealthChecker) checkAll() {
	providers := router.GetProviders()
	if len(providers) == 0 {
		return
	}

	LogInfo("HealthChecker: checking %d providers...", len(providers))

	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		status, latency, errMsg := hc.checkProvider(p)

		// 更新 DB
		UpdateProviderStatus(p.ID, status, latency, errMsg)

		// 更新内存状态
		p.Status = status
		p.LastLatencyMs = latency
		p.LastError = errMsg

		// auth_failed → healthy 时重置退避状态
		if status == "healthy" && p.AuthFailCount > 0 {
			p.ClearAuthFail()
			LogInfo("HealthChecker: %s recovered from auth_failed, backoff reset", p.Name)
		}

		if status != "healthy" {
			LogWarn("HealthChecker: %s (%s) → %s, latency=%dms, err=%s",
				p.Name, p.Type, status, latency, errMsg)
		}
	}
}

// checkProvider 检测单个 Provider
func (hc *HealthChecker) checkProvider(p *Provider) (status string, latencyMs int, errMsg string) {
	switch p.Type {
	case "direct":
		return hc.checkDirect(p)
	case "aggregate":
		return hc.checkAggregate(p)
	case "litellm":
		return hc.checkLiteLLM(p)
	default: // custom + 其他
		return hc.checkGeneric(p)
	}
}

// checkDirect 直连官方 API 检测
func (hc *HealthChecker) checkDirect(p *Provider) (string, int, string) {
	// 根据 base_url 识别厂商，使用最小化测试请求
	endpoint, body := getVendorTestConfig(p.BaseURL)
	if endpoint == "" {
		return hc.checkGeneric(p)
	}
	// 优先使用 Provider 实际配置的模型来检测
	models := parseModelsList(p.Models)
	if len(models) > 0 {
		body = injectModel(body, models[0])
	}
	// 去重：如果 base_url 路径已包含 endpoint 的前缀，则去掉重复部分
	// 例：base_url=.../compatible-mode/v1 + endpoint=/compatible-mode/v1/chat/completions
	//   → 实际 endpoint=/chat/completions
	if strings.HasSuffix(strings.TrimRight(p.BaseURL, "/"), endpoint) {
		endpoint = "/"
	} else {
		parsed := parseURL(p.BaseURL)
		basePath := strings.Trim(parsed.Path, "/")
		if basePath != "" {
			fullPrefix := "/" + basePath + "/"
			if strings.HasPrefix(endpoint, fullPrefix) {
				endpoint = strings.TrimPrefix(endpoint, fullPrefix)
				if !strings.HasPrefix(endpoint, "/") {
					endpoint = "/" + endpoint
				}
			} else if strings.HasSuffix(basePath, "/v1") && strings.HasPrefix(endpoint, "/v1/") {
				endpoint = strings.TrimPrefix(endpoint, "/v1")
				if !strings.HasPrefix(endpoint, "/") {
					endpoint = "/" + endpoint
				}
			}
		}
	}
	return hc.sendTestRequest(p, endpoint, body)
}

// checkAggregate 聚合平台检测（OpenAI 兼容）
func (hc *HealthChecker) checkAggregate(p *Provider) (string, int, string) {
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`
	return hc.sendTestRequest(p, "/v1/chat/completions", body)
}

// checkLiteLLM LiteLLM 检测
func (hc *HealthChecker) checkLiteLLM(p *Provider) (string, int, string) {
	// 先尝试 /health
	status, latency, errMsg := hc.sendGET(p, "/health")
	if status == "healthy" {
		return status, latency, errMsg
	}
	// 回退到 /v1/models
	return hc.sendGET(p, "/v1/models")
}

// checkGeneric 通用检测
func (hc *HealthChecker) checkGeneric(p *Provider) (string, int, string) {
	if p.BaseURL == "" {
		return "dead", 0, "no base_url configured"
	}
	// 尝试 /v1/models
	status, latency, errMsg := hc.sendGET(p, "/v1/models")
	if status == "healthy" {
		return status, latency, errMsg
	}
	// 回退到 chat completions 测试
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`
	return hc.sendTestRequest(p, "/v1/chat/completions", body)
}

// sendTestRequest 发送测试 chat 请求
func (hc *HealthChecker) sendTestRequest(p *Provider, endpoint, body string) (string, int, string) {
	url := strings.TrimRight(p.BaseURL, "/") + endpoint
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "dead", 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	return hc.doRequest(req)
}

// sendGET 发送 GET 请求
func (hc *HealthChecker) sendGET(p *Provider, endpoint string) (string, int, string) {
	url := strings.TrimRight(p.BaseURL, "/") + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "dead", 0, err.Error()
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	return hc.doRequest(req)
}

// doRequest 执行请求并判定状态
func (hc *HealthChecker) doRequest(req *http.Request) (string, int, string) {
	client := &http.Client{Timeout: hc.timeout}
	start := time.Now()

	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return "timeout", latency, "request timeout"
		}
		if strings.Contains(err.Error(), "connection refused") {
			return "dead", latency, "connection refused"
		}
		return "dead", latency, err.Error()
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		return "healthy", latency, ""
	case resp.StatusCode == 429:
		return "rate_limited", latency, "rate limited"
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return "auth_failed", latency, fmt.Sprintf("auth failed (HTTP %d)", resp.StatusCode)
	default:
		return "unhealthy", latency, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
}

// parseURL 安全解析 URL，失败返回空结构体
func parseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}
	return u
}

// injectModel 替换 JSON body 中的 model 字段
func injectModel(body, model string) string {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		return body
	}
	obj["model"] = model
	out, err := json.Marshal(obj)
	if err != nil {
		return body
	}
	return string(out)
}

// --- 厂商测试配置 ---

var vendorTestConfigs []VendorTestConfig

// ReloadVendorTestConfigs 从 DB 重新加载厂商测试配置
func ReloadVendorTestConfigs() {
	vendorTestConfigs = LoadHealthTestConfigs()
	LogInfo("Vendor test configs reloaded: %d entries", len(vendorTestConfigs))
}

func getVendorTestConfig(baseURL string) (endpoint, body string) {
	if len(vendorTestConfigs) == 0 {
		vendorTestConfigs = LoadHealthTestConfigs()
	}
	for _, cfg := range vendorTestConfigs {
		if strings.Contains(strings.ToLower(baseURL), cfg.Domain) {
			return cfg.Endpoint, cfg.Body
		}
	}
	return "", ""
}

// parseModelsList 解析 models JSON 字符串
func parseModelsList(modelsJSON string) []string {
	if modelsJSON == "" || modelsJSON == "[]" {
		return nil
	}
	var models []string
	if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
		return nil
	}
	return models
}
