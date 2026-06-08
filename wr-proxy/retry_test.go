// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"testing"
)

// --- ExtractRetryAfter 测试 ---

func TestExtractRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		expected int
	}{
		// DashScope 精确时间格式
		{
			name:     "DashScope reset at 精确时间",
			msg:      "You have exceeded the 5-hour usage quota. It will reset at 2026-05-14 12:35:20 +0800 CST.",
			expected: 0, // 会计算到实际时间差，只验证>0
		},
		// 通用 "available in" 格式
		{
			name:     "available in seconds",
			msg:      "Expected available in 18000 seconds",
			expected: 18000,
		},
		{
			name:     "available in minutes",
			msg:      "Rate limit exceeded. Expected available in 5 minutes",
			expected: 300,
		},
		{
			name:     "available in hours",
			msg:      "Expected available in 2 hours",
			expected: 7200,
		},
		// "retry after" 格式
		{
			name:     "retry after seconds",
			msg:      "Please retry after 300s",
			expected: 300,
		},
		{
			name:     "retry after minutes",
			msg:      "Retry after 10 minutes",
			expected: 600,
		},
		// N-hour 格式
		{
			name:     "5-hour limit",
			msg:      "You have exceeded the 5-hour usage quota",
			expected: 18000,
		},
		{
			name:     "1 hour limit",
			msg:      "Exceeded 1 hour limit",
			expected: 3600,
		},
		// N-minute 格式
		{
			name:     "10 minute limit",
			msg:      "Exceeded 10 minute limit",
			expected: 600,
		},
		// 无法提取
		{
			name:     "no time info",
			msg:      "Insufficient quota",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRetryAfter(tt.msg)
			if tt.name == "DashScope reset at 精确时间" {
				// 精确时间格式，只要能提取到>0就行
				if got <= 0 {
					t.Errorf("ExtractRetryAfter(%q) = %d, want > 0", tt.msg, got)
				} else {
					t.Logf("ExtractRetryAfter(%q) = %d seconds", tt.msg, got)
				}
			} else if got != tt.expected {
				t.Errorf("ExtractRetryAfter(%q) = %d, want %d", tt.msg, got, tt.expected)
			}
		})
	}
}

// --- DetectUpstreamError 测试 ---

func TestDetectUpstreamError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantType   string
		wantCode   string
	}{
		// DashScope: 额度用完 (AccountQuotaExceeded)
		{
			name:       "DashScope 额度超额",
			statusCode: 400,
			body:       `{"code":"AccountQuotaExceeded","message":"You have exceeded the 5-hour usage quota. It will reset at 2026-05-14 12:35:20 +0800 CST.","param":"","type":""}`,
			wantType:   "quota_exhausted",
			wantCode:   "AccountQuotaExceeded",
		},
		// OpenAI: 额度用完
		{
			name:       "OpenAI insufficient_quota",
			statusCode: 429,
			body:       `{"error":{"code":"insufficient_quota","message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota"}}`,
			wantType:   "quota_exhausted",
			wantCode:   "insufficient_quota",
		},
		// OpenAI: 限流
		{
			name:       "OpenAI rate_limit",
			statusCode: 429,
			body:       `{"error":{"code":"rate_limit_exceeded","message":"You are sending requests too quickly.","type":"rate_limit_error"}}`,
			wantType:   "rate_limited",
			wantCode:   "rate_limit_exceeded",
		},
		// DeepSeek: 余额不足
		{
			name:       "DeepSeek insufficient_balance",
			statusCode: 402,
			body:       `{"error":{"message":"Insufficient balance","type":"insufficient_balance"}}`,
			wantType:   "quota_exhausted",
			wantCode:   "insufficient_balance",
		},
		// DashScope: 限流 Throttling
		{
			name:       "DashScope Throttling",
			statusCode: 429,
			body:       `{"code":"Throttling","message":"Request was throttled. Expected available in 3 seconds","requestId":"xxx"}`,
			wantType:   "rate_limited",
			wantCode:   "Throttling",
		},
		// HTTP 429 无body
		{
			name:       "HTTP 429 no body",
			statusCode: 429,
			body:       ``,
			wantType:   "rate_limited",
			wantCode:   "",
		},
		// HTTP 402 无body
		{
			name:       "HTTP 402 no body",
			statusCode: 402,
			body:       ``,
			wantType:   "quota_exhausted",
			wantCode:   "",
		},
		// HTTP 503 无body
		{
			name:       "HTTP 503 no body",
			statusCode: 503,
			body:       ``,
			wantType:   "timeout",
			wantCode:   "",
		},
		// Claude: credit_limit_reached
		{
			name:       "Claude credit_limit",
			statusCode: 402,
			body:       `{"error":{"type":"error","message":"credit_limit_reached: Your account has reached its credit limit."}}`,
			wantType:   "quota_exhausted",
			wantCode:   "error",
		},
		// 超时
		{
			name:       "timeout error",
			statusCode: 500,
			body:       `{"error":{"message":"context deadline exceeded","code":"timeout"}}`,
			wantType:   "timeout",
			wantCode:   "timeout",
		},
		// HTTP 200 但body含错误（某些厂商的行为）
		{
			name:       "HTTP 200 with error body",
			statusCode: 200,
			body:       `{"error":{"code":"rate_limit_exceeded","message":"Too many requests"}}`,
			wantType:   "rate_limited",
			wantCode:   "rate_limit_exceeded",
		},
		// 未知错误
		{
			name:       "unknown error",
			statusCode: 500,
			body:       `{"error":{"message":"internal server error","code":"server_error"}}`,
			wantType:   "unknown",
			wantCode:   "server_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectUpstreamError(tt.statusCode, []byte(tt.body))
			if string(got.Type) != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if tt.wantCode != "" && got.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", got.Code, tt.wantCode)
			}
			t.Logf("Result: type=%q code=%q msg=%q", got.Type, got.Code, got.Message)
		})
	}
}

// --- extractErrorMessage 测试 ---

func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantMsg  string
		wantCode string
		wantType string
	}{
		{
			name:     "DashScope 格式 (顶层 code+message)",
			body:     `{"code":"AccountQuotaExceeded","message":"quota exceeded","type":""}`,
			wantMsg:  "quota exceeded",
			wantCode: "AccountQuotaExceeded",
			wantType: "",
		},
		{
			name:     "OpenAI 格式 (嵌套 error)",
			body:     `{"error":{"message":"insufficient quota","code":"insufficient_quota","type":"invalid_request_error"}}`,
			wantMsg:  "insufficient quota",
			wantCode: "insufficient_quota",
			wantType: "invalid_request_error",
		},
		{
			name:     "OpenAI 格式 (error.type 补充 code)",
			body:     `{"error":{"message":"balance low","type":"insufficient_balance"}}`,
			wantMsg:  "balance low",
			wantCode: "insufficient_balance", // code为空时，type补充
			wantType: "insufficient_balance",
		},
		{
			name:     "简化格式",
			body:     `{"error":"something went wrong"}`,
			wantMsg:  "something went wrong",
			wantCode: "",
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, code, errType := extractErrorMessage([]byte(tt.body))
			if msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tt.wantMsg)
			}
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
			if errType != tt.wantType {
				t.Errorf("errType = %q, want %q", errType, tt.wantType)
			}
		})
	}
}

// --- ShouldRetrySameProvider 测试 ---

func TestShouldRetrySameProvider(t *testing.T) {
	tests := []struct {
		name    string
		errType string
		want    bool
	}{
		{"timeout 可重试", "timeout", true},
		{"rate_limited 短时限流可重试", "rate_limited", true},
		{"quota_exhausted 不可重试", "quota_exhausted", false},
		{"unknown 直接降级不重试同Provider", "unknown", false},
		{"空 直接降级不重试同Provider", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldRetrySameProvider(UpstreamErrorDetail{Type: UpstreamErrorType(tt.errType)})
			if got != tt.want {
				t.Errorf("ShouldRetrySameProvider(%q) = %v, want %v", tt.errType, got, tt.want)
			}
		})
	}
}

// --- IncreaseMaxTokens 测试 ---

func TestIncreaseMaxTokens(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMax float64
	}{
		{
			name:    "无 max_tokens 字段 → 设为 16384",
			body:    `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`,
			wantMax: 16384,
		},
		{
			name:    "有 max_tokens=1024 → 翻倍到 2048",
			body:    `{"model":"gpt-4","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`,
			wantMax: 2048,
		},
		{
			name:    "max_tokens=20000 → 翻倍超限 → 上限 32768",
			body:    `{"model":"gpt-4","max_tokens":20000,"messages":[{"role":"user","content":"hello"}]}`,
			wantMax: 32768,
		},
		{
			name:    "max_tokens=16384 → 翻倍到 32768",
			body:    `{"model":"gpt-4","max_tokens":16384,"messages":[{"role":"user","content":"hello"}]}`,
			wantMax: 32768,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IncreaseMaxTokens([]byte(tt.body))
			var req map[string]interface{}
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("result is not valid JSON: %v", err)
			}
			gotMax, ok := req["max_tokens"].(float64)
			if !ok {
				t.Fatalf("max_tokens not found or not a number")
			}
			if gotMax != tt.wantMax {
				t.Errorf("max_tokens = %v, want %v", gotMax, tt.wantMax)
			}
		})
	}
}

// --- 截断相关 ShouldFailover 测试 ---

func TestShouldFailoverTruncated(t *testing.T) {
	result := &ProxyResult{
		StatusCode: 200,
		Truncated:  true,
	}
	shouldFail, reason := ShouldFailover(result, 1, "gpt-4", []byte(`{}`))
	if !shouldFail {
		t.Error("ShouldFailover(Truncated) = false, want true")
	}
	if reason != "response_truncated" {
		t.Errorf("reason = %q, want %q", reason, "response_truncated")
	}
}

func TestIsTruncatedRetry(t *testing.T) {
	if !IsTruncatedRetry("response_truncated") {
		t.Error("IsTruncatedRetry('response_truncated') = false, want true")
	}
	if IsTruncatedRetry("quota_exhausted") {
		t.Error("IsTruncatedRetry('quota_exhausted') = true, want false")
	}
}

func TestShouldRetrySameProviderTruncated(t *testing.T) {
	errDetail := UpstreamErrorDetail{Type: UpstreamErrTruncated}
	if !ShouldRetrySameProvider(errDetail) {
		t.Error("ShouldRetrySameProvider(truncated) = false, want true")
	}
}
