// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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

    document.body.classList.toggle('login-mode', path === '/login');

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
      '/login': I18n.t("router.login"),
      '/': I18n.t("nav.dashboard"),
      '/providers': I18n.t("nav.providers"),
      '/channels': I18n.t("nav.channels"),
      '/provider-channels': I18n.t("nav.channels"),
      '/tokens': I18n.t("nav.tokens"),
      '/pricing': I18n.t("nav.pricing"),
      '/modelgrades': I18n.t("nav.modelgrades"),
      '/desensitize': I18n.t("nav.desensitize"),
      '/monitor': I18n.t("nav.monitor"),
      '/alerts': I18n.t("nav.alerts"),
      '/billing': I18n.t("nav.billing"),
      '/team': I18n.t("nav.team"),
      '/cli': I18n.t("nav.cli"),
      '/reqcache': I18n.t("nav.reqcache"),
      '/api-test': I18n.t("nav.apiTest"),
      '/knowledge': I18n.t("nav.knowledge"),
      '/settings': I18n.t("nav.settings"),
    };
    const titleEl = document.getElementById('header-title');
    if (titleEl) titleEl.textContent = titles[path] || I18n.t("router.defaultTitle");

    this.current = path;
    if (handler.load) handler.load();
  },
};

/** 切换导航分组展开/收起 */
function toggleNavGroup(header) {
  header.parentElement.classList.toggle('open');
}
