// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ============================================================
// Anthropic 流式 SSE 转换 — OpenAI SSE → Anthropic SSE
// ============================================================

// StreamAnthropicResponse 读取上游 OpenAI SSE 流，实时转换为 Anthropic SSE 格式
func StreamAnthropicResponse(w http.ResponseWriter, upstreamResp *http.Response, model string) error {
	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	reader := bufio.NewReader(upstreamResp.Body)
	defer upstreamResp.Body.Close()

	// 发送 content_block_start（首个）
	sendAnthropicEvent(w, "content_block_start", map[string]interface{}{
		"type":  "text",
		"index": 0,
		"text":  "",
	})
	flusher.Flush()

	var fullText strings.Builder
	var inputTokens, outputTokens int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read upstream SSE: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 提取 usage（通常在最后一个 chunk）
		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullText.WriteString(choice.Delta.Content)

				sendAnthropicEvent(w, "content_block_delta", map[string]interface{}{
					"type":  "text_delta",
					"index": 0,
					"delta": map[string]interface{}{
						"text": choice.Delta.Content,
					},
				})
				flusher.Flush()
			}

			// finish
			if choice.FinishReason != nil {
				stopReason := mapFinishReason(*choice.FinishReason)

				sendAnthropicEvent(w, "content_block_stop", map[string]interface{}{
					"index": 0,
				})
				flusher.Flush()

				sendAnthropicEvent(w, "message_delta", map[string]interface{}{
					"delta": map[string]interface{}{
						"stop_reason":   stopReason,
						"stop_sequence": nil,
					},
					"usage": map[string]interface{}{
						"input_tokens":  inputTokens,
						"output_tokens": outputTokens,
					},
				})
				flusher.Flush()

				sendAnthropicEvent(w, "message_stop", map[string]interface{}{})
				flusher.Flush()
			}
		}
	}

	return nil
}

// sendAnthropicEvent 发送 Anthropic 格式的 SSE 事件
func sendAnthropicEvent(w io.Writer, event string, data map[string]interface{}) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b))
}
