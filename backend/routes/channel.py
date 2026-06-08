# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""Provider Channel 渠道管理 API"""
import json
from flask import Blueprint, jsonify, request
from models.channel import ProviderChannel
from models.provider import Provider
from models.wr_models import ProviderExt
from extensions import db
from i18n.messages import get_message

channel_bp = Blueprint('channel', __name__)


@channel_bp.route('/<int:provider_id>/channels')
def list_channels(provider_id):
    """列出某 Provider 下的所有渠道"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': get_message('provider_not_found', request)}), 404

    channels = ProviderChannel.query.filter_by(provider_id=provider_id).order_by(
        ProviderChannel.priority.desc(), ProviderChannel.id
    ).all()

    # 解析继承后的实际值
    ext = ProviderExt.query.get(provider_id)
    result = []
    for ch in channels:
        d = ch.to_dict()
        d['resolved_base_url'] = ch.resolve_base_url(provider)
        d['resolved_models'] = ch.resolve_models(provider)
        d['resolved_priority'] = ch.resolve_priority(ext)
        d['resolved_weight'] = ch.resolve_weight(ext)
        d['resolved_cost_multiplier'] = ch.resolve_cost_multiplier(ext)
        result.append(d)

    # 附加 Provider 自身的默认配置（作为"默认渠道"）
    default_channel = {
        'id': 0,
        'provider_id': provider_id,
        'name': '(Provider default)',
        'base_url': provider.base_url,
        'api_key_masked': provider.api_key_masked or '***',
        'models': provider.models_list,
        'priority': ext.priority if ext else 50,
        'weight': ext.weight if ext else 100,
        'status': provider.status or 'unchecked',
        'enabled': provider.enabled,
        'is_default': True,
    }

    return jsonify({
        'provider': {'id': provider.id, 'name': provider.name, 'type': provider.type},
        'default_channel': default_channel,
        'channels': result,
        'total': len(result),
    })


@channel_bp.route('/<int:provider_id>/channels', methods=['POST'])
def create_channel(provider_id):
    """为 Provider 新增渠道"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': get_message('provider_not_found', request)}), 404

    data = request.get_json()
    if not data or not data.get('name'):
        return jsonify({'error': get_message('channel_name_required', request)}), 400

    ch = ProviderChannel(
        provider_id=provider_id,
        name=data['name'].strip(),
        base_url=data.get('base_url', ''),
        api_key=data.get('api_key', ''),
        models=json.dumps(data['models']) if isinstance(data.get('models'), list) else data.get('models', ''),
        priority=data.get('priority', 0),
        weight=data.get('weight', 0),
        rate_limit_rpm=data.get('rate_limit_rpm', 0),
        cost_multiplier=data.get('cost_multiplier', 0),
        enabled=data.get('enabled', True),
        notes=data.get('notes', ''),
    )
    db.session.add(ch)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('channel_created', request), 'channel': ch.to_dict(include_secrets=True)}), 201


@channel_bp.route('/<int:provider_id>/channels/<int:channel_id>', methods=['PUT'])
def update_channel(provider_id, channel_id):
    """更新渠道配置"""
    ch = ProviderChannel.query.filter_by(id=channel_id, provider_id=provider_id).first()
    if not ch:
        return jsonify({'error': 'Channel not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    for field in ['name', 'base_url', 'api_key', 'notes']:
        if field in data:
            setattr(ch, field, data[field])
    # 更新 Key 后重置状态
    if 'api_key' in data and ch.status in ('auth_failed', 'dead'):
        ch.status = 'unchecked'
    if 'models' in data:
        m = data['models']
        ch.models = json.dumps(m) if isinstance(m, list) else m
    for field in ['priority', 'weight', 'rate_limit_rpm']:
        if field in data:
            setattr(ch, field, int(data[field]))
    if 'cost_multiplier' in data:
        ch.cost_multiplier = float(data['cost_multiplier'])
    if 'enabled' in data:
        ch.enabled = bool(data['enabled'])

    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('channel_updated', request), 'channel': ch.to_dict()})


@channel_bp.route('/<int:provider_id>/channels/<int:channel_id>', methods=['DELETE'])
def delete_channel(provider_id, channel_id):
    """删除渠道"""
    ch = ProviderChannel.query.filter_by(id=channel_id, provider_id=provider_id).first()
    if not ch:
        return jsonify({'error': 'Channel not found'}), 404

    db.session.delete(ch)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'deleted': channel_id})


@channel_bp.route('/<int:provider_id>/channels/batch', methods=['POST'])
def batch_create_channels(provider_id):
    """批量创建渠道（一次性添加多个 Key）"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': get_message('provider_not_found', request)}), 404

    data = request.get_json()
    if not data or 'channels' not in data:
        return jsonify({'error': get_message('field_required_channels_array', request)}), 400

    created = []
    for item in data['channels']:
        if not item.get('name'):
            continue
        ch = ProviderChannel(
            provider_id=provider_id,
            name=item['name'].strip(),
            base_url=item.get('base_url', ''),
            api_key=item.get('api_key', ''),
            models=json.dumps(item['models']) if isinstance(item.get('models'), list) else item.get('models', ''),
            priority=item.get('priority', 0),
            weight=item.get('weight', 0),
            rate_limit_rpm=item.get('rate_limit_rpm', 0),
            cost_multiplier=item.get('cost_multiplier', 0),
            enabled=item.get('enabled', True),
            notes=item.get('notes', ''),
        )
        db.session.add(ch)
        created.append(ch)

    db.session.commit()
    _notify_proxy_reload()

    return jsonify({
        'message': get_message('channel_batch_created', request).format(len=len(created)),
        'created': len(created),
        'channels': [ch.to_dict(include_secrets=True) for ch in created],
    }), 201


def _notify_proxy_reload():
    """通知 wr-proxy 重新加载 Provider+Channel"""
    try:
        import requests
        resp = requests.post('http://localhost:5051/admin/reload', timeout=5)
        return resp.json()
    except Exception:
        return None
