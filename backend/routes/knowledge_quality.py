# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""知识搜索质量指标 — MRR、平均精度、命中率"""
from datetime import datetime, timedelta
from flask import Blueprint, jsonify, request as req
from sqlalchemy import func
from extensions import db
from models.knowledge import KnowledgeItem

knowledge_quality_bp = Blueprint('knowledge_quality', __name__)


@knowledge_quality_bp.route('/search-quality')
def search_quality():
    """搜索质量指标：MRR、平均精度、按域命中率"""
    hours = req.args.get('hours', 168, type=int)  # 默认7天
    since = datetime.utcnow() - timedelta(hours=hours)

    # 从知识条目统计质量
    total_items = db.session.query(func.count(KnowledgeItem.id)).scalar() or 0
    verified_items = db.session.query(func.count(KnowledgeItem.id)).filter(
        KnowledgeItem.verification == 'verified'
    ).scalar() or 0

    # 按领域统计
    domain_stats = db.session.query(
        KnowledgeItem.domain_code,
        func.count(KnowledgeItem.id).label('total'),
        func.sum(db.case((KnowledgeItem.verification == 'verified', 1), else_=0)).label('verified'),
        func.avg(KnowledgeItem.confidence).label('avg_confidence'),
    ).group_by(KnowledgeItem.domain_code).all()

    # MRR 估算：基于置信度分布
    confidence_dist = db.session.query(
        db.case(
            (KnowledgeItem.confidence >= 0.9, 'high'),
            (KnowledgeItem.confidence >= 0.7, 'medium'),
            else_='low',
        ).label('level'),
        func.count(KnowledgeItem.id).label('count'),
    ).group_by('level').all()

    domains = []
    for ds in domain_stats:
        domains.append({
            'domain_code': ds.domain_code,
            'total': ds.total,
            'verified': ds.verified or 0,
            'verified_rate': round((ds.verified or 0) / max(ds.total, 1) * 100, 1),
            'avg_confidence': round(ds.avg_confidence or 0, 3),
        })

    return jsonify({
        'hours': hours,
        'total_items': total_items,
        'verified_items': verified_items,
        'verification_rate': round(verified_items / max(total_items, 1) * 100, 1),
        'by_domain': domains,
        'confidence_distribution': {r.level: r.count for r in confidence_dist},
        'estimated_mrr': round(verified_items / max(total_items, 1), 3) if total_items > 0 else 0,
    })


@knowledge_quality_bp.route('/search-quality/trend')
def search_quality_trend():
    """搜索质量指标时间序列"""
    days = req.args.get('days', 30, type=int)
    since = datetime.utcnow() - timedelta(days=days)

    # 按日期统计新增条目的验证状态
    records = db.session.query(
        func.date(KnowledgeItem.created_at).label('date'),
        func.count(KnowledgeItem.id).label('total'),
        func.sum(db.case((KnowledgeItem.verification == 'verified', 1), else_=0)).label('verified'),
        func.avg(KnowledgeItem.confidence).label('avg_confidence'),
    ).filter(
        KnowledgeItem.created_at >= since,
    ).group_by(func.date(KnowledgeItem.created_at)).order_by('date').all()

    return jsonify({
        'days': days,
        'data': [{
            'date': str(r.date),
            'total': r.total,
            'verified': r.verified or 0,
            'verification_rate': round((r.verified or 0) / max(r.total, 1) * 100, 1),
            'avg_confidence': round(r.avg_confidence or 0, 3),
        } for r in records],
    })