// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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
      el.innerHTML = `<div class="empty-state"><div class="icon">🔔</div><p>${I18n.t('alert.noRules')}<br>${I18n.t('alert.addRuleHint')}</p></div>`;
      return;
    }

    const conditionLabels = {
      'key_failed': I18n.t("alert.keyFailed"),
      'balance_low': I18n.t("alert.balanceLow"),
      'error_rate': I18n.t("alert.errorRateHigh"),
      'usage_spike': I18n.t("alert.usageSpike"),
    };

    const levelBadge = (level) => {
      const cls = level === 'critical' ? 'dead' : level === 'warning' ? 'warning' : 'info';
      const labels = { critical: I18n.t("common.critical"), warning: I18n.t("common.warning"), info: I18n.t("common.info") };
      return `<span class="badge badge-${cls}">${labels[level] || level}</span>`;
    };

    el.innerHTML = `<table>
      <thead><tr><th>${I18n.t('common.name')}</th><th>${I18n.t('alert.triggerCondition')}</th><th>${I18n.t('alert.level')}</th><th>${I18n.t('alert.channel')}</th><th>${I18n.t('common.status')}</th><th>${I18n.t('common.actions')}</th></tr></thead>
      <tbody>${rules.map(r => `
        <tr>
          <td>${r.name}</td>
          <td>${conditionLabels[r.condition_type] || r.condition_type}</td>
          <td>${levelBadge(r.level)}</td>
          <td>${(r.channels || []).join(', ')}</td>
          <td>${r.enabled ? `<span class="badge badge-healthy">${I18n.t('common.enabled')}</span>` : `<span class="badge badge-unknown">${I18n.t('common.disabled')}</span>`}</td>
          <td>
            <button class="btn" onclick="AlertPage.editRule(${r.id})">✏️ ${I18n.t('common.edit')}</button>
            <button class="btn" onclick="AlertPage.toggleRule(${r.id}, ${!r.enabled})">${r.enabled ? I18n.t("common.disabled") : I18n.t("common.enable")}</button>
            <button class="btn btn-danger" onclick="AlertPage.deleteRule(${r.id})">${I18n.t('common.delete')}</button>
          </td>
        </tr>
      `).join('')}</tbody>
    </table>`;
  },

  // ======== 表单弹窗 ========

  showForm(rule) {
    this.editingRule = rule;
    const isEdit = !!rule;
    const title = isEdit ? I18n.t('alert.editRule') : I18n.t("alert.addRule");

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
                <label>${I18n.t('alert.ruleNameRequired')}</label>
                <input type="text" id="af-name" required placeholder="${I18n.t('alert.ruleNamePlaceholder')}">
              </div>
              <div class="form-group">
                <label>${I18n.t('alert.conditionRequired')}</label>
                <select id="af-condition" onchange="AlertPage.onConditionChange()">
                  <option value="key_failed">${I18n.t('alert.keyFailedDesc')}</option>
                  <option value="balance_low">${I18n.t('alert.balanceLowDesc')}</option>
                  <option value="error_rate">${I18n.t('alert.errorRateHighDesc')}</option>
                  <option value="usage_spike">${I18n.t('alert.usageSpikeDesc')}</option>
                </select>
              </div>
              <div id="af-threshold-group" class="form-group" style="display:none">
                <label>${I18n.t('alert.threshold')}</label>
                <input type="number" id="af-threshold" step="0.01" min="0" placeholder="${I18n.t('common.placeholderExampleNumber')}">
                <span class="hint">${I18n.t('alert.thresholdHint')}</span>
              </div>
              <div class="form-group">
                <label>${I18n.t('alert.level')}</label>
                <select id="af-level">
                  <option value="info">${I18n.t('common.info')}</option>
                  <option value="warning" selected>${I18n.t('common.warning')}</option>
                  <option value="critical">${I18n.t('common.critical')}</option>
                </select>
              </div>
              <div class="form-group">
                <label>${I18n.t('alert.channel')}</label>
                <div id="af-channels">
                  <label class="switch-label"><input type="checkbox" id="af-ch-wechat" value="wechat" checked><span>${I18n.t('alert.wechat')}</span></label>
                  <label class="switch-label"><input type="checkbox" id="af-ch-email" value="email"><span>${I18n.t('alert.email')}</span></label>
                </div>
                <span class="hint">${I18n.t('alert.channelHint')}</span>
              </div>
              <div class="form-group form-row">
                <label class="switch-label">
                  <input type="checkbox" id="af-enabled" checked>
                  <span>${I18n.t('alert.enableRule')}</span>
                </label>
              </div>
              <div class="form-actions">
                <button type="submit" class="btn-primary">${I18n.t('common.save')}</button>
                <button type="button" class="btn-secondary" onclick="AlertPage.hideForm()">${I18n.t('common.cancel')}</button>
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
    if (!name) { showToast(I18n.t("alert.ruleNameRequiredError")); return; }

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
        showToast(I18n.t("alert.ruleUpdated"));
      } else {
        await API.post('/alerts/rules', data);
        showToast(I18n.t("alert.ruleCreated"));
      }
      this.hideForm();
      this.load();
    } catch (e) {
      showToast(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
    }
  },

  createRule() {
    this.showForm(null);
  },

  async editRule(id) {
    try {
      const rules = (await API.get('/alerts/rules')).rules || [];
      const rule = rules.find(r => r.id === id);
      if (!rule) { showToast(I18n.t("alert.ruleNotFound")); return; }
      this.showForm(rule);
    } catch (e) {
      showToast(I18n.t("alert.loadRuleFailed") + e.message);
    }
  },

  async toggleRule(id, enabled) {
    try {
      await API.put(`/alerts/rules/${id}`, { enabled });
      this.load();
    } catch (e) { showToast(I18n.t("common.operationFailed")); }
  },

  async deleteRule(id) {
    if (!confirm(I18n.t("alert.confirmDelete"))) return;
    try {
      await API.del(`/alerts/rules/${id}`);
      this.load();
    } catch (e) { showToast(I18n.t('common.deleteFailed')); }
  },
};
