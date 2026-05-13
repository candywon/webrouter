package main

// HTTP 处理函数：/v1/* 代理入口

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RegisterHandlers 注册所有 HTTP 路由
func RegisterHandlers(mux *http.ServeMux) {
	// OpenAI 兼容 API
	mux.HandleFunc("/v1/chat/completions", handleProxy)
	mux.HandleFunc("/v1/completions", handleProxy)
	mux.HandleFunc("/v1/embeddings", handleProxy)
	mux.HandleFunc("/v1/images/generations", handleProxy)
	mux.HandleFunc("/v1/models", handleModels)

	// 健康检查
	mux.HandleFunc("/health", handleHealth)

	// 管理接口（Flask 调用）
	mux.HandleFunc("/admin/reload", handleReload)
	mux.HandleFunc("/admin/reload_pricing", handleReloadPricing)
	mux.HandleFunc("/admin/stats", handleStats)
}

// handleProxy 代理转发主逻辑
func handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 1. 鉴权
	token, authErr := authenticateRequest(r)
	if authErr != nil {
		writeJSON(w, authErr.StatusCode, map[string]interface{}{
			"error": map[string]string{
				"message": authErr.Error,
				"type":    "auth_error",
			},
		})
		return
	}

	// 2. 读取请求体
	body, err := io.ReadAll(io.LimitReader(r.Body, cfg.MaxBodySize))
	if err != nil {
		writeJSON(w, 400, map[string]interface{}{
			"error": map[string]string{"message": "Failed to read request body"},
		})
		return
	}

	// 3. 提取 model
	model := extractModel(body)
	if model == "" {
		writeJSON(w, 400, map[string]interface{}{
			"error": map[string]string{"message": "model field is required"},
		})
		return
	}

	// 4. Token 模型白名单
	if !token.CanUseModel(model) {
		writeJSON(w, 403, map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("model %s is not allowed for this token", model),
				"type":    "permission_error",
			},
		})
		return
	}

	// 5. 智能调度 + 转发 + 降级
	endpoint := r.URL.Path
	clientIP := extractClientIP(r)
	reqID := uuid.New().String()

	var excludeIDs []int
	var lastResult *ProxyResult
	var selectedProvider *Provider

	for attempt := 0; attempt <= cfg.MaxFailover; attempt++ {
		// 选 Provider
		provider := router.SelectProvider(model, token, excludeIDs)
		if provider == nil {
			// 所有 Provider 已排除，记录最终失败日志
			if lastResult != nil {
				rlog := BuildRequestLog(reqID, token, selectedProvider, model, endpoint, clientIP, lastResult, true)
				rlog.StatusCode = lastResult.StatusCode
				rlog.ErrorMessage = lastResult.Error
				meter.RecordRequest(rlog)
			}

			writeJSON(w, 503, map[string]interface{}{
				"error": map[string]string{
					"message": "No available provider for model " + model,
					"type":    "server_error",
				},
			})
			return
		}

		selectedProvider = provider

		// 转发
		resp, result := proxySvc.Forward(provider, endpoint, r, body)

		// 失败且可重试（4xx 认证/权限错误也触发降级）
		if result.StatusCode >= 400 || result.Error != "" {
			lastResult = result
			excludeIDs = append(excludeIDs, provider.ID)
			LogInfo("Proxy: %s → %s failed (status=%d, err=%s), failover attempt %d",
				model, provider.Name, result.StatusCode, result.Error, attempt+1)

			if resp != nil {
				resp.Body.Close()
			}

			// 同 Provider 重试
			for retry := 0; retry < provider.MaxRetries && retry < cfg.MaxRetryCount; retry++ {
				resp, result = proxySvc.Forward(provider, endpoint, r, body)
				if result.StatusCode < 400 && result.Error == "" {
					// 重试成功
					rlog := BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, true)
					meter.RecordRequest(rlog)

					if result.IsStream {
						StreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP)
					} else {
						NonStreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP)
					}
					return
				}
				if resp != nil {
					resp.Body.Close()
				}
			}

			continue // 降级到下一个 Provider
		}

		// 成功
		rlog := BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, false)
		meter.RecordRequest(rlog)

		if result.IsStream {
			streamResult := StreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP)
			// 流式完成后补录 usage
			if streamResult.InputTokens > 0 || streamResult.OutputTokens > 0 {
				rlog.InputTokens = streamResult.InputTokens
				rlog.OutputTokens = streamResult.OutputTokens
				rlog.LatencyMs = streamResult.LatencyMs
				meter.RecordRequest(rlog)
			}
		} else {
			NonStreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP)
		}
		return
	}

	// 所有降级都失败
	if lastResult != nil {
		rlog := BuildRequestLog(reqID, token, selectedProvider, model, endpoint, clientIP, lastResult, true)
		rlog.StatusCode = lastResult.StatusCode
		rlog.ErrorMessage = lastResult.Error
		meter.RecordRequest(rlog)

		writeJSON(w, lastResult.StatusCode, map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("All providers failed. Last error: %s", lastResult.Error),
				"type":    "server_error",
			},
		})
	}
}

// handleModels 聚合模型列表
func handleModels(w http.ResponseWriter, r *http.Request) {
	token, authErr := authenticateRequest(r)
	if authErr != nil {
		writeJSON(w, authErr.StatusCode, map[string]interface{}{
			"error": map[string]string{"message": authErr.Error},
		})
		return
	}

	providers := router.GetProviders()
	modelSet := make(map[string]bool)
	for _, p := range providers {
		if !p.Enabled || !p.ProxyEnabled {
			continue
		}
		if token != nil && !token.CanUseProvider(p.ID) {
			continue
		}
		for _, m := range parseModelsList(p.Models) {
			if token == nil || token.CanUseModel(m) {
				modelSet[m] = true
			}
		}
	}

	models := make([]map[string]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, map[string]string{
			"id":      m,
			"object":  "model",
			"owned_by": "webrouter",
		})
	}

	writeJSON(w, 200, map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"status":  "ok",
		"service": "wr-proxy",
	})
}

// handleReload 管理接口：重新加载 Provider
func handleReload(w http.ResponseWriter, r *http.Request) {
	if err := reloadProviders(); err != nil {
		writeJSON(w, 500, map[string]interface{}{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"message":   "Providers reloaded",
		"count":     len(router.GetProviders()),
		"timestamp": time.Now().UTC(),
	})
}

// handleReloadPricing 管理接口：刷新定价缓存
func handleReloadPricing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}
	if err := RefreshPricing(); err != nil {
		writeJSON(w, 500, map[string]interface{}{
			"error":     err.Error(),
			"timestamp": time.Now().UTC(),
		})
		return
	}
	pricing := GetAllPricing()
	writeJSON(w, 200, map[string]interface{}{
		"message":   "Pricing reloaded",
		"count":     len(pricing),
		"timestamp": time.Now().UTC(),
	})
}

// handleStats 管理接口：实时统计
func handleStats(w http.ResponseWriter, r *http.Request) {
	providers := router.GetProviders()
	stats := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		count, tokens, cost := meter.GetProviderMinuteStats(p.ID)
		stats = append(stats, map[string]interface{}{
			"provider_id":   p.ID,
			"name":          p.Name,
			"status":        p.Status,
			"priority":      p.Priority,
			"last_latency":  p.LastLatencyMs,
			"quota_ratio":   p.QuotaRatio(),
			"minute_count":  count,
			"minute_tokens": tokens,
			"minute_cost":   cost,
		})
	}
	writeJSON(w, 200, map[string]interface{}{
		"providers": stats,
	})
}

// --- 辅助 ---

func authenticateRequest(r *http.Request) (*Token, *AuthResult) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, &AuthResult{Error: "Missing Authorization header", StatusCode: 401}
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, &AuthResult{Error: "Invalid Authorization format", StatusCode: 401}
	}

	tokenKey := strings.TrimSpace(parts[1])
	if tokenKey == "" {
		return nil, &AuthResult{Error: "Empty token", StatusCode: 401}
	}

	token, err := LoadTokenByKey(tokenKey)
	if err != nil {
		return nil, &AuthResult{Error: "Database error", StatusCode: 500}
	}
	if token == nil {
		return nil, &AuthResult{Error: "Invalid API key", StatusCode: 401}
	}

	result := Authenticate(r, token)
	if result.Error != "" {
		return nil, result
	}

	return token, nil
}

func extractModel(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	model, ok := req["model"].(string)
	if !ok {
		return ""
	}
	return model
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func reloadProviders() error {
	providers, err := LoadProviders()
	if err != nil {
		return err
	}
	// 展开 Channel 为独立调度项
	providers = LoadChannels(providers)
	router.RefreshProviders(providers)
	return nil
}
