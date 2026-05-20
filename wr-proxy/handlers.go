package main

// HTTP 处理函数：/v1/* 代理入口

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
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

	// 多媒体端点
	mux.HandleFunc("/v1/audio/speech", handleAudioSpeech)
	mux.HandleFunc("/v1/audio/transcriptions", handleAudioTranscription)
	mux.HandleFunc("/v1/images/edits", handleImageEdits)
	mux.HandleFunc("/v1/images/variations", handleImageVariations)
	mux.HandleFunc("/v1/models", handleModels)

	// 健康检查
	mux.HandleFunc("/health", handleHealth)

	// 管理接口（Flask 调用）
	mux.HandleFunc("/admin/reload", handleReload)
	mux.HandleFunc("/admin/reload_pricing", handleReloadPricing)
	mux.HandleFunc("/admin/reload_model_grades", handleReloadModelGrades)
	mux.HandleFunc("/admin/model_aliases", handleModelAliases)
	mux.HandleFunc("/admin/stats", handleStats)
	mux.HandleFunc("/admin/cooldowns", handleCooldowns)
	mux.HandleFunc("/admin/clear_cooldown/", handleClearCooldown)
	mux.HandleFunc("/admin/request_cache", handleRequestCache)
	mux.HandleFunc("/admin/session_sticky", handleSessionSticky)
	mux.HandleFunc("/admin/features", handleAdminFeatures)
	mux.HandleFunc("/admin/knowledge_stats", handleKnowledgeStats)
	mux.HandleFunc("/admin/knowledge_prompt_preview", handleKnowledgePromptPreview)

	// MCP 端点（Agent 可连接）
	mux.HandleFunc("/mcp", handleMCP)

	// 知识分析（Flask 调用）
	mux.HandleFunc("/admin/knowledge_analyze", handleKnowledgeAnalyze)
	mux.HandleFunc("/admin/knowledge_analyze/", handleKnowledgeAnalyze)

	// 知识提取（Flask 调用）
	mux.HandleFunc("/admin/knowledge_extract", handleKnowledgeExtract)

	// Embedding + RAG（Flask 调用）
	mux.HandleFunc("/admin/knowledge_embedding_backfill", handleEmbeddingBackfill)
	mux.HandleFunc("/admin/knowledge_rag_stats", handleRAGStats)
}

// checkProxyEnabled 代理网关总开关检查，关闭时返回 503 + 提示信息
func checkProxyEnabled(w http.ResponseWriter) bool {
	if ProxyEnabled {
		return true
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
		"error": map[string]string{
			"message": "代理网关已关闭，请联系管理员",
			"type":    "service_unavailable",
		},
	})
	return false
}

// handleProxy 代理转发主逻辑
func handleProxy(w http.ResponseWriter, r *http.Request) {
	if !checkProxyEnabled(w) {
		return
	}
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

	// 3.1 模型别名解析（qwen-plus → qwen-plus-2025-07-28）
	if resolved, ok := ResolveModelAlias(model); ok {
		LogInfo("Proxy: model alias resolved: %s → %s", model, resolved)
		model = resolved
	}

	// 3.5 智能模型选择（auto别名 / 自动降级）
	originalModel := model
	smartResult := SmartModelSelect(model, body, token)
	model = smartResult.ResolvedModel

	// Token 模型白名单（用解析后的模型检查）
	if !token.CanUseModel(model) {
		writeJSON(w, 403, map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("model %s is not allowed for this token", model),
				"type":    "permission_error",
			},
		})
		return
	}

	// 3.8 脱敏处理（在 sanitize 之前，确保敏感信息不发送到上游）
	desensitizeResult := DesensitizeRequest(token, body)
	if desensitizeResult.Modified {
		body = desensitizeResult.Body
		LogInfo("Proxy: desensitized request for token=%d: %v", token.ID, desensitizeResult.Redacted)
	}

	// 5. 智能调度 + 转发 + 降级
	endpoint := r.URL.Path
	clientIP := extractClientIP(r)
	reqID := uuid.New().String()
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
	}

	// 5.1 优化特性预处理（动态后置、Token 压缩、会话压缩）
	if transformed, desc := ApplyFeatureTransforms(body, model, token); transformed != nil && desc != "" {
		body = transformed
	}

	// 如果智能选择替换了模型，需要更新 body 中的 model 字段
	if smartResult.Downgraded {
		body = replaceModelInBody(body, originalModel, model)
	}

	// 3.9 知识增强 System Prompt 注入
	if token.KnowledgeCaptureEnabled || token.RAGEnabled || token.SystemPromptKnowledge != "" {
		body = injectKnowledgeSystemPrompt(body, token)
	}

	var excludeIDs []int
	var lastResult *ProxyResult
	var selectedProvider *Provider

	for attempt := 0; attempt <= cfg.MaxFailover; attempt++ {
		// 选 Provider
		provider := router.SelectProvider(model, token, excludeIDs, sessionID)
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

		// 请求格式校验、补全和清洗（根据 Provider 能力）
		sanitizeResult := SanitizeRequest(provider, endpoint, body)
		if !sanitizeResult.Valid {
			writeJSON(w, 400, map[string]interface{}{
				"error": map[string]string{
					"message": sanitizeResult.RejectReason,
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// 记录清洗动作
		if sanitizeResult.Modified {
			LogInfo("Proxy: request sanitized for %s: stripped=%v warnings=%v",
				provider.Name, sanitizeResult.Stripped, sanitizeResult.Warnings)
		}

		// 转发（使用清洗后的 body）
		forwardBody := sanitizeResult.Body
		resp, result := proxySvc.Forward(provider, endpoint, r, forwardBody, model)

		// 使用智能重试引擎判断是否需要 failover
		shouldFail, reason := ShouldFailover(result, token.ID, model, body)

		if shouldFail {
			lastResult = result
			excludeIDs = append(excludeIDs, provider.ID)
			LogInfo("Proxy: %s → %s failed (status=%d, err=%s, upstream=%s, reason=%s), failover attempt %d",
				model, provider.Name, result.StatusCode, result.Error,
				result.UpstreamError.Type, reason, attempt+1)

			if resp != nil {
				resp.Body.Close()
			}

			// 截断特殊处理：同Provider重试 + 增大max_tokens
			if IsTruncatedRetry(reason) {
				// 增大 max_tokens
				retryBody := IncreaseMaxTokens(body)
				LogInfo("Proxy: truncated response, retrying with increased max_tokens for %s → %s",
					model, provider.Name)
				resp, result = proxySvc.Forward(provider, endpoint, r, retryBody, model)
				if !result.Truncated && result.Error == "" {
					// 重试成功
					reqCache.RecordRequestSuccess(token.ID, model, body)
					rlog := BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, false)
					rlog.IsRetry = true // 标记为重试
					meter.RecordRequest(rlog)
					if result.IsStream {
						StreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP, desensitizeResult.Mapping)
					} else {
						NonStreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP, desensitizeResult.Mapping)
					}
					return
				}
				// 增大max_tokens后仍截断，可能是模型本身上下文限制，换Provider
				if resp != nil {
					resp.Body.Close()
				}
				LogInfo("Proxy: still truncated after increasing max_tokens for %s → %s, failover to next provider",
					model, provider.Name)
				continue
			}

			// 冷却机制：长时限流/额度用完 → 标记 Provider 冷却
			if result.UpstreamError.Type == UpstreamErrQuotaExhausted {
				// 额度用完 → 冷却30分钟（等用户充值或换Key）
				cooldown := time.Now().Add(30 * time.Minute)
				provider.CooldownUntil = &cooldown
				LogInfo("Proxy: %s set cooldown 30min (quota exhausted)", provider.Name)
				continue
			}
			if result.UpstreamError.Type == UpstreamErrRateLimited {
				waitSec := ExtractRetryAfter(result.UpstreamError.Message)
				if waitSec > 60 {
					// 长时限流 → 冷却到预计恢复时间（上限2小时）
					cooldownDuration := time.Duration(min(waitSec, 7200)) * time.Second
					cooldown := time.Now().Add(cooldownDuration)
					provider.CooldownUntil = &cooldown
					LogInfo("Proxy: %s set cooldown %dmin (long rate limit: %ds)",
						provider.Name, int(cooldownDuration.Minutes()), waitSec)
					continue
				}
			}

			// 同 Provider 重试（仅限 rate_limited/timeout 等可恢复错误）
			if ShouldRetrySameProvider(result.UpstreamError) {
				for retry := 0; retry < provider.MaxRetries && retry < cfg.MaxRetryCount; retry++ {
					// 限流场景：短暂等待后重试
					if result.UpstreamError.Type == UpstreamErrRateLimited {
						time.Sleep(time.Duration(retry+1) * 500 * time.Millisecond) // 0.5s, 1s 递增
					}
					resp, result = proxySvc.Forward(provider, endpoint, r, forwardBody, model)
					shouldRetry, _ := ShouldFailover(result, token.ID, model, body)
					if !shouldRetry {
						// 重试成功
						reqCache.RecordRequestSuccess(token.ID, model, body)
						rlog := BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, true)
						meter.RecordRequest(rlog)

						if result.IsStream {
							StreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP, desensitizeResult.Mapping)
						} else {
							NonStreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP, desensitizeResult.Mapping)
						}
						return
					}
					if resp != nil {
						resp.Body.Close()
					}
				}
			}

			// 记录失败请求到 Hash 缓存
			reqCache.RecordRequestFailure(token.ID, model, body)
			continue // 降级到下一个 Provider
		}

		// 成功
		reqCache.RecordRequestSuccess(token.ID, model, body)
		rlog := BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, false)
		meter.RecordRequest(rlog)

		if result.IsStream {
			// 流式响应：知识捕获暂不支持流式（后续通过压缩阶段捕获）
			streamResult := StreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP, desensitizeResult.Mapping)
			// 流式完成后检查是否中途被错误中断
			if streamResult.StreamAborted {
				LogWarn("Proxy: stream aborted for %s → %s, upstream=%s, err=%s",
					model, provider.Name, streamResult.UpstreamError.Type, streamResult.Error)
				// 流已开始写入客户端，无法 failover 重发
				// 但可以补录日志和触发告警
				rlog.InputTokens = streamResult.InputTokens
				rlog.OutputTokens = streamResult.OutputTokens
				rlog.LatencyMs = streamResult.LatencyMs
				rlog.ErrorMessage = streamResult.Error
				meter.RecordRequest(rlog)
				reqCache.RecordRequestFailure(token.ID, model, body)
			} else {
				// 流式正常完成，补录 usage
				if streamResult.InputTokens > 0 || streamResult.OutputTokens > 0 {
					rlog.InputTokens = streamResult.InputTokens
					rlog.OutputTokens = streamResult.OutputTokens
					rlog.LatencyMs = streamResult.LatencyMs
					meter.RecordRequest(rlog)
				}
			}
		} else {
			// 非流式响应：捕获响应体用于知识投递
			respBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			// 投递知识（使用脱敏后的数据）
			if readErr == nil && token.KnowledgeCaptureEnabled && knowledgeEnabled {
				DeliverKnowledge(token, reqID, model, endpoint, clientIP,
					extractPrompt(body), extractResponse(respBody), body)
			}

			// 重建响应体供 NonStreamResponse 写入
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			NonStreamResponse(w, resp, reqID, provider, token, model, endpoint, clientIP, desensitizeResult.Mapping)
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
	if !checkProxyEnabled(w) {
		return
	}
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
	// 添加智能模型别名
	models = append(models,
		map[string]string{"id": "auto", "object": "model", "owned_by": "webrouter"},
		map[string]string{"id": "smart", "object": "model", "owned_by": "webrouter"},
	)

	// 添加用户自定义别名
	modelAliasMutex.RLock()
	for alias := range modelAliasMap {
		if !modelSet[alias] {
			models = append(models, map[string]string{
				"id":       alias,
				"object":   "model",
				"owned_by": "webrouter",
			})
		}
	}
	modelAliasMutex.RUnlock()

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

// handleReload 管理接口：重新加载 Provider + 脱敏规则 + 模型分级 + 模型别名 + 厂商测试配置
func handleReload(w http.ResponseWriter, r *http.Request) {
	if err := reloadProviders(); err != nil {
		writeJSON(w, 500, map[string]interface{}{"error": err.Error()})
		return
	}
	// 同时刷新脱敏规则
	if err := LoadDesensitizeRules(); err != nil {
		LogWarn("Reload: failed to reload desensitize rules: %v", err)
	}
	// 同时刷新模型分级
	if err := RefreshModelGrades(); err != nil {
		LogWarn("Reload: failed to reload model grades: %v", err)
	}
	// 同时刷新模型别名
	if err := RefreshModelAliases(); err != nil {
		LogWarn("Reload: failed to reload model aliases: %v", err)
	}
	// 同时刷新厂商测试配置
	ReloadVendorTestConfigs()
	// 同时刷新优化特性开关
	ReloadFeatures()
	writeJSON(w, 200, map[string]interface{}{
		"message":       "Providers, desensitize rules, model grades, aliases, vendor configs, feature toggles and complexity config reloaded",
		"proxy_enabled": ProxyEnabled,
		"count":         len(router.GetProviders()),
		"timestamp":     time.Now().UTC(),
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

// handleReloadModelGrades 管理接口：刷新模型分级缓存
func handleReloadModelGrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}
	if err := RefreshModelGrades(); err != nil {
		writeJSON(w, 500, map[string]interface{}{
			"error":     err.Error(),
			"timestamp": time.Now().UTC(),
		})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"message":   "Model grades reloaded",
		"count":     len(modelGrades),
		"timestamp": time.Now().UTC(),
	})
}

// handleModelAliases 管理接口：查询/添加/删除模型别名
func handleModelAliases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 查询所有别名
		modelAliasMutex.RLock()
		aliases := make([]map[string]string, 0, len(modelAliasMap))
		for alias, target := range modelAliasMap {
			aliases = append(aliases, map[string]string{"alias": alias, "target": target})
		}
		modelAliasMutex.RUnlock()
		writeJSON(w, 200, map[string]interface{}{
			"aliases": aliases,
			"total":   len(aliases),
		})

	case http.MethodPost:
		// 添加别名
		var req struct {
			Alias  string `json:"alias"`
			Target string `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Alias == "" || req.Target == "" {
			writeJSON(w, 400, map[string]string{"error": "alias and target are required"})
			return
		}
		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		_, err := db.Exec(`
			INSERT INTO wr_model_aliases (alias, target, enabled, created_at)
			VALUES (?, ?, 1, ?)
			ON CONFLICT(alias) DO UPDATE SET target = ?, enabled = 1`,
			req.Alias, req.Target, now, req.Target)
		if err != nil {
			writeJSON(w, 500, map[string]interface{}{"error": err.Error()})
			return
		}
		// 同步刷新内存
		RefreshModelAliases()
		writeJSON(w, 200, map[string]interface{}{
			"message":   "Alias added",
			"alias":     req.Alias,
			"target":    req.Target,
			"timestamp": time.Now().UTC(),
		})

	case http.MethodDelete:
		// 删除别名
		var req struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Alias == "" {
			writeJSON(w, 400, map[string]string{"error": "alias is required"})
			return
		}
		_, err := db.Exec(`DELETE FROM wr_model_aliases WHERE alias = ?`, req.Alias)
		if err != nil {
			writeJSON(w, 500, map[string]interface{}{"error": err.Error()})
			return
		}
		RefreshModelAliases()
		writeJSON(w, 200, map[string]interface{}{
			"message":   "Alias deleted",
			"alias":     req.Alias,
			"timestamp": time.Now().UTC(),
		})

	default:
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
	}
}

// handleStats 管理接口：实时统计
func handleStats(w http.ResponseWriter, r *http.Request) {
	providers := router.GetProviders()
	stats := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		count, validCount, tokens, cost := meter.GetProviderMinuteStats(p.ID)
		stat := map[string]interface{}{
			"provider_id":        p.ID,
			"name":               p.Name,
			"status":             p.Status,
			"priority":           p.Priority,
			"last_latency":       p.LastLatencyMs,
			"quota_ratio":        p.QuotaRatio(),
			"supports_tools":     p.SupportsTools,
			"minute_count":       count,
			"minute_valid_count": validCount,
			"minute_tokens":      tokens,
			"minute_cost":        cost,
		}
		// 显示冷却状态
		if p.CooldownUntil != nil && time.Now().Before(*p.CooldownUntil) {
			remaining := time.Until(*p.CooldownUntil)
			stat["cooldown_remaining_sec"] = int(remaining.Seconds())
		}
		stats = append(stats, stat)
	}
	writeJSON(w, 200, map[string]interface{}{
		"providers": stats,
	})
}

// handleCooldowns 管理接口：列出所有冷却中的 Provider
func handleCooldowns(w http.ResponseWriter, r *http.Request) {
	providers := router.GetProviders()
	cooldowns := make([]map[string]interface{}, 0)
	for _, p := range providers {
		if p.CooldownUntil != nil && time.Now().Before(*p.CooldownUntil) {
			remaining := time.Until(*p.CooldownUntil)
			cooldowns = append(cooldowns, map[string]interface{}{
				"provider_id":        p.ID,
				"name":               p.Name,
				"status":             p.Status,
				"cooldown_until":     p.CooldownUntil.UTC().Format(time.RFC3339),
				"cooldown_remaining_sec": int(remaining.Seconds()),
			})
		}
	}
	writeJSON(w, 200, map[string]interface{}{
		"cooldowns": cooldowns,
		"total":     len(cooldowns),
	})
}

// handleClearCooldown 管理接口：手动清除指定 Provider 的冷却状态
func handleClearCooldown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/admin/clear_cooldown/")
	var id int
	fmt.Sscanf(idStr, "%d", &id)
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "Invalid provider_id"})
		return
	}

	providers := router.GetProviders()
	for _, p := range providers {
		if p.ID == id {
			if p.CooldownUntil != nil {
				p.CooldownUntil = nil
				LogInfo("Cooldown cleared for provider %d: %s", id, p.Name)
				writeJSON(w, 200, map[string]interface{}{
					"message":     "Cooldown cleared",
					"provider_id": id,
					"name":        p.Name,
				})
			} else {
				writeJSON(w, 200, map[string]interface{}{
					"message":     "Provider was not in cooldown",
					"provider_id": id,
					"name":        p.Name,
				})
			}
			return
		}
	}

	writeJSON(w, 404, map[string]string{"error": "Provider not found"})
}

// handleRequestCache 管理接口：查询/清理请求 Hash 缓存
func handleRequestCache(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := reqCache.ListEntries()
		writeJSON(w, 200, map[string]interface{}{
			"entries": entries,
			"total":   len(entries),
		})

	case http.MethodDelete:
		n := reqCache.ClearAll()
		writeJSON(w, 200, map[string]interface{}{
			"message":   "Request cache cleared",
			"cleared":   n,
			"timestamp": time.Now().UTC(),
		})

	default:
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
	}
}

// handleSessionSticky 管理接口：查询/清理 Session 粘性路由缓存
func handleSessionSticky(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessionStickyMutex.RLock()
		entries := make([]map[string]interface{}, 0, len(sessionStickyMap))
		for k, v := range sessionStickyMap {
			entries = append(entries, map[string]interface{}{
				"session_id":  k,
				"provider_id": v.ProviderID,
				"provider":    v.ProviderName,
				"last_used":   v.LastUsed.UTC().Format(time.RFC3339),
			})
		}
		sessionStickyMutex.RUnlock()
		writeJSON(w, 200, map[string]interface{}{
			"entries": entries,
			"total":   len(entries),
		})

	case http.MethodDelete:
		sessionStickyMutex.Lock()
		n := len(sessionStickyMap)
		sessionStickyMap = make(map[string]*SessionSticky)
		sessionStickyMutex.Unlock()
		writeJSON(w, 200, map[string]interface{}{
			"message":   "Session sticky cache cleared",
			"cleared":   n,
			"timestamp": time.Now().UTC(),
		})

	default:
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
	}
}

// --- 多媒体 / 二进制快速转发 ---

// isBinaryContentType 判断是否为二进制 Content-Type（跳过 JSON 解析链路）
func isBinaryContentType(ct string) bool {
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "multipart/") ||
		strings.HasPrefix(mediaType, "audio/") ||
		strings.HasPrefix(mediaType, "image/")
}

// modelFromForm 从 multipart 表单提取 model 字段
func modelFromForm(r *http.Request) string {
	if model := r.FormValue("model"); model != "" {
		return model
	}
	// OpenAI 兼容：某些端点用 "Model" 大写
	if model := r.FormValue("Model"); model != "" {
		return model
	}
	return ""
}

// modelFromJSON 从 JSON body 提取 model
func modelFromJSON(body []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	m, _ := obj["model"].(string)
	return m
}

// handleBinaryProxy 二进制/多媒体快速转发
// 跳过 SmartModelSelect、脱敏、sanitize，直接选 Provider 转发
func handleBinaryProxy(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !checkProxyEnabled(w) {
		return
	}
	token, authErr := authenticateRequest(r)
	if authErr != nil {
		writeJSON(w, authErr.StatusCode, map[string]interface{}{
			"error": map[string]string{"message": authErr.Error, "type": "auth_error"},
		})
		return
	}

	ct := r.Header.Get("Content-Type")
	isMultipart := strings.HasPrefix(ct, "multipart/")

	var body []byte
	var model string

	if isMultipart {
		// 解析 multipart 以提取 model，后续还要重读 body
		if err := r.ParseMultipartForm(cfg.MaxBodySize); err != nil {
			writeJSON(w, 400, map[string]interface{}{
				"error": map[string]string{"message": "Failed to parse multipart form"},
			})
			return
		}
		model = modelFromForm(r)
	} else {
		var err error
		body, err = io.ReadAll(io.LimitReader(r.Body, cfg.MaxBodySize))
		if err != nil {
			writeJSON(w, 400, map[string]interface{}{
				"error": map[string]string{"message": "Failed to read request body"},
			})
			return
		}
		model = modelFromJSON(body)
	}

	if model == "" {
		writeJSON(w, 400, map[string]interface{}{
			"error": map[string]string{"message": "model field is required"},
		})
		return
	}

	// Token 模型白名单
	if !token.CanUseModel(model) {
		writeJSON(w, 403, map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("model %s is not allowed for this token", model),
				"type":    "permission_error",
			},
		})
		return
	}

	clientIP := extractClientIP(r)
	reqID := uuid.New().String()
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
	}

	var excludeIDs []int
	var lastResult *ProxyResult
	var selectedProvider *Provider

	for attempt := 0; attempt <= cfg.MaxFailover; attempt++ {
		provider := router.SelectProvider(model, token, excludeIDs, sessionID)
		if provider == nil {
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

		var upstreamBody []byte
		var isBodyRebuilt bool

		if isMultipart {
			// 从 r.MultipartForm 重建 multipart body（r.FormValue 已消费原始 body）
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			// 先写入非文件的 form fields
			for key, values := range r.MultipartForm.Value {
				for _, v := range values {
					fw, _ := writer.CreateFormField(key)
					fw.Write([]byte(v))
				}
			}
			// 再写入文件
			for _, fileHeaders := range r.MultipartForm.File {
				for _, fh := range fileHeaders {
					fw, _ := writer.CreateFormFile(fh.Filename, fh.Filename)
					f, _ := fh.Open()
					io.Copy(fw, f)
					f.Close()
				}
			}
			writer.Close()
			upstreamBody = buf.Bytes()
			isBodyRebuilt = true
			// 用重建的 Content-Type（含 boundary）
			ct = writer.FormDataContentType()
		} else {
			upstreamBody = body
		}

		// 转发（跳过 sanitize，直接 Forward）
		resp, result := proxySvc.ForwardBinary(provider, endpoint, r, upstreamBody, model, ct, isBodyRebuilt)

		// 错误判定
		if resp == nil || resp.StatusCode >= 400 {
			lastResult = result
			excludeIDs = append(excludeIDs, provider.ID)
			LogInfo("BinaryProxy: %s → %s failed (status=%d, err=%s), attempt %d",
				model, provider.Name, result.StatusCode, result.Error, attempt+1)

			// 语义错误检测（200 但 body 含错误）
			if result.Error != "" {
				shouldFail, _ := ShouldFailover(result, token.ID, model, body)
				if !shouldFail {
					// 非可 failover 错误，直接返回
					writeBinaryResponse(w, resp, result, model, provider, reqID, token, endpoint, clientIP)
					return
				}
			}

			if resp != nil {
				resp.Body.Close()
			}

			// 冷却机制
			if result.UpstreamError.Type == UpstreamErrQuotaExhausted {
				cooldown := time.Now().Add(30 * time.Minute)
				provider.CooldownUntil = &cooldown
				LogInfo("BinaryProxy: %s set cooldown 30min (quota exhausted)", provider.Name)
				continue
			}
			if result.UpstreamError.Type == UpstreamErrRateLimited {
				waitSec := ExtractRetryAfter(result.UpstreamError.Message)
				if waitSec > 60 {
					cooldownDuration := time.Duration(min(waitSec, 7200)) * time.Second
					cooldown := time.Now().Add(cooldownDuration)
					provider.CooldownUntil = &cooldown
					LogInfo("BinaryProxy: %s set cooldown %dmin (long rate limit: %ds)",
						provider.Name, int(cooldownDuration.Minutes()), waitSec)
					continue
				}
			}
			continue
		}

		// 成功
		writeBinaryResponse(w, resp, result, model, provider, reqID, token, endpoint, clientIP)
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

// writeBinaryResponse 写入二进制响应（透传 Content-Type，不做 JSON 解析）
func writeBinaryResponse(w http.ResponseWriter, resp *http.Response, result *ProxyResult,
	model string, provider *Provider, reqID string, token *Token, endpoint, clientIP string) {

	// 记录日志
	rlog := BuildRequestLog(reqID, token, provider, model, endpoint, clientIP, result, false)
	meter.RecordRequest(rlog)

	// 透传响应头（特别是 Content-Type）
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// 对于音频端点，响应体是二进制流，直接透传
	if result.StatusCode == 200 {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	} else {
		// 错误响应：尝试读取并转发
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
	resp.Body.Close()
}

// handleAudioSpeech 音频合成：POST JSON → 二进制音频
// body: {"model":"tts-1","input":"hello","voice":"alloy"}
func handleAudioSpeech(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	handleBinaryProxy(w, r, "/v1/audio/speech")
}

// handleAudioTranscription 音频转文字：POST multipart → JSON 文本
// form: file(audio)+model+language(optional)
func handleAudioTranscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	handleBinaryProxy(w, r, "/v1/audio/transcriptions")
}

// handleImageEdits 图片编辑：POST multipart → JSON
// form: image+prompt+model(optional)
func handleImageEdits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	handleBinaryProxy(w, r, "/v1/images/edits")
}

// handleImageVariations 图片变体：POST multipart → JSON
// form: image+model
func handleImageVariations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	handleBinaryProxy(w, r, "/v1/images/variations")
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
		LogError("LoadTokenByKey failed: key=%s err=%v", tokenKey[:min(10, len(tokenKey))]+"...", err)
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
