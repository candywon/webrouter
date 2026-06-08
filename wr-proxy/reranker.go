// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"sort"
	"strings"
)

// ============================================================
// 轻量重排序 — query-term 重叠重排序
// ============================================================

// ReRankResults 对初始检索结果进行重排序
// strategy: "none" 不重排, "overlap" 词重叠重排
func ReRankResults(query string, results []SearchResult, strategy string) []SearchResult {
	if strategy == "" || strategy == "none" || len(results) <= 1 {
		return results
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return results
	}

	// 对每个结果计算重叠分
	type ranked struct {
		result SearchResult
		score  float64
	}

	rankedList := make([]ranked, len(results))
	for i, r := range results {
		docText := strings.ToLower(r.Title + " " + r.Summary)
		overlapCount := 0
		for _, qt := range queryTerms {
			if strings.Contains(docText, qt) {
				overlapCount++
			}
		}
		overlapRatio := float64(overlapCount) / float64(len(queryTerms))
		// 新分 = 原始相似度 * (1 + 重叠率 * 0.2)
		newScore := r.Similarity * (1 + overlapRatio*0.2)
		rankedList[i] = ranked{result: r, score: newScore}
	}

	// 按新分降序排列
	sort.Slice(rankedList, func(i, j int) bool {
		return rankedList[i].score > rankedList[j].score
	})

	result := make([]SearchResult, len(rankedList))
	for i, r := range rankedList {
		result[i] = r.result
		result[i].Similarity = r.score
	}
	return result
}
