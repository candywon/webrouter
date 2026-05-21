# 个人知识库 — 独立项目Python方案

> 版本：v1.0 | 日期：2026-05-19

---

## 1. 变化总结

| 维度 | 原方案(wr-proxy内) | 新方案(独立项目) |
|------|-------------------|-----------------|
| 语言 | Go | Python |
| 部署 | 服务器(企业) | 个人电脑(本地) |
| 并发 | 多用户并发 | 单用户，无并发 |
| 数据归属 | 企业 | 个人 |
| 法律定性 | 组织处理个人信息 | 个人使用工具 |
| 架构 | 嵌入API网关 | 独立应用 |
| 存储 | SQLite共享DB | SQLite本地DB |
| 前端 | Flask管理后台 | 本地Web UI |

---

## 2. 架构变化

### 原架构（嵌入wr-proxy）

```
wr-proxy (Go, :5051)
  ├── API代理（核心功能）
  ├── 智能重试/降级/冷却
  ├── 脱敏引擎
  ├── 知识捕获（嵌入代理流程）
  ├── LLM提取
  ├── 向量检索
  ├── RAG注入
  ├── MCP Server
  └── 记忆/压缩/分析

Flask (Python, :5050)
  └── 管理后台

共享SQLite DB
```

### 新架构（独立项目）

```
personal-kb (Python, 本地运行)
  ├── 本地HTTP Server (FastAPI, :8900)
  │     ├── /mcp              MCP Server端点
  │     ├── /api/knowledge/*  知识CRUD/检索/分析
  │     ├── /api/memory/*     记忆CRUD
  │     └── /                 Web UI (静态页面)
  │
  ├── 知识捕获层
  │     ├── 文件导入（txt/md/json/csv）
  │     ├── 剪贴板捕获
  │     ├── 浏览器插件推送
  │     └── API推送（智能体/Hermes/Claude Code等调用）
  │
  ├── 知识处理层
  │     ├── 信号筛选
  │     ├── LLM提取（调用云端API）
  │     ├── 数据点验证
  │     └── Embedding向量化
  │
  ├── 知识服务层
  │     ├── 向量检索
  │     ├── 关键词检索
  │     ├── RAG自动增强
  │     ├── 全域分析
  │     └── 对话摘要压缩
  │
  ├── MCP Server
  │     └── 智能体工具调用
  │
  └── 本地SQLite DB + 向量文件
```

### 关键区别

```
原方案：知识捕获嵌入在API代理流程中
  → 对话经过wr-proxy时自动捕获
  → 依赖代理网关的中间人位置

新方案：知识捕获是独立的数据入口
  → 不依赖代理网关
  → 用户主动推送/导入/剪贴板/浏览器插件
  → 或者：与wr-proxy配合，wr-proxy推送数据到本地知识库API
```

---

## 3. Go → Python 逐模块调整

### 3.1 不需要移植的模块

| 原Go模块 | 为什么不需要 | 说明 |
|----------|------------|------|
| knowledge_capture.go | 捕获逻辑嵌入代理流程 | 改为API推送模式，用户/智能体主动推送 |
| handlers.go(集成) | 依赖wr-proxy代理框架 | 独立项目有自己的路由 |
| knowledge_system_prompt.go | 依赖wr-proxy的代理注入 | 改为MCP工具，智能体自己决定注入 |
| knowledge_stats.go(部分) | 企业级统计看板 | 简化为个人统计 |
| knowledge_db.go(部分) | 共享DB相关 | 改为本地独立DB |
| rag.go(注入逻辑) | 依赖wr-proxy的代理注入 | 改为MCP工具返回上下文 |

### 3.2 需要移植并调整的模块

| 原Go模块 | 行数 | Python调整 | 新文件 |
|----------|------|-----------|--------|
| knowledge_filter.go | 137 | 信号筛选规则，直接翻译 | filters.py (~120行) |
| knowledge_extract.go | 508 | LLM提取+验证，httpx替代原生HTTP | extractor.py (~400行) |
| knowledge_analyze.go | 376 | 全域分析，逻辑不变 | analyzer.py (~300行) |
| embedding.go | 248 | DashScope Embedding，httpx调用 | embedder.py (~150行) |
| vector_search.go | 200 | 向量检索，numpy计算 | search.py (~150行) |
| memory.go | 285 | 持久记忆，逻辑不变 | memory.py (~200行) |
| compress.go | 221 | 对话压缩，逻辑不变 | compressor.py (~180行) |
| rag_filter.go | 148 | RAG质量控制，逻辑不变 | rag_filter.py (~120行) |
| rag_feedback.go | 203 | RAG反馈，逻辑不变 | rag_feedback.py (~150行) |
| mcp.go | 233 | MCP协议框架，FastAPI实现 | mcp_server.py (~200行) |
| mcp_tools.go | 511 | MCP工具，FastAPI实现 | mcp_tools.py (~350行) |
| knowledge_db.go | 298 | SQLite，SQLAlchemy ORM | database.py (~250行) |
| knowledge_models.go | 78 | 数据模型，Pydantic | models.py (~100行) |
| knowledge_stats.go | 401 | 简化统计 | stats.py (~150行) |

**预估Python总量**：~2720行（vs Go 4993行，Python更简洁约45%）

### 3.3 Python独有的新增模块

| 新模块 | 说明 | 预估行数 |
|--------|------|---------|
| app.py | FastAPI应用入口+路由 | ~100行 |
| capture.py | 数据入口(API推送+文件导入+剪贴板) | ~200行 |
| config.py | 配置管理(YAML) | ~80行 |
| web_ui/ | 前端静态页面(HTML/CSS/JS) | 复用现有knowledge.js改造 |
| cli.py | 命令行接口(启动/导入/搜索) | ~150行 |

---

## 4. 技术选型

### 4.1 Web框架：FastAPI（替代Flask）

```
选择FastAPI而不是Flask的原因：
  1. 原生async → LLM调用/Embedding等IO密集型更高效
  2. 自动生成OpenAPI文档 → MCP/智能体集成更方便
  3. Pydantic模型 → 请求/响应自动校验
  4. 原生SSE支持 → MCP StreamableHTTP需要
  5. 性能更好 → 虽然单用户不讲究，但顺手

保留Flask兼容：
  如果用户已有Flask环境，也可提供Flask版本
  但FastAPI是主推
```

### 4.2 向量存储：numpy + SQLite（无额外依赖）

```
不选ChromaDB/Milvus/Qdrant的原因：
  1. 单用户，知识量通常<1万条
  2. numpy余弦相似度足够快
  3. 减少依赖，pip install即可运行
  4. 向量存为SQLite BLOB，和现有方案一致

如果未来需要更大规模：
  可选chromadb作为可选依赖
  pip install personal-kb[vector] 自动安装
```

### 4.3 LLM调用：httpx

```
不选openai SDK的原因：
  1. 支持多厂商（DashScope/DeepSeek/OpenAI/本地Ollama）
  2. 不绑定特定API格式
  3. httpx更轻量，async原生

统一接口：
  POST {base_url}/chat/completions
  POST {base_url}/embeddings
  → 兼容OpenAI格式的任何API
```

### 4.4 数据库：SQLite + SQLAlchemy

```
和原方案一致，但改为独立本地文件：
  ~/.personal-kb/data/knowledge.db

SQLAlchemy ORM替代原生SQL：
  → 类型安全
  → 迁移管理（Alembic）
  → Pydantic模型自动转换
```

### 4.5 前端：复用knowledge.js，轻量改造

```
原方案：Flask服务端渲染 + knowledge.js(849行)
新方案：FastAPI静态文件 + 改造后的knowledge.js

改造点：
  1. API路径从 /api/knowledge/xxx 改为 /api/knowledge/xxx（不变）
  2. 去掉"审核队列"/"业务域管理"等企业功能
  3. 新增"数据导入"/"剪贴板捕获"入口
  4. 去掉多用户相关UI（Token管理/部门选择）
  5. 整体从"管理后台"风格改为"个人助手"风格
```

---

## 5. 数据入口重新设计

原方案最大的依赖是wr-proxy的中间人位置。独立后需要新的数据入口：

### 5.1 API推送（核心入口）

```python
# 智能体/Hermes/Claude Code主动推送对话内容
POST /api/capture/push
{
    "content": "对话内容或文档内容",
    "source": "hermes|claude_code|openclaw|manual",
    "metadata": {
        "model": "qwen3-coder-flash",
        "session_id": "abc123"
    }
}
```

**与智能体的配合方式**：

```yaml
# Hermes Agent config.yaml
mcp_servers:
  knowledge:
    url: "http://localhost:8900/mcp"
    # MCP工具自动发现，knowledge_search / knowledge_push 等
```

Hermes调用MCP工具 `knowledge_push` 把对话内容推送到本地知识库。

### 5.2 文件导入

```python
# 支持格式
- .txt    纯文本
- .md     Markdown
- .json   JSON数组
- .csv    CSV表格
- .pdf    PDF（需PyMuPDF）

# 导入方式
POST /api/capture/import
Content-Type: multipart/form-data

# 或命令行
python -m personal_kb import ./documents/*.md
python -m personal_kb import ./data.xlsx
```

### 5.3 剪贴板捕获

```python
# 监听剪贴板，用户复制内容时自动弹出"保存到知识库"
# 使用pyperclip库

# 命令行启动
python -m personal_kb clipboard-watch
```

### 5.4 wr-proxy联动（可选）

```
如果用户同时使用wr-proxy：

wr-proxy handleProxy 增加一个推送配置：
  --knowledge-push-url=http://localhost:8900/api/capture/push

对话经过wr-proxy时，异步推送到本地知识库
→ 保留了原方案的自动捕获能力
→ 但数据存在用户自己电脑上，不属于企业
```

---

## 6. MCP工具重新设计

原方案7个工具，调整为个人版6个：

| 工具 | 原方案 | 个人版调整 |
|------|--------|-----------|
| knowledge_search | 部门级过滤 | 个人全部知识，无过滤 |
| knowledge_get_detail | 不变 | 不变 |
| knowledge_list_domains | 企业域管理 | 简化为标签列表 |
| knowledge_analyze | 不变 | 不变 |
| knowledge_get_stats | 企业统计 | 个人统计 |
| knowledge_push | **新增** | 智能体主动推送对话内容 |
| knowledge_suggest | 三期 | 二期提前（个人版更实用） |
| ~~memory_save~~ | 独立记忆 | **合并到knowledge_push** |
| ~~memory_recall~~ | 独立记忆 | **合并到knowledge_search** |

**核心变化**：记忆和知识不再分开。个人视角下"知识"和"记忆"是一回事——都是"我记下来的东西"。

---

## 7. 项目结构

```
personal-kb/
├── pyproject.toml              # 项目配置+依赖
├── README.md
├── LICENSE                     # Apache 2.0
├── NOTICE                      # 合规提示
├── USE_TERMS.md                # 使用条款
│
├── personal_kb/                # 主包
│   ├── __init__.py
│   ├── app.py                  # FastAPI入口
│   ├── config.py               # 配置管理
│   ├── database.py             # SQLite + SQLAlchemy
│   ├── models.py               # Pydantic数据模型
│   │
│   ├── capture/                # 数据入口
│   │   ├── __init__.py
│   │   ├── push.py             # API推送
│   │   ├── file_import.py      # 文件导入
│   │   └── clipboard.py        # 剪贴板监听
│   │
│   ├── processing/             # 知识处理
│   │   ├── __init__.py
│   │   ├── filters.py          # 信号筛选
│   │   ├── extractor.py        # LLM提取+验证
│   │   ├── embedder.py         # Embedding向量化
│   │   └── analyzer.py         # 全域分析
│   │
│   ├── services/               # 知识服务
│   │   ├── __init__.py
│   │   ├── search.py           # 向量+关键词检索
│   │   ├── rag.py              # RAG增强
│   │   ├── rag_filter.py       # RAG质量控制
│   │   ├── rag_feedback.py     # RAG反馈闭环
│   │   ├── memory.py           # 持久记忆
│   │   └── compressor.py       # 对话压缩
│   │
│   ├── mcp/                    # MCP Server
│   │   ├── __init__.py
│   │   ├── server.py           # MCP协议框架
│   │   └── tools.py            # MCP工具定义
│   │
│   ├── api/                    # REST API
│   │   ├── __init__.py
│   │   ├── knowledge.py        # 知识CRUD
│   │   ├── capture.py          # 捕获入口
│   │   ├── analyze.py          # 分析入口
│   │   └── memory.py           # 记忆CRUD
│   │
│   ├── web/                    # 前端
│   │   ├── index.html
│   │   ├── css/
│   │   └── js/
│   │       └── knowledge.js    # 改造自原knowledge.js
│   │
│   └── cli.py                  # 命令行接口
│
├── data/                       # 数据目录(gitignore)
│   ├── knowledge.db            # SQLite
│   └── config.yaml             # 用户配置
│
└── tests/
    ├── test_filters.py
    ├── test_extractor.py
    ├── test_search.py
    └── test_mcp.py
```

---

## 8. 依赖清单

```toml
# pyproject.toml
[project]
name = "personal-kb"
version = "0.1.0"
requires-python = ">=3.10"

dependencies = [
    # Web框架
    "fastapi>=0.100",
    "uvicorn>=0.20",
    
    # 数据库
    "sqlalchemy>=2.0",
    "alembic>=1.12",
    
    # HTTP客户端（LLM/Embedding调用）
    "httpx>=0.25",
    
    # 向量计算
    "numpy>=1.24",
    
    # 数据校验
    "pydantic>=2.0",
    
    # 配置
    "pyyaml>=6.0",
    
    # CLI
    "click>=8.0",
]

[project.optional-dependencies]
# 文件导入
import = [
    "pymupdf>=1.23",       # PDF
    "openpyxl>=3.1",       # Excel
    "python-docx>=0.8",    # Word
]

# 高级向量检索（大规模时）
vector = [
    "chromadb>=0.4",
]

# 剪贴板监听
clipboard = [
    "pyperclip>=1.8",
]

# 全部可选
all = ["personal-kb[import,vector,clipboard]"]
```

**核心依赖只有8个**，pip install个人知识库即可运行。

---

## 9. 与wr-proxy的关系

```
方案A：完全独立
  personal-kb 不依赖 wr-proxy
  数据入口：API推送 + 文件导入 + 剪贴板
  适合：不使用wr-proxy的个人用户

方案B：联动模式
  wr-proxy 作为API网关运行
  wr-proxy 增加配置：--knowledge-push-url=http://localhost:8900
  对话经过wr-proxy时自动推送到本地personal-kb
  适合：同时使用wr-proxy的用户，保留自动捕获能力

两种模式互不冲突，personal-kb本身不依赖wr-proxy。
```

---

## 10. 配置文件

```yaml
# ~/.personal-kb/config.yaml

# 服务器
server:
  host: "127.0.0.1"    # 仅本地访问
  port: 8900

# LLM配置
llm:
  # 提取用（低成本）
  extract:
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: "sk-xxxx"
    model: "qwen3-coder-flash"
  
  # 分析用（高质量）
  analyze:
    base_url: "https://api.deepseek.com/v1"
    api_key: "sk-xxxx"
    model: "deepseek-chat"
  
  # 压缩用
  compress:
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: "sk-xxxx"
    model: "qwen3-coder-flash"

# Embedding
embedding:
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  api_key: "sk-xxxx"
  model: "text-embedding-v3"

# 知识捕获
capture:
  auto_extract: true         # 捕获后自动LLM提取
  extract_batch_size: 20     # 批量提取大小
  extract_interval: 300      # 提取间隔（秒）

# RAG
rag:
  enabled: true
  min_relevance: 0.7
  top_k: 3

# 数据目录
data_dir: "~/.personal-kb/data"
```

---

## 11. 启动方式

```bash
# 安装
pip install personal-kb

# 初始化
personal-kb init
# → 创建 ~/.personal-kb/ 目录和配置文件

# 编辑配置（填入API Key）
vim ~/.personal-kb/config.yaml

# 启动服务
personal-kb serve
# → FastAPI on http://127.0.0.1:8900
# → Web UI on http://127.0.0.1:8900/

# 导入文件
personal-kb import ./documents/*.md
personal-kb import ./report.pdf

# 命令行搜索
personal-kb search "Q3毛利率"

# MCP连接（配置到智能体）
# Hermes: url = "http://localhost:8900/mcp"
# Claude Code: url = "http://localhost:8900/mcp"
```

---

## 12. 合规影响

| 改造项 | 企业知识库(原方案) | 个人知识库(新方案) |
|--------|-------------------|-------------------|
| 知情同意 | P0 必须 | **不需要** |
| 删除入口 | P0 必须 | **自带**（本地DB，自己删） |
| raw表最小化 | P0 必须 | **不需要**（自己的数据自己负责） |
| 审计日志 | P1 必须 | **不需要** |
| 敏感信息检测 | P1 必须 | **可选**（共享时提示） |
| 隐私影响评估 | P2 必须 | **不需要** |
| **合规总工作量** | **5.5天** | **0天** |

**合规工作量从5.5天降到0天。** 个人使用工具不涉及任何PIPL义务。

开源开发方风险：和Hermes/Obsidian/Notion同等——提供个人笔记工具，用户记什么是用户的事。

---

## 13. 开发工作量估算

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| **Week 1** | FastAPI框架+数据库+数据模型+配置 | 3天 |
| | 数据入口(API推送+文件导入) | 2天 |
| **Week 2** | LLM提取+验证+信号筛选 | 3天 |
| | Embedding+向量检索 | 2天 |
| **Week 3** | MCP Server+6个工具 | 2天 |
| | 全域分析 | 2天 |
| | 记忆+压缩 | 1天 |
| **Week 4** | Web UI(改造knowledge.js) | 3天 |
| | CLI接口 | 1天 |
| | 测试+文档+打包 | 1天 |

**总计4周**（vs 原方案8周，减半）

原因：
1. Python比Go简洁，代码量减少约45%
2. 去掉企业级功能（审核/域管理/部门权限/共享）
3. 去掉合规改造（0天 vs 5.5天）
4. 去掉并发处理（单用户无锁/无channel）
5. 不需要嵌入代理框架（独立项目）
