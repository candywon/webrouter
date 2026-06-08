// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ============================================================
// Embedding Provider 接口 -- 支持多种 Embedding 服务
// ============================================================

// EmbeddingProvider 向量生成接口
type EmbeddingProvider interface {
	Name() string
	Embed(text string) ([]float64, error)
	Model() string
	Dimension() int
}

// embeddingProviders 已注册的 provider 实例
var embeddingProviders = make(map[string]EmbeddingProvider)

// GetEmbeddingProvider 根据名称获取 Embedding Provider
func GetEmbeddingProvider(name string) EmbeddingProvider {
	if p, ok := embeddingProviders[name]; ok {
		return p
	}
	// 默认返回 DashScope
	if p, ok := embeddingProviders["dashscope"]; ok {
		return p
	}
	return nil
}

// RegisterEmbeddingProvider 注册 Embedding Provider
func RegisterEmbeddingProvider(name string, p EmbeddingProvider) {
	embeddingProviders[name] = p
}

// --- DashScope Provider ---

type DashScopeProvider struct {
	baseURL   string
	apiKey    string
	model     string
	dimension int
	timeout   time.Duration
}

func NewDashScopeProvider(baseURL, apiKey, model string, dimension int, timeout int) *DashScopeProvider {
	return &DashScopeProvider{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		dimension: dimension,
		timeout:   time.Duration(timeout) * time.Second,
	}
}

func (p *DashScopeProvider) Name() string   { return "dashscope" }
func (p *DashScopeProvider) Model() string  { return p.model }
func (p *DashScopeProvider) Dimension() int { return p.dimension }

func (p *DashScopeProvider) Embed(text string) ([]float64, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("DashScope API key not configured")
	}
	if len(text) > 6000 {
		text = text[:6000] + "..."
	}

	payload := map[string]interface{}{
		"model":      p.model,
		"input":      []string{text},
		"dimensions": p.dimension,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", p.baseURL+"/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: p.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call DashScope: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DashScope status %d: %s", resp.StatusCode, truncateStr(string(respBody), 200))
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return result.Data[0].Embedding, nil
}

// --- OpenAI-compatible Provider (covers OpenAI, DeepSeek, Zhipu, etc.) ---

type OpenAIEmbeddingProvider struct {
	baseURL   string
	apiKey    string
	model     string
	dimension int
	timeout   time.Duration
}

func NewOpenAIEmbeddingProvider(baseURL, apiKey, model string, dimension int, timeout int) *OpenAIEmbeddingProvider {
	return &OpenAIEmbeddingProvider{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		dimension: dimension,
		timeout:   time.Duration(timeout) * time.Second,
	}
}

func (p *OpenAIEmbeddingProvider) Name() string   { return "openai" }
func (p *OpenAIEmbeddingProvider) Model() string  { return p.model }
func (p *OpenAIEmbeddingProvider) Dimension() int { return p.dimension }

func (p *OpenAIEmbeddingProvider) Embed(text string) ([]float64, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OpenAI embedding API key not configured")
	}
	if len(text) > 8000 {
		text = text[:8000] + "..."
	}

	payload := map[string]interface{}{
		"model": p.model,
		"input": []string{text},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", p.baseURL+"/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: p.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call OpenAI embedding: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI embedding status %d: %s", resp.StatusCode, truncateStr(string(respBody), 200))
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return result.Data[0].Embedding, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// initEmbeddingProviders 初始化所有注册的 Embedding Provider
func initEmbeddingProviders() {
	// DashScope (默认)
	RegisterEmbeddingProvider("dashscope", NewDashScopeProvider(
		envStrDefault("WR_EMBEDDING_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode"),
		envStrDefault("WR_EMBEDDING_API_KEY", ""),
		envStrDefault("WR_EMBEDDING_MODEL", "text-embedding-v3"),
		envIntDefault("WR_EMBEDDING_DIMENSION", "1024"),
		envIntDefault("WR_EMBEDDING_TIMEOUT", "30"),
	))

	// OpenAI-compatible (如果配置了单独的 key)
	if key := os.Getenv("WR_EMBEDDING_OPENAI_API_KEY"); key != "" {
		RegisterEmbeddingProvider("openai", NewOpenAIEmbeddingProvider(
			envStrDefault("WR_EMBEDDING_OPENAI_BASE_URL", "https://api.openai.com"),
			key,
			envStrDefault("WR_EMBEDDING_OPENAI_MODEL", "text-embedding-3-small"),
			envIntDefault("WR_EMBEDDING_OPENAI_DIMENSION", "1536"),
			envIntDefault("WR_EMBEDDING_TIMEOUT", "30"),
		))
	}
}
