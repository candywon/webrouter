/* 工具函数 */

function formatNumber(n) {
  if (n === null || n === undefined) return '-';
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
  return n.toString();
}

function formatYuan(cents) {
  if (!cents) return '¥0.00';
  return '¥' + (cents / 100).toFixed(2);
}

function statusBadge(status) {
  const map = {
    healthy: ['● 正常', 'badge-healthy'],
    warning: ['⚠ 警告', 'badge-warning'],
    dead: ['✕ 失效', 'badge-dead'],
    rate_limited: ['⏱ 限速', 'badge-warning'],
    auth_failed: ['✕ 认证失败', 'badge-dead'],
    timeout: ['⏱ 超时', 'badge-warning'],
    unhealthy: ['✕ 异常', 'badge-dead'],
    disabled: ['⏸ 已禁用', 'badge-unknown'],
    unknown: ['? 未知', 'badge-unknown'],
    unchecked: ['○ 未检测', 'badge-unknown'],
  };
  const [text, cls] = map[status] || ['? ' + status, 'badge-unknown'];
  return `<span class="badge ${cls}">${text}</span>`;
}

function copyToClipboard(text) {
  navigator.clipboard.writeText(text).then(() => {
    showToast('已复制到剪贴板');
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
