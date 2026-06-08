# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""模型别名管理 API — 短名称 → 完整模型名映射"""
import json
from flask import Blueprint, jsonify, request
from extensions import db
from i18n.messages import get_message

modelaliases_bp = Blueprint('modelaliases', __name__)


@modelaliases_bp.route('/')
def list_aliases():
    """模型别名列表"""
    items = db.session.execute(
        db.text('SELECT id, alias, target, enabled, created_at FROM wr_model_aliases ORDER BY id ASC')
    ).fetchall()
    return jsonify({
        'aliases': [dict(row._mapping) for row in items],
        'total': len(items),
    })


@modelaliases_bp.route('/', methods=['POST'])
def create_alias():
    """新增模型别名"""
    data = request.get_json()
    if not data or not data.get('alias') or not data.get('target'):
        return jsonify({'error': get_message('field_required_alias_target', request)}), 400

    existing = db.session.execute(
        db.text('SELECT id FROM wr_model_aliases WHERE alias = :alias'),
        {'alias': data['alias']}
    ).fetchone()
    if existing:
        return jsonify({'error': get_message('pricing_already_exists', request).format(model=data["alias"])}), 409

    db.session.execute(
        db.text('INSERT INTO wr_model_aliases (alias, target, enabled) VALUES (:alias, :target, 1)'),
        {'alias': data['alias'], 'target': data['target']}
    )
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('model_alias_created', request), 'alias': data['alias'], 'target': data['target']}), 201


@modelaliases_bp.route('/<string:alias_name>', methods=['PUT'])
def update_alias(alias_name):
    """更新模型别名"""
    data = request.get_json()
    if not data or not data.get('target'):
        return jsonify({'error': get_message('field_required_target', request)}), 400

    result = db.session.execute(
        db.text('UPDATE wr_model_aliases SET target = :target WHERE alias = :alias'),
        {'alias': alias_name, 'target': data['target']}
    )
    db.session.commit()

    if result.rowcount == 0:
        return jsonify({'error': f'Alias {alias_name} not found'}), 404

    _notify_proxy_reload()
    return jsonify({'message': get_message('model_alias_updated', request), 'alias': alias_name, 'target': data['target']})


@modelaliases_bp.route('/<string:alias_name>', methods=['DELETE'])
def delete_alias(alias_name):
    """删除模型别名"""
    result = db.session.execute(
        db.text('DELETE FROM wr_model_aliases WHERE alias = :alias'),
        {'alias': alias_name}
    )
    db.session.commit()

    if result.rowcount == 0:
        return jsonify({'error': f'Alias {alias_name} not found'}), 404

    _notify_proxy_reload()
    return jsonify({'deleted': alias_name})


@modelaliases_bp.route('/reload', methods=['POST'])
def reload_aliases():
    """手动触发 wr-proxy 重新加载模型别名缓存"""
    result = _notify_proxy_reload()
    return jsonify({
        'message': get_message('refresh_sent', request),
        'proxy_response': result,
    })


def _notify_proxy_reload():
    """通知 wr-proxy 重新加载模型别名"""
    try:
        import requests
        resp = requests.post('http://localhost:5051/admin/reload', timeout=5)
        return {'status': resp.status_code, 'body': resp.json() if resp.headers.get('content-type', '').startswith('application/json') else resp.text}
    except Exception as e:
        return {'status': 'error', 'message': str(e)}
