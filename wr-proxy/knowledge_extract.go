// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ============================================================
// LLM 提炼引擎 — 从 raw 表提取结构化知识
// ============================================================

// KnowledgeExtractBatch 批量提取配置
type KnowledgeExtractBatch struct {
	BatchSize  int // 每次处理的 raw 条数
	Model      string
	TimeoutSec int
}

var extractBatch = KnowledgeExtractBatch{
	BatchSize:  5,
	Model:      "qwen3-coder-flash",
	TimeoutSec: 120,
}

// ExtractionResult LLM 单条 raw 提取结果
type ExtractionResult struct {
	HasKnowledge  bool     `json:"has_knowledge"`
	KnowledgeType string   `json:"knowledge_type,omitempty"` // factual/analytical/procedural
	Confidence    float64  `json:"confidence"`
	DomainCode    string   `json:"domain_code,omitempty"`
	Title         string   `json:"title,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	DataPoints    []string `json:"data_points,omitempty"` // factual 类型的数据点
}

// rawEntry 从数据库查询的原始对话条目
type rawEntry struct {
	ID        int
	RequestID string
	TokenID   int
	TokenName string
	ModelName string
	Prompt    string
	Response  string
	TurnCount int
	CreatedAt string
}

// ExtractRawToKnowledge 从 raw 表批量提取知识
func ExtractRawToKnowledge() (processed int, err error) {
	// 1. 查询 pending 状态的 raw 数据
	rows, err := db.Query(`
		SELECT id, request_id, token_id, token_name, model_name,
		       prompt, response, turn_count, created_at
		FROM wr_knowledge_raw
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?`, extractBatch.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("query pending raw: %w", err)
	}
	defer rows.Close()

	var entries []rawEntry
	for rows.Next() {
		var e rawEntry
		if err := rows.Scan(&e.ID, &e.RequestID, &e.TokenID, &e.TokenName, &e.ModelName,
			&e.Prompt, &e.Response, &e.TurnCount, &e.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}

	if len(entries) == 0 {
		return 0, nil
	}

	// 2. 逐条调用 LLM 评估 + 提取
	for i, entry := range entries {
		_ = i // silence unused variable

		// 2.1 标记为 processing
		_, err = db.Exec(`UPDATE wr_knowledge_raw SET status = 'processing' WHERE id = ?`, entry.ID)
		if err != nil {
			LogWarn("[extract] mark processing failed: %v", err)
		}

		// 2.2 LLM 评估
		result, err := assessRawEntry(entry)
		if err != nil {
			LogWarn("[extract] LLM assessment failed for entry %d: %v, falling back to skip", entry.ID, err)
			db.Exec(`UPDATE wr_knowledge_raw SET status = 'skipped', processed_at = ? WHERE id = ?`,
				time.Now().UTC().Format("2006-01-02 15:04:05"), entry.ID)
			continue
		}

		if !result.HasKnowledge {
			// 没有知识价值，跳过
			db.Exec(`UPDATE wr_knowledge_raw SET status = 'skipped', processed_at = ? WHERE id = ?`,
				time.Now().UTC().Format("2006-01-02 15:04:05"), entry.ID)
			continue
		}

		// 2.3 发现新 domain
		if result.DomainCode != "" {
			if err := discoverDomain(result.DomainCode, result.Summary); err != nil {
				LogWarn("[extract] discover domain '%s' failed: %v", result.DomainCode, err)
			}
		}

		// 2.4 写入知识条目
		itemID, err := saveKnowledgeItem(rawEntryForAssess{
			ID:        entry.ID,
			Prompt:    entry.Prompt,
			Response:  entry.Response,
			TurnCount: entry.TurnCount,
			ModelName: entry.ModelName,
			TokenName: entry.TokenName,
		}, result)
		if err != nil {
			LogWarn("[extract] save knowledge item failed for entry %d: %v", entry.ID, err)
			db.Exec(`UPDATE wr_knowledge_raw SET status = 'skipped', processed_at = ? WHERE id = ?`,
				time.Now().UTC().Format("2006-01-02 15:04:05"), entry.ID)
			continue
		}

		// 2.4.1 投递 embedding 任务（异步，非阻塞）
		if result.Summary != "" {
			QueueEmbedding(int(itemID), result.Summary)
		}

		// 2.5 标记为 done
		db.Exec(`UPDATE wr_knowledge_raw SET status = 'done', processed_at = ? WHERE id = ?`,
			time.Now().UTC().Format("2006-01-02 15:04:05"), entry.ID)

		// 提取完成后立即删除 raw 原文（数据安全好实践）
		db.Exec(`DELETE FROM wr_knowledge_raw WHERE id = ?`, entry.ID)

		// 审计日志：知识提取
		LogAudit(AuditKnowledgeExtract, AuditResourceItem,
			entry.RequestID, entry.TokenID, map[string]interface{}{
				"raw_id":         entry.ID,
				"item_id":        itemID,
				"knowledge_type": result.KnowledgeType,
				"domain":         result.DomainCode,
			}, "")

		LogInfo("[extract] entry %d → knowledge item %d (type=%s, domain=%s, confidence=%.0f%%)",
			entry.ID, itemID, result.KnowledgeType, result.DomainCode, result.Confidence*100)
		processed++
	}

	return processed, nil
}

// assessRawEntry 调用 LLM 评估单条 raw 对话的知识价值
func assessRawEntry(entry rawEntry) (ExtractionResult, error) {
	prompt := buildAssessPrompt(rawEntryForAssess{
		ID:        entry.ID,
		Prompt:    entry.Prompt,
		Response:  entry.Response,
		TurnCount: entry.TurnCount,
		ModelName: entry.ModelName,
		TokenName: entry.TokenName,
	})
	response, err := callExtractionLLM(prompt)
	if err != nil {
		return ExtractionResult{}, err
	}

	return parseAssessResponse(response, rawEntryForAssess{
		Prompt:    entry.Prompt,
		Response:  entry.Response,
		TurnCount: entry.TurnCount,
		ModelName: entry.ModelName,
		TokenName: entry.TokenName,
	})
}

// rawEntryForAssess 临时类型，避免循环依赖
type rawEntryForAssess struct {
	ID        int
	Prompt    string
	Response  string
	TurnCount int
	ModelName string
	TokenName string
}

// buildAssessPrompt 构造知识评估 Prompt
func buildAssessPrompt(entry rawEntryForAssess) string {
	var buf bytes.Buffer

	buf.WriteString("你是一个企业知识提取专家。请分析以下对话是否包含有价值的企业知识。\n\n")

	buf.WriteString(fmt.Sprintf("## 对话内容\n"))
	buf.WriteString(fmt.Sprintf("【对话轮数】%d\n", entry.TurnCount))
	buf.WriteString(fmt.Sprintf("【Prompt】\n%s\n\n", truncate(entry.Prompt, 2000)))
	buf.WriteString(fmt.Sprintf("【Response】\n%s\n\n", truncate(entry.Response, 3000)))

	buf.WriteString("## 提取要求\n")
	buf.WriteString("请判断该对话是否包含以下类型的企业知识：\n")
	buf.WriteString("- **factual**（事实性）：具体数据、指标、规则、定义、联系方式、配置信息等\n")
	buf.WriteString("- **analytical**（分析性）：分析结论、趋势判断、原因推断、决策建议等\n")
	buf.WriteString("- **procedural**（流程性）：操作步骤、审批流程、规范指南、SOP等\n\n")

	buf.WriteString("## 输出格式（严格 JSON，不要其他内容）\n")
	buf.WriteString(`{"has_knowledge": true/false, "knowledge_type": "factual/analytical/procedural", "confidence": 0.0-1.0, "domain_code": "对应业务域代码", "title": "知识标题（50字以内）", "summary": "知识摘要（200字以内）", "data_points": ["事实性数据点数组（仅factual类型需要）"]}`)

	return buf.String()
}

// parseAssessResponse 解析 LLM 评估响应
func parseAssessResponse(response string, entry rawEntryForAssess) (ExtractionResult, error) {
	// 尝试从 response 中提取 JSON
	jsonStr := extractJSONFromText(response)
	if jsonStr == "" {
		return ExtractionResult{}, fmt.Errorf("no JSON found in LLM response")
	}

	var result ExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ExtractionResult{}, fmt.Errorf("parse JSON: %w, raw: %s", err, jsonStr)
	}

	// 校验
	if result.KnowledgeType != "" {
		result.KnowledgeType = strings.ToLower(result.KnowledgeType)
		if result.KnowledgeType != "factual" && result.KnowledgeType != "analytical" && result.KnowledgeType != "procedural" {
			result.HasKnowledge = false
		}
	}

	// 如果 LLM 未给出 domain_code，自动匹配
	if result.HasKnowledge && result.DomainCode == "" {
		result.DomainCode = autoMatchDomain(entry.Prompt + " " + entry.Response)
	}

	return result, nil
}

// extractJSONFromText 从文本中提取 JSON 对象
func extractJSONFromText(text string) string {
	text = strings.TrimSpace(text)

	// 尝试直接解析
	if strings.HasPrefix(text, "{") {
		return text
	}

	// 查找第一个 { 到最后一个 }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}

	return ""
}

// autoMatchDomain 自动匹配业务域
func autoMatchDomain(text string) string {
	textLower := strings.ToLower(text)

	// 关键词匹配
	domainKeywords := map[string][]string{
		"legal":     {"法", "合规", "合同", "诉讼", "仲裁", "法律", "律师", "条款", "协议"},
		"finance":   {"财务", "审计", "税", "预算", "决算", "毛利", "利润", "收入", "成本", "报销", "账务", "资金"},
		"hr":        {"人", "招聘", "培训", "薪酬", "绩效", "考勤", "入职", "离职", "员工", "人事"},
		"admin":     {"行政", "办公", "物业", "后勤", "采购", "用品", "印章", "用车"},
		"sales":     {"销售", "客户", "签约", "订单", "渠道", "代理商", "商务", "报价"},
		"marketing": {"市场", "品牌", "运营", "策划", "推广", "营销", "活动", "公关", "社媒"},
		"service":   {"客服", "售后", "投诉", "工单", "咨询", "反馈", "维修"},
		"tech":      {"技术", "代码", "开发", "架构", "部署", "API", "接口", "数据库", "服务器", "算法"},
		"strategy":  {"战略", "规划", "投资", "并购", "方向", "布局", "赛道"},
	}

	bestDomain := ""
	bestScore := 0
	for domain, keywords := range domainKeywords {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestDomain = domain
		}
	}

	if bestScore >= 2 {
		return bestDomain
	}
	return ""
}

// saveKnowledgeItem 将提取结果写入 wr_knowledge_items 表
func saveKnowledgeItem(entry rawEntryForAssess, result ExtractionResult) (int64, error) {
	dataPointsJSON := "[]"
	if len(result.DataPoints) > 0 {
		b, _ := json.Marshal(result.DataPoints)
		dataPointsJSON = string(b)
	}

	// 确定 verification 状态
	verification := "auto"
	if result.KnowledgeType == "factual" {
		verification = "pending" // factual 需要验证
	}

	result_, err := db.Exec(`
		INSERT INTO wr_knowledge_items
		(type, title, summary, domain_code, department,
		 source_request_id, source_turn_index, source_quote, source_char_start, source_char_end,
		 data_points, confidence, verification,
		 token_id, token_name, model_name, sensitivity, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'medium', ?)`,
		result.KnowledgeType,
		truncate(result.Title, 200),
		truncate(result.Summary, 2000),
		result.DomainCode,
		"",       // department 留空，后续可由用户设置
		entry.ID, // 用 raw id 作为 source_request_id 替代
		entry.TurnCount,
		truncate(entry.Response, 1000),
		0,
		len(entry.Response),
		dataPointsJSON,
		result.Confidence,
		verification,
		0, // token_id
		entry.TokenName,
		entry.ModelName,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)

	if err != nil {
		return 0, err
	}

	id, _ := result_.LastInsertId()
	return id, nil
}

// discoverDomain 发现新业务域并插入 wr_knowledge_domains
func discoverDomain(domainCode, sampleText string) error {
	// 检查是否已存在
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM wr_knowledge_domains WHERE domain_code = ?`, domainCode).Scan(&count)
	if count > 0 {
		return nil
	}

	// 使用 autoMatchDomain 推测部门名称
	domainName := autoMatchDomain(domainCode)
	if domainName == "" {
		domainName = domainCode
	}

	_, err := db.Exec(`
		INSERT OR IGNORE INTO wr_knowledge_domains
		(domain_code, domain_name, status, sample_count)
		VALUES (?, ?, 'pending', 1)`,
		domainCode, domainName)

	// 同时插入风险配置（默认 low）
	if err == nil {
		db.Exec(`
			INSERT OR IGNORE INTO wr_knowledge_domain_risk
			(domain_code, risk_level, min_verification, max_age_days)
			VALUES (?, 'low', 'auto', 365)`, domainCode)
	}

	return err
}

// callExtractionLLM 调用上游 LLM 进行评估
func callExtractionLLM(prompt string) (string, error) {
	providers := router.GetProviders()
	if len(providers) == 0 {
		return "", fmt.Errorf("no available provider")
	}

	// 选择一个可用的 Provider（不盲目取第一个）
	var provider *Provider
	for _, p := range providers {
		if p.IsAvailable("") {
			provider = p
			break
		}
	}
	if provider == nil {
		return "", fmt.Errorf("no healthy provider available")
	}

	body := map[string]interface{}{
		"model": extractBatch.Model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": "你是一个企业知识提取专家。请严格按 JSON 格式输出，不要其他内容。"},
			{"role": "user", "content": prompt},
		},
		"max_tokens":      1000,
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, _ := json.Marshal(body)

	// 构造上游 URL（处理 /v1 重复问题）
	upstreamURL := provider.BaseURL + "/v1/chat/completions"
	if strings.HasSuffix(provider.BaseURL, "/v1") {
		upstreamURL = provider.BaseURL + "/chat/completions"
	}

	httpReq, err := http.NewRequest("POST", upstreamURL,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)

	client := &http.Client{Timeout: time.Duration(extractBatch.TimeoutSec) * time.Second}
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

// ============================================================
// 数据点机器验证 — VerifyDataPoint
// ============================================================

// VerifyDataPoint 验证事实性数据点的一致性
// 检查 raw response 中是否包含提取出的 data_point
func VerifyDataPoint(rawResponse, dataPoint string) (bool, string) {
	// 1. 精确匹配
	if strings.Contains(rawResponse, dataPoint) {
		return true, "exact_match"
	}

	// 2. 数字一致性检查：提取 data_point 中的关键数字，检查 raw 中是否存在
	nums := extractNumbers(dataPoint)
	for _, num := range nums {
		if !strings.Contains(rawResponse, num) {
			return false, fmt.Sprintf("number_mismatch: %s not found in source", num)
		}
	}

	if len(nums) > 0 {
		return true, "numbers_verified"
	}

	// 3. 关键词匹配（至少 60% 的词匹配）
	words := extractKeywords(dataPoint)
	if len(words) == 0 {
		return false, "empty_data_point"
	}

	matched := 0
	for _, w := range words {
		if strings.Contains(rawResponse, w) {
			matched++
		}
	}

	ratio := float64(matched) / float64(len(words))
	if ratio >= 0.6 {
		return true, fmt.Sprintf("keyword_match_%.0f%%", ratio*100)
	}

	return false, fmt.Sprintf("low_keyword_match: %.0f%%", ratio*100)
}

// extractNumbers 提取字符串中的数字
func extractNumbers(s string) []string {
	re := regexp.MustCompile(`\d+\.?\d*%?`)
	return re.FindAllString(s, -1)
}

// extractKeywords 提取中文关键词（简单分词）
func extractKeywords(s string) []string {
	// 简单的双字词/三字词提取
	var words []string
	runes := []rune(s)
	for i := 0; i < len(runes)-1; i++ {
		word := string(runes[i : i+2])
		// 过滤标点符号
		if !isPunctuation(word) {
			words = append(words, word)
		}
	}
	return words
}

func isPunctuation(s string) bool {
	punct := "，。！？；：()[]{}<>-+*/=!?.,;:\n\r\t "
	return strings.ContainsAny(s, punct)
}
