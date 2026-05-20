package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// 对话摘要压缩 — 超窗口时自动压缩早期对话
// ============================================================

// CompressConfig 对话压缩配置
type CompressConfig struct {
	MaxMessages    int    // 最大消息数，超过此数触发压缩
	MaxTokens      int    // 最大 token 数
	KeepLast       int    // 保留最近不压缩的消息数
	CompressionModel string // 压缩使用的模型
}

var compressConfig = CompressConfig{
	MaxMessages:      30,
	MaxTokens:        15000,
	KeepLast:         10,
	CompressionModel: "qwen3-coder-flash",
}

// ConversationSummary 对话摘要结构
type ConversationSummary struct {
	Summary    string `json:"summary"`
	KeyFacts   string `json:"key_facts"`
	UserGoals  string `json:"user_goals"`
	Decision   string `json:"decisions"`
	Compressed string `json:"compressed"` // 压缩后的完整上下文
}

// CompressConversation 压缩对话历史
// 将早期对话摘要化，保留最近的 keepLast 条消息完整
func CompressConversation(messages []map[string]interface{}) (compressed []map[string]interface{}, summary ConversationSummary, err error) {
	if len(messages) <= compressConfig.KeepLast {
		return messages, summary, nil
	}

	// 提取要压缩的部分
	toCompress := messages[:len(messages)-compressConfig.KeepLast]
	toKeep := messages[len(messages)-compressConfig.KeepLast:]

	// 构建压缩 prompt
	compressPrompt := buildCompressionPrompt(toCompress)

	// 调用 LLM 压缩
	compressedText, err := callCompressLLM(compressPrompt)
	if err != nil {
		// 压缩失败，直接保留最近消息
		return toKeep, summary, nil
	}

	// 解析 LLM 返回的摘要
	summary = parseCompressResponse(compressedText)

	// 构建压缩后的消息列表
	var result []map[string]interface{}

	// 插入摘要消息
	if summary.Summary != "" {
		result = append(result, map[string]interface{}{
			"role":    "system",
			"content": "【对话摘要】以下是之前对话的摘要：\n" + summary.Compressed,
		})
	}

	// 追加保留的最近消息
	result = append(result, toKeep...)

	return result, summary, nil
}

// buildCompressionPrompt 构建压缩 prompt
func buildCompressionPrompt(messages []map[string]interface{}) string {
	var text string
	for _, m := range messages {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "system" {
			continue // 跳过 system 消息
		}
		text += fmt.Sprintf("[%s]: %s\n\n", role, content)
	}

	return fmt.Sprintf(`请对以下对话历史进行压缩摘要。输出严格 JSON 格式：
{"summary": "整体摘要（100字以内）", "key_facts": "提取的关键事实（每条一行）", "decisions": "达成的决策或结论", "compressed": "压缩后的对话摘要文本，可以传递给后续对话使用（300字以内）"}

对话历史：
%s`, truncate(text, 8000))
}

// callCompressLLM 调用 LLM 进行对话压缩
func callCompressLLM(prompt string) (string, error) {
	providers := router.GetProviders()
	if len(providers) == 0 {
		return "", fmt.Errorf("no available provider")
	}

	provider := providers[0]

	body := map[string]interface{}{
		"model": compressConfig.CompressionModel,
		"messages": []map[string]interface{}{
			{"role": "system", "content": "你是一个对话摘要专家。请严格按 JSON 格式输出。"},
			{"role": "user", "content": prompt},
		},
		"max_tokens":    2000,
		"temperature":   0.1,
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", provider.BaseURL+"/v1/chat/completions",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", provider.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return "", fmt.Errorf("parse LLM: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("empty LLM response")
	}

	return llmResp.Choices[0].Message.Content, nil
}

// parseCompressResponse 解析压缩响应
func parseCompressResponse(response string) ConversationSummary {
	jsonStr := extractJSONFromText(response)
	if jsonStr == "" {
		return ConversationSummary{Compressed: response}
	}

	var result ConversationSummary
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ConversationSummary{Compressed: response}
	}

	return result
}

// needsCompression 检查是否需要压缩
func needsCompression(messages []map[string]interface{}) bool {
	if len(messages) > compressConfig.MaxMessages {
		return true
	}

	// 粗略估算 token 数
	totalTokens := 0
	for _, m := range messages {
		content, _ := m["content"].(string)
		totalTokens += len(content) / 4 // 粗略估算
	}
	return totalTokens > compressConfig.MaxTokens
}

// handleConversationCompress 手动触发对话压缩 API
func handleConversationCompress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}

	compressed, summary, err := CompressConversation(req.Messages)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"compressed":    compressed,
		"original_count": len(req.Messages),
		"new_count":      len(compressed),
		"summary":        summary,
	})
}
