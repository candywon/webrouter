package main

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// setupPauseTestDB 初始化一个隔离 DB 并准备 wr_system_settings 表。
func setupPauseTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pause-test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS wr_system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		value_type TEXT NOT NULL DEFAULT 'string'
	)`); err != nil {
		t.Fatalf("create settings table: %v", err)
	}
}

func setPauseUntil(t *testing.T, raw string, valueType string) {
	t.Helper()
	if _, err := db.Exec(`DELETE FROM wr_system_settings WHERE key='knowledge_pause_until'`); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if raw == "" {
		return
	}
	if _, err := db.Exec(
		`INSERT INTO wr_system_settings (key, value, value_type) VALUES (?, ?, ?)`,
		"knowledge_pause_until", raw, valueType,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestIsKnowledgePaused(t *testing.T) {
	setupPauseTestDB(t)

	cases := []struct {
		name      string
		raw       string
		valueType string
		want      bool
	}{
		{"unset → not paused", "", "", false},
		{"explicit 0 → not paused", "0", "int", false},
		{"past epoch → not paused", "1", "int", false},
		{"future epoch → paused", fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()), "int", true},
		{"-1 permanent → paused", "-1", "int", true},
		{"non-int garbage → not paused (default 0)", "abc", "int", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setPauseUntil(t, c.raw, c.valueType)
			got := IsKnowledgePaused()
			if got != c.want {
				t.Fatalf("IsKnowledgePaused() = %v, want %v (raw=%q)", got, c.want, c.raw)
			}
		})
	}
}

func TestIsKnowledgeEnabled_PausedOverride(t *testing.T) {
	setupPauseTestDB(t)

	// 先开启 capture 开关
	if _, err := db.Exec(
		`INSERT INTO wr_system_settings (key, value, value_type) VALUES (?, ?, ?)`,
		"knowledge_enabled", "true", "bool",
	); err != nil {
		t.Fatalf("insert capture enabled: %v", err)
	}

	// 未暂停：返回 true
	setPauseUntil(t, "0", "int")
	if !IsKnowledgeEnabled() {
		t.Fatalf("expected enabled=true when capture=on and not paused")
	}

	// 永久暂停：暂停态视作未启用
	setPauseUntil(t, "-1", "int")
	if IsKnowledgeEnabled() {
		t.Fatalf("expected enabled=false during permanent pause")
	}

	// 未来 epoch：暂停态视作未启用
	setPauseUntil(t, fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()), "int")
	if IsKnowledgeEnabled() {
		t.Fatalf("expected enabled=false during timed pause")
	}

	// 过期 epoch：恢复
	setPauseUntil(t, "1", "int")
	if !IsKnowledgeEnabled() {
		t.Fatalf("expected enabled=true after pause expires")
	}
}
