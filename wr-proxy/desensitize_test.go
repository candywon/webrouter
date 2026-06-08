// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- 辅助：创建开启脱敏的 Token ---

func newDesensitizeToken(level string) *Token {
	return &Token{
		ID:                 1,
		Name:               "test",
		Key:                "sk-wr-test",
		DesensitizeEnabled: true,
		DesensitizeLevel:   level,
	}
}

func newNoDesensitizeToken() *Token {
	return &Token{
		ID:                 1,
		Name:               "test",
		Key:                "sk-wr-test",
		DesensitizeEnabled: false,
	}
}

// --- 内置正则初始化 ---

func TestInitBuiltinPatterns(t *testing.T) {
	InitBuiltinPatterns()
	for _, bp := range builtinPatterns {
		if bp.compiled == nil {
			t.Errorf("builtin pattern %s not compiled", bp.category)
		}
	}
}

// --- 手机号脱敏 ---

func TestDesensitizePhone(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"请拨打13812345678联系客户"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "13812345678") {
		t.Fatal("phone number should be replaced")
	}
	if !strings.Contains(string(result.Body), "[PHONE_1]") {
		t.Fatalf("should contain [PHONE_1], got: %s", string(result.Body))
	}

	// 还原
	restored := result.Mapping.Restore(string(result.Body))
	if !strings.Contains(restored, "13812345678") {
		t.Fatal("restored body should contain original phone")
	}
}

// --- 身份证脱敏 ---

func TestDesensitizeIDCard(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"身份证号110101199003076534"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "110101199003076534") {
		t.Fatal("ID card should be replaced")
	}
	if !strings.Contains(string(result.Body), "[IDCARD_1]") {
		t.Fatalf("should contain [IDCARD_1], got: %s", string(result.Body))
	}
}

// --- 邮箱脱敏 ---

func TestDesensitizeEmail(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"发送到test@example.com"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "test@example.com") {
		t.Fatal("email should be replaced")
	}
	if !strings.Contains(string(result.Body), "[EMAIL_1]") {
		t.Fatalf("should contain [EMAIL_1], got: %s", string(result.Body))
	}
}

// --- 多种敏感信息同时脱敏 ---

func TestDesensitizeMultiple(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"客户13812345678，邮箱zhang@company.com，身份证110101199003076534"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "13812345678") {
		t.Fatal("phone should be replaced")
	}
	if strings.Contains(string(result.Body), "zhang@company.com") {
		t.Fatal("email should be replaced")
	}
	if strings.Contains(string(result.Body), "110101199003076534") {
		t.Fatal("ID card should be replaced")
	}

	// 还原全部
	restored := result.Mapping.Restore(string(result.Body))
	if !strings.Contains(restored, "13812345678") {
		t.Fatal("restored should have phone")
	}
	if !strings.Contains(restored, "zhang@company.com") {
		t.Fatal("restored should have email")
	}
	if !strings.Contains(restored, "110101199003076534") {
		t.Fatal("restored should have ID card")
	}
}

// --- 相同敏感信息重复出现 ---

func TestDesensitizeSameValueRepeated(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"电话13812345678，再拨13812345678"}]}`)
	result := DesensitizeRequest(token, body)

	// 同一个手机号应该使用同一个标记
	count := strings.Count(string(result.Body), "[PHONE_1]")
	if count != 2 {
		t.Fatalf("should have 2 occurrences of [PHONE_1], got %d", count)
	}

	// 映射表应该只有一条
	if len(result.Mapping.items) != 1 {
		t.Fatalf("mapping should have 1 entry, got %d", len(result.Mapping.items))
	}
}

// --- 不同敏感信息独立编号 ---

func TestDesensitizeDifferentPhones(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"电话13812345678和13987654321"}]}`)
	result := DesensitizeRequest(token, body)

	if !strings.Contains(string(result.Body), "[PHONE_1]") {
		t.Fatal("should contain [PHONE_1]")
	}
	if !strings.Contains(string(result.Body), "[PHONE_2]") {
		t.Fatal("should contain [PHONE_2]")
	}

	// 还原
	restored := result.Mapping.Restore(string(result.Body))
	if !strings.Contains(restored, "13812345678") || !strings.Contains(restored, "13987654321") {
		t.Fatal("restored should have both phones")
	}
}

// --- Token 关闭脱敏 ---

func TestDesensitizeOff(t *testing.T) {
	InitBuiltinPatterns()
	token := newNoDesensitizeToken()

	body := []byte(`{"messages":[{"role":"user","content":"电话13812345678"}]}`)
	result := DesensitizeRequest(token, body)

	if result.Modified {
		t.Fatal("should NOT be modified when desensitize is off")
	}
	if !result.Skipped {
		t.Fatal("should be marked as skipped")
	}
	if strings.Contains(string(result.Body), "[PHONE_") {
		t.Fatal("should not contain markers")
	}
}

// --- off 级别 ---

func TestDesensitizeLevelOff(t *testing.T) {
	InitBuiltinPatterns()
	token := &Token{
		ID:                 1,
		DesensitizeEnabled: true,
		DesensitizeLevel:   DesensitizeOff,
	}

	body := []byte(`{"messages":[{"role":"user","content":"电话13812345678"}]}`)
	result := DesensitizeRequest(token, body)

	if result.Modified {
		t.Fatal("off level should not modify")
	}
}

// --- 嵌套 JSON 中的字符串脱敏 ---

func TestDesensitizeNestedJSON(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{
		"messages": [
			{"role": "user", "content": "客户手机13812345678"},
			{"role": "assistant", "content": "好的"},
			{"role": "user", "content": "邮箱test@example.com"}
		]
	}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "13812345678") {
		t.Fatal("phone should be replaced")
	}
	if strings.Contains(string(result.Body), "test@example.com") {
		t.Fatal("email should be replaced")
	}

	// 还原
	restored := result.Mapping.Restore(string(result.Body))
	if !strings.Contains(restored, "13812345678") {
		t.Fatal("restored should have phone")
	}
}

// --- 非JSON请求体 ---

func TestDesensitizeNonJSON(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`plain text with phone 13812345678`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified even for non-JSON")
	}
	if strings.Contains(string(result.Body), "13812345678") {
		t.Fatal("phone should be replaced")
	}
}

// --- 还原映射表 ---

func TestReplacementMapRestore(t *testing.T) {
	m := NewReplacementMap()
	m.Add("[PHONE_1]", "13812345678")
	m.Add("[EMAIL_1]", "test@example.com")
	m.Add("[IDCARD_1]", "110101199003076534")

	text := "客户[PHONE_1]的邮箱是[EMAIL_1]，身份证[IDCARD_1]"
	restored := m.Restore(text)

	if restored != "客户13812345678的邮箱是test@example.com，身份证110101199003076534" {
		t.Fatalf("unexpected restored text: %s", restored)
	}
}

// --- RestoreResponse ---

func TestRestoreResponse(t *testing.T) {
	m := NewReplacementMap()
	m.Add("[PHONE_1]", "13812345678")

	body := []byte(`{"choices":[{"message":{"content":"客户电话[PHONE_1]"}}]}`)
	restored := RestoreResponse(body, m)

	if !strings.Contains(string(restored), "13812345678") {
		t.Fatal("response should be restored")
	}
	if strings.Contains(string(restored), "[PHONE_1]") {
		t.Fatal("marker should be replaced")
	}
}

// --- RestoreStreamChunk ---

func TestRestoreStreamChunk(t *testing.T) {
	m := NewReplacementMap()
	m.Add("[PHONE_1]", "13812345678")

	chunk := `data: {"choices":[{"delta":{"content":"电话[PHONE_1]"}}]}`
	restored := RestoreStreamChunk(chunk, m)

	if !strings.Contains(restored, "13812345678") {
		t.Fatal("chunk should be restored")
	}
}

// --- RestoreResponse nil mapping ---

func TestRestoreResponseNilMapping(t *testing.T) {
	body := []byte(`{"content":"hello"}`)
	restored := RestoreResponse(body, nil)
	if string(restored) != `{"content":"hello"}` {
		t.Fatal("nil mapping should pass through")
	}
}

// --- IPv4 脱敏 ---

func TestDesensitizeIP(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"服务器地址192.168.1.100"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "192.168.1.100") {
		t.Fatal("IP should be replaced")
	}
}

// --- API Key 格式脱敏 ---

func TestDesensitizeAPIKey(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"api_key=sk-abc123def456ghi789jkl012mno345"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "sk-abc123def456ghi789jkl012mno345") {
		t.Fatal("API key should be replaced")
	}
}

// --- 银行卡号脱敏 ---

func TestDesensitizeBankCard(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"银行卡6222021234567890123"}]}`)
	result := DesensitizeRequest(token, body)

	if !result.Modified {
		t.Fatal("should be modified")
	}
	if strings.Contains(string(result.Body), "6222021234567890123") {
		t.Fatal("bank card should be replaced")
	}
}

// --- 无敏感信息的请求不修改 ---

func TestDesensitizeNoSensitive(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	body := []byte(`{"messages":[{"role":"user","content":"今天天气怎么样"}]}`)
	result := DesensitizeRequest(token, body)

	if result.Modified {
		t.Fatal("should NOT be modified when no sensitive info")
	}
}

// --- JSON 结构完整性 ---

func TestDesensitizeJSONStructure(t *testing.T) {
	InitBuiltinPatterns()
	token := newDesensitizeToken(DesensitizeStandard)

	original := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "电话13812345678",
			},
		},
	}
	body, _ := json.Marshal(original)
	result := DesensitizeRequest(token, body)

	// 解析结果应该是合法 JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result.Body, &parsed); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}

	// model 字段不变
	if parsed["model"] != "gpt-4" {
		t.Fatal("model should not change")
	}

	// messages 结构完整
	msgs, ok := parsed["messages"].([]interface{})
	if !ok || len(msgs) != 1 {
		t.Fatal("messages structure should be intact")
	}
	msg, ok := msgs[0].(map[string]interface{})
	if !ok || msg["role"] != "user" {
		t.Fatal("message role should be intact")
	}
}

// --- truncateSensitive ---

func TestTruncateSensitive(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		expect string
	}{
		{"13812345678", 8, "138...678"},
		{"abc", 8, "***"},
		{"12345678", 8, "********"},
	}
	for _, tt := range tests {
		got := truncateSensitive(tt.input, tt.maxLen)
		if got != tt.expect {
			t.Errorf("truncateSensitive(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expect)
		}
	}
}
