// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// KnowledgeAnalyzeRequest 单域分析请求
type KnowledgeAnalyzeRequest struct {
	DomainCode    string   `json:"domain_code"`    // 目标业务域
	Department    string   `json:"department"`     // 可选：部门筛选
	Types         []string `json:"types"`          // 可选：知识类型筛选
	DateFrom      string   `json:"date_from"`      // 可选：起始日期 YYYY-MM-DD
	DateTo        string   `json:"date_to"`        // 可选：截止日期
	AnalysisType  string   `json:"analysis_type"`  // domain_overview / trend / gap
	ModelOverride string   `json:"model_override"` // 可选：覆盖默认模型
	TokenID       int      `json:"token_id"`       // 发起分析的 Token
}

// KnowledgeAnalyzeResult 分析结果
type KnowledgeAnalyzeResult struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	Summary         string `json:"summary"`
	KeyFindings     string `json:"key_findings"`
	Recommendations string `json:"recommendations"`
	Error           string `json:"error,omitempty"`
}

// analyzeKnowledge 单域分析引擎入口
func analyzeKnowledge(req KnowledgeAnalyzeRequest) (string, error) {
	taskID := fmt.Sprintf("analysis_%d_%d", time.Now().Unix(), req.TokenID)

	// 1. 记录分析任务到数据库
	record := KnowledgeAnalysisRecord{
		TaskID:       taskID,
		TokenID:      req.TokenID,
		DomainCode:   req.DomainCode,
		Department:   req.Department,
		AnalysisType: req.AnalysisType,
		Status:       "processing",
	}

	if err := record.save(); err != nil {
		return "", fmt.Errorf("save analysis record: %w", err)
	}

	// 2. 查询目标域的知识条目
	items, err := queryKnowledgeItems(req)
	if err != nil {
		record.updateStatus("failed", err.Error())
		return "", err
	}

	if len(items) == 0 {
		record.updateStatus("completed", "该领域暂无知识条目可供分析")
		return fmt.Sprintf("业务域「%s」当前暂无知识条目。请先通过对话积累相关知识，或扩大时间范围。", req.DomainCode), nil
	}

	// 3. 统计分析数据
	stats := analyzeItems(items, req)

	// 4. 构造分析 Prompt 并调用 LLM
	result, err := callLLMForAnalysis(items, stats, req)
	if err != nil {
		record.updateStatus("failed", err.Error())
		return "", err
	}

	// 5. 保存结果
	record.updateStatus("completed", result)
	record.updateStats(len(items))

	return result, nil
}

// KnowledgeAnalysisRecord 分析记录 DB 操作
type KnowledgeAnalysisRecord struct {
	TaskID       string
	TokenID      int
	DomainCode   string
	Department   string
	AnalysisType string
	Status       string
	Result       string
}

func (r *KnowledgeAnalysisRecord) save() error {
	_, err := db.Exec(`
		INSERT INTO wr_knowledge_analyses
		(task_id, token_id, domains, analysis_type, status, created_at)
		VALUES (?, ?, ?, ?, 'processing', ?)`,
		r.TaskID, r.TokenID, r.DomainCode, r.AnalysisType,
		time.Now().UTC().Format("2006-01-02 15:04:05"))
	return err
}

func (r *KnowledgeAnalysisRecord) updateStatus(status, result string) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
		UPDATE wr_knowledge_analyses
		SET status = ?, result = ?, completed_at = ?
		WHERE task_id = ?`, status, result, now, r.TaskID)
	if err != nil {
		LogWarn("update analysis record: %v", err)
	}
}

func (r *KnowledgeAnalysisRecord) updateStats(count int) {
	_, err := db.Exec(`
		UPDATE wr_knowledge_analyses SET item_count = ? WHERE task_id = ?`,
		count, r.TaskID)
	if err != nil {
		LogWarn("update analysis stats: %v", err)
	}
}

// knowledgeItemRaw 查询用临时结构
type knowledgeItemRaw struct {
	ID           int
	Type         string
	Title        string
	Summary      string
	DomainCode   string
	Confidence   float64
	Verification string
	CreatedAt    string
}

// queryKnowledgeItems 根据分析请求查询知识条目
func queryKnowledgeItems(req KnowledgeAnalyzeRequest) ([]knowledgeItemRaw, error) {
	var conditions []string
	var args []interface{}

	if req.DomainCode != "" {
		conditions = append(conditions, "domain_code = ?")
		args = append(args, req.DomainCode)
	}
	if req.Department != "" {
		conditions = append(conditions, "department = ?")
		args = append(args, req.Department)
	}
	if len(req.Types) > 0 {
		placeholders := make([]string, len(req.Types))
		for i, t := range req.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, "type IN ("+strings.Join(placeholders, ",")+")")
	}
	if req.DateFrom != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, req.DateFrom)
	}
	if req.DateTo != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, req.DateTo+" 23:59:59")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, type, title, summary, domain_code, confidence, verification, created_at
		FROM wr_knowledge_items %s ORDER BY created_at DESC LIMIT 100`, where)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query knowledge items: %w", err)
	}
	defer rows.Close()

	var items []knowledgeItemRaw
	for rows.Next() {
		var item knowledgeItemRaw
		var createdAtStr string
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Summary,
			&item.DomainCode, &item.Confidence, &item.Verification, &createdAtStr); err != nil {
			continue
		}
		item.CreatedAt = createdAtStr
		items = append(items, item)
	}
	return items, nil
}

// analyzeItems 统计分析
func analyzeItems(items []knowledgeItemRaw, req KnowledgeAnalyzeRequest) map[string]interface{} {
	stats := map[string]interface{}{
		"total_items":    len(items),
		"by_type":        map[string]int{},
		"by_domain":      map[string]int{},
		"avg_confidence": 0.0,
	}

	typeCount := map[string]int{}
	domainCount := map[string]int{}
	totalConfidence := 0.0

	for _, item := range items {
		typeCount[item.Type]++
		if item.DomainCode != "" {
			domainCount[item.DomainCode]++
		}
		totalConfidence += item.Confidence
	}

	stats["by_type"] = typeCount
	stats["by_domain"] = domainCount
	if len(items) > 0 {
		stats["avg_confidence"] = totalConfidence / float64(len(items))
	}
	return stats
}

// callLLMForAnalysis 调用上游 LLM 进行分析
func callLLMForAnalysis(items []knowledgeItemRaw, stats map[string]interface{}, req KnowledgeAnalyzeRequest) (string, error) {
	// 构造分析 Prompt
	prompt := buildAnalysisPrompt(items, stats, req)

	// 选择模型
	model := req.ModelOverride
	if model == "" {
		model = selectAnalysisModel(stats["total_items"].(int))
	}

	// 调用上游 LLM
	result, err := callUpstreamLLM(model, prompt)
	if err != nil {
		// LLM 调用失败时返回本地统计分析
		LogWarn("LLM analysis failed, falling back to local stats: %v", err)
		return buildLocalAnalysis(stats, items), nil
	}

	return result, nil
}

// buildAnalysisPrompt 构造分析 Prompt
func buildAnalysisPrompt(items []knowledgeItemRaw, stats map[string]interface{}, req KnowledgeAnalyzeRequest) string {
	var sb strings.Builder

	sb.WriteString("你是一个企业知识分析专家。请对以下企业知识库中的知识条目进行分析。\n\n")

	sb.WriteString(fmt.Sprintf("## 分析任务\n"))
	sb.WriteString(fmt.Sprintf("- 业务域: %s\n", req.DomainCode))
	if req.AnalysisType != "" {
		sb.WriteString(fmt.Sprintf("- 分析类型: %s\n", req.AnalysisType))
	}
	sb.WriteString(fmt.Sprintf("- 知识条目总数: %d\n\n", len(items)))

	sb.WriteString("## 统计数据\n")
	if byType, ok := stats["by_type"].(map[string]int); ok {
		sb.WriteString(fmt.Sprintf("- 按类型: %v\n", byType))
	}
	sb.WriteString(fmt.Sprintf("- 平均置信度: %.0f%%\n\n", stats["avg_confidence"].(float64)*100))

	sb.WriteString("## 知识条目列表\n")
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, item.Type, item.Title))
		sb.WriteString(fmt.Sprintf("   摘要: %s\n", truncate(item.Summary, 200)))
		sb.WriteString(fmt.Sprintf("   置信度: %.0f%%, 验证: %s\n\n", item.Confidence*100, item.Verification))
	}

	sb.WriteString("## 分析要求\n")
	sb.WriteString("请输出以下三部分内容：\n")
	sb.WriteString("1. **概要**：一段话概括该领域的知识整体情况\n")
	sb.WriteString("2. **关键发现**：列出 3-5 个值得注意的发现\n")
	sb.WriteString("3. **建议**：基于分析结果，给出 2-3 条改进建议\n")

	return sb.String()
}

// selectAnalysisModel 根据知识量自动选择模型
func selectAnalysisModel(itemCount int) string {
	if itemCount <= 10 {
		return "qwen3-coder-flash" // 少量数据用轻量模型
	}
	return "qwen-plus" // 较大数据用中等模型
}

// callUpstreamLLM 调用上游 LLM 进行分析
func callUpstreamLLM(model, prompt string) (string, error) {
	// 查找可用的 Provider
	providers := router.GetProviders()
	if len(providers) == 0 {
		return "", fmt.Errorf("no available provider")
	}

	provider := providers[0]

	// 构造请求体
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": "你是一个企业知识分析专家。"},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  4000,
		"temperature": 0.3,
	}

	bodyBytes, _ := json.Marshal(body)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequest("POST", provider.BaseURL+"/v1/chat/completions", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)

	// 发送请求
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call LLM: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return "", fmt.Errorf("parse LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("empty LLM response")
	}

	return llmResp.Choices[0].Message.Content, nil
}

// buildLocalAnalysis 本地统计分析（LLM 不可用时的降级方案）
func buildLocalAnalysis(stats map[string]interface{}, items []knowledgeItemRaw) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s 领域分析报告\n\n", stats["by_domain"]))
	sb.WriteString(fmt.Sprintf("- 知识条目总数: %d\n", stats["total_items"]))
	sb.WriteString(fmt.Sprintf("- 平均置信度: %.0f%%\n", stats["avg_confidence"].(float64)*100))

	sb.WriteString("\n### 按类型分布\n")
	if byType, ok := stats["by_type"].(map[string]int); ok {
		for t, c := range byType {
			sb.WriteString(fmt.Sprintf("- %s: %d 条\n", t, c))
		}
	}

	sb.WriteString("\n### 最新知识\n")
	limit := min(5, len(items))
	for i := 0; i < limit; i++ {
		sb.WriteString(fmt.Sprintf("- %s [%s]: %s\n", items[i].Type, items[i].CreatedAt, items[i].Title))
	}

	sb.WriteString("\n*注：LLM 分析不可用，以上为本地统计结果。*")
	return sb.String()
}
