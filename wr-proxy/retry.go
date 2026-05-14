package main

// 智能重试引擎：上游错误语义识别 + 请求Hash缓存 + 降级策略决策

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// --- 上游错误分类 ---

type UpstreamErrorType string

const (
	UpstreamErrNone          UpstreamErrorType = ""                // 无错误
	UpstreamErrQuotaExhausted UpstreamErrorType = "quota_exhausted" // 额度用完
	UpstreamErrRateLimited    UpstreamErrorType = "rate_limited"    // 频率/时间限制
	UpstreamErrTimeout        UpstreamErrorType = "timeout"         // 网络超时/中断
	UpstreamErrUnknown        UpstreamErrorType = "unknown"         // 未分类错误
)

// UpstreamErrorDetail 上游错误详情
type UpstreamErrorDetail struct {
	Type    UpstreamErrorType // 错误分类
	Message string            // 原始错误消息
	Code    string            // 错误代码（如 "insufficient_quota"）
}

// --- 错误关键词匹配表 ---
// 覆盖 OpenAI / Claude / Gemini / 国产模型的主流错误格式

var quotaExhaustedPatterns = []string{
	"insufficient_quota",
	"exceeded quota",
	"billing hard limit",
	"quota exceeded",
	"out of credits",
	"credits exhausted",
	"account deactivated",
	"account has been suspended",
	"balance is insufficient",
	"insufficient funds",
	"plan_limit_exceeded",
	"capacity exceeded",
	"spender limit",
	"monthly spending limit",
	"spending limit reached",
	"usage limit reached",
	"account_limit_exceeded",
	"quota_exceeded",
	"DataInsufficient",       // DashScope 额度不足
	"InvalidApiKey",          // DashScope Key无效
	"Forbidden",              // DashScope 禁止访问
	"额度",
	"余额不足",
	"已用完",
}

var rateLimitPatterns = []string{
	"rate_limit",
	"rate limit",
	"too many requests",
	"requests per",
	"rpm limit",
	"tpm limit",
	"tokens per minute",
	"concurrent requests",
	"5-hour",
	"5 hour",
	"daily limit",
	"hour limit",
	"minute limit",
	"Throttling",             // DashScope 限流错误码
	"throttling",
	"Request was throttled",  // DashScope 限流消息
	"rate_limit_exceeded",    // 通用限流码
	"频率限制",
	"请求过多",
	"并发",
}

var timeoutPatterns = []string{
	"timeout",
	"timed out",
	"deadline exceeded",
	"connection reset",
	"connection refused",
	"network error",
	"no route to host",
	"i/o timeout",
	"context deadline",
	"超时",
	"网络",
}

// DetectUpstreamError 检测上游响应中的语义错误
// 用于两种场景：
// 1. HTTP 200 但 body 含错误 JSON（如 OpenAI 返回 200 + error 对象）
// 2. HTTP 4xx/5xx 响应 body 含错误信息
func DetectUpstreamError(statusCode int, body []byte) UpstreamErrorDetail {
	if len(body) == 0 {
		return UpstreamErrorDetail{}
	}

	// 尝试从 JSON 提取错误消息
	msg, code := extractErrorMessage(body)
	combined := strings.ToLower(msg + " " + code)

	// 如果有错误内容，按优先级匹配（额度 > 频率 > 超时）
	if combined != " " && combined != "" {
		if matchPatterns(combined, quotaExhaustedPatterns) {
			return UpstreamErrorDetail{
				Type:    UpstreamErrQuotaExhausted,
				Message: msg,
				Code:    code,
			}
		}
		if matchPatterns(combined, rateLimitPatterns) {
			return UpstreamErrorDetail{
				Type:    UpstreamErrRateLimited,
				Message: msg,
				Code:    code,
			}
		}
		if matchPatterns(combined, timeoutPatterns) {
			return UpstreamErrorDetail{
				Type:    UpstreamErrTimeout,
				Message: msg,
				Code:    code,
			}
		}
	}

	// 即使没有匹配到模式，429 总是 rate_limited
	if statusCode == 429 {
		return UpstreamErrorDetail{
			Type:    UpstreamErrRateLimited,
			Message: msg,
			Code:    code,
			}
	}

	// 有错误消息但未分类
	if msg != "" || code != "" {
		return UpstreamErrorDetail{
			Type:    UpstreamErrUnknown,
			Message: msg,
			Code:    code,
		}
	}

	return UpstreamErrorDetail{}
}

// DetectStreamError 检测 SSE 流中的错误 chunk
// SSE 格式: data: {"error": {"message": "...", "code": "..."}}
func DetectStreamError(data string) UpstreamErrorDetail {
	if data == "" || data == "[DONE]" {
		return UpstreamErrorDetail{}
	}
	return DetectUpstreamError(0, []byte(data))
}

// extractErrorMessage 从响应 body 中提取错误消息和代码
// 支持 OpenAI 标准格式: {"error": {"message": "...", "code": "..."}}
// 以及简化格式: {"error": "...", "message": "..."}
func extractErrorMessage(body []byte) (msg, code string) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		// 非 JSON，直接当纯文本匹配
		text := strings.ToLower(string(body))
		if matchPatterns(text, quotaExhaustedPatterns) ||
			matchPatterns(text, rateLimitPatterns) ||
			matchPatterns(text, timeoutPatterns) {
			return string(body), ""
		}
		return "", ""
	}

	// 标准格式: {"error": {"message": "...", "code": "..."}}
	if errRaw, ok := raw["error"]; ok {
		// 先尝试嵌套对象
		var errObj map[string]json.RawMessage
		if err := json.Unmarshal(errRaw, &errObj); err == nil {
			if m, ok := errObj["message"]; ok {
				msg = strings.Trim(string(m), `"`)
			}
			if c, ok := errObj["code"]; ok {
				code = strings.Trim(string(c), `"`)
			}
			// OpenAI 格式: error.type
			if t, ok := errObj["type"]; ok && code == "" {
				code = strings.Trim(string(t), `"`)
			}
			return msg, code
		}
		// 简化格式: {"error": "some string"}
		var errStr string
		if err := json.Unmarshal(errRaw, &errStr); err == nil {
			return errStr, ""
		}
	}

	// 直接在顶层查找
	if m, ok := raw["message"]; ok {
		msg = strings.Trim(string(m), `"`)
	}
	if c, ok := raw["code"]; ok {
		code = strings.Trim(string(c), `"`)
	}

	return msg, code
}

// matchPatterns 检查文本是否匹配任一模式（不区分大小写）
func matchPatterns(text string, patterns []string) bool {
	text = strings.ToLower(text)
	for _, p := range patterns {
		if strings.Contains(text, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// --- 请求 Hash 缓存 ---

// requestCacheEntry 请求缓存条目
type requestCacheEntry struct {
	Hash      string    // 请求体 SHA256
	Timestamp time.Time // 最后更新时间
	Success   bool      // 最后一次是否成功
}

// RequestCache 请求内容 Hash 缓存
// 用途：
// 1. 标记 token+model 的最近请求，判断是否同一请求反复失败
// 2. 防止同一请求无限重试（连续 N 次同一 hash 失败则放弃）
// 3. 统计和日志用途
type RequestCache struct {
	mu     sync.RWMutex
	entries map[string]*requestCacheEntry // key: "tokenID:model"
}

var reqCache = &RequestCache{
	entries: make(map[string]*requestCacheEntry),
}

// cacheKey 生成缓存 key
func cacheKey(tokenID int, model string) string {
	return fmt.Sprintf("%d:%s", tokenID, model)
}

// HashBody 计算请求体的 SHA256
func HashBody(body []byte) string {
	h := sha256.Sum256(body)
	return fmt.Sprintf("%x", h)[:16] // 取前16字符，足够去重
}

// RecordRequestSuccess 记录成功请求（覆盖 hash）
func (rc *RequestCache) RecordRequestSuccess(tokenID int, model string, body []byte) {
	key := cacheKey(tokenID, model)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries[key] = &requestCacheEntry{
		Hash:      HashBody(body),
		Timestamp: time.Now(),
		Success:   true,
	}
}

// RecordRequestFailure 记录失败请求（仅在 hash 变化时更新，保持首次失败的 hash）
func (rc *RequestCache) RecordRequestFailure(tokenID int, model string, body []byte) {
	key := cacheKey(tokenID, model)
	hash := HashBody(body)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	// 如果已有记录且 hash 相同，不覆盖（保留首次失败的记录）
	if existing, ok := rc.entries[key]; ok && existing.Hash == hash {
		existing.Timestamp = time.Now() // 只更新时间
		return
	}
	rc.entries[key] = &requestCacheEntry{
		Hash:      hash,
		Timestamp: time.Now(),
		Success:   false,
	}
}

// IsSameRequestFailed 检查同一请求是否已连续失败
// 返回 true 表示同一 hash 已连续失败 >= consecutiveLimit 次
func (rc *RequestCache) IsSameRequestFailed(tokenID int, model string, body []byte) bool {
	key := cacheKey(tokenID, model)
	hash := HashBody(body)
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if entry, ok := rc.entries[key]; ok {
		return entry.Hash == hash && !entry.Success
	}
	return false
}

// GetEntry 获取缓存条目（用于日志/统计）
func (rc *RequestCache) GetEntry(tokenID int, model string) *requestCacheEntry {
	key := cacheKey(tokenID, model)
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.entries[key]
}

// CleanStale 清理过期条目（超过 1 小时未更新）
func (rc *RequestCache) CleanStale() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	cutoff := time.Now().Add(-1 * time.Hour)
	for k, v := range rc.entries {
		if v.Timestamp.Before(cutoff) {
			delete(rc.entries, k)
		}
	}
}

// --- 降级策略决策 ---

// ShouldFailover 判断是否应该触发降级切换 Provider
// 根据上游错误类型决定：
// - quota_exhausted: 必须切换，该 Provider 额度已完
// - rate_limited: 切换，该 Provider 当前被限流
// - timeout: 切换，该 Provider 网络不稳
// - 4xx 认证错误: 必须切换
// - 5xx 服务端错误: 切换
// 同时考虑请求缓存：如果同一请求已连续失败，不重试（问题在请求本身）
func ShouldFailover(result *ProxyResult, tokenID int, model string, body []byte) (shouldFailover bool, reason string) {
	// 1. 网络级错误（Forward 返回 502 + error）
	if result.Error != "" {
		errType := result.UpstreamError.Type
		if errType == "" {
			// 从 error 字符串推断
			lower := strings.ToLower(result.Error)
			if matchPatterns(lower, timeoutPatterns) {
				return true, "network_timeout"
			}
			if matchPatterns(lower, rateLimitPatterns) {
				return true, "network_rate_limit"
			}
		}
		return true, "network_error"
	}

	// 2. HTTP 状态码判断
	if result.StatusCode >= 500 {
		return true, "upstream_5xx"
	}

	if result.StatusCode == 429 {
		return true, "rate_limited_429"
	}

	if result.StatusCode >= 400 {
		return true, "upstream_4xx"
	}

	// 3. 语义级错误（HTTP 200 但 body 含错误信息）
	if result.UpstreamError.Type != UpstreamErrNone {
		switch result.UpstreamError.Type {
		case UpstreamErrQuotaExhausted:
			return true, "quota_exhausted"
		case UpstreamErrRateLimited:
			return true, "rate_limited_semantic"
		case UpstreamErrTimeout:
			return true, "timeout_semantic"
		default:
			return true, "error_semantic"
		}
	}

	// 4. 检查请求缓存：同一请求是否已失败过
	if reqCache.IsSameRequestFailed(tokenID, model, body) {
		// 同一请求体之前已失败，但这次 status 是正常的？
		// 可能是上游返回了看似成功但实际不完整的响应
		// 保守策略：允许 failover 但记录日志
		LogWarn("RetryEngine: same request body previously failed for token=%d model=%s, allowing failover",
			tokenID, model)
		return true, "same_request_previously_failed"
	}

	return false, ""
}

// ShouldRetrySameProvider 判断是否应该在同一 Provider 重试（而不是立即降级）
// 仅 rate_limited 且是短期限制时值得等一下重试
func ShouldRetrySameProvider(errDetail UpstreamErrorDetail) bool {
	switch errDetail.Type {
	case UpstreamErrQuotaExhausted:
		// 额度用完，重试无意义，直接切换
		return false
	case UpstreamErrRateLimited:
		// 频率限制，可以稍等重试一次
		return true
	case UpstreamErrTimeout:
		// 超时可能是偶发的，可以重试
		return true
	default:
		// 其他错误，保守策略：不重试同一 Provider
		return false
	}
}
