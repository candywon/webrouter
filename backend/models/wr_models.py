# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""WebRouter 自有数据模型 — 独立 SQLite 数据库"""
import json
import secrets
from extensions import db
from datetime import datetime
from flask_login import UserMixin
from werkzeug.security import generate_password_hash, check_password_hash


# ============================================================
#  Org — 组织架构（企业/部门/小组）
# ============================================================

class Org(db.Model):
    """组织架构 — 树形结构，支持企业 → 部门 → 小组"""
    __tablename__ = 'wr_orgs'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)           # 组织名称
    org_type = db.Column(db.String(20), nullable=False, default='department')  # company/department/group
    parent_id = db.Column(db.Integer, db.ForeignKey('wr_orgs.id'), nullable=True, index=True)  # 父组织
    quota_total = db.Column(db.BigInteger, default=0)          # 组织总额度（分），0=不限
    quota_used = db.Column(db.BigInteger, default=0)           # 已用额度（汇总）
    quota_period = db.Column(db.String(20), default='monthly') # monthly/yearly/none
    enabled = db.Column(db.Boolean, default=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)

    children = db.relationship('Org', backref=db.backref('parent', remote_side=[id]), lazy='dynamic')
    members = db.relationship('WRToken', backref='org', lazy='dynamic', foreign_keys='WRToken.org_id')

    def to_dict(self, member_count=None):
        d = {
            'id': self.id,
            'name': self.name,
            'org_type': self.org_type,
            'parent_id': self.parent_id,
            'quota_total': self.quota_total,
            'quota_used': self.quota_used,
            'quota_period': self.quota_period,
            'enabled': self.enabled,
            'created_at': self.created_at.isoformat() if self.created_at else None,
        }
        if member_count is not None:
            d['member_count'] = member_count
        return d


# ============================================================
#  Admin User — 后台管理员账号
# ============================================================

class AdminUser(UserMixin, db.Model):
    """后台管理员 — 单管理员账号"""
    __tablename__ = 'wr_admin_users'

    id = db.Column(db.Integer, primary_key=True)
    username = db.Column(db.String(64), nullable=False, unique=True, index=True)
    password_hash = db.Column(db.String(255), nullable=False)
    enabled = db.Column(db.Boolean, default=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    last_login_at = db.Column(db.DateTime, nullable=True)

    def set_password(self, password):
        self.password_hash = generate_password_hash(password)

    def check_password(self, password):
        return check_password_hash(self.password_hash, password)

    @classmethod
    def ensure_default(cls, username, password):
        user = cls.query.filter_by(username=username).first()
        if user:
            return user, False
        user = cls(username=username, enabled=True)
        user.set_password(password)
        db.session.add(user)
        db.session.commit()
        return user, True


# ============================================================
#  WR Token — 对外 API Key（核心管控单元）
# ============================================================

class WRToken(db.Model):
    """对外 API Key — 每个员工/部门一个独立 Key"""
    __tablename__ = 'wr_tokens'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)         # Token 名称（如"张三-研发部"）
    key = db.Column(db.String(64), nullable=False, unique=True)  # sk-wr-xxxxxxxxxxxx
    org_id = db.Column(db.Integer, db.ForeignKey('wr_orgs.id'), nullable=True, index=True)  # 所属组织
    member_email = db.Column(db.String(100), default='')      # 成员邮箱（用于通知）
    models = db.Column(db.Text, default='')                   # JSON: ["gpt-4o","claude-3"]
    provider_ids = db.Column(db.Text, default='')             # JSON: [1,3]
    quota_total = db.Column(db.BigInteger, default=0)        # 总额度(分), 0=不限
    quota_used = db.Column(db.BigInteger, default=0)         # 已用额度(分)
    rate_limit_rpm = db.Column(db.Integer, default=0)        # 每分钟限速, 0=不限
    subnet_whitelist = db.Column(db.Text, default='')         # JSON: ["10.0.0.0/8"]
    smart_downgrade = db.Column(db.Boolean, default=False)   # 允许智能降级（强模型→便宜模型）
    desensitize_enabled = db.Column(db.Boolean, default=False)  # 是否启用脱敏
    desensitize_level = db.Column(db.String(20), default='standard')  # 脱敏级别：off/standard/strict
    # 知识库相关字段
    knowledge_capture_enabled = db.Column(db.Boolean, default=False)  # 是否开启知识捕获
    knowledge_department = db.Column(db.String(100), default='')       # 所属部门（用于知识分类）
    rag_enabled = db.Column(db.Boolean, default=False)                 # 是否开启 RAG 注入
    rag_min_relevance = db.Column(db.Float, default=0.7)              # RAG 最低相关度
    rag_top_k = db.Column(db.Integer, default=3)                      # RAG 召回条数
    system_prompt_knowledge = db.Column(db.Text, default='')           # 知识增强 System Prompt
    session_recall_enabled = db.Column(db.Boolean, default=False)      # 是否开启会话记忆召回（@recall / X-Recall-Session）
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
            'org_id': self.org_id,
            'org_name': self.org.name if self.org else None,
            'member_email': self.member_email,
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
            'knowledge_capture_enabled': self.knowledge_capture_enabled,
            'knowledge_department': self.knowledge_department,
            'rag_enabled': self.rag_enabled,
            'rag_min_relevance': self.rag_min_relevance,
            'rag_top_k': self.rag_top_k,
            'system_prompt_knowledge': self.system_prompt_knowledge,
            'session_recall_enabled': self.session_recall_enabled,
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
    fallback_enabled = db.Column(db.Boolean, default=True)    # 有 Channel 时，Provider 主体是否作为兜底渠道参与调度
    api_format = db.Column(db.String(20), default='auto')     # 上游 API 协议: openai/anthropic/auto
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
            'fallback_enabled': self.fallback_enabled,
            'api_format': self.api_format or 'auto',
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
        total = self.quota_total or 0
        used = self.quota_used or 0
        if total <= 0:
            return -1
        return max(0, total - used)

    @property
    def quota_ratio(self):
        total = self.quota_total or 0
        used = self.quota_used or 0
        if total <= 0:
            return 1.0
        return max(0, (total - used) / total)

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
    cached_tokens = db.Column(db.BigInteger, default=0)
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


# ============================================================
#  Model Grade — 模型分级表（智能选模型用）
# ============================================================

class ModelGrade(db.Model):
    """模型分级 — economy/standard/premium，供 auto/smart 降级使用"""
    __tablename__ = 'wr_model_grades'

    id = db.Column(db.Integer, primary_key=True)
    model = db.Column(db.String(100), nullable=False, unique=True, index=True)
    tier = db.Column(db.String(20), nullable=False, index=True)       # economy/standard/enhanced/premium/flagship
    cost_index = db.Column(db.Float, nullable=False, default=1.0)     # 相对成本指数
    vendor = db.Column(db.String(50), default='')                     # 厂商标识
    description = db.Column(db.Text, default='')                      # 描述说明
    enabled = db.Column(db.Boolean, default=True)                     # 是否参与调度
    sort_order = db.Column(db.Integer, default=0)                     # 排序
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            'id': self.id,
            'model': self.model,
            'tier': self.tier,
            'cost_index': self.cost_index,
            'vendor': self.vendor,
            'description': self.description,
            'enabled': self.enabled,
            'sort_order': self.sort_order,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }

    @classmethod
    def seed_defaults(cls):
        """初始化种子分级数据（仅首次建表时）"""
        if cls.query.count() > 0:
            return 0

        seeds = [
            # economy — cheap & fast (chat, translation, short Q&A)
            cls(model='qwen3-coder-flash', tier='economy', cost_index=1.0, vendor='qwen', description='Qwen Coder Flash', sort_order=1),
            cls(model='qwen-turbo', tier='economy', cost_index=1.0, vendor='qwen', description='Qwen Turbo', sort_order=2),
            cls(model='gpt-4o-mini', tier='economy', cost_index=1.5, vendor='openai', description='GPT-4o Mini', sort_order=3),
            # standard — balanced (writing, summarization, formatting)
            cls(model='qwen-plus-2025-07-28', tier='standard', cost_index=3.0, vendor='qwen', description='Qwen Plus', sort_order=10),
            cls(model='qwen-plus', tier='standard', cost_index=3.0, vendor='qwen', description='Qwen Plus', sort_order=11),
            cls(model='gpt-4o', tier='standard', cost_index=5.0, vendor='openai', description='GPT-4o', sort_order=12),
            cls(model='deepseek-chat', tier='standard', cost_index=2.0, vendor='deepseek', description='DeepSeek Chat', sort_order=13),
            # enhanced — capable (code generation, multi-step reasoning, docs)
            cls(model='deepseek-reasoner', tier='enhanced', cost_index=4.0, vendor='deepseek', description='DeepSeek Reasoner', sort_order=30),
            cls(model='claude-sonnet-4', tier='enhanced', cost_index=8.0, vendor='anthropic', description='Claude Sonnet 4', sort_order=31),
            cls(model='gemini-2.5-flash', tier='enhanced', cost_index=5.0, vendor='google', description='Gemini 2.5 Flash', sort_order=32),
            # premium — powerful (complex architecture, math proofs, long analysis)
            cls(model='qwen3.6-plus', tier='premium', cost_index=10.0, vendor='qwen', description='Qwen 3.6 Plus', sort_order=40),
            cls(model='qwen-max', tier='premium', cost_index=12.0, vendor='qwen', description='Qwen Max', sort_order=41),
            cls(model='o1', tier='premium', cost_index=15.0, vendor='openai', description='OpenAI o1', sort_order=42),
            cls(model='o1-mini', tier='premium', cost_index=8.0, vendor='openai', description='OpenAI o1 Mini', sort_order=43),
            cls(model='claude-opus-4', tier='premium', cost_index=18.0, vendor='anthropic', description='Claude Opus 4', sort_order=44),
            # flagship — top-tier (research, competition, deep multimodal reasoning)
            cls(model='o3', tier='flagship', cost_index=30.0, vendor='openai', description='OpenAI o3', sort_order=50),
            cls(model='claude-opus-4-extended', tier='flagship', cost_index=25.0, vendor='anthropic', description='Claude Opus 4 Extended', sort_order=51),
        ]

        for s in seeds:
            db.session.add(s)
        db.session.commit()
        return len(seeds)


# ============================================================
#  System Setting — 系统设置键值存储
# ============================================================

class SystemSetting(db.Model):
    """系统设置 — key-value 存储，value 存 JSON 字符串"""
    __tablename__ = 'wr_system_settings'

    id = db.Column(db.Integer, primary_key=True)
    key = db.Column(db.String(100), nullable=False, unique=True, index=True)
    value = db.Column(db.Text, nullable=False, default='')           # JSON 字符串
    value_type = db.Column(db.String(20), nullable=False, default='string')  # string/int/float/bool/json
    description = db.Column(db.String(255), default='')              # 设置项说明
    category = db.Column(db.String(50), default='general')           # general/proxy/monitor/alert/advanced
    editable = db.Column(db.Boolean, default=True)                   # 是否允许前端编辑
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    @staticmethod
    def get(key_name, default=None):
        """获取设置值，不存在返回默认值"""
        s = SystemSetting.query.filter_by(key=key_name).first()
        if not s:
            return default
        return s.typed_value

    @staticmethod
    def set(key_name, value, value_type=None, description='', category='general', editable=True):
        """设置键值，不存在则创建"""
        s = SystemSetting.query.filter_by(key=key_name).first()
        if s:
            s.value = json.dumps(value, ensure_ascii=False)
            if value_type:
                s.value_type = value_type
            s.updated_at = datetime.utcnow()
        else:
            if value_type is None:
                if isinstance(value, bool):
                    value_type = 'bool'
                elif isinstance(value, int):
                    value_type = 'int'
                elif isinstance(value, float):
                    value_type = 'float'
                elif isinstance(value, (dict, list)):
                    value_type = 'json'
                else:
                    value_type = 'string'
            s = SystemSetting(
                key=key_name,
                value=json.dumps(value, ensure_ascii=False),
                value_type=value_type,
                description=description,
                category=category,
                editable=editable,
            )
            db.session.add(s)
        db.session.commit()
        return s

    @property
    def typed_value(self):
        """根据 value_type 返回对应 Python 类型"""
        try:
            raw = json.loads(self.value)
            if self.value_type == 'bool':
                return bool(raw)
            elif self.value_type == 'int':
                return int(raw)
            elif self.value_type == 'float':
                return float(raw)
            elif self.value_type == 'json':
                return raw
            else:
                return str(raw) if raw is not None else ''
        except (json.JSONDecodeError, TypeError, ValueError):
            return self.value

    def to_dict(self):
        return {
            'id': self.id,
            'key': self.key,
            'value': self.typed_value,
            'value_type': self.value_type,
            'description': self.description,
            'category': self.category,
            'editable': self.editable,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }

    @classmethod
    def seed_defaults(cls):
        """初始化默认设置（仅首次）"""
        defaults = [
            ('proxy_enabled', True, 'bool', '是否启用代理', 'proxy', True),
            ('proxy_url', 'http://localhost:5051', 'string', '代理服务地址', 'proxy', False),
            ('gateway_url', 'http://localhost:5051', 'string', '对外网关地址（用于成员邀请邮件）', 'proxy', True),
            ('routing_strategy', 'smart', 'string', '路由策略（smart/priority/round_robin/least_latency/cost_first）', 'proxy', True),
            ('default_timeout', 60, 'int', '默认请求超时（秒）', 'proxy', True),
            ('max_retry_count', 2, 'int', '最大重试次数', 'proxy', True),
            ('max_failover', 3, 'int', '最大降级次数', 'proxy', True),
            ('log_retention_days', 30, 'int', '日志保留天数（自动清理过期日志）', 'monitor', True),
            ('alert_wechat_sendkey', '', 'string', 'Server酱微信推送 sendkey（https://sct.ftqq.com/ 获取）', 'alert', True),
            ('alert_smtp_host', '', 'string', 'SMTP 服务器地址（如 smtp.gmail.com）', 'alert', True),
            ('alert_smtp_port', 587, 'int', 'SMTP 端口（TLS:587, SSL:465）', 'alert', True),
            ('alert_smtp_user', '', 'string', 'SMTP 用户名/邮箱', 'alert', True),
            ('alert_smtp_password', '', 'string', 'SMTP 密码/应用专用密码', 'alert', True),
            ('alert_smtp_from', '', 'string', '告警邮件发件人地址（留空则使用 SMTP 用户名）', 'alert', True),
            ('alert_email_to', '', 'string', '告警邮件收件人地址（逗号分隔多地址）', 'alert', True),
            ('health_test_configs', [
                {'domain': 'api.openai.com', 'name': 'OpenAI', 'endpoint': '/v1/chat/completions', 'body': '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1}'},
                {'domain': 'api.anthropic.com', 'name': 'Anthropic', 'endpoint': '/v1/messages', 'body': '{"model":"claude-3-haiku-20240307","messages":[{"role":"user","content":"hi"}],"max_tokens":1}'},
                {'domain': 'api.deepseek.com', 'name': 'DeepSeek', 'endpoint': '/v1/chat/completions', 'body': '{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"max_tokens":1}'},
                {'domain': 'dashscope.aliyuncs.com', 'name': '通义千问', 'endpoint': '/compatible-mode/v1/chat/completions', 'body': '{"model":"qwen-turbo","messages":[{"role":"user","content":"hi"}],"max_tokens":1}'},
                {'domain': 'open.bigmodel.cn', 'name': '智谱', 'endpoint': '/v4/chat/completions', 'body': '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":1}'},
            ], 'json', '厂商健康测试配置（按域名匹配测试端点和模型）', 'monitor', True),
        ]
        created = 0
        for key, value, vtype, desc, cat, editable in defaults:
            if cls.query.filter_by(key=key).count() == 0:
                s = cls(
                    key=key,
                    value=json.dumps(value, ensure_ascii=False),
                    value_type=vtype,
                    description=desc,
                    category=cat,
                    editable=editable,
                )
                db.session.add(s)
                created += 1
        if created > 0:
            db.session.commit()
        return created


class ModelAlias(db.Model):
    """模型别名 — 短名称到完整模型名的映射"""
    __tablename__ = 'wr_model_aliases'

    id = db.Column(db.Integer, primary_key=True)
    alias = db.Column(db.String(100), nullable=False, unique=True, index=True)
    target = db.Column(db.String(100), nullable=False)
    enabled = db.Column(db.Boolean, default=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)

    def to_dict(self):
        return {
            'id': self.id,
            'alias': self.alias,
            'target': self.target,
            'enabled': self.enabled,
            'created_at': self.created_at.isoformat() if self.created_at else None,
        }

    @classmethod
    def seed_defaults(cls):
        """初始化种子别名数据（仅首次建表时）"""
        if cls.query.count() > 0:
            return 0

        seeds = [
            cls(alias='gpt-4o', target='gpt-4o-2024-05-13'),
            cls(alias='gpt-4o-mini', target='gpt-4o-mini-2024-07-18'),
            cls(alias='gpt-3.5-turbo', target='gpt-3.5-turbo-0125'),
            cls(alias='claude-3-5-sonnet', target='claude-sonnet-4-20250514'),
            cls(alias='claude-3-opus', target='claude-opus-4-20250414'),
            cls(alias='qwen-plus', target='qwen-plus-2025-07-28'),
            cls(alias='qwen-max', target='qwen-max-2025-01-25'),
            cls(alias='glm-4', target='glm-4-plus'),
        ]
        created = 0
        for s in seeds:
            if cls.query.filter_by(alias=s.alias).count() == 0:
                db.session.add(s)
                created += 1
        if created > 0:
            db.session.commit()
        return created
