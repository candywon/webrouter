"""模型分级管理 API — 智能选模型 (auto/smart) 的分级配置"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import ModelGrade
from extensions import db

modelgrades_bp = Blueprint('modelgrades', __name__)

VALID_TIERS = ['economy', 'standard', 'premium']


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
        return jsonify({'error': 'model 字段必填'}), 400

    tier = data.get('tier', '').strip()
    if tier not in VALID_TIERS:
        return jsonify({'error': f'tier 必须是 {VALID_TIERS} 之一'}), 400

    existing = ModelGrade.query.filter_by(model=data['model']).first()
    if existing:
        return jsonify({'error': f'Model {data["model"]} 已存在，请用 PUT 更新'}), 409

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
    return jsonify({'message': '模型分级创建成功', 'grade': g.to_dict()}), 201


@modelgrades_bp.route('/<string:model_name>', methods=['PUT'])
def update_grade(model_name):
    """更新模型分级"""
    g = ModelGrade.query.filter_by(model=model_name).first()
    if not g:
        return jsonify({'error': f'Model {model_name} not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    if 'tier' in data:
        if data['tier'] not in VALID_TIERS:
            return jsonify({'error': f'tier 必须是 {VALID_TIERS} 之一'}), 400
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
    return jsonify({'message': '模型分级更新成功', 'grade': g.to_dict()})


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
            {'value': 'economy', 'label': '经济型', 'description': '便宜快速，适合简单任务'},
            {'value': 'standard', 'label': '标准型', 'description': '中等性价比，适合日常任务'},
            {'value': 'premium', 'label': '旗舰型', 'description': '最强推理，适合复杂任务'},
        ],
    })


@modelgrades_bp.route('/reload', methods=['POST'])
def reload_grades():
    """手动触发 wr-proxy 刷新模型分级缓存"""
    result = _notify_proxy_reload()
    return jsonify({
        'message': '刷新请求已发送',
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
