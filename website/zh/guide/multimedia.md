---
title: 多媒体支持
description: 通过 wr-proxy 处理音频、图片、视频内容
---

# 多媒体支持

## 概述

wr-proxy 为多媒体 API 端点提供一流支持 — 音频合成、语音转文字、图片生成和图片编辑 — 全部通过统一网关路由，具备自动故障切换和 Provider 选择能力。

## 支持的端点

| 端点 | 方法 | 内容类型 | 说明 |
|------|------|----------|------|
| `/v1/audio/speech` | POST | JSON → 二进制音频 | 文字转语音合成 |
| `/v1/audio/transcriptions` | POST | Multipart → JSON | 语音转文字 |
| `/v1/images/generations` | POST | JSON → JSON | 图片生成（DALL·E 等） |
| `/v1/images/edits` | POST | Multipart → JSON | 图片编辑 |
| `/v1/images/variations` | POST | Multipart → JSON | 图片变体 |

## 二进制内容处理方式

多媒体端点使用专用的快速通道（`handleBinaryProxy`），与标准聊天补全流程不同：

1. **认证** — 与聊天端点相同的 Token 认证
2. **模型提取** — 从 JSON body 或 multipart 表单解析
3. **Provider 选择** — 使用相同的健康感知路由器
4. **二进制透传** — 响应体直接流式传输，不进行 JSON 解析

### 跳过的步骤

二进制代理有意跳过以下步骤（不适用于二进制内容）：
- 复杂度评估（无文本可分析）
- 脱敏处理（音频/图片字节中无 PII）
- 请求清洗（格式因端点而异）
- 知识捕获（二进制内容不可捕获）

### 保留的能力

- Token 认证和模型白名单检查
- Provider 健康感知选择与故障切换
- 冷却机制（额度耗尽、限流）
- 请求日志和成本计量
- 响应 Content-Type 透传

## 音频合成（TTS）

将文字转为语音，可使用任意 Provider 提供的 TTS 模型。

```bash
curl http://localhost:5051/v1/audio/speech \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tts-1",
    "input": "你好，欢迎使用 WebRouter！",
    "voice": "alloy"
  }' \
  --output speech.mp3
```

## 语音转文字（STT）

将音频文件转为文字，支持 multipart 表单上传。

```bash
curl http://localhost:5051/v1/audio/transcriptions \
  -H "Authorization: Bearer <token>" \
  -F "file=@recording.mp3" \
  -F "model=whisper-1"
```

## 图片生成

通过任意 OpenAI 兼容的 Provider 生成图片。

```bash
curl http://localhost:5051/v1/images/generations \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dall-e-3",
    "prompt": "日落时分的宁静山湖",
    "n": 1,
    "size": "1024x1024"
  }'
```

## Provider 兼容性

任何提供 OpenAI 兼容端点的 Provider 均支持：

| Provider | TTS | STT | 图片生成 |
|----------|-----|-----|----------|
| OpenAI | tts-1, tts-1-hd | whisper-1 | dall-e-3 |
| Azure OpenAI | 支持 | 支持 | 支持 |
| 自定义（OpenAI 兼容）| 支持 | 支持 | 支持 |

## 故障切换行为

多媒体请求遵循与聊天请求相同的故障切换逻辑：

1. 若所选 Provider 失败，wr-proxy 尝试下一个拥有相同模型的健康 Provider
2. 被限流的 Provider 进入冷却（可配置时长）
3. 额度耗尽的 Provider 冷却 30 分钟
4. 通过 `max_failover` 和 `max_retry_count` 设置可配置
