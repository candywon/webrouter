# WebRouter 智能调度引擎 — 设计文档

## 核心价值

用户最大的痛点：API 突然不能用了。
- OpenAI 余额用完 → 服务中断 → 业务停摆
- 某个 Provider 宕机 → 无人值守 → 半夜才发现
- 单一依赖 → 无备份 → 出问题就死

WebRouter 的回答：**智能调度 = 永不断线**

## 调度架构

```
请求进入
    │
    ▼
┌─────────────────────────────────────────────┐
│              Router 调度引擎                  │
│                                              │
│  ① 过滤池                                    │
│     ├─ enabled?                              │
│     ├─ healthy? (健康检测)                    │
│     ├─ 支持 model?                           │
│     ├─ Token 白名单?                         │
│     └─ 配额有余? ← 关键：预测性过滤           │
│                                              │
│  ② 分组                                      │
│     ├─ 优先组 (priority ≥ 50)  ← 主力        │
│     └─ 备用组 (priority < 50)  ← 热备        │
│                                              │
│  ③ 组内排序                                   │
│     ├─ 剩余额度比例 DESC (越充沛越优先)        │
│     ├─ 延迟 ASC (越快越优先)                  │
│     ├─ 成本 ASC (越便宜越优先)                │
│     └─ weight 加权随机                        │
│                                              │
│  ④ 选出 Provider → 转发                      │
│     ├─ 成功 → 记录                           │
│     └─ 失败 → 降级到下一个 (同组→备用组)       │
│                                              │
│  ⑤ 预警系统（后台持续运行）                    │
│     ├─ 额度预测: 按近7天用量推算耗尽时间       │
│     ├─ 红线预警: 额度<20% → 立即通知          │
│     ├─ 黄线预警: 预计3天内耗尽 → 提前通知      │
│     └─ 自动降级: 额度<5% → 从调度池移除       │
└─────────────────────────────────────────────┘
```

## Provider 分组模型

```python
# Provider 新增字段
priority = 0-100
  90-100: 主力 (Primary)     — 日常走这里
  50-89:  热备 (Hot Backup)   — 主力挂了秒切
  1-49:   冷备 (Cold Backup)  — 兜底，所有主力+热备都挂了才用
  0:      禁用

# 典型配置示例
Provider A: OpenAI 官方    priority=90  ← 主力
Provider B: OpenAI 聚合1   priority=80  ← 热备1
Provider C: OpenAI 聚合2   priority=70  ← 热备2  
Provider D: Azure OpenAI   priority=50  ← 兜底
```

## 额度预测引擎

```python
class QuotaPredictor:
    """预测 Provider 额度耗尽时间"""
    
    def predict_exhaustion(provider_id) -> dict:
        """
        基于近7天用量趋势，预测额度何时耗尽
        
        返回:
        {
            'provider_id': 1,
            'quota_remaining': 8500,        # 剩余额度(分)
            'daily_burn_rate': 1200,         # 日均消耗(分)
            'days_until_exhaust': 7.1,       # 预计天数
            'predicted_exhaust_date': '2024-01-20',  # 预计耗尽日
            'trend': 'increasing',           # increasing/stable/decreasing
            'confidence': 0.85,              # 预测置信度
        }
        """
        # 1. 取近7天每日用量
        # 2. 线性回归算日均消耗
        # 3. 趋势判断：加速/稳定/减速
        # 4. 推算耗尽日期
        # 5. 无额度信息(直连无余额API)→用健康检测+错误率代替
```

## 预警级别

```
🟢 正常      额度 > 50%，预测 > 7天
🟡 关注      额度 20-50%，或预测 3-7天耗尽
🟠 警告      额度 5-20%，或预测 1-3天耗尽
🔴 紧急      额度 < 5%，或预测 24小时内耗尽
⚫ 已耗尽    额度 = 0，或连续请求失败(402/429)
```

### 预警动作（可配置）

| 级别 | 通知 | 调度动作 |
|------|------|---------|
| 🟡 关注 | 仪表盘标记 | 正常调度，降低优先级 |
| 🟠 警告 | 推送通知 | 降级为热备，主力切到其他 |
| 🔴 紧急 | 即时通知 | 移出调度池，只用备份 |
| ⚫ 已耗尽 | 即时通知 | 完全移除，热备自动升级为主力 |

## 热插拔

```
场景1: 新增 Provider
  管理员 POST /api/providers → 立即参与调度
  无需重启，下一个请求即可路由到新 Provider

场景2: Provider 故障
  健康检测发现 dead → 自动移出调度池
  正在进行的请求 failover 到下一个
  恢复后自动回归调度池

场景3: Provider 额度耗尽
  预测引擎发现 → 提前预警 + 降级
  用户充值后 → 手动/自动恢复

场景4: 紧急切换
  管理员 PATCH /api/providers/:id {priority: 0} → 立即禁用
  或 PATCH {priority: 90} → 立即升级为主力
```

## 请求流转完整流程

```
客户端请求 → /v1/chat/completions
    │
    ├─ 1. 鉴权 Token (sk-wr-xxx)
    │     ├─ 无效 → 401
    │     ├─ 过期 → 401
    │     └─ 额度不足 → 429 "配额已用完，请联系管理员"
    │
    ├─ 2. 提取 model
    │
    ├─ 3. Router.select_provider(model, token)
    │     ├─ 过滤: enabled + healthy + model + quota
    │     ├─ 分组: 优先组 > 热备组 > 冷备组
    │     ├─ 组内: 按策略排序
    │     └─ 无可用 → 503 "所有 Provider 不可用"
    │
    ├─ 4. ProxyService.forward(provider, request)
    │     ├─ 构造上游请求 (替换 API Key)
    │     ├─ 超时控制 (provider.timeout_seconds)
    │     ├─ 流式/非流式 转发
    │     │
    │     ├─ 成功 → 5. 计量返回
    │     │
    │     └─ 失败 → 自动重试
    │           ├─ 同 Provider 重试 (max_retries 次)
    │           └─ 降级到下一个 Provider (回到步骤3)
    │
    ├─ 5. MeterService.record(...)
    │     ├─ 写 wr_request_logs
    │     ├─ 扣 Token 配额
    │     └─ 更新 Provider 用量缓存
    │
    └─ 6. 返回响应给客户端
          (客户端无感知，以为直接调的 OpenAI)
```

## 调度策略配置

```json
// 管理员可配置全局调度策略
{
  "routing_strategy": "smart",        // smart/priority/round_robin/least_latency/cost_first
  "failover_enabled": true,           // 失败自动降级
  "max_failover_attempts": 3,         // 最大降级次数
  "retry_on_error": true,             // 同 Provider 重试
  "max_retries": 2,                   // 同 Provider 最大重试
  "auto_disable_on_exhaust": true,    // 额度耗尽自动禁用
  "quota_warning_threshold": 0.2,     // 20% 预警
  "quota_critical_threshold": 0.05,   // 5% 紧急
  "prediction_days": 7,               // 预测用近7天数据
}
```

## 仪表盘展示

### Provider 状态看板
```
┌──────────────────────────────────────────────┐
│  Provider 健康地图                            │
│                                              │
│  🟢 OpenAI 官方    ████████░░ 82%  主力      │
│  🟡 OpenAI 聚合1   █████░░░░░ 53%  热备  ⚠3天│
│  🟢 OpenAI 聚合2   █████████░ 91%  热备      │
│  🟠 Azure OpenAI   ██░░░░░░░░ 18%  兜底  ⚠! │
│  ⚫ DeepSeek 官方   ░░░░░░░░░░  0%  已耗尽    │
│                                              │
│  🔔 2 条预警                                  │
│    - Azure OpenAI 额度预计1天内耗尽            │
│    - OpenAI 聚合1 用量加速增长，建议关注       │
└──────────────────────────────────────────────┘
```

### 智能调度日志
```
[14:32:01] gpt-4o → OpenAI官方(主力) ✓ 1.2s 856tok
[14:32:05] gpt-4o → OpenAI官方(主力) ✓ 0.8s 423tok  
[14:32:08] gpt-4o → OpenAI官方(主力) ✗ timeout → 
           OpenAI聚合1(热备) ✓ 2.1s 423tok  [自动降级]
[14:35:00] ⚠ Azure OpenAI 额度 < 20%，已降级为冷备
```
