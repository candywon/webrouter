// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// 请求格式校验、补全与清洗
// 核心职责：
// 1. 校验请求是否符合 OpenAI Chat Completions API 规范
// 2. 补全缺失的必要字段（编程工具发的请求可能缺字段）
// 3. 根据 Provider 能力剥离不支持的字段（如 tools/function_calling）
// 4. 防止因发送不支持的功能字段导致厂商封号或中断服务

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- 请求校验结果 ---

type SanitizeResult struct {
	Body         []byte // 清洗后的请求体（可能被修改）
	Warnings     []string
	Stripped     []string // 被剥离的字段列表
	Modified     bool     // 请求体是否被修改
	Valid        bool     // 请求是否合法
	RejectReason string   // 拒绝原因（空=不拒绝）
}

// SanitizeRequest 校验并清洗请求体
// provider: 目标 Provider（用于能力匹配）
// endpoint: 请求路径（不同 endpoint 有不同校验规则）
// body: 原始请求体
func SanitizeRequest(provider *Provider, endpoint string, body []byte) *SanitizeResult {
	result := &SanitizeResult{
		Valid: true,
	}

	// 非 chat/completions 的请求只做基本校验
	if endpoint != "/v1/chat/completions" {
		result.Body = body
		return result
	}

	// 解析 JSON
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		result.Valid = false
		result.RejectReason = fmt.Sprintf("invalid JSON: %v", err)
		result.Body = body
		return result
	}

	// 1. 必要字段校验
	validateRequiredFields(req, result)

	// 2. messages 结构校验和补全
	sanitizeMessages(req, result)

	// 3. 根据 Provider 能力处理工具调用相关字段
	sanitizeToolFields(provider, req, result)

	// 4. 补全缺失但推荐的字段
	enhanceDefaults(req, result)

	// 序列化回 JSON
	if result.Modified {
		newBody, err := json.Marshal(req)
		if err != nil {
			// 序列化失败，用原始 body
			result.Warnings = append(result.Warnings, "failed to re-serialize sanitized request")
			result.Body = body
		} else {
			result.Body = newBody
		}
	} else {
		result.Body = body
	}

	return result
}

// validateRequiredFields 校验必要字段
func validateRequiredFields(req map[string]interface{}, result *SanitizeResult) {
	// model 是必须的
	if _, ok := req["model"]; !ok {
		result.Valid = false
		result.RejectReason = "model field is required"
		return
	}

	// messages 是必须的
	if _, ok := req["messages"]; !ok {
		result.Valid = false
		result.RejectReason = "messages field is required"
		return
	}

	// messages 必须是数组
	msgs, ok := req["messages"].([]interface{})
	if !ok {
		result.Valid = false
		result.RejectReason = "messages must be an array"
		return
	}

	// messages 不能为空
	if len(msgs) == 0 {
		result.Valid = false
		result.RejectReason = "messages array cannot be empty"
		return
	}
}

// sanitizeMessages 校验和补全 messages 结构
func sanitizeMessages(req map[string]interface{}, result *SanitizeResult) {
	msgs, ok := req["messages"].([]interface{})
	if !ok {
		return
	}

	for i, msg := range msgs {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("messages[%d] is not an object, skipping", i))
			continue
		}

		// 每个 message 必须有 role
		if _, hasRole := msgMap["role"]; !hasRole {
			// 尝试推断 role
			inferred := inferRole(msgMap, i, len(msgs))
			if inferred != "" {
				msgMap["role"] = inferred
				result.Modified = true
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("messages[%d] missing role, inferred as '%s'", i, inferred))
			} else {
				// 无法推断，设为 user
				msgMap["role"] = "user"
				result.Modified = true
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("messages[%d] missing role, defaulted to 'user'", i))
			}
		}

		role, _ := msgMap["role"].(string)

		// 校验 role 值
		switch role {
		case "system", "user", "assistant", "tool", "function":
			// 合法
		default:
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("messages[%d] has unusual role: '%s'", i, role))
		}

		// assistant 消息如果含 tool_calls，但 content 为空，补全 content
		// 某些厂商要求 assistant content 不能为 null（有 tool_calls 时）
		if role == "assistant" {
			if _, hasToolCalls := msgMap["tool_calls"]; hasToolCalls {
				if content, hasContent := msgMap["content"]; !hasContent || content == nil {
					msgMap["content"] = ""
					result.Modified = true
				}
			}
		}

		// tool 角色的消息必须有 tool_call_id
		if role == "tool" {
			if _, hasID := msgMap["tool_call_id"]; !hasID {
				// 缺少 tool_call_id，生成一个占位 ID
				msgMap["tool_call_id"] = fmt.Sprintf("auto_%d", i)
				result.Modified = true
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("messages[%d] tool role missing tool_call_id, generated placeholder", i))
			}
		}

		// function 角色的消息必须有 name
		if role == "function" {
			if _, hasName := msgMap["name"]; !hasName {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("messages[%d] function role missing name field", i))
			}
		}
	}
}

// inferRole 根据上下文推断缺失的 role
func inferRole(msg map[string]interface{}, index, total int) string {
	// 有 tool_calls → assistant
	if _, ok := msg["tool_calls"]; ok {
		return "assistant"
	}
	// 有 tool_call_id → tool
	if _, ok := msg["tool_call_id"]; ok {
		return "tool"
	}
	// 有 function → function
	if _, ok := msg["function"]; ok {
		return "function"
	}
	// 第一条且含系统指令 → system
	if index == 0 {
		if content, ok := msg["content"].(string); ok {
			lower := strings.ToLower(content)
			if strings.Contains(lower, "you are") ||
				strings.Contains(lower, "你是") ||
				strings.Contains(lower, "act as") ||
				strings.Contains(lower, "作为") {
				return "system"
			}
		}
	}
	// 默认无法推断
	return ""
}

// sanitizeToolFields 根据 Provider 能力处理工具调用相关字段
// 关键安全逻辑：不支持工具调用的 Provider，必须剥离 tools/function_calling，
// 否则可能被封号或中断服务
func sanitizeToolFields(provider *Provider, req map[string]interface{}, result *SanitizeResult) {
	if provider == nil || provider.SupportsTools {
		return // Provider 支持工具调用，无需处理
	}

	// 需要剥离的工具相关字段
	toolFields := []string{
		"tools",
		"tool_choice",
		"functions",     // 旧版 function calling（已废弃但某些客户端仍用）
		"function_call", // 旧版
		"parallel_tool_calls",
	}

	stripped := false
	for _, field := range toolFields {
		if _, exists := req[field]; exists {
			delete(req, field)
			result.Stripped = append(result.Stripped, field)
			stripped = true
		}
	}

	if stripped {
		result.Modified = true
		LogInfo("SanitizeRequest: stripped tool fields %v from request to provider %s (does not support tools)",
			result.Stripped, provider.Name)

		// 还需要清理 messages 中的 tool_calls / tool 角色消息
		sanitizeToolMessages(req, result)
	}
}

// sanitizeToolMessages 清理 messages 中与工具调用相关的消息
// 当 Provider 不支持工具调用时：
// - 移除 role=tool 的消息（工具返回结果）
// - 移除 assistant 消息中的 tool_calls 字段
// - 保留 assistant 消息的 content（如有）
func sanitizeToolMessages(req map[string]interface{}, result *SanitizeResult) {
	msgs, ok := req["messages"].([]interface{})
	if !ok {
		return
	}

	var cleaned []interface{}
	removedToolMsgs := 0
	removedToolCalls := 0

	for _, msg := range msgs {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			cleaned = append(cleaned, msg)
			continue
		}

		role, _ := msgMap["role"].(string)

		// 移除 tool 角色的消息
		if role == "tool" {
			removedToolMsgs++
			continue
		}

		// 移除 function 角色的消息
		if role == "function" {
			removedToolMsgs++
			continue
		}

		// assistant 消息：剥离 tool_calls，保留 content
		if role == "assistant" {
			if _, hasTC := msgMap["tool_calls"]; hasTC {
				delete(msgMap, "tool_calls")
				removedToolCalls++
				// 如果 content 为空，设一个占位符避免格式错误
				if content, hasContent := msgMap["content"]; !hasContent || content == nil {
					msgMap["content"] = ""
				}
				// 也清除 function_call（旧版）
				delete(msgMap, "function_call")
			}
		}

		cleaned = append(cleaned, msgMap)
	}

	if removedToolMsgs > 0 || removedToolCalls > 0 {
		req["messages"] = cleaned
		result.Modified = true
		result.Stripped = append(result.Stripped,
			fmt.Sprintf("tool_messages(%d)", removedToolMsgs),
			fmt.Sprintf("tool_calls_in_assistant(%d)", removedToolCalls),
		)
		LogInfo("SanitizeRequest: removed %d tool messages and %d tool_calls for provider %s",
			removedToolMsgs, removedToolCalls, req)
	}
}

// enhanceDefaults 补全缺失但推荐的字段
func enhanceDefaults(req map[string]interface{}, result *SanitizeResult) {
	// stream 默认 false（某些客户端不传此字段）
	if _, ok := req["stream"]; !ok {
		// 不强制设置，让上游用默认值
		// 但记录一下
		result.Warnings = append(result.Warnings, "stream field not set, using upstream default")
	}

	// temperature 合法范围校验（不修改，仅警告）
	if temp, ok := req["temperature"].(float64); ok {
		if temp < 0 || temp > 2 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("temperature %.2f out of typical range [0, 2]", temp))
		}
	}

	// max_tokens 合法范围校验
	if mt, ok := req["max_tokens"].(float64); ok {
		if mt <= 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("max_tokens %.0f should be positive", mt))
		}
	}

	// n 合法范围校验
	if n, ok := req["n"].(float64); ok {
		if n <= 0 || n > 10 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("n %.0f out of typical range [1, 10]", n))
		}
	}
}

// HasToolCallRequest 检测请求是否包含工具调用相关字段
// 用于日志和告警
func HasToolCallRequest(body []byte) bool {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}

	// 顶层字段
	toolFields := []string{"tools", "tool_choice", "functions", "function_call"}
	for _, f := range toolFields {
		if _, ok := req[f]; ok {
			return true
		}
	}

	// messages 中的 tool_calls
	if msgs, ok := req["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if _, ok := msgMap["tool_calls"]; ok {
					return true
				}
				if role, _ := msgMap["role"].(string); role == "tool" || role == "function" {
					return true
				}
			}
		}
	}

	return false
}
