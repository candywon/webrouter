/* 健康监控页面逻辑 */
const MonitorPage = {
  async load() {
    // 绑定全部检测按钮
    const btn = document.getElementById('btn-check-all');
    if (btn) {
      btn.onclick = () => this.checkAll();
    }

    try {
      const data = await API.get('/monitor/channels');
      this.renderChannels(data.channels || []);
    } catch (e) {
      console.error('Failed to load monitor channels:', e);
    }
  },

  renderChannels(channels) {
    const el = document.getElementById('monitor-content');
    if (!el) return;
    if (channels.length === 0) {
      el.innerHTML = '<div class="empty-state"><div class="icon">💓</div><p>暂无渠道数据<br>请先在 New-API 中添加渠道</p></div>';
      return;
    }
    el.innerHTML = `<table>
      <thead><tr><th>渠道名</th><th>类型</th><th>健康状态</th><th>延迟</th><th>最近检测</th><th>操作</th></tr></thead>
      <tbody>${channels.map(ch => `
        <tr>
          <td>${ch.name || '-'}</td>
          <td>${ch.type || '-'}</td>
          <td>${statusBadge(ch.health?.status || 'unknown')}</td>
          <td>${ch.health?.latency_ms != null ? ch.health.latency_ms + 'ms' : '-'}</td>
          <td>${formatDate(ch.health?.checked_at)}</td>
          <td>
            <button class="btn" onclick="MonitorPage.checkChannel(${ch.channel_id})">检测</button>
            <button class="btn" onclick="MonitorPage.showHistory(${ch.channel_id})">历史</button>
          </td>
        </tr>
      `).join('')}</tbody>
    </table>`;
  },

  async checkChannel(id) {
    try {
      const result = await API.post(`/monitor/check/${id}`);
      showToast(`渠道检测完成: ${result.status}, 延迟 ${result.latency_ms ?? '-'}ms`);
      this.load();
    } catch (e) {
      showToast('检测失败: ' + e.message);
    }
  },

  async checkAll() {
    try {
      const result = await API.post('/monitor/check_all');
      showToast(`全部检测完成, 共 ${result.results?.length || 0} 个渠道`);
      this.load();
    } catch (e) {
      showToast('全部检测失败: ' + e.message);
    }
  },

  async showHistory(id) {
    try {
      const data = await API.get(`/monitor/history/${id}`);
      const history = data.history || [];
      const el = document.getElementById('monitor-content');
      if (!el) return;

      // Render history inline below the channel table
      const existing = document.getElementById('monitor-history');
      if (existing) existing.remove();

      const histDiv = document.createElement('div');
      histDiv.id = 'monitor-history';
      histDiv.className = 'card';
      histDiv.style.marginTop = '16px';
      histDiv.innerHTML = `
        <div class="card-header">
          <span class="card-title">检测历史</span>
          <button class="btn" onclick="document.getElementById('monitor-history').remove()">关闭</button>
        </div>
        ${history.length === 0 ? '<div class="empty-state"><p>暂无历史记录</p></div>' : `
        <table>
          <thead><tr><th>时间</th><th>状态</th><th>延迟</th><th>错误信息</th></tr></thead>
          <tbody>${history.map(h => `
            <tr>
              <td>${formatDate(h.checked_at)}</td>
              <td>${statusBadge(h.status || 'unknown')}</td>
              <td>${h.latency_ms != null ? h.latency_ms + 'ms' : '-'}</td>
              <td>${h.error_message || '-'}</td>
            </tr>
          `).join('')}</tbody>
        </table>`}`;
      el.parentNode.insertBefore(histDiv, el.nextSibling);
    } catch (e) {
      showToast('加载历史失败: ' + e.message);
    }
  },
};
