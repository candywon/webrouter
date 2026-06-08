# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""脱敏规则管理 API — CRUD + 规则重载通知 wr-proxy"""
import re
from flask import Blueprint, jsonify, request
from models.wr_models import DesensitizeRule
from extensions import db
from i18n.messages import get_message

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
        {'category': 'IDCARD', 'name': 'ID Number', 'pattern': r'[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]', 'level': 'standard'},
        {'category': 'BANKCARD', 'name': 'Bank Card Number', 'pattern': r'\b[3-6]\d{12,18}\b', 'level': 'standard'},
        {'category': 'APIKEY', 'name': 'API Key', 'pattern': r'(?:sk|sk_live|sk_test|key|api_key|apikey|secret|token|Bearer)\s*[:=]\s*["\']?[\w\-]{16,}["\']?', 'level': 'standard'},
        {'category': 'EMAIL', 'name': 'Email', 'pattern': r'[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}', 'level': 'standard'},
        {'category': 'PHONE', 'name': 'Phone Number', 'pattern': r'\b1[3-9]\d{9}\b', 'level': 'standard'},
        {'category': 'IP', 'name': 'IP Address', 'pattern': r'\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b', 'level': 'standard'},
    ]
    return jsonify({'builtin': builtin})


@desensitize_bp.route('/', methods=['POST'])
def create_rule():
    """创建脱敏规则"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    name = (data.get('name') or '').strip()
    if not name:
        return jsonify({'error': get_message('rule_name_required', request)}), 400

    rule_type = data.get('type', 'regex')
    if rule_type not in VALID_TYPES:
        return jsonify({'error': get_message('invalid_type', request).format(VALID_TYPES=VALID_TYPES)}), 400

    pattern = (data.get('pattern') or '').strip()
    if not pattern:
        return jsonify({'error': get_message('pattern_required', request)}), 400

    # 验证正则语法
    if rule_type == 'regex':
        try:
            re.compile(pattern)
        except re.error as e:
            return jsonify({'error': get_message('invalid_regex', request).format(e=e)}), 400

    category = data.get('category', 'CUSTOM')
    if category not in VALID_CATEGORIES:
        return jsonify({'error': get_message('invalid_category', request).format(VALID_CATEGORIES=VALID_CATEGORIES)}), 400

    level = data.get('level', 'standard')
    if level not in VALID_LEVELS:
        return jsonify({'error': get_message('invalid_level', request).format(VALID_LEVELS=VALID_LEVELS)}), 400

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
        'message': get_message('desensitize_rule_created', request),
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
        return jsonify({'error': get_message('no_data', request)}), 400

    if 'name' in data:
        rule.name = data['name'].strip()
    if 'type' in data:
        if data['type'] not in VALID_TYPES:
            return jsonify({'error': get_message('invalid_type', request).format(VALID_TYPES=VALID_TYPES)}), 400
        rule.type = data['type']
    if 'pattern' in data:
        pattern = data['pattern'].strip()
        if not pattern:
            return jsonify({'error': get_message('pattern_required', request)}), 400
        if rule.type == 'regex' or data.get('type') == 'regex':
            try:
                re.compile(pattern)
            except re.error as e:
                return jsonify({'error': get_message('invalid_regex', request).format(e=e)}), 400
        rule.pattern = pattern
    if 'category' in data:
        if data['category'] not in VALID_CATEGORIES:
            return jsonify({'error': get_message('invalid_category', request).format(VALID_CATEGORIES=VALID_CATEGORIES)}), 400
        rule.category = data['category']
    if 'level' in data:
        if data['level'] not in VALID_LEVELS:
            return jsonify({'error': get_message('invalid_level', request).format(VALID_LEVELS=VALID_LEVELS)}), 400
        rule.level = data['level']
    if 'enabled' in data:
        rule.enabled = bool(data['enabled'])
    if 'sort_order' in data:
        rule.sort_order = int(data['sort_order'])

    db.session.commit()

    _notify_reload()

    return jsonify({
        'message': get_message('desensitize_rule_updated', request),
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

    return jsonify({'message': get_message('desensitize_rule_deleted', request)})


@desensitize_bp.route('/test', methods=['POST'])
def test_rule():
    """测试脱敏规则（不持久化，按实际引擎顺序逐条执行）"""
    data = request.get_json()
    if not data or 'text' not in data:
        return jsonify({'error': get_message('field_required_text', request)}), 400

    text = data['text']
    custom_rules = data.get('rules', [])  # [{type, pattern, category, level}]

    # 内置规则（与 wr-proxy/desensitize.go 顺序一致：长规则优先）
    builtin_rules = [
        {'category': 'IDCARD', 'pattern': r'[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]', 'type': 'regex', 'is_builtin': True},
        {'category': 'BANKCARD', 'pattern': r'\b[3-6]\d{12,18}\b', 'type': 'regex', 'is_builtin': True},
        {'category': 'APIKEY', 'pattern': r'(?:sk|sk_live|sk_test|key|api_key|apikey|secret|token|Bearer)\s*[:=]\s*["\']?[\w\-]{16,}["\']?', 'type': 'regex', 'is_builtin': True},
        {'category': 'EMAIL', 'pattern': r'[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}', 'type': 'regex', 'is_builtin': True},
        {'category': 'PHONE', 'pattern': r'\b1[3-9]\d{9}\b', 'type': 'regex', 'is_builtin': True},
        {'category': 'IP', 'pattern': r'\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b', 'type': 'regex', 'is_builtin': True},
    ]

    working_text = text
    results = []
    counters = {}  # category → count

    def process_rules(rules_list):
        nonlocal working_text
        for r in rules_list:
            rule_type = r.get('type', 'regex')
            pattern = r.get('pattern', '')
            category = r.get('category', 'CUSTOM')
            is_builtin = r.get('is_builtin', False)

            try:
                if rule_type == 'regex':
                    regex = re.compile(pattern)
                    matches = regex.findall(working_text)
                elif rule_type == 'exact':
                    count = working_text.count(pattern)
                    matches = [pattern] if count > 0 else []
                else:
                    continue
            except re.error as e:
                results.append({'category': category, 'error': get_message('invalid_regex', request).format(e=e), 'is_builtin': is_builtin})
                continue

            if not matches:
                continue

            # 过滤掉包含方括号的匹配（说明是前一轮替换的残留）
            real_matches = [m for m in matches if '[' not in str(m) and ']' not in str(m)]
            if not real_matches:
                continue

            # 去重并生成标记
            counters.setdefault(category, 0)
            seen = {}
            for m in real_matches:
                m_str = str(m)
                if m_str in seen:
                    working_text = working_text.replace(m_str, seen[m_str])
                    continue
                counters[category] += 1
                n = counters[category]
                marker = f'[{category}_{n}]'
                seen[m_str] = marker
                working_text = working_text.replace(m_str, marker)

            results.append({
                'category': category,
                'pattern': pattern,
                'matches': real_matches,
                'count': len(real_matches),
                'is_builtin': is_builtin,
            })

    # 按顺序：先内置规则，再自定义规则
    process_rules(builtin_rules)
    process_rules(custom_rules)

    return jsonify({
        'original_text': text,
        'sanitized_text': working_text,
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
