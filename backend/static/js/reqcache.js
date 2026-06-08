// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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
        '<div class="empty-state"><div class="icon">❌</div><p>'+ I18n.t('common.loadFailedError') + esc(e.message) + '</p></div>';
    }
  },

  render() {
    const el = document.getElementById('reqcache-content');
    const countEl = document.getElementById('reqcache-count');
    if (!el) return;

    if (countEl) countEl.textContent = this.entries.length + I18n.t('common.entriesUnit');

    if (!this.entries.length) {
      el.innerHTML = '<div class="empty-state"><div class="icon">🔄</div><p>'+ I18n.t('reqcache.noEntries') + '</p></div>';
      return;
    }

    // 按时间排序：最新的在前
    const sorted = [...this.entries].sort((a, b) => b.age_seconds - a.age_seconds);

    let html = '<table><thead><tr><th>Key (Token:Model)</th><th>Body Hash</th><th>' + I18n.t('common.status') + '</th><th>' + I18n.t('reqcache.age') + '</th><th>' + I18n.t('common.time') + '</th></tr></thead><tbody>';
    sorted.forEach(entry => {
      const badge = entry.success
        ? '<span class="badge badge-success">' + I18n.t('common.success') + '</span>'
        : '<span class="badge badge-danger">' + I18n.t('common.failed') + '</span>';
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
    if (!confirm(I18n.t("reqcache.confirmClear"))) return;
    try {
      await API.del('/providers/request_cache');
      showToast(I18n.t("reqcache.cleared"));
      this.refresh();
    } catch (e) {
      showToast(I18n.t("reqcache.clearFailed") + e.message);
    }
  },
};

function formatAge(secs) {
  if (secs == null) return '-';
  if (secs < 60) return secs + I18n.t("reqcache.secondsAgo");
  const m = Math.floor(secs / 60);
  if (m < 60) return m + I18n.t("reqcache.minutesAgo");
  const h = Math.floor(m / 60);
  return h + I18n.t("common.hours") + (m % 60) + I18n.t("reqcache.minutesAgo");
}
