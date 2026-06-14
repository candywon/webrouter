// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// shouldCapture 第1级信号筛选：判断是否值得捕获为知识
// 策略：先识别是否工作/问询内容，再过滤噪音；宁可多捕获再提取，不遗漏有价值信息
func shouldCapture(entry KnowledgeEntry) bool {
	// 1. 极短响应（<30字符），基本是确认词，无知识价值
	if len(entry.Response) < 30 {
		return false
	}

	// 2. 纯寒暄/闲聊检测
	if isSmallTalk(entry.Prompt) && len(entry.Response) < 150 {
		return false
	}

	// 3. 问询/检索意图 → 强信号，直接捕获（知识多来自问询场景）
	if isKnowledgeRetrieval(entry.Prompt) {
		return true
	}

	// 4. 包含工作信号 → 直接捕获（不论长度）
	if hasWorkSignals(entry.Prompt, entry.Response) {
		return true
	}

	// 5. 短响应但包含知识信号（数字、结论、建议等）→ 捕获
	if len(entry.Response) < 200 {
		return hasKnowledgeSignals(entry.Prompt, entry.Response)
	}

	// 6. 明确的翻译/改写意图 → 不捕获
	if isTranslationOrRewrite(entry.Prompt) {
		return false
	}

	// 7. 代码占比过高（>80%），纯编程实现，非知识
	if codeBlockRatio(entry.Prompt+entry.Response) > 0.8 {
		return false
	}

	return true
}

// codeBlockRatio 统计代码块字符占比
func codeBlockRatio(text string) float64 {
	codeChars := 0
	inCodeBlock := false
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			codeChars += len(line)
		}
	}

	totalChars := len(text)
	if totalChars == 0 {
		return 0
	}
	return float64(codeChars) / float64(totalChars)
}

var translationPatterns = []string{
	"翻译", "translate", "翻訳", "번역",
	"改写", "润色", "rewrite", "paraphrase",
	"换种说法", "rephrase",
}

var analysisKeywords = []string{"分析", "计算", "对比", "评估", "analyze", "compare"}

// isSmallTalk 判断是否为寒暄/闲聊
func isSmallTalk(prompt string) bool {
	lower := strings.TrimSpace(strings.ToLower(prompt))
	smallTalkPatterns := []string{
		"你好", "hi", "hello", "hey", "嗨", "早上好", "下午好", "晚上好",
		"good morning", "good afternoon", "good evening",
		"谢谢", "thanks", "thank you", "好的", "ok", "okay",
		"再见", "bye", "goodbye",
	}
	for _, p := range smallTalkPatterns {
		if lower == p || lower == p+"！" || lower == p+"!" || lower == p+"。" {
			return true
		}
	}
	if len([]rune(prompt)) <= 6 {
		return true
	}
	return false
}

// isKnowledgeRetrieval 判断是否为知识问询/检索意图
// 这类 prompt 是最强烈的知识捕获信号：用户在寻找/回忆信息，说明该信息有知识价值
func isKnowledgeRetrieval(prompt string) bool {
	lower := strings.ToLower(prompt)

	// 中文问询/检索词
	retrievalPatterns := []string{
		// 查找/检索类
		"找一下", "查一下", "搜一下", "查到", "找到", "搜索",
		"看一下", "帮我找", "帮我查", "哪里有", "谁有",
		// 问询类
		"问一下", "请问", "咨询一下", "确认一下", "核实",
		// 记忆/回忆类
		"还记得", "记不记得", "回忆一下", "想起来", "之前说",
		"上次", "以前", "之前讨论", "之前聊过", "刚才说的",
		// 了解/获取类
		"了解一下", "说明一下", "介绍一下", "解释一下",
		"告诉我", "说说", "讲一下", "有没有",
		// 归纳/整理类
		"归纳", "梳理", "整理一下", "总结一下", "列一下",
		"哪些", "几个", "多少",
		// 比较类
		"区别", "对比", "比较", "不同", "差异",
	}
	for _, p := range retrievalPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// 英文问询模式
	engPatterns := []string{
		"find", "search", "look up", "check",
		"remember", "recall", "what did we", "previously",
		"tell me about", "explain", "describe",
		"summarize", "organize", "list", "compare",
	}
	for _, p := range engPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// 疑问句式（"XX是什么"、"怎么XX"、"为什么XX"）
	questionPatterns := []string{"是什么", "怎么", "为什么", "如何", "哪些",
		"what is", "how to", "how do", "why", "which"}
	for _, p := range questionPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

// hasWorkSignals 判断是否包含工作/业务信号
func hasWorkSignals(prompt, response string) bool {
	combined := strings.ToLower(prompt + " " + response)

	// 业务/工作关键词
	workKeywords := []string{
		"方案", "策略", "流程", "规范", "标准", "架构", "设计", "部署", "运维",
		"配置", "需求", "排期", "交付", "上线", "回滚", "监控", "告警",
		"报告", "总结", "归纳", "梳理", "清单", "checklist",
		"成本", "预算", "报价", "合同", "供应商", "客户",
		"policy", "strategy", "process", "standard", "architecture",
		"design", "deploy", "config", "requirement", "schedule",
		"release", "rollback", "monitor", "alert", "report",
	}
	for _, w := range workKeywords {
		if strings.Contains(combined, w) {
			return true
		}
	}

	// 包含结构化内容（列表、步骤、表格标记）
	structureSignals := []string{"1.", "1、", "- ", "•", "步骤", "首先", "step "}
	for _, s := range structureSignals {
		if strings.Contains(combined, s) {
			return true
		}
	}

	return false
}

// isTranslationOrRewrite 判断 prompt 是否为纯翻译/改写意图
func isTranslationOrRewrite(prompt string) bool {
	lower := strings.ToLower(prompt)
	for _, p := range translationPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			// 如果同时包含分析/计算等关键词，可能不只是翻译
			for _, ak := range analysisKeywords {
				if strings.Contains(lower, strings.ToLower(ak)) {
					return false // 包含分析意图，保留
				}
			}
			return true
		}
	}
	return false
}

// extractTurnCount 从请求体中提取对话轮数
func extractTurnCount(body []byte) int {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return 1
	}
	messages, ok := obj["messages"]
	if !ok {
		return 1
	}
	msgList, ok := messages.([]interface{})
	if !ok {
		return 1
	}
	// 每轮 = user + assistant
	turns := len(msgList) / 2
	if turns < 1 {
		return 1
	}
	return turns
}

// isSimpleQuery 判断是否为简单查询（"是什么"/"什么是"类问题）
func isSimpleQuery(prompt string, responseLen int) bool {
	lower := strings.ToLower(prompt)
	simplePatterns := []string{"是什么", "什么是", "what is", "define", "定义"}
	for _, p := range simplePatterns {
		if strings.Contains(lower, p) && responseLen < 100 {
			return true
		}
	}
	return false
}

// hasKnowledgeSignals 判断是否包含知识信号（用于补充 shouldCapture）
func hasKnowledgeSignals(prompt, response string) bool {
	lower := strings.ToLower(prompt + " " + response)

	// 包含具体数字/日期/金额
	numRegex := regexp.MustCompile(`\d+\.?\d*\s*(%|元|万|亿|天|人|件|个|月|年|季度)`)
	if numRegex.MatchString(lower) {
		return true
	}

	// 包含判断/结论类关键词
	conclusionWords := []string{"结论", "建议", "分析", "总结", "原因是", "所以",
		"conclusion", "recommend", "analysis", "because"}
	for _, w := range conclusionWords {
		if strings.Contains(lower, w) {
			return true
		}
	}

	return false
}
