package main

import (
	"fmt"
	"strings"
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
	HealthStatus  string     `json:"-"` // 缓存的健康状态
	ConsecFails   int        `json:"-"` // 连续失败次数
	CooldownUntil *time.Time `json:"-"` // 冷却截止时间（长时限流/额度用完时设置）
}

// IsAvailable 判断 Provider 是否可参与调度
func (p *Provider) IsAvailable(model string) bool {
	if !p.Enabled || !p.ProxyEnabled {
		return false
	}
	if p.Status == "dead" || p.Status == "disabled" || p.Status == "auth_failed" {
		return false
	}
	// 冷却期内跳过（长时限流/额度用完，等也没用）
	if p.CooldownUntil != nil && time.Now().Before(*p.CooldownUntil) {
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
	ID              int        `json:"id"`
	Name            string     `json:"name"`
	Key             string     `json:"key"`               // sk-wr-...xxxx
	UserID          int        `json:"user_id"`
	Models          string     `json:"models"`            // JSON array: ["gpt-4o"], 空=全部
	ProviderIDs     string     `json:"provider_ids"`      // JSON array: [1,3], 空=全部
	QuotaTotal      int64      `json:"quota_total"`       // 总额度(分), 0=不限
	QuotaUsed       int64      `json:"quota_used"`        // 已用额度(分)
	RateLimitRPM    int        `json:"rate_limit_rpm"`    // 每分钟限速, 0=不限
	SubnetWhitelist string     `json:"subnet_whitelist"`  // JSON array: ["10.0.0.0/8"]
	SmartDowngrade      bool       `json:"smart_downgrade"`      // 允许自动降级（强模型→便宜模型）
	DesensitizeEnabled bool       `json:"desensitize_enabled"` // 是否启用脱敏
	DesensitizeLevel   string     `json:"desensitize_level"`   // 脱敏级别：off/standard/strict

	// 知识捕获扩展字段
	KnowledgeCaptureEnabled bool   `json:"knowledge_capture_enabled"` // 是否开启知识捕获
	KnowledgeDepartment     string `json:"knowledge_department"`      // 归属部门
	RAGEnabled              bool   `json:"rag_enabled"`               // 是否开启RAG自动注入
	RAGMinRelevance         float64 `json:"rag_min_relevance"`        // RAG最低相关度阈值
	RAGTopK                 int    `json:"rag_top_k"`                 // RAG注入最多条数
	SystemPromptKnowledge   string `json:"system_prompt_knowledge"`   // 自定义System Prompt知识片段

	Enabled            bool       `json:"enabled"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
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
	ID            int64     `json:"id"`
	RequestID     string    `json:"request_id"`
	TokenID       int       `json:"token_id"`
	TokenName     string    `json:"token_name"`
	ProviderID    int       `json:"provider_id"`
	ProviderName  string    `json:"provider_name"`
	ModelName     string    `json:"model_name"`
	Endpoint      string    `json:"endpoint"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	CachedTokens  int64     `json:"cached_tokens"`
	StatusCode    int       `json:"status_code"`
	LatencyMs     int       `json:"latency_ms"`
	CostCents     int64     `json:"cost_cents"`
	IsStream      bool      `json:"is_stream"`
	IsRetry       bool      `json:"is_retry"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	ErrorType     string    `json:"error_type,omitempty"` // quota_exhausted/rate_limited/timeout/unknown
	ClientIP      string    `json:"client_ip"`
	CreatedAt     time.Time `json:"created_at"`
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

// ResolveModelAlias 将短别名解析为完整模型名
// 1. 精确匹配别名表（qwen-plus → qwen-plus-2025-07-28）
// 2. 前缀匹配兜底（gpt-4o → gpt-4o-2024-05-13，如果别名表中没有的话）
func ResolveModelAlias(model string) (string, bool) {
	if model == "" {
		return "", false
	}

	// 1. 精确匹配别名表
	modelAliasMutex.RLock()
	target, ok := modelAliasMap[model]
	modelAliasMutex.RUnlock()
	if ok {
		return target, true
	}

	// 2. 前缀匹配：用模型列表中某个以 model 为前缀的完整名
	return "", false
}

// ResolveModelWithPrefix 尝试前缀匹配：从 modelsJSON 中找以 model 开头的模型
func ResolveModelWithPrefix(model string, modelsJSON string) (string, bool) {
	if model == "" || modelsJSON == "" {
		return "", false
	}

	// 先尝试从别名表解析
	modelAliasMutex.RLock()
	target, ok := modelAliasMap[model]
	modelAliasMutex.RUnlock()
	if ok {
		return target, true
	}

	// 前缀匹配：解析 modelsJSON 中的每个模型，找以 model 为前缀的
	models := parseModelsList(modelsJSON)
	prefix := model + "-"
	for _, m := range models {
		if strings.HasPrefix(m, prefix) {
			return m, true
		}
	}
	return "", false
}

func modelInList(model, modelsJSON string) bool {
	// 1. 精确匹配
	if containsJSONString(modelsJSON, model) {
		return true
	}
	// 2. 别名表映射后精确匹配
	modelAliasMutex.RLock()
	target, ok := modelAliasMap[model]
	modelAliasMutex.RUnlock()
	if ok && containsJSONString(modelsJSON, target) {
		return true
	}
	// 3. 前缀匹配：qwen-plus 匹配 qwen-plus-2025-07-28
	models := parseModelsList(modelsJSON)
	prefix := model + "-"
	for _, m := range models {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

func intInList(id int, listJSON string) bool {
	s := fmt.Sprintf("%d", id)
	return containsJSONString(listJSON, s)
}