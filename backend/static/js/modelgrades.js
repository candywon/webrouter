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
    sel.innerHTML = '<option value="">全部分级</option>' +
      '<option value="economy">🟢 经济型</option>' +
      '<option value="standard">🟡 标准型</option>' +
      '<option value="premium">🔴 旗舰型</option>';
  },

  render() {
    const container = document.getElementById('modelgrades-content');
    if (!container) return;
    const tier = document.getElementById('tier-filter')?.value || '';
    const data = tier ? this.data.filter(g => g.tier === tier) : this.data;

    if (!data.length) {
      container.innerHTML = '<div class="empty-state"><div class="icon">🎯</div><p>暂无模型分级数据</p></div>';
      return;
    }

    const groups = { economy: [], standard: [], premium: [] };
    data.forEach(g => { if (groups[g.tier]) groups[g.tier].push(g); });

    const tierLabels = {
      economy: '🟢 经济型',
      standard: '🟡 标准型',
      premium: '🔴 旗舰型',
    };

    let html = '';
    for (const [tier, items] of Object.entries(groups)) {
      if (!items.length) continue;
      html += `<div class="grade-group"><h4 class="grade-group-title">${tierLabels[tier]}</h4><table class="table"><thead><tr>`;
      html += '<th>模型</th><th>成本指数</th><th>厂商</th><th>描述</th><th>排序</th><th>状态</th><th>操作</th>';
      html += '</tr></thead><tbody>';
      items.forEach(g => {
        html += `<tr>
          <td><strong>${esc(g.model)}</strong></td>
          <td>${g.cost_index}</td>
          <td><span class="badge">${esc(g.vendor)}</span></td>
          <td class="text-muted" style="font-size:12px;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${esc(g.description || '-')}</td>
          <td>${g.sort_order}</td>
          <td><span class="badge badge-${g.enabled ? 'success' : 'danger'}">${g.enabled ? '启用' : '禁用'}</span></td>
          <td class="actions">
            <button class="btn-icon" onclick="modelGradesPage.edit('${esc(g.model)}')" title="编辑">✏️</button>
            <button class="btn-icon" onclick="modelGradesPage.delete('${esc(g.model)}')" title="删除">🗑️</button>
          </td>
        </tr>`;
      });
      html += '</tbody></table></div>';
    }
    container.innerHTML = html;
  },

  onTierFilter() {
    this.render();
  },

  showAddForm() {
    this.editingId = null;
    document.getElementById('grade-form-title').textContent = '添加模型分级';
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
    document.getElementById('grade-form-title').textContent = '编辑模型分级';
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
        alert('保存失败: ' + err.message);
      }
    });
  },

  async delete(model) {
    if (!confirm(`确定删除模型分级 "${model}"？`)) return;
    await API.del(`/modelgrades/${model}`);
    await this.loadGrades();
  },

  async reloadCache() {
    try {
      const res = await API.post('/modelgrades/reload');
      alert('刷新成功: ' + JSON.stringify(res.proxy_response));
    } catch (err) {
      alert('刷新失败: ' + err.message);
    }
  },
};
