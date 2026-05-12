/* 告警页面逻辑 */
const AlertPage = {
  async load() {
    try {
      const data = await API.get('/alerts/rules');
      this.renderRules(data.rules || []);
    } catch (e) {
      console.error('Failed to load alerts:', e);
    }
  },

  renderRules(rules) {
    const el = document.getElementById('alert-rules');
    if (!el) return;
    if (rules.length === 0) {
      el.innerHTML = '<div class="empty-state"><div class="icon">🔔</div><p>暂无告警规则<br>点击"添加规则"创建</p></div>';
      return;
    }
    el.innerHTML = `<table>
      <thead><tr><th>名称</th><th>条件</th><th>级别</th><th>通道</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>${rules.map(r => `
        <tr>
          <td>${r.name}</td>
          <td>${r.condition_type}</td>
          <td><span class="badge badge-${r.level === 'critical' ? 'dead' : r.level === 'warning' ? 'warning' : 'info'}">${r.level}</span></td>
          <td>${(r.channels || []).join(', ')}</td>
          <td>${r.enabled ? '<span class="badge badge-healthy">启用</span>' : '<span class="badge badge-unknown">禁用</span>'}</td>
          <td>
            <button class="btn" onclick="AlertPage.toggleRule(${r.id}, ${!r.enabled})">${r.enabled ? '禁用' : '启用'}</button>
            <button class="btn" onclick="AlertPage.deleteRule(${r.id})">删除</button>
          </td>
        </tr>
      `).join('')}</tbody>
    </table>`;
  },

  async toggleRule(id, enabled) {
    try {
      await API.put(`/alerts/rules/${id}`, { enabled });
      this.load();
    } catch (e) { showToast('操作失败'); }
  },

  async deleteRule(id) {
    if (!confirm('确定删除此规则？')) return;
    try {
      await API.del(`/alerts/rules/${id}`);
      this.load();
    } catch (e) { showToast('删除失败'); }
  },

  async createRule() {
    const name = prompt('规则名称:');
    if (!name) return;
    const type = prompt('条件类型 (key_failed/balance_low/error_rate/usage_spike):', 'key_failed');
    const level = prompt('级别 (critical/warning/info):', 'warning');
    const channels = prompt('告警通道 (逗号分隔, wechat/email):', 'wechat');

    try {
      await API.post('/alerts/rules', {
        name,
        condition_type: type,
        condition_config: {},
        level,
        channels: channels.split(',').map(s => s.trim()),
      });
      this.load();
    } catch (e) { showToast('创建失败'); }
  },
};
