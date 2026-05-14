package main

// 智能重试引擎：上游错误语义识别 + 请求Hash缓存 + 降级策略决策

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
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
	"AccountQuotaExceeded",    // DashScope 5小时额度超额
	"DataInsufficient",        // DashScope 额度不足
	"InvalidApiKey",           // DashScope Key无效
	"Forbidden",               // DashScope 禁止访问
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
// 三层防御策略：
// 1. HTTP状态码优先：429=限流, 402=额度, 403=鉴权, 503=服务不可用
// 2. JSON结构通用提取：从任意层级提取 code+type+message，拼接后做模糊语义匹配
// 3. 兜底：非JSON纯文本匹配 / 有错误信息但未分类→unknown
func DetectUpstreamError(statusCode int, body []byte) UpstreamErrorDetail {
	if len(body) == 0 {
		// 无body时仅靠状态码
		switch {
		case statusCode == 429:
			return UpstreamErrorDetail{Type: UpstreamErrRateLimited, Message: "HTTP 429 Too Many Requests"}
		case statusCode == 402:
			return UpstreamErrorDetail{Type: UpstreamErrQuotaExhausted, Message: "HTTP 402 Payment Required"}
		case statusCode == 403:
			return UpstreamErrorDetail{Type: UpstreamErrQuotaExhausted, Message: "HTTP 403 Forbidden"}
		case statusCode == 503:
			return UpstreamErrorDetail{Type: UpstreamErrTimeout, Message: "HTTP 503 Service Unavailable"}
		}
		return UpstreamErrorDetail{}
	}

	// 从 JSON 提取错误消息和代码
	msg, code, errType := extractErrorMessage(body)
	combined := strings.ToLower(msg + " " + code + " " + errType)

	// 有提取内容时按优先级匹配（额度 > 频率 > 超时）
	if combined != "  " && combined != "" {
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

	// 状态码兜底（即使body匹配不到，状态码也蕴含语义）
	switch {
	case statusCode == 429:
		return UpstreamErrorDetail{Type: UpstreamErrRateLimited, Message: msg, Code: code}
	case statusCode == 402:
		return UpstreamErrorDetail{Type: UpstreamErrQuotaExhausted, Message: msg, Code: code}
	case statusCode == 403:
		// 403可能是鉴权失败也可能是额度问题，如果有msg则保留，否则归为额度
		return UpstreamErrorDetail{Type: UpstreamErrQuotaExhausted, Message: msg, Code: code}
	case statusCode == 503 || statusCode == 502:
		return UpstreamErrorDetail{Type: UpstreamErrTimeout, Message: msg, Code: code}
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

// extractErrorMessage 从响应 body 中提取错误消息、代码和类型
// 支持 OpenAI 标准格式: {"error": {"message": "...", "code": "...", "type": "..."}}
// 以及 DashScope 格式: {"code": "...", "message": "...", "type": "..."}
// 以及简化格式: {"error": "...", "message": "..."}
func extractErrorMessage(body []byte) (msg, code, errType string) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		// 非 JSON，直接当纯文本匹配
		text := strings.ToLower(string(body))
		if matchPatterns(text, quotaExhaustedPatterns) ||
			matchPatterns(text, rateLimitPatterns) ||
			matchPatterns(text, timeoutPatterns) {
			return string(body), "", ""
		}
		return "", "", ""
	}

	// 标准格式: {"error": {"message": "...", "code": "...", "type": "..."}}
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
			if t, ok := errObj["type"]; ok {
				errType = strings.Trim(string(t), `"`)
			}
			// OpenAI 格式: error.type 作为 code 补充
			if code == "" && errType != "" {
				code = errType
			}
			return msg, code, errType
		}
		// 简化格式: {"error": "some string"}
		var errStr string
		if err := json.Unmarshal(errRaw, &errStr); err == nil {
			return errStr, "", ""
		}
	}

	// 直接在顶层查找 (DashScope 等格式)
	if m, ok := raw["message"]; ok {
		msg = strings.Trim(string(m), `"`)
	}
	if c, ok := raw["code"]; ok {
		code = strings.Trim(string(c), `"`)
	}
	if t, ok := raw["type"]; ok {
		errType = strings.Trim(string(t), `"`)
	}

	return msg, code, errType
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
// 仅短时限流（<60s）和超时值得等一下重试
// 长时限流（5小时额度用完等）直接标记冷却，不浪费时间重试
func ShouldRetrySameProvider(errDetail UpstreamErrorDetail) bool {
	switch errDetail.Type {
	case UpstreamErrQuotaExhausted:
		// 额度用完，重试无意义
		return false
	case UpstreamErrRateLimited:
		// 短时限流可以重试，长时限流不行
		waitSec := ExtractRetryAfter(errDetail.Message)
		if waitSec > 60 {
			// 超过60秒的限流 = 长时限流，不重试
			return false
		}
		return true
	case UpstreamErrTimeout:
		// 超时可能是偶发的，可以重试
		return true
	default:
		return false
	}
}

// ExtractRetryAfter 从错误消息中提取等待秒数
// 支持多种格式：
//   - "It will reset at 2026-05-14 12:35:20 +0800 CST"（精确时间）
//   - "Expected available in 18000 seconds"
//   - "retry after 300s"
//   - "5-hour limit"
func ExtractRetryAfter(msg string) int {
	// 1. 优先匹配精确重置时间: "reset at 2026-05-14 12:35:20 +0800 CST"
	if resetTime := parseResetAtTime(msg); !resetTime.IsZero() {
		wait := time.Until(resetTime)
		if wait > 0 {
			return int(wait.Seconds())
		}
	}

	lower := strings.ToLower(msg)

	// 2. "available in N seconds/minute/hour"
	if idx := strings.Index(lower, "available in "); idx >= 0 {
		rest := lower[idx+13:]
		if n := extractLeadingNumber(rest); n > 0 {
			if strings.Contains(rest, "hour") {
				return n * 3600
			}
			if strings.Contains(rest, "minute") {
				return n * 60
			}
			return n // seconds
		}
	}

	// 3. "retry after Ns/Nsec/N seconds/N minutes"
	if idx := strings.Index(lower, "retry after "); idx >= 0 {
		rest := lower[idx+12:]
		if n := extractLeadingNumber(rest); n > 0 {
			if strings.Contains(rest, "hour") {
				return n * 3600
			}
			if strings.Contains(rest, "min") {
				return n * 60
			}
			return n
		}
	}

	// 4. "N-hour limit", "5-hour", "5 hour"
	if idx := strings.Index(lower, "hour"); idx >= 0 {
		prefix := lower[:idx]
		if n := extractTrailingNumber(prefix); n > 0 {
			return n * 3600
		}
	}

	// 5. "N-minute limit"
	if idx := strings.Index(lower, "minute"); idx >= 0 {
		prefix := lower[:idx]
		if n := extractTrailingNumber(prefix); n > 0 {
			return n * 60
		}
	}

	return 0 // 无法提取
}

// parseResetAtTime 从消息中解析 "reset at YYYY-MM-DD HH:MM:SS +0800 CST" 格式
func parseResetAtTime(msg string) time.Time {
	// 匹配 "reset at 2026-05-14 12:35:20 +0800 CST"
	idx := strings.Index(strings.ToLower(msg), "reset at ")
	if idx < 0 {
		return time.Time{}
	}
	timeStr := msg[idx+9:] // "2026-05-14 12:35:20 +0800 CST"

	// 尝试多种时间格式
	formats := []string{
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 +0800 CST",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, timeStr); err == nil {
			return t
		}
		// 只解析前 len(format) 需要的字符
		if len(timeStr) > len(fmt)+5 {
			if t, err := time.Parse(fmt, timeStr[:len(fmt)+5]); err == nil {
				return t
			}
		}
	}

	// 兜底：用正则提取日期时间部分
	// "2026-05-14 12:35:20"
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)
	m := re.FindStringSubmatch(msg)
	if len(m) > 1 {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", m[1], time.FixedZone("CST", 8*3600)); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", m[1]); err == nil {
			return t
		}
	}

	return time.Time{}
}

func extractLeadingNumber(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if n > 0 {
			break
		}
	}
	return n
}

func extractTrailingNumber(s string) int {
	n := 0
	multiplier := 1
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c >= '0' && c <= '9' {
			n += int(c-'0') * multiplier
			multiplier *= 10
		} else if n > 0 {
			break
		}
	}
	return n
}
