package main

// 预警评估：基于健康检测 + 额度预测触发告警

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AlertEngine 告警引擎
type AlertEngine struct {
	mu       sync.Mutex
	cooldown map[string]time.Time // 告警冷却
}

var alertEngine = &AlertEngine{
	cooldown: make(map[string]time.Time),
}

// AlertEvent 告警事件
type AlertEvent struct {
	ProviderID   int    `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	Type         string `json:"type"`    // quota_critical/quota_warning/health_dead/health_degraded/error_spike
	Level        string `json:"level"`   // red/orange/yellow/green
	Message      string `json:"message"`
	Timestamp    string `json:"timestamp"`
}

// EvaluateAll 评估所有 Provider 告警
func (ae *AlertEngine) EvaluateAll() []AlertEvent {
	var events []AlertEvent

	providers := router.GetProviders()
	for _, p := range providers {
		// 1. 健康告警
		if p.Status == "dead" {
			events = append(events, ae.createEvent(p, "health_dead", "red",
				fmt.Sprintf("Provider %s 已宕机，请检查", p.Name)))
		} else if p.Status == "rate_limited" {
			events = append(events, ae.createEvent(p, "health_degraded", "orange",
				fmt.Sprintf("Provider %s 被限速", p.Name)))
		} else if p.Status == "auth_failed" {
			events = append(events, ae.createEvent(p, "health_degraded", "red",
				fmt.Sprintf("Provider %s 认证失败，API Key 可能失效", p.Name)))
		}

		// 2. 连续失败告警
		if p.ConsecFails >= 3 {
			events = append(events, ae.createEvent(p, "error_spike", "orange",
				fmt.Sprintf("Provider %s 连续 %d 次失败", p.Name, p.ConsecFails)))
		}

		// 3. 额度预测告警
		if p.QuotaTotal > 0 {
			pred := predictor.PredictExhaustion(p)
			switch pred.AlertLevel {
			case "red":
				events = append(events, ae.createEvent(p, "quota_critical", "red",
					fmt.Sprintf("Provider %s 额度仅剩 %.0f%%，预计 %s 耗尽，请立即充值！",
						p.Name, p.QuotaRatio()*100, pred.PredictedExhaustDate)))
			case "orange":
				events = append(events, ae.createEvent(p, "quota_warning", "orange",
					fmt.Sprintf("Provider %s 额度剩余 %.0f%%，预计 %s 耗尽，建议关注",
						p.Name, p.QuotaRatio()*100, pred.PredictedExhaustDate)))
			case "yellow":
				events = append(events, ae.createEvent(p, "quota_warning", "yellow",
					fmt.Sprintf("Provider %s 额度剩余 %.0f%%，用量趋势: %s",
						p.Name, p.QuotaRatio()*100, pred.Trend)))
			case "black":
				events = append(events, ae.createEvent(p, "quota_critical", "red",
					fmt.Sprintf("Provider %s 额度已耗尽！", p.Name)))
			}
		}
	}

	// 过滤冷却中的告警
	return ae.filterCooldown(events)
}

func (ae *AlertEngine) createEvent(p *Provider, alertType, level, message string) AlertEvent {
	return AlertEvent{
		ProviderID:   p.ID,
		ProviderName: p.Name,
		Type:         alertType,
		Level:        level,
		Message:      message,
		Timestamp:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// filterCooldown 过滤冷却中的告警
func (ae *AlertEngine) filterCooldown(events []AlertEvent) []AlertEvent {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	var filtered []AlertEvent
	for _, e := range events {
		key := fmt.Sprintf("%d:%s", e.ProviderID, e.Type)
		if last, ok := ae.cooldown[key]; ok {
			if time.Since(last) < cfg.AlertCooldown {
				continue // 冷却中
			}
		}
		ae.cooldown[key] = time.Now()
		filtered = append(filtered, e)
	}
	return filtered
}

// NotifyAlerts 通知告警（写入 DB + 日志）
func NotifyAlerts(events []AlertEvent) {
	for _, e := range events {
		data, _ := json.Marshal(e)
		LogWarn("ALERT [%s] %s", e.Level, e.Message)

		// 写入 DB wr_alert_history
		_, err := db.Exec(`
			INSERT INTO wr_alert_history (event_data, message, level, channels_sent, created_at)
			VALUES (?, ?, ?, '["log"]', ?)`,
			string(data), e.Message, e.Level, time.Now().UTC(),
		)
		if err != nil {
			LogWarn("AlertEngine: write alert history: %v", err)
		}
	}
}
