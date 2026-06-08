# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""扩展监控 API — 延迟分布、错误趋势、Provider 对比"""
from flask import Blueprint, jsonify, request as req
from models.wr_models import RequestLog
from extensions import db
from sqlalchemy import func, text
from datetime import datetime, timedelta

monitoring_ext_bp = Blueprint('monitoring_ext', __name__)


def _apply_org_filter(query, org_filter):
    """对查询应用 org_id 过滤（递归子组织）"""
    if not org_filter:
        return query
    # Import here to avoid circular
    from routes.billing import _get_descendant_org_ids
    org_ids = _get_descendant_org_ids(org_filter)
    if not org_ids:
        return query.filter(text('1=0'))  # no orgs, return empty
    ids_str = ','.join(str(oid) for oid in org_ids)
    token_ids = [row[0] for row in db.session.execute(
        text(f'SELECT id FROM wr_tokens WHERE org_id IN ({ids_str})')
    ).fetchall()]
    if token_ids:
        return query.filter(RequestLog.token_id.in_(token_ids))
    else:
        return query.filter(text('1=0'))


def _percentile(values, p):
    """计算百分位值（0-100），values 已排序"""
    if not values:
        return 0
    k = (len(values) - 1) * p / 100
    f = int(k)
    c = f + 1
    if c >= len(values):
        return values[-1]
    return round(values[f] + (k - f) * (values[c] - values[f]), 2)


@monitoring_ext_bp.route('/latency-distribution')
def latency_distribution():
    """延迟分布：按 model/provider 分组返回 P50/P90/P99"""
    hours = req.args.get('hours', 24, type=int)
    group_by = req.args.get('group_by', 'model')  # model / provider
    org_filter = req.args.get('org_id', 0, type=int)
    since = datetime.utcnow() - timedelta(hours=hours)

    if group_by == 'provider':
        group_col = RequestLog.provider_name
        label_col = RequestLog.provider_name
    else:
        group_col = RequestLog.model_name
        label_col = RequestLog.model_name

    # 获取所有延迟数据
    q = db.session.query(
        label_col,
        RequestLog.latency_ms,
    ).filter(
        RequestLog.created_at >= since,
        RequestLog.latency_ms > 0,
    )
    if org_filter:
        q = _apply_org_filter(q, org_filter)
    records = q.order_by(label_col, RequestLog.latency_ms).all()

    # 按分组聚合
    groups = {}
    for label, lat in records:
        groups.setdefault(label, []).append(lat)

    result = []
    for label, values in sorted(groups.items()):
        result.append({
            'name': label,
            'request_count': len(values),
            'p50_ms': _percentile(values, 50),
            'p90_ms': _percentile(values, 90),
            'p99_ms': _percentile(values, 99),
            'avg_ms': round(sum(values) / len(values), 1),
        })

    return jsonify({
        'hours': hours,
        'group_by': group_by,
        'data': result,
    })


@monitoring_ext_bp.route('/error-rate-trend')
def error_rate_trend():
    """错误率时间序列（按小时桶）"""
    hours = req.args.get('hours', 24, type=int)
    bucket = req.args.get('bucket', 'hour')  # hour / day
    org_filter = req.args.get('org_id', 0, type=int)
    since = datetime.utcnow() - timedelta(hours=hours)

    if bucket == 'day':
        time_key = func.date(RequestLog.created_at)
    else:
        time_key = func.strftime('%Y-%m-%d %H:00', RequestLog.created_at)

    q = db.session.query(
        time_key.label('bucket'),
        func.count(RequestLog.id).label('total'),
        func.sum(
            db.case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('errors'),
    ).filter(
        RequestLog.created_at >= since,
    )
    if org_filter:
        q = _apply_org_filter(q, org_filter)
    records = q.group_by(time_key).order_by(time_key).all()

    return jsonify({
        'hours': hours,
        'bucket': bucket,
        'data': [{
            'bucket': str(r.bucket),
            'total': r.total,
            'errors': r.errors or 0,
            'error_rate': round((r.errors or 0) / max(r.total, 1) * 100, 2),
        } for r in records],
    })


@monitoring_ext_bp.route('/provider-comparison')
def provider_comparison():
    """Provider 对比：延迟、错误率、请求量、成本"""
    hours = req.args.get('hours', 24, type=int)
    model = req.args.get('model', '', type=str)
    org_filter = req.args.get('org_id', 0, type=int)
    since = datetime.utcnow() - timedelta(hours=hours)

    query = db.session.query(
        RequestLog.provider_name,
        RequestLog.model_name,
        func.count(RequestLog.id).label('requests'),
        func.sum(
            db.case((RequestLog.status_code >= 400, 1), else_=0)
        ).label('errors'),
        func.avg(RequestLog.latency_ms).label('avg_latency'),
        func.sum(RequestLog.cost_cents).label('total_cost'),
    ).filter(
        RequestLog.created_at >= since,
    )

    if org_filter:
        query = _apply_org_filter(query, org_filter)

    if model:
        query = query.filter(RequestLog.model_name == model)

    records = query.group_by(
        RequestLog.provider_name, RequestLog.model_name
    ).order_by(
        RequestLog.provider_name, RequestLog.model_name
    ).all()

    # 按 provider 聚合
    providers = {}
    for r in records:
        key = r.provider_name
        if key not in providers:
            providers[key] = {
                'requests': 0, 'errors': 0,
                'latencies': [], 'cost': 0, 'models': set(),
            }
        p = providers[key]
        p['requests'] += r.requests
        p['errors'] += r.errors or 0
        p['cost'] += r.total_cost or 0
        p['models'].add(r.model_name)
        if r.avg_latency:
            p['latencies'].append(r.avg_latency)

    result = []
    for name, p in sorted(providers.items()):
        avg_lat = round(sum(p['latencies']) / len(p['latencies']), 1) if p['latencies'] else 0
        result.append({
            'name': name,
            'request_count': p['requests'],
            'error_count': p['errors'],
            'error_rate': round(p['errors'] / max(p['requests'], 1) * 100, 2),
            'avg_latency_ms': avg_lat,
            'total_cost_cents': p['cost'],
            'model_count': len(p['models']),
        })

    return jsonify({
        'hours': hours,
        'model': model or 'all',
        'data': result,
    })