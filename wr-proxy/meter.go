package main

// 计量模块：写日志、扣配额、限速计数

import (
	"sync"
	"time"
)

// Meter 计量服务
type Meter struct {
	mu sync.Mutex
	// Provider 分钟级用量缓存（用于额度预测）
	providerMinute map[int]*minuteBucket
}

type minuteBucket struct {
	count      int
	validCount int   // 有效请求次数（status < 400 且非重试且无错误）
	tokens     int64
	costCents  int64
	start      time.Time
}

var meter = &Meter{
	providerMinute: make(map[int]*minuteBucket),
}

// RecordRequest 记录一次请求
func (m *Meter) RecordRequest(rlog *RequestLog) {
	// 1. 写 DB 日志
	if err := InsertRequestLog(rlog); err != nil {
		LogWarn("Meter: write log failed: %v", err)
	}

	// 2. 扣 Token 配额
	if rlog.CostCents > 0 && rlog.StatusCode < 400 {
		DeductTokenQuota(rlog.TokenID, rlog.CostCents)
	}

	// 3. 累加 Provider 用量
	if rlog.CostCents > 0 {
		UpdateProviderQuota(rlog.ProviderID, rlog.CostCents)
	}

	// 4. 内存缓存（用于实时统计和预测）
	m.mu.Lock()
	b, ok := m.providerMinute[rlog.ProviderID]
	now := time.Now()
	isValid := rlog.StatusCode < 400 && !rlog.IsRetry && rlog.ErrorMessage == ""
	if !ok || now.Sub(b.start) >= time.Minute {
		m.providerMinute[rlog.ProviderID] = &minuteBucket{
			count:      1,
			validCount: boolToInt(isValid),
			tokens:     rlog.InputTokens + rlog.OutputTokens,
			costCents:  rlog.CostCents,
			start:      now,
		}
	} else {
		b.count++
		if isValid {
			b.validCount++
		}
		b.tokens += rlog.InputTokens + rlog.OutputTokens
		b.costCents += rlog.CostCents
	}
	m.mu.Unlock()
}

// GetProviderMinuteStats 获取 Provider 当前分钟用量
func (m *Meter) GetProviderMinuteStats(providerID int) (count int, validCount int, tokens int64, cost int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.providerMinute[providerID]
	if !ok || time.Since(b.start) >= time.Minute {
		return 0, 0, 0, 0
	}
	return b.count, b.validCount, b.tokens, b.costCents
}

// BuildRequestLog 从代理结果构造 RequestLog
func BuildRequestLog(reqID string, token *Token, provider *Provider,
	model, endpoint, clientIP string, result *ProxyResult, isRetry bool) *RequestLog {

	// 估算成本
	var costCents int64
	if result.InputTokens > 0 || result.OutputTokens > 0 {
		costCents = CalculateCost(model, result.InputTokens, result.OutputTokens, provider.CostMultiplier)
	}

	errMsg := ""
	if result.Error != "" {
		errMsg = result.Error
	}

	return &RequestLog{
		RequestID:    reqID,
		TokenID:      token.ID,
		TokenName:    token.Name,
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		ModelName:    model,
		Endpoint:     endpoint,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		StatusCode:   result.StatusCode,
		LatencyMs:    result.LatencyMs,
		CostCents:    costCents,
		IsStream:     result.IsStream,
		IsRetry:      isRetry,
		ErrorMessage: errMsg,
		ClientIP:     clientIP,
	}
}
