// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"net/http"
)

// ============================================================
// MCP 协议框架 — Model Context Protocol (JSON-RPC 2.0)
// https://spec.modelcontextprotocol.io/specification/2024-11-05/
// ============================================================

// MCPRequest JSON-RPC 2.0 请求
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse JSON-RPC 2.0 响应
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError JSON-RPC 2.0 错误
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// MCPCapabilities 服务端能力声明
type MCPCapabilities struct {
	Tools *struct {
		ListChanged bool `json:"listChanged"`
	} `json:"tools,omitempty"`
	Resources *struct{} `json:"resources,omitempty"`
	Prompts   *struct{} `json:"prompts,omitempty"`
}

// MCPInitParams initialize 请求参数
type MCPInitParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]string      `json:"clientInfo"`
}

// MCPInitResult initialize 响应
type MCPInitResult struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    MCPCapabilities   `json:"capabilities"`
	ServerInfo      map[string]string `json:"serverInfo"`
}

const (
	MCPProtocolVersion = "2024-11-05"
	MCPServerName      = "wr-proxy-knowledge"
	MCPServerVersion   = "1.0.0"

	// JSON-RPC 错误码
	ErrParse    = -32700
	ErrInvalid  = -32600
	ErrMethod   = -32601
	ErrParams   = -32602
	ErrInternal = -32603
)

// handleMCP MCP 协议入口
func handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMCPError(w, http.StatusOK, nil, ErrInvalid, "Only POST allowed")
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMCPError(w, http.StatusOK, nil, ErrParse, "Invalid JSON: "+err.Error())
		return
	}

	// 路由分发
	switch req.Method {
	case "initialize":
		handleMCPInitialize(w, req.ID, req.Params)
	case "notifications/initialized":
		// 客户端通知，无需回复
		writeMCPResponse(w, req.ID, map[string]interface{}{})
	case "tools/list":
		handleMCPToolsList(w, req.ID)
	case "tools/call":
		handleMCPCall(w, req.ID, req.Params)
	case "resources/list":
		handleMCPResourcesList(w, req.ID)
	case "resources/read":
		handleMCPResourceRead(w, req.ID, req.Params)
	case "prompts/list":
		handleMCPPromptsList(w, req.ID)
	case "prompts/get":
		handleMCPPromptsGet(w, req.ID, req.Params)
	default:
		writeMCPError(w, http.StatusOK, req.ID, ErrMethod, "Method not found: "+req.Method)
	}
}

func handleMCPInitialize(w http.ResponseWriter, id *int64, params json.RawMessage) {
	var initParams MCPInitParams
	if params != nil {
		json.Unmarshal(params, &initParams)
	}

	result := MCPInitResult{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities: MCPCapabilities{
			Tools: &struct {
				ListChanged bool `json:"listChanged"`
			}{ListChanged: true},
			Resources: &struct{}{},
			Prompts:   &struct{}{},
		},
		ServerInfo: map[string]string{
			"name":    MCPServerName,
			"version": MCPServerVersion,
		},
	}

	writeMCPResponse(w, id, result)
}

func handleMCPToolsList(w http.ResponseWriter, id *int64) {
	tools := getMCPTools()
	writeMCPResponse(w, id, map[string]interface{}{
		"tools": tools,
	})
}

func handleMCPCall(w http.ResponseWriter, id *int64, params json.RawMessage) {
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &callParams); err != nil {
		writeMCPError(w, http.StatusOK, id, ErrParams, "Invalid arguments")
		return
	}

	if callParams.Name == "" {
		writeMCPError(w, http.StatusOK, id, ErrParams, "tool name is required")
		return
	}

	tool := findMCPTool(callParams.Name)
	if tool == nil {
		writeMCPError(w, http.StatusOK, id, ErrMethod, "Unknown tool: "+callParams.Name)
		return
	}

	result, err := tool(callParams.Arguments)
	if err != nil {
		writeMCPError(w, http.StatusOK, id, ErrInternal, err.Error())
		return
	}

	LogAudit("mcp_tool_call", "mcp", callParams.Name, 0, nil, "")

	writeMCPResponse(w, id, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": result,
			},
		},
	})
}

func writeMCPResponse(w http.ResponseWriter, id *int64, result interface{}) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeMCPError(w http.ResponseWriter, status int, id *int64, code int, message string) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// MCPToolFunc 工具函数签名
type MCPToolFunc func(args map[string]interface{}) (string, error)

// getMCPTools 返回所有已注册的工具列表
func getMCPTools() []map[string]interface{} {
	tools := []map[string]interface{}{}
	for _, t := range mcpToolsRegistry {
		tools = append(tools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return tools
}

// findMCPTool 按名称查找工具
func findMCPTool(name string) MCPToolFunc {
	for _, t := range mcpToolsRegistry {
		if t.Name == name {
			return t.Handler
		}
	}
	return nil
}

// MCPToolDef 工具定义
type MCPToolDef struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     MCPToolFunc
}

// mcpToolsRegistry 全局工具注册表
var mcpToolsRegistry []MCPToolDef

// registerMCPTool 注册 MCP 工具（在 init 中调用）
func registerMCPTool(tool MCPToolDef) {
	mcpToolsRegistry = append(mcpToolsRegistry, tool)
}

func init() {
	registerKnowledgeMCPTools()
}
