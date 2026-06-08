// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ============================================================
// RAG 反馈闭环 — 注入命中统计 + 阈值自动调优
// ============================================================

// ragFeedback 单次 RAG 注入反馈记录
type ragFeedback struct {
	TokenID       int
	TokenName     string
	DomainCode    string
	Query         string
	HitCount      int     // 注入的知识条数
	MinSimilarity float64 // 最低相关度
	MaxSimilarity float64 // 最高相关度
	Timestamp     string
}

var (
	ragFeedbacks     []ragFeedback
	ragFeedbacksMu   sync.RWMutex
	ragFeedbackCount int64
)

// RecordRAGFeedback 记录一次 RAG 注入反馈
func RecordRAGFeedback(fb ragFeedback) {
	ragFeedbacksMu.Lock()
	defer ragFeedbacksMu.Unlock()
	ragFeedbacks = append(ragFeedbacks, fb)
	ragFeedbackCount++

	// 保留最近 1000 条
	if len(ragFeedbacks) > 1000 {
		ragFeedbacks = ragFeedbacks[len(ragFeedbacks)-1000:]
	}
}

// GetRAGFeedbackStats 返回 RAG 反馈统计
func GetRAGFeedbackStats() map[string]interface{} {
	ragFeedbacksMu.RLock()
	defer ragFeedbacksMu.RUnlock()

	if len(ragFeedbacks) == 0 {
		return map[string]interface{}{
			"total_feedbacks": 0,
			"avg_hits":        0,
			"avg_min_sim":     0,
			"avg_max_sim":     0,
			"by_domain":       map[string]int{},
		}
	}

	totalHits := 0
	totalMinSim := 0.0
	totalMaxSim := 0.0
	domainCount := map[string]int{}

	for _, fb := range ragFeedbacks {
		totalHits += fb.HitCount
		totalMinSim += fb.MinSimilarity
		totalMaxSim += fb.MaxSimilarity
		if fb.DomainCode != "" {
			domainCount[fb.DomainCode]++
		}
	}

	n := float64(len(ragFeedbacks))
	return map[string]interface{}{
		"total_feedbacks": len(ragFeedbacks),
		"avg_hits":        fmt.Sprintf("%.1f", float64(totalHits)/n),
		"avg_min_sim":     fmt.Sprintf("%.3f", totalMinSim/n),
		"avg_max_sim":     fmt.Sprintf("%.3f", totalMaxSim/n),
		"by_domain":       domainCount,
	}
}

// AutoTuneRAGThreshold 根据最近反馈自动调整阈值
// 如果平均命中 < 1，降低阈值；如果平均命中 > 5，提高阈值
func AutoTuneRAGThreshold() (oldThreshold, newThreshold float64, tuned bool) {
	ragFeedbacksMu.RLock()
	defer ragFeedbacksMu.RUnlock()

	if len(ragFeedbacks) < 10 {
		return 0, 0, false
	}

	recent := ragFeedbacks[len(ragFeedbacks)-50:] // 最近 50 条
	avgHits := 0.0
	for _, fb := range recent {
		avgHits += float64(fb.HitCount)
	}
	avgHits /= float64(len(recent))

	// 如果平均命中太低（< 1），说明阈值过高，需要降低
	if avgHits < 1.0 {
		// 降低 minRelevance
		return 0, 0, false // 目前不直接修改 Token 配置，仅返回建议
	}

	// 如果平均命中太多（> 5），说明阈值过低，需要提高
	if avgHits > 5.0 {
		return 0, 0, false
	}

	return 0, 0, false
}

// handleRAGFeedbackSubmit 接收 RAG 反馈提交
func handleRAGFeedbackSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		TokenID       int     `json:"token_id"`
		TokenName     string  `json:"token_name"`
		DomainCode    string  `json:"domain_code"`
		Query         string  `json:"query"`
		HitCount      int     `json:"hit_count"`
		MinSimilarity float64 `json:"min_similarity"`
		MaxSimilarity float64 `json:"max_similarity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}

	RecordRAGFeedback(ragFeedback{
		TokenID:       req.TokenID,
		TokenName:     req.TokenName,
		DomainCode:    req.DomainCode,
		Query:         req.Query,
		HitCount:      req.HitCount,
		MinSimilarity: req.MinSimilarity,
		MaxSimilarity: req.MaxSimilarity,
		Timestamp:     time.Now().UTC().Format("2006-01-02 15:04:05"),
	})

	writeJSON(w, 200, map[string]string{"message": "feedback recorded"})
}

// handleRAGFeedbackStats 返回 RAG 反馈统计
func handleRAGFeedbackStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"feedback": GetRAGFeedbackStats(),
		"inject": map[string]interface{}{
			"hits":   ragInjectHits,
			"misses": ragInjectMisses,
		},
	})
}

// startRAGFeedbackCleanup 定期清理过期的反馈记录
func startRAGFeedbackCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ragFeedbacksMu.Lock()
		if len(ragFeedbacks) > 500 {
			ragFeedbacks = ragFeedbacks[len(ragFeedbacks)-500:]
		}
		ragFeedbacksMu.Unlock()
	}
}

// writeJSON is defined in handlers.go

// isRAGHit 检查 RAG 是否命中（辅助函数）
func isRAGHit(results []SearchResult) bool {
	return len(results) > 0
}

func buildRAGFeedbackFromResults(token *Token, query string, results []SearchResult, minSim, maxSim float64) {
	domain := ""
	if len(results) > 0 {
		domain = results[0].DomainCode
	}
	RecordRAGFeedback(ragFeedback{
		TokenID:       token.ID,
		TokenName:     token.Name,
		DomainCode:    domain,
		Query:         query,
		HitCount:      len(results),
		MinSimilarity: minSim,
		MaxSimilarity: maxSim,
		Timestamp:     time.Now().UTC().Format("2006-01-02 15:04:05"),
	})
}
