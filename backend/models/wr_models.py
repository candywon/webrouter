"""WebRouter自有数据模型 — 追加到New-API数据库"""
from extensions import db
from datetime import datetime


class AlertRule(db.Model):
    """告警规则"""
    __tablename__ = 'wr_alert_rules'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)
    condition_type = db.Column(db.String(50), nullable=False)  # key_failed/balance_low/error_rate/usage_spike
    condition_config = db.Column(db.Text, nullable=False)       # JSON配置
    level = db.Column(db.String(20), nullable=False)            # critical/warning/info
    channels = db.Column(db.Text, nullable=False)               # JSON: ["wechat","email"]
    enabled = db.Column(db.Boolean, default=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        import json
        return {
            'id': self.id,
            'name': self.name,
            'condition_type': self.condition_type,
            'condition_config': json.loads(self.condition_config) if self.condition_config else {},
            'level': self.level,
            'channels': json.loads(self.channels) if self.channels else [],
            'enabled': self.enabled,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }


class AlertHistory(db.Model):
    """告警历史"""
    __tablename__ = 'wr_alert_history'

    id = db.Column(db.Integer, primary_key=True)
    rule_id = db.Column(db.Integer, db.ForeignKey('wr_alert_rules.id'))
    event_data = db.Column(db.Text, nullable=False)
    message = db.Column(db.Text, nullable=False)
    level = db.Column(db.String(20), nullable=False)
    channels_sent = db.Column(db.Text)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)

    def to_dict(self):
        import json
        return {
            'id': self.id,
            'rule_id': self.rule_id,
            'event_data': json.loads(self.event_data) if self.event_data else {},
            'message': self.message,
            'level': self.level,
            'channels_sent': json.loads(self.channels_sent) if self.channels_sent else [],
            'created_at': self.created_at.isoformat() if self.created_at else None,
        }


class ChannelHealth(db.Model):
    """渠道/Provider 健康记录"""
    __tablename__ = 'wr_channel_health'

    id = db.Column(db.Integer, primary_key=True)
    provider_id = db.Column(db.Integer, index=True)       # 关联 Provider ID
    channel_id = db.Column(db.Integer, index=True)        # New-API 渠道 ID（仅 newapi/oneapi）
    status = db.Column(db.String(20), nullable=False)  # healthy/warning/dead/rate_limited/disabled
    latency_ms = db.Column(db.Integer)
    error_message = db.Column(db.Text)
    checked_at = db.Column(db.DateTime, default=datetime.utcnow, index=True)

    def to_dict(self):
        return {
            'id': self.id,
            'channel_id': self.channel_id,
            'status': self.status,
            'latency_ms': self.latency_ms,
            'error_message': self.error_message,
            'checked_at': self.checked_at.isoformat() if self.checked_at else None,
        }


class TeamQuota(db.Model):
    """团队额度分配"""
    __tablename__ = 'wr_team_quotas'

    id = db.Column(db.Integer, primary_key=True)
    user_id = db.Column(db.Integer, nullable=False, unique=True)
    quota_total = db.Column(db.BigInteger, default=0)
    quota_used = db.Column(db.BigInteger, default=0)
    period = db.Column(db.String(20), default='monthly')  # monthly/weekly/daily
    reset_at = db.Column(db.DateTime)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)

    @property
    def quota_remaining(self):
        return max(0, self.quota_total - self.quota_used)

    def to_dict(self):
        return {
            'id': self.id,
            'user_id': self.user_id,
            'quota_total': self.quota_total,
            'quota_used': self.quota_used,
            'quota_remaining': self.quota_remaining,
            'period': self.period,
            'reset_at': self.reset_at.isoformat() if self.reset_at else None,
        }


class CostRecord(db.Model):
    """成本记录"""
    __tablename__ = 'wr_cost_records'

    id = db.Column(db.Integer, primary_key=True)
    user_id = db.Column(db.Integer, index=True)
    channel_id = db.Column(db.Integer)
    model_name = db.Column(db.String(100), index=True)
    input_tokens = db.Column(db.BigInteger, default=0)
    output_tokens = db.Column(db.BigInteger, default=0)
    cost_cents = db.Column(db.Integer, default=0)  # 成本(分)
    recorded_at = db.Column(db.DateTime, default=datetime.utcnow, index=True)

    def to_dict(self):
        return {
            'id': self.id,
            'user_id': self.user_id,
            'channel_id': self.channel_id,
            'model_name': self.model_name,
            'input_tokens': self.input_tokens,
            'output_tokens': self.output_tokens,
            'cost_cents': self.cost_cents,
            'cost_yuan': round(self.cost_cents / 100, 4),
            'recorded_at': self.recorded_at.isoformat() if self.recorded_at else None,
        }
