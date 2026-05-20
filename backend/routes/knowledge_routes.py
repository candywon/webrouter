"""企业知识库 API — 知识列表/搜索/统计/域名管理"""
import json
from flask import Blueprint, jsonify, request
from sqlalchemy import func
from extensions import db
from models.knowledge import (
    KnowledgeRaw, KnowledgeItem, KnowledgeDomain,
    KnowledgeDomainRisk, KnowledgeAnalysis,
)

knowledge_bp = Blueprint('knowledge', __name__)


# ============================================================
# 捕获统计
# ============================================================

@knowledge_bp.route('/capture_stats')
def capture_stats():
    """获取知识捕获统计（从 wr-proxy 的 SQLite 直接读 wr_knowledge_raw）"""
    # raw 表统计
    total_raw = db.session.query(func.count(KnowledgeRaw.id)).scalar() or 0
    pending_raw = db.session.query(func.count(KnowledgeRaw.id)).filter(
        KnowledgeRaw.status == 'pending'
    ).scalar() or 0
    processing_raw = db.session.query(func.count(KnowledgeRaw.id)).filter(
        KnowledgeRaw.status == 'processing'
    ).scalar() or 0
    done_raw = db.session.query(func.count(KnowledgeRaw.id)).filter(
        KnowledgeRaw.status == 'done'
    ).scalar() or 0
    skipped_raw = db.session.query(func.count(KnowledgeRaw.id)).filter(
        KnowledgeRaw.status == 'skipped'
    ).scalar() or 0

    # 知识条目统计
    total_items = db.session.query(func.count(KnowledgeItem.id)).scalar() or 0
    total_domains = db.session.query(func.count(KnowledgeDomain.id)).scalar() or 0

    # 按状态分组的知识条目
    items_by_type = db.session.query(
        KnowledgeItem.type,
        func.count(KnowledgeItem.id)
    ).group_by(KnowledgeItem.type).all()

    items_by_verification = db.session.query(
        KnowledgeItem.verification,
        func.count(KnowledgeItem.id)
    ).group_by(KnowledgeItem.verification).all()

    # 今日新增 raw 条目
    today_raw = db.session.query(func.count(KnowledgeRaw.id)).filter(
        KnowledgeRaw.created_at >= func.date('now')
    ).scalar() or 0

    return jsonify({
        'raw': {
            'total': total_raw,
            'pending': pending_raw,
            'processing': processing_raw,
            'done': done_raw,
            'skipped': skipped_raw,
            'today': today_raw,
        },
        'items': {
            'total': total_items,
            'by_type': {t: c for t, c in items_by_type},
            'by_verification': {v: c for v, c in items_by_verification},
        },
        'domains': {
            'total': total_domains,
        },
    })


# ============================================================
# Raw 数据列表
# ============================================================

@knowledge_bp.route('/raw')
def list_raw():
    """原始对话列表（分页）"""
    page = request.args.get('page', 1, type=int)
    per_page = request.args.get('per_page', 20, type=int)
    status = request.args.get('status', '')
    token_id = request.args.get('token_id', '', type=str)
    keyword = request.args.get('keyword', '', type=str)

    q = KnowledgeRaw.query
    if status:
        q = q.filter(KnowledgeRaw.status == status)
    if token_id:
        q = q.filter(KnowledgeRaw.token_id == token_id)
    if keyword:
        q = q.filter(
            db.or_(
                KnowledgeRaw.prompt.contains(keyword),
                KnowledgeRaw.response.contains(keyword),
            )
        )

    total = q.count()
    items = q.order_by(KnowledgeRaw.id.desc()).offset(
        (page - 1) * per_page
    ).limit(per_page).all()

    return jsonify({
        'items': [i.to_dict() for i in items],
        'total': total,
        'page': page,
        'per_page': per_page,
    })


@knowledge_bp.route('/raw/<int:raw_id>')
def get_raw(raw_id):
    """单条 raw 详情"""
    item = KnowledgeRaw.query.get(raw_id)
    if not item:
        return jsonify({'error': 'Not found'}), 404
    return jsonify(item.to_dict())


# ============================================================
# 知识条目列表 & 搜索
# ============================================================

@knowledge_bp.route('/items')
def list_items():
    """知识条目列表（分页+筛选）"""
    page = request.args.get('page', 1, type=int)
    per_page = request.args.get('per_page', 20, type=int)
    domain = request.args.get('domain', '')
    department = request.args.get('department', '')
    item_type = request.args.get('type', '')
    verification = request.args.get('verification', '')
    keyword = request.args.get('keyword', '')

    q = KnowledgeItem.query
    if domain:
        q = q.filter(KnowledgeItem.domain_code == domain)
    if department:
        q = q.filter(KnowledgeItem.department == department)
    if item_type:
        q = q.filter(KnowledgeItem.type == item_type)
    if verification:
        q = q.filter(KnowledgeItem.verification == verification)
    if keyword:
        q = q.filter(
            db.or_(
                KnowledgeItem.title.contains(keyword),
                KnowledgeItem.summary.contains(keyword),
                KnowledgeItem.source_quote.contains(keyword),
            )
        )

    total = q.count()
    items = q.order_by(KnowledgeItem.id.desc()).offset(
        (page - 1) * per_page
    ).limit(per_page).all()

    return jsonify({
        'items': [i.to_dict() for i in items],
        'total': total,
        'page': page,
        'per_page': per_page,
    })


@knowledge_bp.route('/items/<int:item_id>')
def get_item(item_id):
    """单条知识详情"""
    item = KnowledgeItem.query.get(item_id)
    if not item:
        return jsonify({'error': 'Not found'}), 404
    return jsonify(item.to_dict())


@knowledge_bp.route('/search')
def search_knowledge():
    """关键词搜索（LIKE 模糊匹配）"""
    keyword = request.args.get('q', '', type=str)
    if not keyword:
        return jsonify({'items': [], 'total': 0, 'keyword': ''})

    # 搜索 raw 表
    raw_count = KnowledgeRaw.query.filter(
        db.or_(
            KnowledgeRaw.prompt.contains(keyword),
            KnowledgeRaw.response.contains(keyword),
        )
    ).count()

    # 搜索知识条目
    item_q = KnowledgeItem.query.filter(
        db.or_(
            KnowledgeItem.title.contains(keyword),
            KnowledgeItem.summary.contains(keyword),
            KnowledgeItem.source_quote.contains(keyword),
        )
    )
    item_total = item_q.count()
    item_results = item_q.order_by(KnowledgeItem.id.desc()).limit(50).all()

    return jsonify({
        'keyword': keyword,
        'raw_count': raw_count,
        'items': {
            'total': item_total,
            'results': [i.to_dict() for i in item_results],
        },
    })


# ============================================================
# 业务域管理
# ============================================================

@knowledge_bp.route('/domains')
def list_domains():
    """业务域列表"""
    domains = KnowledgeDomain.query.order_by(KnowledgeDomain.id.asc()).all()
    return jsonify({
        'domains': [d.to_dict() for d in domains],
        'total': len(domains),
    })


@knowledge_bp.route('/domains', methods=['POST'])
def create_domain():
    """创建业务域"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    code = (data.get('domain_code') or '').strip()
    name = (data.get('domain_name') or '').strip()
    if not code or not name:
        return jsonify({'error': 'domain_code 和 domain_name 不能为空'}), 400

    # 检查重复
    existing = KnowledgeDomain.query.filter_by(domain_code=code).first()
    if existing:
        return jsonify({'error': f'域代码 {code} 已存在'}), 400

    domain = KnowledgeDomain(
        domain_code=code,
        domain_name=name,
        department=data.get('department', ''),
        status=data.get('status', 'pending'),
        auto_keywords=data.get('auto_keywords', ''),
        description=data.get('description', ''),
    )
    db.session.add(domain)
    db.session.commit()

    return jsonify({'message': '业务域已创建', 'domain': domain.to_dict()}), 201


@knowledge_bp.route('/domains/<domain_code>')
def get_domain(domain_code):
    """业务域详情"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': 'Not found'}), 404
    return jsonify(domain.to_dict())


@knowledge_bp.route('/domains/<domain_code>', methods=['PUT'])
def update_domain(domain_code):
    """更新业务域"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': 'Not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    if 'domain_name' in data:
        domain.domain_name = data['domain_name'].strip()
    if 'department' in data:
        domain.department = data['department'].strip()
    if 'status' in data:
        domain.status = data['status']
    if 'auto_keywords' in data:
        domain.auto_keywords = data['auto_keywords']
    if 'description' in data:
        domain.description = data['description']

    db.session.commit()
    return jsonify({'message': '业务域已更新', 'domain': domain.to_dict()})


# ============================================================
# 领域风险配置
# ============================================================

@knowledge_bp.route('/domain_risk')
def list_domain_risk():
    """所有领域风险配置"""
    configs = KnowledgeDomainRisk.query.all()
    return jsonify({
        'configs': [c.to_dict() for c in configs],
    })


@knowledge_bp.route('/domain_risk/<domain_code>')
def get_domain_risk(domain_code):
    """单个领域风险配置"""
    config = KnowledgeDomainRisk.query.get(domain_code)
    if not config:
        return jsonify({'error': 'Not found'}), 404
    return jsonify(config.to_dict())


@knowledge_bp.route('/domain_risk/<domain_code>', methods=['PUT'])
def update_domain_risk(domain_code):
    """更新领域风险配置"""
    config = KnowledgeDomainRisk.query.get(domain_code)
    if not config:
        return jsonify({'error': 'Not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    if 'risk_level' in data:
        config.risk_level = data['risk_level']
    if 'min_verification' in data:
        config.min_verification = data['min_verification']
    if 'max_age_days' in data:
        config.max_age_days = int(data['max_age_days'])
    if 'disclaimer_template' in data:
        config.disclaimer_template = data['disclaimer_template']
    if 'allow_factual_injection' in data:
        config.allow_factual_injection = bool(data['allow_factual_injection'])
    if 'allow_analytical_injection' in data:
        config.allow_analytical_injection = bool(data['allow_analytical_injection'])
    if 'allow_procedural_injection' in data:
        config.allow_procedural_injection = bool(data['allow_procedural_injection'])

    db.session.commit()
    return jsonify({'message': '风险配置已更新', 'config': config.to_dict()})


# ============================================================
# 分析记录
# ============================================================

@knowledge_bp.route('/analyses')
def list_analyses():
    """分析记录列表"""
    page = request.args.get('page', 1, type=int)
    per_page = request.args.get('per_page', 20, type=int)
    status = request.args.get('status', '')

    q = KnowledgeAnalysis.query
    if status:
        q = q.filter(KnowledgeAnalysis.status == status)

    total = q.count()
    items = q.order_by(KnowledgeAnalysis.id.desc()).offset(
        (page - 1) * per_page
    ).limit(per_page).all()

    return jsonify({
        'items': [i.to_dict() for i in items],
        'total': total,
        'page': page,
        'per_page': per_page,
    })


@knowledge_bp.route('/analyses/<task_id>')
def get_analysis(task_id):
    """分析详情"""
    item = KnowledgeAnalysis.query.get(task_id)
    if not item:
        return jsonify({'error': 'Not found'}), 404
    return jsonify(item.to_dict())


# ============================================================
# 知识分析
# ============================================================

@knowledge_bp.route('/analyze', methods=['POST'])
def analyze_knowledge():
    """发起知识分析（调用 wr-proxy 的 /admin/knowledge_analyze）"""
    import os
    import requests as req_lib

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    domain_code = data.get('domain_code', '').strip()
    if not domain_code:
        return jsonify({'error': 'domain_code 不能为空'}), 400

    # 调用 wr-proxy 的分析端点
    proxy_url = os.environ.get('WR_PROXY_URL', 'http://127.0.0.1:5051')
    try:
        resp = req_lib.post(
            f'{proxy_url}/admin/knowledge_analyze',
            json=data,
            timeout=120,
        )
        result = resp.json()
        return jsonify(result), resp.status_code
    except Exception as e:
        # wr-proxy 不可用时，返回本地统计
        items = KnowledgeItem.query.filter_by(domain_code=domain_code).all()
        return jsonify({
            'result': f'分析服务暂不可用。该域目前有 {len(items)} 条知识。',
            'status': 'local_fallback',
            'item_count': len(items),
        })


@knowledge_bp.route('/extract', methods=['POST'])
def extract_knowledge():
    """触发知识提取（调用 wr-proxy 的 /admin/knowledge_extract）"""
    import os
    import requests as req_lib

    data = request.get_json() or {}
    batch_size = data.get('batch_size', 5)

    proxy_url = os.environ.get('WR_PROXY_URL', 'http://127.0.0.1:5051')
    try:
        resp = req_lib.post(
            f'{proxy_url}/admin/knowledge_extract',
            json={'batch_size': batch_size},
            timeout=180,
        )
        result = resp.json()
        return jsonify(result), resp.status_code
    except Exception as e:
        return jsonify({
            'error': f'提取服务不可用: {e}',
        }), 503
