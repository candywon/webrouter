package main

import (
	"net/http"
	"time"
)

// startKnowledgeCleanup 启动 raw 表定时清理任务
func startKnowledgeCleanup() {
	// 首次启动后 10 分钟执行第一次清理
	time.Sleep(10 * time.Minute)

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := cleanupKnowledgeRaw(30); err != nil {
			LogWarn("[knowledge] cleanup failed: %v", err)
		}
	}
}

// startKnowledgeDailyReset 每天 0 点重置日统计
func startKnowledgeDailyReset() {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	delay := time.Until(next)

	time.Sleep(delay)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ResetDailyStats()
	}
}

// handleKnowledgeStats 知识捕获统计 API（Flask 调用）
func handleKnowledgeStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	stats := GetCaptureStats()

	// 查询 raw 表待处理数量
	var pendingCount int64
	row := db.QueryRow(`SELECT COUNT(*) FROM wr_knowledge_raw WHERE status = 'pending'`)
	row.Scan(&pendingCount)

	// 查询知识条目总数
	var itemCount int64
	row2 := db.QueryRow(`SELECT COUNT(*) FROM wr_knowledge_items`)
	row2.Scan(&itemCount)

	writeJSON(w, 200, map[string]interface{}{
		"captured": stats,
		"pending_processing": pendingCount,
		"total_items":        itemCount,
		"capture_enabled":    knowledgeEnabled,
	})
}
