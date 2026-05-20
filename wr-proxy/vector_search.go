package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ============================================================
// 向量缓存 + 余弦相似度语义检索
// ============================================================

// VectorCacheItem 内存中的向量条目（归一化）
type VectorCacheItem struct {
	ItemID     int
	Title      string
	Summary    string
	DomainCode string
	Type       string
	Department string
	Vector     []float64 // 已归一化的单位向量
}

// SearchResult 语义检索结果
type SearchResult struct {
	ItemID     int     `json:"item_id"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	DomainCode string  `json:"domain_code"`
	Type       string  `json:"type"`
	Similarity float64 `json:"similarity"`
}

var vectorCache = &struct {
	mu    sync.RWMutex
	items []VectorCacheItem
	loaded time.Time
}{}

// InitVectorCache 加载向量缓存并启动定时刷新
func InitVectorCache() {
	go vectorCacheRefresher()
}

// LoadVectorCacheFull 从数据库加载所有向量到内存
func LoadVectorCacheFull() error {
	rows, err := db.Query(`
		SELECT i.id, i.title, i.summary, i.domain_code, i.type, i.department, v.vector
		FROM wr_knowledge_items i
		JOIN wr_knowledge_vectors v ON v.item_id = i.id
		WHERE i.verification != 'rejected'
		ORDER BY i.id ASC`)
	if err != nil {
		return fmt.Errorf("query vectors: %w", err)
	}
	defer rows.Close()

	var items []VectorCacheItem
	for rows.Next() {
		var id int
		var title, summary, domainCode, typ, department, vectorStr string
		if err := rows.Scan(&id, &title, &summary, &domainCode, &typ, &department, &vectorStr); err != nil {
			continue
		}

		var vec []float64
		if err := json.Unmarshal([]byte(vectorStr), &vec); err != nil {
			continue
		}
		if len(vec) == 0 {
			continue
		}

		items = append(items, VectorCacheItem{
			ItemID:     id,
			Title:      title,
			Summary:    summary,
			DomainCode: domainCode,
			Type:       typ,
			Department: department,
			Vector:     normalizeVec(vec),
		})
	}

	vectorCache.mu.Lock()
	vectorCache.items = items
	vectorCache.loaded = time.Now()
	vectorCache.mu.Unlock()

	LogInfo("[vector_cache] loaded %d vectors", len(items))
	return nil
}

// vectorCacheRefresher 定时刷新向量缓存
func vectorCacheRefresher() {
	// 启动后先加载一次
	if err := LoadVectorCacheFull(); err != nil {
		LogWarn("[vector_cache] initial load failed: %v", err)
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := LoadVectorCacheFull(); err != nil {
			LogWarn("[vector_cache] refresh failed: %v", err)
		}
	}
}

// SearchVectors 语义检索：将 query 向量化后与缓存做余弦相似度 top-k
func SearchVectors(query string, topK int, minRelevance float64, domainFilter, typeFilter string) ([]SearchResult, error) {
	// 1. 将 query 向量化
	queryVec, err := callDashScopeEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	queryVec = normalizeVec(queryVec)

	// 2. 读取缓存快照
	vectorCache.mu.RLock()
	items := vectorCache.items
	vectorCache.mu.RUnlock()

	if len(items) == 0 {
		return []SearchResult{}, nil
	}

	// 3. 计算相似度
	var results []SearchResult
	for _, item := range items {
		// 可选过滤
		if domainFilter != "" && item.DomainCode != domainFilter {
			continue
		}
		if typeFilter != "" && item.Type != typeFilter {
			continue
		}

		sim := dotProduct(queryVec, item.Vector)
		if sim >= minRelevance {
			results = append(results, SearchResult{
				ItemID:     item.ItemID,
				Title:      item.Title,
				Summary:    item.Summary,
				DomainCode: item.DomainCode,
				Type:       item.Type,
				Similarity: roundTo(sim, 4),
			})
		}
	}

	// 4. 排序取 top-k
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// normalizeVec 归一化向量
func normalizeVec(v []float64) []float64 {
	norm := 0.0
	for _, x := range v {
		norm += x * x
	}
	if norm == 0 {
		return v
	}
	norm = math.Sqrt(norm)
	result := make([]float64, len(v))
	for i, x := range v {
		result[i] = x / norm
	}
	return result
}

// dotProduct 计算两个向量的点积（假设已归一化）
func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	dot := 0.0
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}

// roundTo 保留指定位数小数
func roundTo(x float64, n int) float64 {
	p := math.Pow10(n)
	return math.Round(x*p) / p
}
