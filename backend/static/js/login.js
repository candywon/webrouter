// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 后台登录页面 */
const LoginPage = {
  async load() {
    try {
      const res = await fetch('/api/auth/status');
      const data = await res.json();
      if (data.authenticated) {
        Router.navigate('/');
        return;
      }
    } catch (_) { /* ignore */ }
    this.render();
  },

  render() {
    const el = document.getElementById('login-page-content');
    if (!el) return;
    el.innerHTML = `
      <div style="min-height:70vh;display:flex;align-items:center;justify-content:center;">
        <div class="card" style="width:360px;padding:28px;">
          <h2 style="margin:0 0 8px;text-align:center;">${I18n.t('login.title')}</h2>
          <p style="margin:0 0 24px;text-align:center;color:var(--text-muted);font-size:13px;">${I18n.t('login.hint')}</p>
          <form id="login-form">
            <div class="form-group">
              <label>${I18n.t('login.username')}</label>
              <input type="text" id="login-username" autocomplete="username" value="admin" required>
            </div>
            <div class="form-group">
              <label>${I18n.t('login.password')}</label>
              <input type="password" id="login-password" autocomplete="current-password" required>
            </div>
            <div class="form-group form-row">
              <label class="switch-label">
                <input type="checkbox" id="login-remember">
                <span>${I18n.t('login.remember')}</span>
              </label>
            </div>
            <button type="submit" class="btn-primary" style="width:100%;padding:10px;">${I18n.t('login.submit')}</button>
          </form>
          <div id="login-error" style="display:none;margin-top:16px;color:var(--danger);font-size:13px;text-align:center;"></div>
        </div>
      </div>
    `;
    document.getElementById('login-form').onsubmit = (e) => {
      e.preventDefault();
      this.submit();
    };
  },

  async submit() {
    const errorEl = document.getElementById('login-error');
    errorEl.style.display = 'none';
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: document.getElementById('login-username').value.trim(),
          password: document.getElementById('login-password').value,
          remember: document.getElementById('login-remember').checked,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || I18n.t("login.failed"));
      Router.navigate('/');
    } catch (e) {
      errorEl.textContent = e.message || I18n.t("login.failed");
      errorEl.style.display = 'block';
    }
  },
};

async function logoutAdmin() {
  try {
    await fetch('/api/auth/logout', { method: 'POST' });
  } catch (_) { /* ignore */ }
  Router.navigate('/login');
}
