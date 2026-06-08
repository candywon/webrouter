// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* API请求封装 */
const API = {
  base: '/api',

  _handleUnauthorized(resp) {
    if (resp.status === 401 && window.location.hash !== '#/login') {
      window.location.hash = '#/login';
      return true;
    }
    return false;
  },

  async get(path) {
    const resp = await fetch(this.base + path, { credentials: 'same-origin' });
    if (this._handleUnauthorized(resp)) throw new Error('unauthorized');
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },

  async post(path, data) {
    const resp = await fetch(this.base + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(data),
    });
    if (this._handleUnauthorized(resp)) throw new Error('unauthorized');
    if (!resp.ok) {
      let msg = `API Error: ${resp.status}`;
      try {
        const body = await resp.json();
        msg = body.error || body.message || msg;
      } catch (_) { /* ignore */ }
      throw new Error(msg);
    }
    return resp.json();
  },

  async put(path, data) {
    const resp = await fetch(this.base + path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(data),
    });
    if (this._handleUnauthorized(resp)) throw new Error('unauthorized');
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },

  async del(path) {
    const resp = await fetch(this.base + path, { method: 'DELETE', credentials: 'same-origin' });
    if (this._handleUnauthorized(resp)) throw new Error('unauthorized');
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },
};
