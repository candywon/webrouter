# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""模型分级管理 API — 智能选模型 (auto/smart) 的分级配置"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import ModelGrade
from extensions import db
from i18n.messages import get_message

modelgrades_bp = Blueprint('modelgrades', __name__)

VALID_TIERS = ['economy', 'standard', 'enhanced', 'premium', 'flagship']


@modelgrades_bp.route('/')
def list_grades():
    """模型分级列表，支持按 tier 和 vendor 过滤"""
    tier = request.args.get('tier', '')
    vendor = request.args.get('vendor', '')
    q = ModelGrade.query
    if tier:
        q = q.filter_by(tier=tier)
    if vendor:
        q = q.filter_by(vendor=vendor)
    items = q.order_by(ModelGrade.sort_order, ModelGrade.id).all()
    return jsonify({
        'grades': [g.to_dict() for g in items],
        'total': len(items),
    })


@modelgrades_bp.route('/<string:model_name>')
def get_grade(model_name):
    """查询单个模型分级"""
    g = ModelGrade.query.filter_by(model=model_name).first()
    if not g:
        return jsonify({'error': f'Model {model_name} not found'}), 404
    return jsonify(g.to_dict())


@modelgrades_bp.route('/', methods=['POST'])
def create_grade():
    """新增模型分级"""
    data = request.get_json()
    if not data or not data.get('model'):
        return jsonify({'error': get_message('field_required_model', request)}), 400

    tier = data.get('tier', '').strip()
    if tier not in VALID_TIERS:
        return jsonify({'error': get_message('invalid_tier', request).format(VALID_TIERS=VALID_TIERS)}), 400

    existing = ModelGrade.query.filter_by(model=data['model']).first()
    if existing:
        return jsonify({'error': get_message('pricing_already_exists', request).format(model=data["model"])}), 409

    g = ModelGrade(
        model=data['model'],
        tier=tier,
        cost_index=float(data.get('cost_index', 1.0)),
        vendor=data.get('vendor', 'other'),
        description=data.get('description', ''),
        enabled=data.get('enabled', True),
        sort_order=int(data.get('sort_order', 0)),
    )
    db.session.add(g)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('model_grade_created', request), 'grade': g.to_dict()}), 201


@modelgrades_bp.route('/<string:model_name>', methods=['PUT'])
def update_grade(model_name):
    """更新模型分级"""
    g = ModelGrade.query.filter_by(model=model_name).first()
    if not g:
        return jsonify({'error': f'Model {model_name} not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    if 'tier' in data:
        if data['tier'] not in VALID_TIERS:
            return jsonify({'error': get_message('invalid_tier', request).format(VALID_TIERS=VALID_TIERS)}), 400
        g.tier = data['tier']
    if 'cost_index' in data:
        g.cost_index = float(data['cost_index'])
    if 'vendor' in data:
        g.vendor = data['vendor']
    if 'description' in data:
        g.description = data['description']
    if 'enabled' in data:
        g.enabled = bool(data['enabled'])
    if 'sort_order' in data:
        g.sort_order = int(data['sort_order'])

    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('model_grade_updated', request), 'grade': g.to_dict()})


@modelgrades_bp.route('/<string:model_name>', methods=['DELETE'])
def delete_grade(model_name):
    """删除模型分级"""
    g = ModelGrade.query.filter_by(model=model_name).first()
    if not g:
        return jsonify({'error': f'Model {model_name} not found'}), 404

    db.session.delete(g)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'deleted': model_name})


@modelgrades_bp.route('/tiers')
def list_tiers():
    """列出所有分级层级"""
    return jsonify({
        'tiers': [
            {'value': 'economy', 'label': 'Economy', 'description': 'Cheap & fast, for simple tasks (chat, translation, short Q&A)'},
            {'value': 'standard', 'label': 'Standard', 'description': 'Balanced, for daily tasks (writing, summarization, formatting)'},
            {'value': 'enhanced', 'label': 'Enhanced', 'description': 'Capable, for code generation, multi-step reasoning, documents'},
            {'value': 'premium', 'label': 'Premium', 'description': 'Powerful, for complex architecture, math proofs, long analysis'},
            {'value': 'flagship', 'label': 'Flagship', 'description': 'Top-tier, for research, competition, deep multimodal reasoning'},
        ],
    })


@modelgrades_bp.route('/reload', methods=['POST'])
def reload_grades():
    """手动触发 wr-proxy 刷新模型分级缓存"""
    result = _notify_proxy_reload()
    return jsonify({
        'message': get_message('refresh_sent', request),
        'proxy_response': result,
    })


def _notify_proxy_reload():
    """通知 wr-proxy 重新加载模型分级"""
    try:
        import requests
        resp = requests.post('http://localhost:5051/admin/reload_model_grades', timeout=5)
        return {'status': resp.status_code, 'body': resp.json() if resp.headers.get('content-type', '').startswith('application/json') else resp.text}
    except Exception as e:
        return {'status': 'error', 'message': str(e)}
