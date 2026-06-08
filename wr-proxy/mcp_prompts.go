// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"net/http"
)

// ============================================================
// MCP Prompts — 提示词模板
// ============================================================

// MCPPromptDef 提示词模板定义
type MCPPromptDef struct {
	Name        string
	Description string
	Arguments   []MCPPromptArgument
	Template    string // 带 {{name}} 占位符的模板字符串
}

// MCPPromptArgument 参数定义
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// mcpPromptsRegistry 全局提示词注册表
var mcpPromptsRegistry []MCPPromptDef

// registerMCPPrompt 注册 MCP 提示词
func registerMCPPrompt(prompt MCPPromptDef) {
	mcpPromptsRegistry = append(mcpPromptsRegistry, prompt)
}

func handleMCPPromptsList(w http.ResponseWriter, id *int64) {
	prompts := make([]map[string]interface{}, 0, len(mcpPromptsRegistry))
	for _, p := range mcpPromptsRegistry {
		prompts = append(prompts, map[string]interface{}{
			"name":        p.Name,
			"description": p.Description,
			"arguments":   p.Arguments,
		})
	}
	writeMCPResponse(w, id, map[string]interface{}{
		"prompts": prompts,
	})
}

func handleMCPPromptsGet(w http.ResponseWriter, id *int64, params json.RawMessage) {
	var reqParams struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &reqParams); err != nil || reqParams.Name == "" {
		writeMCPError(w, http.StatusOK, id, ErrParams, "name is required")
		return
	}

	for _, p := range mcpPromptsRegistry {
		if p.Name == reqParams.Name {
			// 填充模板
			text := p.Template
			for k, v := range reqParams.Arguments {
				placeholder := "{{" + k + "}}"
				text = replacePlaceholder(text, placeholder, v)
			}

			writeMCPResponse(w, id, map[string]interface{}{
				"description": p.Description,
				"messages": []map[string]interface{}{
					{
						"role": "user",
						"content": map[string]interface{}{
							"type": "text",
							"text": text,
						},
					},
				},
			})
			return
		}
	}

	writeMCPError(w, http.StatusOK, id, ErrMethod, "Prompt not found: "+reqParams.Name)
}

func replacePlaceholder(s, placeholder, value string) string {
	result := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if i+len(placeholder) <= len(s) && s[i:i+len(placeholder)] == placeholder {
			result = append(result, []byte(value)...)
			i += len(placeholder)
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

func init() {
	registerMCPPrompt(MCPPromptDef{
		Name:        "knowledge_search",
		Description: "Search the enterprise knowledge base for a given query, optionally scoped by domain",
		Arguments: []MCPPromptArgument{
			{Name: "query", Description: "Search keyword", Required: true},
			{Name: "domain", Description: "Domain code to scope search (e.g. legal, finance, tech)", Required: false},
		},
		Template: `Please search the enterprise knowledge base for information about "{{query}}"` +
			`{{domain}}` +
			`. Return the most relevant results with source citations.`,
	})
	registerMCPPrompt(MCPPromptDef{
		Name:        "system_status",
		Description: "Show a summary of the current system status including providers, usage, and alerts",
		Arguments:   nil,
		Template:    "Please provide a brief summary of the current system status, including active providers, recent request volume, error rate, and any active alerts.",
	})
}
