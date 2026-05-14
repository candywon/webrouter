"""WebRouter 自有数据模型 — 独立 SQLite 数据库"""
import json
import secrets
from extensions import db
from datetime import datetime


# ============================================================
#  WR Token — 对外 API Key（核心管控单元）
# ============================================================

class WRToken(db.Model):
    """对外 API Key — 每个员工/部门一个独立 Key"""
    __tablename__ = 'wr_tokens'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)         # Token 名称（如"张三-研发部"）
    key = db.Column(db.String(64), nullable=False, unique=True)  # sk-wr-xxxxxxxxxxxx
    user_id = db.Column(db.Integer, default=0)               # 关联团队用户
    models = db.Column(db.Text, default='')                   # JSON: ["gpt-4o","claude-3"]
    provider_ids = db.Column(db.Text, default='')             # JSON: [1,3]
    quota_total = db.Column(db.BigInteger, default=0)        # 总额度(分), 0=不限
    quota_used = db.Column(db.BigInteger, default=0)         # 已用额度(分)
    rate_limit_rpm = db.Column(db.Integer, default=0)        # 每分钟限速, 0=不限
    subnet_whitelist = db.Column(db.Text, default='')         # JSON: ["10.0.0.0/8"]
    smart_downgrade = db.Column(db.Boolean, default=False)   # 允许智能降级（强模型→便宜模型）
    desensitize_enabled = db.Column(db.Boolean, default=False)  # 是否启用脱敏
    desensitize_level = db.Column(db.String(20), default='standard')  # 脱敏级别：off/standard/strict
    enabled = db.Column(db.Boolean, default=True)
    expires_at = db.Column(db.DateTime, nullable=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    @staticmethod
    def generate_key():
        """生成 sk-wr-xxx 格式的 API Key"""
        return f"sk-wr-{secrets.token_hex(20)}"

    @property
    def quota_remaining(self):
        if self.quota_total <= 0:
            return -1  # 不限
        return max(0, self.quota_total - self.quota_used)

    @property
    def quota_ratio(self):
        if self.quota_total <= 0:
            return 1.0
        return max(0, (self.quota_total - self.quota_used) / self.quota_total)

    @property
    def is_expired(self):
        if not self.expires_at:
            return False
        return datetime.utcnow() > self.expires_at

    @property
    def models_list(self):
        if not self.models or self.models == '[]':
            return []
        try:
            return json.loads(self.models)
        except (json.JSONDecodeError, TypeError):
            return []

    @property
    def provider_ids_list(self):
        if not self.provider_ids or self.provider_ids == '[]':
            return []
        try:
            return json.loads(self.provider_ids)
        except (json.JSONDecodeError, TypeError):
            return []

    def to_dict(self, include_key=False):
        d = {
            'id': self.id,
            'name': self.name,
            'key_prefix': self.key[:10] + '...' if self.key else '',
            'user_id': self.user_id,
            'models': self.models_list,
            'provider_ids': self.provider_ids_list,
            'quota_total': self.quota_total,
            'quota_used': self.quota_used,
            'quota_remaining': self.quota_remaining,
            'quota_ratio': round(self.quota_ratio, 3),
            'rate_limit_rpm': self.rate_limit_rpm,
            'subnet_whitelist': json.loads(self.subnet_whitelist) if self.subnet_whitelist and self.subnet_whitelist != '[]' else [],
            'smart_downgrade': self.smart_downgrade,
            'desensitize_enabled': self.desensitize_enabled,
            'desensitize_level': self.desensitize_level,
            'enabled': self.enabled,
            'is_expired': self.is_expired,
            'expires_at': self.expires_at.isoformat() if self.expires_at else None,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }
        if include_key:
            d['key'] = self.key
        return d


# ============================================================
#  Provider 扩展字段 — 代理相关配置
# ============================================================

class ProviderExt(db.Model):
    """Provider 代理扩展配置"""
    __tablename__ = 'wr_provider_ext'

    provider_id = db.Column(db.Integer, primary_key=True)
    proxy_enabled = db.Column(db.Boolean, default=True)       # 是否纳入代理池
    rate_limit_rpm = db.Column(db.Integer, default=0)         # 每分钟请求上限
    timeout_seconds = db.Column(db.Integer, default=30)       # 请求超时
    max_retries = db.Column(db.Integer, default=2)            # 最大重试次数
    cost_multiplier = db.Column(db.Float, default=1.0)        # 成本倍率
    priority = db.Column(db.Integer, default=50)              # 0-100: 90+主力, 50-89热备, 1-49冷备
    weight = db.Column(db.Integer, default=100)               # 调度权重
    supports_tools = db.Column(db.Boolean, default=True)      # 是否支持 function calling / tools
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            'provider_id': self.provider_id,
            'proxy_enabled': self.proxy_enabled,
            'rate_limit_rpm': self.rate_limit_rpm,
            'timeout_seconds': self.timeout_seconds,
            'max_retries': self.max_retries,
            'cost_multiplier': self.cost_multiplier,
            'priority': self.priority,
            'weight': self.weight,
            'supports_tools': self.supports_tools,
        }


# ============================================================
#  Provider 额度 — 手动或 API 同步
# ============================================================

class ProviderQuota(db.Model):
    """Provider 额度记录"""
    __tablename__ = 'wr_provider_quota'

    provider_id = db.Column(db.Integer, primary_key=True)
    quota_total = db.Column(db.BigInteger, default=0)         # 总额度(分)
    quota_used = db.Column(db.BigInteger, default=0)          # 已用额度(分)
    quota_source = db.Column(db.String(20), default='manual') # manual/api/unknown
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    @property
    def quota_remaining(self):
        if self.quota_total <= 0:
            return -1
        return max(0, self.quota_total - self.quota_used)

    @property
    def quota_ratio(self):
        if self.quota_total <= 0:
            return 1.0
        return max(0, (self.quota_total - self.quota_used) / self.quota_total)

    def to_dict(self):
        return {
            'provider_id': self.provider_id,
            'quota_total': self.quota_total,
            'quota_used': self.quota_used,
            'quota_remaining': self.quota_remaining,
            'quota_ratio': round(self.quota_ratio, 3),
            'quota_source': self.quota_source,
        }


# ============================================================
#  Request Log — 请求日志（wr-proxy 写入）
# ============================================================

class RequestLog(db.Model):
    """请求日志 — wr-proxy 写入，Flask 读取"""
    __tablename__ = 'wr_request_logs'

    id = db.Column(db.Integer, primary_key=True)
    request_id = db.Column(db.String(36), nullable=False)
    token_id = db.Column(db.Integer, nullable=False, index=True)
    token_name = db.Column(db.String(100), default='')
    provider_id = db.Column(db.Integer, nullable=False, index=True)
    provider_name = db.Column(db.String(100), default='')
    model_name = db.Column(db.String(100), nullable=False, index=True)
    endpoint = db.Column(db.String(200), nullable=False)
    input_tokens = db.Column(db.BigInteger, default=0)
    output_tokens = db.Column(db.BigInteger, default=0)
    status_code = db.Column(db.Integer, default=0)
    latency_ms = db.Column(db.Integer, default=0)
    cost_cents = db.Column(db.BigInteger, default=0)
    is_stream = db.Column(db.Boolean, default=False)
    is_retry = db.Column(db.Boolean, default=False)
    error_message = db.Column(db.Text, default='')
    error_type = db.Column(db.String(30), default='')    # quota_exhausted/rate_limited/timeout/unknown
    client_ip = db.Column(db.String(45), default='')
    created_at = db.Column(db.DateTime, default=datetime.utcnow, index=True)

    def to_dict(self):
        return {
            'id': self.id,
            'request_id': self.request_id,
            'token_id': self.token_id,
            'token_name': self.token_name,
            'provider_id': self.provider_id,
            'provider_name': self.provider_name,
            'model_name': self.model_name,
            'endpoint': self.endpoint,
            'input_tokens': self.input_tokens,
            'output_tokens': self.output_tokens,
            'status_code': self.status_code,
            'latency_ms': self.latency_ms,
            'cost_cents': self.cost_cents,
            'cost_yuan': round(self.cost_cents / 100, 4),
            'is_stream': self.is_stream,
            'is_retry': self.is_retry,
            'error_message': self.error_message,
            'error_type': self.error_type,
            'client_ip': self.client_ip,
            'created_at': self.created_at.isoformat() if self.created_at else None,
        }


# ============================================================
#  以下模型保持不变
# ============================================================

class AlertRule(db.Model):
    """告警规则"""
    __tablename__ = 'wr_alert_rules'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)
    condition_type = db.Column(db.String(50), nullable=False)
    condition_config = db.Column(db.Text, nullable=False)
    level = db.Column(db.String(20), nullable=False)
    channels = db.Column(db.Text, nullable=False)
    enabled = db.Column(db.Boolean, default=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
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
    rule_id = db.Column(db.Integer, db.ForeignKey('wr_alert_rules.id'), nullable=True)
    event_data = db.Column(db.Text, nullable=True)
    message = db.Column(db.Text, nullable=False)
    level = db.Column(db.String(20), nullable=False)
    channels_sent = db.Column(db.Text)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)

    def to_dict(self):
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
    provider_id = db.Column(db.Integer, index=True)
    status = db.Column(db.String(20), nullable=False)
    latency_ms = db.Column(db.Integer)
    error_message = db.Column(db.Text)
    checked_at = db.Column(db.DateTime, default=datetime.utcnow, index=True)

    def to_dict(self):
        return {
            'id': self.id,
            'provider_id': self.provider_id,
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
    period = db.Column(db.String(20), default='monthly')
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


# ============================================================
#  Model Pricing — 模型定价表（DB 存储，可热更新）
# ============================================================

class ModelPricing(db.Model):
    """模型定价 — 单位：分/千token (1元=100分)"""
    __tablename__ = 'wr_model_pricing'

    id = db.Column(db.Integer, primary_key=True)
    model = db.Column(db.String(100), nullable=False, unique=True, index=True)
    input_price = db.Column(db.Float, nullable=False, default=0)   # 输入价格(分/千token)
    output_price = db.Column(db.Float, nullable=False, default=0)  # 输出价格(分/千token)
    vendor = db.Column(db.String(50), default='')                  # 厂商: openai/anthropic/google/deepseek/qwen/zhipu/moonshot/other
    is_default = db.Column(db.Boolean, default=False)              # 是否为未知模型的默认定价
    notes = db.Column(db.Text, default='')                         # 备注
    effective_at = db.Column(db.DateTime, default=datetime.utcnow) # 生效时间
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            'id': self.id,
            'model': self.model,
            'input_price': self.input_price,
            'output_price': self.output_price,
            'vendor': self.vendor,
            'is_default': self.is_default,
            'notes': self.notes,
            'effective_at': self.effective_at.isoformat() if self.effective_at else None,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }

    @classmethod
    def seed_defaults(cls):
        """初始化种子定价数据（仅首次建表时）"""
        if cls.query.count() > 0:
            return 0

        seeds = [
            # OpenAI
            cls(model='gpt-4o', input_price=0.18, output_price=0.54, vendor='openai'),
            cls(model='gpt-4o-mini', input_price=0.012, output_price=0.048, vendor='openai'),
            cls(model='gpt-4-turbo', input_price=0.60, output_price=1.80, vendor='openai'),
            cls(model='gpt-4', input_price=2.10, output_price=6.30, vendor='openai'),
            cls(model='gpt-3.5-turbo', input_price=0.003, output_price=0.006, vendor='openai'),
            cls(model='o1-preview', input_price=1.05, output_price=4.20, vendor='openai'),
            cls(model='o1-mini', input_price=0.21, output_price=0.84, vendor='openai'),
            # Anthropic
            cls(model='claude-3.5-sonnet', input_price=0.21, output_price=1.05, vendor='anthropic'),
            cls(model='claude-3.5-haiku', input_price=0.007, output_price=0.035, vendor='anthropic'),
            cls(model='claude-3-opus', input_price=1.05, output_price=5.25, vendor='anthropic'),
            cls(model='claude-3-sonnet', input_price=0.21, output_price=1.05, vendor='anthropic'),
            cls(model='claude-3-haiku', input_price=0.018, output_price=0.09, vendor='anthropic'),
            # Google
            cls(model='gemini-1.5-pro', input_price=0.16, output_price=0.48, vendor='google'),
            cls(model='gemini-1.5-flash', input_price=0.005, output_price=0.015, vendor='google'),
            cls(model='gemini-2.0-flash', input_price=0.005, output_price=0.015, vendor='google'),
            # DeepSeek
            cls(model='deepseek-chat', input_price=0.009, output_price=0.027, vendor='deepseek'),
            cls(model='deepseek-reasoner', input_price=0.42, output_price=1.26, vendor='deepseek'),
            # 通义千问
            cls(model='qwen-turbo', input_price=0.015, output_price=0.045, vendor='qwen'),
            cls(model='qwen-plus', input_price=0.03, output_price=0.09, vendor='qwen'),
            cls(model='qwen-max', input_price=0.15, output_price=0.45, vendor='qwen'),
            # 智谱
            cls(model='glm-4', input_price=0.09, output_price=0.09, vendor='zhipu'),
            cls(model='glm-4-flash', input_price=0.009, output_price=0.009, vendor='zhipu'),
            # 月之暗面
            cls(model='moonshot-v1-8k', input_price=0.09, output_price=0.09, vendor='moonshot'),
            cls(model='moonshot-v1-32k', input_price=0.18, output_price=0.18, vendor='moonshot'),
            # 默认定价（未知模型用）
            cls(model='__default__', input_price=0.015, output_price=0.06, vendor='other', is_default=True, notes='未知模型默认价格（gpt-4o-mini级别）'),
        ]


# ============================================================
#  Desensitize Rule — 脱敏规则
# ============================================================

class DesensitizeRule(db.Model):
    """脱敏规则 — 内置规则 + 自定义规则"""
    __tablename__ = 'wr_desensitize_rules'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)          # 规则名称
    type = db.Column(db.String(20), nullable=False, default='regex')  # builtin/exact/regex
    pattern = db.Column(db.Text, nullable=False)               # exact=精确文本, regex=正则表达式
    category = db.Column(db.String(20), nullable=False, default='CUSTOM')  # PHONE/IDCARD/EMAIL/BANKCARD/IP/APIKEY/CUSTOM
    level = db.Column(db.String(20), nullable=False, default='standard')  # standard/strict
    enabled = db.Column(db.Boolean, default=True)
    sort_order = db.Column(db.Integer, default=0)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            'id': self.id,
            'name': self.name,
            'type': self.type,
            'pattern': self.pattern,
            'category': self.category,
            'level': self.level,
            'enabled': self.enabled,
            'sort_order': self.sort_order,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }
