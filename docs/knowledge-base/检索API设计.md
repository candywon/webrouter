# 企业知识库 — 检索API与MCP集成设计

> 版本：v1.1 | 日期：2026-05-19

---

## 总览

知识库检索服务有两种接入方式：

| 接入方式 | 消费者 | 协议 | 典型场景 |
|----------|--------|------|----------|
| REST API | Flask管理后台、外部系统集成 | HTTP JSON | 人工检索、管理操作、BI看板 |
| MCP Server | AI智能体（Hermes Agent/Claude Code/OpenClaw等） | MCP StreamableHTTP | AI对话中自动调用知识库工具 |

两种方式共享同一套后端检索逻辑，区别只在传输协议和权限粒度。

---

## Part 1：REST API（人类使用）

### 1. 搜索接口

```
POST /api/knowledge/search
Content-Type: application/json
Authorization: Bearer sk-wr-xxxx
```

```json
{
  "query": "Q3毛利率下降的原因",
  "domain": "finance",
  "department": "财务部",
  "type": "factual",
  "top_k": 5,
  "sensitivity_max": "medium",
  "verification_min": "auto",
  "date_from": "2026-01-01",
  "date_to": "2026-05-14"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 搜索查询 |
| domain | string | 否 | 业务域代码，限定搜索范围 |
| department | string | 否 | 部门名称 |
| type | string | 否 | 知识类型：factual/analytical/procedural |
| top_k | int | 否 | 返回条数，默认5，最大20 |
| sensitivity_max | string | 否 | 最大敏感度：low/medium/high/restricted |
| verification_min | string | 否 | 最低验证级别：auto/pending/verified |
| date_from | string | 否 | 起始日期 |
| date_to | string | 否 | 截止日期 |

### 响应

```json
{
  "results": [
    {
      "id": 42,
      "title": "Q3毛利率数据",
      "summary": "Q3毛利率为18.7%，同比收窄3.2个百分点",
      "type": "factual",
      "domain": "finance",
      "domain_name": "财务审计",
      "department": "财务部",
      "data_points": [
        {
          "metric": "毛利率",
          "value": "18.7%",
          "period": "Q3",
          "unit": "百分比",
          "original_sentence": "Q3毛利率同比收窄3.2个百分点至18.7%",
          "certainty": "certain"
        }
      ],
      "source_quote": "原文完整片段...",
      "source_request_id": "req_abc123",
      "confidence": 0.92,
      "verification": "auto",
      "sensitivity": "medium",
      "relevance": 0.87,
      "created_at": "2026-05-14T10:30:00"
    }
  ],
  "total": 3,
  "query_time_ms": 45
}
```

---

### 2. 知识详情接口

```
GET /api/knowledge/items/:id
Authorization: Bearer sk-wr-xxxx
```

返回单条知识的完整信息，包括完整的 source_quote 和 data_points。

---

### 3. 知识审核接口

#### 获取待审核列表

```
GET /api/knowledge/reviews?status=pending&page=1&per_page=20
Authorization: Bearer sk-wr-xxxx
```

#### 审核操作

```
POST /api/knowledge/items/:id/review
Authorization: Bearer sk-wr-xxxx
```

```json
{
  "action": "approve",
  "note": "数据准确，已核实"
}
```

action 取值：
- `approve` → verification = 'verified'
- `reject` → verification = 'rejected'
- `edit` → 修改 data_points 后重新提交审核

---

### 4. 业务域管理接口

#### 列表

```
GET /api/knowledge/domains?status=active
Authorization: Bearer sk-wr-xxxx
```

#### 确认新域

```
POST /api/knowledge/domains/:code/confirm
Authorization: Bearer sk-wr-xxxx
```

#### 合并域

```
POST /api/knowledge/domains/merge
Authorization: Bearer sk-wr-xxxx
```

```json
{
  "source_code": "cross_border_ecommerce",
  "target_code": "sales",
  "new_name": "跨境电商销售"
}
```

---

### 5. 统计接口

```
GET /api/knowledge/stats
Authorization: Bearer sk-wr-xxxx
```

```json
{
  "total_items": 1523,
  "by_type": {
    "factual": 612,
    "analytical": 543,
    "procedural": 368
  },
  "by_domain": {
    "finance": 423,
    "sales": 312,
    "legal": 201
  },
  "by_verification": {
    "auto": 1201,
    "verified": 234,
    "pending": 88
  },
  "pending_reviews": 88,
  "pending_domains": 2,
  "recent_7d": 147,
  "capture_rate": 0.12
}
```

---

### 6. RAG注入接口（内部）

wr-proxy 内部调用，不对外暴露。

```
POST /internal/knowledge/inject
```

```json
{
  "query": "用户当前prompt的embedding",
  "token_id": 5,
  "top_k": 3,
  "min_relevance": 0.75
}
```

返回相关知识片段，由 wr-proxy 注入到 system prompt 中。

---

## Part 2：MCP Server（AI智能体使用）

### 设计理念

```
传统方式：AI → 自己拼HTTP请求 → 调REST API → 解析JSON → 人工处理
MCP方式：  AI → 自然语言 → MCP工具自动发现+调用 → 结果直接融入对话
```

MCP的核心价值：
1. **工具自动发现**：智能体连接MCP Server后自动看到所有可用工具，无需硬编码
2. **自然语言触发**：用户对话中提到企业知识，AI自动决定调用哪个工具
3. **权限自动绑定**：API Key关联Token，Token关联部门，部门决定可见知识范围
4. **零集成成本**：Hermes Agent/Claude Code/OpenClaw等只需在配置中加一行URL

### 架构

```
wr-proxy (Go)
  ├── 现有HTTP路由不变
  └── [新增] MCP Server端点
        │
        ├── POST /mcp          ← MCP StreamableHTTP 协议入口
        │
        ├── 工具注册 (tools/list)
        │     ├── knowledge_search          知识检索
        │     ├── knowledge_get_detail      知识详情
        │     ├── knowledge_list_domains    业务域列表
        │     ├── knowledge_get_stats       知识库统计
        │     ├── knowledge_analyze         全域/跨域深度分析
        │     └── knowledge_suggest         智能推荐（高级）
        │
        └── 认证
              └── 从MCP请求header的Authorization提取sk-wr-xxxx
                  → 查Token → 关联部门 → 自动注入权限过滤
```

### MCP端点

```
POST /mcp
Content-Type: application/json
Authorization: Bearer sk-wr-xxxx
```

wr-proxy 在现有 HTTP Server 上新增 `/mcp` 路由，实现 MCP StreamableHTTP 传输协议。

### 工具定义

#### Tool 1: knowledge_search — 知识检索

智能体在对话中需要查询企业知识时自动调用。

```json
{
  "name": "knowledge_search",
  "description": "搜索企业知识库。当你需要查找公司内部的事实数据、分析结论、流程规范、合同条款、财务数据、人事信息等企业知识时使用此工具。支持按业务域、部门、知识类型过滤。返回的结果中，factual类型的data_points包含从原文逐字复制的精确数据。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "搜索查询，描述你想查找的知识内容"
      },
      "domain": {
        "type": "string",
        "description": "业务域过滤：legal(法务合规)/finance(财务审计)/hr(人力资源)/admin(行政办公)/sales(销售商务)/service(售后客服)/tech(技术研发)/strategy(战略决策)",
        "enum": ["legal", "finance", "hr", "admin", "sales", "service", "tech", "strategy"]
      },
      "type": {
        "type": "string",
        "description": "知识类型过滤：factual(事实数据)/analytical(分析结论)/procedural(流程规范)",
        "enum": ["factual", "analytical", "procedural"]
      },
      "top_k": {
        "type": "integer",
        "description": "返回结果数量，默认5",
        "default": 5,
        "minimum": 1,
        "maximum": 20
      }
    },
    "required": ["query"]
  }
}
```

**调用示例**（智能体视角）：

```
用户："我们Q3的毛利率是多少？"
智能体自动调用：knowledge_search({ query: "Q3毛利率", domain: "finance", type: "factual" })
获得结果："Q3毛利率同比收窄3.2个百分点至18.7%"
回答："根据公司知识库数据，Q3毛利率为18.7%，同比收窄3.2个百分点。"
```

```
用户："合同审批流程是怎样的？"
智能体自动调用：knowledge_search({ query: "合同审批流程", type: "procedural" })
获得结果：包含完整流程步骤的知识条目
回答："根据公司知识库记录，合同审批流程如下：..."
```

---

#### Tool 2: knowledge_get_detail — 知识详情

获取单条知识的完整信息，包括原文锚点和数据验证状态。

```json
{
  "name": "knowledge_get_detail",
  "description": "获取知识条目的完整详情，包括原文锚点、数据点、验证状态。当你需要查看某条知识的完整上下文或验证数据准确性时使用。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {
        "type": "integer",
        "description": "知识条目ID"
      }
    },
    "required": ["id"]
  }
}
```

---

#### Tool 3: knowledge_list_domains — 业务域列表

查看知识库中有哪些业务域及其知识量。

```json
{
  "name": "knowledge_list_domains",
  "description": "列出企业知识库中的所有业务域及其知识条目数量。当你想了解知识库覆盖了哪些业务领域，或者想缩小搜索范围时使用。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "status": {
        "type": "string",
        "description": "过滤域状态：active(已确认)/pending(待确认)",
        "enum": ["active", "pending"]
      }
    }
  }
}
```

**返回示例**：

```json
{
  "domains": [
    { "code": "finance", "name": "财务审计", "item_count": 423, "status": "active" },
    { "code": "sales", "name": "销售商务", "item_count": 312, "status": "active" },
    { "code": "legal", "name": "法务合规", "item_count": 201, "status": "active" },
    { "code": "cross_border", "name": "跨境电商", "item_count": 23, "status": "pending" }
  ]
}
```

---

#### Tool 4: knowledge_get_stats — 知识库统计

获取知识库整体概览。

```json
{
  "name": "knowledge_get_stats",
  "description": "获取企业知识库的统计概览，包括知识总量、按类型/域分布、待审核数量等。用于了解知识库的整体状况。",
  "inputSchema": {
    "type": "object",
    "properties": {}
  }
}
```

---

#### Tool 5: knowledge_analyze — 全域/跨域深度分析

将指定业务域的全部知识灌入大模型做深度分析（交叉验证/趋势/风险/缺口）。详见[超长上下文分析设计](超长上下文分析设计.md)。

```json
{
  "name": "knowledge_analyze",
  "description": "对指定业务域的全部知识进行深度分析。支持单域分析（数据交叉验证、趋势发现、风险预警）和跨域关联分析（跨域矛盾、协同机会）。当用户要求对公司某个业务领域做全局分析、找数据矛盾、发现趋势时使用此工具。分析结果会引用具体知识条目ID作为证据。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "要分析的业务域代码列表，如 ['finance'] 或 ['sales', 'service']"
      },
      "analysis_focus": {
        "type": "string",
        "description": "分析重点：cross_validation(交叉验证)/trends(趋势)/risks(风险)/full(全面，默认)",
        "enum": ["cross_validation", "trends", "risks", "full"],
        "default": "full"
      },
      "time_range": {
        "type": "string",
        "description": "时间范围，如 'last_30d' / 'last_90d' / 'all'，默认 'all'"
      }
    },
    "required": ["domains"]
  }
}
```

---

#### Tool 6: knowledge_suggest — 智能推荐（高级，三期）

```json
{
  "name": "knowledge_suggest",
  "description": "根据当前对话上下文主动推荐相关知识。当你在回答涉及公司业务的问题时，可以调用此工具获取可能相关的企业内部知识来增强回答质量。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "context": {
        "type": "string",
        "description": "当前对话的上下文摘要或关键问题"
      },
      "top_k": {
        "type": "integer",
        "description": "推荐数量，默认3",
        "default": 3
      }
    },
    "required": ["context"]
  }
}
```

与 knowledge_search 的区别：
- `search`：精确检索，用户有明确查询意图
- `suggest`：模糊推荐，AI自己判断是否需要补充企业知识

---

### 权限模型

```
MCP请求 → Authorization: Bearer sk-wr-xxxx
                │
                ▼
          wr-proxy Token认证
                │
                ├── Token有效？
                │     └── 否 → 返回MCP错误
                │
                ├── Token.knowledge_capture_enabled？
                │     └── 否 → 所有工具返回空结果
                │
                └── 提取 Token.knowledge_department
                      │
                      ▼
                  权限过滤（自动注入到所有查询）
                    ├── knowledge_search → WHERE department = ? OR sensitivity = 'low'
                    ├── knowledge_get_detail → 同上
                    ├── knowledge_list_domains → 显示全量域（但不返回跨部门敏感数据）
                    └── knowledge_suggest → 同search过滤
```

**核心原则**：MCP工具的调用者（AI智能体）不知道权限的存在。权限过滤在底层自动执行，AI只看到自己部门可见的知识。

| 敏感度 | 本部门可见 | 其他部门可见 |
|--------|-----------|-------------|
| low | ✓ | ✓ |
| medium | ✓ | ✗ |
| high | ✓ | ✗ |
| restricted | ✓（仅管理员Token） | ✗ |

---

### 智能体配置方式

#### Hermes Agent

在 `~/.hermes/config.yaml` 中添加：

```yaml
mcp_servers:
  knowledge:
    url: "http://182.168.0.99:5051/mcp"
    headers:
      Authorization: "Bearer sk-wr-xxxxxxxxxxxxxxxxx"
    timeout: 30
```

启动后自动发现工具：`mcp_knowledge_knowledge_search`、`mcp_knowledge_knowledge_get_detail` 等。

#### Claude Code

在 `.claude/mcp.json` 或项目配置中添加：

```json
{
  "mcpServers": {
    "knowledge": {
      "url": "http://182.168.0.99:5051/mcp",
      "headers": {
        "Authorization": "Bearer sk-wr-xxxxxxxxxxxxxxxxx"
      }
    }
  }
}
```

#### OpenClaw

配置MCP Server连接：

```yaml
mcp_servers:
  - name: knowledge
    url: "http://182.168.0.99:5051/mcp"
    auth:
      type: bearer
      token: "sk-wr-xxxxxxxxxxxxxxxxx"
```

#### Cursor / Continue 等其他工具

所有支持MCP的AI工具统一配置格式，只需提供：
1. URL: `http://<webrouter-host>:5051/mcp`
2. Authorization header: `Bearer sk-wr-xxxx`

---

### MCP vs REST API 对照

| 维度 | REST API | MCP Server |
|------|----------|------------|
| 消费者 | 人类（通过Flask后台） | AI智能体（自动调用） |
| 触发方式 | 主动查询 | AI自动判断何时调用 |
| 权限 | Bearer Token + 部门过滤 | 同左，但AI不感知 |
| 返回格式 | 完整JSON | 精简文本（适合LLM上下文） |
| 工具发现 | 需查文档 | 自动发现（tools/list） |
| 适用场景 | 管理后台、BI看板、手动检索 | AI对话增强、自动知识注入 |
| 审核操作 | 支持（approve/reject） | 不支持（仅只读） |
| 域管理 | 支持（confirm/merge） | 不支持（仅只读） |

**MCP工具全部只读**——智能体只能检索知识，不能修改/删除/审核。管理操作通过REST API。

---

### wr-proxy 实现要点

#### MCP协议处理

```go
// RegisterMCPHandlers 注册MCP路由
func RegisterMCPHandlers(mux *http.ServeMux) {
    mux.HandleFunc("/mcp", handleMCP)
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
    // 1. 认证：从Authorization提取Token
    token, err := authenticateMCPRequest(r)
    if err != nil {
        writeMCPError(w, -32600, "Unauthorized")
        return
    }

    // 2. 解析MCP JSON-RPC请求
    var req MCPRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeMCPError(w, -32700, "Parse error")
        return
    }

    // 3. 路由到对应处理方法
    switch req.Method {
    case "initialize":
        handleMCPInitialize(w, req, token)
    case "tools/list":
        handleMCPToolsList(w, req, token)
    case "tools/call":
        handleMCPToolsCall(w, req, token)
    default:
        writeMCPError(w, -32601, "Method not found")
    }
}
```

#### tools/list 返回

```go
func handleMCPToolsList(w http.ResponseWriter, req MCPRequest, token *WRToken) {
    tools := []MCPTool{
        {
            Name:        "knowledge_search",
            Description: "搜索企业知识库...",
            InputSchema: searchInputSchema,
        },
        {
            Name:        "knowledge_get_detail",
            Description: "获取知识条目详情...",
            InputSchema: detailInputSchema,
        },
        // ... 其他工具
    }

    writeMCPResult(w, req.ID, map[string]interface{}{
        "tools": tools,
    })
}
```

#### tools/call 路由

```go
func handleMCPToolsCall(w http.ResponseWriter, req MCPRequest, token *WRToken) {
    params := req.Params.(map[string]interface{})
    toolName := params["name"].(string)
    arguments := params["arguments"].(map[string]interface{})

    // 自动注入部门权限
    if arguments == nil {
        arguments = make(map[string]interface{})
    }
    arguments["_department"] = token.KnowledgeDepartment
    arguments["_token_id"] = token.ID

    var result interface{}
    var err error

    switch toolName {
    case "knowledge_search":
        result, err = mcpKnowledgeSearch(arguments)
    case "knowledge_get_detail":
        result, err = mcpKnowledgeDetail(arguments)
    case "knowledge_list_domains":
        result, err = mcpKnowledgeListDomains(arguments)
    case "knowledge_get_stats":
        result, err = mcpKnowledgeStats(arguments)
    case "knowledge_analyze":
        result, err = mcpKnowledgeAnalyze(arguments)
    case "knowledge_suggest":
        result, err = mcpKnowledgeSuggest(arguments)
    default:
        writeMCPError(w, -32602, "Unknown tool: "+toolName)
        return
    }

    if err != nil {
        writeMCPError(w, -32603, err.Error())
        return
    }

    writeMCPResult(w, req.ID, map[string]interface{}{
        "content": []map[string]interface{}{
            {"type": "text", "text": resultToMCPText(toolName, result)},
        },
    })
}
```

#### MCP返回格式优化

MCP返回给AI的内容需要精简（不像REST API返回完整JSON），让AI高效理解：

```
知识检索结果（共3条）：

【1】Q3毛利率数据 | 类型:事实数据 | 域:财务审计 | 置信度:0.92
摘要: Q3毛利率为18.7%，同比收窄3.2个百分点
数据点:
  - 毛利率: 18.7% (Q3, 百分比) [来源原文: "Q3毛利率同比收窄3.2个百分点至18.7%"]
原文锚点: "根据财务部Q3分析报告，Q3毛利率同比收窄3.2个百分点至18.7%，主要受..."

【2】Q3营收分析 | 类型:分析结论 | 域:财务审计 | 置信度:0.88
摘要: Q3营收增长主要来自跨境电商业务贡献
原文锚点: "Q3营收同比增长15.3%达4.28亿元，其中跨境电商贡献..."

【3】毛利率计算规范 | 类型:流程规范 | 域:财务审计 | 置信度:0.95
摘要: 公司毛利率统一按(营收-成本)/营收计算，不含税费调整项
原文锚点: "根据公司财务制度，毛利率计算公式为..."
```

比原始JSON更适合LLM理解，同时保留了原文锚点以确保可验证。

---

### 安全考虑

| 风险 | 对策 |
|------|------|
| 智能体越权访问其他部门知识 | 部门权限自动注入，AI不感知也无法绕过 |
| MCP工具被恶意调用 | Token认证 + 知识库开关检查 |
| 敏感知识泄露给外部LLM | sensitivity=restricted的知识需要管理员Token才能检索 |
| AI过度调用MCP工具导致性能问题 | 每次MCP会话工具调用次数上限（默认20次/会话） |
| 原文过长导致Token消耗 | MCP返回精简格式，原文锚点截断到500字以内 |

---

### 一期/二期/三期分配

| 功能 | 一期 | 二期 | 三期 |
|------|------|------|------|
| MCP端点 + initialize + tools/list | ✓ | | |
| knowledge_search（关键词） | ✓ | | |
| knowledge_get_detail | ✓ | | |
| knowledge_list_domains | ✓ | | |
| knowledge_search（语义） | | ✓ | |
| knowledge_get_stats | | ✓ | |
| knowledge_suggest | | | ✓ |
| 部门权限自动注入 | ✓ | | |
| sensitivity过滤 | | ✓ | ✓ |
| 调用频率限制 | | ✓ | |
