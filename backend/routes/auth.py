# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""后台登录认证 API"""
from datetime import datetime
from flask import Blueprint, jsonify, request
from flask_login import login_user, logout_user, current_user, login_required
from models.wr_models import AdminUser
from extensions import db
from i18n.messages import get_message, get_lang


auth_bp = Blueprint('auth', __name__)


@auth_bp.route('/status')
def status():
    return jsonify({
        'authenticated': current_user.is_authenticated,
        'user': current_user.username if current_user.is_authenticated else None,
        'demo_mode': current_app.config.get('DEMO_MODE', False),
    })


@auth_bp.route('/login', methods=['POST'])
def login():
    lang = get_lang(request)
    data = request.get_json() or {}
    username = (data.get('username') or '').strip()
    password = data.get('password') or ''

    if not username or not password:
        return jsonify({'error': get_message('username_password_required', lang)}), 400

    user = AdminUser.query.filter_by(username=username, enabled=True).first()
    if not user or not user.check_password(password):
        return jsonify({'error': get_message('invalid_credentials', lang)}), 401

    user.last_login_at = datetime.utcnow()
    db.session.commit()
    login_user(user, remember=bool(data.get('remember')))

    return jsonify({
        'message': get_message('login_success', lang),
        'user': user.username,
    })


@auth_bp.route('/logout', methods=['POST'])
@login_required
def logout():
    lang = get_lang(request)
    logout_user()
    return jsonify({'message': get_message('logout_success', lang)})


@auth_bp.route('/change-password', methods=['POST'])
@login_required
def change_password():
    lang = get_lang(request)
    data = request.get_json() or {}
    old_password = data.get('old_password') or ''
    new_password = data.get('new_password') or ''

    if not current_user.check_password(old_password):
        return jsonify({'error': get_message('wrong_password', lang)}), 400
    if len(new_password) < 8:
        return jsonify({'error': get_message('new_password_min_length', lang)}), 400

    current_user.set_password(new_password)
    db.session.commit()
    return jsonify({'message': get_message('password_changed', lang)})
