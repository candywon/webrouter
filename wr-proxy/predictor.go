// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// 额度预测引擎：推算 Provider 额度耗尽时间

import (
	"fmt"
	"math"
	"time"
)

// Predictor 额度预测器
type Predictor struct {
	cfg *Config
}

var predictor = &Predictor{}

// PredictExhaustion 预测 Provider 额度耗尽时间
func (pr *Predictor) PredictExhaustion(provider *Provider) *QuotaPrediction {
	if provider.QuotaTotal <= 0 {
		// 无额度信息，无法预测
		return &QuotaPrediction{
			ProviderID:     provider.ID,
			QuotaRemaining: -1,
			AlertLevel:     "green",
			Trend:          "unknown",
			Confidence:     0,
		}
	}

	remaining := provider.QuotaRemaining()
	ratio := provider.QuotaRatio()

	// 从 DB 取近 7 天每日用量
	days := cfg.PredictionDays
	if days <= 0 {
		days = 7
	}
	dailyCosts := pr.getDailyCosts(provider.ID, days)

	// 计算日均消耗
	burnRate := pr.calculateBurnRate(dailyCosts)
	if burnRate <= 0 {
		// 无消耗记录
		return &QuotaPrediction{
			ProviderID:       provider.ID,
			QuotaRemaining:   remaining,
			DailyBurnRate:    0,
			DaysUntilExhaust: math.Inf(1),
			Trend:            "stable",
			AlertLevel:       pr.alertLevel(ratio, math.Inf(1)),
			Confidence:       0.3,
		}
	}

	// 预测耗尽天数
	daysUntil := float64(remaining) / burnRate

	// 趋势判断
	trend := pr.detectTrend(dailyCosts)

	// 置信度（数据天数越多越可信）
	confidence := pr.calculateConfidence(dailyCosts)

	prediction := &QuotaPrediction{
		ProviderID:           provider.ID,
		QuotaRemaining:       remaining,
		DailyBurnRate:        burnRate,
		DaysUntilExhaust:     daysUntil,
		PredictedExhaustDate: pr.exhaustDate(daysUntil),
		Trend:                trend,
		Confidence:           confidence,
		AlertLevel:           pr.alertLevel(ratio, daysUntil),
	}

	return prediction
}

// PredictAll 预测所有 Provider
func (pr *Predictor) PredictAll() []*QuotaPrediction {
	providers := router.GetProviders()
	predictions := make([]*QuotaPrediction, 0, len(providers))
	for _, p := range providers {
		if p.QuotaTotal > 0 {
			predictions = append(predictions, pr.PredictExhaustion(p))
		}
	}
	return predictions
}

// getDailyCosts 获取 Provider 近 N 天每日成本
func (pr *Predictor) getDailyCosts(providerID, days int) []float64 {
	rows, err := db.Query(`
		SELECT DATE(created_at) as date, COALESCE(SUM(cost_cents), 0) as cost
		FROM wr_request_logs
		WHERE provider_id = ? AND created_at >= datetime('now', ?)
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, providerID, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil
	}
	defer rows.Close()

	var costs []float64
	for rows.Next() {
		var date string
		var cost int64
		if rows.Scan(&date, &cost) == nil {
			costs = append(costs, float64(cost))
		}
	}
	return costs
}

// calculateBurnRate 计算日均消耗（线性回归）
func (pr *Predictor) calculateBurnRate(dailyCosts []float64) float64 {
	if len(dailyCosts) == 0 {
		return 0
	}

	// 简单平均（数据少时）
	if len(dailyCosts) < 3 {
		var sum float64
		for _, c := range dailyCosts {
			sum += c
		}
		return sum / float64(len(dailyCosts))
	}

	// 加权平均：近期权重更高
	var weightedSum, weightTotal float64
	n := float64(len(dailyCosts))
	for i, c := range dailyCosts {
		weight := float64(i+1) / n // 越近期权重越大
		weightedSum += c * weight
		weightTotal += weight
	}
	return weightedSum / weightTotal
}

// detectTrend 检测消耗趋势
func (pr *Predictor) detectTrend(dailyCosts []float64) string {
	if len(dailyCosts) < 3 {
		return "stable"
	}

	// 比较前半和后半的均值
	mid := len(dailyCosts) / 2
	firstHalf := avg(dailyCosts[:mid])
	secondHalf := avg(dailyCosts[mid:])

	ratio := secondHalf / firstHalf
	switch {
	case ratio > 1.2:
		return "increasing" // 用量加速
	case ratio < 0.8:
		return "decreasing" // 用量减少
	default:
		return "stable"
	}
}

// alertLevel 判定预警级别
func (pr *Predictor) alertLevel(quotaRatio float64, daysUntil float64) string {
	criticalThreshold := cfg.QuotaCriticalThreshold
	warnThreshold := cfg.QuotaWarnThreshold
	if criticalThreshold <= 0 {
		criticalThreshold = 0.05
	}
	if warnThreshold <= 0 {
		warnThreshold = 0.2
	}
	switch {
	case quotaRatio <= 0:
		return "black"
	case quotaRatio < criticalThreshold:
		return "red"
	case daysUntil < 1:
		return "red"
	case quotaRatio < warnThreshold:
		return "orange"
	case daysUntil < 3:
		return "orange"
	case quotaRatio < 0.5:
		return "yellow"
	case daysUntil < 7:
		return "yellow"
	default:
		return "green"
	}
}

// calculateConfidence 计算预测置信度
func (pr *Predictor) calculateConfidence(dailyCosts []float64) float64 {
	n := len(dailyCosts)
	if n >= 7 {
		return 0.9
	}
	if n >= 3 {
		return 0.7
	}
	return 0.4
}

// exhaustDate 推算耗尽日期
func (pr *Predictor) exhaustDate(daysUntil float64) string {
	if math.IsInf(daysUntil, 1) || math.IsInf(daysUntil, -1) {
		return "never"
	}
	if daysUntil <= 0 {
		return "exhausted"
	}
	return time.Now().Add(time.Duration(daysUntil*24) * time.Hour).Format("2006-01-02")
}

func avg(nums []float64) float64 {
	if len(nums) == 0 {
		return 0
	}
	var sum float64
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums))
}
