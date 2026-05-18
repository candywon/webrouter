/* 仪表盘页面逻辑 */
const DashboardPage = {
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
  },

  renderOverview(data) {
    const statsEl = document.getElementById('dashboard-stats');
    if (!statsEl) return;
    statsEl.innerHTML = `
      <div class="stat-card">
        <div class="stat-value" style="color:var(--success)">${data.providers?.healthy || 0}/${data.providers?.total || 0}</div>
        <div class="stat-label">可用数据源</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">${formatNumber(data.usage?.today_requests || 0)}</div>
        <div class="stat-label">今日调用</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:${(data.usage?.error_rate || 0) > 5 ? 'var(--danger)' : 'var(--success)'}">${data.usage?.error_rate || 0}%</div>
        <div class="stat-label">错误率</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">${formatYuan(data.cost?.month_cents || 0)}</div>
        <div class="stat-label">月度成本</div>
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
          label: '调用量',
          data: data.data.map(d => d.request_count || 0),
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
        el.innerHTML = '<div class="empty-state"><div class="icon">📡</div><p>暂无渠道数据<br>请先在"渠道管理"中添加渠道</p></div>';
        return;
      }

      const cols = detailed
        ? '<tr><th>渠道名</th><th>类型</th><th>状态</th><th>健康状态</th><th>延迟</th><th>操作</th></tr>'
        : '<tr><th>渠道名</th><th>类型</th><th>状态</th><th>健康</th><th>操作</th></tr>';

      el.innerHTML = `<table>
        <thead>${cols}</thead>
        <tbody>${channels.map(ch => {
          const statusBadge = ch.status === 'healthy' ? '<span class="badge badge-healthy">健康</span>'
            : ch.status === 'dead' ? '<span class="badge badge-dead">异常</span>'
            : ch.status === 'auth_failed' ? '<span class="badge badge-warning">认证失败</span>'
            : ch.status === 'warning' ? '<span class="badge badge-warning">警告</span>'
            : '<span class="badge badge-unknown">' + (ch.status || '未知') + '</span>';
          const latency = ch.last_latency_ms != null ? ch.last_latency_ms + 'ms' : '-';

          if (detailed) {
            return `<tr>
              <td>${ch.name || '-'}</td>
              <td>${ch.type || '-'}</td>
              <td><span class="badge badge-healthy">启用</span></td>
              <td>${statusBadge}</td>
              <td>${latency}</td>
              <td><button class="btn-sm" onclick="DashboardPage.checkChannel(${ch.id})">检测</button></td>
            </tr>`;
          }
          return `<tr>
            <td>${ch.name || '-'}</td>
            <td>${ch.type || '-'}</td>
            <td><span class="badge badge-healthy">启用</span></td>
            <td>${statusBadge}</td>
            <td><button class="btn-sm" onclick="DashboardPage.checkChannel(${ch.id})">检测</button></td>
          </tr>`;
        }).join('')}</tbody>
      </table>`;
    });
  },

  async checkChannel(id) {
    try {
      const result = await API.post(`/monitor/check/${id}`);
      showToast(`渠道 ${result.name}: ${result.status}`);
      this.load();
    } catch (e) {
      showToast('检测失败: ' + e.message);
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
      if (contentEl) contentEl.innerHTML = '<div class="empty-state"><p>加载失败</p></div>';
    }
  },

  renderCacheHitRate(data) {
    const totalEl = document.getElementById('cache-hit-total');
    const contentEl = document.getElementById('cache-hit-content');

    if (!data.data || data.data.length === 0) {
      if (totalEl) totalEl.style.display = 'none';
      if (contentEl) contentEl.innerHTML = '<div class="empty-state"><div class="icon">💾</div><p>暂无缓存命中数据<br><span class="hint">需要上游 API 支持 prompt cache 才会产生数据</span></p></div>';
      return;
    }

    // 总体汇总（仅按模型分组时有）
    if (data.total) {
      if (totalEl) {
        totalEl.style.display = 'block';
        totalEl.innerHTML = `
          <div style="display:flex;gap:24px;flex-wrap:wrap;">
            <div><span style="color:var(--text-muted);font-size:12px;">总请求</span><br><strong>${formatNumber(data.total.requests)}</strong></div>
            <div><span style="color:var(--text-muted);font-size:12px;">Cached Tokens</span><br><strong style="color:#a78bfa;">${formatNumber(data.total.cached_tokens)}</strong></div>
            <div><span style="color:var(--text-muted);font-size:12px;">综合命中率</span><br><strong style="color:${data.total.hit_rate > 30 ? 'var(--success)' : 'var(--warning)'};">${data.total.hit_rate}%</strong></div>
          </div>
        `;
      }
    } else if (totalEl) {
      totalEl.style.display = 'none';
    }

    const isProvider = data.group_by === 'provider';
    const label = isProvider ? '数据源' : '模型';

    if (contentEl) {
      contentEl.innerHTML = `<table>
        <thead><tr>
          <th>${label}</th>
          <th>请求数</th>
          <th>Input Tokens</th>
          <th>Cached Tokens</th>
          <th>命中率</th>
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
