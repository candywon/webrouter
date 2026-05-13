"""Provider Channel 渠道模型 — 一个 Provider 下挂多个 URL/Key/模型组合"""
import json
from extensions import db
from datetime import datetime


class ProviderChannel(db.Model):
    """Provider 渠道 — 同一厂商的多个 API Key / URL / 模型组合
    
    示例:
      Provider: OpenAI (id=1)
        Channel 1: api.openai.com + sk-aaa + [gpt-4o, gpt-4o-mini]  (主力, priority=90)
        Channel 2: api.openai.com + sk-bbb + [gpt-4o]                (备用, priority=70)
        Channel 3: api.openai-asia.com + sk-ccc + [gpt-4o-mini]      (亚洲节点, priority=60)
    """
    __tablename__ = 'wr_provider_channels'

    id = db.Column(db.Integer, primary_key=True)
    provider_id = db.Column(db.Integer, db.ForeignKey('wr_providers.id'), nullable=False, index=True)
    name = db.Column(db.String(100), nullable=False)              # 渠道名: "主力Key-A"
    
    # 独立配置（为空则继承 Provider）
    base_url = db.Column(db.String(500), default='')              # 空=继承 Provider
    api_key = db.Column(db.String(500), default='')               # 空=继承 Provider
    models = db.Column(db.Text, default='')                        # JSON: ["gpt-4o"], 空=继承 Provider
    
    # 渠道自身调度参数（为0则继承 ProviderExt）
    priority = db.Column(db.Integer, default=0)                    # 0=继承
    weight = db.Column(db.Integer, default=0)                     # 0=继承
    rate_limit_rpm = db.Column(db.Integer, default=0)             # 0=继承
    cost_multiplier = db.Column(db.Float, default=0)              # 0=继承
    
    # 状态
    enabled = db.Column(db.Boolean, default=True)
    status = db.Column(db.String(20), default='unchecked')        # healthy/dead/auth_failed/unchecked
    last_check_at = db.Column(db.DateTime)
    last_latency_ms = db.Column(db.Integer)
    last_error = db.Column(db.Text)
    
    # 元数据
    notes = db.Column(db.Text, default='')
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    @property
    def models_list(self):
        if not self.models or self.models == '[]':
            return []
        try:
            return json.loads(self.models)
        except (json.JSONDecodeError, TypeError):
            return []

    def resolve_base_url(self, provider):
        """解析实际 base_url（渠道 > Provider）"""
        return self.base_url if self.base_url else provider.base_url

    def resolve_api_key(self, provider):
        """解析实际 api_key（渠道 > Provider）"""
        return self.api_key if self.api_key else (provider.api_key or '')

    def resolve_models(self, provider):
        """解析实际模型列表（渠道 > Provider）"""
        own = self.models_list
        return own if own else provider.models_list

    def resolve_priority(self, provider_ext):
        """解析实际优先级"""
        return self.priority if self.priority > 0 else (provider_ext.priority if provider_ext else 50)

    def resolve_weight(self, provider_ext):
        return self.weight if self.weight > 0 else (provider_ext.weight if provider_ext else 100)

    def resolve_rate_limit_rpm(self, provider_ext):
        return self.rate_limit_rpm if self.rate_limit_rpm > 0 else (provider_ext.rate_limit_rpm if provider_ext else 0)

    def resolve_cost_multiplier(self, provider_ext):
        return self.cost_multiplier if self.cost_multiplier > 0 else (provider_ext.cost_multiplier if provider_ext else 1.0)

    def to_dict(self, include_secrets=False):
        d = {
            'id': self.id,
            'provider_id': self.provider_id,
            'name': self.name,
            'base_url': self.base_url or '(继承Provider)',
            'api_key_masked': self.api_key[:4] + '...' + self.api_key[-4:] if self.api_key and len(self.api_key) >= 8 else ('(继承Provider)' if not self.api_key else '***'),
            'models': self.models_list,
            'priority': self.priority or '(继承)',
            'weight': self.weight or '(继承)',
            'rate_limit_rpm': self.rate_limit_rpm or '(继承)',
            'cost_multiplier': self.cost_multiplier or '(继承)',
            'enabled': self.enabled,
            'status': self.status,
            'last_latency_ms': self.last_latency_ms,
            'last_error': self.last_error,
            'notes': self.notes,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }
        if include_secrets:
            d['api_key'] = self.api_key or '(继承Provider)'
        return d
