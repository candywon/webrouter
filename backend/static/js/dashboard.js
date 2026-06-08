// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 仪表盘页面逻辑 */
const DashboardPage = {
  _autoRefreshTimer: null,
  _autoRefreshEnabled: true,

  async load() {
    try {
      const data = await API.get('/dashboard/overview');
      this.renderOverview(data);
    } catch (e) {
      console.error('Failed to load overview:', e);
    }

    try {
      const trendData = await API.get('/dashboard/trends?days=7');
      this.renderTrendChart(trendData);
    } catch (e) {
      console.error('Failed to load trends:', e);
    }

    try {
      const chData = await API.get('/dashboard/channels');
      this.renderChannelList(chData.channels || []);
    } catch (e) {
      console.error('Failed to load channels:', e);
    }

    this.loadCacheHitRate();
    this.loadTopTokens();
    this.loadLatencyDistribution();
  },

  renderOverview(data) {
    const statsEl = document.getElementById('dashboard-stats');
    if (!statsEl) return;
    statsEl.innerHTML = `
      <div class="stat-card">
        <div class="stat-value" style="color:var(--success)">${data.providers?.healthy || 0}/${data.providers?.total || 0}</div>
        <div class="stat-label">${I18n.t('dashboard.availableProviders')}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">${formatNumber(data.usage?.today_requests || 0)}</div>
        <div class="stat-label">${I18n.t('dashboard.todayRequests')}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:${(data.usage?.error_rate || 0) > 5 ? 'var(--danger)' : 'var(--success)'}">${data.usage?.error_rate || 0}%</div>
        <div class="stat-label">${I18n.t('dashboard.errorRate')}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">${formatYuan(data.cost?.month_cents || 0)}</div>
        <div class="stat-label">${I18n.t('dashboard.monthlyCost')}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--accent);font-size:18px;">${data.latency?.p50_ms || 0}<span style="font-size:12px;">ms</span></div>
        <div class="stat-label">P50 ${I18n.t('dashboard.latency')}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--warning);font-size:18px;">${data.latency?.p90_ms || 0}<span style="font-size:12px;">ms</span></div>
        <div class="stat-label">P90 ${I18n.t('dashboard.latency')}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--danger);font-size:18px;">${data.latency?.p99_ms || 0}<span style="font-size:12px;">ms</span></div>
        <div class="stat-label">P99 ${I18n.t('dashboard.latency')}</div>
      </div>
    `;
  },

  renderTrendChart(data) {
    const ctx = document.getElementById('trend-chart');
    if (!ctx || !data.data || data.data.length === 0) return;

    if (this._trendChart) this._trendChart.destroy();
    this._trendChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: data.data.map(d => d.date),
        datasets: [{
          label: I18n.t("dashboard.requestVolume"),
          data: data.data.map(d => d.requests || 0),
          borderColor: '#6366f1',
          backgroundColor: 'rgba(99,102,241,0.1)',
          tension: 0.3,
          fill: true,
        }],
      },
      options: {
        responsive: true,
        plugins: { legend: { display: false } },
        scales: {
          x: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#8b8fa3' } },
          y: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#8b8fa3' }, beginAtZero: true },
        },
      },
    });
  },

  renderChannelList(channels) {
    const targets = document.querySelectorAll('#channel-list, #channel-list-channels');
    if (targets.length === 0) return;

    targets.forEach(el => {
      const detailed = el.id === 'channel-list-channels';

      if (channels.length === 0) {
        el.innerHTML = `<div class="empty-state"><div class="icon">📡</div><p>${I18n.t('dashboard.noChannelData')}<br>${I18n.t('common.goToChannel')}</p></div>`;
        return;
      }

      const cols = detailed
        ? `<tr><th>${I18n.t('dashboard.channelTableHeader')}</th><th>${I18n.t('dashboard.channelTypeHeader')}</th><th>${I18n.t('dashboard.channelStatusHeader')}</th><th>${I18n.t('dashboard.channelHealthHeader')}</th><th>${I18n.t('dashboard.channelLatencyHeader')}</th><th>${I18n.t('dashboard.channelActionsHeader')}</th></tr>`
        : `<tr><th>${I18n.t('dashboard.channelTableHeader')}</th><th>${I18n.t('dashboard.channelTypeHeader')}</th><th>${I18n.t('dashboard.channelStatusHeader')}</th><th>${I18n.t('dashboard.channelHealthHeader')}</th><th>${I18n.t('dashboard.channelActionsHeader')}</th></tr>`;

      el.innerHTML = `<table>
        <thead>${cols}</thead>
        <tbody>${channels.map(ch => {
          const statusBadge = ch.status === 'healthy' ? `<span class="badge badge-healthy">${I18n.t('common.healthy')}</span>`
            : ch.status === 'dead' ? `<span class="badge badge-dead">${I18n.t('common.unhealthy')}</span>`
            : ch.status === 'auth_failed' ? `<span class="badge badge-warning">${I18n.t('common.authFailed')}</span>`
            : ch.status === 'warning' ? `<span class="badge badge-warning">${I18n.t('common.warning')}</span>`
            : '<span class="badge badge-unknown">' + (ch.status || I18n.t('common.unknown')) + '</span>';
          const latency = ch.last_latency_ms != null ? ch.last_latency_ms + 'ms' : '-';

          if (detailed) {
            return `<tr>
              <td>${ch.name || '-'}</td>
              <td>${ch.type || '-'}</td>
              <td><span class="badge badge-healthy">${I18n.t('common.enabled')}</span></td>
              <td>${statusBadge}</td>
              <td>${latency}</td>
              <td><button class="btn-sm" onclick="DashboardPage.checkChannel(${ch.id})">${I18n.t('common.check')}</button></td>
            </tr>`;
          }
          return `<tr>
            <td>${ch.name || '-'}</td>
            <td>${ch.type || '-'}</td>
            <td><span class="badge badge-healthy">${I18n.t('common.enabled')}</span></td>
            <td>${statusBadge}</td>
            <td><button class="btn-sm" onclick="DashboardPage.checkChannel(${ch.id})">${I18n.t('common.check')}</button></td>
          </tr>`;
        }).join('')}</tbody>
      </table>`;
    });
  },

  async checkChannel(id) {
    try {
      const result = await API.post(`/monitor/check/${id}`);
      showToast(I18n.t('dashboard.checkChannelResult', {name: result.name, status: result.status}));
      this.load();
    } catch (e) {
      showToast(I18n.t('common.checkFailed') + e.message);
    }
  },

  async loadCacheHitRate() {
    const hoursEl = document.getElementById('cache-hours');
    const groupByEl = document.getElementById('cache-group-by');
    const hours = hoursEl ? hoursEl.value : 24;
    const groupBy = groupByEl ? groupByEl.value : 'model';

    try {
      const data = await API.get(`/dashboard/cache-hit-rate?hours=${hours}&group_by=${groupBy}`);
      this.renderCacheHitRate(data);
    } catch (e) {
      console.error('Failed to load cache hit rate:', e);
      const contentEl = document.getElementById('cache-hit-content');
      if (contentEl) contentEl.innerHTML = `<div class="empty-state"><p>${I18n.t('common.loadFailed')}</p></div>`;
    }
  },

  renderCacheHitRate(data) {
    const totalEl = document.getElementById('cache-hit-total');
    const contentEl = document.getElementById('cache-hit-content');

    if (!data.data || data.data.length === 0) {
      if (totalEl) totalEl.style.display = 'none';
      if (contentEl) contentEl.innerHTML = `<div class="empty-state"><div class="icon">💾</div><p>${I18n.t('dashboard.noCacheData')}<br><span class="hint">${I18n.t('dashboard.cacheNeedUpstream')}</span></p></div>`;
      return;
    }

    // 总体汇总（仅按模型分组时有）
    if (data.total) {
      if (totalEl) {
        totalEl.style.display = 'block';
        totalEl.innerHTML = `
          <div style="display:flex;gap:24px;flex-wrap:wrap;">
            <div><span style="color:var(--text-muted);font-size:12px;">${I18n.t('common.totalRequests')}</span><br><strong>${formatNumber(data.total.requests)}</strong></div>
            <div><span style="color:var(--text-muted);font-size:12px;">Cached Tokens</span><br><strong style="color:#a78bfa;">${formatNumber(data.total.cached_tokens)}</strong></div>
            <div><span style="color:var(--text-muted);font-size:12px;">${I18n.t('dashboard.overallHitRate')}</span><br><strong style="color:${data.total.hit_rate > 30 ? 'var(--success)' : 'var(--warning)'};">${data.total.hit_rate}%</strong></div>
          </div>
        `;
      }
    } else if (totalEl) {
      totalEl.style.display = 'none';
    }

    const isProvider = data.group_by === 'provider';
    const label = isProvider ? I18n.t("common.provider") : I18n.t("common.model");

    if (contentEl) {
      contentEl.innerHTML = `<table>
        <thead><tr>
          <th>${label}</th>
          <th>${I18n.t('common.requestsCount')}</th>
          <th>Input Tokens</th>
          <th>Cached Tokens</th>
          <th>${I18n.t('dashboard.hitRate')}</th>
        </tr></thead>
        <tbody>${data.data.map(d => {
          const name = isProvider ? d.provider_name : d.model;
          const barWidth = Math.min(d.hit_rate, 100);
          const barColor = d.hit_rate > 50 ? 'var(--success)' : d.hit_rate > 20 ? 'var(--warning)' : 'var(--danger)';
          return `<tr>
            <td>${name}</td>
            <td>${formatNumber(d.requests)}</td>
            <td>${formatNumber(d.input_tokens)}</td>
            <td style="color:#a78bfa;">${formatNumber(d.cached_tokens)}</td>
            <td>
              <div style="display:flex;align-items:center;gap:8px;">
                <div style="flex:1;max-width:120px;height:6px;background:rgba(255,255,255,0.05);border-radius:3px;overflow:hidden;">
                  <div style="width:${barWidth}%;height:100%;background:${barColor};border-radius:3px;"></div>
                </div>
                <span style="color:${barColor};font-weight:600;min-width:45px;">${d.hit_rate}%</span>
              </div>
            </td>
          </tr>`;
        }).join('')}</tbody>
      </table>`;
    }
  },

  async loadTopTokens() {
    const labelEl = document.getElementById('dashboard-top-label');
    const contentEl = document.getElementById('dashboard-top-content');

    try {
      const data = await API.get('/billing/top-tokens?hours=24&top=10');
      const tokens = data.tokens || [];

      if (!tokens.length) {
        if (labelEl) labelEl.textContent = I18n.t("common.noData");
        if (contentEl) contentEl.innerHTML = `<div class="empty-state"><p>${I18n.t('dashboard.noUsageData')}</p></div>`;
        return;
      }

      if (labelEl) labelEl.textContent = I18n.t('dashboard.totalRequestsLabel', {total: formatNumber(data.total_requests)});

      const maxRequests = Math.max(...tokens.map(t => t.request_count));

      let html = '';
      tokens.forEach((t, idx) => {
        const barWidth = maxRequests > 0 ? (t.request_count / maxRequests * 100) : 0;
        const rankColor = idx < 3 ? 'var(--accent)' : 'var(--text-muted)';

        html += `<div style="margin-bottom:14px;border-bottom:1px solid var(--border);padding-bottom:10px;">
          <div style="display:flex;align-items:center;gap:10px;margin-bottom:4px;">
            <span style="font-weight:700;font-size:15px;color:${rankColor};min-width:26px;">#${idx + 1}</span>
            <span style="font-weight:600;flex:1;">${esc(t.token_name)}</span>
            <span style="color:var(--text-muted);font-size:12px;">${formatNumber(t.request_count)} ${I18n.t('common.times')}</span>
            <span style="color:var(--accent);font-size:12px;">${formatYuan(t.cost_cents)}</span>
            <span style="color:var(--text-muted);font-size:11px;min-width:38px;text-align:right;">${t.pct_of_total}%</span>
          </div>
          <div style="background:var(--bg);border-radius:3px;height:6px;margin-bottom:6px;">
            <div style="background:var(--accent);height:100%;border-radius:3px;width:${barWidth}%;transition:width .3s;"></div>
          </div>`;

        if (t.model_distribution && t.model_distribution.length) {
          html += '<div style="display:flex;flex-wrap:wrap;gap:4px;margin-left:36px;">';
          t.model_distribution.forEach(m => {
            const mPct = t.request_count > 0 ? (m.count / t.request_count * 100).toFixed(1) : 0;
            html += `<span style="font-size:11px;background:var(--bg);padding:1px 6px;border-radius:3px;">
              ${esc(m.model_name)} ${mPct}% (${formatYuan(m.cost_cents)})
            </span>`;
          });
          html += '</div>';
        }

        html += `</div>`;
      });

      if (contentEl) contentEl.innerHTML = html;
    } catch (e) {
      console.error('Failed to load top tokens:', e);
      if (labelEl) labelEl.textContent = I18n.t("common.loadFailed");
      if (contentEl) contentEl.innerHTML = `<div class="empty-state"><p>${I18n.t('common.loadFailed')}: ${esc(e.message)}</p></div>`;
    }
  },

  // ----- 延迟分布 -----
  async loadLatencyDistribution() {
    try {
      const data = await API.get('/monitoring/latency-distribution?hours=24&group_by=model');
      this.renderLatencyChart(data);
    } catch (e) {
      console.error('Failed to load latency distribution:', e);
    }
  },

  renderLatencyChart(data) {
    const canvas = document.getElementById('latency-distribution-chart');
    if (!canvas || !data.data || data.data.length === 0) return;

    if (this._latencyChart) this._latencyChart.destroy();

    const labels = data.data.map(d => d.name);
    this._latencyChart = new Chart(canvas, {
      type: 'bar',
      data: {
        labels: labels,
        datasets: [
          {
            label: 'P50',
            data: data.data.map(d => d.p50_ms),
            backgroundColor: 'rgba(99,102,241,0.7)',
            borderRadius: 3,
          },
          {
            label: 'P90',
            data: data.data.map(d => d.p90_ms),
            backgroundColor: 'rgba(251,191,36,0.7)',
            borderRadius: 3,
          },
          {
            label: 'P99',
            data: data.data.map(d => d.p99_ms),
            backgroundColor: 'rgba(239,68,68,0.7)',
            borderRadius: 3,
          },
        ],
      },
      options: {
        responsive: true,
        plugins: {
          legend: {
            position: 'top',
            labels: { color: '#8b8fa3' },
          },
        },
        scales: {
          x: {
            grid: { color: 'rgba(255,255,255,0.05)' },
            ticks: { color: '#8b8fa3' },
          },
          y: {
            grid: { color: 'rgba(255,255,255,0.05)' },
            ticks: { color: '#8b8fa3' },
            beginAtZero: true,
          },
        },
      },
    });
  },

  // ----- 自动刷新 -----
  startAutoRefresh(intervalMs) {
    this.stopAutoRefresh();
    this._autoRefreshEnabled = true;
    this._autoRefreshTimer = setInterval(() => {
      if (this._autoRefreshEnabled) this.load();
    }, intervalMs || 30000);
  },

  stopAutoRefresh() {
    this._autoRefreshEnabled = false;
    if (this._autoRefreshTimer) {
      clearInterval(this._autoRefreshTimer);
      this._autoRefreshTimer = null;
    }
  },

  toggleAutoRefresh() {
    const btn = document.getElementById('auto-refresh-btn');
    if (this._autoRefreshTimer) {
      this.stopAutoRefresh();
      if (btn) btn.textContent = I18n.t('dashboard.startAutoRefresh');
    } else {
      this.startAutoRefresh();
      if (btn) btn.textContent = I18n.t('dashboard.stopAutoRefresh');
    }
  },

  // ----- CSV 导出 -----
  async exportCSV() {
    try {
      const hours = 24;
      const url = `/api/export/dashboard-csv?hours=${hours}`;
      window.open(url, '_blank');
    } catch (e) {
      showToast('Export failed: ' + e.message);
    }
  },

};

/* 渠道管理页面 - 复用 DashboardPage 的数据加载，渲染到 channels 页面容器 */
const ChannelsPage = {
  async load() {
    try {
      const chData = await API.get('/dashboard/channels');
      DashboardPage.renderChannelList(chData.channels || []);
    } catch (e) {
      console.error('Failed to load channels:', e);
    }
  },
};
