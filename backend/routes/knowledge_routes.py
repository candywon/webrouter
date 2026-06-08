# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""企业知识库 API — 知识列表/搜索/统计/域名管理"""
import json
from flask import Blueprint, jsonify, request
from sqlalchemy import func
from extensions import db
from i18n.messages import get_message
from models.knowledge import (
    KnowledgeRaw, KnowledgeItem, KnowledgeDomain,
    KnowledgeDomainRisk, KnowledgeAnalysis, AuditLog,
)
from models.wr_models import SystemSetting

knowledge_bp = Blueprint('knowledge', __name__)


@knowledge_bp.route('/status')
def knowledge_status():
    """Return whether the Knowledge Base feature has been enabled."""
    enabled = bool(SystemSetting.get('knowledge_enabled', False))
    return jsonify({'enabled': enabled})


@knowledge_bp.route('/enable', methods=['POST'])
def enable_knowledge():
    """Enable the Knowledge Base feature after explicit confirmation."""
    data = request.get_json(silent=True) or {}
    if not data.get('confirmed'):
        return jsonify({'enabled': False, 'error': 'Confirmation is required'}), 400

    SystemSetting.set(
        'knowledge_enabled',
        True,
        value_type='bool',
        description='Enable the Knowledge Base module',
        category='knowledge',
        editable=True,
    )
    return jsonify({'enabled': True})


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
        return jsonify({'error': get_message('not_found', request)}), 404
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
    if department:
        q = q.filter(KnowledgeItem.department == department)
    if domain:
        q = q.filter(KnowledgeItem.domain_code == domain)
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
        return jsonify({'error': get_message('not_found', request)}), 404
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
        return jsonify({'error': get_message('no_data', request)}), 400

    code = (data.get('domain_code') or '').strip()
    name = (data.get('domain_name') or '').strip()
    if not code or not name:
        return jsonify({'error': get_message('domain_name_required', request)}), 400

    # 检查重复
    existing = KnowledgeDomain.query.filter_by(domain_code=code).first()
    if existing:
        return jsonify({'error': get_message('domain_code_exists', request).format(code=code)}), 400

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

    return jsonify({'message': get_message('domain_created', request), 'domain': domain.to_dict()}), 201


@knowledge_bp.route('/domains/<domain_code>')
def get_domain(domain_code):
    """业务域详情"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': get_message('not_found', request)}), 404
    return jsonify(domain.to_dict())


@knowledge_bp.route('/domains/<domain_code>', methods=['PUT'])
def update_domain(domain_code):
    """更新业务域"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': get_message('not_found', request)}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

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
    return jsonify({'message': get_message('domain_updated', request), 'domain': domain.to_dict()})


@knowledge_bp.route('/domains/<domain_code>/confirm', methods=['POST'])
def confirm_domain(domain_code):
    """确认业务域（pending → active）"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': get_message('not_found', request)}), 404

    domain.status = 'active'
    domain.confirmed_at = datetime.utcnow()
    db.session.commit()

    return jsonify({'message': get_message('domain_confirmed', request), 'domain': domain.to_dict()})


@knowledge_bp.route('/domains/<domain_code>/merge', methods=['POST'])
def merge_domain(domain_code):
    """合并业务域：将当前域合并到目标域"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': get_message('not_found', request)}), 404

    data = request.get_json()
    target_code = data.get('target_code', '').strip()
    if not target_code:
        return jsonify({'error': get_message('field_required_target_code', request)}), 400

    target = KnowledgeDomain.query.get(target_code)
    if not target:
        return jsonify({'error': get_message('target_domain_not_found', request).format(target_code=target_code)}), 404

    # 将该域下的知识条目迁移到目标域
    migrated = KnowledgeItem.query.filter_by(domain_code=domain_code).update(
        {'domain_code': target_code}, synchronize_session='fetch'
    )

    # 标记当前域为 merged
    domain.status = 'merged'
    domain.merged_into = target.id
    domain.sample_count = 0
    db.session.commit()

    return jsonify({
        'message': get_message('merged_to_domain', request).format(domain=target.domain_name, migrated=migrated),
        'migrated': migrated,
        'domain': domain.to_dict(),
    })


@knowledge_bp.route('/domains/<domain_code>/stats')
def domain_stats(domain_code):
    """业务域统计信息"""
    domain = KnowledgeDomain.query.get(domain_code)
    if not domain:
        return jsonify({'error': get_message('not_found', request)}), 404

    # 知识条目统计
    from sqlalchemy import func

    total = KnowledgeItem.query.filter_by(domain_code=domain_code).count()
    by_type = db.session.query(
        KnowledgeItem.type, func.count(KnowledgeItem.id)
    ).filter_by(domain_code=domain_code).group_by(KnowledgeItem.type).all()
    by_verification = db.session.query(
        KnowledgeItem.verification, func.count(KnowledgeItem.id)
    ).filter_by(domain_code=domain_code).group_by(KnowledgeItem.verification).all()
    raw_count = db.session.query(
        func.count(KnowledgeRaw.id)
    ).filter(
        KnowledgeRaw.status.in_(['pending', 'processing'])
    ).scalar() or 0

    return jsonify({
        'domain': domain.to_dict(),
        'items': {
            'total': total,
            'by_type': {t: c for t, c in by_type},
            'by_verification': {v: c for v, c in by_verification},
        },
        'raw_pending': raw_count,
    })


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
        return jsonify({'error': get_message('not_found', request)}), 404
    return jsonify(config.to_dict())


@knowledge_bp.route('/domain_risk/<domain_code>', methods=['PUT'])
def update_domain_risk(domain_code):
    """更新领域风险配置"""
    config = KnowledgeDomainRisk.query.get(domain_code)
    if not config:
        return jsonify({'error': get_message('not_found', request)}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

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
    return jsonify({'message': get_message('risk_config_updated', request), 'config': config.to_dict()})


# ============================================================
# 分析记录
# ============================================================

def _audit_log_to_dict(item):
    detail = item.detail
    if detail:
        try:
            detail = json.loads(detail)
        except (TypeError, ValueError):
            pass

    return {
        'id': item.id,
        'action': item.action,
        'resource_type': item.resource_type,
        'resource_id': item.resource_id,
        'token_id': item.token_id,
        'detail': detail,
        'client_ip': item.client_ip,
        'created_at': item.created_at.isoformat() if item.created_at else None,
    }


@knowledge_bp.route('/audit_log')
def audit_log():
    """Audit log list."""
    page = max(1, request.args.get('page', 1, type=int))
    per_page = request.args.get('per_page', 20, type=int)
    per_page = max(1, min(per_page, 100))

    q = AuditLog.query.order_by(AuditLog.created_at.desc(), AuditLog.id.desc())
    total = q.count()
    items = q.offset((page - 1) * per_page).limit(per_page).all()

    return jsonify({
        'items': [_audit_log_to_dict(item) for item in items],
        'total': total,
        'page': page,
        'per_page': per_page,
    })


@knowledge_bp.route('/audit_log/<int:log_id>')
def get_audit_log(log_id):
    """Audit log detail."""
    item = AuditLog.query.get(log_id)
    if not item:
        return jsonify({'error': get_message('not_found', request)}), 404
    return jsonify(_audit_log_to_dict(item))


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
        return jsonify({'error': get_message('not_found', request)}), 404
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
        return jsonify({'error': get_message('no_data', request)}), 400

    domain_code = data.get('domain_code', '').strip()
    if not domain_code:
        return jsonify({'error': get_message('domain_code_required', request)}), 400

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
            'result': get_message('analysis_service_unavailable', request).format(n=len(items)),
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
            'error': get_message('extract_service_unavailable', request).format(e=e),
        }), 503


# ============================================================
# 审核队列
# ============================================================

@knowledge_bp.route('/reviews')
def list_reviews():
    """审核队列：pending 知识条目列表"""
    page = request.args.get('page', 1, type=int)
    per_page = request.args.get('per_page', 20, type=int)
    domain = request.args.get('domain', '')
    item_type = request.args.get('type', '')

    q = KnowledgeItem.query.filter_by(verification='pending')
    if domain:
        q = q.filter(KnowledgeItem.domain_code == domain)
    if item_type:
        q = q.filter(KnowledgeItem.type == item_type)

    total = q.count()
    items = q.order_by(KnowledgeItem.id.asc()).offset(
        (page - 1) * per_page
    ).limit(per_page).all()

    return jsonify({
        'items': [i.to_dict() for i in items],
        'total': total,
        'page': page,
        'per_page': per_page,
    })


@knowledge_bp.route('/reviews/<int:item_id>/approve', methods=['POST'])
def approve_item(item_id):
    """审核通过：标记为 verified"""
    item = KnowledgeItem.query.get(item_id)
    if not item:
        return jsonify({'error': get_message('not_found', request)}), 404

    data = request.get_json() or {}

    # 可选：编辑标题/摘要/数据点
    if 'title' in data:
        item.title = data['title'].strip()
    if 'summary' in data:
        item.summary = data['summary']
    if 'data_points' in data:
        item.data_points = json.dumps(data['data_points']) if isinstance(data['data_points'], list) else data['data_points']

    item.verification = 'verified'
    item.verified_at = datetime.utcnow()
    db.session.commit()

    return jsonify({'message': get_message('approved', request), 'item': item.to_dict()})


@knowledge_bp.route('/reviews/<int:item_id>/reject', methods=['POST'])
def reject_item(item_id):
    """审核拒绝：标记为 rejected"""
    item = KnowledgeItem.query.get(item_id)
    if not item:
        return jsonify({'error': get_message('not_found', request)}), 404

    item.verification = 'rejected'
    item.verified_at = datetime.utcnow()
    db.session.commit()

    return jsonify({'message': get_message('rejected', request), 'item': item.to_dict()})


@knowledge_bp.route('/reviews/<int:item_id>/edit', methods=['PUT'])
def edit_review_item(item_id):
    """编辑审核中的知识条目"""
    item = KnowledgeItem.query.get(item_id)
    if not item:
        return jsonify({'error': get_message('not_found', request)}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    if 'title' in data:
        item.title = data['title'].strip()
    if 'summary' in data:
        item.summary = data['summary']
    if 'data_points' in data:
        item.data_points = json.dumps(data['data_points']) if isinstance(data['data_points'], list) else data['data_points']
    if 'confidence' in data:
        item.confidence = float(data['confidence'])
    if 'type' in data:
        item.type = data['type']

    db.session.commit()
    return jsonify({'message': get_message('updated', request), 'item': item.to_dict()})


@knowledge_bp.route('/reviews/batch-approve', methods=['POST'])
def batch_approve():
    """批量审核通过"""
    data = request.get_json()
    if not data or 'ids' not in data:
        return jsonify({'error': get_message('field_required_ids', request)}), 400

    ids = data.get('ids', [])
    count = KnowledgeItem.query.filter(
        KnowledgeItem.id.in_(ids),
        KnowledgeItem.verification == 'pending'
    ).update({
        'verification': 'verified',
        'verified_at': datetime.utcnow(),
    }, synchronize_session='fetch')

    db.session.commit()
    return jsonify({'message': get_message('batch_approved', request).format(count=count), 'count': count})


# ============================================================
# Week 7+8: RAG 反馈 / 记忆 / 导出 / 压缩
# ============================================================

def _proxy_url(path):
    """构造 wr-proxy URL"""
    import os
    base = os.environ.get('WR_PROXY_URL', 'http://127.0.0.1:5051')
    return f'{base}{path}'


@knowledge_bp.route('/rag_feedback', methods=['POST'])
def rag_feedback():
    """提交 RAG 反馈（代理到 wr-proxy）"""
    import requests as req_lib
    try:
        resp = req_lib.post(
            _proxy_url('/admin/knowledge/rag_feedback'),
            json=request.get_json(),
            timeout=10,
        )
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('rag_feedback_service_unavailable', request).format(e=e)}), 503


@knowledge_bp.route('/rag_feedback_stats')
def rag_feedback_stats():
    """RAG 反馈统计"""
    import requests as req_lib
    try:
        resp = req_lib.get(_proxy_url('/admin/knowledge/rag_feedback_stats'), timeout=10)
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('rag_stats_service_unavailable', request).format(e=e)}), 503


@knowledge_bp.route('/memory_list')
def memory_list():
    """记忆列表"""
    import requests as req_lib
    try:
        params = {k: v for k, v in request.args.items()}
        resp = req_lib.get(_proxy_url('/admin/knowledge/memory_list'), params=params, timeout=10)
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('memory_service_unavailable', request).format(e=e)}), 503


@knowledge_bp.route('/memory/<int:memory_id>', methods=['DELETE'])
def memory_delete(memory_id):
    """删除记忆（本地 DB 操作）"""
    from models.knowledge import AgentMemory
    token_id = request.args.get('token_id', 0, type=int)
    mem = AgentMemory.query.get(memory_id)
    if not mem or (token_id and mem.token_id != token_id):
        return jsonify({'error': get_message('not_found', request)}), 404
    db.session.delete(mem)
    db.session.commit()
    return jsonify({'message': get_message('memory_deleted', request)})


@knowledge_bp.route('/memory/<int:memory_id>', methods=['PUT'])
def memory_update(memory_id):
    """更新记忆（本地 DB 操作）"""
    from models.knowledge import AgentMemory
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400
    token_id = request.args.get('token_id', 0, type=int)
    mem = AgentMemory.query.get(memory_id)
    if not mem or (token_id and mem.token_id != token_id):
        return jsonify({'error': get_message('not_found', request)}), 404
    if 'content' in data:
        mem.content = data['content']
    if 'title' in data:
        mem.title = data['title'].strip()
    if 'priority' in data:
        mem.priority = int(data['priority'])
    if 'expires_at' in data and data['expires_at']:
        mem.expires_at = data['expires_at']
    db.session.commit()
    return jsonify({'message': get_message('memory_updated', request), 'memory': mem.to_dict()})


@knowledge_bp.route('/knowledge_export')
def knowledge_export():
    """知识导出（代理到 wr-proxy）"""
    import requests as req_lib
    try:
        params = {k: v for k, v in request.args.items()}
        resp = req_lib.get(_proxy_url('/admin/knowledge/export'), params=params, timeout=30)
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('export_service_unavailable', request).format(e=e)}), 503


@knowledge_bp.route('/conversation_compress', methods=['POST'])
def conversation_compress():
    """对话压缩（代理到 wr-proxy）"""
    import requests as req_lib
    try:
        resp = req_lib.post(
            _proxy_url('/admin/knowledge/conversation_compress'),
            json=request.get_json(),
            timeout=60,
        )
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('compress_service_unavailable', request).format(e=e)}), 503


@knowledge_bp.route('/rag_stats')
def rag_stats():
    """RAG 向量缓存 + 注入统计（代理到 wr-proxy）"""
    import requests as req_lib
    try:
        resp = req_lib.get(_proxy_url('/admin/knowledge/rag_stats'), timeout=10)
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('rag_stats_service_unavailable', request).format(e=e)}), 503


@knowledge_bp.route('/embedding_backfill', methods=['POST'])
def embedding_backfill():
    """向量批量生成（代理到 wr-proxy）"""
    import requests as req_lib
    data = request.get_json() or {}
    try:
        resp = req_lib.post(
            _proxy_url('/admin/knowledge/embedding_backfill'),
            json=data,
            timeout=300,
        )
        return jsonify(resp.json()), resp.status_code
    except Exception as e:
        return jsonify({'error': get_message('vector_service_unavailable', request).format(e=e)}), 503
