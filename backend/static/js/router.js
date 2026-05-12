/* Hash路由 */
const Router = {
  routes: {},
  current: null,

  register(path, handler) {
    this.routes[path] = handler;
  },

  navigate(path) {
    window.location.hash = path;
  },

  init() {
    window.addEventListener('hashchange', () => this.resolve());
    this.resolve();
  },

  resolve() {
    const path = window.location.hash.slice(1) || '/';
    const handler = this.routes[path];
    if (!handler) {
      this.navigate('/');
      return;
    }

    // 隐藏所有页面，显示当前
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    const pageEl = document.getElementById('page-' + path.slice(1) || 'page-dashboard');
    if (pageEl) pageEl.classList.add('active');

    // 更新侧边栏高亮
    document.querySelectorAll('.sidebar-nav a').forEach(a => {
      a.classList.toggle('active', a.getAttribute('href') === '#' + path);
    });

    // 更新标题
    const titles = {
      '/': '仪表盘',
      '/channels': '渠道管理',
      '/monitor': '健康监控',
      '/alerts': '告警规则',
      '/billing': '计费统计',
      '/team': '团队管理',
      '/cli': 'CLI 对接',
      '/settings': '系统设置',
    };
    document.getElementById('header-title').textContent = titles[path] || 'WebRouter';

    this.current = path;
    if (handler.load) handler.load();
  },
};
