# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""会话记忆管理 API — 查看和删除 Token 的会话历史"""
from flask import Blueprint, jsonify, request
from extensions import db
from sqlalchemy import text
from i18n.messages import get_message, get_lang

session_bp = Blueprint('session', __name__)


@session_bp.route('/list')
def list_sessions():
    """列出指定 Token 的所有会话（按最后活跃时间倒序）"""
    lang = get_lang(request)
    token_id = request.args.get('token_id', type=int)
    if not token_id:
        return jsonify({'error': get_message('field_required_token_id', lang)}), 400

    limit = request.args.get('limit', 50, type=int)
    offset = request.args.get('offset', 0, type=int)
    rows = db.session.execute(text("""
        SELECT session_id, MAX(created_at) AS last_at,
               COUNT(*) AS turn_count, MAX(model) AS model
        FROM wr_session_messages
        WHERE token_id = :tid
        GROUP BY session_id
        ORDER BY last_at DESC
        LIMIT :lim OFFSET :off
    """), {'tid': token_id, 'lim': limit, 'off': offset}).fetchall()

    return jsonify({
        'sessions': [{
            'session_id': r[0],
            'last_at': r[1],
            'turn_count': r[2],
            'model': r[3],
        } for r in rows],
    })


@session_bp.route('/<session_id>/messages')
def get_messages(session_id):
    lang = get_lang(request)
    """获取指定会话的消息列表"""
    token_id = request.args.get('token_id', type=int)
    if not token_id:
        return jsonify({'error': get_message('field_required_token_id', lang)}), 400

    rows = db.session.execute(text("""
        SELECT turn_index, role, content, model, created_at
        FROM wr_session_messages
        WHERE session_id = :sid AND token_id = :tid
        ORDER BY turn_index ASC
    """), {'sid': session_id, 'tid': token_id}).fetchall()

    return jsonify({
        'session_id': session_id,
        'messages': [{
            'turn_index': r[0],
            'role': r[1],
            'content': r[2],
            'model': r[3],
            'created_at': r[4],
        } for r in rows],
    })


@session_bp.route('/<session_id>', methods=['DELETE'])
def delete_session(session_id):
    lang = get_lang(request)
    """删除指定会话的所有消息"""
    token_id = request.args.get('token_id', type=int)
    if not token_id:
        return jsonify({'error': get_message('field_required_token_id', lang)}), 400

    result = db.session.execute(text("""
        DELETE FROM wr_session_messages
        WHERE session_id = :sid AND token_id = :tid
    """), {'sid': session_id, 'tid': token_id})
    db.session.commit()

    return jsonify({
        'message': get_message('session_deleted', lang),
        'deleted_count': result.rowcount,
    })
