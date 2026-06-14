// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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
	// Anthropic 兼容 API
	mux.HandleFunc("/v1/messages", handleAnthropicMessages)

	// Cohere 兼容 API
	mux.HandleFunc("/v1/chat", handleCohereChat)

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

	// RAG 反馈（Flask 调用）
	mux.HandleFunc("/admin/rag_feedback_submit", handleRAGFeedbackSubmit)
	mux.HandleFunc("/admin/rag_feedback_stats", handleRAGFeedbackStats)

	// 记忆管理（Flask 调用）
	mux.HandleFunc("/admin/memory_list", handleMemoryList)

	// 知识导出（Flask 调用）
	mux.HandleFunc("/admin/knowledge_export", handleKnowledgeExport)

	// 对话压缩（Flask 调用）
	mux.HandleFunc("/admin/conversation_compress", handleConversationCompress)
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

	// 3.05 规范化模型名：业界 LLM 模型名约定全小写，
	// 用户传入大小写混合（如 "DeepSeek-V4-Flash"）时统一归一化以便匹配。
	if normalized := strings.ToLower(model); normalized != model {
		body = replaceModelInBody(body, model, normalized)
		model = normalized
	}

	// 3.1 模型别名解析（qwen-plus → qwen-plus-2025-07-28）
	if resolved, ok := ResolveModelAlias(model); ok {
		LogInfo("Proxy: model alias resolved: %s → %s", model, resolved)
		model = resolved
	}

	// 3.5 智能模型选择（auto别名 / 自动降级）
	originalModel := model
	// 3.4 提前获取 sessionID（智能路由需要）
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
	}
	// 客户端未传 session ID 时，用 token ID 自动生成，使记忆功能对无 header 的客户端也可用
	if sessionID == "" {
		sessionID = fmt.Sprintf("token-%d", token.ID)
	}

	smartResult := SmartModelSelect(model, body, token, sessionID)
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
	sessionID = r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("token-%d", token.ID)
	}

	// 5.1 优化特性预处理（动态后置、Token 压缩、会话压缩）
	if transformed, desc := ApplyFeatureTransforms(body, model, token); transformed != nil && desc != "" {
		body = transformed
	}

	// 如果智能选择替换了模型，需要更新 body 中的 model 字段
	if smartResult.Downgraded {
		body = replaceModelInBody(body, originalModel, model)
	}

	// 3.85 会话记忆召回（在知识注入之前，使召回历史也受 RAG/knowledge 同等处理）
	if token.SessionRecallEnabled {
		if triggered, recallSessID, cleanedBody := detectRecallTrigger(r, body); triggered && recallSessID != "" {
			body = cleanedBody
			body = injectSessionMemory(body, token, recallSessID, clientIP)
		}
	}

	// 3.9 知识增强 System Prompt 注入（仅 RAG + 自定义知识触发，捕获不注入）
	if token.RAGEnabled || token.SystemPromptKnowledge != "" {
		body = injectKnowledgeSystemPrompt(body, token)
	}

	var excludeIDs []int
	var lastResult *ProxyResult
	var selectedProvider *Provider

	// cost_optimal 策略可能已预选 Provider
	var costOptimalProvider *Provider
	if smartResult.PreferredProvider != nil {
		costOptimalProvider = smartResult.PreferredProvider
	}

	// 如果请求带 tools/functions：优先路由到协议匹配的 provider（无翻译开销）
	// 客户端协议判断：X-WR-Anthropic-Native=1 → Anthropic 客户端，否则 OpenAI 客户端
	// 找不到匹配 provider 时，降级到任意可用 provider 走翻译路径（B 翻译层已支持 tools）
	preferFormat := ""
	if requestHasTools(body) {
		if r.Header.Get("X-WR-Anthropic-Native") == "1" {
			preferFormat = "anthropic"
		} else {
			preferFormat = "openai"
		}
	}
	// selectProvider 封装两阶段选择：先按协议匹配，无候选时去掉协议约束再选
	selectProvider := func(excl []int) *Provider {
		if preferFormat != "" {
			if p := router.SelectProviderWithFormat(model, token, excl, sessionID, preferFormat); p != nil {
				return p
			}
		}
		return router.SelectProvider(model, token, excl, sessionID)
	}

	for attempt := 0; attempt <= cfg.MaxFailover; attempt++ {
		// 选 Provider
		var provider *Provider
		if attempt == 0 && costOptimalProvider != nil && !intInSlice(costOptimalProvider.ID, excludeIDs) {
			provider = costOptimalProvider
			// 即便 cost_optimal 预选，也优先选协议匹配的（如果存在）
			if preferFormat != "" && !provider.HasFormat(preferFormat) {
				if p := router.SelectProviderWithFormat(model, token, excludeIDs, sessionID, preferFormat); p != nil {
					provider = p
				}
			}
		} else {
			provider = selectProvider(excludeIDs)
		}
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
		resp, result := proxySvc.ForwardSmart(provider, endpoint, r, forwardBody, model)

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
				resp, result = proxySvc.ForwardSmart(provider, endpoint, r, retryBody, model)
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
				provider.SetModelCooldown(model, cooldown)
				LogInfo("Proxy: %s/%s set model cooldown 30min (quota exhausted)", provider.Name, model)
				continue
			}
			// auth_failed 退避：上游返回 401/403 → 设置指数退避
			if result.StatusCode == 401 || result.StatusCode == 403 {
				provider.SetAuthFailBackoff()
				provider.Status = "auth_failed"
				UpdateProviderStatus(provider.ID, "auth_failed", result.LatencyMs, result.Error)
				LogInfo("Proxy: %s auth_failed, backoff count=%d", provider.Name, provider.AuthFailCount)
				continue
			}
			if result.UpstreamError.Type == UpstreamErrRateLimited {
				waitSec := ExtractRetryAfter(result.UpstreamError.Message)
				if waitSec > 60 {
					// 长时限流 → 冷却到预计恢复时间（上限2小时）
					cooldownDuration := time.Duration(min(waitSec, 7200)) * time.Second
					cooldown := time.Now().Add(cooldownDuration)
					provider.SetModelCooldown(model, cooldown)
					LogInfo("Proxy: %s/%s set model cooldown %dmin (long rate limit: %ds)",
						provider.Name, model, int(cooldownDuration.Minutes()), waitSec)
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
					resp, result = proxySvc.ForwardSmart(provider, endpoint, r, forwardBody, model)
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

		// 原生直通（如 Anthropic 客户端 → Anthropic 上游）：响应不解析，原样转发
		if result.Passthrough {
			passthroughResponse(w, resp)
			return
		}

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
				// 流式知识捕获
				if token.KnowledgeCaptureEnabled && IsKnowledgeEnabled() && streamResult.StreamContent != "" {
					DeliverKnowledge(token, reqID, model, endpoint, clientIP,
						extractPrompt(body), streamResult.StreamContent, body)
				}
				// 流式会话记忆落盘
				if token.SessionRecallEnabled && sessionID != "" && streamResult.StreamContent != "" {
					DeliverSessionMessages(token, sessionID, model, body, streamResult.StreamContent)
				}
			}
		} else {
			// 非流式响应：捕获响应体用于知识投递
			respBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			// 投递知识（使用脱敏后的数据）
			if readErr == nil && token.KnowledgeCaptureEnabled && IsKnowledgeEnabled() {
				DeliverKnowledge(token, reqID, model, endpoint, clientIP,
					extractPrompt(body), extractResponse(respBody), body)
			}
			// 会话记忆落盘
			if readErr == nil && token.SessionRecallEnabled && sessionID != "" {
				DeliverSessionMessages(token, sessionID, model, body, extractResponse(respBody))
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
			"id":       m,
			"object":   "model",
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
	// 审计：配置全量重载（合规要求）
	LogConfigChange("reload_all", 0, map[string]interface{}{
		"provider_count": len(router.GetProviders()),
		"proxy_enabled":  ProxyEnabled,
		"client_ip":      r.RemoteAddr,
	})
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
		if mc := p.ModelCooldownSnapshot(); len(mc) > 0 {
			cooling := make(map[string]int, len(mc))
			for m, t := range mc {
				cooling[m] = int(time.Until(t).Seconds())
			}
			stat["model_cooldown"] = cooling
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
				"provider_id":            p.ID,
				"name":                   p.Name,
				"status":                 p.Status,
				"scope":                  "provider",
				"cooldown_until":         p.CooldownUntil.UTC().Format(time.RFC3339),
				"cooldown_remaining_sec": int(remaining.Seconds()),
			})
		}
		if mc := p.ModelCooldownSnapshot(); len(mc) > 0 {
			for model, until := range mc {
				cooldowns = append(cooldowns, map[string]interface{}{
					"provider_id":            p.ID,
					"name":                   p.Name,
					"status":                 p.Status,
					"scope":                  "model",
					"model":                  model,
					"cooldown_until":         until.UTC().Format(time.RFC3339),
					"cooldown_remaining_sec": int(time.Until(until).Seconds()),
				})
			}
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
			hadProvider := p.CooldownUntil != nil
			hadModel := len(p.ModelCooldownSnapshot()) > 0
			if hadProvider {
				p.CooldownUntil = nil
			}
			if hadModel {
				p.ClearModelCooldown("")
			}
			if hadProvider || hadModel {
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
		strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "video/")
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

	if sessionID == "" {
		sessionID = fmt.Sprintf("token-%d", token.ID)
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
			// 再写入文件（保留原始 field name）
			for fieldName, fileHeaders := range r.MultipartForm.File {
				for _, fh := range fileHeaders {
					fw, _ := writer.CreateFormFile(fieldName, fh.Filename)
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
				provider.SetModelCooldown(model, cooldown)
				LogInfo("BinaryProxy: %s/%s set model cooldown 30min (quota exhausted)", provider.Name, model)
				continue
			}
			if result.UpstreamError.Type == UpstreamErrRateLimited {
				waitSec := ExtractRetryAfter(result.UpstreamError.Message)
				if waitSec > 60 {
					cooldownDuration := time.Duration(min(waitSec, 7200)) * time.Second
					cooldown := time.Now().Add(cooldownDuration)
					provider.SetModelCooldown(model, cooldown)
					LogInfo("BinaryProxy: %s/%s set model cooldown %dmin (long rate limit: %ds)",
						provider.Name, model, int(cooldownDuration.Minutes()), waitSec)
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

// handleAnthropicMessages Anthropic /v1/messages → 复用 handleProxy 流程
// 通过 X-WR-Anthropic-Native: 1 标记，让 ForwardSmart 知道 body 已经是 Anthropic 格式：
//   - 上游 ApiFormat="anthropic": 纯直通（保留 thinking blocks）
//   - 上游 ApiFormat="openai": 在 ForwardSmart 内部翻译 Anthropic→OpenAI 后再转发
func handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"type":"error","error":{"type":"method_not_allowed","message":"Method not allowed"}}`))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"type":"error","error":{"type":"invalid_request","message":"cannot read body"}}`))
		return
	}

	var anthReq struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	json.Unmarshal(body, &anthReq)

	// 复用 handleProxy 的完整流程；body 保持 Anthropic 格式，ForwardSmart 根据上游协议决定是否翻译
	openAIR := r.Clone(r.Context())
	openAIR.Body = io.NopCloser(bytes.NewReader(body))
	openAIR.ContentLength = int64(len(body))
	openAIR.Header.Set("Content-Type", "application/json")
	openAIR.Header.Set("X-WR-Anthropic-Native", "1")
	// Anthropic SDK 用 x-api-key，转为 Authorization: Bearer
	if openAIR.Header.Get("Authorization") == "" {
		if apiKey := openAIR.Header.Get("x-api-key"); apiKey != "" {
			openAIR.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	rec := &anthropicRecorder{w: w, header: make(http.Header), nativeFlag: true}
	handleProxy(rec, openAIR)
	rec.flushToClient(anthReq.Stream, anthReq.Model)
}

// anthropicRecorder 捕获 handleProxy 的输出。响应可能是：
//  - OpenAI 格式（上游为 OpenAI 协议，ForwardSmart 未做响应翻译）→ 需翻译为 Anthropic
//  - Anthropic 格式（上游为 Anthropic 原生直通）→ 直接透传
// 通过 sniff 响应内容判断。
type anthropicRecorder struct {
	w          http.ResponseWriter
	statusCode int
	header     http.Header
	body       bytes.Buffer
	nativeFlag bool // 仅作语义提示，实际格式仍按 sniff 决定
}

func (r *anthropicRecorder) Header() http.Header         { return r.header }
func (r *anthropicRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *anthropicRecorder) WriteHeader(code int)        { r.statusCode = code }
func (r *anthropicRecorder) Flush()                      {}

// isAnthropicResponse 嗅探响应 body 是否已是 Anthropic 格式
func isAnthropicResponse(body []byte, isStream bool) bool {
	if len(body) == 0 {
		return false
	}
	if isStream {
		// Anthropic SSE 第一个事件通常是 "event: message_start"
		head := body
		if len(head) > 256 {
			head = head[:256]
		}
		s := string(head)
		return strings.Contains(s, "event: message_start") ||
			strings.Contains(s, "event: content_block_") ||
			strings.Contains(s, `"type":"message_start"`)
	}
	// 非流式：Anthropic JSON 顶层 type=="message"
	var probe struct {
		Type   string `json:"type"`
		Object string `json:"object"`
	}
	if json.Unmarshal(body, &probe) != nil {
		return false
	}
	if probe.Type == "message" {
		return true
	}
	if strings.HasPrefix(probe.Object, "chat.completion") {
		return false
	}
	return false
}

func (r *anthropicRecorder) flushToClient(isStream bool, model string) {
	respBody := r.body.Bytes()

	if r.statusCode == 0 {
		r.statusCode = 200
	}

	// 上游错误：透传错误体
	if r.statusCode >= 400 {
		for k, vs := range r.header {
			for _, v := range vs {
				r.w.Header().Add(k, v)
			}
		}
		if r.w.Header().Get("Content-Type") == "" {
			r.w.Header().Set("Content-Type", "application/json")
		}
		r.w.WriteHeader(r.statusCode)
		r.w.Write(respBody)
		return
	}

	alreadyAnthropic := isAnthropicResponse(respBody, isStream)

	if isStream {
		if alreadyAnthropic {
			// Anthropic 直通：丢弃 message_stop 之后的重复块（部分上游会重复发送终止三连）
			cleaned := truncateAfterMessageStop(respBody)
			for k, vs := range r.header {
				for _, v := range vs {
					r.w.Header().Add(k, v)
				}
			}
			r.w.Header().Set("Content-Type", "text/event-stream")
			r.w.Header().Set("Cache-Control", "no-cache")
			r.w.Header().Set("Connection", "keep-alive")
			r.w.WriteHeader(r.statusCode)
			r.w.Write(cleaned)
			if f, ok := r.w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}
		// OpenAI SSE → Anthropic SSE
		upstreamResp := &http.Response{
			StatusCode: r.statusCode,
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}
		StreamAnthropicResponse(r.w, upstreamResp, model)
		return
	}

	// 非流式
	for k, vs := range r.header {
		for _, v := range vs {
			r.w.Header().Add(k, v)
		}
	}
	r.w.Header().Set("Content-Type", "application/json")

	if alreadyAnthropic {
		r.w.WriteHeader(r.statusCode)
		r.w.Write(respBody)
		return
	}

	converted, err := TranslateOpenAIToAnthropic(respBody)
	if err != nil {
		LogWarn("[anthropic] response translate failed: %v", err)
		converted = respBody
	}
	r.w.WriteHeader(r.statusCode)
	r.w.Write(converted)
}

// truncateAfterMessageStop 截断 SSE 流：保留到第一个 message_stop 事件块（含其后的空行），
// 丢弃之后的所有内容。避免某些 Anthropic 兼容上游重复发送终止事件。
func truncateAfterMessageStop(body []byte) []byte {
	marker := []byte("event: message_stop")
	idx := bytes.Index(body, marker)
	if idx < 0 {
		return body
	}
	// 从 marker 起向后找下一个 "\n\n"（事件块边界）
	rest := body[idx:]
	endIdx := bytes.Index(rest, []byte("\n\n"))
	if endIdx < 0 {
		return body
	}
	return body[:idx+endIdx+2]
}

// handleCohereChat Cohere /v1/chat → 转换为 OpenAI 格式后转发
func handleCohereChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"cannot read body"}`, http.StatusBadRequest)
		return
	}

	var coReq struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	json.Unmarshal(body, &coReq)
	isStream := coReq.Stream

	// 转换为 OpenAI 格式
	openAIBody, err := TranslateCohereToOpenAI(body)
	if err != nil {
		LogWarn("[cohere] translate failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"translate failed: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	internalReq, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(openAIBody))
	internalReq.Header = r.Header.Clone()
	internalReq.Header.Set("Content-Type", "application/json")

	token, authResult := authenticateRequest(internalReq)
	if authResult != nil && authResult.Error != "" {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, authResult.Error), authResult.StatusCode)
		return
	}

	model := coReq.Model
	if resolved, ok := ResolveModelAlias(model); ok {
		model = resolved
	}

	provider := router.SelectProvider(model, token, nil, "")
	if provider == nil {
		http.Error(w, `{"error":"no available provider"}`, http.StatusServiceUnavailable)
		return
	}

	upstreamURL := provider.BaseURL + "/v1/chat/completions"
	upstreamReq, _ := http.NewRequest("POST", upstreamURL, bytes.NewReader(openAIBody))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+provider.APIKey)

	client := &http.Client{
		Timeout: 180 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:    100,
			IdleConnTimeout: 90 * time.Second,
		},
	}

	upstreamResp, err := client.Do(upstreamReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"upstream request failed: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer upstreamResp.Body.Close()

	if isStream {
		// Cohere 流式：目前退化为非流式
		respBody, _ := io.ReadAll(upstreamResp.Body)
		converted, err := TranslateOpenAIToCohere(respBody)
		if err != nil {
			LogWarn("[cohere] response translate failed: %v", err)
			converted = respBody
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(upstreamResp.StatusCode)
		w.Write(converted)
		return
	}

	respBody, _ := io.ReadAll(upstreamResp.Body)
	converted, err := TranslateOpenAIToCohere(respBody)
	if err != nil {
		LogWarn("[cohere] response translate failed: %v", err)
		converted = respBody
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(upstreamResp.StatusCode)
	w.Write(converted)
}

// extractModel 从请求体中提取 model 字段
func extractModel(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	return req.Model
}

// requestHasTools 检测请求体是否声明了工具（OpenAI 的 "tools"/"functions" 或 Anthropic 的 "tools"）
// 跨协议翻译目前不会传递 tool_use/tool_calls，带 tools 的请求必须路由到同协议 provider。
func requestHasTools(body []byte) bool {
	var probe struct {
		Tools     json.RawMessage `json:"tools"`
		Functions json.RawMessage `json:"functions"`
	}
	if json.Unmarshal(body, &probe) != nil {
		return false
	}
	hasArray := func(raw json.RawMessage) bool {
		if len(raw) == 0 {
			return false
		}
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) != nil {
			return false
		}
		return len(arr) > 0
	}
	return hasArray(probe.Tools) || hasArray(probe.Functions)
}

// passthroughResponse 把上游响应原样写给客户端（包括 SSE 流）
func passthroughResponse(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

// reloadProviders 重新加载 Provider 列表并刷新路由
func reloadProviders() error {
	providers, err := LoadProviders()
	if err != nil {
		return fmt.Errorf("load providers: %w", err)
	}
	router.RefreshProviders(providers)
	LogInfo("Reload: loaded %d providers", len(providers))
	return nil
}
