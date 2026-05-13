"""团队管理API — 基于 WR Token 的团队成员管理"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import WRToken, TeamQuota
from extensions import db

team_bp = Blueprint('team', __name__)


@team_bp.route('/members')
def members():
    """成员列表 — 每个 WR Token 即为一个团队成员"""
    tokens = WRToken.query.filter_by(enabled=True).all()
    result = []
    for t in tokens:
        member = {
            'id': t.id,
            'name': t.name,
            'key_prefix': t.key[:8] + '...' if t.key else '',
            'enabled': t.enabled,
            'quota_total': t.quota_total,
            'quota_used': t.quota_used,
            'expires_at': t.expires_at.isoformat() if t.expires_at else None,
        }
        result.append(member)
    return jsonify({'members': result})


@team_bp.route('/members', methods=['POST'])
def invite_member():
    """创建新成员（即创建新 WR Token）"""
    data = request.get_json()
    if not data or not data.get('name'):
        return jsonify({'error': 'Missing name'}), 400

    token = WRToken(
        name=data['name'],
        quota_total=data.get('quota_total', 0),
        allowed_models=data.get('allowed_models', ''),
        ip_whitelist=data.get('ip_whitelist', ''),
    )
    if data.get('expires_at'):
        from datetime import datetime
        token.expires_at = datetime.fromisoformat(data['expires_at'])
    token.generate_key()

    db.session.add(token)
    db.session.commit()
    return jsonify({
        'message': '成员已创建',
        'id': token.id,
        'key': token.key,  # 仅创建时返回完整 key
    }), 201


@team_bp.route('/members/<int:member_id>', methods=['PUT'])
def update_member(member_id):
    """更新成员额度/配置"""
    token = WRToken.query.get(member_id)
    if not token:
        return jsonify({'error': 'Member not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    for field in ['name', 'quota_total', 'allowed_models', 'ip_whitelist', 'enabled']:
        if field in data:
            setattr(token, field, data[field])
    if 'expires_at' in data and data['expires_at']:
        from datetime import datetime
        token.expires_at = datetime.fromisoformat(data['expires_at'])

    db.session.commit()
    return jsonify({
        'id': token.id,
        'name': token.name,
        'quota_total': token.quota_total,
        'enabled': token.enabled,
    })


@team_bp.route('/members/<int:member_id>', methods=['DELETE'])
def remove_member(member_id):
    """禁用/删除成员"""
    token = WRToken.query.get(member_id)
    if token:
        token.enabled = False
        db.session.commit()
    return jsonify({'removed': member_id})


@team_bp.route('/members/<int:member_id>/usage')
def member_usage(member_id):
    """成员用量 — 从 wr_request_logs 查询"""
    from models.wr_models import RequestLog
    from sqlalchemy import func as sql_func

    days = request.args.get('days', 30, type=int)
    records = db.session.query(
        RequestLog.model,
        sql_func.sum(RequestLog.input_tokens),
        sql_func.sum(RequestLog.output_tokens),
        sql_func.sum(RequestLog.cost_cents),
    ).filter(
        RequestLog.token_id == member_id,
        RequestLog.created_at >= sql_func.date('now', f'-{days} days'),
    ).group_by(RequestLog.model).all()

    data = [{
        'model': r[0],
        'input_tokens': r[1] or 0,
        'output_tokens': r[2] or 0,
        'cost_cents': r[3] or 0,
    } for r in records]

    return jsonify({'member_id': member_id, 'days': days, 'data': data})
