package main

import (
	"encoding/json"
	"fmt"
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

// startKnowledgeExtractScheduler 启动知识提取定时任务
// 每5分钟检查 pending raw 条目，自动触发 LLM 提炼
func startKnowledgeExtractScheduler() {
	// 首次启动后 2 分钟执行第一次
	time.Sleep(2 * time.Minute)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		processed, err := ExtractRawToKnowledge()
		if err != nil {
			LogWarn("[knowledge] extract scheduler failed: %v", err)
		} else if processed > 0 {
			LogInfo("[knowledge] extract scheduler: processed %d entries", processed)
		}
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

// handleKnowledgePromptPreview 预览 Token 的知识增强 System Prompt
func handleKnowledgePromptPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		TokenID                 int    `json:"token_id"`
		KnowledgeCaptureEnabled bool   `json:"knowledge_capture_enabled"`
		KnowledgeDepartment     string `json:"knowledge_department"`
		RAGEnabled              bool   `json:"rag_enabled"`
		RAGMinRelevance         float64 `json:"rag_min_relevance"`
		RAGTopK                 int    `json:"rag_top_k"`
		SystemPromptKnowledge   string `json:"system_prompt_knowledge"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}

	// 构造临时 Token 用于预览
	token := &Token{
		ID:                      req.TokenID,
		KnowledgeCaptureEnabled: req.KnowledgeCaptureEnabled,
		KnowledgeDepartment:     req.KnowledgeDepartment,
		RAGEnabled:              req.RAGEnabled,
		SystemPromptKnowledge:   req.SystemPromptKnowledge,
	}

	prompt := GetKnowledgeSystemPrompt(token)

	writeJSON(w, 200, map[string]interface{}{
		"system_prompt": prompt,
		"enabled":       token.KnowledgeCaptureEnabled || token.RAGEnabled || token.SystemPromptKnowledge != "",
		"department":    token.KnowledgeDepartment,
	})
}

// handleKnowledgeAnalyze 知识分析 API（Flask 调用）
func handleKnowledgeAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	var req KnowledgeAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.DomainCode == "" {
		writeJSON(w, 400, map[string]string{"error": "domain_code is required"})
		return
	}

	if req.AnalysisType == "" {
		req.AnalysisType = "domain_overview"
	}

	startTime := time.Now()
	result, err := analyzeKnowledge(req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		writeJSON(w, 500, map[string]interface{}{
			"error":   err.Error(),
			"status":  "failed",
		})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"result":      result,
		"status":      "completed",
		"duration_ms": duration,
	})
}

// handleKnowledgeExtract 触发知识提取（Flask 调用）
func handleKnowledgeExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	// 可选：指定批量大小
	var req struct {
		BatchSize int `json:"batch_size"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.BatchSize > 0 && req.BatchSize <= 20 {
		extractBatch.BatchSize = req.BatchSize
	}

	startTime := time.Now()
	processed, err := ExtractRawToKnowledge()
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		writeJSON(w, 500, map[string]interface{}{
			"error":     err.Error(),
			"processed": processed,
		})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"processed":   processed,
		"duration_ms": duration,
		"message":     fmt.Sprintf("成功提取 %d 条知识", processed),
	})
}
