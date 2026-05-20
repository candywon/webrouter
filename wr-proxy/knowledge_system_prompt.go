package main

import (
	"bytes"
	"encoding/json"
	"io"
)

// injectKnowledgeSystemPrompt 在请求体中注入知识增强 System Prompt
// 当 Token 开启了 knowledge_capture_enabled 或 rag_enabled 时生效
// 注入内容：部门标识 + RAG上下文（如果启用）+ 自定义知识提示词
func injectKnowledgeSystemPrompt(body []byte, token *Token) []byte {
	if token == nil {
		return body
	}

	// 检查是否需要注入
	if !token.KnowledgeCaptureEnabled && !token.RAGEnabled {
		return body
	}

	// 构造知识增强提示词
	var parts []string

	// 1. 部门标识
	if token.KnowledgeDepartment != "" {
		parts = append(parts, "【部门标识】你正在为 "+token.KnowledgeDepartment+" 提供服务。")
	}

	// 2. RAG 上下文
	if token.RAGEnabled {
		ragCtx, err := buildRAGContext(body, token)
		if err != nil {
			LogWarn("[RAG] build context failed: %v", err)
		}
		if ragCtx != "" {
			parts = append(parts, ragCtx)
		}
	}

	// 3. 自定义知识提示词
	if token.SystemPromptKnowledge != "" {
		parts = append(parts, "【知识提示】"+token.SystemPromptKnowledge)
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
// 结合公司信息 + 部门职能 + 自定义提示词
func buildKnowledgeSystemPrompt(token *Token) string {
	var parts []string

	// 基础标识
	parts = append(parts, "你是一个专业的企业级AI助手。")

	if token.KnowledgeDepartment != "" {
		parts = append(parts, "当前服务对象来自部门："+token.KnowledgeDepartment+"。")
		parts = append(parts, "请根据该部门的专业领域特点，提供针对性的回答。")
	}

	if token.SystemPromptKnowledge != "" {
		parts = append(parts, token.SystemPromptKnowledge)
	}

	return "\n" + joinStrings(parts, "\n") + "\n"
}

// GetKnowledgeSystemPrompt 获取 Token 的知识增强 System Prompt 内容
// 用于 Flask 后台预览/编辑时实时查看效果
func GetKnowledgeSystemPrompt(token *Token) string {
	if token == nil {
		return ""
	}
	if !token.KnowledgeCaptureEnabled && !token.RAGEnabled && token.SystemPromptKnowledge == "" {
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
