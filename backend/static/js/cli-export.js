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
          <input type="text" id="apikey-${t.id}" class="input-field" placeholder="sk-输入你的API Key" style="flex:1;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:6px;color:var(--text-primary);font-size:13px">
          <button class="btn btn-primary" onclick="CLIPage.exportConfig('${t.id}')">导出</button>
        </div>
      </div>
    `).join('');
    // 输出区
    const outEl = document.getElementById('cli-output');
    if (outEl && !outEl.innerHTML.trim()) {
      outEl.innerHTML = '<div class="empty-state"><div class="icon">📋</div><p>选择工具并输入API Key后点击导出</p></div>';
    }
  },

  async exportConfig(toolId) {
    const inputEl = document.getElementById(`apikey-${toolId}`);
    const apiKey = inputEl ? inputEl.value.trim() : '';
    if (!apiKey) {
      showToast('请先输入 API Key');
      if (inputEl) inputEl.focus();
      return;
    }

    try {
      const data = await API.get(`/cli/export/${toolId}?api_key=${encodeURIComponent(apiKey)}`);
      const el = document.getElementById('cli-output');
      if (!el) return;

      let content = '';
      let label = '';
      if (data.shell) { content = data.shell; label = 'Shell 环境变量'; }
      else if (data.yaml) { content = data.yaml; label = 'YAML 配置'; }
      else if (data.json) { content = JSON.stringify(JSON.parse(data.json), null, 2); label = 'JSON 配置'; }
      else if (data.instructions) { content = data.instructions; label = '使用说明'; }
      else { content = JSON.stringify(data, null, 2); label = '配置'; }

      // 转义HTML
      const escaped = content.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
      // 存储原始内容用于复制
      const contentId = `cli-content-${toolId}`;

      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${data.name} — ${label}</span>
            <button class="btn" onclick="CLIPage.copyContent('${contentId}')">📋 复制</button>
          </div>
          <pre id="${contentId}" class="code-block" style="white-space:pre-wrap;word-break:break-all">${escaped}</pre>
        </div>
      `;
    } catch (e) {
      showToast('导出失败: ' + e.message);
    }
  },

  copyContent(contentId) {
    const el = document.getElementById(contentId);
    if (!el) return;
    const text = el.textContent || el.innerText;
    navigator.clipboard.writeText(text).then(() => {
      showToast('已复制到剪贴板');
    }).catch(() => {
      // fallback
      const ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      showToast('已复制到剪贴板');
    });
  },
};
