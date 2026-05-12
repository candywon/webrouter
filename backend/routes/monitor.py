"""监控API"""
from flask import Blueprint, jsonify, request
from models.wr_models import ChannelHealth
from models.newapi_adapter import NewAPIAdapter
from extensions import db

monitor_bp = Blueprint('monitor', __name__)


@monitor_bp.route('/channels')
def channel_status():
    """所有渠道健康状态"""
    try:
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
    from services.health_checker import HealthChecker
    checker = HealthChecker()
    try:
        channel = NewAPIAdapter.get_channel_by_id(channel_id)
        if not channel:
            return jsonify({'error': 'Channel not found'}), 404
        result = checker.check_channel_sync(channel)
        # 保存结果
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
    from services.health_checker import HealthChecker
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
    return jsonify({'history': [r.to_dict() for r in records]})
