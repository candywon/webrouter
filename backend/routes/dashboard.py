"""仪表盘 API — 数据源改为 wr_request_logs + wr_providers"""
from flask import Blueprint, jsonify, request as req
from models.provider import Provider
from models.wr_models import ChannelHealth, RequestLog, ProviderExt, ProviderQuota, WRToken
from extensions import db
from sqlalchemy import func

dashboard_bp = Blueprint('dashboard', __name__)


@dashboard_bp.route('/overview')
def overview():
    """总览数据：Provider 数、今日调用量、错误率、月成本、Token 数"""
    # Provider 统计
    providers = Provider.query.filter_by(enabled=True).all()
    total_providers = len(providers)
    healthy_count = sum(1 for p in providers if p.status == 'healthy')

    # Token 统计
    total_tokens = WRToken.query.filter_by(enabled=True).count()

    # 今日用量
    today_stats = db.session.query(
        func.count(RequestLog.id).label('requests'),
        func.sum(
            db.case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('errors'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost'),
    ).filter(
        RequestLog.created_at >= func.date('now'),
    ).first()

    today_requests = today_stats.requests or 0
    today_errors = today_stats.errors or 0
    error_rate = round(today_errors / max(today_requests, 1) * 100, 2)

    # 月成本
    month_cost = db.session.query(
        func.coalesce(func.sum(RequestLog.cost_cents), 0),
    ).filter(
        RequestLog.created_at >= func.datetime('now', '-30 days'),
    ).scalar() or 0

    # 今日 token 用量
    today_tokens = db.session.query(
        func.coalesce(func.sum(RequestLog.input_tokens), 0),
        func.coalesce(func.sum(RequestLog.output_tokens), 0),
    ).filter(
        RequestLog.created_at >= func.date('now'),
    ).first()

    return jsonify({
        'providers': {
            'total': total_providers,
            'healthy': healthy_count,
        },
        'tokens': {
            'total': total_tokens,
        },
        'usage': {
            'today_requests': today_requests,
            'today_errors': today_errors,
            'error_rate': error_rate,
            'today_input_tokens': today_tokens[0] if today_tokens else 0,
            'today_output_tokens': today_tokens[1] if today_tokens else 0,
        },
        'cost': {
            'month_cents': month_cost,
            'month_yuan': round(month_cost / 100, 2),
        },
    })


@dashboard_bp.route('/trends')
def trends():
    """N 天调用趋势"""
    days = req.args.get('days', 7, type=int)
    days = min(days, 90)

    records = db.session.query(
        func.date(RequestLog.created_at).label('date'),
        func.count(RequestLog.id).label('requests'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.sum(
            db.case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('errors'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(func.date(RequestLog.created_at)).order_by('date').all()

    return jsonify({
        'days': days,
        'data': [{
            'date': str(r.date),
            'requests': r.requests,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'errors': r.errors,
        } for r in records],
    })


@dashboard_bp.route('/providers')
def providers():
    """Provider 列表 + 健康状态 + 额度"""
    from routes.providers import _provider_full_dict

    provider_list = Provider.query.order_by(Provider.priority.desc(), Provider.id.asc()).all()
    return jsonify({
        'providers': [_provider_full_dict(p) for p in provider_list],
    })


@dashboard_bp.route('/top-tokens')
def top_tokens():
    """用量排行 Top N Token"""
    limit = req.args.get('limit', 10, type=int)
    days = req.args.get('days', 7, type=int)

    stats = db.session.query(
        RequestLog.token_id,
        RequestLog.token_name,
        func.count(RequestLog.id).label('requests'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(RequestLog.token_id, RequestLog.token_name).order_by(
        func.sum(RequestLog.cost_cents).desc()
    ).limit(limit).all()

    return jsonify({
        'days': days,
        'data': [{
            'token_id': r.token_id,
            'token_name': r.token_name or f'Token#{r.token_id}',
            'requests': r.requests,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
        } for r in stats],
    })


@dashboard_bp.route('/top-models')
def top_models():
    """模型用量排行"""
    hours = req.args.get('hours', 24, type=int)
    limit = req.args.get('limit', 10, type=int)

    stats = db.session.query(
        RequestLog.model_name,
        func.count(RequestLog.id).label('requests'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(RequestLog.model_name).order_by(
        func.sum(RequestLog.cost_cents).desc()
    ).limit(limit).all()

    return jsonify({
        'hours': hours,
        'data': [{
            'model': r.model_name,
            'requests': r.requests,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
        } for r in stats],
    })
