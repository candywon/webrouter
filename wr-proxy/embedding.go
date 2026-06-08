// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// ============================================================
// DashScope Embedding 客户端 — 异步向量生成
// ============================================================

type embeddingConfig struct {
	enabled    bool
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	timeoutSec int
}

var embeddingCfg embeddingConfig

func initEmbeddingConfig() {
	embeddingCfg = embeddingConfig{
		enabled:    envStrDefault("WR_EMBEDDING_ENABLED", "0") == "1",
		baseURL:    envStrDefault("WR_EMBEDDING_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode"),
		apiKey:     envStrDefault("WR_EMBEDDING_API_KEY", ""),
		model:      envStrDefault("WR_EMBEDDING_MODEL", "text-embedding-v3"),
		dimension:  envIntDefault("WR_EMBEDDING_DIMENSION", "1024"),
		timeoutSec: envIntDefault("WR_EMBEDDING_TIMEOUT", "30"),
	}
}

type embeddingTask struct {
	itemID int
	text   string
}

var (
	embeddingCh   chan embeddingTask
	embeddingOnce sync.Once
)

// InitEmbedding 启动 embedding 异步 worker
func InitEmbedding() {
	initEmbeddingConfig()
	initEmbeddingProviders()
	if !embeddingCfg.enabled {
		LogInfo("Embedding: DISABLED (set WR_EMBEDDING_ENABLED=1 to enable)")
		return
	}

	embeddingOnce.Do(func() {
		embeddingCh = make(chan embeddingTask, 256)
		go embeddingWorker()
		provider := GetEmbeddingProvider(envStrDefault("WR_EMBEDDING_PROVIDER", "dashscope"))
		if provider != nil {
			LogInfo("Embedding: ENABLED (provider=%s, model=%s, dim=%d)", provider.Name(), provider.Model(), provider.Dimension())
		} else {
			LogInfo("Embedding: ENABLED (model=%s, dim=%d)", embeddingCfg.model, embeddingCfg.dimension)
		}
	})
}

// QueueEmbedding 将任务投递到 embedding 队列（非阻塞）
func QueueEmbedding(itemID int, text string) {
	if !embeddingCfg.enabled {
		return
	}
	select {
	case embeddingCh <- embeddingTask{itemID: itemID, text: text}:
	default:
		LogWarn("[embedding] channel full, dropping item %d", itemID)
	}
}

// embeddingWorker 消费队列，调用 Embedding Provider 生成向量
func embeddingWorker() {
	providerName := envStrDefault("WR_EMBEDDING_PROVIDER", "dashscope")

	for task := range embeddingCh {
		provider := GetEmbeddingProvider(providerName)
		if provider == nil {
			LogWarn("[embedding] provider %s not registered", providerName)
			continue
		}
		vector, err := provider.Embed(task.text)
		if err != nil {
			LogWarn("[embedding] failed for item %d: %v", task.itemID, err)
			continue
		}
		if err := saveKnowledgeVector(task.itemID, vector); err != nil {
			LogWarn("[embedding] save vector failed for item %d: %v", task.itemID, err)
		}
	}
}

// callDashScopeEmbedding 保留向后兼容的包装函数
func callDashScopeEmbedding(text string) ([]float64, error) {
	provider := GetEmbeddingProvider("dashscope")
	if provider == nil {
		return nil, fmt.Errorf("DashScope embedding provider not initialized")
	}
	return provider.Embed(text)
}

// EmbeddingBackfill 为缺少 embedding 的知识条目批量生成向量
func EmbeddingBackfill(limit int) (int, error) {
	provider := GetEmbeddingProvider(envStrDefault("WR_EMBEDDING_PROVIDER", "dashscope"))
	if provider == nil {
		return 0, fmt.Errorf("no embedding provider available")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	rows, err := db.Query(`
		SELECT i.id, i.title, i.summary
		FROM wr_knowledge_items i
		LEFT JOIN wr_knowledge_vectors v ON v.item_id = i.id
		WHERE v.item_id IS NULL
		ORDER BY i.id ASC
		LIMIT ?`, limit)
	if err != nil {
		return 0, fmt.Errorf("query items without embedding: %w", err)
	}
	defer rows.Close()

	type item struct {
		id      int
		title   string
		summary string
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.title, &it.summary); err != nil {
			continue
		}
		text := it.summary
		if text == "" {
			text = it.title
		}
		if text == "" {
			continue
		}
		items = append(items, it)
	}

	processed := 0
	for _, it := range items {
		vector, err := provider.Embed(it.summary)
		if err != nil {
			LogWarn("[backfill] embedding failed for item %d: %v", it.id, err)
			continue
		}
		if err := saveKnowledgeVector(it.id, vector); err != nil {
			LogWarn("[backfill] save failed for item %d: %v", it.id, err)
			continue
		}
		processed++
	}

	return processed, nil
}

// saveKnowledgeVector 保存向量到数据库
func saveKnowledgeVector(itemID int, vector []float64) error {
	vecJSON, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("marshal vector: %w", err)
	}

	_, err = db.Exec(`
		INSERT OR REPLACE INTO wr_knowledge_vectors
		(item_id, vector, model, dimension, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		itemID, string(vecJSON), embeddingCfg.model, embeddingCfg.dimension,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// envStrDefault 从环境变量读取字符串，带默认值
func envStrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envIntDefault 从环境变量读取整数，带默认值
func envIntDefault(key, def string) int {
	v := os.Getenv(key)
	if v == "" {
		v = def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		n, _ = strconv.Atoi(def)
	}
	return n
}
