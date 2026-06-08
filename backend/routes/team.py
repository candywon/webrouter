# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""团队管理 API — 组织架构 + 成员（Token）管理"""
import json, asyncio, smtplib
from email.mime.text import MIMEText
from flask import Blueprint, jsonify, request
from models.wr_models import Org, WRToken, SystemSetting
from extensions import db
from sqlalchemy import func
from i18n.messages import get_message

team_bp = Blueprint('team', __name__)


# ══════════════════════════════════════════
#  邮件通知
# ══════════════════════════════════════════

def _parse_models_str(models_field):
    """解析 token.models 字段（可能是 JSON 字符串或逗号分隔）"""
    if not models_field:
        return []
    if isinstance(models_field, str):
        try:
            parsed = json.loads(models_field)
            return parsed if isinstance(parsed, list) else [models_field]
        except (json.JSONDecodeError, TypeError):
            return [m.strip() for m in models_field.split(',') if m.strip()]
    return models_field


def _send_member_email(token):
    """发送成员邀请/更新邮件，返回是否成功"""
    smtp_host = SystemSetting.get('alert_smtp_host', '')
    smtp_port = int(SystemSetting.get('alert_smtp_port', 587))
    smtp_user = SystemSetting.get('alert_smtp_user', '')
    smtp_password = SystemSetting.get('alert_smtp_password', '')
    smtp_from = SystemSetting.get('alert_smtp_from', smtp_user)

    if not smtp_host or not smtp_user:
        return False

    gateway_url = SystemSetting.get('gateway_url', 'http://localhost:5051')
    member_name = token.name or 'Member'
    api_key = token.key
    to_email = token.member_email

    # Parse model list
    models = _parse_models_str(token.models)
    if not models:
        providers = _parse_models_str(token.provider_ids)
        if providers:
            models = [f'Provider #{p}' for p in providers]

    models_text = '\n'.join(f'  - {m}' for m in models) if models else '  All available models'

    quota_text = 'Unlimited'
    if token.quota_total > 0:
        quota_text = f'¥{token.quota_total / 100:.2f}'

    rpm_text = 'Unlimited'
    if token.rate_limit_rpm > 0:
        rpm_text = f'{token.rate_limit_rpm} req/min'

    subject = f'WebRouter API Gateway Access Credentials — {member_name}'
    body = f"""Hello, {member_name}:

You have been added as a member of the WebRouter API gateway. Your access credentials are below:

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  API Key:    {api_key}
  Gateway:    {gateway_url}
  Quota:      {quota_text}
  Rate limit: {rpm_text}
  Allowed models:
{models_text}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Usage:
  Add the API Key to the request header:
    Authorization: Bearer {api_key}

  Example: request a specific model
    curl {gateway_url}/v1/chat/completions \\
      -H "Authorization: Bearer {api_key}" \\
      -H "Content-Type: application/json" \\
      -d '{{"model":"gpt-4o","messages":[{{"role":"user","content":"Hello"}}]}}'

  ✨ Smart model selection (recommended)
    Set model to "auto" or "smart" and the gateway will pick the best model based on request complexity:
    - Simple requests → economy models (fast, cheap)
    - Medium complexity → standard models
    - Complex reasoning → advanced models
    curl {gateway_url}/v1/chat/completions \\
      -H "Authorization: Bearer {api_key}" \\
      -H "Content-Type: application/json" \\
      -d '{{"model":"auto","messages":[{{"role":"user","content":"Help me analyze this code"}}]}}'

  Or access API endpoints through the gateway URL.

If you have any questions, please contact your administrator.
"""

    msg = MIMEText(body, 'plain', 'utf-8')
    msg['Subject'] = subject
    msg['From'] = smtp_from
    msg['To'] = to_email

    try:
        if smtp_port == 465:
            server = smtplib.SMTP_SSL(smtp_host, smtp_port, timeout=15)
        else:
            server = smtplib.SMTP(smtp_host, smtp_port, timeout=15)
            server.ehlo()
            server.starttls()

        if smtp_user and smtp_password:
            server.login(smtp_user, smtp_password)

        server.sendmail(smtp_from, [to_email], msg.as_string())
        server.quit()
        return True
    except Exception as e:
        print(f"Email send failed: {e}")
        return False


# ══════════════════════════════════════════
#  组织架构 CRUD
# ══════════════════════════════════════════

@team_bp.route('/tree')
def org_tree():
    """返回完整组织树（含成员数、额度使用率）"""
    roots = Org.query.filter_by(parent_id=None, enabled=True).order_by(Org.id).all()

    def build_node(org):
        children = Org.query.filter_by(parent_id=org.id, enabled=True).order_by(Org.id).all()
        member_count = WRToken.query.filter_by(org_id=org.id, enabled=True).count()
        quota_used = db.session.query(
            func.coalesce(func.sum(WRToken.quota_used), 0)
        ).filter_by(org_id=org.id, enabled=True).scalar()

        node = org.to_dict(member_count=member_count)
        node['quota_used'] = int(quota_used)
        node['children'] = [build_node(c) for c in children]
        return node

    return jsonify({'tree': [build_node(r) for r in roots]})


@team_bp.route('/orgs')
def org_list():
    """平铺组织列表（用于下拉选择）"""
    orgs = Org.query.filter_by(enabled=True).order_by(Org.parent_id, Org.id).all()
    return jsonify({
        'orgs': [{
            'id': o.id,
            'name': o.name,
            'org_type': o.org_type,
            'parent_id': o.parent_id,
        } for o in orgs],
    })


@team_bp.route('/orgs', methods=['POST'])
def create_org():
    """创建组织"""
    data = request.get_json()
    if not data or not data.get('name'):
        return jsonify({'error': get_message('org_name_required', request)}), 400

    name = data['name'].strip()
    org_type = data.get('org_type', 'department')
    if org_type not in ('company', 'department', 'group'):
        return jsonify({'error': get_message('invalid_org_type', request)}), 400

    parent_id = data.get('parent_id')
    if parent_id:
        parent = Org.query.get(parent_id)
        if not parent:
            return jsonify({'error': get_message('parent_org_not_found', request)}), 404

    quota_total = int(data.get('quota_total', 0))
    quota_period = data.get('quota_period', 'monthly')

    org = Org(
        name=name,
        org_type=org_type,
        parent_id=parent_id if parent_id else None,
        quota_total=quota_total,
        quota_period=quota_period,
    )
    db.session.add(org)
    db.session.commit()

    return jsonify({'message': get_message('org_created', request), 'org': org.to_dict()}), 201


@team_bp.route('/orgs/<int:org_id>', methods=['PUT'])
def update_org(org_id):
    """更新组织"""
    org = Org.query.get(org_id)
    if not org:
        return jsonify({'error': get_message('org_not_found', request)}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    if 'name' in data:
        org.name = data['name'].strip()
    if 'org_type' in data:
        if data['org_type'] not in ('company', 'department', 'group'):
            return jsonify({'error': get_message('invalid_org_type_value', request)}), 400
        org.org_type = data['org_type']
    if 'parent_id' in data:
        new_parent = data['parent_id']
        if new_parent and new_parent == org_id:
            return jsonify({'error': get_message('cannot_set_self_as_parent', request)}), 400
        if new_parent:
            parent = Org.query.get(new_parent)
            if not parent:
                return jsonify({'error': get_message('parent_org_not_found', request)}), 400
        org.parent_id = new_parent if new_parent else None
    if 'quota_total' in data:
        org.quota_total = int(data['quota_total'])
    if 'quota_period' in data:
        org.quota_period = data['quota_period']
    if 'enabled' in data:
        org.enabled = bool(data['enabled'])

    db.session.commit()
    return jsonify({'message': get_message('org_updated', request), 'org': org.to_dict()})


@team_bp.route('/orgs/<int:org_id>', methods=['DELETE'])
def delete_org(org_id):
    """删除组织（需无成员且无子组织）"""
    org = Org.query.get(org_id)
    if not org:
        return jsonify({'error': get_message('org_not_found', request)}), 404

    if WRToken.query.filter_by(org_id=org_id).count() > 0:
        return jsonify({'error': get_message('org_has_members', request)}), 400
    if Org.query.filter_by(parent_id=org_id).count() > 0:
        return jsonify({'error': get_message('org_has_children', request)}), 400

    db.session.delete(org)
    db.session.commit()
    return jsonify({'message': get_message('org_deleted', request)})


@team_bp.route('/orgs/<int:org_id>/members')
def org_members(org_id):
    """某组织下的成员列表（含直接成员和子组织成员）"""
    include_sub = request.args.get('sub', '1', type=int)  # 默认包含子组织

    if include_sub:
        # 递归查找所有子组织 ID
        child_ids = _get_child_org_ids(org_id)
        child_ids.append(org_id)
        tokens = WRToken.query.filter(WRToken.org_id.in_(child_ids)).order_by(WRToken.id.desc()).all()
    else:
        tokens = WRToken.query.filter_by(org_id=org_id).order_by(WRToken.id.desc()).all()

    return jsonify({
        'org_id': org_id,
        'members': [t.to_dict() for t in tokens],
        'total': len(tokens),
    })


def _get_child_org_ids(parent_id):
    """递归获取所有子组织 ID"""
    children = Org.query.filter_by(parent_id=parent_id).all()
    ids = [c.id for c in children]
    for c in children:
        ids.extend(_get_child_org_ids(c.id))
    return ids


# ══════════════════════════════════════════
#  成员（Token）CRUD
# ══════════════════════════════════════════

@team_bp.route('/members')
def members():
    """成员列表（兼容旧接口，返回全部）"""
    tokens = WRToken.query.order_by(WRToken.id.desc()).all()
    return jsonify({'members': [t.to_dict() for t in tokens]})


@team_bp.route('/members/keys')
def members_keys():
    """成员列表含完整 key（仅管理员 API 测试页使用）"""
    tokens = WRToken.query.order_by(WRToken.id.desc()).all()
    return jsonify({
        'members': [{
            'id': t.id,
            'name': t.name,
            'key': t.key,
            'enabled': t.enabled,
            'member_email': t.member_email,
        } for t in tokens],
    })


@team_bp.route('/members', methods=['POST'])
def invite_member():
    """创建新成员（即创建新 WR Token，关联 org_id）"""
    data = request.get_json()
    if not data or not data.get('name'):
        return jsonify({'error': get_message('name_required', request)}), 400

    org_id = data.get('org_id')
    if org_id:
        org = Org.query.get(org_id)
        if not org:
            return jsonify({'error': get_message('org_not_found', request)}), 404

    token = WRToken(
        name=data['name'].strip(),
        key=WRToken.generate_key(),
        org_id=int(org_id) if org_id else None,
        member_email=data.get('member_email', '').strip(),
        quota_total=int(data.get('quota_total', 0)),
        rate_limit_rpm=int(data.get('rate_limit_rpm', 0)),
        smart_downgrade=data.get('smart_downgrade', False),
        desensitize_enabled=data.get('desensitize_enabled', False),
        session_recall_enabled=data.get('session_recall_enabled', True),
        enabled=data.get('enabled', True),
    )

    # models 和 provider_ids 支持 JSON 或字符串
    if data.get('models'):
        m = data['models']
        token.models = json.dumps(m) if isinstance(m, list) else m
    if data.get('provider_ids'):
        p = data['provider_ids']
        token.provider_ids = json.dumps(p) if isinstance(p, list) else p
    if data.get('subnet_whitelist'):
        sw = data['subnet_whitelist']
        token.subnet_whitelist = json.dumps(sw) if isinstance(sw, list) else sw

    if data.get('expires_at'):
        from dateutil.parser import parse as parse_dt
        try:
            token.expires_at = parse_dt(data['expires_at'])
        except (ValueError, TypeError):
            return jsonify({'error': get_message('invalid_expires_at', request)}), 400

    db.session.add(token)
    db.session.commit()

    # 如果要求发邮件
    send_email = data.get('send_email', False)
    email_sent = False
    if send_email and token.member_email:
        email_sent = _send_member_email(token)

    return jsonify({
        'message': get_message('member_created', request),
        'id': token.id,
        'key': token.key,
        'email_sent': email_sent,
    }), 201


@team_bp.route('/members/<int:member_id>', methods=['PUT'])
def update_member(member_id):
    """更新成员配置"""
    token = WRToken.query.get(member_id)
    if not token:
        return jsonify({'error': get_message('member_not_found', request)}), 404

    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    if 'name' in data:
        token.name = data['name'].strip()
    if 'org_id' in data:
        new_org = data['org_id']
        if new_org:
            org = Org.query.get(new_org)
            if not org:
                return jsonify({'error': get_message('org_not_found', request)}), 404
        token.org_id = int(new_org) if new_org else None
    if 'member_email' in data:
        token.member_email = data['member_email'].strip()
    if 'quota_total' in data:
        token.quota_total = int(data['quota_total'])
    if 'rate_limit_rpm' in data:
        token.rate_limit_rpm = int(data['rate_limit_rpm'])
    if 'models' in data:
        m = data['models']
        token.models = json.dumps(m) if isinstance(m, list) else m
    if 'provider_ids' in data:
        p = data['provider_ids']
        token.provider_ids = json.dumps(p) if isinstance(p, list) else p
    if 'subnet_whitelist' in data:
        sw = data['subnet_whitelist']
        token.subnet_whitelist = json.dumps(sw) if isinstance(sw, list) else sw
    if 'smart_downgrade' in data:
        token.smart_downgrade = bool(data['smart_downgrade'])
    if 'desensitize_enabled' in data:
        token.desensitize_enabled = bool(data['desensitize_enabled'])
    if 'session_recall_enabled' in data:
        token.session_recall_enabled = bool(data['session_recall_enabled'])
    if 'enabled' in data:
        token.enabled = bool(data['enabled'])
    if 'expires_at' in data:
        if data['expires_at']:
            from dateutil.parser import parse as parse_dt
            try:
                token.expires_at = parse_dt(data['expires_at'])
            except (ValueError, TypeError):
                return jsonify({'error': get_message('invalid_expires_at', request)}), 400
        else:
            token.expires_at = None

    db.session.commit()

    # 如果要求发邮件（编辑成员时也支持通知）
    send_email = data.get('send_email', False)
    email_sent = False
    if send_email and token.member_email:
        email_sent = _send_member_email(token)

    return jsonify({'message': get_message('member_updated', request), 'member': token.to_dict(), 'email_sent': email_sent})


@team_bp.route('/members/<int:member_id>', methods=['DELETE'])
def remove_member(member_id):
    """禁用成员"""
    token = WRToken.query.get(member_id)
    if token:
        token.enabled = False
        db.session.commit()
    return jsonify({'removed': member_id})


@team_bp.route('/members/<int:member_id>/move', methods=['PUT'])
def move_member(member_id):
    """转移成员到其他组织"""
    token = WRToken.query.get(member_id)
    if not token:
        return jsonify({'error': get_message('member_not_found', request)}), 404

    data = request.get_json()
    if not data or 'org_id' not in data:
        return jsonify({'error': get_message('org_id_required', request)}), 400

    new_org_id = data['org_id']
    if new_org_id:
        org = Org.query.get(new_org_id)
        if not org:
            return jsonify({'error': get_message('target_org_not_found', request)}), 404
    token.org_id = int(new_org_id) if new_org_id else None

    db.session.commit()
    return jsonify({'message': get_message('member_transferred', request), 'member': token.to_dict()})


@team_bp.route('/members/<int:member_id>/usage')
def member_usage(member_id):
    """成员用量 — 从 wr_request_logs 查询"""
    days = request.args.get('days', 30, type=int)

    records = db.session.query(
        WRToken.model_name if hasattr(WRToken, 'model_name') else None,  # RequestLog model
    ).filter(False).all()  # placeholder

    from models.wr_models import RequestLog
    records = db.session.query(
        RequestLog.model_name,
        func.sum(RequestLog.input_tokens),
        func.sum(RequestLog.output_tokens),
        func.sum(RequestLog.cost_cents),
    ).filter(
        RequestLog.token_id == member_id,
        RequestLog.created_at >= func.date('now', f'-{days} days'),
    ).group_by(RequestLog.model_name).all()

    data = [{
        'model': r[0],
        'input_tokens': r[1] or 0,
        'output_tokens': r[2] or 0,
        'cost_cents': r[3] or 0,
    } for r in records]

    return jsonify({'member_id': member_id, 'days': days, 'data': data})


@team_bp.route('/members/batch', methods=['POST'])
def batch_import_members():
    """批量导入成员 — 支持逐条创建和批量文本导入"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    results = {'success': [], 'errors': [], 'emails_sent': 0}
    send_email = data.get('send_email', False)
    default_quota = int(data.get('quota_total', 0))
    default_rpm = int(data.get('rate_limit_rpm', 0))
    default_session_recall = data.get('session_recall_enabled', True)

    # 支持两种模式：
    # 1. members: [{name, org_id, member_email, ...}, ...]
    # 2. text: "部门 姓名 email\n部门 姓名 email\n..." + org_id_map: {部门名: org_id}
    members_to_create = []

    if 'text' in data and data['text'].strip():
        # 批量文本导入模式（部门 姓名 email）
        org_id_map = data.get('org_id_map', {})
        for line in data['text'].strip().split('\n'):
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            parts = line.split()
            if len(parts) < 3:
                results['errors'].append({'line': line, 'reason': get_message('invalid_format_dept_name_email', request)})
                continue
            dept_name, name, email = parts[0], parts[1], parts[2]
            org_id = org_id_map.get(dept_name)
            if not org_id:
                org = Org.query.filter_by(name=dept_name, enabled=True).first()
                if org:
                    org_id = org.id
                else:
                    # 部门不存在时自动创建
                    new_org = Org(name=dept_name, org_type='department', quota_total=0, quota_period='monthly')
                    db.session.add(new_org)
                    db.session.flush()
                    org_id = new_org.id
            members_to_create.append({
                'name': name,
                'org_id': org_id,
                'member_email': email,
                'quota_total': default_quota,
                'rate_limit_rpm': default_rpm,
            })
    elif 'members' in data and isinstance(data['members'], list):
        members_to_create = data['members']
    else:
        return jsonify({'error': get_message('members_array_or_text_required', request)}), 400

    for item in members_to_create:
        try:
            if not item.get('name'):
                results['errors'].append({'name': item.get('name', ''), 'reason': get_message('name_required', request)})
                continue

            org_id = item.get('org_id')
            if org_id:
                org = Org.query.get(org_id)
                if not org:
                    results['errors'].append({'name': item['name'], 'reason': get_message('org_id_not_found', request).format(org_id=org_id)})
                    continue

            token = WRToken(
                name=item['name'].strip(),
                key=WRToken.generate_key(),
                org_id=int(org_id) if org_id else None,
                member_email=item.get('member_email', '').strip(),
                quota_total=int(item.get('quota_total', default_quota)),
                rate_limit_rpm=int(item.get('rate_limit_rpm', default_rpm)),
                smart_downgrade=item.get('smart_downgrade', False),
                desensitize_enabled=item.get('desensitize_enabled', False),
                session_recall_enabled=item.get('session_recall_enabled', default_session_recall),
                enabled=item.get('enabled', True),
            )

            if item.get('models'):
                m = item['models']
                token.models = json.dumps(m) if isinstance(m, list) else m
            if item.get('provider_ids'):
                p = item['provider_ids']
                token.provider_ids = json.dumps(p) if isinstance(p, list) else p
            if item.get('subnet_whitelist'):
                sw = item['subnet_whitelist']
                token.subnet_whitelist = json.dumps(sw) if isinstance(sw, list) else sw

            db.session.add(token)
            db.session.flush()  # 获取 ID

            # 发送邮件
            email_sent = False
            if send_email and token.member_email:
                email_sent = _send_member_email(token)
                if email_sent:
                    results['emails_sent'] += 1

            results['success'].append({
                'id': token.id,
                'name': token.name,
                'key': token.key,
                'email_sent': email_sent,
            })
        except Exception as e:
            results['errors'].append({'name': item.get('name', ''), 'reason': str(e)})

    try:
        db.session.commit()
    except Exception as e:
        db.session.rollback()
        return jsonify({'error': get_message('db_commit_failed', request).format(e=str(e))}), 500

    return jsonify({
        'message': get_message('batch_import_done', request).format(success=len(results["success"]), failed=len(results["errors"])),
        'results': results,
    }), 201
