// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* CLI对接页面逻辑 */
const CLIPage = {
  tools: [],
  selectedTool: null,

  async load() {
    try {
      const data = await API.get('/cli/tools');
      this.tools = data.tools || [];
      this.renderTools();
    } catch (e) {
      console.error('Failed to load CLI tools:', e);
    }
  },

  renderTools() {
    const el = document.getElementById('cli-tools');
    if (!el) return;
    el.innerHTML = this.tools.map(t => `
      <div class="card">
        <div class="card-header">
          <span class="card-title">${t.name}</span>
          <span class="badge badge-info">${t.id}</span>
        </div>
        <p style="color:var(--text-secondary);margin-bottom:12px;font-size:13px">${t.description}</p>
        <div style="display:flex;gap:8px;align-items:center">
          <input type="text" id="apikey-${t.id}" class="input-field" placeholder="${I18n.t('cli.enterApiKey')}" style="flex:1;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:6px;color:var(--text-primary);font-size:13px">
          <select id="model-${t.id}" style="width:90px;padding:6px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:6px;color:var(--text-primary);font-size:13px;">
            <option value="auto">auto</option>
            <option value="smart">smart</option>
            <option value="gpt-4o">gpt-4o</option>
            <option value="gpt-4o-mini">gpt-4o-mini</option>
            <option value="claude-sonnet-4">claude-sonnet-4</option>
            <option value="claude-opus-4">claude-opus-4</option>
          </select>
          <button class="btn btn-primary" onclick="CLIPage.exportConfig('${t.id}')">${I18n.t('cli.export')}</button>
        </div>
      </div>
    `).join('');
    // 输出区
    const outEl = document.getElementById('cli-output');
    if (outEl && !outEl.innerHTML.trim()) {
      outEl.innerHTML = `<div class="empty-state"><div class="icon">📋</div><p>${I18n.t('cli.selectToolHint')}</p></div>`;
    }
  },

  async exportConfig(toolId) {
    const inputEl = document.getElementById(`apikey-${toolId}`);
    const modelEl = document.getElementById(`model-${toolId}`);
    const apiKey = inputEl ? inputEl.value.trim() : '';
    const model = modelEl ? modelEl.value : 'auto';
    if (!apiKey) {
      showToast(I18n.t("cli.enterApiKey"));
      if (inputEl) inputEl.focus();
      return;
    }

    try {
      const data = await API.get(`/cli/export/${toolId}?api_key=${encodeURIComponent(apiKey)}&model=${encodeURIComponent(model)}`);
      const el = document.getElementById('cli-output');
      if (!el) return;

      let content = '';
      let label = '';
      if (data.shell) { content = data.shell; label = I18n.t('cli.shellEnv'); }
      else if (data.yaml) { content = data.yaml; label = I18n.t("cli.yamlConfig"); }
      else if (data.json) { content = JSON.stringify(JSON.parse(data.json), null, 2); label = I18n.t("cli.jsonConfig"); }
      else if (data.instructions) { content = data.instructions; label = I18n.t("cli.instructions"); }
      else { content = JSON.stringify(data, null, 2); label = I18n.t("cli.config"); }

      // 转义HTML
      const escaped = content.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
      // 存储原始内容用于复制
      const contentId = `cli-content-${toolId}`;

      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${data.name} — ${label}</span>
            <button class="btn" onclick="CLIPage.copyContent('${contentId}')">${I18n.t('common.copy')}</button>
          </div>
          <pre id="${contentId}" class="code-block" style="white-space:pre-wrap;word-break:break-all">${escaped}</pre>
        </div>
      `;
    } catch (e) {
      showToast(I18n.t("cli.exportFailed") + e.message);
    }
  },

  copyContent(contentId) {
    const el = document.getElementById(contentId);
    if (!el) return;
    const text = el.textContent || el.innerText;
    navigator.clipboard.writeText(text).then(() => {
      showToast(I18n.t("common.copiedToClipboard"));
    }).catch(() => {
      // fallback
      const ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      showToast(I18n.t("common.copiedToClipboard"));
    });
  },
};
