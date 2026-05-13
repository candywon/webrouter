"""Provider 管理 API — 数据源的 CRUD 和健康检测"""
import json
from flask import Blueprint, jsonify, request
from models.provider import Provider
from models.wr_models import ChannelHealth
from extensions import db

providers_bp = Blueprint('providers', __name__)


@providers_bp.route('/')
def list_providers():
    """获取所有 Provider 列表"""
    providers = Provider.query.order_by(Provider.priority.desc(), Provider.id.asc()).all()
    return jsonify({
        'providers': [p.to_dict() for p in providers],
        'total': len(providers),
    })


@providers_bp.route('/types')
def provider_types():
    """获取支持的 Provider 类型定义"""
    return jsonify({
        'types': Provider.get_type_config(),
    })


@providers_bp.route('/<int:provider_id>')
def get_provider(provider_id):
    """获取单个 Provider 详情"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    result = provider.to_dict(include_secrets=False)

    # 如果是 newapi/oneapi 类型，附加渠道信息
    if provider.type in (Provider.TYPE_NEWAPI, Provider.TYPE_ONEAPI):
        from models.provider_factory import ProviderFactory
        adapter = ProviderFactory.create(provider.to_dict(include_secrets=True))
        result['channels'] = adapter.get_channels()
        result['users_count'] = len(adapter.get_users())

    # 如果是 litellm 类型，附加模型列表
    if provider.type == Provider.TYPE_LITELLM:
        from models.provider_factory import ProviderFactory
        adapter = ProviderFactory.create(provider.to_dict(include_secrets=True))
        result['discovered_models'] = adapter.get_models()

    return jsonify(result)


@providers_bp.route('/', methods=['POST'])
def create_provider():
    """注册新 Provider"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    # 必填字段校验
    name = data.get('name', '').strip()
    provider_type = data.get('type', '').strip()
    base_url = data.get('base_url', '').strip()

    if not name:
        return jsonify({'error': '名称不能为空'}), 400
    if provider_type not in Provider.VALID_TYPES:
        return jsonify({'error': f'不支持的类型: {provider_type}，可选: {Provider.VALID_TYPES}'}), 400
    if not base_url:
        return jsonify({'error': 'Base URL 不能为空'}), 400

    provider = Provider(
        name=name,
        type=provider_type,
        base_url=base_url,
    )

    # 通用字段
    api_key = data.get('api_key', '').strip()
    if api_key:
        provider.api_key = api_key
        provider.api_key_masked = provider.mask_api_key(api_key)

    # 类型特定字段
    if provider_type in (Provider.TYPE_NEWAPI, Provider.TYPE_ONEAPI):
        admin_token = data.get('admin_token', '').strip()
        if admin_token:
            provider.admin_token = admin_token
        db_uri = data.get('db_uri', '').strip()
        if db_uri:
            provider.db_uri = db_uri

    if provider_type == Provider.TYPE_LITELLM:
        master_key = data.get('master_key', '').strip()
        if master_key:
            provider.master_key = master_key

    if provider_type == Provider.TYPE_CUSTOM:
        health_endpoint = data.get('health_endpoint', '').strip()
        if health_endpoint:
            provider.health_endpoint = health_endpoint

    # 可选配置
    models = data.get('models')
    if models:
        provider.models = json.dumps(models) if isinstance(models, list) else models

    tags = data.get('tags')
    if tags:
        provider.tags = json.dumps(tags) if isinstance(tags, list) else tags

    provider.weight = data.get('weight', 100)
    provider.priority = data.get('priority', 0)
    provider.check_interval = data.get('check_interval', 300)
    provider.enabled = data.get('enabled', True)
    provider.notes = data.get('notes', '')

    db.session.add(provider)
    db.session.commit()

    return jsonify({
        'message': 'Provider 创建成功',
        'provider': provider.to_dict(),
    }), 201


@providers_bp.route('/<int:provider_id>', methods=['PUT'])
def update_provider(provider_id):
    """更新 Provider 配置"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    # 可更新字段
    if 'name' in data:
        provider.name = data['name'].strip()
    if 'base_url' in data:
        provider.base_url = data['base_url'].strip()
    if 'api_key' in data:
        api_key = data['api_key'].strip()
        provider.api_key = api_key
        provider.api_key_masked = provider.mask_api_key(api_key)
    if 'admin_token' in data:
        provider.admin_token = data['admin_token'].strip()
    if 'db_uri' in data:
        provider.db_uri = data['db_uri'].strip()
    if 'master_key' in data:
        provider.master_key = data['master_key'].strip()
    if 'health_endpoint' in data:
        provider.health_endpoint = data['health_endpoint'].strip()
    if 'models' in data:
        models = data['models']
        provider.models = json.dumps(models) if isinstance(models, list) else models
    if 'tags' in data:
        tags = data['tags']
        provider.tags = json.dumps(tags) if isinstance(tags, list) else tags
    if 'weight' in data:
        provider.weight = int(data['weight'])
    if 'priority' in data:
        provider.priority = int(data['priority'])
    if 'check_interval' in data:
        provider.check_interval = int(data['check_interval'])
    if 'enabled' in data:
        provider.enabled = bool(data['enabled'])
        if not provider.enabled:
            provider.status = Provider.STATUS_DISABLED
    if 'notes' in data:
        provider.notes = data['notes']

    db.session.commit()

    return jsonify({
        'message': 'Provider 更新成功',
        'provider': provider.to_dict(),
    })


@providers_bp.route('/<int:provider_id>', methods=['DELETE'])
def delete_provider(provider_id):
    """删除 Provider"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    # 同时删除关联的健康记录
    ChannelHealth.query.filter_by(provider_id=provider_id).delete()

    db.session.delete(provider)
    db.session.commit()

    return jsonify({'message': 'Provider 已删除'})


@providers_bp.route('/<int:provider_id>/check', methods=['POST'])
def check_provider(provider_id):
    """手动触发单个 Provider 健康检测"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    from models.provider_factory import ProviderFactory
    from datetime import datetime

    adapter = ProviderFactory.create(provider.to_dict(include_secrets=True))
    result = adapter.check_health()

    # 更新 Provider 状态
    provider.status = result.get('status', 'unknown')
    provider.last_check_at = datetime.utcnow()
    provider.last_latency_ms = result.get('latency_ms')
    provider.last_error = result.get('error')
    provider.updated_at = datetime.utcnow()

    # 写入健康历史
    health = ChannelHealth(
        provider_id=provider.id,
        status=result.get('status', 'unknown'),
        latency_ms=result.get('latency_ms'),
        error_message=result.get('error'),
    )
    db.session.add(health)
    db.session.commit()

    result['provider_id'] = provider.id
    result['name'] = provider.name
    result['type'] = provider.type
    return jsonify(result)


@providers_bp.route('/check_all', methods=['POST'])
def check_all_providers():
    """手动触发全量 Provider 健康检测"""
    from services.health_checker import HealthChecker
    checker = HealthChecker()
    results = checker.check_all_providers()
    return jsonify({
        'results': results,
        'total': len(results),
    })


@providers_bp.route('/<int:provider_id>/channels')
def provider_channels(provider_id):
    """获取 Provider 下的渠道列表（仅 newapi/oneapi 类型）"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    if provider.type not in (Provider.TYPE_NEWAPI, Provider.TYPE_ONEAPI):
        return jsonify({'channels': [], 'message': '该类型 Provider 不支持渠道列表'})

    from models.provider_factory import ProviderFactory
    adapter = ProviderFactory.create(provider.to_dict(include_secrets=True))
    channels = adapter.get_channels()

    return jsonify({
        'channels': channels,
        'total': len(channels),
    })


@providers_bp.route('/<int:provider_id>/models')
def provider_models(provider_id):
    """获取 Provider 支持的模型列表"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    from models.provider_factory import ProviderFactory
    adapter = ProviderFactory.create(provider.to_dict(include_secrets=True))
    models = adapter.get_models()

    return jsonify({
        'models': models,
        'total': len(models),
    })


@providers_bp.route('/detect', methods=['POST'])
def auto_detect():
    """根据 Base URL 自动检测 Provider 类型"""
    data = request.get_json()
    base_url = (data or {}).get('base_url', '').strip()
    if not base_url:
        return jsonify({'error': 'Base URL 不能为空'}), 400

    from models.provider_factory import ProviderFactory
    detected_type = ProviderFactory.auto_detect_type(base_url)
    type_config = Provider.get_type_config(detected_type)

    return jsonify({
        'detected_type': detected_type,
        'type_config': type_config,
    })
