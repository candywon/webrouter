// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ============================================================
// SSE 转换 — Anthropic SSE → OpenAI SSE
// 用于 OpenAI 客户端 → Anthropic 上游 的流式响应转换
// ============================================================

// AnthropicSSEToOpenAISSE 读取 Anthropic SSE 字节流，输出 OpenAI 格式 SSE 字节流
// dst 任意 io.Writer（http.ResponseWriter 或 io.Pipe writer 都可）
// 如果 dst 实现了 http.Flusher，会在每条 chunk 后 flush
func AnthropicSSEToOpenAISSE(dst io.Writer, src io.Reader, model string) error {
	type flusher interface{ Flush() }
	doFlush := func() {
		if f, ok := dst.(flusher); ok {
			f.Flush()
		}
	}

	reader := bufio.NewReader(src)

	chatID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()

	var inputTokens, outputTokens int
	var stopReason string
	// tool_use 跟踪：Anthropic content_block index → OpenAI tool_calls 数组 index
	toolCallSlot := map[int]int{} // 只对 tool_use 类型记录
	var nextToolIdx int
	var hasToolCalls bool

	// 通用：发普通 chunk（content / reasoning_content / finish）
	emitChunk := func(deltaContent, deltaReasoning, finishReason string, usage map[string]interface{}, role bool) {
		delta := map[string]interface{}{}
		if role {
			delta["role"] = "assistant"
			delta["content"] = ""
		}
		if deltaContent != "" {
			delta["content"] = deltaContent
		}
		if deltaReasoning != "" {
			delta["reasoning_content"] = deltaReasoning
		}
		choice := map[string]interface{}{
			"index": 0,
			"delta": delta,
		}
		if finishReason != "" {
			choice["finish_reason"] = finishReason
		}
		chunk := map[string]interface{}{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []interface{}{choice},
		}
		if usage != nil {
			chunk["usage"] = usage
		}
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(dst, "data: %s\n\n", string(b))
		doFlush()
	}

	// 发 tool_calls 增量
	emitToolCallChunk := func(toolIdx int, id, name, argsDelta string) {
		fn := map[string]interface{}{}
		if name != "" {
			fn["name"] = name
		}
		// arguments 字段：OpenAI 增量协议每条 chunk 里 arguments 都是一段字符串
		fn["arguments"] = argsDelta
		entry := map[string]interface{}{
			"index":    toolIdx,
			"function": fn,
		}
		if id != "" {
			entry["id"] = id
			entry["type"] = "function"
		}
		chunk := map[string]interface{}{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []interface{}{
				map[string]interface{}{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []interface{}{entry},
					},
				},
			},
		}
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(dst, "data: %s\n\n", string(b))
		doFlush()
	}

	var currentEvent string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read upstream SSE: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		switch currentEvent {
		case "message_start":
			var ev struct {
				Message struct {
					Usage *struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal([]byte(data), &ev) == nil && ev.Message.Usage != nil {
				inputTokens = ev.Message.Usage.InputTokens
				outputTokens = ev.Message.Usage.OutputTokens
			}
			emitChunk("", "", "", nil, true)

		case "content_block_start":
			// 仅关心 tool_use 块的开始，发首条 tool_calls chunk（带 id/name）
			var ev struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block"`
			}
			if json.Unmarshal([]byte(data), &ev) == nil && ev.ContentBlock.Type == "tool_use" {
				slot := nextToolIdx
				nextToolIdx++
				toolCallSlot[ev.Index] = slot
				hasToolCalls = true
				emitToolCallChunk(slot, ev.ContentBlock.ID, ev.ContentBlock.Name, "")
			}

		case "content_block_delta":
			var ev struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					Thinking    string `json:"thinking"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &ev) != nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" {
					emitChunk(ev.Delta.Text, "", "", nil, false)
				}
			case "thinking_delta":
				if ev.Delta.Thinking != "" {
					emitChunk("", ev.Delta.Thinking, "", nil, false)
				}
			case "input_json_delta":
				if slot, ok := toolCallSlot[ev.Index]; ok {
					emitToolCallChunk(slot, "", "", ev.Delta.PartialJSON)
				}
			}

		case "message_delta":
			var ev struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal([]byte(data), &ev) == nil {
				if ev.Delta.StopReason != "" {
					stopReason = ev.Delta.StopReason
				}
				if ev.Usage != nil {
					if ev.Usage.OutputTokens > 0 {
						outputTokens = ev.Usage.OutputTokens
					}
					if ev.Usage.InputTokens > 0 {
						inputTokens = ev.Usage.InputTokens
					}
				}
			}

		case "message_stop":
			usage := map[string]interface{}{
				"prompt_tokens":     inputTokens,
				"completion_tokens": outputTokens,
				"total_tokens":      inputTokens + outputTokens,
			}
			finish := mapStopReasonToFinish(stopReason)
			if hasToolCalls && (finish == "stop" || finish == "") {
				finish = "tool_calls"
			}
			emitChunk("", "", finish, usage, false)
			fmt.Fprintf(dst, "data: [DONE]\n\n")
			doFlush()
			return nil
		}
	}

	if stopReason != "" {
		finish := mapStopReasonToFinish(stopReason)
		if hasToolCalls && (finish == "stop" || finish == "") {
			finish = "tool_calls"
		}
		emitChunk("", "", finish, map[string]interface{}{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		}, false)
	}
	fmt.Fprintf(dst, "data: [DONE]\n\n")
	doFlush()
	return nil
}
