# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""系统设置 API — 持久化到 wr_system_settings 表"""
import json
import os
import smtplib
import logging
import requests as _requests
from email.mime.text import MIMEText
from flask import Blueprint, jsonify, request
from flask import current_app
from models.wr_models import SystemSetting
from extensions import db
from i18n.messages import get_message

logger = logging.getLogger(__name__)

settings_bp = Blueprint('settings', __name__)


@settings_bp.route('/', strict_slashes=False)
def get_settings():
    """获取系统设置（优先从 DB 读取，fallback 到 config）"""
    app = current_app._get_current_object()

    settings_map = {
        'proxy_url': lambda: f"http://localhost:{app.config.get('PROXY_PORT', 5051)}",
        'gateway_url': lambda: f"http://localhost:{app.config.get('PROXY_PORT', 5051)}",
        'proxy_enabled': lambda: True,
        'health_check_interval': lambda: app.config.get('HEALTH_CHECK_INTERVAL', 300),
        'alert_cooldown': lambda: app.config.get('ALERT_COOLDOWN', 300),
        'timezone': lambda: app.config.get('TZ', 'Asia/Shanghai'),
        'routing_strategy': lambda: app.config.get('ROUTING_STRATEGY', 'smart'),
        'default_timeout': lambda: app.config.get('DEFAULT_TIMEOUT', 60),
        'max_retry_count': lambda: app.config.get('MAX_RETRY_COUNT', 2),
        'max_failover': lambda: app.config.get('MAX_FAILOVER', 3),
        'quota_warn_threshold': lambda: app.config.get('QUOTA_WARN_THRESHOLD', 0.2),
        'quota_critical_threshold': lambda: app.config.get('QUOTA_CRITICAL_THRESHOLD', 0.05),
        'prediction_days': lambda: app.config.get('PREDICTION_DAYS', 7),
        'idle_conn_timeout': lambda: app.config.get('IDLE_CONN_TIMEOUT', 90),
        'max_idle_conns': lambda: app.config.get('MAX_IDLE_CONNS', 100),
        'log_retention_days': lambda: 30,
        'health_test_configs': lambda: [],
        # wr-proxy 优化特性开关
        'feature_dynamic_content_last': lambda: False,
        'feature_token_compression': lambda: False,
        'feature_session_compression': lambda: False,
        # 智能模型选择 — 六维度复杂度配置
        'smart_complexity_config': lambda: None,
    }

    result = {}
    for key, default_fn in settings_map.items():
        val = SystemSetting.get(key)
        result[key] = val if val is not None else default_fn()

    return jsonify(result)


@settings_bp.route('/', methods=['PUT'], strict_slashes=False)
def update_settings():
    """批量更新系统设置（写入数据库）"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    updated = []
    errors = []

    for key, value in data.items():
        try:
            s = SystemSetting.query.filter_by(key=key).first()
            if not s:
                if isinstance(value, bool):
                    vtype = 'bool'
                elif isinstance(value, int):
                    vtype = 'int'
                elif isinstance(value, float):
                    vtype = 'float'
                elif isinstance(value, (dict, list)):
                    vtype = 'json'
                else:
                    vtype = 'string'
                s = SystemSetting(
                    key=key,
                    value=json.dumps(value, ensure_ascii=False),
                    value_type=vtype,
                    description='',
                    category='general',
                    editable=True,
                )
                db.session.add(s)
            else:
                if not s.editable:
                    errors.append(get_message('setting_not_editable', request).format(key=key))
                    continue
                s.value = json.dumps(value, ensure_ascii=False)
                if isinstance(value, bool) and s.value_type != 'bool':
                    s.value_type = 'bool'
                elif isinstance(value, int) and s.value_type != 'int':
                    s.value_type = 'int'
                elif isinstance(value, float) and s.value_type != 'float':
                    s.value_type = 'float'
                s.updated_at = db.func.now()

            updated.append(key)
        except Exception as e:
            errors.append(f'{key}: {str(e)}')

    try:
        db.session.commit()
    except Exception as e:
        db.session.rollback()
        return jsonify({'error': get_message('db_commit_failed', request).format(e=str(e))}), 500

    result = {'message': get_message('settings_updated_count', request).format(len=len(updated)), 'updated': updated}
    if errors:
        result['errors'] = errors
    return jsonify(result)


@settings_bp.route('/all', strict_slashes=False)
def list_all_settings():
    """列出所有设置项（含元信息，供管理页面使用）"""
    settings = SystemSetting.query.order_by(SystemSetting.category, SystemSetting.key).all()
    return jsonify({
        'settings': [s.to_dict() for s in settings],
        'total': len(settings),
    })


@settings_bp.route('/<string:key>', methods=['PUT'], strict_slashes=False)
def update_single_setting(key):
    """更新单个设置项"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    value = data.get('value')
    if value is None:
        return jsonify({'error': get_message('key_required', request)}), 400

    s = SystemSetting.query.filter_by(key=key).first()
    if not s:
        return jsonify({'error': get_message('setting_not_found', request).format(key=key)}), 404

    if not s.editable:
        return jsonify({'error': get_message('setting_not_editable', request).format(key=key)}), 403

    s.value = json.dumps(value, ensure_ascii=False)
    s.updated_at = db.func.now()
    db.session.commit()

    return jsonify({'message': get_message('settings_updated', request), 'setting': s.to_dict()})


@settings_bp.route('/<string:key>', methods=['DELETE'], strict_slashes=False)
def delete_setting(key):
    """删除自定义设置项（种子设置不可删）"""
    seed_keys = ['proxy_enabled', 'proxy_url', 'gateway_url', 'health_check_interval', 'alert_cooldown', 'timezone',
                 'routing_strategy', 'default_timeout', 'max_retry_count', 'max_failover',
                 'quota_warn_threshold', 'quota_critical_threshold', 'prediction_days',
                 'idle_conn_timeout', 'max_idle_conns',
                 'feature_dynamic_content_last', 'feature_token_compression', 'feature_session_compression',
                 'smart_complexity_config']

    if key in seed_keys:
        return jsonify({'error': f'Cannot delete seed setting {key}'}), 403

    s = SystemSetting.query.filter_by(key=key).first()
    if not s:
        return jsonify({'error': get_message('setting_not_found', request).format(key=key)}), 404

    db.session.delete(s)
    db.session.commit()
    return jsonify({'deleted': key})


@settings_bp.route('/backup', methods=['POST'])
def create_backup():
    """创建备份"""
    import shutil
    from datetime import datetime

    db_uri = current_app.config.get('SQLALCHEMY_DATABASE_URI', '')
    if db_uri.startswith('sqlite:///'):
        db_path = db_uri.replace('sqlite:///', '')
        if os.path.exists(db_path):
            ts = datetime.now().strftime('%Y%m%d_%H%M%S')
            backup_path = db_path + f'.backup_{ts}'
            shutil.copy2(db_path, backup_path)
            return jsonify({'backup': backup_path})

    return jsonify({'message': get_message('backup_sqlite_only', request)})


FEATURE_TOGGLE_DEFS = [
    {
        'key': 'feature_dynamic_content_last',
        'value': False,
        'description': (
            'Dynamic content tailing: move dynamic user message content such as URLs, timestamps, '
            'and random values to the end so the prompt prefix stays as static as possible and improves upstream prompt cache hits. '
            'When enabled, wr-proxy reorders the messages array in the request body and moves messages containing URLs, dates, or numbers '
            'to the end of the same-role group.'
        ),
        'category': 'advanced',
    },
    {
        'key': 'feature_token_compression',
        'value': False,
        'description': (
            'Token compression (RTK - Return Token Key): compresses system prompts and long context before sending requests upstream. '
            'A lightweight model such as qwen-turbo summarizes long text first to reduce input tokens. '
            'This is useful for very long system prompts (>4000 tokens) and can significantly reduce cost, '
            'with a small loss of context precision.'
        ),
        'category': 'advanced',
    },
    {
        'key': 'feature_session_compression',
        'value': False,
        'description': (
            'Session compression: compresses historical messages in multi-turn conversations. '
            'When the conversation exceeds the configured threshold, earlier messages are merged into a summary to reduce context length. '
            'This is useful for long conversations such as support or tutoring sessions. '
            'Compression may lose some details, so it is best for scenarios with moderate context precision requirements.'
        ),
        'category': 'advanced',
    },
]

# 智能模型选择 — 六维度复杂度默认配置
DEFAULT_COMPLEXITY_CONFIG = {
    "tier_thresholds": {
        "simple_max": 0.20,
        "moderate_max": 0.45,
    },
    "input_length": {
        "enabled": True,
        "levels": [
            {"max_chars": 200, "score": 0.05},
            {"max_chars": 800, "score": 0.12},
            {"max_chars": 2000, "score": 0.20},
            {"max_chars": 0, "score": 0.30},
        ],
        "description": "Scores by total message length; longer input is treated as more complex",
    },
    "multi_turn": {
        "enabled": True,
        "levels": [
            {"max_msgs": 2, "score": 0.0},
            {"max_msgs": 5, "score": 0.08},
            {"max_msgs": 10, "score": 0.15},
            {"max_msgs": 0, "score": 0.20},
        ],
        "description": "Scores by conversation turns; more turns are treated as more complex",
    },
    "code_detection": {
        "enabled": True,
        "score": 0.15,
        "keywords": ["```", "def ", "function ", "class ", "import ", "return "],
        "description": "Detects code features such as code blocks and function definitions; matches add complexity score",
    },
    "tools_detection": {
        "enabled": True,
        "tools_score": 0.20,
        "functions_score": 0.15,
        "description": "Detects tools/functions fields; tool usage indicates a more complex task",
    },
    "reasoning_keywords": {
        "enabled": True,
        "score": 0.12,
        "keywords": [
            "explain", "analyze", "reason", "prove", "calculate",
            "derive", "compare", "evaluate", "critique",
            "why", "cause", "principle", "logic",
            "steps", "plan", "strategy", "design",
        ],
        "description": "Adds complexity score when the last user message contains reasoning or analysis keywords",
    },
    "system_prompt": {
        "enabled": True,
        "threshold_chars": 500,
        "score": 0.08,
        "description": "Adds complexity score when the system message exceeds the configured character threshold",
    },
}


@settings_bp.route('/seed-features', methods=['POST'])
def seed_features():
    """初始化 wr-proxy 优化特性开关 + 复杂度配置（创建 DB 记录）"""
    created = []
    for defn in FEATURE_TOGGLE_DEFS:
        existing = SystemSetting.query.filter_by(key=defn['key']).first()
        if not existing:
            s = SystemSetting(
                key=defn['key'],
                value=json.dumps(defn['value'], ensure_ascii=False),
                value_type='bool',
                description=defn['description'],
                category=defn['category'],
                editable=True,
            )
            db.session.add(s)
            created.append(defn['key'])

    # 同时初始化复杂度配置
    existing_cc = SystemSetting.query.filter_by(key='smart_complexity_config').first()
    if not existing_cc:
        s = SystemSetting(
            key='smart_complexity_config',
            value=json.dumps(DEFAULT_COMPLEXITY_CONFIG, ensure_ascii=False),
            value_type='json',
            description='Smart model selection complexity configuration: input length, multi-turn conversation, code detection, tool usage, reasoning keywords, and system prompt length.',
            category='advanced',
            editable=True,
        )
        db.session.add(s)
        created.append('smart_complexity_config')

    try:
        db.session.commit()
    except Exception as e:
        db.session.rollback()
        return jsonify({'error': get_message('db_commit_failed', request).format(e=str(e))}), 500
    return jsonify({'message': get_message('settings_initialized', request).format(len=len(created)), 'created': created})


@settings_bp.route('/restore', methods=['POST'])
def restore_backup():
    """恢复备份"""
    data = request.get_json()
    backup_path = data.get('backup_path', '')
    if not backup_path or not os.path.exists(backup_path):
        return jsonify({'error': get_message('backup_file_not_found', request)}), 404

    db_uri = current_app.config.get('SQLALCHEMY_DATABASE_URI', '')
    if db_uri.startswith('sqlite:///'):
        db_path = db_uri.replace('sqlite:///', '')
        import shutil
        shutil.copy2(backup_path, db_path)
        return jsonify({'message': get_message('restored', request)})

    return jsonify({'error': get_message('restore_sqlite_only', request)}), 400


@settings_bp.route('/test-email', methods=['POST'])
def test_email():
    """发送测试邮件，验证 SMTP 配置是否有效"""
    smtp_host = SystemSetting.get('alert_smtp_host', '')
    smtp_port = SystemSetting.get('alert_smtp_port', 587)
    smtp_user = SystemSetting.get('alert_smtp_user', '')
    smtp_password = SystemSetting.get('alert_smtp_password', '')
    smtp_from = SystemSetting.get('alert_smtp_from', '')
    email_to = SystemSetting.get('alert_email_to', '')

    if not smtp_host:
        return jsonify({'success': False, 'message': get_message('smtp_no_server', request)}), 400
    if not email_to:
        return jsonify({'success': False, 'message': get_message('smtp_no_recipient', request)}), 400
    if not smtp_user or not smtp_password:
        return jsonify({'success': False, 'message': get_message('smtp_no_credentials', request)}), 400

    from_addr = smtp_from or smtp_user
    recipients = [addr.strip() for addr in email_to.split(',') if addr.strip()]

    msg = MIMEText(
        'This is a test email from WebRouter confirming that your SMTP configuration is correct.',
        'plain',
        'utf-8',
    )
    msg['Subject'] = '[WebRouter] SMTP configuration test'
    msg['From'] = from_addr
    msg['To'] = ', '.join(recipients)

    try:
        if int(smtp_port) == 465:
            server = smtplib.SMTP_SSL(smtp_host, int(smtp_port), timeout=15)
        else:
            server = smtplib.SMTP(smtp_host, int(smtp_port), timeout=15)
            server.ehlo()
            server.starttls()
            server.ehlo()
        server.login(smtp_user, smtp_password)
        server.sendmail(from_addr, recipients, msg.as_string())
        server.quit()
        logger.info(f'测试邮件已发送: to={email_to}')
        return jsonify({'success': True, 'message': get_message('smtp_test_sent', request).format(email_to=email_to)})
    except smtplib.SMTPAuthenticationError as e:
        return jsonify({'success': False, 'message': get_message('smtp_auth_failed', request).format(e=str(e))}), 401
    except smtplib.SMTPConnectError as e:
        return jsonify({'success': False, 'message': get_message('smtp_connection_failed', request).format(e=str(e))}), 502
    except smtplib.SMTPSenderRefused as e:
        return jsonify({'success': False, 'message': get_message('smtp_sender_rejected', request).format(e=str(e))}), 403
    except smtplib.SMTPRecipientsRefused as e:
        return jsonify({'success': False, 'message': get_message('smtp_recipient_rejected', request).format(e=str(e))}), 400
    except OSError as e:
        # 连接意外关闭等网络错误
        err_msg = str(e)
        if 'closed' in err_msg.lower() or 'reset' in err_msg.lower():
            return jsonify({'success': False, 'message': get_message('smtp_connection_closed', request)}), 500
        return jsonify({'success': False, 'message': get_message('network_error', request).format(err_msg=err_msg)}), 500
    except smtplib.SMTPException as e:
        return jsonify({'success': False, 'message': get_message('smtp_send_failed', request).format(e=str(e))}), 500
    except Exception as e:
        return jsonify({'success': False, 'message': get_message('unknown_error', request).format(e=str(e))}), 500


@settings_bp.route('/reload-proxy', methods=['POST'])
def reload_proxy():
    """重新加载 wr-proxy（Provider、特性开关、定价等）"""
    import os
    proxy_url = SystemSetting.get('proxy_url')
    if not proxy_url:
        proxy_url = os.environ.get('WR_PROXY_URL', f"http://localhost:{current_app.config.get('PROXY_PORT', 5051)}")
    try:
        resp = _requests.post(f"{proxy_url}/admin/reload", timeout=10)
        data = resp.json() if resp.ok else {}
        if resp.ok:
            return jsonify({'success': True, 'message': get_message('proxy_reloaded', request), 'detail': data.get('message', '')})
        else:
            return jsonify({'success': False, 'message': get_message('proxy_reload_error', request).format(status=resp.status_code, error=data.get("error", ""))})
    except _requests.ConnectionError:
        return jsonify({'success': False, 'message': get_message('proxy_unreachable', request).format(proxy_url=proxy_url)}), 502
    except Exception as e:
        return jsonify({'success': False, 'message': get_message('proxy_reload_failed', request).format(e=str(e))}), 500


@settings_bp.route('/test-proxy', methods=['POST'])
def test_proxy():
    """通过 Flask 转发请求到 wr-proxy，测试 API 调用（避免浏览器直连 wr-proxy）"""
    data = request.get_json()
    if not data:
        return jsonify({'error': get_message('no_data', request)}), 400

    api_key = data.get('api_key', '')
    if not api_key:
        return jsonify({'error': get_message('api_key_required', request)}), 400

    import os
    proxy_url = SystemSetting.get('proxy_url')
    if not proxy_url:
        proxy_url = os.environ.get('WR_PROXY_URL', f"http://localhost:{current_app.config.get('PROXY_PORT', 5051)}")

    body = {
        'model': data.get('model', 'auto'),
        'messages': data.get('messages', [{'role': 'user', 'content': 'hi'}]),
        'stream': data.get('stream', False),
    }

    try:
        # Demo 模式：wr-proxy 没有真实上游 key，直接返回模拟响应
        if current_app.config.get('DEMO_MODE'):
            return jsonify({
                'id': 'demo-chatcmpl-mock',
                'object': 'chat.completion',
                'model': body.get('model', 'auto'),
                'choices': [{
                    'index': 0,
                    'message': {'role': 'assistant', 'content': get_message('demo_test_response', request)},
                    'finish_reason': 'stop',
                }],
                'usage': {'prompt_tokens': 5, 'completion_tokens': 8, 'total_tokens': 13},
            })

        resp = _requests.post(
            f"{proxy_url}/v1/chat/completions",
            headers={
                'Authorization': f'Bearer {api_key}',
                'Content-Type': 'application/json',
            },
            json=body,
            timeout=60,
        )
        # 始终解析响应体（包含成功数据和错误信息）
        try:
            result = resp.json()
        except ValueError:
            result = {'raw_response': resp.text}
        # 如果 wr-proxy 返回了结构化错误，提取 message
        if not resp.ok and isinstance(result, dict):
            err = result.get('error', {})
            if isinstance(err, dict):
                err_msg = err.get('message', resp.text)
            elif isinstance(err, str):
                err_msg = err
            else:
                err_msg = result.get('message', get_message('request_failed', request))

            # 503 "No available provider" → 自动 reload wr-proxy 后重试一次
            if resp.status_code == 503 and 'no available provider' in err_msg.lower():
                try:
                    import time
                    _requests.post(f"{proxy_url}/admin/reload", timeout=10)
                    time.sleep(1)
                    resp2 = _requests.post(
                        f"{proxy_url}/v1/chat/completions",
                        headers={
                            'Authorization': f'Bearer {api_key}',
                            'Content-Type': 'application/json',
                        },
                        json=body,
                        timeout=60,
                    )
                    try:
                        result2 = resp2.json()
                    except ValueError:
                        result2 = {'raw_response': resp2.text}
                    if resp2.ok:
                        return jsonify(result2), resp2.status_code
                    # 重试仍失败，返回友好错误
                    err2 = result2.get('error', {})
                    if isinstance(err2, dict):
                        err_msg2 = err2.get('message', resp2.text)
                    elif isinstance(err2, str):
                        err_msg2 = err2
                    else:
                        err_msg2 = str(result2)
                    return jsonify({'error': get_message('no_available_provider', request).format(model=body.get('model', '')), 'detail': err_msg2}), resp2.status_code
                except Exception:
                    pass
                return jsonify({'error': get_message('no_available_provider', request).format(model=body.get('model', '')), 'detail': err_msg}), resp.status_code

            if isinstance(err, dict):
                return jsonify({'error': err.get('message', resp.text)}), resp.status_code
            elif isinstance(err, str):
                return jsonify({'error': err}), resp.status_code
            return jsonify({'error': err_msg}), resp.status_code
        return jsonify(result), resp.status_code
    except _requests.ConnectionError:
        return jsonify({'error': get_message('proxy_unreachable', request).format(proxy_url=proxy_url)}), 502
    except Exception as e:
        return jsonify({'error': get_message('request_failed_detail', request).format(e=str(e))}), 500
