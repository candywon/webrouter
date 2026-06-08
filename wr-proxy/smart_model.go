// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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
	TierEconomy  ModelTier = "economy"  // 便宜快速（闲聊、翻译、短问答）
	TierStandard ModelTier = "standard" // 日常通用（写作、总结、格式转换）
	TierEnhanced ModelTier = "enhanced" // 增强（代码生成、多步推理、文档撰写）
	TierPremium  ModelTier = "premium"  // 高端（复杂架构、数学证明、长文分析）
	TierFlagship ModelTier = "flagship" // 旗舰（科研、竞赛、多模态深度推理）
)

// ModelGrade 模型分级定义
type ModelGrade struct {
	Model       string
	Tier        ModelTier
	CostIdx     float64 // 相对成本指数，economy=1.0
	Vendor      string
	Description string
	SortOrder   int
}

// modelGrades 模型分级表（运行时从 DB 动态加载，可热刷新）
// 按 sort_order 从低到高排列
var modelGrades []ModelGrade

// RefreshModelGrades 从 DB 重新加载模型分级（热刷新，由 /admin/reload_model_grades 触发）
// 同时自动为未分级的可用模型推断 tier 并写入 DB
func RefreshModelGrades() error {
	// 1. 自动补全：扫描所有可用模型，未分级的按规则推断 tier
	autoGraded, err := autoGradeMissingModels()
	if err != nil {
		LogWarn("RefreshModelGrades: auto-grade failed: %v", err)
		// 不阻断，继续加载已有数据
	} else if autoGraded > 0 {
		LogInfo("RefreshModelGrades: auto-graded %d new models", autoGraded)
	}

	// 2. 加载全部分级数据
	grades, err := LoadModelGrades()
	if err != nil {
		return err
	}
	if len(grades) == 0 {
		LogWarn("RefreshModelGrades: DB returned no grades, keeping existing")
		return nil
	}
	modelGrades = grades
	LogInfo("RefreshModelGrades: %d model grades loaded from DB", len(grades))
	return nil
}

// ── 复杂度评估 ──

// ComplexityLevel 复杂度级别
type ComplexityLevel int

const (
	ComplexitySimple   ComplexityLevel = 0 // 简单
	ComplexityModerate ComplexityLevel = 1 // 中等
	ComplexityHigh     ComplexityLevel = 2 // 较高
	ComplexityComplex  ComplexityLevel = 3 // 复杂
	ComplexityExtreme  ComplexityLevel = 4 // 极端复杂
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

// ── 六维度复杂度配置 ──

// ComplexityConfig 六维度复杂度评估配置（从 DB 加载）
type ComplexityConfig struct {
	SimpleMax   float64
	ModerateMax float64
	HighMax     float64
	ExtremeMax  float64

	InputLengthEnabled bool
	InputLengthLevels  []struct {
		MaxChars int
		Score    float64
	}

	MultiTurnEnabled bool
	MultiTurnLevels  []struct {
		MaxMsgs int
		Score   float64
	}

	CodeDetectionEnabled bool
	CodeDetectionScore   float64
	CodeKeywords         []string

	ToolsDetectionEnabled bool
	ToolsScore            float64
	FunctionsScore        float64

	ReasoningEnabled  bool
	ReasoningScore    float64
	ReasoningKeywords []string

	SystemPromptEnabled   bool
	SystemPromptThreshold int
	SystemPromptScore     float64
}

// complexityConfig 全局复杂度配置（运行时加载）
var complexityConfig ComplexityConfig

// LoadComplexityConfig 从 DB 加载六维度复杂度配置
func LoadComplexityConfig() {
	v := LoadSetting("smart_complexity_config", nil)
	if v == nil {
		LogWarn("LoadComplexityConfig: no config in DB, using defaults")
		setDefaultComplexityConfig()
		return
	}

	raw, err := json.Marshal(v)
	if err != nil {
		LogError("LoadComplexityConfig: marshal error: %v, using defaults", err)
		setDefaultComplexityConfig()
		return
	}

	var cfg struct {
		TierThresholds struct {
			SimpleMax   float64 `json:"simple_max"`
			ModerateMax float64 `json:"moderate_max"`
			HighMax     float64 `json:"high_max"`
			ExtremeMax  float64 `json:"extreme_max"`
		} `json:"tier_thresholds"`
		InputLength struct {
			Enabled bool `json:"enabled"`
			Levels  []struct {
				MaxChars float64 `json:"max_chars"`
				Score    float64 `json:"score"`
			} `json:"levels"`
		} `json:"input_length"`
		MultiTurn struct {
			Enabled bool `json:"enabled"`
			Levels  []struct {
				MaxMsgs float64 `json:"max_msgs"`
				Score   float64 `json:"score"`
			} `json:"levels"`
		} `json:"multi_turn"`
		CodeDetection struct {
			Enabled  bool     `json:"enabled"`
			Score    float64  `json:"score"`
			Keywords []string `json:"keywords"`
		} `json:"code_detection"`
		ToolsDetection struct {
			Enabled    bool    `json:"enabled"`
			ToolsScore float64 `json:"tools_score"`
			FuncsScore float64 `json:"functions_score"`
		} `json:"tools_detection"`
		Reasoning struct {
			Enabled  bool     `json:"enabled"`
			Score    float64  `json:"score"`
			Keywords []string `json:"keywords"`
		} `json:"reasoning_keywords"`
		SystemPrompt struct {
			Enabled  bool    `json:"enabled"`
			MaxChars float64 `json:"threshold_chars"`
			Score    float64 `json:"score"`
		} `json:"system_prompt"`
	}

	if err := json.Unmarshal(raw, &cfg); err != nil {
		LogError("LoadComplexityConfig: unmarshal error: %v, using defaults", err)
		setDefaultComplexityConfig()
		return
	}

	complexityConfig = ComplexityConfig{
		SimpleMax:   cfg.TierThresholds.SimpleMax,
		ModerateMax: cfg.TierThresholds.ModerateMax,
		HighMax:     cfg.TierThresholds.HighMax,
		ExtremeMax:  cfg.TierThresholds.ExtremeMax,
	}

	if cfg.InputLength.Enabled && len(cfg.InputLength.Levels) > 0 {
		complexityConfig.InputLengthEnabled = true
		for _, l := range cfg.InputLength.Levels {
			complexityConfig.InputLengthLevels = append(complexityConfig.InputLengthLevels, struct {
				MaxChars int
				Score    float64
			}{MaxChars: int(l.MaxChars), Score: l.Score})
		}
	}

	if cfg.MultiTurn.Enabled && len(cfg.MultiTurn.Levels) > 0 {
		complexityConfig.MultiTurnEnabled = true
		for _, l := range cfg.MultiTurn.Levels {
			complexityConfig.MultiTurnLevels = append(complexityConfig.MultiTurnLevels, struct {
				MaxMsgs int
				Score   float64
			}{MaxMsgs: int(l.MaxMsgs), Score: l.Score})
		}
	}

	if cfg.CodeDetection.Enabled {
		complexityConfig.CodeDetectionEnabled = true
		complexityConfig.CodeDetectionScore = cfg.CodeDetection.Score
		complexityConfig.CodeKeywords = cfg.CodeDetection.Keywords
	}

	if cfg.ToolsDetection.Enabled {
		complexityConfig.ToolsDetectionEnabled = true
		complexityConfig.ToolsScore = cfg.ToolsDetection.ToolsScore
		complexityConfig.FunctionsScore = cfg.ToolsDetection.FuncsScore
	}

	if cfg.Reasoning.Enabled {
		complexityConfig.ReasoningEnabled = true
		complexityConfig.ReasoningScore = cfg.Reasoning.Score
		complexityConfig.ReasoningKeywords = cfg.Reasoning.Keywords
	}

	if cfg.SystemPrompt.Enabled {
		complexityConfig.SystemPromptEnabled = true
		complexityConfig.SystemPromptThreshold = int(cfg.SystemPrompt.MaxChars)
		complexityConfig.SystemPromptScore = cfg.SystemPrompt.Score
	}

	LogInfo("LoadComplexityConfig: loaded %d input_length levels, %d multi_turn levels, code=%v, tools=%v, reasoning=%v, system_prompt=%v",
		len(complexityConfig.InputLengthLevels), len(complexityConfig.MultiTurnLevels),
		complexityConfig.CodeDetectionEnabled, complexityConfig.ToolsDetectionEnabled,
		complexityConfig.ReasoningEnabled, complexityConfig.SystemPromptEnabled)
}

// ReloadComplexityConfig 热刷新复杂度配置（由 reload 调用）
func ReloadComplexityConfig() {
	LoadComplexityConfig()
}

// setDefaultComplexityConfig 设置内置默认值（DB 无配置时的兜底）
func setDefaultComplexityConfig() {
	complexityConfig = ComplexityConfig{
		SimpleMax:          0.15,
		ModerateMax:        0.30,
		HighMax:            0.50,
		ExtremeMax:         0.70,
		InputLengthEnabled: true,
		InputLengthLevels: []struct {
			MaxChars int
			Score    float64
		}{
			{200, 0.03}, {800, 0.08}, {2000, 0.14}, {5000, 0.22}, {0, 0.30},
		},
		MultiTurnEnabled: true,
		MultiTurnLevels: []struct {
			MaxMsgs int
			Score   float64
		}{
			{2, 0.0}, {5, 0.05}, {10, 0.10}, {20, 0.16}, {0, 0.22},
		},
		CodeDetectionEnabled:  true,
		CodeDetectionScore:    0.14,
		CodeKeywords:          []string{"```", "def ", "function ", "class ", "import ", "return "},
		ToolsDetectionEnabled: true,
		ToolsScore:            0.16,
		FunctionsScore:        0.12,
		ReasoningEnabled:      true,
		ReasoningScore:        0.14,
		ReasoningKeywords: []string{
			"分析", "推理", "证明", "计算", "推导",
			"explain", "analyze", "reason", "prove", "calculate",
			"derive", "compare", "evaluate", "critique",
			"为什么", "原因", "原理", "逻辑",
			"步骤", "方案", "策略", "设计",
		},
		SystemPromptEnabled:   true,
		SystemPromptThreshold: 500,
		SystemPromptScore:     0.06,
	}
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
	cfg := &complexityConfig

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

	// 字数评分
	lengthScore := 0.0
	if cfg.InputLengthEnabled {
		for _, level := range cfg.InputLengthLevels {
			if level.MaxChars == 0 || totalChars < level.MaxChars {
				lengthScore = level.Score
				break
			}
		}
		if totalChars > 200 {
			score.Reasons = append(score.Reasons, "long_input")
		}
	} else {
		lengthScore = 0.0
	}

	// ── 2. 多轮对话 ──
	msgScore := 0.0
	if cfg.MultiTurnEnabled {
		for _, level := range cfg.MultiTurnLevels {
			if level.MaxMsgs == 0 || score.MsgCount <= level.MaxMsgs {
				msgScore = level.Score
				break
			}
		}
		if score.MsgCount > 3 {
			score.Reasons = append(score.Reasons, "multi_turn")
		}
	} else {
		msgScore = 0.0
	}

	// ── 3. 代码检测 ──
	codeScore := 0.0
	fullContent := extractAllContent(messages)
	if cfg.CodeDetectionEnabled {
		for _, kw := range cfg.CodeKeywords {
			if strings.Contains(fullContent, kw) {
				codeScore = cfg.CodeDetectionScore
				score.HasCode = true
				score.Reasons = append(score.Reasons, "has_code")
				break
			}
		}
	}

	// ── 4. Tools / Function Calling ──
	toolsScore := 0.0
	if cfg.ToolsDetectionEnabled {
		if _, hasTools := req["tools"]; hasTools {
			toolsScore = cfg.ToolsScore
			score.HasTools = true
			score.Reasons = append(score.Reasons, "has_tools")
		}
		if _, hasFunc := req["functions"]; hasFunc {
			toolsScore = math.Max(toolsScore, cfg.FunctionsScore)
			score.HasTools = true
			score.Reasons = append(score.Reasons, "has_functions")
		}
	}

	// ── 5. 推理/分析关键词 ──
	reasonScore := 0.0
	if cfg.ReasoningEnabled {
		lastMsg := extractLastUserContent(messages)
		lowerLast := strings.ToLower(lastMsg)
		for _, kw := range cfg.ReasoningKeywords {
			if strings.Contains(lowerLast, kw) {
				reasonScore = cfg.ReasoningScore
				score.Reasons = append(score.Reasons, "reasoning_keyword")
				break
			}
		}
	}

	// ── 6. System Prompt 复杂度 ──
	sysScore := 0.0
	if cfg.SystemPromptEnabled {
		for _, m := range messages {
			msg, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			if role == "system" {
				content, _ := msg["content"].(string)
				if utf8.RuneCountInString(content) > cfg.SystemPromptThreshold {
					sysScore = cfg.SystemPromptScore
					score.Reasons = append(score.Reasons, "complex_system_prompt")
				}
				break
			}
		}
	}

	// ── 汇总 ──
	total := lengthScore + msgScore + codeScore + toolsScore + reasonScore + sysScore
	total = math.Min(total, 1.0)
	score.Score = total

	switch {
	case total < cfg.SimpleMax:
		score.Level = ComplexitySimple
	case total < cfg.ModerateMax:
		score.Level = ComplexityModerate
	case total < cfg.HighMax:
		score.Level = ComplexityHigh
	case total < cfg.ExtremeMax:
		score.Level = ComplexityComplex
	default:
		score.Level = ComplexityExtreme
	}

	return score
}

// ── 智能选择 ──

// SmartSelectResult 智能选择结果
type SmartSelectResult struct {
	OriginalModel     string
	ResolvedModel     string
	Downgraded        bool
	Complexity        ComplexityScore
	TargetTier        ModelTier
	Reason            string
	PreferredProvider *Provider // cost_optimal 策略预选的 Provider
}

// SmartModelSelect 智能模型选择入口
func SmartModelSelect(requestedModel string, body []byte, token *Token, sessionID string) SmartSelectResult {
	result := SmartSelectResult{
		OriginalModel: requestedModel,
		ResolvedModel: requestedModel,
	}

	// 评估复杂度
	complexity := EvalComplexity(body)

	// Session 上下文感知：已累积大量 context 时提升复杂度
	if token != nil {
		if ctxTokens := estimateSessionContextTokens(sessionID, token.ID); ctxTokens > 0 {
			if ctxTokens > 16000 {
				complexity.Score += 0.15
				complexity.Reasons = append(complexity.Reasons, "large_session_context")
			} else if ctxTokens > 8000 {
				complexity.Score += 0.08
				complexity.Reasons = append(complexity.Reasons, "medium_session_context")
			}
			// 重新判定 level
			cfg := &complexityConfig
			switch {
			case complexity.Score < cfg.SimpleMax:
				complexity.Level = ComplexitySimple
			case complexity.Score < cfg.ModerateMax:
				complexity.Level = ComplexityModerate
			case complexity.Score < cfg.HighMax:
				complexity.Level = ComplexityHigh
			case complexity.Score < cfg.ExtremeMax:
				complexity.Level = ComplexityComplex
			default:
				complexity.Level = ComplexityExtreme
			}
		}
	}
	result.Complexity = complexity

	// ── 模式1: auto/smart 别名 ──
	if requestedModel == "auto" || requestedModel == "smart" {
		tier := complexityToTier(complexity.Level)

		// 配额感知：配额紧张时自动降级 tier
		tier = adjustTierForQuota(tier, token)
		result.TargetTier = tier

		// cost_optimal 策略：同时选模型 + Provider
		if router.Strategy() == "cost_optimal" {
			model, prov := selectCostOptimal(tier, token, body)
			if model != "" {
				result.ResolvedModel = model
				result.Reason = "cost_optimal"
				if prov != nil {
					result.PreferredProvider = prov
				}
				if model != requestedModel {
					result.Downgraded = true
				}
				return result
			}
		}

		model := selectModelByTier(tier, token)
		result.ResolvedModel = model
		result.Reason = "auto_mode"
		if model != requestedModel {
			result.Downgraded = true
		}
		return result
	}

	// ── 模式2: 自动降级（Token 开关控制）──
	if token != nil && token.SmartDowngrade {
		requestedTier := findModelTier(requestedModel)
		if requestedTier != TierEconomy {
			// 只对 enhanced/premium/flagship 做降级，economy 不升级
			// 尊重用户主动选择更便宜模型的决定
			optimalTier := complexityToTier(complexity.Level)

			// 只有当最优 tier 低于请求 tier 时才降级
			if tierLessThan(optimalTier, requestedTier) {
				downgradedModel := selectModelByTier(optimalTier, token)
				if downgradedModel != "" && downgradedModel != requestedModel {
					result.ResolvedModel = downgradedModel
					result.TargetTier = optimalTier
					result.Downgraded = true
					result.Reason = "smart_downgrade"
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
	case ComplexityHigh:
		return TierEnhanced
	case ComplexityComplex:
		return TierPremium
	default:
		return TierFlagship
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
	order := map[ModelTier]int{TierEconomy: 0, TierStandard: 1, TierEnhanced: 2, TierPremium: 3, TierFlagship: 4}
	return order[a] < order[b]
}

// selectModelByTier 从指定 tier 中选最便宜的可用模型
func selectModelByTier(tier ModelTier, token *Token) string {
	// 获取当前所有可用 Provider 的模型列表
	availableModels := getAvailableModels(token)

	// 从该 tier 中找成本最低的可用模型
	// 从该 tier 中找成本最低的可用模型
	// 优先用 pricing 表实时单价，无定价数据时 fallback 到静态 CostIdx
	var best *ModelGrade
	var bestCost float64 = -1
	for i := range modelGrades {
		g := &modelGrades[i]
		if g.Tier != tier {
			continue
		}
		if !modelInAvailable(g.Model, availableModels) {
			continue
		}
		cost := modelRealCost(g)
		if best == nil || cost < bestCost {
			best = g
			bestCost = cost
		}
	}
	if best != nil {
		return best.Model
	}

	// 该 tier 没有可用模型，往下一级找（降级优先保可用性）
	if tier == TierFlagship {
		return selectModelByTier(TierPremium, token)
	}
	if tier == TierPremium {
		return selectModelByTier(TierEnhanced, token)
	}
	if tier == TierEnhanced {
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

// modelRealCost 计算模型的真实成本：优先用 pricing 表实时单价，否则用静态 CostIdx
func modelRealCost(g *ModelGrade) float64 {
	if p, ok := GetModelPricing(g.Model); ok {
		// 用 (input_price + output_price) / 2 作为综合成本指标
		// output 通常比 input 贵，这里加权：input 40% + output 60%
		return p.Input*0.4 + p.Output*0.6
	}
	// fallback 到静态 CostIdx
	return g.CostIdx
}

// selectCostOptimal 选择成本最优的模型+Provider 组合
// 综合考虑：模型 tier × 实时单价 × Provider CostMultiplier × 预估 token 数
func selectCostOptimal(tier ModelTier, token *Token, body []byte) (string, *Provider) {
	availableModels := getAvailableModels(token)
	providers := router.GetProviders()

	// 预估 token 数（基于字数粗估）
	estInputTokens := estimateInputTokens(body)

	var bestModel string
	var bestProvider *Provider
	var bestEstCost float64 = -1

	// 遍历 tier 内所有可用模型
	for i := range modelGrades {
		g := &modelGrades[i]
		if g.Tier != tier {
			continue
		}
		if !modelInAvailable(g.Model, availableModels) {
			continue
		}

		// 查模型单价
		var inputPrice, outputPrice float64
		if p, ok := GetModelPricing(g.Model); ok {
			inputPrice = p.Input
			outputPrice = p.Output
		} else {
			// 无定价：按 CostIdx 估算（CostIdx=1 → ~0.01 分/千token）
			inputPrice = g.CostIdx * 0.01
			outputPrice = g.CostIdx * 0.03
		}

		// 查该模型可用的最便宜 Provider
		for _, prov := range providers {
			if !prov.Enabled || !prov.ProxyEnabled {
				continue
			}
			if !prov.IsAvailable(g.Model) {
				continue
			}
			if token != nil && !token.CanUseProvider(prov.ID) {
				continue
			}
			if !modelInProviderModels(g.Model, prov.Models) {
				continue
			}

			// 预估成本 = input_tokens/1000 * input_price * multiplier + output_tokens/1000 * output_price * multiplier
			multiplier := prov.CostMultiplier
			if multiplier <= 0 {
				multiplier = 1.0
			}
			estOutputTokens := estInputTokens / 2 // 粗估 output 为 input 的一半
			estCost := float64(estInputTokens)/1000.0*inputPrice*multiplier +
				float64(estOutputTokens)/1000.0*outputPrice*multiplier

			if bestEstCost < 0 || estCost < bestEstCost {
				bestModel = g.Model
				bestProvider = prov
				bestEstCost = estCost
			}
		}
	}

	return bestModel, bestProvider
}

// estimateInputTokens 基于请求体估算 input token 数
func estimateInputTokens(body []byte) int {
	// 粗估：英文字符/4, 中文字符/2
	charCount := utf8.RuneCount(body)
	// 假设混合内容，取中间值 ~3 字符/token
	return charCount / 3
}

// modelInProviderModels 检查模型是否在 Provider 的模型列表中
func modelInProviderModels(model, modelsJSON string) bool {
	for _, m := range parseModelsList(modelsJSON) {
		if m == model {
			return true
		}
	}
	return false
}

// adjustTierForQuota 根据配额使用率动态调整 tier（配额紧张时自动降级）
func adjustTierForQuota(tier ModelTier, token *Token) ModelTier {
	if token == nil || token.QuotaTotal <= 0 {
		return tier
	}
	ratio := float64(token.QuotaUsed) / float64(token.QuotaTotal)
	switch {
	case ratio > 0.95:
		// 配额 >95% 用完 → 最多给 economy
		return TierEconomy
	case ratio > 0.85:
		// 配额 >85% → 最多给 standard
		if tierOrder(tier) > tierOrder(TierStandard) {
			return TierStandard
		}
	case ratio > 0.70:
		// 配额 >70% → 最多给 enhanced
		if tierOrder(tier) > tierOrder(TierEnhanced) {
			return TierEnhanced
		}
	}
	return tier
}

// tierOrder 返回 tier 的数值排序
func tierOrder(tier ModelTier) int {
	switch tier {
	case TierEconomy:
		return 0
	case TierStandard:
		return 1
	case TierEnhanced:
		return 2
	case TierPremium:
		return 3
	case TierFlagship:
		return 4
	default:
		return 1
	}
}

// estimateSessionContextTokens 估算会话已累积的 context token 数
// 使用 wr_session_messages 表统计该 session 的历史消息字符数，粗估为 token 数
func estimateSessionContextTokens(sessionID string, tokenID int) int {
	if sessionID == "" || tokenID <= 0 {
		return 0
	}
	var totalChars int
	err := db.QueryRow(`
		SELECT COALESCE(SUM(LENGTH(content)), 0)
		FROM wr_session_messages
		WHERE session_id = ? AND token_id = ?`,
		sessionID, tokenID).Scan(&totalChars)
	if err != nil {
		return 0
	}
	return totalChars / 3 // 粗估：~3 字符/token
}

// ── 自动分级推断 ──

// autoGradeMissingModels 扫描所有可用模型，对未分级的模型按名称规则推断 tier 并写入 DB
func autoGradeMissingModels() (int, error) {
	// 1. 收集所有可用模型
	providers := router.GetProviders()
	modelSet := make(map[string]bool)
	for _, p := range providers {
		if !p.Enabled || !p.ProxyEnabled {
			continue
		}
		for _, m := range parseModelsList(p.Models) {
			modelSet[m] = true
		}
	}

	// 2. 已有分级的模型
	graded := make(map[string]bool)
	for _, g := range modelGrades {
		graded[g.Model] = true
	}

	// 3. 对未分级模型推断 tier
	count := 0
	for model := range modelSet {
		if graded[model] {
			continue
		}
		tier, costIdx, vendor := inferTier(model)
		if err := insertModelGrade(model, tier, costIdx, vendor); err != nil {
			LogWarn("autoGrade: failed to insert %s: %v", model, err)
			continue
		}
		LogInfo("autoGrade: %s → tier=%s, cost_idx=%.1f, vendor=%s", model, tier, costIdx, vendor)
		count++
	}
	return count, nil
}

// inferTier 根据模型名称推断 tier、成本指数、厂商
func inferTier(model string) (ModelTier, float64, string) {
	m := strings.ToLower(model)

	// 厂商推断
	vendor := "other"
	switch {
	case strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3") || strings.Contains(m, "dall"):
		vendor = "openai"
	case strings.Contains(m, "claude"):
		vendor = "anthropic"
	case strings.Contains(m, "gemini"):
		vendor = "google"
	case strings.Contains(m, "qwen") || strings.Contains(m, "通义"):
		vendor = "qwen"
	case strings.Contains(m, "deepseek"):
		vendor = "deepseek"
	case strings.Contains(m, "doubao"):
		vendor = "bytedance"
	case strings.Contains(m, "yi-"):
		vendor = "01ai"
	case strings.Contains(m, "glm") || strings.Contains(m, "chatglm"):
		vendor = "zhipu"
	}

	// Tier 推断 — 按优先级从高到低匹配
	// flagship: 顶级推理/多模态
	switch {
	case strings.Contains(m, "o3") && !strings.Contains(m, "mini"):
		return TierFlagship, 30.0, vendor
	case strings.Contains(m, "opus") && (strings.Contains(m, "extended") || strings.Contains(m, "ultra")):
		return TierFlagship, 25.0, vendor
	}

	// premium: 高端推理
	switch {
	case strings.Contains(m, "opus"):
		return TierPremium, 18.0, vendor
	case strings.Contains(m, "o1") && !strings.Contains(m, "mini"):
		return TierPremium, 15.0, vendor
	case strings.Contains(m, "o3-mini") || strings.Contains(m, "o1-mini"):
		return TierPremium, 8.0, vendor
	case strings.Contains(m, "max") && !strings.Contains(m, "mini"):
		return TierPremium, 12.0, vendor
	case strings.Contains(m, "gpt-4") && !strings.Contains(m, "mini") && !strings.Contains(m, "turbo") && !strings.Contains(m, "4o"):
		return TierPremium, 10.0, vendor
	case strings.Contains(m, "ultra"):
		return TierPremium, 14.0, vendor
	}

	// enhanced: 增强能力
	switch {
	case strings.Contains(m, "sonnet"):
		return TierEnhanced, 8.0, vendor
	case strings.Contains(m, "reasoner") || strings.Contains(m, "reasoning"):
		return TierEnhanced, 4.0, vendor
	case strings.Contains(m, "thinking") || strings.Contains(m, "think"):
		return TierEnhanced, 5.0, vendor
	case strings.Contains(m, "pro") && !strings.Contains(m, "mini"):
		return TierEnhanced, 6.0, vendor
	case strings.Contains(m, "gemini-2.5") && !strings.Contains(m, "flash"):
		return TierEnhanced, 7.0, vendor
	case strings.Contains(m, "deepseek-r1") || strings.Contains(m, "deepseek-reasoner"):
		return TierEnhanced, 4.0, vendor
	}

	// economy: 便宜快速
	switch {
	case strings.Contains(m, "mini"):
		return TierEconomy, 1.5, vendor
	case strings.Contains(m, "flash") && !strings.Contains(m, "thinking"):
		return TierEconomy, 1.0, vendor
	case strings.Contains(m, "turbo") && !strings.Contains(m, "gpt-4"):
		return TierEconomy, 1.0, vendor
	case strings.Contains(m, "lite") || strings.Contains(m, "light"):
		return TierEconomy, 1.0, vendor
	case strings.Contains(m, "micro"):
		return TierEconomy, 0.5, vendor
	}

	// standard: 默认（plus, chat, 无特殊后缀）
	return TierStandard, 3.0, vendor
}

// insertModelGrade 向 DB 插入一条模型分级（若已存在则跳过）
func insertModelGrade(model string, tier ModelTier, costIdx float64, vendor string) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO wr_model_grades (model, tier, cost_index, vendor, description, enabled, sort_order)
		VALUES (?, ?, ?, ?, 'auto-graded', 1, 0)
	`, model, string(tier), costIdx, vendor)
	return err
}
