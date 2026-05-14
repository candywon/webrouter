package main

// 入口：加载配置、初始化、启动 HTTP 服务

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var cfg *Config

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   WebRouter Proxy — wr-proxy         ║")
	fmt.Println("║   AI API 智能网关代理引擎             ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// 1. 加载配置
	cfg = LoadConfig()
	LogInfo("Config: %s", cfg.ListenAddr)

	// 2. 初始化数据库
	if err := InitDB(cfg.DBPath); err != nil {
		LogError("DB init failed: %v", err)
		os.Exit(1)
	}

	// 3. 加载 Provider
	providers, err := LoadProviders()
	if err != nil {
		LogWarn("Load providers failed: %v (will retry)", err)
		providers = []*Provider{} // 空列表，后续健康检测会刷新
	}
	router.strategy = cfg.RoutingStrategy
	router.RefreshProviders(providers)
	LogInfo("Loaded %d providers", len(providers))

	// 3.5 加载定价表
	if err := RefreshPricing(); err != nil {
		LogWarn("Load pricing failed: %v (using defaults)", err)
	}

	// 3.6 展开 Channel 为独立调度项
	providers = LoadChannels(providers)
	router.RefreshProviders(providers)
	LogInfo("After channel expansion: %d providers", len(providers))

	// 4. 初始化代理服务
	proxySvc = NewProxyService()

	// 4.5 初始化脱敏引擎
	InitBuiltinPatterns()
	if err := LoadDesensitizeRules(); err != nil {
		LogWarn("Failed to load desensitize rules: %v", err)
	}

	// 5. 启动健康检测
	healthChecker := NewHealthChecker(cfg.HealthCheckInterval, cfg.HealthTimeout)
	healthChecker.Start()

	// 6. 启动预警评估（每分钟）
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			events := alertEngine.EvaluateAll()
			if len(events) > 0 {
				NotifyAlerts(events)
			}
		}
	}()

	// 6.5 清理过期请求缓存（每10分钟）
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			reqCache.CleanStale()
		}
	}()

	// 7. 启动 HTTP 服务
	mux := http.NewServeMux()
	RegisterHandlers(mux)

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // 流式请求需要较长超时
		IdleTimeout:  120 * time.Second,
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		LogInfo("Received signal: %v, shutting down...", sig)

		healthChecker.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	fmt.Printf("\n  wr-proxy listening on %s\n", cfg.ListenAddr)
	fmt.Printf("  DB: %s\n", cfg.DBPath)
	fmt.Printf("  Strategy: %s\n", cfg.RoutingStrategy)
	fmt.Printf("  Health check: %s\n", cfg.HealthCheckInterval)
	fmt.Printf("\n  Endpoints:\n")
	fmt.Printf("    POST /v1/chat/completions\n")
	fmt.Printf("    POST /v1/completions\n")
	fmt.Printf("    POST /v1/embeddings\n")
	fmt.Printf("    POST /v1/images/generations\n")
	fmt.Printf("    GET  /v1/models\n")
	fmt.Printf("    GET  /health\n")
	fmt.Printf("    POST /admin/reload\n")
	fmt.Printf("    GET  /admin/stats\n")
	fmt.Println()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		LogError("Server error: %v", err)
		os.Exit(1)
	}
	LogInfo("Server stopped")
}

// corsMiddleware CORS 中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
