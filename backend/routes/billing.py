"""计费API"""
from flask import Blueprint, jsonify, request
from models.wr_models import CostRecord
from models.newapi_adapter import NewAPIAdapter
from extensions import db
from sqlalchemy import func

billing_bp = Blueprint('billing', __name__)


@billing_bp.route('/usage')
def usage():
    """用量统计"""
    hours = request.args.get('hours', 24, type=int)
    try:
        data = NewAPIAdapter.get_usage_stats(hours=hours)
    except Exception:
        data = []
    return jsonify({'hours': hours, 'data': data})


@billing_bp.route('/cost')
def cost():
    """成本分析"""
    days = request.args.get('days', 30, type=int)
    try:
        records = db.session.query(
            CostRecord.model_name,
            func.sum(CostRecord.input_tokens).label('input_tokens'),
            func.sum(CostRecord.output_tokens).label('output_tokens'),
            func.sum(CostRecord.cost_cents).label('cost_cents'),
        ).filter(
            CostRecord.recorded_at >= func.date('now', f'-{days} days')
        ).group_by(CostRecord.model_name).all()

        data = [{
            'model_name': r.model_name,
            'input_tokens': r.input_tokens or 0,
            'output_tokens': r.output_tokens or 0,
            'cost_cents': r.cost_cents or 0,
            'cost_yuan': round((r.cost_cents or 0) / 100, 2),
        } for r in records]
    except Exception:
        data = []

    return jsonify({'days': days, 'data': data})


@billing_bp.route('/daily')
def daily():
    """每日明细"""
    days = request.args.get('days', 7, type=int)
    try:
        records = db.session.query(
            func.date(CostRecord.recorded_at).label('date'),
            func.sum(CostRecord.input_tokens).label('input_tokens'),
            func.sum(CostRecord.output_tokens).label('output_tokens'),
            func.sum(CostRecord.cost_cents).label('cost_cents'),
        ).filter(
            CostRecord.recorded_at >= func.date('now', f'-{days} days')
        ).group_by(func.date(CostRecord.recorded_at)).order_by('date').all()

        data = [{
            'date': str(r.date),
            'input_tokens': r.input_tokens or 0,
            'output_tokens': r.output_tokens or 0,
            'cost_cents': r.cost_cents or 0,
            'cost_yuan': round((r.cost_cents or 0) / 100, 2),
        } for r in records]
    except Exception:
        data = []

    return jsonify({'days': days, 'data': data})
