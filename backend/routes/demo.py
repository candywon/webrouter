# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""Demo 模式 — 管理接口（重置数据）"""

from flask import Blueprint, jsonify, current_app
from extensions import db
from flask_login import login_user, current_user

demo_bp = Blueprint('demo', __name__)


@demo_bp.route('/reset', methods=['POST'])
def reset_demo_data():
    """清空所有 Demo 数据并重新播种"""
    from seed_demo import seed_demo_data
    seed_demo_data(current_app._get_current_object(), reset=True)
    return jsonify({'message': 'Demo data reset successfully'})


@demo_bp.route('/auto-login')
def auto_login():
    """Demo 模式自动登录"""
    from models.wr_models import AdminUser
    admin = AdminUser.query.filter_by(username='demo').first()
    if admin and not current_user.is_authenticated:
        login_user(admin)
    return jsonify({'message': 'ok'})


@demo_bp.route('/status')
def demo_status():
    return jsonify({'demo_mode': True})