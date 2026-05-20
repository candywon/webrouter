package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// startEmbeddingBackfillScheduler 定时检查并补全缺失的 embedding
func startEmbeddingBackfillScheduler() {
	if !embeddingCfg.enabled {
		return
	}
	// 首次启动后 3 分钟执行
	time.Sleep(3 * time.Minute)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		processed, err := EmbeddingBackfill(10)
		if err != nil {
			LogWarn("[embedding] backfill scheduler failed: %v", err)
		} else if processed > 0 {
			LogInfo("[embedding] backfill scheduler: processed %d entries", processed)
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

// handleEmbeddingBackfill 手动触发 embedding 批量生成
func handleEmbeddingBackfill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		Limit int `json:"limit"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	startTime := time.Now()
	processed, err := EmbeddingBackfill(req.Limit)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		writeJSON(w, 500, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"processed":   processed,
		"duration_ms": duration,
		"message":     fmt.Sprintf("已为 %d 条知识生成向量", processed),
	})
}

// handleRAGStats 返回 RAG 和向量缓存统计
func handleRAGStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	cacheCount, cacheLoaded := GetVectorCacheStats()
	ragHits, ragMisses := GetRAGStats()

	var pendingEmbed int64
	db.QueryRow(`
		SELECT COUNT(*) FROM wr_knowledge_items i
		LEFT JOIN wr_knowledge_vectors v ON v.item_id = i.id
		WHERE v.item_id IS NULL`).Scan(&pendingEmbed)

	writeJSON(w, 200, map[string]interface{}{
		"vector_cache": map[string]interface{}{
			"count":        cacheCount,
			"last_loaded":  cacheLoaded,
			"pending_fill": pendingEmbed,
		},
		"rag_inject": map[string]interface{}{
			"hits":   ragHits,
			"misses": ragMisses,
		},
	})
}

// handleKnowledgeExport 导出知识条目（JSON格式）
func handleKnowledgeExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	domain := r.URL.Query().Get("domain")
	department := r.URL.Query().Get("department")
	itemType := r.URL.Query().Get("type")
	sensitivity := r.URL.Query().Get("sensitivity")

	var conditions []string
	var args []interface{}

	if domain != "" {
		conditions = append(conditions, "domain_code = ?")
		args = append(args, domain)
	}
	if department != "" {
		conditions = append(conditions, "department = ?")
		args = append(args, department)
	}
	if itemType != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, itemType)
	}
	if sensitivity != "" {
		conditions = append(conditions, "sensitivity = ?")
		args = append(args, sensitivity)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, type, title, summary, domain_code, department, source_quote,
		       data_points, confidence, verification, sensitivity, token_name,
		       model_name, created_at
		FROM wr_knowledge_items %s ORDER BY created_at DESC`, where)

	rows, err := db.Query(query, args...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "query failed: " + err.Error()})
		return
	}
	defer rows.Close()

	type exportItem struct {
		ID          int     `json:"id"`
		Type        string  `json:"type"`
		Title       string  `json:"title"`
		Summary     string  `json:"summary"`
		DomainCode  string  `json:"domain_code"`
		Department  string  `json:"department"`
		SourceQuote string  `json:"source_quote"`
		DataPoints  string  `json:"data_points"`
		Confidence  float64 `json:"confidence"`
		Verification string `json:"verification"`
		Sensitivity string  `json:"sensitivity"`
		TokenName   string  `json:"token_name"`
		ModelName   string  `json:"model_name"`
		CreatedAt   string  `json:"created_at"`
	}

	var items []exportItem
	for rows.Next() {
		var it exportItem
		if err := rows.Scan(&it.ID, &it.Type, &it.Title, &it.Summary, &it.DomainCode,
			&it.Department, &it.SourceQuote, &it.DataPoints, &it.Confidence,
			&it.Verification, &it.Sensitivity, &it.TokenName, &it.ModelName, &it.CreatedAt); err != nil {
			continue
		}
		items = append(items, it)
	}

	writeJSON(w, 200, map[string]interface{}{
		"total": len(items),
		"domain": domain,
		"department": department,
		"type": itemType,
		"items": items,
	})
}

// handleMemoryList 记忆列表 API
func handleMemoryList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	tokenID := 0
	category := ""
	limit := 50

	if q := r.URL.Query().Get("token_id"); q != "" {
		fmt.Sscanf(q, "%d", &tokenID)
	}
	category = r.URL.Query().Get("category")
	if q := r.URL.Query().Get("limit"); q != "" {
		fmt.Sscanf(q, "%d", &limit)
	}

	token := &Token{ID: tokenID}
	memories, err := RecallMemories(token, "", category, limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"memories": memories,
		"total":    len(memories),
	})
}
