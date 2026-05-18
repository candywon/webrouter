/**
 * 请求 Hash 缓存管理页面
 */
const reqCachePage = {
  entries: [],

  init() {
    this.refresh();
  },

  load() {
    this.refresh();
  },

  async refresh() {
    try {
      const data = await API.get('/providers/request_cache');
      this.entries = data.entries || [];
      this.render();
    } catch (e) {
      console.error('Failed to load request cache:', e);
      document.getElementById('reqcache-content').innerHTML =
        '<div class="empty-state"><div class="icon">❌</div><p>加载失败: ' + esc(e.message) + '</p></div>';
    }
  },

  render() {
    const el = document.getElementById('reqcache-content');
    const countEl = document.getElementById('reqcache-count');
    if (!el) return;

    if (countEl) countEl.textContent = this.entries.length + ' 条';

    if (!this.entries.length) {
      el.innerHTML = '<div class="empty-state"><div class="icon">🔄</div><p>暂无缓存条目</p></div>';
      return;
    }

    // 按时间排序：最新的在前
    const sorted = [...this.entries].sort((a, b) => b.age_seconds - a.age_seconds);

    let html = '<table><thead><tr><th>Key (Token:Model)</th><th>Body Hash</th><th>状态</th><th>年龄</th><th>时间</th></tr></thead><tbody>';
    sorted.forEach(entry => {
      const badge = entry.success
        ? '<span class="badge badge-success">成功</span>'
        : '<span class="badge badge-danger">失败</span>';
      const age = formatAge(entry.age_seconds);
      const time = entry.timestamp ? formatDate(entry.timestamp) : '-';
      html += `<tr>
        <td><strong>${esc(entry.key)}</strong></td>
        <td><code>${esc(entry.hash)}</code></td>
        <td>${badge}</td>
        <td>${age}</td>
        <td>${time}</td>
      </tr>`;
    });
    html += '</tbody></table>';
    el.innerHTML = html;
  },

  async clearAll() {
    if (!confirm('确定清空所有请求缓存条目？')) return;
    try {
      await API.del('/providers/request_cache');
      showToast('缓存已清空');
      this.refresh();
    } catch (e) {
      showToast('清空失败: ' + e.message);
    }
  },
};

function formatAge(secs) {
  if (secs == null) return '-';
  if (secs < 60) return secs + '秒前';
  const m = Math.floor(secs / 60);
  if (m < 60) return m + '分钟前';
  const h = Math.floor(m / 60);
  return h + '小时' + (m % 60) + '分钟前';
}
