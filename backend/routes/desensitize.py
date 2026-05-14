"""脱敏规则管理 API — CRUD + 规则重载通知 wr-proxy"""
import re
from flask import Blueprint, jsonify, request
from models.wr_models import DesensitizeRule
from extensions import db

desensitize_bp = Blueprint('desensitize', __name__)

VALID_TYPES = ('exact', 'regex')
VALID_CATEGORIES = ('PHONE', 'IDCARD', 'EMAIL', 'BANKCARD', 'IP', 'APIKEY', 'NAME', 'COMPANY', 'CUSTOM')
VALID_LEVELS = ('standard', 'strict')


@desensitize_bp.route('/')
def list_rules():
    """脱敏规则列表"""
    rules = DesensitizeRule.query.order_by(DesensitizeRule.sort_order, DesensitizeRule.id).all()
    return jsonify({
        'rules': [r.to_dict() for r in rules],
        'total': len(rules),
    })


@desensitize_bp.route('/builtin')
def list_builtin():
    """内置规则说明（不可编辑，仅展示）"""
    builtin = [
        {'category': 'IDCARD', 'name': '身份证号', 'pattern': r'[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]', 'level': 'standard'},
        {'category': 'BANKCARD', 'name': '银行卡号', 'pattern': r'\b[3-6]\d{12,18}\b', 'level': 'standard'},
        {'category': 'APIKEY', 'name': 'API密钥', 'pattern': r'(?:sk|sk_live|sk_test|key|api_key|apikey|secret|token|Bearer)\s*[:=]\s*["\']?[\w\-]{16,}["\']?', 'level': 'standard'},
        {'category': 'EMAIL', 'name': '邮箱', 'pattern': r'[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}', 'level': 'standard'},
        {'category': 'PHONE', 'name': '手机号', 'pattern': r'1[3-9]\d{9}', 'level': 'standard'},
        {'category': 'IP', 'name': 'IP地址', 'pattern': r'\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b', 'level': 'standard'},
    ]
    return jsonify({'builtin': builtin})


@desensitize_bp.route('/', methods=['POST'])
def create_rule():
    """创建脱敏规则"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    name = (data.get('name') or '').strip()
    if not name:
        return jsonify({'error': '规则名称不能为空'}), 400

    rule_type = data.get('type', 'regex')
    if rule_type not in VALID_TYPES:
        return jsonify({'error': f'type 必须为 {VALID_TYPES}'}), 400

    pattern = (data.get('pattern') or '').strip()
    if not pattern:
        return jsonify({'error': 'pattern 不能为空'}), 400

    # 验证正则语法
    if rule_type == 'regex':
        try:
            re.compile(pattern)
        except re.error as e:
            return jsonify({'error': f'正则语法错误: {e}'}), 400

    category = data.get('category', 'CUSTOM')
    if category not in VALID_CATEGORIES:
        return jsonify({'error': f'category 必须为 {VALID_CATEGORIES}'}), 400

    level = data.get('level', 'standard')
    if level not in VALID_LEVELS:
        return jsonify({'error': f'level 必须为 {VALID_LEVELS}'}), 400

    rule = DesensitizeRule(
        name=name,
        type=rule_type,
        pattern=pattern,
        category=category,
        level=level,
        enabled=bool(data.get('enabled', True)),
        sort_order=int(data.get('sort_order', 0)),
    )

    db.session.add(rule)
    db.session.commit()

    # 通知 wr-proxy 重载规则
    _notify_reload()

    return jsonify({
        'message': '脱敏规则创建成功',
        'rule': rule.to_dict(),
    }), 201


@desensitize_bp.route('/<int:rule_id>', methods=['PUT'])
def update_rule(rule_id):
    """更新脱敏规则"""
    rule = DesensitizeRule.query.get(rule_id)
    if not rule:
        return jsonify({'error': 'Rule not found'}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    if 'name' in data:
        rule.name = data['name'].strip()
    if 'type' in data:
        if data['type'] not in VALID_TYPES:
            return jsonify({'error': f'type 必须为 {VALID_TYPES}'}), 400
        rule.type = data['type']
    if 'pattern' in data:
        pattern = data['pattern'].strip()
        if not pattern:
            return jsonify({'error': 'pattern 不能为空'}), 400
        if rule.type == 'regex' or data.get('type') == 'regex':
            try:
                re.compile(pattern)
            except re.error as e:
                return jsonify({'error': f'正则语法错误: {e}'}), 400
        rule.pattern = pattern
    if 'category' in data:
        if data['category'] not in VALID_CATEGORIES:
            return jsonify({'error': f'category 必须为 {VALID_CATEGORIES}'}), 400
        rule.category = data['category']
    if 'level' in data:
        if data['level'] not in VALID_LEVELS:
            return jsonify({'error': f'level 必须为 {VALID_LEVELS}'}), 400
        rule.level = data['level']
    if 'enabled' in data:
        rule.enabled = bool(data['enabled'])
    if 'sort_order' in data:
        rule.sort_order = int(data['sort_order'])

    db.session.commit()

    _notify_reload()

    return jsonify({
        'message': '脱敏规则更新成功',
        'rule': rule.to_dict(),
    })


@desensitize_bp.route('/<int:rule_id>', methods=['DELETE'])
def delete_rule(rule_id):
    """删除脱敏规则"""
    rule = DesensitizeRule.query.get(rule_id)
    if not rule:
        return jsonify({'error': 'Rule not found'}), 404

    db.session.delete(rule)
    db.session.commit()

    _notify_reload()

    return jsonify({'message': '脱敏规则已删除'})


@desensitize_bp.route('/test', methods=['POST'])
def test_rule():
    """测试脱敏规则（不持久化，仅返回脱敏结果预览）"""
    data = request.get_json()
    if not data or 'text' not in data:
        return jsonify({'error': '需要 text 字段'}), 400

    text = data['text']
    rules = data.get('rules', [])  # [{type, pattern, category, level}]

    results = []
    for r in rules:
        rule_type = r.get('type', 'regex')
        pattern = r.get('pattern', '')
        category = r.get('category', 'CUSTOM')

        if rule_type == 'regex':
            try:
                matches = re.findall(pattern, text)
                if matches:
                    results.append({
                        'category': category,
                        'pattern': pattern,
                        'matches': matches,
                        'count': len(matches),
                    })
            except re.error:
                results.append({'category': category, 'error': '正则语法错误'})
        elif rule_type == 'exact':
            count = text.count(pattern)
            if count > 0:
                results.append({
                    'category': category,
                    'pattern': pattern,
                    'matches': [pattern],
                    'count': count,
                })

    return jsonify({
        'original_text': text,
        'results': results,
        'total_matches': sum(r.get('count', 0) for r in results),
    })


def _notify_reload():
    """通知 wr-proxy 重载规则"""
    try:
        import requests
        resp = requests.post('http://127.0.0.1:5051/admin/reload', timeout=3)
        if resp.status_code == 200:
            pass  # 成功
        else:
            from flask import current_app
            current_app.logger.warning(f'Notify wr-proxy reload failed: {resp.status_code}')
    except Exception:
        pass  # wr-proxy 可能未启动，忽略
