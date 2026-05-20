package main

import (
	"fmt"
	"os"
	"time"
)

// Config 全局配置
type Config struct {
	// 服务
	ListenAddr string // 监听地址，默认 :5051
	DBPath     string // SQLite 数据库路径
	FlaskURL   string // Flask 管理后台地址，用于读取 Provider/Token

	// 代理
	DefaultTimeout  time.Duration // 请求超时，默认 60s
	MaxRetryCount   int           // 同 Provider 最大重试，默认 2
	MaxFailover     int           // 最大降级次数，默认 3
	IdleConnTimeout time.Duration // 连接池空闲超时，默认 90s
	MaxIdleConns    int           // 连接池最大连接数，默认 100
	MaxBodySize     int64         // 非流式请求最大 body，默认 10MB

	// 调度
	RoutingStrategy string // smart/priority/round_robin/least_latency/cost_first

	// 额度
	QuotaWarnThreshold    float64 // 额度预警阈值，默认 0.2 (20%)
	QuotaCriticalThreshold float64 // 额度紧急阈值，默认 0.05 (5%)
	PredictionDays        int     // 预测用近N天数据，默认 7

	// 健康检测
	HealthCheckInterval time.Duration // 检测间隔，默认 5min
	HealthTimeout       time.Duration // 单次检测超时，默认 15s

	// 告警
	AlertCooldown time.Duration // 同一告警冷却时间，默认 5min

	// 知识捕获
	KnowledgeCapture bool // 是否开启知识捕获
}

func LoadConfig() *Config {
	c := &Config{
		ListenAddr:            ":5051",
		DBPath:               envStr("WR_DB_PATH", "data/webrouter.db"),
		FlaskURL:             envStr("WR_FLASK_URL", "http://localhost:5050"),
		DefaultTimeout:        envDuration("WR_TIMEOUT", 60*time.Second),
		MaxRetryCount:         envInt("WR_MAX_RETRY", 2),
		MaxFailover:           envInt("WR_MAX_FAILOVER", 3),
		IdleConnTimeout:       envDuration("WR_IDLE_TIMEOUT", 90*time.Second),
		MaxIdleConns:          envInt("WR_MAX_CONNS", 100),
		MaxBodySize:           10 * 1024 * 1024,
		RoutingStrategy:       envStr("WR_ROUTING", "smart"),
		QuotaWarnThreshold:    envFloat("WR_QUOTA_WARN", 0.2),
		QuotaCriticalThreshold: envFloat("WR_QUOTA_CRIT", 0.05),
		PredictionDays:        envInt("WR_PREDICT_DAYS", 7),
		HealthCheckInterval:   envDuration("WR_HEALTH_INTERVAL", 5*time.Minute),
		HealthTimeout:         envDuration("WR_HEALTH_TIMEOUT", 15*time.Second),
		AlertCooldown:         envDuration("WR_ALERT_COOLDOWN", 5*time.Minute),

		KnowledgeCapture:      envStr("WR_KNOWLEDGE_CAPTURE", "0") == "1",
	}
	return c
}

// --- 环境变量辅助 ---

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	// 支持 "60s", "5m", "1h" 等格式
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return fallback
}
