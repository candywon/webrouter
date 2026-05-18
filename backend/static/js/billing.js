/* 计费统计页面逻辑 */
const BillingPage = {
  hours: 24,
  activeTab: 'models',
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
          <div class="stat-label">本月成本</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">${formatNumber(m.valid_requests)}</div>
          <div class="stat-label">本月有效请求</div>
        </div>
        <div class="stat-card">
          <div class="stat-value" style="color:var(--accent)">${formatYuan(w.cost_cents)}</div>
          <div class="stat-label">本周成本</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">${formatNumber(t.valid_requests)}</div>
          <div class="stat-label">今日请求</div>
        </div>
        <div class="stat-card">
          <div class="stat-value" style="color:var(--accent)">${formatYuan(t.cost_cents)}</div>
          <div class="stat-label">今日成本</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">${formatNumber(t.input_tokens)}</div>
          <div class="stat-label">今日输入 Token</div>
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
            label: '请求量',
            data: dailyData.map(d => d.request_count || 0),
            borderColor: '#6366f1',
            backgroundColor: 'rgba(99,102,241,0.3)',
            type: 'bar',
            yAxisID: 'y',
            order: 2,
          },
          {
            label: '成本 (¥)',
            data: dailyData.map(d => d.cost_yuan || 0),
            borderColor: '#f59e0b',
            backgroundColor: 'transparent',
            tension: 0.3,
            type: 'line',
            yAxisID: 'y1',
            order: 1,
          },
          {
            label: '错误数',
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
    el.innerHTML = '<div class="empty-state"><p>加载中...</p></div>';

    const h = this.hours;
    const tab = this.activeTab;

    try {
      if (tab === 'models') {
        const data = await API.get(`/billing/usage?hours=${h}`);
        this.renderModelsTable(el, data.data || []);
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
      }
    } catch (e) {
      el.innerHTML = `<div class="empty-state"><p>加载失败: ${e.message}</p></div>`;
    }
  },

  // ── 各 Tab 渲染 ──
  renderModelsTable(el, data) {
    if (!data.length) { el.innerHTML = '<div class="empty-state"><p>暂无模型数据</p></div>'; return; }
    let totalCost = 0;
    data.forEach(d => totalCost += (d.cost_cents || 0));

    el.innerHTML = `
      <div class="card-header"><span class="card-title">按模型统计</span><span style="color:var(--text-muted);font-size:13px;">总成本: ${formatYuan(totalCost)}</span></div>
      <table>
        <thead><tr><th>模型</th><th>请求量</th><th>有效请求</th><th>输入Token</th><th>输出Token</th><th>成本</th><th>平均延迟</th><th>错误</th></tr></thead>
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

  renderTokensTable(el, data) {
    if (!data.length) { el.innerHTML = '<div class="empty-state"><p>暂无令牌数据</p></div>'; return; }
    el.innerHTML = `
      <div class="card-header"><span class="card-title">按令牌统计</span></div>
      <table>
        <thead><tr><th>令牌名称</th><th>请求量</th><th>有效请求</th><th>输入Token</th><th>输出Token</th><th>成本</th><th>平均延迟</th><th>错误</th></tr></thead>
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
    if (!data.length) { el.innerHTML = '<div class="empty-state"><p>暂无数据源数据</p></div>'; return; }
    let totalCost = 0;
    data.forEach(d => totalCost += (d.cost_cents || 0));

    el.innerHTML = `
      <div class="card-header"><span class="card-title">按数据源统计</span><span style="color:var(--text-muted);font-size:13px;">总成本: ${formatYuan(totalCost)}</span></div>
      <table>
        <thead><tr><th>数据源</th><th>请求量</th><th>有效请求</th><th>输入Token</th><th>输出Token</th><th>成本</th><th>平均延迟</th></tr></thead>
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
    if (!data.length) { el.innerHTML = '<div class="empty-state"><p>暂无客户端数据</p></div>'; return; }
    el.innerHTML = `
      <div class="card-header"><span class="card-title">按客户端 IP 统计</span></div>
      <table>
        <thead><tr><th>客户端 IP</th><th>请求量</th><th>有效请求</th><th>输入Token</th><th>输出Token</th><th>成本</th><th>平均延迟</th><th>错误</th></tr></thead>
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
    if (!data.length) { el.innerHTML = '<div class="empty-state"><p>暂无接口数据</p></div>'; return; }
    el.innerHTML = `
      <div class="card-header"><span class="card-title">按接口统计</span></div>
      <table>
        <thead><tr><th>接口路径</th><th>请求量</th><th>有效请求</th><th>输入Token</th><th>输出Token</th><th>成本</th><th>平均延迟</th><th>错误</th></tr></thead>
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
    html += `<div style="padding:16px 16px 0 16px;"><strong>按错误类型</strong></div>`;
    if (byType.length === 0) {
      html += '<div style="padding:12px 16px;"><span style="color:var(--text-muted);">无异常请求</span></div>';
    } else {
      html += `<table><thead><tr><th>错误类型</th><th>数量</th><th>平均延迟</th></tr></thead><tbody>`;
      for (const r of byType) {
        html += `<tr><td><code>${esc(r.error_type)}</code></td><td>${formatNumber(r.count)}</td><td>${r.avg_latency_ms}ms</td></tr>`;
      }
      html += '</tbody></table>';
    }

    // 按 Provider + 错误类型
    html += `<div style="padding:16px 16px 0 16px;margin-top:16px;border-top:1px solid var(--border);"><strong>按数据源 + 错误类型（Top 20）</strong></div>`;
    if (byProvider.length === 0) {
      html += '<div style="padding:12px 16px;"><span style="color:var(--text-muted);">无异常请求</span></div>';
    } else {
      html += `<table><thead><tr><th>数据源</th><th>错误类型</th><th>数量</th></tr></thead><tbody>`;
      for (const r of byProvider) {
        html += `<tr><td>${esc(r.provider_name)}</td><td><code>${esc(r.error_type)}</code></td><td>${formatNumber(r.count)}</td></tr>`;
      }
      html += '</tbody></table>';
    }

    el.innerHTML = html;
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
        headers = ['模型', '请求量', '有效请求', '输入Token', '输出Token', '成本(元)', '平均延迟(ms)', '错误数'];
        rows = (data.data || []).map(d => [d.model_name, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_duration || '', d.error_count]);
      } else if (tab === 'tokens') {
        const data = await API.get(`/billing/by-token?hours=${h}`);
        headers = ['令牌名称', '请求量', '有效请求', '输入Token', '输出Token', '成本(元)', '平均延迟(ms)', '错误数'];
        rows = (data.data || []).map(d => [d.token_name, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      } else if (tab === 'providers') {
        const days = Math.ceil(h / 24) + 1;
        const data = await API.get(`/billing/provider?days=${days}`);
        headers = ['数据源', '请求量', '有效请求', '输入Token', '输出Token', '成本(元)', '平均延迟(ms)'];
        rows = (data.data || []).map(d => [d.provider_name, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '']);
      } else if (tab === 'clients') {
        const data = await API.get(`/billing/by-client?hours=${h}`);
        headers = ['客户端IP', '请求量', '有效请求', '输入Token', '输出Token', '成本(元)', '平均延迟(ms)', '错误数'];
        rows = (data.data || []).map(d => [d.client_ip, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      } else if (tab === 'endpoints') {
        const data = await API.get(`/billing/by-endpoint?hours=${h}`);
        headers = ['接口路径', '请求量', '有效请求', '输入Token', '输出Token', '成本(元)', '平均延迟(ms)', '错误数'];
        rows = (data.data || []).map(d => [d.endpoint, d.request_count, d.valid_count, d.input_tokens, d.output_tokens, d.cost_yuan, d.avg_latency_ms || '', d.error_count]);
      } else if (tab === 'errors') {
        const data = await API.get(`/billing/error-types?hours=${h}`);
        headers = ['数据源', '错误类型', '数量'];
        rows = (data.by_provider || []).map(d => [d.provider_name, d.error_type, d.count]);
      }

      if (rows.length === 0) {
        showToast('当前视图暂无数据可导出');
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
      showToast(`已导出 ${rows.length} 条记录`);
    } catch (e) {
      showToast('导出失败: ' + e.message, 'error');
    }
  },
};
