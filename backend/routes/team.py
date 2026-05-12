"""团队管理API"""
from flask import Blueprint, jsonify, request
from models.wr_models import TeamQuota
from models.newapi_adapter import NewAPIAdapter
from extensions import db

team_bp = Blueprint('team', __name__)


@team_bp.route('/members')
def list_members():
    """成员列表（含额度）"""
    try:
        users = NewAPIAdapter.get_users()
    except Exception:
        users = []

    for u in users:
        quota = TeamQuota.query.filter_by(user_id=u['id']).first()
        u['quota'] = quota.to_dict() if quota else None

    return jsonify({'members': users})


@team_bp.route('/members', methods=['POST'])
def add_member():
    """邀请/添加成员并分配额度"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    user_id = data.get('user_id')
    quota_total = data.get('quota_total', 0)

    existing = TeamQuota.query.filter_by(user_id=user_id).first()
    if existing:
        return jsonify({'error': 'Member already exists'}), 409

    quota = TeamQuota(
        user_id=user_id,
        quota_total=quota_total,
        period=data.get('period', 'monthly'),
    )
    db.session.add(quota)
    db.session.commit()
    return jsonify(quota.to_dict()), 201


@team_bp.route('/members/<int:user_id>', methods=['PUT'])
def update_member(user_id):
    """更新成员额度"""
    data = request.get_json()
    quota = TeamQuota.query.filter_by(user_id=user_id).first()
    if not quota:
        return jsonify({'error': 'Not found'}), 404

    if 'quota_total' in data:
        quota.quota_total = data['quota_total']
    if 'period' in data:
        quota.period = data['period']
    if 'reset_at' in data:
        quota.reset_at = data['reset_at']

    db.session.commit()
    return jsonify(quota.to_dict())


@team_bp.route('/members/<int:user_id>', methods=['DELETE'])
def remove_member(user_id):
    """移除成员"""
    quota = TeamQuota.query.filter_by(user_id=user_id).first()
    if not quota:
        return jsonify({'error': 'Not found'}), 404
    db.session.delete(quota)
    db.session.commit()
    return jsonify({'deleted': user_id})


@team_bp.route('/members/<int:user_id>/usage')
def member_usage(user_id):
    """成员用量"""
    hours = request.args.get('hours', 168, type=int)  # 默认7天
    try:
        stats = NewAPIAdapter.get_usage_stats(hours=hours)
        # 按用户过滤（简化版，实际需从logs表关联user_id）
        return jsonify({'user_id': user_id, 'hours': hours, 'data': stats})
    except Exception:
        return jsonify({'user_id': user_id, 'hours': hours, 'data': []})
