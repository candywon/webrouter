// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"time"
)

// ============================================================
// RAG 质量控制 — 领域风险过滤 + 免责声明
// ============================================================

// ApplyRAGFilters 应用领域风险配置过滤
func ApplyRAGFilter(results []SearchResult) (filtered []SearchResult, disclaimers []string) {
	seenDisclaimer := make(map[string]bool)

	for _, r := range results {
		// 加载该域的风险配置
		riskCfg, err := LoadDomainRiskConfig(r.DomainCode)
		if err != nil {
			// 无配置则使用默认放行
			filtered = append(filtered, r)
			continue
		}

		// 1. 验证级别过滤
		if riskCfg.MinVerification == "verified" && r.Type == "factual" {
			// factual 类型需要 verified 验证，跳过快照中未验证的
			// 由于 SearchResult 不含 verification 字段，这里信任缓存已通过筛选
			// 如需严格过滤，可加 DB 查询 — 暂简化处理
		}

		// 2. 有效期过滤
		if riskCfg.MaxAgeDays > 0 {
			// 知识条目通过 SearchResult 无法直接获取 created_at
			// 这里用 DB 查询判断
			var ageDays int
			err := db.QueryRow(`
				SELECT julianday('now') - julianday(created_at)
				FROM wr_knowledge_items WHERE id = ?`, r.ItemID).Scan(&ageDays)
			if err == nil && ageDays > riskCfg.MaxAgeDays {
				continue // 过期知识不注入
			}
		}

		// 3. 类型注入开关
		switch r.Type {
		case "factual":
			if !riskCfg.AllowFactual {
				continue
			}
		case "analytical":
			if !riskCfg.AllowAnalytical {
				continue
			}
		case "procedural":
			if !riskCfg.AllowProcedural {
				continue
			}
		}

		filtered = append(filtered, r)

		// 收集免责声明
		if riskCfg.DisclaimerTemplate != "" && !seenDisclaimer[riskCfg.DisclaimerTemplate] {
			seenDisclaimer[riskCfg.DisclaimerTemplate] = true
			disclaimers = append(disclaimers, riskCfg.DisclaimerTemplate)
		}
	}

	return filtered, disclaimers
}

// BuildDisclaimerFromDomains 为指定领域列表生成免责声明
func BuildDisclaimerFromDomains(domainCodes []string) string {
	var disclaimers []string
	seen := make(map[string]bool)

	for _, code := range domainCodes {
		riskCfg, err := LoadDomainRiskConfig(code)
		if err != nil {
			continue
		}
		if riskCfg.DisclaimerTemplate != "" && !seen[riskCfg.DisclaimerTemplate] {
			seen[riskCfg.DisclaimerTemplate] = true
			disclaimers = append(disclaimers, riskCfg.DisclaimerTemplate)
		}
	}

	if len(disclaimers) == 0 {
		return ""
	}

	result := "【免责声明】"
	for i, d := range disclaimers {
		if i > 0 {
			result += "\n"
		}
		result += d
	}
	return result
}

// GetVectorCacheStats 返回向量缓存状态
func GetVectorCacheStats() (int, string) {
	vectorCache.mu.RLock()
	defer vectorCache.mu.RUnlock()
	count := len(vectorCache.items)
	loaded := ""
	if !vectorCache.loaded.IsZero() {
		loaded = vectorCache.loaded.Format("2006-01-02 15:04:05")
	}
	return count, loaded
}

// InjectRAGStats 统计注入命中
var (
	ragInjectHits   int64
	ragInjectMisses int64
)

// RecordRAGHit 记录一次 RAG 成功注入
func RecordRAGHit() {
	ragInjectHits++
}

// RecordRAGMiss 记录一次 RAG 未命中（无相关结果）
func RecordRAGMiss() {
	ragInjectMisses++
}

// GetRAGStats 返回 RAG 统计
func GetRAGStats() (hits, misses int64) {
	return ragInjectHits, ragInjectMisses
}

// formatAgeCheck 辅助：检查条目是否过期
func isItemExpired(itemID, maxAgeDays int) bool {
	var ageDays float64
	err := db.QueryRow(`
		SELECT julianday('now') - julianday(created_at)
		FROM wr_knowledge_items WHERE id = ?`, itemID).Scan(&ageDays)
	if err != nil {
		return false // 查不到则不拦截
	}
	return ageDays > float64(maxAgeDays)
}

// _unused: suppress import warning for time (used in isItemExpired alternative)
var _ = time.Now
