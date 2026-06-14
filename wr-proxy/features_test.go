package main

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------- dynamic_content_last ----------

func TestReorderContentParagraphs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single paragraph - no change",
			in:   "static only line",
			want: "static only line",
		},
		{
			name: "all static - no change",
			in:   "para a\n\npara b\n\npara c",
			want: "para a\n\npara b\n\npara c",
		},
		{
			name: "all dynamic - no change (no benefit)",
			in:   "see https://a.com\n\nat 12:34:56\n\nuuid 1234567890abcdef1234567890abcdef",
			want: "see https://a.com\n\nat 12:34:56\n\nuuid 1234567890abcdef1234567890abcdef",
		},
		{
			name: "mixed - dynamic sinks to tail",
			in:   "rules of engagement\n\ncurrent time 2026-06-14\n\nbe polite",
			want: "rules of engagement\n\nbe polite\n\ncurrent time 2026-06-14",
		},
		{
			name: "preserve relative order within static and dynamic",
			in:   "static A\n\ndynamic 2026-01-01\n\nstatic B\n\ndynamic https://x.io\n\nstatic C",
			want: "static A\n\nstatic B\n\nstatic C\n\ndynamic 2026-01-01\n\ndynamic https://x.io",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := reorderContentParagraphs(c.in)
			if got != c.want {
				t.Fatalf("reorderContentParagraphs:\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

func TestReorderDynamicContentLast_PreservesMessageOrder(t *testing.T) {
	// 关键：messages[] 的相对顺序绝对不能动（破坏 prompt cache 前缀）
	messages := []interface{}{
		map[string]interface{}{"role": "system", "content": "rules\n\ncurrent time 2026-06-14"},
		map[string]interface{}{"role": "user", "content": "first question"},
		map[string]interface{}{"role": "assistant", "content": "first answer at 12:34"},
		map[string]interface{}{"role": "user", "content": "follow up\n\nat 13:45:00"},
	}

	out := reorderDynamicContentLast(messages)

	// 顺序保持
	if len(out) != 4 {
		t.Fatalf("len got %d want 4", len(out))
	}
	roles := make([]string, len(out))
	for i, m := range out {
		roles[i] = m.(map[string]interface{})["role"].(string)
	}
	wantRoles := []string{"system", "user", "assistant", "user"}
	for i := range wantRoles {
		if roles[i] != wantRoles[i] {
			t.Fatalf("role order changed: got %v want %v", roles, wantRoles)
		}
	}

	// system 内容动态后置
	sysContent := out[0].(map[string]interface{})["content"].(string)
	if !strings.HasPrefix(sysContent, "rules") || !strings.HasSuffix(sysContent, "current time 2026-06-14") {
		t.Fatalf("system not reordered: %q", sysContent)
	}

	// 中间 user / assistant 不动（不是最后一个 user，也不是 system）
	if out[1].(map[string]interface{})["content"].(string) != "first question" {
		t.Fatalf("non-last user got mutated")
	}
	if out[2].(map[string]interface{})["content"].(string) != "first answer at 12:34" {
		t.Fatalf("assistant got mutated")
	}

	// 最后一个 user 内容动态后置
	lastUser := out[3].(map[string]interface{})["content"].(string)
	if !strings.HasPrefix(lastUser, "follow up") || !strings.HasSuffix(lastUser, "at 13:45:00") {
		t.Fatalf("last user not reordered: %q", lastUser)
	}
}

func TestReorderDynamicContentLast_MultimodalSkipped(t *testing.T) {
	// 多模态 content 是 array，不应该被处理
	mm := []interface{}{
		map[string]interface{}{"type": "text", "text": "see https://x.com"},
		map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:..."}},
	}
	messages := []interface{}{
		map[string]interface{}{"role": "user", "content": mm},
	}

	out := reorderDynamicContentLast(messages)

	gotContent := out[0].(map[string]interface{})["content"]
	if _, isArr := gotContent.([]interface{}); !isArr {
		t.Fatalf("multimodal content type changed: %T", gotContent)
	}
}

// ---------- token_compression ----------

func TestNormalizeWhitespace(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "collapse spaces and tabs",
			in:   "a    b\t\tc   d",
			want: "a b c d",
		},
		{
			name: "trailing whitespace removed",
			in:   "line one   \nline two\t\n",
			want: "line one\nline two",
		},
		{
			name: "3+ newlines folded to 2",
			in:   "a\n\n\n\nb",
			want: "a\n\nb",
		},
		{
			name: "consecutive hr collapsed",
			in:   "intro\n\n---\n---\n---\n\nbody",
			want: "intro\n\n---\n\nbody",
		},
		{
			name: "no change for already-clean text",
			in:   "alpha\n\nbeta\n\ngamma",
			want: "alpha\n\nbeta\n\ngamma",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeWhitespace(c.in)
			if got != c.want {
				t.Fatalf("normalizeWhitespace:\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

func TestCompressSystemPrompts_ThresholdAndIdempotent(t *testing.T) {
	short := "short    system    prompt"
	long := strings.Repeat("paragraph    with    extra    spaces.\n\n\n\n", 200) // 远超 4000

	messages := []interface{}{
		map[string]interface{}{"role": "system", "content": short},
		map[string]interface{}{"role": "system", "content": long},
		map[string]interface{}{"role": "user", "content": "user    extra    spaces    here"},
	}

	out := compressSystemPrompts(messages)

	// 短 system 不动
	if got := out[0].(map[string]interface{})["content"].(string); got != short {
		t.Fatalf("short system mutated: %q", got)
	}
	// 长 system 被规范化
	got := out[1].(map[string]interface{})["content"].(string)
	if got == long {
		t.Fatalf("long system not normalized")
	}
	if strings.Contains(got, "    ") {
		t.Fatalf("long system still has multi-space runs: %q", got[:80])
	}
	if strings.Contains(got, "\n\n\n") {
		t.Fatalf("long system still has 3+ newlines")
	}
	// user 不被规范化
	if got := out[2].(map[string]interface{})["content"].(string); got != "user    extra    spaces    here" {
		t.Fatalf("user mutated: %q", got)
	}

	// 没有非标字段
	for _, m := range out {
		mm := m.(map[string]interface{})
		if _, has := mm["__compressed"]; has {
			t.Fatalf("__compressed field leaked")
		}
	}

	// 幂等：再来一次结果一致
	out2 := compressSystemPrompts(out)
	if out2[1].(map[string]interface{})["content"].(string) != got {
		t.Fatalf("normalize not idempotent")
	}
}

// ---------- session_compression ----------

// resetSessionCache 用于测试间清空摘要缓存
func resetSessionCache() {
	sessionSummaryCache.Range(func(k, _ interface{}) bool {
		sessionSummaryCache.Delete(k)
		return true
	})
	sessionSummaryInflight.Range(func(k, _ interface{}) bool {
		sessionSummaryInflight.Delete(k)
		return true
	})
	atomic.StoreInt64(&sessionSummaryEntries, 0)
}

func makeMsg(role, content string) interface{} {
	return map[string]interface{}{"role": role, "content": content}
}

func buildLongHistory(n int) []interface{} {
	msgs := []interface{}{makeMsg("system", "system rule")}
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs = append(msgs, makeMsg(role, "round "+string(rune('A'+i%26))))
	}
	return msgs
}

func TestCompressSessionHistory_BelowThresholdNoOp(t *testing.T) {
	resetSessionCache()
	short := buildLongHistory(5) // 1 system + 5 = 6, 远低于阈值 10
	out := compressSessionHistory(short)
	if len(out) != len(short) {
		t.Fatalf("below threshold mutated: %d -> %d", len(short), len(out))
	}
}

func TestCompressSessionHistory_CacheMissReturnsOriginal(t *testing.T) {
	resetSessionCache()
	long := buildLongHistory(20) // 1 system + 20，远超阈值

	out := compressSessionHistory(long)

	// 第一次：cache miss → 应原样返回（异步预热在后台跑，但同步路径不替换）
	if len(out) != len(long) {
		t.Fatalf("cache miss should return original, got len=%d want %d", len(out), len(long))
	}

	// 应有 inflight 标记
	count := 0
	sessionSummaryInflight.Range(func(_, _ interface{}) bool { count++; return true })
	if count != 1 {
		t.Fatalf("expected 1 inflight entry, got %d", count)
	}

	// 重置 inflight，避免 goroutine 真的去打 LLM 报错（无 provider）
	resetSessionCache()
}

func TestCompressSessionHistory_CacheHitReplaces(t *testing.T) {
	resetSessionCache()
	long := buildLongHistory(20)

	// 复用生产代码的拆分逻辑，确保 key 一致
	compressable := make([]interface{}, 0)
	systemMsgs := make([]interface{}, 0)
	for _, msg := range long {
		m := msg.(map[string]interface{})
		if m["role"] == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			compressable = append(compressable, msg)
		}
	}
	toCompress := compressable[:len(compressable)-sessionCompressKeepRecent]
	toKeep := compressable[len(compressable)-sessionCompressKeepRecent:]
	key := hashMessages(toCompress)

	// 预填缓存
	sessionSummaryCache.Store(key, "fake LLM summary")

	out := compressSessionHistory(long)

	// 期望长度：systemMsgs + 1 摘要 + toKeep
	wantLen := len(systemMsgs) + 1 + len(toKeep)
	if len(out) != wantLen {
		t.Fatalf("len got %d want %d", len(out), wantLen)
	}

	// 摘要消息应当紧跟在 system 后
	summaryMsg := out[len(systemMsgs)].(map[string]interface{})
	if r := summaryMsg["role"].(string); r != "system" {
		t.Fatalf("summary role got %q want system", r)
	}
	if c := summaryMsg["content"].(string); !strings.Contains(c, "fake LLM summary") || !strings.HasPrefix(c, "[历史摘要]") {
		t.Fatalf("summary content got %q", c)
	}

	// 不应注入非标字段
	if _, has := summaryMsg["__session_summary"]; has {
		t.Fatalf("__session_summary field leaked")
	}
	if _, has := summaryMsg["__compressed_count"]; has {
		t.Fatalf("__compressed_count field leaked")
	}

	// 序列化校验：能过严格 JSON 编码（无非标字段）
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resetSessionCache()
}

func TestHashMessages_StableAndDistinct(t *testing.T) {
	a := []interface{}{makeMsg("user", "hello"), makeMsg("assistant", "hi")}
	b := []interface{}{makeMsg("user", "hello"), makeMsg("assistant", "hi")}
	c := []interface{}{makeMsg("user", "hello"), makeMsg("assistant", "yo")}

	if hashMessages(a) != hashMessages(b) {
		t.Fatalf("equal content should hash equal")
	}
	if hashMessages(a) == hashMessages(c) {
		t.Fatalf("different content should hash differently")
	}
}

func TestPrewarmSessionSummary_InflightDedup(t *testing.T) {
	// 不真的调 LLM —— 模拟两次并发触发同一 key，验证只有一个 goroutine 进入预热
	// 通过手动 LoadOrStore 模拟生产路径
	resetSessionCache()
	key := "test-key"

	var calls int32
	var wg sync.WaitGroup
	wg.Add(2)

	tryFire := func() {
		defer wg.Done()
		if _, loaded := sessionSummaryInflight.LoadOrStore(key, struct{}{}); !loaded {
			atomic.AddInt32(&calls, 1)
		}
	}
	go tryFire()
	go tryFire()
	wg.Wait()

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected exactly 1 prewarm fire, got %d", calls)
	}

	resetSessionCache()
}
