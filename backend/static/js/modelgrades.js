// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/**
 * 模型分级管理页面
 */
const modelGradesPage = {
  data: [],
  editingId: null,

  init() {
    this.loadGrades();
    this.loadTiers();
    this.bindForm();
  },

  load() {
    this.loadGrades();
    this.loadTiers();
  },

  async loadGrades() {
    const res = await API.get('/modelgrades/');
    this.data = res.grades || [];
    this.render();
  },

  async loadTiers() {
    const sel = document.getElementById('tier-filter');
    if (!sel) return;
    try {
      const res = await API.get('/modelgrades/tiers');
      const tiers = res.tiers || [];
      sel.innerHTML = `<option value="">${I18n.t('modelgrades.allTiers')}</option>` +
        tiers.map(t => `<option value="${t.value}">● ${t.label}</option>`).join('');
      // Store for render
      this._tiers = tiers;
    } catch (e) {
      sel.innerHTML = `<option value="">${I18n.t('modelgrades.allTiers')}</option>`;
    }
  },

  render() {
    const container = document.getElementById('modelgrades-content');
    if (!container) return;
    const tier = document.getElementById('tier-filter')?.value || '';
    const data = tier ? this.data.filter(g => g.tier === tier) : this.data;

    if (!data.length) {
      container.innerHTML = `<div class="empty-state"><div class="icon">🎯</div><p>${I18n.t('modelgrades.noData')}</p></div>`;
      return;
    }

    const groups = { economy: [], standard: [], enhanced: [], premium: [], flagship: [] };
    data.forEach(g => { if (groups[g.tier]) groups[g.tier].push(g); });

    const tierDotClass = { economy: 'dot-healthy', standard: 'dot-healthy', enhanced: 'dot-warning', premium: 'dot-dead', flagship: 'dot-dead' };
    const tierLabels = {};
    (this._tiers || []).forEach(t => {
      tierLabels[t.value] = `<span class="status-dot ${tierDotClass[t.value] || 'dot-unknown'}"></span> ${t.label}`;
    });

    let html = '';
    for (const [tier, items] of Object.entries(groups)) {
      html += `<div class="grade-group"><h4 class="grade-group-title">${tierLabels[tier]}</h4>`;
      if (!items.length) {
        html += `<div class="empty-state" style="padding:24px 0;"><p style="color:var(--text-muted);font-size:13px;">${I18n.t('modelgrades.noData')}</p></div>`;
      } else {
        html += `<table class="table"><thead><tr>`;
        html += `<th>${I18n.t('common.model')}</th><th>${I18n.t('modelgrades.costIndex')}</th><th>${I18n.t('common.vendor')}</th><th>${I18n.t('common.description')}</th><th>${I18n.t('common.sortOrder')}</th><th>${I18n.t('common.status')}</th><th>${I18n.t('common.actions')}</th>`;
        html += '</tr></thead><tbody>';
        items.forEach(g => {
          html += `<tr>
            <td><strong>${esc(g.model)}</strong></td>
            <td>${g.cost_index}</td>
            <td><span class="badge">${esc(g.vendor)}</span></td>
            <td class="text-muted" style="font-size:12px;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${esc(g.description || '-')}</td>
            <td>${g.sort_order}</td>
            <td><span class="badge badge-${g.enabled ? 'success' : 'danger'}">${g.enabled ? I18n.t("common.enable") : I18n.t("common.disabled")}</span></td>
            <td class="actions">
              <button class="btn-icon" onclick="modelGradesPage.edit('${esc(g.model)}')" title="${I18n.t('common.edit')}">✏️</button>
              <button class="btn-icon" onclick="modelGradesPage.delete('${esc(g.model)}')" title="${I18n.t('common.delete')}">🗑️</button>
            </td>
          </tr>`;
        });
        html += '</tbody></table>';
      }
      html += '</div>';
    }
    container.innerHTML = html;
  },

  onTierFilter() {
    this.render();
  },

  showAddForm() {
    this.editingId = null;
    document.getElementById('grade-form-title').textContent = I18n.t("modelgrades.addFormTitle");
    document.getElementById('gr-model').value = '';
    document.getElementById('gr-model').disabled = false;
    document.getElementById('gr-tier').value = 'economy';
    document.getElementById('gr-cost-index').value = '1.0';
    document.getElementById('gr-vendor').value = 'other';
    document.getElementById('gr-description').value = '';
    document.getElementById('gr-sort-order').value = '0';
    document.getElementById('gr-enabled').checked = true;
    document.getElementById('grade-form-modal').style.display = 'flex';
  },

  edit(model) {
    const g = this.data.find(x => x.model === model);
    if (!g) return;
    this.editingId = model;
    document.getElementById('grade-form-title').textContent = I18n.t("modelgrades.editFormTitle");
    document.getElementById('gr-model').value = g.model;
    document.getElementById('gr-model').disabled = true;
    document.getElementById('gr-tier').value = g.tier;
    document.getElementById('gr-cost-index').value = g.cost_index;
    document.getElementById('gr-vendor').value = g.vendor || 'other';
    document.getElementById('gr-description').value = g.description || '';
    document.getElementById('gr-sort-order').value = g.sort_order;
    document.getElementById('gr-enabled').checked = g.enabled;
    document.getElementById('grade-form-modal').style.display = 'flex';
  },

  hideForm() {
    document.getElementById('grade-form-modal').style.display = 'none';
  },

  bindForm() {
    const form = document.getElementById('grade-form');
    if (!form) return;
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const model = document.getElementById('gr-model').value.trim();
      const data = {
        model,
        tier: document.getElementById('gr-tier').value,
        cost_index: parseFloat(document.getElementById('gr-cost-index').value),
        vendor: document.getElementById('gr-vendor').value,
        description: document.getElementById('gr-description').value,
        sort_order: parseInt(document.getElementById('gr-sort-order').value),
        enabled: document.getElementById('gr-enabled').checked,
      };
      try {
        if (this.editingId) {
          await API.put(`/modelgrades/${model}`, data);
        } else {
          await API.post('/modelgrades/', data);
        }
        this.hideForm();
        await this.loadGrades();
      } catch (err) {
        alert(I18n.t("common.saveFailed") + err.message);
      }
    });
  },

  async delete(model) {
    if (!confirm(I18n.t('modelgrades.confirmDelete', {model}))) return;
    await API.del(`/modelgrades/${model}`);
    await this.loadGrades();
  },

  async reloadCache() {
    try {
      const res = await API.post('/modelgrades/reload');
      alert(I18n.t("modelgrades.reloadSuccess") + JSON.stringify(res.proxy_response));
    } catch (err) {
      alert(I18n.t("pricing.reloadFailed") + err.message);
    }
  },
};
