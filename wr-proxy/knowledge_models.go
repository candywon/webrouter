package main

// 知识捕获模块 — 数据模型

// KnowledgeEntry 捕获条目，由 knowledgeWorker 异步处理
type KnowledgeEntry struct {
	RequestID  string
	TokenID    int
	TokenName  string
	Department string // 来自 Token.KnowledgeDepartment
	Model      string
	Endpoint   string // /v1/chat/completions 等
	Prompt     string // 脱敏后的完整 prompt
	Response   string // 脱敏后的完整 response
	TurnCount  int    // 对话轮数
	ClientIP   string
	Timestamp  string // UTC formatted
}

// KnowledgeDomain 业务域
type KnowledgeDomain struct {
	ID          int    `json:"id"`
	DomainCode  string `json:"domain_code"`
	DomainName  string `json:"domain_name"`
	Department  string `json:"department"`
	Status      string `json:"status"`       // active/pending/merged
	SampleCount int    `json:"sample_count"`
	Keywords    string `json:"auto_keywords"` // JSON array
	Description string `json:"description"`
	MergedInto  int    `json:"merged_into"`
	CreatedAt   string `json:"created_at"`
}

// 初始预设的 9 个业务域
var initialDomains = []struct {
	Code, Name string
}{
	{"legal", "法务合规"},
	{"finance", "财务审计"},
	{"hr", "人力资源"},
	{"admin", "行政办公"},
	{"sales", "销售商务"},
	{"service", "售后客服"},
	{"tech", "技术研发"},
	{"strategy", "战略决策"},
	{"marketing", "市场营销"},
}

// DomainRiskConfig 领域风险配置
type DomainRiskConfig struct {
	DomainCode           string
	RiskLevel            string  // high/medium/low
	MinVerification      string  // auto/verified
	MaxAgeDays           int     // 知识最大有效期
	DisclaimerTemplate   string  // 免责声明模板
	AllowFactual         bool    // 是否注入 factual 数据
	AllowAnalytical      bool    // 是否注入 analytical 结论
	AllowProcedural      bool    // 是否注入 procedural 流程
}

// CaptureStats 捕获统计计数器
type CaptureStats struct {
	TotalCaptured int64 `json:"total_captured"`
	TotalFiltered int64 `json:"total_filtered"`
	TotalSaved    int64 `json:"total_saved"`
	TodayCaptured int64 `json:"today_captured"`
	TodayFiltered int64 `json:"today_filtered"`
	TodaySaved    int64 `json:"today_saved"`
}

// KnowledgeVector 知识条目向量嵌入
type KnowledgeVector struct {
	ItemID    int     `json:"item_id"`
	Vector    string  `json:"vector"` // JSON array of float64
	Model     string  `json:"model"`
	Dimension int     `json:"dimension"`
	CreatedAt string  `json:"created_at"`
}
