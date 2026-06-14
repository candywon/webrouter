// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

// wr-proxy 优化特性开关
// 功能由 wr-proxy 在请求转发到上游之前自动处理请求体实现

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FeatureToggles 优化特性开关状态
var FeatureToggles struct {
	DynamicContentLast bool // 动态内容后置
	TokenCompression   bool // Token 压缩（RTK）
	SessionCompression bool // 会话压缩
}

// ProxyEnabled 代理网关总开关（控制用户端 API 请求是否可用）
var ProxyEnabled = true

// LoadFeatureToggles 从 DB 加载特性开关状态
func LoadFeatureToggles() {
	FeatureToggles.DynamicContentLast = LoadSetting("feature_dynamic_content_last", false).(bool)
	FeatureToggles.TokenCompression = LoadSetting("feature_token_compression", false).(bool)
	FeatureToggles.SessionCompression = LoadSetting("feature_session_compression", false).(bool)

	// 代理网关总开关
	prev := ProxyEnabled
	ProxyEnabled = LoadSetting("proxy_enabled", true).(bool)
	if prev != ProxyEnabled {
		LogInfo("Proxy gateway %s (proxy_enabled=%v)", map[bool]string{true: "ENABLED", false: "DISABLED"}[ProxyEnabled], ProxyEnabled)
	}

	LogInfo("Feature toggles: dynamic_content_last=%v, token_compression=%v, session_compression=%v, proxy_enabled=%v",
		FeatureToggles.DynamicContentLast,
		FeatureToggles.TokenCompression,
		FeatureToggles.SessionCompression,
		ProxyEnabled)
}

// ApplyFeatureTransforms 根据特性开关对请求 body 进行预处理
// 返回处理后的 body 和处理描述
func ApplyFeatureTransforms(body []byte, model string, token *Token) ([]byte, string) {
	if len(body) == 0 {
		return body, ""
	}

	var actions []string
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body, ""
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return body, ""
	}

	// 1. 动态内容后置
	if FeatureToggles.DynamicContentLast {
		messages = reorderDynamicContentLast(messages)
		actions = append(actions, "dynamic_content_reorder")
	}

	// 2. Token 压缩（对 system prompt 长文本做摘要）
	if FeatureToggles.TokenCompression {
		messages = compressSystemPrompts(messages)
		actions = append(actions, "token_compression")
	}

	// 3. 会话压缩（对多轮对话历史做摘要）
	if FeatureToggles.SessionCompression {
		messages = compressSessionHistory(messages)
		actions = append(actions, "session_compression")
	}

	if len(actions) == 0 {
		return body, ""
	}

	req["messages"] = messages
	result, err := json.Marshal(req)
	if err != nil {
		return body, ""
	}

	LogInfo("FeatureTransform: model=%s actions=%v", model, actions)
	return result, strings.Join(actions, ", ")
}

// --- 动态内容后置 ---
//
// 目的：让上游 prompt cache 命中率更高。cache 看的是请求前缀的字节稳定性，
// 因此 messages[] 的相对顺序绝对不能动；只能在单条消息的 content 内部把
// 「易变片段」下沉到尾部，让前缀更稳定。
//
// 改动范围：仅 system 消息 + 最后一条 user 消息（cache 命中收益最大的两个位置）。
// 多模态 content（content 是 array）整条跳过——重排数组里的 part 会破坏图文配对。

// 动态内容正则模式（命中任一即视为动态片段）
var dynamicPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^\s)]+`),            // URL
	regexp.MustCompile(`\d{4}[-/]\d{1,2}[-/]\d{1,2}`), // 日期
	regexp.MustCompile(`\d{1,2}:\d{2}(:\d{2})?`),      // 时间
	regexp.MustCompile(`\b[a-f0-9]{32,}\b`),           // 哈希/UUID
	regexp.MustCompile(`\d{11,}`),                     // 长数字
}

// hasDynamicContent 判断文本是否包含动态片段
func hasDynamicContent(text string) bool {
	for _, pat := range dynamicPatterns {
		if pat.MatchString(text) {
			return true
		}
	}
	return false
}

// reorderDynamicContentLast 在不破坏 messages 顺序的前提下，把可重排消息
// 的 content 内部按段落分成 静态 || 动态 两部分，动态段落整体下沉到末尾。
func reorderDynamicContentLast(messages []interface{}) []interface{} {
	// 找到最后一条 user 消息的下标，仅它和所有 system 消息参与重排
	lastUserIdx := -1
	for i, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := m["role"].(string); role == "user" {
			lastUserIdx = i
		}
	}

	for i, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role != "system" && i != lastUserIdx {
			continue
		}
		content, isString := m["content"].(string)
		if !isString || content == "" {
			// 多模态 content（array）或空内容：跳过
			continue
		}
		reordered := reorderContentParagraphs(content)
		if reordered == content {
			continue
		}
		m["content"] = reordered
		messages[i] = m
	}
	return messages
}

// reorderContentParagraphs 按空行切段，含动态片段的段整体下沉到末尾，
// 静态段和动态段各自的相对顺序保留。
func reorderContentParagraphs(content string) string {
	// 用空行（\n\n）切段，单换行视为段内换行
	paragraphs := strings.Split(content, "\n\n")
	if len(paragraphs) < 2 {
		return content
	}
	var staticParts, dynamicParts []string
	hasDynamic := false
	for _, p := range paragraphs {
		if hasDynamicContent(p) {
			dynamicParts = append(dynamicParts, p)
			hasDynamic = true
		} else {
			staticParts = append(staticParts, p)
		}
	}
	if !hasDynamic || len(staticParts) == 0 {
		// 全静态或全动态：重排无收益
		return content
	}
	return strings.Join(append(staticParts, dynamicParts...), "\n\n")
}

// --- Token 压缩（无损规范化） ---
//
// 名为压缩，实为「确定性、零内容损失的规范化」：
//   - 折叠连续空白：多个空格/制表符 → 单空格；3+ 个换行 → 2 个换行
//   - 去除每行尾随空白
//   - 折叠重复 markdown 分隔线（连续多条 `---` / `***` → 1 条）
//
// 不做语义改写，不调上游 LLM；不再注入 `__compressed` 等非标字段
// （严格上游会因未知字段返回 400）。

var (
	tokenSpaceRun  = regexp.MustCompile(`[ \t]+`)
	tokenManyLF    = regexp.MustCompile(`\n{3,}`)
	tokenLineTrail = regexp.MustCompile(`[ \t]+\n`)
	tokenHRRun     = regexp.MustCompile(`(?m)^([-*]{3,})[ \t]*\n(?:[ \t]*[-*]{3,}[ \t]*\n){1,}`)
)

// normalizeWhitespace 对长 system content 做无损规范化，长度短于阈值则原样返回
func normalizeWhitespace(content string) string {
	out := tokenSpaceRun.ReplaceAllString(content, " ")
	out = tokenLineTrail.ReplaceAllString(out, "\n")
	out = tokenManyLF.ReplaceAllString(out, "\n\n")
	out = tokenHRRun.ReplaceAllString(out, "$1\n")
	return strings.TrimRight(out, " \n\t")
}

// compressSystemPrompts 仅对超长 system 消息触发规范化，短 prompt 不折腾
func compressSystemPrompts(messages []interface{}) []interface{} {
	const systemPromptThreshold = 4000

	for i, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := m["role"].(string); role != "system" {
			continue
		}
		content, isString := m["content"].(string)
		if !isString || len(content) < systemPromptThreshold {
			continue
		}
		normalized := normalizeWhitespace(content)
		if normalized == content {
			continue
		}
		m["content"] = normalized
		messages[i] = m
	}
	return messages
}

// --- 会话压缩 ---
//
// 目的：长会话发到上游会撞 token 上限 / 拖慢首字。压缩思路是：把早期对话用 LLM
// 摘要成一条 system 消息，仅保留最近 N 轮原文。
//
// 关键约束：
//  1. LLM 调用耗时秒级，不能阻塞用户请求 → 走「异步预热缓存」：第一次命中阈值时
//     直接放行原 messages，并在后台调 LLM 算摘要、按内容哈希入内存缓存；下一次
//     同样的历史前缀进来才用缓存里的摘要替换。冷启动一次，热路径零延迟。
//  2. 缓存键用早期消息的内容哈希 —— 多轮对话只在尾部增长，前缀稳定，命中率高。
//  3. 缓存 miss 时返回原 messages 而不是降级为字符串拼接 —— 上游能吃下完整对话
//     总比拿到一个糟糕摘要好。
//  4. 不再注入 __session_summary / __compressed_count 字段（严格上游会因未知字段 400）。

const (
	sessionCompressThreshold = 10 // 超过这个轮数（含 system）才触发压缩判定
	sessionCompressKeepRecent = 5 // 保留最近 N 条非 system 消息原文
	sessionSummaryMaxEntries  = 256 // sync.Map 简易容量上限，超过则不再写入新条目
)

// 会话摘要缓存（key = sha256(canonical-json of toCompress)）
var (
	sessionSummaryCache    sync.Map // map[string]string
	sessionSummaryInflight sync.Map // map[string]struct{}，去重并发预热
	sessionSummaryEntries  int64    // atomic 计数，配合 sessionSummaryMaxEntries 控制写入
)

// compressSessionHistory 命中阈值时尝试用缓存的 LLM 摘要替换早期消息。
// 缓存未命中：原 messages 原样返回 + 后台预热一次。
func compressSessionHistory(messages []interface{}) []interface{} {
	if len(messages) <= sessionCompressThreshold {
		return messages
	}

	// 拆出 system 消息和可压缩消息（保持各自相对顺序）
	systemMsgs := make([]interface{}, 0)
	compressable := make([]interface{}, 0)
	for _, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			compressable = append(compressable, msg)
		}
	}
	if len(compressable) <= sessionCompressKeepRecent {
		return messages
	}

	toCompress := compressable[:len(compressable)-sessionCompressKeepRecent]
	toKeep := compressable[len(compressable)-sessionCompressKeepRecent:]

	key := hashMessages(toCompress)

	if cached, ok := sessionSummaryCache.Load(key); ok {
		summary, _ := cached.(string)
		if summary == "" {
			return messages
		}
		summaryMsg := map[string]interface{}{
			"role":    "system",
			"content": "[历史摘要] " + summary,
		}
		result := make([]interface{}, 0, len(systemMsgs)+1+len(toKeep))
		result = append(result, systemMsgs...)
		result = append(result, summaryMsg)
		result = append(result, toKeep...)
		return result
	}

	// 缓存 miss：异步预热（同一 key 已有 inflight 不重复触发）
	if _, loaded := sessionSummaryInflight.LoadOrStore(key, struct{}{}); !loaded {
		go prewarmSessionSummary(key, toCompress)
	}
	return messages
}

// hashMessages 对 toCompress 计算稳定哈希，作为摘要缓存键
func hashMessages(messages []interface{}) string {
	raw, err := json.Marshal(messages)
	if err != nil {
		// json.Marshal 对 map[string]interface{} 实际不会失败；兜底回退到长度 + 首尾内容
		return fmt.Sprintf("fallback:%d", len(messages))
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// prewarmSessionSummary 后台调 LLM 生成摘要并写入缓存
func prewarmSessionSummary(key string, messages []interface{}) {
	defer sessionSummaryInflight.Delete(key)

	if atomic.LoadInt64(&sessionSummaryEntries) >= sessionSummaryMaxEntries {
		LogWarn("[session-compress] cache full, skip prewarm key=%s", key[:12])
		return
	}

	prompt := buildSessionSummaryPrompt(messages)
	summary, err := callExtractionLLM(prompt)
	if err != nil {
		LogWarn("[session-compress] prewarm failed key=%s: %v", key[:12], err)
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	if _, loaded := sessionSummaryCache.LoadOrStore(key, summary); !loaded {
		atomic.AddInt64(&sessionSummaryEntries, 1)
		LogInfo("[session-compress] cached summary key=%s len=%d msgs=%d", key[:12], len(summary), len(messages))
	}
}

// buildSessionSummaryPrompt 把对话历史拼成发给摘要 LLM 的 prompt
func buildSessionSummaryPrompt(messages []interface{}) string {
	var b strings.Builder
	b.WriteString("请用不超过 200 字的中文，提炼以下多轮对话的核心信息（保留用户的关键问题、已确认的事实/决定、未解决的问题），用于替代原始上下文。直接输出摘要正文，不要任何前后缀：\n\n")
	for _, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if content == "" || role == "" {
			continue
		}
		b.WriteString(role)
		b.WriteString(": ")
		// 单条裁掉过长内容，避免 prompt 爆炸
		if len(content) > 2000 {
			b.WriteString(content[:2000])
			b.WriteString("……")
		} else {
			b.WriteString(content)
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// ReloadFeatures 热刷新特性开关（由 reload 调用）
func ReloadFeatures() {
	LoadFeatureToggles()
	ReloadComplexityConfig()
}

// --- Admin handler ---

// handleAdminFeatures 管理接口：查询/刷新特性开关
func handleAdminFeatures(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]interface{}{
			"dynamic_content_last": FeatureToggles.DynamicContentLast,
			"token_compression":    FeatureToggles.TokenCompression,
			"session_compression":  FeatureToggles.SessionCompression,
		})

	case http.MethodPost:
		LoadFeatureToggles()
		writeJSON(w, 200, map[string]interface{}{
			"message":   "Feature toggles reloaded",
			"timestamp": time.Now().UTC(),
			"features": map[string]interface{}{
				"dynamic_content_last": FeatureToggles.DynamicContentLast,
				"token_compression":    FeatureToggles.TokenCompression,
				"session_compression":  FeatureToggles.SessionCompression,
			},
		})

	default:
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
	}
}
