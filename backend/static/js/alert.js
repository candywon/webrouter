/* 告警页面逻辑 */
const AlertPage = {
  editingRule: null,

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

    const conditionLabels = {
      'key_failed': 'Key 失效',
      'balance_low': '余额不足',
      'error_rate': '错误率过高',
      'usage_spike': '用量突增',
    };

    const levelBadge = (level) => {
      const cls = level === 'critical' ? 'dead' : level === 'warning' ? 'warning' : 'info';
      const labels = { critical: '严重', warning: '警告', info: '提示' };
      return `<span class="badge badge-${cls}">${labels[level] || level}</span>`;
    };

    el.innerHTML = `<table>
      <thead><tr><th>名称</th><th>触发条件</th><th>级别</th><th>告警通道</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>${rules.map(r => `
        <tr>
          <td>${r.name}</td>
          <td>${conditionLabels[r.condition_type] || r.condition_type}</td>
          <td>${levelBadge(r.level)}</td>
          <td>${(r.channels || []).join(', ')}</td>
          <td>${r.enabled ? '<span class="badge badge-healthy">启用</span>' : '<span class="badge badge-unknown">禁用</span>'}</td>
          <td>
            <button class="btn" onclick="AlertPage.editRule(${r.id})">✏️ 编辑</button>
            <button class="btn" onclick="AlertPage.toggleRule(${r.id}, ${!r.enabled})">${r.enabled ? '禁用' : '启用'}</button>
            <button class="btn btn-danger" onclick="AlertPage.deleteRule(${r.id})">删除</button>
          </td>
        </tr>
      `).join('')}</tbody>
    </table>`;
  },

  // ======== 表单弹窗 ========

  showForm(rule) {
    this.editingRule = rule;
    const isEdit = !!rule;
    const title = isEdit ? '编辑告警规则' : '添加告警规则';

    const modalDiv = document.getElementById('alert-form-modal');
    if (modalDiv) modalDiv.remove();

    const html = `
      <div id="alert-form-modal" class="modal" style="display:flex">
        <div class="modal-content">
          <div class="modal-header">
            <h3>${title}</h3>
            <button class="modal-close" onclick="AlertPage.hideForm()">&times;</button>
          </div>
          <div class="modal-body">
            <form id="alert-form">
              <div class="form-group">
                <label>规则名称 *</label>
                <input type="text" id="af-name" required placeholder="如: DashScope Key失效告警">
              </div>
              <div class="form-group">
                <label>触发条件 *</label>
                <select id="af-condition" onchange="AlertPage.onConditionChange()">
                  <option value="key_failed">Key 失效 — 渠道健康检测连续失败</option>
                  <option value="balance_low">余额不足 — 剩余额度低于阈值</option>
                  <option value="error_rate">错误率过高 — 请求错误率超过阈值</option>
                  <option value="usage_spike">用量突增 — 短时间内调用量异常飙升</option>
                </select>
              </div>
              <div id="af-threshold-group" class="form-group" style="display:none">
                <label>阈值</label>
                <input type="number" id="af-threshold" step="0.01" min="0" placeholder="如: 0.8">
                <span class="hint">余额不足时为额度值，错误率时为百分比（0-1）</span>
              </div>
              <div class="form-group">
                <label>告警级别</label>
                <select id="af-level">
                  <option value="info">提示</option>
                  <option value="warning" selected>警告</option>
                  <option value="critical">严重</option>
                </select>
              </div>
              <div class="form-group">
                <label>告警通道</label>
                <div id="af-channels">
                  <label class="switch-label"><input type="checkbox" id="af-ch-wechat" value="wechat" checked><span>微信</span></label>
                  <label class="switch-label"><input type="checkbox" id="af-ch-email" value="email"><span>邮件</span></label>
                </div>
                <span class="hint">可多选，目前仅支持 wechat/email</span>
              </div>
              <div class="form-group form-row">
                <label class="switch-label">
                  <input type="checkbox" id="af-enabled" checked>
                  <span>启用规则</span>
                </label>
              </div>
              <div class="form-actions">
                <button type="submit" class="btn-primary">保存</button>
                <button type="button" class="btn-secondary" onclick="AlertPage.hideForm()">取消</button>
              </div>
            </form>
          </div>
        </div>
      </div>
    `;

    // 插入到页面中
    document.body.insertAdjacentHTML('beforeend', html);

    // 填充表单数据
    if (isEdit) {
      document.getElementById('af-name').value = rule.name || '';
      document.getElementById('af-condition').value = rule.condition_type || 'key_failed';
      document.getElementById('af-level').value = rule.level || 'warning';
      document.getElementById('af-enabled').checked = rule.enabled !== false;

      // 阈值
      const cfg = rule.condition_config || {};
      if (cfg.threshold != null) {
        document.getElementById('af-threshold').value = cfg.threshold;
      }

      // 通道
      const channels = rule.channels || [];
      document.getElementById('af-ch-wechat').checked = channels.includes('wechat');
      document.getElementById('af-ch-email').checked = channels.includes('email');

      this.onConditionChange();
    } else {
      this.onConditionChange();
    }

    const form = document.getElementById('alert-form');
    form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
  },

  hideForm() {
    const modal = document.getElementById('alert-form-modal');
    if (modal) modal.remove();
    this.editingRule = null;
  },

  onConditionChange() {
    const type = document.getElementById('af-condition').value;
    const group = document.getElementById('af-threshold-group');
    if (type === 'balance_low' || type === 'error_rate') {
      group.style.display = '';
    } else {
      group.style.display = 'none';
    }
  },

  async submitForm() {
    const name = document.getElementById('af-name').value.trim();
    if (!name) { showToast('规则名称不能为空'); return; }

    const conditionType = document.getElementById('af-condition').value;
    const level = document.getElementById('af-level').value;
    const enabled = document.getElementById('af-enabled').checked;

    // 收集通道
    const channels = [];
    if (document.getElementById('af-ch-wechat').checked) channels.push('wechat');
    if (document.getElementById('af-ch-email').checked) channels.push('email');

    // 条件配置
    const config = {};
    const threshold = parseFloat(document.getElementById('af-threshold').value);
    if (!isNaN(threshold)) {
      config.threshold = threshold;
    }

    const data = {
      name,
      condition_type: conditionType,
      condition_config: config,
      level,
      channels,
      enabled,
    };

    try {
      if (this.editingRule) {
        await API.put(`/alerts/rules/${this.editingRule.id}`, data);
        showToast('规则已更新');
      } else {
        await API.post('/alerts/rules', data);
        showToast('规则已创建');
      }
      this.hideForm();
      this.load();
    } catch (e) {
      showToast('保存失败: ' + (e.message || '未知错误'));
    }
  },

  createRule() {
    this.showForm(null);
  },

  async editRule(id) {
    try {
      const rules = (await API.get('/alerts/rules')).rules || [];
      const rule = rules.find(r => r.id === id);
      if (!rule) { showToast('规则不存在'); return; }
      this.showForm(rule);
    } catch (e) {
      showToast('加载规则失败: ' + e.message);
    }
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
};
