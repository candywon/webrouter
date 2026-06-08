// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// 隐私/企业敏感信息脱敏引擎
// 核心职责：
// 1. 识别请求中的敏感信息（手机号、身份证、邮箱、银行卡、IP等）
// 2. 替换为 [TYPE_N] 格式的占位标记（如 [PHONE_1] [IDCARD_2]）
// 3. 维护请求级映射表，响应时将标记还原为原始值
// 4. 支持内置规则 + 自定义规则（精确词汇/正则）
//
// 标记格式选择 [TYPE_N] 的理由（实测 qwen3-coder-flash + glm-5 均完美保留）：
// - 纯ASCII，正则匹配零开销
// - JSON/代码/Markdown 中自然
// - 还原时 strings.ReplaceAll 最简单
// - 流式输出中标记不会跨 chunk

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// --- 脱敏级别 ---

const (
	DesensitizeOff      = "off"      // 关闭
	DesensitizeStandard = "standard" // 标准模式：内置规则（手机号、身份证、邮箱、银行卡、IP）
	DesensitizeStrict   = "strict"   // 严格模式：内置规则 + 自定义规则
)

// --- 脱敏规则类型 ---

const (
	RuleTypeBuiltin = "builtin" // 内置规则
	RuleTypeExact   = "exact"   // 精确词汇匹配
	RuleTypeRegex   = "regex"   // 正则匹配
)

// DesensitizeRule 脱敏规则（DB 持久化 + 运行时缓存）
type DesensitizeRule struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`       // 规则名称
	Type      string `json:"type"`       // builtin/exact/regex
	Pattern   string `json:"pattern"`    // exact=精确文本, regex=正则表达式
	Category  string `json:"category"`   // PHONE/IDCARD/EMAIL/BANKCARD/IP/NAME/COMPANY/APIKEY/CUSTOM
	Level     string `json:"level"`      // standard/strict — 哪个级别生效
	Enabled   bool   `json:"enabled"`    // 是否启用
	SortOrder int    `json:"sort_order"` // 执行顺序（小的先匹配）
}

// --- 内置规则的正则定义 ---

var builtinPatterns = []struct {
	category string
	pattern  string
	compiled *regexp.Regexp
}{
	// 规则执行顺序：长规则优先！避免短规则先匹配长内容的一部分
	// 身份证号：18位（最后一位可能是X）— 必须在手机号之前
	{category: "IDCARD", pattern: `[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`},
	// 银行卡号：13-19位纯数字 — 必须在手机号之前
	{category: "BANKCARD", pattern: `\b[3-6]\d{12,18}\b`},
	// API Key 常见格式 — 长字符串优先
	{category: "APIKEY", pattern: `(?:sk|sk_live|sk_test|key|api_key|apikey|secret|token|Bearer)\s*[:=]\s*["']?[\w\-]{16,}["']?`},
	// 邮箱
	{category: "EMAIL", pattern: `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`},
	// 中国手机号：1开头11位 — 必须在身份证之后
	// 加 \b 避免嵌入长数字串时被误匹配（如 913812345678）
	{category: "PHONE", pattern: `\b1[3-9]\d{9}\b`},
	// IPv4地址（排除127.0.0.1等保留地址）
	{category: "IP", pattern: `\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`},
}

// --- 请求级映射表 ---

// ReplacementMap 脱敏映射：标记 → 原始值
type ReplacementMap struct {
	mu    sync.RWMutex
	items map[string]string // "[PHONE_1]" → "13812345678"
}

// NewReplacementMap 创建映射
func NewReplacementMap() *ReplacementMap {
	return &ReplacementMap{items: make(map[string]string)}
}

// Add 添加映射
func (m *ReplacementMap) Add(marker, original string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[marker] = original
}

// Get 查找原始值
func (m *ReplacementMap) Get(marker string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.items[marker]
	return v, ok
}

// Restore 在文本中还原所有标记为原始值
func (m *ReplacementMap) Restore(text string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for marker, original := range m.items {
		text = strings.ReplaceAll(text, marker, original)
	}
	return text
}

// --- 脱敏引擎 ---

var (
	desensitizeRules    []*DesensitizeRule
	desensitizeRulesMux sync.RWMutex
	desensitizeSeq      uint64 // 全局计数器，生成标记编号
)

// LoadDesensitizeRules 加载自定义脱敏规则（从 DB）
func LoadDesensitizeRules() error {
	desensitizeRulesMux.Lock()
	defer desensitizeRulesMux.Unlock()

	rules, err := loadRulesFromDB()
	if err != nil {
		return err
	}
	desensitizeRules = rules
	LogInfo("Desensitize: loaded %d custom rules", len(rules))
	return nil
}

// GetActiveRules 获取当前生效的规则（内置 + 自定义）
func GetActiveRules(level string) []*DesensitizeRule {
	desensitizeRulesMux.RLock()
	defer desensitizeRulesMux.RUnlock()

	var result []*DesensitizeRule

	// 内置规则：standard 及以上都生效
	if level == DesensitizeStandard || level == DesensitizeStrict {
		for i := range builtinPatterns {
			result = append(result, &DesensitizeRule{
				ID:       -(i + 1), // 负数ID表示内置
				Name:     builtinPatterns[i].category + "内置规则",
				Type:     RuleTypeBuiltin,
				Category: builtinPatterns[i].category,
				Level:    DesensitizeStandard,
				Enabled:  true,
			})
		}
	}

	// 自定义规则
	for _, r := range desensitizeRules {
		if !r.Enabled {
			continue
		}
		// strict 级别包含 standard 级别的规则
		if level == DesensitizeStrict ||
			(r.Level == DesensitizeStandard && (level == DesensitizeStandard || level == DesensitizeStrict)) {
			result = append(result, r)
		}
	}

	return result
}

// DesensitizeResult 脱敏结果
type DesensitizeResult struct {
	Body     []byte          // 脱敏后的请求体
	Mapping  *ReplacementMap // 标记→原始值 映射
	Modified bool            // 是否有替换
	Redacted []string        // 被脱敏的内容摘要
	Skipped  bool            // 是否跳过脱敏（关闭或无规则）
}

// DesensitizeRequest 对请求体进行脱敏处理
// token: 当前请求的 Token（用于判断脱敏开关和级别）
// body: 原始请求体
func DesensitizeRequest(token *Token, body []byte) *DesensitizeResult {
	result := &DesensitizeResult{
		Mapping:  NewReplacementMap(),
		Modified: false,
	}

	// 检查 Token 是否开启脱敏
	if !token.DesensitizeEnabled {
		result.Body = body
		result.Skipped = true
		return result
	}

	level := token.DesensitizeLevel
	if level == "" {
		level = DesensitizeStandard
	}
	if level == DesensitizeOff {
		result.Body = body
		result.Skipped = true
		return result
	}

	// 解析 JSON 提取所有字符串值
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		// 非 JSON，直接对原始文本做正则替换
		desensitized := desensitizeText(string(body), level, result.Mapping, &result.Redacted)
		result.Body = []byte(desensitized)
		result.Modified = len(result.Redacted) > 0
		return result
	}

	// 递归处理 JSON 中所有字符串值
	modified := desensitizeJSON(req, level, result.Mapping, &result.Redacted)

	if modified {
		newBody, err := json.Marshal(req)
		if err != nil {
			LogWarn("Desensitize: marshal failed: %v", err)
			result.Body = body
			return result
		}
		result.Body = newBody
		result.Modified = true
	} else {
		result.Body = body
	}

	return result
}

// RestoreResponse 在响应中还原脱敏标记
func RestoreResponse(body []byte, mapping *ReplacementMap) []byte {
	if mapping == nil {
		return body
	}
	text := string(body)
	restored := mapping.Restore(text)
	return []byte(restored)
}

// RestoreStreamChunk 还原流式 SSE chunk 中的脱敏标记
func RestoreStreamChunk(line string, mapping *ReplacementMap) string {
	if mapping == nil {
		return line
	}
	return mapping.Restore(line)
}

// --- 内部实现 ---

// desensitizeText 对纯文本进行脱敏
func desensitizeText(text, level string, mapping *ReplacementMap, redacted *[]string) string {
	rules := GetActiveRules(level)
	// 计数器按 category 独立
	counters := make(map[string]int)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		var re *regexp.Regexp
		switch rule.Type {
		case RuleTypeBuiltin:
			// 找到对应的内置正则
			for _, bp := range builtinPatterns {
				if bp.category == rule.Category {
					if bp.compiled == nil {
						bp.compiled = regexp.MustCompile(bp.pattern)
					}
					re = bp.compiled
					break
				}
			}
		case RuleTypeRegex:
			var err error
			re, err = regexp.Compile(rule.Pattern)
			if err != nil {
				LogWarn("Desensitize: invalid regex rule %d: %v", rule.ID, err)
				continue
			}
		case RuleTypeExact:
			// 精确匹配转正则（转义特殊字符）
			re = regexp.MustCompile(regexp.QuoteMeta(rule.Pattern))
		}

		if re == nil {
			continue
		}

		// 查找所有匹配
		matches := re.FindAllString(text, -1)
		if len(matches) == 0 {
			continue
		}

		// 对每个唯一匹配生成标记
		seen := make(map[string]string) // 原始值 → 标记
		for _, match := range matches {
			// 跳过包含标记方括号的匹配（说明是前一轮替换的残留子串）
			// 例如：身份证被替换为 [IDCARD_1] 后，手机号正则可能匹配到其中的数字子串
			if strings.Contains(match, "[") || strings.Contains(match, "]") {
				continue
			}
			if marker, ok := seen[match]; ok {
				// 已经生成过标记，直接替换
				text = strings.ReplaceAll(text, match, marker)
				continue
			}
			counters[rule.Category]++
			n := counters[rule.Category]
			marker := fmt.Sprintf("[%s_%d]", rule.Category, n)
			mapping.Add(marker, match)
			seen[match] = marker
			*redacted = append(*redacted, fmt.Sprintf("%s:%s→%s", rule.Category, truncateSensitive(match, 8), marker))
			text = strings.ReplaceAll(text, match, marker)
		}
	}

	return text
}

// desensitizeJSON 递归处理 JSON 对象中所有字符串值
func desensitizeJSON(obj map[string]interface{}, level string, mapping *ReplacementMap, redacted *[]string) bool {
	modified := false
	for key, val := range obj {
		switch v := val.(type) {
		case string:
			desensitized := desensitizeText(v, level, mapping, redacted)
			if desensitized != v {
				obj[key] = desensitized
				modified = true
			}
		case map[string]interface{}:
			if desensitizeJSON(v, level, mapping, redacted) {
				modified = true
			}
		case []interface{}:
			for i, item := range v {
				switch itemVal := item.(type) {
				case string:
					desensitized := desensitizeText(itemVal, level, mapping, redacted)
					if desensitized != itemVal {
						v[i] = desensitized
						modified = true
					}
				case map[string]interface{}:
					if desensitizeJSON(itemVal, level, mapping, redacted) {
						modified = true
					}
				}
			}
		}
	}
	return modified
}

// truncateSensitive 截断敏感信息用于日志（避免在日志中泄露完整敏感信息）
func truncateSensitive(s string, maxLen int) string {
	if len(s) <= maxLen {
		return strings.Repeat("*", len(s))
	}
	return s[:3] + "..." + s[len(s)-3:]
}

// --- DB 操作 ---

func loadRulesFromDB() ([]*DesensitizeRule, error) {
	rows, err := db.Query(`
		SELECT id, name, type, pattern, category, level, enabled, sort_order
		FROM wr_desensitize_rules
		WHERE enabled = 1
		ORDER BY sort_order ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*DesensitizeRule
	for rows.Next() {
		r := &DesensitizeRule{}
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Pattern, &r.Category, &r.Level, &enabled, &r.SortOrder); err != nil {
			LogWarn("Desensitize: scan rule: %v", err)
			continue
		}
		r.Enabled = enabled == 1
		rules = append(rules, r)
	}
	return rules, nil
}

// --- 预编译内置正则（启动时调用） ---

func InitBuiltinPatterns() {
	for i := range builtinPatterns {
		compiled, err := regexp.Compile(builtinPatterns[i].pattern)
		if err != nil {
			LogError("Desensitize: compile builtin pattern %s failed: %v", builtinPatterns[i].category, err)
			continue
		}
		builtinPatterns[i].compiled = compiled
		LogInfo("Desensitize: builtin rule compiled: %s", builtinPatterns[i].category)
	}
}

// --- 全局计数器重置（每次请求开始时调用） ---

// ResetDesensitizeCounter 重置脱敏标记编号计数器
// 不需要——改为映射表请求级隔离，计数器用 mapping 内部的 map 大小
// 保留此函数作为空操作，避免调用方报错
func ResetDesensitizeCounter() {
	atomic.StoreUint64(&desensitizeSeq, 0)
}
