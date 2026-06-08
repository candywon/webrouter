// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 计费统计页面逻辑 */
const BillingPage = {
  hours: 24,
  activeTab: 'top',
  _chart: null,

  async load() {
    await Promise.all([
      this.loadSummary(),
      this.loadDaily(),
    ]);
    this.loadTabData();
  },

  onHoursChange() {
    this.hours = parseInt(document.getElementById('billing-hours').value);
    this.load();
  },

  switchTab(btn) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    btn.classList.add('active');
    this.activeTab = btn.dataset.tab;
    this.loadTabData();
  },

  // ── 概览汇总 ──
  async loadSummary() {
    try {
      const data = await API.get('/billing/summary');
      const el = document.getElementById('billing-summary');
      if (!el) return;

      const m = data.month || {};
      const w = data.week || {};
      const t = data.today || {};

      el.innerHTML = `
        <div class="stat-card">
          <div class="stat-value" style="color:var(--accent)">${formatYuan(m.cost_cents)}</div>
          <div class="stat-label">${I18n.t('billing.monthCost')}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">${formatNumber(m.valid_requests)}</div>
          <div class="stat-label">${I18n.t('billing.monthValidRequests')}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value" style="color:var(--accent)">${formatYuan(w.cost_cents)}</div>
          <div class="stat-label">${I18n.t('billing.weekCost')}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">${formatNumber(t.valid_requests)}</div>
          <div class="stat-label">${I18n.t('billing.todayRequests')}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value" style="color:var(--accent)">${formatYuan(t.cost_cents)}</div>
          <div class="stat-label">${I18n.t('billing.todayCost')}</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">${formatNumber(t.input_tokens)}</div>
          <div class="stat-label">${I18n.t('billing.todayInputTokens')}</div>
        </div>`;
    } catch (e) {
      console.error('Failed to load summary:', e);
    }
  },

  // ── 每日趋势 ──
  async loadDaily() {
    try {
      const data = await API.get(`/billing/daily?days=${Math.ceil(this.hours / 24) + 1}`);
      this.renderDailyChart(data.data || []);
    } catch (e) {
      console.error('Failed to load daily:', e);
    }
  },

  renderDailyChart(dailyData) {
    const ctx = document.getElementById('billing-daily-chart');
    if (!ctx) return;
    if (!dailyData.length) {
      if (this._chart) { this._chart.destroy(); this._chart = null; }
      return;
    }

    if (this._chart) this._chart.destroy();
    this._chart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: dailyData.map(d => d.date),
        datasets: [
          {
            label: I18n.t("common.requestCount"),
            data: dailyData.map(d => d.request_count || 0),
            borderColor: '#6366f1',
            backgroundColor: 'rgba(99,102,241,0.3)',
            type: 'bar',
            yAxisID: 'y',
            order: 2,
          },
          {
            label: I18n.t("billing.costYuan"),
            data: dailyData.map(d => d.cost_yuan || 0),
            borderColor: '#f59e0b',
            backgroundColor: 'transparent',
            tension: 0.3,
            type: 'line',
            yAxisID: 'y1',
            order: 1,
          },
          {
            label: I18n.t("billing.errorCount"),
            data: dailyData.map(d => d.error_count || 0),
            borderColor: '#ef4444',
            backgroundColor: 'transparent',
            tension: 0.3,
            borderDash: [5, 5],
            type: 'line',
            yAxisID: 'y',
            order: 2,
          },
        ],
      },
      options: {
        responsive: true,
        plugins: { legend: { labels: { color: '#8b8fa3' } } },
        scales: {
          x: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#8b8fa3' } },
          y: {
            type: 'linear', position: 'left',
            grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#8b8fa3' },
            beginAtZero: true,
          },
          y1: {
            type: 'linear', position: 'right',
            grid: { display: false }, ticks: { color: '#f59e0b' },
            beginAtZero: true,
          },
        },
      },
    });
  },

  // ── Tab 数据加载 ──
  async loadTabData() {
    const el = document.getElementById('billing-tab-content');
    if (!el) return;
    el.innerHTML = `<div class="empty-state"><p>${I18n.t('common.loading')}</p></div>`;

    const h = this.hours;
    const tab = this.activeTab;

    try {
      if (tab === 'models') {
        const data = await API.get(`/billing/usage?hours=${h}`);
        this.renderModelsTable(el, data.data || []);
      } else if (tab === 'top') {
        const data = await API.get(`/billing/top-tokens?hours=${h}&top=10`);
        this.renderTopTokens(el, data);
      } else if (tab === 'tokens') {
        const data = await API.get(`/billing/by-token?hours=${h}`);
        this.renderTokensTable(el, data.data || []);
      } else if (tab === 'providers') {
        const data = await API.get(`/billing/provider?days=${Math.ceil(h / 24) + 1}`);
        this.renderProvidersTable(el, data.data || []);
      } else if (tab === 'clients') {
        const data = await API.get(`/billing/by-client?hours=${h}`);
        this.renderClientsTable(el, data.data || []);
      } else if (tab === 'endpoints') {
        const data = await API.get(`/billing/by-endpoint?hours=${h}`);
        this.renderEndpointsTable(el, data.data || []);
      } else if (tab === 'errors') {
        const data = await API.get(`/billing/error-types?hours=${h}`);
        this.renderErrorsTable(el, data);
      } else if (tab === 'orgs') {
        const groupBy = document.getElementById('billing-org-group-by')?.value || 'org';
        const orgType = document.getElementById('billing-org-type')?.value || '';
        const params = new URLSearchParams({ hours: h, group_by: groupBy });
        if (orgType) params.set('org_type', orgType);
        const data = await API.get(`/billing/by-org?${params.toString()}`);
        this.renderOrgsTable(el, data.data || [], { groupBy, orgType });
      }
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><p>${I18n.t('common.loadFailedError')} ${e.message}</p></div>`;
    }
  },

  // ── 各 Tab 渲染 ──
  renderModelsTable(el, data) {
    if (!data.length) { el.innerHTML = `<div class="empty-state"><p>${I18n.t('billing.noModelData')}</p></div>`; return; }
    let totalCost = 0;
    data.forEach(d => totalCost += (d.cost_cents || 0));

    el.innerHTML = `
      <div class="card-header"><span class="card-title">${I18n.t('billing.byModel')}</span><span style="color:var(--text-muted);font-size:13px;">${I18n.t('billing.totalCost')} ${formatYuan(totalCost)}</span></div>
      <table>
        <thead><tr><th>${I18n.t('common.model')}</th><th>${I18n.t('billing.requestVolume')}</th><th>${I18n.t('billing.validRequests')}</th><th>${I18n.t('billing.inputTokens')}</th><th>${I18n.t('billing.outputTokens')}</th><th>${I18n.t('common.cost')}</th><th>${I18n.t('billing.avgLatency')}</th><th>${I18n.t('billing.errors')}</th></tr></thead>
        <tbody>${data.map(d => `
          <tr>
            <td><code>${esc(d.model_name)}</code></td>
            <td>${formatNumber(d.request_count)}</td>
            <td>${formatNumber(d.valid_count)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td>${formatNumber(d.output_tokens)}</td>
            <td>${formatYuan(d.cost_cents)}</td>
            <td>${d.avg_duration != null ? d.avg_duration + 'ms' : '-'}</td>
            <td>${d.error_count || 0}</td>
          </tr>`).join('')}
        </tbody>
      </table>`;
  },

  renderTopTokens(el, data) {
    const tokens = data.tokens || [];
    if (!tokens.length) { el.innerHTML = `<div class="empty-state"><p>${I18n.t('common.noData')}</p></div>`; return; }

    const maxRequests = Math.max(...tokens.map(t => t.request_count));

    let html = `<div class="card-header">
      <span class="card-title">${I18n.t('billing.top10Usage')}</span>
      <span style="color:var(--text-muted);font-size:13px;">
        ${I18n.t('billing.totalRequestsStats', {total: formatNumber(data.total_requests), hours: data.hours})}
      </span>
    </div>`;

    tokens.forEach((t, idx) => {
      const pct = t.pct_of_total;
      const barWidth = maxRequests > 0 ? (t.request_count / maxRequests * 100) : 0;
      const rankColor = idx < 3 ? 'var(--accent)' : 'var(--text-muted)';

      html += `<div style="margin-bottom:16px;border-bottom:1px solid var(--border);padding-bottom:12px;">
        <div style="display:flex;align-items:center;gap:12px;margin-bottom:6px;">
          <span style="font-weight:700;font-size:16px;color:${rankColor};min-width:28px;">#${idx + 1}</span>
          <span style="font-weight:600;flex:1;">${esc(t.token_name)}</span>
          <span style="color:var(--text-muted);font-size:13px;">${formatNumber(t.request_count)} ${I18n.t('common.times')}</span>
          <span style="color:var(--accent);font-size:13px;">${formatYuan(t.cost_cents)}</span>
          <span style="color:var(--text-muted);font-size:12px;min-width:42px;text-align:right;">${pct}%</span>
        </div>
        <div style="background:var(--bg);border-radius:4px;height:8px;margin-bottom:8px;">
          <div style="background:var(--accent);height:100%;border-radius:4px;width:${barWidth}%;transition:width .3s;"></div>
        </div>`;

      if (t.model_distribution && t.model_distribution.length) {
        html += '<div style="display:flex;flex-wrap:wrap;gap:6px;margin-left:40px;">';
        t.model_distribution.forEach(m => {
          const mPct = t.request_count > 0 ? (m.count / t.request_count * 100).toFixed(1) : 0;
          html += `<span style="font-size:12px;background:var(--bg);padding:2px 8px;border-radius:4px;">
            ${esc(m.model_name)} ${mPct}% (${formatYuan(m.cost_cents)})
          </span>`;
        });
        html += '</div>';
      }

      html += `</div>`;
    });

    el.innerHTML = html;
  },

  renderTokensTable(el, data) {
    if (!data.length) { el.innerHTML = `<div class="empty-state"><p>${I18n.t('billing.noTokenData')}</p></div>`; return; }
    el.innerHTML = `
      <div class="card-header"><span class="card-title">${I18n.t('billing.byToken')}</span></div>
      <table>
        <thead><tr><th>${I18n.t('billing.tokenName')}</th><th>${I18n.t('billing.requestVolume')}</th><th>${I18n.t('billing.validRequests')}</th><th>${I18n.t('billing.inputTokens')}</th><th>${I18n.t('billing.outputTokens')}</th><th>${I18n.t('common.cost')}</th><th>${I18n.t('billing.avgLatency')}</th><th>${I18n.t('billing.errors')}</th></tr></thead>
        <tbody>${data.map(d => `
          <tr>
            <td>${esc(d.token_name)}</td>
            <td>${formatNumber(d.request_count)}</td>
            <td>${formatNumber(d.valid_count)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td>${formatNumber(d.output_tokens)}</td>
            <td>${formatYuan(d.cost_cents)}</td>
            <td>${d.avg_latency_ms != null ? d.avg_latency_ms + 'ms' : '-'}</td>
            <td>${d.error_count || 0}</td>
          </tr>`).join('')}
        </tbody>
      </table>`;
  },

  renderProvidersTable(el, data) {
    if (!data.length) { el.innerHTML = `<div class="empty-state"><p>${I18n.t('billing.noProviderData')}</p></div>`; return; }
    let totalCost = 0;
    data.forEach(d => totalCost += (d.cost_cents || 0));

    el.innerHTML = `
      <div class="card-header"><span class="card-title">${I18n.t('billing.byProvider')}</span><span style="color:var(--text-muted);font-size:13px;">${I18n.t('billing.totalCost')} ${formatYuan(totalCost)}</span></div>
      <table>
        <thead><tr><th>${I18n.t('common.provider')}</th><th>${I18n.t('billing.requestVolume')}</th><th>${I18n.t('billing.validRequests')}</th><th>${I18n.t('billing.inputTokens')}</th><th>${I18n.t('billing.outputTokens')}</th><th>${I18n.t('common.cost')}</th><th>${I18n.t('billing.avgLatency')}</th></tr></thead>
        <tbody>${data.map(d => `
          <tr>
            <td>${esc(d.provider_name)}</td>
            <td>${formatNumber(d.request_count)}</td>
            <td>${formatNumber(d.valid_count)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td>${formatNumber(d.output_tokens)}</td>
            <td>${formatYuan(d.cost_cents)}</td>
            <td>${d.avg_latency_ms != null ? d.avg_latency_ms + 'ms' : '-'}</td>
          </tr>`).join('')}
        </tbody>
      </table>`;
  },

  renderClientsTable(el, data) {
    if (!data.length) { el.innerHTML = `<div class="empty-state"><p>${I18n.t('billing.noClientData')}</p></div>`; return; }
    el.innerHTML = `
      <div class="card-header"><span class="card-title">${I18n.t('billing.byClient')}</span></div>
      <table>
        <thead><tr><th>${I18n.t('billing.clientIp')}</th><th>${I18n.t('billing.requestVolume')}</th><th>${I18n.t('billing.validRequests')}</th><th>${I18n.t('billing.inputTokens')}</th><th>${I18n.t('billing.outputTokens')}</th><th>${I18n.t('common.cost')}</th><th>${I18n.t('billing.avgLatency')}</th><th>${I18n.t('billing.errors')}</th></tr></thead>
        <tbody>${data.map(d => `
          <tr>
            <td><code>${esc(d.client_ip)}</code></td>
            <td>${formatNumber(d.request_count)}</td>
            <td>${formatNumber(d.valid_count)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td>${formatNumber(d.output_tokens)}</td>
            <td>${formatYuan(d.cost_cents)}</td>
            <td>${d.avg_latency_ms != null ? d.avg_latency_ms + 'ms' : '-'}</td>
            <td>${d.error_count || 0}</td>
          </tr>`).join('')}
        </tbody>
      </table>`;
  },

  renderEndpointsTable(el, data) {
    if (!data.length) { el.innerHTML = `<div class="empty-state"><p>${I18n.t('billing.noEndpointData')}</p></div>`; return; }
    el.innerHTML = `
      <div class="card-header"><span class="card-title">${I18n.t('billing.byEndpoint')}</span></div>
      <table>
        <thead><tr><th>${I18n.t('billing.endpointPath')}</th><th>${I18n.t('billing.requestVolume')}</th><th>${I18n.t('billing.validRequests')}</th><th>${I18n.t('billing.inputTokens')}</th><th>${I18n.t('billing.outputTokens')}</th><th>${I18n.t('common.cost')}</th><th>${I18n.t('billing.avgLatency')}</th><th>${I18n.t('billing.errors')}</th></tr></thead>
        <tbody>${data.map(d => `
          <tr>
            <td><code>${esc(d.endpoint)}</code></td>
            <td>${formatNumber(d.request_count)}</td>
            <td>${formatNumber(d.valid_count)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td>${formatNumber(d.output_tokens)}</td>
            <td>${formatYuan(d.cost_cents)}</td>
            <td>${d.avg_latency_ms != null ? d.avg_latency_ms + 'ms' : '-'}</td>
            <td>${d.error_count || 0}</td>
          </tr>`).join('')}
        </tbody>
      </table>`;
  },

  renderErrorsTable(el, data) {
    const byType = data.by_type || [];
    const byProvider = data.by_provider || [];

    let html = '';

    // 按错误类型
    html += `<div style="padding:16px 16px 0 16px;"><strong>${I18n.t('billing.byErrorType')}</strong></div>`;
    if (byType.length === 0) {
      html += `<div style="padding:12px 16px;"><span style="color:var(--text-muted);">${I18n.t('billing.noErrorRequests')}</span></div>`;
    } else {
      html += `<table><thead><tr><th>${I18n.t('billing.errorType')}</th><th>${I18n.t('common.count')}</th><th>${I18n.t('billing.avgLatency')}</th></tr></thead><tbody>`;
      for (const r of byType) {
        html += `<tr><td><code>${esc(r.error_type)}</code></td><td>${formatNumber(r.count)}</td><td>${r.avg_latency_ms}ms</td></tr>`;
      }
      html += '</tbody></table>';
    }

    // 按 Provider + 错误类型
    html += `<div style="padding:16px 16px 0 16px;margin-top:16px;border-top:1px solid var(--border);"><strong>${I18n.t('billing.byProviderErrorType')}</strong></div>`;
    if (byProvider.length === 0) {
      html += `<div style="padding:12px 16px;"><span style="color:var(--text-muted);">${I18n.t('billing.noErrorRequests')}</span></div>`;
    } else {
      html += `<table><thead><tr><th>${I18n.t('common.provider')}</th><th>${I18n.t('billing.errorType')}</th><th>${I18n.t('common.count')}</th></tr></thead><tbody>`;
      for (const r of byProvider) {
        html += `<tr><td>${esc(r.provider_name)}</td><td><code>${esc(r.error_type)}</code></td><td>${formatNumber(r.count)}</td></tr>`;
      }
      html += '</tbody></table>';
    }

    el.innerHTML = html;
  },

  // ── 按组织渲染 ──
  renderOrgsTable(el, data, opts = {}) {
    const groupBy = opts.groupBy || 'org';
    const orgType = opts.orgType || '';
    let totalCost = 0;
    data.forEach(d => totalCost += (d.cost_cents || 0));

    const controls = `
      <div style="display:flex;gap:12px;align-items:center;margin-bottom:12px;flex-wrap:wrap;">
        <label style="color:var(--text-muted);font-size:13px;">${I18n.t('billing.groupMode')}</label>
        <select id="billing-org-group-by" onchange="BillingPage.loadTabData()" style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);padding:6px 12px;">
          <option value="org" ${groupBy === 'org' ? 'selected' : ''}>${I18n.t('billing.byOrgNode')}</option>
          <option value="type" ${groupBy === 'type' ? 'selected' : ''}>${I18n.t('billing.byOrgType')}</option>
        </select>
        <label style="color:var(--text-muted);font-size:13px;">${I18n.t('billing.orgType')}</label>
        <select id="billing-org-type" onchange="BillingPage.loadTabData()" style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);padding:6px 12px;">
          <option value="" ${orgType === '' ? 'selected' : ''}>${I18n.t('common.all')}</option>
          <option value="company" ${orgType === 'company' ? 'selected' : ''}>${I18n.t('common.companyEnterprise')}</option>
          <option value="department" ${orgType === 'department' ? 'selected' : ''}>${I18n.t('knowledge.department')}</option>
          <option value="project" ${orgType === 'project' ? 'selected' : ''}>${I18n.t('billing.projectGroup')}</option>
          <option value="group" ${orgType === 'group' ? 'selected' : ''}>${I18n.t('common.group')}</option>
        </select>
      </div>`;

    if (!data.length) { el.innerHTML = controls + `<div class="empty-state"><p>${I18n.t('billing.noOrgData')}</p></div>`; return; }

    const nameHeader = groupBy === 'type' ? I18n.t('billing.orgType') : I18n.t('billing.orgName');
    el.innerHTML = `
      ${controls}
      <div class="card-header"><span class="card-title">${I18n.t('billing.byOrg')}</span><span style="color:var(--text-muted);font-size:13px;">${I18n.t('billing.totalCost')} ${formatYuan(totalCost)}</span></div>
      <table>
        <thead><tr><th>${nameHeader}</th><th>${I18n.t('billing.orgType')}</th><th>${I18n.t('billing.orgCount')}</th><th>${I18n.t('billing.requestVolume')}</th><th>${I18n.t('billing.validRequests')}</th><th>${I18n.t('billing.inputTokens')}</th><th>${I18n.t('billing.outputTokens')}</th><th>${I18n.t('common.cost')}</th><th>${I18n.t('billing.avgLatency')}</th><th>${I18n.t('billing.errors')}</th></tr></thead>
        <tbody>${data.map(d => `
          <tr>
            <td><strong>${esc(d.org_name)}</strong></td>
            <td>${this.formatOrgType(d.org_type)}</td>
            <td>${formatNumber(d.org_count || 1)}</td>
            <td>${formatNumber(d.request_count)}</td>
            <td>${formatNumber(d.valid_count)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td>${formatNumber(d.output_tokens)}</td>
            <td>${formatYuan(d.cost_cents)}</td>
            <td>${d.avg_latency_ms != null ? d.avg_latency_ms + 'ms' : '-'}</td>
            <td>${d.error_count || 0}</td>
          </tr>`).join('')}
        </tbody>
      </table>`;
  },

  formatOrgType(type) {
    const map = {
      company: I18n.t('common.companyEnterprise'),
      department: I18n.t('knowledge.department'),
      project: I18n.t('billing.projectGroup'),
      group: I18n.t('common.group'),
      unassigned: I18n.t('billing.unassigned'),
      '': '-',
    };
    return map[type] || type || '-';
  },

  // ── 导出 CSV ──
  async exportData() {
    const tab = this.activeTab;
    const h = this.hours;

    try {
      let rows = [];
      let headers = [];

      if (tab === 'models') {
        const data = await API.get(`/billing/usage?hours=${h}`);
        headers = [I18n.t("common.model"), I18n.t("common.requestCount"), I18n.t("billing.validRequests"), I18n.t("billing.inputTokens"), I18n.t("billing.outputTokens"), I18n.t("billing.costYuan"), I18n.t("billing.avgLatency") + '(ms)', I18n.t("billing.errorCount")];
        rows = (data.data || []).map(d => [d.model_name, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_duration || '', d.error_count]);
      } else if (tab === 'tokens') {
        const data = await API.get(`/billing/by-token?hours=${h}`);
        headers = [I18n.t("billing.tokenName"), I18n.t("common.requestCount"), I18n.t("billing.validRequests"), I18n.t("billing.inputTokens"), I18n.t("billing.outputTokens"), I18n.t("billing.costYuan"), I18n.t("billing.avgLatency") + '(ms)', I18n.t("billing.errorCount")];
        rows = (data.data || []).map(d => [d.token_name, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      } else if (tab === 'providers') {
        const days = Math.ceil(h / 24) + 1;
        const data = await API.get(`/billing/provider?days=${days}`);
        headers = [I18n.t("common.provider"), I18n.t("common.requestCount"), I18n.t("billing.validRequests"), I18n.t("billing.inputTokens"), I18n.t("billing.outputTokens"), I18n.t("billing.costYuan"), I18n.t("billing.avgLatency") + '(ms)'];
        rows = (data.data || []).map(d => [d.provider_name, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '']);
      } else if (tab === 'clients') {
        const data = await API.get(`/billing/by-client?hours=${h}`);
        headers = [I18n.t("billing.clientIp"), I18n.t("common.requestCount"), I18n.t("billing.validRequests"), I18n.t("billing.inputTokens"), I18n.t("billing.outputTokens"), I18n.t("billing.costYuan"), I18n.t("billing.avgLatency") + '(ms)', I18n.t("billing.errorCount")];
        rows = (data.data || []).map(d => [d.client_ip, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      } else if (tab === 'endpoints') {
        const data = await API.get(`/billing/by-endpoint?hours=${h}`);
        headers = [I18n.t("billing.endpointPath"), I18n.t("common.requestCount"), I18n.t("billing.validRequests"), I18n.t("billing.inputTokens"), I18n.t("billing.outputTokens"), I18n.t("billing.costYuan"), I18n.t("billing.avgLatency") + '(ms)', I18n.t("billing.errorCount")];
        rows = (data.data || []).map(d => [d.endpoint, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      } else if (tab === 'errors') {
        const data = await API.get(`/billing/error-types?hours=${h}`);
        headers = [I18n.t("common.provider"), I18n.t("billing.errorType"), I18n.t("common.count")];
        rows = (data.by_provider || []).map(d => [d.provider_name, d.error_type, d.count]);
      } else if (tab === 'orgs') {
        const groupBy = document.getElementById('billing-org-group-by')?.value || 'org';
        const orgType = document.getElementById('billing-org-type')?.value || '';
        const params = new URLSearchParams({ hours: h, group_by: groupBy });
        if (orgType) params.set('org_type', orgType);
        const data = await API.get(`/billing/by-org?${params.toString()}`);
        headers = [I18n.t("billing.orgName"), I18n.t("billing.orgType"), I18n.t("billing.orgCount"), I18n.t("common.requestCount"), I18n.t("billing.validRequests"), I18n.t("billing.inputTokens"), I18n.t("billing.outputTokens"), I18n.t("billing.costYuan"), I18n.t("billing.avgLatency") + '(ms)', I18n.t("billing.errorCount")];
        rows = (data.data || []).map(d => [d.org_name, d.org_type, d.org_count || 1, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      }

      if (rows.length === 0) {
        showToast(I18n.t("billing.noDataExport"));
        return;
      }

      // 生成 CSV（UTF-8 BOM 以便 Excel 正确识别中文）
      let csv = '﻿' + headers.join(',') + '\n';
      for (const row of rows) {
        csv += row.map(v => {
          const s = String(v ?? '');
          return s.includes(',') || s.includes('"') || s.includes('\n') ? '"' + s.replace(/"/g, '""') + '"' : s;
        }).join(',') + '\n';
      }

      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `billing_${tab}_${this.hours}h_${new Date().toISOString().slice(0, 10)}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      showToast(I18n.t('common.exportedCount', {count: rows.length}));
    } catch (e) {
      showToast(I18n.t('billing.exportFailed') + e.message, 'error');
    }
  },
};
