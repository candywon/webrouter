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
  },

  renderOverview(data) {
    const statsEl = document.getElementById('dashboard-stats');
    if (!statsEl) return;
    statsEl.innerHTML = `
      <div class="stat-card">
        <div class="stat-value" style="color:var(--success)">${data.channels?.healthy || 0}/${data.channels?.total || 0}</div>
        <div class="stat-label">可用渠道</div>
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
    // Target both dashboard (#channel-list) and channels page (#channel-list-channels)
    const targets = document.querySelectorAll('#channel-list, #channel-list-channels');
    if (targets.length === 0) return;

    targets.forEach(el => {
      // Determine if this is the detailed (channels page) view
      const detailed = el.id === 'channel-list-channels';

      if (channels.length === 0) {
        el.innerHTML = '<div class="empty-state"><div class="icon">📡</div><p>暂无渠道数据<br>请先在 New-API 中添加渠道</p></div>';
        return;
      }

      const cols = detailed
        ? '<tr><th>渠道名</th><th>类型</th><th>状态</th><th>健康状态</th><th>延迟</th><th>操作</th></tr>'
        : '<tr><th>渠道名</th><th>类型</th><th>状态</th><th>健康</th><th>操作</th></tr>';

      el.innerHTML = `<table>
        <thead>${cols}</thead>
        <tbody>${channels.map(ch => {
          if (detailed) {
            return `<tr>
              <td>${ch.name || '-'}</td>
              <td>${ch.type || '-'}</td>
              <td>${ch.status === 1 ? '<span class="badge badge-healthy">启用</span>' : '<span class="badge badge-unknown">禁用</span>'}</td>
              <td>${statusBadge(ch.health?.status || 'unknown')}</td>
              <td>${ch.health?.latency_ms != null ? ch.health.latency_ms + 'ms' : '-'}</td>
              <td><button class="btn" onclick="DashboardPage.checkChannel(${ch.id})">检测</button></td>
            </tr>`;
          }
          return `<tr>
            <td>${ch.name || '-'}</td>
            <td>${ch.type || '-'}</td>
            <td>${ch.status === 1 ? '<span class="badge badge-healthy">启用</span>' : '<span class="badge badge-unknown">禁用</span>'}</td>
            <td>${statusBadge(ch.health?.status || 'unknown')}</td>
            <td><button class="btn" onclick="DashboardPage.checkChannel(${ch.id})">检测</button></td>
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
