"""计费API"""
from flask import Blueprint, jsonify, request
from models.wr_models import CostRecord
from extensions import db
from sqlalchemy import func

billing_bp = Blueprint('billing', __name__)


def _has_newapi():
    try:
        from models.newapi_adapter import NewAPIAdapter
        return len(NewAPIAdapter.get_channels()) > 0
    except Exception:
        return False


@billing_bp.route('/usage')
def usage():
    """用量统计"""
    hours = request.args.get('hours', 24, type=int)

    if not _has_newapi():
        from services.demo_data import get_demo_trends
        return jsonify({'hours': hours, 'data': get_demo_trends(1)})

    try:
        from models.newapi_adapter import NewAPIAdapter
        data = NewAPIAdapter.get_usage_stats(hours=hours)
    except Exception:
        data = []
    return jsonify({'hours': hours, 'data': data})


@billing_bp.route('/cost')
def cost():
    """成本分析"""
    days = request.args.get('days', 30, type=int)

    # 先查本地数据库
    try:
        records = db.session.query(
            CostRecord.model_name,
            func.sum(CostRecord.input_tokens).label('input_tokens'),
            func.sum(CostRecord.output_tokens).label('output_tokens'),
            func.sum(CostRecord.cost_cents).label('cost_cents'),
        ).filter(
            CostRecord.recorded_at >= func.date('now', f'-{days} days')
        ).group_by(CostRecord.model_name).all()

        if records:
            data = [{
                'model_name': r.model_name,
                'input_tokens': r.input_tokens or 0,
                'output_tokens': r.output_tokens or 0,
                'cost_cents': r.cost_cents or 0,
                'cost_yuan': round((r.cost_cents or 0) / 100, 2),
            } for r in records]
            return jsonify({'days': days, 'data': data})
    except Exception:
        pass

    # 无数据则返回demo
    from services.demo_data import get_demo_cost
    return jsonify({'days': days, 'data': get_demo_cost(days)})


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

        if records:
            data = [{
                'date': str(r.date),
                'input_tokens': r.input_tokens or 0,
                'output_tokens': r.output_tokens or 0,
                'cost_cents': r.cost_cents or 0,
                'cost_yuan': round((r.cost_cents or 0) / 100, 2),
            } for r in records]
            return jsonify({'days': days, 'data': data})
    except Exception:
        pass

    from services.demo_data import get_demo_trends
    return jsonify({'days': days, 'data': get_demo_trends(days)})
