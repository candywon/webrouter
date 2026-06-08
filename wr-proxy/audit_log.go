// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"sync"
	"time"
)

// 审计日志常量
const (
	AuditKnowledgeCapture = "knowledge_capture"
	AuditKnowledgeExtract = "knowledge_extract"
	AuditKnowledgeAccess  = "knowledge_access"
	AuditConfigChange     = "config_change"
	AuditDataDelete       = "data_delete"
	AuditRawCleanup       = "raw_cleanup"
	AuditRetentionCleanup = "retention_cleanup"
)

const (
	AuditResourceRaw    = "raw"
	AuditResourceItem   = "item"
	AuditResourceDomain = "domain"
	AuditResourceToken  = "token"
	AuditResourceConfig = "config"
)

// auditCh 异步审计日志 channel
var (
	auditCh   chan AuditEntry
	auditOnce sync.Once
)

// AuditEntry 单条审计日志
type AuditEntry struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	TokenID      int    `json:"token_id"`
	Detail       string `json:"detail"`
	ClientIP     string `json:"client_ip"`
}

// InitAuditLogger 初始化审计日志模块
func InitAuditLogger() {
	auditOnce.Do(func() {
		auditCh = make(chan AuditEntry, 256)
		go auditWorker()
		LogInfo("Audit logger initialized: channel buffer=256")
	})
}

// auditWorker 后台协程：批量写入审计日志
func auditWorker() {
	var batch []AuditEntry
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case entry := <-auditCh:
			batch = append(batch, entry)
			if len(batch) >= 50 {
				flushAuditBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushAuditBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushAuditBatch 批量写入审计日志到数据库
func flushAuditBatch(entries []AuditEntry) {
	if db == nil {
		return
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for _, e := range entries {
		_, err := db.Exec(`
			INSERT INTO wr_audit_log (action, resource_type, resource_id, token_id, detail, client_ip, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			e.Action, e.ResourceType, e.ResourceID, e.TokenID, e.Detail, e.ClientIP, now)
		if err != nil {
			LogWarn("[audit] write failed: %v", err)
		}
	}
	if len(entries) > 0 {
		LogInfo("[audit] flushed %d entries", len(entries))
	}
}

// LogAudit 记录审计日志（非阻塞异步）
func LogAudit(action, resourceType, resourceID string, tokenID int, detail interface{}, clientIP string) {
	var detailStr string
	if detail != nil {
		b, _ := json.Marshal(detail)
		detailStr = string(b)
	}

	entry := AuditEntry{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		TokenID:      tokenID,
		Detail:       detailStr,
		ClientIP:     clientIP,
	}

	select {
	case auditCh <- entry:
	default:
		LogWarn("[audit] channel full, dropping entry: %s", action)
	}
}

// LogConfigChange 快捷方法：记录配置变更
func LogConfigChange(resourceName string, tokenID int, detail interface{}) {
	LogAudit(AuditConfigChange, AuditResourceConfig, resourceName, tokenID, detail, "")
}

// LogKnowledgeAccess 快捷方法：记录知识数据访问
func LogKnowledgeAccess(resourceType, resourceID string, tokenID int) {
	LogAudit(AuditKnowledgeAccess, resourceType, resourceID, tokenID, nil, "")
}
