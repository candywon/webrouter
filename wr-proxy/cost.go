// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// 成本估算：从 DB 加载定价表 + 内存缓存 + 热刷新

import (
	"sync"
)

// ModelPricing 模型定价（单位：分/千token）
type ModelPricing struct {
	Input  float64 // 输入价格
	Output float64 // 输出价格
}

// PricingCache 定价缓存
type PricingCache struct {
	mu       sync.RWMutex
	table    map[string]ModelPricing
	default_ ModelPricing // 未知模型默认价格
}

var pricingCache = &PricingCache{
	table:    make(map[string]ModelPricing),
	default_: ModelPricing{Input: 0.015, Output: 0.06}, // gpt-4o-mini 级别
}

// RefreshPricing 从 DB 重新加载定价表到内存
func RefreshPricing() error {
	rows, err := db.Query(`
		SELECT model, input_price, output_price, is_default
		FROM wr_model_pricing
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	newTable := make(map[string]ModelPricing)
	newDefault := ModelPricing{Input: 0.015, Output: 0.06}

	for rows.Next() {
		var model string
		var inputPrice, outputPrice float64
		var isDefault bool

		if err := rows.Scan(&model, &inputPrice, &outputPrice, &isDefault); err != nil {
			LogWarn("scan pricing: %v", err)
			continue
		}

		p := ModelPricing{Input: inputPrice, Output: outputPrice}
		if isDefault {
			newDefault = p
		} else {
			newTable[model] = p
		}
	}

	pricingCache.mu.Lock()
	pricingCache.table = newTable
	pricingCache.default_ = newDefault
	pricingCache.mu.Unlock()

	LogInfo("Pricing: refreshed %d models, default={in=%.4f, out=%.4f}",
		len(newTable), newDefault.Input, newDefault.Output)
	return nil
}

// CalculateCost 计算请求成本（分）
func CalculateCost(model string, inputTokens, outputTokens int64, multiplier float64) int64 {
	pricingCache.mu.RLock()
	p, ok := pricingCache.table[model]
	d := pricingCache.default_
	pricingCache.mu.RUnlock()

	if !ok {
		p = d
	}

	inputCost := float64(inputTokens) / 1000.0 * p.Input
	outputCost := float64(outputTokens) / 1000.0 * p.Output
	total := (inputCost + outputCost) * multiplier

	// 最低 1 分
	cents := int64(total + 0.5)
	if cents < 1 && (inputTokens > 0 || outputTokens > 0) {
		cents = 1
	}
	return cents
}

// GetModelPricing 获取模型定价
func GetModelPricing(model string) (ModelPricing, bool) {
	pricingCache.mu.RLock()
	defer pricingCache.mu.RUnlock()
	p, ok := pricingCache.table[model]
	return p, ok
}

// GetAllPricing 获取全部定价表（供 admin/stats 使用）
func GetAllPricing() map[string]ModelPricing {
	pricingCache.mu.RLock()
	defer pricingCache.mu.RUnlock()

	// 返回副本
	result := make(map[string]ModelPricing, len(pricingCache.table)+1)
	for k, v := range pricingCache.table {
		result[k] = v
	}
	result["__default__"] = pricingCache.default_
	return result
}
