package main

// 智能模型选择：根据请求复杂度自动匹配最优模型
// 两种模式：
//   1. model=auto/smart → 完全由网关选择模型
//   2. Token 启用 SmartDowngrade → 对指定强模型的简单请求自动降级

import (
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"
)

// ── 模型分级 ──

type ModelTier string

const (
	TierEconomy  ModelTier = "economy"  // 便宜快速
	TierStandard ModelTier = "standard" // 中等性价比
	TierPremium  ModelTier = "premium"  // 最强推理
)

// ModelGrade 模型分级定义
type ModelGrade struct {
	Model   string
	Tier    ModelTier
	CostIdx float64 // 相对成本指数，economy=1.0
}

// modelGrades 模型分级表（从 DB/配置加载，此处为默认值）
// 按成本从低到高排列
var modelGrades = []ModelGrade{
	// economy
	{"qwen3-coder-flash", TierEconomy, 1.0},
	{"qwen-turbo", TierEconomy, 1.0},
	{"gpt-4o-mini", TierEconomy, 1.5},
	// standard
	{"qwen-plus-2025-07-28", TierStandard, 3.0},
	{"qwen-plus", TierStandard, 3.0},
	{"gpt-4o", TierStandard, 5.0},
	{"deepseek-chat", TierStandard, 2.0},
	// premium
	{"qwen3.6-plus", TierPremium, 8.0},
	{"qwen-max", TierPremium, 10.0},
	{"o1", TierPremium, 15.0},
	{"o1-mini", TierPremium, 8.0},
	{"claude-sonnet-4", TierPremium, 12.0},
}

// ── 复杂度评估 ──

// ComplexityLevel 复杂度级别
type ComplexityLevel int

const (
	ComplexitySimple   ComplexityLevel = 0 // 简单
	ComplexityModerate ComplexityLevel = 1 // 中等
	ComplexityComplex  ComplexityLevel = 2 // 复杂
)

// ComplexityScore 复杂度评分结果
type ComplexityScore struct {
	Level      ComplexityLevel
	Score      float64 // 0~1 连续分数
	Reasons    []string
	InputChars int
	MsgCount   int
	HasCode    bool
	HasTools   bool
}

// EvalComplexity 评估请求内容的复杂度
func EvalComplexity(body []byte) ComplexityScore {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ComplexityScore{Level: ComplexityModerate, Score: 0.5, Reasons: []string{"parse_error"}}
	}

	score := ComplexityScore{
		Reasons: make([]string, 0),
	}

	// ── 1. 输入长度 ──
	totalChars := 0
	messages, _ := req["messages"].([]interface{})
	score.MsgCount = len(messages)

	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		content, _ := msg["content"].(string)
		totalChars += utf8.RuneCountInString(content)
	}
	score.InputChars = totalChars

	// 字数评分：0~0.3
	charScore := 0.0
	switch {
	case totalChars < 200:
		charScore = 0.05
	case totalChars < 800:
		charScore = 0.12
	case totalChars < 2000:
		charScore = 0.20
	default:
		charScore = 0.30
	}
	if totalChars > 200 {
		score.Reasons = append(score.Reasons, "long_input")
	}

	// ── 2. 多轮对话 ──
	msgScore := 0.0
	switch {
	case score.MsgCount <= 2:
		msgScore = 0.0
	case score.MsgCount <= 5:
		msgScore = 0.08
	case score.MsgCount <= 10:
		msgScore = 0.15
	default:
		msgScore = 0.20
	}
	if score.MsgCount > 3 {
		score.Reasons = append(score.Reasons, "multi_turn")
	}

	// ── 3. 代码检测 ──
	codeScore := 0.0
	fullContent := extractAllContent(messages)
	if strings.Contains(fullContent, "```") || strings.Contains(fullContent, "def ") ||
		strings.Contains(fullContent, "function ") || strings.Contains(fullContent, "class ") ||
		strings.Contains(fullContent, "import ") || strings.Contains(fullContent, "return ") {
		codeScore = 0.15
		score.HasCode = true
		score.Reasons = append(score.Reasons, "has_code")
	}

	// ── 4. Tools / Function Calling ──
	toolsScore := 0.0
	if _, hasTools := req["tools"]; hasTools {
		toolsScore = 0.20
		score.HasTools = true
		score.Reasons = append(score.Reasons, "has_tools")
	}
	if _, hasFunc := req["functions"]; hasFunc {
		toolsScore = math.Max(toolsScore, 0.15)
		score.HasTools = true
		score.Reasons = append(score.Reasons, "has_functions")
	}

	// ── 5. 推理/分析关键词 ──
	reasonScore := 0.0
	reasonKeywords := []string{
		"分析", "推理", "证明", "计算", "推导",
		"explain", "analyze", "reason", "prove", "calculate",
		"derive", "compare", "evaluate", "critique",
		"为什么", "原因", "原理", "逻辑",
		"步骤", "方案", "策略", "设计",
	}
	lastMsg := extractLastUserContent(messages)
	lowerLast := strings.ToLower(lastMsg)
	for _, kw := range reasonKeywords {
		if strings.Contains(lowerLast, kw) {
			reasonScore = 0.12
			score.Reasons = append(score.Reasons, "reasoning_keyword")
			break
		}
	}

	// ── 6. System Prompt 复杂度 ──
	sysScore := 0.0
	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "system" {
			content, _ := msg["content"].(string)
			if utf8.RuneCountInString(content) > 500 {
				sysScore = 0.08
				score.Reasons = append(score.Reasons, "complex_system_prompt")
			}
			break
		}
	}

	// ── 汇总 ──
	total := charScore + msgScore + codeScore + toolsScore + reasonScore + sysScore
	total = math.Min(total, 1.0)
	score.Score = total

	switch {
	case total < 0.20:
		score.Level = ComplexitySimple
	case total < 0.45:
		score.Level = ComplexityModerate
	default:
		score.Level = ComplexityComplex
	}

	return score
}

// ── 智能选择 ──

// SmartSelectResult 智能选择结果
type SmartSelectResult struct {
	OriginalModel  string
	ResolvedModel  string
	Downgraded     bool
	Complexity     ComplexityScore
	TargetTier     ModelTier
	Reason         string
}

// SmartModelSelect 智能模型选择入口
func SmartModelSelect(requestedModel string, body []byte, token *Token) SmartSelectResult {
	result := SmartSelectResult{
		OriginalModel: requestedModel,
		ResolvedModel: requestedModel,
	}

	// 评估复杂度
	complexity := EvalComplexity(body)
	result.Complexity = complexity

	// ── 模式1: auto/smart 别名 ──
	if requestedModel == "auto" || requestedModel == "smart" {
		tier := complexityToTier(complexity.Level)
		result.TargetTier = tier
		model := selectModelByTier(tier, token)
		result.ResolvedModel = model
		result.Reason = "auto_mode"
		if model != requestedModel {
			result.Downgraded = true
		}
		LogInfo("SmartSelect: auto → %s (tier=%s, score=%.2f, reasons=%v)",
			model, tier, complexity.Score, complexity.Reasons)
		return result
	}

	// ── 模式2: 自动降级（Token 开关控制）──
	if token != nil && token.SmartDowngrade {
		requestedTier := findModelTier(requestedModel)
		if requestedTier == TierPremium || requestedTier == TierEconomy {
			// 只对 premium 模型做降级，economy 不升级
			// 但如果请求的就是 economy，也不需要升级（尊重用户选择更便宜的）
			optimalTier := complexityToTier(complexity.Level)

			// 只有当最优 tier 低于请求 tier 时才降级
			if tierLessThan(optimalTier, requestedTier) {
				downgradedModel := selectModelByTier(optimalTier, token)
				if downgradedModel != "" && downgradedModel != requestedModel {
					result.ResolvedModel = downgradedModel
					result.TargetTier = optimalTier
					result.Downgraded = true
					result.Reason = "smart_downgrade"
					LogInfo("SmartSelect: %s → %s (tier=%s→%s, score=%.2f, reasons=%v)",
						requestedModel, downgradedModel, requestedTier, optimalTier,
						complexity.Score, complexity.Reasons)
					return result
				}
			}
		}
	}

	return result
}

// ── 辅助函数 ──

func complexityToTier(level ComplexityLevel) ModelTier {
	switch level {
	case ComplexitySimple:
		return TierEconomy
	case ComplexityModerate:
		return TierStandard
	default:
		return TierPremium
	}
}

func findModelTier(model string) ModelTier {
	for _, g := range modelGrades {
		if g.Model == model {
			return g.Tier
		}
	}
	// 未知模型默认 standard
	return TierStandard
}

func tierLessThan(a, b ModelTier) bool {
	order := map[ModelTier]int{TierEconomy: 0, TierStandard: 1, TierPremium: 2}
	return order[a] < order[b]
}

// selectModelByTier 从指定 tier 中选最便宜的可用模型
func selectModelByTier(tier ModelTier, token *Token) string {
	// 获取当前所有可用 Provider 的模型列表
	availableModels := getAvailableModels(token)

	// 从该 tier 中找成本最低的可用模型
	var best *ModelGrade
	for i := range modelGrades {
		g := &modelGrades[i]
		if g.Tier != tier {
			continue
		}
		if !modelInAvailable(g.Model, availableModels) {
			continue
		}
		if best == nil || g.CostIdx < best.CostIdx {
			best = g
		}
	}
	if best != nil {
		return best.Model
	}

	// 该 tier 没有可用模型，往上一级找
	if tier == TierPremium {
		return selectModelByTier(TierStandard, token)
	}
	if tier == TierStandard {
		return selectModelByTier(TierEconomy, token)
	}

	// 兜底：返回第一个可用模型
	if len(availableModels) > 0 {
		return availableModels[0]
	}
	return "qwen3-coder-flash" // 终极兜底
}

func getAvailableModels(token *Token) []string {
	providers := router.GetProviders()
	modelSet := make(map[string]bool)
	for _, p := range providers {
		if !p.Enabled || !p.ProxyEnabled {
			continue
		}
		if !p.IsAvailable("") { // 不限 model，只看 Provider 状态
			continue
		}
		if token != nil && !token.CanUseProvider(p.ID) {
			continue
		}
		for _, m := range parseModelsList(p.Models) {
			if token == nil || token.CanUseModel(m) {
				modelSet[m] = true
			}
		}
	}
	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	return models
}

func extractAllContent(messages []interface{}) string {
	var sb strings.Builder
	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		content, _ := msg["content"].(string)
		sb.WriteString(content)
		sb.WriteString(" ")
	}
	return sb.String()
}

func extractLastUserContent(messages []interface{}) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "user" {
			content, _ := msg["content"].(string)
			return content
		}
	}
	return ""
}

func modelInAvailable(model string, available []string) bool {
	for _, m := range available {
		if m == model {
			return true
		}
	}
	return false
}

// replaceModelInBody 替换请求 body 中的 model 字段值
func replaceModelInBody(body []byte, oldModel, newModel string) []byte {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	req["model"] = newModel
	newBody, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return newBody
}
