package main

import (
	"fmt"
	"time"
)

// Provider 上游 API 数据源
type Provider struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`            // direct/aggregate/litellm/custom
	BaseURL        string    `json:"base_url"`
	APIKey         string    `json:"api_key,omitempty"`
	Models         string    `json:"models"`          // JSON array string: ["gpt-4o","claude-3"]
	Tags           string    `json:"tags"`            // JSON array string
	Priority       int       `json:"priority"`        // 0-100: 90+主力, 50-89热备, 1-49冷备, 0禁用
	Weight         int       `json:"weight"`          // 调度权重 0-100
	ProxyEnabled   bool      `json:"proxy_enabled"`   // 是否纳入代理池
	RateLimitRPM   int       `json:"rate_limit_rpm"`  // 每分钟请求上限, 0=不限
	TimeoutSeconds int       `json:"timeout_seconds"` // 请求超时
	MaxRetries     int       `json:"max_retries"`     // 最大重试次数
	CostMultiplier float64  `json:"cost_multiplier"` // 成本倍率
	Enabled        bool      `json:"enabled"`
	Status         string    `json:"status"`          // healthy/warning/dead/rate_limited/disabled
	LastCheckAt    *time.Time `json:"last_check_at"`
	LastLatencyMs  int       `json:"last_latency_ms"`
	LastError      string    `json:"last_error,omitempty"`

	// 额度信息（从 Flask API 获取或手动配置）
	QuotaTotal    int64   `json:"quota_total"`     // 总额度(分), 0=未知/不限
	QuotaUsed     int64   `json:"quota_used"`      // 已用额度(分)
	QuotaSource   string  `json:"quota_source"`    // manual/api/unknown

	// 能力标记
	SupportsTools bool `json:"supports_tools"` // 是否支持 function calling / tools

	// 运行时状态（不持久化）
	HealthStatus  string  `json:"-"` // 缓存的健康状态
	ConsecFails   int     `json:"-"` // 连续失败次数
}

// IsAvailable 判断 Provider 是否可参与调度
func (p *Provider) IsAvailable(model string) bool {
	if !p.Enabled || !p.ProxyEnabled {
		return false
	}
	if p.Status == "dead" || p.Status == "disabled" {
		return false
	}
	if p.Priority == 0 {
		return false
	}
	// 检查模型支持
	if model != "" && p.Models != "" {
		if !modelInList(model, p.Models) {
			return false
		}
	}
	return true
}

// PriorityGroup 返回优先级分组
func (p *Provider) PriorityGroup() string {
	switch {
	case p.Priority >= 90:
		return "primary" // 主力
	case p.Priority >= 50:
		return "hot"     // 热备
	default:
		return "cold"    // 冷备
	}
}

// QuotaRatio 返回额度剩余比例，0=无额度信息
func (p *Provider) QuotaRatio() float64 {
	if p.QuotaTotal <= 0 {
		return 1.0 // 未知额度视为充裕
	}
	remaining := p.QuotaTotal - p.QuotaUsed
	if remaining <= 0 {
		return 0
	}
	return float64(remaining) / float64(p.QuotaTotal)
}

// QuotaRemaining 返回剩余额度
func (p *Provider) QuotaRemaining() int64 {
	if p.QuotaTotal <= 0 {
		return -1 // 未知
	}
	r := p.QuotaTotal - p.QuotaUsed
	if r < 0 {
		return 0
	}
	return r
}

// Token 对外 API Key
type Token struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Key             string    `json:"key"`               // sk-wr-xxxxxxxxxxxx
	UserID          int       `json:"user_id"`
	Models          string    `json:"models"`            // JSON array: ["gpt-4o"], 空=全部
	ProviderIDs     string    `json:"provider_ids"`      // JSON array: [1,3], 空=全部
	QuotaTotal      int64     `json:"quota_total"`       // 总额度(分), 0=不限
	QuotaUsed       int64     `json:"quota_used"`        // 已用额度(分)
	RateLimitRPM    int       `json:"rate_limit_rpm"`    // 每分钟限速, 0=不限
	SubnetWhitelist string    `json:"subnet_whitelist"`  // JSON array: ["10.0.0.0/8"]
	Enabled         bool      `json:"enabled"`
	ExpiresAt       *time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// IsExpired 检查 Token 是否过期
func (t *Token) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// QuotaRemaining 返回 Token 剩余额度
func (t *Token) QuotaRemaining() int64 {
	if t.QuotaTotal <= 0 {
		return -1 // 不限
	}
	r := t.QuotaTotal - t.QuotaUsed
	if r < 0 {
		return 0
	}
	return r
}

// QuotaRatio 返回额度剩余比例
func (t *Token) QuotaRatio() float64 {
	if t.QuotaTotal <= 0 {
		return 1.0
	}
	remaining := t.QuotaTotal - t.QuotaUsed
	if remaining <= 0 {
		return 0
	}
	return float64(remaining) / float64(t.QuotaTotal)
}

// CanUseModel 检查 Token 是否允许使用该模型
func (t *Token) CanUseModel(model string) bool {
	if t.Models == "" || t.Models == "[]" {
		return true // 未限制
	}
	return modelInList(model, t.Models)
}

// CanUseProvider 检查 Token 是否允许使用该 Provider
func (t *Token) CanUseProvider(providerID int) bool {
	if t.ProviderIDs == "" || t.ProviderIDs == "[]" {
		return true
	}
	return intInList(providerID, t.ProviderIDs)
}

// RequestLog 请求日志
type RequestLog struct {
	ID           int64     `json:"id"`
	RequestID    string    `json:"request_id"`
	TokenID      int       `json:"token_id"`
	TokenName    string    `json:"token_name"`
	ProviderID   int       `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	ModelName    string    `json:"model_name"`
	Endpoint     string    `json:"endpoint"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	StatusCode   int       `json:"status_code"`
	LatencyMs    int       `json:"latency_ms"`
	CostCents    int64     `json:"cost_cents"`
	IsStream     bool      `json:"is_stream"`
	IsRetry      bool      `json:"is_retry"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ClientIP     string    `json:"client_ip"`
	CreatedAt    time.Time `json:"created_at"`
}

// QuotaPrediction 额度预测结果
type QuotaPrediction struct {
	ProviderID            int       `json:"provider_id"`
	QuotaRemaining        int64     `json:"quota_remaining"`
	DailyBurnRate         float64   `json:"daily_burn_rate"`
	DaysUntilExhaust      float64   `json:"days_until_exhaust"`
	PredictedExhaustDate  string    `json:"predicted_exhaust_date"`
	Trend                 string    `json:"trend"` // increasing/stable/decreasing
	Confidence            float64   `json:"confidence"`
	AlertLevel            string    `json:"alert_level"` // green/yellow/orange/red/black
}

// --- 辅助函数 ---

func modelInList(model, modelsJSON string) bool {
	// 简单的字符串匹配，避免每个请求都 JSON parse
	// 匹配 "gpt-4o" 在 ["gpt-4o","claude-3"] 中
	return containsJSONString(modelsJSON, model)
}

func intInList(id int, listJSON string) bool {
	s := fmt.Sprintf("%d", id)
	return containsJSONString(listJSON, s)
}