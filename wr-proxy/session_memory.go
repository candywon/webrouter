// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// Session Memory Recall — 会话历史召回模块
//
// 目的：当客户端丢失上下文（重启等）时，按 token 配置 + 显式触发
// 从服务端恢复历史对话，注入回当前请求的 messages 数组。
//
// 触发：
//   1. X-Recall-Session: <session_id> header
//   2. user 消息含 "@recall" 魔术词（使用当前 X-Session-Id）
//
// 与 wr_knowledge_raw / wr_agent_memory 的区别：
//   - knowledge_raw：知识沉淀素材，会被 LLM 提炼成结构化知识
//   - agent_memory：提炼后的事实/偏好（结构化条目）
//   - session_messages（本模块）：原始对话回放，无提炼

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultRecallBudget         = 8000  // 默认 token 预算
	maxSessionMessageLen        = 10000 // 单条 message 最大字符数（safety）
	defaultSessionRetentionDays = 30    // 默认会话保留期
	sessionMsgChannelBuffer     = 1024
	recallMagicWord             = "@recall"
	recallHeaderKey             = "X-Recall-Session"
)

// recallPhrases 中文自然语言触发词（不剥离内容，仅作为触发信号）
var recallPhrases = []string{
	"还记得",
	"继续上次",
	"继续聊",
	"接着说",
	"之前聊过",
	"上次说到",
	"接着上次",
}

var (
	sessionMsgCh   chan sessionMsgTask
	sessionMsgOnce sync.Once
)

// sessionMsgTask 单次落盘任务（仅 append 增量：本轮 user + 本次 assistant）
type sessionMsgTask struct {
	SessionID string
	TokenID   int
	Model     string
	UserMsg   string // 最后一条 user 内容（已脱敏）
	AssistMsg string // 本次 assistant 响应内容（已脱敏）
}

// InitSessionMemoryTables 创建 wr_session_messages 表 + 索引
func InitSessionMemoryTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS wr_session_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			token_id   INTEGER NOT NULL,
			turn_index INTEGER NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			model      TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_smsg_sess_token ON wr_session_messages(session_id, token_id)`,
		`CREATE INDEX IF NOT EXISTS idx_smsg_token_created ON wr_session_messages(token_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_smsg_sess_turn ON wr_session_messages(session_id, token_id, turn_index)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("init session_messages: %w", err)
		}
	}
	return nil
}

// EstimateTokens 启发式 token 估算：
//   - CJK 字符：1 token / 字
//   - ASCII / 其他：4 chars / token
//
// 偏保守（略多估）以便预算控制。TODO: 后续可替换 tiktoken-go。
func EstimateTokens(text string) int {
	cjkRunes := 0
	otherRunes := 0
	for _, r := range text {
		if isCJK(r) {
			cjkRunes++
		} else {
			otherRunes++
		}
	}
	return cjkRunes + (otherRunes+3)/4
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified
		return true
	case r >= 0x3040 && r <= 0x30FF: // Hiragana / Katakana
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Ext A
		return true
	}
	return false
}

// detectRecallTrigger 判断本请求是否触发会话召回
// 返回 (是否触发, 目标 sessionID, 清理过 @recall 的 body)
func detectRecallTrigger(r *http.Request, body []byte) (bool, string, []byte) {
	// 1. header 优先
	if sid := strings.TrimSpace(r.Header.Get(recallHeaderKey)); sid != "" {
		return true, sid, body
	}

	// 2. 魔术词 / 自然语言触发：检查最后一条 user 消息
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return false, "", body
	}
	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return false, "", body
	}

	// 获取 session id（自然语言触发需要 X-Session-Id）
	sid := r.Header.Get("X-Session-Id")
	if sid == "" {
		sid = r.Header.Get("X-Session-ID")
	}

	// 检查 messages 中是否已有 assistant 回复（有 = 客户端有上下文，不需要自动召回）
	hasAssistant := false
	for _, m := range messages {
		if msg, ok := m.(map[string]interface{}); ok {
			if role, _ := msg["role"].(string); role == "assistant" {
				hasAssistant = true
				break
			}
		}
	}

	// 从后往前找最后一条 user 消息
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "user" {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			return false, "", body
		}

		// @recall 永远触发（显式魔术词），需剥离
		if containsFold(content, recallMagicWord) {
			cleaned := stripRecallWord(content)
			msg["content"] = cleaned
			newBody, err := json.Marshal(req)
			if err != nil {
				return false, "", body
			}
			if sid == "" {
				return false, "", body
			}
			return true, sid, newBody
		}

		// 自然语言触发词：仅在客户端无上下文时触发
		if !hasAssistant {
			for _, phrase := range recallPhrases {
				if strings.Contains(content, phrase) {
					if sid == "" {
						return false, "", body
					}
					return true, sid, body
				}
			}
		}

		return false, "", body
	}
	return false, "", body
}

// containsFold case-insensitive substring check
func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// stripRecallWord 移除 @recall（不区分大小写）并清理多余空白
func stripRecallWord(s string) string {
	lower := strings.ToLower(s)
	idx := strings.Index(lower, strings.ToLower(recallMagicWord))
	if idx < 0 {
		return s
	}
	end := idx + len(recallMagicWord)
	cleaned := s[:idx] + s[end:]
	return strings.TrimFunc(cleaned, unicode.IsSpace)
}

// recalledTurn 一条召回的历史消息（仅内存使用）
type recalledTurn struct {
	TurnIndex int
	Role      string
	Content   string
	Model     string
}

// injectSessionMemory 将历史会话注入当前请求 messages
// 顺序：[原有 system（若有）] + [召回历史 chronological] + [其余原有 messages]
func injectSessionMemory(body []byte, token *Token, sessionID, clientIP string) []byte {
	if sessionID == "" {
		return body
	}

	budget := defaultRecallBudget
	if v := LoadSetting("session_recall_budget", defaultRecallBudget); v != nil {
		if n, ok := v.(int); ok && n > 0 {
			budget = n
		}
	}

	turns, err := loadSessionTurns(sessionID, token.ID)
	if err != nil {
		LogWarn("[session_recall] load failed token=%d session=%s err=%v", token.ID, sessionID, err)
		return body
	}
	if len(turns) == 0 {
		return body
	}

	// 从最新往回累加，直到 budget 用尽
	usedTokens := 0
	cutoff := len(turns) // 默认全部用上
	for i := len(turns) - 1; i >= 0; i-- {
		cost := EstimateTokens(turns[i].Content)
		if usedTokens+cost > budget {
			cutoff = i + 1
			break
		}
		usedTokens += cost
		if i == 0 {
			cutoff = 0
		}
	}
	selected := turns[cutoff:]
	if len(selected) == 0 {
		return body
	}

	// 解析当前 body
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	messages, ok := req["messages"].([]interface{})
	if !ok {
		return body
	}

	// 构造召回 messages（chronological）
	recalled := make([]interface{}, 0, len(selected))
	for _, t := range selected {
		recalled = append(recalled, map[string]interface{}{
			"role":       t.Role,
			"content":    t.Content,
			"__recalled": true,
		})
	}

	// 拼接：系统消息（若有）+ recalled + 其余
	newMessages := make([]interface{}, 0, len(messages)+len(recalled))
	startIdx := 0
	if len(messages) > 0 {
		if firstMsg, ok := messages[0].(map[string]interface{}); ok {
			if role, _ := firstMsg["role"].(string); role == "system" {
				newMessages = append(newMessages, firstMsg)
				startIdx = 1
			}
		}
	}
	newMessages = append(newMessages, recalled...)
	newMessages = append(newMessages, messages[startIdx:]...)
	req["messages"] = newMessages

	newBody, err := json.Marshal(req)
	if err != nil {
		return body
	}

	LogInfo("[session_recall] token=%d session=%s injected=%d turns budget=%d used≈%d",
		token.ID, sessionID, len(selected), budget, usedTokens)

	LogAudit(AuditKnowledgeAccess, "session", sessionID, token.ID,
		map[string]interface{}{
			"recalled_turns":   len(selected),
			"estimated_tokens": usedTokens,
			"budget":           budget,
		}, clientIP)

	return newBody
}

// loadSessionTurns 加载会话全部 turns（按 turn_index 升序）
func loadSessionTurns(sessionID string, tokenID int) ([]recalledTurn, error) {
	rows, err := db.Query(`
		SELECT turn_index, role, content, COALESCE(model, '')
		FROM wr_session_messages
		WHERE session_id = ? AND token_id = ?
		ORDER BY turn_index ASC`,
		sessionID, tokenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var turns []recalledTurn
	for rows.Next() {
		var t recalledTurn
		if err := rows.Scan(&t.TurnIndex, &t.Role, &t.Content, &t.Model); err != nil {
			return nil, err
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

// DeliverSessionMessages 异步落盘本轮 user + assistant（增量 append）
// 仅在 token.SessionRecallEnabled 且 sessionID 非空时调用
func DeliverSessionMessages(token *Token, sessionID, model string, reqBody []byte, assistantContent string) {
	if token == nil || !token.SessionRecallEnabled {
		return
	}
	if sessionID == "" || assistantContent == "" {
		return
	}

	userMsg := extractLastUserMessage(reqBody)
	if userMsg == "" {
		return
	}

	task := sessionMsgTask{
		SessionID: sessionID,
		TokenID:   token.ID,
		Model:     model,
		UserMsg:   truncate(userMsg, maxSessionMessageLen),
		AssistMsg: truncate(assistantContent, maxSessionMessageLen),
	}

	go func() {
		select {
		case sessionMsgCh <- task:
		default:
			LogWarn("[session_recall] channel full, dropping session=%s", sessionID)
		}
	}()
}

// extractLastUserMessage 提取请求 body 里最后一条 user role 消息内容
// 跳过 __recalled=true 的（已是历史，不重复入库）
func extractLastUserMessage(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	messages, ok := req["messages"].([]interface{})
	if !ok {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		if recalled, _ := msg["__recalled"].(bool); recalled {
			continue
		}
		if role, _ := msg["role"].(string); role != "user" {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			// 多模态：拼接所有 text 段
			if arr, ok := msg["content"].([]interface{}); ok {
				var b strings.Builder
				for _, part := range arr {
					if p, ok := part.(map[string]interface{}); ok {
						if t, _ := p["text"].(string); t != "" {
							b.WriteString(t)
							b.WriteString("\n")
						}
					}
				}
				return strings.TrimSpace(b.String())
			}
			return ""
		}
		return content
	}
	return ""
}

// InitSessionMemoryWorker 启动消费协程
func InitSessionMemoryWorker() {
	sessionMsgOnce.Do(func() {
		sessionMsgCh = make(chan sessionMsgTask, sessionMsgChannelBuffer)
		go sessionMessageWorker()
		LogInfo("Session Memory worker initialized: channel buffer=%d", sessionMsgChannelBuffer)
	})
}

// sessionMessageWorker 单 goroutine 顺序消费，避免 turn_index 竞态
func sessionMessageWorker() {
	for task := range sessionMsgCh {
		if err := saveSessionTurn(task); err != nil {
			LogWarn("[session_recall] save failed token=%d session=%s err=%v",
				task.TokenID, task.SessionID, err)
		}
	}
}

// saveSessionTurn 同一会话顺序写入 user + assistant 两条
func saveSessionTurn(task sessionMsgTask) error {
	nextIdx, err := nextTurnIndex(task.SessionID, task.TokenID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	if _, err := db.Exec(`
		INSERT INTO wr_session_messages (session_id, token_id, turn_index, role, content, model, created_at)
		VALUES (?, ?, ?, 'user', ?, ?, ?)`,
		task.SessionID, task.TokenID, nextIdx, task.UserMsg, task.Model, now); err != nil {
		return err
	}
	if _, err := db.Exec(`
		INSERT INTO wr_session_messages (session_id, token_id, turn_index, role, content, model, created_at)
		VALUES (?, ?, ?, 'assistant', ?, ?, ?)`,
		task.SessionID, task.TokenID, nextIdx+1, task.AssistMsg, task.Model, now); err != nil {
		return err
	}
	return nil
}

func nextTurnIndex(sessionID string, tokenID int) (int, error) {
	var maxIdx sql.NullInt64
	err := db.QueryRow(`SELECT MAX(turn_index) FROM wr_session_messages WHERE session_id=? AND token_id=?`,
		sessionID, tokenID).Scan(&maxIdx)
	if err != nil {
		return 0, err
	}
	if !maxIdx.Valid {
		return 0, nil
	}
	return int(maxIdx.Int64) + 1, nil
}

// startSessionMemoryCleanup 定期清理超过保留期的会话
func startSessionMemoryCleanup() {
	time.Sleep(15 * time.Minute) // 延迟首次执行
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		days := loadSessionRetentionDays()
		cutoff := fmt.Sprintf("-%d days", days)
		result, err := db.Exec(`
			DELETE FROM wr_session_messages
			WHERE id IN (
				SELECT id FROM wr_session_messages
				WHERE (session_id, token_id) IN (
					SELECT session_id, token_id FROM wr_session_messages
					GROUP BY session_id, token_id
					HAVING MAX(created_at) < datetime('now', ?)
				)
			)`, cutoff)
		if err != nil {
			LogWarn("[session_recall] cleanup failed: %v", err)
			continue
		}
		n, _ := result.RowsAffected()
		if n > 0 {
			LogInfo("[session_recall] cleanup: deleted %d messages older than %d days", n, days)
			LogAudit(AuditRetentionCleanup, "session", "", 0,
				map[string]interface{}{"deleted_rows": n, "retention_days": days}, "")
		}
	}
}

func loadSessionRetentionDays() int {
	v := LoadSetting("session_recall_retention_days", defaultSessionRetentionDays)
	if n, ok := v.(int); ok && n > 0 {
		return n
	}
	return defaultSessionRetentionDays
}
