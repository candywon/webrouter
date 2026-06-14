// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"io"
)

// injectKnowledgeSystemPrompt 在请求体中注入知识增强 System Prompt
// 仅当 Token 开启了 rag_enabled 或配置了 system_prompt_knowledge 时生效
// KnowledgeCaptureEnabled 只控制对话捕获，不触发 prompt 注入
// 部门信息仅用于 RAG 搜索的 domain filter，不注入 prompt 文本
func injectKnowledgeSystemPrompt(body []byte, token *Token) []byte {
	if token == nil {
		return body
	}

	if IsKnowledgePaused() {
		return body
	}

	// 检查是否需要注入（仅 RAG + 自定义知识）
	if !token.RAGEnabled && token.SystemPromptKnowledge == "" {
		return body
	}

	// 构造知识增强提示词
	var parts []string

	// 1. RAG 上下文
	if token.RAGEnabled {
		ragCtx, err := buildRAGContext(body, token)
		if err != nil {
			LogWarn("[RAG] build context failed: %v", err)
		}
		if ragCtx != "" {
			parts = append(parts, ragCtx)
		}
	}

	// 2. 自定义知识提示词（原文注入，不加前缀）
	if token.SystemPromptKnowledge != "" {
		parts = append(parts, token.SystemPromptKnowledge)
	}

	if len(parts) == 0 {
		return body
	}

	knowledgePrompt := "\n" + joinStrings(parts, "\n") + "\n"

	// 解析请求体中的 messages 数组
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return body
	}

	// 检查第一条消息是否已经是 system role
	if firstMsg, ok := messages[0].(map[string]interface{}); ok {
		if role, _ := firstMsg["role"].(string); role == "system" {
			// 追加到已有 system prompt 末尾
			if content, _ := firstMsg["content"].(string); content != "" {
				firstMsg["content"] = content + knowledgePrompt
				updated, _ := json.Marshal(req)
				return updated
			}
		}
	}

	// 在消息数组开头插入 system 消息
	systemMsg := map[string]interface{}{
		"role":    "system",
		"content": knowledgePrompt,
	}
	req["messages"] = append([]interface{}{systemMsg}, messages...)

	updated, _ := json.Marshal(req)
	return updated
}

// joinStrings 简单字符串拼接（避免 import strings）
func joinStrings(parts []string, sep string) string {
	var buf bytes.Buffer
	for i, p := range parts {
		if i > 0 {
			buf.WriteString(sep)
		}
		buf.WriteString(p)
	}
	return buf.String()
}

// buildKnowledgeSystemPrompt 构建完整的知识增强 System Prompt
// 用于后台预览，不含部门标签
func buildKnowledgeSystemPrompt(token *Token) string {
	var parts []string

	if token.RAGEnabled {
		parts = append(parts, "以下回答可参考内部知识库中的相关信息。")
	}

	if token.SystemPromptKnowledge != "" {
		parts = append(parts, token.SystemPromptKnowledge)
	}

	if len(parts) == 0 {
		return ""
	}
	return "\n" + joinStrings(parts, "\n") + "\n"
}

// GetKnowledgeSystemPrompt 获取 Token 的知识增强 System Prompt 内容
// 用于 Flask 后台预览/编辑时实时查看效果
func GetKnowledgeSystemPrompt(token *Token) string {
	if token == nil {
		return ""
	}
	if !token.RAGEnabled && token.SystemPromptKnowledge == "" {
		return ""
	}
	return buildKnowledgeSystemPrompt(token)
}

// injectReader wraps io.Reader to intercept and modify request body
// 用于流式请求的 system prompt 注入（暂不实现，预留接口）
type injectReader struct {
	reader io.Reader
	buf    *bytes.Buffer
}

func (r *injectReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}
