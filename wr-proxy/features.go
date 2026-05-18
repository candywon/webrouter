package main

// wr-proxy 优化特性开关
// 功能由 wr-proxy 在请求转发到上游之前自动处理请求体实现

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// FeatureToggles 优化特性开关状态
var FeatureToggles struct {
	DynamicContentLast bool // 动态内容后置
	TokenCompression   bool // Token 压缩（RTK）
	SessionCompression bool // 会话压缩
}

// LoadFeatureToggles 从 DB 加载特性开关状态
func LoadFeatureToggles() {
	FeatureToggles.DynamicContentLast = LoadSetting("feature_dynamic_content_last", false).(bool)
	FeatureToggles.TokenCompression = LoadSetting("feature_token_compression", false).(bool)
	FeatureToggles.SessionCompression = LoadSetting("feature_session_compression", false).(bool)

	LogInfo("Feature toggles: dynamic_content_last=%v, token_compression=%v, session_compression=%v",
		FeatureToggles.DynamicContentLast,
		FeatureToggles.TokenCompression,
		FeatureToggles.SessionCompression)
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

// 动态内容正则模式
var dynamicPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^\s\)]+`),        // URL
	regexp.MustCompile(`\d{4}[-/]\d{1,2}[-/]\d{1,2}`), // 日期
	regexp.MustCompile(`\d{1,2}:\d{2}(:\d{2})?`),  // 时间
	regexp.MustCompile(`\b[a-f0-9]{32,}\b`),        // 哈希/UUID
	regexp.MustCompile(`\d{11,}`),                  // 长数字
}

// hasDynamicContent 判断消息是否包含动态内容
func hasDynamicContent(text string) bool {
	for _, pat := range dynamicPatterns {
		if pat.MatchString(text) {
			return true
		}
	}
	return false
}

// reorderDynamicContentLast 将包含动态内容的消息移到同 role 组的末尾
func reorderDynamicContentLast(messages []interface{}) []interface{} {
	// 按 role 分组，把含动态内容的 message 移到该 role 组内最后
	type group struct {
		role     string
		messages []interface{}
	}
	var groups []group

	for _, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)

		// 找到或创建对应 role 的组
		idx := -1
		for i, g := range groups {
			if g.role == role {
				idx = i
				break
			}
		}
		if idx == -1 {
			groups = append(groups, group{role: role})
			idx = len(groups) - 1
		}

		if hasDynamicContent(content) {
			// 动态内容：追加到该 role 组末尾
			groups[idx].messages = append(groups[idx].messages, msg)
		} else {
			// 静态内容：插入到该 role 组开头
			groups[idx].messages = append([]interface{}{msg}, groups[idx].messages...)
		}
	}

	// 展平
	result := make([]interface{}, 0, len(messages))
	for _, g := range groups {
		result = append(result, g.messages...)
	}
	return result
}

// --- Token 压缩（简化版） ---

// compressSystemPrompts 对超长的 system prompt 进行简化标记
// 真正的压缩需要调用一次轻量模型做摘要，这里先做结构优化：
// 将长 system prompt 中的 instruction 部分提取为简短标记
func compressSystemPrompts(messages []interface{}) []interface{} {
	const systemPromptThreshold = 2000 // 超过 2000 字符的 system prompt 触发处理

	for i, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role != "system" {
			continue
		}
		content, _ := m["content"].(string)
		if len(content) < systemPromptThreshold {
			continue
		}

		// 简化策略：保留前 500 字符和后 200 字符，中间用标记替换
		// 这是对上游 prompt cache 的优化，减少重复发送的 token
		prefix := content[:min(500, len(content))]
		suffix := ""
		if len(content) > 700 {
			suffix = content[len(content)-200:]
		}

		compressed := fmt.Sprintf("%s\n\n[... %d 字符省略 ...]\n\n%s", prefix, len(content)-700, suffix)
		m["content"] = compressed
		m["__compressed"] = true
		m["__original_length"] = len(content)
		messages[i] = m
	}
	return messages
}

// --- 会话压缩 ---

// compressSessionHistory 当对话轮数过多时，压缩早期消息
func compressSessionHistory(messages []interface{}) []interface{} {
	const compressionThreshold = 10 // 超过 10 轮对话触发压缩
	const keepRecent = 5            // 保留最近 5 轮不压缩

	if len(messages) <= compressionThreshold {
		return messages
	}

	// 找到需要压缩的部分（除了最近 keepRecent 条之外的非 system 消息）
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

	if len(compressable) <= keepRecent {
		return messages
	}

	// 分离要压缩的和要保留的
	toCompress := compressable[:len(compressable)-keepRecent]
	toKeep := compressable[len(compressable)-keepRecent:]

	// 生成摘要
	summary := generateSessionSummary(toCompress)
	summaryMsg := map[string]interface{}{
		"role":    "system",
		"content": fmt.Sprintf("[会话摘要 - 压缩了 %d 条历史消息]\n%s", len(toCompress), summary),
		"__session_summary": true,
		"__compressed_count": len(toCompress),
	}

	// 重组：system messages + 摘要 + 最近消息
	result := make([]interface{}, 0, len(systemMsgs)+1+len(toKeep))
	result = append(result, systemMsgs...)
	result = append(result, summaryMsg)
	result = append(result, toKeep...)

	return result
}

// generateSessionSummary 生成会话摘要（简化版：提取关键信息）
func generateSessionSummary(messages []interface{}) string {
	var builder strings.Builder
	topics := make(map[string]int)
	totalLen := 0

	for _, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		totalLen += len(content)

		// 提取前 50 个字符作为话题关键词
		if role == "user" && len(content) > 0 {
			key := strings.TrimSpace(content[:min(50, len(content))])
			topics[key]++
		}
	}

	builder.WriteString(fmt.Sprintf("共 %d 轮对话，约 %d 字符。主要话题：", len(messages), totalLen))
	i := 0
	for topic := range topics {
		if i >= 5 {
			break
		}
		if i > 0 {
			builder.WriteString("、")
		}
		builder.WriteString(topic)
		i++
	}
	return builder.String()
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
