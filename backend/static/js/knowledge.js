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
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('domains')">业务域</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('analyze')">单域分析</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('analyses')">分析记录</button>
          <button class="btn btn-sm" onclick="KnowledgePage.switchTab('search')">搜索</button>
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
      case 'domains': this.loadDomains(); break;
      case 'analyze': this.renderAnalyze(); break;
      case 'analyses': this.loadAnalyses(); break;
      case 'search': this.renderSearch(); break;
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
    const el = document.getElementById('knowledge-tab-content');
    el.innerHTML = '<div class="empty-state"><div class="icon">⏳</div><p>加载中...</p></div>';
    try {
      const res = await fetch(`/api/knowledge/items?page=${page}&per_page=20`);
      const data = await res.json();
      if (!data.items.length) {
        el.innerHTML = '<div class="empty-state"><div class="icon">📭</div><p>暂无知识条目</p></div>';
        return;
      }
      el.innerHTML = `
        <div class="card">
          <div class="card-header">
            <span class="card-title">知识条目 (${data.total} 条)</span>
          </div>
          <table class="table">
            <thead><tr>
              <th>ID</th><th>类型</th><th>标题</th><th>领域</th><th>置信度</th><th>验证</th><th>时间</th>
            </tr></thead>
            <tbody>
              ${data.items.map(i => `<tr>
                <td>${i.id}</td>
                <td><span class="badge badge-${this.typeColor(i.type)}">${i.type}</span></td>
                <td>${this.escapeHtml(i.title)}</td>
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
          <div class="card-header"><span class="card-title">业务域 (${domData.total})</span></div>
          <table class="table">
            <thead><tr>
              <th>代码</th><th>名称</th><th>部门</th><th>状态</th><th>风险等级</th><th>最小验证</th><th>过期天数</th>
            </tr></thead>
            <tbody>
              ${domData.domains.map(d => {
                const r = riskMap[d.domain_code] || {};
                return `<tr>
                  <td><code>${d.domain_code}</code></td>
                  <td>${d.domain_name}</td>
                  <td>${d.department || '-'}</td>
                  <td><span class="badge badge-${d.status === 'active' ? 'success' : 'warning'}">${d.status}</span></td>
                  <td>${r.risk_level || '-'}</td>
                  <td>${r.min_verification || '-'}</td>
                  <td>${r.max_age_days || '-'}</td>
                </tr>`;
              }).join('')}
            </tbody>
          </table>
        </div>
      `;
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><div class="icon">⚠️</div><p>加载失败：${e.message}</p></div>`;
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

  escapeHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  },
};
