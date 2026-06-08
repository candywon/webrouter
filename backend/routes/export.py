# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""仪表盘数据导出 — CSV 导出"""
import csv
import io
from datetime import datetime, timedelta
from flask import Blueprint, jsonify, request as req, Response
from sqlalchemy import func
from extensions import db
from models.wr_models import RequestLog

export_bp = Blueprint('export', __name__)


@export_bp.route('/dashboard-csv')
def export_dashboard_csv():
    """导出当前仪表盘数据为 CSV"""
    hours = req.args.get('hours', 24, type=int)
    since = datetime.utcnow() - timedelta(hours=hours)

    records = db.session.query(
        RequestLog.created_at,
        RequestLog.token_name,
        RequestLog.provider_name,
        RequestLog.model_name,
        RequestLog.endpoint,
        RequestLog.input_tokens,
        RequestLog.output_tokens,
        RequestLog.latency_ms,
        RequestLog.cost_cents,
        RequestLog.status_code,
        RequestLog.error_type,
    ).filter(
        RequestLog.created_at >= since,
    ).order_by(RequestLog.created_at.desc()).limit(10000).all()

    output = io.StringIO()
    writer = csv.writer(output)
    writer.writerow([
        'Time', 'Token', 'Provider', 'Model', 'Endpoint',
        'Input Tokens', 'Output Tokens', 'Latency (ms)',
        'Cost (cents)', 'Status Code', 'Error Type',
    ])

    for r in records:
        writer.writerow([
            r.created_at.strftime('%Y-%m-%d %H:%M:%S') if r.created_at else '',
            r.token_name, r.provider_name, r.model_name,
            r.endpoint, r.input_tokens, r.output_tokens,
            r.latency_ms, r.cost_cents, r.status_code, r.error_type or '',
        ])

    csv_content = output.getvalue()
    output.close()

    return Response(
        csv_content,
        mimetype='text/csv',
        headers={
            'Content-Disposition': f'attachment; filename=dashboard_{hours}h_{datetime.utcnow().strftime("%Y%m%d_%H%M%S")}.csv',
            'Content-Type': 'text/csv; charset=utf-8-sig',
        },
    )