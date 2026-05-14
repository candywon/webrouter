# 03 - 智能模型选择 (smart_model.go)

## 两种模式

1. **model=auto/smart** → 完全由网关根据复杂度选模型
2. **Token启用SmartDowngrade** → 对指定强模型的简单请求自动降级到便宜模型

## 模型分级

### ModelTier
```
TierEconomy  = "economy"   // 便宜快速
TierStandard = "standard"  // 中等性价比
TierPremium  = "premium"   // 最强推理
```

### modelGrades 分级表
```
economy (CostIdx 1.0~1.5):
  qwen3-coder-flash  1.0
  qwen-turbo         1.0
  gpt-4o-mini        1.5

standard (CostIdx 2.0~5.0):
  deepseek-chat         2.0
  qwen-plus-2025-07-28  3.0
  qwen-plus             3.0
  gpt-4o                5.0

premium (CostIdx 8.0~15.0):
  qwen3.6-plus   8.0
  o1-mini        8.0
  qwen-max      10.0
  claude-sonnet-4 12.0
  o1            15.0
```

## 6维复杂度评分 (EvalComplexity)

```
输入: body (请求JSON)
输出: ComplexityScore
```

### 评分维度

| # | 维度 | 权重 | 逻辑 |
|---|------|------|------|
| 1 | 输入长度 | 0~0.30 | <200字=0.05, <800=0.12, <2000=0.20, >=2000=0.30 |
| 2 | 多轮对话 | 0~0.20 | <=2轮=0, <=5=0.08, <=10=0.15, >10=0.20 |
| 3 | 代码检测 | 0~0.15 | 含```/def /function /class /import /return → 0.15 |
| 4 | Tools/FC | 0~0.20 | 有tools=0.20, 有functions=0.15 |
| 5 | 推理关键词 | 0~0.12 | 分析/推理/证明/explain/analyze/为什么/步骤 等 |
| 6 | System Prompt | 0~0.08 | system内容>500字 → 0.08 |

### 总分 → 复杂度级别
```
total < 0.20  → ComplexitySimple   (0)
total < 0.45  → ComplexityModerate (1)
total >= 0.45 → ComplexityComplex  (2)
```

## ComplexityScore 结构
```
type ComplexityScore struct {
    Level      ComplexityLevel  // 0/1/2
    Score      float64          // 0~1 连续分数
    Reasons    []string         // 触发的原因标签
    InputChars int
    MsgCount   int
    HasCode    bool
    HasTools   bool
}
```

## SmartModelSelect 主函数

```
输入: requestedModel, body, token
输出: SmartSelectResult
```

### 逻辑

1. **auto/smart别名**
   - complexity = EvalComplexity(body)
   - tier = complexityToTier(level): Simple→Economy, Moderate→Standard, Complex→Premium
   - resolvedModel = selectModelByTier(tier, token)

2. **SmartDowngrade降级**
   - 条件: token.SmartDowngrade == true && 指定了具体强模型(非auto)
   - complexity = EvalComplexity(body)
   - 如果 Simple 且 请求模型是Premium/Standard → 降级到Economy
   - Downgraded = true

3. **其他** → 原样返回

## SmartSelectResult
```
type SmartSelectResult struct {
    OriginalModel  string
    ResolvedModel  string
    Downgraded     bool
    Complexity     ComplexityScore
    TargetTier     ModelTier
    Reason         string
}
```

## 辅助函数

### selectModelByTier(tier, token) → model名
- getAvailableModels(token) → Token可用的模型列表
- 在可用列表中找对应tier的第一个模型
- 找不到 → 降一级再找
- 都找不到 → 返回原请求模型

### replaceModelInBody(body, oldModel, newModel) → []byte
- JSON解析 → 替换model字段 → JSON序列化回
- 序列化失败 → 返回原body
