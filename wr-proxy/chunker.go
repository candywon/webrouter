// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ============================================================
// 文本分块 — 支持多种分块策略
// ============================================================

// ChunkConfig 分块配置
type ChunkConfig struct {
	Strategy string // fixed/sentence/paragraph
	Size     int    // 块大小（字符或 token 数）
	Overlap  int    // 块重叠大小
}

// DefaultChunkConfig 默认分块配置
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		Strategy: "paragraph",
		Size:     512,
		Overlap:  64,
	}
}

// Chunk 对文本按指定策略分块
func Chunk(text string, cfg ChunkConfig) []string {
	if text == "" {
		return nil
	}

	switch cfg.Strategy {
	case "fixed":
		return chunkFixed(text, cfg.Size, cfg.Overlap)
	case "sentence":
		return chunkSentence(text, cfg.Size, cfg.Overlap)
	case "paragraph":
		return chunkParagraph(text, cfg.Size, cfg.Overlap)
	default:
		return chunkParagraph(text, cfg.Size, cfg.Overlap)
	}
}

// chunkFixed 固定大小分块
func chunkFixed(text string, size, overlap int) []string {
	if size <= 0 {
		size = 512
	}
	if overlap >= size {
		overlap = size / 4
	}

	var chunks []string
	runes := []rune(text)
	start := 0

	for start < len(runes) {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
		start += size - overlap
		if start >= len(runes) {
			break
		}
	}
	return chunks
}

// chunkSentence 按句子边界分块
func chunkSentence(text string, size, overlap int) []string {
	sentences := splitSentences(text)
	return mergeChunks(sentences, size, overlap)
}

// chunkParagraph 按段落边界分块
func chunkParagraph(text string, size, overlap int) []string {
	paragraphs := strings.Split(text, "\n\n")
	// 过滤空段落
	var nonEmpty []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return mergeChunks(nonEmpty, size, overlap)
}

// splitSentences 将文本按句号拆分为句子
func splitSentences(text string) []string {
	var sentences []string
	var buf strings.Builder

	for _, r := range text {
		buf.WriteRune(r)
		if r == '。' || r == '.' || r == '!' || r == '？' || r == '\n' {
			s := strings.TrimSpace(buf.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// mergeChunks 将片段合并为不超过 size 的块
func mergeChunks(segments []string, size, overlap int) []string {
	if len(segments) == 0 {
		return nil
	}

	var chunks []string
	var current strings.Builder

	for _, seg := range segments {
		segLen := utf8.RuneCountInString(seg)
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+segLen > size {
			chunks = append(chunks, current.String())
			current.Reset()

			// 保留重叠（从上一块末尾取 overlap chars）
			if overlap > 0 && len(chunks) > 0 {
				prev := chunks[len(chunks)-1]
				prevRunes := []rune(prev)
				overlapStart := len(prevRunes) - overlap
				if overlapStart < 0 {
					overlapStart = 0
				}
				current.WriteString(string(prevRunes[overlapStart:]))
			}
		}
		current.WriteString(seg)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// CountTokens 简单估算字符数（近似 token 数 * 2）
func CountTokens(text string) int {
	count := 0
	for _, r := range text {
		if r > unicode.MaxASCII {
			count += 2 // 非 ASCII 字符算 2 个 token
		} else {
			count++
		}
	}
	return count / 2 // 粗略估算
}
