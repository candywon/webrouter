// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"math"
	"sort"
	"strings"
	"sync"
)

// ============================================================
// BM25 检索算法 — 纯 Go 实现，无外部依赖
// ============================================================

// BM25Index BM25 索引
type BM25Index struct {
	mu           sync.RWMutex
	docCount     int
	avgDocLength float64
	docLengths   map[int]int            // docID → 文档长度（词数）
	docTerms     map[int]map[string]int // docID → term → 词频
	idf          map[string]float64     // term → IDF 值
	docs         []bm25Doc
}

type bm25Doc struct {
	docID   int
	title   string
	summary string
}

const (
	bm25K1 = 1.2  // BM25 k1 参数
	bm25B  = 0.75 // BM25 b 参数
)

// NewBM25Index 创建新的 BM25 索引
func NewBM25Index() *BM25Index {
	return &BM25Index{
		docLengths: make(map[int]int),
		docTerms:   make(map[int]map[string]int),
		idf:        make(map[string]float64),
	}
}

// tokenize 简单分词（按非字母数字字符分割，转小写）
func tokenize(text string) []string {
	var tokens []string
	var buf strings.Builder
	for _, ch := range strings.ToLower(text) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || (ch >= '一' && ch <= '鿿') {
			buf.WriteRune(ch)
		} else {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
		}
	}
	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}
	return tokens
}

// Build 从向量缓存条目构建 BM25 索引
func (idx *BM25Index) Build(items []VectorCacheItem) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 重置
	idx.docCount = 0
	idx.avgDocLength = 0
	idx.docLengths = make(map[int]int)
	idx.docTerms = make(map[int]map[string]int)
	idx.idf = make(map[string]float64)
	idx.docs = nil

	if len(items) == 0 {
		return
	}

	// 统计词频和文档频率
	docFreq := make(map[string]int) // 包含该 term 的文档数
	totalLength := 0

	for _, item := range items {
		text := item.Title + " " + item.Summary
		tokens := tokenize(text)
		length := len(tokens)
		totalLength += length

		idx.docLengths[item.ItemID] = length
		idx.docs = append(idx.docs, bm25Doc{
			docID:   item.ItemID,
			title:   item.Title,
			summary: item.Summary,
		})

		// 统计该文档中的词频
		termFreq := make(map[string]int)
		seen := make(map[string]bool)
		for _, token := range tokens {
			termFreq[token]++
			if !seen[token] {
				docFreq[token]++
				seen[token] = true
			}
		}
		idx.docTerms[item.ItemID] = termFreq
	}

	idx.docCount = len(items)
	idx.avgDocLength = float64(totalLength) / float64(len(items))

	// 计算 IDF
	for term, df := range docFreq {
		idx.idf[term] = math.Log(1 + (float64(idx.docCount)-float64(df)+0.5)/(float64(df)+0.5))
	}
}

// Score 计算 BM25 分数：query 对 docID 的匹配度
func (idx *BM25Index) Score(query string, docID int) float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	terms := tokenize(query)
	docLength := idx.docLengths[docID]
	termFreqs := idx.docTerms[docID]

	if docLength == 0 || termFreqs == nil {
		return 0
	}

	var score float64
	seen := make(map[string]bool)
	for _, term := range terms {
		if seen[term] {
			continue
		}
		seen[term] = true

		idf := idx.idf[term]
		if idf == 0 {
			continue
		}

		tf := float64(termFreqs[term])
		numer := tf * (bm25K1 + 1)
		denom := tf + bm25K1*(1-bm25B+bm25B*float64(docLength)/idx.avgDocLength)
		score += idf * numer / denom
	}

	return score
}

// Search 对 query 在所有文档上计算 BM25 分数，返回 topN
func (idx *BM25Index) Search(query string, topN int) []struct {
	DocID int
	Score float64
} {
	idx.mu.RLock()
	docs := make([]bm25Doc, len(idx.docs))
	copy(docs, idx.docs)
	idx.mu.RUnlock()

	type scoredDoc struct {
		DocID int
		Score float64
	}

	var results []scoredDoc
	for _, d := range docs {
		s := idx.Score(query, d.docID)
		if s > 0 {
			results = append(results, scoredDoc{DocID: d.docID, Score: s})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topN {
		results = results[:topN]
	}

	result := make([]struct {
		DocID int
		Score float64
	}, len(results))
	for i, r := range results {
		result[i] = struct {
			DocID int
			Score float64
		}{r.DocID, r.Score}
	}
	return result
}

// globalBM25 全局 BM25 索引（在向量缓存刷新时同步构建）
var globalBM25 = NewBM25Index()
