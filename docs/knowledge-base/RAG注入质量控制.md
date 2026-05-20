# 企业知识库 — RAG注入质量控制

> 版本：v1.0 | 日期：2026-05-19

---

## 1. 问题本质

RAG注入的核心风险不是"找不到知识"，而是**注入了错误的知识**。

```
场景1（领域错配）：
  用户："合同违约金怎么算？"
  RAG注入了：Q3毛利率18.7%的数据    ← 完全无关，浪费token
  AI回答："根据公司数据，Q3毛利率..."  ← 答非所问

场景2（过时知识）：
  用户："合同审批流程是什么？"
  RAG注入了：去年旧流程（已变更）     ← 严重误导
  AI回答："审批流程是3步..."          ← 实际已改为5步

场景3（跨域冲突）：
  用户："这个条款合规吗？"
  RAG注入了：销售域的"客户要求灵活条款"+ 法务域的"必须严格合规"
  AI回答："可以灵活处理..."           ← 法务上不可灵活

场景4（幻觉放大）：
  用户："违约金比例是多少？"
  RAG注入了：uncertain的知识（未验证）
  AI回答："违约金是20%"              ← 把未验证当确定
```

**法务领域最敏感**：一条过时的法规信息可能导致实际法律风险。

---

## 2. 三层防护体系

```
RAG检索结果
    │
    ▼ 第1层：相关度门控 — "该不该注入？"
    │  检索结果和用户问题真的相关吗？
    │  相关度 < 阈值 → 不注入
    │
    ▼ 第2层：领域感知 — "注入什么级别的内容？"
    │  不同领域的风险等级不同
    │  法务/财务 → 只注入verified知识 + 强免责声明
    │  行政/销售 → 可注入auto验证知识 + 温和提示
    │
    ▼ 第3层：格式约束 — "怎么注入才不会被AI当成确定答案？"
       明确区分"参考信息"和"确定答案"
       标注来源、验证状态、时效性
```

---

## 3. 第1层：相关度门控

### 问题：向量相似度不可靠

```
用户："合同违约金条款怎么写？"

向量检索Top3：
  1. 相似度0.82 → "合同违约金为总金额20%"     ← 真相关 ✅
  2. 相似度0.74 → "合同模板更新通知"            ← 有点相关 ⚠️
  3. 相似度0.71 → "员工合同到期提醒流程"        ← 只是都含"合同" ❌
```

纯向量相似度会把"合同违约金"和"员工合同到期"混淆，因为embedding空间中"合同"这个词权重很高。

### 解决：向量检索 + 关键词交叉验证

```go
func retrieveRAGContext(prompt string, department string, topK int, minRelevance float64) []RAGItem {
    // Step 1: 向量检索候选集（多取一些）
    candidates := vectorSearch(prompt, topK*3)  // 取15条候选
    
    // Step 2: 关键词交叉验证
    promptKeywords := extractKeywords(prompt)   // 提取用户prompt的关键词
    
    scored := make([]RAGItem, 0, len(candidates))
    for _, item := range candidates {
        // 向量相似度（0-1）
        vectorScore := item.Relevance
        
        // 关键词重叠度（0-1）：用户关键词有多少出现在知识条目中
        itemKeywords := extractKeywords(item.Title + " " + item.Summary)
        keywordScore := jaccardSimilarity(promptKeywords, itemKeywords)
        
        // 加权综合分：向量70% + 关键词30%
        // 如果向量说相关但关键词毫无重叠，综合分会被拉低
        compositeScore := 0.7*vectorScore + 0.3*keywordScore
        
        // 领域匹配加分：用户问题属于某领域，知识也属于该领域，加分
        if promptDomainMatch(prompt, item.Domain) {
            compositeScore += 0.05  // 微调加分，不是决定性的
        }
        
        if compositeScore >= minRelevance {
            item.CompositeScore = compositeScore
            scored = append(scored, item)
        }
    }
    
    // 按综合分排序，取top-k
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].CompositeScore > scored[j].CompositeScore
    })
    
    if len(scored) > topK {
        scored = scored[:topK]
    }
    
    return scored
}
```

### 关键词提取

```go
// extractKeywords 提取有意义的关键词（去除停用词）
func extractKeywords(text string) []string {
    stopwords := map[string]bool{
        "的": true, "了": true, "是": true, "在": true, "有": true,
        "和": true, "与": true, "或": true, "不": true, "吗": true,
        "怎么": true, "什么": true, "如何": true, "哪": true,
        "the": true, "is": true, "a": true, "an": true, "of": true,
        // ... 更多停用词
    }
    
    // 分词（中文需要jieba或简单按字/词切分）
    words := tokenize(text)
    
    result := make([]string, 0)
    for _, w := range words {
        if len(w) >= 2 && !stopwords[w] {  // 至少2字，非停用词
            result = append(result, w)
        }
    }
    return result
}
```

### Jaccard相似度

```go
func jaccardSimilarity(a, b []string) float64 {
    setA := make(map[string]bool)
    for _, s := range a { setA[s] = true }
    
    intersection := 0
    for _, s := range b {
        if setA[s] { intersection++ }
    }
    
    union := len(a) + len(b) - intersection
    if union == 0 { return 0 }
    return float64(intersection) / float64(union)
}
```

### 领域感知关键词权重

```go
// 领域特有关键词表 — 用于判断用户问题属于哪个领域
var domainSignals = map[string][]string{
    "legal": {
        "合同", "违约", "法务", "合规", "诉讼", "仲裁", "条款",
        "协议", "法律", "法规", "知识产权", "保密", "免责",
        "contract", "legal", "compliance", "lawsuit", "clause",
    },
    "finance": {
        "预算", "报表", "毛利率", "营收", "成本", "税务", "审计",
        "财务", "利润", "现金流", "折旧", "营收", "回款",
        "budget", "revenue", "margin", "tax", "audit",
    },
    "hr": {
        "招聘", "薪酬", "绩效", "离职", "入职", "培训", "考勤",
        "社保", "公积金", "加班", "假期", "劳动合同",
        "recruit", "salary", "performance", "onboard",
    },
    // ... 其他领域
}

// promptDomainMatch 判断用户prompt是否和某知识条目的领域匹配
func promptDomainMatch(prompt string, itemDomain string) bool {
    signals, ok := domainSignals[itemDomain]
    if !ok { return false }
    
    lower := strings.ToLower(prompt)
    matched := 0
    for _, s := range signals {
        if strings.Contains(lower, strings.ToLower(s)) {
            matched++
        }
    }
    // 至少命中1个领域关键词才算匹配
    return matched >= 1
}
```

---

## 4. 第2层：领域感知注入策略

### 核心思想：不同领域的风险等级不同，注入策略不同

```
领域风险分级：

高风险（legal, finance）
  → 只注入 verification='verified' 的知识
  → 必须带强免责声明
  → 过期知识（>90天）不注入
  → factual类型必须带original_sentence

中风险（hr, admin, strategy）  
  → 可注入 verification='auto' 的知识
  → 带温和提示
  → 过期知识（>180天）不注入

低风险（sales, service, tech）
  → 可注入 verification='auto' 的知识
  → 简短提示
  → 无过期限制
```

### 领域风险配置表

```sql
CREATE TABLE IF NOT EXISTS wr_knowledge_domain_risk (
    domain_code TEXT PRIMARY KEY,
    risk_level TEXT NOT NULL DEFAULT 'medium',  -- high/medium/low
    min_verification TEXT NOT NULL DEFAULT 'auto',  -- auto/verified
    max_age_days INTEGER DEFAULT 180,           -- 知识最大有效期（天）
    disclaimer_template TEXT DEFAULT '',         -- 免责声明模板
    allow_factual_injection INTEGER DEFAULT 1,  -- 是否注入factual数据
    allow_analytical_injection INTEGER DEFAULT 1,
    allow_procedural_injection INTEGER DEFAULT 1,
    
    FOREIGN KEY (domain_code) REFERENCES wr_knowledge_domains(domain_code)
);

-- 初始配置
INSERT OR IGNORE INTO wr_knowledge_domain_risk VALUES
    ('legal',   'high',   'verified', 90,  '【注意】以下法务信息仅供参考，不构成法律意见。具体法律问题请咨询公司法务部门。', 1, 0, 1),
    ('finance', 'high',   'verified', 90,  '【注意】以下财务数据仅供参考，正式报告以财务部官方数据为准。', 1, 1, 0),
    ('hr',      'medium', 'auto',    180,  '【提示】以下人事信息请以最新公司制度为准。', 1, 0, 1),
    ('admin',   'medium', 'auto',    180,  '【提示】以下行政信息请以最新公司制度为准。', 1, 0, 1),
    ('strategy','medium', 'auto',    180,  '【提示】以下战略信息供内部参考。', 0, 1, 0),
    ('sales',   'low',    'auto',    365,  '', 1, 1, 1),
    ('service', 'low',    'auto',    365,  '', 1, 1, 1),
    ('tech',    'low',    'auto',    365,  '', 1, 1, 1);
```

### 法务领域的特殊规则

```go
func filterRAGItemsForInjection(items []RAGItem, domain string) []RAGItem {
    riskConfig := getDomainRiskConfig(domain)
    now := time.Now()
    
    filtered := make([]RAGItem, 0)
    for _, item := range items {
        // 1. 验证级别检查
        if item.Verification == "pending" || item.Verification == "rejected" {
            continue  // 待审核和已拒绝的一律不注入
        }
        
        // 2. 最低验证级别检查
        if riskConfig.MinVerification == "verified" && item.Verification != "verified" {
            continue  // 高风险域只注入人工verified的
        }
        
        // 3. 时效性检查
        ageDays := now.Sub(item.CreatedAt).Hours() / 24
        if int(ageDays) > riskConfig.MaxAgeDays {
            continue  // 过期知识不注入
        }
        
        // 4. 知识类型检查
        switch item.Type {
        case "factual":
            if riskConfig.AllowFactualInjection == 0 { continue }
        case "analytical":
            if riskConfig.AllowAnalyticalInjection == 0 { continue }
        case "procedural":
            if riskConfig.AllowProceduralInjection == 0 { continue }
        }
        
        filtered = append(filtered, item)
    }
    
    return filtered
}
```

### 法务领域为什么禁止analytical注入？

```
factual（事实数据）："合同违约金为合同总金额的20%" — 可以注入，因为有原文锚定
analytical（分析结论）："该条款存在法律风险" — 不能注入，因为这是LLM的分析，不是法律意见
procedural（流程规范）："合同审批流程5步" — 可以注入，因为是确定性流程

法务领域的特殊性：
  - 分析结论可能被AI当成法律建议 → 实际法律风险
  - 只有verified的factual和procedural可以注入
  - analytical类型只能通过MCP工具主动查询，不能自动注入
```

---

## 5. 第3层：格式约束

### 注入格式模板

```
// 高风险域（legal/finance）
const highRiskRAGTemplate = `以下是从公司知识库自动检索到的参考信息（共%d条）：

%s

【重要提示】
1. 以上信息来自公司知识库，仅供参考，%s
2. 数据最后更新时间：%s，如需最新信息请通过知识库工具查询
3. 带有[待验证]标记的信息尚未经人工审核，请谨慎参考`

// 中风险域（hr/admin/strategy）
const mediumRiskRAGTemplate = `以下是从公司知识库检索到的参考信息：

%s

【提示】以上信息请以公司最新制度为准。`

// 低风险域（sales/service/tech）
const lowRiskRAGTemplate = `以下是公司相关知识供参考：

%s`
```

### 单条知识的注入格式

```go
func formatRAGItem(item RAGItem, riskLevel string) string {
    var sb strings.Builder
    
    // 标题
    sb.WriteString(fmt.Sprintf("■ %s", item.Title))
    
    // 验证状态标记
    if item.Verification == "auto" && riskLevel != "low" {
        sb.WriteString(" [待验证]")  // auto=机器验证，非人工
    }
    sb.WriteString("\n")
    
    // 摘要
    sb.WriteString(fmt.Sprintf("  摘要：%s\n", item.Summary))
    
    // factual类型：注入数据点（带原文锚点）
    if item.Type == "factual" && len(item.DataPoints) > 0 {
        for _, dp := range item.DataPoints {
            sb.WriteString(fmt.Sprintf("  数据：%s = %s", dp.Metric, dp.Value))
            if dp.Period != "" {
                sb.WriteString(fmt.Sprintf("（%s）", dp.Period))
            }
            // 高风险域必须带原文出处
            if riskLevel == "high" {
                sb.WriteString(fmt.Sprintf("\n  原文：「%s」", dp.OriginalSentence))
            }
            if dp.Certainty == "uncertain" {
                sb.WriteString(" [存疑]")
            }
            sb.WriteString("\n")
        }
    }
    
    // 来源时间
    sb.WriteString(fmt.Sprintf("  来源时间：%s\n", item.CreatedAt.Format("2006-01-02")))
    
    return sb.String()
}
```

### 实际注入效果对比

**用户问："合同违约金条款怎么写？"**

❌ 无格式约束的注入：
```
公司知识：
合同违约金为20%
```
AI回答："违约金是20%" — 省略了"合同总金额的"限定词，且当成确定答案

✅ 有格式约束的注入（高风险域）：
```
以下是从公司知识库自动检索到的参考信息（共2条）：

■ 合同违约金规定 [待验证]
  摘要：公司标准合同违约金为合同总金额的20%
  数据：违约金比例 = 20%
  原文：「公司标准合同违约金为合同总金额的20%」
  来源时间：2026-03-15

■ 合同审批流程
  摘要：合同审批需经部门主管、法务、财务三方会签
  来源时间：2026-05-01

【重要提示】
1. 以上信息来自公司知识库，仅供参考，不构成法律意见。具体法律问题请咨询公司法务部门。
2. 数据最后更新时间：2026-05-01，如需最新信息请通过知识库工具查询
3. 带有[待验证]标记的信息尚未经人工审核，请谨慎参考
```

AI回答："根据公司知识库记录，标准合同的违约金为合同总金额的20%。不过请注意这是参考信息，具体条款建议咨询公司法务部门确认。"

**关键区别**：
- 保留了"合同总金额的"这个限定词（因为原文锚点里有）
- AI主动加了免责声明（因为提示告诉它不构成法律意见）
- 标注了[待验证]，AI会更谨慎

---

## 6. RAG注入决策流程

```
用户prompt进入wr-proxy
        │
        ▼
   是否开启RAG？（Token.rag_enabled）
        │ 否 → 不注入
        ▼ 是
   检测用户问题所属领域
   （通过domainSignals关键词匹配）
        │
        ▼
   向量检索 + 关键词交叉验证
   （取topK×3候选，综合评分）
        │
        ▼
   相关度门控
   （compositeScore >= minRelevance？）
        │ 低于阈值 → 不注入（宁可没有，不能错）
        ▼ 通过
   领域风险过滤
   （验证级别/时效性/类型限制）
        │
        ▼
   格式化注入
   （按风险等级选择模板+免责声明）
        │
        ▼
   注入到system prompt
   （插入到messages最前面）
```

### "宁可没有，不能错"原则

```
RAG注入的核心原则：

1. 不确定就不注入
   - 相关度不够 → 不注入
   - 验证状态不够 → 不注入
   - 过期了 → 不注入
   - 领域不匹配 → 不注入

2. 注入了就标明边界
   - 来源时间
   - 验证状态
   - 免责声明
   - 原文锚点（高风险域）

3. AI有权忽略
   - 注入格式中明确说明"如不相关请忽略"
   - AI不应强行引用注入的信息
```

---

## 7. 反馈闭环

RAG注入后，需要持续监控注入质量：

```go
// 在handleProxy响应回写后，异步记录RAG效果
type RAGFeedback struct {
    RequestID     string
    RAGItemsUsed  []int   // 注入了哪些知识条目ID
    UserQuestion  string  // 用户原始问题
    AIResponse    string  // AI回答
    Domain        string
    InjectedCount int
}

// 分析维度：
// 1. AI是否引用了注入的知识？（正面信号）
// 2. AI是否说了"我不确定"后知识又被注入了？（负面信号，说明注入不相关）
// 3. 用户是否紧接着又问了一遍同样的问题？（负面信号，说明AI回答不满足）
```

### 质量指标

| 指标 | 目标 | 说明 |
|------|------|------|
| 注入命中率 | >60% | 注入的知识中，被AI实际引用的比例 |
| 误注入率 | <10% | 注入的知识与问题不相关的比例 |
| 用户满意度 | >80% | RAG注入后用户追问率下降 |
| 高风险域零误注入 | 0% | legal/finance域的误注入必须为零 |

### 自动调优

```
定期分析RAG反馈：
  - 某个域的误注入率高 → 提高该域的minRelevance阈值
  - 某个域的命中率低 → 降低阈值 或 优化关键词提取
  - 某条知识反复被误注入 → 降权 或 标记为"不适合RAG"
```

---

## 8. System Prompt预注入的质量控制

System Prompt是静态注入，风险可控但也要注意：

### 适合注入的内容

```
✅ 你是{公司名}的AI助手。以下是公司基本信息供你了解：

## 公司概况
{50字以内的公司简介}

## 部门职能
- 法务合规部：合同审核、法律咨询、合规管理
- 财务审计部：预算管理、报表编制、税务申报
- 人力资源部：招聘、薪酬、绩效管理
...

## 重要提示
- 涉及法务问题时，提醒用户咨询公司法务部门
- 涉及财务数据时，以财务部官方数据为准
- 如需查询具体公司知识，请使用知识库检索工具
```

### 不适合注入的内容

```
❌ 具体数据（会过时）
  "Q3毛利率18.7%"    → 下一季度就变了

❌ 具体流程（太长+会变）
  "合同审批流程：1.提交 2.部门审批 3.法务审核 4.财务审核 5.总经理签字"
  → 改成提示："合同审批流程请查询知识库"

❌ 敏感信息
  "公司银行账号：6225xxxx"
  → 绝对不能出现在system prompt

❌ 法律条款原文
  "根据《合同法》第114条..."
  → 用提示代替："涉及合同条款请咨询法务"
```

### 法务领域的System Prompt处理

```
法务问题 → System Prompt应该做什么？

1. 角色定位：我是公司AI助手，不是法律顾问
2. 边界提醒：法务问题需要咨询专业法务
3. 能力提示：我可以帮你检索公司知识库中的法务相关信息
4. 禁止行为：不直接给出法律建议、不替用户做法律判断

示例System Prompt片段：
---
## 法务相关指引
- 你的角色是信息检索和整理，不是法律顾问
- 当用户询问法务问题时：
  a. 可以检索知识库中的相关法务知识
  b. 引用知识时必须标注来源和时效性
  c. 必须提醒"以上仅供参考，具体请咨询公司法务部门"
  d. 不要对法律条款做解读或给出是否合规的判断
---
```

---

## 9. wr_tokens 扩展字段

```sql
ALTER TABLE wr_tokens ADD COLUMN rag_enabled INTEGER DEFAULT 0;
ALTER TABLE wr_tokens ADD COLUMN rag_min_relevance REAL DEFAULT 0.7;
ALTER TABLE wr_tokens ADD COLUMN rag_top_k INTEGER DEFAULT 3;
ALTER TABLE wr_tokens ADD COLUMN system_prompt_knowledge TEXT DEFAULT '';
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| rag_enabled | 是否开启RAG自动注入 | 0（关闭） |
| rag_min_relevance | 最低相关度阈值 | 0.7 |
| rag_top_k | 每次注入最多几条知识 | 3 |
| system_prompt_knowledge | 自定义System Prompt知识片段 | 空（使用默认模板） |

---

## 10. 成本影响

| 环节 | 额外延迟 | 额外token |
|------|---------|-----------|
| 领域检测 | <1ms | 0 |
| 向量检索 | 50-200ms | 0 |
| 关键词交叉验证 | <5ms | 0 |
| 风险过滤 | <1ms | 0 |
| 注入到system prompt | 0ms | 500-3000（取决于条数和风险等级） |

**总延迟**：50-200ms（主要是向量检索）
**总token开销**：每次请求多500-3000 tokens（约¥0.001-0.005）

---

## 11. 阶段规划

| 功能 | 一期 | 二期 | 三期 |
|------|------|------|------|
| System Prompt预注入 | ✓ | | |
| 领域检测（关键词匹配） | ✓ | | |
| 向量检索+关键词交叉验证 | | ✓ | |
| 领域风险分级+过滤 | | ✓ | |
| 格式约束+免责声明 | | ✓ | |
| RAG反馈闭环 | | | ✓ |
| 自动调优 | | | ✓ |
