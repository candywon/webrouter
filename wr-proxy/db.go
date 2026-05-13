package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// InitDB 初始化数据库连接和表结构
func InitDB(dbPath string) error {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create db dir: %w", err)
		}
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	// SQLite 优化配置
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000", // 64MB
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			LogWarn("pragma %s: %v", p, err)
		}
	}

	// 自动迁移
	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	LogInfo("DB initialized: %s", dbPath)
	return nil
}

func migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS wr_request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			token_id INTEGER NOT NULL,
			token_name TEXT DEFAULT '',
			provider_id INTEGER NOT NULL,
			provider_name TEXT DEFAULT '',
			model_name TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			status_code INTEGER DEFAULT 0,
			latency_ms INTEGER DEFAULT 0,
			cost_cents INTEGER DEFAULT 0,
			is_stream INTEGER DEFAULT 0,
			is_retry INTEGER DEFAULT 0,
			error_message TEXT DEFAULT '',
			client_ip TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rlog_token ON wr_request_logs(token_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_rlog_provider ON wr_request_logs(provider_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_rlog_model ON wr_request_logs(model_name, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_rlog_created ON wr_request_logs(created_at)`,

		`CREATE TABLE IF NOT EXISTS wr_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			key TEXT NOT NULL UNIQUE,
			user_id INTEGER DEFAULT 0,
			models TEXT DEFAULT '',
			provider_ids TEXT DEFAULT '',
			quota_total INTEGER DEFAULT 0,
			quota_used INTEGER DEFAULT 0,
			rate_limit_rpm INTEGER DEFAULT 0,
			subnet_whitelist TEXT DEFAULT '',
			enabled INTEGER DEFAULT 1,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS wr_provider_quota (
			provider_id INTEGER PRIMARY KEY,
			quota_total INTEGER DEFAULT 0,
			quota_used INTEGER DEFAULT 0,
			quota_source TEXT DEFAULT 'manual',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Provider 扩展字段（如果 wr_providers 表由 Flask 创建，这里只加代理相关字段）
		`CREATE TABLE IF NOT EXISTS wr_provider_ext (
			provider_id INTEGER PRIMARY KEY,
			proxy_enabled INTEGER DEFAULT 1,
			rate_limit_rpm INTEGER DEFAULT 0,
			timeout_seconds INTEGER DEFAULT 30,
			max_retries INTEGER DEFAULT 2,
			cost_multiplier REAL DEFAULT 1.0,
			priority INTEGER DEFAULT 50,
			weight INTEGER DEFAULT 100,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("migration: %w\nSQL: %s", err, m)
			}
		}
	}
	return nil
}

// --- Provider 查询 ---

// LoadProviders 从 DB 加载所有 Provider（主表 + 扩展表 JOIN）
func LoadProviders() ([]*Provider, error) {
	rows, err := db.Query(`
		SELECT p.id, p.name, p.type, p.base_url, p.api_key,
		       p.models, p.tags, p.enabled, p.status,
		       p.last_latency_ms, p.last_error,
		       COALESCE(e.priority, 50) as priority,
		       COALESCE(e.weight, 100) as weight,
		       COALESCE(e.proxy_enabled, 1) as proxy_enabled,
		       COALESCE(e.rate_limit_rpm, 0) as rate_limit_rpm,
		       COALESCE(e.timeout_seconds, 30) as timeout_seconds,
		       COALESCE(e.max_retries, 2) as max_retries,
		       COALESCE(e.cost_multiplier, 1.0) as cost_multiplier,
		       COALESCE(q.quota_total, 0) as quota_total,
		       COALESCE(q.quota_used, 0) as quota_used,
		       COALESCE(q.quota_source, 'unknown') as quota_source
		FROM wr_providers p
		LEFT JOIN wr_provider_ext e ON p.id = e.provider_id
		LEFT JOIN wr_provider_quota q ON p.id = q.provider_id
		WHERE p.enabled = 1
		ORDER BY COALESCE(e.priority, 50) DESC, p.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p := &Provider{}
		var lastErr sql.NullString
		var lastCheck sql.NullTime

		if err := rows.Scan(
			&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey,
			&p.Models, &p.Tags, &p.Enabled, &p.Status,
			&p.LastLatencyMs, &lastErr,
			&p.Priority, &p.Weight, &p.ProxyEnabled,
			&p.RateLimitRPM, &p.TimeoutSeconds, &p.MaxRetries, &p.CostMultiplier,
			&p.QuotaTotal, &p.QuotaUsed, &p.QuotaSource,
		); err != nil {
			LogWarn("scan provider: %v", err)
			continue
		}
		if lastErr.Valid {
			p.LastError = lastErr.String
		}
		if lastCheck.Valid {
			p.LastCheckAt = &lastCheck.Time
		}
		providers = append(providers, p)
	}
	return providers, nil
}

// UpdateProviderStatus 更新 Provider 健康状态
func UpdateProviderStatus(id int, status string, latencyMs int, errMsg string) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
		UPDATE wr_providers 
		SET status = ?, last_latency_ms = ?, last_error = ?, last_check_at = ?, updated_at = ?
		WHERE id = ?`,
		status, latencyMs, errMsg, now, now, id,
	)
	if err != nil {
		LogWarn("update provider status id=%d: %v", id, err)
	}
}

// UpdateProviderQuota 更新 Provider 额度
func UpdateProviderQuota(id int, used int64) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
		INSERT INTO wr_provider_quota (provider_id, quota_used, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET quota_used = ?, updated_at = ?`,
		id, used, now, used, now,
	)
	if err != nil {
		LogWarn("update provider quota id=%d: %v", id, err)
	}
}

// --- Token 查询 ---

// LoadTokenByKey 根据 key 查找 Token
func LoadTokenByKey(key string) (*Token, error) {
	t := &Token{}
	var expiresAt sql.NullTime

	err := db.QueryRow(`
		SELECT id, name, key, user_id, models, provider_ids,
		       quota_total, quota_used, rate_limit_rpm,
		       subnet_whitelist, enabled, expires_at, created_at
		FROM wr_tokens WHERE key = ?`, key,
	).Scan(&t.ID, &t.Name, &t.Key, &t.UserID, &t.Models, &t.ProviderIDs,
		&t.QuotaTotal, &t.QuotaUsed, &t.RateLimitRPM,
		&t.SubnetWhitelist, &t.Enabled, &expiresAt, &t.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	return t, nil
}

// DeductTokenQuota 扣减 Token 配额，返回是否成功
func DeductTokenQuota(tokenID int, costCents int64) bool {
	if costCents <= 0 {
		return true
	}
	result, err := db.Exec(`
		UPDATE wr_tokens 
		SET quota_used = quota_used + ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND (quota_total = 0 OR quota_used + ? <= quota_total)`,
		costCents, tokenID, costCents,
	)
	if err != nil {
		LogWarn("deduct token quota id=%d: %v", tokenID, err)
		return false
	}
	n, _ := result.RowsAffected()
	return n > 0
}

// --- RequestLog ---

// InsertRequestLog 写入请求日志
func InsertRequestLog(log *RequestLog) error {
	_, err := db.Exec(`
		INSERT INTO wr_request_logs 
		(request_id, token_id, token_name, provider_id, provider_name,
		 model_name, endpoint, input_tokens, output_tokens,
		 status_code, latency_ms, cost_cents, is_stream, is_retry,
		 error_message, client_ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.RequestID, log.TokenID, log.TokenName, log.ProviderID, log.ProviderName,
		log.ModelName, log.Endpoint, log.InputTokens, log.OutputTokens,
		log.StatusCode, log.LatencyMs, log.CostCents, boolToInt(log.IsStream), boolToInt(log.IsRetry),
		log.ErrorMessage, log.ClientIP, time.Now().UTC(),
	)
	if err != nil {
		LogWarn("insert request log: %v", err)
	}
	return err
}

// GetDailyUsage 获取近 N 天每日用量
func GetDailyUsage(days int) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT DATE(created_at) as date,
		       COUNT(*) as requests,
		       COALESCE(SUM(input_tokens), 0) as input_tokens,
		       COALESCE(SUM(output_tokens), 0) as output_tokens,
		       COALESCE(SUM(cost_cents), 0) as cost_cents,
		       SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as errors
		FROM wr_request_logs
		WHERE created_at >= datetime('now', ?)
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var date string
		var requests, inputTok, outputTok, costCents, errors int64
		if err := rows.Scan(&date, &requests, &inputTok, &outputTok, &costCents, &errors); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"date": date, "requests": requests,
			"input_tokens": inputTok, "output_tokens": outputTok,
			"cost_cents": costCents, "errors": errors,
		})
	}
	return result, nil
}

// GetModelUsage 获取按模型聚合的用量
func GetModelUsage(hours int) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT model_name,
		       COUNT(*) as requests,
		       COALESCE(SUM(input_tokens), 0) as input_tokens,
		       COALESCE(SUM(output_tokens), 0) as output_tokens,
		       COALESCE(SUM(cost_cents), 0) as cost_cents,
		       AVG(latency_ms) as avg_latency,
		       SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as errors
		FROM wr_request_logs
		WHERE created_at >= datetime('now', ?)
		GROUP BY model_name
		ORDER BY cost_cents DESC
	`, fmt.Sprintf("-%d hours", hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var model string
		var requests, inputTok, outputTok, costCents, errors int64
		var avgLatency float64
		if err := rows.Scan(&model, &requests, &inputTok, &outputTok, &costCents, &errors, &avgLatency); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"model": model, "requests": requests,
			"input_tokens": inputTok, "output_tokens": outputTok,
			"cost_cents": costCents, "avg_latency": avgLatency, "errors": errors,
		})
	}
	return result, nil
}

// --- 辅助 ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
