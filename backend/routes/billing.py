"""计费 & 统计 API — 数据源改为 wr_request_logs"""
from flask import Blueprint, jsonify, request
from models.wr_models import RequestLog
from extensions import db
from sqlalchemy import func, case

billing_bp = Blueprint('billing', __name__)

# ── 有效请求判定条件 ──
VALID_COND = (RequestLog.status_code < 400) & (RequestLog.is_retry == False) & (func.coalesce(RequestLog.error_message, '') == '')


# ══════════════════════════════════════════
#  一、原有接口补全 valid_requests
# ══════════════════════════════════════════

@billing_bp.route('/summary')
def summary():
    """账单概览 — 本月/本周/今日汇总"""
    from datetime import datetime, timedelta

    now = datetime.utcnow()
    month_start = now.replace(day=1, hour=0, minute=0, second=0, microsecond=0)
    week_start = now - timedelta(days=now.weekday())
    week_start = week_start.replace(hour=0, minute=0, second=0, microsecond=0)
    today_start = now.replace(hour=0, minute=0, second=0, microsecond=0)

    periods = {
        'month': month_start,
        'week': week_start,
        'today': today_start,
    }

    result = {}
    for label, start in periods.items():
        row = db.session.query(
            func.count(RequestLog.id).label('requests'),
            func.sum(case((VALID_COND, 1), else_=0)).label('valid_requests'),
            func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
            func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
            func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        ).filter(RequestLog.created_at >= start).first()

        result[label] = {
            'requests': row.requests or 0,
            'valid_requests': row.valid_requests or 0,
            'input_tokens': row.input_tokens or 0,
            'output_tokens': row.output_tokens or 0,
            'cost_cents': row.cost_cents or 0,
            'cost_yuan': round((row.cost_cents or 0) / 100, 2),
        }

    return jsonify(result)


@billing_bp.route('/usage')
def usage():
    """用量统计 — 按模型聚合"""
    hours = request.args.get('hours', 24, type=int)

    stats = db.session.query(
        RequestLog.model_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_duration'),
        func.sum(
            case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('error_count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(RequestLog.model_name).order_by(func.sum(RequestLog.cost_cents).desc()).all()

    return jsonify({
        'hours': hours,
        'data': [{
            'model_name': r.model_name,
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_duration': round(r.avg_duration or 0, 1),
            'error_count': r.error_count,
        } for r in stats],
    })


@billing_bp.route('/cost')
def cost():
    """成本分析 — 按 Token / 模型聚合"""
    days = request.args.get('days', 30, type=int)

    # 按 Token 聚合
    token_stats = db.session.query(
        RequestLog.token_id,
        RequestLog.token_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(RequestLog.token_id, RequestLog.token_name).order_by(func.sum(RequestLog.cost_cents).desc()).all()

    # 按模型聚合
    model_stats = db.session.query(
        RequestLog.model_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(RequestLog.model_name).order_by(func.sum(RequestLog.cost_cents).desc()).all()

    return jsonify({
        'days': days,
        'by_token': [{
            'token_id': r.token_id,
            'token_name': r.token_name or f'Token#{r.token_id}',
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
        } for r in token_stats],
        'by_model': [{
            'model_name': r.model_name,
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
        } for r in model_stats],
    })


@billing_bp.route('/daily')
def daily():
    """每日明细"""
    days = request.args.get('days', 7, type=int)

    records = db.session.query(
        func.date(RequestLog.created_at).label('date'),
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.sum(
            case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('error_count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(func.date(RequestLog.created_at)).order_by('date').all()

    return jsonify({
        'days': days,
        'data': [{
            'date': str(r.date),
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'error_count': r.error_count,
        } for r in records],
    })


@billing_bp.route('/provider')
def by_provider():
    """按 Provider 聚合成本"""
    days = request.args.get('days', 30, type=int)

    records = db.session.query(
        RequestLog.provider_id,
        RequestLog.provider_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{days} days'),
    ).group_by(RequestLog.provider_id, RequestLog.provider_name).order_by(func.sum(RequestLog.cost_cents).desc()).all()

    return jsonify({
        'days': days,
        'data': [{
            'provider_id': r.provider_id,
            'provider_name': r.provider_name or f'Provider#{r.provider_id}',
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
        } for r in records],
    })


# ══════════════════════════════════════════
#  二、新增统计接口
# ══════════════════════════════════════════

@billing_bp.route('/errors')
def error_details():
    """异常请求明细 — 支持按 error_type / provider / model 筛选"""
    hours = request.args.get('hours', 24, type=int)
    error_type = request.args.get('error_type', '')    # quota_exhausted/rate_limited/timeout/unknown
    provider_id = request.args.get('provider_id', 0, type=int)
    model_name = request.args.get('model', '')
    limit = request.args.get('limit', 50, type=int)

    q = RequestLog.query.filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
        RequestLog.status_code >= 400,
    )

    if error_type:
        q = q.filter(RequestLog.error_type == error_type)
    if provider_id:
        q = q.filter(RequestLog.provider_id == provider_id)
    if model_name:
        q = q.filter(RequestLog.model_name == model_name)

    rows = q.order_by(RequestLog.created_at.desc()).limit(limit).all()

    return jsonify({
        'hours': hours,
        'filters': {
            'error_type': error_type or None,
            'provider_id': provider_id or None,
            'model': model_name or None,
        },
        'count': len(rows),
        'data': [r.to_dict() for r in rows],
    })


@billing_bp.route('/by-endpoint')
def by_endpoint():
    """按 Endpoint 聚合统计"""
    hours = request.args.get('hours', 24, type=int)

    stats = db.session.query(
        RequestLog.endpoint,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
        func.sum(
            case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('error_count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(RequestLog.endpoint).order_by(func.count(RequestLog.id).desc()).all()

    return jsonify({
        'hours': hours,
        'data': [{
            'endpoint': r.endpoint,
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
            'error_count': r.error_count,
        } for r in stats],
    })


@billing_bp.route('/by-client')
def by_client():
    """按 client_ip 聚合统计（多员工 Key 场景下按 IP 识别来源）"""
    hours = request.args.get('hours', 24, type=int)

    stats = db.session.query(
        RequestLog.client_ip,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
        func.sum(
            case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('error_count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(RequestLog.client_ip).order_by(func.count(RequestLog.id).desc()).all()

    return jsonify({
        'hours': hours,
        'data': [{
            'client_ip': r.client_ip,
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
            'error_count': r.error_count,
        } for r in stats],
    })


@billing_bp.route('/by-token')
def by_token():
    """按 Token（用户API Key）聚合统计 — 员工维度"""
    hours = request.args.get('hours', 24, type=int)

    stats = db.session.query(
        RequestLog.token_id,
        RequestLog.token_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
        func.sum(
            case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('error_count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(RequestLog.token_id, RequestLog.token_name).order_by(func.count(RequestLog.id).desc()).all()

    return jsonify({
        'hours': hours,
        'data': [{
            'token_id': r.token_id,
            'token_name': r.token_name or f'Token#{r.token_id}',
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
            'error_count': r.error_count,
        } for r in stats],
    })


@billing_bp.route('/by-provider-model')
def by_provider_model():
    """按 Provider+Model 交叉统计 — 调度策略分析"""
    hours = request.args.get('hours', 24, type=int)

    stats = db.session.query(
        RequestLog.provider_id,
        RequestLog.provider_name,
        RequestLog.model_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
        func.sum(
            case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('error_count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
    ).group_by(
        RequestLog.provider_id, RequestLog.provider_name, RequestLog.model_name,
    ).order_by(func.count(RequestLog.id).desc()).all()

    return jsonify({
        'hours': hours,
        'data': [{
            'provider_id': r.provider_id,
            'provider_name': r.provider_name or f'Provider#{r.provider_id}',
            'model_name': r.model_name,
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round(r.cost_cents / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
            'error_count': r.error_count,
        } for r in stats],
    })


@billing_bp.route('/error-types')
def error_types():
    """按异常类型分类统计 — quota_exhausted / rate_limited / timeout / unknown"""
    hours = request.args.get('hours', 24, type=int)

    stats = db.session.query(
        func.coalesce(RequestLog.error_type, '').label('error_type'),
        func.count(RequestLog.id).label('count'),
        func.coalesce(func.avg(RequestLog.latency_ms), 0).label('avg_latency'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
        RequestLog.status_code >= 400,
    ).group_by(
        func.coalesce(RequestLog.error_type, ''),
    ).order_by(func.count(RequestLog.id).desc()).all()

    # 补充：还按 provider 维度看错误分布
    provider_errors = db.session.query(
        RequestLog.provider_id,
        RequestLog.provider_name,
        RequestLog.error_type,
        func.count(RequestLog.id).label('count'),
    ).filter(
        RequestLog.created_at >= func.datetime('now', f'-{hours} hours'),
        RequestLog.status_code >= 400,
    ).group_by(
        RequestLog.provider_id, RequestLog.provider_name, RequestLog.error_type,
    ).order_by(func.count(RequestLog.id).desc()).limit(20).all()

    return jsonify({
        'hours': hours,
        'by_type': [{
            'error_type': r.error_type or 'unclassified',
            'count': r.count,
            'avg_latency_ms': round(r.avg_latency or 0, 1),
        } for r in stats],
        'by_provider': [{
            'provider_id': r.provider_id,
            'provider_name': r.provider_name or f'Provider#{r.provider_id}',
            'error_type': r.error_type or 'unclassified',
            'count': r.count,
        } for r in provider_errors],
    })
