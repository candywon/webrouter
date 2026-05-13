/* 设置页面逻辑 — 必须在路由注册前定义 */
const SettingsPage = {
  async load() {
    try {
      const data = await API.get('/settings/');
      const el = document.getElementById('settings-content');
      if (!el) return;
      el.innerHTML = `
        <div class="card">
          <div class="card-header"><span class="card-title">系统配置</span></div>
          <table>
            <tr><td>New-API 地址</td><td>${data.newapi_url || '-'}</td></tr>
            <tr><td>健康检测间隔</td><td>${data.health_check_interval || 300}s</td></tr>
            <tr><td>告警冷却时间</td><td>${data.alert_cooldown || 600}s</td></tr>
            <tr><td>时区</td><td>${data.timezone || 'Asia/Shanghai'}</td></tr>
          </table>
        </div>
        <div class="card">
          <div class="card-header"><span class="card-title">数据备份</span></div>
          <button class="btn btn-primary" onclick="SettingsPage.backup()">创建备份</button>
        </div>`;
    } catch (e) {
      console.error('Failed to load settings:', e);
    }
  },

  async backup() {
    try {
      const result = await API.post('/settings/backup');
      showToast('备份成功: ' + (result.backup || ''));
    } catch (e) { showToast('备份失败'); }
  },
};

/* 应用入口 */
(function () {
  Router.register('/', DashboardPage);
  Router.register('/providers', providersPage);
  Router.register('/channels', ChannelsPage);
  Router.register('/monitor', MonitorPage);
  Router.register('/alerts', AlertPage);
  Router.register('/billing', BillingPage);
  Router.register('/team', TeamPage);
  Router.register('/cli', CLIPage);
  Router.register('/settings', SettingsPage);
  Router.init();
})();
