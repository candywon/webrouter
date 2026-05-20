package main

import (
	"encoding/json"
	"fmt"
)

// ============================================================
// RAG 自动注入 — 从用户请求到知识上下文构建
// ============================================================

// buildRAGContext 完整的 RAG 流程：提取 query → 向量化 → 检索 → 格式化
func buildRAGContext(body []byte, token *Token) (string, error) {
	// 1. 提取用户最后一个 query
	query := extractUserQuery(body)
	if query == "" {
		return "", nil
	}

	// 2. 语义检索
	topK := token.RAGTopK
	if topK <= 0 {
		topK = 3
	}
	minRelevance := token.RAGMinRelevance
	if minRelevance <= 0 {
		minRelevance = 0.7
	}

	results, err := SearchVectors(query, topK*2, minRelevance*0.8, token.KnowledgeDepartment, "")
	if err != nil {
		return "", fmt.Errorf("search vectors: %w", err)
	}

	if len(results) == 0 {
		RecordRAGMiss()
		return "", nil
	}

	// 记录反馈
	minSim := results[len(results)-1].Similarity
	maxSim := results[0].Similarity
	RecordRAGHit()
	buildRAGFeedbackFromResults(token, query, results, minSim, maxSim)

	// 3. 质量控制过滤
	filtered, disclaimers := ApplyRAGFilter(results)

	if len(filtered) == 0 {
		return "", nil
	}

	// 4. 格式化为文本
	ctx := formatRAGContext(filtered)

	// 5. 追加免责声明
	if len(disclaimers) > 0 {
		ctx += "\n" + joinStrings(disclaimers, "\n")
	}

	return ctx, nil
}

// extractUserQuery 提取请求体中最后一条 user 消息
func extractUserQuery(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	msgs, ok := req["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		return ""
	}

	// 从后往前找最后一条 user 消息
	for i := len(msgs) - 1; i >= 0; i-- {
		msg, ok := msgs[i].(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := msg["role"].(string); role == "user" {
			content, _ := msg["content"].(string)
			if len(content) > 500 {
				content = content[:500]
			}
			return content
		}
	}
	return ""
}

// formatRAGContext 将检索结果格式化为可读上下文
func formatRAGContext(results []SearchResult) string {
	var buf string

	buf += "【内部知识参考】以下信息来自企业内部知识库，供回答时参考：\n\n"

	for i, r := range results {
		buf += fmt.Sprintf("%d. [%s|%s] %s\n", i+1, r.Type, r.DomainCode, r.Title)
		if r.Summary != "" {
			buf += "   " + r.Summary + "\n"
		}
		buf += fmt.Sprintf("   (相关度: %.0f%%)\n\n", r.Similarity*100)
	}

	return buf
}
