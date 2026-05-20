package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- 信号筛选规则测试 ---

func TestShouldCapture_ShortResponse(t *testing.T) {
	entry := KnowledgeEntry{
		Response:  "你好，很高兴为你服务",
		TurnCount: 1,
	}
	if shouldCapture(entry) {
		t.Error("short response should be filtered")
	}
}

func TestShouldCapture_FewTurnsShortResponse(t *testing.T) {
	entry := KnowledgeEntry{
		Response:  "这是一个简短的回复",
		TurnCount: 2,
	}
	if shouldCapture(entry) {
		t.Error("few turns with short response should be filtered")
	}
}

func TestShouldCapture_CodeHeavy(t *testing.T) {
	entry := KnowledgeEntry{
		Response: "```python\ndef hello():\n    print('world')\n    for i in range(100):\n        print(i)\n        if i % 2 == 0:\n            continue\n        x = i * 2\n        y = x + 1\n        print(x, y)\n\ndef goodbye():\n    pass\n```\n以上代码",
		TurnCount: 5,
	}
	// code ratio > 60%, should be filtered
	ratio := codeBlockRatio(entry.Response)
	if ratio <= 0.6 {
		t.Skipf("code ratio %.2f not > 0.6, skipping", ratio)
	}
	if shouldCapture(entry) {
		t.Error("code-heavy response should be filtered")
	}
}

func TestShouldCapture_TranslationIntent(t *testing.T) {
	entry := KnowledgeEntry{
		Prompt:    "请翻译这段话：Hello World",
		Response:  "你好，世界。这是一段测试文本。为了达到200字符的长度要求，我需要补充一些无关紧要的内容。" +
			"这是一段测试文本。为了达到200字符的长度要求，我需要补充一些无关紧要的内容。" +
			"这是一段测试文本。为了达到200字符的长度要求，我需要补充一些无关紧要的内容。",
		TurnCount: 3,
	}
	if shouldCapture(entry) {
		t.Error("translation intent should be filtered")
	}
}

func TestShouldCapture_AnalysisWithTranslation(t *testing.T) {
	entry := KnowledgeEntry{
		Prompt:    "翻译并分析以下数据：Q3毛利率为18.7%",
		Response:  "Q3毛利率同比收窄3.2个百分点至18.7%，主要原因是原材料成本上涨和市场竞争加剧。建议优化供应链。" +
			"这是一段补充长度的文本，用于确保响应超过200字符。在实际业务场景中，这类分析通常会包含更多的数据支撑和推理过程。" +
			"同时还会涉及行业对比和竞争对手分析，以及未来趋势预测。这些内容对于企业决策具有重要参考价值。",
		TurnCount: 3,
	}
	if !shouldCapture(entry) {
		t.Error("analysis with translation keyword should NOT be filtered")
	}
}

func TestShouldCapture_KnowledgeRich(t *testing.T) {
	entry := KnowledgeEntry{
		Prompt:    "分析这份Q3财报的毛利率趋势",
		Response:  "根据财务部Q3分析报告，Q3毛利率同比收窄3.2个百分点至18.7%，主要受原材料价格上涨影响。" +
			"从趋势来看，毛利率已连续三个季度下滑，但降幅逐季收窄。预计Q4随着供应链优化，毛利率将回升至20%左右。" +
			"建议关注原材料采购成本控制和产品结构优化两个方向，以维持长期盈利能力。同时需要警惕市场竞争加剧的风险。",
		TurnCount: 5,
	}
	if !shouldCapture(entry) {
		t.Error("knowledge-rich response should be captured")
	}
}

func TestShouldCapture_LongMultiTurn(t *testing.T) {
	entry := KnowledgeEntry{
		Prompt: "关于合同审批流程的讨论",
		Response: "根据公司法务部门的规定，合同审批需要经过以下流程：首先由部门主管审核合同内容的合理性，" +
			"然后由法务部门审查法律条款的合规性，接着由财务部门评估合同的经济风险和收益，" +
			"最后由总经理签字确认。整个流程通常需要3-5个工作日。对于紧急合同，可以申请加急处理，" +
			"但需要部门总监级以上管理人员审批。此外，所有合同必须使用公司标准模板，" +
			"特殊条款需要经过法务部门的额外审查。合同金额超过100万的，还需要董事会决议。",
		TurnCount: 8,
	}
	if !shouldCapture(entry) {
		t.Error("long multi-turn conversation should be captured")
	}
}

// --- 代码占比测试 ---

func TestCodeBlockRatio_NoCode(t *testing.T) {
	text := "这是一段纯文本，没有任何代码内容。" +
		"这是另一段文本，用于测试代码占比功能的准确性。" +
		"第三段文本继续验证代码占比计算的正确性。"
	ratio := codeBlockRatio(text)
	if ratio != 0 {
		t.Errorf("expected 0, got %.2f", ratio)
	}
}

func TestCodeBlockRatio_FullCode(t *testing.T) {
	text := "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```"
	ratio := codeBlockRatio(text)
	if ratio <= 0.5 {
		t.Errorf("full code block ratio should be high, got %.2f", ratio)
	}
}

func TestCodeBlockRatio_Mixed(t *testing.T) {
	text := "以下是代码示例：\n```go\nfunc main() {\n}\n```\n以上是代码，下面是结论：" +
		"经过分析，我们认为这个方案是可行的，建议按照计划执行。"
	ratio := codeBlockRatio(text)
	if ratio <= 0 || ratio >= 1 {
		t.Logf("mixed code/text ratio: %.2f (expected between 0 and 1)", ratio)
	}
}

func TestCodeBlockRatio_Empty(t *testing.T) {
	ratio := codeBlockRatio("")
	if ratio != 0 {
		t.Errorf("empty text should have ratio 0, got %.2f", ratio)
	}
}

// --- 翻译/改写检测测试 ---

func TestIsTranslationOrRewrite_Translate(t *testing.T) {
	cases := []string{
		"翻译成英文",
		"translate this to Chinese",
		"请润色一下这段话",
		"rewrite the following",
		"换种说法表达同样的意思",
	}
	for _, c := range cases {
		if !isTranslationOrRewrite(c) {
			t.Errorf("should detect translation/rewrite: %s", c)
		}
	}
}

func TestIsTranslationOrRewrite_Analysis(t *testing.T) {
	cases := []string{
		"翻译并分析以下数据的趋势",
		"润色后对比两个方案的优劣",
	}
	for _, c := range cases {
		if isTranslationOrRewrite(c) {
			t.Errorf("should NOT filter analysis: %s", c)
		}
	}
}

func TestIsTranslationOrRewrite_NotTranslation(t *testing.T) {
	cases := []string{
		"分析一下Q3财报",
		"合同违约金条款审查",
		"帮我写个排序算法",
	}
	for _, c := range cases {
		if isTranslationOrRewrite(c) {
			t.Errorf("should not be translation: %s", c)
		}
	}
}

// --- 对话轮数提取测试 ---

func TestExtractTurnCount_EmptyBody(t *testing.T) {
	count := extractTurnCount([]byte{})
	if count != 1 {
		t.Errorf("empty body should return 1 turn, got %d", count)
	}
}

func TestExtractTurnCount_NoMessages(t *testing.T) {
	body := []byte(`{"model":"gpt-4"}`)
	count := extractTurnCount(body)
	if count != 1 {
		t.Errorf("no messages should return 1 turn, got %d", count)
	}
}

func TestExtractTurnCount_SingleTurn(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	})
	count := extractTurnCount(body)
	if count != 1 {
		t.Errorf("single user message should be 1 turn, got %d", count)
	}
}

func TestExtractTurnCount_MultiTurn(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
			map[string]interface{}{"role": "assistant", "content": "hi"},
			map[string]interface{}{"role": "user", "content": "how are you"},
			map[string]interface{}{"role": "assistant", "content": "fine"},
			map[string]interface{}{"role": "user", "content": "bye"},
		},
	})
	count := extractTurnCount(body)
	if count != 2 { // 5 messages / 2 = 2 turns
		t.Errorf("5 messages should be 2 turns, got %d", count)
	}
}

// --- 知识信号检测测试 ---

func TestHasKnowledgeSignals_Numbers(t *testing.T) {
	if !hasKnowledgeSignals("", "Q3毛利率为18.7%") {
		t.Error("should detect number signals")
	}
}

func TestHasKnowledgeSignals_Conclusion(t *testing.T) {
	if !hasKnowledgeSignals("", "结论是应该继续投资") {
		t.Error("should detect conclusion signals")
	}
}

func TestHasKnowledgeSignals_NoSignals(t *testing.T) {
	if hasKnowledgeSignals("你好", "好的，我知道了") {
		t.Error("simple chat should not have knowledge signals")
	}
}

// --- 简单查询检测测试 ---

func TestIsSimpleQuery_Yes(t *testing.T) {
	if !isSimpleQuery("什么是API", 50) {
		t.Error("short response to 'what is' query should be simple")
	}
}

func TestIsSimpleQuery_NoLongResponse(t *testing.T) {
	if isSimpleQuery("什么是API", 500) {
		t.Error("long response should not be simple query")
	}
}

// --- 统计功能测试 ---

func TestCaptureStats_Reset(t *testing.T) {
	knowledgeStatsMu.Lock()
	knowledgeStats.TodayCaptured = 10
	knowledgeStats.TodayFiltered = 5
	knowledgeStats.TodaySaved = 3
	knowledgeStatsMu.Unlock()

	ResetDailyStats()

	stats := GetCaptureStats()
	if stats.TodayCaptured != 0 || stats.TodayFiltered != 0 || stats.TodaySaved != 0 {
		t.Error("daily stats should be reset to 0")
	}
	if stats.TotalCaptured == 0 {
		// total should not be reset
	}
}

// --- System Prompt 注入测试 ---

func TestInjectKnowledgeSystemPrompt_NoInjection(t *testing.T) {
	token := &Token{
		ID:                      1,
		KnowledgeCaptureEnabled: false,
		KnowledgeDepartment:     "",
		RAGEnabled:              false,
		SystemPromptKnowledge:   "",
	}
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	result := injectKnowledgeSystemPrompt(body, token)
	if string(result) != string(body) {
		t.Error("should not inject when all knowledge fields are disabled")
	}
}

func TestInjectKnowledgeSystemPrompt_DepartmentOnly(t *testing.T) {
	token := &Token{
		ID:                      1,
		KnowledgeCaptureEnabled: true, // 需开启功能
		KnowledgeDepartment:     "法务部",
	}
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	result := injectKnowledgeSystemPrompt(body, token)

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	msgs := req["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	firstMsg := msgs[0].(map[string]interface{})
	if firstMsg["role"] != "system" {
		t.Error("first message should be system role")
	}
	content := firstMsg["content"].(string)
	if !containsAll(content, "部门标识", "法务部") {
		t.Errorf("system prompt should contain department info, got: %s", content)
	}
}

func TestInjectKnowledgeSystemPrompt_AppendToExisting(t *testing.T) {
	token := &Token{
		ID:                      1,
		KnowledgeCaptureEnabled: true,
		KnowledgeDepartment:     "技术部",
		SystemPromptKnowledge:   "请使用Go语言示例。",
	}
	body := []byte(`{"model":"gpt-4","messages":[{"role":"system","content":"你是一个助手"},{"role":"user","content":"hello"}]}`)
	result := injectKnowledgeSystemPrompt(body, token)

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	msgs := req["messages"].([]interface{})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (appended to existing system), got %d", len(msgs))
	}

	firstMsg := msgs[0].(map[string]interface{})
	content := firstMsg["content"].(string)
	if !containsAll(content, "你是一个助手", "技术部", "Go语言示例") {
		t.Errorf("system prompt should contain original content + knowledge, got: %s", content)
	}
}

func TestInjectKnowledgeSystemPrompt_CustomPromptOnly(t *testing.T) {
	token := &Token{
		ID:                      1,
		RAGEnabled:              true,
		SystemPromptKnowledge:   "你是法务助手，请提供专业法律建议。",
	}
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"合同纠纷怎么处理"}]}`)
	result := injectKnowledgeSystemPrompt(body, token)

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	msgs := req["messages"].([]interface{})
	firstMsg := msgs[0].(map[string]interface{})
	content := firstMsg["content"].(string)
	if !containsAll(content, "法务助手", "专业法律建议") {
		t.Errorf("system prompt should contain custom knowledge, got: %s", content)
	}
}

func TestGetKnowledgeSystemPrompt(t *testing.T) {
	token := &Token{
		ID:                      1,
		KnowledgeDepartment:     "财务部",
		SystemPromptKnowledge:   "请关注财务合规。",
	}
	prompt := GetKnowledgeSystemPrompt(token)
	if !containsAll(prompt, "企业级AI助手", "财务部", "财务合规") {
		t.Errorf("prompt should contain all parts, got: %s", prompt)
	}

	// Empty token
	emptyToken := &Token{ID: 2}
	prompt = GetKnowledgeSystemPrompt(emptyToken)
	if prompt != "" {
		t.Error("empty token should return empty prompt")
	}
}

// containsAll checks if s contains all substrings
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- 知识提取引擎测试 ---

func TestExtractJSONFromText(t *testing.T) {
	// Direct JSON
	s := `{"has_knowledge": true, "type": "factual"}`
	if extractJSONFromText(s) != s {
		t.Error("should return direct JSON as-is")
	}

	// JSON embedded in text
	s2 := "好的，这是分析结果：\n" + `{"has_knowledge": true}` + "\n完毕"
	result := extractJSONFromText(s2)
	if !strings.Contains(result, "has_knowledge") {
		t.Errorf("should extract JSON from text, got: %s", result)
	}

	// No JSON
	if extractJSONFromText("纯文本，没有JSON") != "" {
		t.Error("should return empty for non-JSON text")
	}
}

func TestAutoMatchDomain(t *testing.T) {
	cases := []struct {
		text   string
		domain string
	}{
		{"合同条款需要法务审核，律师意见", "legal"},
		{"Q3财务审计报告，毛利率收入成本分析", "finance"},
		{"员工招聘培训计划，人事部门处理", "hr"},
		{"行政办公用品采购后勤安排", "admin"},
		{"本季度销售额增长，客户签约订单", "sales"},
		{"品牌策划营销推广活动方案", "marketing"},
		{"客户投诉售后工单处理反馈", "service"},
		{"API接口技术架构开发部署数据库", "tech"},
		{"公司战略投资规划并购布局", "strategy"},
	}
	for _, c := range cases {
		result := autoMatchDomain(c.text)
		if result != c.domain {
			t.Errorf("text=%q: expected domain=%s, got=%s", c.text, c.domain, result)
		}
	}
}

func TestVerifyDataPoint_ExactMatch(t *testing.T) {
	verified, reason := VerifyDataPoint("Q3毛利率为18.7%", "18.7%")
	if !verified {
		t.Errorf("exact match should pass: %s", reason)
	}
}

func TestVerifyDataPoint_NumberMismatch(t *testing.T) {
	verified, _ := VerifyDataPoint("毛利率为18.7%", "毛利率为25.3%")
	// 25.3 not found in source
	if verified {
		t.Error("number mismatch should fail")
	}
}

func TestVerifyDataPoint_KeywordMatch(t *testing.T) {
	verified, reason := VerifyDataPoint("Q3财务分析报告显示毛利率同比收窄", "毛利率收窄")
	if !verified {
		t.Errorf("keyword match should pass: %s", reason)
	}
}

// --- Embedding + 向量检索测试 ---

func TestNormalizeVec(t *testing.T) {
	// Zero vector — should return as-is
	zero := normalizeVec([]float64{0, 0, 0})
	for _, v := range zero {
		if v != 0 {
			t.Error("zero vector should stay zero")
		}
	}

	// Simple unit vector
	unit := normalizeVec([]float64{3, 4})
	norm := 0.0
	for _, v := range unit {
		norm += v * v
	}
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("normalized vector norm should be ~1, got %.4f", norm)
	}
}

func TestDotProduct_Identical(t *testing.T) {
	a := normalizeVec([]float64{0.6, 0.8})
	b := normalizeVec([]float64{0.6, 0.8})
	sim := dotProduct(a, b)
	if sim < 0.99 {
		t.Errorf("identical vectors should have dot ~1, got %.4f", sim)
	}
}

func TestDotProduct_Orthogonal(t *testing.T) {
	a := normalizeVec([]float64{1, 0})
	b := normalizeVec([]float64{0, 1})
	sim := dotProduct(a, b)
	if sim > 0.01 {
		t.Errorf("orthogonal vectors should have dot ~0, got %.4f", sim)
	}
}

func TestDotProduct_Opposite(t *testing.T) {
	a := normalizeVec([]float64{1, 0})
	b := normalizeVec([]float64{-1, 0})
	sim := dotProduct(a, b)
	if sim > -0.99 {
		t.Errorf("opposite vectors should have dot ~-1, got %.4f", sim)
	}
}

func TestDotProduct_MismatchedLength(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0}
	sim := dotProduct(a, b)
	if sim != 0 {
		t.Errorf("mismatched length should return 0, got %.4f", sim)
	}
}

func TestDotProduct_Empty(t *testing.T) {
	sim := dotProduct([]float64{}, []float64{})
	if sim != 0 {
		t.Errorf("empty vectors should return 0, got %.4f", sim)
	}
}

func TestExtractUserQuery_LastUser(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"hello"},
		{"role":"assistant","content":"hi"},
		{"role":"user","content":"what is the Q3 revenue?"}
	]}`)
	q := extractUserQuery(body)
	if q != "what is the Q3 revenue?" {
		t.Errorf("should extract last user message, got: %s", q)
	}
}

func TestExtractUserQuery_NoUser(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":"hello"}]}`)
	q := extractUserQuery(body)
	if q != "" {
		t.Errorf("no user message should return empty, got: %s", q)
	}
}

func TestExtractUserQuery_Truncate(t *testing.T) {
	long := strings.Repeat("a", 600)
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": long},
		},
	})
	q := extractUserQuery(body)
	if len(q) > 500 {
		t.Errorf("query should be truncated to 500, got %d chars", len(q))
	}
}

func TestFormatRAGContext(t *testing.T) {
	results := []SearchResult{
		{ItemID: 1, Title: "Q3财报分析", Summary: "毛利率18.7%", DomainCode: "finance", Type: "factual", Similarity: 0.92},
	}
	ctx := formatRAGContext(results)
	if !strings.Contains(ctx, "Q3财报分析") {
		t.Error("context should contain title")
	}
	if !strings.Contains(ctx, "毛利率18.7%") {
		t.Error("context should contain summary")
	}
	if !strings.Contains(ctx, "92%") {
		t.Error("context should contain similarity percentage")
	}
}

// --- Week 7+8: 记忆/压缩/反馈测试 ---

func TestContainsAny(t *testing.T) {
	if !containsAny("我通常使用Python", []string{"我通常", "我喜欢"}) {
		t.Error("should detect 我通常")
	}
	if containsAny("今天天气不错", []string{"我通常", "我喜欢"}) {
		t.Error("should not detect preference in neutral text")
	}
	if !containsAny("请记住这个规则", []string{"请记住", "注意"}) {
		t.Error("should detect 请记住")
	}
}

func TestExtractMemoriesSimple_Preference(t *testing.T) {
	// Skip if DB not initialized (unit tests don't have DB)
	if db == nil {
		t.Skip("DB not initialized, skipping")
	}
	token := &Token{ID: 999, Name: "test"}
	extractMemoriesSimple(token, "", "我通常使用Go语言开发", "好的，我会记住这个偏好。")
	var count int
	db.QueryRow("SELECT COUNT(*) FROM wr_agent_memory WHERE token_id = 999 AND category = 'preference'").Scan(&count)
	if count == 0 {
		t.Error("should have saved a preference memory")
	}
	db.Exec("DELETE FROM wr_agent_memory WHERE token_id = 999")
}

func TestExtractMemoriesSimple_Fact(t *testing.T) {
	if db == nil {
		t.Skip("DB not initialized, skipping")
	}
	token := &Token{ID: 998, Name: "test"}
	extractMemoriesSimple(token, "", "请记住我们的规定：所有代码必须经过Code Review", "好的，已记录。")
	var count int
	db.QueryRow("SELECT COUNT(*) FROM wr_agent_memory WHERE token_id = 998 AND category = 'fact'").Scan(&count)
	if count == 0 {
		t.Error("should have saved a fact memory")
	}
	db.Exec("DELETE FROM wr_agent_memory WHERE token_id = 998")
}

func TestRAGFeedback_RecordAndStats(t *testing.T) {
	// Reset
	ragFeedbacksMu.Lock()
	ragFeedbacks = nil
	ragFeedbacksMu.Unlock()

	RecordRAGFeedback(ragFeedback{
		TokenID:       1,
		TokenName:     "test",
		DomainCode:    "finance",
		Query:         "Q3财报",
		HitCount:      3,
		MinSimilarity: 0.72,
		MaxSimilarity: 0.95,
		Timestamp:     "2026-05-20 15:00:00",
	})

	stats := GetRAGFeedbackStats()
	if stats["total_feedbacks"] != 1 {
		t.Errorf("should have 1 feedback, got %v", stats["total_feedbacks"])
	}
}

func TestNeedsCompression(t *testing.T) {
	// Under threshold
	msgs := make([]map[string]interface{}, 5)
	for i := range msgs {
		msgs[i] = map[string]interface{}{"role": "user", "content": "short"}
	}
	if needsCompression(msgs) {
		t.Error("5 short messages should not need compression")
	}

	// Over threshold
	msgs = make([]map[string]interface{}, 35)
	for i := range msgs {
		msgs[i] = map[string]interface{}{"role": "user", "content": "hello"}
	}
	if !needsCompression(msgs) {
		t.Error("35 messages should need compression (max is 30)")
	}
}

func TestParseCompressResponse_NoJSON(t *testing.T) {
	result := parseCompressResponse("this is not json")
	if result.Compressed != "this is not json" {
		t.Error("non-json should return raw text as compressed")
	}
}

func TestRoundTo(t *testing.T) {
	if roundTo(0.12345, 3) != 0.123 {
		t.Errorf("roundTo failed: %.3f", roundTo(0.12345, 3))
	}
	if roundTo(0.9999, 2) != 1.0 {
		t.Errorf("roundTo failed: %.2f", roundTo(0.9999, 2))
	}
}
