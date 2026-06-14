// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"sync"
	"time"
)

// 知识捕获全局变量
var (
	knowledgeCh      chan KnowledgeEntry
	knowledgeOnce    sync.Once
	knowledgeStats   CaptureStats
	knowledgeStatsMu sync.Mutex
)

// IsKnowledgePaused 读取暂停状态：knowledge_pause_until=-1 永久暂停；>0 且未过期则暂停
func IsKnowledgePaused() bool {
	v := LoadSetting("knowledge_pause_until", 0)
	var until int64
	switch x := v.(type) {
	case int:
		until = int64(x)
	case int64:
		until = x
	case float64:
		until = int64(x)
	default:
		return false
	}
	if until == -1 {
		return true
	}
	if until > 0 && time.Now().Unix() < until {
		return true
	}
	return false
}

// IsKnowledgeEnabled 动态读取知识库开关（从 wr_system_settings）；暂停态视作未启用
func IsKnowledgeEnabled() bool {
	if IsKnowledgePaused() {
		return false
	}
	v := LoadSetting("knowledge_capture_enabled", false)
	b, _ := v.(bool)
	return b
}

// InitKnowledge 初始化知识捕获模块（带缓冲 channel + worker）
func InitKnowledge() {
	knowledgeOnce.Do(func() {
		knowledgeCh = make(chan KnowledgeEntry, 1024)
		go knowledgeWorker()
		LogInfo("Knowledge capture initialized: channel buffer=1024")
	})
}

// DeliverKnowledge 异步投递知识捕获条目（非阻塞）
// 在 handleProxy 响应完成后调用
func DeliverKnowledge(token *Token, reqID, model, endpoint, clientIP string,
	sanitizedPrompt, sanitizedResponse string, body []byte) {

	if !IsKnowledgeEnabled() {
		return
	}
	if !token.KnowledgeCaptureEnabled {
		return
	}

	turnCount := extractTurnCount(body)

	entry := KnowledgeEntry{
		RequestID:  reqID,
		TokenID:    token.ID,
		TokenName:  token.Name,
		Department: token.KnowledgeDepartment,
		Model:      model,
		Endpoint:   endpoint,
		Prompt:     sanitizedPrompt,
		Response:   sanitizedResponse,
		TurnCount:  turnCount,
		ClientIP:   clientIP,
		Timestamp:  time.Now().UTC().Format("2006-01-02 15:04:05"),
	}

	// 独立 goroutine 投递，不阻塞 handleProxy
	go func() {
		select {
		case knowledgeCh <- entry:
			// 投递成功
		default:
			// channel 满，丢弃，不阻塞
			LogWarn("[knowledge] channel full, dropping request %s", reqID)
		}
	}()
}

// knowledgeWorker 后台协程：消费 knowledgeCh，筛选并存储
func knowledgeWorker() {
	for entry := range knowledgeCh {
		processKnowledgeEntry(entry)
	}
}

// processKnowledgeEntry 处理单条知识条目
func processKnowledgeEntry(entry KnowledgeEntry) {
	knowledgeStatsMu.Lock()
	knowledgeStats.TotalCaptured++
	knowledgeStats.TodayCaptured++
	knowledgeStatsMu.Unlock()

	// 第1级：信号筛选
	if !shouldCapture(entry) {
		knowledgeStatsMu.Lock()
		knowledgeStats.TotalFiltered++
		knowledgeStats.TodayFiltered++
		knowledgeStatsMu.Unlock()
		return
	}

	// 写入 raw 表
	if err := saveKnowledgeRaw(entry); err != nil {
		LogWarn("[knowledge] failed to save raw entry: %v", err)
		return
	}

	// 审计日志：知识捕获
	LogAudit(AuditKnowledgeCapture, AuditResourceRaw, entry.RequestID, entry.TokenID, map[string]interface{}{
		"model":    entry.Model,
		"turns":    entry.TurnCount,
		"token":    entry.TokenName,
		"endpoint": entry.Endpoint,
	}, entry.ClientIP)

	knowledgeStatsMu.Lock()
	knowledgeStats.TotalSaved++
	knowledgeStats.TodaySaved++
	knowledgeStatsMu.Unlock()
}

// GetCaptureStats 获取捕获统计信息
func GetCaptureStats() CaptureStats {
	knowledgeStatsMu.Lock()
	defer knowledgeStatsMu.Unlock()
	return knowledgeStats
}

// ResetDailyStats 重置日统计（每天 0 点调用）
func ResetDailyStats() {
	knowledgeStatsMu.Lock()
	knowledgeStats.TodayCaptured = 0
	knowledgeStats.TodayFiltered = 0
	knowledgeStats.TodaySaved = 0
	knowledgeStatsMu.Unlock()
}

// extractPrompt 从请求体中提取 prompt 文本（脱敏后的）
func extractPrompt(body []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	messages, ok := obj["messages"]
	if !ok {
		return ""
	}
	msgList, ok := messages.([]interface{})
	if !ok {
		return ""
	}

	var combined string
	for _, m := range msgList {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := msg["content"]
		if !ok {
			continue
		}
		switch v := content.(type) {
		case string:
			combined += v + "\n"
		case []interface{}:
			// multi-modal: extract text parts
			for _, part := range v {
				if p, ok := part.(map[string]interface{}); ok {
					if t, ok := p["text"].(string); ok {
						combined += t + "\n"
					}
				}
			}
		}
	}
	return combined
}

// extractResponse 从响应体中提取 response 文本（脱敏后的）
func extractResponse(body []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}

	// 非流式响应
	if choices, ok := obj["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok && content != "" {
					return content
				}
				if reasoning, ok := message["reasoning_content"].(string); ok && reasoning != "" {
					return reasoning
				}
			}
		}
	}

	// 流式响应的 delta（仅取第一条）
	if choices, ok := obj["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok && content != "" {
					return content
				}
				if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
					return reasoning
				}
			}
		}
	}

	return ""
}
