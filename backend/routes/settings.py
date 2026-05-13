"""系统设置API"""
import os
from flask import Blueprint, jsonify, request
from flask import current_app

settings_bp = Blueprint('settings', __name__)


@settings_bp.route('/', strict_slashes=False)
def get_settings():
    """获取系统设置"""
    app = current_app._get_current_object()
    return jsonify({
        'proxy_url': f"http://localhost:{app.config.get('PROXY_PORT', 5051)}",
        'health_check_interval': app.config.get('HEALTH_CHECK_INTERVAL', 300),
        'alert_cooldown': app.config.get('ALERT_COOLDOWN', 300),
        'timezone': app.config.get('TZ', 'Asia/Shanghai'),
        'proxy_enabled': True,
    })


@settings_bp.route('/', methods=['PUT'], strict_slashes=False)
def update_settings():
    """更新系统设置"""
    data = request.get_json()
    if not data:
        return jsonify({'error': 'No data'}), 400
    # TODO: 持久化到数据库或配置文件
    return jsonify({'message': '设置已更新'})


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
