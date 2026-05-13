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
    TYPE_NEWAPI = 'newapi'
    TYPE_ONEAPI = 'oneapi'
    TYPE_LITELLM = 'litellm'
    TYPE_CUSTOM = 'custom'

    VALID_TYPES = [TYPE_DIRECT, TYPE_AGGREGATE, TYPE_NEWAPI,
                   TYPE_ONEAPI, TYPE_LITELLM, TYPE_CUSTOM]

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

    # 认证信息（敏感字段，应加密存储）
    api_key = db.Column(db.String(500))
    api_key_masked = db.Column(db.String(50))     # 脱敏显示 sk-xxx...xxxx

    # newapi/oneapi 专有
    admin_token = db.Column(db.String(500))
    db_uri = db.Column(db.String(500))

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
            d['admin_token'] = self.admin_token
            d['db_uri'] = self.db_uri
            d['master_key'] = self.master_key

        # 类型特定字段
        if self.type in (self.TYPE_NEWAPI, self.TYPE_ONEAPI):
            d['admin_token_set'] = bool(self.admin_token)
            d['db_uri_set'] = bool(self.db_uri)
        if self.type == self.TYPE_LITELLM:
            d['master_key_set'] = bool(self.master_key)
        if self.type == self.TYPE_CUSTOM:
            d['health_endpoint'] = self.health_endpoint

        return d

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
            cls.TYPE_NEWAPI: {
                'label': '自建 New-API',
                'description': '自建 New-API 实例，支持完整数据读取',
                'icon': '🏗️',
                'fields': ['name', 'base_url', 'api_key', 'admin_token', 'db_uri'],
                'data_capabilities': ['health', 'latency', 'channels', 'users',
                                      'logs', 'costs', 'balance'],
            },
            cls.TYPE_ONEAPI: {
                'label': '自建 One-API',
                'description': '自建 One-API 实例，支持完整数据读取',
                'icon': '🏗️',
                'fields': ['name', 'base_url', 'api_key', 'admin_token', 'db_uri'],
                'data_capabilities': ['health', 'latency', 'channels', 'users', 'logs'],
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
