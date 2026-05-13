package main

// 成本估算：模型定价表 + 费用计算

// ModelPricing 模型定价（单位：分/千token）
type ModelPricing struct {
	Input  float64 // 输入价格
	Output float64 // 输出价格
}

// 主流模型定价表（2024年参考价，可配置覆盖）
// 价格单位：分/千token (1元=100分)
var pricingTable = map[string]ModelPricing{
	// OpenAI
	"gpt-4o":            {Input: 0.18, Output: 0.54},
	"gpt-4o-mini":       {Input: 0.012, Output: 0.048},
	"gpt-4-turbo":       {Input: 0.60, Output: 1.80},
	"gpt-4":             {Input: 2.10, Output: 6.30},
	"gpt-3.5-turbo":     {Input: 0.003, Output: 0.006},
	"o1-preview":        {Input: 1.05, Output: 4.20},
	"o1-mini":           {Input: 0.21, Output: 0.84},

	// Anthropic
	"claude-3.5-sonnet":  {Input: 0.21, Output: 1.05},
	"claude-3.5-haiku":   {Input: 0.007, Output: 0.035},
	"claude-3-opus":      {Input: 1.05, Output: 5.25},
	"claude-3-sonnet":    {Input: 0.21, Output: 1.05},
	"claude-3-haiku":     {Input: 0.018, Output: 0.09},

	// Google
	"gemini-1.5-pro":     {Input: 0.16, Output: 0.48},
	"gemini-1.5-flash":   {Input: 0.005, Output: 0.015},
	"gemini-2.0-flash":   {Input: 0.005, Output: 0.015},

	// DeepSeek
	"deepseek-chat":      {Input: 0.009, Output: 0.027},
	"deepseek-reasoner":  {Input: 0.42, Output: 1.26},

	// 通义千问
	"qwen-turbo":         {Input: 0.015, Output: 0.045},
	"qwen-plus":          {Input: 0.03, Output: 0.09},
	"qwen-max":           {Input: 0.15, Output: 0.45},

	// 智谱
	"glm-4":              {Input: 0.09, Output: 0.09},
	"glm-4-flash":        {Input: 0.009, Output: 0.009},

	// 月之暗面
	"moonshot-v1-8k":     {Input: 0.09, Output: 0.09},
	"moonshot-v1-32k":    {Input: 0.18, Output: 0.18},
}

// CalculateCost 计算请求成本（分）
func CalculateCost(model string, inputTokens, outputTokens int64, multiplier float64) int64 {
	pricing, ok := pricingTable[model]
	if !ok {
		// 未知模型用默认价格（gpt-4o-mini 级别）
		pricing = ModelPricing{Input: 0.015, Output: 0.06}
	}

	inputCost := float64(inputTokens) / 1000.0 * pricing.Input
	outputCost := float64(outputTokens) / 1000.0 * pricing.Output
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
	p, ok := pricingTable[model]
	return p, ok
}

// GetAllPricing 获取全部定价表
func GetAllPricing() map[string]ModelPricing {
	return pricingTable
}
