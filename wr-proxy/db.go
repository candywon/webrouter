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
			supports_tools INTEGER DEFAULT 1,
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

	// 增量迁移：为已有表添加新列（SQLite ALTER TABLE 只支持 ADD COLUMN）
	alterMigrations := []string{
		`ALTER TABLE wr_provider_ext ADD COLUMN supports_tools INTEGER DEFAULT 1`,
		`ALTER TABLE wr_request_logs ADD COLUMN error_type TEXT DEFAULT ''`,
	}
	for _, m := range alterMigrations {
		if _, err := db.Exec(m); err != nil {
			// 列已存在是正常情况，忽略
			if !strings.Contains(err.Error(), "duplicate column") {
				LogWarn("alter migration: %v", err)
			}
		}
	}

	return nil
}

// --- Provider 查询 ---

// LoadProviders 从 DB 加载所有 Provider（主表 + 扩展表 JOIN）
func LoadProviders() ([]*Provider, error) {
	rows, err := db.Query(`
		SELECT p.id, p.name, p.type, p.base_url,
		       COALESCE(p.api_key, '') as api_key,
		       COALESCE(p.models, '') as models,
		       COALESCE(p.tags, '') as tags,
		       p.enabled,
		       COALESCE(p.status, 'unchecked') as status,
		       COALESCE(p.last_latency_ms, 0) as last_latency_ms,
		       p.last_error,
		       COALESCE(e.priority, 50) as priority,
		       COALESCE(e.weight, 100) as weight,
		       COALESCE(e.proxy_enabled, 1) as proxy_enabled,
		       COALESCE(e.rate_limit_rpm, 0) as rate_limit_rpm,
		       COALESCE(e.timeout_seconds, 30) as timeout_seconds,
		       COALESCE(e.max_retries, 2) as max_retries,
		       COALESCE(e.cost_multiplier, 1.0) as cost_multiplier,
		       COALESCE(q.quota_total, 0) as quota_total,
		       COALESCE(q.quota_used, 0) as quota_used,
		       COALESCE(q.quota_source, 'unknown') as quota_source,
		       COALESCE(e.supports_tools, 1) as supports_tools
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
			&p.SupportsTools,
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

// LoadChannels 从 DB 加载 Channel 并展开为 Provider 调度项
// 每个 Channel 会创建一个独立的 Provider 对象（ID 编码为 channelID*100000+providerID）
// 无 Channel 的 Provider 仍作为默认调度项参与
func LoadChannels(providers []*Provider) []*Provider {
	// 收集有 Channel 的 Provider ID
	hasChannel := make(map[int]bool)

	rows, err := db.Query(`
		SELECT c.id, c.provider_id, c.name,
		       COALESCE(c.base_url, '') as base_url,
		       COALESCE(c.api_key, '') as api_key,
		       COALESCE(c.models, '') as models,
		       COALESCE(c.priority, 0) as priority,
		       COALESCE(c.weight, 0) as weight,
		       COALESCE(c.rate_limit_rpm, 0) as rate_limit_rpm,
		       COALESCE(c.cost_multiplier, 0) as cost_multiplier,
		       c.enabled,
		       COALESCE(c.status, 'unchecked') as status,
		       COALESCE(c.last_latency_ms, 0) as last_latency_ms,
		       COALESCE(c.last_error, '') as last_error
		FROM wr_provider_channels c
		WHERE c.enabled = 1
		ORDER BY c.provider_id, COALESCE(c.priority, 0) DESC
	`)
	if err != nil {
		LogWarn("load channels: %v", err)
		return providers
	}
	defer rows.Close()

	// 构建 Provider ID → Provider 的映射
	providerMap := make(map[int]*Provider)
	for _, p := range providers {
		providerMap[p.ID] = p
	}

	var channelProviders []*Provider
	for rows.Next() {
		var chID, providerID, priority, weight, rateLimitRPM, lastLatencyMs int
		var name, baseURL, apiKey, models, status, lastErr string
		var costMultiplier float64
		var enabled bool

		if err := rows.Scan(&chID, &providerID, &name, &baseURL, &apiKey, &models,
			&priority, &weight, &rateLimitRPM, &costMultiplier,
			&enabled, &status, &lastLatencyMs, &lastErr); err != nil {
			LogWarn("scan channel: %v", err)
			continue
		}

		parent, ok := providerMap[providerID]
		if !ok {
			continue // 父 Provider 不存在或未启用
		}

		hasChannel[providerID] = true

		// 继承逻辑：Channel 字段为空则用 Provider 的
		resolvedBaseURL := baseURL
		if resolvedBaseURL == "" {
			resolvedBaseURL = parent.BaseURL
		}
		resolvedAPIKey := apiKey
		if resolvedAPIKey == "" {
			resolvedAPIKey = parent.APIKey
		}
		resolvedModels := models
		if resolvedModels == "" {
			resolvedModels = parent.Models
		}
		resolvedPriority := priority
		if resolvedPriority == 0 {
			resolvedPriority = parent.Priority
		}
		resolvedWeight := weight
		if resolvedWeight == 0 {
			resolvedWeight = parent.Weight
		}
		resolvedRPM := rateLimitRPM
		if resolvedRPM == 0 {
			resolvedRPM = parent.RateLimitRPM
		}
		resolvedCostMul := costMultiplier
		if resolvedCostMul == 0 {
			resolvedCostMul = parent.CostMultiplier
		}

		// 编码 ID: channelID*100000 + providerID，确保唯一
		encodedID := chID*100000 + providerID

		cp := &Provider{
			ID:             encodedID,
			Name:           fmt.Sprintf("%s/%s", parent.Name, name),
			Type:           parent.Type,
			BaseURL:        resolvedBaseURL,
			APIKey:         resolvedAPIKey,
			Models:         resolvedModels,
			Tags:           parent.Tags,
			Priority:       resolvedPriority,
			Weight:         resolvedWeight,
			ProxyEnabled:   parent.ProxyEnabled,
			RateLimitRPM:   resolvedRPM,
			TimeoutSeconds: parent.TimeoutSeconds,
			MaxRetries:     parent.MaxRetries,
			CostMultiplier: resolvedCostMul,
			Enabled:        enabled,
			Status:         status,
			LastLatencyMs:  lastLatencyMs,
			LastError:      lastErr,
			QuotaTotal:     parent.QuotaTotal,
			QuotaUsed:      parent.QuotaUsed,
			QuotaSource:    parent.QuotaSource,
			SupportsTools:  parent.SupportsTools,
		}
		channelProviders = append(channelProviders, cp)
	}

	// 合并：无 Channel 的 Provider 保留原样，有 Channel 的 Provider 只保留 Channel 展开项
	var result []*Provider
	for _, p := range providers {
		if !hasChannel[p.ID] {
			result = append(result, p)
		}
	}
	result = append(result, channelProviders...)

	LogInfo("LoadChannels: %d providers, %d have channels, %d expanded",
		len(providers), len(hasChannel), len(channelProviders))

	return result
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
	LogInfo("InsertRequestLog: reqID=%s token=%d provider=%d model=%s status=%d latency=%d",
		log.RequestID, log.TokenID, log.ProviderID, log.ModelName, log.StatusCode, log.LatencyMs)
	_, err := db.Exec(`
		INSERT INTO wr_request_logs 
		(request_id, token_id, token_name, provider_id, provider_name,
		 model_name, endpoint, input_tokens, output_tokens,
		 status_code, latency_ms, cost_cents, is_stream, is_retry,
		 error_message, error_type, client_ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.RequestID, log.TokenID, log.TokenName, log.ProviderID, log.ProviderName,
		log.ModelName, log.Endpoint, log.InputTokens, log.OutputTokens,
		log.StatusCode, log.LatencyMs, log.CostCents, boolToInt(log.IsStream), boolToInt(log.IsRetry),
		log.ErrorMessage, log.ErrorType, log.ClientIP, time.Now().UTC(),
	)
	if err != nil {
		LogError("insert request log FAILED: %v", err)
	} else {
		LogInfo("insert request log OK: reqID=%s", log.RequestID)
	}
	return err
}

// GetDailyUsage 获取近 N 天每日用量
func GetDailyUsage(days int) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT DATE(created_at) as date,
		       COUNT(*) as requests,
		       SUM(CASE WHEN status_code < 400 AND is_retry = 0 AND error_message = '' THEN 1 ELSE 0 END) as valid_requests,
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
		var requests, validRequests, inputTok, outputTok, costCents, errors int64
		if err := rows.Scan(&date, &requests, &validRequests, &inputTok, &outputTok, &costCents, &errors); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"date": date, "requests": requests, "valid_requests": validRequests,
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
		       SUM(CASE WHEN status_code < 400 AND is_retry = 0 AND error_message = '' THEN 1 ELSE 0 END) as valid_requests,
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
		var requests, validRequests, inputTok, outputTok, costCents, errors int64
		var avgLatency float64
		if err := rows.Scan(&model, &requests, &validRequests, &inputTok, &outputTok, &costCents, &errors, &avgLatency); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"model": model, "requests": requests, "valid_requests": validRequests,
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
