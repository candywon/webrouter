// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 工具函数 */

function formatNumber(n) {
  if (n === null || n === undefined) return '-';
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
  return n.toString();
}

function formatYuan(cents) {
  const sym = (I18n.lang || '').startsWith('zh') ? '¥' : '$';
  if (!cents) return sym + '0.00';
  return sym + (cents / 100).toFixed(2);
}

function statusBadge(status) {
  const map = {
    healthy: [I18n.t("common.statusHealthy"), 'badge-healthy'],
    warning: [I18n.t("common.statusWarning"), 'badge-warning'],
    dead: [I18n.t("common.statusDead"), 'badge-dead'],
    rate_limited: [I18n.t("common.statusRateLimited"), 'badge-warning'],
    auth_failed: [I18n.t("common.statusAuthFailed"), 'badge-dead'],
    timeout: [I18n.t("common.statusTimeout"), 'badge-warning'],
    unhealthy: [I18n.t("common.statusUnhealthy"), 'badge-dead'],
    disabled: [I18n.t("common.statusDisabled"), 'badge-unknown'],
    unknown: [I18n.t("common.statusUnknown"), 'badge-unknown'],
    unchecked: [I18n.t("common.statusUnchecked"), 'badge-unknown'],
  };
  const [text, cls] = map[status] || ['? ' + status, 'badge-unknown'];
  return `<span class="badge ${cls}">${text}</span>`;
}

function copyToClipboard(text) {
  navigator.clipboard.writeText(text).then(() => {
    showToast(I18n.t("common.copiedToClipboard"));
  });
}

function showToast(msg, duration = 2000) {
  const el = document.createElement('div');
  el.textContent = msg;
  el.style.cssText = `
    position:fixed;bottom:24px;left:50%;transform:translateX(-50%);
    background:var(--bg-card);color:var(--text-primary);
    padding:10px 20px;border-radius:8px;font-size:14px;
    box-shadow:0 4px 12px rgba(0,0,0,0.4);z-index:9999;
    border:1px solid var(--border);
  `;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), duration);
}

function formatDate(iso) {
  if (!iso) return '-';
  const d = new Date(iso);
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

// HTML 转义
function esc(str) {
  if (!str) return '';
  return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// 格式化冷却倒计时
function formatCooldown(secs) {
  if (secs <= 0) return I18n.t('common.expired');
  if (secs < 60) return secs + I18n.t("common.seconds");
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return m + I18n.t('common.minutes') + s + I18n.t("common.seconds");
}
