"""监控API — 基于 WebRouter 自有的 Provider 健康状态"""
from flask import Blueprint, jsonify, request
from models.wr_models import ChannelHealth
from models.provider import Provider
from extensions import db

monitor_bp = Blueprint('monitor', __name__)


@monitor_bp.route('/channels')
def channel_status():
    """所有 Provider 渠道健康状态"""
    providers = Provider.query.filter_by(enabled=True).all()
    result = []
    for p in providers:
        latest = ChannelHealth.query.filter_by(
            provider_id=p.id
        ).order_by(ChannelHealth.checked_at.desc()).first()
        result.append({
            'provider_id': p.id,
            'name': p.name,
            'type': p.type,
            'status': p.status or 'unknown',
            'priority': p.priority,
            'health': latest.to_dict() if latest else {'status': 'unchecked'},
        })
    return jsonify({'channels': result})


@monitor_bp.route('/check/<int:provider_id>', methods=['POST'])
def check_channel(provider_id):
    """手动触发单个 Provider 检测"""
    provider = Provider.query.get(provider_id)
    if not provider:
        return jsonify({'error': 'Provider not found'}), 404

    from services.health_checker import HealthChecker
    checker = HealthChecker()
    try:
        result = checker.check_provider(provider.to_dict(include_secrets=True))
        health = ChannelHealth(
            provider_id=provider_id,
            status=result.get('status', 'unknown'),
            latency_ms=result.get('latency_ms'),
            error_message=result.get('error'),
        )
        db.session.add(health)
        # 更新 Provider 状态
        provider.status = result.get('status', 'unknown')
        provider.last_latency_ms = result.get('latency_ms')
        provider.last_error = result.get('error')
        db.session.commit()
        result['provider_id'] = provider_id
        result['name'] = provider.name
        return jsonify(result)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@monitor_bp.route('/check_all', methods=['POST'])
def check_all():
    """手动触发全量检测"""
    from services.health_checker import HealthChecker
    checker = HealthChecker()
    try:
        results = checker.check_all_providers()
        return jsonify({'results': results})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@monitor_bp.route('/history/<int:provider_id>')
def channel_history(provider_id):
    """Provider 检测历史"""
    limit = request.args.get('limit', 50, type=int)
    records = ChannelHealth.query.filter_by(
        provider_id=provider_id
    ).order_by(ChannelHealth.checked_at.desc()).limit(limit).all()
    return jsonify({'history': [r.to_dict() for r in records]})
