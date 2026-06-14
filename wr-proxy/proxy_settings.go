// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"sync"
	"time"
)

// 动态代理设置：reload 时从 DB 覆盖，env 仅作为初始 fallback。
// 这样 admin 后台改 routing_strategy / default_timeout / max_failover / max_retry_count
// 后点 Reload 即生效，不需要重启进程。

var (
	proxySettingsMu sync.RWMutex
	dynRoutingStrategy string
	dynDefaultTimeout  time.Duration
	dynMaxFailover     int
	dynMaxRetryCount   int
)

// InitProxySettings 启动时调用：用 cfg 中的 env 默认值初始化，再尝试用 DB 覆盖。
func InitProxySettings() {
	proxySettingsMu.Lock()
	dynRoutingStrategy = cfg.RoutingStrategy
	dynDefaultTimeout = cfg.DefaultTimeout
	dynMaxFailover = cfg.MaxFailover
	dynMaxRetryCount = cfg.MaxRetryCount
	proxySettingsMu.Unlock()
	LoadProxySettings()
}

// LoadProxySettings 从 DB 加载动态字段，未配置则保留当前值（即 env/默认值）。
// 由 handleReload 调用以达到"无重启生效"。
func LoadProxySettings() {
	proxySettingsMu.Lock()
	defer proxySettingsMu.Unlock()

	if v := LoadSetting("routing_strategy", nil); v != nil {
		if s, ok := v.(string); ok && s != "" {
			dynRoutingStrategy = s
		}
	}
	if v := LoadSetting("default_timeout", nil); v != nil {
		if n, ok := toInt(v); ok && n > 0 {
			dynDefaultTimeout = time.Duration(n) * time.Second
		}
	}
	if v := LoadSetting("max_failover", nil); v != nil {
		if n, ok := toInt(v); ok && n >= 0 {
			dynMaxFailover = n
		}
	}
	if v := LoadSetting("max_retry_count", nil); v != nil {
		if n, ok := toInt(v); ok && n >= 0 {
			dynMaxRetryCount = n
		}
	}

	// 同步到 router
	router.SetStrategy(dynRoutingStrategy)

	LogInfo("LoadProxySettings: strategy=%s, default_timeout=%s, max_failover=%d, max_retry_count=%d",
		dynRoutingStrategy, dynDefaultTimeout, dynMaxFailover, dynMaxRetryCount)
}

func GetRoutingStrategy() string {
	proxySettingsMu.RLock()
	defer proxySettingsMu.RUnlock()
	return dynRoutingStrategy
}

func GetDefaultTimeout() time.Duration {
	proxySettingsMu.RLock()
	defer proxySettingsMu.RUnlock()
	return dynDefaultTimeout
}

func GetMaxFailover() int {
	proxySettingsMu.RLock()
	defer proxySettingsMu.RUnlock()
	return dynMaxFailover
}

func GetMaxRetryCount() int {
	proxySettingsMu.RLock()
	defer proxySettingsMu.RUnlock()
	return dynMaxRetryCount
}

// toInt 兼容 LoadSetting 可能返回的几种数字类型。
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
