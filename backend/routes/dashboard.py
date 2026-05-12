"""仪表盘API"""
from flask import Blueprint, jsonify
from models.wr_models import ChannelHealth, CostRecord
from extensions import db
from sqlalchemy import func

dashboard_bp = Blueprint('dashboard', __name__)

# 检测是否有New-API数据
def _has_newapi_data():
    """尝试读New-API，失败则返回False"""
    try:
        from models.newapi_adapter import NewAPIAdapter
        channels = NewAPIAdapter.get_channels()
        return len(channels) > 0
    except Exception:
        return False


@dashboard_bp.route('/overview')
def overview():
    """总览数据：可用Key数、今日调用量、错误率、月成本"""
    if not _has_newapi_data():
        from services.demo_data import get_demo_overview
        return jsonify(get_demo_overview())

    try:
        from models.newapi_adapter import NewAPIAdapter
        channels = NewAPIAdapter.get_channels()
    except Exception:
        channels = []

    total_channels = len(channels)
    active_channels = sum(1 for c in channels if c.get('status') == 1)

    health_map = {}
    try:
        health_records = db.session.query(
            ChannelHealth.channel_id,
            ChannelHealth.status,
        ).distinct(ChannelHealth.channel_id).order_by(
            ChannelHealth.channel_id, ChannelHealth.checked_at.desc()
        ).all()
        for r in health_records:
            health_map[r.channel_id] = r.status
    except Exception:
        pass

    healthy_count = sum(1 for s in health_map.values() if s == 'healthy')

    try:
        from models.newapi_adapter import NewAPIAdapter
        usage = NewAPIAdapter.get_usage_stats(hours=24)
        today_requests = sum(u.get('request_count', 0) for u in usage)
        today_errors = sum(u.get('error_count', 0) for u in usage)
        error_rate = round(today_errors / max(today_requests, 1) * 100, 2)
    except Exception:
        today_requests = 0
        error_rate = 0

    try:
        month_cost = db.session.query(
            func.sum(CostRecord.cost_cents)
        ).filter(
            CostRecord.recorded_at >= func.date('now', '-30 days')
        ).scalar() or 0
    except Exception:
        month_cost = 0

    return jsonify({
        'channels': {
            'total': total_channels,
            'active': active_channels,
            'healthy': healthy_count,
        },
        'usage': {
            'today_requests': today_requests,
            'error_rate': error_rate,
        },
        'cost': {
            'month_cents': month_cost,
            'month_yuan': round(month_cost / 100, 2),
        },
    })


@dashboard_bp.route('/trends')
def trends():
    """7天/30天调用趋势"""
    from flask import request
    days = request.args.get('days', 7, type=int)
    days = min(days, 90)

    if not _has_newapi_data():
        from services.demo_data import get_demo_trends
        return jsonify({'days': days, 'data': get_demo_trends(days)})

    try:
        from models.newapi_adapter import NewAPIAdapter
        data = NewAPIAdapter.get_daily_usage(days=days)
    except Exception:
        data = []
    return jsonify({'days': days, 'data': data})


@dashboard_bp.route('/channels')
def channels():
    """渠道列表+健康状态"""
    if not _has_newapi_data():
        from services.demo_data import get_demo_channels
        return jsonify({'channels': get_demo_channels()})

    try:
        from models.newapi_adapter import NewAPIAdapter
        channel_list = NewAPIAdapter.get_channels()
    except Exception:
        channel_list = []

    for ch in channel_list:
        latest = ChannelHealth.query.filter_by(
            channel_id=ch['id']
        ).order_by(ChannelHealth.checked_at.desc()).first()
        ch['health'] = latest.to_dict() if latest else {'status': 'unknown'}

    return jsonify({'channels': channel_list})
