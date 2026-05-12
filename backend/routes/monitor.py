"""监控API"""
from flask import Blueprint, jsonify, request
from models.wr_models import ChannelHealth
from extensions import db

monitor_bp = Blueprint('monitor', __name__)

DEMO_CHANNELS = None  # 延迟导入


def _get_demo_channels():
    global DEMO_CHANNELS
    if DEMO_CHANNELS is None:
        from services.demo_data import get_demo_channels
        DEMO_CHANNELS = get_demo_channels()
    return DEMO_CHANNELS


def _has_newapi():
    try:
        from models.newapi_adapter import NewAPIAdapter
        return len(NewAPIAdapter.get_channels()) > 0
    except Exception:
        return False


@monitor_bp.route('/channels')
def channel_status():
    """所有渠道健康状态"""
    if not _has_newapi():
        from services.demo_data import get_demo_channels
        demo = get_demo_channels()
        result = [{
            'channel_id': ch['id'],
            'name': ch['name'],
            'type': ch.get('type', ''),
            'status': ch.get('status'),
            'health': ch.get('health', {'status': 'unchecked'}),
        } for ch in demo]
        return jsonify({'channels': result})

    try:
        from models.newapi_adapter import NewAPIAdapter
        channels = NewAPIAdapter.get_channels()
    except Exception:
        channels = []

    result = []
    for ch in channels:
        latest = ChannelHealth.query.filter_by(
            channel_id=ch['id']
        ).order_by(ChannelHealth.checked_at.desc()).first()
        result.append({
            'channel_id': ch['id'],
            'name': ch.get('name', ''),
            'type': ch.get('type', ''),
            'status': ch.get('status'),
            'health': latest.to_dict() if latest else {'status': 'unchecked'},
        })

    return jsonify({'channels': result})


@monitor_bp.route('/check/<int:channel_id>', methods=['POST'])
def check_channel(channel_id):
    """手动触发单个渠道检测"""
    if not _has_newapi():
        # Demo模式：模拟检测
        from services.demo_data import DEMO_HEALTH
        health = DEMO_HEALTH.get(channel_id, {'status': 'unknown', 'latency_ms': 0, 'error_message': None})
        return jsonify({
            'channel_id': channel_id,
            'status': health['status'],
            'latency_ms': health.get('latency_ms', 0),
            'error': health.get('error_message'),
            'name': f'Channel-{channel_id}',
        })

    from services.health_checker import HealthChecker
    from models.newapi_adapter import NewAPIAdapter
    checker = HealthChecker()
    try:
        channel = NewAPIAdapter.get_channel_by_id(channel_id)
        if not channel:
            return jsonify({'error': 'Channel not found'}), 404
        result = checker.check_channel_sync(channel)
        health = ChannelHealth(
            channel_id=channel_id,
            status=result['status'],
            latency_ms=result.get('latency_ms'),
            error_message=result.get('error'),
        )
        db.session.add(health)
        db.session.commit()
        return jsonify(result)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@monitor_bp.route('/check_all', methods=['POST'])
def check_all():
    """手动触发全量检测"""
    if not _has_newapi():
        from services.demo_data import get_demo_channels, DEMO_HEALTH
        results = []
        for ch in get_demo_channels():
            h = DEMO_HEALTH.get(ch['id'], {'status': 'unknown', 'latency_ms': 0})
            results.append({
                'channel_id': ch['id'],
                'status': h['status'],
                'latency_ms': h.get('latency_ms', 0),
                'error': h.get('error_message'),
            })
        return jsonify({'results': results})

    from services.health_checker import HealthChecker
    from models.newapi_adapter import NewAPIAdapter
    checker = HealthChecker()
    try:
        channels = NewAPIAdapter.get_channels()
        results = []
        for ch in channels:
            result = checker.check_channel_sync(ch)
            health = ChannelHealth(
                channel_id=ch['id'],
                status=result['status'],
                latency_ms=result.get('latency_ms'),
                error_message=result.get('error'),
            )
            db.session.add(health)
            results.append(result)
        db.session.commit()
        return jsonify({'results': results})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@monitor_bp.route('/history/<int:channel_id>')
def channel_history(channel_id):
    """渠道检测历史"""
    limit = request.args.get('limit', 50, type=int)
    records = ChannelHealth.query.filter_by(
        channel_id=channel_id
    ).order_by(ChannelHealth.checked_at.desc()).limit(limit).all()

    if not records and not _has_newapi():
        # Demo: 返回模拟历史
        from datetime import datetime, timedelta
        from services.demo_data import DEMO_HEALTH
        h = DEMO_HEALTH.get(channel_id, {'status': 'unknown', 'latency_ms': 0, 'error_message': None})
        history = []
        for i in range(5):
            history.append({
                'channel_id': channel_id,
                'status': h['status'] if i == 0 else 'healthy',
                'latency_ms': h.get('latency_ms', 200) + i * 50 if i > 0 else h.get('latency_ms'),
                'error_message': h.get('error_message') if i == 0 else None,
                'checked_at': (datetime.utcnow() - timedelta(minutes=5 * i)).isoformat(),
            })
        return jsonify({'history': history})

    return jsonify({'history': [r.to_dict() for r in records]})
