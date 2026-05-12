/* CLI对接页面逻辑 */
const CLIPage = {
  tools: [],

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
        <button class="btn btn-primary" onclick="CLIPage.exportConfig('${t.id}')">导出配置</button>
      </div>
    `).join('');
  },

  async exportConfig(toolId) {
    const apiKey = prompt('请输入你的 API Key:', 'sk-');
    if (!apiKey) return;

    try {
      const data = await API.get(`/cli/export/${toolId}?api_key=${encodeURIComponent(apiKey)}`);
      const el = document.getElementById('cli-output');
      if (!el) return;

      let content = '';
      if (data.shell) content = data.shell;
      else if (data.yaml) content = data.yaml;
      else if (data.json) content = JSON.stringify(JSON.parse(data.json), null, 2);
      else if (data.instructions) content = data.instructions;
      else content = JSON.stringify(data, null, 2);

      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${data.name} 配置</span>
            <button class="btn" onclick="copyToClipboard(\`${content.replace(/`/g, '\\`')}\`)">复制</button>
          </div>
          <div class="code-block">${content}</div>
        </div>
      `;
    } catch (e) {
      showToast('导出失败: ' + e.message);
    }
  },
};
