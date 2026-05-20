/* 企业知识库管理页面 */
const KnowledgePage = {
  activeTab: 'stats',

  load() {
    this.render();
    this.loadStats();
  },

  render() {
    const el = document.getElementById('knowledge-page-content');
    el.innerHTML = `
      <div class="page-header">
        <span class="page-title">企业知识库</span>
        <div style="display:flex;gap:8px;">
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('stats')">捕获统计</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('raw')">原始对话</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('items')">知识条目</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('reviews')">审核队列</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('domains')">业务域</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('analyze')">单域分析</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('analyses')">分析记录</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('extract')">知识提取</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('search')">搜索</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('memories')">记忆管理</button>
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
            <div class="stat-label">原始对话总数</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color:#f59e0b">${data.raw.pending}</div>
            <div class="stat-label">待处理</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color:#10b981">${data.raw.done}</div>
            <div class="stat-label">已处理</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${data.raw.today}</div>
            <div class="stat-label">今日新增</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color:#8b5cf6">${data.items.total}</div>
            <div class="stat-label">知识条目</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${data.domains.total}</div>
            <div class="stat-label">业务域</div>
          </div>
        </div>
        <div class="card" style="margin-top:16px">
          <div class="card-header"><span class="card-title">知识条目分布</span></div>
          <div style="padding:20px">
            <p>按类型：${this.objToText(data.items.by_type || {})}</p>
            <p style="margin-top:8px">按验证状态：${this.objToText(data.items.by_verification || {})}</p>
          </div>
        </div>
      `;
    } catch (e) {
      document.getElementById('knowledge-tab-content').innerHTML =
        `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
    }
  },

  objToText(obj) {
    const entries = Object.entries(obj);
    if (!entries.length) return '无数据';
    return entries.map(([k, v]) => `${k}: ${v}`).join('、');
  },

  // ============================================================
  // 原始对话
  // ============================================================
  async loadRaw(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = '<div class="empty-state"><div class="icon">⏳</div><p>加载中...</p></div>';
    try {
      const res = await fetch(`/api/knowledge/raw?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = '<div class="empty-state"><div class="icon">📭</div><p>暂无原始对话数据</p></div>';
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">原始对话 (${data.total} 条)</span>
            <span class="text-sm text-muted">第 ${data.page} 页 / 共 ${Math.ceil(data.total / data.per_page) || 1} 页</span>
          </div>
          <table class="table">
            <thead><tr>
              <th>ID</th><th>Token</th><th>模型</th><th>轮数</th><th>状态</th><th>时间</th><th>操作</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td>${i.id}</td>
                <td>${i.token_name || '-'}</td>
                <td><code>${i.model_name}</code></td>
                <td>${i.turn_count}</td>
                <td><span class="badge badge-${this.statusColor(i.status)}">${i.status}</span></td>
                <td class="text-sm text-muted">${i.created_at ? i.created_at.slice(0, 19) : '-'}</td>
                <td><button class="btn btn-sm" onclick="KnowledgePage.showRawDetail(${i.id})">查看</button></td>
              </tr>`).join('')}
            </tbody>
          </table>
          <div style="padding:12px 20px;display:flex;gap:8px;justify-content:center">
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadRaw(${data.page - 1})">上一页</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadRaw(${data.page + 1})">下一页</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
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
            <span class="card-title">原始对话详情 #${d.id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadRaw(1)">返回列表</button>
          </div>
          <div style="padding:20px">
            <p><b>Token:</b> ${d.token_name} (ID: ${d.token_id})</p>
            <p><b>模型:</b> <code>${d.model_name}</code></p>
            <p><b>状态:</b> <span class="badge badge-${this.statusColor(d.status)}">${d.status}</span></p>
            <p><b>对话轮数:</b> ${d.turn_count}</p>
            <p><b>时间:</b> ${d.created_at ? d.created_at.slice(0, 19) : '-'}</p>
            <hr>
            <p><b>Prompt:</b></p>
            <pre style="max-height:200px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(d.prompt)}</pre>
            <p><b>Response:</b></p>
            <pre style="max-height:300px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(d.response)}</pre>
          </div>
        </div>
      `;
    } catch (e) {
      alert('加载详情失败: ' + e.message);
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
    const deptFilter = departments.length ? `<label style="font-size:12px;color:var(--text-muted)">部门:</label>
      <select id="items-dept-filter" onchange="KnowledgePage.loadItems(1)" style="padding:6px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
        <option value="">全部</option>
        ${departments.map(d => `<option value="${d}">${d}</option>`).join('')}
      </select>` : '';
    el.innerHTML = `<div class="empty-state"><div class="icon">⏳</div><p>加载中...</p></div>`;
    try {
      const dept = document.getElementById('items-dept-filter')?.value || '';
      const res = await fetch(`/api/knowledge/items?page=${page}&per_page=20${dept ? '&department=' + encodeURIComponent(dept) : ''}`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = '<div class="empty-state"><div class="icon">📭</div><p>暂无知识条目</p></div>';
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">知识条目 (${data.total} 条)</span>
            ${deptFilter ? '<div style="display:flex;gap:8px;align-items:center">' + deptFilter + '</div>' : ''}
          </div>
          <table class="table">
            <thead><tr>
              <th>ID</th><th>类型</th><th>标题</th><th>部门</th><th>领域</th><th>置信度</th><th>验证</th><th>时间</th>
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
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadItems(${data.page - 1})">上一页</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadItems(${data.page + 1})">下一页</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
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
    el.innerHTML = '<div class="empty-state"><div class="icon">⏳</div><p>加载中...</p></div>';
    try {
      const res = await fetch(`/api/knowledge/reviews?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = '<div class="empty-state"><div class="icon">✅</div><p>审核队列为空，所有知识条目已审核完毕</p></div>';
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">审核队列 (${data.total} 条待审)</span>
            <button class="btn btn-sm" onclick="KnowledgePage.batchApprove()" style="color:#10b981">批量通过</button>
          </div>
          <table class="table">
            <thead><tr>
              <th><input type="checkbox" id="review-select-all" onchange="KnowledgePage.toggleAllReviews(this.checked)"/></th>
              <th>ID</th><th>类型</th><th>标题</th><th>领域</th><th>置信度</th><th>时间</th><th>操作</th>
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
                  <button class="btn btn-sm" onclick="KnowledgePage.showReviewEdit(${i.id})">编辑</button>
                  <button class="btn btn-sm" style="color:#10b981" onclick="KnowledgePage.approveItem(${i.id})">通过</button>
                  <button class="btn btn-sm" style="color:var(--color-danger)" onclick="KnowledgePage.rejectItem(${i.id})">拒绝</button>
                </td>
              </tr>`).join('')}
            </tbody>
          </table>
          <div style="padding:12px 20px;display:flex;gap:8px;justify-content:center">
            ${data.page > 1 ? `<button class="btn btn-sm" onclick="KnowledgePage.loadReviews(${data.page - 1})">上一页</button>` : ''}
            ${data.page < Math.ceil(data.total / data.per_page) ? `<button class="btn btn-sm" onclick="KnowledgePage.loadReviews(${data.page + 1})">下一页</button>` : ''}
          </div>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
    }
  },

  toggleAllReviews(checked) {
    document.querySelectorAll('.review-checkbox').forEach(cb => { cb.checked = checked; });
  },

  getSelectedReviewIds() {
    return Array.from(document.querySelectorAll('.review-checkbox:checked')).map(cb => parseInt(cb.value));
  },

  async approveItem(id) {
    if (!confirm('确认通过该知识条目？')) return;
    try {
      const res = await fetch(`/api/knowledge/reviews/${id}/approve`, { method: 'POST' });
      const data = await res.json();
      if (data.error) { alert('审核失败：' + data.error); return; }
      alert('已通过');
      this.loadReviews(1);
    } catch (e) { alert('请求失败：' + e.message); }
  },

  async rejectItem(id) {
    if (!confirm('确认拒绝该知识条目？')) return;
    try {
      const res = await fetch(`/api/knowledge/reviews/${id}/reject`, { method: 'POST' });
      const data = await res.json();
      if (data.error) { alert('操作失败：' + data.error); return; }
      alert('已拒绝');
      this.loadReviews(1);
    } catch (e) { alert('请求失败：' + e.message); }
  },

  async showReviewEdit(id) {
    try {
      const res = await fetch(`/api/knowledge/items/${id}`);
      const item = await res.json();
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">编辑知识条目 #${id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadReviews(1)">返回</button>
          </div>
          <div style="padding:20px">
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">标题</label>
              <input type="text" id="edit-item-title" value="${this.escapeHtml(item.title)}"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">摘要</label>
              <textarea id="edit-item-summary" rows="4"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">${this.escapeHtml(item.summary)}</textarea>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;margin-bottom:16px">
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">类型</label>
                <select id="edit-item-type" style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                  <option value="factual" ${item.type==='factual'?'selected':''}>factual</option>
                  <option value="analytical" ${item.type==='analytical'?'selected':''}>analytical</option>
                  <option value="procedural" ${item.type==='procedural'?'selected':''}>procedural</option>
                </select>
              </div>
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">置信度</label>
                <input type="number" id="edit-item-confidence" value="${item.confidence}" min="0" max="1" step="0.01"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">领域</label>
                <input type="text" id="edit-item-domain" value="${item.domain_code || ''}"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
            </div>
            <hr style="margin:16px 0;border-color:var(--border)"/>
            <p style="font-size:12px;color:var(--text-muted);margin-bottom:8px">原始引用：</p>
            <pre style="max-height:200px;overflow:auto;white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:12px">${this.escapeHtml(item.source_quote)}</pre>
            <div style="margin-top:16px;display:flex;gap:8px">
              <button class="btn" onclick="KnowledgePage.saveReviewEdit(${id})">保存</button>
              <button class="btn" onclick="KnowledgePage.loadReviews(1)">取消</button>
            </div>
          </div>
        </div>
      `;
    } catch (e) { alert('加载失败：' + e.message); }
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
      if (data.error) { alert('保存失败：' + data.error); return; }
      alert('已保存');
      this.loadReviews(1);
    } catch (e) { alert('请求失败：' + e.message); }
  },

  async batchApprove() {
    const ids = this.getSelectedReviewIds();
    if (!ids.length) { alert('请先勾选要审核通过的条目'); return; }
    if (!confirm(`确认批量通过 ${ids.length} 条知识？`)) return;
    try {
      const res = await fetch('/api/knowledge/reviews/batch-approve', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ ids }),
      });
      const data = await res.json();
      if (data.error) { alert('批量审核失败：' + data.error); return; }
      alert(data.message || '批量通过成功');
      this.loadReviews(1);
    } catch (e) { alert('请求失败：' + e.message); }
  },

  // ============================================================
  // 业务域管理
  // ============================================================
  async loadDomains() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = '<div class="empty-state"><div class="icon">⏳</div><p>加载中...</p></div>';
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
            <span class="card-title">业务域 (${domData.total})</span>
            <button class="btn btn-sm" onclick="KnowledgePage.showCreateDomain()">新增业务域</button>
          </div>
          <table class="table">
            <thead><tr>
              <th>代码</th><th>名称</th><th>部门</th><th>状态</th><th>知识条数</th><th>风险等级</th><th>最小验证</th><th>操作</th>
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
                    ${isPending ? `<button class="btn btn-sm" style="color:#10b981" onclick="KnowledgePage.confirmDomain('${d.domain_code}')">确认</button>` : ''}
                    ${!isPending && !isMerged ? `<button class="btn btn-sm" onclick="KnowledgePage.showDomainStats('${d.domain_code}')">统计</button>
                    <button class="btn btn-sm" onclick="KnowledgePage.showMergeDomain('${d.domain_code}')">合并</button>` : ''}
                    ${isMerged ? `<span class="text-sm text-muted">已合并</span>` : ''}
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
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
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
    if (!confirm(`确认将业务域 "${code}" 标记为 active？`)) return;
    try {
      const res = await fetch(`/api/knowledge/domains/${code}/confirm`, { method: 'POST' });
      const data = await res.json();
      if (data.error) { alert('操作失败：' + data.error); return; }
      alert('已确认');
      this.loadDomains();
    } catch (e) { alert('请求失败：' + e.message); }
  },

  async showCreateDomain() {
    const panel = document.getElementById('domain-action-panel');
    panel.innerHTML = `
      <div class="card">
        <div class="card-header">
          <span class="card-title">新增业务域</span>
          <button class="btn btn-sm" onclick="document.getElementById('domain-action-panel').innerHTML=''">关闭</button>
        </div>
        <div style="padding:20px">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">域代码 *</label>
              <input type="text" id="new-domain-code" placeholder="如: legal"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">域名称 *</label>
              <input type="text" id="new-domain-name" placeholder="如: 法务部"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
          </div>
          <div style="margin-bottom:16px">
            <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">部门</label>
            <input type="text" id="new-domain-dept" placeholder="如: 法务合规部"
              style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
          </div>
          <div style="margin-bottom:16px">
            <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">描述</label>
            <textarea id="new-domain-desc" rows="2"
              style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"></textarea>
          </div>
          <button class="btn" onclick="KnowledgePage.createDomain()">创建</button>
        </div>
      </div>
    `;
  },

  async createDomain() {
    const code = document.getElementById('new-domain-code').value.trim();
    const name = document.getElementById('new-domain-name').value.trim();
    const dept = document.getElementById('new-domain-dept').value.trim();
    const desc = document.getElementById('new-domain-desc').value.trim();
    if (!code || !name) { alert('域代码和名称不能为空'); return; }
    try {
      const res = await fetch('/api/knowledge/domains', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ domain_code: code, domain_name: name, department: dept, description: desc }),
      });
      const data = await res.json();
      if (data.error) { alert('创建失败：' + data.error); return; }
      alert('创建成功');
      this.loadDomains();
    } catch (e) { alert('请求失败：' + e.message); }
  },

  async showDomainStats(code) {
    const panel = document.getElementById('domain-action-panel');
    panel.innerHTML = '<div class="card"><div style="padding:20px"><p class="text-muted">加载中...</p></div></div>';
    try {
      const res = await fetch(`/api/knowledge/domains/${code}/stats`);
      const data = await res.json();
      if (data.error) { panel.innerHTML = `<div class="card"><div style="padding:20px"><p style="color:var(--color-danger)">${data.error}</p></div></div>`; return; }
      const d = data.domain;
      const ibt = Object.entries(data.items.by_type || {}).map(([k,v]) => `${k}: ${v}`).join('、') || '无';
      const ibv = Object.entries(data.items.by_verification || {}).map(([k,v]) => `${k}: ${v}`).join('、') || '无';
      panel.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">业务域统计 — ${d.domain_name} (${code})</span>
            <button class="btn btn-sm" onclick="document.getElementById('domain-action-panel').innerHTML=''">关闭</button>
          </div>
          <div style="padding:20px">
            <div class="stats-grid" style="margin-bottom:16px">
              <div class="stat-card"><div class="stat-value">${data.items.total}</div><div class="stat-label">知识条目</div></div>
              <div class="stat-card"><div class="stat-value" style="color:#f59e0b">${data.raw_pending}</div><div class="stat-label">待处理 Raw</div></div>
              <div class="stat-card"><div class="stat-value">${d.sample_count || 0}</div><div class="stat-label">样本数</div></div>
            </div>
            <p><b>按类型：</b>${ibt}</p>
            <p style="margin-top:8px"><b>按验证状态：</b>${ibv}</p>
            <p style="margin-top:8px"><b>状态：</b><span class="badge badge-${d.status === 'active' ? 'success' : 'warning'}">${d.status}</span></p>
          </div>
        </div>
      `;
    } catch (e) { panel.innerHTML = `<div class="card"><div style="padding:20px"><p style="color:var(--color-danger)">加载失败：${e.message}</p></div></div>`; }
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
            <span class="card-title">合并业务域 — 将 "${code}" 合并到</span>
            <button class="btn btn-sm" onclick="document.getElementById('domain-action-panel').innerHTML=''">关闭</button>
          </div>
          <div style="padding:20px">
            <p class="text-muted" style="margin-bottom:12px">合并后，"${code}" 下的所有知识条目将迁移到目标域，当前域将被标记为 merged。</p>
            <div style="display:flex;gap:8px;align-items:center;margin-bottom:16px">
              <select id="merge-target" style="padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                <option value="">请选择目标域</option>
                ${options}
              </select>
              <button class="btn" onclick="KnowledgePage.mergeDomain('${code}')">确认合并</button>
            </div>
          </div>
        </div>
      `;
    } catch (e) { alert('加载域列表失败：' + e.message); }
  },

  async mergeDomain(code) {
    const target = document.getElementById('merge-target').value;
    if (!target) { alert('请选择目标域'); return; }
    if (!confirm(`确认将 "${code}" 合并到 "${target}"？此操作不可撤销。`)) return;
    try {
      const res = await fetch(`/api/knowledge/domains/${code}/merge`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ target_code: target }),
      });
      const data = await res.json();
      if (data.error) { alert('合并失败：' + data.error); return; }
      alert(data.message || '合并成功');
      this.loadDomains();
    } catch (e) { alert('请求失败：' + e.message); }
  },

  // ============================================================
  // 知识提取
  // ============================================================
  renderExtract() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">LLM 知识提取</span></div>
        <div style="padding:20px">
          <p class="text-muted" style="margin-bottom:16px">
            从原始对话（wr_knowledge_raw）中自动提取结构化知识条目。<br>
            LLM 将评估每条对话的知识价值，并提取为 factual/analytical/procedural 类型知识。
          </p>
          <div style="display:flex;gap:8px;margin-bottom:16px">
            <input type="number" id="extract-batch-size" value="5" min="1" max="20"
              style="width:80px;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"
              placeholder="批量大小"/>
            <span style="line-height:36px;color:var(--text-muted)">条/批</span>
            <button class="btn" onclick="KnowledgePage.runExtract()">开始提取</button>
          </div>
          <div id="extract-result"></div>
        </div>
      </div>
    `;
  },

  async runExtract() {
    const batchSize = parseInt(document.getElementById('extract-batch-size').value) || 5;
    const el = document.getElementById('extract-result');
    el.innerHTML = '<p class="text-muted">提取中，请稍候（可能需要几分钟）...</p>';
    try {
      const res = await fetch('/api/knowledge/extract', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({batch_size: batchSize}),
      });
      const data = await res.json();
      if (data.error) {
        el.innerHTML = `<p style="color:var(--color-danger)">提取失败：${data.error}</p>`;
        return;
      }
      el.innerHTML = `
        <div style="padding:16px;background:var(--bg-secondary);border-radius:8px">
          <p><b>提取完成</b></p>
          <p>处理条数：${data.processed || 0}</p>
          <p>耗时：${data.duration_ms || 0}ms</p>
          <p>${data.message || ''}</p>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<p style="color:var(--color-danger)">请求失败：${e.message}</p>`;
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
        <div class="card-header"><span class="card-title">单域分析</span></div>
        <div style="padding:20px">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">业务域 *</label>
              <select id="analyze-domain" style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                <option value="">请选择业务域</option>
                ${domains.map(d => `<option value="${d.domain_code}">${d.domain_name} (${d.domain_code})</option>`).join('')}
              </select>
            </div>
            <div>
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">分析类型</label>
              <select id="analyze-type" style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
                <option value="domain_overview">领域概览</option>
                <option value="trend">趋势分析</option>
                <option value="gap">知识缺口分析</option>
              </select>
            </div>
          </div>
          <button class="btn" onclick="KnowledgePage.runAnalysis()">开始分析</button>
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
      el.innerHTML = '<p style="color:var(--color-danger)">请选择业务域</p>';
      return;
    }

    el.innerHTML = '<p class="text-muted">分析中，请稍候...</p>';
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
        el.innerHTML = `<p style="color:var(--color-danger)">分析失败：${data.error}</p>`;
        return;
      }
      // 简单 markdown 渲染
      const result = (data.result || '').replace(/\n/g, '<br>').replace(/\*\*(.+?)\*\*/g, '<b>$1</b>');
      el.innerHTML = `<div style="padding:16px;background:var(--bg-secondary);border-radius:8px;white-space:pre-wrap;line-height:1.8">${result}</div>`;
    } catch (e) {
      el.innerHTML = `<p style="color:var(--color-danger)">请求失败：${e.message}</p>`;
    }
  },

  // ============================================================
  // 分析记录
  // ============================================================
  async loadAnalyses(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = '<div class="empty-state"><div class="icon">⏳</div><p>加载中...</p></div>';
    try {
      const res = await fetch(`/api/knowledge/analyses?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = '<div class="empty-state"><div class="icon">📭</div><p>暂无分析记录</p></div>';
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header"><span class="card-title">分析记录 (${data.total})</span></div>
          <table class="table">
            <thead><tr>
              <th>任务ID</th><th>领域</th><th>类型</th><th>条数</th><th>状态</th><th>时间</th>
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
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
    }
  },

  // ============================================================
  // 搜索
  // ============================================================
  renderSearch() {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">知识搜索</span></div>
        <div style="padding:20px">
          <div style="display:flex;gap:8px;margin-bottom:16px">
            <input type="text" id="knowledge-search-input" placeholder="输入关键词搜索..."
              style="flex:1;padding:8px 12px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"
              onkeydown="if(event.key==='Enter')KnowledgePage.doSearch()"/>
            <button class="btn" onclick="KnowledgePage.doSearch()">搜索</button>
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
      el.innerHTML = '<p class="text-muted">请输入搜索关键词</p>';
      return;
    }
    el.innerHTML = '<p class="text-muted">搜索中...</p>';
    try {
      const res = await fetch(`/api/knowledge/search?q=${encodeURIComponent(q)}`);
      const data = await res.json();
      if (data.items.total === 0 && data.raw_count === 0) {
        el.innerHTML = '<p class="text-muted">未找到匹配结果</p>';
        return;
      }
      let html = `<p class="text-sm text-muted">找到 ${data.items.total} 条知识，${data.raw_count} 条原始对话</p>`;
      if (data.items.results.length) {
        html += '<table class="table"><thead><tr><th>标题</th><th>类型</th><th>领域</th><th>置信度</th></tr></thead><tbody>';
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
      el.innerHTML = `<p style="color:var(--color-danger)">搜索失败：${e.message}</p>`;
    }
  },

  // ============================================================
  // 记忆管理
  // ============================================================
  async loadMemories(page = 1) {
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = `
      <div class="card">
        <div class="card-header"><span class="card-title">记忆管理</span></div>
        <div style="padding:20px">
          <div style="display:flex;gap:8px;margin-bottom:16px;align-items:center">
            <label style="font-size:12px;color:var(--text-muted)">Token ID:</label>
            <input type="number" id="mem-token-id" value="0" min="0"
              style="width:80px;padding:6px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            <label style="font-size:12px;color:var(--text-muted)">分类:</label>
            <select id="mem-category"
              style="padding:6px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">
              <option value="">全部分类</option>
              <option value="preference">偏好 (preference)</option>
              <option value="fact">事实 (fact)</option>
              <option value="context">上下文 (context)</option>
              <option value="goal">目标 (goal)</option>
              <option value="constraint">约束 (constraint)</option>
            </select>
            <button class="btn btn-sm" onclick="KnowledgePage.loadMemories(1)">查询</button>
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
    container.innerHTML = '<p class="text-muted">加载中...</p>';
    try {
      let url = `/api/knowledge/memory_list?limit=50`;
      if (tokenId && tokenId !== '0') url += `&token_id=${tokenId}`;
      if (category) url += `&category=${category}`;
      const res = await fetch(url);
      const data = await res.json();
      const memories = data.memories || [];
      if (!memories.length) {
        container.innerHTML = '<div class="empty-state"><div class="icon">📭</div><p>暂无记忆数据</p></div>';
        return;
      }
      container.innerHTML = `
        <table class="table">
          <thead><tr>
            <th>ID</th><th>Token</th><th>分类</th><th>标题</th><th>内容</th><th>优先级</th><th>过期时间</th><th>操作</th>
          </tr></thead>
          <tbody>
            ${memories.map(m => `<tr>
              <td>${m.id}</td>
              <td class="text-sm">${m.token_name || '-'}</td>
              <td><span class="badge badge-${this.memCategoryColor(m.category)}">${m.category}</span></td>
              <td>${this.escapeHtml(m.title)}</td>
              <td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${this.escapeHtml(m.content)}">${this.escapeHtml(m.content.slice(0, 80))}${m.content.length > 80 ? '...' : ''}</td>
              <td>${'⭐'.repeat(m.priority || 3)}</td>
              <td class="text-sm text-muted">${m.expires_at ? m.expires_at.slice(0, 16) : '永久'}</td>
              <td>
                <button class="btn btn-sm" onclick="KnowledgePage.showMemoryDetail(${m.id})">详情</button>
                <button class="btn btn-sm" style="color:var(--color-danger)" onclick="KnowledgePage.deleteMemory(${m.id})">删除</button>
              </td>
            </tr>`).join('')}
          </tbody>
        </table>
        <p class="text-sm text-muted" style="padding:8px 0">共 ${memories.length} 条记忆</p>
      `;
    } catch (e) {
      container.innerHTML = `<p style="color:var(--color-danger)">加载失败：${e.message}</p>`;
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
      if (!mem) { alert('记忆条目未找到'); return; }
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">记忆详情 #${mem.id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.loadMemories(1)">返回列表</button>
          </div>
          <div style="padding:20px">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
              <p><b>Token:</b> ${mem.token_name} (ID: ${mem.token_id})</p>
              <p><b>分类:</b> <span class="badge badge-${this.memCategoryColor(mem.category)}">${mem.category}</span></p>
              <p><b>会话:</b> ${mem.session_id || '全局'}</p>
              <p><b>优先级:</b> ${'⭐'.repeat(mem.priority || 3)} (${mem.priority || 3}/5)</p>
              <p><b>创建时间:</b> ${mem.created_at ? mem.created_at.slice(0, 19) : '-'}</p>
              <p><b>过期时间:</b> ${mem.expires_at ? mem.expires_at.slice(0, 19) : '永久'}</p>
            </div>
            <hr>
            <p><b>标题:</b> ${this.escapeHtml(mem.title)}</p>
            <p style="margin-top:8px"><b>内容:</b></p>
            <pre style="white-space:pre-wrap;background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:13px">${this.escapeHtml(mem.content)}</pre>
            <p style="margin-top:8px"><b>标签:</b> ${mem.tags || '[]'}</p>
            <hr style="margin:16px 0;border-color:var(--border)">
            <div style="display:flex;gap:8px">
              <button class="btn" onclick="KnowledgePage.showMemoryEdit(${mem.id})">编辑</button>
              <button class="btn" style="color:var(--color-danger)" onclick="KnowledgePage.deleteMemory(${mem.id})">删除</button>
            </div>
          </div>
        </div>
      `;
    } catch (e) { alert('加载详情失败：' + e.message); }
  },

  async showMemoryEdit(id) {
    const tokenId = document.getElementById('mem-token-id').value || '0';
    try {
      const res = await fetch(`/api/knowledge/memory_list?token_id=${tokenId}&limit=200`);
      const data = await res.json();
      const mem = (data.memories || []).find(m => m.id === id);
      if (!mem) { alert('记忆条目未找到'); return; }
      const el = document.getElementById('knowledge-tab-content');
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">编辑记忆 #${id}</span>
            <button class="btn btn-sm" onclick="KnowledgePage.showMemoryDetail(${id})">返回</button>
          </div>
          <div style="padding:20px">
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">标题</label>
              <input type="text" id="edit-mem-title" value="${this.escapeHtml(mem.title)}"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
            </div>
            <div style="margin-bottom:12px">
              <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">内容</label>
              <textarea id="edit-mem-content" rows="6"
                style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)">${this.escapeHtml(mem.content)}</textarea>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px">
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">优先级 (1-5)</label>
                <input type="number" id="edit-mem-priority" value="${mem.priority || 3}" min="1" max="5"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
              <div>
                <label style="display:block;margin-bottom:4px;font-size:12px;color:var(--text-muted)">过期时间（可选）</label>
                <input type="datetime-local" id="edit-mem-expires" value="${mem.expires_at ? mem.expires_at.slice(0, 16) : ''}"
                  style="width:100%;padding:8px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-primary)"/>
              </div>
            </div>
            <button class="btn" onclick="KnowledgePage.saveMemoryEdit(${id})">保存</button>
          </div>
        </div>
      `;
    } catch (e) { alert('加载编辑失败：' + e.message); }
  },

  async saveMemoryEdit(id) {
    const title = document.getElementById('edit-mem-title').value;
    const content = document.getElementById('edit-mem-content').value;
    const priority = parseInt(document.getElementById('edit-mem-priority').value);
    const expiresAt = document.getElementById('edit-mem-expires').value || null;
    const tokenId = document.getElementById('mem-token-id').value || '0';
    if (!title || !content) { alert('标题和内容不能为空'); return; }
    try {
      const body = { title, content, priority };
      if (expiresAt) body.expires_at = expiresAt;
      const res = await fetch(`/api/knowledge/memory/${id}?token_id=${tokenId}`, {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.error) { alert('保存失败：' + data.error); return; }
      alert('已保存');
      this.showMemoryDetail(id);
    } catch (e) { alert('请求失败：' + e.message); }
  },

  async deleteMemory(id) {
    if (!confirm('确认删除该记忆条目？')) return;
    const tokenId = document.getElementById('mem-token-id').value || '0';
    try {
      const res = await fetch(`/api/knowledge/memory/${id}?token_id=${tokenId}`, { method: 'DELETE' });
      const data = await res.json();
      if (data.error) { alert('删除失败：' + data.error); return; }
      alert('已删除');
      this.fetchMemoriesTable();
    } catch (e) { alert('请求失败：' + e.message); }
  },

  escapeHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  },
};
