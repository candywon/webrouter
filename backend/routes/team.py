"""团队管理API"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import TeamQuota
from extensions import db

team_bp = Blueprint('team', __name__)


@team_bp.route('/members')
def members():
    """成员列表"""
    try:
        from models.newapi_adapter import NewAPIAdapter
        users = NewAPIAdapter.get_users()
    except Exception:
        users = []

    # 附加额度信息
    result = []
    for u in users:
        quota = TeamQuota.query.filter_by(user_id=u['id']).first()
        member = dict(u)
        member['quota'] = quota.to_dict() if quota else None
        result.append(member)

    # 无数据则返回demo
    if not result:
        from services.demo_data import get_demo_team
        return jsonify({'members': get_demo_team()})

    return jsonify({'members': result})


@team_bp.route('/members', methods=['POST'])
def invite_member():
    """邀请成员"""
    data = request.get_json()
    if not data or not data.get('username'):
        return jsonify({'error': 'Missing username'}), 400
    # TODO: 调用New-API创建用户
    return jsonify({'message': '邀请已发送'}), 201


@team_bp.route('/members/<int:user_id>', methods=['PUT'])
def update_member(user_id):
    """更新成员额度"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    quota = TeamQuota.query.filter_by(user_id=user_id).first()
    if not quota:
        quota = TeamQuota(user_id=user_id)
        db.session.add(quota)

    if 'quota_total' in data:
        quota.quota_total = data['quota_total']
    if 'period' in data:
        quota.period = data['period']

    db.session.commit()
    return jsonify(quota.to_dict())


@team_bp.route('/members/<int:user_id>', methods=['DELETE'])
def remove_member(user_id):
    """移除成员"""
    quota = TeamQuota.query.filter_by(user_id=user_id).first()
    if quota:
        db.session.delete(quota)
        db.session.commit()
    # TODO: 调用New-API禁用用户
    return jsonify({'removed': user_id})


@team_bp.route('/members/<int:user_id>/usage')
def member_usage(user_id):
    """成员用量"""
    from models.wr_models import CostRecord
    from sqlalchemy import func as sql_func

    days = request.args.get('days', 30, type=int)
    records = db.session.query(
        CostRecord.model_name,
        sql_func.sum(CostRecord.input_tokens),
        sql_func.sum(CostRecord.output_tokens),
        sql_func.sum(CostRecord.cost_cents),
    ).filter(
        CostRecord.user_id == user_id,
        CostRecord.recorded_at >= sql_func.date('now', f'-{days} days'),
    ).group_by(CostRecord.model_name).all()

    data = [{
        'model_name': r[0],
        'input_tokens': r[1] or 0,
        'output_tokens': r[2] or 0,
        'cost_cents': r[3] or 0,
    } for r in records]

    return jsonify({'user_id': user_id, 'days': days, 'data': data})
