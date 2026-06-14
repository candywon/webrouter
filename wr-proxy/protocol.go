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

// anthropicContentBlock 涵盖 Anthropic 所有 content 块类型
//   - text: text
//   - thinking: thinking
//   - tool_use: id/name/input
//   - tool_result: tool_use_id/content
type anthropicContentBlock struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	Thinking   string          `json:"thinking,omitempty"`
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	ToolUseID  string          `json:"tool_use_id,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
	ToolResult json.RawMessage `json:"content,omitempty"` // tool_result 的 content (string 或 block 数组)
}

// anthropicTool Anthropic 工具定义
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicToolChoice "auto" / "any" / {"type":"tool","name":"x"}
type anthropicToolChoice struct {
	Type string `json:"type"`           // auto / any / tool
	Name string `json:"name,omitempty"` // type=tool 时
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
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    json.RawMessage    `json:"tool_choice,omitempty"`
}

// openAIToolCall OpenAI assistant 消息中的工具调用
type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // 通常 "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // 注意：JSON-encoded string
	} `json:"function"`
}

type openAIMessage struct {
	Role             string           `json:"role"`
	Content          json.RawMessage  `json:"content,omitempty"` // string or []contentBlock (multimodal)
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"` // role=tool 时
	Name             string           `json:"name,omitempty"`
}

// openAIContentBlock 用于 OpenAI 多模态请求中的 content 数组
type openAIContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// openAITool OpenAI 工具定义
type openAITool struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Parameters  json.RawMessage `json:"parameters,omitempty"`
	} `json:"function"`
}

// extractOpenAIText 从 OpenAI content（可能是 string 或 [{type,text}] 数组）中提取文本
func extractOpenAIText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []openAIContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var texts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

// jsonString 把字符串编码成 json.RawMessage（用于设置 Content 字段）
func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// extractAnthropicToolResultText 从 tool_result 块的 content（string 或 [{type,text,...}]）中提取文本
func extractAnthropicToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []anthropicContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
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

	// tools 反向映射：Anthropic {name,description,input_schema} → OpenAI {type:function, function:{...}}
	for _, t := range anthReq.Tools {
		var oaT openAITool
		oaT.Type = "function"
		oaT.Function.Name = t.Name
		oaT.Function.Description = t.Description
		oaT.Function.Parameters = t.InputSchema
		oaReq.Tools = append(oaReq.Tools, oaT)
	}

	// tool_choice 反向：Anthropic {type:auto|any|tool, name?} → OpenAI string 或 function 对象
	if len(anthReq.ToolChoice) > 0 {
		oaReq.ToolChoice = translateToolChoiceAnthToOA(anthReq.ToolChoice)
	}

	// 将 system 作为首条消息
	if anthReq.System != "" {
		oaReq.Messages = append(oaReq.Messages, openAIMessage{
			Role:    "system",
			Content: jsonString(anthReq.System),
		})
	}

	// 转换 messages
	for _, msg := range anthReq.Messages {
		// 尝试解析为 block 数组（带 tool_use/tool_result 时必须）
		var blocks []anthropicContentBlock
		isBlockArr := false
		if len(msg.Content) > 0 && msg.Content[0] == '[' {
			if err := json.Unmarshal(msg.Content, &blocks); err == nil {
				isBlockArr = true
			}
		}

		if isBlockArr {
			// 拆分：tool_use 块 → assistant.tool_calls；tool_result 块 → 独立的 role=tool 消息
			var textParts []string
			var toolCalls []openAIToolCall
			var toolResults []openAIMessage // role=tool 消息
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if b.Text != "" {
						textParts = append(textParts, b.Text)
					}
				case "thinking":
					// 不放进 OpenAI 请求（主要是历史回放，OpenAI 不接受 reasoning_content 在请求侧）
				case "tool_use":
					var tc openAIToolCall
					tc.ID = b.ID
					tc.Type = "function"
					tc.Function.Name = b.Name
					if len(b.Input) > 0 {
						tc.Function.Arguments = string(b.Input)
					} else {
						tc.Function.Arguments = "{}"
					}
					toolCalls = append(toolCalls, tc)
				case "tool_result":
					toolResults = append(toolResults, openAIMessage{
						Role:       "tool",
						ToolCallID: b.ToolUseID,
						Content:    jsonString(extractAnthropicToolResultText(b.ToolResult)),
					})
				}
			}

			if len(toolResults) > 0 {
				oaReq.Messages = append(oaReq.Messages, toolResults...)
			}
			if len(textParts) > 0 || len(toolCalls) > 0 {
				m := openAIMessage{Role: msg.Role}
				if len(textParts) > 0 {
					m.Content = jsonString(strings.Join(textParts, "\n"))
				}
				if len(toolCalls) > 0 {
					m.ToolCalls = toolCalls
				}
				oaReq.Messages = append(oaReq.Messages, m)
			}
			continue
		}

		// 普通 string content
		content := extractTextContent(msg.Content)
		role := msg.Role
		oaReq.Messages = append(oaReq.Messages, openAIMessage{
			Role:    role,
			Content: jsonString(content),
		})
	}

	return json.Marshal(oaReq)
}

// translateToolChoiceAnthToOA Anthropic tool_choice → OpenAI tool_choice
//   {"type":"auto"} → "auto"
//   {"type":"any"}  → "required"
//   {"type":"tool","name":"x"} → {"type":"function","function":{"name":"x"}}
func translateToolChoiceAnthToOA(raw json.RawMessage) json.RawMessage {
	var obj struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	switch obj.Type {
	case "auto":
		return json.RawMessage(`"auto"`)
	case "any":
		return json.RawMessage(`"required"`)
	case "tool":
		out, _ := json.Marshal(map[string]interface{}{
			"type":     "function",
			"function": map[string]string{"name": obj.Name},
		})
		return out
	}
	return nil
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
		text := extractOpenAIText(oaResp.Choices[0].Message.Content)
		if text == "" {
			text = oaResp.Choices[0].Message.ReasoningContent
		}
		anthResp.Content = []anthropicContentBlock{
			{Type: "text", Text: text},
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

// mapStopReasonToFinish 反向映射：Anthropic stop_reason → OpenAI finish_reason
func mapStopReasonToFinish(sr string) string {
	switch sr {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return sr
	}
}

// --- OpenAI → Anthropic 请求转换（用于 Anthropic 上游） ---

// TranslateOpenAIToAnthropicRequest 将 OpenAI /v1/chat/completions 请求转为 Anthropic /v1/messages
func TranslateOpenAIToAnthropicRequest(body []byte) ([]byte, error) {
	var oaReq openAIRequest
	if err := json.Unmarshal(body, &oaReq); err != nil {
		return nil, fmt.Errorf("unmarshal openai request: %w", err)
	}

	anthReq := anthropicRequest{
		Model:         oaReq.Model,
		MaxTokens:     oaReq.MaxTokens,
		Stream:        oaReq.Stream,
		StopSequences: oaReq.Stop,
		Temperature:   oaReq.Temperature,
		TopP:          oaReq.TopP,
	}
	// Anthropic 要求 max_tokens 必填，给个兜底
	if anthReq.MaxTokens <= 0 {
		anthReq.MaxTokens = 4096
	}

	// tools: OpenAI {type:function, function:{name,description,parameters}} → Anthropic {name,description,input_schema}
	for _, t := range oaReq.Tools {
		anthReq.Tools = append(anthReq.Tools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	// tool_choice: OpenAI 的 "auto"/"none"/"required"/{type:function,function:{name}} → Anthropic
	if len(oaReq.ToolChoice) > 0 {
		anthReq.ToolChoice = translateToolChoiceOAToAnth(oaReq.ToolChoice)
	}

	// system 消息提取到顶层
	var sysParts []string
	for _, msg := range oaReq.Messages {
		if msg.Role == "system" {
			text := extractOpenAIText(msg.Content)
			if text != "" {
				sysParts = append(sysParts, text)
			}
			continue
		}

		// role=tool: 转 Anthropic user 消息中的 tool_result block
		if msg.Role == "tool" {
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
			}
			block.ToolResult = jsonString(extractOpenAIText(msg.Content))
			blocks := []anthropicContentBlock{block}
			cb, _ := json.Marshal(blocks)
			anthReq.Messages = append(anthReq.Messages, anthropicMessage{
				Role:    "user",
				Content: cb,
			})
			continue
		}

		// assistant 带 tool_calls：转 tool_use blocks
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var blocks []anthropicContentBlock
			if t := extractOpenAIText(msg.Content); t != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: t})
			}
			for _, tc := range msg.ToolCalls {
				input := json.RawMessage(tc.Function.Arguments)
				if len(input) == 0 || !json.Valid(input) {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			cb, _ := json.Marshal(blocks)
			anthReq.Messages = append(anthReq.Messages, anthropicMessage{
				Role:    "assistant",
				Content: cb,
			})
			continue
		}

		role := msg.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		// 提取文本：兼容 string 和 [{type:text,text:...}] 两种格式
		text := extractOpenAIText(msg.Content)
		contentBytes, _ := json.Marshal(text)
		anthReq.Messages = append(anthReq.Messages, anthropicMessage{
			Role:    role,
			Content: contentBytes,
		})
	}
	if len(sysParts) > 0 {
		anthReq.System = strings.Join(sysParts, "\n\n")
	}

	return json.Marshal(anthReq)
}

// translateToolChoiceOAToAnth 把 OpenAI tool_choice 翻译成 Anthropic 格式
//   "auto" → {"type":"auto"}
//   "none" → 省略（Anthropic 没有等价"禁用"，靠不传 tools 或 tool_choice 留空）
//   "required" → {"type":"any"}
//   {"type":"function","function":{"name":"x"}} → {"type":"tool","name":"x"}
func translateToolChoiceOAToAnth(raw json.RawMessage) json.RawMessage {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		switch s {
		case "auto":
			return json.RawMessage(`{"type":"auto"}`)
		case "required":
			return json.RawMessage(`{"type":"any"}`)
		case "none":
			return nil
		}
		return nil
	}
	var obj struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Type == "function" && obj.Function.Name != "" {
		out, _ := json.Marshal(map[string]string{"type": "tool", "name": obj.Function.Name})
		return out
	}
	return nil
}

// --- Anthropic → OpenAI 响应转换（用于 Anthropic 上游非流式响应） ---

type anthropicResponseFull struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Model      string                  `json:"model"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      *anthropicUsage         `json:"usage,omitempty"`
}

// TranslateAnthropicToOpenAIResponse 将 Anthropic 非流式响应转为 OpenAI 格式
func TranslateAnthropicToOpenAIResponse(body []byte) ([]byte, error) {
	var anthResp anthropicResponseFull
	if err := json.Unmarshal(body, &anthResp); err != nil {
		return nil, fmt.Errorf("unmarshal anthropic response: %w", err)
	}

	var textParts, thinkingParts []string
	var toolCalls []map[string]interface{}
	for _, b := range anthResp.Content {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "thinking":
			thinkingParts = append(thinkingParts, b.Thinking)
		case "tool_use":
			args := string(b.Input)
			if args == "" || !json.Valid(b.Input) {
				args = "{}"
			}
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   b.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      b.Name,
					"arguments": args,
				},
			})
		}
	}

	message := map[string]interface{}{
		"role":              "assistant",
		"content":           strings.Join(textParts, ""),
		"reasoning_content": strings.Join(thinkingParts, ""),
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	finishReason := mapStopReasonToFinish(anthResp.StopReason)
	if len(toolCalls) > 0 && (finishReason == "stop" || finishReason == "") {
		finishReason = "tool_calls"
	}

	oaResp := map[string]interface{}{
		"id":     anthResp.ID,
		"object": "chat.completion",
		"model":  anthResp.Model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
	}

	if anthResp.Usage != nil {
		oaResp["usage"] = map[string]interface{}{
			"prompt_tokens":     anthResp.Usage.InputTokens,
			"completion_tokens": anthResp.Usage.OutputTokens,
			"total_tokens":      anthResp.Usage.InputTokens + anthResp.Usage.OutputTokens,
		}
	}

	return json.Marshal(oaResp)
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
			Content: jsonString(coReq.Preamble),
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
			Content: jsonString(ch.Message),
		})
	}

	// 当前 message 作为最后一条 user 消息
	oaReq.Messages = append(oaReq.Messages, openAIMessage{
		Role:    "user",
		Content: jsonString(coReq.Message),
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
		coResp.Text = extractOpenAIText(oaResp.Choices[0].Message.Content)
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
