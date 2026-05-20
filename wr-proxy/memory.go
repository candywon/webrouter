package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 持久记忆层 — wr_agent_memory 表 + 记忆提取 + 记忆加载
// ============================================================

// AgentMemory 持久化记忆条目
type AgentMemory struct {
	ID        int     `json:"id"`
	TokenID   int     `json:"token_id"`
	TokenName string  `json:"token_name"`
	SessionID string  `json:"session_id"`
	Category  string  `json:"category"` // preference/fact/context/goal/constraint
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Tags      string  `json:"tags"` // JSON array
	Priority  int     `json:"priority"` // 1-5, 5 = most important
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	ExpiresAt string  `json:"expires_at"`
}

// InitMemoryTables 创建记忆表
func InitMemoryTables() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS wr_agent_memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER NOT NULL DEFAULT 0,
			token_name TEXT DEFAULT '',
			session_id TEXT DEFAULT '',
			category TEXT NOT NULL DEFAULT 'context',
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT DEFAULT '[]',
			priority INTEGER DEFAULT 3,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_amem_token ON wr_agent_memory(token_id)`,
		`CREATE INDEX IF NOT EXISTS idx_amem_session ON wr_agent_memory(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_amem_category ON wr_agent_memory(category)`,
		`CREATE INDEX IF NOT EXISTS idx_amem_priority ON wr_agent_memory(priority DESC)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("memory migration: %w", err)
			}
		}
	}
	return nil
}

// SaveMemory 保存记忆条目
func SaveMemory(token *Token, sessionID, category, title, content string, tags []string, priority int, expiresAt string) (int64, error) {
	tagsJSON := "[]"
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsJSON = string(b)
	}

	result, err := db.Exec(`
		INSERT INTO wr_agent_memory
		(token_id, token_name, session_id, category, title, content, tags, priority, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		token.ID, token.Name, sessionID, category, title, content, tagsJSON, priority,
		expiresAt,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// RecallMemories 检索记忆
func RecallMemories(token *Token, sessionID, category string, limit int) ([]AgentMemory, error) {
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "token_id = ?")
	args = append(args, token.ID)

	if sessionID != "" {
		conditions = append(conditions, "(session_id = ? OR session_id = '')")
		args = append(args, sessionID)
	}

	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	// 排除过期记忆
	conditions = append(conditions, "(expires_at IS NULL OR expires_at > ?)")
	args = append(args, time.Now().UTC().Format("2006-01-02 15:04:05"))

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT id, token_id, token_name, session_id, category, title, content, tags, priority,
		       created_at, updated_at, expires_at
		FROM wr_agent_memory
		WHERE %s
		ORDER BY priority DESC, created_at DESC
		LIMIT %d`, where, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []AgentMemory
	for rows.Next() {
		m := AgentMemory{}
		var tagsStr string
		if err := rows.Scan(&m.ID, &m.TokenID, &m.TokenName, &m.SessionID, &m.Category,
			&m.Title, &m.Content, &tagsStr, &m.Priority,
			&m.CreatedAt, &m.UpdatedAt, &m.ExpiresAt); err != nil {
			continue
		}
		m.Tags = tagsStr
		memories = append(memories, m)
	}

	return memories, nil
}

// DeleteMemory 删除记忆
func DeleteMemory(memoryID int, tokenID int) error {
	result, err := db.Exec(`DELETE FROM wr_agent_memory WHERE id = ? AND token_id = ?`, memoryID, tokenID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory #%d not found or no permission", memoryID)
	}
	return nil
}

// UpdateMemory 更新记忆
func UpdateMemory(memoryID int, tokenID int, content, title string) error {
	_, err := db.Exec(`
		UPDATE wr_agent_memory SET content = ?, title = ?, updated_at = ?
		WHERE id = ? AND token_id = ?`,
		content, title,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
		memoryID, tokenID,
	)
	return err
}

// CleanupExpiredMemories 清理过期记忆
func CleanupExpiredMemories() (int, error) {
	result, err := db.Exec(`
		DELETE FROM wr_agent_memory
		WHERE expires_at IS NOT NULL AND expires_at < ?`,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		LogInfo("[memory] cleaned up %d expired memories", n)
	}
	return int(n), nil
}

// startMemoryCleanup 定期清理过期记忆
func startMemoryCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if _, err := CleanupExpiredMemories(); err != nil {
			LogWarn("[memory] cleanup failed: %v", err)
		}
	}
}

// BuildMemoryContext 构建记忆上下文字符串（注入到 system prompt）
func BuildMemoryContext(token *Token, sessionID string) string {
	memories, err := RecallMemories(token, sessionID, "", 10)
	if err != nil || len(memories) == 0 {
		return ""
	}

	var sb string
	sb += "【持久记忆】以下信息来自历史对话记忆，供回答时参考：\n\n"

	for i, m := range memories {
		sb += fmt.Sprintf("%d. [%s] %s\n", i+1, m.Category, m.Title)
		sb += "   " + m.Content + "\n\n"
	}

	return sb
}

// MemoryExtractFromConversation 从对话中提取记忆（异步投递到 memory channel）
type memoryExtractTask struct {
	token      *Token
	sessionID  string
	prompt     string
	response   string
}

var (
	memoryCh   chan memoryExtractTask
	memoryOnce sync.Once
)

// InitMemoryWorker 启动记忆提取 worker
func InitMemoryWorker() {
	memoryOnce.Do(func() {
		memoryCh = make(chan memoryExtractTask, 128)
		go memoryExtractWorker()
		LogInfo("Memory worker: started")
	})
}

// QueueMemoryExtract 投递记忆提取任务（非阻塞）
func QueueMemoryExtract(token *Token, sessionID, prompt, response string) {
	select {
	case memoryCh <- memoryExtractTask{token: token, sessionID: sessionID, prompt: prompt, response: response}:
	default:
		// 队列满时丢弃，不影响主流程
	}
}

// memoryExtractWorker 消费记忆提取任务
func memoryExtractWorker() {
	for task := range memoryCh {
		// 简单启发式规则提取（不依赖 LLM，避免额外开销）
		extractMemoriesSimple(task.token, task.sessionID, task.prompt, task.response)
	}
}

// extractMemoriesSimple 基于简单规则从对话中提取记忆
func extractMemoriesSimple(token *Token, sessionID, prompt, response string) {
	// 规则 1：检测用户偏好表达（"我通常", "我喜欢", "我们一般"）
	if containsAny(prompt, []string{"我通常", "我喜欢", "我们一般", "我总是", "我们习惯"}) {
		SaveMemory(token, sessionID, "preference", "用户偏好", prompt, []string{"auto_extracted"}, 3, "")
		LogInfo("[memory] extracted preference for token %d", token.ID)
	}

	// 规则 2：检测事实性信息（"请注意", "记住", "我们的规定是"）
	if containsAny(prompt, []string{"请记住", "注意", "我们的规定", "我们的政策", "我们公司是"}) {
		SaveMemory(token, sessionID, "fact", "事实信息", prompt, []string{"auto_extracted"}, 3, "")
		LogInfo("[memory] extracted fact for token %d", token.ID)
	}

	// 规则 3：检测目标/任务（"我们的目标是", "需要完成", "计划")
	if containsAny(prompt, []string{"我们的目标是", "需要完成", "计划在本", "计划在"}) {
		SaveMemory(token, sessionID, "goal", "目标/任务", prompt, []string{"auto_extracted"}, 4, "")
		LogInfo("[memory] extracted goal for token %d", token.ID)
	}
}

// containsAny 检查文本是否包含任一子串
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
