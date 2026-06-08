// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// shouldCapture 第1级信号筛选：判断是否值得捕获为知识
func shouldCapture(entry KnowledgeEntry) bool {
	// 1. 响应太短（<200字符），大概率是简单问答
	if len(entry.Response) < 200 {
		return false
	}

	// 2. 对话轮数太少（<3轮）且响应不够长，大概率是简单查询
	if entry.TurnCount < 3 && len(entry.Response) < 500 {
		return false
	}

	// 3. 代码占比过高（>60%），大概率是编程
	if codeBlockRatio(entry.Prompt+entry.Response) > 0.6 {
		return false
	}

	// 4. 明确的翻译/改写意图
	if isTranslationOrRewrite(entry.Prompt) {
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
