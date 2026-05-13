package main

// 通用工具函数

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// --- 日志 ---

var (
	infoLog  = log.New(os.Stdout, "[INFO]  ", log.LstdFlags|log.Lmsgprefix)
	warnLog  = log.New(os.Stderr, "[WARN]  ", log.LstdFlags|log.Lmsgprefix)
	errorLog = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix)
)

func LogInfo(format string, args ...interface{}) {
	infoLog.Printf(format, args...)
}

func LogWarn(format string, args ...interface{}) {
	warnLog.Printf(format, args...)
}

func LogError(format string, args ...interface{}) {
	errorLog.Printf(format, args...)
}

// --- JSON 工具 ---

// containsJSONString 检查 JSON 数组字符串中是否包含某个值
// 例如: containsJSONString(`["gpt-4o","claude-3"]`, "gpt-4o") → true
func containsJSONString(jsonArr, target string) bool {
	if jsonArr == "" || jsonArr == "[]" {
		return false
	}
	// 快速路径：字符串包含
	search := fmt.Sprintf(`"%s"`, target)
	return strings.Contains(jsonArr, search)
}

// parseJSONArray 解析 JSON 数组字符串
func parseJSONArray(jsonArr string) []string {
	if jsonArr == "" || jsonArr == "[]" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonArr), &result); err != nil {
		return nil
	}
	return result
}

// parseJSONFloat64 解析 JSON 数值
func parseJSONFloat64(data string) float64 {
	var f float64
	json.Unmarshal([]byte(data), &f)
	return f
}
