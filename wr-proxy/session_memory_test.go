// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- EstimateTokens ----------

func TestEstimateTokens_English(t *testing.T) {
	got := EstimateTokens("hello world")
	// "hello world" = 11 ASCII chars → ceil(11/4) = 3
	if got < 2 || got > 4 {
		t.Errorf("expected 2-4 for 'hello world', got %d", got)
	}
}

func TestEstimateTokens_CJK(t *testing.T) {
	got := EstimateTokens("你好世界")
	// 4 CJK = 4 tokens
	if got != 4 {
		t.Errorf("expected 4 for '你好世界', got %d", got)
	}
}

func TestEstimateTokens_Mixed(t *testing.T) {
	got := EstimateTokens("hello 世界")
	// 5 ASCII + 1 space = 6 ASCII → ceil(6/4)=2, +2 CJK = 4
	if got < 3 || got > 5 {
		t.Errorf("expected 3-5 for 'hello 世界', got %d", got)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("expected 0 for empty, got %d", got)
	}
}

// ---------- detectRecallTrigger ----------

func TestDetectRecallTrigger_Header(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Recall-Session", "abc-123")
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)

	triggered, sid, newBody := detectRecallTrigger(r, body)
	if !triggered {
		t.Fatal("should trigger with X-Recall-Session header")
	}
	if sid != "abc-123" {
		t.Errorf("expected sid=abc-123, got %s", sid)
	}
	if string(newBody) != string(body) {
		t.Error("body should not change for header trigger")
	}
}

func TestDetectRecallTrigger_MagicWord(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Session-Id", "sess-1")
	body := []byte(`{"messages":[{"role":"user","content":"@recall 继续上次的讨论"}]}`)

	triggered, sid, newBody := detectRecallTrigger(r, body)
	if !triggered {
		t.Fatal("should trigger with @recall magic word")
	}
	if sid != "sess-1" {
		t.Errorf("expected sid=sess-1, got %s", sid)
	}

	// 检查 @recall 已剥离
	var req map[string]interface{}
	if err := json.Unmarshal(newBody, &req); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	msgs := req["messages"].([]interface{})
	content := msgs[0].(map[string]interface{})["content"].(string)
	if strings.Contains(strings.ToLower(content), "@recall") {
		t.Errorf("@recall should be stripped, got: %s", content)
	}
	if !strings.Contains(content, "继续上次的讨论") {
		t.Errorf("original text should remain, got: %s", content)
	}
}

func TestDetectRecallTrigger_MagicWord_NoSessionID(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	body := []byte(`{"messages":[{"role":"user","content":"@recall test"}]}`)

	triggered, _, _ := detectRecallTrigger(r, body)
	if triggered {
		t.Error("should NOT trigger without X-Session-Id")
	}
}

func TestDetectRecallTrigger_None(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Session-Id", "sess-1")
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)

	triggered, sid, _ := detectRecallTrigger(r, body)
	if triggered || sid != "" {
		t.Errorf("should not trigger, got triggered=%v sid=%s", triggered, sid)
	}
}

func TestDetectRecallTrigger_NaturalPhrase(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Session-Id", "sess-nl")
	body := []byte(`{"messages":[{"role":"user","content":"还记得昨天我跟你说过吗？"}]}`)

	triggered, sid, newBody := detectRecallTrigger(r, body)
	if !triggered {
		t.Fatal("should trigger with 还记得")
	}
	if sid != "sess-nl" {
		t.Errorf("expected sid=sess-nl, got %s", sid)
	}
	if string(newBody) != string(body) {
		t.Error("body should not change for natural phrase trigger")
	}
}

func TestDetectRecallTrigger_NaturalPhrase_WithAssistant(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Session-Id", "sess-nl")
	// 有 assistant 消息 = 客户端有上下文，不应触发自然语言短语
	body := []byte(`{"messages":[
		{"role":"user","content":"你好"},
		{"role":"assistant","content":"你好！"},
		{"role":"user","content":"还记得我之前说的吗？"}
	]}`)

	triggered, _, _ := detectRecallTrigger(r, body)
	if triggered {
		t.Error("natural phrase should NOT trigger when assistant is present (client has context)")
	}
}

func TestDetectRecallTrigger_Recall_WithAssistant(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Session-Id", "sess-rec")
	// @recall 始终触发，即使有 assistant
	body := []byte(`{"messages":[
		{"role":"user","content":"你好"},
		{"role":"assistant","content":"你好！"},
		{"role":"user","content":"@recall 接着说"}
	]}`)

	triggered, sid, newBody := detectRecallTrigger(r, body)
	if !triggered {
		t.Fatal("@recall should trigger even with assistant present")
	}
	if sid != "sess-rec" {
		t.Errorf("expected sid=sess-rec, got %s", sid)
	}
	// 确认 @recall 被剥离
	if strings.Contains(string(newBody), "@recall") {
		t.Error("@recall should be stripped")
	}
}

func TestDetectRecallTrigger_NaturalPhrase_NoSessionID(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	body := []byte(`{"messages":[{"role":"user","content":"还记得昨天的事吗？"}]}`)

	triggered, _, _ := detectRecallTrigger(r, body)
	if triggered {
		t.Error("natural phrase should NOT trigger without X-Session-Id")
	}
}

func TestDetectRecallTrigger_HeaderTakesPriority(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Recall-Session", "from-header")
	r.Header.Set("X-Session-Id", "from-current")
	body := []byte(`{"messages":[{"role":"user","content":"@recall test"}]}`)

	triggered, sid, _ := detectRecallTrigger(r, body)
	if !triggered || sid != "from-header" {
		t.Errorf("header should win, got triggered=%v sid=%s", triggered, sid)
	}
}

// ---------- stripRecallWord ----------

func TestStripRecallWord(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"@recall 继续", "继续"},
		{"@RECALL hello", "hello"},
		{"前 @recall 后", "前  后"}, // 中间剥离不去多余空格（只 trim 两端）
		{"无魔术词", "无魔术词"},
	}
	for _, c := range cases {
		got := stripRecallWord(c.in)
		if got != c.want {
			t.Errorf("stripRecallWord(%q): want %q, got %q", c.in, c.want, got)
		}
	}
}

// ---------- extractLastUserMessage ----------

func TestExtractLastUserMessage(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"system","content":"sys"},
		{"role":"user","content":"first"},
		{"role":"assistant","content":"reply"},
		{"role":"user","content":"latest user msg"}
	]}`)
	got := extractLastUserMessage(body)
	if got != "latest user msg" {
		t.Errorf("expected 'latest user msg', got %q", got)
	}
}

func TestExtractLastUserMessage_SkipsRecalled(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":"old","__recalled":true},
		{"role":"assistant","content":"old reply","__recalled":true},
		{"role":"user","content":"new question"}
	]}`)
	got := extractLastUserMessage(body)
	if got != "new question" {
		t.Errorf("expected 'new question', got %q", got)
	}
}

// ---------- containsFold ----------

func TestContainsFold(t *testing.T) {
	if !containsFold("Hello @Recall World", "@recall") {
		t.Error("should match case-insensitively")
	}
	if containsFold("hello world", "@recall") {
		t.Error("should not match")
	}
}
