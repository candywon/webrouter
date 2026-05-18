/* 团队管理页面逻辑 — 组织架构 + 成员 */
const TeamPage = {
  orgTree: [],
  orgsFlat: [],
  selectedOrgId: null,
  members: [],
  editingOrgId: null,
  editingMemberId: null,
  providers: [],
  allModels: [],

  async load() {
    await Promise.all([this.loadTree(), this.loadOrgs(), this.loadProviders()]);
  },

  async loadProviders() {
    try {
      const data = await API.get('/providers/');
      this.providers = data.providers || [];
      const modelSet = new Set();
      for (const p of this.providers) {
        if (p.models) {
          for (const m of p.models) modelSet.add(m);
        }
      }
      this.allModels = Array.from(modelSet).sort();
    } catch (e) {
      console.error('Failed to load providers:', e);
    }
  },

  // ── 组织树 ──

  async loadTree() {
    try {
      const data = await API.get('/team/tree');
      this.orgTree = data.tree || [];
      this.renderTree();
    } catch (e) {
      console.error('Failed to load org tree:', e);
    }
  },

  async loadOrgs() {
    try {
      const data = await API.get('/team/orgs');
      this.orgsFlat = data.orgs || [];
    } catch (e) {
      console.error('Failed to load orgs:', e);
    }
  },

  renderTree() {
    const el = document.getElementById('team-tree');
    if (!el) return;

    if (this.orgTree.length === 0) {
      el.innerHTML = `
        <div class="empty-state">
          <div class="icon">🏢</div>
          <p>暂无组织架构</p>
          <p class="hint">点击"创建组织"开始搭建团队结构</p>
        </div>`;
      return;
    }

    el.innerHTML = `<div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px;">
      <strong style="font-size:15px;">组织架构</strong>
      <button class="btn-icon" onclick="TeamPage.showOrgForm()" title="添加组织">+</button>
    </div>
    <ul class="org-tree">${this.renderTreeNodes(this.orgTree)}</ul>`;
  },

  renderTreeNodes(nodes, depth = 0) {
    const typeIcons = { company: '🏢', department: '📁', group: '👥' };
    let html = '';
    for (const node of nodes) {
      const isActive = node.id === this.selectedOrgId;
      const quotaPct = node.quota_total > 0 ? Math.round((node.quota_used || 0) / node.quota_total * 100) : null;
      const hasChildren = node.children && node.children.length > 0;

      html += `<li class="org-node">
        <div class="org-node-content ${isActive ? 'active' : ''}" onclick="TeamPage.selectOrg(${node.id})">
          <span class="org-node-name">
            ${hasChildren ? `<span class="org-node-toggle" onclick="event.stopPropagation(); TeamPage.toggleNode(this)">&#9660;</span>` : '<span class="org-node-toggle"></span>'}
            <span class="org-node-icon">${typeIcons[node.org_type] || '📁'}</span>
            <span class="org-node-label">${this.esc(node.name)}</span>
          </span>
          <span class="org-node-count">${node.member_count || 0} 人</span>
          <span class="org-node-actions">
            <button class="org-node-action" onclick="event.stopPropagation(); TeamPage.editOrg(${node.id})" title="编辑">✏️</button>
            <button class="org-node-action" onclick="event.stopPropagation(); TeamPage.addChildOrg(${node.id})" title="添加子组织">➕</button>
            <button class="org-node-action" onclick="event.stopPropagation(); TeamPage.deleteOrg(${node.id})" title="删除">🗑️</button>
          </span>
        </div>`;

      if (quotaPct !== null) {
        const color = quotaPct >= 90 ? '#ef4444' : quotaPct >= 60 ? '#f59e0b' : '#22c55e';
        html += `<div class="org-quota-bar">
          额度: ${this.formatYuan(node.quota_used || 0)} / ${this.formatYuan(node.quota_total)} (${quotaPct}%)
          <div class="org-quota-track"><div class="org-quota-fill" style="width:${quotaPct}%;background:${color};"></div></div>
        </div>`;
      }

      if (hasChildren) {
        html += `<ul style="display:block;">${this.renderTreeNodes(node.children, depth + 1)}</ul>`;
      }
      html += '</li>';
    }
    return html;
  },

  toggleNode(btn) {
    const li = btn.closest('.org-node');
    const subUl = li.querySelector('ul');
    if (!subUl) return;
    const visible = subUl.style.display !== 'none';
    subUl.style.display = visible ? 'none' : 'block';
    btn.textContent = visible ? '▶' : '▼';
  },

  selectOrg(orgId) {
    this.selectedOrgId = orgId;
    this.renderTree();
    this.loadMembers(orgId);
  },

  // ── 成员列表 ──

  async loadMembers(orgId) {
    try {
      const data = await API.get(`/team/orgs/${orgId}/members`);
      this.members = data.members || [];
      this.renderMembers();
    } catch (e) {
      console.error('Failed to load members:', e);
    }
  },

  renderMembers() {
    const el = document.getElementById('team-content');
    if (!el) return;

    if (this.members.length === 0) {
      const org = this.orgsFlat.find(o => o.id === this.selectedOrgId);
      const orgName = org ? org.name : '';
      el.innerHTML = `
        <div class="member-header">
          <strong>成员列表 — ${this.esc(orgName)}</strong>
          <div style="display:flex;gap:8px;">
            <button class="btn-primary btn-sm" onclick="TeamPage.showMemberForm()">+ 添加成员</button>
            <button class="btn-secondary btn-sm" onclick="TeamPage.showBatchImport('org')">📥 批量导入</button>
          </div>
        </div>
        <div class="empty-state"><p>该组织暂无成员</p></div>`;
      return;
    }

    const org = this.orgsFlat.find(o => o.id === this.selectedOrgId);
    const orgName = org ? org.name : '';

    let html = `
      <div class="member-header">
        <strong>成员列表 — ${this.esc(orgName)} (${this.members.length} 人)</strong>
        <div style="display:flex;gap:8px;">
          <button class="btn-primary btn-sm" onclick="TeamPage.showMemberForm()">+ 添加成员</button>
          <button class="btn-secondary btn-sm" onclick="TeamPage.showBatchImport()">📥 批量导入</button>
        </div>
      </div>
      <table>
        <thead><tr><th>名称</th><th>API Key</th><th>邮箱</th><th>额度</th><th>已用</th><th>RPM</th><th>到期</th><th>操作</th></tr></thead>
        <tbody>`;

    for (const m of this.members) {
      const total = m.quota_total || 0;
      const used = m.quota_used || 0;
      const remaining = total > 0 ? Math.max(0, total - used) : -1;
      const statusBadge = m.enabled
        ? '<span class="badge badge-healthy">启用</span>'
        : '<span class="badge badge-dead">禁用</span>';

      html += `<tr>
        <td><strong>${this.esc(m.name)}</strong> ${statusBadge}</td>
        <td><code class="api-key" style="font-size:12px;">${this.esc(m.key_prefix || '-')}</code></td>
        <td style="font-size:12px;">${this.esc(m.member_email || '-')}</td>
        <td>${total > 0 ? this.formatYuan(total) : '<span style="color:var(--text-muted)">不限</span>'}</td>
        <td>${remaining > 0 ? this.formatYuan(remaining) : (remaining === 0 ? '¥0.00' : '<span style="color:var(--text-muted)">—</span>')}</td>
        <td>${m.rate_limit_rpm > 0 ? m.rate_limit_rpm : '<span style="color:var(--text-muted)">不限</span>'}</td>
        <td style="font-size:12px;">${m.expires_at ? this.formatDate(m.expires_at) : '<span style="color:var(--text-muted)">永久</span>'}</td>
        <td>
          <button class="btn-sm" onclick="TeamPage.editMember(${m.id})" title="编辑">✏️</button>
          <button class="btn-sm" onclick="TeamPage.showMoveDialog(${m.id})" title="转移组织">📦</button>
          <button class="btn-sm btn-danger" onclick="TeamPage.removeMember(${m.id})" title="移除">🗑️</button>
        </td>
      </tr>`;
    }

    html += '</tbody></table>';
    el.innerHTML = html;
  },

  // ── 组织表单 ──

  showOrgForm(org) {
    this.editingOrgId = org ? org.id : null;
    document.getElementById('org-form-title').textContent = org ? '编辑组织' : '创建组织';
    document.getElementById('of-name').value = org ? org.name : '';
    document.getElementById('of-type').value = org ? org.org_type : 'department';
    document.getElementById('of-quota').value = org ? (org.quota_total > 0 ? org.quota_total / 100 : 0) : 0;
    document.getElementById('of-period').value = org ? (org.quota_period || 'monthly') : 'monthly';

    // 填充父组织下拉
    const sel = document.getElementById('of-parent');
    sel.innerHTML = '<option value="">— 顶级组织 —</option>';
    for (const o of this.orgsFlat) {
      if (org && o.id === org.id) continue; // 不能选自己
      const indent = o.parent_id ? '&nbsp;&nbsp;└ ' : '';
      sel.innerHTML += `<option value="${o.id}" ${org && org.parent_id === o.id ? 'selected' : ''}>${indent}${this.esc(o.name)}</option>`;
    }

    document.getElementById('org-modal').style.display = 'flex';
    document.getElementById('org-form').onsubmit = (e) => { e.preventDefault(); this.submitOrgForm(); };
  },

  hideOrgForm() {
    document.getElementById('org-modal').style.display = 'none';
    this.editingOrgId = null;
  },

  async submitOrgForm() {
    const name = document.getElementById('of-name').value.trim();
    if (!name) { showToast('名称不能为空'); return; }

    const parentId = document.getElementById('of-parent').value;
    const quotaYuan = parseFloat(document.getElementById('of-quota').value) || 0;

    const data = {
      name,
      org_type: document.getElementById('of-type').value,
      parent_id: parentId ? parseInt(parentId) : null,
      quota_total: Math.round(quotaYuan * 100),
      quota_period: document.getElementById('of-period').value,
    };

    try {
      if (this.editingOrgId) {
        await API.put(`/team/orgs/${this.editingOrgId}`, data);
        showToast('组织已更新');
      } else {
        await API.post('/team/orgs', data);
        showToast('组织已创建');
      }
      this.hideOrgForm();
      await this.load();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
    } catch (e) {
      showToast('保存失败: ' + (e.message || '未知错误'));
    }
  },

  editOrg(id) {
    // 需要在树中查找节点
    const org = this.findOrgInTree(id);
    if (!org) return;
    this.showOrgForm(org);
  },

  addChildOrg(parentId) {
    const org = this.findOrgInTree(parentId);
    if (!org) return;
    this.showOrgForm(null);
    // 设置父级为当前组织
    setTimeout(() => {
      document.getElementById('of-parent').value = parentId;
    }, 50);
  },

  async deleteOrg(id) {
    if (!confirm('确定删除此组织吗？\n需确保该组织下无成员且无子组织。')) return;
    try {
      await API.del(`/team/orgs/${id}`);
      showToast('组织已删除');
      if (this.selectedOrgId === id) this.selectedOrgId = null;
      await this.load();
    } catch (e) {
      showToast('删除失败: ' + (e.message || '未知错误'));
    }
  },

  findOrgInTree(id, nodes) {
    if (!nodes) nodes = this.orgTree;
    for (const n of nodes) {
      if (n.id === id) return n;
      if (n.children) {
        const found = this.findOrgInTree(id, n.children);
        if (found) return found;
      }
    }
    return null;
  },

  // ── 成员表单 ──

  showMemberForm(member) {
    this.editingMemberId = member ? member.id : null;
    document.getElementById('team-form-title').textContent = member ? '编辑成员' : '添加成员';
    document.getElementById('tf-name').value = member ? member.name : '';
    document.getElementById('tf-quota').value = member ? (member.quota_total > 0 ? member.quota_total / 100 : 0) : 0;
    document.getElementById('tf-rpm').value = member ? (member.rate_limit_rpm || 0) : 0;
    document.getElementById('tf-email').value = member ? (member.member_email || '') : '';
    document.getElementById('tf-enabled').checked = member ? member.enabled !== false : true;

    if (member && member.expires_at) {
      document.getElementById('tf-expires').value = member.expires_at.substring(0, 16);
    } else {
      document.getElementById('tf-expires').value = '';
    }

    // 填充组织下拉 — 编辑模式选中成员所属组织，新建模式默认当前选中组织
    const sel = document.getElementById('tf-org');
    sel.innerHTML = '<option value="">— 未分配 —</option>';
    for (const o of this.orgsFlat) {
      const indent = o.parent_id ? '└ ' : '';
      const isOrgSelected = member
        ? (member.org_id === o.id)
        : (this.selectedOrgId === o.id);
      sel.innerHTML += `<option value="${o.id}" ${isOrgSelected ? 'selected' : ''}>${indent}${this.esc(o.name)}</option>`;
    }

    // 渲染多选下拉
    const models = member && member.models && member.models.length > 0 ? member.models : [];
    const providerIds = member && member.provider_ids && member.provider_ids.length > 0 ? member.provider_ids : [];
    this.renderMultiSelects(models, providerIds);

    document.getElementById('tf-key-section').style.display = 'none';
    const notifyGroup = document.getElementById('tf-email-notify-group');
    if (notifyGroup) {
      notifyGroup.style.display = 'block';
      document.getElementById('tf-email-notify').checked = !this.editingMemberId;
    }
    document.getElementById('team-modal').style.display = 'flex';
    document.getElementById('team-form').onsubmit = (e) => { e.preventDefault(); this.submitMemberForm(); };
  },

  hideMemberForm() {
    document.getElementById('team-modal').style.display = 'none';
    this.editingMemberId = null;
  },

  async submitMemberForm() {
    const name = document.getElementById('tf-name').value.trim();
    if (!name) { showToast('名称不能为空'); return; }

    const quotaYuan = parseFloat(document.getElementById('tf-quota').value) || 0;
    const orgId = document.getElementById('tf-org').value;

    const data = {
      name,
      org_id: orgId ? parseInt(orgId) : null,
      member_email: document.getElementById('tf-email').value.trim(),
      quota_total: Math.round(quotaYuan * 100),
      rate_limit_rpm: parseInt(document.getElementById('tf-rpm').value) || 0,
      enabled: document.getElementById('tf-enabled').checked,
    };

    const models = this.getSelectedModels();
    if (models.length > 0) data.models = models;
    const providerIds = this.getSelectedProviderIds();
    if (providerIds.length > 0) data.provider_ids = providerIds;

    const expiresVal = document.getElementById('tf-expires').value;
    if (expiresVal) {
      data.expires_at = new Date(expiresVal).toISOString();
    }

    // 邮件通知（新建和编辑均支持）
    if (document.getElementById('tf-email-notify')?.checked) {
      data.send_email = true;
    }

    try {
      if (this.editingMemberId) {
        await API.put(`/team/members/${this.editingMemberId}`, data);
        let msg = '成员信息已更新';
        if (data.send_email && data.member_email) msg += '，通知邮件已发送';
        showToast(msg);
      } else {
        const result = await API.post('/team/members', data);
        if (result.key) {
          document.getElementById('tf-key').value = result.key;
          document.getElementById('tf-key-section').style.display = 'block';
        }
        let msg = '成员已创建，请保存 API Key';
        if (result.email_sent) msg += '，邀请邮件已发送';
        showToast(msg);
      }
      this.hideMemberForm();
      await this.load();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
    } catch (e) {
      showToast('保存失败: ' + (e.message || '未知错误'));
    }
  },

  editMember(id) {
    const m = this.members.find(x => x.id === id);
    if (!m) return;
    this.showMemberForm(m);
  },

  async removeMember(id) {
    if (!confirm('确定移除此成员？\n该成员的 API Key 将立即失效。')) return;
    try {
      await API.del(`/team/members/${id}`);
      showToast('成员已移除');
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
      this.loadTree();
    } catch (e) {
      showToast('移除失败');
    }
  },

  // ── 转移成员 ──

  showMoveDialog(memberId) {
    const m = this.members.find(x => x.id === memberId);
    if (!m) return;

    let optionsHtml = '<option value="">— 未分配 —</option>';
    for (const o of this.orgsFlat) {
      const indent = o.parent_id ? '└ ' : '';
      optionsHtml += `<option value="${o.id}">${indent}${this.esc(o.name)}</option>`;
    }

    const modalHtml = `
      <div id="move-member-modal" class="modal" style="display:flex">
        <div class="modal-content" style="max-width:420px;">
          <div class="modal-header">
            <h3>转移成员 — ${this.esc(m.name)}</h3>
            <button class="modal-close" onclick="document.getElementById('move-member-modal').remove()">&times;</button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label>目标组织</label>
              <select id="move-org-select">${optionsHtml}</select>
            </div>
            <div class="form-actions">
              <button class="btn-primary" onclick="TeamPage.moveMember(${memberId})">确认转移</button>
              <button class="btn-secondary" onclick="document.getElementById('move-member-modal').remove()">取消</button>
            </div>
          </div>
        </div>
      </div>`;

    const old = document.getElementById('move-member-modal');
    if (old) old.remove();
    document.body.insertAdjacentHTML('beforeend', modalHtml);
  },

  async moveMember(memberId) {
    const orgId = document.getElementById('move-org-select').value;
    try {
      await API.put(`/team/members/${memberId}/move`, { org_id: orgId ? parseInt(orgId) : null });
      showToast('成员已转移');
      document.getElementById('move-member-modal').remove();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
      this.loadTree();
    } catch (e) {
      showToast('转移失败: ' + (e.message || '未知错误'));
    }
  },

  // ── 多选下拉组件 ──

  renderMultiSelects(selectedModels, selectedProviderIds) {
    this._renderModelSelect(selectedModels);
    this._renderProviderSelect(selectedProviderIds);
  },

  _renderModelSelect(selected) {
    const container = document.getElementById('tf-models-select');
    if (!container) return;
    const isAll = selected.length === 0;
    const displayText = isAll ? '全部模型' : selected.join(', ');

    let optionsHtml = '';
    for (const model of this.allModels) {
      const checked = isAll ? 'checked disabled' : (selected.includes(model) ? 'checked' : '');
      optionsHtml += `<label class="ms-item"><input type="checkbox" value="${this.esc(model)}" ${checked}><span>${this.esc(model)}</span></label>`;
    }

    container.innerHTML = `
      <div class="ms-display" onclick="this.parentElement.classList.toggle('ms-open')">
        <span class="ms-label">${this.esc(displayText)}</span>
        <span class="ms-arrow">▼</span>
      </div>
      <div class="ms-dropdown">
        <label class="ms-item ms-all">
          <input type="checkbox" id="ms-models-all" ${isAll ? 'checked' : ''}>
          <span>全部</span>
        </label>
        <div class="ms-options">${optionsHtml}</div>
      </div>
    `;

    const allCb = container.querySelector('#ms-models-all');
    allCb.onchange = () => {
      const items = container.querySelectorAll('.ms-options input[type=checkbox]');
      items.forEach(cb => { cb.checked = allCb.checked; cb.disabled = allCb.checked; });
      this._updateModelDisplay(container);
    };

    container.querySelectorAll('.ms-options input[type=checkbox]').forEach(cb => {
      cb.onchange = () => {
        const allChecked = Array.from(container.querySelectorAll('.ms-options input[type=checkbox]')).every(c => c.checked);
        allCb.checked = allChecked;
        this._updateModelDisplay(container);
      };
    });

    setTimeout(() => {
      document.addEventListener('click', (e) => {
        if (!container.contains(e.target)) container.classList.remove('ms-open');
      }, { once: false });
    }, 100);
  },

  _renderProviderSelect(selected) {
    const container = document.getElementById('tf-provider-select');
    if (!container) return;
    const isAll = selected.length === 0;
    const displayText = isAll ? '全部数据源' : selected.map(id => {
      const p = this.providers.find(x => x.id === id);
      return p ? p.name : `#${id}`;
    }).join(', ');

    let optionsHtml = '';
    for (const p of this.providers) {
      const checked = isAll ? 'checked disabled' : (selected.includes(p.id) ? 'checked' : '');
      optionsHtml += `<label class="ms-item"><input type="checkbox" value="${p.id}" ${checked}><span>${this.esc(p.name)}</span></label>`;
    }

    if (this.providers.length === 0) {
      optionsHtml = '<div style="padding:8px;color:var(--text-muted);font-size:12px">暂无数据源，请先添加</div>';
    }

    container.innerHTML = `
      <div class="ms-display" onclick="this.parentElement.classList.toggle('ms-open')">
        <span class="ms-label">${this.esc(displayText)}</span>
        <span class="ms-arrow">▼</span>
      </div>
      <div class="ms-dropdown">
        <label class="ms-item ms-all">
          <input type="checkbox" id="ms-providers-all" ${isAll ? 'checked' : ''}>
          <span>全部</span>
        </label>
        <div class="ms-options">${optionsHtml}</div>
      </div>
    `;

    const allCb = container.querySelector('#ms-providers-all');
    if (allCb) {
      allCb.onchange = () => {
        const items = container.querySelectorAll('.ms-options input[type=checkbox]');
        items.forEach(cb => { cb.checked = allCb.checked; cb.disabled = allCb.checked; });
        this._updateProviderDisplay(container);
      };
    }

    container.querySelectorAll('.ms-options input[type=checkbox]').forEach(cb => {
      cb.onchange = () => {
        const allChecked = Array.from(container.querySelectorAll('.ms-options input[type=checkbox]')).every(c => c.checked);
        allCb.checked = allChecked;
        this._updateProviderDisplay(container);
      };
    });

    setTimeout(() => {
      document.addEventListener('click', (e) => {
        if (!container.contains(e.target)) container.classList.remove('ms-open');
      }, { once: false });
    }, 100);
  },

  _updateModelDisplay(container) {
    const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
    const checked = Array.from(cbs).filter(cb => cb.checked).map(cb => cb.value);
    const isAll = container.querySelector('#ms-models-all')?.checked || checked.length === 0;
    const label = container.querySelector('.ms-label');
    if (label) label.textContent = isAll ? '全部模型' : checked.join(', ');
  },

  _updateProviderDisplay(container) {
    const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
    const checkedIds = Array.from(cbs).filter(cb => cb.checked).map(cb => parseInt(cb.value, 10));
    const isAll = container.querySelector('#ms-providers-all')?.checked || checkedIds.length === 0;
    const label = container.querySelector('.ms-label');
    if (label) {
      label.textContent = isAll ? '全部数据源' : checkedIds.map(id => {
        const p = this.providers.find(x => x.id === id);
        return p ? p.name : `#${id}`;
      }).join(', ');
    }
  },

  getSelectedModels() {
    const container = document.getElementById('tf-models-select');
    if (!container) return [];
    const isAll = container.querySelector('#ms-models-all')?.checked || false;
    if (isAll) return [];
    const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
    return Array.from(cbs).filter(cb => cb.checked).map(cb => cb.value);
  },

  getSelectedProviderIds() {
    const container = document.getElementById('tf-provider-select');
    if (!container) return [];
    const isAll = container.querySelector('#ms-providers-all')?.checked || false;
    if (isAll) return [];
    const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
    return Array.from(cbs).filter(cb => cb.checked).map(cb => parseInt(cb.value, 10));
  },

  // ── 工具函数 ──

  formatYuan(cents) {
    if (!cents) return '¥0.00';
    return '¥' + (cents / 100).toFixed(2);
  },

  formatDate(dateStr) {
    if (!dateStr) return '';
    try {
      const d = new Date(dateStr);
      const now = new Date();
      if (d < now) return `<span class="badge badge-dead">${d.toLocaleDateString('zh-CN')} 已过期</span>`;
      return d.toLocaleDateString('zh-CN');
    } catch {
      return dateStr;
    }
  },

  esc(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#039;');
  },

  // ── 批量导入 ──
  // 顶部入口：格式 "部门 姓名 email"，系统自动按部门归类
  // 成员列表内入口：格式 "姓名 email"，全部归入当前选中组织

  showBatchImport(mode) {
    // mode: 'global' (顶部入口) 或 'org' (列表内入口)
    if (mode === 'org' && !this.selectedOrgId) {
      showToast('请先选择左侧的组织');
      return;
    }

    const isGlobal = mode === 'global';
    const title = isGlobal ? '📥 批量导入成员（全局）' : '📥 批量导入成员';
    const subtitle = isGlobal
      ? '支持跨多个部门导入，系统自动匹配/创建部门。'
      : `成员将添加到当前选中的组织"${this.orgsFlat.find(o => o.id === this.selectedOrgId)?.name || ''}"。`;
    const placeholder = isGlobal
      ? '研发部 张三 zhangsan@example.com\n研发部 李四 lisi@example.com\n市场部 王五 wangwu@example.com'
      : '张三 zhangsan@example.com\n李四 lisi@example.com\n王五 wangwu@example.com';
    const hint = isGlobal
      ? '每行格式：<code>部门 姓名 email</code>，以 <code>#</code> 开头的行会被忽略。部门名称自动匹配现有组织，不存在时自动创建。'
      : '每行格式：<code>姓名 email</code>，以 <code>#</code> 开头的行会被忽略。所有成员归入当前选中的组织。';

    const modalHtml = `
      <div id="batch-import-modal" class="modal" style="display:flex">
        <div class="modal-content" style="max-width:650px;">
          <div class="modal-header">
            <h3>${title}</h3>
            <button class="modal-close" onclick="TeamPage.hideBatchImport()">&times;</button>
          </div>
          <div class="modal-body">
            <p style="color:var(--text-muted);font-size:13px;margin-bottom:12px;">${subtitle}</p>
            <div class="form-group">
              <label>导入数据</label>
              <textarea id="bi-text" rows="10" placeholder="${placeholder}"
style="width:100%;padding:10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);font-size:13px;font-family:monospace;resize:vertical;"></textarea>
              <span class="hint">${hint}</span>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
              <div class="form-group">
                <label>总额度（元，0=不限）</label>
                <input type="number" id="bi-quota" value="0" min="0" step="0.01">
              </div>
              <div class="form-group">
                <label>RPM 限速（0=不限）</label>
                <input type="number" id="bi-rpm" value="0" min="0">
              </div>
            </div>
            <div class="form-group">
              <label><input type="checkbox" id="bi-email-notify"> 发送邮件通知（含 API Key 和网关地址）</label>
            </div>
            <div id="bi-preview" style="display:none;">
              <h4 style="margin:8px 0 4px;font-size:14px;">预览（<span id="bi-count">0</span> 条）：</h4>
              <div id="bi-preview-list" style="max-height:150px;overflow-y:auto;font-size:12px;color:var(--text-muted);"></div>
            </div>
            <div class="form-actions">
              <button class="btn-primary" onclick="TeamPage.submitBatchImport('${mode}')">🚀 确认导入</button>
              <button class="btn-secondary" onclick="TeamPage.hideBatchImport()">取消</button>
            </div>
          </div>
        </div>
      </div>`;

    const old = document.getElementById('batch-import-modal');
    if (old) old.remove();
    document.body.insertAdjacentHTML('beforeend', modalHtml);

    // 实时预览
    const textarea = document.getElementById('bi-text');
    textarea.oninput = () => this.updateBatchPreview(isGlobal);
    // 存储模式
    this._batchImportMode = mode;
  },

  hideBatchImport() {
    const modal = document.getElementById('batch-import-modal');
    if (modal) modal.remove();
  },

  updateBatchPreview(isGlobal) {
    if (isGlobal === undefined) isGlobal = this._batchImportMode === 'global';
    const text = document.getElementById('bi-text').value;
    const previewEl = document.getElementById('bi-preview');
    const listEl = document.getElementById('bi-preview-list');
    const countEl = document.getElementById('bi-count');
    if (!previewEl) return;

    const lines = text.split('\n').filter(l => l.trim() && !l.trim().startsWith('#'));
    if (lines.length === 0) {
      previewEl.style.display = 'none';
      return;
    }

    previewEl.style.display = 'block';
    countEl.textContent = lines.length;
    listEl.innerHTML = lines.slice(0, 20).map(l => {
      const parts = l.trim().split(/\s+/);
      if (isGlobal) {
        return `<div style="padding:2px 0;border-bottom:1px solid var(--border);"><span style="color:var(--accent);">${this.esc(parts[0] || '?')}</span> → ${this.esc(parts[1] || '?')} — ${this.esc(parts[2] || '')}</div>`;
      } else {
        return `<div style="padding:2px 0;border-bottom:1px solid var(--border);">${this.esc(parts[0] || '?')} — ${this.esc(parts[1] || '')}</div>`;
      }
    }).join('');
    if (lines.length > 20) {
      listEl.innerHTML += `<div style="padding:4px;color:var(--text-muted);">... 还有 ${lines.length - 20} 条</div>`;
    }
  },

  async submitBatchImport(mode) {
    const isGlobal = mode === 'global';
    const text = document.getElementById('bi-text').value.trim();
    if (!text) {
      showToast('请输入导入数据');
      return;
    }

    const quotaYuan = parseFloat(document.getElementById('bi-quota').value) || 0;
    const rpm = parseInt(document.getElementById('bi-rpm').value) || 0;
    const sendEmail = document.getElementById('bi-email-notify').checked;

    const members = [];

    if (isGlobal) {
      // 模式：部门 姓名 email
      for (const line of text.split('\n')) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) continue;
        const parts = trimmed.split(/\s+/);
        if (parts.length < 3) {
          console.warn(`跳过无效行: ${trimmed}（需要3列：部门 姓名 email）`);
          continue;
        }
        const [dept, name, email] = parts;
        members.push({
          _dept: dept,
          name,
          member_email: email,
          quota_total: Math.round(quotaYuan * 100),
          rate_limit_rpm: rpm,
        });
      }
    } else {
      // 模式：姓名 email（归入当前选中组织）
      if (!this.selectedOrgId) {
        showToast('请先选择左侧的组织');
        return;
      }
      for (const line of text.split('\n')) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) continue;
        const parts = trimmed.split(/\s+/);
        if (parts.length < 2) continue;
        members.push({
          name: parts[0],
          member_email: parts[1],
          org_id: this.selectedOrgId,
          quota_total: Math.round(quotaYuan * 100),
          rate_limit_rpm: rpm,
        });
      }
    }

    if (members.length === 0) {
      showToast('未解析到有效的成员数据');
      return;
    }

    try {
      let result;
      if (isGlobal) {
        // 全局模式：发送 text 格式，后端按部门匹配
        result = await API.post('/team/members/batch', {
          text,
          send_email: sendEmail,
          quota_total: Math.round(quotaYuan * 100),
          rate_limit_rpm: rpm,
        });
      } else {
        result = await API.post('/team/members/batch', {
          members,
          send_email: sendEmail,
          quota_total: Math.round(quotaYuan * 100),
          rate_limit_rpm: rpm,
        });
      }

      const r = result.results;
      let msg = `批量导入完成: 成功 ${r.success.length} 个`;
      if (r.errors.length > 0) {
        msg += `, 失败 ${r.errors.length} 个`;
        console.warn('导入失败:', r.errors);
      }
      if (r.emails_sent > 0) msg += `, 邮件已发送 ${r.emails_sent} 封`;

      // 显示结果
      if (r.success.length > 0) {
        const keysHtml = r.success.map(s =>
          `<tr><td>${this.esc(s.name)}</td><td><code style="font-size:11px;">${this.esc(s.key)}</code></td><td>${s.email_sent ? '✅' : '—'}</td></tr>`
        ).join('');
        const resultModal = `
          <div id="batch-result-modal" class="modal" style="display:flex">
            <div class="modal-content" style="max-width:700px;">
              <div class="modal-header">
                <h3>导入结果</h3>
                <button class="modal-close" onclick="document.getElementById('batch-result-modal').remove()">&times;</button>
              </div>
              <div class="modal-body">
                <p style="margin-bottom:12px;">${this.esc(msg)}</p>
                <table>
                  <thead><tr><th>名称</th><th>API Key（请妥善保存）</th><th>邮件</th></tr></thead>
                  <tbody>${keysHtml}</tbody>
                </table>
                ${r.errors.length > 0 ? `<p style="margin-top:12px;color:var(--danger);font-size:12px;">失败项: ${r.errors.map(e => `${e.name || ''}: ${e.reason}`).join('; ')}</p>` : ''}
                <div class="form-actions" style="margin-top:16px;">
                  <button class="btn-primary" onclick="document.getElementById('batch-result-modal').remove()">完成</button>
                </div>
              </div>
            </div>
          </div>`;
        document.body.insertAdjacentHTML('beforeend', resultModal);
      } else {
        showToast(msg, 'error');
      }

      this.hideBatchImport();
      await this.load();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
    } catch (e) {
      showToast('批量导入失败: ' + (e.message || '未知错误'), 'error');
    }
  },
};
