# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""Provider 管理 API — 数据源的 CRUD、健康检测、代理配置"""
import json
import requests as http
from flask import Blueprint, jsonify, request
from models.provider import Provider
from models.wr_models import ChannelHealth, ProviderExt, ProviderQuota
from extensions import db
from i18n.messages import get_message

providers_bp = Blueprint('providers', __name__)


def _auto_detect_api_format(base_url: str) -> str:
    """根据已知 vendor URL 模式推断上游 API 协议（仅返回格式，不返回命中规则）。
    用于 _provider_full_dict 给前端兜底显示，正式建议见 /detect 接口。
    """
    from models.provider_factory import ProviderFactory
    fmt, _ = ProviderFactory.auto_detect_api_format(base_url)
    return fmt


def _get_or_create_ext(provider_id):
    """获取或创建 Provider 扩展配置"""
    ext = ProviderExt.query.get(provider_id)
    if not ext:
        ext = ProviderExt(provider_id=provider_id)
        db.session.add(ext)
        db.session.flush()
    return ext


def _get_or_create_quota(provider_id):
    """获取或创建 Provider 额度记录"""
    quota = ProviderQuota.query.get(provider_id)
    if not quota:
        quota = ProviderQuota(provider_id=provider_id)
        db.session.add(quota)
        db.session.flush()
    return quota


def _provider_full_dict(provider, include_secrets=False):
    """Provider 完整信息（含扩展+额度）"""
    d = provider.to_dict(include_secrets=include_secrets)

    # 扩展字段
    ext = ProviderExt.query.get(provider.id)
    if ext:
        d.update(ext.to_dict())
    else:
        d.update({
            'proxy_enabled': True,
            'rate_limit_rpm': 0,
            'timeout_seconds': 30,
            'max_retries': 2,
            'cost_multiplier': 1.0,
            'priority': 50,
            'weight': 100,
            'supports_tools': True,
            'api_format': _auto_detect_api_format(provider.base_url),
        })

    # 额度信息
    quota = ProviderQuota.query.get(provider.id)
    if quota:
        d.update(quota.to_dict())

    # 预测信息
    if quota and (quota.quota_total or 0) > 0:
        d['prediction'] = _predict_quota(quota)

    return d


def _predict_quota(quota):
    """简单额度预测"""
    from models.wr_models import RequestLog
    from sqlalchemy import func
    from datetime import datetime, timedelta

    # 近7天日均消耗
    week_ago = datetime.utcnow() - timedelta(days=7)
    daily = db.session.query(
        func.coalesce(func.sum(RequestLog.cost_cents), 0),
    ).filter(
        RequestLog.provider_id == quota.provider_id,
        RequestLog.created_at >= week_ago,
    ).scalar() or 0

    daily_burn = daily / 7.0
    remaining = quota.quota_total - quota.quota_used

    if daily_burn <= 0:
        return {'days_until_exhaust': -1, 'trend': 'no_usage'}

    days_left = remaining / daily_burn
    exhaust_date = (datetime.utcnow() + timedelta(days=days_left)).strftime('%Y-%m-%d')

    # 趋势：比较前半和后半
    half = db.session.query(
        func.coalesce(func.sum(RequestLog.cost_cents), 0),
    ).filter(
        RequestLog.provider_id == quota.provider_id,
        RequestLog.created_at >= datetime.utcnow() - timedelta(days=3.5),
    ).scalar() or 0

    older = daily - half
    trend = 'stable'
    if older > 0 and half / older > 1.3:
        trend = 'increasing'
    elif older > 0 and half / older < 0.7:
        trend = 'decreasing'

    # 告警级别
    ratio = quota.quota_ratio
    if ratio <= 0:
        level = 'black'
    elif ratio < 0.05:
        level = 'red'
    elif ratio < 0.2 or days_left < 3:
        level = 'orange'
    elif ratio < 0.5 or days_left < 7:
        level = 'yellow'
    else:
        level = 'green'

    return {
        'daily_burn_rate': round(daily_burn, 1),
        'days_until_exhaust': round(days_left, 1),
        'predicted_exhaust_date': exhaust_date,
        'trend': trend,
        'alert_level': level,
    }


@providers_bp.route('/')
def list_providers():
    """获取所有 Provider 列表"""
    providers = Provider.query.order_by(Provider.priority.desc(), Provider.id.asc()).all()
    return jsonify({
        'providers': [_provider_full_dict(p) for p in providers],
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
        return jsonify({'error': get_message('provider_not_found', request)}), 404
    return jsonify(_provider_full_dict(provider, include_secrets=False))


@providers_bp.route('/', methods=['POST'])
def create_provider():
    """注册新 Provider"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    name = (data.get('name') or '').strip()
    provider_type = (data.get('type') or '').strip()
    base_url = (data.get('base_url') or '').strip()

    if not name:
        return jsonify({'error': get_message('name_required', request)}), 400
    if provider_type not in Provider.VALID_TYPES:
        return jsonify({'error': get_message('unsupported_type', request).format(provider_type=provider_type)}), 400
    if not base_url:
        return jsonify({'error': get_message('base_url_required', request)}), 400

    provider = Provider(name=name, type=provider_type, base_url=base_url)

    # 可选 Anthropic 兼容端点（双 URL 场景）
    anthropic_base_url = (data.get('anthropic_base_url') or '').strip()
    if anthropic_base_url:
        provider.anthropic_base_url = anthropic_base_url

    # 通用字段
    api_key = (data.get('api_key') or '').strip()
    if api_key:
        provider.api_key = api_key
        provider.api_key_masked = provider.mask_api_key(api_key)

    # 类型特定字段
    if provider_type == Provider.TYPE_LITELLM:
        master_key = (data.get('master_key') or '').strip()
        if master_key:
            provider.master_key = master_key

    if provider_type == Provider.TYPE_CUSTOM:
        health_endpoint = (data.get('health_endpoint') or '').strip()
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
    provider.priority = data.get('priority', 50)
    provider.check_interval = data.get('check_interval', 300)
    provider.enabled = data.get('enabled', True)
    provider.notes = data.get('notes', '')

    db.session.add(provider)
    db.session.flush()  # 拿到 provider.id

    # 创建扩展配置
    ext = ProviderExt(provider_id=provider.id)
    if 'proxy_enabled' in data:
        ext.proxy_enabled = bool(data['proxy_enabled'])
    if 'rate_limit_rpm' in data:
        ext.rate_limit_rpm = int(data['rate_limit_rpm'])
    if 'timeout_seconds' in data:
        ext.timeout_seconds = int(data['timeout_seconds'])
    if 'max_retries' in data:
        ext.max_retries = int(data['max_retries'])
    if 'cost_multiplier' in data:
        ext.cost_multiplier = float(data['cost_multiplier'])
    if 'priority' in data:
        ext.priority = int(data['priority'])
        provider.priority = ext.priority  # 同步到主表
    if 'weight' in data:
        ext.weight = int(data['weight'])
        provider.weight = ext.weight
    if 'supports_tools' in data:
        ext.supports_tools = bool(data['supports_tools'])
    if 'api_format' in data and data['api_format']:
        ext.api_format = str(data['api_format'])
    elif not ext.api_format:
        # 不再自动写入具体格式：保持 'auto'，由代理运行时按 vendor 规则兜底，
        # 用户可在 UI 下拉显式选择以覆盖。
        ext.api_format = 'auto'
    db.session.add(ext)

    # 创建额度记录（可选）
    if data.get('quota_total'):
        quota = ProviderQuota(
            provider_id=provider.id,
            quota_total=int(data['quota_total']),
            quota_source=data.get('quota_source', 'manual'),
        )
        db.session.add(quota)

    db.session.commit()

    # 通知 wr-proxy 刷新
    _notify_proxy_reload()

    return jsonify({
        'message': get_message('provider_created', request),
        'provider': _provider_full_dict(provider),
    }), 201


@providers_bp.route('/<int:provider_id>', methods=['PUT'])
def update_provider(provider_id):
    """更新 Provider 配置"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': get_message('provider_not_found', request)}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    # 主表字段
    if 'name' in data:
        provider.name = data['name'].strip()
    if 'base_url' in data:
        provider.base_url = data['base_url'].strip()
    if 'anthropic_base_url' in data:
        provider.anthropic_base_url = (data.get('anthropic_base_url') or '').strip()
    if 'api_key' in data:
        api_key = data['api_key'].strip()
        provider.api_key = api_key
        provider.api_key_masked = provider.mask_api_key(api_key)
        # 更新 Key 后重置状态，允许下次健康检测重新验证
        if provider.status in ('auth_failed', 'dead'):
            provider.status = 'unchecked'
    if 'models' in data:
        models = data['models']
        provider.models = json.dumps(models) if isinstance(models, list) else models
    if 'tags' in data:
        tags = data['tags']
        provider.tags = json.dumps(tags) if isinstance(tags, list) else tags
    if 'enabled' in data:
        provider.enabled = bool(data['enabled'])
        if not provider.enabled:
            provider.status = Provider.STATUS_DISABLED
    if 'notes' in data:
        provider.notes = data['notes']

    # 扩展字段
    ext = _get_or_create_ext(provider_id)
    if 'proxy_enabled' in data:
        ext.proxy_enabled = bool(data['proxy_enabled'])
    if 'rate_limit_rpm' in data:
        ext.rate_limit_rpm = int(data['rate_limit_rpm'])
    if 'timeout_seconds' in data:
        ext.timeout_seconds = int(data['timeout_seconds'])
    if 'max_retries' in data:
        ext.max_retries = int(data['max_retries'])
    if 'cost_multiplier' in data:
        ext.cost_multiplier = float(data['cost_multiplier'])
    if 'priority' in data:
        ext.priority = int(data['priority'])
        provider.priority = ext.priority
    if 'weight' in data:
        ext.weight = int(data['weight'])
        provider.weight = ext.weight
    if 'supports_tools' in data:
        ext.supports_tools = bool(data['supports_tools'])
    if 'api_format' in data:
        ext.api_format = str(data['api_format'] or 'auto')

    # 额度字段
    if 'quota_total' in data or 'quota_used' in data or 'quota_source' in data:
        quota = _get_or_create_quota(provider_id)
        if 'quota_total' in data:
            quota.quota_total = int(data['quota_total'])
        if 'quota_used' in data:
            quota.quota_used = int(data['quota_used'])
        if 'quota_source' in data:
            quota.quota_source = data['quota_source']

    db.session.commit()

    # 更新额度后自动清除冷却（如充值恢复）
    if 'quota_total' in data or 'quota_used' in data:
        _notify_clear_cooldown(provider_id)

    # 通知 wr-proxy 刷新
    _notify_proxy_reload()

    return jsonify({
        'message': get_message('provider_updated', request),
        'provider': _provider_full_dict(provider),
    })


@providers_bp.route('/<int:provider_id>', methods=['DELETE'])
def delete_provider(provider_id):
    """删除 Provider"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': get_message('provider_not_found', request)}), 404

    # 同时删除关联数据
    ChannelHealth.query.filter_by(provider_id=provider_id).delete()
    ProviderExt.query.filter_by(provider_id=provider_id).delete()
    ProviderQuota.query.filter_by(provider_id=provider_id).delete()

    db.session.delete(provider)
    db.session.commit()

    _notify_proxy_reload()
    return jsonify({'message': get_message('provider_deleted', request)})


@providers_bp.route('/<int:provider_id>/check', methods=['POST'])
def check_provider(provider_id):
    """手动触发单个 Provider 健康检测"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': get_message('provider_not_found', request)}), 404

    from models.provider_factory import ProviderFactory
    from datetime import datetime

    adapter = ProviderFactory.create(provider.to_dict(include_secrets=True))
    result = adapter.check_health()

    # 更新状态
    provider.status = result.get('status', 'unknown')
    provider.last_check_at = datetime.utcnow()
    provider.last_latency_ms = result.get('latency_ms')
    provider.last_error = result.get('error')
    provider.updated_at = datetime.utcnow()

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


@providers_bp.route('/<int:provider_id>/quota', methods=['PUT'])
def update_quota(provider_id):
    """更新 Provider 额度"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    quota = _get_or_create_quota(provider_id)
    if 'quota_total' in data:
        quota.quota_total = int(data['quota_total'])
    if 'quota_used' in data:
        quota.quota_used = int(data['quota_used'])
    if 'quota_source' in data:
        quota.quota_source = data['quota_source']

    db.session.commit()

    # 充值后自动清除冷却
    _notify_clear_cooldown(provider_id)

    _notify_proxy_reload()
    return jsonify({
        'message': get_message('quota_updated', request),
        'quota': quota.to_dict(),
    })


@providers_bp.route('/detect', methods=['POST'])
def auto_detect():
    """根据 Base URL 自动检测 Provider 类型"""
    data = request.get_json()
    base_url = (data or {}).get('base_url', '').strip()
    if not base_url:
        return jsonify({'error': get_message('base_url_required', request)}), 400

    from models.provider_factory import ProviderFactory
    detected_type = ProviderFactory.auto_detect_type(base_url)
    detected_anthropic_url = ProviderFactory.auto_detect_anthropic_url(base_url)
    suggested_api_format, matched_pattern = ProviderFactory.auto_detect_api_format(base_url)
    type_config = Provider.get_type_config(detected_type)

    return jsonify({
        'detected_type': detected_type,
        'detected_anthropic_base_url': detected_anthropic_url,
        # 'suggested_*' 表明这是建议而非强制——用户可在 UI 覆盖
        'suggested_api_format': suggested_api_format,
        'matched_pattern': matched_pattern,
        # 兼容旧前端字段名（已废弃，新代码用 suggested_api_format）
        'detected_api_format': suggested_api_format,
        'type_config': type_config,
    })


# ============================================================
#  通知 wr-proxy 刷新 Provider 列表
# ============================================================

def _get_proxy_url():
    """获取 wr-proxy 地址：优先读数据库设置，fallback 到环境变量"""
    from models.wr_models import SystemSetting
    proxy_url = SystemSetting.get('proxy_url')
    if proxy_url:
        return proxy_url
    import os
    return os.environ.get('WR_PROXY_URL', 'http://localhost:5051')


def _notify_proxy_reload():
    """通知 wr-proxy 重新加载 Provider 列表"""
    proxy_url = _get_proxy_url()
    try:
        http.post(f"{proxy_url}/admin/reload", timeout=3)
    except Exception:
        pass  # wr-proxy 可能未启动，静默失败


def _notify_clear_cooldown(provider_id):
    """通知 wr-proxy 清除指定 Provider 的冷却状态"""
    proxy_url = _get_proxy_url()
    try:
        http.post(f"{proxy_url}/admin/clear_cooldown/{provider_id}", timeout=3)
    except Exception:
        pass  # wr-proxy 可能未启动，静默失败


# ============================================================
#  Provider 冷却管理（查询 wr-proxy 运行时冷却状态）
# ============================================================

@providers_bp.route('/cooldowns')
def list_cooldowns():
    """列出所有冷却中的 Provider（查询 wr-proxy 运行时状态）"""
    proxy_url = _get_proxy_url()
    try:
        resp = http.get(f"{proxy_url}/admin/cooldowns", timeout=3)
        return jsonify(resp.json())
    except Exception as e:
        return jsonify({'cooldowns': [], 'total': 0, 'error': str(e)})


@providers_bp.route('/<int:provider_id>/clear_cooldown', methods=['POST'])
def clear_cooldown(provider_id):
    """手动清除指定 Provider 的冷却状态"""
    _notify_clear_cooldown(provider_id)
    return jsonify({'message': get_message('cooldown_clear_sent', request), 'provider_id': provider_id})


@providers_bp.route('/request_cache')
def list_request_cache():
    """列出 wr-proxy 请求 Hash 缓存"""
    proxy_url = _get_proxy_url()
    try:
        resp = http.get(f"{proxy_url}/admin/request_cache", timeout=3)
        return jsonify(resp.json())
    except Exception as e:
        return jsonify({'entries': [], 'total': 0, 'error': str(e)})


@providers_bp.route('/request_cache', methods=['DELETE'])
def clear_request_cache():
    """清空 wr-proxy 请求 Hash 缓存"""
    proxy_url = _get_proxy_url()
    try:
        resp = http.delete(f"{proxy_url}/admin/request_cache", timeout=3)
        return jsonify(resp.json())
    except Exception as e:
        return jsonify({'error': str(e)})
