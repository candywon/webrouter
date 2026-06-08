// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ============================================================
// API 协议转换层 — Anthropic/Cohere ↔ OpenAI 格式互转
// ============================================================

// --- Anthropic /v1/messages → OpenAI /v1/chat/completions ---

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []contentBlock
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	System        string             `json:"system,omitempty"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
}

// TranslateAnthropicToOpenAI 将 Anthropic /v1/messages 请求体转为 OpenAI /v1/chat/completions
func TranslateAnthropicToOpenAI(body []byte) ([]byte, error) {
	var anthReq anthropicRequest
	if err := json.Unmarshal(body, &anthReq); err != nil {
		return nil, fmt.Errorf("unmarshal anthropic request: %w", err)
	}

	oaReq := openAIRequest{
		Model:     anthReq.Model,
		MaxTokens: anthReq.MaxTokens,
		Stream:    anthReq.Stream,
		Stop:      anthReq.StopSequences,
	}

	if anthReq.Temperature != nil {
		oaReq.Temperature = anthReq.Temperature
	}
	if anthReq.TopP != nil {
		oaReq.TopP = anthReq.TopP
	}

	// 将 system 作为首条消息
	if anthReq.System != "" {
		oaReq.Messages = append(oaReq.Messages, openAIMessage{
			Role:    "system",
			Content: anthReq.System,
		})
	}

	// 转换 messages
	for _, msg := range anthReq.Messages {
		content := extractTextContent(msg.Content)
		role := msg.Role
		// 将 assistant 映射为 assistant（OpenAI 也用 assistant）
		oaReq.Messages = append(oaReq.Messages, openAIMessage{
			Role:    role,
			Content: content,
		})
	}

	return json.Marshal(oaReq)
}

func extractTextContent(raw json.RawMessage) string {
	// 尝试解析为字符串
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// 尝试解析为 content block 数组
	var blocks []anthropicContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var texts []string
		for _, b := range blocks {
			if b.Type == "text" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return string(raw)
}

// --- OpenAI → Anthropic 响应转换 ---

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	Model      string                  `json:"model"`
	StopReason string                  `json:"stop_reason,omitempty"`
	StopSeq    *string                 `json:"stop_sequence,omitempty"`
	Usage      *anthropicUsage         `json:"usage,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

// TranslateOpenAIToAnthropic 将 OpenAI 非流式响应转为 Anthropic 格式
func TranslateOpenAIToAnthropic(body []byte) ([]byte, error) {
	var oaResp openAIResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return nil, fmt.Errorf("unmarshal openai response: %w", err)
	}

	anthResp := anthropicResponse{
		ID:    oaResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: oaResp.Model,
	}

	if len(oaResp.Choices) > 0 {
		anthResp.Content = []anthropicContentBlock{
			{Type: "text", Text: oaResp.Choices[0].Message.Content},
		}
		anthResp.StopReason = mapFinishReason(oaResp.Choices[0].FinishReason)
	}

	if oaResp.Usage != nil {
		anthResp.Usage = &anthropicUsage{
			InputTokens:  oaResp.Usage.PromptTokens,
			OutputTokens: oaResp.Usage.CompletionTokens,
		}
	}

	return json.Marshal(anthResp)
}

func mapFinishReason(fr string) string {
	switch fr {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "content_filter"
	default:
		return fr
	}
}

// --- Cohere /v1/chat → OpenAI /v1/chat/completions ---

type cohereChatHistory struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type cohereRequest struct {
	Model       string              `json:"model"`
	Message     string              `json:"message"`
	ChatHistory []cohereChatHistory `json:"chat_history,omitempty"`
	Preamble    string              `json:"preamble,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

// TranslateCohereToOpenAI 将 Cohere /v1/chat 转为 OpenAI /v1/chat/completions
func TranslateCohereToOpenAI(body []byte) ([]byte, error) {
	var coReq cohereRequest
	if err := json.Unmarshal(body, &coReq); err != nil {
		return nil, fmt.Errorf("unmarshal cohere request: %w", err)
	}

	oaReq := openAIRequest{
		Model:     coReq.Model,
		MaxTokens: coReq.MaxTokens,
		Stream:    coReq.Stream,
	}
	if coReq.Temperature != nil {
		oaReq.Temperature = coReq.Temperature
	}

	// preamble → system message
	if coReq.Preamble != "" {
		oaReq.Messages = append(oaReq.Messages, openAIMessage{
			Role:    "system",
			Content: coReq.Preamble,
		})
	}

	// chat_history → messages
	for _, ch := range coReq.ChatHistory {
		role := "user"
		if ch.Role == "CHATBOT" || ch.Role == "assistant" {
			role = "assistant"
		}
		oaReq.Messages = append(oaReq.Messages, openAIMessage{
			Role:    role,
			Content: ch.Message,
		})
	}

	// 当前 message 作为最后一条 user 消息
	oaReq.Messages = append(oaReq.Messages, openAIMessage{
		Role:    "user",
		Content: coReq.Message,
	})

	return json.Marshal(oaReq)
}

// --- OpenAI → Cohere 响应转换 ---

type cohereResponse struct {
	Message      string      `json:"message"`
	Text         string      `json:"text"`
	GenerationID string      `json:"generation_id"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Meta         *cohereMeta `json:"meta,omitempty"`
}

type cohereMeta struct {
	BilledUnits *cohereBilledUnits `json:"billed_units,omitempty"`
}

type cohereBilledUnits struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// TranslateOpenAIToCohere 将 OpenAI 非流式响应转为 Cohere 格式
func TranslateOpenAIToCohere(body []byte) ([]byte, error) {
	var oaResp openAIResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return nil, fmt.Errorf("unmarshal openai response: %w", err)
	}

	coResp := cohereResponse{
		GenerationID: oaResp.ID,
		FinishReason: "COMPLETE",
	}

	if len(oaResp.Choices) > 0 {
		coResp.Text = oaResp.Choices[0].Message.Content
		if oaResp.Choices[0].FinishReason == "length" {
			coResp.FinishReason = "MAX_TOKENS"
		}
	}

	if oaResp.Usage != nil {
		coResp.Meta = &cohereMeta{
			BilledUnits: &cohereBilledUnits{
				InputTokens:  oaResp.Usage.PromptTokens,
				OutputTokens: oaResp.Usage.CompletionTokens,
			},
		}
	}

	return json.Marshal(coResp)
}
