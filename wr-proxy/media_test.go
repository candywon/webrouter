// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mediaTestHarness 多媒体代理测试环境
type mediaTestHarness struct {
	mockUpstream *httptest.Server // 模拟上游 API
	proxyServer  *httptest.Server // wr-proxy 实例
	dbPath       string
	providerID   int
	tokenKey     string
}

// setUpMediaTest 搭建测试环境：临时 DB + mock upstream + proxy
func setUpMediaTest(t *testing.T) *mediaTestHarness {
	t.Helper()

	// 1. 模拟上游 API（记录接收到的请求用于断言）
	var lastMethod, lastPath, lastCT string
	var lastBody []byte
	var lastFormFiles map[string][]byte
	var upstreamResponseStatus int
	var upstreamResponseCT string
	var upstreamResponseBody []byte

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		lastCT = r.Header.Get("Content-Type")
		lastBody, _ = io.ReadAll(r.Body)

		// 解析 multipart 文件
		lastFormFiles = make(map[string][]byte)
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseMultipartForm(10 << 20); err == nil {
				for field, fhs := range r.MultipartForm.File {
					for _, fh := range fhs {
						f, _ := fh.Open()
						data, _ := io.ReadAll(f)
						f.Close()
						lastFormFiles[field] = data
					}
				}
			}
		}

		// 鉴权检查
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key-12345" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
			return
		}

		w.Header().Set("Content-Type", upstreamResponseCT)
		w.WriteHeader(upstreamResponseStatus)
		w.Write(upstreamResponseBody)
	}))

	// 2. 创建临时 SQLite 数据库
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_media.db")

	// 初始化 wr-proxy 全局配置（最小化）
	cfg = &Config{
		ListenAddr:             ":0",
		DBPath:                 dbPath,
		FlaskURL:               "http://localhost:5050",
		DefaultTimeout:         60 * time.Second,
		StreamTimeout:          180 * time.Second,
		MaxRetryCount:          0,
		MaxFailover:            0,
		IdleConnTimeout:        90 * time.Second,
		MaxIdleConns:           10,
		MaxBodySize:            10 << 20,
		RoutingStrategy:        "priority",
		QuotaWarnThreshold:     0.2,
		QuotaCriticalThreshold: 0.05,
		PredictionDays:         7,
		HealthCheckInterval:    5 * time.Minute,
		HealthTimeout:          15 * time.Second,
		AlertCooldown:          5 * time.Minute,
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	// 初始化必要模块
	proxySvc = NewProxyService()
	reqCache = &RequestCache{entries: make(map[string]*requestCacheEntry)}
	InitBuiltinPatterns()

	// 3. 插入测试 Token
	tokenKey := "sk-wr-test-media-001"
	_, err := db.Exec(`INSERT INTO wr_tokens (name, key, models, provider_ids, quota_total, quota_used, enabled, created_at)
		VALUES ('test-token', ?, '["tts-1","whisper-1","dall-e-3","gpt-4o"]', '[]', 0, 0, 1, datetime('now'))`, tokenKey)
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}

	// 4. 插入测试 Provider（指向上游 mock）
	result, err := db.Exec(`INSERT INTO wr_providers
		(name, type, base_url, api_key, models, priority, weight, proxy_enabled, enabled, status, timeout_seconds, max_retries, cost_multiplier)
		VALUES ('test-media', 'direct', ?, 'test-api-key-12345', '["tts-1","whisper-1","dall-e-3","gpt-4o"]', 90, 50, 1, 1, 'healthy', 60, 2, 1.0)`,
		mockUpstream.URL)
	if err != nil {
		t.Fatalf("insert provider: %v", err)
	}
	id, _ := result.LastInsertId()
	providerID := int(id)

	// 5. 重新加载 Provider
	if err := reloadProviders(); err != nil {
		t.Fatalf("reloadProviders: %v", err)
	}

	// 6. 创建 proxy HTTP 测试服务器
	mux := http.NewServeMux()
	RegisterHandlers(mux)
	proxyServer := httptest.NewServer(corsMiddleware(mux))

	h := &mediaTestHarness{
		mockUpstream: mockUpstream,
		proxyServer:  proxyServer,
		dbPath:       dbPath,
		providerID:   providerID,
		tokenKey:     tokenKey,
	}

	// 将测试辅助变量通过闭包暴露
	t.Cleanup(func() {
		mockUpstream.Close()
		proxyServer.Close()
		db.Close()
	})

	// 每次子测试前重置 mock upstream 状态
	_ = h
	_ = lastMethod
	_ = lastPath
	_ = lastCT
	_ = lastBody
	_ = lastFormFiles
	_ = upstreamResponseStatus
	_ = upstreamResponseCT
	_ = upstreamResponseBody

	return h
}

// setMockResponse 配置 mock upstream 的响应
func setMockResponse(status int, ct string, body []byte) {
	// 通过重新创建 handler 来设置 —— 改用包级变量
}

// 为测试用的可变 mock 状态
var (
	mockStatus  int
	mockCT      string
	mockBody    []byte
	mockRequest struct {
		method    string
		path      string
		ct        string
		body      []byte
		auth      string
		formFiles map[string][]byte
	}
)

// TestAudioSpeech 测试音频合成端点
func TestAudioSpeech(t *testing.T) {
	// 创建 mock upstream 返回假 MP3 数据
	fakeMP3 := make([]byte, 1024)
	for i := range fakeMP3 {
		fakeMP3[i] = byte(i % 256)
	}
	// MP3 魔数
	copy(fakeMP3[:3], []byte{0xFF, 0xFB, 0x90})

	mockStatus = 200

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockRequest.method = r.Method
		mockRequest.path = r.URL.Path
		mockRequest.ct = r.Header.Get("Content-Type")
		mockRequest.body, _ = io.ReadAll(r.Body)
		mockRequest.auth = r.Header.Get("Authorization")

		if mockRequest.auth != "Bearer test-api-key-12345" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if r.URL.Path != "/v1/audio/speech" {
			w.WriteHeader(404)
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fakeMP3)))
		w.WriteHeader(mockStatus)
		w.Write(fakeMP3)
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	// 发送 speech 请求
	reqBody := `{"model":"tts-1","input":"Hello world","voice":"alloy"}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/audio/speech", reqBody, "application/json")

	// 验证
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "audio/mpeg" {
		t.Errorf("Content-Type = %q, want audio/mpeg", resp.Header.Get("Content-Type"))
	}
	if len(resp.Body) != len(fakeMP3) {
		t.Errorf("body length = %d, want %d", len(resp.Body), len(fakeMP3))
	}
	if !bytes.Equal(resp.Body[:3], []byte{0xFF, 0xFB, 0x90}) {
		t.Error("not valid MP3 (missing magic bytes)")
	}

	// 验证上游收到的请求
	if mockRequest.method != "POST" {
		t.Errorf("upstream method = %q, want POST", mockRequest.method)
	}
	if mockRequest.path != "/v1/audio/speech" {
		t.Errorf("upstream path = %q, want /v1/audio/speech", mockRequest.path)
	}
	if mockRequest.auth != "Bearer test-api-key-12345" {
		t.Errorf("upstream auth = %q, want Bearer test-api-key-12345", mockRequest.auth)
	}

	t.Log("Audio speech: PASS (binary audio/mpeg forwarded correctly)")
}

// TestAudioSpeechError 测试音频合成错误响应
func TestAudioSpeechError(t *testing.T) {
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":{"message":"Invalid voice","type":"invalid_request_error"}}`))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	reqBody := `{"model":"tts-1","input":"test","voice":"invalid_voice"}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/audio/speech", reqBody, "application/json")

	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	t.Log("Audio speech error: PASS (upstream error forwarded correctly)")
}

// TestAudioTranscription 测试音频转文字端点（multipart）
func TestAudioTranscription(t *testing.T) {
	fakeWAV := make([]byte, 512)
	copy(fakeWAV[:4], []byte("RIFF"))
	copy(fakeWAV[8:12], []byte("WAVE"))

	transcriptJSON := `{"text":"Hello world, this is a transcription test."}`

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockRequest.method = r.Method
		mockRequest.path = r.URL.Path
		mockRequest.ct = r.Header.Get("Content-Type")
		mockRequest.auth = r.Header.Get("Authorization")
		mockRequest.formFiles = make(map[string][]byte)

		if err := r.ParseMultipartForm(10 << 20); err == nil {
			for field, fhs := range r.MultipartForm.File {
				for _, fh := range fhs {
					f, _ := fh.Open()
					data, _ := io.ReadAll(f)
					f.Close()
					mockRequest.formFiles[field] = data
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(transcriptJSON))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	// 构建 multipart 请求
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("model", "whisper-1")
	writer.WriteField("language", "en")
	fw, _ := writer.CreateFormFile("file", "test.wav")
	fw.Write(fakeWAV)
	writer.Close()

	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/audio/transcriptions", buf.String(), writer.FormDataContentType())

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (body: %s)", resp.StatusCode, string(resp.Body))
	}
	if string(resp.Body) != transcriptJSON {
		t.Errorf("body = %q, want %q", string(resp.Body), transcriptJSON)
	}

	// 验证 multipart field name 保留正确（修复的 bug）
	if _, ok := mockRequest.formFiles["file"]; !ok {
		t.Errorf("upstream received form field %q, want 'file'. formFiles: %v",
			firstKey(mockRequest.formFiles), mockRequest.formFiles)
	}
	if !bytes.Equal(mockRequest.formFiles["file"], fakeWAV) {
		t.Error("upstream received corrupted file data")
	}

	t.Log("Audio transcription: PASS (multipart file field preserved correctly)")
}

// TestImageGenerations 测试图片生成端点（JSON → JSON，含 URL）
func TestImageGenerations(t *testing.T) {
	imageResponse := `{
		"created": 1234567890,
		"data": [
			{"url": "https://example.com/gen-image.png", "revised_prompt": "A beautiful landscape"}
		]
	}`

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockRequest.method = r.Method
		mockRequest.path = r.URL.Path
		mockRequest.ct = r.Header.Get("Content-Type")
		mockRequest.body, _ = io.ReadAll(r.Body)
		mockRequest.auth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(imageResponse))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	reqBody := `{"model":"dall-e-3","prompt":"A beautiful landscape","n":1,"size":"1024x1024"}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/images/generations", reqBody, "application/json")

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (body: %s)", resp.StatusCode, string(resp.Body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	data := result["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("data length = %d, want 1", len(data))
	}

	// 验证上游收到正确路径
	if mockRequest.path != "/v1/images/generations" {
		t.Errorf("upstream path = %q, want /v1/images/generations", mockRequest.path)
	}

	t.Log("Image generations: PASS (JSON → JSON via handleProxy)")
}

// TestImageGenerationsStream 测试图片生成（某些 API 用 SSE 返回进度）
func TestImageGenerationsStream(t *testing.T) {
	// 注意：/v1/images/generations 走 handleProxy，stream=true 时走 SSE
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"progress\":50}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"progress\":100,\"data\":[{\"url\":\"https://example.com/img.png\"}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	reqBody := `{"model":"dall-e-3","prompt":"test","stream":true}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/images/generations", reqBody, "application/json")

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(resp.Body), "data: [DONE]") {
		t.Error("streaming response missing [DONE] marker")
	}
	if !strings.Contains(string(resp.Body), "progress") {
		t.Error("streaming response missing progress events")
	}

	t.Log("Image generations stream: PASS (SSE streaming forwarded correctly)")
}

// TestImageEdits 测试图片编辑端点（multipart）
func TestImageEdits(t *testing.T) {
	fakePNG := make([]byte, 256)
	copy(fakePNG[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) // PNG 魔数

	editResponse := `{
		"created": 1234567890,
		"data": [{"url": "https://example.com/edited.png"}]
	}`

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockRequest.method = r.Method
		mockRequest.path = r.URL.Path
		mockRequest.ct = r.Header.Get("Content-Type")
		mockRequest.formFiles = make(map[string][]byte)

		if err := r.ParseMultipartForm(10 << 20); err == nil {
			for field, fhs := range r.MultipartForm.File {
				for _, fh := range fhs {
					f, _ := fh.Open()
					data, _ := io.ReadAll(f)
					f.Close()
					mockRequest.formFiles[field] = data
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(editResponse))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("model", "dall-e-3")
	writer.WriteField("prompt", "add a rainbow")
	fw, _ := writer.CreateFormFile("image", "test.png")
	fw.Write(fakePNG)
	writer.Close()

	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/images/edits", buf.String(), writer.FormDataContentType())

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (body: %s)", resp.StatusCode, string(resp.Body))
	}
	if _, ok := mockRequest.formFiles["image"]; !ok {
		t.Errorf("upstream received form field %q, want 'image'", firstKey(mockRequest.formFiles))
	}

	t.Log("Image edits: PASS (multipart image → JSON)")
}

// TestImageVariations 测试图片变体端点（multipart）
func TestImageVariations(t *testing.T) {
	fakePNG := make([]byte, 128)
	copy(fakePNG[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	varResponse := `{"created":1234567890,"data":[{"url":"https://example.com/variant.png"}]}`

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockRequest.method = r.Method
		mockRequest.path = r.URL.Path
		mockRequest.formFiles = make(map[string][]byte)

		if err := r.ParseMultipartForm(10 << 20); err == nil {
			for field, fhs := range r.MultipartForm.File {
				for _, fh := range fhs {
					f, _ := fh.Open()
					data, _ := io.ReadAll(f)
					f.Close()
					mockRequest.formFiles[field] = data
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(varResponse))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("model", "dall-e-3")
	fw, _ := writer.CreateFormFile("image", "source.png")
	fw.Write(fakePNG)
	writer.Close()

	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/images/variations", buf.String(), writer.FormDataContentType())

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if _, ok := mockRequest.formFiles["image"]; !ok {
		t.Errorf("upstream form field = %q, want 'image'", firstKey(mockRequest.formFiles))
	}

	t.Log("Image variations: PASS (multipart image → JSON)")
}

// TestVideoEndpointNotRegistered 验证视频端点尚未注册
func TestVideoEndpointNotRegistered(t *testing.T) {
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for unregistered video endpoint")
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	reqBody := `{"model":"sora-1","prompt":"a video of a cat"}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/video/generations", reqBody, "application/json")

	// 期望 404，因为 /v1/video/* 未注册
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404 (video endpoint not registered)", resp.StatusCode)
	}

	t.Log("Video endpoint: PASS (correctly returns 404, not registered)")
}

// TestVideoGenerationSimulated 模拟测试视频生成（通过 /v1/images/generations 模拟验证透传能力）
// 当未来添加 /v1/video/* 端点时，可直接参考此测试
func TestVideoGenerationSimulated(t *testing.T) {
	// 用图片生成端点模拟视频：验证 JSON body + model 提取 + auth + 透传均正常
	videoLikeResponse := `{
		"id": "vid_abc123",
		"object": "video",
		"status": "completed",
		"data": [{"url": "https://example.com/output.mp4"}]
	}`

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockRequest.method = r.Method
		mockRequest.path = r.URL.Path
		mockRequest.body, _ = io.ReadAll(r.Body)
		mockRequest.auth = r.Header.Get("Authorization")

		// 验证 JSON body 中的关键字段
		var req map[string]interface{}
		json.Unmarshal(mockRequest.body, &req)
		if req["model"] != "gpt-4o" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"unknown model"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(videoLikeResponse))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	// 用 chat/completions 模拟：完整 JSON 透传链路
	reqBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"generate video"}]}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/chat/completions", reqBody, "application/json")

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (body: %s)", resp.StatusCode, string(resp.Body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["object"] != "video" {
		t.Errorf("object = %q, want 'video'", result["object"])
	}

	t.Log("Video simulation: PASS (JSON request/response forwarding verified)")
}

// TestLargeBinaryResponse 测试大二进制响应（模拟高清图片/长音频）
func TestLargeBinaryResponse(t *testing.T) {
	// 32KB 假图片数据
	largeImage := make([]byte, 32<<10)
	copy(largeImage[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(largeImage)))
		w.WriteHeader(200)
		w.Write(largeImage)
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	reqBody := `{"model":"tts-1","input":"` + strings.Repeat("hello ", 1000) + `","voice":"alloy"}`
	resp := doProxyRequest(t, proxyServer.URL, "POST", "/v1/audio/speech", reqBody, "application/json")

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if len(resp.Body) != len(largeImage) {
		t.Errorf("body length = %d, want %d", len(resp.Body), len(largeImage))
	}
	if !bytes.Equal(resp.Body, largeImage) {
		t.Error("large binary response corrupted")
	}

	t.Log("Large binary response: PASS (32KB image forwarded intact)")
}

// TestAuthRejection 测试未授权请求被拒绝
func TestAuthRejection(t *testing.T) {
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for unauthorized requests")
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	// 使用无效 token
	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/audio/speech",
		strings.NewReader(`{"model":"tts-1","input":"test"}`))
	req.Header.Set("Authorization", "Bearer sk-invalid-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}

	t.Log("Auth rejection: PASS (invalid token rejected)")
}

// TestModelWhitelist 测试模型白名单
func TestModelWhitelist(t *testing.T) {
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for unauthorized models")
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	// Token 白名单不含此模型
	reqBody := `{"model":"sora-1","input":"test"}`
	req, _ := http.NewRequest("POST", proxyServer.URL+"/v1/audio/speech",
		strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer sk-wr-test-media-001")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 403 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 403 (body: %s)", resp.StatusCode, string(body))
	}

	t.Log("Model whitelist: PASS (unauthorized model rejected)")
}

// ── 辅助函数 ──

type proxyResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// setupProxyForTest 创建测试 proxy 服务器并加载 provider
func setupProxyForTest(t *testing.T, upstreamURL string) *httptest.Server {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_proxy.db")

	cfg = &Config{
		ListenAddr:             ":0",
		DBPath:                 dbPath,
		FlaskURL:               "http://localhost:5050",
		DefaultTimeout:         60 * time.Second,
		StreamTimeout:          180 * time.Second,
		MaxRetryCount:          0,
		MaxFailover:            0,
		IdleConnTimeout:        90 * time.Second,
		MaxIdleConns:           10,
		MaxBodySize:            10 << 20,
		RoutingStrategy:        "priority",
		QuotaWarnThreshold:     0.2,
		QuotaCriticalThreshold: 0.05,
		PredictionDays:         7,
		HealthCheckInterval:    5 * time.Minute,
		HealthTimeout:          15 * time.Second,
		AlertCooldown:          5 * time.Minute,
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	proxySvc = NewProxyService()
	reqCache = &RequestCache{entries: make(map[string]*requestCacheEntry)}
	InitBuiltinPatterns()

	// 插入测试 Token
	_, err := db.Exec(`INSERT INTO wr_tokens (name, key, models, provider_ids, quota_total, quota_used, enabled, created_at)
		VALUES ('test', 'sk-wr-test-media-001', '["tts-1","whisper-1","dall-e-3","gpt-4o"]', '[]', 0, 0, 1, datetime('now'))`)
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}

	// 插入 Provider（基础字段）
	provResult, err := db.Exec(`INSERT INTO wr_providers
		(name, type, base_url, api_key, models, enabled, status)
		VALUES ('test-provider', 'direct', ?, 'test-api-key-12345', '["tts-1","whisper-1","dall-e-3","gpt-4o"]', 1, 'healthy')`,
		upstreamURL)
	if err != nil {
		t.Fatalf("insert provider: %v", err)
	}
	provID, _ := provResult.LastInsertId()

	// 插入扩展字段
	_, err = db.Exec(`INSERT INTO wr_provider_ext
		(provider_id, proxy_enabled, rate_limit_rpm, timeout_seconds, max_retries, cost_multiplier, priority, weight, supports_tools, fallback_enabled)
		VALUES (?, 1, 0, 60, 2, 1.0, 90, 50, 1, 1)`, provID)
	if err != nil {
		t.Fatalf("insert provider_ext: %v", err)
	}

	// 插入额度
	_, err = db.Exec(`INSERT INTO wr_provider_quota (provider_id, quota_total, quota_used, quota_source)
		VALUES (?, 0, 0, 'unknown')`, provID)
	if err != nil {
		t.Fatalf("insert provider_quota: %v", err)
	}

	if err := reloadProviders(); err != nil {
		t.Fatalf("reloadProviders: %v", err)
	}

	// 清空 mock 状态
	mockRequest = struct {
		method    string
		path      string
		ct        string
		body      []byte
		auth      string
		formFiles map[string][]byte
	}{}

	mux := http.NewServeMux()
	RegisterHandlers(mux)
	return httptest.NewServer(corsMiddleware(mux))
}

// doProxyRequest 向 proxy 发送请求并返回响应
func doProxyRequest(t *testing.T, proxyURL, method, path, body, contentType string) *proxyResponse {
	t.Helper()

	req, err := http.NewRequest(method, proxyURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer sk-wr-test-media-001")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return &proxyResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Header:     resp.Header,
	}
}

func firstKey(m map[string][]byte) string {
	for k := range m {
		return k
	}
	return "(none)"
}

// ── 单元测试：二进制 Content-Type 检测 ──

func TestIsBinaryContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"audio/mpeg", true},
		{"audio/wav", true},
		{"audio/ogg; codecs=opus", true},
		{"image/png", true},
		{"image/jpeg", true},
		{"image/webp", true},
		{"multipart/form-data; boundary=abc", true},
		{"video/mp4", true},
		{"video/webm", true},
		{"application/json", false},
		{"text/plain", false},
		{"application/octet-stream", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isBinaryContentType(tt.ct)
		if got != tt.want {
			t.Errorf("isBinaryContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

// TestModelFromForm 测试从 multipart form 提取 model
func TestModelFromForm(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("model", "whisper-1")
	w.WriteField("language", "en")
	w.Close()

	req := httptest.NewRequest("POST", "/v1/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.ParseMultipartForm(10 << 20)

	got := modelFromForm(req)
	if got != "whisper-1" {
		t.Errorf("modelFromForm = %q, want whisper-1", got)
	}
}

// TestModelFromJSON 测试从 JSON body 提取 model
func TestModelFromJSON(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{`{"model":"tts-1","input":"hello"}`, "tts-1"},
		{`{"prompt":"test"}`, ""},
		{`invalid`, ""},
	}
	for _, tt := range tests {
		got := modelFromJSON([]byte(tt.body))
		if got != tt.want {
			t.Errorf("modelFromJSON(%q) = %q, want %q", tt.body, got, tt.want)
		}
	}
}

// TestProxyEnabledCheck 测试代理开关
func TestProxyEnabledCheck(t *testing.T) {
	// 保存原值
	orig := ProxyEnabled
	defer func() { ProxyEnabled = orig }()

	// 关闭代理
	ProxyEnabled = false
	w := httptest.NewRecorder()
	if checkProxyEnabled(w) {
		t.Error("checkProxyEnabled should return false when proxy is disabled")
	}
	if w.Code != 503 {
		t.Errorf("status = %d, want 503", w.Code)
	}

	// 开启代理
	ProxyEnabled = true
	w2 := httptest.NewRecorder()
	if !checkProxyEnabled(w2) {
		t.Error("checkProxyEnabled should return true when proxy is enabled")
	}
}

// ── 综合：多媒体端点完整性检查 ──

func TestMediaEndpointsRegistered(t *testing.T) {
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockUpstream.Close()

	proxyServer := setupProxyForTest(t, mockUpstream.URL)
	defer proxyServer.Close()

	endpoints := []struct {
		method      string
		path        string
		contentType string
		body        string
		shouldWork  bool
		label       string
	}{
		{"POST", "/v1/audio/speech", "application/json", `{"model":"tts-1","input":"hi"}`, true, "audio/speech"},
		{"POST", "/v1/audio/transcriptions", "", "", true, "audio/transcriptions"}, // multipart, tested separately
		{"POST", "/v1/images/generations", "application/json", `{"model":"dall-e-3","prompt":"test"}`, true, "images/generations"},
		{"POST", "/v1/images/edits", "", "", true, "images/edits"},
		{"POST", "/v1/images/variations", "", "", true, "images/variations"},
		{"POST", "/v1/video/generations", "application/json", `{"model":"sora-1"}`, false, "video/generations (not registered)"},
		{"POST", "/v1/video/edits", "application/json", `{"model":"sora-1"}`, false, "video/edits (not registered)"},
	}

	for _, ep := range endpoints {
		t.Run(ep.label, func(t *testing.T) {
			var resp *proxyResponse
			if ep.path == "/v1/audio/transcriptions" || ep.path == "/v1/images/edits" || ep.path == "/v1/images/variations" {
				// multipart 端点用最小 form
				var buf bytes.Buffer
				w := multipart.NewWriter(&buf)
				w.WriteField("model", "whisper-1")
				fw, _ := w.CreateFormFile("file", "test.bin")
				fw.Write([]byte{0, 1, 2, 3})
				w.Close()
				resp = doProxyRequest(t, proxyServer.URL, "POST", ep.path, buf.String(), w.FormDataContentType())
			} else {
				resp = doProxyRequest(t, proxyServer.URL, ep.method, ep.path, ep.body, ep.contentType)
			}

			if ep.shouldWork && resp.StatusCode != 200 {
				t.Errorf("%s: status = %d, want 200 (body: %s)", ep.label, resp.StatusCode, string(resp.Body))
			}
			if !ep.shouldWork && resp.StatusCode == 200 {
				t.Errorf("%s: status = 200, want non-200 (endpoint should not exist)", ep.label)
			}
		})
	}

	t.Log("Media endpoint registration: PASS")
}

// TestMain 测试入口
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
