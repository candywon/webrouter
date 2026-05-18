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
    let path = window.location.hash.slice(1) || '/';

    // 支持 /providers/:id/channels 子路由
    if (path.match(/^\/providers\/\d+\/channels/)) {
      path = '/provider-channels';
    }

    const handler = this.routes[path];
    if (!handler) {
      this.navigate('/');
      return;
    }

    // 隐藏所有页面，显示当前
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    // 映射路由到页面容器
    let pageId = 'page-' + path.slice(1);
    if (pageId === 'page-provider-channels') pageId = 'page-channels';
    const pageEl = document.getElementById(pageId);
    if (pageEl) pageEl.classList.add('active');

    // 更新侧边栏高亮
    document.querySelectorAll('.sidebar-nav a, .nav-group-items a').forEach(a => {
      const href = a.getAttribute('href').slice(1);
      // /providers/:id/channels 子路由高亮 /channels
      let activePath = path;
      if (activePath === '/provider-channels') activePath = '/channels';
      a.classList.toggle('active', href === activePath);
    });

    // 自动展开包含 active 项的分组
    document.querySelectorAll('.nav-group').forEach(group => {
      const hasActive = group.querySelector('a.active');
      group.classList.toggle('open', !!hasActive);
    });

    // 更新标题
    const titles = {
      '/': '仪表盘',
      '/providers': '数据源管理',
      '/channels': '渠道管理',
      '/provider-channels': '渠道管理',
      '/tokens': '令牌管理',
      '/pricing': '模型定价',
      '/modelgrades': '模型分级',
      '/desensitize': '脱敏规则',
      '/monitor': '健康监控',
      '/alerts': '告警规则',
      '/billing': '计费统计',
      '/team': '团队管理',
      '/cli': 'CLI 对接',
      '/reqcache': '请求缓存',
      '/api-test': 'API 测试',
      '/settings': '系统设置',
    };
    document.getElementById('header-title').textContent = titles[path] || 'AI-API综合管理平台';

    this.current = path;
    if (handler.load) handler.load();
  },
};

/** 切换导航分组展开/收起 */
function toggleNavGroup(header) {
  header.parentElement.classList.toggle('open');
}
