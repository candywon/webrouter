# 05 - 请求校验与清洗 (sanitizer.go)

## 核心职责
1. 校验请求是否符合 OpenAI Chat Completions API 规范
2. 补全缺失的必要字段（编程工具发的请求可能缺字段）
3. 根据 Provider 能力剥离不支持的字段（如 tools/function_calling）
4. 防止因发送不支持的功能字段导致厂商封号或中断服务

## SanitizeResult
```
type SanitizeResult struct {
    Body         []byte    // 清洗后的请求体
    Warnings     []string  // 警告信息
    Stripped     []string  // 被剥离的字段列表
    Modified     bool      // 请求体是否被修改
    Valid        bool      // 请求是否合法
    RejectReason string    // 拒绝原因（空=不拒绝）
}
```

## SanitizeRequest 主函数

```
输入: provider, endpoint, body
输出: *SanitizeResult
```

### 仅对 /v1/chat/completions 做完整校验，其他endpoint直接返回

### 流程

```
1. JSON解析
   失败 → Valid=false, RejectReason="invalid JSON"

2. validateRequiredFields(req, result)
   - model 必须存在
   - messages 必须存在
   - messages 必须是数组
   - messages 不能为空

3. sanitizeMessages(req, result)
   - 遍历每条message:
     a. role 缺失 → 推断(inferRole)
        - 第1条且无system → "system"
        - 其余 → "user"
     b. content 缺失且role!=tool → 设为 ""
     c. role=tool 但缺 tool_call_id → 设为 "auto"
     d. role=assistant 且有 tool_calls → 检查每个tool_call:
        - id 缺失 → 生成 "call_<index>"
        - type 缺失 → 设为 "function"
        - function.name 缺失 → 设为 "unknown"
        - function.arguments 缺失 → 设为 "{}"
   - 如果第一条不是system → 插入空system prompt
   - Modified=true

4. sanitizeToolFields(provider, req, result)
   如果 !provider.SupportsTools:
     a. 删除 "tools" 字段 → Stripped append "tools"
     b. 删除 "functions" 字段 → Stripped append "functions"
     c. 删除 "tool_choice" 字段 → Stripped append "tool_choice"
     d. 删除 "function_call" 字段 → Stripped append "function_call"
     e. sanitizeToolMessages: 清理messages中role=tool的内容
        - role=tool → 改为 role=assistant, content改为提示文本
        - assistant的tool_calls → 删除tool_calls, 保留content
     f. Modified=true

5. enhanceDefaults(req, result)
   - stream 缺失 → 设为 false
   - temperature 缺失 → 设为 0.7
   - Modified=true (如有补全)

6. 序列化
   Modified=true → json.Marshal(req) → result.Body
   Modified=false → result.Body = 原始body
```

## inferRole 推断角色

```
index=0 且 无system消息 → "system"
其他 → "user"
```

## HasToolCallRequest(body []byte) bool

检测请求体中是否包含 tools 或 functions 字段（供外部模块判断）
