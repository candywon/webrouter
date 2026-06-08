# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""计费 & 统计 API — 数据源改为 wr_request_logs"""
from flask import Blueprint, jsonify, request
from models.wr_models import RequestLog
from extensions import db
from sqlalchemy import func, case, text

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


@billing_bp.route('/top-tokens')
def top_tokens():
    """Top Token usage for dashboard."""
    hours = request.args.get('hours', 24, type=int)
    top = request.args.get('top', 10, type=int)
    top = max(1, min(top, 100))

    since_filter = RequestLog.created_at >= func.datetime('now', f'-{hours} hours')
    total_requests = db.session.query(func.count(RequestLog.id)).filter(since_filter).scalar() or 0

    stats = db.session.query(
        RequestLog.token_id,
        RequestLog.token_name,
        func.count(RequestLog.id).label('request_count'),
        func.sum(case((VALID_COND, 1), else_=0)).label('valid_count'),
        func.coalesce(func.sum(RequestLog.input_tokens), 0).label('input_tokens'),
        func.coalesce(func.sum(RequestLog.output_tokens), 0).label('output_tokens'),
        func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
    ).filter(
        since_filter,
    ).group_by(
        RequestLog.token_id, RequestLog.token_name,
    ).order_by(
        func.count(RequestLog.id).desc(),
    ).limit(top).all()

    token_ids = [r.token_id for r in stats]
    model_map = {token_id: [] for token_id in token_ids}
    if token_ids:
        model_rows = db.session.query(
            RequestLog.token_id,
            RequestLog.model_name,
            func.count(RequestLog.id).label('count'),
            func.coalesce(func.sum(RequestLog.cost_cents), 0).label('cost_cents'),
        ).filter(
            since_filter,
            RequestLog.token_id.in_(token_ids),
        ).group_by(
            RequestLog.token_id, RequestLog.model_name,
        ).order_by(
            RequestLog.token_id, func.count(RequestLog.id).desc(),
        ).all()

        for row in model_rows:
            model_map.setdefault(row.token_id, []).append({
                'model_name': row.model_name or 'unknown',
                'count': row.count,
                'cost_cents': row.cost_cents or 0,
            })

    return jsonify({
        'hours': hours,
        'top': top,
        'total_requests': total_requests,
        'tokens': [{
            'token_id': r.token_id,
            'token_name': r.token_name or f'Token#{r.token_id}',
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens or 0,
            'output_tokens': r.output_tokens or 0,
            'cost_cents': r.cost_cents or 0,
            'cost_yuan': round((r.cost_cents or 0) / 100, 2),
            'pct_of_total': round((r.request_count / total_requests * 100), 1) if total_requests else 0,
            'model_distribution': model_map.get(r.token_id, []),
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


# ══════════════════════════════════════════
#  三、按组织架构统计
# ══════════════════════════════════════════

def _get_descendant_org_ids(root_id):
    """递归查询指定组织及其所有子组织 ID（SQLite WITH RECURSIVE）"""
    if not root_id:
        return []
    sql = text("""
        WITH RECURSIVE org_tree AS (
            SELECT id FROM wr_orgs WHERE id = :root_id
            UNION ALL
            SELECT o.id FROM wr_orgs o
            JOIN org_tree ot ON o.parent_id = ot.id
        )
        SELECT id FROM org_tree
    """)
    result = db.session.execute(sql, {'root_id': root_id})
    return [row[0] for row in result]



@billing_bp.route('/by-org')
def by_org():
    """按组织架构聚合用量统计 — 支持组织节点、组织类型和递归子组织汇总"""
    from datetime import datetime, timedelta

    hours = request.args.get('hours', 24, type=int)
    org_filter = request.args.get('org_id', 0, type=int)
    org_type = request.args.get('org_type', '', type=str)
    group_by = request.args.get('group_by', 'org', type=str)  # org / type
    since = (datetime.utcnow() - timedelta(hours=hours)).strftime('%Y-%m-%d %H:%M:%S')

    where_parts = ['r.created_at >= :since']
    params = {'since': since}

    if org_filter:
        org_ids = _get_descendant_org_ids(org_filter)
        if not org_ids:
            return jsonify({'hours': hours, 'org_id': org_filter, 'org_type': org_type or None, 'group_by': group_by, 'data': []})
        where_parts.append('t.org_id IN (' + ','.join(str(oid) for oid in org_ids) + ')')

    if org_type:
        where_parts.append('o.org_type = :org_type')
        params['org_type'] = org_type

    where_sql = ' AND '.join(where_parts)

    if group_by == 'type':
        select_sql = """
            COALESCE(o.org_type, 'unassigned') AS org_type,
            CASE COALESCE(o.org_type, 'unassigned')
                WHEN 'company' THEN 'Company'
                WHEN 'department' THEN 'Department'
                WHEN 'project' THEN 'Project Group'
                WHEN 'group' THEN 'Group'
                ELSE 'Unassigned'
            END AS org_name,
            COUNT(DISTINCT t.org_id) AS org_count
        """
        group_sql = "GROUP BY COALESCE(o.org_type, 'unassigned')"
    else:
        select_sql = """
            COALESCE(t.org_id, 0) AS org_id,
            COALESCE(o.name, 'Unassigned') AS org_name,
            COALESCE(o.org_type, 'unassigned') AS org_type,
            1 AS org_count
        """
        group_sql = 'GROUP BY t.org_id'

    sql_str = f"""
        SELECT
            {select_sql},
            COUNT(r.id) AS request_count,
            SUM(CASE WHEN r.status_code < 400 AND (r.is_retry IS NULL OR r.is_retry = 0)
                     AND COALESCE(r.error_message, '') = '' THEN 1 ELSE 0 END) AS valid_count,
            COALESCE(SUM(r.input_tokens), 0) AS input_tokens,
            COALESCE(SUM(r.output_tokens), 0) AS output_tokens,
            COALESCE(SUM(r.cost_cents), 0) AS cost_cents,
            AVG(r.latency_ms) AS avg_latency,
            SUM(CASE WHEN r.status_code >= 400 THEN 1 ELSE 0 END) AS error_count
        FROM wr_request_logs r
        LEFT JOIN wr_tokens t ON r.token_id = t.id
        LEFT JOIN wr_orgs o ON t.org_id = o.id
        WHERE {where_sql}
        {group_sql}
        ORDER BY cost_cents DESC
    """

    rows = db.session.execute(text(sql_str), params).fetchall()

    result = []
    for r in rows:
        item = {
            'org_name': r.org_name,
            'org_type': r.org_type,
            'org_count': r.org_count,
            'request_count': r.request_count,
            'valid_count': r.valid_count or 0,
            'input_tokens': r.input_tokens,
            'output_tokens': r.output_tokens,
            'cost_cents': r.cost_cents,
            'cost_yuan': round((r.cost_cents or 0) / 100, 2),
            'avg_latency_ms': round(r.avg_latency or 0, 1),
            'error_count': r.error_count or 0,
        }
        if group_by != 'type':
            item['org_id'] = r.org_id
        result.append(item)

    return jsonify({
        'hours': hours,
        'org_id': org_filter or None,
        'org_type': org_type or None,
        'group_by': group_by,
        'data': result,
    })
