"""企业知识库 — Flask 数据模型"""
from extensions import db
from datetime import datetime


class KnowledgeRaw(db.Model):
    """原始对话暂存表 — wr-proxy 捕获的原始对话"""
    __tablename__ = 'wr_knowledge_raw'

    id = db.Column(db.Integer, primary_key=True)
    request_id = db.Column(db.String(100), nullable=False, index=True)
    token_id = db.Column(db.Integer, nullable=False, index=True)
    token_name = db.Column(db.String(100), default='')
    model_name = db.Column(db.String(100), nullable=False)
    prompt = db.Column(db.Text, nullable=False)
    response = db.Column(db.Text, nullable=False)
    turn_count = db.Column(db.Integer, default=1)
    client_ip = db.Column(db.String(50), default='')
    status = db.Column(db.String(20), default='pending')  # pending/processing/done/skipped
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    processed_at = db.Column(db.DateTime)

    def to_dict(self):
        return {
            'id': self.id,
            'request_id': self.request_id,
            'token_id': self.token_id,
            'token_name': self.token_name,
            'model_name': self.model_name,
            'prompt': self.prompt,
            'response': self.response,
            'turn_count': self.turn_count,
            'client_ip': self.client_ip,
            'status': self.status,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'processed_at': self.processed_at.isoformat() if self.processed_at else None,
        }


class KnowledgeItem(db.Model):
    """知识条目表 — 经 LLM 提炼后的结构化知识"""
    __tablename__ = 'wr_knowledge_items'

    id = db.Column(db.Integer, primary_key=True)
    type = db.Column(db.String(20), nullable=False)  # factual/analytical/procedural
    title = db.Column(db.String(200), nullable=False)
    summary = db.Column(db.Text, default='')
    domain_code = db.Column(db.String(50), default='', index=True)
    department = db.Column(db.String(100), default='', index=True)
    source_request_id = db.Column(db.String(100), nullable=False)
    source_turn_index = db.Column(db.Integer, default=0)
    source_quote = db.Column(db.Text, nullable=False)
    source_char_start = db.Column(db.Integer, default=0)
    source_char_end = db.Column(db.Integer, default=0)
    data_points = db.Column(db.Text, default='')  # JSON array
    confidence = db.Column(db.Float, default=0.0)
    verification = db.Column(db.String(20), default='auto')  # auto/pending/verified/rejected
    verified_by = db.Column(db.Integer, default=0)
    verified_at = db.Column(db.DateTime)
    token_id = db.Column(db.Integer, nullable=False)
    token_name = db.Column(db.String(100), default='')
    model_name = db.Column(db.String(100), default='')
    sensitivity = db.Column(db.String(20), default='medium')  # low/medium/high/restricted
    retention_until = db.Column(db.DateTime)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    def to_dict(self):
        return {
            'id': self.id,
            'type': self.type,
            'title': self.title,
            'summary': self.summary,
            'domain_code': self.domain_code,
            'department': self.department,
            'source_request_id': self.source_request_id,
            'source_turn_index': self.source_turn_index,
            'source_quote': self.source_quote,
            'data_points': self.data_points,
            'confidence': self.confidence,
            'verification': self.verification,
            'token_id': self.token_id,
            'token_name': self.token_name,
            'model_name': self.model_name,
            'sensitivity': self.sensitivity,
            'retention_until': self.retention_until.isoformat() if self.retention_until else None,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }


class KnowledgeDomain(db.Model):
    """业务域管理表"""
    __tablename__ = 'wr_knowledge_domains'

    id = db.Column(db.Integer, primary_key=True)
    domain_code = db.Column(db.String(50), unique=True, nullable=False)
    domain_name = db.Column(db.String(100), nullable=False)
    department = db.Column(db.String(100), default='')
    status = db.Column(db.String(20), default='pending')
    sample_count = db.Column(db.Integer, default=0)
    auto_keywords = db.Column(db.String(500), default='')
    description = db.Column(db.Text, default='')
    merged_into = db.Column(db.Integer, default=0)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    confirmed_at = db.Column(db.DateTime)
    confirmed_by = db.Column(db.Integer)

    def to_dict(self):
        return {
            'id': self.id,
            'domain_code': self.domain_code,
            'domain_name': self.domain_name,
            'department': self.department,
            'status': self.status,
            'sample_count': self.sample_count,
            'auto_keywords': self.auto_keywords,
            'description': self.description,
            'merged_into': self.merged_into,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'confirmed_at': self.confirmed_at.isoformat() if self.confirmed_at else None,
        }


class KnowledgeDomainRisk(db.Model):
    """领域风险配置表"""
    __tablename__ = 'wr_knowledge_domain_risk'

    domain_code = db.Column(db.String(50), primary_key=True)
    risk_level = db.Column(db.String(20), nullable=False, default='medium')
    min_verification = db.Column(db.String(20), nullable=False, default='auto')
    max_age_days = db.Column(db.Integer, default=180)
    disclaimer_template = db.Column(db.Text, default='')
    allow_factual_injection = db.Column(db.Boolean, default=True)
    allow_analytical_injection = db.Column(db.Boolean, default=False)
    allow_procedural_injection = db.Column(db.Boolean, default=True)

    def to_dict(self):
        return {
            'domain_code': self.domain_code,
            'risk_level': self.risk_level,
            'min_verification': self.min_verification,
            'max_age_days': self.max_age_days,
            'disclaimer_template': self.disclaimer_template,
            'allow_factual_injection': self.allow_factual_injection,
            'allow_analytical_injection': self.allow_analytical_injection,
            'allow_procedural_injection': self.allow_procedural_injection,
        }


class AgentMemory(db.Model):
    """持久记忆表 — wr-proxy 对话记忆"""
    __tablename__ = 'wr_agent_memory'

    id = db.Column(db.Integer, primary_key=True)
    token_id = db.Column(db.Integer, nullable=False, index=True)
    token_name = db.Column(db.String(100), default='')
    session_id = db.Column(db.String(100), default='', index=True)
    category = db.Column(db.String(20), default='context')  # preference/fact/context/goal/constraint
    title = db.Column(db.String(200), nullable=False)
    content = db.Column(db.Text, nullable=False)
    tags = db.Column(db.Text, default='[]')  # JSON array
    priority = db.Column(db.Integer, default=3)  # 1-5
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    expires_at = db.Column(db.DateTime)

    def to_dict(self):
        return {
            'id': self.id,
            'token_id': self.token_id,
            'token_name': self.token_name,
            'session_id': self.session_id,
            'category': self.category,
            'title': self.title,
            'content': self.content,
            'tags': self.tags,
            'priority': self.priority,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
            'expires_at': self.expires_at.isoformat() if self.expires_at else None,
        }


class KnowledgeAnalysis(db.Model):
    """分析记录表"""
    __tablename__ = 'wr_knowledge_analyses'

    id = db.Column(db.Integer, primary_key=True)
    task_id = db.Column(db.String(100), unique=True, nullable=False)
    token_id = db.Column(db.Integer, nullable=False, index=True)
    token_name = db.Column(db.String(100), default='')
    domains = db.Column(db.Text, nullable=False)  # JSON array
    departments = db.Column(db.Text, default='[]')
    types = db.Column(db.Text, default='[]')
    date_from = db.Column(db.String(20), default='')
    date_to = db.Column(db.String(20), default='')
    item_count = db.Column(db.Integer, default=0)
    analysis_type = db.Column(db.String(50), default='domain_overview')
    model_used = db.Column(db.String(100), default='')
    strategy = db.Column(db.Text, default='')
    segment_count = db.Column(db.Integer, default=1)
    tokens_consumed = db.Column(db.Integer, default=0)
    cost = db.Column(db.Float, default=0.0)
    duration_ms = db.Column(db.Integer, default=0)
    status = db.Column(db.String(20), default='processing')
    result = db.Column(db.Text, default='')
    error_message = db.Column(db.Text, default='')
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    completed_at = db.Column(db.DateTime)

    def to_dict(self):
        return {
            'id': self.id,
            'task_id': self.task_id,
            'token_id': self.token_id,
            'token_name': self.token_name,
            'domains': self.domains,
            'departments': self.departments,
            'types': self.types,
            'date_from': self.date_from,
            'date_to': self.date_to,
            'item_count': self.item_count,
            'analysis_type': self.analysis_type,
            'model_used': self.model_used,
            'strategy': self.strategy,
            'segment_count': self.segment_count,
            'tokens_consumed': self.tokens_consumed,
            'cost': self.cost,
            'duration_ms': self.duration_ms,
            'status': self.status,
            'result': self.result,
            'error_message': self.error_message,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'completed_at': self.completed_at.isoformat() if self.completed_at else None,
        }
