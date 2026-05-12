"""系统设置API"""
import os
import json
from flask import Blueprint, jsonify, request
from extensions import db

settings_bp = Blueprint('settings', __name__)


@settings_bp.route('/')
def get_settings():
    """获取系统设置"""
    return jsonify({
        'newapi_url': os.environ.get('NEWAPI_URL', 'http://localhost:3000'),
        'health_check_interval': os.environ.get('HEALTH_CHECK_INTERVAL', '300'),
        'alert_cooldown': os.environ.get('ALERT_COOLDOWN', '300'),
        'timezone': os.environ.get('TZ', 'Asia/Shanghai'),
    })


@settings_bp.route('/', methods=['PUT'])
def update_settings():
    """更新系统设置（写入.env文件）"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400

    env_path = os.path.join(os.path.dirname(__file__), '..', '.env')
    existing = {}
    if os.path.exists(env_path):
        with open(env_path) as f:
            for line in f:
                line = line.strip()
                if line and '=' in line and not line.startswith('#'):
                    k, v = line.split('=', 1)
                    existing[k.strip()] = v.strip()

    # 更新允许的键
    allowed_keys = {
        'NEWAPI_URL', 'HEALTH_CHECK_INTERVAL',
        'ALERT_COOLDOWN', 'TZ',
    }
    for k, v in data.items():
        if k in allowed_keys:
            existing[k] = str(v)

    with open(env_path, 'w') as f:
        for k, v in existing.items():
            f.write(f"{k}={v}\n")

    return jsonify({'updated': True, 'settings': existing})


@settings_bp.route('/backup', methods=['POST'])
def create_backup():
    """创建数据库备份"""
    import shutil
    from datetime import datetime

    db_uri = os.environ.get('DATABASE_URI', 'sqlite:///data/webrouter.db')
    if not db_uri.startswith('sqlite:///'):
        return jsonify({'error': 'Only SQLite backup supported in MVP'}), 400

    db_path = db_uri.replace('sqlite:///', '')
    if not os.path.exists(db_path):
        return jsonify({'error': 'Database file not found'}), 404

    backup_dir = os.path.join(os.path.dirname(db_path), 'backups')
    os.makedirs(backup_dir, exist_ok=True)

    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    backup_path = os.path.join(backup_dir, f'webrouter_{timestamp}.db')
    shutil.copy2(db_path, backup_path)

    return jsonify({
        'backup': backup_path,
        'size_bytes': os.path.getsize(backup_path),
    })


@settings_bp.route('/restore', methods=['POST'])
def restore_backup():
    """从备份恢复"""
    data = request.get_json()
    backup_path = data.get('backup_path', '')

    if not os.path.exists(backup_path):
        return jsonify({'error': 'Backup file not found'}), 404

    db_uri = os.environ.get('DATABASE_URI', 'sqlite:///data/webrouter.db')
    db_path = db_uri.replace('sqlite:///', '')

    import shutil
    shutil.copy2(backup_path, db_path)

    return jsonify({'restored': True, 'from': backup_path})
