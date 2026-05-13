package main

// 鉴权模块：Token 验证、过期检查、配额检查、限速

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter 内存 RPM 限速器
type RateLimiter struct {
	mu       sync.Mutex
	counters map[int]*rateCounter
}

type rateCounter struct {
	count    int
	windowStart time.Time
}

var limiter = &RateLimiter{
	counters: make(map[int]*rateCounter),
}

// CheckRateLimit 检查 Token RPM 限速，返回是否允许
func (rl *RateLimiter) CheckRateLimit(tokenID int, rpm int) bool {
	if rpm <= 0 {
		return true // 不限速
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	c, ok := rl.counters[tokenID]
	if !ok || now.Sub(c.windowStart) >= time.Minute {
		// 新窗口
		rl.counters[tokenID] = &rateCounter{count: 1, windowStart: now}
		return true
	}
	c.count++
	return c.count <= rpm
}

// AuthResult 鉴权结果
type AuthResult struct {
	Token      *Token
	Error      string
	StatusCode int
}

// Authenticate 验证请求的 Token
func Authenticate(r *http.Request, token *Token) *AuthResult {
	// 1. 检查启用
	if !token.Enabled {
		return &AuthResult{
			Error:      "Token 已禁用",
			StatusCode: 401,
		}
	}

	// 2. 检查过期
	if token.IsExpired() {
		return &AuthResult{
			Error:      "Token 已过期",
			StatusCode: 401,
		}
	}

	// 3. 检查配额
	if token.QuotaTotal > 0 && token.QuotaUsed >= token.QuotaTotal {
		return &AuthResult{
			Error:      "Token 配额已用完，请联系管理员充值",
			StatusCode: 429,
		}
	}

	// 4. 检查 RPM 限速
	if !limiter.CheckRateLimit(token.ID, token.RateLimitRPM) {
		return &AuthResult{
			Error:      "请求频率超限，请稍后再试",
			StatusCode: 429,
		}
	}

	// 5. 检查 IP 白名单
	if token.SubnetWhitelist != "" && token.SubnetWhitelist != "[]" {
		clientIP := extractClientIP(r)
		if !isIPAllowed(clientIP, token.SubnetWhitelist) {
			return &AuthResult{
				Error:      "IP 地址不在白名单中",
				StatusCode: 403,
			}
		}
	}

	return &AuthResult{Token: token}
}

// extractClientIP 提取客户端真实 IP
func extractClientIP(r *http.Request) string {
	// 优先 X-Forwarded-For（Nginx 反代场景）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isIPAllowed 检查 IP 是否在白名单中
func isIPAllowed(ip string, whitelistJSON string) bool {
	// 解析白名单列表 ["10.0.0.0/8", "192.168.1.0/24"]
	cidrs := parseJSONArray(whitelistJSON)
	if len(cidrs) == 0 {
		return true
	}

	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	for _, cidr := range cidrs {
		// 支持 CIDR 和 单 IP
		if !strings.Contains(cidr, "/") {
			cidr = cidr + "/32"
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(clientIP) {
			return true
		}
	}
	return false
}
