# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""Provider 数据源模型 — 统一管理不同类型的 API 源"""
import json
from extensions import db
from datetime import datetime


class Provider(db.Model):
    """数据源 — 统一纳管直连/聚合/自建等不同类型的 API 源"""
    __tablename__ = 'wr_providers'

    # 支持的 Provider 类型
    TYPE_DIRECT = 'direct'
    TYPE_AGGREGATE = 'aggregate'
    TYPE_LITELLM = 'litellm'
    TYPE_CUSTOM = 'custom'

    VALID_TYPES = [TYPE_DIRECT, TYPE_AGGREGATE, TYPE_LITELLM, TYPE_CUSTOM]

    # 健康状态
    STATUS_UNCHECKED = 'unchecked'
    STATUS_HEALTHY = 'healthy'
    STATUS_WARNING = 'warning'
    STATUS_DEAD = 'dead'
    STATUS_DISABLED = 'disabled'
    STATUS_RATE_LIMITED = 'rate_limited'

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100), nullable=False)
    type = db.Column(db.String(20), nullable=False, default=TYPE_CUSTOM)
    base_url = db.Column(db.String(500), nullable=False)
    # Anthropic 兼容端点（可选）。多数厂商同时提供 OpenAI / Anthropic 两套兼容接口，
    # base_url 为 OpenAI 端点，anthropic_base_url 为 Anthropic 端点。配置后，wr-proxy
    # 会按客户端协议选对应端点直通，避免协议翻译导致的能力损失。
    anthropic_base_url = db.Column(db.String(500), default='')

    # 认证信息
    api_key = db.Column(db.String(500))
    api_key_masked = db.Column(db.String(50))     # 脱敏显示

    # litellm 专有
    master_key = db.Column(db.String(500))

    # custom 专有
    health_endpoint = db.Column(db.String(500))

    # 通用配置
    models = db.Column(db.Text)                   # JSON: 模型列表
    tags = db.Column(db.Text)                     # JSON: 标签
    weight = db.Column(db.Integer, default=100)   # 调度权重 0-100
    priority = db.Column(db.Integer, default=0)   # 优先级
    check_interval = db.Column(db.Integer, default=300)  # 检测间隔(秒)
    enabled = db.Column(db.Boolean, default=True)

    # 状态（系统自动维护）
    status = db.Column(db.String(20), default=STATUS_UNCHECKED)
    last_check_at = db.Column(db.DateTime)
    last_latency_ms = db.Column(db.Integer)
    last_error = db.Column(db.Text)

    # 元数据
    notes = db.Column(db.Text)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow,
                           onupdate=datetime.utcnow)

    @property
    def models_list(self):
        """解析 models JSON 为列表"""
        if not self.models:
            return []
        try:
            return json.loads(self.models)
        except (json.JSONDecodeError, TypeError):
            return []

    @models_list.setter
    def models_list(self, value):
        self.models = json.dumps(value) if value else None

    @property
    def tags_list(self):
        if not self.tags:
            return []
        try:
            return json.loads(self.tags)
        except (json.JSONDecodeError, TypeError):
            return []

    @tags_list.setter
    def tags_list(self, value):
        self.tags = json.dumps(value) if value else None

    def mask_api_key(self, raw_key):
        """生成脱敏显示的 Key"""
        if not raw_key or len(raw_key) < 8:
            return '***'
        return f"{raw_key[:4]}...{raw_key[-4:]}"

    def to_dict(self, include_secrets=False):
        """序列化为字典"""
        d = {
            'id': self.id,
            'name': self.name,
            'type': self.type,
            'base_url': self.base_url,
            'anthropic_base_url': self.anthropic_base_url or '',
            'api_key_masked': self.api_key_masked,
            'models': self.models_list,
            'tags': self.tags_list,
            'weight': self.weight,
            'priority': self.priority,
            'check_interval': self.check_interval,
            'enabled': self.enabled,
            'status': self.status,
            'last_check_at': self.last_check_at.isoformat() if self.last_check_at else None,
            'last_latency_ms': self.last_latency_ms,
            'last_error': self.last_error,
            'notes': self.notes,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }

        if include_secrets:
            d['api_key'] = self.api_key
            d['master_key'] = self.master_key

        # 类型特定字段
        if self.type == self.TYPE_LITELLM:
            d['master_key_set'] = bool(self.master_key)
        if self.type == self.TYPE_CUSTOM:
            d['health_endpoint'] = self.health_endpoint

        return d

    @classmethod
    def seed_defaults(cls):
        """初始化主流直连厂商模板（仅补缺，不覆盖已有配置）"""
        templates = [
            {
                'name': 'OpenAI',
                'base_url': 'https://api.openai.com/v1',
                'models': ['gpt-4o', 'gpt-4o-mini', 'o1', 'o1-mini'],
                'tags': ['direct', 'openai', 'global'],
            },
            {
                'name': 'Anthropic',
                'base_url': 'https://api.anthropic.com',
                'models': ['claude-sonnet-4', 'claude-haiku-4-5', 'claude-opus-4-7'],
                'tags': ['direct', 'anthropic', 'global'],
            },
            {
                'name': 'Google Gemini',
                'base_url': 'https://generativelanguage.googleapis.com/v1beta/openai',
                'models': ['gemini-2.0-flash', 'gemini-1.5-pro', 'gemini-1.5-flash'],
                'tags': ['direct', 'google', 'global'],
            },
            {
                'name': 'DeepSeek',
                'base_url': 'https://api.deepseek.com/v1',
                'models': ['deepseek-chat', 'deepseek-reasoner'],
                'tags': ['direct', 'deepseek', 'china'],
            },
            {
                'name': 'DashScope/Qwen',
                'base_url': 'https://dashscope.aliyuncs.com/compatible-mode/v1',
                'models': ['qwen-plus', 'qwen-turbo', 'qwen-max', 'qwen3-coder-flash'],
                'tags': ['direct', 'qwen', 'china'],
            },
            {
                'name': 'Zhipu GLM',
                'base_url': 'https://open.bigmodel.cn/api/paas/v4',
                'models': ['glm-4', 'glm-4-plus', 'glm-4-flash'],
                'tags': ['direct', 'zhipu', 'china'],
            },
            {
                'name': 'Moonshot/Kimi',
                'base_url': 'https://api.moonshot.cn/v1',
                'models': ['moonshot-v1-8k', 'moonshot-v1-32k', 'moonshot-v1-128k'],
                'tags': ['direct', 'moonshot', 'china'],
            },
            {
                'name': 'Volcengine Ark/Doubao',
                'base_url': 'https://ark.cn-beijing.volces.com/api/v3',
                'models': ['doubao-pro-32k', 'doubao-lite-32k', 'doubao-seed-1-6'],
                'tags': ['direct', 'doubao', 'china'],
            },
            {
                'name': 'MiniMax',
                'base_url': 'https://api.minimax.chat/v1',
                'models': ['abab6.5s-chat', 'abab6.5g-chat', 'minimax-text-01'],
                'tags': ['direct', 'minimax', 'china'],
            },
            {
                'name': 'StepFun',
                'base_url': 'https://api.stepfun.com/v1',
                'models': ['step-1-8k', 'step-1-32k', 'step-2-16k'],
                'tags': ['direct', 'stepfun', 'china'],
            },
            {
                'name': 'xAI Grok',
                'base_url': 'https://api.x.ai/v1',
                'models': ['grok-4', 'grok-3', 'grok-3-mini'],
                'tags': ['direct', 'xai', 'global'],
            },
            {
                'name': 'Mistral',
                'base_url': 'https://api.mistral.ai/v1',
                'models': ['mistral-large-latest', 'mistral-small-latest', 'codestral-latest'],
                'tags': ['direct', 'mistral', 'global'],
            },
            {
                'name': 'OpenRouter',
                'base_url': 'https://openrouter.ai/api/v1',
                'models': ['openai/gpt-4o', 'anthropic/claude-sonnet-4', 'google/gemini-2.0-flash-001'],
                'tags': ['aggregate', 'openrouter', 'global'],
            },
            {
                'name': 'Groq',
                'base_url': 'https://api.groq.com/openai/v1',
                'models': ['llama-3.3-70b-versatile', 'llama-3.1-8b-instant', 'mixtral-8x7b-32768'],
                'tags': ['direct', 'groq', 'global'],
            },
            {
                'name': 'Together AI',
                'base_url': 'https://api.together.xyz/v1',
                'models': ['meta-llama/Llama-3.3-70B-Instruct-Turbo', 'Qwen/Qwen2.5-72B-Instruct-Turbo'],
                'tags': ['direct', 'together', 'global'],
            },
        ]

        created = 0
        for t in templates:
            exists = cls.query.filter((cls.name == t['name']) | (cls.base_url == t['base_url'])).first()
            if exists:
                continue
            p = cls(
                name=t['name'],
                type=cls.TYPE_DIRECT,
                base_url=t['base_url'],
                models=json.dumps(t['models'], ensure_ascii=False),
                tags=json.dumps(t['tags'], ensure_ascii=False),
                priority=0,
                weight=100,
                enabled=False,
                status=cls.STATUS_DISABLED,
                notes='Built-in mainstream provider template: fill in API Key and enable to activate.',
            )
            db.session.add(p)
            db.session.flush()
            from models.wr_models import ProviderExt
            db.session.add(ProviderExt(provider_id=p.id, proxy_enabled=False, priority=0, weight=100))
            created += 1
        if created:
            db.session.commit()
        return created

    @classmethod
    def get_type_config(cls, provider_type=None):
        """获取 Provider 类型定义配置"""
        types = {
            cls.TYPE_DIRECT: {
                'label': '直连官方',
                'description': '直接调用大模型官方 API（OpenAI/Claude/Gemini 等）',
                'icon': '🔌',
                'fields': ['name', 'base_url', 'api_key', 'models'],
                'data_capabilities': ['health', 'latency'],
            },
            cls.TYPE_AGGREGATE: {
                'label': '聚合平台',
                'description': '第三方聚合中转平台（AnyRoute/OhMyGPT/API2D 等）',
                'icon': '🔀',
                'fields': ['name', 'base_url', 'api_key', 'models'],
                'data_capabilities': ['health', 'latency', 'manual_cost'],
            },
            cls.TYPE_LITELLM: {
                'label': 'LiteLLM 代理',
                'description': '自建 LiteLLM 代理，支持自动模型发现',
                'icon': '🦙',
                'fields': ['name', 'base_url', 'master_key'],
                'data_capabilities': ['health', 'latency', 'models'],
            },
            cls.TYPE_CUSTOM: {
                'label': '自定义网关',
                'description': '自研或其他 OpenAI 兼容网关',
                'icon': '⚙️',
                'fields': ['name', 'base_url', 'api_key', 'health_endpoint'],
                'data_capabilities': ['health', 'latency'],
            },
        }
        if provider_type:
            return types.get(provider_type)
        return types
