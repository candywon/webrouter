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

    let html = '<div class="card-header"><span class="card-title">⏳ 冷却中的 Provider</span><button class="btn-icon" onclick="MonitorPage.loadCooldowns()" title="刷新">🔄</button></div>';
    html += '<table><thead><tr><th>Provider</th><th>状态</th><th>剩余时间</th><th>操作</th></tr></thead><tbody>';
    cooldowns.forEach(cd => {
      const secs = cd.cooldown_remaining_sec;
      const timeStr = formatCooldown(secs);
      html += `<tr>
        <td><strong>${esc(cd.name)}</strong></td>
        <td><span class="badge badge-warning">冷却中</span></td>
        <td id="cooldown-${cd.provider_id}">${timeStr}</td>
        <td><button class="btn btn-sm" onclick="MonitorPage.clearCooldown(${cd.provider_id})">清除冷却</button></td>
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
      showToast('冷却已清除');
      this.loadCooldowns();
      this.load();
    } catch (e) {
      showToast('清除失败: ' + e.message);
    }
  },

  renderChannels(channels) {
    const el = document.getElementById('monitor-content');
    if (!el) return;
    if (channels.length === 0) {
      el.innerHTML = '<div class="empty-state"><div class="icon">💓</div><p>暂无渠道数据<br>请先在"渠道管理"中添加渠道</p></div>';
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
            <button class="btn" onclick="MonitorPage.checkChannel(${ch.provider_id})">检测</button>
            <button class="btn" onclick="MonitorPage.showHistory(${ch.provider_id})">历史</button>
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
    const btn = document.getElementById('btn-check-all');
    if (!btn) return;

    // 禁用按钮并显示检测中状态
    btn.disabled = true;
    btn.style.opacity = '0.7';
    btn.style.cursor = 'not-allowed';
    btn.textContent = '检测中...';

    try {
      const result = await API.post('/monitor/check_all');
      const count = result.results?.length || 0;
      showToast(`全部检测完成，共 ${count} 个渠道`);
      this.load();
    } catch (e) {
      showToast('全部检测失败: ' + e.message);
    } finally {
      // 恢复按钮状态
      btn.disabled = false;
      btn.style.opacity = '';
      btn.style.cursor = '';
      btn.textContent = '全部检测';
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
