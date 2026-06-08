// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 健康监控页面逻辑 */
const MonitorPage = {
  cooldownTimer: null,

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

    // 加载冷却状态
    this.loadCooldowns();

    // 加载 Provider 延迟对比
    this.loadProviderComparison();
  },

  async loadProviderComparison() {
    try {
      const data = await API.get('/monitoring/provider-comparison?hours=24');
      this.renderProviderComparison(data);
    } catch (e) {
      console.error('Failed to load provider comparison:', e);
    }
  },

  renderProviderComparison(data) {
    const canvas = document.getElementById('provider-comparison-chart');
    if (!canvas || !data.data || data.data.length === 0) return;

    if (this._providerChart) this._providerChart.destroy();

    const sorted = [...data.data].sort((a, b) => b.avg_latency_ms - a.avg_latency_ms);
    const labels = sorted.map(d => d.name);

    this._providerChart = new Chart(canvas, {
      type: 'bar',
      data: {
        labels: labels,
        datasets: [
          {
            label: I18n.t('common.avgLatency') + ' (ms)',
            data: sorted.map(d => d.avg_latency_ms),
            backgroundColor: 'rgba(99,102,241,0.7)',
            borderRadius: 3,
          },
          {
            label: I18n.t('billing.errorCount'),
            data: sorted.map(d => d.error_count),
            backgroundColor: 'rgba(239,68,68,0.6)',
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

  async loadCooldowns() {
    try {
      const data = await API.get('/providers/cooldowns');
      this.renderCooldowns(data.cooldowns || []);
    } catch (e) {
      console.error('Failed to load cooldowns:', e);
    }
  },

  renderCooldowns(cooldowns) {
    // 移除旧的冷却卡片
    const existing = document.getElementById('cooldown-card');
    if (existing) existing.remove();

    if (!cooldowns.length) return;

    const el = document.getElementById('monitor-content');
    if (!el) return;

    const card = document.createElement('div');
    card.id = 'cooldown-card';
    card.className = 'card';
    card.style.marginBottom = '16px';

    let html = `<div class="card-header"><span class="card-title">⏳ ${I18n.t('monitor.coolingProviders')}</span><button class="btn-icon" onclick="MonitorPage.loadCooldowns()" title="Refresh">🔄</button></div>`;
    html += `<table><thead><tr><th>Provider</th><th>${I18n.t('common.status')}</th><th>${I18n.t('monitor.remainingTime')}</th><th>${I18n.t('common.actions')}</th></tr></thead><tbody>`;
    cooldowns.forEach(cd => {
      const secs = cd.cooldown_remaining_sec;
      const timeStr = formatCooldown(secs);
      html += `<tr>
        <td><strong>${esc(cd.name)}</strong></td>
        <td><span class="badge badge-warning">${I18n.t('monitor.cooling')}</span></td>
        <td id="cooldown-${cd.provider_id}">${timeStr}</td>
        <td><button class="btn btn-sm" onclick="MonitorPage.clearCooldown(${cd.provider_id})">${I18n.t('monitor.clearCooldown')}</button></td>
      </tr>`;
    });
    html += '</tbody></table>';
    card.innerHTML = html;

    // 插入到最前面
    el.parentNode.insertBefore(card, el);

    // 启动倒计时刷新
    this.startCooldownTimer(cooldowns);
  },

  startCooldownTimer(cooldowns) {
    if (this.cooldownTimer) clearInterval(this.cooldownTimer);
    this.cooldownTimer = setInterval(() => {
      cooldowns.forEach(cd => {
        const el = document.getElementById(`cooldown-${cd.provider_id}`);
        if (el && cd.cooldown_remaining_sec > 0) {
          cd.cooldown_remaining_sec--;
          el.textContent = formatCooldown(cd.cooldown_remaining_sec);
        }
      });
      // 全部过期后重新加载
      if (cooldowns.every(cd => cd.cooldown_remaining_sec <= 0)) {
        clearInterval(this.cooldownTimer);
        this.cooldownTimer = null;
        this.loadCooldowns();
      }
    }, 1000);
  },

  async clearCooldown(providerId) {
    try {
      await API.post(`/providers/${providerId}/clear_cooldown`);
      showToast(I18n.t("monitor.cooldownCleared"));
      this.loadCooldowns();
      this.load();
    } catch (e) {
      showToast(I18n.t("monitor.clearFailed") + e.message);
    }
  },

  renderChannels(channels) {
    const el = document.getElementById('monitor-content');
    if (!el) return;
    if (channels.length === 0) {
      el.innerHTML = `<div class="empty-state"><div class="icon">💓</div><p>${I18n.t('dashboard.noChannelData')}<br>${I18n.t('common.goToChannel')}</p></div>`;
      return;
    }
    el.innerHTML = `<table>
      <thead><tr><th>${I18n.t('dashboard.channelTableHeader')}</th><th>${I18n.t('dashboard.channelTypeHeader')}</th><th>${I18n.t('dashboard.channelHealthHeader')}</th><th>${I18n.t('dashboard.channelLatencyHeader')}</th><th>${I18n.t('monitor.lastCheck')}</th><th>${I18n.t('common.actions')}</th></tr></thead>
      <tbody>${channels.map(ch => `
        <tr>
          <td>${ch.name || '-'}</td>
          <td>${ch.type || '-'}</td>
          <td>${statusBadge(ch.health?.status || 'unknown')}</td>
          <td>${ch.health?.latency_ms != null ? ch.health.latency_ms + 'ms' : '-'}</td>
          <td>${formatDate(ch.health?.checked_at)}</td>
          <td>
            <button class="btn" onclick="MonitorPage.checkChannel(${ch.provider_id})">${I18n.t('common.check')}</button>
            <button class="btn" onclick="MonitorPage.showHistory(${ch.provider_id})">${I18n.t('monitor.history')}</button>
          </td>
        </tr>
      `).join('')}</tbody>
    </table>`;
  },

  async checkChannel(id) {
    try {
      const result = await API.post(`/monitor/check/${id}`);
      showToast(I18n.t('monitor.checkDone', {status: result.status, latency: result.latency_ms ?? '-'}));
      this.load();
    } catch (e) {
      showToast(I18n.t('common.checkFailed') + e.message);
    }
  },

  async checkAll() {
    const btn = document.getElementById('btn-check-all');
    if (!btn) return;

    // 禁用按钮并显示检测中状态
    btn.disabled = true;
    btn.style.opacity = '0.7';
    btn.style.cursor = 'not-allowed';
    btn.textContent = I18n.t('common.processing');

    try {
      const result = await API.post('/monitor/check_all');
      const count = result.results?.length || 0;
      showToast(I18n.t('monitor.checkAllDone', {count: count}));
      this.load();
    } catch (e) {
      showToast(I18n.t("monitor.checkAllFailed") + e.message);
    } finally {
      // 恢复按钮状态
      btn.disabled = false;
      btn.style.opacity = '';
      btn.style.cursor = '';
      btn.textContent = I18n.t('monitor.checkAll');
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
          <span class="card-title">${I18n.t('monitor.checkHistory')}</span>
          <button class="btn" onclick="document.getElementById('monitor-history').remove()">${I18n.t('common.close')}</button>
        </div>
        ${history.length === 0 ? `<div class="empty-state"><p>${I18n.t('monitor.noHistory')}</p></div>` : `
        <table>
          <thead><tr><th>${I18n.t('common.time')}</th><th>${I18n.t('common.status')}</th><th>${I18n.t('common.latency')}</th><th>${I18n.t('monitor.errorMessage')}</th></tr></thead>
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
      showToast(I18n.t('monitor.loadHistoryFailed') + e.message);
    }
  },
};
