// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"strings"
)

// registerKnowledgeMCPTools 注册知识库相关 MCP 工具
func registerKnowledgeMCPTools() {
	registerMCPTool(MCPToolDef{
		Name:        "knowledge_search",
		Description: "搜索企业知识库中的知识条目和原始对话，支持关键词模糊匹配。返回匹配的知识条目摘要。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"keyword": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词，支持中英文",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "可选：限定业务域（如 legal, finance, tech 等）",
				},
			},
			"required": []string{"keyword"},
		},
		Handler: mcpToolKnowledgeSearch,
	})

	registerMCPTool(MCPToolDef{
		Name:        "knowledge_get_detail",
		Description: "获取单条知识条目的详细内容，包括标题、摘要、来源引用、数据点和置信度。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "integer",
					"description": "知识条目 ID",
				},
			},
			"required": []string{"id"},
		},
		Handler: mcpToolKnowledgeGetDetail,
	})

	registerMCPTool(MCPToolDef{
		Name:        "knowledge_list_domains",
		Description: "列出所有已注册的业务域及其部门归属、状态、风险等级。",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: mcpToolKnowledgeListDomains,
	})

	registerMCPTool(MCPToolDef{
		Name:        "knowledge_list_items",
		Description: "列出知识条目，支持按域/部门/类型/验证状态筛选。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "业务域代码",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "知识类型: factual/analytical/procedural",
				},
				"verification": map[string]interface{}{
					"type":        "string",
					"description": "验证状态: auto/pending/verified/rejected",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回条数上限，默认10",
				},
			},
		},
		Handler: mcpToolKnowledgeListItems,
	})

	registerMCPTool(MCPToolDef{
		Name:        "knowledge_stats",
		Description: "获取知识库统计信息：原始对话数量、知识条目数量、各域分布等。",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: mcpToolKnowledgeStats,
	})

	// 持久记忆工具
	registerMCPTool(MCPToolDef{
		Name:        "memory_save",
		Description: "保存一条持久化记忆。用于记住用户偏好、事实、目标等跨会话信息。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type":        "string",
					"description": "记忆类别: preference/fact/context/goal/constraint",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "记忆标题",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "记忆内容",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "标签列表",
				},
				"priority": map[string]interface{}{
					"type":        "integer",
					"description": "优先级 1-5，5最重要",
				},
				"expires_at": map[string]interface{}{
					"type":        "string",
					"description": "过期时间（可选），格式: 2006-01-02 15:04:05",
				},
			},
			"required": []string{"category", "title", "content"},
		},
		Handler: mcpToolMemorySave,
	})

	registerMCPTool(MCPToolDef{
		Name:        "memory_recall",
		Description: "检索持久化记忆。支持按类别过滤，返回相关历史信息。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type":        "string",
					"description": "可选：按类别过滤",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回条数上限，默认10",
				},
			},
		},
		Handler: mcpToolMemoryRecall,
	})

	// ---- CE 新增工具 ----
	registerMCPTool(MCPToolDef{
		Name:        "provider_health",
		Description: "列出所有活跃 Provider 的健康状态、类型、延迟。帮助了解当前各 API 源的工作状况。",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: mcpToolProviderHealth,
	})

	registerMCPTool(MCPToolDef{
		Name:        "model_list",
		Description: "列出所有最近使用过的模型。支持按 provider 过滤。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider": map[string]interface{}{
					"type":        "string",
					"description": "可选：按 Provider 名称过滤",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回条数上限，默认 20",
				},
			},
		},
		Handler: mcpToolModelList,
	})

	registerMCPTool(MCPToolDef{
		Name:        "request_stats",
		Description: "查询请求统计聚合数据，支持按小时/天分组，按 model/provider/token 聚合。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hours": map[string]interface{}{
					"type":        "integer",
					"description": "统计时间范围（小时），默认 24",
				},
				"group_by": map[string]interface{}{
					"type":        "string",
					"description": "聚合维度: model/provider/token，默认 model",
				},
			},
		},
		Handler: mcpToolRequestStats,
	})

	registerMCPTool(MCPToolDef{
		Name:        "routing_strategy",
		Description: "查询当前路由策略配置。",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: mcpToolRoutingStrategy,
	})
}
func mcpToolKnowledgeSearch(args map[string]interface{}) (string, error) {
	keyword, ok := args["keyword"].(string)
	if !ok || keyword == "" {
		return "", fmt.Errorf("keyword parameter is required")
	}

	domain, _ := args["domain"].(string)
	department, _ := args["department"].(string)

	// 搜索知识条目
	var query string
	var queryArgs []interface{}
	if domain != "" {
		query = `SELECT id, type, title, summary, domain_code, confidence, verification, created_at
			FROM wr_knowledge_items
			WHERE domain_code = ? AND (title LIKE ? OR summary LIKE ? OR source_quote LIKE ?)`
		queryArgs = append(queryArgs, domain, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	} else {
		query = `SELECT id, type, title, summary, domain_code, confidence, verification, created_at
			FROM wr_knowledge_items
			WHERE title LIKE ? OR summary LIKE ? OR source_quote LIKE ?`
		queryArgs = append(queryArgs, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 部门权限过滤
	if department != "" {
		query += " AND (department = ? OR department = '')"
		queryArgs = append(queryArgs, department)
	}

	query += " ORDER BY confidence DESC LIMIT 10"

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var id int
		var typ, title, summary, domainCode, verification, createdAt string
		var confidence float64
		if err := rows.Scan(&id, &typ, &title, &summary, &domainCode, &confidence, &verification, &createdAt); err != nil {
			continue
		}
		results = append(results, fmt.Sprintf(
			"[%d] [%s] %s (域: %s, 置信度: %.0f%%, 状态: %s)\n  %s",
			id, typ, title, domainCode, confidence*100, verification,
			truncate(summary, 120),
		))
	}

	// 同时搜索 raw 表数量
	var rawCount int
	rawQuery := `SELECT COUNT(*) FROM wr_knowledge_raw WHERE status != 'skipped' AND (prompt LIKE ? OR response LIKE ?)`
	db.QueryRow(rawQuery, "%"+keyword+"%", "%"+keyword+"%").Scan(&rawCount)

	if len(results) == 0 && rawCount == 0 {
		return fmt.Sprintf("未找到包含「%s」的知识条目或原始对话。", keyword), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索「%s」：找到 %d 条知识，%d 条原始对话。\n\n", keyword, len(results), rawCount))
	for _, r := range results {
		sb.WriteString(r + "\n\n")
	}
	return sb.String(), nil
}

// mcpToolKnowledgeGetDetail 获取知识详情
func mcpToolKnowledgeGetDetail(args map[string]interface{}) (string, error) {
	idFloat, ok := args["id"].(float64)
	if !ok {
		return "", fmt.Errorf("id parameter is required (integer)")
	}
	id := int(idFloat)

	var typ, title, summary, domainCode, department, sourceQuote, sourceReqID, dataPoints, tokenName, modelName, sensitivity string
	var confidence float64
	var verification, createdAt, updatedAt string
	var turnIndex, charStart, charEnd int

	err := db.QueryRow(`
		SELECT id, type, title, summary, domain_code, department, source_request_id,
		       source_turn_index, source_quote, source_char_start, source_char_end,
		       data_points, confidence, verification, token_name, model_name,
		       sensitivity, created_at, updated_at
		FROM wr_knowledge_items WHERE id = ?`, id,
	).Scan(&id, &typ, &title, &summary, &domainCode, &department, &sourceReqID,
		&turnIndex, &sourceQuote, &charStart, &charEnd,
		&dataPoints, &confidence, &verification, &tokenName, &modelName,
		&sensitivity, &createdAt, &updatedAt)
	if err != nil {
		return "", fmt.Errorf("知识条目 #%d 不存在", id)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString(fmt.Sprintf("- **类型**: %s\n", typ))
	sb.WriteString(fmt.Sprintf("- **业务域**: %s (%s)\n", domainCode, department))
	sb.WriteString(fmt.Sprintf("- **置信度**: %.0f%%\n", confidence*100))
	sb.WriteString(fmt.Sprintf("- **验证状态**: %s\n", verification))
	sb.WriteString(fmt.Sprintf("- **敏感度**: %s\n", sensitivity))
	sb.WriteString(fmt.Sprintf("- **来源**: Token=%s, Model=%s\n", tokenName, modelName))
	sb.WriteString(fmt.Sprintf("- **创建时间**: %s\n\n", createdAt))
	sb.WriteString(fmt.Sprintf("## 摘要\n%s\n\n", summary))
	sb.WriteString(fmt.Sprintf("## 来源引用\n> %s\n\n", truncate(sourceQuote, 300)))
	if dataPoints != "" {
		sb.WriteString(fmt.Sprintf("## 数据点\n%s\n", dataPoints))
	}
	return sb.String(), nil
}

// mcpToolKnowledgeListDomains 列出业务域
func mcpToolKnowledgeListDomains(args map[string]interface{}) (string, error) {
	rows, err := db.Query(`
		SELECT d.domain_code, d.domain_name, d.department, d.status, d.sample_count,
		       COALESCE(r.risk_level, 'unknown') as risk_level,
		       COALESCE(r.min_verification, 'auto') as min_verification,
		       COALESCE(r.max_age_days, 180) as max_age_days
		FROM wr_knowledge_domains d
		LEFT JOIN wr_knowledge_domain_risk r ON d.domain_code = r.domain_code
		ORDER BY d.id ASC`)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("## 业务域列表\n\n")
	sb.WriteString("| 代码 | 名称 | 部门 | 状态 | 样本数 | 风险 | 最小验证 | 最长有效期 |\n")
	sb.WriteString("|------|------|------|------|--------|------|----------|------------|\n")

	count := 0
	for rows.Next() {
		var code, name, dept, status, risk, minVer string
		var sampleCount, maxAge int
		if err := rows.Scan(&code, &name, &dept, &status, &sampleCount, &risk, &minVer, &maxAge); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d | %s | %s | %d天 |\n",
			code, name, dept, status, sampleCount, risk, minVer, maxAge))
		count++
	}

	if count == 0 {
		sb.WriteString("暂无业务域数据。\n")
	}
	return sb.String(), nil
}

// mcpToolKnowledgeListItems 列出知识条目
func mcpToolKnowledgeListItems(args map[string]interface{}) (string, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	var conditions []string
	var queryArgs []interface{}

	if domain, ok := args["domain"].(string); ok && domain != "" {
		conditions = append(conditions, "domain_code = ?")
		queryArgs = append(queryArgs, domain)
	}
	if typ, ok := args["type"].(string); ok && typ != "" {
		conditions = append(conditions, "type = ?")
		queryArgs = append(queryArgs, typ)
	}
	if ver, ok := args["verification"].(string); ok && ver != "" {
		conditions = append(conditions, "verification = ?")
		queryArgs = append(queryArgs, ver)
	}
	if dept, ok := args["department"].(string); ok && dept != "" {
		conditions = append(conditions, "(department = ? OR department = '')")
		queryArgs = append(queryArgs, dept)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, type, title, summary, domain_code, confidence, verification, created_at
		FROM wr_knowledge_items %s ORDER BY created_at DESC LIMIT %d`, where, limit)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("## 知识条目\n\n")

	count := 0
	for rows.Next() {
		var id int
		var typ, title, summary, domainCode, verification, createdAt string
		var confidence float64
		if err := rows.Scan(&id, &typ, &title, &summary, &domainCode, &confidence, &verification, &createdAt); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- **[%d]** [%s] %s (域: %s, 置信度: %.0f%%)\n  %s\n\n",
			id, typ, title, domainCode, confidence*100, truncate(summary, 100)))
		count++
	}

	if count == 0 {
		sb.WriteString("暂无匹配的知识条目。\n")
	} else {
		sb.WriteString(fmt.Sprintf("\n共返回 %d 条。", count))
	}
	return sb.String(), nil
}

// mcpToolKnowledgeStats 知识统计
func mcpToolKnowledgeStats(args map[string]interface{}) (string, error) {
	var totalRaw, pendingRaw, doneRaw int
	db.QueryRow("SELECT COUNT(*) FROM wr_knowledge_raw").Scan(&totalRaw)
	db.QueryRow("SELECT COUNT(*) FROM wr_knowledge_raw WHERE status = 'pending'").Scan(&pendingRaw)
	db.QueryRow("SELECT COUNT(*) FROM wr_knowledge_raw WHERE status = 'done'").Scan(&doneRaw)

	var totalItems int
	db.QueryRow("SELECT COUNT(*) FROM wr_knowledge_items").Scan(&totalItems)

	// 按类型统计
	typeCount := make(map[string]int)
	rows, _ := db.Query("SELECT type, COUNT(*) FROM wr_knowledge_items GROUP BY type")
	for rows != nil && rows.Next() {
		var typ string
		var cnt int
		if err := rows.Scan(&typ, &cnt); err == nil {
			typeCount[typ] = cnt
		}
	}

	// 按域统计
	domainCount := make(map[string]int)
	rows2, _ := db.Query("SELECT domain_code, COUNT(*) FROM wr_knowledge_items WHERE domain_code != '' GROUP BY domain_code")
	for rows2 != nil && rows2.Next() {
		var code string
		var cnt int
		if err := rows2.Scan(&code, &cnt); err == nil {
			domainCount[code] = cnt
		}
	}

	var totalDomains int
	db.QueryRow("SELECT COUNT(*) FROM wr_knowledge_domains").Scan(&totalDomains)

	var sb strings.Builder
	sb.WriteString("## 知识库统计\n\n")
	sb.WriteString(fmt.Sprintf("- 原始对话: %d 条 (待处理: %d, 已处理: %d)\n", totalRaw, pendingRaw, doneRaw))
	sb.WriteString(fmt.Sprintf("- 知识条目: %d 条\n", totalItems))
	sb.WriteString(fmt.Sprintf("- 业务域: %d 个\n", totalDomains))
	sb.WriteString("\n### 按类型\n")
	for t, c := range typeCount {
		sb.WriteString(fmt.Sprintf("- %s: %d 条\n", t, c))
	}
	sb.WriteString("\n### 按领域\n")
	for d, c := range domainCount {
		sb.WriteString(fmt.Sprintf("- %s: %d 条\n", d, c))
	}
	return sb.String(), nil
}

// mcpToolMemorySave 保存持久记忆
func mcpToolMemorySave(args map[string]interface{}) (string, error) {
	category, ok := args["category"].(string)
	if !ok || category == "" {
		return "", fmt.Errorf("category is required")
	}
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	tokenID := 0
	if tid, ok := args["token_id"].(float64); ok {
		tokenID = int(tid)
	}
	tokenName := ""
	if tn, ok := args["token_name"].(string); ok {
		tokenName = tn
	}

	priority := 3
	if p, ok := args["priority"].(float64); ok && p >= 1 && p <= 5 {
		priority = int(p)
	}

	expiresAt := ""
	if exp, ok := args["expires_at"].(string); ok {
		expiresAt = exp
	}

	var tags []string
	if t, ok := args["tags"].([]interface{}); ok {
		for _, v := range t {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	token := &Token{ID: tokenID, Name: tokenName}
	id, err := SaveMemory(token, "", category, title, content, tags, priority, expiresAt)
	if err != nil {
		return "", fmt.Errorf("save memory failed: %w", err)
	}

	return fmt.Sprintf("记忆已保存 (ID: %d, 类别: %s)", id, category), nil
}

// mcpToolMemoryRecall 检索持久记忆
func mcpToolMemoryRecall(args map[string]interface{}) (string, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	category, _ := args["category"].(string)
	tokenID := 0
	if tid, ok := args["token_id"].(float64); ok {
		tokenID = int(tid)
	}

	token := &Token{ID: tokenID}
	memories, err := RecallMemories(token, "", category, limit)
	if err != nil {
		return "", fmt.Errorf("recall failed: %w", err)
	}

	if len(memories) == 0 {
		return "未找到相关记忆。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 记忆 (%d 条)\n\n", len(memories)))
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- **[%s]** %s (优先级: %d)\n", m.Category, m.Title, m.Priority))
		sb.WriteString(fmt.Sprintf("  %s\n", truncate(m.Content, 200)))
		if m.Tags != "" && m.Tags != "[]" {
			sb.WriteString(fmt.Sprintf("  标签: %s\n", m.Tags))
		}
		sb.WriteString(fmt.Sprintf("  创建于: %s\n\n", m.CreatedAt))
	}
	return sb.String(), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ---- CE 新增工具实现 ----

func mcpToolProviderHealth(args map[string]interface{}) (string, error) {
	rows, err := db.Query(`
		SELECT id, name, type, status, last_latency_ms
		FROM wr_providers WHERE enabled=1
		ORDER BY name`)
	if err != nil {
		return "", fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	result := "## Provider Health Status\n\n"
	result += "| ID | Name | Type | Status | Latency |\n|---|---|---|---|---|\n"
	for rows.Next() {
		var id int
		var name, ptype, status string
		var latency int
		if err := rows.Scan(&id, &name, &ptype, &status, &latency); err != nil {
			continue
		}
		latencyStr := "-"
		if latency > 0 {
			latencyStr = fmt.Sprintf("%dms", latency)
		}
		result += fmt.Sprintf("| %d | %s | %s | %s | %s |\n", id, name, ptype, status, latencyStr)
	}
	return result, nil
}

func mcpToolModelList(args map[string]interface{}) (string, error) {
	provider, _ := args["provider"].(string)
	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	var query string
	var queryArgs []interface{}

	if provider != "" {
		query = `SELECT DISTINCT model_name, COUNT(*) as cnt
			FROM wr_request_logs
			WHERE provider_name = ?
			GROUP BY model_name ORDER BY cnt DESC LIMIT ?`
		queryArgs = append(queryArgs, provider, limit)
	} else {
		query = `SELECT DISTINCT model_name, COUNT(*) as cnt
			FROM wr_request_logs
			GROUP BY model_name ORDER BY cnt DESC LIMIT ?`
		queryArgs = append(queryArgs, limit)
	}

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return "", fmt.Errorf("query models: %w", err)
	}
	defer rows.Close()

	result := "## Recent Models\n\n"
	result += "| Model | Request Count |\n|---|---|\n"
	for rows.Next() {
		var model string
		var cnt int
		if err := rows.Scan(&model, &cnt); err != nil {
			continue
		}
		result += fmt.Sprintf("| %s | %d |\n", model, cnt)
	}
	return result, nil
}

func mcpToolRequestStats(args map[string]interface{}) (string, error) {
	hours := 24
	if h, ok := args["hours"].(float64); ok && h > 0 {
		hours = int(h)
	}
	groupBy, _ := args["group_by"].(string)
	if groupBy == "" {
		groupBy = "model"
	}

	var selectCol string
	var label string
	switch groupBy {
	case "provider":
		selectCol = "provider_name"
		label = "Provider"
	case "token":
		selectCol = "token_name"
		label = "Token"
	default:
		selectCol = "model_name"
		label = "Model"
	}

	query := fmt.Sprintf(`
		SELECT %s,
			COUNT(*) as requests,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as errors,
			AVG(latency_ms) as avg_latency,
			SUM(cost_cents) as total_cost
		FROM wr_request_logs
		WHERE created_at >= datetime('now', '-%d hours')
		GROUP BY %s
		ORDER BY requests DESC
		LIMIT 20`, selectCol, hours, selectCol)

	rows, err := db.Query(query)
	if err != nil {
		return "", fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()

	result := fmt.Sprintf("## Request Stats (last %dh, by %s)\n\n", hours, label)
	result += fmt.Sprintf("| %s | Requests | Errors | Avg Latency | Cost (cents) |\n|---|---|---|---|---|\n", label)
	for rows.Next() {
		var name string
		var requests, errors int
		var avgLatency float64
		var totalCost int64
		if err := rows.Scan(&name, &requests, &errors, &avgLatency, &totalCost); err != nil {
			continue
		}
		latencyStr := "-"
		if avgLatency > 0 {
			latencyStr = fmt.Sprintf("%.1fms", avgLatency)
		}
		result += fmt.Sprintf("| %s | %d | %d | %s | %d |\n", name, requests, errors, latencyStr, totalCost)
	}
	return result, nil
}

func mcpToolRoutingStrategy(args map[string]interface{}) (string, error) {
	var strategy string
	err := db.QueryRow("SELECT value FROM wr_system_settings WHERE key='routing_strategy'").Scan(&strategy)
	if err != nil {
		// Default
		strategy = "smart"
	}

	result := "## Current Routing Strategy\n\n"
	result += fmt.Sprintf("**Strategy**: %s\n\n", strategy)
	result += "Available strategies: smart, priority, round_robin, least_latency, cost_first\n\n"
	result += "To change the strategy, go to System Settings → Routing Strategy."
	return result, nil
}
