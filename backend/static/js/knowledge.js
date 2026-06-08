// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 企业知识库管理页面 */
const KnowledgePage = {
  activeTab: 'stats',
  _enabled: null, // null = 未检查, true/false

  async load() {
    // 先检查知识库是否已开通
    try {
      const res = await fetch('/api/knowledge/status');
      const data = await res.json();
      this._enabled = data.enabled;
    } catch (e) {
      this._enabled = false;
    }

    if (!this._enabled) {
      this.renderOnboarding();
      return;
    }

    this.render();
    this.loadStats();
  },

  renderOnboarding() {
    const el = document.getElementById('knowledge-page-content');
    el.innerHTML = `
      <div class="page-header">
        <span class="page-title">${I18n.t('knowledge.enterpriseKnowledge')}</span>
      </div>
      <div class="empty-state" style="padding:60px 20px;text-align:center;">
        <div class="icon" style="font-size:64px;margin-bottom:20px;">🔐</div>
        <h2 style="margin-bottom:12px;color:var(--text-primary);">${I18n.t('knowledge.notEnabled')}</h2>
        <p style="color:var(--text-secondary);max-width:480px;margin:0 auto 8px;line-height:1.6;">
          ${I18n.t('knowledge.onboardingDesc')}
        </p>
        <p style="color:var(--text-muted);font-size:13px;max-width:480px;margin:0 auto 24px;">
          ${I18n.t('knowledge.dataLocal')}
        </p>
        <button class="btn-primary" style="font-size:15px;padding:10px 32px;"
                onclick="KnowledgePage.enableKnowledge()">${I18n.t('knowledge.enableBtn')}</button>
      </div>
    `;
  },

  async enableKnowledge() {
    // 第一次：请求确认
    if (!confirm(I18n.t('knowledge.confirmEnable'))) {
      return;
    }

    try {
      const res = await fetch('/api/knowledge/enable', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ confirmed: true }),
      });
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      if (data.enabled) {
        this._enabled = true;
        this.render();
        this.loadStats();
        if (typeof showToast === 'function') showToast(I18n.t("knowledge.enabled"));
      }
    } catch (e) {
      alert(I18n.t("knowledge.enableFailed") + (e.message || I18n.t("common.networkError")));
    }
  },

  render() {
    const el = document.getElementById('knowledge-page-content');
    el.innerHTML = `
      <div class="page-header">
        <span class="page-title">${I18n.t('knowledge.enterpriseKnowledge')}</span>
        <div style="display:flex;gap:8px;">
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('stats')">${I18n.t('knowledge.captureStats')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('raw')">${I18n.t('knowledge.rawDialogs')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('items')">${I18n.t('knowledge.knowledgeItems')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('reviews')">${I18n.t('knowledge.reviewQueue')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('domains')">${I18n.t('knowledge.domains')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('analyze')">${I18n.t('knowledge.singleDomainAnalysis')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('analyses')">${I18n.t('knowledge.analysisRecords')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('extract')">${I18n.t('knowledge.extract')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('search')">${I18n.t('common.search')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('memories')">${I18n.t('knowledge.memoryManagement')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('compliance')">${I18n.t('knowledge.compliance')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('quality')">${I18n.t('knowledge.searchQuality')}</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('audit')">${I18n.t('knowledge.auditLog')}</button>
        </div>
      </div>
      <div id="knowledge-tab-content"></div>
    `;
  },

  switchTab(tab) {
    this.activeTab = tab;
    this.render();
    switch (tab) {
      case 'stats': this.loadStats(); break;
      case 'raw': this.loadRaw(1); break;
      case 'items': this.loadItems(1); break;
      case 'reviews': this.loadReviews(1); break;
      case 'domains': this.loadDomains(); break;
      case 'analyze': this.renderAnalyze(); break;
      case 'analyses': this.loadAnalyses(); break;
      case 'extract': this.renderExtract(); break;
      case 'search': this.renderSearch(); break;
      case 'memories': this.loadMemories(1); break;
      case 'compliance': this.renderCompliance(); break;
      case 'quality': this.loadQuality(); break;
      case 'audit': this.loadAuditLog(1); break;
    }
  },

  // ============================================================
  // 捕获统计
  // ============================================================
  async loadStats() {
    try {
      const res = await fetch('/api/knowledge/capture_stats');
      const data = await res.json();
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-value">${data.raw.total}</div>
            <div class="stat-label">${I18n.t('knowledge.rawTotal')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color:#f59e0b">${data.raw.pending}</div>
            <div class="stat-label">${I18n.t('knowledge.pending')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color:#10b981">${data.raw.done}</div>
            <div class="stat-label">${I18n.t('knowledge.done')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${data.raw.today}</div>
            <div class="stat-label">${I18n.t('knowledge.todayNew')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color:#8b5cf6">${data.items.total}</div>
            <div class="stat-label">${I18n.t('knowledge.knowledgeItems')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${data.domains.total}</div>
            <div class="stat-label">${I18n.t('knowledge.domains')}</div>
          </div>
        </div>
        <div class="card" style="margin-top:16px">
          <div class="card-header"><span class="card-title">${I18n.t('knowledge.itemsDistribution')}</span></div>
          <div style="padding:20px">
            <p>${I18n.t('knowledge.byType')}${this.objToText(data.items.by_type || {})}</p>
            <p style="margin-top:8px">${I18n.t('knowledge.byVerification')}${this.objToText(data.items.by_verification || {})}</p>
          </div>
        </div>
      `;
    } catch (e) {
      document.getElementById('knowledge-tab-content').innerHTML =
        `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  objToText(obj) {
    const entries = Object.entries(obj);
    if (!entries.length) return I18n.t('common.noData');
    return entries.map(([k, v]) => `${k}: ${v}`).join('、');
  },

  // ============================================================
  // 原始对话
  // ============================================================
  async loadRaw(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>${I18n.t('common.loading')}</p></div>`;
    try {
      const res = await fetch(`/api/knowledge/raw?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = `<div class="empty-state"><div class="icon">📭</div><p>${I18n.t('knowledge.noRawData')}</p></div>`;
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.rawDialogsWith', {count: data.total})}</span>
            <span class="text-sm text-muted">${I18n.t('common.pageOf', {page: data.page, total: Math.ceil(data.total / data.per_page) || 1})}</span>
          </div>
          <table class="table">
            <thead><tr>
              <th>${I18n.t('common.id')}</th><th>${I18n.t('common.tokens')}</th><th>${I18n.t('common.model')}</th><th>${I18n.t('knowledge.turns')}</th><th>${I18n.t('common.status')}</th><th>${I18n.t('common.time')}</th><th>${I18n.t('common.actions')}</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td>${i.id}</td>
                <td>${i.token_name || '-'}</td>
                <td><code>${i.model_name}</code></td>
                <td>${i.turn_count}</td>
                <td><span class="badge badge-${this.statusColor(i.status)}">${i.status}</span></td>
                <td class="text-sm text-muted">${i.created_at ? i.created_at.slice(0, 19) : '-'}</td>
                <td><button class="btn btn-sm" onclick="KnowledgePage.showRawDetail(${i.id})">${I18n.t('common.view')}</button></td>
              </tr>`).join('')}
            </tbody>
          </table>
          <div style="padding:12px 20px;display:flex;gap:8px;justify-content:center">
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadRaw(${data.page - 1})">${I18n.t('common.prevPage')}</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadRaw(${data.page + 1})">${I18n.t('common.nextPage')}</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  statusColor(s) {
    return { pending: 'warning', processing: 'info', done: 'success', skipped: 'default' }[s] || 'default';
  },

  async showRawDetail(id) {
    try {
      const res = await fetch(`/api/knowledge/raw/${id}`);
      const d = await res.json();
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.rawDialogDetail')}${d.id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadRaw(1)">${I18n.t('common.backToList')}</button>
          </div>
          <div style="padding:20px">
            <p><b>${I18n.t('knowledge.tokenLabel')}</b> ${d.token_name} (ID: ${d.token_id})</p>
            <p><b>${I18n.t('knowledge.modelLabel')}</b> <code>${d.model_name}</code></p>
            <p><b>${I18n.t('knowledge.statusLabel')}</b> <span class="badge badge-${this.statusColor(d.status)}">${d.status}</span></p>
            <p><b>${I18n.t('knowledge.turnCountLabel')}</b> ${d.turn_count}</p>
            <p><b>${I18n.t('knowledge.timeLabel')}</b> ${d.created_at ? d.created_at.slice(0, 19) : '-'}</p>
            <hr>
            <p><b>${I18n.t('knowledge.promptLabel')}</b></p>
            <pre style="max-height:200px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(d.prompt)}</pre>
            <p><b>${I18n.t('knowledge.responseLabel')}</b></p>
            <pre style="max-height:300px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(d.response)}</pre>
          </div>
        </div>
      `;
    } catch (e) {
      alert(I18n.t("tokens.loadDetailFailed") + e.message);
    }
  },

  // ============================================================
  // 知识条目
  // ============================================================
  async loadItems(page = 1) {
    // 先加载部门列表
    let departments = [];
    try {
      const res = await fetch('/api/knowledge/domains');
      const data = await res.json();
      departments = [...new Set((data.domains || []).filter(d => d.department).map(d => d.department))];
    } catch (e) {}

    const el = document.getElementById('knowledge-tab-content');
    const deptFilter = departments.length ? `<label style="font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.department')}:</label>
      <select id="items-dept-filter" onchange="KnowledgePage.loadItems(1)" style="padding:6px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
        <option value="">${I18n.t('common.all')}</option>
        ${departments.map(d => `<option value="${d}">${d}</option>`).join('')}
      </select>` : '';
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>${I18n.t('common.loading')}</p></div>`;
    try {
      const dept = document.getElementById('items-dept-filter')?.value || '';
      const res = await fetch(`/api/knowledge/items?page=${page}&per_page=20${dept ? '&department=' + encodeURIComponent(dept) : ''}`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = `<div class="empty-state"><div class="icon">📭</div><p>${I18n.t('knowledge.noItems')}</p></div>`;
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.itemsWith', {total: data.total})}</span>
            ${deptFilter ? '<div style="display:flex;gap:8px;align-items:center">' + deptFilter + '</div>' : ''}
          </div>
          <table class="table">
            <thead><tr>
              <th>${I18n.t('common.id')}</th><th>${I18n.t('common.type')}</th><th>${I18n.t('common.title')}</th><th>${I18n.t('knowledge.department')}</th><th>${I18n.t('knowledge.domain')}</th><th>${I18n.t('knowledge.confidence')}</th><th>${I18n.t('knowledge.verification')}</th><th>${I18n.t('common.time')}</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td>${i.id}</td>
                <td><span class="badge badge-${this.typeColor(i.type)}">${i.type}</span></td>
                <td>${this.escapeHtml(i.title)}</td>
                <td>${i.department || '-'}</td>
                <td>${i.domain_code || '-'}</td>
                <td>${(i.confidence * 100).toFixed(0)}%</td>
                <td><span class="badge badge-${this.verificationColor(i.verification)}">${i.verification}</span></td>
                <td class="text-sm text-muted">${i.created_at ? i.created_at.slice(0, 10) : '-'}</td>
              </tr>`).join('')}
            </tbody>
          </table>
          <div style="padding:12px 20px;display:flex;gap:8px;justify-content:center">
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadItems(${data.page - 1})">${I18n.t('common.prevPage')}</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadItems(${data.page + 1})">${I18n.t('common.nextPage')}</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  typeColor(t) {
    return { factual: 'info', analytical: 'warning', procedural: 'success' }[t] || 'default';
  },
  verificationColor(v) {
    return { auto: 'default', pending: 'warning', verified: 'success', rejected: 'danger' }[v] || 'default';
  },

  // ============================================================
  // 审核队列
  // ============================================================
  async loadReviews(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>${I18n.t('common.loading')}</p></div>`;
    try {
      const res = await fetch(`/api/knowledge/reviews?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = `<div class="empty-state"><div class="icon">✅</div><p>${I18n.t('knowledge.reviewEmpty')}</p></div>`;
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.reviewQueueWith', {total: data.total})}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.batchApprove()" style="color:#10b981">${I18n.t('knowledge.batchApprove')}</button>
          </div>
          <table class="table">
            <thead><tr>
              <th><input type="checkbox" id="review-select-all" onchange="KnowledgePage.toggleAllReviews(this.checked)"/></th>
              <th>${I18n.t('common.id')}</th><th>${I18n.t('common.type')}</th><th>${I18n.t('common.title')}</th><th>${I18n.t('knowledge.domain')}</th><th>${I18n.t('knowledge.confidence')}</th><th>${I18n.t('common.time')}</th><th>${I18n.t('common.actions')}</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td><input type="checkbox" class="review-checkbox" value="${i.id}"/></td>
                <td>${i.id}</td>
                <td><span class="badge badge-${this.typeColor(i.type)}">${i.type}</span></td>
                <td>${this.escapeHtml(i.title)}</td>
                <td>${i.domain_code || '-'}</td>
                <td>${(i.confidence * 100).toFixed(0)}%</td>
                <td class="text-sm text-muted">${i.created_at ? i.created_at.slice(0, 10) : '-'}</td>
                <td>
                  <button class="btn btn-sm" onclick="KnowledgePage.showReviewEdit(${i.id})">${I18n.t('common.edit')}</button>
                  <button class="btn btn-sm" style="color:#10b981" onclick="KnowledgePage.approveItem(${i.id})">${I18n.t('knowledge.approve')}</button>
                  <button class="btn btn-sm" style="color:var(--color-danger)" onclick="KnowledgePage.rejectItem(${i.id})">${I18n.t('knowledge.reject')}</button>
                </td>
              </tr>`).join('')}
            </tbody>
          </table>
          <div style="padding:12px 20px;display:flex;gap:8px;justify-content:center">
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadReviews(${data.page - 1})">${I18n.t('common.prevPage')}</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadReviews(${data.page + 1})">${I18n.t('common.nextPage')}</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  toggleAllReviews(checked) {
    document.querySelectorAll('.review-checkbox').forEach(cb => { cb.checked = checked; });
  },

  getSelectedReviewIds() {
    return Array.from(document.querySelectorAll('.review-checkbox:checked')).map(cb => parseInt(cb.value));
  },

  async approveItem(id) {
    if (!confirm(I18n.t("knowledge.confirmApprove"))) return;
    try {
      const res = await fetch(`/api/knowledge/reviews/${id}/approve`, { method: 'POST' });
      const data = await res.json();
      if (data.error) { alert(I18n.t("knowledge.reviewFailed") + data.error); return; }
      alert(I18n.t("knowledge.approved"));
      this.loadReviews(1);
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  async rejectItem(id) {
    if (!confirm(I18n.t("knowledge.confirmReject"))) return;
    try {
      const res = await fetch(`/api/knowledge/reviews/${id}/reject`, { method: 'POST' });
      const data = await res.json();
      if (data.error) { alert(I18n.t("knowledge.operationFailed") + data.error); return; }
      alert(I18n.t("knowledge.rejected"));
      this.loadReviews(1);
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  async showReviewEdit(id) {
    try {
      const res = await fetch(`/api/knowledge/items/${id}`);
      const item = await res.json();
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.editItem')}${id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadReviews(1)">${I18n.t('common.back')}</button>
          </div>
          <div style="padding:20px">
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('common.title')}</label>
              <input type="text" id="edit-item-title" value="${this.escapeHtml(item.title)}"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.summary')}</label>
              <textarea id="edit-item-summary" rows="4"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">${this.escapeHtml(item.summary)}</textarea>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;margin-bottom:16px">
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('common.type')}</label>
                <select id="edit-item-type" style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                  <option value="factual" ${item.type==='factual'?'selected':''}>factual</option>
                  <option value="analytical" ${item.type==='analytical'?'selected':''}>analytical</option>
                  <option value="procedural" ${item.type==='procedural'?'selected':''}>procedural</option>
                </select>
              </div>
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.confidence')}</label>
                <input type="number" id="edit-item-confidence" value="${item.confidence}" min="0" max="1" step="0.01"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.domain')}</label>
                <input type="text" id="edit-item-domain" value="${item.domain_code || ''}"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
            </div>
            <hr style="margin:16px 0;border-color:var(--border)"/>
            <p style="font-size:12px;color:var(--text-muted);margin-bottom:8px">${I18n.t('knowledge.sourceQuote')}</p>
            <pre style="max-height:200px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(item.source_quote)}</pre>
            <div style="margin-top:16px;display:flex;gap:8px">
              <button class="btn" onclick="KnowledgePage.saveReviewEdit(${id})">${I18n.t('common.save')}</button>
              <button class="btn" onclick="KnowledgePage.loadReviews(1)">${I18n.t('common.cancel')}</button>
            </div>
          </div>
        </div>
      `;
    } catch (e) { alert(I18n.t('common.loadFailed') + '：' + e.message); }
  },

  async saveReviewEdit(id) {
    const title = document.getElementById('edit-item-title').value;
    const summary = document.getElementById('edit-item-summary').value;
    const type = document.getElementById('edit-item-type').value;
    const confidence = parseFloat(document.getElementById('edit-item-confidence').value);
    try {
      const res = await fetch(`/api/knowledge/reviews/${id}/edit`, {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ title, summary, type, confidence }),
      });
      const data = await res.json();
      if (data.error) { alert(I18n.t('common.saveFailed') + data.error); return; }
      alert(I18n.t("knowledge.saved"));
      this.loadReviews(1);
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  async batchApprove() {
    const ids = this.getSelectedReviewIds();
    if (!ids.length) { alert(I18n.t("knowledge.selectItemsFirst")); return; }
    if (!confirm(I18n.t('knowledge.confirmBatchApprove', {count: ids.length}))) return;
    try {
      const res = await fetch('/api/knowledge/reviews/batch-approve', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ ids }),
      });
      const data = await res.json();
      if (data.error) { alert(I18n.t("knowledge.batchReviewFailed") + data.error); return; }
      alert(data.message || I18n.t("knowledge.batchApproveSuccess"));
      this.loadReviews(1);
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  // ============================================================
  // 业务域管理
  // ============================================================
  async loadDomains() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>${I18n.t('common.loading')}</p></div>`;
    try {
      const [domRes, riskRes] = await Promise.all([
        fetch('/api/knowledge/domains'),
        fetch('/api/knowledge/domain_risk'),
      ]);
      const domData = await domRes.json();
      const riskData = await riskRes.json();
      const riskMap = {};
      (riskData.configs || []).forEach(c => { riskMap[c.domain_code] = c; });

      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.domainsWith', {total: domData.total})}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.showCreateDomain()">${I18n.t('knowledge.addDomain')}</button>
          </div>
          <table class="table">
            <thead><tr>
              <th>${I18n.t('knowledge.code')}</th><th>${I18n.t('common.name')}</th><th>${I18n.t('knowledge.department')}</th><th>${I18n.t('common.status')}</th><th>${I18n.t('knowledge.itemCount')}</th><th>${I18n.t('knowledge.riskLevel')}</th><th>${I18n.t('knowledge.minVerification')}</th><th>${I18n.t('common.actions')}</th>
            </tr></thead>
            <tbody>
              ${domData.domains.map(d => {
                const r = riskMap[d.domain_code] || {};
                const isPending = d.status === 'pending';
                const isMerged = d.status === 'merged';
                return `<tr style="${isMerged ? 'opacity:0.5' : ''}">
                  <td><code>${d.domain_code}</code></td>
                  <td>${d.domain_name}</td>
                  <td>${d.department || '-'}</td>
                  <td><span class="badge badge-${d.status === 'active' ? 'success' : isMerged ? 'default' : 'warning'}">${d.status}</span></td>
                  <td><span id="domain-count-${d.domain_code}">-</span></td>
                  <td>${r.risk_level || '-'}</td>
                  <td>${r.min_verification || '-'}</td>
                  <td>
                    ${isPending ? `<button class="btn btn-sm" style="color:#10b981" onclick="KnowledgePage.confirmDomain('${d.domain_code}')">${I18n.t('common.confirm')}</button>` : ''}
                    ${!isPending && !isMerged ? `<button class="btn btn-sm" onclick="KnowledgePage.showDomainStats('${d.domain_code}')">${I18n.t('knowledge.stats')}</button>
                    <button class="btn btn-sm" onclick="KnowledgePage.showMergeDomain('${d.domain_code}')">${I18n.t('knowledge.merge')}</button>` : ''}
                    ${isMerged ? `<span class="text-sm text-muted">${I18n.t('knowledge.merged')}</span>` : ''}
                  </td>
                </tr>`;
              }).join('')}
            </tbody>
          </table>
        </div>
        <div id="domain-action-panel" style="margin-top:16px"></div>
      `;

      // 异步加载各域的知识条数
      domData.domains.forEach(d => {
        if (d.status !== 'merged') {
          this.fetchDomainCount(d.domain_code);
        }
      });
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  async fetchDomainCount(code) {
    try {
      const res = await fetch(`/api/knowledge/items?page=1&per_page=1&domain=${code}`);
      const data = await res.json();
      const el = document.getElementById(`domain-count-${code}`);
      if (el) el.textContent = data.total || 0;
    } catch (e) {}
  },

  async confirmDomain(code) {
    if (!confirm(I18n.t('knowledge.confirmDomain', {code}))) return;
    try {
      const res = await fetch(`/api/knowledge/domains/${code}/confirm`, { method: 'POST' });
      const data = await res.json();
      if (data.error) { alert(I18n.t("knowledge.operationFailed") + data.error); return; }
      alert(I18n.t("knowledge.confirmed"));
      this.loadDomains();
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  async showCreateDomain() {
    const panel = document.getElementById('domain-action-panel');
    panel.innerHTML = `
      <div class="card">
        <div class="card-header">
          <span class="card-title">${I18n.t('knowledge.addDomain')}</span>
          <button class="btn btn-sm" onclick="document.getElementById('domain-action-panel').innerHTML=''">${I18n.t('common.close')}</button>
        </div>
        <div style="padding:20px">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.domainCodeRequired')}</label>
              <input type="text" id="new-domain-code" placeholder="${I18n.t('knowledge.domainCodePlaceholder')}"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.domainNameRequired')}</label>
              <input type="text" id="new-domain-name" placeholder="${I18n.t('knowledge.domainNamePlaceholder')}"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
          </div>
          <div style="margin-bottom:16px">
            <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.department')}</label>
            <input type="text" id="new-domain-dept" placeholder="${I18n.t('knowledge.deptPlaceholder')}"
              style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
          </div>
          <div style="margin-bottom:16px">
            <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('common.description')}</label>
            <textarea id="new-domain-desc" rows="2"
              style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"></textarea>
          </div>
          <button class="btn" onclick="KnowledgePage.createDomain()">${I18n.t('common.create')}</button>
        </div>
      </div>
    `;
  },

  async createDomain() {
    const code = document.getElementById('new-domain-code').value.trim();
    const name = document.getElementById('new-domain-name').value.trim();
    const dept = document.getElementById('new-domain-dept').value.trim();
    const desc = document.getElementById('new-domain-desc').value.trim();
    if (!code || !name) { alert(I18n.t("knowledge.domainCodeNameRequired")); return; }
    try {
      const res = await fetch('/api/knowledge/domains', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ domain_code: code, domain_name: name, department: dept, description: desc }),
      });
      const data = await res.json();
      if (data.error) { alert(I18n.t("knowledge.createFailed") + data.error); return; }
      alert(I18n.t("knowledge.created"));
      this.loadDomains();
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  async showDomainStats(code) {
    const panel = document.getElementById('domain-action-panel');
    panel.innerHTML = `<div class="card"><div style="padding:20px"><p class="text-muted">${I18n.t('common.loading')}</p></div></div>`;
    try {
      const res = await fetch(`/api/knowledge/domains/${code}/stats`);
      const data = await res.json();
      if (data.error) { panel.innerHTML = `<div class="card"><div style="padding:20px"><p style="color:var(--color-danger)">${data.error}</p></div></div>`; return; }
      const d = data.domain;
      const ibt = Object.entries(data.items.by_type || {}).map(([k,v]) => `${k}: ${v}`).join('、') || I18n.t('common.noData');
      const ibv = Object.entries(data.items.by_verification || {}).map(([k,v]) => `${k}: ${v}`).join('、') || I18n.t('common.noData');
      panel.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.domainStats')}${d.domain_name} (${code})</span>
            <button class="btn btn-sm" onclick="document.getElementById('domain-action-panel').innerHTML=''">${I18n.t('common.close')}</button>
          </div>
          <div style="padding:20px">
            <div class="stats-grid" style="margin-bottom:16px">
              <div class="stat-card"><div class="stat-value">${data.items.total}</div><div class="stat-label">${I18n.t('knowledge.knowledgeItems')}</div></div>
              <div class="stat-card"><div class="stat-value" style="color:#f59e0b">${data.raw_pending}</div><div class="stat-label">${I18n.t('knowledge.pendingRaw')}</div></div>
              <div class="stat-card"><div class="stat-value">${d.sample_count || 0}</div><div class="stat-label">${I18n.t('knowledge.sampleCount')}</div></div>
            </div>
            <p><b>${I18n.t('knowledge.byType')}</b>${ibt}</p>
            <p style="margin-top:8px"><b>${I18n.t('knowledge.byVerification')}</b>${ibv}</p>
            <p style="margin-top:8px"><b>${I18n.t('common.status')}：</b><span class="badge badge-${d.status === 'active' ? 'success' : 'warning'}">${d.status}</span></p>
          </div>
        </div>
      `;
    } catch (e) { panel.innerHTML = `<div class="card"><div style="padding:20px"><p style="color:var(--color-danger)">${I18n.t('common.loadFailed')}：${e.message}</p></div></div>`; }
  },

  async showMergeDomain(code) {
    // 先加载可选目标域
    try {
      const res = await fetch('/api/knowledge/domains');
      const data = await res.json();
      const options = data.domains.filter(d => d.domain_code !== code && d.status !== 'merged')
        .map(d => `<option value="${d.domain_code}">${d.domain_name} (${d.domain_code})</option>`).join('');
      const panel = document.getElementById('domain-action-panel');
      panel.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.mergeDomainTitle', {code})}</span>
            <button class="btn btn-sm" onclick="document.getElementById('domain-action-panel').innerHTML=''">${I18n.t('common.close')}</button>
          </div>
          <div style="padding:20px">
            <p class="text-muted" style="margin-bottom:12px">${I18n.t('knowledge.mergeHint', {code})}</p>
            <div style="display:flex;gap:8px;align-items:center;margin-bottom:16px">
              <select id="merge-target" style="padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                <option value="">${I18n.t('knowledge.selectTargetDomain')}</option>
                ${options}
              </select>
              <button class="btn" onclick="KnowledgePage.mergeDomain('${code}')">${I18n.t('knowledge.confirmMerge')}</button>
            </div>
          </div>
        </div>
      `;
    } catch (e) { alert(I18n.t("knowledge.loadDomainsFailed") + e.message); }
  },

  async mergeDomain(code) {
    const target = document.getElementById('merge-target').value;
    if (!target) { alert(I18n.t("knowledge.selectTargetDomain")); return; }
    if (!confirm(I18n.t('knowledge.confirmMergeIrreversible', {code, target}))) return;
    try {
      const res = await fetch(`/api/knowledge/domains/${code}/merge`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ target_code: target }),
      });
      const data = await res.json();
      if (data.error) { alert(I18n.t("knowledge.mergeFailed") + data.error); return; }
      alert(data.message || I18n.t("knowledge.mergeSuccess"));
      this.loadDomains();
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  // ============================================================
  // 知识提取
  // ============================================================
  renderExtract() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">${I18n.t('knowledge.llmExtract')}</span></div>
        <div style="padding:20px">
          <p class="text-muted" style="margin-bottom:16px">
            ${I18n.t('knowledge.extractDesc')}<br>
            ${I18n.t('knowledge.extractDetail')}
          </p>
          <div style="display:flex;gap:8px;margin-bottom:16px">
            <input type="number" id="extract-batch-size" value="5" min="1" max="20"
              style="width:80px;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"
              placeholder="${I18n.t('knowledge.batchSize')}"/>
            <span style="line-height:36px;color:var(--text-muted)">${I18n.t('knowledge.perBatch')}</span>
            <button class="btn" onclick="KnowledgePage.runExtract()">${I18n.t('knowledge.startExtract')}</button>
          </div>
          <div id="extract-result"></div>
        </div>
      </div>
    `;
  },

  async runExtract() {
    const batchSize = parseInt(document.getElementById('extract-batch-size').value) || 5;
    const el = document.getElementById('extract-result');
    el.innerHTML = `<p class="text-muted">${I18n.t('knowledge.extracting')}</p>`;
    try {
      const res = await fetch('/api/knowledge/extract', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({batch_size: batchSize}),
      });
      const data = await res.json();
      if (data.error) {
        el.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('knowledge.extractFailed')}${data.error}</p>`;
        return;
      }
      el.innerHTML = `
        <div style="padding:16px;background:var(--bg-secondary);border-radius:8px">
          <p><b>${I18n.t('knowledge.extractDone')}</b></p>
          <p>${I18n.t('knowledge.processedCount')}${data.processed || 0}</p>
          <p>${I18n.t('knowledge.duration')}${data.duration_ms || 0}ms</p>
          <p>${data.message || ''}</p>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('knowledge.requestFailed')}${e.message}</p>`;
    }
  },

  // ============================================================
  // 单域分析
  // ============================================================
  async renderAnalyze() {
    // 先加载域名列表
    let domains = [];
    try {
      const res = await fetch('/api/knowledge/domains');
      const data = await res.json();
      domains = data.domains || [];
    } catch (e) {}

    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">${I18n.t('knowledge.singleDomainAnalysis')}</span></div>
        <div style="padding:20px">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.domainRequired')}</label>
              <select id="analyze-domain" style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                <option value="">${I18n.t('knowledge.selectDomain')}</option>
                ${domains.map(d => `<option value="${d.domain_code}">${d.domain_name} (${d.domain_code})</option>`).join('')}
              </select>
            </div>
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.analysisType')}</label>
              <select id="analyze-type" style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                <option value="domain_overview">${I18n.t('knowledge.domainOverview')}</option>
                <option value="trend">${I18n.t('knowledge.trendAnalysis')}</option>
                <option value="gap">${I18n.t('knowledge.gapAnalysis')}</option>
              </select>
            </div>
          </div>
          <button class="btn" onclick="KnowledgePage.runAnalysis()">${I18n.t('knowledge.startAnalysis')}</button>
          <div id="analyze-result" style="margin-top:16px"></div>
        </div>
      </div>
    `;
  },

  async runAnalysis() {
    const domain = document.getElementById('analyze-domain').value;
    const analysisType = document.getElementById('analyze-type').value;
    const el = document.getElementById('analyze-result');

    if (!domain) {
      el.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('knowledge.selectDomain')}</p>`;
      return;
    }

    el.innerHTML = `<p class="text-muted">${I18n.t('knowledge.analyzing')}</p>`;
    try {
      const res = await fetch('/api/knowledge/analyze', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          domain_code: domain,
          analysis_type: analysisType,
        }),
      });
      const data = await res.json();
      if (data.error) {
        el.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('knowledge.analysisFailed')}${data.error}</p>`;
        return;
      }
      // 简单 markdown 渲染
      const result = (data.result || '').replace(/\n/g, '<br>').replace(/\*\*(.+?)\*\*/g, '<b>$1</b>');
      el.innerHTML = `<div style="padding:16px;background:var(--bg-secondary);border-radius:8px;white-space:pre-wrap;line-height:1.8">${result}</div>`;
    } catch (e) {
      el.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('knowledge.requestFailed')}${e.message}</p>`;
    }
  },

  // ============================================================
  // 分析记录
  // ============================================================
  async loadAnalyses(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>${I18n.t('common.loading')}</p></div>`;
    try {
      const res = await fetch(`/api/knowledge/analyses?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = `<div class="empty-state"><div class="icon">📭</div><p>${I18n.t('knowledge.noAnalyses')}</p></div>`;
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header"><span class="card-title">${I18n.t('knowledge.analysesWith', {total: data.total})}</span></div>
          <table class="table">
            <thead><tr>
              <th>${I18n.t('knowledge.taskId')}</th><th>${I18n.t('knowledge.domain')}</th><th>${I18n.t('common.type')}</th><th>${I18n.t('knowledge.count')}</th><th>${I18n.t('common.status')}</th><th>${I18n.t('common.time')}</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td><code class="text-sm">${i.task_id ? i.task_id.slice(0,24)+'...' : '-'}</code></td>
                <td>${i.domains || '-'}</td>
                <td>${i.analysis_type || '-'}</td>
                <td>${i.item_count || 0}</td>
                <td><span class="badge badge-${i.status === 'completed' ? 'success' : i.status === 'failed' ? 'danger' : 'warning'}">${i.status}</span></td>
                <td class="text-sm text-muted">${i.created_at ? i.created_at.slice(0, 19) : '-'}</td>
              </tr>`).join('')}
            </tbody>
          </table>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  // ============================================================
  // 搜索
  // ============================================================
  renderSearch() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">${I18n.t('knowledge.domainSearch')}</span></div>
        <div style="padding:20px">
          <div style="display:flex;gap:8px;margin-bottom:16px">
            <input type="text" id="knowledge-search-input" placeholder="${I18n.t('knowledge.searchPlaceholder')}"
              style="flex:1;padding:8px 12px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"
              onkeydown="if(event.key==='Enter')KnowledgePage.doSearch()"/>
            <button class="btn" onclick="KnowledgePage.doSearch()">${I18n.t('common.search')}</button>
          </div>
          <div id="knowledge-search-results"></div>
        </div>
      </div>
    `;
    document.getElementById('knowledge-search-input').focus();
  },

  async doSearch() {
    const q = document.getElementById('knowledge-search-input').value.trim();
    const el = document.getElementById('knowledge-search-results');
    if (!q) {
      el.innerHTML = `<p class="text-muted">${I18n.t('knowledge.enterSearchKeyword')}</p>`;
      return;
    }
    el.innerHTML = `<p class="text-muted">${I18n.t('knowledge.searching')}</p>`;
    try {
      const res = await fetch(`/api/knowledge/search?q=${encodeURIComponent(q)}`);
      const data = await res.json();
      if (data.items.total === 0 && data.raw_count === 0) {
        el.innerHTML = `<p class="text-muted">${I18n.t('knowledge.noSearchResults')}</p>`;
        return;
      }
      let html = `<p class="text-sm text-muted">${I18n.t('knowledge.found')}${data.items.total} ${I18n.t('knowledge.itemsFound')}${data.raw_count} ${I18n.t('knowledge.rawFound')}</p>`;
      if (data.items.results.length) {
        html += `<table class="table"><thead><tr><th>${I18n.t('common.title')}</th><th>${I18n.t('common.type')}</th><th>${I18n.t('knowledge.domain')}</th><th>${I18n.t('knowledge.confidence')}</th></tr></thead><tbody>`;
        data.items.results.forEach(i => {
          html += `<tr>
            <td>${this.escapeHtml(i.title)}</td>
            <td><span class="badge badge-${this.typeColor(i.type)}">${i.type}</span></td>
            <td>${i.domain_code || '-'}</td>
            <td>${(i.confidence * 100).toFixed(0)}%</td>
          </tr>`;
        });
        html += '</tbody></table>';
      }
      el.innerHTML = html;
    } catch (e) {
      el.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('knowledge.searchFailed')}${e.message}</p>`;
    }
  },

  // ============================================================
  // 记忆管理
  // ============================================================
  async loadMemories(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">${I18n.t('knowledge.memoryManagement')}</span></div>
        <div style="padding:20px">
          <div style="display:flex;gap:8px;margin-bottom:16px;align-items:center">
            <label style="font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.tokenIdLabel')}</label>
            <input type="number" id="mem-token-id" value="0" min="0"
              style="width:80px;padding:6px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            <label style="font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.categoryLabel')}</label>
            <select id="mem-category"
              style="padding:6px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
              <option value="">${I18n.t('knowledge.allCategories')}</option>
              <option value="preference">${I18n.t('knowledge.preference')}</option>
              <option value="fact">${I18n.t('knowledge.fact')}</option>
              <option value="context">${I18n.t('knowledge.context')}</option>
              <option value="goal">${I18n.t('knowledge.goal')}</option>
              <option value="constraint">${I18n.t('knowledge.constraint')}</option>
            </select>
            <button class="btn btn-sm" onclick="KnowledgePage.loadMemories(1)">${I18n.t('common.query')}</button>
          </div>
          <div id="memories-table-container"></div>
        </div>
      </div>
    `;
    // 自动加载
    this.fetchMemoriesTable();
  },

  async fetchMemoriesTable() {
    const tokenId = document.getElementById('mem-token-id').value;
    const category = document.getElementById('mem-category').value;
    const container = document.getElementById('memories-table-container');
    container.innerHTML = `<p class="text-muted">${I18n.t('common.loading')}</p>`;
    try {
      let url = `/api/knowledge/memory_list?limit=50`;
      if (tokenId && tokenId !== '0') url += `&token_id=${tokenId}`;
      if (category) url += `&category=${category}`;
      const res = await fetch(url);
      const data = await res.json();
      const memories = data.memories || [];
      if (!memories.length) {
        container.innerHTML = `<div class="empty-state"><div class="icon">📭</div><p>${I18n.t('knowledge.noMemories')}</p></div>`;
        return;
      }
      container.innerHTML = `
        <table class="table">
          <thead><tr>
            <th>${I18n.t('common.id')}</th><th>${I18n.t('common.tokens')}</th><th>${I18n.t('common.category')}</th><th>${I18n.t('common.title')}</th><th>${I18n.t('common.content')}</th><th>${I18n.t('common.priority')}</th><th>${I18n.t('common.expiresAt')}</th><th>${I18n.t('common.actions')}</th>
          </tr></thead>
          <tbody>
            ${memories.map(m => `<tr>
              <td>${m.id}</td>
              <td class="text-sm">${m.token_name || '-'}</td>
              <td><span class="badge badge-${this.memCategoryColor(m.category)}">${m.category}</span></td>
              <td>${this.escapeHtml(m.title)}</td>
              <td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${this.escapeHtml(m.content)}">${this.escapeHtml(m.content.slice(0, 80))}${m.content.length > 80 ? '...' : ''}</td>
              <td>${'⭐'.repeat(m.priority || 3)}</td>
              <td class="text-sm text-muted">${m.expires_at ? m.expires_at.slice(0, 16) : I18n.t('common.permanent')}</td>
              <td>
                <button class="btn btn-sm" onclick="KnowledgePage.showMemoryDetail(${m.id})">${I18n.t('common.detail')}</button>
                <button class="btn btn-sm" style="color:var(--color-danger)" onclick="KnowledgePage.deleteMemory(${m.id})">${I18n.t('common.delete')}</button>
              </td>
            </tr>`).join('')}
          </tbody>
        </table>
        <p class="text-sm text-muted" style="padding:8px 0">${I18n.t('knowledge.totalCount')}${memories.length} ${I18n.t('knowledge.memoryUnit')}</p>
      `;
    } catch (e) {
      container.innerHTML = `<p style="color:var(--color-danger)">${I18n.t('common.loadFailed')}：${e.message}</p>`;
    }
  },

  memCategoryColor(c) {
    return { preference: 'info', fact: 'success', context: 'default', goal: 'warning', constraint: 'danger' }[c] || 'default';
  },

  async showMemoryDetail(id) {
    // 通过 memory_list API 查找单条记忆（遍历）
    const tokenId = document.getElementById('mem-token-id').value || '0';
    try {
      const res = await fetch(`/api/knowledge/memory_list?token_id=${tokenId}&limit=200`);
      const data = await res.json();
      const mem = (data.memories || []).find(m => m.id === id);
      if (!mem) { alert(I18n.t("knowledge.memoryNotFound")); return; }
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.memoryDetail')}${mem.id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadMemories(1)">${I18n.t('common.backToList')}</button>
          </div>
          <div style="padding:20px">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
              <p><b>${I18n.t('knowledge.tokenLabel')}</b> ${mem.token_name} (ID: ${mem.token_id})</p>
              <p><b>${I18n.t('knowledge.categoryLabel')}</b> <span class="badge badge-${this.memCategoryColor(mem.category)}">${mem.category}</span></p>
              <p><b>${I18n.t('knowledge.sessionLabel')}</b> ${mem.session_id || I18n.t('knowledge.global')}</p>
              <p><b>${I18n.t('knowledge.priorityLabel')}</b> ${'⭐'.repeat(mem.priority || 3)} (${mem.priority || 3}/5)</p>
              <p><b>${I18n.t('knowledge.createdAtLabel')}</b> ${mem.created_at ? mem.created_at.slice(0, 19) : '-'}</p>
              <p><b>${I18n.t('knowledge.expiresAtLabel')}</b> ${mem.expires_at ? mem.expires_at.slice(0, 19) : I18n.t('common.permanent')}</p>
            </div>
            <hr>
            <p><b>${I18n.t('knowledge.titleLabel')}</b> ${this.escapeHtml(mem.title)}</p>
            <p style="margin-top:8px"><b>${I18n.t('knowledge.contentLabel')}</b></p>
            <pre style="white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:13px">${this.escapeHtml(mem.content)}</pre>
            <p style="margin-top:8px"><b>${I18n.t('knowledge.tagsLabel')}</b> ${mem.tags || '[]'}</p>
            <hr style="margin:16px 0;border-color:var(--border)">
            <div style="display:flex;gap:8px">
              <button class="btn" onclick="KnowledgePage.showMemoryEdit(${mem.id})">${I18n.t('common.edit')}</button>
              <button class="btn" style="color:var(--color-danger)" onclick="KnowledgePage.deleteMemory(${mem.id})">${I18n.t('common.delete')}</button>
            </div>
          </div>
        </div>
      `;
    } catch (e) { alert(I18n.t('knowledge.loadDetailFailed') + e.message); }
  },

  async showMemoryEdit(id) {
    const tokenId = document.getElementById('mem-token-id').value || '0';
    try {
      const res = await fetch(`/api/knowledge/memory_list?token_id=${tokenId}&limit=200`);
      const data = await res.json();
      const mem = (data.memories || []).find(m => m.id === id);
      if (!mem) { alert(I18n.t("knowledge.memoryNotFound")); return; }
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.editMemory')}${id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.showMemoryDetail(${id})">${I18n.t('common.back')}</button>
          </div>
          <div style="padding:20px">
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('common.title')}</label>
              <input type="text" id="edit-mem-title" value="${this.escapeHtml(mem.title)}"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('common.content')}</label>
              <textarea id="edit-mem-content" rows="6"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">${this.escapeHtml(mem.content)}</textarea>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.priorityRange')}</label>
                <input type="number" id="edit-mem-priority" value="${mem.priority || 3}" min="1" max="5"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">${I18n.t('knowledge.expiresOptional')}</label>
                <input type="datetime-local" id="edit-mem-expires" value="${mem.expires_at ? mem.expires_at.slice(0, 16) : ''}"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
            </div>
            <button class="btn" onclick="KnowledgePage.saveMemoryEdit(${id})">${I18n.t('common.save')}</button>
          </div>
        </div>
      `;
    } catch (e) { alert(I18n.t("knowledge.loadEditFailed") + e.message); }
  },

  async saveMemoryEdit(id) {
    const title = document.getElementById('edit-mem-title').value;
    const content = document.getElementById('edit-mem-content').value;
    const priority = parseInt(document.getElementById('edit-mem-priority').value);
    const expiresAt = document.getElementById('edit-mem-expires').value || null;
    const tokenId = document.getElementById('mem-token-id').value || '0';
    if (!title || !content) { alert(I18n.t("knowledge.titleContentRequired")); return; }
    try {
      const body = { title, content, priority };
      if (expiresAt) body.expires_at = expiresAt;
      const res = await fetch(`/api/knowledge/memory/${id}?token_id=${tokenId}`, {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.error) { alert(I18n.t('common.saveFailed') + data.error); return; }
      alert(I18n.t("knowledge.saved"));
      this.showMemoryDetail(id);
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  async deleteMemory(id) {
    if (!confirm(I18n.t("knowledge.confirmDeleteMemory"))) return;
    const tokenId = document.getElementById('mem-token-id').value || '0';
    try {
      const res = await fetch(`/api/knowledge/memory/${id}?token_id=${tokenId}`, { method: 'DELETE' });
      const data = await res.json();
      if (data.error) { alert(I18n.t('common.deleteFailed') + data.error); return; }
      alert(I18n.t("knowledge.deleted"));
      this.fetchMemoriesTable();
    } catch (e) { alert(I18n.t("knowledge.requestFailed") + e.message); }
  },

  // ============================================================
  // 合规说明（v2.0 — 企业数据资产管理定性）
  // ============================================================
  renderCompliance() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div style="max-width:900px">
        <div class="card" style="margin-bottom:16px">
          <div class="card-header"><span class="card-title">${I18n.t('knowledge.complianceTitle')}</span></div>
          <div style="padding:20px;line-height:1.8">
            <p style="color:var(--text-muted);font-size:13px;margin-bottom:12px">${I18n.t('knowledge.complianceLegal')}</p>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceWhat')}</h4>
            <p>${I18n.t('knowledge.complianceWhatDetail')}</p>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceWhy')}</h4>
            <p>${I18n.t('knowledge.complianceWhyDetail')}</p>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceScope')}</h4>
            <ul style="margin:0;padding-left:20px">
              <li>${I18n.t('knowledge.complianceScope1')}</li>
              <li>${I18n.t('knowledge.complianceScope2')}</li>
              <li>${I18n.t('knowledge.complianceScope3')}</li>
              <li>${I18n.t('knowledge.complianceScope4')}</li>
            </ul>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceSecurity')}</h4>
            <ul style="margin:0;padding-left:20px">
              <li>${I18n.t('knowledge.complianceSecurity1')}</li>
              <li>${I18n.t('knowledge.complianceSecurity2')}</li>
              <li>${I18n.t('knowledge.complianceSecurity3')}</li>
            </ul>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceRetention')}</h4>
            <p>${I18n.t('knowledge.complianceRetentionDetail')}</p>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><span class="card-title">${I18n.t('knowledge.compliancePurpose')}</span></div>
          <div style="padding:20px;line-height:1.8">
            <p style="color:var(--text-muted);font-size:13px;margin-bottom:12px">${I18n.t('knowledge.compliancePurposeDesc')}</p>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceLegalUse')}</h4>
            <ul style="margin:0;padding-left:20px">
              <li>${I18n.t('knowledge.complianceLegal1')}</li>
              <li>${I18n.t('knowledge.complianceLegal2')}</li>
              <li>${I18n.t('knowledge.complianceLegal3')}</li>
              <li>${I18n.t('knowledge.complianceLegal4')}</li>
            </ul>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceProhibited')}</h4>
            <ul style="margin:0;padding-left:20px">
              <li>${I18n.t('knowledge.complianceProhibit1')}</li>
              <li>${I18n.t('knowledge.complianceProhibit2')}</li>
              <li>${I18n.t('knowledge.complianceProhibit3')}</li>
            </ul>

            <h4 style="margin:16px 0 8px">${I18n.t('knowledge.complianceAnalogy')}</h4>
            <p>${I18n.t('knowledge.complianceAnalogyDetail')}</p>

            <div style="margin-top:20px;padding:12px 16px;background:var(--bg-secondary);border-radius:8px;border-left:3px solid var(--color-warning)">
              <p style="font-size:13px;margin:0"><b>${I18n.t('knowledge.complianceObligation')}</b>${I18n.t('knowledge.complianceObligationDetail')}</p>
            </div>
          </div>
        </div>
      </div>
    `;
  },

  // ============================================================
  // 审计日志
  // ============================================================
  async loadAuditLog(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>${I18n.t('common.loading')}</p></div>`;
    try {
      const res = await fetch(`/api/knowledge/audit_log?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = `<div class="empty-state"><div class="icon">📭</div><p>${I18n.t('knowledge.noAuditLog')}</p></div>`;
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.auditLogWith', {total: data.total})}</span>
            <span class="text-sm text-muted">${I18n.t('common.pageOf', {page: data.page, total: Math.ceil(data.total / data.per_page) || 1})}</span>
          </div>
          <table class="table">
            <thead><tr>
              <th>${I18n.t('common.time')}</th><th>${I18n.t('knowledge.action')}</th><th>${I18n.t('knowledge.resourceType')}</th><th>${I18n.t('knowledge.resourceId')}</th><th>${I18n.t('common.tokens')}</th><th>${I18n.t('common.detail')}</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td class="text-sm text-muted">${i.created_at ? i.created_at.slice(0, 19) : '-'}</td>
                <td><span class="badge badge-${this.auditActionColor(i.action)}">${this.translateAction(i.action)}</span></td>
                <td>${i.resource_type || '-'}</td>
                <td><code class="text-sm">${this.escapeHtml(i.resource_id || '-')}</code></td>
                <td>${i.token_id || '-'}</td>
                <td><button class="btn btn-sm" onclick="KnowledgePage.showAuditDetail(${i.id})">${I18n.t('common.view')}</button></td>
              </tr>`).join('')}
            </tbody>
          </table>
          <div style="padding:12px 20px;display:flex;gap:8px;justify-content:center">
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadAuditLog(${data.page - 1})">${I18n.t('common.prevPage')}</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadAuditLog(${data.page + 1})">${I18n.t('common.nextPage')}</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>${I18n.t('common.loadFailed')}：${e.message}</p></div>`;
    }
  },

  auditActionColor(action) {
    return {
      knowledge_capture: 'info',
      knowledge_extract: 'success',
      knowledge_access: 'default',
      config_change: 'warning',
      data_delete: 'danger',
      raw_cleanup: 'default',
      retention_cleanup: 'default',
    }[action] || 'default';
  },

  translateAction(action) {
    return {
      knowledge_capture: I18n.t("knowledge.actionCapture"),
      knowledge_extract: I18n.t("knowledge.extract"),
      knowledge_access: I18n.t("knowledge.actionAccess"),
      config_change: I18n.t("knowledge.actionConfigChange"),
      data_delete: I18n.t("knowledge.actionDataDelete"),
      raw_cleanup: I18n.t("knowledge.actionRawCleanup"),
      retention_cleanup: I18n.t("knowledge.actionRetentionCleanup"),
    }[action] || action;
  },

  async showAuditDetail(id) {
    try {
      const res = await fetch(`/api/knowledge/audit_log/${id}`);
      if (res.status === 404) { alert(I18n.t("knowledge.auditLogNotFound")); return; }
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const item = await res.json();
      const el = document.getElementById('knowledge-tab-content');
      let detailStr = '-';
      if (item.detail) {
        detailStr = typeof item.detail === 'string' ? item.detail : JSON.stringify(item.detail, null, 2);
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">${I18n.t('knowledge.auditDetail')}${item.id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadAuditLog(1)">${I18n.t('common.backToList')}</button>
          </div>
          <div style="padding:20px">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
              <p><b>${I18n.t('knowledge.timeLabel')}</b> ${item.created_at ? item.created_at.slice(0, 19) : '-'}</p>
              <p><b>${I18n.t('knowledge.actionLabel')}</b> <span class="badge badge-${this.auditActionColor(item.action)}">${this.translateAction(item.action)}</span></p>
              <p><b>${I18n.t('knowledge.resourceTypeLabel')}</b> ${item.resource_type || '-'}</p>
              <p><b>${I18n.t('knowledge.resourceIdLabel')}</b> <code>${this.escapeHtml(item.resource_id || '-')}</code></p>
              <p><b>${I18n.t('knowledge.tokenIdLabel')}</b> ${item.token_id || '-'}</p>
              <p><b>${I18n.t('knowledge.clientIpLabel')}</b> ${item.client_ip || '-'}</p>
            </div>
            <hr>
            <p><b>${I18n.t('knowledge.detailLabel')}</b></p>
            <pre style="max-height:400px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(detailStr)}</pre>
          </div>
        </div>
      `;
    } catch (e) { alert(I18n.t('common.loadFailed') + '：' + e.message); }
  },


  // ============================================================
  // 搜索质量指标
  // ============================================================
  async loadQuality() {
    const container = document.getElementById('knowledge-tab-content');
    container.innerHTML = '<div class="loading">' + I18n.t('common.loading') + '</div>';

    try {
      const res = await fetch('/api/knowledge/search-quality');
      const data = await res.json();

      let html = '<div style="padding:20px">';

      // 概览卡片
      html += '<div class="stats-grid" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px;margin-bottom:24px">';
      html += '<div class="stat-card"><div class="stat-value">' + data.total_items + '</div><div class="stat-label">' + I18n.t('knowledge.totalCount') + '</div></div>';
      html += '<div class="stat-card"><div class="stat-value">' + data.verified_items + '</div><div class="stat-label">' + I18n.t('knowledge.verification') + '</div></div>';
      html += '<div class="stat-card"><div class="stat-value">' + data.verification_rate + '%</div><div class="stat-label">' + I18n.t('knowledge.mrr') + '</div></div>';
      html += '</div>';

      // 按领域质量表格
      html += '<h3 style="margin:20px 0 12px">' + I18n.t('knowledge.qualityByDomain') + '</h3>';
      html += '<table class="data-table"><thead><tr>';
      html += '<th>' + I18n.t('knowledge.code') + '</th>';
      html += '<th>' + I18n.t('knowledge.totalCount') + '</th>';
      html += '<th>' + I18n.t('knowledge.verification') + '</th>';
      html += '<th>' + I18n.t('knowledge.verified') + '</th>';
      html += '<th>' + I18n.t('knowledge.confidence') + '</th>';
      html += '<th>' + I18n.t('knowledge.mrr') + '</th>';
      html += '</tr></thead><tbody>';

      if (data.by_domain && data.by_domain.length > 0) {
        data.by_domain.forEach(function(d) {
          html += '<tr>';
          html += '<td><code>' + d.domain_code + '</code></td>';
          html += '<td>' + d.total + '</td>';
          html += '<td>' + (d.verified || 0) + '</td>';
          html += '<td>' + d.verified_rate + '%</td>';
          html += '<td>' + d.avg_confidence + '</td>';
          html += '<td>' + (d.verified / d.total).toFixed(3) + '</td>';
          html += '</tr>';
        });
      } else {
        html += '<tr><td colspan="6" style="text-align:center;color:var(--text-muted)">' + I18n.t('common.noData') + '</td></tr>';
      }
      html += '</tbody></table>';

      // 置信度分布
      if (data.confidence_distribution) {
        html += '<h3 style="margin:24px 0 12px">' + I18n.t('knowledge.confidenceDist') + '</h3>';
        html += '<div style="max-width:400px"><canvas id="quality-confidence-chart" height="200"></canvas></div>';
      }

      html += '</div>';
      container.innerHTML = html;

      // 渲染置信度分布图
      if (data.confidence_distribution) {
        setTimeout(function() {
          const ctx = document.getElementById('quality-confidence-chart');
          if (!ctx) return;
          const dist = data.confidence_distribution;
          new Chart(ctx, {
            type: 'doughnut',
            data: {
              labels: ['high', 'medium', 'low'],
              datasets: [{
                data: [(dist.high || 0), (dist.medium || 0), (dist.low || 0)],
                backgroundColor: ['#10b981', '#f59e0b', '#ef4444']
              }]
            },
            options: {
              responsive: true,
              plugins: {
                legend: { position: 'bottom' }
              }
            }
          });
        }, 100);
      }
    } catch (e) {
      container.innerHTML = '<div class="error-message">' + I18n.t('common.loadFailed') + ': ' + (e.message || '') + '</div>';
    }
  },

    escapeHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  },
};
