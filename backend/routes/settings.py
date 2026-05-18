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

logger = logging.getLogger(__name__)

settings_bp = Blueprint('settings', __name__)


@settings_bp.route('/', strict_slashes=False)
def get_settings():
    """获取系统设置（优先从 DB 读取，fallback 到 config）"""
    app = current_app._get_current_object()

    settings_map = {
        'proxy_url': lambda: f"http://localhost:{app.config.get('PROXY_PORT', 5051)}",
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
        return jsonify({'error': 'No data provided'}), 400

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
                    errors.append(f'{key}: 该设置不可编辑')
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
        return jsonify({'error': f'数据库提交失败: {str(e)}'}), 500

    result = {'message': f'已更新 {len(updated)} 项设置', 'updated': updated}
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
        return jsonify({'error': 'No data provided'}), 400

    value = data.get('value')
    if value is None:
        return jsonify({'error': 'value field required'}), 400

    s = SystemSetting.query.filter_by(key=key).first()
    if not s:
        return jsonify({'error': f'Setting {key} not found'}), 404

    if not s.editable:
        return jsonify({'error': f'Setting {key} is not editable'}), 403

    s.value = json.dumps(value, ensure_ascii=False)
    s.updated_at = db.func.now()
    db.session.commit()

    return jsonify({'message': '设置已更新', 'setting': s.to_dict()})


@settings_bp.route('/<string:key>', methods=['DELETE'], strict_slashes=False)
def delete_setting(key):
    """删除自定义设置项（种子设置不可删）"""
    seed_keys = ['proxy_enabled', 'proxy_url', 'health_check_interval', 'alert_cooldown', 'timezone',
                 'routing_strategy', 'default_timeout', 'max_retry_count', 'max_failover',
                 'quota_warn_threshold', 'quota_critical_threshold', 'prediction_days',
                 'idle_conn_timeout', 'max_idle_conns',
                 'feature_dynamic_content_last', 'feature_token_compression', 'feature_session_compression',
                 'smart_complexity_config']

    if key in seed_keys:
        return jsonify({'error': f'Cannot delete seed setting {key}'}), 403

    s = SystemSetting.query.filter_by(key=key).first()
    if not s:
        return jsonify({'error': f'Setting {key} not found'}), 404

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

    return jsonify({'message': '备份功能仅支持SQLite'})


FEATURE_TOGGLE_DEFS = [
    {
        'key': 'feature_dynamic_content_last',
        'value': False,
        'description': (
            '动态内容后置：将 user 消息中的动态部分（URL、时间、随机数等）移到末尾，'
            '使 prompt 前缀尽可能静态，提升上游 prompt cache 命中率。'
            '开启后 wr-proxy 会对请求 body 中的 messages 数组重新排序，'
            '把包含 URL、日期、数字等动态内容的 message 移到同 role 组的最后。'
        ),
        'category': 'advanced',
    },
    {
        'key': 'feature_token_compression',
        'value': False,
        'description': (
            'Token 压缩（RTK - Return Token Key）：对系统提示词和长上下文进行压缩预处理。'
            '在请求发送到上游之前，先通过一次轻量模型（如 qwen-turbo）对长文本做摘要，'
            '减少输入 token 数量。适用于 system prompt 很长的场景（>4000 tokens），'
            '可显著降低调用成本，但会损失少量上下文精度。'
        ),
        'category': 'advanced',
    },
    {
        'key': 'feature_session_compression',
        'value': False,
        'description': (
            '会话压缩：对多轮对话的历史消息进行压缩。'
            '当对话轮数超过阈值时，将早期消息合并为摘要，减少后续请求的上下文长度。'
            '适用于长对话场景（如客服、助教），可将数十轮对话压缩为几轮摘要。'
            '注意：压缩会丢失部分细节，适合对上下文精度要求不高的场景。'
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
        "description": "按消息总字符数评分，输入越长越复杂",
    },
    "multi_turn": {
        "enabled": True,
        "levels": [
            {"max_msgs": 2, "score": 0.0},
            {"max_msgs": 5, "score": 0.08},
            {"max_msgs": 10, "score": 0.15},
            {"max_msgs": 0, "score": 0.20},
        ],
        "description": "按对话轮数评分，轮数越多越复杂",
    },
    "code_detection": {
        "enabled": True,
        "score": 0.15,
        "keywords": ["```", "def ", "function ", "class ", "import ", "return "],
        "description": "检测代码特征（代码块、函数定义等），命中即加分",
    },
    "tools_detection": {
        "enabled": True,
        "tools_score": 0.20,
        "functions_score": 0.15,
        "description": "检测 tools/functions 字段，使用工具调用意味着复杂任务",
    },
    "reasoning_keywords": {
        "enabled": True,
        "score": 0.12,
        "keywords": [
            "分析", "推理", "证明", "计算", "推导",
            "explain", "analyze", "reason", "prove", "calculate",
            "derive", "compare", "evaluate", "critique",
            "为什么", "原因", "原理", "逻辑",
            "步骤", "方案", "策略", "设计",
        ],
        "description": "最后一条用户消息包含推理/分析关键词即加分",
    },
    "system_prompt": {
        "enabled": True,
        "threshold_chars": 500,
        "score": 0.08,
        "description": "system 消息超过指定字符数即加分",
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
            description='智能模型选择六维度复杂度配置：输入长度、多轮对话、代码检测、工具调用、推理关键词、系统提示词',
            category='advanced',
            editable=True,
        )
        db.session.add(s)
        created.append('smart_complexity_config')

    try:
        db.session.commit()
    except Exception as e:
        db.session.rollback()
        return jsonify({'error': f'数据库提交失败: {str(e)}'}), 500
    return jsonify({'message': f'已初始化 {len(created)} 项', 'created': created})


@settings_bp.route('/restore', methods=['POST'])
def restore_backup():
    """恢复备份"""
    data = request.get_json()
    backup_path = data.get('backup_path', '')
    if not backup_path or not os.path.exists(backup_path):
        return jsonify({'error': '备份文件不存在'}), 404

    db_uri = current_app.config.get('SQLALCHEMY_DATABASE_URI', '')
    if db_uri.startswith('sqlite:///'):
        db_path = db_uri.replace('sqlite:///', '')
        import shutil
        shutil.copy2(backup_path, db_path)
        return jsonify({'message': '已恢复'})

    return jsonify({'error': '仅支持SQLite恢复'}), 400


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
        return jsonify({'success': False, 'message': '未配置 SMTP 服务器地址'}), 400
    if not email_to:
        return jsonify({'success': False, 'message': '未配置收件人地址'}), 400
    if not smtp_user or not smtp_password:
        return jsonify({'success': False, 'message': '未配置 SMTP 用户名或密码'}), 400

    from_addr = smtp_from or smtp_user
    recipients = [addr.strip() for addr in email_to.split(',') if addr.strip()]

    msg = MIMEText(
        '这是一封来自 WebRouter 的测试邮件，说明您的 SMTP 配置正确。',
        'plain',
        'utf-8',
    )
    msg['Subject'] = '[WebRouter] SMTP 配置测试成功'
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
        return jsonify({'success': True, 'message': f'测试邮件已发送至 {email_to}'})
    except smtplib.SMTPAuthenticationError as e:
        return jsonify({'success': False, 'message': f'SMTP 认证失败: {str(e)}\n提示：QQ 邮箱请使用授权码，而非登录密码。'}), 401
    except smtplib.SMTPConnectError as e:
        return jsonify({'success': False, 'message': f'SMTP 连接失败: {str(e)}'}), 502
    except smtplib.SMTPSenderRefused as e:
        return jsonify({'success': False, 'message': f'发件人地址被拒绝: {str(e)}'}), 403
    except smtplib.SMTPRecipientsRefused as e:
        return jsonify({'success': False, 'message': f'收件人地址被拒绝: {str(e)}'}), 400
    except OSError as e:
        # 连接意外关闭等网络错误
        err_msg = str(e)
        if 'closed' in err_msg.lower() or 'reset' in err_msg.lower():
            return jsonify({'success': False, 'message': f'SMTP 连接异常关闭，可能是认证失败或被服务器拒绝。\n提示：QQ 邮箱请使用授权码（在 设置 → 账户 → POP3/IMAP/SMTP 中获取），而非登录密码。'}), 500
        return jsonify({'success': False, 'message': f'网络连接失败: {err_msg}'}), 500
    except smtplib.SMTPException as e:
        return jsonify({'success': False, 'message': f'邮件发送失败: {str(e)}'}), 500
    except Exception as e:
        return jsonify({'success': False, 'message': f'未知错误: {str(e)}'}), 500


@settings_bp.route('/reload-proxy', methods=['POST'])
def reload_proxy():
    """重新加载 wr-proxy（Provider、特性开关、定价等）"""
    proxy_url = SystemSetting.get('proxy_url')
    if not proxy_url:
        proxy_url = f"http://localhost:{current_app.config.get('PROXY_PORT', 5051)}"
    try:
        resp = _requests.post(f"{proxy_url}/admin/reload", timeout=10)
        data = resp.json() if resp.ok else {}
        if resp.ok:
            return jsonify({'success': True, 'message': 'wr-proxy 已重新加载', 'detail': data.get('message', '')})
        else:
            return jsonify({'success': False, 'message': f'wr-proxy 返回 {resp.status_code}: {data.get("error", "")}'})
    except _requests.ConnectionError:
        return jsonify({'success': False, 'message': f'无法连接 wr-proxy（{proxy_url}），请确认 wr-proxy 正在运行'}), 502
    except Exception as e:
        return jsonify({'success': False, 'message': f'重载失败: {str(e)}'}), 500


@settings_bp.route('/test-proxy', methods=['POST'])
def test_proxy():
    """通过 Flask 转发请求到 wr-proxy，测试 API 调用（避免浏览器直连 wr-proxy）"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data provided'}), 400

    api_key = data.get('api_key', '')
    if not api_key:
        return jsonify({'error': 'api_key required'}), 400

    proxy_url = SystemSetting.get('proxy_url')
    if not proxy_url:
        proxy_url = f"http://localhost:{current_app.config.get('PROXY_PORT', 5051)}"

    body = {
        'model': data.get('model', 'auto'),
        'messages': data.get('messages', [{'role': 'user', 'content': 'hi'}]),
        'stream': data.get('stream', False),
    }

    try:
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
                return jsonify({'error': err.get('message', resp.text)}), resp.status_code
            elif isinstance(err, str):
                return jsonify({'error': err}), resp.status_code
            return jsonify({'error': result.get('message', '请求失败')}), resp.status_code
        return jsonify(result), resp.status_code
    except _requests.ConnectionError:
        return jsonify({'error': f'无法连接 wr-proxy（{proxy_url}），请确认正在运行'}), 502
    except Exception as e:
        return jsonify({'error': f'请求失败: {str(e)}'}), 500
