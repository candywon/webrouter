"""告警API"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import AlertRule, AlertHistory
from extensions import db

alert_bp = Blueprint('alert', __name__)


@alert_bp.route('/rules')
def list_rules():
    """告警规则列表"""
    rules = AlertRule.query.order_by(AlertRule.id.desc()).all()
    return jsonify({'rules': [r.to_dict() for r in rules]})


@alert_bp.route('/rules', methods=['POST'])
def create_rule():
    """创建告警规则"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    rule = AlertRule(
        name=data.get('name', ''),
        condition_type=data.get('condition_type', ''),
        condition_config=json.dumps(data.get('condition_config', {})),
        level=data.get('level', 'warning'),
        channels=json.dumps(data.get('channels', [])),
        enabled=data.get('enabled', True),
    )
    db.session.add(rule)
    db.session.commit()
    return jsonify(rule.to_dict()), 201


@alert_bp.route('/rules/<int:rule_id>', methods=['PUT'])
def update_rule(rule_id):
    """更新告警规则"""
    rule = AlertRule.query.get(rule_id)
    if not rule:
        return jsonify({'error': 'Not found'}), 404

    data = request.get_json()
    if 'name' in data:
        rule.name = data['name']
    if 'condition_type' in data:
        rule.condition_type = data['condition_type']
    if 'condition_config' in data:
        rule.condition_config = json.dumps(data['condition_config'])
    if 'level' in data:
        rule.level = data['level']
    if 'channels' in data:
        rule.channels = json.dumps(data['channels'])
    if 'enabled' in data:
        rule.enabled = data['enabled']

    db.session.commit()
    return jsonify(rule.to_dict())


@alert_bp.route('/rules/<int:rule_id>', methods=['DELETE'])
def delete_rule(rule_id):
    """删除告警规则"""
    rule = AlertRule.query.get(rule_id)
    if not rule:
        return jsonify({'error': 'Not found'}), 404
    db.session.delete(rule)
    db.session.commit()
    return jsonify({'deleted': rule_id})


@alert_bp.route('/history')
def alert_history():
    """告警历史"""
    limit = request.args.get('limit', 50, type=int)
    records = AlertHistory.query.order_by(
        AlertHistory.created_at.desc()
    ).limit(limit).all()
    return jsonify({'history': [r.to_dict() for r in records]})
