// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ============================================================
// MCP Resources — 资源列表与读取
// ============================================================

// MCPResourceDef 资源定义
type MCPResourceDef struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	Handler     func(uri string) (string, string, error) // 返回 (content, mimeType, error)
}

// mcpResourcesRegistry 全局资源注册表
var mcpResourcesRegistry []MCPResourceDef

// registerMCPResource 注册 MCP 资源
func registerMCPResource(res MCPResourceDef) {
	mcpResourcesRegistry = append(mcpResourcesRegistry, res)
}

func handleMCPResourcesList(w http.ResponseWriter, id *int64) {
	resources := make([]map[string]interface{}, 0, len(mcpResourcesRegistry))
	for _, r := range mcpResourcesRegistry {
		resources = append(resources, map[string]interface{}{
			"uri":         r.URI,
			"name":        r.Name,
			"description": r.Description,
			"mimeType":    r.MimeType,
		})
	}
	writeMCPResponse(w, id, map[string]interface{}{
		"resources": resources,
	})
}

func handleMCPResourceRead(w http.ResponseWriter, id *int64, params json.RawMessage) {
	var reqParams struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &reqParams); err != nil || reqParams.URI == "" {
		writeMCPError(w, http.StatusOK, id, ErrParams, "uri is required")
		return
	}

	for _, r := range mcpResourcesRegistry {
		if r.URI == reqParams.URI {
			content, mimeType, err := r.Handler(reqParams.URI)
			if err != nil {
				writeMCPError(w, http.StatusOK, id, ErrInternal, err.Error())
				return
			}
			writeMCPResponse(w, id, map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"uri":      reqParams.URI,
						"mimeType": mimeType,
						"text":     content,
					},
				},
			})
			return
		}
	}

	writeMCPError(w, http.StatusOK, id, ErrMethod, "Resource not found: "+reqParams.URI)
}

// --- 内置资源实现 ---

func resourceKnowledgeStats(uri string) (string, string, error) {
	var total int
	db.QueryRow("SELECT COUNT(*) FROM wr_knowledge_items").Scan(&total)

	var byType string
	rows, err := db.Query("SELECT type, COUNT(*) FROM wr_knowledge_items GROUP BY type")
	if err == nil {
		defer rows.Close()
		typeMap := make(map[string]int)
		for rows.Next() {
			var t string
			var c int
			rows.Scan(&t, &c)
			typeMap[t] = c
		}
		b, _ := json.Marshal(typeMap)
		byType = string(b)
	}

	result := fmt.Sprintf("Total knowledge items: %d\nBy type: %s", total, byType)
	return result, "text/plain", nil
}

func resourceKnowledgeDomains(uri string) (string, string, error) {
	rows, err := db.Query("SELECT domain_code, domain_name, department, status FROM wr_knowledge_domains ORDER BY domain_code")
	if err != nil {
		return "", "", fmt.Errorf("query domains: %w", err)
	}
	defer rows.Close()

	var result string
	for rows.Next() {
		var code, name, dept, status string
		rows.Scan(&code, &name, &dept, &status)
		result += fmt.Sprintf("- %s (%s) — %s [%s]\n", code, name, dept, status)
	}
	if result == "" {
		result = "(no domains)"
	}
	return result, "text/plain", nil
}

func resourceProviderHealth(uri string) (string, string, error) {
	rows, err := db.Query("SELECT id, name, type, status, last_latency_ms FROM wr_providers WHERE enabled=1 ORDER BY name")
	if err != nil {
		return "", "", fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var result string
	for rows.Next() {
		var id int
		var name, ptype, status string
		var latency int
		rows.Scan(&id, &name, &ptype, &status, &latency)
		result += fmt.Sprintf("- %s [%s] type=%s latency=%dms\n", name, status, ptype, latency)
	}
	if result == "" {
		result = "(no active providers)"
	}
	return result, "text/plain", nil
}

func resourceSystemConfig(uri string) (string, string, error) {
	rows, err := db.Query("SELECT key, value FROM wr_system_settings ORDER BY key")
	if err != nil {
		return "", "", fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()

	var result string
	for rows.Next() {
		var key, value string
		rows.Scan(&key, &value)
		result += fmt.Sprintf("- %s = %s\n", key, value)
	}
	if result == "" {
		result = "(no settings)"
	}
	return result, "text/plain", nil
}

func init() {
	registerMCPResource(MCPResourceDef{
		URI:         "knowledge://stats",
		Name:        "Knowledge Base Statistics",
		Description: "Total knowledge item count and breakdown by type",
		MimeType:    "text/plain",
		Handler:     resourceKnowledgeStats,
	})
	registerMCPResource(MCPResourceDef{
		URI:         "knowledge://domains",
		Name:        "Knowledge Domains",
		Description: "All registered business domains with status",
		MimeType:    "text/plain",
		Handler:     resourceKnowledgeDomains,
	})
	registerMCPResource(MCPResourceDef{
		URI:         "health://providers",
		Name:        "Provider Health Status",
		Description: "Health status of all active providers",
		MimeType:    "text/plain",
		Handler:     resourceProviderHealth,
	})
	registerMCPResource(MCPResourceDef{
		URI:         "system://config",
		Name:        "System Configuration",
		Description: "Current system settings key-value pairs",
		MimeType:    "text/plain",
		Handler:     resourceSystemConfig,
	})
}
