package main

// 智能调度引擎：选 Provider、分组、降级、热插拔

import (
	"math/rand"
	"sort"
	"sync"
)

// Router 调度引擎
type Router struct {
	mu        sync.RWMutex
	providers []*Provider
	strategy  string
}

var router = &Router{
	strategy: "smart",
}

// RefreshProviders 刷新 Provider 列表（热插拔核心）
func (r *Router) RefreshProviders(providers []*Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = providers
	LogInfo("Router: refreshed %d providers", len(providers))
}

// GetProviders 获取当前 Provider 列表快照
func (r *Router) GetProviders() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Provider, len(r.providers))
	copy(out, r.providers)
	return out
}

// SelectProvider 选择最优 Provider
// excludeIDs: 本次请求链中已失败的 Provider，避免循环
func (r *Router) SelectProvider(model string, token *Token, excludeIDs []int) *Provider {
	r.mu.RLock()
	providers := r.providers
	r.mu.RUnlock()

	// 1. 过滤
	var candidates []*Provider
	for _, p := range providers {
		if !p.IsAvailable(model) {
			continue
		}
		if token != nil && !token.CanUseProvider(p.ID) {
			continue
		}
		if intInSlice(p.ID, excludeIDs) {
			continue
		}
		// 额度紧急 → 跳过
		if p.QuotaTotal > 0 && p.QuotaRatio() < cfg.QuotaCriticalThreshold {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return nil
	}

	// 2. 分组: 主力 → 热备 → 冷备
	primary, hot, cold := groupByPriority(candidates)

	// 3. 按策略选
	switch r.strategy {
	case "least_latency":
		return selectFromGroups(selectLeastLatency, primary, hot, cold)
	case "cost_first":
		return selectFromGroups(selectCostFirst, primary, hot, cold)
	case "round_robin":
		return selectFromGroups(selectWeightedRandom, primary, hot, cold)
	default: // smart + priority
		return selectFromGroups(selectSmart, primary, hot, cold)
	}
}

// selectFromGroups 按分组顺序选，主力优先
func selectFromGroups(fn func([]*Provider) *Provider, groups ...[]*Provider) *Provider {
	for _, g := range groups {
		if len(g) > 0 {
			if p := fn(g); p != nil {
				return p
			}
		}
	}
	return nil
}

// --- 调度策略 ---

// selectSmart 综合：额度充裕度 > 延迟 > 成本 > 加权随机
func selectSmart(group []*Provider) *Provider {
	if len(group) == 1 {
		return group[0]
	}
	// 评分排序
	sort.SliceStable(group, func(i, j int) bool {
		ri, rj := group[i].QuotaRatio(), group[j].QuotaRatio()
		// 额度充裕的优先
		if ri > 0.5 && rj <= 0.5 {
			return true
		}
		if rj > 0.5 && ri <= 0.5 {
			return false
		}
		// 同级别比延迟
		return group[i].LastLatencyMs < group[j].LastLatencyMs
	})
	// 前3名加权随机，避免全走一个
	topN := min(3, len(group))
	weights := make([]int, topN)
	for i := range weights {
		weights[i] = 100 - i*30 // 100, 70, 40
	}
	return weightedPick(group[:topN], weights)
}

// selectLeastLatency 选延迟最低的
func selectLeastLatency(group []*Provider) *Provider {
	if len(group) == 1 {
		return group[0]
	}
	sort.Slice(group, func(i, j int) bool {
		return group[i].LastLatencyMs < group[j].LastLatencyMs
	})
	return group[0]
}

// selectCostFirst 选成本最低的
func selectCostFirst(group []*Provider) *Provider {
	if len(group) == 1 {
		return group[0]
	}
	sort.Slice(group, func(i, j int) bool {
		return group[i].CostMultiplier < group[j].CostMultiplier
	})
	return group[0]
}

// selectWeightedRandom 按 weight 加权随机
func selectWeightedRandom(group []*Provider) *Provider {
	if len(group) == 1 {
		return group[0]
	}
	weights := make([]int, len(group))
	for i, p := range group {
		if p.Weight <= 0 {
			weights[i] = 1
		} else {
			weights[i] = p.Weight
		}
	}
	return weightedPick(group, weights)
}

// --- 辅助 ---

func groupByPriority(providers []*Provider) (primary, hot, cold []*Provider) {
	for _, p := range providers {
		switch p.PriorityGroup() {
		case "primary":
			primary = append(primary, p)
		case "hot":
			hot = append(hot, p)
		default:
			cold = append(cold, p)
		}
	}
	return
}

// weightedPick 加权随机选择
func weightedPick(items []*Provider, weights []int) *Provider {
	total := 0
	for _, w := range weights {
		total += w
	}
	if total <= 0 {
		return items[0]
	}
	r := rand.Intn(total)
	cum := 0
	for i, w := range weights {
		cum += w
		if r < cum {
			return items[i]
		}
	}
	return items[0]
}

func intInSlice(id int, ids []int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
