package main

import (
	"fmt"
	"strings"
)

// InitKnowledgeTables 知识库相关表迁移（在 InitDB 的 migrate() 之后调用）
func InitKnowledgeTables() error {
	migrations := []string{
		// 原始对话暂存表
		`CREATE TABLE IF NOT EXISTS wr_knowledge_raw (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			token_id INTEGER NOT NULL,
			token_name TEXT DEFAULT '',
			model_name TEXT NOT NULL,
			prompt TEXT NOT NULL,
			response TEXT NOT NULL,
			turn_count INTEGER DEFAULT 1,
			client_ip TEXT DEFAULT '',
			status TEXT DEFAULT 'pending',     -- pending/processing/done/skipped
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			processed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kraw_status ON wr_knowledge_raw(status)`,
		`CREATE INDEX IF NOT EXISTS idx_kraw_token ON wr_knowledge_raw(token_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kraw_created ON wr_knowledge_raw(created_at)`,

		// 知识条目表（二期使用，一期先建表）
		`CREATE TABLE IF NOT EXISTS wr_knowledge_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,                -- factual/analytical/procedural
			title TEXT NOT NULL,
			summary TEXT DEFAULT '',
			domain_code TEXT DEFAULT '',
			department TEXT DEFAULT '',
			source_request_id TEXT NOT NULL,
			source_turn_index INTEGER DEFAULT 0,
			source_quote TEXT NOT NULL,
			source_char_start INTEGER DEFAULT 0,
			source_char_end INTEGER DEFAULT 0,
			data_points TEXT DEFAULT '',       -- JSON array (factual only)
			confidence REAL DEFAULT 0.0,
			verification TEXT DEFAULT 'auto',  -- auto/pending/verified/rejected
			verified_by INTEGER DEFAULT 0,
			verified_at DATETIME,
			token_id INTEGER NOT NULL,
			token_name TEXT DEFAULT '',
			model_name TEXT DEFAULT '',
			sensitivity TEXT DEFAULT 'medium', -- low/medium/high/restricted
			retention_until DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kitem_domain ON wr_knowledge_items(domain_code)`,
		`CREATE INDEX IF NOT EXISTS idx_kitem_dept ON wr_knowledge_items(department)`,
		`CREATE INDEX IF NOT EXISTS idx_kitem_type ON wr_knowledge_items(type)`,
		`CREATE INDEX IF NOT EXISTS idx_kitem_created ON wr_knowledge_items(created_at)`,

		// 业务域管理表
		`CREATE TABLE IF NOT EXISTS wr_knowledge_domains (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain_code TEXT UNIQUE NOT NULL,
			domain_name TEXT NOT NULL,
			department TEXT DEFAULT '',
			status TEXT DEFAULT 'pending',
			sample_count INTEGER DEFAULT 0,
			auto_keywords TEXT DEFAULT '',
			description TEXT DEFAULT '',
			merged_into INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			confirmed_at DATETIME,
			confirmed_by INTEGER
		)`,

		// 领域风险配置表（三期使用）
		`CREATE TABLE IF NOT EXISTS wr_knowledge_domain_risk (
			domain_code TEXT PRIMARY KEY,
			risk_level TEXT NOT NULL DEFAULT 'medium',
			min_verification TEXT NOT NULL DEFAULT 'auto',
			max_age_days INTEGER DEFAULT 180,
			disclaimer_template TEXT DEFAULT '',
			allow_factual_injection INTEGER DEFAULT 1,
			allow_analytical_injection INTEGER DEFAULT 1,
			allow_procedural_injection INTEGER DEFAULT 1
		)`,

		// 分析记录表（一期使用）
		`CREATE TABLE IF NOT EXISTS wr_knowledge_analyses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT UNIQUE NOT NULL,
			token_id INTEGER NOT NULL,
			token_name TEXT DEFAULT '',
			domains TEXT NOT NULL,
			departments TEXT DEFAULT '[]',
			types TEXT DEFAULT '[]',
			date_from TEXT DEFAULT '',
			date_to TEXT DEFAULT '',
			item_count INTEGER DEFAULT 0,
			analysis_type TEXT DEFAULT 'domain_overview',
			model_used TEXT DEFAULT '',
			strategy TEXT DEFAULT '',
			segment_count INTEGER DEFAULT 1,
			tokens_consumed INTEGER DEFAULT 0,
			cost REAL DEFAULT 0.0,
			duration_ms INTEGER DEFAULT 0,
			status TEXT DEFAULT 'processing',
			result TEXT DEFAULT '',
			error_message TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kanalyses_token ON wr_knowledge_analyses(token_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kanalyses_status ON wr_knowledge_analyses(status)`,
		`CREATE INDEX IF NOT EXISTS idx_kanalyses_created ON wr_knowledge_analyses(created_at)`,

		// 向量嵌入表（二期C Embedding使用）
		`CREATE TABLE IF NOT EXISTS wr_knowledge_vectors (
			item_id INTEGER PRIMARY KEY,
			vector TEXT NOT NULL,
			model TEXT DEFAULT 'text-embedding-v3',
			dimension INTEGER DEFAULT 1024,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (item_id) REFERENCES wr_knowledge_items(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kvec_model ON wr_knowledge_vectors(model)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("knowledge migration: %w\nSQL: %s", err, m)
			}
		}
	}

	// wr_tokens 扩展字段
	alterMigrations := []string{
		`ALTER TABLE wr_tokens ADD COLUMN knowledge_capture_enabled INTEGER DEFAULT 0`,
		`ALTER TABLE wr_tokens ADD COLUMN knowledge_department TEXT DEFAULT ''`,
		`ALTER TABLE wr_tokens ADD COLUMN rag_enabled INTEGER DEFAULT 0`,
		`ALTER TABLE wr_tokens ADD COLUMN rag_min_relevance REAL DEFAULT 0.7`,
		`ALTER TABLE wr_tokens ADD COLUMN rag_top_k INTEGER DEFAULT 3`,
		`ALTER TABLE wr_tokens ADD COLUMN system_prompt_knowledge TEXT DEFAULT ''`,
	}
	for _, m := range alterMigrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				LogWarn("knowledge alter migration: %v", err)
			}
		}
	}

	// 插入初始 8 个业务域
	if err := seedInitialDomains(); err != nil {
		return fmt.Errorf("seed initial domains: %w", err)
	}

	// 插入初始领域风险配置
	if err := seedInitialDomainRisk(); err != nil {
		LogWarn("seed domain risk config: %v", err)
	}

	LogInfo("Knowledge tables initialized")
	return nil
}

// seedInitialDomains 插入初始 8 个预设业务域
func seedInitialDomains() error {
	for _, d := range initialDomains {
		_, err := db.Exec(`
			INSERT OR IGNORE INTO wr_knowledge_domains (domain_code, domain_name, status, sample_count)
			VALUES (?, ?, 'active', 0)`,
			d.Code, d.Name,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// seedInitialDomainRisk 插入初始领域风险配置
func seedInitialDomainRisk() error {
	configs := []DomainRiskConfig{
		{"legal", "high", "verified", 90, "【注意】以下法务信息仅供参考，不构成法律意见。", true, false, true},
		{"finance", "high", "verified", 90, "【注意】以下财务数据仅供参考，正式报告以财务部官方数据为准。", true, true, false},
		{"hr", "medium", "auto", 180, "【提示】以下人事信息请以最新公司制度为准。", true, false, true},
		{"admin", "medium", "auto", 180, "【提示】以下行政信息请以最新公司制度为准。", true, false, true},
		{"strategy", "medium", "auto", 180, "【提示】以下战略信息供内部参考。", false, true, false},
		{"sales", "low", "auto", 365, "", true, true, true},
		{"marketing", "low", "auto", 365, "", true, true, true},
		{"service", "low", "auto", 365, "", true, true, true},
		{"tech", "low", "auto", 365, "", true, true, true},
	}

	for _, c := range configs {
		_, err := db.Exec(`
			INSERT OR IGNORE INTO wr_knowledge_domain_risk
			(domain_code, risk_level, min_verification, max_age_days, disclaimer_template,
			 allow_factual_injection, allow_analytical_injection, allow_procedural_injection)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			c.DomainCode, c.RiskLevel, c.MinVerification, c.MaxAgeDays, c.DisclaimerTemplate,
			btoi(c.AllowFactual), btoi(c.AllowAnalytical), btoi(c.AllowProcedural),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// saveKnowledgeRaw 将知识条目写入 raw 表
func saveKnowledgeRaw(entry KnowledgeEntry) error {
	_, err := db.Exec(`
		INSERT INTO wr_knowledge_raw
		(request_id, token_id, token_name, model_name, prompt, response, turn_count, client_ip, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		entry.RequestID, entry.TokenID, entry.TokenName, entry.Model,
		entry.Prompt, entry.Response, entry.TurnCount, entry.ClientIP,
		entry.Timestamp,
	)
	return err
}

// cleanupKnowledgeRaw 清理已处理的 raw 数据（超过 N 天）
func cleanupKnowledgeRaw(days int) error {
	result, err := db.Exec(`
		DELETE FROM wr_knowledge_raw
		WHERE status = 'done' AND processed_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		LogInfo("knowledge cleanup: deleted %d raw entries older than %d days", n, days)
	}
	return nil
}

// LoadKnowledgeDomains 加载所有业务域
func LoadKnowledgeDomains() ([]*KnowledgeDomain, error) {
	rows, err := db.Query(`
		SELECT id, domain_code, domain_name, department, status,
		       sample_count, COALESCE(auto_keywords, ''), COALESCE(description, ''),
		       merged_into, created_at
		FROM wr_knowledge_domains
		ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []*KnowledgeDomain
	for rows.Next() {
		d := &KnowledgeDomain{}
		if err := rows.Scan(&d.ID, &d.DomainCode, &d.DomainName, &d.Department,
			&d.Status, &d.SampleCount, &d.Keywords, &d.Description,
			&d.MergedInto, &d.CreatedAt); err != nil {
			LogWarn("scan knowledge domain: %v", err)
			continue
		}
		domains = append(domains, d)
	}
	return domains, nil
}

// LoadDomainRiskConfig 加载单个领域的风险配置
func LoadDomainRiskConfig(domainCode string) (*DomainRiskConfig, error) {
	c := &DomainRiskConfig{}
	var allowFactual, allowAnalytical, allowProcedural int
	err := db.QueryRow(`
		SELECT domain_code, risk_level, min_verification, max_age_days,
		       COALESCE(disclaimer_template, ''),
		       allow_factual_injection, allow_analytical_injection, allow_procedural_injection
		FROM wr_knowledge_domain_risk
		WHERE domain_code = ?`, domainCode,
	).Scan(&c.DomainCode, &c.RiskLevel, &c.MinVerification, &c.MaxAgeDays,
		&c.DisclaimerTemplate, &allowFactual, &allowAnalytical, &allowProcedural)
	if err != nil {
		return nil, err
	}
	c.AllowFactual = allowFactual == 1
	c.AllowAnalytical = allowAnalytical == 1
	c.AllowProcedural = allowProcedural == 1
	return c, nil
}

// btoi bool to int
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
