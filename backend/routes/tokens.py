"""Token 管理 API — 对外 API Key 的 CRUD"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import WRToken
from extensions import db

tokens_bp = Blueprint('tokens', __name__)


@tokens_bp.route('/')
def list_tokens():
    """Token 列表"""
    tokens = WRToken.query.order_by(WRToken.id.desc()).all()
    return jsonify({
        'tokens': [t.to_dict() for t in tokens],
        'total': len(tokens),
    })


@tokens_bp.route('/', methods=['POST'])
def create_token():
    """创建 Token"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    name = (data.get('name') or '').strip()
    if not name:
        return jsonify({'error': '名称不能为空'}), 400

    token = WRToken(
        name=name,
        key=WRToken.generate_key(),
    )

    # 可选字段
    if 'org_id' in data:
        token.org_id = int(data['org_id']) if data['org_id'] else None
    if 'member_email' in data:
        token.member_email = data['member_email'].strip()
    if 'models' in data:
        m = data['models']
        token.models = json.dumps(m) if isinstance(m, list) else m
    if 'provider_ids' in data:
        p = data['provider_ids']
        token.provider_ids = json.dumps(p) if isinstance(p, list) else p
    if 'quota_total' in data:
        token.quota_total = int(data['quota_total'])
    if 'rate_limit_rpm' in data:
        token.rate_limit_rpm = int(data['rate_limit_rpm'])
    if 'subnet_whitelist' in data:
        sw = data['subnet_whitelist']
        token.subnet_whitelist = json.dumps(sw) if isinstance(sw, list) else sw
    if 'smart_downgrade' in data:
        token.smart_downgrade = bool(data['smart_downgrade'])
    if 'desensitize_enabled' in data:
        token.desensitize_enabled = bool(data['desensitize_enabled'])
    if 'desensitize_level' in data:
        level = data['desensitize_level']
        if level not in ('off', 'standard', 'strict'):
            return jsonify({'error': 'desensitize_level 必须为 off/standard/strict'}), 400
        token.desensitize_level = level
    # 知识库相关字段
    if 'knowledge_capture_enabled' in data:
        token.knowledge_capture_enabled = bool(data['knowledge_capture_enabled'])
    if 'knowledge_department' in data:
        token.knowledge_department = data['knowledge_department'].strip()
    if 'rag_enabled' in data:
        token.rag_enabled = bool(data['rag_enabled'])
    if 'rag_min_relevance' in data:
        token.rag_min_relevance = float(data['rag_min_relevance'])
    if 'rag_top_k' in data:
        token.rag_top_k = int(data['rag_top_k'])
    if 'system_prompt_knowledge' in data:
        token.system_prompt_knowledge = data['system_prompt_knowledge']
    if 'enabled' in data:
        token.enabled = bool(data['enabled'])
    if 'expires_at' in data and data['expires_at']:
        from dateutil.parser import parse as parse_dt
        try:
            token.expires_at = parse_dt(data['expires_at'])
        except (ValueError, TypeError):
            return jsonify({'error': 'expires_at 格式无效'}), 400

    db.session.add(token)
    db.session.commit()

    return jsonify({
        'message': 'Token 创建成功',
        'token': token.to_dict(include_key=True),  # 创建时返回完整 Key
    }), 201


@tokens_bp.route('/<int:token_id>')
def get_token(token_id):
    """Token 详情"""
    token = WRToken.query.get(token_id)
    if not token:
        return jsonify({'error': 'Token not found'}), 404

    result = token.to_dict()

    # 附加用量摘要
    from models.wr_models import RequestLog
    from sqlalchemy import func

    # 总用量
    total = db.session.query(
        func.count(RequestLog.id),
        func.coalesce(func.sum(RequestLog.input_tokens), 0),
        func.coalesce(func.sum(RequestLog.output_tokens), 0),
        func.coalesce(func.sum(RequestLog.cost_cents), 0),
    ).filter(RequestLog.token_id == token_id).first()

    # 今日用量
    today = db.session.query(
        func.count(RequestLog.id),
        func.coalesce(func.sum(RequestLog.cost_cents), 0),
    ).filter(
        RequestLog.token_id == token_id,
        RequestLog.created_at >= func.date('now'),
    ).first()

    result['usage'] = {
        'total_requests': total[0] or 0,
        'total_input_tokens': total[1] or 0,
        'total_output_tokens': total[2] or 0,
        'total_cost_cents': total[3] or 0,
        'today_requests': today[0] or 0,
        'today_cost_cents': today[1] or 0,
    }

    return jsonify(result)


@tokens_bp.route('/<int:token_id>', methods=['PUT'])
def update_token(token_id):
    """更新 Token"""
    token = WRToken.query.get(token_id)
    if not token:
        return jsonify({'error': 'Token not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    if 'org_id' in data:
        token.org_id = int(data['org_id']) if data['org_id'] else None
    if 'member_email' in data:
        token.member_email = data['member_email'].strip()
    if 'name' in data:
        token.name = data['name'].strip()
    if 'models' in data:
        m = data['models']
        token.models = json.dumps(m) if isinstance(m, list) else m
    if 'provider_ids' in data:
        p = data['provider_ids']
        token.provider_ids = json.dumps(p) if isinstance(p, list) else p
    if 'quota_total' in data:
        token.quota_total = int(data['quota_total'])
    if 'rate_limit_rpm' in data:
        token.rate_limit_rpm = int(data['rate_limit_rpm'])
    if 'subnet_whitelist' in data:
        sw = data['subnet_whitelist']
        token.subnet_whitelist = json.dumps(sw) if isinstance(sw, list) else sw
    if 'smart_downgrade' in data:
        token.smart_downgrade = bool(data['smart_downgrade'])
    if 'desensitize_enabled' in data:
        token.desensitize_enabled = bool(data['desensitize_enabled'])
    if 'desensitize_level' in data:
        level = data['desensitize_level']
        if level not in ('off', 'standard', 'strict'):
            return jsonify({'error': 'desensitize_level 必须为 off/standard/strict'}), 400
        token.desensitize_level = level
    # 知识库相关字段
    if 'knowledge_capture_enabled' in data:
        token.knowledge_capture_enabled = bool(data['knowledge_capture_enabled'])
    if 'knowledge_department' in data:
        token.knowledge_department = data['knowledge_department'].strip()
    if 'rag_enabled' in data:
        token.rag_enabled = bool(data['rag_enabled'])
    if 'rag_min_relevance' in data:
        token.rag_min_relevance = float(data['rag_min_relevance'])
    if 'rag_top_k' in data:
        token.rag_top_k = int(data['rag_top_k'])
    if 'system_prompt_knowledge' in data:
        token.system_prompt_knowledge = data['system_prompt_knowledge']
    if 'enabled' in data:
        token.enabled = bool(data['enabled'])
    if 'expires_at' in data:
        if data['expires_at']:
            from dateutil.parser import parse as parse_dt
            try:
                token.expires_at = parse_dt(data['expires_at'])
            except (ValueError, TypeError):
                return jsonify({'error': 'expires_at 格式无效'}), 400
        else:
            token.expires_at = None

    db.session.commit()

    return jsonify({
        'message': 'Token 更新成功',
        'token': token.to_dict(),
    })


@tokens_bp.route('/<int:token_id>', methods=['DELETE'])
def delete_token(token_id):
    """删除 Token"""
    token = WRToken.query.get(token_id)
    if not token:
        return jsonify({'error': 'Token not found'}), 404

    db.session.delete(token)
    db.session.commit()
    return jsonify({'message': 'Token 已删除'})


@tokens_bp.route('/<int:token_id>/reset-quota', methods=['POST'])
def reset_quota(token_id):
    """重置 Token 配额"""
    token = WRToken.query.get(token_id)
    if not token:
        return jsonify({'error': 'Token not found'}), 404

    data = request.get_json() or {}
    # 可指定新的总额度，不传则只清零已用
    if 'quota_total' in data:
        token.quota_total = int(data['quota_total'])
    token.quota_used = 0

    db.session.commit()

    return jsonify({
        'message': '配额已重置',
        'token': token.to_dict(),
    })


@tokens_bp.route('/<int:token_id>/usage')
def token_usage(token_id):
    """Token 用量明细"""
    hours = request.args.get('hours', 168, type=int)  # 默认7天

    from models.wr_models import RequestLog
    from sqlalchemy import func

    # 按模型聚合
    model_stats = db.session.query(
        RequestLog.model_name,
        func.count(RequestLog.id).label('requests'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
    ).filter(
        RequestLog.token_id == token_id,
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(RequestLog.model_name).order_by(func.sum(RequestLog.cost_cents).desc()).all()

    # 按日聚合
    daily_stats = db.session.query(
        func.date(RequestLog.created_at).label('date'),
        func.count(RequestLog.id).label('requests'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
    ).filter(
        RequestLog.token_id == token_id,
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(func.date(RequestLog.created_at)).order_by('date').all()

    return jsonify({
        'token_id': token_id,
        'hours': hours,
        'by_model': [{
            'model': r.model_name,
            'requests': r.requests,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
        } for r in model_stats],
        'daily': [{
            'date': str(r.date),
            'requests': r.requests,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
        } for r in daily_stats],
    })


@tokens_bp.route('/<int:token_id>/cost')
def token_cost(token_id):
    """Token 成本明细"""
    days = request.args.get('days', 30, type=int)

    from models.wr_models import RequestLog
    from sqlalchemy import func

    records = db.session.query(
        func.date(RequestLog.created_at).label('date'),
        RequestLog.model_name,
        func.count(RequestLog.id).label('requests'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
    ).filter(
        RequestLog.token_id == token_id,
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(func.date(RequestLog.created_at), RequestLog.model_name).order_by('date').all()

    return jsonify({
        'token_id': token_id,
        'days': days,
        'data': [{
            'date': str(r.date),
            'model': r.model_name,
            'requests': r.requests,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
        } for r in records],
    })
