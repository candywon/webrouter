// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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
          <p>${I18n.t('team.noOrg')}</p>
          <p class="hint">${I18n.t('team.createOrgHint')}</p>
        </div>`;
      return;
    }

    el.innerHTML = `<div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px;">
      <strong style="font-size:15px;">${I18n.t('team.orgStructure')}</strong>
      <button class="btn-icon" onclick="TeamPage.showOrgForm()" title="${I18n.t('team.addOrg')}">+</button>
    </div>
    <ul class="org-tree">${this.renderTreeNodes(this.orgTree)}</ul>`;
  },

  renderTreeNodes(nodes, depth = 0) {
    const typeIcons = { company: '🏢', department: '📁', project: '📌', group: '👥' };
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
          <span class="org-node-count">${node.member_count || 0} ${I18n.t('team.people')}</span>
          <span class="org-node-actions">
            <button class="org-node-action" onclick="event.stopPropagation(); TeamPage.editOrg(${node.id})" title="${I18n.t('common.edit')}">✏️</button>
            <button class="org-node-action" onclick="event.stopPropagation(); TeamPage.addChildOrg(${node.id})" title="${I18n.t('team.addChildOrg')}">➕</button>
            <button class="org-node-action" onclick="event.stopPropagation(); TeamPage.deleteOrg(${node.id})" title="${I18n.t('common.delete')}">🗑️</button>
          </span>
        </div>`;

      if (quotaPct !== null) {
        const color = quotaPct >= 90 ? '#ef4444' : quotaPct >= 60 ? '#f59e0b' : '#22c55e';
        html += `<div class="org-quota-bar">
          ${I18n.t('team.quotaLabel')}${this.formatYuan(node.quota_used || 0)} / ${this.formatYuan(node.quota_total)} (${quotaPct}%)
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
          <strong>${I18n.t('team.memberList')}${this.esc(orgName)}</strong>
          <div style="display:flex;gap:8px;">
            <button class="btn-primary btn-sm" onclick="TeamPage.showMemberForm()">${I18n.t('team.addMember')}</button>
            <button class="btn-secondary btn-sm" onclick="TeamPage.showBatchImport('org')">${I18n.t('team.batchImport')}</button>
          </div>
        </div>
        <div class="empty-state"><p>${I18n.t('team.noMembers')}</p></div>`;
      return;
    }

    const org = this.orgsFlat.find(o => o.id === this.selectedOrgId);
    const orgName = org ? org.name : '';

    let html = `
      <div class="member-header">
        <strong>${I18n.t('team.memberList')}${this.esc(orgName)} (${this.members.length} ${I18n.t('team.people')})</strong>
        <div style="display:flex;gap:8px;">
          <button class="btn-primary btn-sm" onclick="TeamPage.showMemberForm()">${I18n.t('team.addMember')}</button>
          <button class="btn-secondary btn-sm" onclick="TeamPage.showBatchImport()">${I18n.t('team.batchImport')}</button>
        </div>
      </div>
      <table>
        <thead><tr><th>${I18n.t('common.name')}</th><th>${I18n.t('common.apiKey')}</th><th>${I18n.t('team.email')}</th><th>${I18n.t('team.quota')}</th><th>${I18n.t('team.used')}</th><th>${I18n.t('team.rpm')}</th><th>${I18n.t('team.expiry')}</th><th>${I18n.t('common.actions')}</th></tr></thead>
        <tbody>`;

    for (const m of this.members) {
      const total = m.quota_total || 0;
      const used = m.quota_used || 0;
      const remaining = total > 0 ? Math.max(0, total - used) : -1;
      const statusBadge = m.enabled
        ? `<span class="badge badge-healthy">${I18n.t('common.enabled')}</span>`
        : `<span class="badge badge-dead">${I18n.t('common.disabled')}</span>`;

      html += `<tr>
        <td><strong>${this.esc(m.name)}</strong> ${statusBadge}</td>
        <td><code class="api-key" style="font-size:12px;">${this.esc(m.key_prefix || '-')}</code></td>
        <td style="font-size:12px;">${this.esc(m.member_email || '-')}</td>
        <td>${total > 0 ? this.formatYuan(total) : `<span style="color:var(--text-muted)">${I18n.t('common.unlimited')}</span>`}</td>
        <td>${remaining > 0 ? this.formatYuan(remaining) : (remaining === 0 ? this.formatYuan(0) : '<span style="color:var(--text-muted)">—</span>')}</td>
        <td>${m.rate_limit_rpm > 0 ? m.rate_limit_rpm : `<span style="color:var(--text-muted)">${I18n.t('common.unlimited')}</span>`}</td>
        <td style="font-size:12px;">${m.expires_at ? this.formatDate(m.expires_at) : `<span style="color:var(--text-muted)">${I18n.t('team.permanent')}</span>`}</td>
        <td>
          <button class="btn-sm" onclick="TeamPage.editMember(${m.id})" title="${I18n.t('common.edit')}">✏️</button>
          <button class="btn-sm" onclick="TeamPage.showMoveDialog(${m.id})" title="${I18n.t('team.transferOrg')}">📦</button>
          <button class="btn-sm btn-danger" onclick="TeamPage.removeMember(${m.id})" title="${I18n.t('team.remove')}">🗑️</button>
        </td>
      </tr>`;
    }

    html += '</tbody></table>';
    el.innerHTML = html;
  },

  // ── 组织表单 ──

  showOrgForm(org) {
    this.editingOrgId = org ? org.id : null;
    document.getElementById('org-form-title').textContent = org ? I18n.t("team.editOrg") : I18n.t("team.createOrg");
    document.getElementById('of-name').value = org ? org.name : '';
    document.getElementById('of-type').value = org ? org.org_type : 'department';
    document.getElementById('of-quota').value = org ? (org.quota_total > 0 ? org.quota_total / 100 : 0) : 0;
    document.getElementById('of-period').value = org ? (org.quota_period || 'monthly') : 'monthly';

    // 填充父组织下拉
    const sel = document.getElementById('of-parent');
    sel.innerHTML = `<option value="">${I18n.t('team.topLevelOrg')}</option>`;
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
    if (!name) { showToast(I18n.t("common.nameRequiredError")); return; }

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
        showToast(I18n.t("team.orgUpdated"));
      } else {
        await API.post('/team/orgs', data);
        showToast(I18n.t("team.orgCreated"));
      }
      this.hideOrgForm();
      await this.load();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
    } catch (e) {
      showToast(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
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
    if (!confirm(I18n.t('team.confirmDeleteOrg'))) return;
    try {
      await API.del(`/team/orgs/${id}`);
      showToast(I18n.t("team.orgDeleted"));
      if (this.selectedOrgId === id) this.selectedOrgId = null;
      await this.load();
    } catch (e) {
      showToast(I18n.t("common.deleteFailed") + (e.message || I18n.t("common.unknownError")));
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
    document.getElementById('team-form-title').textContent = member ? I18n.t("team.editMember") : I18n.t('team.addMember');
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
    sel.innerHTML = `<option value="">${I18n.t('common.unassigned')}</option>`;
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
    // Session Memory Recall 默认启用
    const recallCb = document.getElementById('tf-session-recall-enabled');
    if (recallCb) {
      recallCb.checked = member ? !!member.session_recall_enabled : true;
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
    if (!name) { showToast(I18n.t("common.nameRequiredError")); return; }

    const quotaYuan = parseFloat(document.getElementById('tf-quota').value) || 0;
    const orgId = document.getElementById('tf-org').value;

    const data = {
      name,
      org_id: orgId ? parseInt(orgId) : null,
      member_email: document.getElementById('tf-email').value.trim(),
      quota_total: Math.round(quotaYuan * 100),
      rate_limit_rpm: parseInt(document.getElementById('tf-rpm').value) || 0,
      enabled: document.getElementById('tf-enabled').checked,
      session_recall_enabled: document.getElementById('tf-session-recall-enabled')?.checked ?? true,
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
        let msg = I18n.t("team.memberUpdated");
        if (data.send_email && data.member_email) msg += I18n.t("team.notificationSent");
        showToast(msg);
      } else {
        const result = await API.post('/team/members', data);
        if (result.key) {
          document.getElementById('tf-key').value = result.key;
          document.getElementById('tf-key-section').style.display = 'block';
        }
        let msg = I18n.t("team.memberCreated");
        if (result.email_sent) msg += I18n.t("team.invitationSent");
        showToast(msg);
      }
      this.hideMemberForm();
      await this.load();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
    } catch (e) {
      showToast(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
    }
  },

  editMember(id) {
    const m = this.members.find(x => x.id === id);
    if (!m) return;
    this.showMemberForm(m);
  },

  async removeMember(id) {
    if (!confirm(I18n.t('team.confirmRemoveMember'))) return;
    try {
      await API.del(`/team/members/${id}`);
      showToast(I18n.t("team.memberRemoved"));
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
      this.loadTree();
    } catch (e) {
      showToast(I18n.t("team.removeFailed"));
    }
  },

  // ── 转移成员 ──

  showMoveDialog(memberId) {
    const m = this.members.find(x => x.id === memberId);
    if (!m) return;

    let optionsHtml = `<option value="">${I18n.t('common.unassigned')}</option>`;
    for (const o of this.orgsFlat) {
      const indent = o.parent_id ? '└ ' : '';
      optionsHtml += `<option value="${o.id}">${indent}${this.esc(o.name)}</option>`;
    }

    const modalHtml = `
      <div id="move-member-modal" class="modal" style="display:flex">
        <div class="modal-content" style="max-width:420px;">
          <div class="modal-header">
            <h3>${I18n.t('team.transferMember')}${this.esc(m.name)}</h3>
            <button class="modal-close" onclick="document.getElementById('move-member-modal').remove()">&times;</button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label>${I18n.t('team.targetOrg')}</label>
              <select id="move-org-select">${optionsHtml}</select>
            </div>
            <div class="form-actions">
              <button class="btn-primary" onclick="TeamPage.moveMember(${memberId})">${I18n.t('team.confirmTransfer')}</button>
              <button class="btn-secondary" onclick="document.getElementById('move-member-modal').remove()">${I18n.t('common.cancel')}</button>
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
      showToast(I18n.t("team.memberTransferred"));
      document.getElementById('move-member-modal').remove();
      if (this.selectedOrgId) this.loadMembers(this.selectedOrgId);
      this.loadTree();
    } catch (e) {
      showToast(I18n.t("team.transferFailed") + (e.message || I18n.t("common.unknownError")));
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
    const displayText = isAll ? I18n.t("common.allModels") : selected.join(', ');

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
          <span>${I18n.t('common.all')}</span>
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
    const displayText = isAll ? I18n.t("common.allProviders") : selected.map(id => {
      const p = this.providers.find(x => x.id === id);
      return p ? p.name : `#${id}`;
    }).join(', ');

    let optionsHtml = '';
    for (const p of this.providers) {
      const checked = isAll ? 'checked disabled' : (selected.includes(p.id) ? 'checked' : '');
      optionsHtml += `<label class="ms-item"><input type="checkbox" value="${p.id}" ${checked}><span>${this.esc(p.name)}</span></label>`;
    }

    if (this.providers.length === 0) {
      optionsHtml = `<div style="padding:8px;color:var(--text-muted);font-size:12px">${I18n.t('tokens.noProvidersFirst')}</div>`;
    }

    container.innerHTML = `
      <div class="ms-display" onclick="this.parentElement.classList.toggle('ms-open')">
        <span class="ms-label">${this.esc(displayText)}</span>
        <span class="ms-arrow">▼</span>
      </div>
      <div class="ms-dropdown">
        <label class="ms-item ms-all">
          <input type="checkbox" id="ms-providers-all" ${isAll ? 'checked' : ''}>
          <span>${I18n.t('common.all')}</span>
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
    if (label) label.textContent = isAll ? I18n.t("common.allModels") : checked.join(', ');
  },

  _updateProviderDisplay(container) {
    const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
    const checkedIds = Array.from(cbs).filter(cb => cb.checked).map(cb => parseInt(cb.value, 10));
    const isAll = container.querySelector('#ms-providers-all')?.checked || checkedIds.length === 0;
    const label = container.querySelector('.ms-label');
    if (label) {
      label.textContent = isAll ? I18n.t("common.allProviders") : checkedIds.map(id => {
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
    const sym = (I18n.lang || '').startsWith('zh') ? '¥' : '$';
    if (!cents) return sym + '0.00';
    return sym + (cents / 100).toFixed(2);
  },

  formatDate(dateStr) {
    if (!dateStr) return '';
    try {
      const d = new Date(dateStr);
      const now = new Date();
      if (d < now) return `<span class="badge badge-dead">${I18n.t('team.expiredDate', {date: d.toLocaleDateString('zh-CN')})}</span>`;
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
      showToast(I18n.t("team.selectOrgFirst"));
      return;
    }

    const isGlobal = mode === 'global';
    const title = isGlobal ? I18n.t("team.batchImportGlobal") : I18n.t("team.batchImportTitle");
    const subtitle = isGlobal
      ? I18n.t("team.batchImportGlobalHint")
      : I18n.t('team.willAddToOrg', {org: this.orgsFlat.find(o => o.id === this.selectedOrgId)?.name || ''});
    const placeholder = isGlobal
      ? I18n.t('team.orgExample')
      : I18n.t('team.noOrgExample');
    const hint = isGlobal
      ? I18n.t("team.orgFormatHint")
      : I18n.t("team.noOrgFormatHint");

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
              <label>${I18n.t('team.importData')}</label>
              <textarea id="bi-text" rows="10" placeholder="${placeholder}"
style="width:100%;padding:10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);font-size:13px;font-family:monospace;resize:vertical;"></textarea>
              <span class="hint">${hint}</span>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
              <div class="form-group">
                <label>${I18n.t('team.quotaTotalHint')}</label>
                <input type="number" id="bi-quota" value="0" min="0" step="0.01">
              </div>
              <div class="form-group">
                <label>${I18n.t('team.rpmHint')}</label>
                <input type="number" id="bi-rpm" value="0" min="0">
              </div>
            </div>
            <div class="form-group">
              <label><input type="checkbox" id="bi-email-notify"> ${I18n.t('team.emailNotifyHint')}</label>
            </div>
            <div class="form-group">
              <label><input type="checkbox" id="bi-session-recall" checked> ${I18n.t('tokens.sessionRecall')}</label>
            </div>
            <div id="bi-preview" style="display:none;">
              <h4 style="margin:8px 0 4px;font-size:14px;">${I18n.t('team.preview')} (<span id="bi-count">0</span> ${I18n.t('team.previewCountSuffix')}):</h4>
              <div id="bi-preview-list" style="max-height:150px;overflow-y:auto;font-size:12px;color:var(--text-muted);"></div>
            </div>
            <div class="form-actions">
              <button class="btn-primary" onclick="TeamPage.submitBatchImport('${mode}')">${I18n.t('team.confirmImport')}</button>
              <button class="btn-secondary" onclick="TeamPage.hideBatchImport()">${I18n.t('common.cancel')}</button>
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
      listEl.innerHTML += `<div style="padding:4px;color:var(--text-muted);">${I18n.t('team.moreItems', {count: lines.length - 20})}</div>`;
    }
  },

  async submitBatchImport(mode) {
    const isGlobal = mode === 'global';
    const text = document.getElementById('bi-text').value.trim();
    if (!text) {
      showToast(I18n.t("team.enterImportData"));
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
          console.warn(I18n.t('team.skipInvalidRow', {row: trimmed}));
          continue;
        }
        const [dept, name, email] = parts;
        members.push({
          _dept: dept,
          name,
          member_email: email,
          quota_total: Math.round(quotaYuan * 100),
          rate_limit_rpm: rpm,
          session_recall_enabled: document.getElementById('bi-session-recall')?.checked ?? true,
        });
      }
    } else {
      // 模式：姓名 email（归入当前选中组织）
      if (!this.selectedOrgId) {
        showToast(I18n.t("team.selectOrgFirst"));
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
          session_recall_enabled: document.getElementById('bi-session-recall')?.checked ?? true,
        });
      }
    }

    if (members.length === 0) {
      showToast(I18n.t("team.noValidData"));
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
      let msg = `${I18n.t('team.batchImportDone')}${r.success.length} ${I18n.t('team.countUnit')}`;
      if (r.errors.length > 0) {
        msg += I18n.t('team.batchImportFailed', {failed: r.errors.length});
        console.warn(I18n.t('team.importFailed') + ':', r.errors);
      }
      if (r.emails_sent > 0) msg += `${I18n.t('team.emailsSent')}${r.emails_sent} ${I18n.t('team.emailUnit')}`;

      // 显示结果
      if (r.success.length > 0) {
        const keysHtml = r.success.map(s =>
          `<tr><td>${this.esc(s.name)}</td><td><code style="font-size:11px;">${this.esc(s.key)}</code></td><td>${s.email_sent ? '✅' : '—'}</td></tr>`
        ).join('');
        const resultModal = `
          <div id="batch-result-modal" class="modal" style="display:flex">
            <div class="modal-content" style="max-width:700px;">
              <div class="modal-header">
                <h3>${I18n.t('team.importResult')}</h3>
                <button class="modal-close" onclick="document.getElementById('batch-result-modal').remove()">&times;</button>
              </div>
              <div class="modal-body">
                <p style="margin-bottom:12px;">${this.esc(msg)}</p>
                <table>
                  <thead><tr><th>${I18n.t('common.name')}</th><th>${I18n.t('team.apiKeySave')}</th><th>${I18n.t('team.email')}</th></tr></thead>
                  <tbody>${keysHtml}</tbody>
                </table>
                ${r.errors.length > 0 ? `<p style="margin-top:12px;color:var(--danger);font-size:12px;">${I18n.t('team.failedItems')}${r.errors.map(e => `${e.name || ''}: ${e.reason}`).join('; ')}</p>` : ''}
                <div class="form-actions" style="margin-top:16px;">
                  <button class="btn-primary" onclick="document.getElementById('batch-result-modal').remove()">${I18n.t('common.done')}</button>
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
      showToast(I18n.t("team.batchImportError") + (e.message || I18n.t("common.unknownError")), 'error');
    }
  },
};
