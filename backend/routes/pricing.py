# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""模型定价管理 API — 支持 CRUD + 批量导入 + 通知 wr-proxy 刷新"""
import json
from flask import Blueprint, jsonify, request
from models.wr_models import ModelPricing
from extensions import db
from i18n.messages import get_message

pricing_bp = Blueprint('pricing', __name__)


@pricing_bp.route('/')
def list_pricing():
    """定价表列表，支持按 vendor 过滤"""
    vendor = request.args.get('vendor', '')
    q = ModelPricing.query
    if vendor:
        q = q.filter_by(vendor=vendor)
    items = q.order_by(ModelPricing.vendor, ModelPricing.model).all()
    return jsonify({
        'pricing': [p.to_dict() for p in items],
        'total': len(items),
    })


@pricing_bp.route('/<string:model_name>')
def get_pricing(model_name):
    """查询单个模型定价"""
    p = ModelPricing.query.filter_by(model=model_name).first()
    if not p:
        return jsonify({'error': f'Model {model_name} not found'}), 404
    return jsonify(p.to_dict())


@pricing_bp.route('/', methods=['POST'])
def create_pricing():
    """新增模型定价"""
    data = request.get_json()
    if not data or not data.get('model'):
        return jsonify({'error': get_message('field_required_model', request)}), 400

    existing = ModelPricing.query.filter_by(model=data['model']).first()
    if existing:
        return jsonify({'error': get_message('pricing_already_exists', request).format(model=data["model"])}), 409

    p = ModelPricing(
        model=data['model'],
        input_price=float(data.get('input_price', 0)),
        output_price=float(data.get('output_price', 0)),
        vendor=data.get('vendor', 'other'),
        is_default=data.get('is_default', False),
        notes=data.get('notes', ''),
    )
    db.session.add(p)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('pricing_created', request), 'pricing': p.to_dict()}), 201


@pricing_bp.route('/<string:model_name>', methods=['PUT'])
def update_pricing(model_name):
    """更新模型定价"""
    p = ModelPricing.query.filter_by(model=model_name).first()
    if not p:
        return jsonify({'error': f'Model {model_name} not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    if 'input_price' in data:
        p.input_price = float(data['input_price'])
    if 'output_price' in data:
        p.output_price = float(data['output_price'])
    if 'vendor' in data:
        p.vendor = data['vendor']
    if 'is_default' in data:
        # 只允许一个默认定价
        if data['is_default']:
            ModelPricing.query.filter(ModelPricing.is_default == True, ModelPricing.model != model_name).update({'is_default': False})
        p.is_default = bool(data['is_default'])
    if 'notes' in data:
        p.notes = data['notes']

    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('pricing_updated', request), 'pricing': p.to_dict()})


@pricing_bp.route('/<string:model_name>', methods=['DELETE'])
def delete_pricing(model_name):
    """删除模型定价"""
    p = ModelPricing.query.filter_by(model=model_name).first()
    if not p:
        return jsonify({'error': f'Model {model_name} not found'}), 404

    if p.is_default:
        return jsonify({'error': get_message('cannot_delete_default_pricing', request)}), 400

    db.session.delete(p)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'deleted': model_name})


@pricing_bp.route('/batch', methods=['POST'])
def batch_update():
    """批量更新定价（用于价格调整、定时同步）"""
    data = request.get_json()
    if not data or 'items' not in data:
        return jsonify({'error': get_message('field_required_items_array', request)}), 400

    items = data['items']
    created = 0
    updated = 0

    for item in items:
        model = item.get('model', '')
        if not model:
            continue

        p = ModelPricing.query.filter_by(model=model).first()
        if p:
            # 更新
            if 'input_price' in item:
                p.input_price = float(item['input_price'])
            if 'output_price' in item:
                p.output_price = float(item['output_price'])
            if 'vendor' in item:
                p.vendor = item['vendor']
            if 'notes' in item:
                p.notes = item['notes']
            updated += 1
        else:
            # 新增
            p = ModelPricing(
                model=model,
                input_price=float(item.get('input_price', 0)),
                output_price=float(item.get('output_price', 0)),
                vendor=item.get('vendor', 'other'),
                notes=item.get('notes', ''),
            )
            db.session.add(p)
            created += 1

    db.session.commit()
    _notify_proxy_reload()

    return jsonify({
        'message': get_message('pricing_batch_update_done', request).format(created=created, updated=updated),
        'created': created,
        'updated': updated,
    })


@pricing_bp.route('/vendors')
def list_vendors():
    """列出所有厂商"""
    from sqlalchemy import func
    vendors = db.session.query(
        ModelPricing.vendor,
        func.count(ModelPricing.id).label('model_count'),
    ).group_by(ModelPricing.vendor).all()

    return jsonify({
        'vendors': [{'name': v[0], 'model_count': v[1]} for v in vendors],
    })


@pricing_bp.route('/reload', methods=['POST'])
def reload_pricing():
    """手动触发 wr-proxy 刷新定价缓存"""
    result = _notify_proxy_reload()
    return jsonify({
        'message': get_message('refresh_sent', request),
        'proxy_response': result,
    })


def _notify_proxy_reload():
    """通知 wr-proxy 重新加载定价（从 DB 刷新缓存）"""
    try:
        import requests
        resp = requests.post('http://localhost:5051/admin/reload_pricing', timeout=5)
        return {'status': resp.status_code, 'body': resp.json() if resp.headers.get('content-type', '').startswith('application/json') else resp.text}
    except Exception as e:
        return {'status': 'error', 'message': str(e)}
